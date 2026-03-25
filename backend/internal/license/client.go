package license

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/proisp/backend/internal/security"
)

// ValidationState holds the encrypted validation state for offline grace period
type ValidationState struct {
	LastValid   int64  `json:"last_valid"`   // Unix timestamp of last successful validation
	HardwareID  string `json:"hardware_id"`  // Hardware ID to ensure state is from this machine
	LicenseKey  string `json:"license_key"`  // License key hash (not full key)
	Signature   string `json:"signature"`    // HMAC signature for tamper detection
}

const (
	validationStateFile = "/var/lib/proxpanel/.license_state"
	validationGracePeriod = 4 * time.Hour
)

// saveValidationState saves encrypted validation state after successful validation
func saveValidationState(licenseKey string) error {
	hardwareID := getStableHardwareID()

	// Create state without signature first
	state := ValidationState{
		LastValid:  time.Now().Unix(),
		HardwareID: hardwareID,
		LicenseKey: hashString(licenseKey)[:16], // Only store partial hash
	}

	// Create signature
	stateData := fmt.Sprintf("%d|%s|%s", state.LastValid, state.HardwareID, state.LicenseKey)
	state.Signature = createHMAC(stateData, hardwareID+licenseKey)

	// Marshal to JSON
	jsonData, err := json.Marshal(state)
	if err != nil {
		return err
	}

	// Encrypt the state
	encryptedData, err := encryptState(jsonData, hardwareID+licenseKey)
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(validationStateFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(validationStateFile, encryptedData, 0600)
}

// loadValidationState loads and verifies the encrypted validation state
// Returns the state and whether it's still within the grace period
func loadValidationState(licenseKey string) (*ValidationState, bool) {
	hardwareID := getStableHardwareID()

	// Read encrypted file
	encryptedData, err := os.ReadFile(validationStateFile)
	if err != nil {
		return nil, false
	}

	// Decrypt
	jsonData, err := decryptState(encryptedData, hardwareID+licenseKey)
	if err != nil {
		log.Printf("Failed to decrypt validation state: %v", err)
		return nil, false
	}

	// Parse JSON
	var state ValidationState
	if err := json.Unmarshal(jsonData, &state); err != nil {
		log.Printf("Failed to parse validation state: %v", err)
		return nil, false
	}

	// Verify hardware ID matches
	if state.HardwareID != hardwareID {
		log.Printf("Validation state hardware ID mismatch")
		return nil, false
	}

	// Verify license key hash matches
	if state.LicenseKey != hashString(licenseKey)[:16] {
		log.Printf("Validation state license key mismatch")
		return nil, false
	}

	// Verify signature
	stateData := fmt.Sprintf("%d|%s|%s", state.LastValid, state.HardwareID, state.LicenseKey)
	expectedSig := createHMAC(stateData, hardwareID+licenseKey)
	if state.Signature != expectedSig {
		log.Printf("Validation state signature mismatch (tampering detected)")
		return nil, false
	}

	// Check if within grace period
	lastValidTime := time.Unix(state.LastValid, 0)
	withinGrace := time.Since(lastValidTime) <= validationGracePeriod

	return &state, withinGrace
}

// hashString creates SHA256 hash of a string
func hashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

// createHMAC creates HMAC-SHA256 signature
func createHMAC(data, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// encryptState encrypts data using AES-256-GCM
func encryptState(data []byte, key string) ([]byte, error) {
	// Derive 32-byte key from password
	keyHash := sha256.Sum256([]byte(key))

	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, data, nil), nil
}

// decryptState decrypts AES-256-GCM encrypted data
func decryptState(data []byte, key string) ([]byte, error) {
	// Derive 32-byte key from password
	keyHash := sha256.Sum256([]byte(key))

	block, err := aes.NewCipher(keyHash[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(data) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// getStableHardwareID generates hardware ID in the same format as license server expects
// Format: stable_<sha256(stable|MAC|product_uuid|machine_id)>
// Note: Hostname and IP are NOT included - customers can change them freely
// This must match the format used in config.go for secrets fetching
func getStableHardwareID() string {
	serverMAC := os.Getenv("SERVER_MAC")

	// Read product_uuid (motherboard BIOS UUID - very stable, hard to fake)
	productUUID := readFileContent("/sys/class/dmi/id/product_uuid")
	if productUUID == "" {
		// Try mounted host path (for Docker containers)
		productUUID = readFileContent("/host/sys/class/dmi/id/product_uuid")
	}

	// Read machine_id (Linux system ID - generated at OS install)
	machineID := readFileContent("/etc/machine-id")
	if machineID == "" {
		// Try mounted host path (for Docker containers)
		machineID = readFileContent("/host/etc/machine-id")
	}

	if serverMAC == "" {
		// Fallback to security package hardware ID
		return security.GetHardwareID()
	}

	// Generate stable hardware ID: stable_<sha256(stable|MAC|product_uuid|machine_id)>
	// This is much harder to spoof than MAC + hostname
	data := fmt.Sprintf("stable|%s|%s|%s", serverMAC, productUUID, machineID)
	hash := sha256.Sum256([]byte(data))
	return "stable_" + hex.EncodeToString(hash[:])
}

// readFileContent reads a file and returns its trimmed content
func readFileContent(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if len(content) > 0 && content[len(content)-1] == '\n' {
		return string(content[:len(content)-1])
	}
	return string(content)
}

// Config holds license client configuration
type Config struct {
	ServerURL     string
	LicenseKey    string
	CheckInterval time.Duration
}

// Client handles license validation and communication
type Client struct {
	config        Config
	httpClient    *http.Client
	licenseInfo   *LicenseInfo
	encryptionKey string
	mutex         sync.RWMutex
	stopChan      chan struct{}
	isValid       bool
	lastCheck     time.Time
	gracePeriod   bool
}

// LicenseInfo contains license details from server
type LicenseInfo struct {
	Valid           bool       `json:"valid"`
	Message         string     `json:"message"`
	LicenseID       uint       `json:"license_id,omitempty"`
	CustomerName    string     `json:"customer_name,omitempty"`
	Tier            string     `json:"tier,omitempty"`
	MaxSubscribers  int        `json:"max_subscribers,omitempty"`
	Features        string     `json:"features,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	IsLifetime      bool       `json:"is_lifetime,omitempty"`
	EncryptionKey   string     `json:"encryption_key,omitempty"`
	GracePeriod     bool       `json:"grace_period,omitempty"`
	DaysRemaining   int        `json:"days_remaining,omitempty"`
	// WHMCS-style license status
	LicenseStatus   string `json:"license_status"`    // active, warning, grace, readonly, blocked
	ReadOnly        bool   `json:"read_only"`         // true if system should be read-only
	DaysUntilExpiry int    `json:"days_until_expiry"` // negative if expired
	WarningMessage  string `json:"warning_message,omitempty"`
}

// HeartbeatRequest contains usage data for heartbeat
type HeartbeatRequest struct {
	LicenseKey      string  `json:"license_key"`
	ServerIP        string  `json:"server_ip"`
	SubscriberCount int     `json:"subscriber_count"`
	OnlineCount     int     `json:"online_count"`
	CPUUsage        float64 `json:"cpu_usage"`
	MemoryUsage     float64 `json:"memory_usage"`
	DiskUsage       float64 `json:"disk_usage"`
	Version         string  `json:"version"`
}

// Global client instance
var defaultClient *Client
var devMode bool

// buildMode is set at compile time: -ldflags "-X github.com/proisp/backend/internal/license.buildMode=dev"
// For production builds, this remains empty and license validation is enforced
var buildMode string

// Initialize creates and starts the license client
func Initialize(serverURL, licenseKey string) error {
	// Internal development check - uses build-time variable, not environment
	// This is set only when building for development: go build -ldflags "-X github.com/proisp/backend/internal/license.buildMode=dev"
	if buildMode == "dev" {
		devMode = true
		defaultClient = &Client{
			isValid: true,
			licenseInfo: &LicenseInfo{
				Valid:          true,
				LicenseStatus:  "active",
				Message:        "Development build",
				CustomerName:   "Development",
				Tier:           "unlimited",
				MaxSubscribers: 999999,
				IsLifetime:     true,
			},
			stopChan: make(chan struct{}),
		}
		return nil
	}

	config := Config{
		ServerURL:     serverURL,
		LicenseKey:    licenseKey,
		CheckInterval: 30 * time.Second, // Check every 30 seconds (pro security)
	}

	client := &Client{
		config: config,
		httpClient: security.CreatePinnedHTTPClient(),
		stopChan: make(chan struct{}),
	}

	// Initialize enterprise security features
	security.InitEnterpriseSecurity(licenseKey)

	// Register stealth check function for random delayed checks
	security.RegisterStealthCheck(func() bool {
		if client == nil {
			return false
		}
		err := client.validate()
		return err == nil
	})

	// Initial validation - always set defaultClient so status can be queried
	validationErr := client.validate()

	// Set the client even if validation failed, so status can still be reported
	defaultClient = client

	// If online validation failed, check saved state for grace period
	if validationErr != nil {
		log.Printf("Online validation failed: %v. Checking saved validation state...", validationErr)

		state, withinGrace := loadValidationState(licenseKey)
		if state != nil && withinGrace {
			log.Printf("Valid saved state found (last validation: %v ago). Allowing startup in grace period.",
				time.Since(time.Unix(state.LastValid, 0)).Round(time.Second))

			// Set client as valid with grace period flag
			client.mutex.Lock()
			client.isValid = true
			client.gracePeriod = true
			client.lastCheck = time.Unix(state.LastValid, 0)
			client.licenseInfo = &LicenseInfo{
				Valid:         true,
				GracePeriod:   true,
				LicenseStatus: "grace",
				Message:       "Operating in grace period (offline)",
			}
			client.mutex.Unlock()

			validationErr = nil // Clear error - we're allowing startup
		} else if state != nil {
			log.Printf("Saved state found but expired (last validation: %v ago). Grace period exceeded.",
				time.Since(time.Unix(state.LastValid, 0)).Round(time.Second))
		} else {
			log.Printf("No valid saved state found.")
		}
	}

	// Set validation point for initialization
	security.SetValidationPoint("init", validationErr == nil)

	// Start background validation
	go client.backgroundCheck()

	if validationErr != nil {
		// Log the license status for debugging
		if client.licenseInfo != nil {
			log.Printf("License validation failed. Status: %s, Message: %s",
				client.licenseInfo.LicenseStatus, client.licenseInfo.Message)
		}
		return fmt.Errorf("initial license validation failed: %v", validationErr)
	}

	log.Printf("License client initialized. Customer: %s, Tier: %s, Max Subscribers: %d",
		client.licenseInfo.CustomerName, client.licenseInfo.Tier, client.licenseInfo.MaxSubscribers)

	return nil
}

// IsValid returns whether the license is currently valid
func IsValid() bool {
	if defaultClient == nil {
		return false
	}
	defaultClient.mutex.RLock()
	defer defaultClient.mutex.RUnlock()

	// Also check all validation points (multi-point validation)
	if !security.CheckAllValidationPoints() {
		// Set validation point to track this check
		security.SetValidationPoint("isvalid_check", false)
		return false
	}

	security.SetValidationPoint("isvalid_check", true)
	return defaultClient.isValid
}

// GetEncryptionKey returns the license encryption key
func GetEncryptionKey() string {
	if defaultClient == nil {
		return ""
	}
	defaultClient.mutex.RLock()
	defer defaultClient.mutex.RUnlock()
	return defaultClient.encryptionKey
}

// GetLicenseInfo returns current license information
func GetLicenseInfo() *LicenseInfo {
	if defaultClient == nil {
		return nil
	}
	defaultClient.mutex.RLock()
	defer defaultClient.mutex.RUnlock()
	return defaultClient.licenseInfo
}

// GetMaxSubscribers returns the maximum allowed subscribers
func GetMaxSubscribers() int {
	if defaultClient == nil {
		return 0
	}
	defaultClient.mutex.RLock()
	defer defaultClient.mutex.RUnlock()
	if defaultClient.licenseInfo != nil {
		return defaultClient.licenseInfo.MaxSubscribers
	}
	return 0
}

// Secrets contains database/service secrets fetched from license server
type Secrets struct {
	DBPassword    string `json:"db_password"`
	RedisPassword string `json:"redis_password"`
	JWTSecret     string `json:"jwt_secret"`
	EncryptionKey string `json:"encryption_key"`
	SSHPassword   string `json:"ssh_password"` // SSH password for Remote Support
}

// FetchSecrets retrieves database/service secrets from the license server
// This is used for Option 2 security - secrets are not stored on customer disk
func FetchSecrets(serverURL, licenseKey string) (*Secrets, error) {
	if serverURL == "" || licenseKey == "" {
		return nil, fmt.Errorf("server URL and license key are required")
	}

	// Build hardware ID for verification (must match stable format)
	hardwareID := getStableHardwareID()

	url := fmt.Sprintf("%s/api/v1/license/secrets", serverURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("X-License-Key", licenseKey)
	req.Header.Set("X-Hardware-ID", hardwareID)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch secrets: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != 200 {
		var errResp struct {
			Message string `json:"message"`
		}
		json.Unmarshal(body, &errResp)
		return nil, fmt.Errorf("license server error: %s", errResp.Message)
	}

	var response struct {
		Success bool    `json:"success"`
		Data    Secrets `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("failed to fetch secrets")
	}

	log.Printf("Secrets fetched from license server successfully")
	return &response.Data, nil
}

// CanAddSubscriber checks if adding a new subscriber is allowed
// Returns: allowed, currentCount, maxAllowed, error
func CanAddSubscriber(currentCount int) (bool, int, int, error) {
	if defaultClient == nil {
		return false, 0, 0, fmt.Errorf("license client not initialized")
	}

	// In dev mode, always allow
	if devMode {
		return true, currentCount, 999999, nil
	}

	maxSubscribers := GetMaxSubscribers()
	if maxSubscribers == 0 {
		// No limit set - allow
		return true, currentCount, 0, nil
	}

	if currentCount >= maxSubscribers {
		return false, currentCount, maxSubscribers, nil
	}

	return true, currentCount, maxSubscribers, nil
}

// VerifySubscriberCount verifies subscriber count with license server (remote check)
func VerifySubscriberCount(currentCount int) (bool, string, error) {
	if defaultClient == nil {
		return false, "", fmt.Errorf("license client not initialized")
	}

	// In dev mode, always allow
	if devMode {
		return true, "", nil
	}

	serverIP, _ := getOutboundIP()

	req := map[string]interface{}{
		"license_key":       defaultClient.config.LicenseKey,
		"server_ip":         serverIP,
		"subscriber_count":  currentCount,
		"action":            "add_subscriber",
	}

	body, err := json.Marshal(req)
	if err != nil {
		return false, "", err
	}

	resp, err := defaultClient.httpClient.Post(
		defaultClient.config.ServerURL+"/api/v1/license/verify-subscriber",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		// On network error, fall back to local check
		log.Printf("Warning: Could not verify with license server: %v, using local check", err)
		allowed, _, _, _ := CanAddSubscriber(currentCount)
		return allowed, "", nil
	}
	defer resp.Body.Close()

	var result struct {
		Success bool   `json:"success"`
		Allowed bool   `json:"allowed"`
		Message string `json:"message"`
		Max     int    `json:"max_subscribers"`
		Current int    `json:"current_count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, "", err
	}

	if !result.Allowed {
		return false, result.Message, nil
	}

	return true, "", nil
}

// InGracePeriod returns whether we're in the grace period
func InGracePeriod() bool {
	if defaultClient == nil {
		return false
	}
	defaultClient.mutex.RLock()
	defer defaultClient.mutex.RUnlock()
	return defaultClient.gracePeriod
}

// IsReadOnly returns whether the system should be in read-only mode
func IsReadOnly() bool {
	if defaultClient == nil {
		return false
	}
	defaultClient.mutex.RLock()
	defer defaultClient.mutex.RUnlock()
	if defaultClient.licenseInfo != nil {
		return defaultClient.licenseInfo.ReadOnly
	}
	return false
}

// GetLicenseStatus returns the current license status (active, warning, grace, readonly, blocked)
func GetLicenseStatus() string {
	if defaultClient == nil {
		return "unknown"
	}
	defaultClient.mutex.RLock()
	defer defaultClient.mutex.RUnlock()
	if defaultClient.licenseInfo != nil && defaultClient.licenseInfo.LicenseStatus != "" {
		return defaultClient.licenseInfo.LicenseStatus
	}
	if defaultClient.isValid {
		return "active"
	}
	return "blocked"
}

// IsDevMode returns true if running in development mode (no license checks)
func IsDevMode() bool {
	return devMode
}

// GetWarningMessage returns any license warning message
func GetWarningMessage() string {
	if defaultClient == nil {
		return ""
	}
	defaultClient.mutex.RLock()
	defer defaultClient.mutex.RUnlock()
	if defaultClient.licenseInfo != nil {
		return defaultClient.licenseInfo.WarningMessage
	}
	return ""
}

// GetDaysUntilExpiry returns days until license expires (negative if expired)
func GetDaysUntilExpiry() int {
	if defaultClient == nil {
		return 0
	}
	defaultClient.mutex.RLock()
	defer defaultClient.mutex.RUnlock()
	if defaultClient.licenseInfo != nil {
		return defaultClient.licenseInfo.DaysUntilExpiry
	}
	return 0
}

// SendHeartbeat sends usage statistics to license server
func SendHeartbeat(subscriberCount, onlineCount int, cpuUsage, memUsage, diskUsage float64) error {
	if defaultClient == nil {
		return fmt.Errorf("license client not initialized")
	}
	return defaultClient.sendHeartbeat(subscriberCount, onlineCount, cpuUsage, memUsage, diskUsage)
}

// Stop gracefully stops the license client
func Stop() {
	if defaultClient != nil && defaultClient.stopChan != nil {
		close(defaultClient.stopChan)
	}
}

// Revalidate forces a re-validation with the license server
func Revalidate() error {
	if defaultClient == nil {
		return fmt.Errorf("license client not initialized")
	}
	// Skip validation in dev mode - always valid
	if devMode {
		return nil
	}
	return defaultClient.validate()
}

// validate performs license validation with the server
func (c *Client) validate() error {
	serverIP, _ := getOutboundIP()
	serverMAC, _ := getMACAddress()
	// Use HOST_HOSTNAME env var first (explicitly set for Docker), then HOSTNAME
	hostname := os.Getenv("HOST_HOSTNAME")
	if hostname == "" {
		hostname = os.Getenv("HOSTNAME")
	}
	if hostname == "" {
		hostname, _ = os.Hostname()
	}
	hardwareID := getStableHardwareID()

	version := getVersion()
	log.Printf("License validation: version='%s', server_ip='%s'", version, serverIP)

	req := map[string]string{
		"license_key": c.config.LicenseKey,
		"server_ip":   serverIP,
		"server_mac":  serverMAC,
		"hostname":    hostname,
		"version":     version,
		"hardware_id": hardwareID,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	log.Printf("License validation request JSON: %s", string(body))

	resp, err := c.httpClient.Post(
		c.config.ServerURL+"/api/v1/license/validate",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return fmt.Errorf("failed to contact license server: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var info LicenseInfo
	if err := json.Unmarshal(respBody, &info); err != nil {
		return fmt.Errorf("invalid response from license server: %v", err)
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.licenseInfo = &info
	c.isValid = info.Valid
	c.encryptionKey = info.EncryptionKey
	c.gracePeriod = info.GracePeriod
	c.lastCheck = time.Now()

	// Record check time for timing anomaly detection
	security.RecordCheckTime()

	// Set validation point
	security.SetValidationPoint("validate", info.Valid)

	// Kill switch - immediate termination if license is killed by server
	if info.LicenseStatus == "killed" || info.LicenseStatus == "terminated" {
		log.Println("FATAL: License has been terminated by server. Shutting down.")
		os.Exit(1)
	}

	if !info.Valid {
		return fmt.Errorf("license validation failed: %s", info.Message)
	}

	// Save validation state for offline grace period support
	if err := saveValidationState(c.config.LicenseKey); err != nil {
		log.Printf("Warning: Failed to save validation state: %v", err)
	}

	return nil
}

// backgroundCheck runs periodic license checks
func (c *Client) backgroundCheck() {
	ticker := time.NewTicker(c.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			if err := c.validate(); err != nil {
				log.Printf("License revalidation failed: %v", err)
				// Don't immediately invalidate - use grace period
				c.mutex.Lock()
				if time.Since(c.lastCheck) > 5*time.Minute {
					c.isValid = false
					log.Println("License marked as invalid after 5 minutes without successful validation")
				}
				c.mutex.Unlock()
			}
		}
	}
}

// sendHeartbeat sends usage data to license server
func (c *Client) sendHeartbeat(subscriberCount, onlineCount int, cpuUsage, memUsage, diskUsage float64) error {
	serverIP, _ := getOutboundIP()

	req := HeartbeatRequest{
		LicenseKey:      c.config.LicenseKey,
		ServerIP:        serverIP,
		SubscriberCount: subscriberCount,
		OnlineCount:     onlineCount,
		CPUUsage:        cpuUsage,
		MemoryUsage:     memUsage,
		DiskUsage:       diskUsage,
		Version:         getVersion(),
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Post(
		c.config.ServerURL+"/api/v1/license/heartbeat",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.Success {
		security.SetValidationPoint("heartbeat", false)
		return fmt.Errorf("heartbeat failed: %s", result.Message)
	}

	security.SetValidationPoint("heartbeat", true)
	return nil
}

// Helper functions

func getOutboundIP() (string, error) {
	// Use SERVER_IP from environment if set (for Docker containers)
	if serverIP := os.Getenv("SERVER_IP"); serverIP != "" {
		return serverIP, nil
	}
	// Fallback to auto-detection
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

func getMACAddress() (string, error) {
	// Use SERVER_MAC from environment if set (for Docker containers)
	if serverMAC := os.Getenv("SERVER_MAC"); serverMAC != "" {
		return serverMAC, nil
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
			mac := iface.HardwareAddr.String()
			if mac != "" {
				return mac, nil
			}
		}
	}

	return "", fmt.Errorf("no MAC address found")
}

func getVersion() string {
	// Try hardcoded path first
	data, err := os.ReadFile("/opt/proxpanel/VERSION")
	if err == nil && len(data) > 0 {
		version := strings.TrimSpace(string(data))
		if version != "" {
			return version
		}
	}

	// Fallback to environment variable
	envVersion := os.Getenv("PROXPANEL_VERSION")
	if envVersion != "" {
		return envVersion
	}

	// Default fallback
	return "unknown"
}

// AsyncSubscriberValidation runs subscriber count validation in background
// This doesn't block the application but enforces limits
func AsyncSubscriberValidation(currentCount int) {
	if defaultClient == nil || devMode {
		return
	}

	go func() {
		allowed, msg, err := VerifySubscriberCount(currentCount)
		if err != nil {
			log.Printf("Async subscriber validation error: %v", err)
			return
		}
		if !allowed {
			log.Printf("WARNING: Subscriber limit exceeded: %s", msg)
			// Update local state to reflect limit exceeded
			defaultClient.mutex.Lock()
			if defaultClient.licenseInfo != nil {
				defaultClient.licenseInfo.WarningMessage = msg
			}
			defaultClient.mutex.Unlock()
		}
	}()
}

// StartAsyncHeartbeat starts a background goroutine that sends heartbeats
// without blocking the main application
func StartAsyncHeartbeat(getStats func() (int, int, float64, float64, float64)) {
	if defaultClient == nil || devMode {
		return
	}

	go func() {
		// Initial delay before first heartbeat
		time.Sleep(10 * time.Second)

		ticker := time.NewTicker(60 * time.Second) // Heartbeat every 60 seconds
		defer ticker.Stop()

		for {
			select {
			case <-defaultClient.stopChan:
				return
			case <-ticker.C:
				subscriberCount, onlineCount, cpu, mem, disk := getStats()
				if err := defaultClient.sendHeartbeat(subscriberCount, onlineCount, cpu, mem, disk); err != nil {
					log.Printf("Async heartbeat failed: %v", err)
				}
			}
		}
	}()
}

// WASubscriptionStatus holds the result of a WhatsApp subscription check
type WASubscriptionStatus struct {
	CanUse    bool       `json:"can_use"`
	Type      string     `json:"type"`      // trial, active, expired, cancelled
	TrialEnd  *time.Time `json:"trial_end"`
	ExpiresAt *time.Time `json:"expires_at"`
	DaysLeft  int        `json:"days_left"`
}

// waSubCache is a local cache of the last known WhatsApp subscription status per reseller.
// Key: resellerDBID (as string). Cached for 5 minutes to survive brief outages.
var (
	waSubCache   = make(map[int]*WASubscriptionStatus)
	waSubCacheAt = make(map[int]time.Time)
	waSubMu      sync.Mutex
)

const waSubCacheTTL = 5 * time.Minute

// CheckWhatsAppSubscription checks (or creates) the WhatsApp subscription for the given reseller.
// resellerDBID=0 means the admin account.
// On network/license errors it returns the last cached result (fail-secure: deny if no cache).
func CheckWhatsAppSubscription(resellerDBID int, resellerName, accountUnique string) (*WASubscriptionStatus, error) {
	if defaultClient == nil || devMode {
		return &WASubscriptionStatus{CanUse: true, Type: "trial", DaysLeft: 2}, nil
	}

	serverURL := defaultClient.config.ServerURL
	licenseKey := defaultClient.config.LicenseKey

	type reqBody struct {
		ResellerDBID  *int   `json:"reseller_db_id"`
		ResellerName  string `json:"reseller_name"`
		AccountUnique string `json:"account_unique"`
	}

	var rdbID *int
	if resellerDBID > 0 {
		rdbID = &resellerDBID
	}

	bodyData, _ := json.Marshal(reqBody{
		ResellerDBID:  rdbID,
		ResellerName:  resellerName,
		AccountUnique: accountUnique,
	})

	req, err := http.NewRequest("POST", serverURL+"/api/v1/license/whatsapp/check", bytes.NewBuffer(bodyData))
	if err != nil {
		return waSubCachedOrDeny(resellerDBID, fmt.Sprintf("failed to build request: %v", err))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-License-Key", licenseKey)

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("WhatsApp subscription check failed (network error): %v — using cache/deny", err)
		return waSubCachedOrDeny(resellerDBID, err.Error())
	}
	defer resp.Body.Close()

	var result struct {
		Success   bool       `json:"success"`
		CanUse    bool       `json:"can_use"`
		Type      string     `json:"type"`
		TrialEnd  *time.Time `json:"trial_end"`
		ExpiresAt *time.Time `json:"expires_at"`
		DaysLeft  int        `json:"days_left"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("WhatsApp subscription check: bad response — using cache/deny")
		return waSubCachedOrDeny(resellerDBID, "bad response from license server")
	}

	status := &WASubscriptionStatus{
		CanUse:    result.CanUse,
		Type:      result.Type,
		TrialEnd:  result.TrialEnd,
		ExpiresAt: result.ExpiresAt,
		DaysLeft:  result.DaysLeft,
	}

	// Cache the successful response
	waSubMu.Lock()
	waSubCache[resellerDBID] = status
	waSubCacheAt[resellerDBID] = time.Now()
	waSubMu.Unlock()

	return status, nil
}

// waSubCachedOrDeny returns the cached subscription status if still fresh, otherwise denies.
func waSubCachedOrDeny(resellerDBID int, reason string) (*WASubscriptionStatus, error) {
	waSubMu.Lock()
	cached, ok := waSubCache[resellerDBID]
	cachedAt := waSubCacheAt[resellerDBID]
	waSubMu.Unlock()

	if ok && time.Since(cachedAt) < waSubCacheTTL {
		log.Printf("WhatsApp subscription: using cached result (CanUse=%v) due to: %s", cached.CanUse, reason)
		return cached, nil
	}

	log.Printf("WhatsApp subscription: no valid cache, denying access due to: %s", reason)
	return &WASubscriptionStatus{CanUse: false, Type: "error", DaysLeft: 0}, nil
}
