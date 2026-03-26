package services

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"text/template"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
	"golang.org/x/crypto/curve25519"
)

// WireGuardManager manages WireGuard peers for SaaS tenants
type WireGuardManager struct {
	mu         sync.Mutex
	configPath string
	serverIP   string // Public IP of the SaaS server
	listenPort int
}

// wgExec runs a wg command, using nsenter to access host namespace if running in Docker
func wgExec(args ...string) *exec.Cmd {
	// Check if we're in a Docker container by looking for /.dockerenv
	if _, err := os.Stat("/.dockerenv"); err == nil {
		// Running inside Docker, use nsenter to access host's network namespace
		nsArgs := append([]string{"--target", "1", "--net", "--", "wg"}, args...)
		return exec.Command("nsenter", nsArgs...)
	}
	return exec.Command("wg", args...)
}

// NewWireGuardManager creates a new WireGuard manager
func NewWireGuardManager(serverIP string) *WireGuardManager {
	return &WireGuardManager{
		configPath: "/etc/wireguard/wg0.conf",
		serverIP:   serverIP,
		listenPort: 51820,
	}
}

// GenerateKeyPair generates a WireGuard key pair (private key, public key)
func GenerateKeyPair() (string, string, error) {
	// Generate private key (32 random bytes, clamped for Curve25519)
	var privateKey [32]byte
	if _, err := rand.Read(privateKey[:]); err != nil {
		return "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	// Clamp the private key for Curve25519
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	// Derive public key
	var publicKey [32]byte
	curve25519.ScalarBaseMult(&publicKey, &privateKey)

	privKeyB64 := base64.StdEncoding.EncodeToString(privateKey[:])
	pubKeyB64 := base64.StdEncoding.EncodeToString(publicKey[:])

	return privKeyB64, pubKeyB64, nil
}

// SetupTenantVPN generates WireGuard keys and assigns subnet for a tenant
func (wg *WireGuardManager) SetupTenantVPN(tenant *models.Tenant) error {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	// Use the ACTUAL server public key (from the running wg0 interface)
	_, serverPub, err := wg.getOrCreateServerKeys()
	if err != nil {
		return fmt.Errorf("failed to get server keys: %w", err)
	}

	// Generate client-side key pair (for the MikroTik)
	clientPriv, clientPub, err := GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate client keys: %w", err)
	}

	// Assign subnet based on tenant ID: 10.100.{tenant_id}.0/24
	tenantOctet := tenant.ID
	if tenantOctet > 254 {
		return fmt.Errorf("tenant ID %d exceeds maximum WireGuard subnet (254)", tenantOctet)
	}

	tenant.WGServerPrivateKey = "" // Server key is shared, not per-tenant
	tenant.WGServerPublicKey = serverPub
	tenant.WGClientPrivateKey = clientPriv
	tenant.WGClientPublicKey = clientPub
	tenant.WGSubnet = fmt.Sprintf("10.100.%d.0/24", tenantOctet)
	tenant.WGServerIP = fmt.Sprintf("10.100.%d.1", tenantOctet)
	tenant.WGClientIP = fmt.Sprintf("10.100.%d.2", tenantOctet)
	tenant.MikrotikAPIIP = tenant.WGClientIP // MikroTik reachable via VPN
	tenant.MikrotikAPIPort = 8728
	tenant.MikrotikAPIUser = "proxrad"

	// Generate MikroTik API password for remote management
	apiPassBytes := make([]byte, 12)
	rand.Read(apiPassBytes)
	tenant.MikrotikAPIPassword = base64.URLEncoding.EncodeToString(apiPassBytes)[:16]

	// Generate a random RADIUS secret
	secretBytes := make([]byte, 16)
	rand.Read(secretBytes)
	tenant.RadiusSecret = base64.URLEncoding.EncodeToString(secretBytes)[:20]

	return nil
}

// RegenerateConfig regenerates the wg0.conf with all active tenant peers
func (wg *WireGuardManager) RegenerateConfig() error {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	// We need a master server key - read from existing config or generate
	serverPrivKey, serverPubKey, err := wg.getOrCreateServerKeys()
	if err != nil {
		return err
	}

	// Load all active tenants with WireGuard config
	var tenants []models.Tenant
	if err := database.DB.Where("status = 'active' AND wg_client_public_key != ''").Find(&tenants).Error; err != nil {
		return fmt.Errorf("failed to load tenants: %w", err)
	}

	// Generate config
	confTemplate := `# WireGuard SaaS Hub - Auto-generated
# DO NOT EDIT MANUALLY

[Interface]
PrivateKey = {{.ServerPrivateKey}}
ListenPort = {{.ListenPort}}
Address = 10.100.0.1/16

# Post-up: enable IP forwarding
PostUp = sysctl -w net.ipv4.ip_forward=1
PostUp = iptables -A FORWARD -i wg0 -j ACCEPT
PostUp = iptables -A FORWARD -o wg0 -j ACCEPT
PostDown = iptables -D FORWARD -i wg0 -j ACCEPT
PostDown = iptables -D FORWARD -o wg0 -j ACCEPT
{{range .Peers}}
# Tenant: {{.Name}} (ID: {{.ID}})
[Peer]
PublicKey = {{.WGClientPublicKey}}
AllowedIPs = {{.WGClientIP}}/32
{{end}}`

	tmpl, err := template.New("wg").Parse(confTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	data := struct {
		ServerPrivateKey string
		ServerPublicKey  string
		ListenPort       int
		Peers            []models.Tenant
	}{
		ServerPrivateKey: serverPrivKey,
		ServerPublicKey:  serverPubKey,
		ListenPort:       wg.listenPort,
		Peers:            tenants,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Write config file
	if err := os.WriteFile(wg.configPath, []byte(buf.String()), 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	log.Printf("WireGuard: Generated config with %d peers", len(tenants))
	return nil
}

// SyncConfig hot-reloads the WireGuard config without disconnecting existing peers
func (wg *WireGuardManager) SyncConfig() error {
	if err := wg.RegenerateConfig(); err != nil {
		return err
	}

	// Use wg syncconf for hot reload (doesn't drop existing connections)
	cmd := wgExec("syncconf", "wg0", wg.configPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("wg syncconf failed: %s: %w", string(output), err)
	}

	log.Println("WireGuard: Config synced successfully")
	return nil
}

// getOrCreateServerKeys reads the server's WireGuard private key from existing config
// or generates a new one
func (wg *WireGuardManager) getOrCreateServerKeys() (string, string, error) {
	// Try to read existing key from wg interface
	cmd := wgExec("show", "wg0", "private-key")
	output, err := cmd.Output()
	if err == nil && len(strings.TrimSpace(string(output))) > 0 {
		privKey := strings.TrimSpace(string(output))
		// Derive public key
		cmd2 := wgExec("pubkey")
		cmd2.Stdin = strings.NewReader(privKey)
		pubOutput, err := cmd2.Output()
		if err == nil {
			return privKey, strings.TrimSpace(string(pubOutput)), nil
		}
	}

	// Generate new keys
	privKey, pubKey, err := GenerateKeyPair()
	if err != nil {
		return "", "", err
	}
	log.Println("WireGuard: Generated new server key pair")
	return privKey, pubKey, nil
}

// GetServerPublicKey returns the server's WireGuard public key
func (wg *WireGuardManager) GetServerPublicKey() (string, error) {
	_, pubKey, err := wg.getOrCreateServerKeys()
	return pubKey, err
}

// GenerateMikroTikScript generates a RouterOS script for the tenant to paste
// Single-line format with semicolons — works in any MikroTik terminal width
func (wg *WireGuardManager) GenerateMikroTikScript(tenant *models.Tenant) string {
	serverPubKey, _ := wg.GetServerPublicKey()

	// RouterOS v7 compatible script — everything on ONE line with semicolons
	// Key v7 fixes: separate /ppp aaa and /radius incoming commands, :if prefix for fasttrack
	script := fmt.Sprintf(
		`:{put "\n\n\n\n\r================================\r\n   ProxRad Connection Script\r\n================================\n"; `+
			`/interface wireguard peers remove [find where interface="proxrad-vpn"]; `+
			`/interface wireguard remove [find where name="proxrad-vpn"]; `+
			`/ip firewall filter remove [find where comment~"proxrad"]; `+
			`/radius remove [find where comment="proxrad"]; `+
			`/user remove [find where name="proxrad"]; `+
			`put "Cleaning old config... Done!"; `+
			`/interface wireguard add name=proxrad-vpn mtu=1420 listen-port=0 private-key="%s"; `+
			`put "Creating VPN interface... Done!"; `+
			`/interface/wireguard/peers/add interface=proxrad-vpn public-key="%s" endpoint-address=%s endpoint-port=%d allowed-address=10.100.0.0/16 persistent-keepalive=25; `+
			`put "Adding server peer... Done!"; `+
			`/ip address add address=%s/24 interface=proxrad-vpn; `+
			`put "Assigning VPN IP... Done!"; `+
			`/ip firewall filter add chain=input src-address=10.100.0.0/16 in-interface=proxrad-vpn action=accept place-before=*0 comment="#proxrad"; `+
			`put "Adding firewall rule... Done!"; `+
			`/ip service set api port=8728 disabled=no address=10.100.0.0/16; `+
			`/user add name=proxrad password="%s" group=full comment="ProxRad SaaS API"; `+
			`put "Creating API user... Done!"; `+
			`/radius add address=%s secret="%s" service=ppp timeout=3000ms comment="proxrad"; `+
			`/ppp aaa set use-radius=yes; `+
			`/radius incoming set accept=yes port=1700; `+
			`put "Configuring RADIUS... Done!"; `+
			`:if ([len [tostr [/ip firewall filter find action=fasttrack-connection disabled=no]]]>0) do={/ip firewall filter disable [/ip firewall filter find action=fasttrack-connection];}; `+
			`put "\r\nSUCCESS!! Your router is connected to ProxRad SaaS!\r\nPanel URL: https://%s.saas.proxrad.com\r\nVPN IP: %s\r\nVerify: /ping %s\r\n"; }`,
		tenant.WGClientPrivateKey,
		serverPubKey,
		wg.serverIP,
		wg.listenPort,
		tenant.WGClientIP,
		tenant.MikrotikAPIPassword,
		tenant.WGServerIP,
		tenant.RadiusSecret,
		tenant.Subdomain,
		tenant.WGClientIP,
		tenant.WGServerIP,
	)

	return script
}

// AddPeer adds a single peer to the running WireGuard interface
func (wg *WireGuardManager) AddPeer(tenant *models.Tenant) error {
	if tenant.WGClientPublicKey == "" {
		return fmt.Errorf("tenant has no WireGuard public key")
	}

	// Use wg set to add peer without disrupting others
	cmd := wgExec("set", "wg0",
		"peer", tenant.WGClientPublicKey,
		"allowed-ips", tenant.WGClientIP+"/32",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add WireGuard peer: %s: %w", string(output), err)
	}

	// Add tenant's server-side VPN IP to wg0 so MikroTik can ping/reach it
	if tenant.WGServerIP != "" {
		ipCmd := exec.Command("ip", "addr", "add", tenant.WGServerIP+"/32", "dev", "wg0")
		if _, err := os.Stat("/.dockerenv"); err == nil {
			ipCmd = exec.Command("nsenter", "--target", "1", "--net", "--", "ip", "addr", "add", tenant.WGServerIP+"/32", "dev", "wg0")
		}
		if output, err := ipCmd.CombinedOutput(); err != nil {
			log.Printf("WireGuard: Note: could not add server IP %s: %s", tenant.WGServerIP, string(output))
		} else {
			log.Printf("WireGuard: Added server IP %s to wg0", tenant.WGServerIP)
		}
	}

	// Add a specific route for the tenant's subnet with correct source IP
	// This ensures API connections to the MikroTik use the tenant's server IP as source,
	// which the MikroTik will route back through the WireGuard tunnel
	if tenant.WGServerIP != "" && tenant.WGSubnet != "" {
		// Extract /24 subnet from WGSubnet (e.g., "10.100.25.0/24")
		routeCmd := exec.Command("ip", "route", "replace", tenant.WGSubnet, "dev", "wg0", "src", tenant.WGServerIP)
		if _, err := os.Stat("/.dockerenv"); err == nil {
			routeCmd = exec.Command("nsenter", "--target", "1", "--net", "--", "ip", "route", "replace", tenant.WGSubnet, "dev", "wg0", "src", tenant.WGServerIP)
		}
		if output, err := routeCmd.CombinedOutput(); err != nil {
			log.Printf("WireGuard: Note: could not add route for %s: %s", tenant.WGSubnet, string(output))
		} else {
			log.Printf("WireGuard: Added route %s src %s", tenant.WGSubnet, tenant.WGServerIP)
		}
	}

	log.Printf("WireGuard: Added peer for tenant %s (%s)", tenant.Name, tenant.WGClientIP)
	return nil
}

// RemovePeer removes a peer from the running WireGuard interface
func (wg *WireGuardManager) RemovePeer(publicKey string) error {
	cmd := wgExec("set", "wg0", "peer", publicKey, "remove")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove WireGuard peer: %s: %w", string(output), err)
	}
	return nil
}

// IsPeerConnected checks if a tenant's MikroTik is connected via WireGuard
func (wg *WireGuardManager) IsPeerConnected(clientPublicKey string) bool {
	cmd := wgExec("show", "wg0", "latest-handshakes")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	for _, line := range strings.Split(string(output), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[0] == clientPublicKey {
			// Has a handshake timestamp = peer has connected at some point
			return parts[1] != "0"
		}
	}
	return false
}
