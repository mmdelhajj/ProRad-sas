package database

import (
	"crypto/rand"
	"encoding/hex"
	"log"

	"github.com/proisp/backend/internal/config"
)

const jwtSecretKey = "jwt_secret"

// SystemPreference represents a system preference
type SystemPreference struct {
	ID        uint   `gorm:"column:id;primaryKey"`
	Key       string `gorm:"column:key;size:100;uniqueIndex;not null"`
	Value     string `gorm:"column:value;type:text"`
	ValueType string `gorm:"column:value_type;size:20;default:string"`
}

func (SystemPreference) TableName() string {
	return "system_preferences"
}

// EnsureJWTSecret ensures JWT secret is persisted in database
// If not exists, generates and saves. Returns the secret.
func EnsureJWTSecret(cfg *config.Config) string {
	if DB == nil {
		log.Println("Warning: Database not connected, cannot persist JWT secret")
		return cfg.JWTSecret
	}

	var pref SystemPreference
	result := DB.Where("key = ?", jwtSecretKey).First(&pref)

	if result.Error == nil && pref.Value != "" {
		// Found existing secret in database
		log.Println("JWT secret loaded from database - sessions will persist across restarts")
		return pref.Value
	}

	// Generate new secret or use from config
	secret := cfg.JWTSecret
	if secret == "" {
		secret = generateSecureSecret(32)
	}

	// Save to database
	pref = SystemPreference{
		Key:       jwtSecretKey,
		Value:     secret,
		ValueType: "string",
	}

	if err := DB.Create(&pref).Error; err != nil {
		// Try update if create fails (race condition)
		DB.Model(&SystemPreference{}).Where("key = ?", jwtSecretKey).Update("value", secret)
	}

	log.Println("JWT secret generated and persisted to database")
	return secret
}

// generateSecureSecret generates a cryptographically secure random secret
func generateSecureSecret(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback
		return hex.EncodeToString([]byte("fallback-secret-change-me"))
	}
	return hex.EncodeToString(bytes)
}

// GetJWTSecret retrieves the JWT secret from database
func GetJWTSecret() string {
	if DB == nil {
		return ""
	}

	var pref SystemPreference
	if err := DB.Where("key = ?", jwtSecretKey).First(&pref).Error; err != nil {
		return ""
	}

	return pref.Value
}
