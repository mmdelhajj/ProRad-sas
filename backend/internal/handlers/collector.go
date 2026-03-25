package handlers

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/middleware"
	"github.com/proisp/backend/internal/models"
	"gorm.io/gorm"
)

// CollectorHandler handles collector-related requests
type CollectorHandler struct{}

// NewCollectorHandler creates a new collector handler
func NewCollectorHandler() *CollectorHandler {
	return &CollectorHandler{}
}

// ---- Admin/Reseller endpoints ----

// ListCollectors returns all collectors with assignment stats
func (h *CollectorHandler) ListCollectors(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)

	var collectors []models.User
	query := database.DB.Where("user_type = ?", models.UserTypeCollector)

	// Resellers only see collectors they created (by reseller_id match)
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		query = query.Where("reseller_id = ?", *user.ResellerID)
	}

	if err := query.Find(&collectors).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch collectors",
		})
	}

	type CollectorWithStats struct {
		models.User
		TotalAssigned  int64   `json:"total_assigned"`
		TotalCollected int64   `json:"total_collected"`
		TotalPending   int64   `json:"total_pending"`
		TotalAmount    float64 `json:"total_amount"`
	}

	// Batch-fetch all collector stats in one query (avoid N+1)
	collectorIDs := make([]uint, len(collectors))
	for i, col := range collectors {
		collectorIDs[i] = col.ID
	}

	type batchStats struct {
		CollectorID    uint    `gorm:"column:collector_id"`
		TotalAssigned  int64   `gorm:"column:total_assigned"`
		TotalCollected int64   `gorm:"column:total_collected"`
		TotalPending   int64   `gorm:"column:total_pending"`
		TotalAmount    float64 `gorm:"column:total_amount"`
	}

	var statsList []batchStats
	if len(collectorIDs) > 0 {
		database.DB.Model(&models.CollectionAssignment{}).
			Select(`collector_id,
				COUNT(*) as total_assigned,
				COUNT(CASE WHEN status = 'collected' THEN 1 END) as total_collected,
				COUNT(CASE WHEN status = 'pending' THEN 1 END) as total_pending,
				COALESCE(SUM(CASE WHEN status = 'collected' THEN amount ELSE 0 END), 0) as total_amount`).
			Where("collector_id IN ?", collectorIDs).
			Group("collector_id").
			Scan(&statsList)
	}

	statsMap := make(map[uint]batchStats)
	for _, s := range statsList {
		statsMap[s.CollectorID] = s
	}

	var result []CollectorWithStats
	for _, col := range collectors {
		s := statsMap[col.ID]
		result = append(result, CollectorWithStats{
			User:           col,
			TotalAssigned:  s.TotalAssigned,
			TotalCollected: s.TotalCollected,
			TotalPending:   s.TotalPending,
			TotalAmount:    s.TotalAmount,
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

// GetCollector returns a single collector with stats
func (h *CollectorHandler) GetCollector(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid collector ID"})
	}

	var collector models.User
	if err := database.DB.Where("id = ? AND user_type = ?", id, models.UserTypeCollector).First(&collector).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Collector not found"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    collector,
	})
}

// GetCollectorAssignments returns assignments for a specific collector
func (h *CollectorHandler) GetCollectorAssignments(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid collector ID"})
	}

	status := c.Query("status", "")

	var assignments []models.CollectionAssignment
	query := database.DB.Where("collector_id = ?", id)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	query = query.Order("created_at DESC")

	if err := query.Find(&assignments).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch assignments",
		})
	}

	h.loadAssignmentRelations(assignments)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    assignments,
	})
}

// CreateAssignment creates new collection assignments
func (h *CollectorHandler) CreateAssignment(c *fiber.Ctx) error {
	var req struct {
		CollectorID      uint   `json:"collector_id"`
		SubscriberIDs    []uint `json:"subscriber_ids"`
		AutoRenew        bool   `json:"auto_renew"`
		SendNotification bool   `json:"send_notification"`
		Notes            string `json:"notes"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	if req.CollectorID == 0 || len(req.SubscriberIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Collector ID and at least one subscriber ID are required"})
	}

	// Verify collector exists and is type 5
	var collector models.User
	if err := database.DB.Where("id = ? AND user_type = ?", req.CollectorID, models.UserTypeCollector).First(&collector).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid collector"})
	}

	user := middleware.GetCurrentUser(c)
	created := 0

	for _, subID := range req.SubscriberIDs {
		var subscriber models.Subscriber
		if err := database.DB.Preload("Service").First(&subscriber, subID).Error; err != nil {
			continue
		}

		// Reseller can only assign their own subscribers
		if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
			var reseller models.Reseller
			if database.DB.First(&reseller, *user.ResellerID).Error == nil {
				if subscriber.ResellerID != reseller.ID {
					continue
				}
			}
		}

		// Get amount from subscriber's price (or service price)
		amount := subscriber.Price
		if amount == 0 && subscriber.Service != nil {
			amount = subscriber.Service.Price
		}

		// Find latest unpaid invoice for subscriber
		var invoice models.Invoice
		var invoiceID *uint
		if err := database.DB.Where("subscriber_id = ? AND status IN (?, ?)", subID, "pending", "partial").
			Order("created_at DESC").First(&invoice).Error; err == nil {
			invoiceID = &invoice.ID
			amount = invoice.Total - invoice.AmountPaid
		}

		assignment := models.CollectionAssignment{
			CollectorID:      req.CollectorID,
			SubscriberID:     subID,
			ResellerID:       subscriber.ResellerID,
			InvoiceID:        invoiceID,
			Status:           "pending",
			AutoRenew:        req.AutoRenew,
			SendNotification: req.SendNotification,
			Amount:           amount,
			Notes:            req.Notes,
			CreatedBy:        user.ID,
		}

		if err := database.DB.Create(&assignment).Error; err != nil {
			log.Printf("Failed to create assignment for subscriber %d: %v", subID, err)
			continue
		}
		created++
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Created %d assignment(s)", created),
	})
}

// DeleteAssignment cancels an assignment
func (h *CollectorHandler) DeleteAssignment(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid assignment ID"})
	}

	var assignment models.CollectionAssignment
	if err := database.DB.First(&assignment, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Assignment not found"})
	}

	if assignment.Status == "collected" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Cannot delete a collected assignment"})
	}

	assignment.Status = "cancelled"
	database.DB.Save(&assignment)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Assignment cancelled",
	})
}

// CollectorReport returns performance report for collectors
func (h *CollectorHandler) CollectorReport(c *fiber.Ctx) error {
	startDate := c.Query("start_date", time.Now().AddDate(0, -1, 0).Format("2006-01-02"))
	endDate := c.Query("end_date", time.Now().Format("2006-01-02"))
	user := middleware.GetCurrentUser(c)

	type ReportRow struct {
		CollectorID    uint    `json:"collector_id"`
		CollectorName  string  `json:"collector_name"`
		TotalAssigned  int64   `json:"total_assigned"`
		TotalCollected int64   `json:"total_collected"`
		TotalFailed    int64   `json:"total_failed"`
		TotalPending   int64   `json:"total_pending"`
		TotalAmount    float64 `json:"total_amount"`
		SuccessRate    float64 `json:"success_rate"`
	}

	// Get collectors
	var collectors []models.User
	collQuery := database.DB.Where("user_type = ?", models.UserTypeCollector)
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		collQuery = collQuery.Where("reseller_id = ?", *user.ResellerID)
	}
	collQuery.Find(&collectors)

	var report []ReportRow
	for _, col := range collectors {
		var row ReportRow
		row.CollectorID = col.ID
		row.CollectorName = col.FullName
		if row.CollectorName == "" {
			row.CollectorName = col.Username
		}

		baseQ := database.DB.Model(&models.CollectionAssignment{}).
			Where("collector_id = ? AND created_at >= ? AND created_at <= ?", col.ID, startDate, endDate+" 23:59:59")

		baseQ.Count(&row.TotalAssigned)

		database.DB.Model(&models.CollectionAssignment{}).
			Where("collector_id = ? AND status = ? AND collected_at >= ? AND collected_at <= ?", col.ID, "collected", startDate, endDate+" 23:59:59").
			Count(&row.TotalCollected)

		database.DB.Model(&models.CollectionAssignment{}).
			Where("collector_id = ? AND status = ? AND created_at >= ? AND created_at <= ?", col.ID, "failed", startDate, endDate+" 23:59:59").
			Count(&row.TotalFailed)

		database.DB.Model(&models.CollectionAssignment{}).
			Where("collector_id = ? AND status = ? AND created_at >= ? AND created_at <= ?", col.ID, "pending", startDate, endDate+" 23:59:59").
			Count(&row.TotalPending)

		var totalAmt *float64
		database.DB.Model(&models.CollectionAssignment{}).
			Where("collector_id = ? AND status = ? AND collected_at >= ? AND collected_at <= ?", col.ID, "collected", startDate, endDate+" 23:59:59").
			Select("COALESCE(SUM(amount), 0)").Scan(&totalAmt)
		if totalAmt != nil {
			row.TotalAmount = *totalAmt
		}

		if row.TotalAssigned > 0 {
			row.SuccessRate = float64(row.TotalCollected) / float64(row.TotalAssigned) * 100
		}

		report = append(report, row)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    report,
	})
}

// ---- Collector self-service endpoints ----

// MyDashboard returns stats for the currently logged-in collector
func (h *CollectorHandler) MyDashboard(c *fiber.Ctx) error {
	userID := middleware.GetCurrentUserID(c)

	var totalAssigned, totalCollected, collectedToday int64
	var totalAmount float64

	database.DB.Model(&models.CollectionAssignment{}).
		Where("collector_id = ? AND status = ?", userID, "pending").
		Count(&totalAssigned)

	todayStart := time.Now().Truncate(24 * time.Hour)
	database.DB.Model(&models.CollectionAssignment{}).
		Where("collector_id = ? AND status = ? AND collected_at >= ?", userID, "collected", todayStart).
		Count(&collectedToday)

	database.DB.Model(&models.CollectionAssignment{}).
		Where("collector_id = ? AND status = ?", userID, "collected").
		Count(&totalCollected)

	var amt *float64
	database.DB.Model(&models.CollectionAssignment{}).
		Where("collector_id = ? AND status = ?", userID, "collected").
		Select("COALESCE(SUM(amount), 0)").Scan(&amt)
	if amt != nil {
		totalAmount = *amt
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"pending_count":    totalAssigned,
			"collected_today":  collectedToday,
			"total_collected":  totalCollected,
			"total_amount":     totalAmount,
		},
	})
}

// MyAssignments returns assignments for the logged-in collector
func (h *CollectorHandler) MyAssignments(c *fiber.Ctx) error {
	userID := middleware.GetCurrentUserID(c)
	status := c.Query("status", "")

	var assignments []models.CollectionAssignment
	query := database.DB.Where("collector_id = ?", userID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	query = query.Order("CASE WHEN status = 'pending' THEN 0 ELSE 1 END, created_at DESC")

	if err := query.Find(&assignments).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch assignments",
		})
	}

	h.loadAssignmentRelations(assignments)

	// Strip sensitive subscriber data — collector only sees limited info
	type LimitedSubscriber struct {
		ID        uint    `json:"id"`
		FullName  string  `json:"full_name"`
		Phone     string  `json:"phone"`
		Address   string  `json:"address"`
		Region    string  `json:"region"`
		Building  string  `json:"building"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}

	type SafeAssignment struct {
		models.CollectionAssignment
		SubscriberInfo *LimitedSubscriber `json:"subscriber_info"`
	}

	var safe []SafeAssignment
	for _, a := range assignments {
		sa := SafeAssignment{CollectionAssignment: a}
		if a.Subscriber != nil {
			sa.SubscriberInfo = &LimitedSubscriber{
				ID:        a.Subscriber.ID,
				FullName:  a.Subscriber.FullName,
				Phone:     a.Subscriber.Phone,
				Address:   a.Subscriber.Address,
				Region:    a.Subscriber.Region,
				Building:  a.Subscriber.Building,
				Latitude:  a.Subscriber.Latitude,
				Longitude: a.Subscriber.Longitude,
			}
		}
		sa.Subscriber = nil // hide full subscriber
		safe = append(safe, sa)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    safe,
	})
}

// GetMyAssignment returns a single assignment for the logged-in collector
func (h *CollectorHandler) GetMyAssignment(c *fiber.Ctx) error {
	userID := middleware.GetCurrentUserID(c)
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid assignment ID"})
	}

	var assignment models.CollectionAssignment
	if err := database.DB.Where("id = ? AND collector_id = ?", id, userID).First(&assignment).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Assignment not found"})
	}

	h.loadAssignmentRelations([]models.CollectionAssignment{assignment})

	return c.JSON(fiber.Map{
		"success": true,
		"data":    assignment,
	})
}

// MarkCollected records a payment collection
func (h *CollectorHandler) MarkCollected(c *fiber.Ctx) error {
	userID := middleware.GetCurrentUserID(c)
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid assignment ID"})
	}

	var req struct {
		Amount    float64 `json:"amount"`
		Notes     string  `json:"notes"`
		Reference string  `json:"reference"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	var assignment models.CollectionAssignment
	if err := database.DB.Where("id = ? AND collector_id = ?", id, userID).First(&assignment).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Assignment not found"})
	}

	if assignment.Status != "pending" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Assignment is not pending"})
	}

	amount := req.Amount
	if amount <= 0 {
		amount = assignment.Amount
	}

	// 1. Create Payment record
	collectorID := userID
	payment := models.Payment{
		SubscriberID: assignment.SubscriberID,
		InvoiceID:    assignment.InvoiceID,
		ResellerID:   assignment.ResellerID,
		CollectorID:  &collectorID,
		Amount:       amount,
		Method:       "cash",
		Reference:    req.Reference,
		Notes:        req.Notes,
		Status:       models.PaymentStatusCompleted,
	}
	if err := database.DB.Create(&payment).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to create payment"})
	}

	// 2. Update invoice if linked
	if assignment.InvoiceID != nil {
		var invoice models.Invoice
		if database.DB.First(&invoice, *assignment.InvoiceID).Error == nil {
			invoice.AmountPaid += amount
			if invoice.AmountPaid >= invoice.Total {
				invoice.Status = models.PaymentStatusCompleted
				now := time.Now()
				invoice.PaidDate = &now
			}
			database.DB.Save(&invoice)
		}
	}

	// 3. Update assignment
	now := time.Now()
	assignment.Status = "collected"
	assignment.CollectedAt = &now
	assignment.PaymentID = &payment.ID
	assignment.Amount = amount
	if req.Notes != "" {
		assignment.Notes = req.Notes
	}
	database.DB.Save(&assignment)

	// 4. Auto-renew if enabled
	if assignment.AutoRenew {
		h.autoRenewSubscriber(assignment.SubscriberID)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Payment collected successfully",
		"data": fiber.Map{
			"assignment_id": assignment.ID,
			"payment_id":    payment.ID,
			"amount":        amount,
		},
	})
}

// MarkFailed marks a collection visit as failed
func (h *CollectorHandler) MarkFailed(c *fiber.Ctx) error {
	userID := middleware.GetCurrentUserID(c)
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid assignment ID"})
	}

	var req struct {
		Notes string `json:"notes"`
	}
	c.BodyParser(&req)

	var assignment models.CollectionAssignment
	if err := database.DB.Where("id = ? AND collector_id = ?", id, userID).First(&assignment).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Assignment not found"})
	}

	if assignment.Status != "pending" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Assignment is not pending"})
	}

	now := time.Now()
	assignment.Status = "failed"
	assignment.CollectedAt = &now // reuse as "resolved at" timestamp
	if req.Notes != "" {
		assignment.Notes = req.Notes
	}
	database.DB.Save(&assignment)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Assignment marked as failed",
	})
}

// ---- Helpers ----

func (h *CollectorHandler) loadAssignmentRelations(assignments []models.CollectionAssignment) {
	for i := range assignments {
		// Load subscriber (limited fields)
		var sub models.Subscriber
		if database.DB.Select("id, full_name, phone, address, region, building, latitude, longitude, service_id, reseller_id, expiry_date, status, price, override_price").
			First(&sub, assignments[i].SubscriberID).Error == nil {
			// Load service name
			var svc models.Service
			if database.DB.Select("id, name, price").First(&svc, sub.ServiceID).Error == nil {
				sub.Service = &svc
			}
			assignments[i].Subscriber = &sub
		}

		// Load invoice
		if assignments[i].InvoiceID != nil {
			var inv models.Invoice
			if database.DB.First(&inv, *assignments[i].InvoiceID).Error == nil {
				assignments[i].Invoice = &inv
			}
		}

		// Load collector name
		var col models.User
		if database.DB.Select("id, username, full_name").First(&col, assignments[i].CollectorID).Error == nil {
			assignments[i].Collector = &col
		}
	}
}

// autoRenewSubscriber extends the subscriber's expiry and resets FUP
func (h *CollectorHandler) autoRenewSubscriber(subscriberID uint) {
	var subscriber models.Subscriber
	if err := database.DB.Preload("Service").First(&subscriber, subscriberID).Error; err != nil {
		log.Printf("Collector autoRenew: subscriber %d not found: %v", subscriberID, err)
		return
	}

	if subscriber.Service == nil {
		log.Printf("Collector autoRenew: subscriber %d has no service", subscriberID)
		return
	}

	// Calculate new expiry
	var newExpiry time.Time
	if subscriber.ExpiryDate.After(time.Now()) {
		if subscriber.Service.ExpiryUnit == models.ExpiryUnitMonths {
			newExpiry = subscriber.ExpiryDate.AddDate(0, subscriber.Service.ExpiryValue, 0)
		} else {
			newExpiry = subscriber.ExpiryDate.AddDate(0, 0, subscriber.Service.ExpiryValue)
		}
	} else {
		if subscriber.Service.ExpiryUnit == models.ExpiryUnitMonths {
			newExpiry = time.Now().AddDate(0, subscriber.Service.ExpiryValue, 0)
		} else {
			newExpiry = time.Now().AddDate(0, 0, subscriber.Service.ExpiryValue)
		}
	}

	now := time.Now()
	subscriber.ExpiryDate = newExpiry
	subscriber.Status = models.SubscriberStatusActive
	subscriber.FUPLevel = 0
	subscriber.DailyDownloadUsed = 0
	subscriber.DailyUploadUsed = 0
	subscriber.DailyQuotaUsed = 0
	subscriber.LastDailyReset = &now
	subscriber.MonthlyFUPLevel = 0
	subscriber.MonthlyDownloadUsed = 0
	subscriber.MonthlyUploadUsed = 0
	subscriber.MonthlyQuotaUsed = 0
	subscriber.LastMonthlyReset = &now

	database.DB.Save(&subscriber)

	// Update RADIUS expiration
	database.DB.Where("username = ? AND attribute = ?", subscriber.Username, "Expiration").Delete(&models.RadCheck{})
	database.DB.Create(&models.RadCheck{
		Username:  subscriber.Username,
		Attribute: "Expiration",
		Op:        ":=",
		Value:     newExpiry.Format("Jan 02 2006 15:04:05"),
	})

	// Deduct reseller balance if applicable
	if subscriber.ResellerID > 0 {
		database.DB.Model(&models.Reseller{}).Where("id = ?", subscriber.ResellerID).
			Update("balance", gorm.Expr("balance - ?", subscriber.Price))

		// Create transaction
		database.DB.Create(&models.Transaction{
			Type:         models.TransactionTypeRenewal,
			Amount:       -subscriber.Price,
			Description:  fmt.Sprintf("Auto-renewal via collector for %s", subscriber.Username),
			ResellerID:   subscriber.ResellerID,
			SubscriberID: &subscriber.ID,
			ServiceName:  subscriber.Service.Name,
			CreatedBy:    subscriber.ResellerID,
		})
	}

	log.Printf("Collector autoRenew: renewed %s until %s", subscriber.Username, newExpiry.Format("2006-01-02"))
}
