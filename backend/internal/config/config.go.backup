package config

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/proisp/backend/internal/security"
)

// LicenseSecrets holds secrets fetched from license server
type LicenseSecrets struct {
	DBPassword    string `json:"db_password"`
	RedisPassword string `json:"redis_password"`
	JWTSecret     string `json:"jwt_secret"`
	EncryptionKey string `json:"encryption_key"`
}

// Global secrets from license server
var licenseSecrets *LicenseSecrets

// GetLicenseSecrets returns the fetched license secrets
func GetLicenseSecrets() *LicenseSecrets {
	return licenseSecrets
}

// getStableHardwareID generates hardware ID in the same format as license server expects
// Format: stable_<sha256(stable|MAC|product_uuid|machine_id)>
// Note: Hostname and IP are NOT included - customers can change them freely
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
	return string(content[:len(content)-1]) // Remove trailing newline
}

// fetchSecretsFromLicenseServer fetches secrets from the license server
// This is Option 2 security - secrets never stored on customer's disk
func fetchSecretsFromLicenseServer() (*LicenseSecrets, error) {
	serverURL := os.Getenv("LICENSE_SERVER")
	licenseKey := os.Getenv("LICENSE_KEY")

	if serverURL == "" || licenseKey == "" {
		return nil, fmt.Errorf("LICENSE_SERVER and LICENSE_KEY environment variables required")
	}

	// Build hardware ID in same format as license server expects
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
		Success bool           `json:"success"`
		Data    LicenseSecrets `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("failed to fetch secrets from license server")
	}

	log.Println("Secrets fetched from license server successfully (Option 2 security active)")
	return &response.Data, nil
}

type Config struct {
	// Database
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string

	// Redis
	RedisHost     string
	RedisPort     int
	RedisPassword string

	// JWT
	JWTSecret      string
	JWTExpireHours int

	// API
	APIPort int

	// RADIUS
	RadiusAuthPort int
	RadiusAcctPort int
	RadiusSecret   string
}

// generateSecureSecret generates a cryptographically secure random secret
func generateSecureSecret(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to a timestamp-based approach if crypto/rand fails
		return hex.EncodeToString([]byte(os.Getenv("HOSTNAME") + string(rune(length))))
	}
	return hex.EncodeToString(bytes)
}

func Load() *Config {
	var jwtSecret, dbPassword, redisPassword string

	// Try to fetch secrets from license server (Option 2 security)
	secrets, err := fetchSecretsFromLicenseServer()
	if err != nil {
		log.Printf("Could not fetch secrets from license server: %v", err)
		log.Println("Falling back to environment variables for secrets...")

		// Fallback to environment variables
		jwtSecret = os.Getenv("JWT_SECRET")
		if jwtSecret == "" {
			jwtSecret = generateSecureSecret(32) // 64 character hex string
			log.Println("WARNING: JWT_SECRET not set - generated random secret. Sessions will not persist across restarts.")
		}

		dbPassword = getEnv("DB_PASSWORD", "")
		if dbPassword == "" {
			log.Println("WARNING: DB_PASSWORD not set - this is insecure for production!")
			dbPassword = "changeme"
		}

		redisPassword = getEnv("REDIS_PASSWORD", "")
		if redisPassword == "" {
			log.Println("WARNING: REDIS_PASSWORD not set - Redis is not secured!")
		}
	} else {
		// Use secrets from license server
		licenseSecrets = secrets
		jwtSecret = secrets.JWTSecret
		dbPassword = secrets.DBPassword
		redisPassword = secrets.RedisPassword
		log.Println("Using secrets from license server (passwords not stored on disk)")
	}

	// RADIUS secret - warn if using default
	radiusSecret := getEnv("RADIUS_SECRET", "")
	if radiusSecret == "" {
		log.Println("WARNING: RADIUS_SECRET not set - using insecure default!")
		radiusSecret = "changeme"
	}

	return &Config{
		// Database
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnvInt("DB_PORT", 5432),
		DBUser:     getEnv("DB_USER", "proisp"),
		DBPassword: dbPassword,
		DBName:     getEnv("DB_NAME", "proisp"),

		// Redis
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnvInt("REDIS_PORT", 6379),
		RedisPassword: redisPassword,

		// JWT
		JWTSecret:      jwtSecret,
		JWTExpireHours: getEnvInt("JWT_EXPIRE_HOURS", 168), // 7 days default

		// API
		APIPort: getEnvInt("API_PORT", 8080),

		// RADIUS
		RadiusAuthPort: getEnvInt("RADIUS_AUTH_PORT", 1812),
		RadiusAcctPort: getEnvInt("RADIUS_ACCT_PORT", 1813),
		RadiusSecret:   radiusSecret,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
