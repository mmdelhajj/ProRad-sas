package handlers

import (
	"encoding/json"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/services"
)

type BandwidthHandler struct {
	svc *services.BandwidthRuleService
}

func NewBandwidthHandler(svc *services.BandwidthRuleService) *BandwidthHandler {
	return &BandwidthHandler{svc: svc}
}

// BandwidthRule model for the database
type BandwidthRule struct {
	ID                 uint            `gorm:"primaryKey" json:"id"`
	Name               string          `gorm:"size:100;not null" json:"name"`
	TriggerType        string          `gorm:"size:20;not null;default:time" json:"trigger_type"` // time, quota, fup
	StartTime          string          `gorm:"size:10" json:"start_time"`
	EndTime            string          `gorm:"size:10" json:"end_time"`
	DaysOfWeek         json.RawMessage `gorm:"type:json" json:"days_of_week"`
	UploadMultiplier   int             `gorm:"default:100" json:"upload_multiplier"`   // 100 = no change, 200 = double
	DownloadMultiplier int             `gorm:"default:100" json:"download_multiplier"` // 100 = no change, 50 = half
	ServiceIDs         json.RawMessage `gorm:"type:json" json:"service_ids"`
	Priority           int             `gorm:"default:10" json:"priority"`
	Enabled            bool            `gorm:"default:true" json:"enabled"`
	AutoApply          bool            `gorm:"default:false" json:"auto_apply"` // true = apply automatically on schedule
}

func (BandwidthRule) TableName() string {
	return "bandwidth_rules"
}

// ListRules returns all bandwidth rules
func (h *BandwidthHandler) ListRules(c *fiber.Ctx) error {
	var rules []BandwidthRule

	// Auto-migrate the table if it doesn't exist
	database.DB.AutoMigrate(&BandwidthRule{})

	database.DB.Order("priority ASC, id DESC").Find(&rules)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    rules,
	})
}

// GetRule returns a single rule
func (h *BandwidthHandler) GetRule(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid rule ID",
		})
	}

	var rule BandwidthRule
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

// CreateRule creates a new bandwidth rule
func (h *BandwidthHandler) CreateRule(c *fiber.Ctx) error {
	var rule BandwidthRule
	if err := c.BodyParser(&rule); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if rule.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Rule name is required",
		})
	}

	// Set defaults
	if rule.UploadMultiplier == 0 {
		rule.UploadMultiplier = 100
	}
	if rule.DownloadMultiplier == 0 {
		rule.DownloadMultiplier = 100
	}
	if rule.Priority == 0 {
		rule.Priority = 10
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

// UpdateRule updates an existing rule
func (h *BandwidthHandler) UpdateRule(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid rule ID",
		})
	}

	var rule BandwidthRule
	if err := database.DB.First(&rule, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Rule not found",
		})
	}

	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Handle JSON fields that need to be marshaled
	if serviceIDs, ok := updates["service_ids"]; ok {
		if serviceIDs != nil {
			jsonBytes, err := json.Marshal(serviceIDs)
			if err == nil {
				updates["service_ids"] = json.RawMessage(jsonBytes)
			}
		}
	}
	if daysOfWeek, ok := updates["days_of_week"]; ok {
		if daysOfWeek != nil {
			jsonBytes, err := json.Marshal(daysOfWeek)
			if err == nil {
				updates["days_of_week"] = json.RawMessage(jsonBytes)
			}
		}
	}

	// Use Select to explicitly update all fields including false booleans
	if err := database.DB.Model(&rule).Select("*").Updates(updates).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update rule: " + err.Error(),
		})
	}

	database.DB.First(&rule, id)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    rule,
	})
}

// DeleteRule deletes a bandwidth rule
func (h *BandwidthHandler) DeleteRule(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid rule ID",
		})
	}

	result := database.DB.Delete(&BandwidthRule{}, id)
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Rule not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Rule deleted successfully",
	})
}

// ApplyNow manually applies a bandwidth rule immediately
func (h *BandwidthHandler) ApplyNow(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid rule ID",
		})
	}

	var rule BandwidthRule
	if err := database.DB.First(&rule, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Rule not found",
		})
	}

	// Apply the rule immediately using the service
	appliedCount, err := h.svc.ApplyRuleNow(uint(id))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success":       true,
		"message":       "Rule applied successfully",
		"rule_id":       rule.ID,
		"rule_name":     rule.Name,
		"applied_count": appliedCount,
	})
}
