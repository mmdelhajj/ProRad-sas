package handlers

import (
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
	"github.com/proisp/backend/internal/services"
	"golang.org/x/crypto/bcrypt"
)

// OnboardingHandler handles public tenant signup and connection verification
type OnboardingHandler struct {
	wgManager     *services.WireGuardManager
	workerManager *services.TenantWorkerManager
}

// NewOnboardingHandler creates a new onboarding handler
func NewOnboardingHandler(wgManager *services.WireGuardManager, workerManager *services.TenantWorkerManager) *OnboardingHandler {
	return &OnboardingHandler{
		wgManager:     wgManager,
		workerManager: workerManager,
	}
}

// Signup creates a new tenant account (public endpoint)
func (h *OnboardingHandler) Signup(c *fiber.Ctx) error {
	var req struct {
		Name          string `json:"name"`
		Email         string `json:"email"`
		Subdomain     string `json:"subdomain"`
		AdminUsername string `json:"admin_username"`
		Password      string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request"})
	}

	// Default admin username
	if req.AdminUsername == "" {
		req.AdminUsername = "admin"
	}

	// Validate
	if req.Name == "" || req.Email == "" || req.Subdomain == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "All fields are required"})
	}

	req.Subdomain = strings.ToLower(strings.TrimSpace(req.Subdomain))
	if !isValidSubdomain(req.Subdomain) {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid subdomain (3-50 chars, lowercase alphanumeric + hyphens)"})
	}

	// Check availability
	var count int64
	database.DB.Model(&models.Tenant{}).Where("subdomain = ?", req.Subdomain).Count(&count)
	if count > 0 {
		return c.Status(409).JSON(fiber.Map{"success": false, "message": "Subdomain already taken"})
	}

	// Check if email is already used by another tenant
	var emailCount int64
	database.DB.Model(&models.Tenant{}).Where("admin_email = ?", req.Email).Count(&emailCount)
	if emailCount > 0 {
		return c.Status(409).JSON(fiber.Map{"success": false, "message": "This email is already registered. Please use a different email."})
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Internal error"})
	}

	// Create tenant with trial status
	trialEnd := time.Now().Add(14 * 24 * time.Hour) // 14-day trial
	tenant := models.Tenant{
		Name:              req.Name,
		Subdomain:         req.Subdomain,
		SchemaName:        fmt.Sprintf("tenant_%s", req.Subdomain),
		Status:            "trial",
		Plan:              "free",
		MaxSubscribers:    50,
		MaxRouters:        1,
		AdminUsername:      req.AdminUsername,
		AdminPasswordHash: string(passwordHash),
		AdminEmail:        req.Email,
		TrialEndsAt:       &trialEnd,
	}

	if err := database.DB.Create(&tenant).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": fmt.Sprintf("Failed to create account: %v", err)})
	}

	// Setup WireGuard VPN
	var mikrotikScript string
	if h.wgManager != nil {
		if err := h.wgManager.SetupTenantVPN(&tenant); err != nil {
			log.Printf("SaaS: WireGuard setup failed for tenant %d: %v", tenant.ID, err)
		} else {
			database.DB.Save(&tenant)

			// Add NAS mapping
			nasMap := models.NasTenantMap{
				NasIP:    tenant.WGClientIP,
				TenantID: tenant.ID,
				WGSubnet: tenant.WGSubnet,
			}
			database.DB.Create(&nasMap)

			// Add peer and get script
			h.wgManager.AddPeer(&tenant)
			mikrotikScript = h.wgManager.GenerateMikroTikScript(&tenant)
		}
	}

	// Provision tenant database schema
	if err := database.ProvisionTenantSchema(tenant.SchemaName); err != nil {
		log.Printf("SaaS: Schema provision failed for tenant %d: %v", tenant.ID, err)
		// Don't fail signup - schema can be provisioned later
	} else {
		// Update the template admin user with tenant-specific credentials
		updateSQL := fmt.Sprintf("UPDATE %s.users SET username = ?, password = ?, password_plain = ?, email = ?, force_password_change = false, updated_at = NOW() WHERE user_type = 4 AND id = 1", tenant.SchemaName)
		result := database.DB.Exec(updateSQL, tenant.AdminUsername, string(passwordHash), req.Password, req.Email)
		if result.Error != nil {
			log.Printf("SaaS Signup: Failed to update admin user for %s: %v", tenant.SchemaName, result.Error)
		} else {
			log.Printf("SaaS Signup: Updated admin user for %s (rows: %d, email: %s)", tenant.SchemaName, result.RowsAffected, req.Email)
		}
	}

	return c.Status(201).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"tenant_id":       tenant.ID,
			"subdomain":       tenant.Subdomain,
			"panel_url":       fmt.Sprintf("https://%s.saas.proxrad.com", tenant.Subdomain),
			"trial_ends_at":   trialEnd,
			"mikrotik_script": mikrotikScript,
			"wg_client_ip":    tenant.WGClientIP,
			"wg_server_ip":    tenant.WGServerIP,
		},
		"message": "Account created! Paste the MikroTik script on your router to connect.",
	})
}

// CheckSubdomain checks if a subdomain is available
func (h *OnboardingHandler) CheckSubdomain(c *fiber.Ctx) error {
	subdomain := strings.ToLower(strings.TrimSpace(c.Params("name")))

	if !isValidSubdomain(subdomain) {
		return c.JSON(fiber.Map{
			"success":   true,
			"available": false,
			"message":   "Invalid subdomain format",
		})
	}

	var count int64
	database.DB.Model(&models.Tenant{}).Where("subdomain = ?", subdomain).Count(&count)

	return c.JSON(fiber.Map{
		"success":   true,
		"available": count == 0,
		"subdomain": subdomain,
		"url":       fmt.Sprintf("https://%s.saas.proxrad.com", subdomain),
	})
}

// VerifyConnection checks if a tenant's MikroTik VPN is connected
func (h *OnboardingHandler) VerifyConnection(c *fiber.Ctx) error {
	tenantID := c.Params("tenant_id")

	var tenant models.Tenant
	if err := database.DB.First(&tenant, tenantID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Tenant not found"})
	}

	result := fiber.Map{
		"vpn_connected":    false,
		"mikrotik_reachable": false,
		"radius_ready":      false,
	}

	// Check WireGuard peer status
	if h.wgManager != nil && tenant.WGClientPublicKey != "" {
		result["vpn_connected"] = h.wgManager.IsPeerConnected(tenant.WGClientPublicKey)
	}

	// Try to ping the MikroTik via VPN
	if tenant.WGClientIP != "" {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", tenant.WGClientIP, tenant.MikrotikAPIPort), 5*time.Second)
		if err == nil {
			conn.Close()
			result["mikrotik_reachable"] = true
			result["radius_ready"] = true // If MikroTik API is reachable, RADIUS should work
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

// Ensure log import is used
var _ = log.Printf
