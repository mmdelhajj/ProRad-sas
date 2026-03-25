package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

// CustomerNotificationHandler handles customer notification operations
type CustomerNotificationHandler struct{}

// NewCustomerNotificationHandler creates a new customer notification handler
func NewCustomerNotificationHandler() *CustomerNotificationHandler {
	return &CustomerNotificationHandler{}
}

// GetPendingNotifications returns unread in-app notifications for the customer
func (h *CustomerNotificationHandler) GetPendingNotifications(c *fiber.Ctx) error {
	licenseKey := os.Getenv("LICENSE_KEY")
	licenseServer := os.Getenv("LICENSE_SERVER")

	if licenseKey == "" || licenseServer == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "License configuration not found",
		})
	}

	// Call license server to get pending notifications
	url := fmt.Sprintf("%s/api/v1/license/notifications/pending", licenseServer)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("ERROR: Failed to create notification request: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch notifications",
		})
	}

	req.Header.Set("X-License-Key", licenseKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("ERROR: Failed to fetch notifications: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to contact license server",
		})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("ERROR: License server returned status %d", resp.StatusCode)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success":       true,
			"notifications": []interface{}{},
		})
	}

	var result struct {
		Success       bool                          `json:"success"`
		Notifications []models.PendingNotification  `json:"notifications"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("ERROR: Failed to decode notifications: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to parse notifications",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":       true,
		"notifications": result.Notifications,
	})
}

// MarkNotificationRead marks a notification as read
func (h *CustomerNotificationHandler) MarkNotificationRead(c *fiber.Ctx) error {
	notificationID := c.Params("id")
	licenseKey := os.Getenv("LICENSE_KEY")
	licenseServer := os.Getenv("LICENSE_SERVER")

	if licenseKey == "" || licenseServer == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "License configuration not found",
		})
	}

	// Call license server to mark notification as read
	url := fmt.Sprintf("%s/api/v1/license/notifications/%s/read", licenseServer, notificationID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		log.Printf("ERROR: Failed to create mark read request: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to mark notification as read",
		})
	}

	req.Header.Set("X-License-Key", licenseKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("ERROR: Failed to mark notification as read: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to contact license server",
		})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"success": false,
			"message": "Failed to mark notification as read",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Notification marked as read",
	})
}

// GetNotificationSettings returns notification preferences (for future use)
func (h *CustomerNotificationHandler) GetNotificationSettings(c *fiber.Ctx) error {
	// Get notification preferences from system_preferences table
	var preferences []struct {
		Key   string
		Value string
	}

	database.DB.Table("system_preferences").
		Where("key LIKE 'notification_%'").
		Select("key, value").
		Find(&preferences)

	settings := make(map[string]interface{})
	for _, pref := range preferences {
		settings[pref.Key] = pref.Value
	}

	// Set defaults if not found
	if _, ok := settings["notification_updates_enabled"]; !ok {
		settings["notification_updates_enabled"] = true
	}
	if _, ok := settings["notification_email_enabled"]; !ok {
		settings["notification_email_enabled"] = true
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":  true,
		"settings": settings,
	})
}

// UpdateNotificationSettings updates notification preferences
func (h *CustomerNotificationHandler) UpdateNotificationSettings(c *fiber.Ctx) error {
	var settings map[string]interface{}
	if err := c.BodyParser(&settings); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Update settings in database
	for key, value := range settings {
		var strValue string
		switch v := value.(type) {
		case bool:
			if v {
				strValue = "true"
			} else {
				strValue = "false"
			}
		case string:
			strValue = v
		default:
			strValue = fmt.Sprintf("%v", v)
		}

		database.DB.Exec("INSERT INTO system_preferences (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value",
			key, strValue)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Notification settings updated successfully",
	})
}
