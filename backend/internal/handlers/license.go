package handlers

import (
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/license"
)

// LicenseHandler handles license-related requests
type LicenseHandler struct{}

// NewLicenseHandler creates a new license handler
func NewLicenseHandler() *LicenseHandler {
	return &LicenseHandler{}
}

// GetLicenseInfo returns current license information
func (h *LicenseHandler) GetLicenseInfo(c *fiber.Ctx) error {
	info := license.GetLicenseInfo()

	if info == nil {
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"valid":           false,
				"license_key":     os.Getenv("LICENSE_KEY"),
				"message":         "License not initialized",
			},
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"valid":           info.Valid,
			"license_key":     os.Getenv("LICENSE_KEY"),
			"customer_name":   info.CustomerName,
			"tier":            info.Tier,
			"max_subscribers": info.MaxSubscribers,
			"features":        info.Features,
			"expires_at":      info.ExpiresAt,
			"is_lifetime":     info.IsLifetime,
			"grace_period":    info.GracePeriod,
			"days_remaining":  info.DaysRemaining,
			"message":         info.Message,
		},
	})
}

// GetLicenseStatus returns WHMCS-style license status for frontend
func (h *LicenseHandler) GetLicenseStatus(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"success":           true,
		"valid":             license.IsValid(),
		"grace_period":      license.InGracePeriod(),
		"license_status":    license.GetLicenseStatus(),
		"read_only":         license.IsReadOnly(),
		"days_until_expiry": license.GetDaysUntilExpiry(),
		"warning_message":   license.GetWarningMessage(),
	})
}

// RevalidateLicense forces a re-validation with the license server
func (h *LicenseHandler) RevalidateLicense(c *fiber.Ctx) error {
	err := license.Revalidate()
	if err != nil {
		return c.JSON(fiber.Map{
			"success":         false,
			"message":         err.Error(),
			"valid":           license.IsValid(),
			"license_status":  license.GetLicenseStatus(),
			"read_only":       license.IsReadOnly(),
			"warning_message": license.GetWarningMessage(),
		})
	}

	return c.JSON(fiber.Map{
		"success":           true,
		"message":           "License validated successfully",
		"valid":             license.IsValid(),
		"license_status":    license.GetLicenseStatus(),
		"read_only":         license.IsReadOnly(),
		"days_until_expiry": license.GetDaysUntilExpiry(),
		"warning_message":   license.GetWarningMessage(),
	})
}
