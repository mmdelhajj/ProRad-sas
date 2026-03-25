package handlers

import (
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/middleware"
	"github.com/proisp/backend/internal/mikrotik"
	"github.com/proisp/backend/internal/models"
	"github.com/proisp/backend/internal/radius"
)

type InvoiceHandler struct{}

func NewInvoiceHandler() *InvoiceHandler {
	return &InvoiceHandler{}
}

// List returns all invoices
func (h *InvoiceHandler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 25)
	status := c.Query("status", "")
	subscriberID := c.QueryInt("subscriber_id", 0)

	if page < 1 {
		page = 1
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	query := database.DB.Model(&models.Invoice{})

	// Reseller filtering — resellers only see their own invoices
	user := middleware.GetCurrentUser(c)
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		query = query.Where("reseller_id = ?", *user.ResellerID)
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if subscriberID > 0 {
		query = query.Where("subscriber_id = ?", subscriberID)
	}

	var total int64
	query.Count(&total)

	var invoices []models.Invoice
	query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&invoices)

	// Batch-load items and subscribers (avoid N+1 queries)
	if len(invoices) > 0 {
		invoiceIDs := make([]uint, len(invoices))
		subscriberIDs := make([]uint, 0, len(invoices))
		for i, inv := range invoices {
			invoiceIDs[i] = inv.ID
			if inv.SubscriberID > 0 {
				subscriberIDs = append(subscriberIDs, inv.SubscriberID)
			}
		}

		var allItems []models.InvoiceItem
		database.DB.Where("invoice_id IN ?", invoiceIDs).Find(&allItems)
		itemsByInvoice := make(map[uint][]models.InvoiceItem)
		for _, item := range allItems {
			itemsByInvoice[item.InvoiceID] = append(itemsByInvoice[item.InvoiceID], item)
		}

		subByID := make(map[uint]models.Subscriber)
		if len(subscriberIDs) > 0 {
			var subs []models.Subscriber
			database.DB.Where("id IN ?", subscriberIDs).Find(&subs)
			for _, sub := range subs {
				subByID[sub.ID] = sub
			}
		}

		for i := range invoices {
			invoices[i].Items = itemsByInvoice[invoices[i].ID]
			if sub, ok := subByID[invoices[i].SubscriberID]; ok {
				invoices[i].Subscriber = sub
			}
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    invoices,
		"meta": fiber.Map{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// Get returns a single invoice
func (h *InvoiceHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")

	var invoice models.Invoice
	if err := database.DB.First(&invoice, "id = ?", id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Invoice not found",
		})
	}

	// Load items manually (gorm:"-" prevents Preload)
	var items []models.InvoiceItem
	database.DB.Where("invoice_id = ?", invoice.ID).Find(&items)
	invoice.Items = items

	// Load subscriber manually
	var subscriber models.Subscriber
	if database.DB.First(&subscriber, "id = ?", invoice.SubscriberID).Error == nil {
		invoice.Subscriber = subscriber
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    invoice,
	})
}

// Create creates a new invoice
func (h *InvoiceHandler) Create(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)

	type ItemRequest struct {
		Description string  `json:"description"`
		Quantity    int     `json:"quantity"`
		UnitPrice   float64 `json:"unit_price"`
	}

	type CreateRequest struct {
		SubscriberID uint          `json:"subscriber_id"`
		DueDate      string        `json:"due_date"`
		Notes        string        `json:"notes"`
		Items        []ItemRequest `json:"items"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Get subscriber to get reseller ID
	var subscriber models.Subscriber
	if err := database.DB.First(&subscriber, req.SubscriberID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber not found",
		})
	}

	// Generate invoice number
	invoiceNumber := fmt.Sprintf("INV-%d-%04d", time.Now().Year(), time.Now().Unix()%10000)

	// Calculate totals
	var subtotal float64
	for _, item := range req.Items {
		subtotal += item.UnitPrice * float64(item.Quantity)
	}

	dueDate, _ := time.Parse("2006-01-02", req.DueDate)
	if dueDate.IsZero() {
		dueDate = time.Now().AddDate(0, 0, 30)
	}

	resellerID := uint(1) // default
	if user.ResellerID != nil {
		resellerID = *user.ResellerID
	}

	invoice := models.Invoice{
		InvoiceNumber: invoiceNumber,
		SubscriberID:  req.SubscriberID,
		ResellerID:    resellerID,
		SubTotal:      subtotal,
		Tax:           0,
		Total:         subtotal,
		AmountPaid:    0,
		Status:        models.PaymentStatusPending,
		DueDate:       dueDate,
		Notes:         req.Notes,
	}

	if err := database.DB.Create(&invoice).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create invoice",
		})
	}

	// Create invoice items
	for _, item := range req.Items {
		invoiceItem := models.InvoiceItem{
			InvoiceID:   invoice.ID,
			Description: item.Description,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			Total:       item.UnitPrice * float64(item.Quantity),
		}
		database.DB.Create(&invoiceItem)
	}

	// Reload invoice with items and subscriber (manual load, gorm:"-" prevents Preload)
	var createdItems []models.InvoiceItem
	database.DB.Where("invoice_id = ?", invoice.ID).Find(&createdItems)
	invoice.Items = createdItems
	database.DB.First(&subscriber, subscriber.ID)
	invoice.Subscriber = subscriber

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    invoice,
	})
}

// Update updates an invoice
func (h *InvoiceHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	var invoice models.Invoice
	if err := database.DB.First(&invoice, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Invoice not found",
		})
	}

	if invoice.Status == models.PaymentStatusCompleted {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cannot update completed invoice",
		})
	}

	type UpdateRequest struct {
		DueDate string `json:"due_date"`
		Notes   string `json:"notes"`
		Status  string `json:"status"`
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	updates := map[string]interface{}{}

	if req.DueDate != "" {
		dueDate, _ := time.Parse("2006-01-02", req.DueDate)
		updates["due_date"] = dueDate
	}
	if req.Notes != "" {
		updates["notes"] = req.Notes
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}

	database.DB.Model(&invoice).Updates(updates)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    invoice,
	})
}

// Delete deletes an invoice
func (h *InvoiceHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")

	var invoice models.Invoice
	if err := database.DB.First(&invoice, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Invoice not found",
		})
	}

	if invoice.Status == models.PaymentStatusCompleted {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cannot delete completed invoice",
		})
	}

	// Delete items first
	database.DB.Where("invoice_id = ?", id).Delete(&models.InvoiceItem{})
	database.DB.Delete(&invoice)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Invoice deleted",
	})
}

// AddPayment adds a payment to an invoice
func (h *InvoiceHandler) AddPayment(c *fiber.Ctx) error {
	id := c.Params("id")

	var invoice models.Invoice
	if err := database.DB.First(&invoice, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Invoice not found",
		})
	}

	type PaymentRequest struct {
		Amount    float64 `json:"amount"`
		Method    string  `json:"method"`
		Reference string  `json:"reference"`
		Notes     string  `json:"notes"`
	}

	var req PaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Create payment
	payment := models.Payment{
		InvoiceID:    &invoice.ID,
		SubscriberID: invoice.SubscriberID,
		ResellerID:   invoice.ResellerID,
		Amount:       req.Amount,
		Method:       req.Method,
		Reference:    req.Reference,
		Notes:        req.Notes,
		Status:       models.PaymentStatusCompleted,
	}
	database.DB.Create(&payment)

	// Update invoice
	newAmountPaid := invoice.AmountPaid + req.Amount
	var newStatus models.PaymentStatus

	if newAmountPaid >= invoice.Total {
		newStatus = models.PaymentStatusCompleted
		now := time.Now()
		database.DB.Model(&invoice).Updates(map[string]interface{}{
			"amount_paid": newAmountPaid,
			"status":      newStatus,
			"paid_date":   &now,
		})
		// Auto-renew subscriber if invoice is auto-generated
		if invoice.AutoGenerated {
			go h.autoRenewSubscriber(invoice.SubscriberID)
		}
	} else {
		newStatus = models.PaymentStatusPending // partial payment
		database.DB.Model(&invoice).Updates(map[string]interface{}{
			"amount_paid": newAmountPaid,
			"status":      newStatus,
		})
	}

	// Create transaction
	transaction := models.Transaction{
		ResellerID:   invoice.ResellerID,
		SubscriberID: &invoice.SubscriberID,
		Type:         models.TransactionTypeRenewal,
		Amount:       req.Amount,
		Description:  fmt.Sprintf("Payment for invoice %s", invoice.InvoiceNumber),
	}
	database.DB.Create(&transaction)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    payment,
	})
}

// GetPayments returns payments for an invoice
func (h *InvoiceHandler) GetPayments(c *fiber.Ctx) error {
	id := c.Params("id")

	var payments []models.Payment
	database.DB.Where("invoice_id = ?", id).Order("created_at DESC").Find(&payments)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    payments,
	})
}

// autoRenewSubscriber extends expiry, resets quotas, and updates RADIUS when an auto-generated invoice is fully paid.
func (h *InvoiceHandler) autoRenewSubscriber(subscriberID uint) {
	var sub models.Subscriber
	if err := database.DB.Preload("Service").First(&sub, subscriberID).Error; err != nil {
		log.Printf("AutoRenew: subscriber %d not found: %v", subscriberID, err)
		return
	}
	if sub.Service == nil {
		log.Printf("AutoRenew: subscriber %d has no service", subscriberID)
		return
	}

	// Calculate new expiry (same logic as BulkAction "renew")
	var newExpiry time.Time
	if sub.ExpiryDate.After(time.Now()) {
		if sub.Service.ExpiryUnit == models.ExpiryUnitMonths {
			newExpiry = sub.ExpiryDate.AddDate(0, sub.Service.ExpiryValue, 0)
		} else {
			newExpiry = sub.ExpiryDate.AddDate(0, 0, sub.Service.ExpiryValue)
		}
	} else {
		if sub.Service.ExpiryUnit == models.ExpiryUnitMonths {
			newExpiry = time.Now().AddDate(0, sub.Service.ExpiryValue, 0)
		} else {
			newExpiry = time.Now().AddDate(0, 0, sub.Service.ExpiryValue)
		}
	}

	// Reset FUP counters
	now := time.Now()
	database.DB.Model(&sub).Updates(map[string]interface{}{
		"expiry_date":           newExpiry,
		"status":                models.SubscriberStatusActive,
		"fup_level":             0,
		"daily_download_used":   0,
		"daily_upload_used":     0,
		"daily_quota_used":      0,
		"last_daily_reset":      now,
		"monthly_fup_level":     0,
		"monthly_download_used": 0,
		"monthly_upload_used":   0,
		"monthly_quota_used":    0,
		"last_monthly_reset":    now,
	})

	// Update RADIUS: Expiration
	database.DB.Where("username = ? AND attribute = ?", sub.Username, "Expiration").Delete(&models.RadCheck{})
	database.DB.Create(&models.RadCheck{
		Username: sub.Username, Attribute: "Expiration", Op: ":=",
		Value: newExpiry.Format("Jan 02 2006 15:04:05"),
	})

	// Remove Auth-Type := Reject if exists
	database.DB.Where("username = ? AND attribute = ? AND value = ?", sub.Username, "Auth-Type", "Reject").Delete(&models.RadCheck{})

	// Reset rate limit to full speed
	database.DB.Where("username = ? AND attribute = ?", sub.Username, "Mikrotik-Rate-Limit").Delete(&models.RadReply{})
	fullSpeedLimit := fmt.Sprintf("%dk/%dk", sub.Service.UploadSpeed, sub.Service.DownloadSpeed)
	database.DB.Create(&models.RadReply{
		Username:  sub.Username,
		Attribute: "Mikrotik-Rate-Limit",
		Op:        "=",
		Value:     fullSpeedLimit,
	})

	// Apply speed via CoA if online
	if sub.NasID != nil && *sub.NasID > 0 && sub.IsOnline {
		var nas models.Nas
		if database.DB.First(&nas, *sub.NasID).Error == nil && nas.IPAddress != "" {
			rateLimit := fmt.Sprintf("%dk/%dk", sub.Service.UploadSpeed, sub.Service.DownloadSpeed)
			coaClient := radius.NewCOAClient(nas.IPAddress, nas.CoAPort, nas.Secret)
			if err := coaClient.UpdateRateLimitViaRadclient(sub.Username, sub.SessionID, rateLimit); err != nil {
				// Fallback to MikroTik API
				client := mikrotik.NewClient(
					fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
					nas.APIUsername,
					nas.APIPassword,
				)
				client.UpdateUserRateLimit(sub.Username, int(sub.Service.DownloadSpeed), int(sub.Service.UploadSpeed))
				client.Close()
			}
		}
	}

	log.Printf("AutoRenew: Subscriber %s renewed until %s (invoice paid)", sub.Username, newExpiry.Format("2006-01-02"))
}

// CalculateProrata calculates prorated billing for service changes
func (h *InvoiceHandler) CalculateProrata(c *fiber.Ctx) error {
	var req struct {
		SubscriberID uint   `json:"subscriber_id"`
		NewServiceID uint   `json:"new_service_id"`
		ChangeDate   string `json:"change_date"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request"})
	}

	var sub models.Subscriber
	if err := database.DB.Preload("Service").First(&sub, req.SubscriberID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	var newService models.Service
	if err := database.DB.First(&newService, req.NewServiceID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "New service not found"})
	}

	changeDate := time.Now()
	if req.ChangeDate != "" {
		if parsed, err := time.Parse("2006-01-02", req.ChangeDate); err == nil {
			changeDate = parsed
		}
	}

	// Calculate remaining days
	remainingDays := int(sub.ExpiryDate.Sub(changeDate).Hours() / 24)
	if remainingDays < 0 {
		remainingDays = 0
	}

	// Calculate prices
	oldPrice := sub.Price
	if sub.Service != nil && !sub.OverridePrice {
		oldPrice = sub.Service.Price
	}
	newPrice := newService.Price

	oldDailyRate := oldPrice / 30.0
	newDailyRate := newPrice / 30.0

	credit := float64(remainingDays) * oldDailyRate
	charge := float64(remainingDays) * newDailyRate
	difference := charge - credit

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"remaining_days":  remainingDays,
			"old_service":     sub.Service,
			"new_service":     newService,
			"old_daily_rate":  oldDailyRate,
			"new_daily_rate":  newDailyRate,
			"credit":          credit,
			"charge":          charge,
			"difference":      difference,
		},
	})
}

// GetCommissions returns reseller commission records
func (h *InvoiceHandler) GetCommissions(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	resellerID := c.QueryInt("reseller_id", 0)
	status := c.Query("status")

	query := database.DB.Table("reseller_commissions")

	if resellerID > 0 {
		query = query.Where("reseller_id = ?", resellerID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var commissions []models.ResellerCommission
	offset := (page - 1) * limit
	query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&commissions)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    commissions,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}
