package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

type CommunicationHandler struct{}

func NewCommunicationHandler() *CommunicationHandler {
	return &CommunicationHandler{}
}

// Templates

// ListTemplates returns all communication templates
func (h *CommunicationHandler) ListTemplates(c *fiber.Ctx) error {
	templateType := c.Query("type", "")

	query := database.DB.Model(&models.CommunicationTemplate{})
	if templateType != "" {
		query = query.Where("type = ?", templateType)
	}

	var templates []models.CommunicationTemplate
	query.Order("name").Find(&templates)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    templates,
	})
}

// GetTemplate returns a single template
func (h *CommunicationHandler) GetTemplate(c *fiber.Ctx) error {
	id := c.Params("id")

	var template models.CommunicationTemplate
	if err := database.DB.First(&template, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Template not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    template,
	})
}

// CreateTemplate creates a new template
func (h *CommunicationHandler) CreateTemplate(c *fiber.Ctx) error {
	var template models.CommunicationTemplate
	if err := c.BodyParser(&template); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if err := database.DB.Create(&template).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create template",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    template,
	})
}

// UpdateTemplate updates a template
func (h *CommunicationHandler) UpdateTemplate(c *fiber.Ctx) error {
	id := c.Params("id")

	var template models.CommunicationTemplate
	if err := database.DB.First(&template, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Template not found",
		})
	}

	var updates models.CommunicationTemplate
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	database.DB.Model(&template).Updates(updates)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    template,
	})
}

// DeleteTemplate deletes a template
func (h *CommunicationHandler) DeleteTemplate(c *fiber.Ctx) error {
	id := c.Params("id")

	result := database.DB.Delete(&models.CommunicationTemplate{}, id)
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Template not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Template deleted",
	})
}

// Rules

// ListRules returns all communication rules
func (h *CommunicationHandler) ListRules(c *fiber.Ctx) error {
	trigger := c.Query("trigger", "")

	query := database.DB.Model(&models.CommunicationRule{})
	if trigger != "" {
		query = query.Where("trigger_event = ?", trigger)
	}

	var rules []models.CommunicationRule
	query.Order("name").Find(&rules)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    rules,
	})
}

// GetRule returns a single rule
func (h *CommunicationHandler) GetRule(c *fiber.Ctx) error {
	id := c.Params("id")

	var rule models.CommunicationRule
	if err := database.DB.First(&rule, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Rule not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    rule,
	})
}

// CreateRule creates a new rule
func (h *CommunicationHandler) CreateRule(c *fiber.Ctx) error {
	var rule models.CommunicationRule
	if err := c.BodyParser(&rule); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if err := database.DB.Create(&rule).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create rule",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    rule,
	})
}

// UpdateRule updates a rule
func (h *CommunicationHandler) UpdateRule(c *fiber.Ctx) error {
	id := c.Params("id")

	var rule models.CommunicationRule
	if err := database.DB.First(&rule, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Rule not found",
		})
	}

	var updates models.CommunicationRule
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	database.DB.Model(&rule).Updates(updates)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    rule,
	})
}

// DeleteRule deletes a rule
func (h *CommunicationHandler) DeleteRule(c *fiber.Ctx) error {
	id := c.Params("id")

	result := database.DB.Delete(&models.CommunicationRule{}, id)
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Rule not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Rule deleted",
	})
}

// Logs

// ListLogs returns communication logs
func (h *CommunicationHandler) ListLogs(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 25)
	status := c.Query("status", "")

	if page < 1 {
		page = 1
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	query := database.DB.Model(&models.CommunicationLog{}).Preload("Subscriber").Preload("Rule")

	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var logs []models.CommunicationLog
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
