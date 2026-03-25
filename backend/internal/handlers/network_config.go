package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// NetworkConfigHandler handles network configuration
type NetworkConfigHandler struct{}

// NewNetworkConfigHandler creates a new network config handler
func NewNetworkConfigHandler() *NetworkConfigHandler {
	return &NetworkConfigHandler{}
}

// NetworkConfigRequest represents network configuration request
type NetworkConfigRequest struct {
	Interface string `json:"interface"`  // eth0, ens3, etc.
	IPAddress string `json:"ip_address"` // 192.168.1.100/24
	Gateway   string `json:"gateway"`    // 192.168.1.1
	DNS1      string `json:"dns1"`       // 8.8.8.8
	DNS2      string `json:"dns2"`       // 8.8.4.4
	DNSMethod string `json:"dns_method"` // "netplan" or "resolv"
}

// TestNetworkConfig applies network changes for 60 seconds (TrueNAS-style)
func (h *NetworkConfigHandler) TestNetworkConfig(c *fiber.Ctx) error {
	var req NetworkConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request",
		})
	}

	// Validate required fields
	if req.Interface == "" || req.IPAddress == "" || req.Gateway == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Interface, IP address, and gateway are required",
		})
	}

	// Default DNS if not provided
	if req.DNS1 == "" {
		req.DNS1 = "8.8.8.8"
	}
	if req.DNS2 == "" {
		req.DNS2 = "8.8.4.4"
	}

	// Build and execute test script
	script := buildNetworkTestScript(req)
	cmd := exec.Command("bash", "-c", script)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to apply network configuration",
			"error":   string(output),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "TEST MODE: Changes will revert in 60 seconds",
		"test_mode": true,
		"new_ip": req.IPAddress,
		"revert_seconds": 60,
	})
}

// ApplyNetworkConfig makes network settings permanent
func (h *NetworkConfigHandler) ApplyNetworkConfig(c *fiber.Ctx) error {
	var req NetworkConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request",
		})
	}

	// Kill auto-revert process by PID
	pidBytes, _ := os.ReadFile("/opt/proxpanel/network-revert.pid")
	if len(pidBytes) > 0 {
		pid := strings.TrimSpace(string(pidBytes))
		exec.Command("kill", pid).Run()
	}

	// Remove backup file so revert doesn't run
	os.Remove("/opt/proxpanel/network-backup.conf")
	os.Remove("/opt/proxpanel/network-revert.pid")

	// Write netplan config for persistence across reboots
	netplanConfig := fmt.Sprintf(`network:
  version: 2
  ethernets:
    %s:
      addresses:
        - %s
      routes:
        - to: default
          via: %s
      nameservers:
        addresses:
          - %s
          - %s`,
		req.Interface,
		req.IPAddress,
		req.Gateway,
		req.DNS1,
		req.DNS2,
	)

	if err := os.WriteFile("/etc/netplan/50-cloud-init.yaml", []byte(netplanConfig), 0600); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to write netplan config",
			"error":   err.Error(),
		})
	}

	// ALSO update the cached file so GetCurrentNetworkConfig reads correct DNS
	os.MkdirAll("/opt/proxpanel/host-network", 0755)
	os.WriteFile("/opt/proxpanel/host-network/netplan.yaml", []byte(netplanConfig), 0600)

	// Apply DNS based on selected method
	if req.DNSMethod == "resolv" {
		// Method 1: Direct /etc/resolv.conf (Traditional)
		resolvConf := fmt.Sprintf("nameserver %s\nnameserver %s\n", req.DNS1, req.DNS2)
		writeResolv := fmt.Sprintf(`nsenter -t 1 -m -u -i -n -p -- bash -c "echo '%s' > /etc/resolv.conf"`, resolvConf)
		exec.Command("bash", "-c", writeResolv).Run()
	} else {
		// Method 2: Netplan + systemd-resolved (Ubuntu/Debian - default)
		setDNS := fmt.Sprintf(`nsenter -t 1 -m -u -i -n -p -- bash -c "resolvectl dns %s %s %s && resolvectl domain %s '~.'"`,
			req.Interface, req.DNS1, req.DNS2, req.Interface)
		exec.Command("bash", "-c", setDNS).Run()

		// Also restart systemd-resolved to ensure changes take effect
		restartDNS := `nsenter -t 1 -m -u -i -n -p -- systemctl restart systemd-resolved 2>/dev/null || true`
		exec.Command("bash", "-c", restartDNS).Run()
	}

	// Remove backup files
	os.Remove("/opt/proxpanel/network-backup.conf")
	os.Remove("/opt/proxpanel/network-revert.pid")

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Network settings applied permanently",
	})
}

// buildNetworkTestScript creates a bash script for network testing (TrueNAS-style)
func buildNetworkTestScript(config NetworkConfigRequest) string {
	// With privileged + pid:host, we can directly access host network
	// IMPORTANT: Script must return immediately, not wait for revert
	return fmt.Sprintf(`#!/bin/bash
set -e

# Get current IP and gateway from host
CURRENT_IP=$(nsenter -t 1 -n ip -4 addr show %s | grep -oP 'inet \K[\d.]+/\d+' | head -1)
CURRENT_GW=$(nsenter -t 1 -n ip route | grep '^default' | awk '{print $3}' | head -1)

# Save for revert
echo "$CURRENT_IP|$CURRENT_GW" > /opt/proxpanel/network-backup.conf

# Apply new configuration on host
nsenter -t 1 -n ip addr del "$CURRENT_IP" dev %s 2>/dev/null || true
nsenter -t 1 -n ip addr add %s dev %s
nsenter -t 1 -n ip route del default 2>/dev/null || true
nsenter -t 1 -n ip route add default via %s

# Start background revert process (detached, doesn't block script return)
nohup bash -c '
  sleep 60
  if [ -f /opt/proxpanel/network-backup.conf ]; then
    BACKUP=$(cat /opt/proxpanel/network-backup.conf)
    OLD_IP=$(echo "$BACKUP" | cut -d"|" -f1)
    OLD_GW=$(echo "$BACKUP" | cut -d"|" -f2)

    nsenter -t 1 -n ip addr del %s dev %s 2>/dev/null || true
    nsenter -t 1 -n ip addr add "$OLD_IP" dev %s
    nsenter -t 1 -n ip route del default 2>/dev/null || true
    nsenter -t 1 -n ip route add default via "$OLD_GW"

    rm -f /opt/proxpanel/network-backup.conf /opt/proxpanel/network-revert.pid
  fi
' > /opt/proxpanel/network-revert.log 2>&1 &

echo $! > /opt/proxpanel/network-revert.pid

# Return immediately (don't wait for sleep)
echo "TEST MODE ACTIVE"
`,
		config.Interface,   // Get current IP
		config.Interface,   // Delete old IP
		config.IPAddress,   // Add new IP
		config.Interface,   // Dev
		config.Gateway,     // New gateway
		config.IPAddress,   // Revert: delete new IP
		config.Interface,   // Revert: dev
		config.Interface,   // Revert: add old IP dev
	)
}

// DetectDNSMethod detects which DNS configuration method the system uses
func (h *NetworkConfigHandler) DetectDNSMethod(c *fiber.Ctx) error {
	// Check if system uses systemd-resolved (netplan method)
	resolvConfOutput, _ := exec.Command("bash", "-c", "nsenter -t 1 -m -u -i -n -p -- cat /etc/resolv.conf 2>/dev/null").Output()
	usesSystemdResolved := strings.Contains(string(resolvConfOutput), "127.0.0.53") || strings.Contains(string(resolvConfOutput), "systemd-resolved")

	// Get DNS from netplan
	var netplanDNS []string
	netplanOutput, _ := exec.Command("bash", "-c", "cat /etc/netplan/50-cloud-init.yaml 2>/dev/null || echo ''").Output()
	if len(netplanOutput) > 0 {
		lines := strings.Split(string(netplanOutput), "\n")
		inNameservers := false
		for _, line := range lines {
			if strings.Contains(line, "nameservers:") {
				inNameservers = true
				continue
			}
			if inNameservers && strings.Contains(line, "addresses:") {
				continue
			}
			if inNameservers && strings.TrimSpace(line) != "" && strings.HasPrefix(strings.TrimSpace(line), "- ") {
				dns := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
				netplanDNS = append(netplanDNS, dns)
			}
			if inNameservers && !strings.HasPrefix(strings.TrimSpace(line), "-") && strings.TrimSpace(line) != "" && !strings.Contains(line, "addresses:") {
				inNameservers = false
			}
		}
	}

	// Get DNS from resolv.conf (excluding 127.0.0.53)
	var resolvDNS []string
	for _, line := range strings.Split(string(resolvConfOutput), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "nameserver") {
			parts := strings.Fields(line)
			if len(parts) >= 2 && parts[1] != "127.0.0.53" && parts[1] != "127.0.0.11" {
				resolvDNS = append(resolvDNS, parts[1])
			}
		}
	}

	// If resolv.conf has 127.0.0.53, get actual DNS from systemd-resolved
	if usesSystemdResolved {
		resolvectlOutput, _ := exec.Command("bash", "-c", "nsenter -t 1 -m -u -i -n -p -- resolvectl status 2>/dev/null | grep 'DNS Servers' | head -1").Output()
		if len(resolvectlOutput) > 0 {
			outputStr := strings.TrimSpace(string(resolvectlOutput))
			// Remove "DNS Servers:" prefix (with possible leading whitespace)
			if idx := strings.Index(outputStr, "DNS Servers:"); idx >= 0 {
				dnsStr := strings.TrimSpace(outputStr[idx+len("DNS Servers:"):])
				if dnsStr != "" {
					resolvDNS = strings.Fields(dnsStr)
				}
			}
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"detected_method":      map[bool]string{true: "netplan", false: "resolv"}[usesSystemdResolved],
			"uses_systemd_resolved": usesSystemdResolved,
			"netplan_dns":          netplanDNS,
			"resolv_dns":           resolvDNS,
		},
	})
}

// GetCurrentNetworkConfig returns current network configuration
func (h *NetworkConfigHandler) GetCurrentNetworkConfig(c *fiber.Ctx) error {
	// Read from HOST network namespace (not container), use nsenter to access host
	ipOutput, err := exec.Command("bash", "-c", "nsenter -t 1 -n ip addr show 2>/dev/null || cat /opt/proxpanel/host-network/ip-addr.txt 2>/dev/null || ip addr show").Output()
	if err != nil {
		ipOutput = []byte("")
	}

	routeOutput, err := exec.Command("bash", "-c", "nsenter -t 1 -n ip route show 2>/dev/null || cat /opt/proxpanel/host-network/ip-route.txt 2>/dev/null || ip route show").Output()
	if err != nil {
		routeOutput = []byte("")
	}

	// Read from REAL netplan file first (not cache), so DNS changes show immediately
	netplanOutput, err := exec.Command("bash", "-c", "cat /etc/netplan/50-cloud-init.yaml 2>/dev/null || cat /opt/proxpanel/host-network/netplan.yaml 2>/dev/null || echo ''").Output()
	if err != nil {
		netplanOutput = []byte("")
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"netplan_config":  string(netplanOutput),
			"current_ip_info": string(ipOutput),
			"current_routes":  string(routeOutput),
		},
	})
}
