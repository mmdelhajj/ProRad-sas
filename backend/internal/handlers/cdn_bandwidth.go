package handlers

import (
	"encoding/json"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/services"
)

type CDNBandwidthHandler struct {
	svc *services.CDNBandwidthRuleService
}

func NewCDNBandwidthHandler(svc *services.CDNBandwidthRuleService) *CDNBandwidthHandler {
	return &CDNBandwidthHandler{svc: svc}
}

// CDNBandwidthRule model for the database
type CDNBandwidthRule struct {
	ID                 uint            `gorm:"primaryKey" json:"id"`
	Name               string          `gorm:"size:100;not null" json:"name"`
	StartTime          string          `gorm:"size:10" json:"start_time"`           // HH:MM format (24h)
	EndTime            string          `gorm:"size:10" json:"end_time"`             // HH:MM format (24h)
	DaysOfWeek         json.RawMessage `gorm:"type:json" json:"days_of_week"`       // Array of day numbers (0=Sun to 6=Sat)
	SpeedMultiplier    int             `gorm:"default:100" json:"speed_multiplier"` // 100 = no change, 200 = double, 50 = half
	CDNIDs             json.RawMessage `gorm:"type:json" json:"cdn_ids"`            // Array of CDN IDs rule applies to (empty = all)
	ServiceIDs         json.RawMessage `gorm:"type:json" json:"service_ids"`        // Array of service IDs rule applies to (empty = all)
	Priority           int             `gorm:"default:10" json:"priority"`          // Lower number = higher priority
	Enabled            bool            `gorm:"default:true" json:"enabled"`
	AutoApply          bool            `gorm:"default:false" json:"auto_apply"` // true = apply automatically on schedule
}

func (CDNBandwidthRule) TableName() string {
	return "cdn_bandwidth_rules"
}

// ListRules returns all CDN bandwidth rules
func (h *CDNBandwidthHandler) ListRules(c *fiber.Ctx) error {
	var rules []CDNBandwidthRule

	// Auto-migrate the table if it doesn't exist
	database.DB.AutoMigrate(&CDNBandwidthRule{})

	database.DB.Order("priority ASC, id DESC").Find(&rules)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    rules,
	})
}

// GetRule returns a single rule
func (h *CDNBandwidthHandler) GetRule(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid rule ID",
		})
	}

	var rule CDNBandwidthRule
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

// CreateRule creates a new CDN bandwidth rule
func (h *CDNBandwidthHandler) CreateRule(c *fiber.Ctx) error {
	var rule CDNBandwidthRule
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
	if rule.SpeedMultiplier == 0 {
		rule.SpeedMultiplier = 100
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
func (h *CDNBandwidthHandler) UpdateRule(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid rule ID",
		})
	}

	var rule CDNBandwidthRule
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
	if cdnIDs, ok := updates["cdn_ids"]; ok {
		if cdnIDs != nil {
			jsonBytes, err := json.Marshal(cdnIDs)
			if err == nil {
				updates["cdn_ids"] = json.RawMessage(jsonBytes)
			}
		}
	}
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

// DeleteRule deletes a CDN bandwidth rule
func (h *CDNBandwidthHandler) DeleteRule(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid rule ID",
		})
	}

	result := database.DB.Delete(&CDNBandwidthRule{}, id)
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

// ApplyNow manually applies a CDN bandwidth rule immediately
func (h *CDNBandwidthHandler) ApplyNow(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid rule ID",
		})
	}

	var rule CDNBandwidthRule
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
		"message":       "CDN rule applied successfully",
		"rule_id":       rule.ID,
		"rule_name":     rule.Name,
		"applied_count": appliedCount,
	})
}
