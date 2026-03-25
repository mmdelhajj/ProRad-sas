package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const (
	configFileName = ".proisp.enc"
	configVersion  = "v1"
)

// EncryptedConfig holds sensitive configuration
type EncryptedConfig struct {
	Version    string `json:"version"`
	DBHost     string `json:"db_host"`
	DBPort     string `json:"db_port"`
	DBUser     string `json:"db_user"`
	DBPassword string `json:"db_password"`
	DBName     string `json:"db_name"`
	RedisHost  string `json:"redis_host"`
	RedisPort  string `json:"redis_port"`
	RedisPass  string `json:"redis_password"`
	JWTSecret  string `json:"jwt_secret"`
}

// deriveKey creates an encryption key from hardware fingerprint
func deriveConfigKey() []byte {
	fingerprint := HardwareFingerprint()
	// Add salt
	salted := fingerprint + "proisp-config-encryption-v1"
	hash := sha256.Sum256([]byte(salted))
	return hash[:]
}

// SaveEncryptedConfig encrypts and saves configuration
func SaveEncryptedConfig(config *EncryptedConfig) error {
	config.Version = configVersion

	plaintext, err := json.Marshal(config)
	if err != nil {
		return err
	}

	key := deriveConfigKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	encoded := base64.StdEncoding.EncodeToString(ciphertext)

	// Write to file
	configPath := getConfigPath()
	return os.WriteFile(configPath, []byte(encoded), 0600)
}

// LoadEncryptedConfig loads and decrypts configuration
func LoadEncryptedConfig() (*EncryptedConfig, error) {
	configPath := getConfigPath()

	encoded, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	ciphertext, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return nil, err
	}

	key := deriveConfigKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("decryption failed - hardware mismatch or corrupted config")
	}

	var config EncryptedConfig
	if err := json.Unmarshal(plaintext, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// ConfigExists checks if encrypted config file exists
func ConfigExists() bool {
	configPath := getConfigPath()
	_, err := os.Stat(configPath)
	return err == nil
}

func getConfigPath() string {
	// Store in /etc/proisp/ or current directory
	etcPath := "/etc/proisp"
	if _, err := os.Stat(etcPath); err == nil {
		return filepath.Join(etcPath, configFileName)
	}

	// Fallback to /app directory (Docker)
	return filepath.Join("/app", configFileName)
}

// MigrateEnvToEncrypted migrates environment variables to encrypted config
func MigrateEnvToEncrypted() error {
	config := &EncryptedConfig{
		DBHost:     os.Getenv("DB_HOST"),
		DBPort:     os.Getenv("DB_PORT"),
		DBUser:     os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:     os.Getenv("DB_NAME"),
		RedisHost:  os.Getenv("REDIS_HOST"),
		RedisPort:  os.Getenv("REDIS_PORT"),
		RedisPass:  os.Getenv("REDIS_PASSWORD"),
		JWTSecret:  os.Getenv("JWT_SECRET"),
	}

	return SaveEncryptedConfig(config)
}
