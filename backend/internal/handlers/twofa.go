package handlers

import (
	"bytes"
	"encoding/base64"
	"image/png"

	"github.com/gofiber/fiber/v2"
	"github.com/pquerna/otp/totp"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/middleware"
	"github.com/proisp/backend/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type TwoFAHandler struct{}

func NewTwoFAHandler() *TwoFAHandler {
	return &TwoFAHandler{}
}

// Setup generates a new 2FA secret and returns QR code
func (h *TwoFAHandler) Setup(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not found",
		})
	}

	// Generate new TOTP key with company name as issuer
	issuer := database.GetCompanyName()
	if issuer == "" {
		issuer = "ISP Management"
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: user.Username,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate 2FA secret",
		})
	}

	// Generate QR code image
	img, err := key.Image(200, 200)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate QR code",
		})
	}

	// Convert to base64
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to encode QR code",
		})
	}
	qrBase64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	// Store secret temporarily (not enabled yet until verified)
	database.DB.Model(&models.User{}).Where("id = ?", user.ID).Update("two_factor_secret", key.Secret())

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"secret":   key.Secret(),
			"qr_code":  "data:image/png;base64," + qrBase64,
			"otpauth":  key.URL(),
		},
	})
}

// Verify verifies the 2FA code and enables 2FA
func (h *TwoFAHandler) Verify(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not found",
		})
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if req.Code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Code is required",
		})
	}

	// Get fresh user data with secret
	var freshUser models.User
	if err := database.DB.First(&freshUser, user.ID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get user data",
		})
	}

	if freshUser.TwoFactorSecret == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "2FA not set up. Please call setup first",
		})
	}

	// Verify the code
	valid := totp.Validate(req.Code, freshUser.TwoFactorSecret)
	if !valid {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid code. Please try again",
		})
	}

	// Enable 2FA
	database.DB.Model(&models.User{}).Where("id = ?", user.ID).Update("two_factor_enabled", true)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "2FA enabled successfully",
	})
}

// Disable disables 2FA for the user
func (h *TwoFAHandler) Disable(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not found",
		})
	}

	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Get fresh user data
	var freshUser models.User
	if err := database.DB.First(&freshUser, user.ID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get user data",
		})
	}

	if !freshUser.TwoFactorEnabled {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "2FA is not enabled",
		})
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(freshUser.Password), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid password",
		})
	}

	// Verify 2FA code
	valid := totp.Validate(req.Code, freshUser.TwoFactorSecret)
	if !valid {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid 2FA code",
		})
	}

	// Disable 2FA
	database.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"two_factor_enabled": false,
		"two_factor_secret":  "",
	})

	return c.JSON(fiber.Map{
		"success": true,
		"message": "2FA disabled successfully",
	})
}

// Status returns 2FA status for current user
func (h *TwoFAHandler) Status(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User not found",
		})
	}

	// Get fresh user data
	var freshUser models.User
	if err := database.DB.First(&freshUser, user.ID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get user data",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"enabled": freshUser.TwoFactorEnabled,
		},
	})
}
