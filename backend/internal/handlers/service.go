package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/middleware"
	"github.com/proisp/backend/internal/models"
)

// convertSpeedForMikrotik converts speed to kb format
// All speeds stored as kb: 2000k, 1200k, 4000k
func convertSpeedForMikrotik(speedStr string) string {
	if speedStr == "" {
		return ""
	}

	speedStr = strings.TrimSpace(speedStr)

	// Already in k format - keep as-is
	if strings.HasSuffix(strings.ToLower(speedStr), "k") {
		return speedStr
	}

	// If ends with M, convert to k
	if strings.HasSuffix(strings.ToLower(speedStr), "m") {
		numStr := speedStr[:len(speedStr)-1]
		val, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return speedStr
		}
		return fmt.Sprintf("%dk", int64(val*1000))
	}

	// Plain number - treat as kb value, add k suffix
	return speedStr + "k"
}

type ServiceHandler struct{}

func NewServiceHandler() *ServiceHandler {
	return &ServiceHandler{}
}

// List returns all services (filtered by reseller assignment if user is reseller)
func (h *ServiceHandler) List(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	var services []models.Service

	query := database.DB.Model(&models.Service{}).Where("is_active = ?", true)

	// Get all or filter by active
	showAll := c.Query("all", "false") == "true"
	if showAll {
		query = database.DB.Model(&models.Service{})
	}

	// If user is a reseller, only show assigned services
	if user != nil && user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		// Get assigned service IDs
		var serviceIDs []uint
		database.DB.Model(&models.ResellerService{}).
			Where("reseller_id = ? AND is_enabled = ?", *user.ResellerID, true).
			Pluck("service_id", &serviceIDs)

		if len(serviceIDs) > 0 {
			query = query.Where("id IN ?", serviceIDs)
		} else {
			// If no services assigned, return empty list
			return c.JSON(fiber.Map{
				"success": true,
				"data":    []models.Service{},
			})
		}
	}

	if err := query.Order("sort_order ASC, name ASC").Find(&services).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch services",
		})
	}

	// If reseller, include reseller-specific pricing
	if user != nil && user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		// Get reseller pricing for these services
		var resellerServices []models.ResellerService
		database.DB.Where("reseller_id = ?", *user.ResellerID).Find(&resellerServices)

		// Create a map of service ID to reseller price
		priceMap := make(map[uint]float64)
		dayPriceMap := make(map[uint]float64)
		for _, rs := range resellerServices {
			priceMap[rs.ServiceID] = rs.Price
			dayPriceMap[rs.ServiceID] = rs.DayPrice
		}

		// Update service prices to reseller prices
		for i := range services {
			if price, ok := priceMap[services[i].ID]; ok {
				services[i].Price = price
			}
			if dayPrice, ok := dayPriceMap[services[i].ID]; ok && dayPrice > 0 {
				services[i].DayPrice = dayPrice
			}
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    services,
	})
}

// Get returns a single service
func (h *ServiceHandler) Get(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid service ID",
		})
	}

	var service models.Service
	if err := database.DB.First(&service, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Service not found",
		})
	}

	// Get subscriber count
	var subscriberCount int64
	database.DB.Model(&models.Subscriber{}).Where("service_id = ?", id).Count(&subscriberCount)

	return c.JSON(fiber.Map{
		"success":          true,
		"data":             service,
		"subscriber_count": subscriberCount,
	})
}

// CreateServiceRequest represents create service request
type CreateServiceRequest struct {
	Name             string  `json:"name"`
	CommercialName   string  `json:"commercial_name"`
	Description      string  `json:"description"`
	DownloadSpeed    int64   `json:"download_speed"`
	UploadSpeed      int64   `json:"upload_speed"`
	DownloadSpeedStr string  `json:"download_speed_str"`
	UploadSpeedStr   string  `json:"upload_speed_str"`
	BurstDownload    int64   `json:"burst_download"`
	BurstUpload      int64   `json:"burst_upload"`
	BurstThreshold   int64   `json:"burst_threshold"`
	BurstTime        int     `json:"burst_time"`
	DailyQuota       int64   `json:"daily_quota"`
	MonthlyQuota     int64   `json:"monthly_quota"`
	TimeQuota        int     `json:"time_quota"`
	// Multi-tier Daily FUP with direct speeds (in Kbps, e.g., 700 = 700k)
	FUP1Threshold     int64 `json:"fup1_threshold"`      // bytes
	FUP1DownloadSpeed int64 `json:"fup1_download_speed"` // Kbps
	FUP1UploadSpeed   int64 `json:"fup1_upload_speed"`   // Kbps
	FUP2Threshold     int64 `json:"fup2_threshold"`
	FUP2DownloadSpeed int64 `json:"fup2_download_speed"`
	FUP2UploadSpeed   int64 `json:"fup2_upload_speed"`
	FUP3Threshold     int64 `json:"fup3_threshold"`
	FUP3DownloadSpeed int64 `json:"fup3_download_speed"`
	FUP3UploadSpeed   int64 `json:"fup3_upload_speed"`
	FUP4Threshold     int64 `json:"fup4_threshold"`
	FUP4DownloadSpeed int64 `json:"fup4_download_speed"`
	FUP4UploadSpeed   int64 `json:"fup4_upload_speed"`
	FUP5Threshold     int64 `json:"fup5_threshold"`
	FUP5DownloadSpeed int64 `json:"fup5_download_speed"`
	FUP5UploadSpeed   int64 `json:"fup5_upload_speed"`
	FUP6Threshold     int64 `json:"fup6_threshold"`
	FUP6DownloadSpeed int64 `json:"fup6_download_speed"`
	FUP6UploadSpeed   int64 `json:"fup6_upload_speed"`
	// Monthly FUP (resets on renewal)
	MonthlyFUP1Threshold     int64 `json:"monthly_fup1_threshold"`
	MonthlyFUP1DownloadSpeed int64 `json:"monthly_fup1_download_speed"`
	MonthlyFUP1UploadSpeed   int64 `json:"monthly_fup1_upload_speed"`
	MonthlyFUP2Threshold     int64 `json:"monthly_fup2_threshold"`
	MonthlyFUP2DownloadSpeed int64 `json:"monthly_fup2_download_speed"`
	MonthlyFUP2UploadSpeed   int64 `json:"monthly_fup2_upload_speed"`
	MonthlyFUP3Threshold     int64 `json:"monthly_fup3_threshold"`
	MonthlyFUP3DownloadSpeed int64 `json:"monthly_fup3_download_speed"`
	MonthlyFUP3UploadSpeed   int64 `json:"monthly_fup3_upload_speed"`
	MonthlyFUP4Threshold     int64 `json:"monthly_fup4_threshold"`
	MonthlyFUP4DownloadSpeed int64 `json:"monthly_fup4_download_speed"`
	MonthlyFUP4UploadSpeed   int64 `json:"monthly_fup4_upload_speed"`
	MonthlyFUP5Threshold     int64 `json:"monthly_fup5_threshold"`
	MonthlyFUP5DownloadSpeed int64 `json:"monthly_fup5_download_speed"`
	MonthlyFUP5UploadSpeed   int64 `json:"monthly_fup5_upload_speed"`
	MonthlyFUP6Threshold     int64 `json:"monthly_fup6_threshold"`
	MonthlyFUP6DownloadSpeed int64 `json:"monthly_fup6_download_speed"`
	MonthlyFUP6UploadSpeed   int64 `json:"monthly_fup6_upload_speed"`
	// CDN FUP
	CDNFUPEnabled            bool  `json:"cdn_fup_enabled"`
	CDNFUP1Threshold         int64 `json:"cdn_fup1_threshold"`
	CDNFUP1DownloadSpeed     int64 `json:"cdn_fup1_download_speed"`
	CDNFUP1UploadSpeed       int64 `json:"cdn_fup1_upload_speed"`
	CDNFUP2Threshold         int64 `json:"cdn_fup2_threshold"`
	CDNFUP2DownloadSpeed     int64 `json:"cdn_fup2_download_speed"`
	CDNFUP2UploadSpeed       int64 `json:"cdn_fup2_upload_speed"`
	CDNFUP3Threshold         int64 `json:"cdn_fup3_threshold"`
	CDNFUP3DownloadSpeed     int64 `json:"cdn_fup3_download_speed"`
	CDNFUP3UploadSpeed       int64 `json:"cdn_fup3_upload_speed"`
	CDNMonthlyFUP1Threshold     int64 `json:"cdn_monthly_fup1_threshold"`
	CDNMonthlyFUP1DownloadSpeed int64 `json:"cdn_monthly_fup1_download_speed"`
	CDNMonthlyFUP1UploadSpeed   int64 `json:"cdn_monthly_fup1_upload_speed"`
	CDNMonthlyFUP2Threshold     int64 `json:"cdn_monthly_fup2_threshold"`
	CDNMonthlyFUP2DownloadSpeed int64 `json:"cdn_monthly_fup2_download_speed"`
	CDNMonthlyFUP2UploadSpeed   int64 `json:"cdn_monthly_fup2_upload_speed"`
	CDNMonthlyFUP3Threshold     int64 `json:"cdn_monthly_fup3_threshold"`
	CDNMonthlyFUP3DownloadSpeed int64 `json:"cdn_monthly_fup3_download_speed"`
	CDNMonthlyFUP3UploadSpeed   int64 `json:"cdn_monthly_fup3_upload_speed"`
	Price            float64 `json:"price"`
	DayPrice         float64 `json:"day_price"`
	ResetPrice       float64 `json:"reset_price"`
	ExpiryValue      int     `json:"expiry_value"`
	ExpiryUnit       int     `json:"expiry_unit"`
	EntireMonth      bool    `json:"entire_month"`
	MonthlyAccount   bool    `json:"monthly_account"`
	NasID            *uint   `json:"nas_id"`
	PoolName         string  `json:"pool_name"`
	AddressListIn    string  `json:"address_list_in"`
	AddressListOut   string  `json:"address_list_out"`
	QueueType        string  `json:"queue_type"`
	SortOrder        int     `json:"sort_order"`
	// Time-based speed control
	TimeBasedSpeedEnabled bool `json:"time_based_speed_enabled"`
	TimeFromHour          int  `json:"time_from_hour"`
	TimeFromMinute        int  `json:"time_from_minute"`
	TimeToHour            int  `json:"time_to_hour"`
	TimeToMinute          int  `json:"time_to_minute"`
	TimeDownloadRatio     int  `json:"time_download_ratio"`
	TimeUploadRatio       int  `json:"time_upload_ratio"`
}

// Create creates a new service
func (h *ServiceHandler) Create(c *fiber.Ctx) error {
	var req CreateServiceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Service name is required",
		})
	}

	// Check if name exists
	var existingCount int64
	database.DB.Model(&models.Service{}).Where("name = ?", req.Name).Count(&existingCount)
	if existingCount > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Service name already exists",
		})
	}

	// Convert speed strings from Mbps to kbps format for MikroTik compatibility
	// e.g., "1.4M" or "1.4" -> "1400k"
	downloadSpeedStr := convertSpeedForMikrotik(req.DownloadSpeedStr)
	uploadSpeedStr := convertSpeedForMikrotik(req.UploadSpeedStr)

	service := models.Service{
		Name:             req.Name,
		CommercialName:   req.CommercialName,
		Description:      req.Description,
		DownloadSpeed:    req.DownloadSpeed,
		UploadSpeed:      req.UploadSpeed,
		DownloadSpeedStr: downloadSpeedStr,
		UploadSpeedStr:   uploadSpeedStr,
		BurstDownload:    req.BurstDownload,
		BurstUpload:      req.BurstUpload,
		BurstThreshold:   req.BurstThreshold,
		BurstTime:        req.BurstTime,
		DailyQuota:       req.DailyQuota,
		MonthlyQuota:     req.MonthlyQuota,
		TimeQuota:        req.TimeQuota,
		// Multi-tier Daily FUP with direct speeds
		FUP1Threshold:     req.FUP1Threshold,
		FUP1DownloadSpeed: req.FUP1DownloadSpeed,
		FUP1UploadSpeed:   req.FUP1UploadSpeed,
		FUP2Threshold:     req.FUP2Threshold,
		FUP2DownloadSpeed: req.FUP2DownloadSpeed,
		FUP2UploadSpeed:   req.FUP2UploadSpeed,
		FUP3Threshold:     req.FUP3Threshold,
		FUP3DownloadSpeed: req.FUP3DownloadSpeed,
		FUP3UploadSpeed:   req.FUP3UploadSpeed,
		FUP4Threshold:     req.FUP4Threshold,
		FUP4DownloadSpeed: req.FUP4DownloadSpeed,
		FUP4UploadSpeed:   req.FUP4UploadSpeed,
		FUP5Threshold:     req.FUP5Threshold,
		FUP5DownloadSpeed: req.FUP5DownloadSpeed,
		FUP5UploadSpeed:   req.FUP5UploadSpeed,
		FUP6Threshold:     req.FUP6Threshold,
		FUP6DownloadSpeed: req.FUP6DownloadSpeed,
		FUP6UploadSpeed:   req.FUP6UploadSpeed,
		// Monthly FUP (resets on renewal)
		MonthlyFUP1Threshold:     req.MonthlyFUP1Threshold,
		MonthlyFUP1DownloadSpeed: req.MonthlyFUP1DownloadSpeed,
		MonthlyFUP1UploadSpeed:   req.MonthlyFUP1UploadSpeed,
		MonthlyFUP2Threshold:     req.MonthlyFUP2Threshold,
		MonthlyFUP2DownloadSpeed: req.MonthlyFUP2DownloadSpeed,
		MonthlyFUP2UploadSpeed:   req.MonthlyFUP2UploadSpeed,
		MonthlyFUP3Threshold:     req.MonthlyFUP3Threshold,
		MonthlyFUP3DownloadSpeed: req.MonthlyFUP3DownloadSpeed,
		MonthlyFUP3UploadSpeed:   req.MonthlyFUP3UploadSpeed,
		MonthlyFUP4Threshold:     req.MonthlyFUP4Threshold,
		MonthlyFUP4DownloadSpeed: req.MonthlyFUP4DownloadSpeed,
		MonthlyFUP4UploadSpeed:   req.MonthlyFUP4UploadSpeed,
		MonthlyFUP5Threshold:     req.MonthlyFUP5Threshold,
		MonthlyFUP5DownloadSpeed: req.MonthlyFUP5DownloadSpeed,
		MonthlyFUP5UploadSpeed:   req.MonthlyFUP5UploadSpeed,
		MonthlyFUP6Threshold:     req.MonthlyFUP6Threshold,
		MonthlyFUP6DownloadSpeed: req.MonthlyFUP6DownloadSpeed,
		MonthlyFUP6UploadSpeed:   req.MonthlyFUP6UploadSpeed,
		// CDN FUP
		CDNFUPEnabled:            req.CDNFUPEnabled,
		CDNFUP1Threshold:         req.CDNFUP1Threshold,
		CDNFUP1DownloadSpeed:     req.CDNFUP1DownloadSpeed,
		CDNFUP1UploadSpeed:       req.CDNFUP1UploadSpeed,
		CDNFUP2Threshold:         req.CDNFUP2Threshold,
		CDNFUP2DownloadSpeed:     req.CDNFUP2DownloadSpeed,
		CDNFUP2UploadSpeed:       req.CDNFUP2UploadSpeed,
		CDNFUP3Threshold:         req.CDNFUP3Threshold,
		CDNFUP3DownloadSpeed:     req.CDNFUP3DownloadSpeed,
		CDNFUP3UploadSpeed:       req.CDNFUP3UploadSpeed,
		CDNMonthlyFUP1Threshold:     req.CDNMonthlyFUP1Threshold,
		CDNMonthlyFUP1DownloadSpeed: req.CDNMonthlyFUP1DownloadSpeed,
		CDNMonthlyFUP1UploadSpeed:   req.CDNMonthlyFUP1UploadSpeed,
		CDNMonthlyFUP2Threshold:     req.CDNMonthlyFUP2Threshold,
		CDNMonthlyFUP2DownloadSpeed: req.CDNMonthlyFUP2DownloadSpeed,
		CDNMonthlyFUP2UploadSpeed:   req.CDNMonthlyFUP2UploadSpeed,
		CDNMonthlyFUP3Threshold:     req.CDNMonthlyFUP3Threshold,
		CDNMonthlyFUP3DownloadSpeed: req.CDNMonthlyFUP3DownloadSpeed,
		CDNMonthlyFUP3UploadSpeed:   req.CDNMonthlyFUP3UploadSpeed,
		Price:            req.Price,
		DayPrice:         req.DayPrice,
		ResetPrice:       req.ResetPrice,
		ExpiryValue:      req.ExpiryValue,
		ExpiryUnit:       models.ExpiryUnit(req.ExpiryUnit),
		EntireMonth:      req.EntireMonth,
		MonthlyAccount:   req.MonthlyAccount,
		NasID:            req.NasID,
		PoolName:         req.PoolName,
		AddressListIn:    req.AddressListIn,
		AddressListOut:   req.AddressListOut,
		QueueType:        req.QueueType,
		SortOrder:        req.SortOrder,
		// Time-based speed control
		TimeBasedSpeedEnabled: req.TimeBasedSpeedEnabled,
		TimeFromHour:          req.TimeFromHour,
		TimeFromMinute:        req.TimeFromMinute,
		TimeToHour:            req.TimeToHour,
		TimeToMinute:          req.TimeToMinute,
		TimeDownloadRatio:     req.TimeDownloadRatio,
		TimeUploadRatio:       req.TimeUploadRatio,
		IsActive:              true,
	}

	if service.ExpiryValue == 0 {
		service.ExpiryValue = 30
	}
	if service.ExpiryUnit == 0 {
		service.ExpiryUnit = models.ExpiryUnitDays
	}
	if service.QueueType == "" {
		service.QueueType = "simple"
	}
	if service.TimeDownloadRatio == 0 {
		service.TimeDownloadRatio = 100
	}
	if service.TimeUploadRatio == 0 {
		service.TimeUploadRatio = 100
	}

	if err := database.DB.Create(&service).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create service",
		})
	}

	// Create audit log
	user := middleware.GetCurrentUser(c)
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionCreate,
		EntityType:  "service",
		EntityID:    service.ID,
		EntityName:  service.Name,
		Description: "Created new service",
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Service created successfully",
		"data":    service,
	})
}

// Update updates a service
func (h *ServiceHandler) Update(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid service ID",
		})
	}

	var service models.Service
	if err := database.DB.First(&service, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Service not found",
		})
	}

	var req map[string]interface{}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Update allowed fields
	allowedFields := []string{
		"name", "commercial_name", "description",
		"download_speed", "upload_speed", "download_speed_str", "upload_speed_str",
		"burst_download", "burst_upload", "burst_threshold", "burst_time",
		"daily_quota", "monthly_quota", "time_quota",
		"fup1_threshold", "fup1_download_speed", "fup1_upload_speed",
		"fup2_threshold", "fup2_download_speed", "fup2_upload_speed",
		"fup3_threshold", "fup3_download_speed", "fup3_upload_speed",
		"fup4_threshold", "fup4_download_speed", "fup4_upload_speed",
		"fup5_threshold", "fup5_download_speed", "fup5_upload_speed",
		"fup6_threshold", "fup6_download_speed", "fup6_upload_speed",
		"monthly_fup1_threshold", "monthly_fup1_download_speed", "monthly_fup1_upload_speed",
		"monthly_fup2_threshold", "monthly_fup2_download_speed", "monthly_fup2_upload_speed",
		"monthly_fup3_threshold", "monthly_fup3_download_speed", "monthly_fup3_upload_speed",
		"monthly_fup4_threshold", "monthly_fup4_download_speed", "monthly_fup4_upload_speed",
		"monthly_fup5_threshold", "monthly_fup5_download_speed", "monthly_fup5_upload_speed",
		"monthly_fup6_threshold", "monthly_fup6_download_speed", "monthly_fup6_upload_speed",
		"price", "day_price", "reset_price",
		"expiry_value", "expiry_unit", "entire_month", "monthly_account",
		"nas_id", "pool_name", "address_list_in", "address_list_out", "queue_type",
		"time_based_speed_enabled",
		"time_from_hour", "time_from_minute", "time_to_hour", "time_to_minute",
		"time_download_ratio", "time_upload_ratio",
		"sort_order", "is_active",
		"cdn_fup_enabled",
		"cdn_fup1_threshold", "cdn_fup1_download_speed", "cdn_fup1_upload_speed",
		"cdn_fup2_threshold", "cdn_fup2_download_speed", "cdn_fup2_upload_speed",
		"cdn_fup3_threshold", "cdn_fup3_download_speed", "cdn_fup3_upload_speed",
		"cdn_monthly_fup1_threshold", "cdn_monthly_fup1_download_speed", "cdn_monthly_fup1_upload_speed",
		"cdn_monthly_fup2_threshold", "cdn_monthly_fup2_download_speed", "cdn_monthly_fup2_upload_speed",
		"cdn_monthly_fup3_threshold", "cdn_monthly_fup3_download_speed", "cdn_monthly_fup3_upload_speed",
	}

	updates := make(map[string]interface{})
	for _, field := range allowedFields {
		if val, ok := req[field]; ok {
			// Convert speed strings from Mbps to kbps format
			if field == "download_speed_str" || field == "upload_speed_str" {
				if strVal, ok := val.(string); ok {
					updates[field] = convertSpeedForMikrotik(strVal)
					continue
				}
			}
			updates[field] = val
		}
	}

	if err := database.DB.Model(&service).Updates(updates).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update service",
		})
	}

	// Update RADIUS reply for all subscribers using this service
	// TODO: Update Mikrotik-Rate-Limit for affected subscribers

	// Create audit log
	user := middleware.GetCurrentUser(c)
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionUpdate,
		EntityType:  "service",
		EntityID:    service.ID,
		EntityName:  service.Name,
		Description: "Updated service",
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	database.DB.First(&service, id)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Service updated successfully",
		"data":    service,
	})
}

// Delete deletes a service
func (h *ServiceHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid service ID",
		})
	}

	var service models.Service
	if err := database.DB.First(&service, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Service not found",
		})
	}

	// Check if service is in use
	var subscriberCount int64
	database.DB.Model(&models.Subscriber{}).Where("service_id = ?", id).Count(&subscriberCount)
	if subscriberCount > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cannot delete service with active subscribers",
		})
	}

	if err := database.DB.Delete(&service).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to delete service",
		})
	}

	// Create audit log
	user := middleware.GetCurrentUser(c)
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionDelete,
		EntityType:  "service",
		EntityID:    service.ID,
		EntityName:  service.Name,
		Description: "Deleted service",
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Service deleted successfully",
	})
}
