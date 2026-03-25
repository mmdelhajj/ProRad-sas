package handlers

import (
	"crypto/rand"
	"fmt"
	"encoding/json"
	"net/http"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/pquerna/otp/totp"
	"github.com/proisp/backend/internal/config"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/middleware"
	"github.com/proisp/backend/internal/models"
	"github.com/proisp/backend/internal/services"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// LoginAttempt tracks failed login attempts
type LoginAttempt struct {
	Count     int
	LastTry   time.Time
	BlockedAt *time.Time
}

var (
	loginAttempts = make(map[string]*LoginAttempt)
	attemptsMutex sync.RWMutex
)

// getSecuritySetting retrieves a security setting from database
func getSecuritySetting(key string, defaultVal int) int {
	var pref models.SystemPreference
	if err := database.DB.Where("key = ?", key).First(&pref).Error; err != nil {
		return defaultVal
	}
	if val, err := strconv.Atoi(pref.Value); err == nil {
		return val
	}
	return defaultVal
}

// getSecuritySettingStr retrieves a string security setting
func getSecuritySettingStr(key string, defaultVal string) string {
	var pref models.SystemPreference
	if err := database.DB.Where("key = ?", key).First(&pref).Error; err != nil {
		return defaultVal
	}
	return pref.Value
}

// isIPBlocked checks if IP has too many failed attempts
func isIPBlocked(ip string) (bool, int) {
	attemptsMutex.RLock()
	attempt, exists := loginAttempts[ip]
	attemptsMutex.RUnlock()

	if !exists {
		return false, 0
	}

	maxAttempts := getSecuritySetting("max_login_attempts", 5)
	blockDuration := 15 * time.Minute // Block for 15 minutes

	// Check if blocked
	if attempt.BlockedAt != nil {
		if time.Since(*attempt.BlockedAt) < blockDuration {
			remaining := int(blockDuration.Minutes() - time.Since(*attempt.BlockedAt).Minutes())
			return true, remaining
		}
		// Block expired, reset
		attemptsMutex.Lock()
		delete(loginAttempts, ip)
		attemptsMutex.Unlock()
		return false, 0
	}

	// Check if attempts expired (reset after 15 minutes of no attempts)
	if time.Since(attempt.LastTry) > blockDuration {
		attemptsMutex.Lock()
		delete(loginAttempts, ip)
		attemptsMutex.Unlock()
		return false, 0
	}

	return attempt.Count >= maxAttempts, 0
}

// recordFailedAttempt records a failed login attempt
func recordFailedAttempt(ip string) int {
	attemptsMutex.Lock()
	defer attemptsMutex.Unlock()

	maxAttempts := getSecuritySetting("max_login_attempts", 5)

	if _, exists := loginAttempts[ip]; !exists {
		loginAttempts[ip] = &LoginAttempt{Count: 0}
	}

	loginAttempts[ip].Count++
	loginAttempts[ip].LastTry = time.Now()

	if loginAttempts[ip].Count >= maxAttempts {
		now := time.Now()
		loginAttempts[ip].BlockedAt = &now
	}

	return maxAttempts - loginAttempts[ip].Count
}

// clearFailedAttempts clears failed attempts on successful login
func clearFailedAttempts(ip string) {
	attemptsMutex.Lock()
	defer attemptsMutex.Unlock()
	delete(loginAttempts, ip)
}

// isAdminIPAllowed checks if IP is in allowed admin IPs list
func isAdminIPAllowed(ip string) bool {
	allowedIPs := getSecuritySettingStr("allowed_ips", "")
	if allowedIPs == "" {
		return true // No restriction
	}

	// Parse comma-separated IPs
	ips := strings.Split(allowedIPs, ",")
	for _, allowed := range ips {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		if allowed == ip {
			return true
		}
		// Support CIDR notation (basic)
		if strings.Contains(allowed, "/") && strings.HasPrefix(ip, strings.Split(allowed, "/")[0][:strings.LastIndex(strings.Split(allowed, "/")[0], ".")]) {
			return true
		}
	}
	return false
}

type AuthHandler struct {
	cfg *config.Config
}

func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{cfg: cfg}
}

// LoginRequest represents login request body
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
	TwoFACode string `json:"two_fa_code"`
}

// LoginResponse represents login response
type LoginResponse struct {
	Success             bool      `json:"success"`
	Message             string    `json:"message,omitempty"`
	Token               string    `json:"token,omitempty"`
	User                *UserInfo `json:"user,omitempty"`
	Requires2FA         bool      `json:"requires_2fa,omitempty"`
	ForcePasswordChange bool      `json:"force_password_change,omitempty"`
}

// UserInfo represents user info in response
type UserInfo struct {
	ID                  uint            `json:"id"`
	Username            string          `json:"username"`
	Email               string          `json:"email"`
	FullName            string          `json:"full_name"`
	UserType            models.UserType `json:"user_type"`
	ResellerID          *uint           `json:"reseller_id,omitempty"`
	Permissions         []string        `json:"permissions,omitempty"`
	ForcePasswordChange bool            `json:"force_password_change"`
}

// getResellerPermissions returns the list of permission names for a reseller
func getResellerPermissions(db *gorm.DB, resellerID uint) []string {
	var reseller models.Reseller
	if err := db.First(&reseller, resellerID).Error; err != nil {
		return nil
	}

	if reseller.PermissionGroup == nil {
		return nil
	}

	// Load permissions from junction table (Preload doesn't work with gorm:"-")
	var permissions []string
	db.Table("permissions").
		Joins("JOIN permission_group_permissions pgp ON pgp.permission_id = permissions.id").
		Where("pgp.permission_group_id = ?", *reseller.PermissionGroup).
		Pluck("name", &permissions)

	return permissions
}

// getUserPermissions returns the list of permission names for a user based on their permission_group
func getUserPermissions(db *gorm.DB, permissionGroupID *uint) []string {
	if permissionGroupID == nil {
		return nil
	}

	var permissions []string
	db.Table("permissions").
		Joins("JOIN permission_group_permissions pgp ON pgp.permission_id = permissions.id").
		Where("pgp.permission_group_id = ?", *permissionGroupID).
		Pluck("name", &permissions)

	return permissions
}

// Login handles user login
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	clientIP := c.IP()
	db := middleware.GetTenantDBFromCtx(c)

	// Check if IP is blocked due to too many failed attempts
	if blocked, remaining := isIPBlocked(clientIP); blocked {
		return c.Status(fiber.StatusTooManyRequests).JSON(LoginResponse{
			Success: false,
			Message: "Too many failed login attempts. Please try again in " + strconv.Itoa(remaining) + " minutes",
		})
	}

	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(LoginResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	if req.Username == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(LoginResponse{
			Success: false,
			Message: "Username and password are required",
		})
	}

	// Find user
	var user models.User
	if err := db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		remaining := recordFailedAttempt(clientIP)
		msg := "Invalid username or password"
		if remaining > 0 {
			msg += ". " + strconv.Itoa(remaining) + " attempts remaining"
		}
		return c.Status(fiber.StatusUnauthorized).JSON(LoginResponse{
			Success: false,
			Message: msg,
		})
	}

	// Check if user is active
	if !user.IsActive {
		return c.Status(fiber.StatusUnauthorized).JSON(LoginResponse{
			Success: false,
			Message: "Account is disabled",
		})
	}

	// Check if reseller account is active
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		var reseller models.Reseller
		if err := db.First(&reseller, *user.ResellerID).Error; err == nil {
			if !reseller.IsActive {
				return c.Status(fiber.StatusUnauthorized).JSON(LoginResponse{
					Success: false,
					Message: "Reseller account is deactivated",
				})
			}
		}
	}

	// Check if admin IP is allowed (only for admin users)
	if user.UserType == models.UserTypeAdmin && !isAdminIPAllowed(clientIP) {
		return c.Status(fiber.StatusForbidden).JSON(LoginResponse{
			Success: false,
			Message: "Access denied from this IP address",
		})
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		remaining := recordFailedAttempt(clientIP)

		// Feature 6: Failed login alert — check if 3+ failures from this IP
		go checkFailedLoginAlert(clientIP, req.Username)

		msg := "Invalid username or password"
		if remaining > 0 {
			msg += ". " + strconv.Itoa(remaining) + " attempts remaining"
		}
		return c.Status(fiber.StatusUnauthorized).JSON(LoginResponse{
			Success: false,
			Message: msg,
		})
	}

	// Check if 2FA is enabled for this user
	if user.TwoFactorEnabled {
		if req.TwoFACode == "" {
			// Password is correct, but need 2FA code
			return c.JSON(LoginResponse{
				Success:     false,
				Requires2FA: true,
				Message:     "2FA code required",
			})
		}
		// Verify 2FA code
		if !totp.Validate(req.TwoFACode, user.TwoFactorSecret) {
			remaining := recordFailedAttempt(clientIP)
			msg := "Invalid 2FA code"
			if remaining > 0 {
				msg += ". " + strconv.Itoa(remaining) + " attempts remaining"
			}
			return c.Status(fiber.StatusUnauthorized).JSON(LoginResponse{
				Success: false,
				Message: msg,
			})
		}
	}

	// Clear failed attempts on successful login
	clearFailedAttempts(clientIP)

	// Generate token
	token, err := middleware.GenerateToken(&user, h.cfg)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(LoginResponse{
			Success: false,
			Message: "Failed to generate token",
		})
	}

	// Update last login
	now := time.Now()
	db.Model(&user).Update("last_login", now)

	// Log the login
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionLogin,
		EntityType:  "user",
		EntityID:    user.ID,
		EntityName:  user.Username,
		Description: "User logged in",
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	db.Create(&auditLog)

	// Get permissions for reseller users
	var permissions []string
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		permissions = getResellerPermissions(middleware.GetTenantDBFromCtx(c), *user.ResellerID)
	}

	// Get permissions for non-reseller users (support, collector, readonly) based on their permission_group
	if user.UserType != models.UserTypeReseller && user.UserType != models.UserTypeAdmin {
		permissions = getUserPermissions(middleware.GetTenantDBFromCtx(c), user.PermissionGroup)
	}

	return c.JSON(LoginResponse{
		Success:             true,
		Token:               token,
		ForcePasswordChange: user.ForcePasswordChange,
		User: &UserInfo{
			ID:                  user.ID,
			Username:            user.Username,
			Email:               user.Email,
			FullName:            user.FullName,
			UserType:            user.UserType,
			ResellerID:          user.ResellerID,
			Permissions:         permissions,
			ForcePasswordChange: user.ForcePasswordChange,
		},
	})
}

// Logout handles user logout
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user != nil {
		// Log the logout
		auditLog := models.AuditLog{
			UserID:      user.ID,
			Username:    user.Username,
			UserType:    user.UserType,
			Action:      models.AuditActionLogout,
			EntityType:  "user",
			EntityID:    user.ID,
			EntityName:  user.Username,
			Description: "User logged out",
			IPAddress:   c.IP(),
			UserAgent:   c.Get("User-Agent"),
		}
		middleware.GetTenantDBFromCtx(c).Create(&auditLog)
	}

	// Add token to blacklist in Redis
	// This prevents the token from being used after logout
	authHeader := c.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			token := parts[1]
			// Blacklist token for the remaining JWT expiry duration (default 168 hours = 7 days)
			expiryDuration := time.Duration(h.cfg.JWTExpireHours) * time.Hour
			if err := database.BlacklistToken(token, expiryDuration); err != nil {
				// Log error but don't fail logout
				// Token will still expire naturally
			}
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Logged out successfully",
	})
}

// Me returns current user info
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not found",
		})
	}

	// Get reseller info if applicable
	var reseller *models.Reseller
	var permissions []string
	if user.ResellerID != nil {
		reseller = &models.Reseller{}
		middleware.GetTenantDBFromCtx(c).First(reseller, *user.ResellerID)
		// Get permissions for reseller
		if user.UserType == models.UserTypeReseller {
			permissions = getResellerPermissions(middleware.GetTenantDBFromCtx(c), *user.ResellerID)
		}
	}

	// Get permissions for non-reseller users (support, collector, readonly) based on their permission_group
	if user.UserType != models.UserTypeReseller && user.UserType != models.UserTypeAdmin {
		permissions = getUserPermissions(middleware.GetTenantDBFromCtx(c), user.PermissionGroup)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"user": fiber.Map{
			"id":               user.ID,
			"username":         user.Username,
			"email":            user.Email,
			"full_name":        user.FullName,
			"phone":            user.Phone,
			"user_type":        user.UserType,
			"reseller_id":      user.ResellerID,
			"reseller":         reseller,
			"is_active":        user.IsActive,
			"last_login":       user.LastLogin,
			"created_at":       user.CreatedAt,
			"permissions":      permissions,
			"permission_group": user.PermissionGroup,
		},
	})
}

// ChangePassword handles password change
func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not found",
		})
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Verify current password (skip if force_password_change is true - first login)
	if !user.ForcePasswordChange {
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Current password is incorrect",
			})
		}
	}

	// Validate new password
	if len(req.NewPassword) < 6 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Password must be at least 6 characters",
		})
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to hash password",
		})
	}

	// Update password and clear force_password_change flag
	if err := middleware.GetTenantDBFromCtx(c).Model(user).Updates(map[string]interface{}{
		"password":              string(hashedPassword),
		"force_password_change": false,
	}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update password",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Password changed successfully",
	})
}

// RefreshToken generates a new token
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not found",
		})
	}

	token, err := middleware.GenerateToken(user, h.cfg)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate token",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"token":   token,
	})
}

// HashPassword hashes a password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// Impersonation token storage (in-memory with expiry)
var (
	impersonateTokens      = make(map[string]uint) // token -> reseller_id
	impersonateTokenExpiry = make(map[string]time.Time)
	impersonateTokenMutex  sync.RWMutex
)

// generateRandomToken creates a random token for impersonation
func generateRandomToken() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	token := make([]byte, 64)
	randomBytes := make([]byte, 64)
	if _, err := rand.Read(randomBytes); err != nil {
		// Fallback: should never happen
		for i := range token {
			token[i] = chars[time.Now().UnixNano()%int64(len(chars))]
			time.Sleep(1 * time.Nanosecond)
		}
		return string(token)
	}
	for i := range token {
		token[i] = chars[randomBytes[i]%byte(len(chars))]
	}
	return string(token)
}

// GetImpersonateToken generates a temporary token for impersonating a reseller
// This token can be used in a new browser tab to login as the reseller
func (h *AuthHandler) GetImpersonateToken(c *fiber.Ctx) error {
	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized",
		})
	}

	// Only admins can impersonate
	if currentUser.UserType != models.UserTypeAdmin {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"message": "Only admins can impersonate resellers",
		})
	}

	resellerID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid reseller ID",
		})
	}

	// Verify reseller exists
	var reseller models.Reseller
	if err := database.DB.First(&reseller, resellerID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Reseller not found",
		})
	}

	// Generate temporary token
	token := generateRandomToken()
	expiry := time.Now().Add(30 * time.Second)

	impersonateTokenMutex.Lock()
	impersonateTokens[token] = uint(resellerID)
	impersonateTokenExpiry[token] = expiry
	impersonateTokenMutex.Unlock()

	// Clean up expired tokens
	go func() {
		time.Sleep(35 * time.Second)
		impersonateTokenMutex.Lock()
		delete(impersonateTokens, token)
		delete(impersonateTokenExpiry, token)
		impersonateTokenMutex.Unlock()
	}()

	return c.JSON(fiber.Map{
		"success": true,
		"token":   token,
	})
}

// ExchangeImpersonateToken exchanges a temporary impersonate token for a real session
func (h *AuthHandler) ExchangeImpersonateToken(c *fiber.Ctx) error {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request",
		})
	}

	if req.Token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Token is required",
		})
	}

	// Look up and validate token
	impersonateTokenMutex.RLock()
	resellerID, exists := impersonateTokens[req.Token]
	expiry, _ := impersonateTokenExpiry[req.Token]
	impersonateTokenMutex.RUnlock()

	if !exists {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Invalid or expired token",
		})
	}

	if time.Now().After(expiry) {
		// Clean up expired token
		impersonateTokenMutex.Lock()
		delete(impersonateTokens, req.Token)
		delete(impersonateTokenExpiry, req.Token)
		impersonateTokenMutex.Unlock()

		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Token has expired",
		})
	}

	// Token is valid - delete it (one-time use)
	impersonateTokenMutex.Lock()
	delete(impersonateTokens, req.Token)
	delete(impersonateTokenExpiry, req.Token)
	impersonateTokenMutex.Unlock()

	// Get reseller and user
	var reseller models.Reseller
	if err := database.DB.Preload("User").First(&reseller, resellerID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Reseller not found",
		})
	}

	// Generate JWT token for the reseller
	jwtToken, err := middleware.GenerateToken(reseller.User, h.cfg)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate token",
		})
	}

	// Get permissions for reseller
	var permissions []string
	if reseller.PermissionGroup != nil {
		database.DB.Table("permissions").
			Joins("JOIN permission_group_permissions pgp ON pgp.permission_id = permissions.id").
			Where("pgp.permission_group_id = ?", *reseller.PermissionGroup).
			Pluck("name", &permissions)
	}

	// Build user response
	userResponse := fiber.Map{
		"id":                    reseller.User.ID,
		"username":              reseller.User.Username,
		"email":                 reseller.User.Email,
		"phone":                 reseller.User.Phone,
		"full_name":             reseller.User.FullName,
		"user_type":             "reseller",
		"is_active":             reseller.User.IsActive,
		"reseller_id":           reseller.ID,
		"permissions":           permissions,
		"force_password_change": reseller.User.ForcePasswordChange,
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"token": jwtToken,
			"user":  userResponse,
		},
	})
}

// checkFailedLoginAlert sends an alert if 3+ failed login attempts from same IP
func checkFailedLoginAlert(ip, username string) {
	// Check if alerts are enabled
	alertEnabled := ""
	database.DB.Raw("SELECT value FROM system_preferences WHERE key = 'failed_login_alert_enabled'").Scan(&alertEnabled)
	if alertEnabled != "true" {
		return
	}

	// Count failed attempts from this IP using the in-memory tracker
	attemptsMutex.RLock()
	attempts, exists := loginAttempts[ip]
	attemptsMutex.RUnlock()

	if !exists || attempts.Count < 3 {
		return
	}

	// Only alert once per IP per 10 minutes (dedup via communication_logs)
	alertKey := "failed_login_alert:" + ip
	var existing int64
	database.DB.Raw("SELECT COUNT(*) FROM communication_logs WHERE type = 'whatsapp' AND recipient = ? AND created_at > NOW() - INTERVAL '10 minutes'", alertKey).Scan(&existing)
	if existing > 0 {
		return
	}

	msg := "Security Alert: " + strconv.Itoa(attempts.Count) + " failed login attempts for user '" + username + "' from IP " + ip

	// Log the alert
	database.DB.Exec("INSERT INTO communication_logs (type, recipient, message, status, created_at) VALUES ('whatsapp', ?, ?, 'sent', NOW())", alertKey, msg)
}

// ==================== Forgot Password ====================

// resetToken stores password reset tokens in memory
type resetToken struct {
	Email     string
	Schema    string
	Token     string
	ExpiresAt time.Time
}

var (
	resetTokens   = make(map[string]*resetToken) // token -> resetToken
	resetTokensMu sync.Mutex
)

// generateResetToken creates a cryptographic random token
func generateResetToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	const chars = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	result := make([]byte, 32)
	for i := range result {
		result[i] = chars[int(b[i])%len(chars)]
	}
	return string(result)
}

// ForgotPassword sends a password reset email
func (h *AuthHandler) ForgotPassword(c *fiber.Ctx) error {
	var req struct {
		Email string `json:"email"`
	}
	if err := c.BodyParser(&req); err != nil || req.Email == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Email is required"})
	}

	tenantSchema, _ := c.Locals("tenant_schema").(string)

	// Find user by email using schema-qualified query to avoid connection pool race
	var user models.User
	var findErr error
	if tenantSchema != "" {
		findErr = database.DB.Raw(
			fmt.Sprintf("SELECT * FROM %s.users WHERE email = $1 AND is_active = true AND deleted_at IS NULL LIMIT 1", tenantSchema),
			req.Email,
		).Scan(&user).Error
		if user.ID == 0 {
			findErr = gorm.ErrRecordNotFound
		}
	} else {
		findErr = database.DB.Where("email = ? AND is_active = true", req.Email).First(&user).Error
	}
	if findErr != nil {
		// Don't reveal if email exists
		return c.JSON(fiber.Map{"success": true, "message": "If an account with that email exists, a reset link has been sent."})
	}

	// Generate token
	token := generateResetToken()

	resetTokensMu.Lock()
	// Clean expired tokens
	now := time.Now()
	for k, v := range resetTokens {
		if now.After(v.ExpiresAt) {
			delete(resetTokens, k)
		}
	}
	resetTokens[token] = &resetToken{
		Email:     req.Email,
		Schema:    tenantSchema,
		Token:     token,
		ExpiresAt: now.Add(30 * time.Minute),
	}
	resetTokensMu.Unlock()

	// Build reset URL
	host := c.Hostname()
	resetURL := "https://" + host + "/reset-password?token=" + token

	// Send email
	// Use tenant DB for email (reading SMTP config); search_path race is OK here since fallback exists
	emailDB := database.GetTenantDB(tenantSchema)
	go sendPasswordResetEmail(emailDB, user.FullName, req.Email, resetURL)

	return c.JSON(fiber.Map{"success": true, "message": "If an account with that email exists, a reset link has been sent."})
}

// ResetPassword resets the password using a token
func (h *AuthHandler) ResetPassword(c *fiber.Ctx) error {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := c.BodyParser(&req); err != nil || req.Token == "" || req.NewPassword == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Token and new password are required"})
	}

	if len(req.NewPassword) < 6 {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Password must be at least 6 characters"})
	}

	resetTokensMu.Lock()
	rt, exists := resetTokens[req.Token]
	if exists {
		delete(resetTokens, req.Token)
	}
	resetTokensMu.Unlock()

	if !exists || time.Now().After(rt.ExpiresAt) {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid or expired reset token"})
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to process password"})
	}

	// Update password using raw SQL with schema-qualified table name
	// This avoids the SET search_path connection pool race condition
	var result *gorm.DB
	if rt.Schema != "" {
		result = database.DB.Exec(
			fmt.Sprintf("UPDATE %s.users SET password = $1, password_plain = $2, force_password_change = false, updated_at = NOW() WHERE email = $3 AND deleted_at IS NULL", rt.Schema),
			string(hashedPassword), req.NewPassword, rt.Email,
		)
	} else {
		result = database.DB.Exec(
			"UPDATE users SET password = $1, password_plain = $2, force_password_change = false, updated_at = NOW() WHERE email = $3 AND deleted_at IS NULL",
			string(hashedPassword), req.NewPassword, rt.Email,
		)
	}
	if result.Error != nil {
		log.Printf("ResetPassword: DB error: %v", result.Error)
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to update password"})
	}
	if result.RowsAffected == 0 {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "User not found"})
	}
	log.Printf("ResetPassword: Password updated for %s in schema %s (rows: %d)", rt.Email, rt.Schema, result.RowsAffected)

	return c.JSON(fiber.Map{"success": true, "message": "Password has been reset successfully. You can now log in."})
}

// sendPasswordResetEmail sends the reset email via SMTP
func sendPasswordResetEmail(db *gorm.DB, name, email, resetURL string) {
	emailSvc := services.NewEmailServiceWithDB(db)

	if name == "" {
		name = "User"
	}

	subject := "Password Reset Request"
	body := "<html><body style=\"font-family:Arial,sans-serif;max-width:600px;margin:0 auto;padding:20px;background:#f5f5f5;\">" +
		"<div style=\"background:#1a1a2e;color:white;padding:30px;text-align:center;border-radius:8px 8px 0 0;\">" +
		"<h1 style=\"margin:0;font-size:24px;\">Password Reset</h1>" +
		"</div>" +
		"<div style=\"background:white;padding:30px;border-radius:0 0 8px 8px;border:1px solid #e0e0e0;\">" +
		"<p>Hello <strong>" + name + "</strong>,</p>" +
		"<p>We received a request to reset your password. Click the button below to set a new password:</p>" +
		"<div style=\"text-align:center;margin:24px 0;\">" +
		"<a href=\"" + resetURL + "\" style=\"display:inline-block;background:#2563eb;color:white;padding:14px 32px;border-radius:8px;font-weight:bold;text-decoration:none;font-size:16px;\">Reset Password</a>" +
		"</div>" +
		"<p style=\"color:#666;font-size:13px;\">This link will expire in 30 minutes.</p>" +
		"<p style=\"color:#666;font-size:13px;\">If you did not request a password reset, you can safely ignore this email.</p>" +
		"</div>" +
		"</body></html>"

	if err := emailSvc.SendEmail(email, subject, body, true); err != nil {
		// Fallback: try direct SMTP to ProxRad mail server
		sendDirectResetEmail(name, email, resetURL)
	}
}

// sendDirectResetEmail sends reset email via license server relay
func sendDirectResetEmail(name, email, resetURL string) {
	subject := "Password Reset Request"
	htmlBody := "<!DOCTYPE html><html><body style=\"font-family:Arial,sans-serif;max-width:600px;margin:0 auto;padding:20px;background:#f5f5f5;\">" +
		"<div style=\"background:#1a1a2e;color:white;padding:30px;text-align:center;border-radius:8px 8px 0 0;\">" +
		"<h1 style=\"margin:0;font-size:24px;\">Password Reset</h1>" +
		"</div>" +
		"<div style=\"background:white;padding:30px;border-radius:0 0 8px 8px;border:1px solid #e0e0e0;\">" +
		"<p>Hello <strong>" + name + "</strong>,</p>" +
		"<p>Click the button below to reset your password:</p>" +
		"<div style=\"text-align:center;margin:24px 0;\">" +
		"<a href=\"" + resetURL + "\" style=\"display:inline-block;background:#2563eb;color:white;padding:14px 32px;border-radius:8px;font-weight:bold;text-decoration:none;font-size:16px;\">Reset Password</a>" +
		"</div>" +
		"<p style=\"color:#666;font-size:13px;\">This link expires in 30 minutes. If you did not request this, ignore this email.</p>" +
		"</div></body></html>"

	// Use license server email relay API
	payload := map[string]string{
		"to":      email,
		"subject": subject,
		"body":    htmlBody,
	}
	payloadJSON, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("POST", "https://license.proxrad.com/api/v1/license/saas-email-relay", strings.NewReader(string(payloadJSON)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-SaaS-Secret", "proxrad-saas-2026")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("ERROR: Reset email relay failed: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("INFO: Password reset email relayed for %s (status: %d)", email, resp.StatusCode)
}
