package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

type LogsHandler struct{}

func NewLogsHandler() *LogsHandler {
	return &LogsHandler{}
}

// ListRadius returns RADIUS log entries with filters
func (h *LogsHandler) ListRadius(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	eventType := c.Query("event_type", "")
	username := c.Query("username", "")
	nasIP := c.Query("nas_ip", "")
	dateFrom := c.Query("date_from", "")
	dateTo := c.Query("date_to", "")

	if page < 1 {
		page = 1
	}
	if limit > 200 {
		limit = 200
	}
	offset := (page - 1) * limit

	query := database.DB.Model(&models.RadiusLog{})

	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}
	if username != "" {
		query = query.Where("username ILIKE ?", "%"+username+"%")
	}
	if nasIP != "" {
		query = query.Where("nas_ip = ?", nasIP)
	}
	if dateFrom != "" {
		query = query.Where("created_at >= ?", dateFrom)
	}
	if dateTo != "" {
		query = query.Where("created_at <= ?", dateTo+" 23:59:59")
	}

	var total int64
	query.Count(&total)

	var logs []models.RadiusLog
	query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    logs,
		"meta": fiber.Map{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// ListAuth returns auth-specific RADIUS log entries with summary stats
func (h *LogsHandler) ListAuth(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	result := c.Query("result", "")
	reason := c.Query("reason", "")
	username := c.Query("username", "")
	dateFrom := c.Query("date_from", "")
	dateTo := c.Query("date_to", "")

	if page < 1 {
		page = 1
	}
	if limit > 200 {
		limit = 200
	}
	offset := (page - 1) * limit

	query := database.DB.Model(&models.RadiusLog{}).
		Where("event_type IN ?", []string{"auth_accept", "auth_reject"})

	if result == "accept" {
		query = query.Where("event_type = ?", "auth_accept")
	} else if result == "reject" {
		query = query.Where("event_type = ?", "auth_reject")
	}
	if reason != "" {
		query = query.Where("reason = ?", reason)
	}
	if username != "" {
		query = query.Where("username ILIKE ?", "%"+username+"%")
	}
	if dateFrom != "" {
		query = query.Where("created_at >= ?", dateFrom)
	}
	if dateTo != "" {
		query = query.Where("created_at <= ?", dateTo+" 23:59:59")
	}

	var total int64
	query.Count(&total)

	var logs []models.RadiusLog
	query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs)

	// Summary stats for last 24 hours
	var totalLast24h, acceptsLast24h, rejectsLast24h int64
	database.DB.Model(&models.RadiusLog{}).
		Where("event_type IN ? AND created_at >= NOW() - INTERVAL '24 hours'", []string{"auth_accept", "auth_reject"}).
		Count(&totalLast24h)
	database.DB.Model(&models.RadiusLog{}).
		Where("event_type = ? AND created_at >= NOW() - INTERVAL '24 hours'", "auth_accept").
		Count(&acceptsLast24h)
	database.DB.Model(&models.RadiusLog{}).
		Where("event_type = ? AND created_at >= NOW() - INTERVAL '24 hours'", "auth_reject").
		Count(&rejectsLast24h)

	var rejectPercent float64
	if totalLast24h > 0 {
		rejectPercent = float64(rejectsLast24h) / float64(totalLast24h) * 100
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    logs,
		"meta": fiber.Map{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
		"summary": fiber.Map{
			"total_24h":      totalLast24h,
			"accepts_24h":    acceptsLast24h,
			"rejects_24h":    rejectsLast24h,
			"reject_percent": rejectPercent,
		},
	})
}

// ListSystem returns system log entries with filters
func (h *LogsHandler) ListSystem(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	level := c.Query("level", "")
	module := c.Query("module", "")
	search := c.Query("search", "")
	dateFrom := c.Query("date_from", "")
	dateTo := c.Query("date_to", "")

	if page < 1 {
		page = 1
	}
	if limit > 200 {
		limit = 200
	}
	offset := (page - 1) * limit

	query := database.DB.Model(&models.SystemLog{})

	if level != "" {
		query = query.Where("level = ?", level)
	}
	if module != "" {
		query = query.Where("module = ?", module)
	}
	if search != "" {
		query = query.Where("message ILIKE ? OR details ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if dateFrom != "" {
		query = query.Where("created_at >= ?", dateFrom)
	}
	if dateTo != "" {
		query = query.Where("created_at <= ?", dateTo+" 23:59:59")
	}

	var total int64
	query.Count(&total)

	var logs []models.SystemLog
	query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs)

	// Get distinct modules for filter dropdown
	var modules []string
	database.DB.Model(&models.SystemLog{}).Distinct("module").Where("module != ''").Pluck("module", &modules)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    logs,
		"modules": modules,
		"meta": fiber.Map{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}
