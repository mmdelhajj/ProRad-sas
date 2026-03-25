package handlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/middleware"
	"github.com/proisp/backend/internal/models"
)

// NotificationBannerHandler handles notification banner CRUD
type NotificationBannerHandler struct{}

// NewNotificationBannerHandler creates a new handler
func NewNotificationBannerHandler() *NotificationBannerHandler {
	return &NotificationBannerHandler{}
}

// CreateBannerRequest represents the create/update request body
type CreateBannerRequest struct {
	Title       string `json:"title"`
	Message     string `json:"message"`
	BannerType  string `json:"banner_type"`
	Target      string `json:"target"`
	TargetIDs   string `json:"target_ids"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Dismissible *bool  `json:"dismissible"`
	Enabled     *bool  `json:"enabled"`
}

// List returns all banners - admin sees all, reseller sees only their own
func (h *NotificationBannerHandler) List(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized",
		})
	}

	var banners []models.NotificationBanner
	query := database.DB.Order("created_at DESC")

	if user.UserType != models.UserTypeAdmin {
		// Resellers see only their own banners
		if user.ResellerID != nil {
			query = query.Where("reseller_id = ?", *user.ResellerID)
		} else {
			return c.JSON(fiber.Map{"success": true, "data": []models.NotificationBanner{}})
		}
	}

	if err := query.Find(&banners).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch banners",
		})
	}

	return c.JSON(fiber.Map{"success": true, "data": banners})
}

// Create creates a new notification banner
func (h *NotificationBannerHandler) Create(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized",
		})
	}

	var req CreateBannerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if req.Title == "" || req.Message == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Title and message are required",
		})
	}

	startDate, err := time.Parse("2006-01-02T15:04", req.StartDate)
	if err != nil {
		startDate, err = time.Parse("2006-01-02T15:04:05", req.StartDate)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Invalid start date format",
			})
		}
	}

	endDate, err := time.Parse("2006-01-02T15:04", req.EndDate)
	if err != nil {
		endDate, err = time.Parse("2006-01-02T15:04:05", req.EndDate)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Invalid end date format",
			})
		}
	}

	resellerIDVal := uint(0)
	if user.ResellerID != nil {
		resellerIDVal = *user.ResellerID
	}
	log.Printf("BannerCreate: user=%s userType=%d resellerID=%d target=%s", user.Username, user.UserType, resellerIDVal, req.Target)

	banner := models.NotificationBanner{
		Title:         req.Title,
		Message:       req.Message,
		BannerType:    coalesceStr(req.BannerType, "info"),
		Target:        coalesceStr(req.Target, "all"),
		TargetIDs:     req.TargetIDs,
		StartDate:     startDate,
		EndDate:       endDate,
		Dismissible:   req.Dismissible == nil || *req.Dismissible,
		Enabled:       req.Enabled == nil || *req.Enabled,
		CreatedByID:   user.ID,
		CreatedByName: user.Username,
	}

	// Resellers can target their subscribers or sub-resellers
	if user.UserType == models.UserTypeReseller {
		if user.ResellerID != nil {
			banner.ResellerID = *user.ResellerID
		}
		// Allow "subscribers" or "sub_resellers"
		if req.Target == "sub_resellers" {
			banner.Target = "sub_resellers"
			banner.TargetIDs = req.TargetIDs
		} else {
			banner.Target = "subscribers"
			banner.TargetIDs = ""
		}
	}

	if err := database.DB.Create(&banner).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create banner",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    banner,
		"message": "Banner created successfully",
	})
}

// Update updates an existing notification banner
func (h *NotificationBannerHandler) Update(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized",
		})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid banner ID",
		})
	}

	var banner models.NotificationBanner
	if err := database.DB.First(&banner, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Banner not found",
		})
	}

	// Ownership check for resellers
	if user.UserType == models.UserTypeReseller {
		if user.ResellerID == nil || banner.ResellerID != *user.ResellerID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": "You can only edit your own banners",
			})
		}
	}

	var req CreateBannerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if req.Title != "" {
		banner.Title = req.Title
	}
	if req.Message != "" {
		banner.Message = req.Message
	}
	if req.BannerType != "" {
		banner.BannerType = req.BannerType
	}
	if req.StartDate != "" {
		if t, err := time.Parse("2006-01-02T15:04", req.StartDate); err == nil {
			banner.StartDate = t
		} else if t, err := time.Parse("2006-01-02T15:04:05", req.StartDate); err == nil {
			banner.StartDate = t
		}
	}
	if req.EndDate != "" {
		if t, err := time.Parse("2006-01-02T15:04", req.EndDate); err == nil {
			banner.EndDate = t
		} else if t, err := time.Parse("2006-01-02T15:04:05", req.EndDate); err == nil {
			banner.EndDate = t
		}
	}
	if req.Dismissible != nil {
		banner.Dismissible = *req.Dismissible
	}
	if req.Enabled != nil {
		banner.Enabled = *req.Enabled
	}

	// Admin can set any target, resellers can set subscribers or sub_resellers
	if user.UserType == models.UserTypeAdmin {
		if req.Target != "" {
			banner.Target = req.Target
		}
		banner.TargetIDs = req.TargetIDs
	} else {
		if req.Target == "sub_resellers" {
			banner.Target = "sub_resellers"
			banner.TargetIDs = req.TargetIDs
		} else {
			banner.Target = "subscribers"
			banner.TargetIDs = ""
		}
	}

	if err := database.DB.Save(&banner).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update banner",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    banner,
		"message": "Banner updated successfully",
	})
}

// Delete soft-deletes a notification banner
func (h *NotificationBannerHandler) Delete(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized",
		})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid banner ID",
		})
	}

	var banner models.NotificationBanner
	if err := database.DB.First(&banner, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Banner not found",
		})
	}

	// Ownership check for resellers
	if user.UserType == models.UserTypeReseller {
		if user.ResellerID == nil || banner.ResellerID != *user.ResellerID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": "You can only delete your own banners",
			})
		}
	}

	if err := database.DB.Delete(&banner).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to delete banner",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Banner deleted successfully",
	})
}

// GetActive returns banners active NOW for the current user
func (h *NotificationBannerHandler) GetActive(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized",
		})
	}

	now := time.Now()
	var banners []models.NotificationBanner

	resellerIDVal := uint(0)
	if user.ResellerID != nil {
		resellerIDVal = *user.ResellerID
	}
	log.Printf("GetActive: user=%s userType=%d resellerID=%d", user.Username, user.UserType, resellerIDVal)

	query := database.DB.Where("enabled = ? AND start_date <= ? AND end_date >= ?", true, now, now)

	switch user.UserType {
	case models.UserTypeAdmin:
		// Admin sees all admin-created banners targeting "all" or "resellers"
		query = query.Where("reseller_id = 0")

	case models.UserTypeReseller:
		// Reseller sees:
		// 1. Admin banners for "all"
		// 2. Admin banners for "resellers" matching their ID (or no target_ids = all resellers)
		// 3. Their own reseller-created banners
		// 4. Parent reseller's banners targeting "sub_resellers" (if this is a sub-reseller)
		resellerID := uint(0)
		if user.ResellerID != nil {
			resellerID = *user.ResellerID
		}
		resellerIDStr := fmt.Sprintf("%d", resellerID)

		// Check if this reseller has a parent (is a sub-reseller)
		var parentID uint
		var reseller models.Reseller
		if err := database.DB.Select("id, parent_id").First(&reseller, resellerID).Error; err == nil && reseller.ParentID != nil {
			parentID = *reseller.ParentID
		}

		if parentID > 0 {
			// Sub-reseller: also see parent's banners + admin banners targeting sub_resellers
			query = query.Where(`(
				(reseller_id = 0 AND target = 'all')
				OR (reseller_id = 0 AND target = 'resellers' AND (target_ids = '' OR target_ids LIKE ?))
				OR (reseller_id = 0 AND target = 'sub_resellers' AND (target_ids = '' OR target_ids LIKE ?))
				OR (reseller_id = ? AND target = 'sub_resellers' AND (target_ids = '' OR target_ids LIKE ?))
				OR (reseller_id = ?)
			)`, "%"+resellerIDStr+"%", "%"+resellerIDStr+"%", parentID, "%"+resellerIDStr+"%", resellerID)
		} else {
			query = query.Where(`(
				(reseller_id = 0 AND target = 'all')
				OR (reseller_id = 0 AND target = 'resellers' AND (target_ids = '' OR target_ids LIKE ?))
				OR (reseller_id = 0 AND target = 'sub_resellers' AND (target_ids = '' OR target_ids LIKE ?))
				OR (reseller_id = ?)
			)`, "%"+resellerIDStr+"%", "%"+resellerIDStr+"%", resellerID)
		}

	default:
		// Subscriber/customer: sees banners targeting all or subscribers
		// including reseller-created banners for their reseller's subscribers
		query = query.Where("target IN (?, ?)", "all", "subscribers")
	}

	if err := query.Order("created_at DESC").Find(&banners).Error; err != nil {
		log.Printf("GetActive: ERROR fetching banners: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch active banners",
		})
	}

	log.Printf("GetActive: found %d banners for user %s", len(banners), user.Username)

	// For resellers, do precise target_ids matching (the LIKE above may match partial IDs)
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		resellerIDStr := fmt.Sprintf("%d", *user.ResellerID)
		filtered := make([]models.NotificationBanner, 0, len(banners))
		for _, b := range banners {
			// Reseller's own banners always included
			if b.ResellerID == *user.ResellerID {
				filtered = append(filtered, b)
				continue
			}
			// Banners with no target_ids restriction: show to all
			if b.TargetIDs == "" {
				filtered = append(filtered, b)
				continue
			}
			// Check exact ID match in comma-separated target_ids
			ids := strings.Split(b.TargetIDs, ",")
			for _, id := range ids {
				if strings.TrimSpace(id) == resellerIDStr {
					filtered = append(filtered, b)
					break
				}
			}
		}
		banners = filtered
	}

	return c.JSON(fiber.Map{"success": true, "data": banners})
}

// GetActiveForCustomer returns active banners for customer portal (no auth required beyond customer auth)
func (h *NotificationBannerHandler) GetActiveForCustomer(c *fiber.Ctx) error {
	now := time.Now()
	var banners []models.NotificationBanner

	// Get subscriber's reseller_id from their username
	customerUsername, _ := c.Locals("customer_username").(string)
	log.Printf("GetActiveForCustomer: customerUsername=%s", customerUsername)

	var subscriberResellerID uint
	if customerUsername != "" {
		var sub models.Subscriber
		if err := database.DB.Select("reseller_id").Where("username = ?", customerUsername).First(&sub).Error; err == nil {
			subscriberResellerID = sub.ResellerID
		} else {
			log.Printf("GetActiveForCustomer: subscriber lookup error: %v", err)
		}
	}

	resellerIDStr := fmt.Sprintf("%d", subscriberResellerID)
	log.Printf("GetActiveForCustomer: subscriberResellerID=%d", subscriberResellerID)

	// Customers see:
	// 1. Admin banners (reseller_id=0) targeting "all"
	// 2. Admin banners (reseller_id=0) targeting "subscribers" where target_ids is empty OR contains their reseller_id
	// 3. Their reseller's banners targeting "subscribers"
	if subscriberResellerID > 0 {
		if err := database.DB.Where(
			`enabled = ? AND start_date <= ? AND end_date >= ? AND (
				(target = 'all' AND reseller_id = 0)
				OR (target = 'subscribers' AND reseller_id = 0 AND (target_ids = '' OR target_ids LIKE ?))
				OR (target = 'subscribers' AND reseller_id = ?)
			)`,
			true, now, now, "%"+resellerIDStr+"%", subscriberResellerID,
		).Order("created_at DESC").Find(&banners).Error; err != nil {
			log.Printf("GetActiveForCustomer: query error: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to fetch banners",
			})
		}
	} else {
		// No reseller — only admin banners targeting all or subscribers with no reseller filter
		if err := database.DB.Where(
			"enabled = ? AND start_date <= ? AND end_date >= ? AND target IN (?, ?) AND reseller_id = 0 AND target_ids = ''",
			true, now, now, "all", "subscribers",
		).Order("created_at DESC").Find(&banners).Error; err != nil {
			log.Printf("GetActiveForCustomer: query error: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to fetch banners",
			})
		}
	}

	// Precise target_ids filtering (LIKE may match partial IDs like "3" in "13")
	if subscriberResellerID > 0 {
		filtered := make([]models.NotificationBanner, 0, len(banners))
		for _, b := range banners {
			// Reseller's own banners always included
			if b.ResellerID == subscriberResellerID {
				filtered = append(filtered, b)
				continue
			}
			// Admin banners with no target_ids restriction
			if b.TargetIDs == "" {
				filtered = append(filtered, b)
				continue
			}
			// Check exact ID match in comma-separated target_ids
			ids := strings.Split(b.TargetIDs, ",")
			for _, id := range ids {
				if strings.TrimSpace(id) == resellerIDStr {
					filtered = append(filtered, b)
					break
				}
			}
		}
		banners = filtered
	}

	log.Printf("GetActiveForCustomer: returning %d banners for customer %s (resellerID=%d)", len(banners), customerUsername, subscriberResellerID)
	return c.JSON(fiber.Map{"success": true, "data": banners})
}

// GetSubResellers returns the current reseller's sub-resellers (for notification targeting)
func (h *NotificationBannerHandler) GetSubResellers(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil || user.ResellerID == nil {
		return c.JSON(fiber.Map{"success": true, "data": []interface{}{}})
	}

	var subResellers []models.Reseller
	database.DB.Preload("User").Where("parent_id = ?", *user.ResellerID).Find(&subResellers)

	type SubResellerInfo struct {
		ID       uint   `json:"id"`
		Name     string `json:"name"`
		Username string `json:"username"`
	}

	result := make([]SubResellerInfo, 0, len(subResellers))
	for _, r := range subResellers {
		info := SubResellerInfo{ID: r.ID, Name: r.Name}
		if r.User != nil {
			info.Username = r.User.Username
			if info.Name == "" {
				info.Name = r.User.Username
			}
		}
		result = append(result, info)
	}

	return c.JSON(fiber.Map{"success": true, "data": result})
}

func coalesceStr(val, def string) string {
	if val == "" {
		return def
	}
	return val
}
