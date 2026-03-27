package handlers

import (
	"fmt"
	"math"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/proisp/backend/internal/config"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
	"github.com/proisp/backend/internal/security"
	"gorm.io/gorm"
)

type CustomerPortalHandler struct {
	cfg *config.Config
}

func NewCustomerPortalHandler(cfg *config.Config) *CustomerPortalHandler {
	return &CustomerPortalHandler{cfg: cfg}
}

// CustomerLoginRequest represents customer login request
type CustomerLoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// CustomerLoginResponse represents customer login response
type CustomerLoginResponse struct {
	Success  bool            `json:"success"`
	Message  string          `json:"message,omitempty"`
	Token    string          `json:"token,omitempty"`
	Customer *CustomerInfo   `json:"customer,omitempty"`
}

// CustomerInfo represents customer info in response
type CustomerInfo struct {
	Username    string    `json:"username"`
	FullName    string    `json:"full_name"`
	Email       string    `json:"email"`
	Phone       string    `json:"phone"`
	ServiceName string    `json:"service_name"`
	Status      string    `json:"status"`
	ExpiryDate  time.Time `json:"expiry_date"`
	DaysLeft    int       `json:"days_left"`
}

// CustomerDashboard represents customer dashboard data
type CustomerDashboard struct {
	// Profile
	Username    string    `json:"username"`
	FullName    string    `json:"full_name"`
	Email       string    `json:"email"`
	Phone       string    `json:"phone"`
	Address     string    `json:"address"`

	// Service info
	ServiceName   string    `json:"service_name"`
	Status        string    `json:"status"`
	ExpiryDate    time.Time `json:"expiry_date"`
	DaysLeft      int       `json:"days_left"`
	DownloadSpeed int64     `json:"download_speed"` // Mbps
	UploadSpeed   int64     `json:"upload_speed"`   // Mbps

	// Current speed (considering FUP)
	CurrentDownloadSpeed int64 `json:"current_download_speed"` // Kbps
	CurrentUploadSpeed   int64 `json:"current_upload_speed"`   // Kbps
	FUPLevel             int   `json:"fup_level"`
	MonthlyFUPLevel      int   `json:"monthly_fup_level"`

	// Quota usage
	DailyDownloadUsed   int64 `json:"daily_download_used"`   // bytes
	DailyUploadUsed     int64 `json:"daily_upload_used"`     // bytes
	MonthlyDownloadUsed int64 `json:"monthly_download_used"` // bytes
	MonthlyUploadUsed   int64 `json:"monthly_upload_used"`   // bytes

	// Quotas from service
	DailyQuota        int64 `json:"daily_quota"`         // bytes (0 = unlimited)
	MonthlyQuota      int64 `json:"monthly_quota"`       // bytes (0 = unlimited)
	MonthlyBonusQuota int64 `json:"monthly_bonus_quota"` // bytes, total bonus purchased
	MonthlyBonusUsed  int64 `json:"monthly_bonus_used"`  // bytes, how much bonus consumed

	// Pricing
	Price         float64 `json:"price"`
	OverridePrice bool    `json:"override_price"`

	// Wallet
	Balance float64 `json:"balance"`

	// Connection status
	IsOnline   bool       `json:"is_online"`
	LastSeen   *time.Time `json:"last_seen"`
	IPAddress  string     `json:"ip_address"`
	MACAddress string     `json:"mac_address"`
}

// CustomerSession represents a customer session
type CustomerSession struct {
	SessionID       string     `json:"session_id"`
	StartTime       *time.Time `json:"start_time"`
	Duration        int        `json:"duration"` // seconds
	IPAddress       string     `json:"ip_address"`
	MACAddress      string     `json:"mac_address"`
	BytesIn         int64      `json:"bytes_in"`
	BytesOut        int64      `json:"bytes_out"`
	NasIPAddress    string     `json:"nas_ip_address"`
}

// Login authenticates a customer using PPPoE credentials
func (h *CustomerPortalHandler) Login(c *fiber.Ctx) error {
	var req CustomerLoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(CustomerLoginResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	if req.Username == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(CustomerLoginResponse{
			Success: false,
			Message: "Username and password are required",
		})
	}

	// Find subscriber by username
	var subscriber models.Subscriber
	if err := database.DB.Preload("Service").Where("username = ?", req.Username).First(&subscriber).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(CustomerLoginResponse{
			Success: false,
			Message: "Invalid username or password",
		})
	}

	// Verify password against radcheck table (Cleartext-Password)
	var radcheck models.RadCheck
	if err := database.DB.Where("username = ? AND attribute = ?", req.Username, "Cleartext-Password").First(&radcheck).Error; err != nil {
		// Try checking against subscriber's encrypted password
		plainPassword := security.DecryptPassword(subscriber.PasswordPlain)
		if plainPassword != req.Password {
			return c.Status(fiber.StatusUnauthorized).JSON(CustomerLoginResponse{
				Success: false,
				Message: "Invalid username or password",
			})
		}
	} else if radcheck.Value != req.Password {
		return c.Status(fiber.StatusUnauthorized).JSON(CustomerLoginResponse{
			Success: false,
			Message: "Invalid username or password",
		})
	}

	// Generate JWT token for customer
	token, err := h.generateCustomerToken(subscriber.Username)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(CustomerLoginResponse{
			Success: false,
			Message: "Failed to generate token",
		})
	}

	// Calculate days left
	daysLeft := 0
	if subscriber.ExpiryDate.After(time.Now()) {
		daysLeft = int(time.Until(subscriber.ExpiryDate).Hours() / 24)
	}

	// Get status string
	status := "active"
	switch subscriber.Status {
	case models.SubscriberStatusInactive:
		status = "inactive"
	case models.SubscriberStatusExpired:
		status = "expired"
	case models.SubscriberStatusStopped:
		status = "stopped"
	}

	return c.JSON(CustomerLoginResponse{
		Success: true,
		Token:   token,
		Customer: &CustomerInfo{
			Username:    subscriber.Username,
			FullName:    subscriber.FullName,
			Email:       subscriber.Email,
			Phone:       subscriber.Phone,
			ServiceName: subscriber.Service.Name,
			Status:      status,
			ExpiryDate:  subscriber.ExpiryDate,
			DaysLeft:    daysLeft,
		},
	})
}

// Dashboard returns customer dashboard data
func (h *CustomerPortalHandler) Dashboard(c *fiber.Ctx) error {
	username := c.Locals("customer_username").(string)

	var subscriber models.Subscriber
	if err := database.DB.Preload("Service").Where("username = ?", username).First(&subscriber).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber not found",
		})
	}

	// Calculate days left
	daysLeft := 0
	if subscriber.ExpiryDate.After(time.Now()) {
		daysLeft = int(time.Until(subscriber.ExpiryDate).Hours() / 24)
	}

	// Get status string
	status := "active"
	switch subscriber.Status {
	case models.SubscriberStatusInactive:
		status = "inactive"
	case models.SubscriberStatusExpired:
		status = "expired"
	case models.SubscriberStatusStopped:
		status = "stopped"
	}

	// Current speed — speeds are already stored in kb format
	currentDownload := subscriber.Service.DownloadSpeed
	currentUpload := subscriber.Service.UploadSpeed

	// Daily FUP speed
	var dailyDL, dailyUL int64
	switch subscriber.FUPLevel {
	case 1:
		dailyDL, dailyUL = subscriber.Service.FUP1DownloadSpeed, subscriber.Service.FUP1UploadSpeed
	case 2:
		dailyDL, dailyUL = subscriber.Service.FUP2DownloadSpeed, subscriber.Service.FUP2UploadSpeed
	case 3:
		dailyDL, dailyUL = subscriber.Service.FUP3DownloadSpeed, subscriber.Service.FUP3UploadSpeed
	}

	// Monthly FUP speed
	var monthlyDL, monthlyUL int64
	switch subscriber.MonthlyFUPLevel {
	case 1:
		monthlyDL, monthlyUL = subscriber.Service.MonthlyFUP1DownloadSpeed, subscriber.Service.MonthlyFUP1UploadSpeed
	case 2:
		monthlyDL, monthlyUL = subscriber.Service.MonthlyFUP2DownloadSpeed, subscriber.Service.MonthlyFUP2UploadSpeed
	case 3:
		monthlyDL, monthlyUL = subscriber.Service.MonthlyFUP3DownloadSpeed, subscriber.Service.MonthlyFUP3UploadSpeed
	case 4:
		monthlyDL, monthlyUL = subscriber.Service.MonthlyFUP4DownloadSpeed, subscriber.Service.MonthlyFUP4UploadSpeed
	case 5:
		monthlyDL, monthlyUL = subscriber.Service.MonthlyFUP5DownloadSpeed, subscriber.Service.MonthlyFUP5UploadSpeed
	case 6:
		monthlyDL, monthlyUL = subscriber.Service.MonthlyFUP6DownloadSpeed, subscriber.Service.MonthlyFUP6UploadSpeed
	}

	// Pick the effective FUP speed (slower of daily vs monthly, matching quota_sync logic)
	if dailyDL > 0 && monthlyDL > 0 {
		if monthlyDL < dailyDL {
			currentDownload, currentUpload = monthlyDL, monthlyUL
		} else {
			currentDownload, currentUpload = dailyDL, dailyUL
		}
	} else if dailyDL > 0 {
		currentDownload, currentUpload = dailyDL, dailyUL
	} else if monthlyDL > 0 {
		currentDownload, currentUpload = monthlyDL, monthlyUL
	}

	// Determine effective price (override_price takes precedence over service price)
	effectivePrice := subscriber.Service.Price
	if subscriber.OverridePrice && subscriber.Price > 0 {
		effectivePrice = subscriber.Price
	}

	// Get IP from active session
	ipAddress := subscriber.IPAddress
	var activeSession models.RadAcct
	if err := database.DB.Where("username = ? AND acctstoptime IS NULL", username).
		Order("acctstarttime DESC").First(&activeSession).Error; err == nil {
		ipAddress = activeSession.FramedIPAddress
	}

	dashboard := CustomerDashboard{
		Username:             subscriber.Username,
		FullName:             subscriber.FullName,
		Email:                subscriber.Email,
		Phone:                subscriber.Phone,
		Address:              subscriber.Address,
		ServiceName:          subscriber.Service.Name,
		Status:               status,
		ExpiryDate:           subscriber.ExpiryDate,
		DaysLeft:             daysLeft,
		DownloadSpeed:        subscriber.Service.DownloadSpeed,
		UploadSpeed:          subscriber.Service.UploadSpeed,
		CurrentDownloadSpeed: currentDownload,
		CurrentUploadSpeed:   currentUpload,
		FUPLevel:             subscriber.FUPLevel,
		MonthlyFUPLevel:      subscriber.MonthlyFUPLevel,
		DailyDownloadUsed:    subscriber.DailyDownloadUsed,
		DailyUploadUsed:      subscriber.DailyUploadUsed,
		MonthlyDownloadUsed:  subscriber.MonthlyDownloadUsed,
		MonthlyUploadUsed:    subscriber.MonthlyUploadUsed,
		DailyQuota:           subscriber.Service.DailyQuota,
		MonthlyQuota:         subscriber.Service.MonthlyQuota,
		MonthlyBonusQuota:    subscriber.MonthlyBonusQuota,
		MonthlyBonusUsed:     subscriber.MonthlyBonusUsed,
		Price:                effectivePrice,
		OverridePrice:        subscriber.OverridePrice,
		Balance:              subscriber.Balance,
		IsOnline:             subscriber.IsOnline,
		LastSeen:             subscriber.LastSeen,
		IPAddress:            ipAddress,
		MACAddress:           subscriber.MACAddress,
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    dashboard,
	})
}

// Sessions returns customer session history
func (h *CustomerPortalHandler) Sessions(c *fiber.Ctx) error {
	username := c.Locals("customer_username").(string)

	// Get recent sessions (last 30 days)
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)

	var sessions []models.RadAcct
	if err := database.DB.Where("username = ? AND acctstarttime >= ?", username, thirtyDaysAgo).
		Order("acctstarttime DESC").
		Limit(100).
		Find(&sessions).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch sessions",
		})
	}

	result := make([]CustomerSession, len(sessions))
	for i, s := range sessions {
		result[i] = CustomerSession{
			SessionID:    s.AcctSessionID,
			StartTime:    s.AcctStartTime,
			Duration:     s.AcctSessionTime,
			IPAddress:    s.FramedIPAddress,
			MACAddress:   s.CallingStationID,
			BytesIn:      s.AcctInputOctets,
			BytesOut:     s.AcctOutputOctets,
			NasIPAddress: s.NasIPAddress,
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

// UsageHistory returns daily usage history for the customer
func (h *CustomerPortalHandler) UsageHistory(c *fiber.Ctx) error {
	username := c.Locals("customer_username").(string)

	// Find subscriber for live counters and monthly usage
	var subscriber models.Subscriber
	if err := database.DB.Preload("Service").Where("username = ?", username).First(&subscriber).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber not found",
		})
	}

	// 1) Query daily_usage_history — the accurate source (saved periodically + at reset)
	type HistoryRow struct {
		Date          string `json:"date"`
		DownloadBytes int64  `json:"download_bytes"`
		UploadBytes   int64  `json:"upload_bytes"`
	}
	var historyRows []HistoryRow
	database.DB.Raw(`
		SELECT date::text, download_bytes, upload_bytes
		FROM daily_usage_history
		WHERE subscriber_id = ? AND date >= CURRENT_DATE - INTERVAL '30 days'
		ORDER BY date ASC
	`, subscriber.ID).Scan(&historyRows)

	// Build map for quick lookup
	historyMap := make(map[string]HistoryRow)
	for _, row := range historyRows {
		historyMap[row.Date] = row
	}

	// 2) Fallback: query radacct for days missing from history (grouped by session start date)
	type RadacctDay struct {
		Date     string `json:"date"`
		Download int64  `json:"download"`
		Upload   int64  `json:"upload"`
		Sessions int    `json:"sessions"`
	}
	var radacctDays []RadacctDay
	database.DB.Raw(`
		SELECT acctstarttime::date::text AS date,
		       COALESCE(SUM(acctoutputoctets), 0) AS download,
		       COALESCE(SUM(acctinputoctets), 0) AS upload,
		       COUNT(*) AS sessions
		FROM radacct
		WHERE username = ? AND acctstarttime >= CURRENT_DATE - INTERVAL '30 days'
		GROUP BY acctstarttime::date
		ORDER BY date ASC
	`, username).Scan(&radacctDays)

	radacctMap := make(map[string]RadacctDay)
	for _, rd := range radacctDays {
		radacctMap[rd.Date] = rd
	}

	// 3) Build daily array: history > radacct fallback, live counters for today
	type DailyUsage struct {
		Date     string `json:"date"`
		Download int64  `json:"download"`
		Upload   int64  `json:"upload"`
		Sessions int    `json:"sessions"`
	}

	today := time.Now().Format("2006-01-02")
	result := make([]DailyUsage, 0, 31)

	// Generate last 30 days
	for i := 29; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		if date == today {
			continue // handled separately below
		}
		if h, ok := historyMap[date]; ok {
			// Accurate source
			entry := DailyUsage{Date: h.Date, Download: h.DownloadBytes, Upload: h.UploadBytes}
			if rd, ok2 := radacctMap[date]; ok2 {
				entry.Sessions = rd.Sessions
			}
			result = append(result, entry)
		} else if rd, ok := radacctMap[date]; ok {
			// Fallback to radacct (better than empty)
			result = append(result, DailyUsage{Date: rd.Date, Download: rd.Download, Upload: rd.Upload, Sessions: rd.Sessions})
		}
	}

	// Today: use live counters from subscriber record (always accurate)
	todayEntry := DailyUsage{
		Date:     today,
		Download: subscriber.DailyDownloadUsed,
		Upload:   subscriber.DailyUploadUsed,
	}
	if rd, ok := radacctMap[today]; ok {
		todayEntry.Sessions = rd.Sessions
	}
	result = append(result, todayEntry)

	// Monthly quota info
	var monthlyQuota int64
	if subscriber.Service != nil {
		monthlyQuota = subscriber.Service.MonthlyQuota
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"daily": result,
			"monthly": fiber.Map{
				"download_used":  subscriber.MonthlyDownloadUsed,
				"upload_used":    subscriber.MonthlyUploadUsed,
				"download_quota": monthlyQuota,
			},
		},
	})
}

// generateCustomerToken generates a JWT token for customer portal
func (h *CustomerPortalHandler) generateCustomerToken(username string) (string, error) {
	claims := jwt.MapClaims{
		"customer_username": username,
		"type":              "customer",
		"exp":               time.Now().Add(24 * time.Hour).Unix(),
		"iat":               time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.JWTSecret))
}

// CustomerAuthMiddleware validates customer JWT token
func CustomerAuthMiddleware(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Authorization header required",
			})
		}

		// Extract token from "Bearer <token>"
		tokenString := ""
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenString = authHeader[7:]
		} else {
			tokenString = authHeader
		}

		// Parse and validate token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid token")
			}
			return []byte(cfg.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Invalid or expired token",
			})
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Invalid token claims",
			})
		}

		// Verify it's a customer token
		if claims["type"] != "customer" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Invalid token type",
			})
		}

		// Set customer username in context
		c.Locals("customer_username", claims["customer_username"])

		return c.Next()
	}
}

// CustomerTicket represents a ticket for customer view
type CustomerTicket struct {
	ID            uint      `json:"id"`
	TicketNumber  string    `json:"ticket_number"`
	Subject       string    `json:"subject"`
	Description   string    `json:"description"`
	Status        string    `json:"status"`
	Priority      string    `json:"priority"`
	Category      string    `json:"category"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	RepliesCount  int       `json:"replies_count"`
	HasAdminReply bool      `json:"has_admin_reply"`
}

// CustomerTicketReply represents a reply for customer view (excludes internal notes)
type CustomerTicketReply struct {
	ID        uint      `json:"id"`
	Message   string    `json:"message"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
}

// ListTickets returns customer's tickets
func (h *CustomerPortalHandler) ListTickets(c *fiber.Ctx) error {
	username := c.Locals("customer_username").(string)

	// Find subscriber
	var subscriber models.Subscriber
	if err := database.DB.Where("username = ?", username).First(&subscriber).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber not found",
		})
	}

	// Get tickets for this subscriber
	var tickets []models.Ticket
	if err := database.DB.Where("subscriber_id = ?", subscriber.ID).
		Order("created_at DESC").
		Find(&tickets).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch tickets",
		})
	}

	// Convert to customer view
	result := make([]CustomerTicket, len(tickets))
	for i, t := range tickets {
		// Count non-internal replies
		var repliesCount int64
		database.DB.Model(&models.TicketReply{}).Where("ticket_id = ? AND is_internal = false", t.ID).Count(&repliesCount)

		// Check if last reply is from admin (UserID > 0)
		var lastReply models.TicketReply
		hasAdminReply := false
		if err := database.DB.Where("ticket_id = ? AND is_internal = false", t.ID).Order("created_at DESC").First(&lastReply).Error; err == nil {
			hasAdminReply = lastReply.UserID > 0
		}

		result[i] = CustomerTicket{
			ID:            t.ID,
			TicketNumber:  t.TicketNumber,
			Subject:       t.Subject,
			Description:   t.Description,
			Status:        t.Status,
			Priority:      t.Priority,
			Category:      t.Category,
			CreatedAt:     t.CreatedAt,
			UpdatedAt:     t.UpdatedAt,
			RepliesCount:  int(repliesCount),
			HasAdminReply: hasAdminReply,
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

// GetTicket returns a single ticket with replies (excluding internal notes)
func (h *CustomerPortalHandler) GetTicket(c *fiber.Ctx) error {
	username := c.Locals("customer_username").(string)
	ticketID := c.Params("id")

	// Find subscriber
	var subscriber models.Subscriber
	if err := database.DB.Where("username = ?", username).First(&subscriber).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber not found",
		})
	}

	// Get ticket
	var ticket models.Ticket
	if err := database.DB.Where("id = ? AND subscriber_id = ?", ticketID, subscriber.ID).
		First(&ticket).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Ticket not found",
		})
	}

	// Get non-internal replies
	var replies []models.TicketReply
	database.DB.Where("ticket_id = ? AND is_internal = false", ticket.ID).
		Order("created_at ASC").
		Find(&replies)

	// Convert replies to customer view
	replyResults := make([]CustomerTicketReply, len(replies))
	for i, r := range replies {
		replyResults[i] = CustomerTicketReply{
			ID:        r.ID,
			Message:   r.Message,
			IsAdmin:   r.UserID > 0, // If UserID is set, it's from admin/staff
			CreatedAt: r.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"id":            ticket.ID,
			"ticket_number": ticket.TicketNumber,
			"subject":       ticket.Subject,
			"description":   ticket.Description,
			"status":        ticket.Status,
			"priority":      ticket.Priority,
			"category":      ticket.Category,
			"created_at":    ticket.CreatedAt,
			"updated_at":    ticket.UpdatedAt,
			"replies":       replyResults,
		},
	})
}

// CreateTicket creates a new ticket from customer
func (h *CustomerPortalHandler) CreateTicket(c *fiber.Ctx) error {
	username := c.Locals("customer_username").(string)

	// Find subscriber
	var subscriber models.Subscriber
	if err := database.DB.Where("username = ?", username).First(&subscriber).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber not found",
		})
	}

	type CreateRequest struct {
		Subject     string `json:"subject"`
		Description string `json:"description"`
		Priority    string `json:"priority"`
		Category    string `json:"category"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if req.Subject == "" || req.Description == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Subject and description are required",
		})
	}

	// Default values
	if req.Priority == "" {
		req.Priority = "normal"
	}
	if req.Category == "" {
		req.Category = "general"
	}

	// Generate ticket number
	ticketNumber := fmt.Sprintf("TKT-%d-%04d", time.Now().Year(), time.Now().Unix()%10000)

	ticket := models.Ticket{
		TicketNumber: ticketNumber,
		Subject:      req.Subject,
		Description:  req.Description,
		Message:      req.Description,
		Priority:     req.Priority,
		Category:     req.Category,
		Status:       "open",
		CreatorType:  "subscriber",
		SubscriberID: &subscriber.ID,
	}

	if err := database.DB.Create(&ticket).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create ticket",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": CustomerTicket{
			ID:           ticket.ID,
			TicketNumber: ticket.TicketNumber,
			Subject:      ticket.Subject,
			Description:  ticket.Description,
			Status:       ticket.Status,
			Priority:     ticket.Priority,
			Category:     ticket.Category,
			CreatedAt:    ticket.CreatedAt,
			UpdatedAt:    ticket.UpdatedAt,
			RepliesCount: 0,
		},
	})
}

// ReplyTicket adds a reply to a ticket from customer
func (h *CustomerPortalHandler) ReplyTicket(c *fiber.Ctx) error {
	username := c.Locals("customer_username").(string)
	ticketID := c.Params("id")

	// Find subscriber
	var subscriber models.Subscriber
	if err := database.DB.Where("username = ?", username).First(&subscriber).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber not found",
		})
	}

	// Get ticket and verify ownership
	var ticket models.Ticket
	if err := database.DB.Where("id = ? AND subscriber_id = ?", ticketID, subscriber.ID).
		First(&ticket).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Ticket not found",
		})
	}

	type ReplyRequest struct {
		Message string `json:"message"`
	}

	var req ReplyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if req.Message == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Message is required",
		})
	}

	// Create reply (UserID = 0 indicates customer reply)
	reply := models.TicketReply{
		TicketID:   ticket.ID,
		UserID:     0, // Customer reply
		Message:    req.Message,
		IsInternal: false,
	}

	if err := database.DB.Create(&reply).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to add reply",
		})
	}

	// If ticket was closed, reopen it
	if ticket.Status == "closed" || ticket.Status == "resolved" {
		database.DB.Model(&ticket).Update("status", "open")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": CustomerTicketReply{
			ID:        reply.ID,
			Message:   reply.Message,
			IsAdmin:   false,
			CreatedAt: reply.CreatedAt,
		},
	})
}

// Invoices returns invoices for the logged-in customer
func (h *CustomerPortalHandler) Invoices(c *fiber.Ctx) error {
	username := c.Locals("customer_username").(string)

	var subscriber models.Subscriber
	if err := database.DB.Where("username = ?", username).First(&subscriber).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber not found",
		})
	}

	var invoices []models.Invoice
	database.DB.Where("subscriber_id = ?", subscriber.ID).
		Order("created_at DESC").
		Find(&invoices)

	// Load items manually (gorm:"-" prevents Preload)
	for i := range invoices {
		var items []models.InvoiceItem
		database.DB.Where("invoice_id = ?", invoices[i].ID).Find(&items)
		invoices[i].Items = items
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    invoices,
	})
}

// GetInvoice returns a single invoice for the logged-in customer
func (h *CustomerPortalHandler) GetInvoice(c *fiber.Ctx) error {
	username := c.Locals("customer_username").(string)
	invoiceID := c.Params("id")

	var subscriber models.Subscriber
	if err := database.DB.Where("username = ?", username).First(&subscriber).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber not found",
		})
	}

	var invoice models.Invoice
	if err := database.DB.Where("id = ? AND subscriber_id = ?", invoiceID, subscriber.ID).
		First(&invoice).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Invoice not found",
		})
	}

	// Load items and subscriber manually
	var items []models.InvoiceItem
	database.DB.Where("invoice_id = ?", invoice.ID).Find(&items)
	invoice.Items = items
	invoice.Subscriber = subscriber

	return c.JSON(fiber.Map{
		"success": true,
		"data":    invoice,
	})
}

// Transactions returns wallet transaction history for the logged-in customer
func (h *CustomerPortalHandler) Transactions(c *fiber.Ctx) error {
	username := c.Locals("customer_username").(string)

	var subscriber models.Subscriber
	if err := database.DB.Where("username = ?", username).First(&subscriber).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber not found",
		})
	}

	page := 1
	limit := 50
	if p := c.QueryInt("page", 1); p > 0 {
		page = p
	}
	if l := c.QueryInt("limit", 50); l > 0 && l <= 100 {
		limit = l
	}
	offset := (page - 1) * limit

	var total int64
	database.DB.Model(&models.Transaction{}).
		Where("subscriber_id = ? AND type IN (?, ?)", subscriber.ID,
			models.TransactionTypeSubscriberTopup,
			models.TransactionTypeSubscriberPurchase).
		Count(&total)

	var transactions []models.Transaction
	database.DB.Where("subscriber_id = ? AND type IN (?, ?)", subscriber.ID,
		models.TransactionTypeSubscriberTopup,
		models.TransactionTypeSubscriberPurchase).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&transactions)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    transactions,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

// AvailableServices returns services available for the customer to switch to
func (h *CustomerPortalHandler) AvailableServices(c *fiber.Ctx) error {
	username := c.Locals("customer_username").(string)

	var subscriber models.Subscriber
	if err := database.DB.Preload("Service").Where("username = ?", username).First(&subscriber).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	// Get all active services (optionally filtered by reseller's allowed services)
	var services []models.Service
	query := database.DB.Where("is_active = ? AND deleted_at IS NULL", true).Order("price ASC")

	// If subscriber belongs to a reseller, filter to reseller's allowed services
	if subscriber.ResellerID > 0 {
		var allowedIDs []uint
		database.DB.Model(&models.ResellerService{}).Where("reseller_id = ?", subscriber.ResellerID).Pluck("service_id", &allowedIDs)
		if len(allowedIDs) > 0 {
			query = query.Where("id IN ?", allowedIDs)
		}
	}

	query.Find(&services)

	type ServiceOption struct {
		ID             uint    `json:"id"`
		Name           string  `json:"name"`
		DownloadSpeed  int64   `json:"download_speed"`
		UploadSpeed    int64   `json:"upload_speed"`
		Price          float64 `json:"price"`
		DailyQuota     int64   `json:"daily_quota"`
		MonthlyQuota   int64   `json:"monthly_quota"`
		IsCurrent      bool    `json:"is_current"`
	}

	result := make([]ServiceOption, len(services))
	for i, s := range services {
		result[i] = ServiceOption{
			ID:            s.ID,
			Name:          s.Name,
			DownloadSpeed: s.DownloadSpeed,
			UploadSpeed:   s.UploadSpeed,
			Price:         s.Price,
			DailyQuota:    s.DailyQuota,
			MonthlyQuota:  s.MonthlyQuota,
			IsCurrent:     s.ID == subscriber.ServiceID,
		}
	}

	// Check if change plan is enabled (global + reseller)
	changeEnabled := getSystemPreferenceBool("customer_change_service", true)
	if changeEnabled && subscriber.ResellerID > 0 {
		var reseller models.Reseller
		if err := database.DB.First(&reseller, subscriber.ResellerID).Error; err == nil {
			changeEnabled = reseller.CustomerChangePlan
		}
	}

	return c.JSON(fiber.Map{
		"success":                true,
		"data":                   result,
		"balance":                subscriber.Balance,
		"current_service_id":     subscriber.ServiceID,
		"change_service_enabled": changeEnabled,
		"allow_downgrade":        getSystemPreferenceBool("allow_downgrade", true),
	})
}

// ChangeService allows a customer to change their service plan if they have enough balance
func (h *CustomerPortalHandler) ChangeService(c *fiber.Ctx) error {
	username := c.Locals("customer_username").(string)

	var req struct {
		ServiceID uint `json:"service_id"`
	}
	if err := c.BodyParser(&req); err != nil || req.ServiceID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Service ID is required"})
	}

	var subscriber models.Subscriber
	if err := database.DB.Preload("Service").Where("username = ?", username).First(&subscriber).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	// Check if self-service plan change is enabled globally
	allowSelfChange := getSystemPreferenceBool("customer_change_service", true)
	if !allowSelfChange {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": "Plan changes are disabled. Contact your provider."})
	}

	// Check if reseller enabled it for their subscribers
	if subscriber.ResellerID > 0 {
		var reseller models.Reseller
		if err := database.DB.First(&reseller, subscriber.ResellerID).Error; err == nil {
			if !reseller.CustomerChangePlan {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": "Plan changes are disabled. Contact your provider."})
			}
		}
	}

	if req.ServiceID == subscriber.ServiceID {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Already on this plan"})
	}

	var newService models.Service
	if err := database.DB.First(&newService, req.ServiceID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Service not found"})
	}

	// Check upgrade/downgrade rules
	isDowngrade := newService.Price < subscriber.Service.Price
	isUpgrade := newService.Price > subscriber.Service.Price

	if isDowngrade && !getSystemPreferenceBool("allow_downgrade", true) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": "Downgrade is not allowed. Contact your provider."})
	}

	// Calculate prorated cost based on remaining days
	now := time.Now()
	remainingDays := 0
	if subscriber.ExpiryDate.After(now) {
		remainingDays = int(subscriber.ExpiryDate.Sub(now).Hours()/24) + 1 // +1 to include today
	}
	if remainingDays > 30 {
		remainingDays = 30
	}

	oldDailyPrice := subscriber.Service.Price / 30.0
	newDailyPrice := newService.Price / 30.0
	dailyDiff := newDailyPrice - oldDailyPrice
	proratedAmount := dailyDiff * float64(remainingDays)
	// Round to 2 decimals
	proratedAmount = math.Round(proratedAmount*100) / 100

	chargeAmount := 0.0  // positive = deduct from balance
	refundAmount := 0.0  // positive = add to balance

	if isUpgrade {
		chargeAmount = proratedAmount // prorated upgrade cost
		upgradeFee := getSystemPreferenceFloat("upgrade_change_service_fee", 0)
		if upgradeFee > 0 {
			chargeAmount += upgradeFee
		}
	} else if isDowngrade {
		refundEnabled := getSystemPreferenceBool("downgrade_refund", false)
		if refundEnabled {
			refundAmount = -proratedAmount // proratedAmount is negative for downgrade, so negate
		}
		downgradeFee := getSystemPreferenceFloat("downgrade_change_service_fee", 0)
		if downgradeFee > 0 {
			// Fee reduces the refund or becomes a charge
			if refundAmount >= downgradeFee {
				refundAmount -= downgradeFee
			} else {
				chargeAmount = downgradeFee - refundAmount
				refundAmount = 0
			}
		}
	}

	// Check balance for upgrade
	if chargeAmount > 0 && subscriber.Balance < chargeAmount {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Insufficient balance. Need $%.2f more (prorated %d days). Current balance: $%.2f", chargeAmount-subscriber.Balance, remainingDays, subscriber.Balance),
		})
	}

	// Calculate new balance
	newBalance := subscriber.Balance - chargeAmount + refundAmount
	newBalance = math.Round(newBalance*100) / 100

	// Apply change
	oldServiceName := subscriber.Service.Name
	updateFields := map[string]interface{}{
		"service_id": req.ServiceID,
		"price":      newService.Price,
		"balance":    newBalance,
	}

	if err := database.DB.Model(&models.Subscriber{}).Where("id = ?", subscriber.ID).Updates(updateFields).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to change service"})
	}

	// Update RADIUS attributes (speed limits)
	database.DB.Where("username = ? AND attribute = ?", username, "Mikrotik-Rate-Limit").Delete(&models.RadReply{})
	rateLimit := fmt.Sprintf("%dk/%dk", newService.UploadSpeed, newService.DownloadSpeed)
	database.DB.Create(&models.RadReply{
		Username:  username,
		Attribute: "Mikrotik-Rate-Limit",
		Op:        ":=",
		Value:     rateLimit,
	})

	// Delete old Framed-IP-Address if pool changed (let RADIUS assign new IP on reconnect)
	if newService.PoolName != subscriber.Service.PoolName {
		database.DB.Where("username = ? AND attribute = ?", username, "Framed-IP-Address").Delete(&models.RadReply{})
	}

	// Update Framed-Pool if service has pool
	if newService.PoolName != "" {
		database.DB.Where("username = ? AND attribute = ?", username, "Framed-Pool").Delete(&models.RadReply{})
		database.DB.Create(&models.RadReply{
			Username:  username,
			Attribute: "Framed-Pool",
			Op:        ":=",
			Value:     newService.PoolName,
		})
	}

	// Create transaction record
	subID := subscriber.ID
	if chargeAmount > 0 {
		database.DB.Create(&models.Transaction{
			SubscriberID:   &subID,
			Amount:         -chargeAmount,
			BalanceBefore:  subscriber.Balance,
			BalanceAfter:   newBalance,
			Type:           "service_change",
			Description:    fmt.Sprintf("Upgrade: %s → %s (prorated %d days, $%.2f)", oldServiceName, newService.Name, remainingDays, chargeAmount),
			OldServiceName: oldServiceName,
			NewServiceName: newService.Name,
		})
	}
	if refundAmount > 0 {
		database.DB.Create(&models.Transaction{
			SubscriberID:   &subID,
			Amount:         refundAmount,
			BalanceBefore:  subscriber.Balance,
			BalanceAfter:   newBalance,
			Type:           "service_change",
			Description:    fmt.Sprintf("Downgrade refund: %s → %s (prorated %d days, $%.2f)", oldServiceName, newService.Name, remainingDays, refundAmount),
			OldServiceName: oldServiceName,
			NewServiceName: newService.Name,
		})
	}

	return c.JSON(fiber.Map{
		"success":       true,
		"message":       fmt.Sprintf("Plan changed to %s", newService.Name),
		"new_balance":   newBalance,
		"charged":       chargeAmount,
		"refunded":      refundAmount,
		"prorated_days": remainingDays,
	})
}

// TopUpData allows a customer to buy extra GB from their balance
func (h *CustomerPortalHandler) TopUpData(c *fiber.Ctx) error {
	username := c.Locals("customer_username").(string)

	var req struct {
		GB int `json:"gb"`
	}
	if err := c.BodyParser(&req); err != nil || req.GB <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "GB amount must be positive"})
	}

	var subscriber models.Subscriber
	if err := database.DB.Preload("Service").Where("username = ?", username).First(&subscriber).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	// Get price per GB from settings
	pricePerGB := getSystemPreferenceFloat("topup_data_price_per_gb", 0)
	if pricePerGB <= 0 {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": "Data top-up is not available. Contact your provider."})
	}

	totalCost := pricePerGB * float64(req.GB)
	totalCost = math.Round(totalCost*100) / 100

	// Check balance
	if subscriber.Balance < totalCost {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Insufficient balance. Need $%.2f for %d GB. Current balance: $%.2f", totalCost, req.GB, subscriber.Balance),
		})
	}

	// Add bonus quota (GB to bytes)
	bonusBytes := int64(req.GB) * 1024 * 1024 * 1024
	newBalance := math.Round((subscriber.Balance-totalCost)*100) / 100

	database.DB.Model(&models.Subscriber{}).Where("id = ?", subscriber.ID).Updates(map[string]interface{}{
		"monthly_bonus_quota": gorm.Expr("monthly_bonus_quota + ?", bonusBytes),
		"monthly_bonus_used":  0, // Reset — new bonus starts fresh
		"balance":             newBalance,
		"monthly_fup_level":   0,
	})

	// Record in bonus_topups history
	database.DB.Create(&models.BonusTopUp{
		SubscriberID: subscriber.ID,
		GB:           req.GB,
		Bytes:        bonusBytes,
		PricePerGB:   pricePerGB,
		TotalCost:    totalCost,
		Source:       "customer",
		CreatedBy:    username,
	})

	// Transaction
	subID := subscriber.ID
	database.DB.Create(&models.Transaction{
		SubscriberID:   &subID,
		Amount:         -totalCost,
		BalanceBefore:  subscriber.Balance,
		BalanceAfter:   newBalance,
		Type:           "data_topup",
		Description:    fmt.Sprintf("Data top-up: %d GB ($%.2f/GB)", req.GB, pricePerGB),
	})

	return c.JSON(fiber.Map{
		"success":     true,
		"message":     fmt.Sprintf("Added %d GB. Speed will restore within 30 seconds.", req.GB),
		"new_balance": newBalance,
		"gb_added":    req.GB,
		"charged":     totalCost,
	})
}

// GetTopUpInfo returns top-up pricing and current bonus quota
func (h *CustomerPortalHandler) GetTopUpInfo(c *fiber.Ctx) error {
	username := c.Locals("customer_username").(string)

	var subscriber models.Subscriber
	if err := database.DB.Where("username = ?", username).First(&subscriber).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	pricePerGB := getSystemPreferenceFloat("topup_data_price_per_gb", 0)

	return c.JSON(fiber.Map{
		"success":       true,
		"price_per_gb":       pricePerGB,
		"enabled":            pricePerGB > 0,
		"balance":            subscriber.Balance,
		"bonus_quota_gb":     float64(subscriber.MonthlyBonusQuota) / 1024 / 1024 / 1024,
		"bonus_used_gb":      float64(subscriber.MonthlyBonusUsed) / 1024 / 1024 / 1024,
		"bonus_remaining_gb": float64(subscriber.MonthlyBonusQuota-subscriber.MonthlyBonusUsed) / 1024 / 1024 / 1024,
	})
}

// GetBonusHistory returns the subscriber's data top-up purchase history
func (h *CustomerPortalHandler) GetBonusHistory(c *fiber.Ctx) error {
	username := c.Locals("customer_username").(string)

	var subscriber models.Subscriber
	if err := database.DB.Where("username = ?", username).First(&subscriber).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	var topups []models.BonusTopUp
	database.DB.Where("subscriber_id = ?", subscriber.ID).Order("created_at DESC").Limit(50).Find(&topups)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    topups,
	})
}
