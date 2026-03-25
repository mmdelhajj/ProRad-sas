package security

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
)

// HardwareFingerprint generates a unique hardware ID for this server
// Uses SERVER_IP, SERVER_MAC, HOST_HOSTNAME environment variables if set (for Docker containers)
// Otherwise falls back to system detection
func HardwareFingerprint() string {
	// Check for environment variables first (Docker container support)
	serverIP := os.Getenv("SERVER_IP")
	serverMAC := os.Getenv("SERVER_MAC")
	// Use HOST_HOSTNAME first (explicitly set), fall back to HOSTNAME
	hostname := os.Getenv("HOST_HOSTNAME")
	if hostname == "" {
		hostname = os.Getenv("HOSTNAME")
	}

	// If all env vars are set, use simple hash matching license server format
	if serverIP != "" && serverMAC != "" && hostname != "" {
		data := serverIP + "|" + serverMAC + "|" + hostname
		hash := sha256.Sum256([]byte(data))
		return hex.EncodeToString(hash[:])
	}

	// Fallback to system detection
	var components []string

	// Get MAC addresses
	macs := getMACAddresses()
	components = append(components, macs...)

	// Get CPU info
	cpuID := getCPUID()
	if cpuID != "" {
		components = append(components, cpuID)
	}

	// Get machine ID (Linux)
	machineID := getMachineID()
	if machineID != "" {
		components = append(components, machineID)
	}

	// Get disk serial
	diskSerial := getDiskSerial()
	if diskSerial != "" {
		components = append(components, diskSerial)
	}

	// Sort for consistency
	sort.Strings(components)

	// Hash all components
	combined := strings.Join(components, "|")
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:16]) // Return first 32 chars
}

func getMACAddresses() []string {
	var macs []string
	interfaces, err := net.Interfaces()
	if err != nil {
		return macs
	}

	for _, iface := range interfaces {
		// Skip loopback and virtual interfaces
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if strings.HasPrefix(iface.Name, "docker") ||
			strings.HasPrefix(iface.Name, "veth") ||
			strings.HasPrefix(iface.Name, "br-") {
			continue
		}

		mac := iface.HardwareAddr.String()
		if mac != "" {
			macs = append(macs, mac)
		}
	}
	return macs
}

func getCPUID() string {
	if runtime.GOOS != "linux" {
		return ""
	}

	// Try to get CPU info from /proc/cpuinfo
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Serial") || strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func getMachineID() string {
	// Linux machine-id
	paths := []string{
		"/etc/machine-id",
		"/var/lib/dbus/machine-id",
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

func getDiskSerial() string {
	if runtime.GOOS != "linux" {
		return ""
	}

	// Method 1: Read from /dev/disk/by-id/ (most reliable, hard to spoof)
	diskByID := getDiskByID()
	if diskByID != "" {
		return diskByID
	}

	// Method 2: Try lsblk to get disk serial (fallback)
	cmd := exec.Command("lsblk", "-o", "SERIAL", "-n", "-d")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		serial := strings.TrimSpace(line)
		if serial != "" {
			return serial
		}
	}
	return ""
}

// getDiskByID reads disk identifier from /dev/disk/by-id/ - very hard to spoof
func getDiskByID() string {
	entries, err := os.ReadDir("/dev/disk/by-id")
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		name := entry.Name()
		// Skip partition entries, get main disk only
		if strings.Contains(name, "-part") {
			continue
		}
		// Prefer SCSI/SATA/NVMe identifiers
		if strings.HasPrefix(name, "scsi-") ||
			strings.HasPrefix(name, "ata-") ||
			strings.HasPrefix(name, "nvme-") ||
			strings.HasPrefix(name, "wwn-") {
			return name
		}
	}

	// Fallback: return first non-partition entry
	for _, entry := range entries {
		if !strings.Contains(entry.Name(), "-part") {
			return entry.Name()
		}
	}
	return ""
}

// ValidateHardwareBinding checks if current hardware matches registered hardware
func ValidateHardwareBinding(registeredFingerprint string) bool {
	if registeredFingerprint == "" {
		return true // No binding set
	}

	currentFingerprint := HardwareFingerprint()
	return currentFingerprint == registeredFingerprint
}
