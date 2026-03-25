package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/middleware"
	"github.com/proisp/backend/internal/models"
)

type PrepaidHandler struct{}

func NewPrepaidHandler() *PrepaidHandler {
	return &PrepaidHandler{}
}

// List returns all prepaid cards
func (h *PrepaidHandler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 25)
	isUsed := c.Query("is_used", "")
	batchID := c.Query("batch_id", "")

	if page < 1 {
		page = 1
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	query := database.DB.Model(&models.PrepaidCard{}).Preload("Service")

	if isUsed == "true" {
		query = query.Where("is_used = ?", true)
	} else if isUsed == "false" {
		query = query.Where("is_used = ?", false)
	}
	if batchID != "" {
		query = query.Where("batch_id = ?", batchID)
	}

	var total int64
	query.Count(&total)

	var cards []models.PrepaidCard
	query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&cards)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    cards,
		"meta": fiber.Map{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// Get returns a single prepaid card
func (h *PrepaidHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")

	var card models.PrepaidCard
	if err := database.DB.Preload("Service").First(&card, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Card not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    card,
	})
}

// Generate generates a batch of prepaid cards
func (h *PrepaidHandler) Generate(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)

	type GenerateRequest struct {
		ServiceID   uint    `json:"service_id"`
		Count       int     `json:"count"`
		Value       float64 `json:"value"`
		Days        int     `json:"days"`
		QuotaRefill int64   `json:"quota_refill"`
		Prefix      string  `json:"prefix"`
		CodeLength  int     `json:"code_length"`
		PINLength   int     `json:"pin_length"`
	}

	var req GenerateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if req.Count < 1 || req.Count > 1000 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Count must be between 1 and 1000",
		})
	}

	if req.CodeLength < 8 {
		req.CodeLength = 12
	}
	if req.PINLength < 4 {
		req.PINLength = 4
	}

	batchID := fmt.Sprintf("BATCH-%d", time.Now().Unix())
	cards := make([]models.PrepaidCard, 0, req.Count)

	resellerID := uint(1)
	if user.ResellerID != nil {
		resellerID = *user.ResellerID
	}

	for i := 0; i < req.Count; i++ {
		code := generateCode(req.Prefix, req.CodeLength)
		pin := generatePIN(req.PINLength)

		card := models.PrepaidCard{
			Code:        code,
			PIN:         pin,
			Value:       req.Value,
			Days:        req.Days,
			QuotaRefill: req.QuotaRefill,
			ServiceID:   req.ServiceID,
			ResellerID:  resellerID,
			BatchID:     batchID,
			BatchNumber: i + 1,
			IsUsed:      false,
			IsActive:    true,
			CreatedBy:   user.ID,
		}
		cards = append(cards, card)
	}

	if err := database.DB.Create(&cards).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate cards",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"batch_id": batchID,
			"count":    len(cards),
			"cards":    cards,
		},
	})
}

// Use redeems a prepaid card
func (h *PrepaidHandler) Use(c *fiber.Ctx) error {
	type UseRequest struct {
		Code         string `json:"code"`
		PIN          string `json:"pin"`
		SubscriberID uint   `json:"subscriber_id"`
	}

	var req UseRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	var card models.PrepaidCard
	if err := database.DB.Where("code = ? AND pin = ?", req.Code, req.PIN).First(&card).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Invalid card code or PIN",
		})
	}

	if card.IsUsed {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Card has already been used",
		})
	}

	if !card.IsActive {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Card is not active",
		})
	}

	// Check expiry if set
	if card.ExpiryDate != nil && card.ExpiryDate.Before(time.Now()) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Card has expired",
		})
	}

	// Get subscriber
	var subscriber models.Subscriber
	if err := database.DB.First(&subscriber, req.SubscriberID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber not found",
		})
	}

	now := time.Now()

	// Update card
	database.DB.Model(&card).Updates(map[string]interface{}{
		"is_used": true,
		"used_by": req.SubscriberID,
		"used_at": &now,
	})

	// Update subscriber
	updates := map[string]interface{}{}

	if card.Days > 0 {
		newExpiry := subscriber.ExpiryDate
		if newExpiry.Before(now) {
			newExpiry = now
		}
		newExpiry = newExpiry.AddDate(0, 0, card.Days)
		updates["expiry_date"] = newExpiry
	}

	if card.ServiceID > 0 {
		updates["service_id"] = card.ServiceID
	}

	if card.QuotaRefill > 0 {
		updates["daily_quota_used"] = 0
		updates["monthly_quota_used"] = 0
	}

	database.DB.Model(&subscriber).Updates(updates)

	// Create transaction
	transaction := models.Transaction{
		ResellerID:   card.ResellerID,
		SubscriberID: &subscriber.ID,
		Type:         models.TransactionTypePrepaidCard,
		Amount:       card.Value,
		Description:  fmt.Sprintf("Prepaid card redeemed: %s", card.Code),
	}
	database.DB.Create(&transaction)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Card redeemed successfully",
		"data": fiber.Map{
			"value":       card.Value,
			"days":        card.Days,
			"quota":       card.QuotaRefill,
		},
	})
}

// Delete deletes unused prepaid cards
func (h *PrepaidHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")

	var card models.PrepaidCard
	if err := database.DB.First(&card, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Card not found",
		})
	}

	if card.IsUsed {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cannot delete used cards",
		})
	}

	database.DB.Delete(&card)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Card deleted",
	})
}

// DeleteBatch deletes all unused cards in a batch
func (h *PrepaidHandler) DeleteBatch(c *fiber.Ctx) error {
	batchID := c.Params("batch")

	result := database.DB.Where("batch_id = ? AND is_used = ?", batchID, false).Delete(&models.PrepaidCard{})

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Deleted %d cards from batch", result.RowsAffected),
	})
}

// GetBatches returns all unique batches
func (h *PrepaidHandler) GetBatches(c *fiber.Ctx) error {
	type BatchInfo struct {
		BatchID string `json:"batch_id"`
		Total   int64  `json:"total"`
		Used    int64  `json:"used"`
		Active  int64  `json:"active"`
	}

	var batches []string
	database.DB.Model(&models.PrepaidCard{}).Distinct("batch_id").Pluck("batch_id", &batches)

	result := make([]BatchInfo, 0, len(batches))
	for _, batchID := range batches {
		var total, used, active int64
		database.DB.Model(&models.PrepaidCard{}).Where("batch_id = ?", batchID).Count(&total)
		database.DB.Model(&models.PrepaidCard{}).Where("batch_id = ? AND is_used = ?", batchID, true).Count(&used)
		database.DB.Model(&models.PrepaidCard{}).Where("batch_id = ? AND is_used = ? AND is_active = ?", batchID, false, true).Count(&active)

		result = append(result, BatchInfo{
			BatchID: batchID,
			Total:   total,
			Used:    used,
			Active:  active,
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

func generateCode(prefix string, length int) string {
	bytes := make([]byte, length/2)
	rand.Read(bytes)
	code := hex.EncodeToString(bytes)
	if prefix != "" {
		return prefix + "-" + code[:length-len(prefix)-1]
	}
	return code[:length]
}

func generatePIN(length int) string {
	const digits = "0123456789"
	pin := make([]byte, length)
	for i := range pin {
		b := make([]byte, 1)
		rand.Read(b)
		pin[i] = digits[int(b[0])%len(digits)]
	}
	return string(pin)
}
