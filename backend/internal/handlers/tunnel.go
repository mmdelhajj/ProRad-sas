package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

const tunnelStateFile = "/var/lib/proxpanel/.tunnel_state"

// TunnelHandler manages the Cloudflare Tunnel (Remote Access) feature
type TunnelHandler struct{}

// NewTunnelHandler creates a new TunnelHandler instance
func NewTunnelHandler() *TunnelHandler {
	return &TunnelHandler{}
}

// getCFCredentials reads CF credentials from system_preferences DB first, then falls back to env vars.
func getCFCredentials() (cfToken, cfZoneID, cfDomain string) {
	getDBPref := func(key string) string {
		var pref models.SystemPreference
		if err := database.DB.Where("key = ?", key).First(&pref).Error; err == nil && pref.Value != "" {
			return pref.Value
		}
		return ""
	}
	cfToken = getDBPref("cf_api_token")
	if cfToken == "" {
		cfToken = os.Getenv("CF_API_TOKEN")
	}
	cfZoneID = getDBPref("cf_zone_id")
	if cfZoneID == "" {
		cfZoneID = os.Getenv("CF_ZONE_ID")
	}
	cfDomain = getDBPref("cf_domain")
	if cfDomain == "" {
		cfDomain = os.Getenv("CF_DOMAIN")
	}
	if cfDomain == "" {
		cfDomain = "proxrad.com"
	}
	return
}

// SaveCFCredentials saves Cloudflare credentials to system_preferences DB.
func (h *TunnelHandler) SaveCFCredentials(c *fiber.Ctx) error {
	var req struct {
		CFAPIToken string `json:"cf_api_token"`
		CFZoneID   string `json:"cf_zone_id"`
		CFDomain   string `json:"cf_domain"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request"})
	}

	savePref := func(key, value string) {
		var pref models.SystemPreference
		if database.DB.Where("key = ?", key).First(&pref).Error != nil {
			database.DB.Create(&models.SystemPreference{Key: key, Value: value})
		} else {
			database.DB.Model(&pref).Update("value", value)
		}
	}

	if req.CFAPIToken != "" {
		savePref("cf_api_token", req.CFAPIToken)
	}
	if req.CFZoneID != "" {
		savePref("cf_zone_id", req.CFZoneID)
	}
	if req.CFDomain != "" {
		savePref("cf_domain", req.CFDomain)
	}

	return c.JSON(fiber.Map{"success": true, "message": "Cloudflare credentials saved"})
}

// getTunnelSubdomain generates a subdomain from the server's hardware ID.
// It reads HARDWARE_ID env var (format: stable_abc123def456...), takes the last
// 6 chars of the part after "stable_". Falls back to first 6 chars of /etc/machine-id.
func getTunnelSubdomain() string {
	suffix := ""

	hwID := os.Getenv("HARDWARE_ID")
	if hwID != "" {
		// Format: stable_<hexchars>
		if strings.HasPrefix(hwID, "stable_") {
			rest := strings.TrimPrefix(hwID, "stable_")
			if len(rest) >= 6 {
				suffix = rest[len(rest)-6:]
			} else if len(rest) > 0 {
				suffix = rest
			}
		}
	}

	if suffix == "" {
		// Fallback: read /etc/machine-id
		data, err := os.ReadFile("/etc/machine-id")
		if err == nil {
			mid := strings.TrimSpace(string(data))
			if len(mid) >= 6 {
				suffix = mid[:6]
			} else if len(mid) > 0 {
				suffix = mid
			}
		}
	}

	if suffix == "" {
		suffix = "000000"
	}

	return "panel-" + suffix
}

// loadTunnelState reads and parses the tunnel state JSON file.
// Returns an empty map if the file does not exist or cannot be parsed.
func loadTunnelState() map[string]string {
	state := make(map[string]string)
	data, err := os.ReadFile(tunnelStateFile)
	if err != nil {
		return state
	}
	if err := json.Unmarshal(data, &state); err != nil {
		log.Printf("TunnelHandler: failed to parse state file: %v", err)
		return make(map[string]string)
	}
	return state
}

// saveTunnelState writes the state map to the tunnel state JSON file.
func saveTunnelState(state map[string]string) error {
	// Ensure the directory exists
	dir := "/var/lib/proxpanel"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal tunnel state: %w", err)
	}
	if err := os.WriteFile(tunnelStateFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write tunnel state file: %w", err)
	}
	return nil
}

// cfRequest makes an authenticated request to the Cloudflare API.
// method is the HTTP method, url is the full API URL, token is the CF API token,
// and body (if non-nil) is marshalled to JSON. Returns the parsed JSON response.
func cfRequest(method, url, token string, body interface{}) (map[string]interface{}, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respData, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Cloudflare response (status %d): %w", resp.StatusCode, err)
	}

	return result, nil
}

// GetTunnelStatus returns the current status of the Cloudflare tunnel.
// Response: {"success": true, "enabled": bool, "running": bool, "url": string|null, "subdomain": string}
func (h *TunnelHandler) GetTunnelStatus(c *fiber.Ctx) error {
	state := loadTunnelState()

	// Determine if the tunnel is enabled (tunnel_id present in state)
	tunnelID := state["tunnel_id"]
	enabled := tunnelID != ""

	// Check if cloudflared service is running (use nsenter to check on host)
	out, err := exec.Command("nsenter", "-t", "1", "-m", "-u", "-i", "-n", "--", "systemctl", "is-active", "cloudflared").Output()
	running := err == nil && strings.TrimSpace(string(out)) == "active"

	var tunnelURL interface{}
	if url, ok := state["tunnel_url"]; ok && url != "" {
		tunnelURL = url
	}

	subdomain := state["subdomain"]
	if subdomain == "" {
		subdomain = getTunnelSubdomain()
	}

	cfToken, cfZoneID, cfDomain := getCFCredentials()
	credentialsSet := cfToken != "" && cfZoneID != ""

	return c.JSON(fiber.Map{
		"success":          true,
		"enabled":          enabled,
		"running":          running,
		"url":              tunnelURL,
		"subdomain":        subdomain,
		"credentials_set":  credentialsSet,
		"cf_domain":        cfDomain,
	})
}

// EnableTunnel creates or re-enables the Cloudflare tunnel.
// Steps:
//  1. Validate CF_API_TOKEN and CF_ZONE_ID env vars
//  2. Generate subdomain
//  3. Return early if already enabled and running
//  4. Get CF Account ID from zone info
//  5. Create tunnel (if needed)
//  6. Get tunnel token
//  7. Configure tunnel ingress
//  8. Create/update DNS CNAME record
//  9. Write systemd service file
//  10. daemon-reload, enable, restart cloudflared
//  11. Wait 3s, verify running
//  12. Save state
func (h *TunnelHandler) EnableTunnel(c *fiber.Ctx) error {
	cfToken, cfZoneID, cfDomain := getCFCredentials()

	if cfToken == "" || cfZoneID == "" {
		log.Println("TunnelHandler: CF_API_TOKEN or CF_ZONE_ID not set")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cloudflare credentials not configured. Please enter your CF API Token and Zone ID in the Remote Access settings.",
		})
	}

	subdomain := getTunnelSubdomain()
	tunnelURL := fmt.Sprintf("https://%s.%s", subdomain, cfDomain)
	tunnelName := fmt.Sprintf("proxpanel-%s", subdomain)
	fqdn := fmt.Sprintf("%s.%s", subdomain, cfDomain)

	// Load existing state
	state := loadTunnelState()
	tunnelID := state["tunnel_id"]

	// If already enabled and running, return success immediately
	if tunnelID != "" {
		out, err := exec.Command("nsenter", "-t", "1", "-m", "-u", "-i", "-n", "--", "systemctl", "is-active", "cloudflared").Output()
		if err == nil && strings.TrimSpace(string(out)) == "active" {
			return c.JSON(fiber.Map{
				"success":   true,
				"url":       tunnelURL,
				"subdomain": subdomain,
				"message":   "Remote access is already enabled",
			})
		}
	}

	// Step 4: Get CF Account ID from zone info
	log.Printf("TunnelHandler: Fetching account ID for zone %s", cfZoneID)
	zoneURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s", cfZoneID)
	zoneResp, err := cfRequest("GET", zoneURL, cfToken, nil)
	if err != nil {
		log.Printf("TunnelHandler: Failed to get zone info: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to get Cloudflare zone info: %v", err),
		})
	}

	result, ok := zoneResp["result"].(map[string]interface{})
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Invalid response from Cloudflare zone API",
		})
	}
	account, ok := result["account"].(map[string]interface{})
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Could not extract account info from Cloudflare response",
		})
	}
	accountID, ok := account["id"].(string)
	if !ok || accountID == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Could not extract account ID from Cloudflare response",
		})
	}
	log.Printf("TunnelHandler: Account ID: %s", accountID)

	// Step 5: Create tunnel if no tunnel_id in state
	if tunnelID == "" {
		log.Printf("TunnelHandler: Creating tunnel '%s'", tunnelName)
		createURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel", accountID)
		createBody := map[string]interface{}{
			"name":       tunnelName,
			"config_src": "cloudflare",
		}
		createResp, err := cfRequest("POST", createURL, cfToken, createBody)
		if err != nil {
			log.Printf("TunnelHandler: Failed to create tunnel: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": fmt.Sprintf("Failed to create Cloudflare tunnel: %v", err),
			})
		}
		tunnelResult, ok := createResp["result"].(map[string]interface{})
		if !ok {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Invalid response from Cloudflare create tunnel API",
			})
		}
		tunnelID, ok = tunnelResult["id"].(string)
		if !ok || tunnelID == "" {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Could not extract tunnel ID from Cloudflare response",
			})
		}
		log.Printf("TunnelHandler: Created tunnel ID: %s", tunnelID)
	}

	// Step 6: Get tunnel token
	log.Printf("TunnelHandler: Fetching token for tunnel %s", tunnelID)
	tokenURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s/token", accountID, tunnelID)
	tokenResp, err := cfRequest("GET", tokenURL, cfToken, nil)
	if err != nil {
		log.Printf("TunnelHandler: Failed to get tunnel token: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to get tunnel token: %v", err),
		})
	}
	tunnelToken, ok := tokenResp["result"].(string)
	if !ok || tunnelToken == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Could not extract tunnel token from Cloudflare response",
		})
	}
	log.Printf("TunnelHandler: Retrieved tunnel token (length: %d)", len(tunnelToken))

	// Step 7: Configure tunnel ingress
	log.Printf("TunnelHandler: Configuring tunnel ingress for %s", fqdn)
	configURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s/configurations", accountID, tunnelID)
	configBody := map[string]interface{}{
		"config": map[string]interface{}{
			"ingress": []map[string]interface{}{
				{
					"hostname": fqdn,
					"service":  "http://localhost:80",
				},
				{
					"service": "http_status:404",
				},
			},
		},
	}
	_, err = cfRequest("PUT", configURL, cfToken, configBody)
	if err != nil {
		log.Printf("TunnelHandler: Failed to configure tunnel ingress: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to configure tunnel ingress: %v", err),
		})
	}
	log.Printf("TunnelHandler: Tunnel ingress configured successfully")

	// Step 8: Create/update DNS CNAME record
	log.Printf("TunnelHandler: Checking DNS record for %s", fqdn)
	dnsListURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?name=%s&type=CNAME", cfZoneID, fqdn)
	dnsListResp, err := cfRequest("GET", dnsListURL, cfToken, nil)
	if err != nil {
		log.Printf("TunnelHandler: Failed to check DNS records: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to check DNS records: %v", err),
		})
	}

	dnsRecordBody := map[string]interface{}{
		"type":    "CNAME",
		"name":    fqdn,
		"content": fmt.Sprintf("%s.cfargotunnel.com", tunnelID),
		"proxied": true,
		"ttl":     1,
	}

	dnsResults, _ := dnsListResp["result"].([]interface{})
	if len(dnsResults) > 0 {
		// Update existing record
		existingRecord, ok := dnsResults[0].(map[string]interface{})
		if !ok {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Invalid DNS record response from Cloudflare",
			})
		}
		recordID, ok := existingRecord["id"].(string)
		if !ok || recordID == "" {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Could not extract DNS record ID",
			})
		}
		log.Printf("TunnelHandler: Updating existing DNS record %s", recordID)
		updateURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", cfZoneID, recordID)
		_, err = cfRequest("PUT", updateURL, cfToken, dnsRecordBody)
		if err != nil {
			log.Printf("TunnelHandler: Failed to update DNS record: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": fmt.Sprintf("Failed to update DNS record: %v", err),
			})
		}
	} else {
		// Create new record
		log.Printf("TunnelHandler: Creating DNS record for %s", fqdn)
		createDNSURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", cfZoneID)
		_, err = cfRequest("POST", createDNSURL, cfToken, dnsRecordBody)
		if err != nil {
			log.Printf("TunnelHandler: Failed to create DNS record: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": fmt.Sprintf("Failed to create DNS record: %v", err),
			})
		}
	}
	log.Printf("TunnelHandler: DNS record configured for %s → %s.cfargotunnel.com", fqdn, tunnelID)

	// Step 9: Write systemd service file (written to host via /proc/1/root path,
	// since the API runs in a Docker container with pid=host and privileged mode)
	serviceContent := fmt.Sprintf(`[Unit]
Description=ProxPanel Remote Access Tunnel
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/cloudflared tunnel --no-autoupdate run --token %s
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`, tunnelToken)

	// Write service file to host filesystem via /proc/1/root (available in privileged+pid=host mode)
	serviceFilePath := "/proc/1/root/etc/systemd/system/cloudflared.service"
	log.Printf("TunnelHandler: Writing systemd service to host at %s", serviceFilePath)
	if err := os.WriteFile(serviceFilePath, []byte(serviceContent), 0644); err != nil {
		// Fallback: try writing directly (works if running on host or in non-Docker env)
		serviceFilePath = "/etc/systemd/system/cloudflared.service"
		log.Printf("TunnelHandler: /proc/1/root write failed, trying %s: %v", serviceFilePath, err)
		if err2 := os.WriteFile(serviceFilePath, []byte(serviceContent), 0644); err2 != nil {
			log.Printf("TunnelHandler: Failed to write service file: %v", err2)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": fmt.Sprintf("Failed to write cloudflared service file: %v", err2),
			})
		}
	}

	// Step 10: daemon-reload, enable, restart — use nsenter to run on host
	// nsenter -t 1 -m -u -i -n -- systemctl ... runs inside host's namespaces
	hostSystemctl := func(args ...string) error {
		cmd := append([]string{"-t", "1", "-m", "-u", "-i", "-n", "--", "systemctl"}, args...)
		out, err := exec.Command("nsenter", cmd...).CombinedOutput()
		if err != nil {
			log.Printf("TunnelHandler: nsenter systemctl %v failed: %v — %s", args, err, string(out))
		}
		return err
	}

	log.Println("TunnelHandler: Running systemctl daemon-reload on host")
	hostSystemctl("daemon-reload")

	log.Println("TunnelHandler: Enabling cloudflared on host")
	hostSystemctl("enable", "cloudflared")

	log.Println("TunnelHandler: Restarting cloudflared on host")
	hostSystemctl("restart", "cloudflared")

	// Step 11: Wait 3 seconds and check if running
	time.Sleep(3 * time.Second)
	out, err := exec.Command("nsenter", "-t", "1", "-m", "-u", "-i", "-n", "--", "systemctl", "is-active", "cloudflared").Output()
	running := err == nil && strings.TrimSpace(string(out)) == "active"
	log.Printf("TunnelHandler: cloudflared running after restart: %v", running)

	// Step 12: Save state
	newState := map[string]string{
		"tunnel_id":  tunnelID,
		"subdomain":  subdomain,
		"tunnel_url": tunnelURL,
		"token":      tunnelToken,
	}
	if err := saveTunnelState(newState); err != nil {
		log.Printf("TunnelHandler: Failed to save state: %v", err)
	}

	return c.JSON(fiber.Map{
		"success":   true,
		"url":       tunnelURL,
		"subdomain": subdomain,
		"message":   "Remote access enabled",
	})
}

// DisableTunnel stops and disables the Cloudflare tunnel service.
// It keeps the tunnel_id and subdomain in the state file but clears the active status.
func (h *TunnelHandler) DisableTunnel(c *fiber.Ctx) error {
	log.Println("TunnelHandler: Stopping cloudflared service on host")
	if out, err := exec.Command("nsenter", "-t", "1", "-m", "-u", "-i", "-n", "--", "systemctl", "stop", "cloudflared").CombinedOutput(); err != nil {
		log.Printf("TunnelHandler: stop cloudflared failed (may already be stopped): %v — %s", err, string(out))
	}

	log.Println("TunnelHandler: Disabling cloudflared service on host")
	if out, err := exec.Command("nsenter", "-t", "1", "-m", "-u", "-i", "-n", "--", "systemctl", "disable", "cloudflared").CombinedOutput(); err != nil {
		log.Printf("TunnelHandler: disable cloudflared failed: %v — %s", err, string(out))
	}

	// Update state file: keep tunnel_id/subdomain but mark disabled by clearing token
	state := loadTunnelState()
	state["token"] = ""
	if err := saveTunnelState(state); err != nil {
		log.Printf("TunnelHandler: Failed to save state after disable: %v", err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Remote access disabled",
	})
}
