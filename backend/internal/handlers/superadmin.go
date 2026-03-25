package handlers

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
	"github.com/proisp/backend/internal/services"
	"golang.org/x/crypto/bcrypt"
)

// SuperAdminHandler handles super-admin operations for the SaaS platform
type SuperAdminHandler struct {
	wgManager     *services.WireGuardManager
	workerManager *services.TenantWorkerManager
}

// NewSuperAdminHandler creates a new super-admin handler
func NewSuperAdminHandler(wgManager *services.WireGuardManager, workerManager *services.TenantWorkerManager) *SuperAdminHandler {
	return &SuperAdminHandler{
		wgManager:     wgManager,
		workerManager: workerManager,
	}
}

// SuperAdminLogin authenticates a super-admin
func (h *SuperAdminHandler) SuperAdminLogin(c *fiber.Ctx) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request"})
	}

	var admin models.SuperAdmin
	if err := database.DB.Where("username = ?", req.Username).First(&admin).Error; err != nil {
		return c.Status(401).JSON(fiber.Map{"success": false, "message": "Invalid credentials"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
		return c.Status(401).JSON(fiber.Map{"success": false, "message": "Invalid credentials"})
	}

	// Generate JWT
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "saas-super-admin-secret-key"
	}

	claims := jwt.MapClaims{
		"user_id":        admin.ID,
		"username":       admin.Username,
		"is_super_admin": true,
		"exp":            time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to generate token"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"token":   tokenString,
		"user": fiber.Map{
			"id":       admin.ID,
			"username": admin.Username,
			"email":    admin.Email,
		},
	})
}

// TenantLogin authenticates a user against a specific tenant schema
func (h *SuperAdminHandler) TenantLogin(c *fiber.Ctx) error {
	var req struct {
		Username  string `json:"username"`
		Password  string `json:"password"`
		Subdomain string `json:"subdomain"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request"})
	}

	if req.Username == "" || req.Password == "" || req.Subdomain == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Username, password, and subdomain are required"})
	}

	// Resolve tenant from subdomain
	var tenant models.Tenant
	if err := database.DB.Where("subdomain = ? AND status != 'deleted'", req.Subdomain).First(&tenant).Error; err != nil {
		return c.Status(401).JSON(fiber.Map{"success": false, "message": "Invalid tenant"})
	}

	if tenant.Status == "suspended" {
		return c.Status(403).JSON(fiber.Map{"success": false, "message": "This account has been suspended"})
	}

	// Get tenant-scoped DB
	tenantDB := database.GetTenantDB(tenant.SchemaName)

	// Find user by username or email using schema-qualified query
	var user models.User
	if err := database.DB.Raw(
		fmt.Sprintf("SELECT * FROM %s.users WHERE (username = $1 OR email = $1) AND deleted_at IS NULL LIMIT 1", tenant.SchemaName),
		req.Username,
	).Scan(&user).Error; err != nil || user.ID == 0 {
		return c.Status(401).JSON(fiber.Map{"success": false, "message": "Invalid username or password"})
	}
	if !user.IsActive {
		return c.Status(401).JSON(fiber.Map{"success": false, "message": "Account is disabled"})
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return c.Status(401).JSON(fiber.Map{"success": false, "message": "Invalid username or password"})
	}

	// Generate JWT with tenant claims
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "saas-super-admin-secret-key"
	}

	claims := jwt.MapClaims{
		"user_id":       user.ID,
		"username":      user.Username,
		"user_type":     user.UserType,
		"tenant_id":     tenant.ID,
		"tenant_schema": tenant.SchemaName,
		"exp":           time.Now().Add(168 * time.Hour).Unix(),
		"iat":           time.Now().Unix(),
		"iss":           "proisp",
	}
	if user.ResellerID != nil {
		claims["reseller_id"] = *user.ResellerID
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to generate token"})
	}

	// Load user permissions for resellers
	var permissions []string
	if user.UserType == models.UserTypeAdmin {
		permissions = []string{"*"}
	} else if user.ResellerID != nil {
		var reseller models.Reseller
		if err := tenantDB.Preload("User").First(&reseller, *user.ResellerID).Error; err == nil {
			if reseller.PermissionGroup != nil {
				tenantDB.Table("permissions").
					Joins("JOIN permission_group_permissions pgp ON pgp.permission_id = permissions.id").
					Where("pgp.permission_group_id = ?", *reseller.PermissionGroup).
					Pluck("name", &permissions)
			}
		}
	}

	return c.JSON(fiber.Map{
		"success":               true,
		"token":                 tokenString,
		"force_password_change": user.ForcePasswordChange,
		"user": fiber.Map{
			"id":                    user.ID,
			"username":              user.Username,
			"user_type":             user.UserType,
			"is_active":             user.IsActive,
			"permissions":           permissions,
			"force_password_change": user.ForcePasswordChange,
		},
		"tenant": fiber.Map{
			"id":        tenant.ID,
			"name":      tenant.Name,
			"subdomain": tenant.Subdomain,
			"plan":      tenant.Plan,
		},
	})
}

// TenantChangePassword handles password change for tenant users
func (h *SuperAdminHandler) TenantChangePassword(c *fiber.Ctx) error {
	tenantSchema, _ := c.Locals("tenant_schema").(string)
	if tenantSchema == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "No tenant context"})
	}

	userID, _ := c.Locals("userID").(uint)
	tenantDB := database.GetTenantDB(tenantSchema)

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request"})
	}

	if req.NewPassword == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "New password is required"})
	}

	var user models.User
	if err := tenantDB.First(&user, userID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "User not found"})
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		return c.Status(401).JSON(fiber.Map{"success": false, "message": "Current password is incorrect"})
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to hash password"})
	}

	// Update password and clear force_password_change
	if err := tenantDB.Model(&user).Updates(map[string]interface{}{
		"password":              string(hashedPassword),
		"force_password_change": false,
	}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to update password"})
	}

	return c.JSON(fiber.Map{"success": true, "message": "Password changed successfully"})
}

// TenantAuthMe returns the current tenant user's info (like /auth/me but tenant-aware)
func (h *SuperAdminHandler) TenantAuthMe(c *fiber.Ctx) error {
	tenantSchema, _ := c.Locals("tenant_schema").(string)
	if tenantSchema == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "No tenant context"})
	}

	userID, _ := c.Locals("userID").(uint)
	tenantDB := database.GetTenantDB(tenantSchema)

	var user models.User
	if err := tenantDB.First(&user, userID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "User not found"})
	}

	result := fiber.Map{
		"id":        user.ID,
		"username":  user.Username,
		"user_type": user.UserType,
		"is_active": user.IsActive,
	}

	// Load permissions for resellers
	if user.ResellerID != nil {
		var reseller models.Reseller
		if err := tenantDB.Preload("User").First(&reseller, *user.ResellerID).Error; err == nil {
			result["reseller"] = reseller
			if reseller.PermissionGroup != nil {
				var permissions []string
				tenantDB.Table("permissions").
					Joins("JOIN permission_group_permissions pgp ON pgp.permission_id = permissions.id").
					Where("pgp.permission_group_id = ?", *reseller.PermissionGroup).
					Pluck("name", &permissions)
				result["permissions"] = permissions
			}
		}
	} else if user.UserType == models.UserTypeAdmin {
		result["permissions"] = []string{"*"}
	}

	return c.JSON(fiber.Map{"success": true, "user": result})
}

// CreateTenant creates a new tenant (provisions schema, WireGuard, etc.)
func (h *SuperAdminHandler) CreateTenant(c *fiber.Ctx) error {
	var req struct {
		Name          string `json:"name"`
		Subdomain     string `json:"subdomain"`
		AdminUsername string `json:"admin_username"`
		AdminPassword string `json:"admin_password"`
		AdminEmail    string `json:"admin_email"`
		Plan          string `json:"plan"`
		MaxSubscribers int   `json:"max_subscribers"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request"})
	}

	// Validate required fields
	if req.Name == "" || req.Subdomain == "" || req.AdminUsername == "" || req.AdminPassword == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Name, subdomain, admin username, and admin password are required"})
	}

	// Validate subdomain format
	req.Subdomain = strings.ToLower(strings.TrimSpace(req.Subdomain))
	if !isValidSubdomain(req.Subdomain) {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid subdomain format (lowercase alphanumeric + hyphens, 3-50 chars)"})
	}

	// Check subdomain availability
	var count int64
	database.DB.Model(&models.Tenant{}).Where("subdomain = ?", req.Subdomain).Count(&count)
	if count > 0 {
		return c.Status(409).JSON(fiber.Map{"success": false, "message": "Subdomain already taken"})
	}

	// Check if email is already used by another tenant
	if req.AdminEmail != "" {
		var emailCount int64
		database.DB.Model(&models.Tenant{}).Where("admin_email = ?", req.AdminEmail).Count(&emailCount)
		log.Printf("SaaS: Email check for %s: count=%d", req.AdminEmail, emailCount)
		if emailCount > 0 {
			return c.Status(409).JSON(fiber.Map{"success": false, "message": "This email is already registered. Please use a different email."})
		}
	}

	// Hash admin password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to hash password"})
	}

	// Create tenant record first to get ID
	tenant := models.Tenant{
		Name:              req.Name,
		Subdomain:         req.Subdomain,
		SchemaName:        fmt.Sprintf("tenant_%s", req.Subdomain),
		Status:            "active",
		Plan:              req.Plan,
		MaxSubscribers:    req.MaxSubscribers,
		MaxRouters:        1,
		AdminUsername:      req.AdminUsername,
		AdminPasswordHash: string(passwordHash),
		AdminEmail:        req.AdminEmail,
	}

	if tenant.Plan == "" {
		tenant.Plan = "free"
	}
	if tenant.MaxSubscribers == 0 {
		tenant.MaxSubscribers = 50
	}

	if err := database.DB.Create(&tenant).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": fmt.Sprintf("Failed to create tenant: %v", err)})
	}

	// Setup WireGuard VPN
	if h.wgManager != nil {
		if err := h.wgManager.SetupTenantVPN(&tenant); err != nil {
			log.Printf("SaaS: WireGuard setup failed for tenant %d: %v", tenant.ID, err)
		} else {
			// Update tenant with WireGuard info
			database.DB.Save(&tenant)

			// Add NAS→tenant mapping
			nasMap := models.NasTenantMap{
				NasIP:    tenant.WGClientIP,
				TenantID: tenant.ID,
				WGSubnet: tenant.WGSubnet,
			}
			database.DB.Create(&nasMap)

			// Add peer to WireGuard interface
			h.wgManager.AddPeer(&tenant)
		}
	}

	// Provision tenant schema (clone from template)
	if err := database.ProvisionTenantSchema(tenant.SchemaName); err != nil {
		log.Printf("SaaS: Schema provisioning failed for tenant %d: %v", tenant.ID, err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to provision schema: %v", err),
		})
	}

	// Update the template admin user with tenant-specific credentials
	updateSQL := fmt.Sprintf("UPDATE %s.users SET username = ?, password = ?, password_plain = ?, email = ?, force_password_change = false, updated_at = NOW() WHERE user_type = 4 AND id = 1", tenant.SchemaName)
	updateResult := database.DB.Exec(updateSQL, tenant.AdminUsername, string(passwordHash), req.AdminPassword, req.AdminEmail)
	if updateResult.Error != nil {
		log.Printf("SaaS: Failed to update admin user for tenant %s: %v", tenant.SchemaName, updateResult.Error)
	} else {
		log.Printf("SaaS: Updated admin user for tenant %s (rows: %d, email: %s)", tenant.SchemaName, updateResult.RowsAffected, req.AdminEmail)
	}

	// Generate MikroTik script
	var mikrotikScript string
	if h.wgManager != nil {
		mikrotikScript = h.wgManager.GenerateMikroTikScript(&tenant)
	}

	return c.Status(201).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"tenant":          tenant,
			"mikrotik_script": mikrotikScript,
			"panel_url":       fmt.Sprintf("https://%s.saas.proxrad.com", tenant.Subdomain),
		},
	})
}

// ListTenants returns all tenants with stats
func (h *SuperAdminHandler) ListTenants(c *fiber.Ctx) error {
	var tenants []models.Tenant
	if err := database.DB.Order("id ASC").Find(&tenants).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to load tenants"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    tenants,
		"total":   len(tenants),
	})
}

// GetTenant returns tenant details
func (h *SuperAdminHandler) GetTenant(c *fiber.Ctx) error {
	id := c.Params("id")

	var tenant models.Tenant
	if err := database.DB.First(&tenant, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Tenant not found"})
	}

	// Get live subscriber count from tenant schema
	tenantDB := database.GetTenantDB(tenant.SchemaName)
	var subCount int64
	tenantDB.Model(&models.Subscriber{}).Where("deleted_at IS NULL").Count(&subCount)
	tenant.CurrentSubscriberCount = int(subCount)

	// Check VPN status
	vpnConnected := false
	if h.wgManager != nil && tenant.WGClientPublicKey != "" {
		vpnConnected = h.wgManager.IsPeerConnected(tenant.WGClientPublicKey)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"tenant":        tenant,
			"vpn_connected": vpnConnected,
		},
	})
}

// UpdateTenant updates tenant details
func (h *SuperAdminHandler) UpdateTenant(c *fiber.Ctx) error {
	id := c.Params("id")

	var tenant models.Tenant
	if err := database.DB.First(&tenant, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Tenant not found"})
	}

	var req struct {
		Name           string `json:"name"`
		Status         string `json:"status"`
		Plan           string `json:"plan"`
		MaxSubscribers int    `json:"max_subscribers"`
		MaxRouters     int    `json:"max_routers"`
		CustomDomain   string `json:"custom_domain"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request"})
	}

	if req.Name != "" {
		tenant.Name = req.Name
	}
	if req.Status != "" {
		tenant.Status = req.Status
	}
	if req.Plan != "" {
		tenant.Plan = req.Plan
	}
	if req.MaxSubscribers > 0 {
		tenant.MaxSubscribers = req.MaxSubscribers
	}
	if req.MaxRouters > 0 {
		tenant.MaxRouters = req.MaxRouters
	}
	if req.CustomDomain != "" {
		tenant.CustomDomain = req.CustomDomain
	}

	if err := database.DB.Save(&tenant).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to update tenant"})
	}

	return c.JSON(fiber.Map{"success": true, "data": tenant})
}

// DeleteTenant suspends or deletes a tenant
func (h *SuperAdminHandler) DeleteTenant(c *fiber.Ctx) error {
	id := c.Params("id")

	var tenant models.Tenant
	if err := database.DB.First(&tenant, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Tenant not found"})
	}

	// Soft delete - just mark as suspended
	tenant.Status = "suspended"
	database.DB.Save(&tenant)

	// Remove WireGuard peer
	if h.wgManager != nil && tenant.WGClientPublicKey != "" {
		h.wgManager.RemovePeer(tenant.WGClientPublicKey)
	}

	return c.JSON(fiber.Map{"success": true, "message": "Tenant suspended"})
}

// GetTenantScript returns the MikroTik connection script for a tenant
func (h *SuperAdminHandler) GetTenantScript(c *fiber.Ctx) error {
	id := c.Params("id")

	var tenant models.Tenant
	if err := database.DB.First(&tenant, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Tenant not found"})
	}

	if h.wgManager == nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "WireGuard not configured"})
	}

	script := h.wgManager.GenerateMikroTikScript(&tenant)
	serverPubKey, _ := h.wgManager.GetServerPublicKey()

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"script":         script,
			"server_ip":      serverPubKey,
			"wg_client_ip":   tenant.WGClientIP,
			"wg_server_ip":   tenant.WGServerIP,
			"radius_secret":  tenant.RadiusSecret,
		},
	})
}

// GetGlobalStats returns global SaaS platform statistics
func (h *SuperAdminHandler) GetGlobalStats(c *fiber.Ctx) error {
	var totalTenants int64
	var activeTenants int64
	var trialTenants int64

	database.DB.Model(&models.Tenant{}).Count(&totalTenants)
	database.DB.Model(&models.Tenant{}).Where("status = 'active'").Count(&activeTenants)
	database.DB.Model(&models.Tenant{}).Where("status = 'trial'").Count(&trialTenants)

	// Count total subscribers across all tenants
	var totalSubscribers int64
	database.DB.Model(&models.Tenant{}).Select("COALESCE(SUM(current_subscriber_count), 0)").Scan(&totalSubscribers)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"total_tenants":    totalTenants,
			"active_tenants":   activeTenants,
			"trial_tenants":    trialTenants,
			"total_subscribers": totalSubscribers,
			"active_workers":   h.workerManager.GetWorkerCount(),
		},
	})
}

// isValidSubdomain checks if a subdomain is valid
func isValidSubdomain(s string) bool {
	if len(s) < 3 || len(s) > 50 {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	if s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	return true
}
