package handlers

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/middleware"
	"github.com/proisp/backend/internal/mikrotik"
	"github.com/proisp/backend/internal/models"
	"github.com/proisp/backend/internal/radius"
)

type FUPHandler struct{}

func NewFUPHandler() *FUPHandler {
	return &FUPHandler{}
}

// QuotaStats represents quota statistics
type QuotaStats struct {
	TotalSubscribers     int64 `json:"total_subscribers"`
	ActiveFUP            int64 `json:"active_fup"`
	DailyQuotaExceeded   int64 `json:"daily_quota_exceeded"`
	MonthlyQuotaExceeded int64 `json:"monthly_quota_exceeded"`
	UnlimitedQuota       int64 `json:"unlimited_quota"`
}

// SubscriberQuota represents subscriber quota data
type SubscriberQuota struct {
	ID               uint    `json:"id"`
	Username         string  `json:"username"`
	FullName         string  `json:"full_name"`
	ServiceName      string  `json:"service_name"`
	ResellerName     string  `json:"reseller_name"`
	DailyQuota       int64   `json:"daily_quota"`
	DailyUsed        int64   `json:"daily_used"`
	DailyPercent     float64 `json:"daily_percent"`
	MonthlyQuota     int64   `json:"monthly_quota"`
	MonthlyUsed      int64   `json:"monthly_used"`
	MonthlyPercent   float64 `json:"monthly_percent"`
	FUPLevel         int     `json:"fup_level"`
	IsOnline         bool    `json:"is_online"`
	LastQuotaReset   *string `json:"last_quota_reset"`
}

// GetStats returns FUP/Quota statistics
func (h *FUPHandler) GetStats(c *fiber.Ctx) error {
	var stats QuotaStats

	// Total subscribers
	database.DB.Model(&models.Subscriber{}).Count(&stats.TotalSubscribers)

	// Active FUP (FUP level > 0)
	database.DB.Model(&models.Subscriber{}).Where("fup_level > 0").Count(&stats.ActiveFUP)

	// Daily quota exceeded
	database.DB.Model(&models.Subscriber{}).
		Joins("JOIN services ON subscribers.service_id = services.id").
		Where("services.daily_quota > 0 AND subscribers.daily_quota_used >= services.daily_quota").
		Count(&stats.DailyQuotaExceeded)

	// Monthly quota exceeded
	database.DB.Model(&models.Subscriber{}).
		Joins("JOIN services ON subscribers.service_id = services.id").
		Where("services.monthly_quota > 0 AND subscribers.monthly_quota_used >= services.monthly_quota").
		Count(&stats.MonthlyQuotaExceeded)

	// Unlimited quota (services with 0 quota)
	database.DB.Model(&models.Subscriber{}).
		Joins("JOIN services ON subscribers.service_id = services.id").
		Where("services.daily_quota = 0 AND services.monthly_quota = 0").
		Count(&stats.UnlimitedQuota)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    stats,
	})
}

// ListQuotas returns subscribers with quota usage
func (h *FUPHandler) ListQuotas(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	// Parse query params
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "25"))
	search := c.Query("search", "")
	fupStatus := c.Query("fup_status", "")
	quotaStatus := c.Query("quota_status", "")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 25
	}
	offset := (page - 1) * limit

	// Build query
	query := database.DB.Model(&models.Subscriber{}).
		Select(`subscribers.id, subscribers.username, subscribers.full_name,
			subscribers.daily_quota_used, subscribers.monthly_quota_used,
			subscribers.fup_level, subscribers.is_online, subscribers.last_quota_reset,
			services.name as service_name, services.daily_quota, services.monthly_quota,
			resellers.name as reseller_name`).
		Joins("JOIN services ON subscribers.service_id = services.id").
		Joins("JOIN resellers ON subscribers.reseller_id = resellers.id")

	// Filter by reseller for non-admin users
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		query = query.Where("subscribers.reseller_id IN (SELECT id FROM resellers WHERE id = ? OR parent_id = ?)", *user.ResellerID, *user.ResellerID)
	}

	// Search filter
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("subscribers.username ILIKE ? OR subscribers.full_name ILIKE ?", searchPattern, searchPattern)
	}

	// FUP status filter
	switch fupStatus {
	case "active":
		query = query.Where("subscribers.fup_level > 0")
	case "normal":
		query = query.Where("subscribers.fup_level = 0")
	}

	// Quota status filter
	switch quotaStatus {
	case "daily_exceeded":
		query = query.Where("services.daily_quota > 0 AND subscribers.daily_quota_used >= services.daily_quota")
	case "monthly_exceeded":
		query = query.Where("services.monthly_quota > 0 AND subscribers.monthly_quota_used >= services.monthly_quota")
	case "warning":
		query = query.Where("(services.daily_quota > 0 AND subscribers.daily_quota_used >= services.daily_quota * 0.8) OR (services.monthly_quota > 0 AND subscribers.monthly_quota_used >= services.monthly_quota * 0.8)")
	case "unlimited":
		query = query.Where("services.daily_quota = 0 AND services.monthly_quota = 0")
	}

	// Count total
	var total int64
	query.Count(&total)

	// Fetch data
	var results []struct {
		ID             uint       `json:"id"`
		Username       string     `json:"username"`
		FullName       string     `json:"full_name"`
		DailyQuotaUsed int64      `json:"daily_quota_used"`
		MonthlyQuotaUsed int64    `json:"monthly_quota_used"`
		FUPLevel       int        `json:"fup_level"`
		IsOnline       bool       `json:"is_online"`
		LastQuotaReset *time.Time `json:"last_quota_reset"`
		ServiceName    string     `json:"service_name"`
		DailyQuota     int64      `json:"daily_quota"`
		MonthlyQuota   int64      `json:"monthly_quota"`
		ResellerName   string     `json:"reseller_name"`
	}

	query.Order("subscribers.fup_level DESC, subscribers.monthly_quota_used DESC").
		Offset(offset).Limit(limit).Scan(&results)

	// Calculate percentages
	quotas := make([]SubscriberQuota, len(results))
	for i, r := range results {
		quotas[i] = SubscriberQuota{
			ID:           r.ID,
			Username:     r.Username,
			FullName:     r.FullName,
			ServiceName:  r.ServiceName,
			ResellerName: r.ResellerName,
			DailyQuota:   r.DailyQuota,
			DailyUsed:    r.DailyQuotaUsed,
			MonthlyQuota: r.MonthlyQuota,
			MonthlyUsed:  r.MonthlyQuotaUsed,
			FUPLevel:     r.FUPLevel,
			IsOnline:     r.IsOnline,
		}

		if r.DailyQuota > 0 {
			quotas[i].DailyPercent = float64(r.DailyQuotaUsed) / float64(r.DailyQuota) * 100
		}
		if r.MonthlyQuota > 0 {
			quotas[i].MonthlyPercent = float64(r.MonthlyQuotaUsed) / float64(r.MonthlyQuota) * 100
		}
		if r.LastQuotaReset != nil {
			formatted := r.LastQuotaReset.Format("2006-01-02 15:04:05")
			quotas[i].LastQuotaReset = &formatted
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    quotas,
		"meta": fiber.Map{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// ResetFUP resets FUP for a subscriber
func (h *FUPHandler) ResetFUP(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	var subscriber models.Subscriber
	if err := database.DB.Preload("Nas").Preload("Service").First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	now := time.Now()
	updates := map[string]interface{}{
		"fup_level":             0,
		"daily_quota_used":      0,
		"monthly_quota_used":    0,
		"daily_download_used":   0,
		"daily_upload_used":     0,
		"monthly_download_used": 0,
		"monthly_upload_used":   0,
		"last_quota_reset":      now,
		"last_daily_reset":      now,
		"last_monthly_reset":    now,
	}

	// If user is online, get current MikroTik session bytes as baseline
	// This prevents QuotaSync from recalculating all previous usage
	var client *mikrotik.Client
	var session *mikrotik.ActiveSession
	if subscriber.Nas != nil && subscriber.IsOnline {
		client = mikrotik.NewClient(
			fmt.Sprintf("%s:%d", subscriber.Nas.IPAddress, subscriber.Nas.APIPort),
			subscriber.Nas.APIUsername,
			subscriber.Nas.APIPassword,
		)
		defer client.Close()

		session, err = client.GetActiveSession(subscriber.Username)
		if err != nil {
			log.Printf("FUP ResetFUP: Failed to get session for %s: %v", subscriber.Username, err)
			updates["last_session_download"] = 0
			updates["last_session_upload"] = 0
		} else {
			// Set current session bytes as baseline so delta will be 0
			updates["last_session_download"] = session.TxBytes
			updates["last_session_upload"] = session.RxBytes
			log.Printf("FUP ResetFUP: Setting baseline for %s: dl=%d, ul=%d", subscriber.Username, session.TxBytes, session.RxBytes)
		}
	} else {
		updates["last_session_download"] = 0
		updates["last_session_upload"] = 0
	}

	database.DB.Model(&subscriber).Updates(updates)

	// Restore original speed in RADIUS radreply table (format: upload/download for MikroTik rx/tx)
	if subscriber.Service.ID > 0 {
		rateLimit := fmt.Sprintf("%dM/%dM", subscriber.Service.UploadSpeed, subscriber.Service.DownloadSpeed)
		database.DB.Model(&models.RadReply{}).
			Where("username = ? AND attribute = ?", subscriber.Username, "Mikrotik-Rate-Limit").
			Update("value", rateLimit)
	}

	// Restore original speed on MikroTik using CoA
	// Speeds are already in kb (e.g., 2000 = 2000k), no conversion needed
	if session != nil && subscriber.Service.ID > 0 {
		originalRateLimitK := fmt.Sprintf("%dk/%dk", subscriber.Service.UploadSpeed, subscriber.Service.DownloadSpeed)
		coaClient := radius.NewCOAClient(subscriber.Nas.IPAddress, subscriber.Nas.CoAPort, subscriber.Nas.Secret)

		// Try radclient-based CoA (most reliable)
		if err := coaClient.UpdateRateLimitViaRadclient(subscriber.Username, session.SessionID, originalRateLimitK); err != nil {
			log.Printf("FUP ResetFUP: Radclient CoA failed for %s: %v, trying MikroTik API", subscriber.Username, err)
			// Try MikroTik API as fallback
			if err := client.RestoreUserSpeedWithIP(subscriber.Username, session.Address, subscriber.Service.DownloadSpeed, subscriber.Service.UploadSpeed); err != nil {
				log.Printf("FUP ResetFUP: MikroTik API restore also failed for %s: %v", subscriber.Username, err)
			} else {
				log.Printf("FUP ResetFUP: Restored %s speed via MikroTik API", subscriber.Username)
			}
		} else {
			log.Printf("FUP ResetFUP: Restored %s speed via radclient CoA to %s", subscriber.Username, originalRateLimitK)
		}
	}

	// Create audit log
	user := middleware.GetCurrentUser(c)
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionResetFUP,
		EntityType:  "subscriber",
		EntityID:    subscriber.ID,
		EntityName:  subscriber.Username,
		Description: "Reset FUP and quota",
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "FUP reset successfully",
	})
}

// BulkResetRequest represents bulk reset request
type BulkResetRequest struct {
	SubscriberIDs []uint `json:"subscriber_ids"`
	ResetType     string `json:"reset_type"` // fup, daily, monthly, all
}

// BulkReset resets FUP/quota for multiple subscribers
func (h *FUPHandler) BulkReset(c *fiber.Ctx) error {
	var req BulkResetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	if len(req.SubscriberIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "No subscribers selected"})
	}

	now := time.Now()
	updates := map[string]interface{}{
		"last_quota_reset": now,
	}

	switch req.ResetType {
	case "fup":
		updates["fup_level"] = 0
	case "daily":
		updates["daily_quota_used"] = 0
	case "monthly":
		updates["monthly_quota_used"] = 0
	case "all":
		updates["fup_level"] = 0
		updates["daily_quota_used"] = 0
		updates["monthly_quota_used"] = 0
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid reset type"})
	}

	result := database.DB.Model(&models.Subscriber{}).Where("id IN ?", req.SubscriberIDs).Updates(updates)

	// Create audit log
	user := middleware.GetCurrentUser(c)
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionResetFUP,
		EntityType:  "subscriber",
		Description: "Bulk reset " + req.ResetType + " for " + strconv.Itoa(len(req.SubscriberIDs)) + " subscribers",
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Reset completed for " + strconv.Itoa(int(result.RowsAffected)) + " subscribers",
	})
}

// ResetAllFUP resets FUP for all subscribers with active FUP
func (h *FUPHandler) ResetAllFUP(c *fiber.Ctx) error {
	now := time.Now()
	result := database.DB.Model(&models.Subscriber{}).
		Where("fup_level > 0").
		Updates(map[string]interface{}{
			"fup_level":        0,
			"daily_quota_used": 0,
			"last_quota_reset": now,
		})

	// Create audit log
	user := middleware.GetCurrentUser(c)
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionResetFUP,
		EntityType:  "subscriber",
		Description: "Reset FUP for all subscribers with active FUP",
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Reset completed for " + strconv.Itoa(int(result.RowsAffected)) + " subscribers",
	})
}

// GetQuotaHistory returns quota usage history for a subscriber
func (h *FUPHandler) GetQuotaHistory(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	period := c.Query("period", "daily") // daily or monthly
	limit, _ := strconv.Atoi(c.Query("limit", "30"))

	if period == "daily" {
		var history []models.DailyQuota
		database.DB.Where("subscriber_id = ?", id).
			Order("date DESC").
			Limit(limit).
			Find(&history)

		return c.JSON(fiber.Map{
			"success": true,
			"data":    history,
		})
	}

	var history []models.MonthlyQuota
	database.DB.Where("subscriber_id = ?", id).
		Order("month DESC").
		Limit(limit).
		Find(&history)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    history,
	})
}

// GetTopUsers returns top quota users
func (h *FUPHandler) GetTopUsers(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	period := c.Query("period", "monthly") // daily or monthly

	var results []struct {
		ID          uint   `json:"id"`
		Username    string `json:"username"`
		FullName    string `json:"full_name"`
		ServiceName string `json:"service_name"`
		QuotaUsed   int64  `json:"quota_used"`
		QuotaLimit  int64  `json:"quota_limit"`
		Percent     float64 `json:"percent"`
	}

	if period == "daily" {
		database.DB.Model(&models.Subscriber{}).
			Select(`subscribers.id, subscribers.username, subscribers.full_name,
				services.name as service_name, subscribers.daily_quota_used as quota_used,
				services.daily_quota as quota_limit`).
			Joins("JOIN services ON subscribers.service_id = services.id").
			Where("services.daily_quota > 0").
			Order("subscribers.daily_quota_used DESC").
			Limit(limit).
			Scan(&results)
	} else {
		database.DB.Model(&models.Subscriber{}).
			Select(`subscribers.id, subscribers.username, subscribers.full_name,
				services.name as service_name, subscribers.monthly_quota_used as quota_used,
				services.monthly_quota as quota_limit`).
			Joins("JOIN services ON subscribers.service_id = services.id").
			Where("services.monthly_quota > 0").
			Order("subscribers.monthly_quota_used DESC").
			Limit(limit).
			Scan(&results)
	}

	// Calculate percentages
	for i := range results {
		if results[i].QuotaLimit > 0 {
			results[i].Percent = float64(results[i].QuotaUsed) / float64(results[i].QuotaLimit) * 100
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    results,
	})
}
