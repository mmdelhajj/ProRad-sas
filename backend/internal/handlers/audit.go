package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/middleware"
	"github.com/proisp/backend/internal/models"
)

type AuditHandler struct{}

func NewAuditHandler() *AuditHandler {
	return &AuditHandler{}
}

// List returns audit logs
func (h *AuditHandler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	action := c.Query("action", "")
	entityType := c.Query("entity_type", "")
	userID := c.QueryInt("user_id", 0)
	dateFrom := c.Query("date_from", "")
	dateTo := c.Query("date_to", "")

	if page < 1 {
		page = 1
	}
	if limit > 200 {
		limit = 200
	}
	offset := (page - 1) * limit

	query := database.DB.Model(&models.AuditLog{}).Preload("User")

	// Resellers only see their own actions
	user := middleware.GetCurrentUser(c)
	if user != nil && user.UserType == models.UserTypeReseller {
		query = query.Where("user_id = ?", user.ID)
	}

	if action != "" {
		query = query.Where("action = ?", action)
	}
	if entityType != "" {
		query = query.Where("entity_type = ?", entityType)
	}
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if dateFrom != "" {
		query = query.Where("created_at >= ?", dateFrom)
	}
	if dateTo != "" {
		query = query.Where("created_at <= ?", dateTo+" 23:59:59")
	}

	var total int64
	query.Count(&total)

	var logs []models.AuditLog
	query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    logs,
		"meta": fiber.Map{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// Get returns a single audit log entry
func (h *AuditHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")

	var log models.AuditLog
	if err := database.DB.Preload("User").First(&log, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Log entry not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    log,
	})
}

// GetActions returns available action types
func (h *AuditHandler) GetActions(c *fiber.Ctx) error {
	var actions []string
	database.DB.Model(&models.AuditLog{}).Distinct("action").Pluck("action", &actions)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    actions,
	})
}

// GetEntityTypes returns available entity types
func (h *AuditHandler) GetEntityTypes(c *fiber.Ctx) error {
	var entityTypes []string
	database.DB.Model(&models.AuditLog{}).Distinct("entity_type").Pluck("entity_type", &entityTypes)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    entityTypes,
	})
}

// LogAction creates an audit log entry (helper function)
func LogAction(c *fiber.Ctx, action models.AuditAction, entityType string, entityID uint, description string) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return
	}

	log := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      action,
		EntityType:  entityType,
		EntityID:    entityID,
		Description: description,
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&log)
}
