package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

// ExternalAPIHandler handles external API endpoints
type ExternalAPIHandler struct{}

// NewExternalAPIHandler creates a new external API handler
func NewExternalAPIHandler() *ExternalAPIHandler {
	return &ExternalAPIHandler{}
}

// --- Response helpers ---

func extSuccess(c *fiber.Ctx, data interface{}) error {
	return c.JSON(fiber.Map{
		"success":   true,
		"data":      data,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func extSuccessPaginated(c *fiber.Ctx, data interface{}, page, limit int, total int64) error {
	pages := int(total) / limit
	if int(total)%limit > 0 {
		pages++
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data":    data,
		"pagination": fiber.Map{
			"page":  page,
			"limit": limit,
			"total": total,
			"pages": pages,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func extError(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{
		"success":   false,
		"error":     fiber.Map{"code": code, "message": message},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// parsePagination extracts page/limit from query params
func parsePagination(c *fiber.Ctx) (int, int, int) {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit
	return page, limit, offset
}

// --- Subscriber endpoints ---

// ListSubscribers returns a paginated list of subscribers
func (h *ExternalAPIHandler) ListSubscribers(c *fiber.Ctx) error {
	page, limit, offset := parsePagination(c)

	query := database.DB.Model(&models.Subscriber{}).Where("deleted_at IS NULL")

	// Filters
	if username := c.Query("username"); username != "" {
		query = query.Where("username ILIKE ?", "%"+username+"%")
	}
	if status := c.Query("status"); status != "" {
		if s, err := strconv.Atoi(status); err == nil {
			query = query.Where("status = ?", s)
		}
	}
	if serviceID := c.Query("service_id"); serviceID != "" {
		query = query.Where("service_id = ?", serviceID)
	}
	if nasID := c.Query("nas_id"); nasID != "" {
		query = query.Where("nas_id = ?", nasID)
	}
	if isOnline := c.Query("is_online"); isOnline != "" {
		query = query.Where("is_online = ?", isOnline == "true")
	}

	var total int64
	query.Count(&total)

	var subscribers []models.Subscriber
	query.Preload("Service").
		Order("id DESC").
		Offset(offset).Limit(limit).
		Find(&subscribers)

	// Build clean response
	result := make([]fiber.Map, len(subscribers))
	for i, sub := range subscribers {
		item := fiber.Map{
			"id":                    sub.ID,
			"username":              sub.Username,
			"full_name":             sub.FullName,
			"email":                 sub.Email,
			"phone":                 sub.Phone,
			"address":               sub.Address,
			"status":                sub.Status,
			"service_id":            sub.ServiceID,
			"expiry_date":           sub.ExpiryDate,
			"is_online":             sub.IsOnline,
			"ip_address":            sub.IPAddress,
			"mac_address":           sub.MACAddress,
			"daily_download_used":   sub.DailyDownloadUsed,
			"daily_upload_used":     sub.DailyUploadUsed,
			"monthly_download_used": sub.MonthlyDownloadUsed,
			"monthly_upload_used":   sub.MonthlyUploadUsed,
			"fup_level":             sub.FUPLevel,
			"monthly_fup_level":     sub.MonthlyFUPLevel,
			"created_at":            sub.CreatedAt,
			"updated_at":            sub.UpdatedAt,
		}
		if sub.Service != nil {
			item["service_name"] = sub.Service.Name
		}
		if sub.NasID != nil {
			item["nas_id"] = *sub.NasID
		}
		if sub.ResellerID != 0 {
			item["reseller_id"] = sub.ResellerID
		}
		result[i] = item
	}

	return extSuccessPaginated(c, result, page, limit, total)
}

// GetSubscriber returns a single subscriber by ID
func (h *ExternalAPIHandler) GetSubscriber(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return extError(c, 400, "INVALID_PARAMETER", "Invalid subscriber ID")
	}

	var sub models.Subscriber
	if err := database.DB.Preload("Service").Where("deleted_at IS NULL").First(&sub, id).Error; err != nil {
		return extError(c, 404, "NOT_FOUND", "Subscriber not found")
	}

	data := fiber.Map{
		"id":                    sub.ID,
		"username":              sub.Username,
		"full_name":             sub.FullName,
		"email":                 sub.Email,
		"phone":                 sub.Phone,
		"address":               sub.Address,
		"region":                sub.Region,
		"building":              sub.Building,
		"nationality":           sub.Nationality,
		"country":               sub.Country,
		"note":                  sub.Note,
		"status":                sub.Status,
		"service_id":            sub.ServiceID,
		"expiry_date":           sub.ExpiryDate,
		"is_online":             sub.IsOnline,
		"ip_address":            sub.IPAddress,
		"mac_address":           sub.MACAddress,
		"static_ip":             sub.StaticIP,
		"price":                 sub.Price,
		"override_price":        sub.OverridePrice,
		"auto_renew":            sub.AutoRenew,
		"daily_download_used":   sub.DailyDownloadUsed,
		"daily_upload_used":     sub.DailyUploadUsed,
		"monthly_download_used": sub.MonthlyDownloadUsed,
		"monthly_upload_used":   sub.MonthlyUploadUsed,
		"fup_level":             sub.FUPLevel,
		"monthly_fup_level":     sub.MonthlyFUPLevel,
		"reseller_id":           sub.ResellerID,
		"created_at":            sub.CreatedAt,
		"updated_at":            sub.UpdatedAt,
	}
	if sub.NasID != nil {
		data["nas_id"] = *sub.NasID
	}
	if sub.Service != nil {
		data["service_name"] = sub.Service.Name
		data["service"] = fiber.Map{
			"id":             sub.Service.ID,
			"name":           sub.Service.Name,
			"download_speed": sub.Service.DownloadSpeed,
			"upload_speed":   sub.Service.UploadSpeed,
			"daily_quota":    sub.Service.DailyQuota,
			"monthly_quota":  sub.Service.MonthlyQuota,
			"price":          sub.Service.Price,
		}
	}

	return extSuccess(c, data)
}

// GetSubscriberByUsername returns a subscriber by username
func (h *ExternalAPIHandler) GetSubscriberByUsername(c *fiber.Ctx) error {
	username := c.Params("username")
	if username == "" {
		return extError(c, 400, "INVALID_PARAMETER", "Username is required")
	}

	var sub models.Subscriber
	if err := database.DB.Preload("Service").Where("username = ? AND deleted_at IS NULL", username).First(&sub).Error; err != nil {
		return extError(c, 404, "NOT_FOUND", "Subscriber not found")
	}

	data := fiber.Map{
		"id":                    sub.ID,
		"username":              sub.Username,
		"full_name":             sub.FullName,
		"email":                 sub.Email,
		"phone":                 sub.Phone,
		"status":                sub.Status,
		"service_id":            sub.ServiceID,
		"expiry_date":           sub.ExpiryDate,
		"is_online":             sub.IsOnline,
		"ip_address":            sub.IPAddress,
		"mac_address":           sub.MACAddress,
		"daily_download_used":   sub.DailyDownloadUsed,
		"daily_upload_used":     sub.DailyUploadUsed,
		"monthly_download_used": sub.MonthlyDownloadUsed,
		"monthly_upload_used":   sub.MonthlyUploadUsed,
		"fup_level":             sub.FUPLevel,
		"created_at":            sub.CreatedAt,
	}
	if sub.Service != nil {
		data["service_name"] = sub.Service.Name
	}

	return extSuccess(c, data)
}

// CreateSubscriber creates a new subscriber via external API
func (h *ExternalAPIHandler) CreateSubscriber(c *fiber.Ctx) error {
	var req struct {
		Username  string  `json:"username"`
		Password  string  `json:"password"`
		FullName  string  `json:"full_name"`
		Email     string  `json:"email"`
		Phone     string  `json:"phone"`
		Address   string  `json:"address"`
		ServiceID uint    `json:"service_id"`
		ExpiryDate string `json:"expiry_date"`
		Price     *float64 `json:"price"`
		AutoRenew bool    `json:"auto_renew"`
	}
	if err := c.BodyParser(&req); err != nil {
		return extError(c, 400, "INVALID_BODY", "Invalid request body")
	}

	if req.Username == "" {
		return extError(c, 400, "INVALID_PARAMETER", "username is required")
	}
	if req.Password == "" {
		return extError(c, 400, "INVALID_PARAMETER", "password is required")
	}
	if req.ServiceID == 0 {
		return extError(c, 400, "INVALID_PARAMETER", "service_id is required")
	}

	// Verify service exists
	var service models.Service
	if err := database.DB.First(&service, req.ServiceID).Error; err != nil {
		return extError(c, 400, "INVALID_PARAMETER", "Service not found")
	}

	// Check duplicate username
	var count int64
	database.DB.Model(&models.Subscriber{}).Where("username = ? AND deleted_at IS NULL", req.Username).Count(&count)
	if count > 0 {
		return extError(c, 409, "DUPLICATE", "Username already exists")
	}

	// Parse expiry date
	var expiryDate time.Time
	if req.ExpiryDate != "" {
		t, err := time.Parse("2006-01-02", req.ExpiryDate)
		if err != nil {
			return extError(c, 400, "INVALID_PARAMETER", "Invalid expiry_date format. Use YYYY-MM-DD")
		}
		expiryDate = t
	} else {
		expiryDate = time.Now().AddDate(0, 1, 0) // Default: 1 month
	}

	price := service.Price
	overridePrice := false
	if req.Price != nil {
		price = *req.Price
		overridePrice = true
	}

	sub := models.Subscriber{
		Username:      req.Username,
		PasswordPlain: req.Password,
		FullName:      req.FullName,
		Email:         req.Email,
		Phone:         req.Phone,
		Address:       req.Address,
		ServiceID:     req.ServiceID,
		Status:        models.SubscriberStatusActive,
		ExpiryDate:    expiryDate,
		Price:         price,
		OverridePrice: overridePrice,
		AutoRenew:     req.AutoRenew,
	}

	if err := database.DB.Create(&sub).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return extError(c, 409, "DUPLICATE", "Username already exists")
		}
		return extError(c, 500, "CREATE_FAILED", "Failed to create subscriber")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"id":           sub.ID,
			"username":     sub.Username,
			"full_name":    sub.FullName,
			"service_id":   sub.ServiceID,
			"status":       sub.Status,
			"expiry_date":  sub.ExpiryDate,
			"created_at":   sub.CreatedAt,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// UpdateSubscriber updates an existing subscriber
func (h *ExternalAPIHandler) UpdateSubscriber(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return extError(c, 400, "INVALID_PARAMETER", "Invalid subscriber ID")
	}

	var sub models.Subscriber
	if err := database.DB.Where("deleted_at IS NULL").First(&sub, id).Error; err != nil {
		return extError(c, 404, "NOT_FOUND", "Subscriber not found")
	}

	var req struct {
		FullName    *string  `json:"full_name"`
		Email       *string  `json:"email"`
		Phone       *string  `json:"phone"`
		Address     *string  `json:"address"`
		Password    *string  `json:"password"`
		ServiceID   *uint    `json:"service_id"`
		ExpiryDate  *string  `json:"expiry_date"`
		Price       *float64 `json:"price"`
		AutoRenew   *bool    `json:"auto_renew"`
	}
	if err := c.BodyParser(&req); err != nil {
		return extError(c, 400, "INVALID_BODY", "Invalid request body")
	}

	updates := map[string]interface{}{}
	if req.FullName != nil {
		updates["full_name"] = *req.FullName
	}
	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.Phone != nil {
		updates["phone"] = *req.Phone
	}
	if req.Address != nil {
		updates["address"] = *req.Address
	}
	if req.Password != nil && *req.Password != "" {
		updates["password_plain"] = *req.Password
	}
	if req.ServiceID != nil {
		var svc models.Service
		if err := database.DB.First(&svc, *req.ServiceID).Error; err != nil {
			return extError(c, 400, "INVALID_PARAMETER", "Service not found")
		}
		updates["service_id"] = *req.ServiceID
	}
	if req.ExpiryDate != nil {
		t, err := time.Parse("2006-01-02", *req.ExpiryDate)
		if err != nil {
			return extError(c, 400, "INVALID_PARAMETER", "Invalid expiry_date format. Use YYYY-MM-DD")
		}
		updates["expiry_date"] = t
	}
	if req.Price != nil {
		updates["price"] = *req.Price
		updates["override_price"] = true
	}
	if req.AutoRenew != nil {
		updates["auto_renew"] = *req.AutoRenew
	}

	if len(updates) == 0 {
		return extError(c, 400, "NO_CHANGES", "No fields to update")
	}

	updates["updated_at"] = time.Now()
	if err := database.DB.Model(&sub).Updates(updates).Error; err != nil {
		return extError(c, 500, "UPDATE_FAILED", "Failed to update subscriber")
	}

	return extSuccess(c, fiber.Map{
		"id":         sub.ID,
		"username":   sub.Username,
		"message":    "Subscriber updated successfully",
	})
}

// DeleteSubscriber soft-deletes a subscriber
func (h *ExternalAPIHandler) DeleteSubscriber(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return extError(c, 400, "INVALID_PARAMETER", "Invalid subscriber ID")
	}

	var sub models.Subscriber
	if err := database.DB.Where("deleted_at IS NULL").First(&sub, id).Error; err != nil {
		return extError(c, 404, "NOT_FOUND", "Subscriber not found")
	}

	if err := database.DB.Delete(&sub).Error; err != nil {
		return extError(c, 500, "DELETE_FAILED", "Failed to delete subscriber")
	}

	return extSuccess(c, fiber.Map{
		"id":       sub.ID,
		"username": sub.Username,
		"message":  "Subscriber deleted successfully",
	})
}

// SuspendSubscriber suspends a subscriber
func (h *ExternalAPIHandler) SuspendSubscriber(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return extError(c, 400, "INVALID_PARAMETER", "Invalid subscriber ID")
	}

	result := database.DB.Model(&models.Subscriber{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"status":     models.SubscriberStatusInactive,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return extError(c, 500, "UPDATE_FAILED", "Failed to suspend subscriber")
	}
	if result.RowsAffected == 0 {
		return extError(c, 404, "NOT_FOUND", "Subscriber not found")
	}

	return extSuccess(c, fiber.Map{"id": id, "status": "suspended"})
}

// ActivateSubscriber activates a subscriber
func (h *ExternalAPIHandler) ActivateSubscriber(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return extError(c, 400, "INVALID_PARAMETER", "Invalid subscriber ID")
	}

	result := database.DB.Model(&models.Subscriber{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"status":     models.SubscriberStatusActive,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return extError(c, 500, "UPDATE_FAILED", "Failed to activate subscriber")
	}
	if result.RowsAffected == 0 {
		return extError(c, 404, "NOT_FOUND", "Subscriber not found")
	}

	return extSuccess(c, fiber.Map{"id": id, "status": "active"})
}

// GetSubscriberUsage returns current usage stats for a subscriber
func (h *ExternalAPIHandler) GetSubscriberUsage(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return extError(c, 400, "INVALID_PARAMETER", "Invalid subscriber ID")
	}

	var sub models.Subscriber
	if err := database.DB.Preload("Service").Where("deleted_at IS NULL").First(&sub, id).Error; err != nil {
		return extError(c, 404, "NOT_FOUND", "Subscriber not found")
	}

	data := fiber.Map{
		"id":                    sub.ID,
		"username":              sub.Username,
		"is_online":             sub.IsOnline,
		"daily_download_used":   sub.DailyDownloadUsed,
		"daily_upload_used":     sub.DailyUploadUsed,
		"monthly_download_used": sub.MonthlyDownloadUsed,
		"monthly_upload_used":   sub.MonthlyUploadUsed,
		"fup_level":             sub.FUPLevel,
		"monthly_fup_level":     sub.MonthlyFUPLevel,
	}
	if sub.Service != nil {
		data["daily_quota"] = sub.Service.DailyQuota
		data["monthly_quota"] = sub.Service.MonthlyQuota
		if sub.Service.DailyQuota > 0 {
			data["daily_usage_percent"] = float64(sub.DailyDownloadUsed+sub.DailyUploadUsed) / float64(sub.Service.DailyQuota) * 100
		}
		if sub.Service.MonthlyQuota > 0 {
			data["monthly_usage_percent"] = float64(sub.MonthlyDownloadUsed+sub.MonthlyUploadUsed) / float64(sub.Service.MonthlyQuota) * 100
		}
	}

	return extSuccess(c, data)
}

// --- Service endpoints ---

// ListServices returns all services
func (h *ExternalAPIHandler) ListServices(c *fiber.Ctx) error {
	var services []models.Service
	database.DB.Where("deleted_at IS NULL").Order("name ASC").Find(&services)

	result := make([]fiber.Map, len(services))
	for i, svc := range services {
		result[i] = fiber.Map{
			"id":             svc.ID,
			"name":           svc.Name,
			"description":    svc.Description,
			"download_speed": svc.DownloadSpeed,
			"upload_speed":   svc.UploadSpeed,
			"daily_quota":    svc.DailyQuota,
			"monthly_quota":  svc.MonthlyQuota,
			"price":          svc.Price,
			"created_at":     svc.CreatedAt,
		}
	}

	return extSuccess(c, result)
}

// GetService returns a single service by ID
func (h *ExternalAPIHandler) GetService(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return extError(c, 400, "INVALID_PARAMETER", "Invalid service ID")
	}

	var svc models.Service
	if err := database.DB.Where("deleted_at IS NULL").First(&svc, id).Error; err != nil {
		return extError(c, 404, "NOT_FOUND", "Service not found")
	}

	return extSuccess(c, fiber.Map{
		"id":                svc.ID,
		"name":              svc.Name,
		"description":       svc.Description,
		"download_speed":    svc.DownloadSpeed,
		"upload_speed":      svc.UploadSpeed,
		"download_speed_str": svc.DownloadSpeedStr,
		"upload_speed_str":  svc.UploadSpeedStr,
		"daily_quota":       svc.DailyQuota,
		"monthly_quota":     svc.MonthlyQuota,
		"price":             svc.Price,
		"created_at":        svc.CreatedAt,
	})
}

// --- NAS endpoints ---

// ListNAS returns all NAS devices
func (h *ExternalAPIHandler) ListNAS(c *fiber.Ctx) error {
	var devices []models.Nas
	database.DB.Where("deleted_at IS NULL").Order("name ASC").Find(&devices)

	result := make([]fiber.Map, len(devices))
	for i, nas := range devices {
		result[i] = fiber.Map{
			"id":              nas.ID,
			"name":            nas.Name,
			"ip_address":      nas.IPAddress,
			"type":            nas.Type,
			"is_active":       nas.IsActive,
			"is_online":       nas.IsOnline,
			"last_seen":       nas.LastSeen,
			"active_sessions": nas.ActiveSessions,
			"version":         nas.Version,
			"created_at":      nas.CreatedAt,
		}
	}

	return extSuccess(c, result)
}

// GetNAS returns a single NAS device by ID
func (h *ExternalAPIHandler) GetNAS(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return extError(c, 400, "INVALID_PARAMETER", "Invalid NAS ID")
	}

	var nas models.Nas
	if err := database.DB.Where("deleted_at IS NULL").First(&nas, id).Error; err != nil {
		return extError(c, 404, "NOT_FOUND", "NAS device not found")
	}

	// Count online users for this NAS
	var onlineCount int64
	database.DB.Model(&models.Subscriber{}).Where("nas_id = ? AND is_online = true AND deleted_at IS NULL", id).Count(&onlineCount)

	return extSuccess(c, fiber.Map{
		"id":              nas.ID,
		"name":            nas.Name,
		"ip_address":      nas.IPAddress,
		"type":            nas.Type,
		"is_active":       nas.IsActive,
		"is_online":       nas.IsOnline,
		"last_seen":       nas.LastSeen,
		"active_sessions": nas.ActiveSessions,
		"version":         nas.Version,
		"online_users":    onlineCount,
		"created_at":      nas.CreatedAt,
	})
}

// --- Transaction endpoints ---

// ListTransactions returns a paginated list of transactions
func (h *ExternalAPIHandler) ListTransactions(c *fiber.Ctx) error {
	page, limit, offset := parsePagination(c)

	query := database.DB.Model(&models.Transaction{})

	// Filters
	if subID := c.Query("subscriber_id"); subID != "" {
		query = query.Where("subscriber_id = ?", subID)
	}
	if txType := c.Query("type"); txType != "" {
		query = query.Where("type = ?", txType)
	}
	if dateFrom := c.Query("date_from"); dateFrom != "" {
		if t, err := time.Parse("2006-01-02", dateFrom); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if dateTo := c.Query("date_to"); dateTo != "" {
		if t, err := time.Parse("2006-01-02", dateTo); err == nil {
			query = query.Where("created_at < ?", t.AddDate(0, 0, 1))
		}
	}

	var total int64
	query.Count(&total)

	var transactions []models.Transaction
	query.Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&transactions)

	result := make([]fiber.Map, len(transactions))
	for i, tx := range transactions {
		item := fiber.Map{
			"id":          tx.ID,
			"type":        tx.Type,
			"amount":      tx.Amount,
			"description": tx.Description,
			"reseller_id": tx.ResellerID,
			"created_at":  tx.CreatedAt,
		}
		if tx.SubscriberID != nil {
			item["subscriber_id"] = *tx.SubscriberID
		}
		if tx.ServiceName != "" {
			item["service_name"] = tx.ServiceName
		}
		result[i] = item
	}

	return extSuccessPaginated(c, result, page, limit, total)
}

// CreateTransaction creates a new transaction (payment/charge)
func (h *ExternalAPIHandler) CreateTransaction(c *fiber.Ctx) error {
	var req struct {
		Type         string  `json:"type"`
		Amount       float64 `json:"amount"`
		Description  string  `json:"description"`
		SubscriberID *uint   `json:"subscriber_id"`
		ResellerID   uint    `json:"reseller_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return extError(c, 400, "INVALID_BODY", "Invalid request body")
	}

	if req.Type == "" {
		return extError(c, 400, "INVALID_PARAMETER", "type is required")
	}
	if req.Amount == 0 {
		return extError(c, 400, "INVALID_PARAMETER", "amount is required and cannot be zero")
	}

	// If reseller_id not provided, use the API key owner's info
	if req.ResellerID == 0 {
		user := c.Locals("user").(*models.User)
		if user.ResellerID != nil {
			req.ResellerID = *user.ResellerID
		}
	}

	tx := models.Transaction{
		Type:         models.TransactionType(req.Type),
		Amount:       req.Amount,
		Description:  req.Description,
		SubscriberID: req.SubscriberID,
		ResellerID:   req.ResellerID,
		IPAddress:    c.IP(),
	}

	if err := database.DB.Create(&tx).Error; err != nil {
		return extError(c, 500, "CREATE_FAILED", "Failed to create transaction")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"id":          tx.ID,
			"type":        tx.Type,
			"amount":      tx.Amount,
			"description": tx.Description,
			"created_at":  tx.CreatedAt,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// --- System endpoints ---

// GetSystemStats returns system statistics
func (h *ExternalAPIHandler) GetSystemStats(c *fiber.Ctx) error {
	var onlineUsers int64
	var totalSubscribers int64
	var activeSubscribers int64
	var nasCount int64

	database.DB.Model(&models.Subscriber{}).Where("is_online = true AND deleted_at IS NULL").Count(&onlineUsers)
	database.DB.Model(&models.Subscriber{}).Where("deleted_at IS NULL").Count(&totalSubscribers)
	database.DB.Model(&models.Subscriber{}).Where("status = ? AND deleted_at IS NULL", models.SubscriberStatusActive).Count(&activeSubscribers)
	database.DB.Model(&models.Nas{}).Where("deleted_at IS NULL AND is_active = true").Count(&nasCount)

	return extSuccess(c, fiber.Map{
		"online_users":       onlineUsers,
		"total_subscribers":  totalSubscribers,
		"active_subscribers": activeSubscribers,
		"nas_count":          nasCount,
	})
}

// GetSystemHealth returns API health check
func (h *ExternalAPIHandler) GetSystemHealth(c *fiber.Ctx) error {
	// Check DB connectivity
	sqlDB, err := database.DB.DB()
	dbOk := err == nil
	if dbOk {
		err = sqlDB.Ping()
		dbOk = err == nil
	}

	status := "healthy"
	if !dbOk {
		status = "degraded"
	}

	return extSuccess(c, fiber.Map{
		"status":   status,
		"database": dbOk,
		"version":  fmt.Sprintf("ProxPanel External API v1.0"),
	})
}
