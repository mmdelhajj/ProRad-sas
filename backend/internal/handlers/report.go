package handlers

import (
	"fmt"
	"math"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

type ReportHandler struct{}

func NewReportHandler() *ReportHandler {
	return &ReportHandler{}
}

// GetSubscriberStats returns subscriber statistics
func (h *ReportHandler) GetSubscriberStats(c *fiber.Ctx) error {
	resellerID := c.QueryInt("reseller_id", 0)

	var total, active, expired, suspended, online int64

	query := database.DB.Model(&models.Subscriber{})
	if resellerID > 0 {
		query = query.Where("reseller_id = ?", resellerID)
	}

	query.Count(&total)
	database.DB.Model(&models.Subscriber{}).Where("status = ?", "active").Count(&active)
	database.DB.Model(&models.Subscriber{}).Where("status = ?", "expired").Count(&expired)
	database.DB.Model(&models.Subscriber{}).Where("status = ?", "suspended").Count(&suspended)
	database.DB.Model(&models.Session{}).Where("status = ?", "online").Count(&online)

	// New subscribers this month
	startOfMonth := time.Now().AddDate(0, 0, -time.Now().Day()+1).Truncate(24 * time.Hour)
	var newThisMonth int64
	database.DB.Model(&models.Subscriber{}).Where("created_at >= ?", startOfMonth).Count(&newThisMonth)

	// Expiring soon (next 7 days)
	var expiringSoon int64
	database.DB.Model(&models.Subscriber{}).
		Where("expiry_date BETWEEN ? AND ?", time.Now(), time.Now().AddDate(0, 0, 7)).
		Count(&expiringSoon)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"total":         total,
			"active":        active,
			"expired":       expired,
			"suspended":     suspended,
			"online":        online,
			"newThisMonth":  newThisMonth,
			"expiringSoon":  expiringSoon,
		},
	})
}

// revenueTypes are transaction types that count as income
var revenueTypes = []string{
	"renewal", "new", "prepaid_card", "addon", "refill",
	"static_ip", "change_service", "reset_fup", "rename",
}

// GetRevenueStats returns revenue statistics from the transactions table
func (h *ReportHandler) GetRevenueStats(c *fiber.Ctx) error {
	period := c.Query("period", "month")
	resellerID := c.QueryInt("reseller_id", 0)
	dateFrom := c.Query("date_from", "")
	dateTo := c.Query("date_to", "")

	now := time.Now()
	today := now.Truncate(24 * time.Hour)

	// Build base WHERE clause for revenue types + optional reseller filter
	baseWhere := "type IN ?"
	baseArgs := []interface{}{revenueTypes}
	if resellerID > 0 {
		baseWhere += " AND reseller_id = ?"
		baseArgs = append(baseArgs, resellerID)
	}

	// --- Period summary cards (always computed) ---
	type SumResult struct {
		Total float64
		Count int64
	}

	sumQuery := func(from, to time.Time) SumResult {
		var r SumResult
		database.DB.Model(&models.Transaction{}).
			Where(baseWhere, baseArgs...).
			Where("created_at >= ? AND created_at < ?", from, to).
			Select("COALESCE(SUM(ABS(amount)), 0) as total, COUNT(*) as count").
			Scan(&r)
		return r
	}

	// Today vs yesterday
	todayR := sumQuery(today, today.AddDate(0, 0, 1))
	yesterdayR := sumQuery(today.AddDate(0, 0, -1), today)

	// This week (Mon-Sun) vs last week
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekStart := today.AddDate(0, 0, -(weekday - 1))
	weekR := sumQuery(weekStart, weekStart.AddDate(0, 0, 7))
	prevWeekR := sumQuery(weekStart.AddDate(0, 0, -7), weekStart)

	// This month vs last month
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthR := sumQuery(monthStart, monthStart.AddDate(0, 1, 0))
	prevMonthR := sumQuery(monthStart.AddDate(0, -1, 0), monthStart)

	// This year vs last year
	yearStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	yearR := sumQuery(yearStart, yearStart.AddDate(1, 0, 0))
	prevYearR := sumQuery(yearStart.AddDate(-1, 0, 0), yearStart)

	pctChange := func(current, previous float64) float64 {
		if previous > 0 {
			return math.Round(((current-previous)/previous)*10000) / 100
		}
		return 0
	}

	// --- Selected period data ---
	var periodStart, periodEnd time.Time
	switch period {
	case "day":
		periodStart = today
		periodEnd = today.AddDate(0, 0, 1)
	case "week":
		periodStart = weekStart
		periodEnd = weekStart.AddDate(0, 0, 7)
	case "year":
		periodStart = yearStart
		periodEnd = yearStart.AddDate(1, 0, 0)
	case "custom":
		if dateFrom != "" {
			if t, err := time.Parse("2006-01-02", dateFrom); err == nil {
				periodStart = t
			}
		}
		if dateTo != "" {
			if t, err := time.Parse("2006-01-02", dateTo); err == nil {
				periodEnd = t.AddDate(0, 0, 1) // include the end date
			}
		}
		if periodStart.IsZero() {
			periodStart = monthStart
		}
		if periodEnd.IsZero() {
			periodEnd = now
		}
	default: // month
		periodStart = monthStart
		periodEnd = monthStart.AddDate(0, 1, 0)
	}

	// Transaction count for selected period
	periodR := sumQuery(periodStart, periodEnd)

	// Daily revenue for bar chart
	type DailyRevenue struct {
		Date   string  `json:"date"`
		Amount float64 `json:"amount"`
		Count  int64   `json:"count"`
	}
	var dailyRevenue []DailyRevenue
	database.DB.Model(&models.Transaction{}).
		Where(baseWhere, baseArgs...).
		Where("created_at >= ? AND created_at < ?", periodStart, periodEnd).
		Select("DATE(created_at) as date, COALESCE(SUM(ABS(amount)), 0) as amount, COUNT(*) as count").
		Group("DATE(created_at)").
		Order("date").
		Scan(&dailyRevenue)

	// Revenue by type
	type TypeRevenue struct {
		Type   string  `json:"type"`
		Amount float64 `json:"amount"`
		Count  int64   `json:"count"`
	}
	var byType []TypeRevenue
	database.DB.Model(&models.Transaction{}).
		Where(baseWhere, baseArgs...).
		Where("created_at >= ? AND created_at < ?", periodStart, periodEnd).
		Select("type, COALESCE(SUM(ABS(amount)), 0) as amount, COUNT(*) as count").
		Group("type").
		Order("amount DESC").
		Scan(&byType)

	// Revenue by service (raw SQL JOIN)
	type ServiceRevenue struct {
		ServiceName string  `json:"service_name"`
		Amount      float64 `json:"amount"`
		Count       int64   `json:"count"`
	}
	var byService []ServiceRevenue
	if resellerID > 0 {
		database.DB.Raw(`
			SELECT COALESCE(sv.name, t.service_name, 'Unknown') as service_name,
			       COALESCE(SUM(ABS(t.amount)), 0) as amount,
			       COUNT(*) as count
			FROM transactions t
			LEFT JOIN subscribers s ON t.subscriber_id = s.id
			LEFT JOIN services sv ON s.service_id = sv.id
			WHERE t.type IN ? AND t.created_at >= ? AND t.created_at < ? AND t.reseller_id = ?
			GROUP BY service_name ORDER BY amount DESC`,
			revenueTypes, periodStart, periodEnd, resellerID).Scan(&byService)
	} else {
		database.DB.Raw(`
			SELECT COALESCE(sv.name, t.service_name, 'Unknown') as service_name,
			       COALESCE(SUM(ABS(t.amount)), 0) as amount,
			       COUNT(*) as count
			FROM transactions t
			LEFT JOIN subscribers s ON t.subscriber_id = s.id
			LEFT JOIN services sv ON s.service_id = sv.id
			WHERE t.type IN ? AND t.created_at >= ? AND t.created_at < ?
			GROUP BY service_name ORDER BY amount DESC`,
			revenueTypes, periodStart, periodEnd).Scan(&byService)
	}

	// Revenue by reseller (raw SQL JOIN, top 20)
	type ResellerRevenue struct {
		ResellerName string  `json:"reseller_name"`
		Amount       float64 `json:"amount"`
		Count        int64   `json:"count"`
	}
	var byReseller []ResellerRevenue
	database.DB.Raw(`
		SELECT COALESCE(r.name, 'Direct/Admin') as reseller_name,
		       COALESCE(SUM(ABS(t.amount)), 0) as amount,
		       COUNT(*) as count
		FROM transactions t
		LEFT JOIN resellers r ON t.reseller_id = r.id
		WHERE t.type IN ? AND t.created_at >= ? AND t.created_at < ?
		GROUP BY reseller_name ORDER BY amount DESC LIMIT 20`,
		revenueTypes, periodStart, periodEnd).Scan(&byReseller)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"today_revenue":     todayR.Total,
			"today_change":      pctChange(todayR.Total, yesterdayR.Total),
			"week_revenue":      weekR.Total,
			"week_change":       pctChange(weekR.Total, prevWeekR.Total),
			"month_revenue":     monthR.Total,
			"month_change":      pctChange(monthR.Total, prevMonthR.Total),
			"year_revenue":      yearR.Total,
			"year_change":       pctChange(yearR.Total, prevYearR.Total),
			"transaction_count": periodR.Count,
			"daily_revenue":     dailyRevenue,
			"by_type":           byType,
			"by_service":        byService,
			"by_reseller":       byReseller,
		},
	})
}

// GetServiceStats returns service plan statistics
func (h *ReportHandler) GetServiceStats(c *fiber.Ctx) error {
	type ServiceStat struct {
		ID              uint    `json:"id"`
		Name            string  `json:"name"`
		SubscriberCount int64   `json:"subscriber_count"`
		Revenue         float64 `json:"revenue"`
	}

	var stats []ServiceStat
	database.DB.Model(&models.Service{}).
		Select(`services.id, services.name,
			(SELECT COUNT(*) FROM subscribers WHERE service_id = services.id) as subscriber_count,
			(SELECT COALESCE(SUM(amount), 0) FROM payments p
			 JOIN subscribers s ON p.subscriber_id = s.id
			 WHERE s.service_id = services.id) as revenue`).
		Scan(&stats)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    stats,
	})
}

// GetResellerStats returns reseller statistics
func (h *ReportHandler) GetResellerStats(c *fiber.Ctx) error {
	type ResellerStat struct {
		ID              uint    `json:"id"`
		Name            string  `json:"name"`
		Balance         float64 `json:"balance"`
		SubscriberCount int64   `json:"subscriber_count"`
		ActiveCount     int64   `json:"active_count"`
	}

	var stats []ResellerStat
	database.DB.Model(&models.Reseller{}).
		Select(`resellers.id, resellers.name, resellers.balance,
			(SELECT COUNT(*) FROM subscribers WHERE reseller_id = resellers.id) as subscriber_count,
			(SELECT COUNT(*) FROM subscribers WHERE reseller_id = resellers.id AND status = 'active') as active_count`).
		Scan(&stats)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    stats,
	})
}

// GetUsageStats returns bandwidth usage statistics
func (h *ReportHandler) GetUsageStats(c *fiber.Ctx) error {
	period := c.Query("period", "day") // day, week, month

	var startDate time.Time
	switch period {
	case "week":
		startDate = time.Now().AddDate(0, 0, -7)
	case "month":
		startDate = time.Now().AddDate(0, -1, 0)
	default: // day
		startDate = time.Now().Truncate(24 * time.Hour)
	}

	// Total usage
	type UsageStat struct {
		TotalUpload   int64 `json:"total_upload"`
		TotalDownload int64 `json:"total_download"`
	}
	var totalUsage UsageStat
	database.DB.Model(&models.RadiusAccounting{}).
		Select("COALESCE(SUM(acct_input_octets), 0) as total_upload, COALESCE(SUM(acct_output_octets), 0) as total_download").
		Where("acct_start_time >= ?", startDate).
		Scan(&totalUsage)

	// Top users by usage
	type TopUser struct {
		Username string `json:"username"`
		Upload   int64  `json:"upload"`
		Download int64  `json:"download"`
	}
	var topUsers []TopUser
	database.DB.Model(&models.RadiusAccounting{}).
		Select("username, SUM(acct_input_octets) as upload, SUM(acct_output_octets) as download").
		Where("acct_start_time >= ?", startDate).
		Group("username").
		Order("download DESC").
		Limit(20).
		Scan(&topUsers)

	// Hourly usage for chart
	type HourlyUsage struct {
		Hour     int   `json:"hour"`
		Upload   int64 `json:"upload"`
		Download int64 `json:"download"`
	}
	var hourlyUsage []HourlyUsage
	database.DB.Model(&models.RadiusAccounting{}).
		Select("EXTRACT(HOUR FROM acct_start_time) as hour, SUM(acct_input_octets) as upload, SUM(acct_output_octets) as download").
		Where("acct_start_time >= ?", time.Now().Truncate(24*time.Hour)).
		Group("hour").
		Order("hour").
		Scan(&hourlyUsage)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"totalUsage":  totalUsage,
			"topUsers":    topUsers,
			"hourlyUsage": hourlyUsage,
		},
	})
}

// GetExpiryReport returns subscribers expiring within a date range
func (h *ReportHandler) GetExpiryReport(c *fiber.Ctx) error {
	days := c.QueryInt("days", 7)
	resellerID := c.QueryInt("reseller_id", 0)

	query := database.DB.Model(&models.Subscriber{}).
		Preload("Service").
		Preload("Reseller").
		Where("expiry_date BETWEEN ? AND ?", time.Now(), time.Now().AddDate(0, 0, days))

	if resellerID > 0 {
		query = query.Where("reseller_id = ?", resellerID)
	}

	var subscribers []models.Subscriber
	query.Order("expiry_date").Find(&subscribers)

	// Group by days until expiry
	type DayGroup struct {
		Day   int   `json:"day"`
		Count int64 `json:"count"`
	}
	var dayGroups []DayGroup
	for i := 0; i <= days; i++ {
		targetDate := time.Now().AddDate(0, 0, i).Truncate(24 * time.Hour)
		var count int64
		database.DB.Model(&models.Subscriber{}).
			Where("DATE(expiry_date) = ?", targetDate.Format("2006-01-02")).
			Count(&count)
		if count > 0 {
			dayGroups = append(dayGroups, DayGroup{Day: i, Count: count})
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"subscribers": subscribers,
			"byDay":       dayGroups,
			"total":       len(subscribers),
		},
	})
}

// GetTransactionReport returns transaction report
func (h *ReportHandler) GetTransactionReport(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	transType := c.Query("type", "")
	dateFrom := c.Query("date_from", "")
	dateTo := c.Query("date_to", "")
	resellerID := c.QueryInt("reseller_id", 0)

	if page < 1 {
		page = 1
	}
	if limit > 200 {
		limit = 200
	}
	offset := (page - 1) * limit

	query := database.DB.Model(&models.Transaction{}).Preload("Subscriber")

	if transType != "" {
		query = query.Where("type = ?", transType)
	}
	if dateFrom != "" {
		query = query.Where("created_at >= ?", dateFrom)
	}
	if dateTo != "" {
		query = query.Where("created_at <= ?", dateTo+" 23:59:59")
	}
	if resellerID > 0 {
		query = query.Where("reseller_id = ?", resellerID)
	}

	var total int64
	query.Count(&total)

	var transactions []models.Transaction
	query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&transactions)

	// Summary
	type Summary struct {
		Type   string  `json:"type"`
		Amount float64 `json:"amount"`
		Count  int64   `json:"count"`
	}
	var summary []Summary
	summaryQuery := database.DB.Model(&models.Transaction{}).
		Select("type, COALESCE(SUM(amount), 0) as amount, COUNT(*) as count")
	if dateFrom != "" {
		summaryQuery = summaryQuery.Where("created_at >= ?", dateFrom)
	}
	if dateTo != "" {
		summaryQuery = summaryQuery.Where("created_at <= ?", dateTo+" 23:59:59")
	}
	summaryQuery.Group("type").Scan(&summary)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    transactions,
		"summary": summary,
		"meta": fiber.Map{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// GetNASStats returns NAS device statistics
func (h *ReportHandler) GetNASStats(c *fiber.Ctx) error {
	type NASStat struct {
		ID            uint   `json:"id"`
		Name          string `json:"name"`
		IPAddress     string `json:"ip_address"`
		OnlineCount   int64  `json:"online_count"`
		TotalSessions int64  `json:"total_sessions"`
	}

	var stats []NASStat
	database.DB.Model(&models.Nas{}).
		Select(`nas.id, nas.name, nas.ip_address,
			(SELECT COUNT(*) FROM sessions WHERE nas_id = nas.id AND status = 'online') as online_count,
			(SELECT COUNT(*) FROM radacct WHERE nas_ip_address = nas.ip_address) as total_sessions`).
		Scan(&stats)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    stats,
	})
}

// ExportReport exports report data in various formats
func (h *ReportHandler) ExportReport(c *fiber.Ctx) error {
	reportType := c.Params("type")
	format := c.Query("format", "json") // json, csv

	switch reportType {
	case "subscribers":
		var subscribers []models.Subscriber
		database.DB.Preload("Service").Preload("Reseller").Find(&subscribers)
		if format == "csv" {
			c.Set("Content-Type", "text/csv")
			c.Set("Content-Disposition", "attachment; filename=subscribers.csv")
			// Generate CSV
			csv := "ID,Username,FullName,Status,Service,ExpiryDate\n"
			for _, s := range subscribers {
				serviceName := s.Service.Name
				csv += fmt.Sprintf("%d,%s,%s,%d,%s,%s\n", s.ID, s.Username, s.FullName, s.Status, serviceName, s.ExpiryDate.Format("2006-01-02"))
			}
			return c.SendString(csv)
		}
		return c.JSON(fiber.Map{"success": true, "data": subscribers})

	case "transactions":
		var transactions []models.Transaction
		database.DB.Preload("Subscriber").Find(&transactions)
		return c.JSON(fiber.Map{"success": true, "data": transactions})

	case "revenue":
		dateFrom := c.Query("date_from", "")
		dateTo := c.Query("date_to", "")
		var transactions []models.Transaction
		q := database.DB.Where("type IN ?", revenueTypes)
		if dateFrom != "" {
			q = q.Where("created_at >= ?", dateFrom)
		}
		if dateTo != "" {
			q = q.Where("created_at <= ?", dateTo+" 23:59:59")
		}
		q.Order("created_at DESC").Find(&transactions)
		if format == "csv" {
			c.Set("Content-Type", "text/csv")
			c.Set("Content-Disposition", "attachment; filename=revenue.csv")
			csvStr := "Date,Type,Amount,Description\n"
			for _, t := range transactions {
				csvStr += fmt.Sprintf("%s,%s,%.2f,\"%s\"\n",
					t.CreatedAt.Format("2006-01-02 15:04"),
					t.Type, math.Abs(t.Amount), t.Description)
			}
			return c.SendString(csvStr)
		}
		return c.JSON(fiber.Map{"success": true, "data": transactions})

	case "invoices":
		var invoices []models.Invoice
		database.DB.Preload("Subscriber").Preload("Items").Find(&invoices)
		return c.JSON(fiber.Map{"success": true, "data": invoices})

	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid report type",
		})
	}
}

// GetRevenueForecast projects future revenue based on historical trends
func (h *ReportHandler) GetRevenueForecast(c *fiber.Ctx) error {
	// Get last 6 months revenue
	type MonthRevenue struct {
		Month   string  `json:"month"`
		Revenue float64 `json:"revenue"`
	}

	var history []MonthRevenue
	database.DB.Raw(`
		SELECT TO_CHAR(created_at, 'YYYY-MM') as month, COALESCE(SUM(amount), 0) as revenue
		FROM transactions
		WHERE created_at > NOW() - INTERVAL '6 months' AND amount > 0
		GROUP BY month ORDER BY month`).Scan(&history)

	// Calculate simple growth rate
	growthRate := 0.0
	if len(history) >= 2 {
		totalGrowth := 0.0
		growthPeriods := 0
		for i := 1; i < len(history); i++ {
			if history[i-1].Revenue > 0 {
				totalGrowth += (history[i].Revenue - history[i-1].Revenue) / history[i-1].Revenue
				growthPeriods++
			}
		}
		if growthPeriods > 0 {
			growthRate = totalGrowth / float64(growthPeriods)
		}
	}

	// Project next 3 months
	var forecast []MonthRevenue
	lastRevenue := 0.0
	if len(history) > 0 {
		lastRevenue = history[len(history)-1].Revenue
	}

	now := time.Now()
	for i := 1; i <= 3; i++ {
		projected := lastRevenue * math.Pow(1+growthRate, float64(i))
		month := now.AddDate(0, i, 0).Format("2006-01")
		forecast = append(forecast, MonthRevenue{Month: month, Revenue: math.Round(projected*100) / 100})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"history":     history,
			"forecast":    forecast,
			"growth_rate": math.Round(growthRate*10000) / 100, // percentage
		},
	})
}

// GetResellerPerformance returns performance KPIs for all resellers
func (h *ReportHandler) GetResellerPerformance(c *fiber.Ctx) error {
	type ResellerKPI struct {
		ResellerID      uint    `json:"reseller_id"`
		ResellerName    string  `json:"reseller_name"`
		Username        string  `json:"username"`
		FullName        string  `json:"full_name"`
		TotalSubs       int64   `json:"total_subscribers"`
		ActiveSubs      int64   `json:"active_subscribers"`
		ActivePercent   float64 `json:"active_percent"`
		NewThisMonth    int64   `json:"new_this_month"`
		Revenue         float64 `json:"revenue"`
		Commission      float64 `json:"commission"`
		AvgLifetime     float64 `json:"avg_lifetime_days"`
		TicketCount     int64   `json:"ticket_count"`
	}

	// Get all active resellers
	var resellers []struct {
		ID       uint   `json:"id"`
		Username string `json:"username"`
		FullName string `json:"full_name"`
	}
	database.DB.Raw(`
		SELECT r.id, u.username, u.full_name FROM resellers r
		JOIN users u ON u.id = r.user_id
		WHERE r.deleted_at IS NULL AND r.is_active = true`).Scan(&resellers)

	monthStart := time.Now().Format("2006-01") + "-01"

	var kpis []ResellerKPI
	for _, r := range resellers {
		resellerName := r.Username
		if r.FullName != "" {
			resellerName = r.FullName
		}
		kpi := ResellerKPI{
			ResellerID:   r.ID,
			ResellerName: resellerName,
			Username:     r.Username,
			FullName:     r.FullName,
		}

		database.DB.Model(&models.Subscriber{}).Where("reseller_id = ? AND deleted_at IS NULL", r.ID).Count(&kpi.TotalSubs)
		database.DB.Model(&models.Subscriber{}).Where("reseller_id = ? AND status = 1 AND deleted_at IS NULL", r.ID).Count(&kpi.ActiveSubs)
		if kpi.TotalSubs > 0 {
			kpi.ActivePercent = math.Round(float64(kpi.ActiveSubs)/float64(kpi.TotalSubs)*10000) / 100
		}

		database.DB.Model(&models.Subscriber{}).Where("reseller_id = ? AND created_at >= ? AND deleted_at IS NULL", r.ID, monthStart).Count(&kpi.NewThisMonth)

		database.DB.Raw(`SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE reseller_id = ? AND amount > 0`, r.ID).Scan(&kpi.Revenue)
		database.DB.Raw(`SELECT COALESCE(SUM(commission_amount), 0) FROM reseller_commissions WHERE reseller_id = ?`, r.ID).Scan(&kpi.Commission)

		database.DB.Raw(`SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (COALESCE(expiry_date, NOW()) - created_at))/86400), 0) FROM subscribers WHERE reseller_id = ? AND deleted_at IS NULL`, r.ID).Scan(&kpi.AvgLifetime)
		kpi.AvgLifetime = math.Round(kpi.AvgLifetime*10) / 10

		database.DB.Model(&models.Ticket{}).Where("reseller_id = ?", r.ID).Count(&kpi.TicketCount)

		kpis = append(kpis, kpi)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    kpis,
	})
}

// GetChurnReport returns churn prediction data
func (h *ReportHandler) GetChurnReport(c *fiber.Ctx) error {
	riskLevel := c.Query("risk_level")

	// Summary counts
	type RiskCount struct {
		RiskLevel string `json:"risk_level"`
		Count     int64  `json:"count"`
	}
	var summary []RiskCount
	database.DB.Raw(`
		SELECT risk_level, COUNT(*) as count FROM churn_scores
		GROUP BY risk_level`).Scan(&summary)

	// Detailed list
	query := database.DB.Table("churn_scores cs").
		Select("cs.*, s.username, s.full_name, s.status, s.expiry_date, sv.name as service_name").
		Joins("JOIN subscribers s ON s.id = cs.subscriber_id").
		Joins("LEFT JOIN services sv ON sv.id = s.service_id").
		Where("s.deleted_at IS NULL")

	if riskLevel != "" {
		query = query.Where("cs.risk_level = ?", riskLevel)
	}

	query = query.Order("cs.score DESC").Limit(200)

	type ChurnEntry struct {
		SubscriberID    uint    `json:"subscriber_id"`
		Username        string  `json:"username"`
		FullName        string  `json:"full_name"`
		ServiceName     string  `json:"service_name"`
		Score           int     `json:"score"`
		RiskLevel       string  `json:"risk_level"`
		Factors         string  `json:"factors"`
		DaysUntilExpiry int     `json:"days_until_expiry"`
		UsageTrend      string  `json:"usage_trend"`
		TicketCount     int     `json:"ticket_count"`
		PaymentDelays   int     `json:"payment_delays"`
	}
	var entries []ChurnEntry
	query.Scan(&entries)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"summary":     summary,
			"subscribers": entries,
		},
	})
}
