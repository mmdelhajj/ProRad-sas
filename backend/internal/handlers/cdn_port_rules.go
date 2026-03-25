package handlers

import (
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/mikrotik"
	"github.com/proisp/backend/internal/models"
)

// ListPortRules returns all CDN port rules
func (h *CDNHandler) ListPortRules(c *fiber.Ctx) error {
	var rules []models.CDNPortRule
	if err := database.DB.Where("deleted_at IS NULL").Order("created_at DESC").Find(&rules).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch port rules",
		})
	}
	return c.JSON(fiber.Map{"success": true, "data": rules})
}

// CreatePortRule creates a new CDN port rule
func (h *CDNHandler) CreatePortRule(c *fiber.Ctx) error {
	var body struct {
		Name        string `json:"name"`
		Port        string `json:"port"`
		Direction   string `json:"direction"`
		DSCPValue   *int   `json:"dscp_value"`
		SpeedMbps   int64  `json:"speed_mbps"`
		NASID       *uint  `json:"nas_id"`
		IsActive    bool   `json:"is_active"`
		ShowInGraph bool   `json:"show_in_graph"`
		Color       string `json:"color"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request"})
	}
	if body.Name == "" || body.SpeedMbps <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Name and speed are required"})
	}
	if body.Direction == "" {
		body.Direction = "both"
	}
	if body.Direction == "dscp" {
		if body.DSCPValue == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "DSCP value is required for DSCP direction"})
		}
	} else if body.Port == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Port is required for non-DSCP direction"})
	}
	if body.Color == "" {
		body.Color = "#8B5CF6"
	}

	rule := models.CDNPortRule{
		Name:        body.Name,
		Port:        body.Port,
		Direction:   body.Direction,
		DSCPValue:   body.DSCPValue,
		SpeedMbps:   body.SpeedMbps,
		NASID:       body.NASID,
		IsActive:    body.IsActive,
		ShowInGraph: body.ShowInGraph,
		Color:       body.Color,
	}
	if err := database.DB.Create(&rule).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to create port rule"})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": rule})
}

// UpdatePortRule updates an existing CDN port rule
func (h *CDNHandler) UpdatePortRule(c *fiber.Ctx) error {
	id := c.Params("id")
	var rule models.CDNPortRule
	if err := database.DB.Where("deleted_at IS NULL").First(&rule, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Port rule not found"})
	}

	var body struct {
		Name        string `json:"name"`
		Port        string `json:"port"`
		Direction   string `json:"direction"`
		DSCPValue   *int   `json:"dscp_value"`
		SpeedMbps   int64  `json:"speed_mbps"`
		NASID       *uint  `json:"nas_id"`
		IsActive    bool   `json:"is_active"`
		ShowInGraph bool   `json:"show_in_graph"`
		Color       string `json:"color"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request"})
	}
	if body.Color == "" {
		body.Color = "#8B5CF6"
	}

	rule.Name = body.Name
	rule.Port = body.Port
	rule.Direction = body.Direction
	rule.DSCPValue = body.DSCPValue
	rule.SpeedMbps = body.SpeedMbps
	rule.NASID = body.NASID
	rule.IsActive = body.IsActive
	rule.ShowInGraph = body.ShowInGraph
	rule.Color = body.Color

	if err := database.DB.Save(&rule).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to update port rule"})
	}
	return c.JSON(fiber.Map{"success": true, "data": rule})
}

// DeletePortRule soft-deletes a CDN port rule
func (h *CDNHandler) DeletePortRule(c *fiber.Ctx) error {
	id := c.Params("id")
	var rule models.CDNPortRule
	if err := database.DB.Where("deleted_at IS NULL").First(&rule, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Port rule not found"})
	}
	if err := database.DB.Delete(&rule).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to delete port rule"})
	}
	return c.JSON(fiber.Map{"success": true, "message": "Port rule deleted"})
}

// SyncPortRuleToNAS syncs a single port rule to MikroTik
func (h *CDNHandler) SyncPortRuleToNAS(c *fiber.Ctx) error {
	id := c.Params("id")
	var rule models.CDNPortRule
	if err := database.DB.Where("deleted_at IS NULL").First(&rule, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Port rule not found"})
	}
	if !rule.IsActive {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Port rule is not active"})
	}

	go syncPortRuleToNAS(rule)
	return c.JSON(fiber.Map{"success": true, "message": fmt.Sprintf("Syncing port rule '%s' to MikroTik", rule.Name)})
}

// SyncAllPortRulesToNAS syncs all active port rules to MikroTik
func (h *CDNHandler) SyncAllPortRulesToNAS(c *fiber.Ctx) error {
	var rules []models.CDNPortRule
	database.DB.Where("deleted_at IS NULL AND is_active = ?", true).Find(&rules)
	if len(rules) == 0 {
		return c.JSON(fiber.Map{"success": true, "message": "No active port rules found"})
	}
	for _, rule := range rules {
		r := rule
		go syncPortRuleToNAS(r)
	}
	return c.JSON(fiber.Map{"success": true, "message": fmt.Sprintf("Syncing %d port rules to MikroTik", len(rules))})
}

// syncPortRuleToNAS syncs a port rule to the specified NAS (or all NAS if not specified)
func syncPortRuleToNAS(rule models.CDNPortRule) {
	companyName := getCDNCompanyName()

	dscpValue := 0
	if rule.DSCPValue != nil {
		dscpValue = *rule.DSCPValue
	}
	config := mikrotik.PortRuleConfig{
		Name:        rule.Name,
		Port:        rule.Port,
		Direction:   rule.Direction,
		DSCPValue:   dscpValue,
		SpeedLimitM: rule.SpeedMbps,
		CompanyName: companyName,
	}

	// Get NAS list
	var nasList []models.Nas
	if rule.NASID != nil {
		var nas models.Nas
		if err := database.DB.Where("id = ? AND is_active = ?", *rule.NASID, true).First(&nas).Error; err != nil {
			log.Printf("Port Rule Sync: NAS %d not found", *rule.NASID)
			return
		}
		nasList = []models.Nas{nas}
	} else {
		database.DB.Where("is_active = ?", true).Find(&nasList)
	}

	for _, nas := range nasList {
		client := mikrotik.NewClient(
			fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
			nas.APIUsername,
			nas.APIPassword,
		)
		if err := client.SyncPortRule(config); err != nil {
			log.Printf("Port Rule Sync: Failed for NAS %s: %v", nas.Name, err)
		} else {
			log.Printf("Port Rule Sync: Success for NAS %s (rule=%s port=%s)", nas.Name, rule.Name, rule.Port)
		}
		client.Close()
	}
}
