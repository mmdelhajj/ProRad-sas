package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

// rateLimitEntry tracks request counts per API key
type rateLimitEntry struct {
	count    int
	windowStart time.Time
}

var (
	apiKeyRateLimits = make(map[uint]*rateLimitEntry)
	apiKeyRateMu     sync.Mutex
)

const apiKeyRateLimit = 60 // requests per minute

// APIKeyAuth middleware authenticates requests using X-API-Key header
func APIKeyAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiKey := c.Get("X-API-Key")
		if apiKey == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success":   false,
				"error":     fiber.Map{"code": "MISSING_API_KEY", "message": "X-API-Key header is required"},
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		}

		// Hash the key
		hash := sha256.Sum256([]byte(apiKey))
		keyHash := hex.EncodeToString(hash[:])

		// Look up the key
		var key models.APIKey
		if err := database.DB.Where("key_hash = ?", keyHash).First(&key).Error; err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success":   false,
				"error":     fiber.Map{"code": "INVALID_API_KEY", "message": "Invalid API key"},
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		}

		// Check if active
		if !key.IsActive {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success":   false,
				"error":     fiber.Map{"code": "API_KEY_REVOKED", "message": "This API key has been revoked"},
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		}

		// Check expiry
		if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success":   false,
				"error":     fiber.Map{"code": "API_KEY_EXPIRED", "message": "This API key has expired"},
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		}

		// Rate limit check
		if !checkAPIKeyRateLimit(key.ID) {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"success":   false,
				"error":     fiber.Map{"code": "RATE_LIMIT_EXCEEDED", "message": fmt.Sprintf("Rate limit exceeded. Maximum %d requests per minute.", apiKeyRateLimit)},
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		}

		// Look up the user
		var user models.User
		if err := database.DB.First(&user, key.UserID).Error; err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success":   false,
				"error":     fiber.Map{"code": "INVALID_USER", "message": "API key owner not found"},
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		}

		// Store context (same locals as JWT auth)
		c.Locals("user", &user)
		c.Locals("userID", user.ID)
		c.Locals("username", user.Username)
		c.Locals("userType", user.UserType)
		c.Locals("resellerID", user.ResellerID)
		c.Locals("apiKeyID", key.ID)
		c.Locals("apiKeyScopes", key.Scopes)

		// Update last_used_at in goroutine (non-blocking)
		go func() {
			now := time.Now()
			database.DB.Model(&models.APIKey{}).Where("id = ?", key.ID).Update("last_used_at", now)
		}()

		// Log request in goroutine (non-blocking)
		startTime := time.Now()
		c.Locals("apiKeyStartTime", startTime)

		// Continue to handler
		err := c.Next()

		// Log after response
		go func() {
			duration := time.Since(startTime).Milliseconds()
			logEntry := models.APIKeyLog{
				APIKeyID:   key.ID,
				Method:     c.Method(),
				Path:       c.Path(),
				StatusCode: c.Response().StatusCode(),
				IPAddress:  c.IP(),
				DurationMs: int(duration),
			}
			if dbErr := database.DB.Create(&logEntry).Error; dbErr != nil {
				log.Printf("Failed to log API key usage: %v", dbErr)
			}
		}()

		return err
	}
}

// RequireScope checks if the API key has the required scope
func RequireScope(scope string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		scopes, ok := c.Locals("apiKeyScopes").(string)
		if !ok {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success":   false,
				"error":     fiber.Map{"code": "SCOPE_REQUIRED", "message": fmt.Sprintf("This endpoint requires '%s' scope", scope)},
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		}

		scopeList := strings.Split(scopes, ",")
		for _, s := range scopeList {
			if strings.TrimSpace(s) == scope {
				return c.Next()
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success":   false,
			"error":     fiber.Map{"code": "INSUFFICIENT_SCOPE", "message": fmt.Sprintf("This endpoint requires '%s' scope. Your key has: %s", scope, scopes)},
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// checkAPIKeyRateLimit returns true if the request is within rate limit
func checkAPIKeyRateLimit(keyID uint) bool {
	apiKeyRateMu.Lock()
	defer apiKeyRateMu.Unlock()

	now := time.Now()
	entry, exists := apiKeyRateLimits[keyID]

	if !exists || now.Sub(entry.windowStart) > time.Minute {
		// New window
		apiKeyRateLimits[keyID] = &rateLimitEntry{
			count:       1,
			windowStart: now,
		}
		return true
	}

	entry.count++
	return entry.count <= apiKeyRateLimit
}

// CleanupAPIKeyRateLimits removes expired rate limit entries (call periodically)
func init() {
	go func() {
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			apiKeyRateMu.Lock()
			now := time.Now()
			for id, entry := range apiKeyRateLimits {
				if now.Sub(entry.windowStart) > 2*time.Minute {
					delete(apiKeyRateLimits, id)
				}
			}
			apiKeyRateMu.Unlock()
		}
	}()
}
