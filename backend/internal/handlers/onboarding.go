package handlers

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/middleware"
	"github.com/proisp/backend/internal/models"
	"github.com/proisp/backend/internal/services"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
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

	// Validate required fields
	if req.Name == "" || req.Email == "" || req.Subdomain == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "All fields are required"})
	}

	// Validate email format
	if !strings.Contains(req.Email, "@") || !strings.Contains(req.Email, ".") {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid email address"})
	}

	// Validate password length
	if len(req.Password) < 6 {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Password must be at least 6 characters"})
	}

	req.Subdomain = strings.ToLower(strings.TrimSpace(req.Subdomain))
	if !isValidSubdomain(req.Subdomain) {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid subdomain (3-50 chars, lowercase alphanumeric + hyphens)"})
	}

	// Check subdomain availability
	var count int64
	database.DB.Model(&models.Tenant{}).Where("subdomain = ?", req.Subdomain).Count(&count)
	if count > 0 {
		return c.Status(409).JSON(fiber.Map{"success": false, "message": "Subdomain already taken"})
	}

	// Check email availability
	database.DB.Model(&models.Tenant{}).Where("admin_email = ?", req.Email).Count(&count)
	if count > 0 {
		return c.Status(409).JSON(fiber.Map{"success": false, "message": "An account with this email already exists"})
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
		MaxResellers:     2,
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

	// Set shared RADIUS secret on tenant
	saasSecret := os.Getenv("SAAS_RADIUS_SECRET")
	if saasSecret == "" {
		saasSecret = "ProxPanel-SaaS-2026"
	}
	tenant.RadiusSecret = saasSecret
	database.DB.Save(&tenant)

	// Provision tenant database schema
	if err := database.ProvisionTenantSchema(tenant.SchemaName); err != nil {
		log.Printf("SaaS: Schema provision failed for tenant %d: %v", tenant.ID, err)
		// Don't fail signup - schema can be provisioned later
	} else {
		// Seed admin user
		tenantDB := database.GetTenantDB(tenant.SchemaName)
		tenantDB.Exec(`
			INSERT INTO users (username, password, password_plain, email, user_type, is_active, created_at, updated_at)
			VALUES (?, ?, '', ?, 4, true, NOW(), NOW())
			ON CONFLICT (username) DO UPDATE SET password = EXCLUDED.password, email = EXCLUDED.email
		`, tenant.AdminUsername, string(passwordHash), req.Email)

		// Auto-create NAS in tenant schema with VPN IP + API credentials
		nasIP := tenant.WGClientIP
		if nasIP == "" {
			nasIP = "0.0.0.0"
		}
		nas := models.Nas{
			Name:        "MikroTik Router",
			ShortName:   "MK1",
			IPAddress:   nasIP,
			Secret:      saasSecret,
			AuthPort:    1812,
			AcctPort:    1813,
			CoAPort:     1700,
			APIPort:     8728,
			APIUsername:  tenant.MikrotikAPIUser,
			APIPassword: tenant.MikrotikAPIPassword,
			IsActive:    true,
		}
		tenantDB.Create(&nas)
		log.Printf("SaaS: Auto-created NAS in tenant %s — IP: %s, API user: %s", tenant.SchemaName, nasIP, tenant.MikrotikAPIUser)
	}

	// Generate MikroTik RADIUS commands for the customer
	serverIP := os.Getenv("SERVER_IP")
	if serverIP == "" {
		serverIP = "139.162.153.201"
	}
	mikrotikCommands := fmt.Sprintf(
		"/radius add address=%s secret=%s service=ppp\n"+
			"/radius incoming set accept=yes port=1700\n"+
			"/ppp aaa set use-radius=yes accounting=yes interim-update=00:00:30",
		serverIP, saasSecret,
	)

	// Send welcome email with credentials and MikroTik commands
	panelURL := fmt.Sprintf("https://%s.saas.proxrad.com", tenant.Subdomain)
	emailBody := fmt.Sprintf(`<html><body>
<h2>Welcome to ProxPanel!</h2>
<p>Your panel is ready:</p>
<ul>
  <li><b>URL:</b> <a href="%s">%s</a></li>
  <li><b>Username:</b> %s</li>
  <li><b>Password:</b> %s</li>
</ul>
<h3>MikroTik Configuration</h3>
<p>Paste these commands in your MikroTik terminal:</p>
<pre style="background:#f4f4f4;padding:10px;border-radius:4px;font-family:monospace">%s</pre>
<p>After pasting, your subscribers will authenticate through ProxPanel automatically.</p>
<p>Your dashboard will show "RADIUS Connected" once the first subscriber connects.</p>
</body></html>`,
		panelURL, panelURL,
		tenant.AdminUsername, req.Password,
		mikrotikCommands,
	)
	emailService := services.NewEmailService()
	go emailService.SendEmail(tenant.AdminEmail, "Your ProxPanel Demo is Ready", emailBody, true)

	return c.Status(201).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"tenant_id":         tenant.ID,
			"subdomain":         tenant.Subdomain,
			"panel_url":         panelURL,
			"trial_ends_at":     trialEnd,
			"mikrotik_script":   mikrotikScript,
			"mikrotik_commands": mikrotikCommands,
			"wg_client_ip":      tenant.WGClientIP,
			"wg_server_ip":      tenant.WGServerIP,
		},
		"message": "Account created! Check your email for MikroTik RADIUS commands.",
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

// checkWGPeerOnline checks if a WireGuard peer has a recent handshake (within 3 minutes)
func checkWGPeerOnline(clientPublicKey string) bool {
	cmd := exec.Command("wg", "show", "wg0", "latest-handshakes")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(output), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[0] == clientPublicKey {
			ts, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil || ts == 0 {
				return false
			}
			// WireGuard renegotiates handshake every ~2 minutes
			// Consider online if handshake was within last 2.5 minutes
			return time.Since(time.Unix(ts, 0)) < 150*time.Second
		}
	}
	return false
}

// RadiusStatus returns RADIUS connection status for SaaS tenants
func RadiusStatus(c *fiber.Ctx) error {
	// Determine tenant schema — use JWT claim or fallback to middleware
	schemaName, _ := c.Locals("tenant_schema").(string)

	var activeCount int64
	var nas models.Nas
	var nasFound bool
	var subscriberCount int64

	if schemaName != "" {
		// Use a transaction to pin the DB connection, ensuring SET search_path
		// persists for all queries (avoids connection pool reassignment)
		database.DB.Transaction(func(tx *gorm.DB) error {
			tx.Exec(fmt.Sprintf("SET search_path TO %s, public", schemaName))
			tx.Table("radacct").Where("acctstoptime IS NULL").Count(&activeCount)
			nasFound = tx.Where("is_active = true").Order("last_seen DESC NULLS LAST").First(&nas).Error == nil
			tx.Table("subscribers").Where("deleted_at IS NULL").Count(&subscriberCount)
			return nil
		})
	} else {
		db := middleware.GetTenantDBFromCtx(c)
		db.Table("radacct").Where("acctstoptime IS NULL").Count(&activeCount)
		nasFound = db.Where("is_active = true").Order("last_seen DESC NULLS LAST").First(&nas).Error == nil
		db.Table("subscribers").Where("deleted_at IS NULL").Count(&subscriberCount)
	}

	connected := activeCount > 0
	nasIP := ""
	if nasFound {
		nasIP = nas.IPAddress
	}

	// Check if NAS has been seen (API connected or RADIUS packets received)
	var nasEverSeen bool
	if nasFound && nas.LastSeen != nil {
		nasEverSeen = true
	}

	// Check if RADIUS has ever received any auth attempts for this tenant
	var authCount int64
	if schemaName != "" {
		database.DB.Transaction(func(tx *gorm.DB) error {
			tx.Exec(fmt.Sprintf("SET search_path TO %s, public", schemaName))
			tx.Table("radpostauth").Count(&authCount)
			return nil
		})
	}

	// Check WireGuard VPN connectivity and get tenant info
	routerOnline := false
	var tenantStatus, tenantPlan string
	var maxSubscribers, maxResellers int
	var trialEndsAt *time.Time
	tenantID, _ := c.Locals("tenant_id").(uint)
	if tenantID > 0 {
		var tenant models.Tenant
		if err := database.DB.First(&tenant, tenantID).Error; err == nil {
			if tenant.WGClientPublicKey != "" {
				routerOnline = checkWGPeerOnline(tenant.WGClientPublicKey)
			}
			tenantStatus = tenant.Status
			tenantPlan = tenant.Plan
			maxSubscribers = tenant.MaxSubscribers
			maxResellers = tenant.MaxResellers
			trialEndsAt = tenant.TrialEndsAt
		}
	}

	// Count resellers in tenant schema
	var resellerCount int64
	if schemaName != "" {
		database.DB.Transaction(func(tx *gorm.DB) error {
			tx.Exec(fmt.Sprintf("SET search_path TO %s, public", schemaName))
			tx.Table("resellers").Where("deleted_at IS NULL").Count(&resellerCount)
			return nil
		})
	}

	// Calculate trial days remaining
	var trialDaysLeft int
	if trialEndsAt != nil {
		remaining := time.Until(*trialEndsAt)
		if remaining > 0 {
			trialDaysLeft = int(remaining.Hours()/24) + 1
		}
	}

	// Determine status message for frontend
	status := "not_configured"
	if !nasFound {
		status = "not_configured"
	} else if connected {
		status = "connected" // Active RADIUS sessions
	} else if nasEverSeen || authCount > 0 {
		status = "configured" // NAS seen via API or RADIUS auth received, but no active sessions now
	} else if subscriberCount > 0 {
		status = "ready" // NAS + subscribers created, waiting for PPPoE connection
	} else {
		status = "waiting" // NAS created but no subscribers yet
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"connected":        connected,
		"active_sessions":  activeCount,
		"nas_ip":           nasIP,
		"nas_configured":   nasFound && nas.IPAddress != "0.0.0.0",
		"status":           status,
		"subscriber_count": subscriberCount,
		"router_online":    routerOnline,
		"tenant_status":    tenantStatus,
		"plan":             tenantPlan,
		"max_subscribers":  maxSubscribers,
		"max_resellers":   maxResellers,
		"reseller_count":  resellerCount,
		"trial_days_left":  trialDaysLeft,
		"trial_ends_at":    trialEndsAt,
	})
}

// GetMikroTikConfig returns the MikroTik configuration script for the current tenant
func (h *OnboardingHandler) GetMikroTikConfig(c *fiber.Ctx) error {
	tenantID, _ := c.Locals("tenant_id").(uint)
	if tenantID == 0 {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Tenant not found"})
	}

	var tenant models.Tenant
	if err := database.DB.First(&tenant, tenantID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Tenant not found"})
	}

	// Generate WireGuard + RADIUS script
	var mikrotikScript string
	if h.wgManager != nil && tenant.WGClientPrivateKey != "" {
		mikrotikScript = h.wgManager.GenerateMikroTikScript(&tenant)
	}

	// Generate RADIUS-only commands (simpler version for reconfiguration)
	serverIP := os.Getenv("SERVER_IP")
	if serverIP == "" {
		serverIP = "139.162.153.201"
	}
	saasSecret := os.Getenv("SAAS_RADIUS_SECRET")
	if saasSecret == "" {
		saasSecret = "ProxPanel-SaaS-2026"
	}
	radiusCommands := fmt.Sprintf(
		"/radius add address=%s secret=%s service=ppp\n"+
			"/radius incoming set accept=yes port=1700\n"+
			"/ppp aaa set use-radius=yes accounting=yes interim-update=00:00:30",
		serverIP, saasSecret,
	)

	return c.JSON(fiber.Map{
		"success":          true,
		"mikrotik_script":  mikrotikScript,
		"radius_commands":  radiusCommands,
		"panel_url":        fmt.Sprintf("https://%s.saas.proxrad.com", tenant.Subdomain),
		"wg_client_ip":     tenant.WGClientIP,
		"wg_server_ip":     tenant.WGServerIP,
	})
}

// Ensure log import is used
var _ = log.Printf
