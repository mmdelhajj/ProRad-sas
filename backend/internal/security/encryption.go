package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"sync"
	"time"
)

var (
	encryptionKey     []byte
	keyMutex          sync.RWMutex
	keyExpiry         time.Time
	keyInitialized    bool
	ErrNoKey          = errors.New("encryption key not initialized")
	ErrKeyExpired     = errors.New("encryption key expired")
	ErrDecryptFailed  = errors.New("decryption failed")
)

// InitializeEncryption sets the encryption key from license server
func InitializeEncryption(key string) error {
	return InitializeKey(key)
}

// InitializeKey sets the encryption key from license server
func InitializeKey(key string) error {
	keyMutex.Lock()
	defer keyMutex.Unlock()

	// Derive 32-byte key from provided key using SHA-256
	hash := sha256.Sum256([]byte(key))
	encryptionKey = hash[:]
	keyExpiry = time.Now().Add(24 * time.Hour)
	keyInitialized = true

	return nil
}

// IsKeyValid checks if key is initialized and not expired
func IsKeyValid() bool {
	keyMutex.RLock()
	defer keyMutex.RUnlock()

	return keyInitialized && time.Now().Before(keyExpiry)
}

// GetKeyExpiry returns when the key expires
func GetKeyExpiry() time.Time {
	keyMutex.RLock()
	defer keyMutex.RUnlock()
	return keyExpiry
}

// Encrypt encrypts plaintext using AES-256-GCM
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	keyMutex.RLock()
	if !keyInitialized {
		keyMutex.RUnlock()
		return "", ErrNoKey
	}
	key := make([]byte, len(encryptionKey))
	copy(key, encryptionKey)
	keyMutex.RUnlock()

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts ciphertext using AES-256-GCM
func Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	keyMutex.RLock()
	if !keyInitialized {
		keyMutex.RUnlock()
		return "", ErrNoKey
	}
	key := make([]byte, len(encryptionKey))
	copy(key, encryptionKey)
	keyMutex.RUnlock()

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		// Not encrypted data, return as-is (for migration)
		return ciphertext, nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(data) < gcm.NonceSize() {
		// Not encrypted data, return as-is
		return ciphertext, nil
	}

	nonce, ciphertextBytes := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		// Decryption failed - might be unencrypted data
		return ciphertext, nil
	}

	return string(plaintext), nil
}

// EncryptBytes encrypts byte data
func EncryptBytes(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	keyMutex.RLock()
	if !keyInitialized {
		keyMutex.RUnlock()
		return nil, ErrNoKey
	}
	key := make([]byte, len(encryptionKey))
	copy(key, encryptionKey)
	keyMutex.RUnlock()

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, data, nil), nil
}

// DecryptBytes decrypts byte data
func DecryptBytes(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	keyMutex.RLock()
	if !keyInitialized {
		keyMutex.RUnlock()
		return nil, ErrNoKey
	}
	key := make([]byte, len(encryptionKey))
	copy(key, encryptionKey)
	keyMutex.RUnlock()

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(data) < gcm.NonceSize() {
		return nil, ErrDecryptFailed
	}

	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// HashString creates a one-way hash (for comparison without decryption)
func HashString(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

// GenerateRandomKey generates a random encryption key
func GenerateRandomKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}
