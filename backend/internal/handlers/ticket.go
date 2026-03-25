package handlers

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/middleware"
	"github.com/proisp/backend/internal/models"
)

type TicketHandler struct{}

func NewTicketHandler() *TicketHandler {
	return &TicketHandler{}
}

// TicketWithNotification represents a ticket with notification info
type TicketWithNotification struct {
	models.Ticket
	HasCustomerReply bool `json:"has_customer_reply"`
}

// List returns all tickets
func (h *TicketHandler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 25)
	status := c.Query("status", "")
	priority := c.Query("priority", "")
	subscriberID := c.QueryInt("subscriber_id", 0)
	assignedTo := c.QueryInt("assigned_to", 0)

	if page < 1 {
		page = 1
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	query := database.DB.Model(&models.Ticket{}).
		Preload("Subscriber").
		Preload("AssignedUser").
		Preload("CreatedByUser")

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if priority != "" {
		query = query.Where("priority = ?", priority)
	}
	if subscriberID > 0 {
		query = query.Where("subscriber_id = ?", subscriberID)
	}
	if assignedTo > 0 {
		query = query.Where("assigned_to = ?", assignedTo)
	}

	var total int64
	query.Count(&total)

	var tickets []models.Ticket
	query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&tickets)

	// Check for customer replies (UserID = 0 means customer reply)
	result := make([]TicketWithNotification, len(tickets))
	for i, t := range tickets {
		var lastReply models.TicketReply
		hasCustomerReply := false
		if err := database.DB.Where("ticket_id = ?", t.ID).Order("created_at DESC").First(&lastReply).Error; err == nil {
			// If last reply is from customer (UserID = 0), show notification
			hasCustomerReply = lastReply.UserID == 0
		}
		result[i] = TicketWithNotification{
			Ticket:           t,
			HasCustomerReply: hasCustomerReply,
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    result,
		"meta": fiber.Map{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// Get returns a single ticket with replies
func (h *TicketHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")

	var ticket models.Ticket
	if err := database.DB.
		Preload("Subscriber").
		Preload("AssignedUser").
		Preload("CreatedByUser").
		Preload("Replies").
		Preload("Replies.User").
		First(&ticket, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Ticket not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    ticket,
	})
}

// Create creates a new ticket
func (h *TicketHandler) Create(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)

	type CreateRequest struct {
		Subject      string `json:"subject"`
		Description  string `json:"description"`
		Priority     string `json:"priority"`
		Category     string `json:"category"`
		SubscriberID *uint  `json:"subscriber_id"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Generate ticket number
	ticketNumber := fmt.Sprintf("TKT-%d-%04d", time.Now().Year(), time.Now().Unix()%10000)

	ticket := models.Ticket{
		TicketNumber: ticketNumber,
		Subject:      req.Subject,
		Description:  req.Description,
		Message:      req.Description, // Set message same as description for backward compatibility
		Priority:     req.Priority,
		Category:     req.Category,
		Status:       "open",
		SubscriberID: req.SubscriberID,
		CreatedBy:    &user.ID,
	}

	// Auto-assign to reseller if subscriber has one
	if req.SubscriberID != nil && *req.SubscriberID > 0 {
		var sub models.Subscriber
		if database.DB.Select("reseller_id").First(&sub, *req.SubscriberID).Error == nil && sub.ResellerID > 0 {
			// Find the user account for this reseller
			var reseller struct{ UserID uint }
			if database.DB.Table("resellers").Select("user_id").Where("id = ?", sub.ResellerID).Scan(&reseller).Error == nil && reseller.UserID > 0 {
				ticket.AssignedTo = &reseller.UserID
			}
		}
	}

	if err := database.DB.Create(&ticket).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create ticket",
		})
	}

	// Add auto-assignment note if assigned
	if ticket.AssignedTo != nil {
		autoReply := models.TicketReply{
			TicketID:   ticket.ID,
			Message:    "Ticket auto-assigned to subscriber's reseller",
			UserID:     0,
			IsInternal: true,
		}
		database.DB.Create(&autoReply)
	}

	database.DB.Preload("Subscriber").Preload("CreatedByUser").Preload("AssignedUser").First(&ticket, ticket.ID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    ticket,
	})
}

// Update updates a ticket
func (h *TicketHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	var ticket models.Ticket
	if err := database.DB.First(&ticket, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Ticket not found",
		})
	}

	type UpdateRequest struct {
		Subject    string `json:"subject"`
		Priority   string `json:"priority"`
		Category   string `json:"category"`
		Status     string `json:"status"`
		AssignedTo *uint  `json:"assigned_to"`
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	updates := map[string]interface{}{}

	if req.Subject != "" {
		updates["subject"] = req.Subject
	}
	if req.Priority != "" {
		updates["priority"] = req.Priority
	}
	if req.Category != "" {
		updates["category"] = req.Category
	}
	if req.Status != "" {
		updates["status"] = req.Status
		if req.Status == "closed" {
			now := time.Now()
			updates["closed_at"] = &now
		}
	}
	if req.AssignedTo != nil {
		updates["assigned_to"] = req.AssignedTo
	}

	database.DB.Model(&ticket).Updates(updates)
	database.DB.Preload("Subscriber").Preload("AssignedUser").First(&ticket, ticket.ID)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    ticket,
	})
}

// Delete deletes a ticket
func (h *TicketHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")

	var ticket models.Ticket
	if err := database.DB.First(&ticket, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Ticket not found",
		})
	}

	// Delete replies first
	database.DB.Where("ticket_id = ?", id).Delete(&models.TicketReply{})
	database.DB.Delete(&ticket)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Ticket deleted",
	})
}

// AddReply adds a reply to a ticket
func (h *TicketHandler) AddReply(c *fiber.Ctx) error {
	id := c.Params("id")
	user := middleware.GetCurrentUser(c)

	var ticket models.Ticket
	if err := database.DB.First(&ticket, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Ticket not found",
		})
	}

	type ReplyRequest struct {
		Message    string `json:"message"`
		IsInternal bool   `json:"is_internal"`
	}

	var req ReplyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	reply := models.TicketReply{
		TicketID:   ticket.ID,
		UserID:     user.ID,
		Message:    req.Message,
		IsInternal: req.IsInternal,
	}

	database.DB.Create(&reply)

	// Update ticket status if closed and customer replied
	if ticket.Status == "closed" {
		database.DB.Model(&ticket).Update("status", "open")
	}

	database.DB.Preload("User").First(&reply, reply.ID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    reply,
	})
}

// GetStats returns ticket statistics
func (h *TicketHandler) GetStats(c *fiber.Ctx) error {
	var openCount, pendingCount, closedCount, totalCount int64

	database.DB.Model(&models.Ticket{}).Where("status = ?", "open").Count(&openCount)
	database.DB.Model(&models.Ticket{}).Where("status = ?", "pending").Count(&pendingCount)
	database.DB.Model(&models.Ticket{}).Where("status = ?", "closed").Count(&closedCount)
	database.DB.Model(&models.Ticket{}).Count(&totalCount)

	// Average resolution time (closed tickets in last 30 days)
	type AvgResult struct {
		AvgHours float64
	}
	var avgResult AvgResult
	database.DB.Raw(`
		SELECT AVG(EXTRACT(EPOCH FROM (closed_at - created_at))/3600) as avg_hours
		FROM tickets
		WHERE status = 'closed' AND closed_at IS NOT NULL
		AND created_at > NOW() - INTERVAL '30 days'
	`).Scan(&avgResult)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"open":                openCount,
			"pending":             pendingCount,
			"closed":              closedCount,
			"total":               totalCount,
			"avgResolutionHours": avgResult.AvgHours,
		},
	})
}
