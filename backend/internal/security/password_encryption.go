package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

const (
	// PasswordEncryptionPrefix marks encrypted passwords
	PasswordEncryptionPrefix = "ENC:"
)

var (
	passwordEncryptionKey []byte
	passwordKeyOnce       sync.Once
	passwordKeyErr        error
)

// initPasswordKey initializes the password encryption key from environment
func initPasswordKey() {
	keyHex := os.Getenv("PROISP_PASSWORD_KEY")
	if keyHex == "" {
		// Try to derive from hardware fingerprint as fallback
		log.Println("WARNING: PROISP_PASSWORD_KEY not set, using hardware-derived key")
		passwordEncryptionKey = deriveConfigKey()
		return
	}

	// Decode hex key (64 chars = 32 bytes)
	key, err := hexDecode(keyHex)
	if err != nil || len(key) != 32 {
		passwordKeyErr = errors.New("PROISP_PASSWORD_KEY must be 64 hex characters (32 bytes)")
		log.Printf("ERROR: %v", passwordKeyErr)
		return
	}
	passwordEncryptionKey = key
}

// hexDecode decodes hex string to bytes
func hexDecode(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, errors.New("hex string has odd length")
	}
	result := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		var b byte
		for j := 0; j < 2; j++ {
			c := s[i+j]
			switch {
			case c >= '0' && c <= '9':
				b = b*16 + (c - '0')
			case c >= 'a' && c <= 'f':
				b = b*16 + (c - 'a' + 10)
			case c >= 'A' && c <= 'F':
				b = b*16 + (c - 'A' + 10)
			default:
				return nil, errors.New("invalid hex character")
			}
		}
		result[i/2] = b
	}
	return result, nil
}

// getPasswordKey returns the password encryption key, initializing if needed
func getPasswordKey() ([]byte, error) {
	passwordKeyOnce.Do(initPasswordKey)
	if passwordKeyErr != nil {
		return nil, passwordKeyErr
	}
	if passwordEncryptionKey == nil {
		return nil, errors.New("password encryption key not initialized")
	}
	return passwordEncryptionKey, nil
}

// EncryptPassword encrypts a plaintext password for database storage
// Returns the encrypted password prefixed with "ENC:" or the original if encryption fails
func EncryptPassword(plaintext string) string {
	if plaintext == "" {
		return ""
	}

	// Already encrypted?
	if strings.HasPrefix(plaintext, PasswordEncryptionPrefix) {
		return plaintext
	}

	key, err := getPasswordKey()
	if err != nil {
		log.Printf("Warning: Password encryption disabled: %v", err)
		return plaintext
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		log.Printf("Warning: Failed to create cipher: %v", err)
		return plaintext
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		log.Printf("Warning: Failed to create GCM: %v", err)
		return plaintext
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		log.Printf("Warning: Failed to generate nonce: %v", err)
		return plaintext
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	encoded := base64.StdEncoding.EncodeToString(ciphertext)

	return PasswordEncryptionPrefix + encoded
}

// DecryptPassword decrypts an encrypted password
// Returns the plaintext password or the original string if not encrypted
func DecryptPassword(encrypted string) string {
	if encrypted == "" {
		return ""
	}

	// Not encrypted?
	if !strings.HasPrefix(encrypted, PasswordEncryptionPrefix) {
		return encrypted
	}

	// Remove prefix
	encoded := strings.TrimPrefix(encrypted, PasswordEncryptionPrefix)

	key, err := getPasswordKey()
	if err != nil {
		log.Printf("Warning: Password decryption failed - key error: %v", err)
		return ""
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		log.Printf("Warning: Password decryption failed - base64 decode: %v", err)
		return ""
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		log.Printf("Warning: Password decryption failed - cipher: %v", err)
		return ""
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		log.Printf("Warning: Password decryption failed - GCM: %v", err)
		return ""
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		log.Printf("Warning: Password decryption failed - ciphertext too short")
		return ""
	}

	nonce, cipherData := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		log.Printf("Warning: Password decryption failed - decryption: %v", err)
		return ""
	}

	return string(plaintext)
}

// IsPasswordEncrypted checks if a password is already encrypted
func IsPasswordEncrypted(password string) bool {
	return strings.HasPrefix(password, PasswordEncryptionPrefix)
}

// GeneratePasswordKey generates a new random encryption key for passwords
// This should be called during initial setup to generate PROISP_PASSWORD_KEY
func GeneratePasswordKey() string {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return ""
	}

	// Convert to hex
	const hexChars = "0123456789abcdef"
	result := make([]byte, 64)
	for i, b := range key {
		result[i*2] = hexChars[b>>4]
		result[i*2+1] = hexChars[b&0x0f]
	}
	return string(result)
}
