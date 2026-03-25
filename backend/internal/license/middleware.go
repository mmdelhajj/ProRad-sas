package license

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

// RequireLicense middleware checks if the license is valid
func RequireLicense() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !IsValid() {
			return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
				"success":        false,
				"message":        "Invalid or expired license. Please contact support.",
				"code":           "LICENSE_INVALID",
				"license_status": "blocked",
			})
		}
		return c.Next()
	}
}

// RequireWriteAccess middleware blocks write operations based on license status
// - active/warning: Full access
// - grace: Block new creations (POST to collection endpoints), allow modifications
// - readonly/blocked: Block all write operations
func RequireWriteAccess() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Allow GET and HEAD requests always
		method := c.Method()
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			return c.Next()
		}

		status := GetLicenseStatus()

		switch status {
		case "active", "warning":
			// Full access - allow all operations
			return c.Next()

		case "grace":
			// Grace period - block new creations but allow modifications
			if method == "POST" && isCreationEndpoint(c.Path()) {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"success":        false,
					"message":        "License expired. Creating new records is disabled during grace period. Please renew your license.",
					"code":           "LICENSE_GRACE_PERIOD",
					"license_status": status,
					"days_remaining": GetDaysUntilExpiry(),
					"action_blocked": "create",
				})
			}
			return c.Next()

		case "readonly", "blocked":
			// Blocked - only allow read operations
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success":        false,
				"message":        "License expired. System is in read-only mode. Please renew your license.",
				"code":           "LICENSE_READONLY",
				"license_status": status,
				"days_remaining": GetDaysUntilExpiry(),
				"action_blocked": "write",
			})

		default:
			// Unknown status - check IsReadOnly as fallback
			if IsReadOnly() {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"success":        false,
					"message":        "License expired. System is in read-only mode. Please renew your license.",
					"code":           "LICENSE_READONLY",
					"license_status": status,
					"days_remaining": GetDaysUntilExpiry(),
				})
			}
			return c.Next()
		}
	}
}

// isCreationEndpoint checks if the path is for creating new records
func isCreationEndpoint(path string) bool {
	creationPaths := []string{
		"/api/subscribers",
		"/api/services",
		"/api/nas",
		"/api/resellers",
		"/api/users",
		"/api/cdns",
		"/api/bandwidth-rules",
		"/api/prepaid-cards",
		"/api/prepaid-cards/generate",
	}

	for _, p := range creationPaths {
		// Match exact path (POST to collection creates new item)
		if path == p {
			return true
		}
	}
	return false
}

// CheckSubscriberLimit middleware checks if subscriber limit is exceeded
func CheckSubscriberLimit(getCurrentCount func() int) fiber.Handler {
	return func(c *fiber.Ctx) error {
		maxSubs := GetMaxSubscribers()
		if maxSubs == 0 {
			return c.Next() // No limit set
		}

		currentCount := getCurrentCount()
		if currentCount >= maxSubs {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success":     false,
				"message":     "Subscriber limit exceeded. Please upgrade your license.",
				"code":        "LIMIT_EXCEEDED",
				"max_allowed": maxSubs,
				"current":     currentCount,
			})
		}

		return c.Next()
	}
}

// LicenseStatusMiddleware adds license status headers to all responses
func LicenseStatusMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()

		// Add license status headers
		status := GetLicenseStatus()
		c.Set("X-License-Status", status)

		if status != "active" {
			warning := GetWarningMessage()
			if warning != "" {
				c.Set("X-License-Warning", warning)
			}
			c.Set("X-License-Days", fmt.Sprintf("%d", GetDaysUntilExpiry()))
		}

		if IsReadOnly() {
			c.Set("X-License-ReadOnly", "true")
		}

		return err
	}
}

// GracePeriodWarning adds grace period warning to responses (deprecated, use LicenseStatusMiddleware)
func GracePeriodWarning() fiber.Handler {
	return LicenseStatusMiddleware()
}
