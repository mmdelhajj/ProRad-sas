package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

// APIKeyHandler handles API key CRUD operations
type APIKeyHandler struct{}

// NewAPIKeyHandler creates a new API key handler
func NewAPIKeyHandler() *APIKeyHandler {
	return &APIKeyHandler{}
}

// Create generates a new API key
func (h *APIKeyHandler) Create(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Authentication required"})
	}

	var req struct {
		Name      string  `json:"name"`
		Scopes    string  `json:"scopes"`
		ExpiresAt *string `json:"expires_at"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	if req.Name == "" {
		req.Name = "API Key"
	}
	if req.Scopes == "" {
		req.Scopes = "read"
	}

	// Generate random 40-char key: pk_live_ + 32 hex chars
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to generate key"})
	}
	fullKey := "pk_live_" + hex.EncodeToString(randomBytes)
	keyPrefix := fullKey[:8]

	// Hash the key for storage
	hash := sha256.Sum256([]byte(fullKey))
	keyHash := hex.EncodeToString(hash[:])

	// Parse expiry
	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse("2006-01-02", *req.ExpiresAt)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid expires_at format. Use YYYY-MM-DD"})
		}
		expiresAt = &t
	}

	apiKey := models.APIKey{
		UserID:    userID,
		Name:      req.Name,
		KeyPrefix: keyPrefix,
		KeyHash:   keyHash,
		Scopes:    req.Scopes,
		IsActive:  true,
		ExpiresAt: expiresAt,
	}

	if err := database.DB.Create(&apiKey).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to create API key"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"id":         apiKey.ID,
			"name":       apiKey.Name,
			"key":        fullKey, // Only returned once!
			"key_prefix": apiKey.KeyPrefix,
			"scopes":     apiKey.Scopes,
			"is_active":  apiKey.IsActive,
			"expires_at": apiKey.ExpiresAt,
			"created_at": apiKey.CreatedAt,
		},
		"message": "API key created. Copy the key now — it won't be shown again.",
	})
}

// List returns all API keys for the current user
func (h *APIKeyHandler) List(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Authentication required"})
	}

	var keys []models.APIKey
	if err := database.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&keys).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to list API keys"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    keys,
	})
}

// Revoke deactivates an API key
func (h *APIKeyHandler) Revoke(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Authentication required"})
	}

	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid key ID"})
	}

	result := database.DB.Model(&models.APIKey{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("is_active", false)

	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to revoke key"})
	}
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "API key not found"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "API key revoked successfully",
	})
}

// GetLogs returns usage logs for a specific API key
func (h *APIKeyHandler) GetLogs(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Authentication required"})
	}

	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid key ID"})
	}

	// Verify ownership
	var key models.APIKey
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&key).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "API key not found"})
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	offset := (page - 1) * limit

	var logs []models.APIKeyLog
	var total int64

	database.DB.Model(&models.APIKeyLog{}).Where("api_key_id = ?", id).Count(&total)
	database.DB.Where("api_key_id = ?", id).
		Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&logs)

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    logs,
		"pagination": fiber.Map{
			"page":  page,
			"limit": limit,
			"total": total,
			"pages": totalPages,
		},
	})
}

// GetStats returns aggregate stats for API key usage
func (h *APIKeyHandler) GetStats(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Authentication required"})
	}

	var totalKeys int64
	var activeKeys int64
	database.DB.Model(&models.APIKey{}).Where("user_id = ?", userID).Count(&totalKeys)
	database.DB.Model(&models.APIKey{}).Where("user_id = ? AND is_active = true", userID).Count(&activeKeys)

	// Request count last 24h
	var requestCount24h int64
	database.DB.Model(&models.APIKeyLog{}).
		Joins("JOIN api_keys ON api_keys.id = api_key_logs.api_key_id").
		Where("api_keys.user_id = ? AND api_key_logs.created_at > ?", userID, time.Now().Add(-24*time.Hour)).
		Count(&requestCount24h)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"total_keys":        totalKeys,
			"active_keys":       activeKeys,
			"requests_last_24h": requestCount24h,
			"rate_limit":        fmt.Sprintf("%d requests/minute", 60),
		},
	})
}
