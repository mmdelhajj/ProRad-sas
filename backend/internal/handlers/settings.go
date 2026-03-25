package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/middleware"
	"github.com/proisp/backend/internal/models"
)

// Common timezone list for dropdown - organized by region
var CommonTimezones = []map[string]string{
	// UTC
	{"value": "UTC", "label": "UTC (Coordinated Universal Time)"},

	// Middle East
	{"value": "Asia/Beirut", "label": "Asia/Beirut - Lebanon (EET/EEST)"},
	{"value": "Asia/Baghdad", "label": "Asia/Baghdad - Iraq (AST)"},
	{"value": "Asia/Damascus", "label": "Asia/Damascus - Syria (EET/EEST)"},
	{"value": "Asia/Amman", "label": "Asia/Amman - Jordan (EET/EEST)"},
	{"value": "Asia/Jerusalem", "label": "Asia/Jerusalem - Israel (IST/IDT)"},
	{"value": "Asia/Riyadh", "label": "Asia/Riyadh - Saudi Arabia (AST)"},
	{"value": "Asia/Kuwait", "label": "Asia/Kuwait - Kuwait (AST)"},
	{"value": "Asia/Qatar", "label": "Asia/Qatar - Qatar (AST)"},
	{"value": "Asia/Dubai", "label": "Asia/Dubai - UAE (GST)"},
	{"value": "Asia/Muscat", "label": "Asia/Muscat - Oman (GST)"},
	{"value": "Asia/Bahrain", "label": "Asia/Bahrain - Bahrain (AST)"},
	{"value": "Asia/Tehran", "label": "Asia/Tehran - Iran (IRST/IRDT)"},

	// Europe
	{"value": "Europe/London", "label": "Europe/London - UK (GMT/BST)"},
	{"value": "Europe/Paris", "label": "Europe/Paris - France (CET/CEST)"},
	{"value": "Europe/Berlin", "label": "Europe/Berlin - Germany (CET/CEST)"},
	{"value": "Europe/Rome", "label": "Europe/Rome - Italy (CET/CEST)"},
	{"value": "Europe/Madrid", "label": "Europe/Madrid - Spain (CET/CEST)"},
	{"value": "Europe/Amsterdam", "label": "Europe/Amsterdam - Netherlands (CET/CEST)"},
	{"value": "Europe/Brussels", "label": "Europe/Brussels - Belgium (CET/CEST)"},
	{"value": "Europe/Vienna", "label": "Europe/Vienna - Austria (CET/CEST)"},
	{"value": "Europe/Warsaw", "label": "Europe/Warsaw - Poland (CET/CEST)"},
	{"value": "Europe/Prague", "label": "Europe/Prague - Czech Republic (CET/CEST)"},
	{"value": "Europe/Budapest", "label": "Europe/Budapest - Hungary (CET/CEST)"},
	{"value": "Europe/Athens", "label": "Europe/Athens - Greece (EET/EEST)"},
	{"value": "Europe/Bucharest", "label": "Europe/Bucharest - Romania (EET/EEST)"},
	{"value": "Europe/Sofia", "label": "Europe/Sofia - Bulgaria (EET/EEST)"},
	{"value": "Europe/Kiev", "label": "Europe/Kiev - Ukraine (EET/EEST)"},
	{"value": "Europe/Moscow", "label": "Europe/Moscow - Russia (MSK)"},
	{"value": "Europe/Istanbul", "label": "Europe/Istanbul - Turkey (TRT)"},
	{"value": "Europe/Helsinki", "label": "Europe/Helsinki - Finland (EET/EEST)"},
	{"value": "Europe/Stockholm", "label": "Europe/Stockholm - Sweden (CET/CEST)"},
	{"value": "Europe/Oslo", "label": "Europe/Oslo - Norway (CET/CEST)"},
	{"value": "Europe/Copenhagen", "label": "Europe/Copenhagen - Denmark (CET/CEST)"},
	{"value": "Europe/Lisbon", "label": "Europe/Lisbon - Portugal (WET/WEST)"},
	{"value": "Europe/Dublin", "label": "Europe/Dublin - Ireland (GMT/IST)"},
	{"value": "Europe/Zurich", "label": "Europe/Zurich - Switzerland (CET/CEST)"},

	// Asia
	{"value": "Asia/Karachi", "label": "Asia/Karachi - Pakistan (PKT)"},
	{"value": "Asia/Kolkata", "label": "Asia/Kolkata - India (IST)"},
	{"value": "Asia/Dhaka", "label": "Asia/Dhaka - Bangladesh (BST)"},
	{"value": "Asia/Bangkok", "label": "Asia/Bangkok - Thailand (ICT)"},
	{"value": "Asia/Ho_Chi_Minh", "label": "Asia/Ho_Chi_Minh - Vietnam (ICT)"},
	{"value": "Asia/Jakarta", "label": "Asia/Jakarta - Indonesia (WIB)"},
	{"value": "Asia/Singapore", "label": "Asia/Singapore - Singapore (SGT)"},
	{"value": "Asia/Kuala_Lumpur", "label": "Asia/Kuala_Lumpur - Malaysia (MYT)"},
	{"value": "Asia/Manila", "label": "Asia/Manila - Philippines (PHT)"},
	{"value": "Asia/Hong_Kong", "label": "Asia/Hong_Kong - Hong Kong (HKT)"},
	{"value": "Asia/Shanghai", "label": "Asia/Shanghai - China (CST)"},
	{"value": "Asia/Taipei", "label": "Asia/Taipei - Taiwan (CST)"},
	{"value": "Asia/Seoul", "label": "Asia/Seoul - South Korea (KST)"},
	{"value": "Asia/Tokyo", "label": "Asia/Tokyo - Japan (JST)"},

	// Africa
	{"value": "Africa/Cairo", "label": "Africa/Cairo - Egypt (EET)"},
	{"value": "Africa/Johannesburg", "label": "Africa/Johannesburg - South Africa (SAST)"},
	{"value": "Africa/Lagos", "label": "Africa/Lagos - Nigeria (WAT)"},
	{"value": "Africa/Nairobi", "label": "Africa/Nairobi - Kenya (EAT)"},
	{"value": "Africa/Casablanca", "label": "Africa/Casablanca - Morocco (WET/WEST)"},
	{"value": "Africa/Tunis", "label": "Africa/Tunis - Tunisia (CET)"},
	{"value": "Africa/Algiers", "label": "Africa/Algiers - Algeria (CET)"},
	{"value": "Africa/Tripoli", "label": "Africa/Tripoli - Libya (EET)"},
	{"value": "Africa/Khartoum", "label": "Africa/Khartoum - Sudan (CAT)"},
	{"value": "Africa/Addis_Ababa", "label": "Africa/Addis_Ababa - Ethiopia (EAT)"},

	// Americas
	{"value": "America/New_York", "label": "America/New_York - US Eastern (EST/EDT)"},
	{"value": "America/Chicago", "label": "America/Chicago - US Central (CST/CDT)"},
	{"value": "America/Denver", "label": "America/Denver - US Mountain (MST/MDT)"},
	{"value": "America/Los_Angeles", "label": "America/Los_Angeles - US Pacific (PST/PDT)"},
	{"value": "America/Toronto", "label": "America/Toronto - Canada Eastern (EST/EDT)"},
	{"value": "America/Vancouver", "label": "America/Vancouver - Canada Pacific (PST/PDT)"},
	{"value": "America/Mexico_City", "label": "America/Mexico_City - Mexico (CST/CDT)"},
	{"value": "America/Bogota", "label": "America/Bogota - Colombia (COT)"},
	{"value": "America/Lima", "label": "America/Lima - Peru (PET)"},
	{"value": "America/Santiago", "label": "America/Santiago - Chile (CLT/CLST)"},
	{"value": "America/Buenos_Aires", "label": "America/Buenos_Aires - Argentina (ART)"},
	{"value": "America/Sao_Paulo", "label": "America/Sao_Paulo - Brazil (BRT)"},
	{"value": "America/Caracas", "label": "America/Caracas - Venezuela (VET)"},

	// Australia & Pacific
	{"value": "Australia/Sydney", "label": "Australia/Sydney - Australia Eastern (AEST/AEDT)"},
	{"value": "Australia/Melbourne", "label": "Australia/Melbourne - Australia (AEST/AEDT)"},
	{"value": "Australia/Brisbane", "label": "Australia/Brisbane - Australia (AEST)"},
	{"value": "Australia/Perth", "label": "Australia/Perth - Australia Western (AWST)"},
	{"value": "Pacific/Auckland", "label": "Pacific/Auckland - New Zealand (NZST/NZDT)"},
	{"value": "Pacific/Fiji", "label": "Pacific/Fiji - Fiji (FJT)"},
	{"value": "Pacific/Honolulu", "label": "Pacific/Honolulu - Hawaii (HST)"},
}

type SettingsHandler struct{}

func NewSettingsHandler() *SettingsHandler {
	return &SettingsHandler{}
}

// List returns all system preferences (with Redis caching)
func (h *SettingsHandler) List(c *fiber.Ctx) error {
	// Try cache first
	type cachedSettings struct {
		Settings map[string]interface{}  `json:"settings"`
		Items    []models.SystemPreference `json:"items"`
	}
	var cached cachedSettings
	if err := database.CacheGet(database.CacheKeySettings, &cached); err == nil {
		return c.JSON(fiber.Map{
			"success": true,
			"data":    cached.Settings,
			"items":   cached.Items,
		})
	}

	// Fetch from database
	var preferences []models.SystemPreference
	database.DB.Order("key").Find(&preferences)

	// Convert to map for easier frontend use
	settings := make(map[string]interface{})
	for _, p := range preferences {
		settings[p.Key] = p.Value
	}

	// Cache the result
	database.CacheSet(database.CacheKeySettings, cachedSettings{Settings: settings, Items: preferences}, database.CacheTTLSettings)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    settings,
		"items":   preferences,
	})
}

// Get returns a single preference
func (h *SettingsHandler) Get(c *fiber.Ctx) error {
	key := c.Params("key")

	var pref models.SystemPreference
	if err := database.DB.Where("key = ?", key).First(&pref).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Setting not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    pref,
	})
}

// Update updates or creates a preference
func (h *SettingsHandler) Update(c *fiber.Ctx) error {
	type UpdateRequest struct {
		Key       string `json:"key"`
		Value     string `json:"value"`
		ValueType string `json:"value_type"`
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if req.ValueType == "" {
		req.ValueType = "string"
	}

	var pref models.SystemPreference
	result := database.DB.Where("key = ?", req.Key).First(&pref)

	if result.Error != nil {
		// Create new
		pref = models.SystemPreference{
			Key:       req.Key,
			Value:     req.Value,
			ValueType: req.ValueType,
		}
		database.DB.Create(&pref)
	} else {
		// Update existing
		database.DB.Model(&pref).Updates(map[string]interface{}{
			"value":      req.Value,
			"value_type": req.ValueType,
		})
	}

	// Invalidate settings cache
	database.InvalidateSettingsCache()
	database.InvalidateSettingsCacheNow()

	return c.JSON(fiber.Map{
		"success": true,
		"data":    pref,
	})
}

// BulkUpdate updates multiple preferences
func (h *SettingsHandler) BulkUpdate(c *fiber.Ctx) error {
	type SettingItem struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	type BulkRequest struct {
		Settings []SettingItem `json:"settings"`
	}

	var req BulkRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	remoteSupportChanged := false
	remoteSupportEnabled := false

	for _, item := range req.Settings {
		if item.Key == "" {
			continue
		}

		// Track remote support changes
		if item.Key == "remote_support_enabled" {
			remoteSupportChanged = true
			remoteSupportEnabled = item.Value == "true" || item.Value == "1"
		}

		var pref models.SystemPreference
		result := database.DB.Where("key = ?", item.Key).First(&pref)

		if result.Error != nil {
			pref = models.SystemPreference{Key: item.Key, Value: item.Value, ValueType: "string"}
			database.DB.Create(&pref)
		} else {
			database.DB.Model(&pref).Update("value", item.Value)
		}
	}

	// Invalidate settings cache
	database.InvalidateSettingsCache()
	database.InvalidateSettingsCacheNow()

	// Handle remote support toggle - write to control file and notify license server
	if remoteSupportChanged {
		controlFile := "/opt/proxpanel/remote-support-enabled"
		if remoteSupportEnabled {
			os.WriteFile(controlFile, []byte("true"), 0644)
			// Send SSH credentials to license server
			go sendSSHCredentialsToLicenseServer()
		} else {
			os.Remove(controlFile)
			// Clear SSH credentials from license server
			go clearSSHCredentialsFromLicenseServer()
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Settings updated",
	})
}

// Delete removes a preference
func (h *SettingsHandler) Delete(c *fiber.Ctx) error {
	key := c.Params("key")

	result := database.DB.Where("key = ?", key).Delete(&models.SystemPreference{})
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Setting not found",
		})
	}

	// Invalidate settings cache
	database.InvalidateSettingsCache()
	database.InvalidateSettingsCacheNow()

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Setting deleted",
	})
}

// GetTimezones returns list of available timezones
func (h *SettingsHandler) GetTimezones(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"success": true,
		"data":    CommonTimezones,
	})
}

// GetServerTime returns current server time in configured timezone
func (h *SettingsHandler) GetServerTime(c *fiber.Ctx) error {
	tz := GetConfiguredTimezone()
	now := time.Now().In(tz)

	return c.JSON(fiber.Map{
		"success":  true,
		"time":     now.Format("15:04:05"),
		"date":     now.Format("2006-01-02"),
		"datetime": now.Format("2006-01-02 15:04:05"),
		"timezone": tz.String(),
		"unix":     now.Unix(),
	})
}

// GetConfiguredTimezone returns the configured timezone from system preferences
// Falls back to UTC if not configured or invalid
func GetConfiguredTimezone() *time.Location {
	var pref models.SystemPreference
	if err := database.DB.Where("key = ?", "system_timezone").First(&pref).Error; err != nil {
		return time.UTC
	}

	loc, err := time.LoadLocation(pref.Value)
	if err != nil {
		return time.UTC
	}

	return loc
}

// GetConfiguredTimezoneString returns the timezone string
func GetConfiguredTimezoneString() string {
	var pref models.SystemPreference
	if err := database.DB.Where("key = ?", "system_timezone").First(&pref).Error; err != nil {
		return "UTC"
	}
	return pref.Value
}

// GetBranding returns public branding info (no auth required)
func (h *SettingsHandler) GetBranding(c *fiber.Ctx) error {
	// Check if request comes from a custom reseller domain (works even before login)
	host := c.Hostname() // Gets Host header without port
	if host != "" && host != "localhost" && host != "127.0.0.1" {
		var reseller models.Reseller
		if err := database.DB.Where("custom_domain = ? AND rebrand_enabled = true AND deleted_at IS NULL", host).First(&reseller).Error; err == nil {
			var rb models.ResellerBranding
			database.DB.Where("reseller_id = ?", reseller.ID).First(&rb)
			companyName := reseller.Name
			if rb.CompanyName != "" {
				companyName = rb.CompanyName
			}
			primaryColor := "#2563eb"
			if rb.PrimaryColor != "" {
				primaryColor = rb.PrimaryColor
			}
			logoURL := ""
			if rb.LogoPath != "" {
				logoURL = "/uploads/" + filepath.Base(rb.LogoPath)
			}
			return c.JSON(fiber.Map{
				"success": true,
				"data": fiber.Map{
					"company_name":        companyName,
					"company_logo":        logoURL,
					"primary_color":       primaryColor,
					"login_background":    "",
					"favicon":             "",
					"footer_text":         rb.FooterText,
					"login_tagline":       rb.Tagline,
					"show_login_features": "false",
					"reseller_domain":     true,
				},
			})
		}
	}

	// Check if authenticated reseller with rebranding enabled
	user := middleware.GetCurrentUser(c)
	if user != nil && user.ResellerID != nil {
		var reseller models.Reseller
		if err := database.DB.First(&reseller, *user.ResellerID).Error; err == nil && reseller.RebrandEnabled {
			var rb models.ResellerBranding
			database.DB.Where("reseller_id = ?", reseller.ID).First(&rb)
			companyName := reseller.Name
			if rb.CompanyName != "" {
				companyName = rb.CompanyName
			}
			primaryColor := "#2563eb"
			if rb.PrimaryColor != "" {
				primaryColor = rb.PrimaryColor
			}
			logoURL := ""
			if rb.LogoPath != "" {
				logoURL = "/uploads/" + filepath.Base(rb.LogoPath)
			}
			return c.JSON(fiber.Map{
				"success": true,
				"data": fiber.Map{
					"company_name":        companyName,
					"company_logo":        logoURL,
					"primary_color":       primaryColor,
					"login_background":    "",
					"favicon":             "",
					"footer_text":         rb.FooterText,
					"login_tagline":       rb.Tagline,
					"show_login_features": "false",
				},
			})
		}
	}

	branding := map[string]string{
		"company_name":          "", // Empty by default - customer sets their own
		"company_logo":          "",
		"primary_color":         "#2563eb",
		"login_background":      "",
		"favicon":               "",
		"footer_text":           "",
		"login_tagline":         "High Performance ISP Management Solution",
		"show_login_features":   "true",
		"login_feature_1_title": "PPPoE Management",
		"login_feature_1_desc":  "Complete subscriber and session management with real-time monitoring",
		"login_feature_2_title": "Bandwidth Control",
		"login_feature_2_desc":  "FUP quotas, time-based speed control, and usage monitoring",
		"login_feature_3_title": "MikroTik Integration",
		"login_feature_3_desc":  "Seamless RADIUS and API integration with MikroTik routers",
	}

	var preferences []models.SystemPreference
	database.DB.Where("key IN ?", []string{
		"company_name", "company_logo", "primary_color",
		"login_background", "favicon", "footer_text",
		"login_tagline", "show_login_features",
		"login_feature_1_title", "login_feature_1_desc",
		"login_feature_2_title", "login_feature_2_desc",
		"login_feature_3_title", "login_feature_3_desc",
	}).Find(&preferences)

	for _, p := range preferences {
		// Always use database value (even if empty - customer may have cleared it)
		branding[p.Key] = p.Value
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    branding,
	})
}

// UploadLogo handles logo file upload
func (h *SettingsHandler) UploadLogo(c *fiber.Ctx) error {
	file, err := c.FormFile("logo")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "No file uploaded",
		})
	}

	// Validate file type
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".svg": true, ".webp": true}
	if !allowedExts[ext] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid file type. Allowed: PNG, JPG, JPEG, SVG, WEBP",
		})
	}

	// Validate file size (max 2MB)
	if file.Size > 2*1024*1024 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "File too large. Maximum size is 2MB",
		})
	}

	// Create uploads directory if not exists
	uploadDir := "/app/uploads"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create upload directory",
		})
	}

	// Delete old logo if exists
	var oldPref models.SystemPreference
	if err := database.DB.Where("key = ?", "company_logo").First(&oldPref).Error; err == nil {
		if oldPref.Value != "" {
			oldPath := filepath.Join(uploadDir, filepath.Base(oldPref.Value))
			os.Remove(oldPath)
		}
	}

	// Generate unique filename
	filename := fmt.Sprintf("logo_%s%s", uuid.New().String()[:8], ext)
	savePath := filepath.Join(uploadDir, filename)

	// Save file
	if err := c.SaveFile(file, savePath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to save file",
		})
	}

	// Update setting
	logoURL := "/uploads/" + filename
	var pref models.SystemPreference
	result := database.DB.Where("key = ?", "company_logo").First(&pref)
	if result.Error != nil {
		pref = models.SystemPreference{Key: "company_logo", Value: logoURL, ValueType: "string"}
		database.DB.Create(&pref)
	} else {
		database.DB.Model(&pref).Update("value", logoURL)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"url": logoURL,
		},
		"message": "Logo uploaded successfully",
	})
}

// DeleteLogo removes the company logo
func (h *SettingsHandler) DeleteLogo(c *fiber.Ctx) error {
	var pref models.SystemPreference
	if err := database.DB.Where("key = ?", "company_logo").First(&pref).Error; err != nil {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "No logo to delete",
		})
	}

	// Delete file
	if pref.Value != "" {
		uploadDir := "/app/uploads"
		filePath := filepath.Join(uploadDir, filepath.Base(pref.Value))
		os.Remove(filePath)
	}

	// Clear setting
	database.DB.Model(&pref).Update("value", "")

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Logo deleted",
	})
}

// UploadLoginBackground handles login background image upload
func (h *SettingsHandler) UploadLoginBackground(c *fiber.Ctx) error {
	file, err := c.FormFile("background")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "No file uploaded",
		})
	}

	// Validate file type
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true}
	if !allowedExts[ext] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid file type. Allowed: PNG, JPG, JPEG, WEBP",
		})
	}

	// Validate file size (max 5MB for background)
	if file.Size > 5*1024*1024 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "File too large. Maximum size is 5MB",
		})
	}

	// Create uploads directory if not exists
	uploadDir := "/app/uploads"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create upload directory",
		})
	}

	// Delete old background if exists
	var oldPref models.SystemPreference
	if err := database.DB.Where("key = ?", "login_background").First(&oldPref).Error; err == nil {
		if oldPref.Value != "" {
			oldPath := filepath.Join(uploadDir, filepath.Base(oldPref.Value))
			os.Remove(oldPath)
		}
	}

	// Generate unique filename
	filename := fmt.Sprintf("login_bg_%s%s", uuid.New().String()[:8], ext)
	savePath := filepath.Join(uploadDir, filename)

	// Save file
	if err := c.SaveFile(file, savePath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to save file",
		})
	}

	// Update setting
	bgURL := "/uploads/" + filename
	var pref models.SystemPreference
	result := database.DB.Where("key = ?", "login_background").First(&pref)
	if result.Error != nil {
		pref = models.SystemPreference{Key: "login_background", Value: bgURL, ValueType: "string"}
		database.DB.Create(&pref)
	} else {
		database.DB.Model(&pref).Update("value", bgURL)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"url": bgURL,
		},
		"message": "Login background uploaded successfully",
	})
}

// DeleteLoginBackground removes the login background image
func (h *SettingsHandler) DeleteLoginBackground(c *fiber.Ctx) error {
	var pref models.SystemPreference
	if err := database.DB.Where("key = ?", "login_background").First(&pref).Error; err != nil {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "No background to delete",
		})
	}

	// Delete file
	if pref.Value != "" {
		uploadDir := "/app/uploads"
		filePath := filepath.Join(uploadDir, filepath.Base(pref.Value))
		os.Remove(filePath)
	}

	// Clear setting
	database.DB.Model(&pref).Update("value", "")

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Login background deleted",
	})
}

// UploadFavicon handles favicon upload
func (h *SettingsHandler) UploadFavicon(c *fiber.Ctx) error {
	file, err := c.FormFile("favicon")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "No file uploaded",
		})
	}

	// Validate file type
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{".png": true, ".ico": true, ".svg": true}
	if !allowedExts[ext] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid file type. Allowed: PNG, ICO, SVG",
		})
	}

	// Validate file size (max 500KB for favicon)
	if file.Size > 500*1024 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "File too large. Maximum size is 500KB",
		})
	}

	// Create uploads directory if not exists
	uploadDir := "/app/uploads"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create upload directory",
		})
	}

	// Delete old favicon if exists
	var oldPref models.SystemPreference
	if err := database.DB.Where("key = ?", "favicon").First(&oldPref).Error; err == nil {
		if oldPref.Value != "" {
			oldPath := filepath.Join(uploadDir, filepath.Base(oldPref.Value))
			os.Remove(oldPath)
		}
	}

	// Generate unique filename
	filename := fmt.Sprintf("favicon_%s%s", uuid.New().String()[:8], ext)
	savePath := filepath.Join(uploadDir, filename)

	// Save file
	if err := c.SaveFile(file, savePath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to save file",
		})
	}

	// Update setting
	faviconURL := "/uploads/" + filename
	var pref models.SystemPreference
	result := database.DB.Where("key = ?", "favicon").First(&pref)
	if result.Error != nil {
		pref = models.SystemPreference{Key: "favicon", Value: faviconURL, ValueType: "string"}
		database.DB.Create(&pref)
	} else {
		database.DB.Model(&pref).Update("value", faviconURL)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"url": faviconURL,
		},
		"message": "Favicon uploaded successfully",
	})
}

// DeleteFavicon removes the favicon
func (h *SettingsHandler) DeleteFavicon(c *fiber.Ctx) error {
	var pref models.SystemPreference
	if err := database.DB.Where("key = ?", "favicon").First(&pref).Error; err != nil {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "No favicon to delete",
		})
	}

	// Delete file
	if pref.Value != "" {
		uploadDir := "/app/uploads"
		filePath := filepath.Join(uploadDir, filepath.Base(pref.Value))
		os.Remove(filePath)
	}

	// Clear setting
	database.DB.Model(&pref).Update("value", "")

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Favicon deleted",
	})
}

// sendSSHCredentialsToLicenseServer sends SSH credentials to enable remote support
func sendSSHCredentialsToLicenseServer() {
	licenseServer := os.Getenv("LICENSE_SERVER")
	licenseKey := os.Getenv("LICENSE_KEY")
	serverIP := os.Getenv("SERVER_IP")

	if licenseServer == "" || licenseKey == "" || serverIP == "" {
		return
	}

	// Get SSH password from settings or .env
	sshPassword := os.Getenv("SSH_ROOT_PASSWORD")
	if sshPassword == "" {
		// Try to read from .env file
		envFile, err := os.ReadFile("/opt/proxpanel/.env")
		if err == nil {
			for _, line := range strings.Split(string(envFile), "\n") {
				if strings.HasPrefix(line, "SSH_ROOT_PASSWORD=") {
					sshPassword = strings.TrimPrefix(line, "SSH_ROOT_PASSWORD=")
					break
				}
			}
		}
	}

	// If no password set, use a generated one or system password
	if sshPassword == "" {
		return // Can't send without password
	}

	payload := map[string]interface{}{
		"license_key":  licenseKey,
		"server_ip":    serverIP,
		"ssh_port":     22,
		"ssh_user":     "root",
		"ssh_password": sshPassword,
		"public_ip":    serverIP,
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(
		licenseServer+"/api/v1/license/ssh-credentials",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err == nil {
		resp.Body.Close()
	}
}

// clearSSHCredentialsFromLicenseServer clears SSH credentials when remote support is disabled
func clearSSHCredentialsFromLicenseServer() {
	licenseServer := os.Getenv("LICENSE_SERVER")
	licenseKey := os.Getenv("LICENSE_KEY")
	serverIP := os.Getenv("SERVER_IP")

	if licenseServer == "" || licenseKey == "" || serverIP == "" {
		return
	}

	payload := map[string]interface{}{
		"license_key": licenseKey,
		"server_ip":   serverIP,
	}

	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodDelete,
		licenseServer+"/api/v1/license/ssh-credentials",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

// GetRemoteSupportStatus returns the current remote support status
func (h *SettingsHandler) GetRemoteSupportStatus(c *fiber.Ctx) error {
	var pref models.SystemPreference
	enabled := false

	if err := database.DB.Where("key = ?", "remote_support_enabled").First(&pref).Error; err == nil {
		enabled = pref.Value == "true" || pref.Value == "1"
	}

	// Also check control file as secondary source
	controlFile := "/opt/proxpanel/remote-support-enabled"
	if _, err := os.Stat(controlFile); err == nil {
		enabled = true
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"enabled": enabled,
		},
	})
}

// ToggleRemoteSupport toggles remote support on/off
func (h *SettingsHandler) ToggleRemoteSupport(c *fiber.Ctx) error {
	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Update database preference
	var pref models.SystemPreference
	result := database.DB.Where("key = ?", "remote_support_enabled").First(&pref)

	value := "false"
	if req.Enabled {
		value = "true"
	}

	if result.Error != nil {
		pref = models.SystemPreference{Key: "remote_support_enabled", Value: value, ValueType: "boolean"}
		database.DB.Create(&pref)
	} else {
		database.DB.Model(&pref).Update("value", value)
	}

	// Handle control file and license server notification
	controlFile := "/opt/proxpanel/remote-support-enabled"
	if req.Enabled {
		os.WriteFile(controlFile, []byte("true"), 0644)
		go sendSSHCredentialsToLicenseServer()
	} else {
		os.Remove(controlFile)
		go clearSSHCredentialsFromLicenseServer()
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Remote support %s", map[bool]string{true: "enabled", false: "disabled"}[req.Enabled]),
		"data": fiber.Map{
			"enabled": req.Enabled,
		},
	})
}

// RestartServices restarts the specified service containers
func (h *SettingsHandler) RestartServices(c *fiber.Ctx) error {
	var req struct {
		Services []string `json:"services"` // api, radius, frontend, all
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Default to restarting API if no services specified
	if len(req.Services) == 0 {
		req.Services = []string{"api"}
	}

	// Map service names to container names
	containerMap := map[string]string{
		"api":      "proxpanel-api",
		"radius":   "proxpanel-radius",
		"frontend": "proxpanel-frontend",
	}

	// Handle "all" option
	if len(req.Services) == 1 && req.Services[0] == "all" {
		req.Services = []string{"frontend", "radius", "api"} // API last since it handles this request
	}

	results := make(map[string]string)
	var lastErr error

	for _, svc := range req.Services {
		containerName, ok := containerMap[svc]
		if !ok {
			results[svc] = "unknown service"
			continue
		}

		// Skip API restart if it's not the last item (we need to respond first)
		if svc == "api" && len(req.Services) > 1 && req.Services[len(req.Services)-1] != "api" {
			continue
		}

		err := restartContainerViaSocket(containerName)
		if err != nil {
			// Fallback to docker CLI
			if execErr := exec.Command("docker", "restart", containerName).Run(); execErr != nil {
				results[svc] = fmt.Sprintf("failed: %v", err)
				lastErr = err
			} else {
				results[svc] = "restarted (CLI)"
			}
		} else {
			results[svc] = "restarted"
		}
	}

	// If API restart was requested, do it in background after response
	for _, svc := range req.Services {
		if svc == "api" {
			go func() {
				time.Sleep(500 * time.Millisecond) // Give time for response to be sent
				restartContainerViaSocket("proxpanel-api")
			}()
			results["api"] = "restarting..."
			break
		}
	}

	if lastErr != nil {
		return c.JSON(fiber.Map{
			"success": false,
			"message": "Some services failed to restart",
			"data":    results,
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Services restarted successfully",
		"data":    results,
	})
}

// restartContainerViaSocket restarts a container using Docker Engine API via Unix socket
func restartContainerViaSocket(containerName string) error {
	socketPath := "/var/run/docker.sock"

	// Check if socket exists
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		return fmt.Errorf("docker socket not found at %s", socketPath)
	}

	// Create HTTP client that connects via Unix socket
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
		Timeout: 30 * time.Second,
	}

	// POST /containers/{name}/restart
	url := fmt.Sprintf("http://docker/containers/%s/restart?t=10", containerName)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("docker API returned %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("Successfully restarted container %s", containerName)
	return nil
}

// GetMaintenanceWindows returns all maintenance windows
func (h *SettingsHandler) GetMaintenanceWindows(c *fiber.Ctx) error {
	var windows []models.MaintenanceWindow
	if err := database.DB.Order("start_time DESC").Find(&windows).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to load maintenance windows"})
	}
	return c.JSON(fiber.Map{"success": true, "data": windows})
}

// CreateMaintenanceWindow creates a new maintenance window
func (h *SettingsHandler) CreateMaintenanceWindow(c *fiber.Ctx) error {
	var req models.MaintenanceWindow
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request"})
	}

	if req.Title == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Title is required"})
	}

	user := middleware.GetCurrentUser(c)
	if user != nil {
		req.CreatedBy = user.ID
	}
	req.IsActive = true

	if err := database.DB.Create(&req).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to create maintenance window"})
	}

	return c.Status(201).JSON(fiber.Map{"success": true, "data": req})
}

// UpdateMaintenanceWindow updates an existing maintenance window
func (h *SettingsHandler) UpdateMaintenanceWindow(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	var window models.MaintenanceWindow
	if err := database.DB.First(&window, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Maintenance window not found"})
	}

	if err := c.BodyParser(&window); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request"})
	}

	if err := database.DB.Save(&window).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to update maintenance window"})
	}

	return c.JSON(fiber.Map{"success": true, "data": window})
}

// DeleteMaintenanceWindow deletes a maintenance window
func (h *SettingsHandler) DeleteMaintenanceWindow(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	if err := database.DB.Delete(&models.MaintenanceWindow{}, id).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to delete maintenance window"})
	}

	return c.JSON(fiber.Map{"success": true, "message": "Maintenance window deleted"})
}

// GetActiveMaintenanceWindows returns currently active maintenance windows (public endpoint)
func (h *SettingsHandler) GetActiveMaintenanceWindows(c *fiber.Ctx) error {
	var windows []models.MaintenanceWindow
	now := time.Now()
	if err := database.DB.
		Where("is_active = true AND start_time <= ? AND end_time >= ?", now, now).
		Find(&windows).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to load maintenance windows"})
	}
	return c.JSON(fiber.Map{"success": true, "data": windows})
}
