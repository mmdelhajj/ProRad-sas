package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
)

// Update package public signing key (base64 encoded ED25519 public key)
// This is the public part of PROXPANEL_SIGNING_KEY from the license server
// It's safe to embed this in the client - it can only VERIFY, not sign
var updateSigningPublicKey ed25519.PublicKey

// DecryptUpdatePackage decrypts an AES-256-GCM encrypted update package
func DecryptUpdatePackage(encryptedData []byte, keyHex string) ([]byte, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption key format: %v", err)
	}

	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes (64 hex chars)")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %v", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		return nil, fmt.Errorf("encrypted data too short")
	}

	nonce, cipherData := encryptedData[:nonceSize], encryptedData[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %v", err)
	}

	return plaintext, nil
}

// SetUpdateSigningPublicKey sets the public key for signature verification
// This should be called during initialization with the key from the license server
func SetUpdateSigningPublicKey(publicKeyBase64 string) error {
	keyBytes, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		return fmt.Errorf("failed to decode public key: %v", err)
	}

	if len(keyBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key size: got %d, expected %d", len(keyBytes), ed25519.PublicKeySize)
	}

	updateSigningPublicKey = keyBytes
	return nil
}

// LoadUpdateSigningPublicKeyFromEnv loads the public key from environment variable
func LoadUpdateSigningPublicKeyFromEnv() error {
	pubKeyBase64 := os.Getenv("PROXPANEL_UPDATE_PUBLIC_KEY")
	if pubKeyBase64 == "" {
		// If not set, signature verification will be skipped with a warning
		return nil
	}
	return SetUpdateSigningPublicKey(pubKeyBase64)
}

// VerifyUpdateSignature verifies the ED25519 signature of an update
// signData should be "version|checksum" format
// signature should be base64 encoded
func VerifyUpdateSignature(signData, signatureBase64 string) error {
	if updateSigningPublicKey == nil {
		// No public key configured - skip verification with warning
		return nil
	}

	signature, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %v", err)
	}

	if !ed25519.Verify(updateSigningPublicKey, []byte(signData), signature) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// CalculateChecksum calculates SHA-256 checksum of data
func CalculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// VerifyUpdatePackage verifies both the signature and checksum of a decrypted package
func VerifyUpdatePackage(data []byte, version, expectedChecksum, signatureBase64 string) error {
	// Calculate checksum of decrypted data
	actualChecksum := CalculateChecksum(data)
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	// Verify signature
	signData := version + "|" + expectedChecksum
	if err := VerifyUpdateSignature(signData, signatureBase64); err != nil {
		return fmt.Errorf("signature verification failed: %v", err)
	}

	return nil
}
