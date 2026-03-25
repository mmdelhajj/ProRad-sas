package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/ippool"
	"github.com/proisp/backend/internal/license"
	"github.com/proisp/backend/internal/middleware"
	"github.com/proisp/backend/internal/mikrotik"
	"github.com/proisp/backend/internal/models"
	"github.com/proisp/backend/internal/radius"
	"github.com/proisp/backend/internal/security"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type SubscriberHandler struct{}

// checkResellerServiceLimit checks if a reseller has reached the subscriber limit for a specific service
func checkResellerServiceLimit(db *gorm.DB, resellerID uint, serviceID uint) error {
	var limit models.ResellerServiceLimit
	err := db.Where("reseller_id = ? AND service_id = ?", resellerID, serviceID).First(&limit).Error
	if err != nil {
		return nil // No limit set = unlimited
	}
	if limit.MaxSubscribers == 0 {
		return nil // 0 = unlimited
	}
	var count int64
	db.Model(&models.Subscriber{}).Where("reseller_id = ? AND service_id = ? AND status != 'deleted'", resellerID, serviceID).Count(&count)
	if count >= int64(limit.MaxSubscribers) {
		return fmt.Errorf("Reseller limit reached: maximum %d subscribers allowed for this service", limit.MaxSubscribers)
	}
	return nil
}

func NewSubscriberHandler() *SubscriberHandler {
	return &SubscriberHandler{}
}

// checkUserPermission checks if a user has a specific permission
// Admins always have all permissions
func checkUserPermission(user *models.User, permission string) bool {
	if user.UserType == models.UserTypeAdmin {
		return true
	}

	if user.ResellerID == nil {
		return false
	}

	// Get reseller's permission group
	var reseller models.Reseller
	if err := database.DB.First(&reseller, *user.ResellerID).Error; err != nil {
		return false
	}

	// If no permission group assigned, allow all (default behavior for backward compatibility)
	if reseller.PermissionGroup == nil {
		return true
	}

	// Check if permission exists in the group
	var count int64
	database.DB.Table("permissions").
		Joins("JOIN permission_group_permissions pgp ON pgp.permission_id = permissions.id").
		Where("pgp.permission_group_id = ? AND permissions.name = ?", *reseller.PermissionGroup, permission).
		Count(&count)

	return count > 0
}

// disconnectSubscriberByCoA sends CoA disconnect to force user offline
// Used when assigning static IP that's currently in use by another user
func disconnectSubscriberByCoA(sub *models.Subscriber) {
	if sub == nil || sub.NasID == nil || sub.SessionID == "" {
		log.Printf("Cannot disconnect subscriber %s: missing NAS or session info", sub.Username)
		return
	}

	// Get NAS info
	var nas models.Nas
	if err := database.DB.First(&nas, *sub.NasID).Error; err != nil {
		log.Printf("Cannot disconnect %s: NAS not found", sub.Username)
		return
	}

	// Try CoA disconnect via radclient
	coaClient := radius.NewCOAClient(nas.IPAddress, nas.CoAPort, nas.Secret)
	if err := coaClient.DisconnectViaRadclient(sub.Username, sub.SessionID); err != nil {
		log.Printf("CoA disconnect failed for %s: %v, trying MikroTik API", sub.Username, err)

		// Fallback to MikroTik API
		if nas.APIUsername != "" && nas.APIPassword != "" {
			client := mikrotik.NewClient(nas.IPAddress, nas.APIUsername, nas.APIPassword)
			if err := client.DisconnectUser(sub.Username); err != nil {
				log.Printf("MikroTik API disconnect also failed for %s: %v", sub.Username, err)
			} else {
				log.Printf("Disconnected %s via MikroTik API (static IP conflict)", sub.Username)
			}
		}
	} else {
		log.Printf("Disconnected %s via CoA (static IP conflict)", sub.Username)
	}

	// Mark as offline in database
	database.DB.Model(&models.Subscriber{}).Where("id = ?", sub.ID).Updates(map[string]interface{}{
		"is_online":  false,
		"session_id": "",
	})
}

// ListRequest represents list request params
type ListRequest struct {
	Page     int    `query:"page"`
	Limit    int    `query:"limit"`
	Search   string `query:"search"`
	Status   string `query:"status"`
	Service  uint   `query:"service"`
	Reseller uint   `query:"reseller"`
	Online   string `query:"online"`
	SortBy   string `query:"sort_by"`
	SortDir  string `query:"sort_dir"`
}

// List returns all subscribers with pagination
func (h *SubscriberHandler) List(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	// Parse query params
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "25"))
	search := c.Query("search", "")
	status := c.Query("status", "")
	serviceID, _ := strconv.Atoi(c.Query("service_id", "0"))
	nasID, _ := strconv.Atoi(c.Query("nas_id", "0"))
	online := c.Query("online", "")
	fupLevel := c.Query("fup_level", "")
	sortBy := c.Query("sort_by", "created_at")
	sortDir := c.Query("sort_dir", "desc")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 25
	}

	offset := (page - 1) * limit

	// Build query (no Preload - manually load relations to avoid garble/GORM issues)
	query := database.DB.Model(&models.Subscriber{})

	// Filter by reseller for non-admin users (unless they have view_all permission)
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		if !checkUserPermission(user, "subscribers.view_all") {
			// Get reseller and their sub-resellers
			query = query.Where("reseller_id IN (SELECT id FROM resellers WHERE id = ? OR parent_id = ?)", *user.ResellerID, *user.ResellerID)
		}
	}

	// Search filter
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where(
			"username ILIKE ? OR full_name ILIKE ? OR phone ILIKE ? OR email ILIKE ? OR address ILIKE ? OR mac_address ILIKE ? OR ip_address ILIKE ?",
			searchPattern, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern,
		)
	}

	// Status filter
	if status != "" {
		switch status {
		case "active":
			query = query.Where("status = ?", models.SubscriberStatusActive)
		case "inactive":
			query = query.Where("status = ?", models.SubscriberStatusInactive)
		case "expired":
			query = query.Where("expiry_date < ?", time.Now())
		case "expiring":
			query = query.Where("expiry_date BETWEEN ? AND ?", time.Now(), time.Now().AddDate(0, 0, 7))
		case "online":
			query = query.Where("is_online = ?", true)
		case "offline":
			query = query.Where("is_online = ?", false)
		}
	}

	// Service filter
	if serviceID > 0 {
		query = query.Where("service_id = ?", serviceID)
	}

	// Online filter
	if online != "" {
		isOnline := online == "true" || online == "1"
		query = query.Where("is_online = ?", isOnline)
	}

	// NAS filter
	if nasID > 0 {
		query = query.Where("nas_id = ?", nasID)
	}

	// FUP level filter
	if fupLevel != "" {
		fupLevelInt, err := strconv.Atoi(fupLevel)
		if err == nil && fupLevelInt >= 0 && fupLevelInt <= 6 {
			query = query.Where("fup_level = ?", fupLevelInt)
		}
	}

	// Monthly FUP filter
	if c.Query("monthly_fup") == "true" {
		query = query.Where("monthly_fup_level >= 1")
	}

	// Reseller filter (for admin to filter by specific reseller)
	filterResellerID, _ := strconv.Atoi(c.Query("reseller_id", "0"))
	if filterResellerID > 0 {
		// Include the reseller and their sub-resellers
		query = query.Where("reseller_id IN (SELECT id FROM resellers WHERE id = ? OR parent_id = ?)", filterResellerID, filterResellerID)
	}

	// Count total (use a fresh session clone to avoid GORM statement mutation affecting the main query)
	var total int64
	query.Session(&gorm.Session{}).Count(&total)

	// Apply sorting
	if sortBy == "daily_usage" {
		query = query.Order("(daily_download_used + daily_upload_used) DESC")
	} else if sortBy == "monthly_usage" {
		query = query.Order("(monthly_download_used + monthly_upload_used) DESC")
	} else {
		allowedSortFields := map[string]bool{
			"username": true, "full_name": true, "created_at": true, "expiry_date": true, "is_online": true,
		}
		if !allowedSortFields[sortBy] {
			sortBy = "created_at"
		}
		if sortDir != "asc" && sortDir != "desc" {
			sortDir = "desc"
		}
		query = query.Order(fmt.Sprintf("%s %s", sortBy, sortDir))
	}

	// Fetch subscribers
	var subscribers []models.Subscriber
	if err := query.Offset(offset).Limit(limit).Find(&subscribers).Error; err != nil {
		log.Printf("ERROR: Failed to fetch subscribers: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch subscribers: " + err.Error(),
		})
	}

	// Manually load Service and Reseller relations (to avoid garble/GORM Preload issues)
	if len(subscribers) > 0 {
		// Collect unique IDs
		serviceIDs := make(map[uint]bool)
		resellerIDs := make(map[uint]bool)
		for _, s := range subscribers {
			if s.ServiceID > 0 {
				serviceIDs[s.ServiceID] = true
			}
			if s.ResellerID > 0 {
				resellerIDs[s.ResellerID] = true
			}
		}

		// Load services
		var services []models.Service
		if len(serviceIDs) > 0 {
			ids := make([]uint, 0, len(serviceIDs))
			for id := range serviceIDs {
				ids = append(ids, id)
			}
			if err := database.DB.Where("id IN ?", ids).Find(&services).Error; err != nil {
				log.Printf("ERROR: Failed to load services: %v", err)
			}
		}
		serviceMap := make(map[uint]*models.Service)
		for i := range services {
			serviceMap[services[i].ID] = &services[i]
		}

		// Load resellers with User and Parent info
		var resellers []models.Reseller
		if len(resellerIDs) > 0 {
			ids := make([]uint, 0, len(resellerIDs))
			for id := range resellerIDs {
				ids = append(ids, id)
			}
			if err := database.DB.Preload("User").Preload("Parent").Preload("Parent.User").Where("id IN ?", ids).Find(&resellers).Error; err != nil {
				log.Printf("ERROR: Failed to load resellers: %v", err)
			}
		}
		resellerMap := make(map[uint]*models.Reseller)
		for i := range resellers {
			resellerMap[resellers[i].ID] = &resellers[i]
		}

		// Assign to subscribers
		for i := range subscribers {
			if svc, ok := serviceMap[subscribers[i].ServiceID]; ok {
				subscribers[i].Service = svc
			}
			if res, ok := resellerMap[subscribers[i].ResellerID]; ok {
				subscribers[i].Reseller = res
			}
		}
	}

	// Calculate stats
	var stats struct {
		Total      int64 `json:"total"`
		Online     int64 `json:"online"`
		Offline    int64 `json:"offline"`
		Active     int64 `json:"active"`
		Inactive   int64 `json:"inactive"`
		Expired    int64 `json:"expired"`
		Expiring   int64 `json:"expiring"`
		FUP0       int64 `json:"fup0"`
		FUP1       int64 `json:"fup1"`
		FUP2       int64 `json:"fup2"`
		FUP3       int64 `json:"fup3"`
		FUP4       int64 `json:"fup4"`
		FUP5       int64 `json:"fup5"`
		FUP6       int64 `json:"fup6"`
		MonthlyFUP int64 `json:"monthly_fup"`
	}

	// Build reseller filter condition (unless they have view_all permission)
	resellerFilter := ""
	var resellerID uint
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		if !checkUserPermission(user, "subscribers.view_all") {
			resellerFilter = "reseller_id IN (SELECT id FROM resellers WHERE id = ? OR parent_id = ?)"
			resellerID = *user.ResellerID
		}
	}

	// Helper function to create filtered query
	filteredQuery := func() *gorm.DB {
		q := database.DB.Model(&models.Subscriber{})
		if resellerFilter != "" {
			q = q.Where(resellerFilter, resellerID, resellerID)
		}
		return q
	}

	filteredQuery().Count(&stats.Total)
	filteredQuery().Where("is_online = true").Count(&stats.Online)
	stats.Offline = stats.Total - stats.Online
	filteredQuery().Where("status = ?", models.SubscriberStatusActive).Count(&stats.Active)
	filteredQuery().Where("status = ?", models.SubscriberStatusInactive).Count(&stats.Inactive)
	filteredQuery().Where("expiry_date < ?", time.Now()).Count(&stats.Expired)
	filteredQuery().Where("expiry_date BETWEEN ? AND ?", time.Now(), time.Now().AddDate(0, 0, 7)).Count(&stats.Expiring)

	// FUP level stats - also filtered by reseller
	filteredQuery().Where("fup_level = ?", 0).Count(&stats.FUP0)
	filteredQuery().Where("fup_level = ?", 1).Count(&stats.FUP1)
	filteredQuery().Where("fup_level = ?", 2).Count(&stats.FUP2)
	filteredQuery().Where("fup_level = ?", 3).Count(&stats.FUP3)
	filteredQuery().Where("fup_level = ?", 4).Count(&stats.FUP4)
	filteredQuery().Where("fup_level = ?", 5).Count(&stats.FUP5)
	filteredQuery().Where("fup_level = ?", 6).Count(&stats.FUP6)
	filteredQuery().Where("monthly_fup_level >= ?", 1).Count(&stats.MonthlyFUP)

	// Include WAN check settings in meta so resellers can see WAN status
	// without needing settings.view permission
	var wanEnabled, wanPort string
	var wanPref models.SystemPreference
	if err := database.DB.Where("key = ?", "wan_check_enabled").First(&wanPref).Error; err == nil {
		wanEnabled = wanPref.Value
	}
	if err := database.DB.Where("key = ?", "wan_check_port").First(&wanPref).Error; err == nil {
		wanPort = wanPref.Value
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    subscribers,
		"meta": fiber.Map{
			"page":             page,
			"limit":            limit,
			"total":            total,
			"totalPages":       (total + int64(limit) - 1) / int64(limit),
			"wan_check_enabled": wanEnabled == "true" || wanEnabled == "1",
			"wan_check_port":   wanPort,
		},
		"stats": stats,
	})
}

// Get returns a single subscriber
func (h *SubscriberHandler) Get(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid subscriber ID",
		})
	}

	// Get password via direct map query first
	var passwordResult map[string]interface{}
	if err := database.DB.Raw("SELECT password_plain FROM subscribers WHERE id = ? AND deleted_at IS NULL", id).Scan(&passwordResult).Error; err != nil {
		log.Printf("Password query error: %v", err)
	}
	passwordPlain := ""
	if passwordResult != nil {
		if pw, ok := passwordResult["password_plain"].(string); ok {
			passwordPlain = pw
		}
	}

	var subscriber models.Subscriber
	// Use Table() instead of model directly to avoid GORM relation errors with garble
	if err := database.DB.Table("subscribers").Where("id = ? AND deleted_at IS NULL", id).First(&subscriber).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber not found",
		})
	}

	// Manually load relations (to avoid garble/GORM Preload issues)
	if subscriber.ServiceID > 0 {
		var service models.Service
		if database.DB.First(&service, subscriber.ServiceID).Error == nil {
			subscriber.Service = &service
		}
	}
	if subscriber.ResellerID > 0 {
		var reseller models.Reseller
		if database.DB.First(&reseller, subscriber.ResellerID).Error == nil {
			subscriber.Reseller = &reseller
		}
	}
	if subscriber.NasID != nil && *subscriber.NasID > 0 {
		var nas models.Nas
		if database.DB.First(&nas, *subscriber.NasID).Error == nil {
			subscriber.Nas = &nas
		}
	}
	if subscriber.SwitchID != nil && *subscriber.SwitchID > 0 {
		var sw models.Switch
		if database.DB.First(&sw, *subscriber.SwitchID).Error == nil {
			subscriber.Switch = &sw
		}
	}

	// Check access permission
	user := middleware.GetCurrentUser(c)
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		if subscriber.ResellerID != *user.ResellerID {
			// Check if subscriber belongs to a sub-reseller
			var count int64
			database.DB.Model(&models.Reseller{}).Where("id = ? AND parent_id = ?", subscriber.ResellerID, *user.ResellerID).Count(&count)
			if count == 0 {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"success": false,
					"message": "Access denied",
				})
			}
		}
	}

	// Get recent sessions from radacct
	var sessions []models.RadAcct
	if err := database.DB.Where("username = ?", subscriber.Username).Order("acctstarttime DESC").Limit(10).Find(&sessions).Error; err != nil {
		// Log error but continue - sessions are optional
		sessions = []models.RadAcct{}
	}

	// Use real-time quota data from subscribers table (updated every 30s by QuotaSync)
	now := time.Now()
	today := now.Format("2006-01-02")
	month := now.Format("2006-01")
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	dailyQuota := models.DailyQuota{
		SubscriberID: subscriber.ID,
		Date:         today,
		Download:     subscriber.DailyDownloadUsed,
		Upload:       subscriber.DailyUploadUsed,
		Total:        subscriber.DailyDownloadUsed + subscriber.DailyUploadUsed,
	}

	monthlyQuota := models.MonthlyQuota{
		SubscriberID: subscriber.ID,
		Month:        month,
		Download:     subscriber.MonthlyDownloadUsed,
		Upload:       subscriber.MonthlyUploadUsed,
		Total:        subscriber.MonthlyDownloadUsed + subscriber.MonthlyUploadUsed,
	}

	// Get daily breakdown for each day of the current month
	daysInMonth := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()
	dailyDownload := make([]int64, daysInMonth)
	dailyUpload := make([]int64, daysInMonth)

	// Step 1: Seed from radacct as a baseline (for days without history records)
	var dailyBreakdown []struct {
		Day         int
		TotalInput  int64
		TotalOutput int64
	}
	database.DB.Model(&models.RadAcct{}).
		Select("EXTRACT(DAY FROM acctstarttime)::int as day, COALESCE(SUM(acctinputoctets), 0) as total_input, COALESCE(SUM(acctoutputoctets), 0) as total_output").
		Where("username = ? AND acctstarttime >= ? AND acctstarttime < ?", subscriber.Username, startOfMonth, startOfMonth.AddDate(0, 1, 0)).
		Group("EXTRACT(DAY FROM acctstarttime)").
		Scan(&dailyBreakdown)

	for _, d := range dailyBreakdown {
		if d.Day >= 1 && d.Day <= daysInMonth {
			dailyDownload[d.Day-1] = d.TotalOutput // output = download
			dailyUpload[d.Day-1] = d.TotalInput    // input = upload
		}
	}

	// Step 2: Override past days with daily_usage_history (accurate: saved before daily reset,
	// so sessions spanning midnight are correctly attributed to the actual usage day).
	// radacct groups by session START time → sessions started on day N-1 but used on day N
	// would show on the wrong bar. History records what QuotaSync actually accumulated each day.
	var historyRows []struct {
		Date          time.Time
		DownloadBytes int64
		UploadBytes   int64
	}
	database.DB.Raw(`
		SELECT date, download_bytes, upload_bytes
		FROM daily_usage_history
		WHERE subscriber_id = ? AND date >= ? AND date < ?
	`, subscriber.ID, startOfMonth.Format("2006-01-02"), now.Format("2006-01-02")).Scan(&historyRows)

	for _, row := range historyRows {
		day := row.Date.Day()
		if day >= 1 && day <= daysInMonth {
			dailyDownload[day-1] = row.DownloadBytes
			dailyUpload[day-1] = row.UploadBytes
		}
	}

	// Step 3: Override today's bar with the real-time counter (not yet saved to history).
	todayDay := now.Day()
	if todayDay >= 1 && todayDay <= daysInMonth {
		if subscriber.DailyDownloadUsed > dailyDownload[todayDay-1] {
			dailyDownload[todayDay-1] = subscriber.DailyDownloadUsed
		}
		if subscriber.DailyUploadUsed > dailyUpload[todayDay-1] {
			dailyUpload[todayDay-1] = subscriber.DailyUploadUsed
		}
	}

	// Get quota limits from service
	var downloadLimit, uploadLimit, monthlyDownloadLimit, monthlyUploadLimit int64
	if subscriber.ServiceID > 0 {
		downloadLimit = subscriber.Service.DailyQuota
		uploadLimit = subscriber.Service.DailyQuota
		monthlyDownloadLimit = subscriber.Service.MonthlyQuota
		monthlyUploadLimit = subscriber.Service.MonthlyQuota
	}

	// Decrypt password for display in edit form
	decryptedPassword := security.DecryptPassword(passwordPlain)

	return c.JSON(fiber.Map{
		"success":       true,
		"data":          subscriber,
		"password":      decryptedPassword,
		"sessions":      sessions,
		"daily_quota": fiber.Map{
			"download_used":   dailyQuota.Download,
			"upload_used":     dailyQuota.Upload,
			"total_used":      dailyQuota.Total,
			"download_limit":  downloadLimit,
			"upload_limit":    uploadLimit,
			"daily_download":  dailyDownload,
			"daily_upload":    dailyUpload,
		},
		"monthly_quota": fiber.Map{
			"download_used":  monthlyQuota.Download,
			"upload_used":    monthlyQuota.Upload,
			"total_used":     monthlyQuota.Total,
			"download_limit": monthlyDownloadLimit,
			"upload_limit":   monthlyUploadLimit,
		},
	})
}

// CreateSubscriberRequest represents create request body
type CreateSubscriberRequest struct {
	Username             string  `json:"username"`
	Password             string  `json:"password"`
	FullName             string  `json:"full_name"`
	Email                string  `json:"email"`
	Phone                string  `json:"phone"`
	Address              string  `json:"address"`
	Region               string  `json:"region"`
	Building             string  `json:"building"`
	Nationality          string  `json:"nationality"`
	Country              string  `json:"country"`
	Note                 string  `json:"note"`
	ServiceID            uint    `json:"service_id"`
	ExpiryDays           int     `json:"expiry_days"`
	Price                float64 `json:"price"`
	OverridePrice        bool    `json:"override_price"`
	SwitchID             *uint   `json:"switch_id"`
	NasID                *uint   `json:"nas_id"`
	Latitude             float64 `json:"latitude"`
	Longitude            float64 `json:"longitude"`
	SimultaneousSessions int     `json:"simultaneous_sessions"`
	StaticIP             string  `json:"static_ip"`
	MACAddress           string  `json:"mac_address"`
	SaveMAC              bool    `json:"save_mac"`
}

// GetPassword returns the password for a subscriber (requires subscribers.view permission)
func (h *SubscriberHandler) GetPassword(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	var subscriber models.Subscriber
	query := database.DB.Where("id = ?", id)

	// Resellers can only view their own subscribers (unless they have view_all permission)
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		if !checkUserPermission(user, "subscribers.view_all") {
			query = query.Where("reseller_id IN (SELECT id FROM resellers WHERE id = ? OR parent_id = ?)", *user.ResellerID, *user.ResellerID)
		}
	}

	if err := query.First(&subscriber).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	// Decrypt password before returning
	decryptedPassword := security.DecryptPassword(subscriber.PasswordPlain)

	return c.JSON(fiber.Map{
		"success":  true,
		"password": decryptedPassword,
	})
}

// Create creates a new subscriber
func (h *SubscriberHandler) Create(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	var req CreateSubscriberRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Validate required fields
	if req.Username == "" || req.Password == "" || req.ServiceID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Username, password, and service are required",
		})
	}

	// Check subscriber limit with license server
	var currentSubCount int64
	database.DB.Model(&models.Subscriber{}).Count(&currentSubCount)

	allowed, msg, err := license.VerifySubscriberCount(int(currentSubCount))
	if err != nil {
		log.Printf("Warning: Subscriber verification error: %v", err)
		// Fall back to local check
		allowed, _, maxSubs, _ := license.CanAddSubscriber(int(currentSubCount))
		if !allowed {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": fmt.Sprintf("Subscriber limit reached (%d/%d). Please upgrade your license.", currentSubCount, maxSubs),
			})
		}
	} else if !allowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"message": msg,
		})
	}

	// Check if username exists
	var existingCount int64
	database.DB.Model(&models.Subscriber{}).Where("username = ?", req.Username).Count(&existingCount)
	if existingCount > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Username already exists",
		})
	}

	// Check if static IP is already assigned to another subscriber
	if req.StaticIP != "" {
		var staticIPCount int64
		database.DB.Model(&models.Subscriber{}).Where("static_ip = ?", req.StaticIP).Count(&staticIPCount)
		if staticIPCount > 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Static IP is already assigned to another subscriber",
			})
		}
		// If IP is currently in use by another subscriber (dynamic), auto-disconnect them
		var currentUser models.Subscriber
		if err := database.DB.Where("ip_address = ? AND is_online = ?", req.StaticIP, true).First(&currentUser).Error; err == nil {
			log.Printf("Static IP %s requested - auto-disconnecting current user %s", req.StaticIP, currentUser.Username)
			// Send CoA disconnect
			disconnectSubscriberByCoA(&currentUser)
		}
	}

	// Get service
	var service models.Service
	if err := database.DB.First(&service, req.ServiceID).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Service not found",
		})
	}

	// Get reseller ID
	var resellerID uint
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		resellerID = *user.ResellerID

		// Check balance
		var reseller models.Reseller
		database.DB.First(&reseller, resellerID)

		price := service.Price
		if req.OverridePrice && req.Price > 0 {
			price = req.Price
		}

		if reseller.Balance < price {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Insufficient balance",
			})
		}
	} else if user.UserType == models.UserTypeAdmin {
		// Admin can specify reseller
		resellerID = 1 // Default reseller
	}

	// Check reseller per-service subscriber limit
	if resellerID > 0 {
		if err := checkResellerServiceLimit(database.DB, resellerID, req.ServiceID); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": err.Error()})
		}
	}

	// Calculate expiry date
	expiryDays := req.ExpiryDays
	if expiryDays == 0 {
		if service.ExpiryUnit == models.ExpiryUnitMonths {
			expiryDays = service.ExpiryValue * 30
		} else {
			expiryDays = service.ExpiryValue
		}
	}
	expiryDate := time.Now().AddDate(0, 0, expiryDays)

	// Hash password
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

	// Set default simultaneous sessions if not provided
	simultaneousSessions := req.SimultaneousSessions
	if simultaneousSessions <= 0 {
		simultaneousSessions = 1
	}

	// Create subscriber
	subscriber := models.Subscriber{
		Username:             req.Username,
		Password:             string(hashedPassword),
		PasswordPlain:        security.EncryptPassword(req.Password), // Encrypted for RADIUS CHAP
		FullName:             req.FullName,
		Email:                req.Email,
		Phone:                req.Phone,
		Address:              req.Address,
		Region:               req.Region,
		Building:             req.Building,
		Nationality:          req.Nationality,
		Country:              req.Country,
		Note:                 req.Note,
		ServiceID:            req.ServiceID,
		Status:               models.SubscriberStatusActive,
		ExpiryDate:           expiryDate,
		Price:                service.Price,
		OverridePrice:        req.OverridePrice,
		ResellerID:           resellerID,
		SwitchID:             req.SwitchID,
		NasID:                req.NasID,
		Latitude:             req.Latitude,
		Longitude:            req.Longitude,
		SimultaneousSessions: simultaneousSessions,
		StaticIP:             req.StaticIP,
		MACAddress:           req.MACAddress,
		SaveMAC:              req.SaveMAC,
	}

	if req.OverridePrice && req.Price > 0 {
		subscriber.Price = req.Price
	}

	if err := database.DB.Create(&subscriber).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create subscriber",
		})
	}

	// Create RADIUS check attributes
	radCheck := []models.RadCheck{
		{Username: subscriber.Username, Attribute: "Cleartext-Password", Op: ":=", Value: req.Password},
		{Username: subscriber.Username, Attribute: "Expiration", Op: ":=", Value: expiryDate.Format("Jan 02 2006 15:04:05")},
		{Username: subscriber.Username, Attribute: "Simultaneous-Use", Op: ":=", Value: fmt.Sprintf("%d", simultaneousSessions)},
	}
	database.DB.Create(&radCheck)

	// Create RADIUS reply attributes
	var radReply []models.RadReply

	// Build rate limit string
	uploadSpeed := service.UploadSpeedStr
	downloadSpeed := service.DownloadSpeedStr
	if uploadSpeed == "" && service.UploadSpeed > 0 {
		uploadSpeed = fmt.Sprintf("%dM", service.UploadSpeed)
	}
	if downloadSpeed == "" && service.DownloadSpeed > 0 {
		downloadSpeed = fmt.Sprintf("%dM", service.DownloadSpeed)
	}
	if uploadSpeed != "" || downloadSpeed != "" {
		rateLimit := fmt.Sprintf("%s/%s", uploadSpeed, downloadSpeed)
		radReply = append(radReply, models.RadReply{Username: subscriber.Username, Attribute: "Mikrotik-Rate-Limit", Op: "=", Value: rateLimit})
	}

	if service.PoolName != "" {
		radReply = append(radReply, models.RadReply{Username: subscriber.Username, Attribute: "Framed-Pool", Op: "=", Value: service.PoolName})
	}
	if len(radReply) > 0 {
		database.DB.Create(&radReply)
	}

	// Deduct balance from reseller
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		database.DB.Model(&models.Reseller{}).Where("id = ?", resellerID).Update("balance", database.DB.Raw("balance - ?", subscriber.Price))

		// Create transaction
		transaction := models.Transaction{
			Type:         models.TransactionTypeNew,
			Amount:       -subscriber.Price,
			ResellerID:   resellerID,
			SubscriberID: &subscriber.ID,
			Description:  fmt.Sprintf("New subscriber: %s", subscriber.Username),
			IPAddress:    c.IP(),
			UserAgent:    c.Get("User-Agent"),
			CreatedBy:    user.ID,
		}
		database.DB.Create(&transaction)
	}

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionCreate,
		EntityType:  "subscriber",
		EntityID:    subscriber.ID,
		EntityName:  subscriber.Username,
		Description: "Created new subscriber",
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	// Sync static IP to MikroTik address list (for pool exclusion)
	if req.StaticIP != "" && req.NasID != nil {
		go func() {
			var nas models.Nas
			if err := database.DB.First(&nas, *req.NasID).Error; err == nil && nas.IsActive {
				client := mikrotik.NewClient(
					fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
					nas.APIUsername,
					nas.APIPassword,
				)
				if err := client.AddStaticIPToAddressList(req.StaticIP, subscriber.Username); err != nil {
					log.Printf("Failed to add static IP to MikroTik address-list: %v", err)
				}
				// Also reserve in PPP secrets to prevent pool from assigning this IP
				if err := client.ReserveStaticIPInPPP(req.StaticIP, subscriber.Username); err != nil {
					log.Printf("Failed to reserve static IP in MikroTik PPP: %v", err)
				}
				client.Close()
			}
		}()
	}

	// Load relations for response
	database.DB.Preload("Service").First(&subscriber, subscriber.ID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Subscriber created successfully",
		"data":    subscriber,
	})
}

// buildSubscriberChanges compares the old subscriber state against the updates map
// and returns a human-readable description of every field that changed.
func buildSubscriberChanges(old *models.Subscriber, updates map[string]interface{}, passwordChanged bool) string {
	fieldLabels := map[string]string{
		"full_name":             "Name",
		"email":                 "Email",
		"phone":                 "Phone",
		"address":               "Address",
		"region":                "Region",
		"building":              "Building",
		"nationality":           "Nationality",
		"country":               "Country",
		"note":                  "Note",
		"service_id":            "Service",
		"nas_id":                "NAS",
		"switch_id":             "Switch",
		"status":                "Status",
		"static_ip":             "Static IP",
		"simultaneous_sessions": "Sessions",
		"expiry_date":           "Expiry Date",
		"auto_renew":            "Auto Renew",
		"save_mac":              "Save MAC",
		"auto_recharge":         "Auto Recharge",
		"auto_recharge_days":    "Auto Recharge Days",
		"reseller_id":           "Reseller",
		"price":                 "Price",
		"override_price":        "Override Price",
		"latitude":              "Latitude",
		"longitude":             "Longitude",
	}
	statusLabels := map[int]string{1: "Active", 2: "Inactive", 3: "Expired", 4: "Stopped"}

	// Return current value of a field from the old subscriber struct as a display string.
	getOldValue := func(field string) string {
		switch field {
		case "full_name":
			return old.FullName
		case "email":
			return old.Email
		case "phone":
			return old.Phone
		case "address":
			return old.Address
		case "region":
			return old.Region
		case "building":
			return old.Building
		case "nationality":
			return old.Nationality
		case "country":
			return old.Country
		case "note":
			return old.Note
		case "service_id":
			return fmt.Sprintf("#%d", old.ServiceID)
		case "nas_id":
			if old.NasID != nil {
				return fmt.Sprintf("#%d", *old.NasID)
			}
			return "none"
		case "switch_id":
			if old.SwitchID != nil {
				return fmt.Sprintf("#%d", *old.SwitchID)
			}
			return "none"
		case "status":
			if label, ok := statusLabels[int(old.Status)]; ok {
				return label
			}
			return fmt.Sprintf("%d", int(old.Status))
		case "static_ip":
			return old.StaticIP
		case "simultaneous_sessions":
			return fmt.Sprintf("%d", old.SimultaneousSessions)
		case "expiry_date":
			return old.ExpiryDate.Format("2006-01-02")
		case "auto_renew":
			return fmt.Sprintf("%v", old.AutoRenew)
		case "save_mac":
			return fmt.Sprintf("%v", old.SaveMAC)
		case "auto_recharge":
			return fmt.Sprintf("%v", old.AutoRecharge)
		case "reseller_id":
			if old.ResellerID == 0 {
				return "none"
			}
			return fmt.Sprintf("#%d", old.ResellerID)
		case "price":
			return fmt.Sprintf("$%.2f", old.Price)
		case "override_price":
			return fmt.Sprintf("%v", old.OverridePrice)
		case "latitude":
			return fmt.Sprintf("%.6f", old.Latitude)
		case "longitude":
			return fmt.Sprintf("%.6f", old.Longitude)
		}
		return ""
	}

	// Format a new value from the updates map as a display string.
	getNewValue := func(field string, val interface{}) string {
		switch field {
		case "status":
			s := 0
			switch v := val.(type) {
			case int:
				s = v
			case float64:
				s = int(v)
			}
			if label, ok := statusLabels[s]; ok {
				return label
			}
			return fmt.Sprintf("%d", s)
		case "price":
			if f, ok := val.(float64); ok {
				return fmt.Sprintf("$%.2f", f)
			}
		case "expiry_date":
			if t, ok := val.(time.Time); ok {
				return t.Format("2006-01-02")
			}
		case "override_price", "auto_renew", "save_mac", "auto_recharge":
			if b, ok := val.(bool); ok {
				return fmt.Sprintf("%v", b)
			}
		case "service_id", "nas_id", "switch_id", "reseller_id":
			switch v := val.(type) {
			case int:
				return fmt.Sprintf("#%d", v)
			case float64:
				return fmt.Sprintf("#%d", int(v))
			}
		}
		return fmt.Sprintf("%v", val)
	}

	var changes []string
	if passwordChanged {
		changes = append(changes, "Password changed")
	}
	for field, newVal := range updates {
		if field == "password" || field == "password_plain" {
			continue
		}
		label, ok := fieldLabels[field]
		if !ok {
			label = field
		}
		oldStr := getOldValue(field)
		newStr := getNewValue(field, newVal)
		if oldStr == newStr {
			continue
		}
		if oldStr == "" || oldStr == "none" {
			changes = append(changes, fmt.Sprintf("%s set to %s", label, newStr))
		} else if newStr == "" || newStr == "none" || newStr == "0" {
			changes = append(changes, fmt.Sprintf("%s cleared (was %s)", label, oldStr))
		} else {
			changes = append(changes, fmt.Sprintf("%s: %s → %s", label, oldStr, newStr))
		}
	}
	if len(changes) == 0 {
		return fmt.Sprintf(`Updated subscriber "%s"`, old.Username)
	}
	return fmt.Sprintf(`Updated subscriber "%s": %s`, old.Username, strings.Join(changes, ", "))
}

// Update updates a subscriber
func (h *SubscriberHandler) Update(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid subscriber ID",
		})
	}

	var subscriber models.Subscriber
	if err := database.DB.First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber not found",
		})
	}

	var req map[string]interface{}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Check if static IP is being changed and if it's already used by another subscriber
	if staticIP, ok := req["static_ip"].(string); ok && staticIP != "" && staticIP != subscriber.StaticIP {
		var staticIPCount int64
		database.DB.Model(&models.Subscriber{}).Where("static_ip = ? AND id != ?", staticIP, id).Count(&staticIPCount)
		if staticIPCount > 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Static IP is already assigned to another subscriber",
			})
		}
		// If IP is currently in use by another subscriber (dynamic), auto-disconnect them
		var currentUser models.Subscriber
		if err := database.DB.Where("ip_address = ? AND is_online = ? AND id != ?", staticIP, true, id).First(&currentUser).Error; err == nil {
			log.Printf("Static IP %s requested - auto-disconnecting current user %s", staticIP, currentUser.Username)
			// Send CoA disconnect
			disconnectSubscriberByCoA(&currentUser)
		}
	}

	// Update allowed fields (username and mac_address are NOT allowed to be changed after creation)
	allowedFields := []string{
		"full_name", "email", "phone", "address", "region", "building",
		"nationality", "country", "note", "service_id", "switch_id", "nas_id",
		"latitude", "longitude", "save_mac", "auto_recharge", "auto_recharge_days",
		"status", "static_ip", "simultaneous_sessions", "expiry_date",
		"auto_renew", "auto_invoice", "reseller_id",
		"price", "override_price",
	}

	// Store old values for RADIUS and MikroTik updates
	oldUsername := subscriber.Username
	oldStaticIP := subscriber.StaticIP
	oldServiceID := subscriber.ServiceID

	updates := make(map[string]interface{})
	for _, field := range allowedFields {
		if val, ok := req[field]; ok {
			// Handle type conversions
			switch field {
			case "service_id", "nas_id", "reseller_id", "switch_id", "status", "simultaneous_sessions":
				// Convert float64 to int for integer fields
				if f, ok := val.(float64); ok {
					updates[field] = int(f)
				} else if str, ok := val.(string); ok && str != "" {
					// Handle string numbers
					if i, err := strconv.Atoi(str); err == nil {
						updates[field] = i
					}
				} else if val == "" || val == nil {
					// Skip empty values for optional fields
					if field != "service_id" && field != "status" {
						continue
					}
				} else {
					updates[field] = val
				}
			case "expiry_date":
				// Convert string to time.Time
				if str, ok := val.(string); ok && str != "" {
					if t, err := time.Parse("2006-01-02", str); err == nil {
						updates[field] = t
					} else if t, err := time.Parse(time.RFC3339, str); err == nil {
						updates[field] = t
					}
				}
			case "auto_renew", "save_mac", "auto_recharge":
				// Ensure boolean values
				if b, ok := val.(bool); ok {
					updates[field] = b
				}
			case "static_ip":
				// Allow empty string to clear static IP
				if str, ok := val.(string); ok {
					updates[field] = str
				}
			case "price":
				if f, ok := val.(float64); ok {
					updates[field] = f
				}
			case "override_price":
				if b, ok := val.(bool); ok {
					updates[field] = b
				}
			default:
				// Skip empty strings for optional fields
				if str, ok := val.(string); ok && str == "" {
					continue
				}
				updates[field] = val
			}
		}
	}

	// Handle password update
	var passwordToUpdate string
	var passwordChanged bool
	if password, ok := req["password"].(string); ok && password != "" {
		// Skip if password is already encrypted (user didn't change it)
		if strings.HasPrefix(password, "ENC:") {
			// Don't update password - it's the encrypted value from the form
		} else {
			// Compare against current plaintext password to detect actual changes.
			// The frontend always sends the decrypted password back, so we must compare
			// to avoid marking unchanged passwords as "Password changed" in audit log.
			currentPlain := security.DecryptPassword(subscriber.PasswordPlain)
			if password != currentPlain {
				hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
				updates["password"] = string(hashedPassword)
				updates["password_plain"] = security.EncryptPassword(password)
				passwordToUpdate = password
				passwordChanged = true
			}
		}
	}

	// Snapshot old subscriber state before applying updates (used for audit diff)
	oldSubscriber := subscriber

	if err := database.DB.Model(&subscriber).Updates(updates).Error; err != nil {
		fmt.Printf("Update error: %v, updates: %+v\n", err, updates)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update subscriber: " + err.Error(),
		})
	}

	// Determine the current username (may have been updated)
	currentUsername := oldUsername
	if newUsername, ok := req["username"].(string); ok && newUsername != "" && newUsername != oldUsername {
		// Update radcheck
		database.DB.Model(&models.RadCheck{}).Where("username = ?", oldUsername).Update("username", newUsername)
		// Update radreply
		database.DB.Model(&models.RadReply{}).Where("username = ?", oldUsername).Update("username", newUsername)
		// Update radacct (accounting records)
		database.DB.Exec("UPDATE radacct SET username = ? WHERE username = ?", newUsername, oldUsername)
		currentUsername = newUsername
	}

	// Update RADIUS password if changed
	if passwordToUpdate != "" {
		database.DB.Where("username = ? AND attribute = ?", currentUsername, "Cleartext-Password").Delete(&models.RadCheck{})
		database.DB.Create(&models.RadCheck{Username: currentUsername, Attribute: "Cleartext-Password", Op: ":=", Value: passwordToUpdate})
	}

	// Update Simultaneous-Use if changed
	if simSessions, ok := updates["simultaneous_sessions"]; ok {
		var simValue int
		switch v := simSessions.(type) {
		case int:
			simValue = v
		case float64:
			simValue = int(v)
		}
		if simValue <= 0 {
			simValue = 1
		}
		database.DB.Where("username = ? AND attribute = ?", currentUsername, "Simultaneous-Use").Delete(&models.RadCheck{})
		database.DB.Create(&models.RadCheck{Username: currentUsername, Attribute: "Simultaneous-Use", Op: ":=", Value: fmt.Sprintf("%d", simValue)})
	}

	// Pass detailed audit description to middleware via Fiber locals.
	// The middleware creates the actual DB entry after c.Next() returns.
	// (Direct database.DB.Create fails silently when OldValue/NewValue are empty jsonb columns)
	c.Locals("audit_description", buildSubscriberChanges(&oldSubscriber, updates, passwordChanged))
	c.Locals("audit_entity_id", subscriber.ID)
	c.Locals("audit_entity_name", subscriber.Username)

	// Sync static IP changes to MikroTik address list
	if newStaticIP, ok := req["static_ip"].(string); ok {
		// Reload subscriber to get the updated NAS ID
		database.DB.First(&subscriber, id)

		// If static IP changed, update MikroTik address list
		if newStaticIP != oldStaticIP && subscriber.NasID != nil {
			go func() {
				var nas models.Nas
				if err := database.DB.First(&nas, *subscriber.NasID).Error; err == nil && nas.IsActive {
					client := mikrotik.NewClient(
						fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
						nas.APIUsername,
						nas.APIPassword,
					)
					// Remove old static IP if it existed
					if oldStaticIP != "" {
						if err := client.RemoveStaticIPFromAddressList(oldStaticIP); err != nil {
							log.Printf("Failed to remove old static IP from MikroTik: %v", err)
						}
						if err := client.RemoveStaticIPReservation(oldStaticIP); err != nil {
							log.Printf("Failed to remove old static IP PPP reservation: %v", err)
						}
					}
					// Add new static IP if set
					if newStaticIP != "" {
						if err := client.AddStaticIPToAddressList(newStaticIP, subscriber.Username); err != nil {
							log.Printf("Failed to add static IP to MikroTik address-list: %v", err)
						}
						// Reserve in PPP secrets to prevent pool from assigning this IP
						if err := client.ReserveStaticIPInPPP(newStaticIP, subscriber.Username); err != nil {
							log.Printf("Failed to reserve static IP in MikroTik PPP: %v", err)
						}
					}
					client.Close()
				}
			}()
		}
	}

	// If service changed, remove old Framed-IP-Address so user gets new IP from correct pool
	database.DB.First(&subscriber, id)
	if subscriber.ServiceID != oldServiceID {
		// Delete old static IP from radreply - on reconnect, RADIUS will allocate from new pool
		database.DB.Where("username = ? AND attribute = ?", subscriber.Username, "Framed-IP-Address").Delete(&models.RadReply{})
		// Release old IP in pool tracking
		ippool.ReleaseIPForUser(subscriber.Username)
		log.Printf("ServiceChange: Cleared Framed-IP-Address for %s (service changed, will get new IP from correct pool)", subscriber.Username)
	}

	// Auto-disconnect user if service changed (so they reconnect with new pool IP)
	if subscriber.ServiceID != oldServiceID && subscriber.NasID != nil {
		// Service changed - disconnect user via MikroTik API so they reconnect with new pool
		go func(username string, nasID uint) {
			var nas models.Nas
			if err := database.DB.First(&nas, nasID).Error; err != nil {
				log.Printf("ServiceChange: Failed to find NAS %d for disconnect: %v", nasID, err)
				return
			}

			log.Printf("ServiceChange: Service changed for %s, disconnecting from NAS %s", username, nas.IPAddress)

			// Try MikroTik API first (most reliable for PPPoE)
			client := mikrotik.NewClient(
				fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
				nas.APIUsername,
				nas.APIPassword,
			)
			if err := client.DisconnectUser(username); err != nil {
				log.Printf("ServiceChange: MikroTik API disconnect failed for %s: %v, trying CoA", username, err)

				// Fallback: try CoA Disconnect-Request
				coaClient := radius.NewCOAClient(nas.IPAddress, nas.CoAPort, nas.Secret)
				if err := coaClient.DisconnectUser(username, ""); err != nil {
					log.Printf("ServiceChange: CoA disconnect also failed for %s: %v", username, err)
				} else {
					log.Printf("ServiceChange: Disconnected %s via CoA (service changed)", username)
				}
			} else {
				log.Printf("ServiceChange: Disconnected %s via MikroTik API (service changed, will reconnect with new pool)", username)
			}
			client.Close()
		}(subscriber.Username, *subscriber.NasID)
	}

	database.DB.First(&subscriber, id)

	// Manually load relations
	if subscriber.ServiceID > 0 {
		var service models.Service
		if database.DB.First(&service, subscriber.ServiceID).Error == nil {
			subscriber.Service = &service
		}
	}
	if subscriber.ResellerID > 0 {
		var reseller models.Reseller
		if database.DB.First(&reseller, subscriber.ResellerID).Error == nil {
			subscriber.Reseller = &reseller
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Subscriber updated successfully",
		"data":    subscriber,
	})
}

// Delete deletes a subscriber
func (h *SubscriberHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid subscriber ID",
		})
	}

	var subscriber models.Subscriber
	if err := database.DB.First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber not found",
		})
	}

	// Delete RADIUS attributes
	database.DB.Where("username = ?", subscriber.Username).Delete(&models.RadCheck{})
	database.DB.Where("username = ?", subscriber.Username).Delete(&models.RadReply{})
	database.DB.Where("username = ?", subscriber.Username).Delete(&models.RadUserGroup{})

	// Soft delete subscriber + set deleted_by fields in one operation
	user := middleware.GetCurrentUser(c)
	now := time.Now()
	if err := database.DB.Model(&subscriber).Updates(map[string]interface{}{
		"deleted_by_id":   user.ID,
		"deleted_by_name": user.Username,
		"deleted_at":      now,
	}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to delete subscriber",
		})
	}

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionDelete,
		EntityType:  "subscriber",
		EntityID:    subscriber.ID,
		EntityName:  subscriber.Username,
		Description: "Deleted subscriber",
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Subscriber deleted successfully",
	})
}

// Renew renews a subscriber
func (h *SubscriberHandler) Renew(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	var subscriber models.Subscriber
	if err := database.DB.Preload("Service").First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	user := middleware.GetCurrentUser(c)

	// Check reseller balance
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		var reseller models.Reseller
		database.DB.First(&reseller, *user.ResellerID)
		if reseller.Balance < subscriber.Price {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Insufficient balance",
			})
		}
	}

	// Calculate new expiry
	var newExpiry time.Time
	if subscriber.ExpiryDate.After(time.Now()) {
		// Add days to current expiry
		if subscriber.Service.ExpiryUnit == models.ExpiryUnitMonths {
			newExpiry = subscriber.ExpiryDate.AddDate(0, subscriber.Service.ExpiryValue, 0)
		} else {
			newExpiry = subscriber.ExpiryDate.AddDate(0, 0, subscriber.Service.ExpiryValue)
		}
	} else {
		// Start from now
		if subscriber.Service.ExpiryUnit == models.ExpiryUnitMonths {
			newExpiry = time.Now().AddDate(0, subscriber.Service.ExpiryValue, 0)
		} else {
			newExpiry = time.Now().AddDate(0, 0, subscriber.Service.ExpiryValue)
		}
	}

	// Update subscriber
	subscriber.ExpiryDate = newExpiry
	subscriber.Status = models.SubscriberStatusActive

	// Reset daily and monthly FUP and usage on renewal
	now := time.Now()
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

	// If user is online, update session baseline to current MikroTik values
	// This prevents QuotaSync from adding back the old usage
	if subscriber.IsOnline && subscriber.NasID != nil {
		var nas models.Nas
		if database.DB.First(&nas, *subscriber.NasID).Error == nil {
			client := mikrotik.NewClient(
				fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
				nas.APIUsername,
				nas.APIPassword,
			)
			if session, err := client.GetActiveSession(subscriber.Username); err == nil {
				subscriber.LastSessionDownload = session.TxBytes
				subscriber.LastSessionUpload = session.RxBytes
				subscriber.LastQuotaSync = &now
				log.Printf("Renew: Updated session baseline for %s: dl=%d ul=%d", subscriber.Username, session.TxBytes, session.RxBytes)
			} else {
				log.Printf("Renew: Failed to get session for %s: %v", subscriber.Username, err)
			}
			client.Close()
		}
	} else {
		log.Printf("Renew: User %s is offline or no NAS, skipping session baseline update", subscriber.Username)
	}

	database.DB.Save(&subscriber)

	// Update RADIUS expiration
	database.DB.Where("username = ? AND attribute = ?", subscriber.Username, "Expiration").Delete(&models.RadCheck{})
	database.DB.Create(&models.RadCheck{
		Username:  subscriber.Username,
		Attribute: "Expiration",
		Op:        ":=",
		Value:     newExpiry.Format("Jan 02 2006 15:04:05"),
	})

	// Deduct balance
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		database.DB.Model(&models.Reseller{}).Where("id = ?", *user.ResellerID).Update("balance", database.DB.Raw("balance - ?", subscriber.Price))

		// Create transaction
		transaction := models.Transaction{
			Type:         models.TransactionTypeRenewal,
			Amount:       -subscriber.Price,
			ResellerID:   *user.ResellerID,
			SubscriberID: &subscriber.ID,
			Description:  fmt.Sprintf("Renewal: %s", subscriber.Username),
			IPAddress:    c.IP(),
			CreatedBy:    user.ID,
		}
		database.DB.Create(&transaction)
	}

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionRenew,
		EntityType:  "subscriber",
		EntityID:    subscriber.ID,
		EntityName:  subscriber.Username,
		Description: fmt.Sprintf("Renewed until %s", newExpiry.Format("2006-01-02")),
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Subscriber renewed successfully",
		"data": fiber.Map{
			"new_expiry": newExpiry,
		},
	})
}

// Disconnect disconnects a subscriber
func (h *SubscriberHandler) Disconnect(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	var subscriber models.Subscriber
	if err := database.DB.Preload("Nas").First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	// Disconnect via MikroTik API if NAS is configured
	if subscriber.Nas != nil && subscriber.Nas.IPAddress != "" {
		client := mikrotik.NewClient(
			fmt.Sprintf("%s:%d", subscriber.Nas.IPAddress, subscriber.Nas.APIPort),
			subscriber.Nas.APIUsername,
			subscriber.Nas.APIPassword,
		)
		defer client.Close()

		if err := client.DisconnectUser(subscriber.Username); err != nil {
			// Log error but continue to update database
			fmt.Printf("MikroTik disconnect error for %s: %v\n", subscriber.Username, err)
		}
	}

	// Update subscriber status
	subscriber.IsOnline = false
	subscriber.SessionID = ""
	database.DB.Save(&subscriber)

	// Create audit log
	user := middleware.GetCurrentUser(c)
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionDisconnect,
		EntityType:  "subscriber",
		EntityID:    subscriber.ID,
		EntityName:  subscriber.Username,
		Description: "Disconnected subscriber",
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Subscriber disconnected successfully",
	})
}

// ResetFUP resets subscriber's FUP
func (h *SubscriberHandler) ResetFUP(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	var subscriber models.Subscriber
	if err := database.DB.Preload("Nas").Preload("Service").First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	// Reset FUP level and daily quota counters only (not monthly)
	now := time.Now()
	updates := map[string]interface{}{
		"fup_level":           0,
		"daily_quota_used":    0,
		"daily_download_used": 0,
		"daily_upload_used":   0,
		"last_daily_reset":    now,
	}

	// If user is online, get current MikroTik session bytes as baseline
	// This prevents QuotaSync from recalculating all previous usage
	var client *mikrotik.Client
	var session *mikrotik.ActiveSession
	if subscriber.Nas != nil && subscriber.IsOnline {
		client = mikrotik.NewClient(
			fmt.Sprintf("%s:%d", subscriber.Nas.IPAddress, subscriber.Nas.APIPort),
			subscriber.Nas.APIUsername,
			subscriber.Nas.APIPassword,
		)
		defer client.Close()

		var err error
		session, err = client.GetActiveSession(subscriber.Username)
		if err != nil {
			log.Printf("ResetFUP: Failed to get session for %s: %v", subscriber.Username, err)
			// Fallback to 0 if we can't get session
			updates["last_session_download"] = 0
			updates["last_session_upload"] = 0
		} else {
			// Set current session bytes as baseline so delta will be 0
			updates["last_session_download"] = session.TxBytes
			updates["last_session_upload"] = session.RxBytes
			log.Printf("ResetFUP: Setting baseline for %s: dl=%d, ul=%d", subscriber.Username, session.TxBytes, session.RxBytes)
		}
	} else {
		updates["last_session_download"] = 0
		updates["last_session_upload"] = 0
	}

	database.DB.Model(&subscriber).Updates(updates)

	// Restore original speed in RADIUS radreply table (format: upload/download for MikroTik rx/tx)
	if subscriber.Service.ID > 0 {
		rateLimit := fmt.Sprintf("%dk/%dk", subscriber.Service.UploadSpeed, subscriber.Service.DownloadSpeed)
		database.DB.Model(&models.RadReply{}).
			Where("username = ? AND attribute = ?", subscriber.Username, "Mikrotik-Rate-Limit").
			Update("value", rateLimit)
	}

	// Restore original speed on MikroTik using CoA
	// Speeds are already in kb (e.g., 2000 = 2000k), no conversion needed
	if session != nil && subscriber.Service.ID > 0 {
		originalRateLimitK := fmt.Sprintf("%dk/%dk", subscriber.Service.UploadSpeed, subscriber.Service.DownloadSpeed)
		coaClient := radius.NewCOAClient(subscriber.Nas.IPAddress, subscriber.Nas.CoAPort, subscriber.Nas.Secret)
		speedRestored := false

		// Method 1: Try radclient-based CoA (most reliable)
		if err := coaClient.UpdateRateLimitViaRadclient(subscriber.Username, session.SessionID, originalRateLimitK); err != nil {
			log.Printf("ResetFUP: Radclient CoA failed for %s: %v", subscriber.Username, err)
		} else {
			log.Printf("ResetFUP: Restored %s speed via radclient CoA to %s", subscriber.Username, originalRateLimitK)
			speedRestored = true
		}

		// Method 2: Try MikroTik API as fallback
		if !speedRestored {
			if err := client.RestoreUserSpeedWithIP(subscriber.Username, session.Address, subscriber.Service.DownloadSpeed, subscriber.Service.UploadSpeed); err != nil {
				log.Printf("ResetFUP: MikroTik API restore failed for %s: %v", subscriber.Username, err)
			} else {
				log.Printf("ResetFUP: Restored %s speed via MikroTik API", subscriber.Username)
				speedRestored = true
			}
		}

		// Method 3: Try native Go CoA
		if !speedRestored {
			if err := coaClient.UpdateRateLimit(subscriber.Username, session.SessionID, originalRateLimitK); err != nil {
				log.Printf("ResetFUP: Native CoA failed for %s: %v", subscriber.Username, err)
			} else {
				log.Printf("ResetFUP: Restored %s speed via native CoA", subscriber.Username)
			}
		}
	}

	// Create audit log
	user := middleware.GetCurrentUser(c)
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionResetFUP,
		EntityType:  "subscriber",
		EntityID:    subscriber.ID,
		EntityName:  subscriber.Username,
		Description: "Reset FUP",
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "FUP reset successfully",
	})
}

// ResetMAC resets subscriber's MAC address
func (h *SubscriberHandler) ResetMAC(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	var subscriber models.Subscriber
	if err := database.DB.First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	oldMAC := subscriber.MACAddress
	subscriber.MACAddress = ""
	database.DB.Save(&subscriber)

	// Remove MAC from RADIUS
	database.DB.Where("username = ? AND attribute = ?", subscriber.Username, "Calling-Station-Id").Delete(&models.RadCheck{})

	// Create audit log
	user := middleware.GetCurrentUser(c)
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionResetMAC,
		EntityType:  "subscriber",
		EntityID:    subscriber.ID,
		EntityName:  subscriber.Username,
		OldValue:    oldMAC,
		Description: "Reset MAC address",
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "MAC address reset successfully",
	})
}

// ResetQuota resets subscriber's daily and monthly quota counters
func (h *SubscriberHandler) ResetQuota(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	// Get quota type from query (daily, monthly, or both)
	quotaType := c.Query("type", "both")

	var subscriber models.Subscriber
	if err := database.DB.First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	now := time.Now()
	updates := map[string]interface{}{
		"last_session_download": int64(0),
		"last_session_upload":   int64(0),
	}

	description := ""
	switch quotaType {
	case "daily":
		updates["daily_download_used"] = int64(0)
		updates["daily_upload_used"] = int64(0)
		updates["daily_quota_used"] = int64(0)
		updates["last_daily_reset"] = now
		description = "Reset daily quota"
	case "monthly":
		updates["monthly_download_used"] = int64(0)
		updates["monthly_upload_used"] = int64(0)
		updates["monthly_quota_used"] = int64(0)
		updates["last_monthly_reset"] = now
		description = "Reset monthly quota"
	default: // both
		updates["daily_download_used"] = int64(0)
		updates["daily_upload_used"] = int64(0)
		updates["daily_quota_used"] = int64(0)
		updates["monthly_download_used"] = int64(0)
		updates["monthly_upload_used"] = int64(0)
		updates["monthly_quota_used"] = int64(0)
		updates["last_daily_reset"] = now
		updates["last_monthly_reset"] = now
		description = "Reset all quota counters"
	}

	database.DB.Model(&subscriber).Updates(updates)

	// Create audit log
	user := middleware.GetCurrentUser(c)
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      "reset_quota",
		EntityType:  "subscriber",
		EntityID:    subscriber.ID,
		EntityName:  subscriber.Username,
		Description: description,
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Quota reset successfully",
	})
}

// BulkImportResult represents result for each row
type BulkImportResult struct {
	Row      int    `json:"row"`
	Username string `json:"username"`
	Success  bool   `json:"success"`
	Message  string `json:"message"`
}

// BulkImport imports subscribers from CSV
func (h *SubscriberHandler) BulkImport(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "No file uploaded",
		})
	}

	// Open file
	f, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Failed to open file",
		})
	}
	defer f.Close()

	// Parse CSV
	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1 // Allow variable fields

	// Read header
	header, err := reader.Read()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Failed to read CSV header",
		})
	}

	// Map header columns to indices
	colMap := make(map[string]int)
	for i, col := range header {
		colMap[strings.ToLower(strings.TrimSpace(col))] = i
	}

	// Get service ID from form
	serviceID, _ := strconv.Atoi(c.FormValue("service_id", "0"))
	if serviceID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Service ID is required",
		})
	}

	// Get service
	var service models.Service
	if err := database.DB.First(&service, serviceID).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Service not found",
		})
	}

	// Get reseller ID
	var resellerID uint
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		resellerID = *user.ResellerID
	} else {
		resellerID = 1
	}

	// Calculate expiry
	expiryDays := service.ExpiryValue
	if service.ExpiryUnit == models.ExpiryUnitMonths {
		expiryDays = service.ExpiryValue * 30
	}

	results := []BulkImportResult{}
	created := 0
	failed := 0
	row := 1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		row++

		if err != nil {
			results = append(results, BulkImportResult{Row: row, Success: false, Message: "Failed to read row"})
			failed++
			continue
		}

		// Get values from record
		getValue := func(names ...string) string {
			for _, name := range names {
				if idx, ok := colMap[name]; ok && idx < len(record) {
					return strings.TrimSpace(record[idx])
				}
			}
			return ""
		}

		username := getValue("username", "user")
		password := getValue("password", "pass")
		fullName := getValue("full_name", "fullname", "name")
		email := getValue("email")
		phone := getValue("phone", "mobile")
		address := getValue("address")

		if username == "" {
			results = append(results, BulkImportResult{Row: row, Success: false, Message: "Username is required"})
			failed++
			continue
		}

		if password == "" {
			password = username // Default password = username
		}

		// Check if username exists
		var existingCount int64
		database.DB.Model(&models.Subscriber{}).Where("username = ?", username).Count(&existingCount)
		if existingCount > 0 {
			results = append(results, BulkImportResult{Row: row, Username: username, Success: false, Message: "Username already exists"})
			failed++
			continue
		}

		// Create subscriber
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		expiryDate := time.Now().AddDate(0, 0, expiryDays)

		subscriber := models.Subscriber{
			Username:      username,
			Password:      string(hashedPassword),
			PasswordPlain: security.EncryptPassword(password),
			FullName:      fullName,
			Email:         email,
			Phone:         phone,
			Address:       address,
			ServiceID:     uint(serviceID),
			Status:        models.SubscriberStatusActive,
			ExpiryDate:    expiryDate,
			Price:         service.Price,
			ResellerID:    resellerID,
		}

		if err := database.DB.Create(&subscriber).Error; err != nil {
			results = append(results, BulkImportResult{Row: row, Username: username, Success: false, Message: "Failed to create"})
			failed++
			continue
		}

		// Create RADIUS attributes
		radCheck := []models.RadCheck{
			{Username: username, Attribute: "Cleartext-Password", Op: ":=", Value: password},
			{Username: username, Attribute: "Expiration", Op: ":=", Value: expiryDate.Format("Jan 02 2006 15:04:05")},
			{Username: username, Attribute: "Simultaneous-Use", Op: ":=", Value: "1"},
		}
		database.DB.Create(&radCheck)

		radReply := []models.RadReply{
			{Username: username, Attribute: "Mikrotik-Rate-Limit", Op: "=", Value: fmt.Sprintf("%s/%s", service.UploadSpeedStr, service.DownloadSpeedStr)},
		}
		database.DB.Create(&radReply)

		results = append(results, BulkImportResult{Row: row, Username: username, Success: true, Message: "Created"})
		created++
	}

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionCreate,
		EntityType:  "subscriber",
		Description: fmt.Sprintf("Bulk imported %d subscribers (%d failed)", created, failed),
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Imported %d subscribers, %d failed", created, failed),
		"data": fiber.Map{
			"created": created,
			"failed":  failed,
			"results": results,
		},
	})
}

// BulkUpdateRequest represents bulk update request
type BulkUpdateRequest struct {
	IDs     []uint                 `json:"ids"`
	Updates map[string]interface{} `json:"updates"`
}

// BulkUpdate updates multiple subscribers
func (h *SubscriberHandler) BulkUpdate(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	var req BulkUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if len(req.IDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "No subscribers selected",
		})
	}

	// Filter allowed fields
	allowedFields := map[string]bool{
		"service_id": true, "status": true, "nas_id": true,
		"region": true, "note": true, "auto_recharge": true,
		"auto_invoice": true,
	}

	updates := make(map[string]interface{})
	for key, val := range req.Updates {
		if allowedFields[key] {
			updates[key] = val
		}
	}

	if len(updates) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "No valid fields to update",
		})
	}

	// Build query based on user type
	query := database.DB.Model(&models.Subscriber{}).Where("id IN ?", req.IDs)
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		if !checkUserPermission(user, "subscribers.view_all") {
			query = query.Where("reseller_id IN (SELECT id FROM resellers WHERE id = ? OR parent_id = ?)", *user.ResellerID, *user.ResellerID)
		}
	}

	result := query.Updates(updates)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update subscribers",
		})
	}

	// If service changed, update RADIUS
	if serviceID, ok := updates["service_id"]; ok {
		var service models.Service
		if database.DB.First(&service, serviceID).Error == nil {
			var subscribers []models.Subscriber
			database.DB.Where("id IN ?", req.IDs).Find(&subscribers)
			for _, sub := range subscribers {
				database.DB.Where("username = ? AND attribute = ?", sub.Username, "Mikrotik-Rate-Limit").Delete(&models.RadReply{})
				database.DB.Create(&models.RadReply{
					Username:  sub.Username,
					Attribute: "Mikrotik-Rate-Limit",
					Op:        "=",
					Value:     fmt.Sprintf("%s/%s", service.UploadSpeedStr, service.DownloadSpeedStr),
				})
			}
		}
	}

	// Create audit log
	updatesJSON, _ := json.Marshal(updates)
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionUpdate,
		EntityType:  "subscriber",
		Description: fmt.Sprintf("Bulk updated %d subscribers", result.RowsAffected),
		NewValue:    string(updatesJSON),
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Updated %d subscribers", result.RowsAffected),
		"data": fiber.Map{
			"updated": result.RowsAffected,
		},
	})
}

// BulkActionRequest represents bulk action request
type BulkActionRequest struct {
	IDs    []uint `json:"ids"`
	Action string `json:"action"` // renew, disconnect, enable, disable, reset_fup
}

// BulkAction performs action on multiple subscribers
func (h *SubscriberHandler) BulkAction(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	var req BulkActionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Check permission based on action type (admins bypass this check)
	if user.UserType != models.UserTypeAdmin {
		requiredPermission := ""
		switch req.Action {
		case "renew":
			requiredPermission = "subscribers.renew"
		case "disconnect":
			requiredPermission = "subscribers.disconnect"
		case "enable", "disable":
			requiredPermission = "subscribers.inactivate"
		case "reset_fup":
			requiredPermission = "subscribers.reset_fup"
		case "delete":
			requiredPermission = "subscribers.delete"
		}

		if requiredPermission != "" && !checkUserPermission(user, requiredPermission) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": "You don't have permission to perform this action",
			})
		}
	}

	log.Printf("BulkAction: action=%s, ids=%v", req.Action, req.IDs)

	if len(req.IDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "No subscribers selected",
		})
	}

	// Get subscribers
	query := database.DB.Preload("Service").Where("id IN ?", req.IDs)
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		if !checkUserPermission(user, "subscribers.view_all") {
			query = query.Where("reseller_id IN (SELECT id FROM resellers WHERE id = ? OR parent_id = ?)", *user.ResellerID, *user.ResellerID)
		}
	}

	var subscribers []models.Subscriber
	if err := query.Find(&subscribers).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch subscribers",
		})
	}

	// Pre-load reseller once (if user is reseller) - avoids N+1 query
	var currentReseller *models.Reseller
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		currentReseller = &models.Reseller{}
		database.DB.First(currentReseller, *user.ResellerID)
	}

	// Pre-load all NAS devices used by subscribers - avoids N+1 query
	nasIDs := make([]uint, 0)
	for _, sub := range subscribers {
		if sub.NasID != nil && *sub.NasID > 0 {
			nasIDs = append(nasIDs, *sub.NasID)
		}
	}
	nasMap := make(map[uint]*models.Nas)
	if len(nasIDs) > 0 {
		var nasList []models.Nas
		database.DB.Where("id IN ?", nasIDs).Find(&nasList)
		for i := range nasList {
			nasMap[nasList[i].ID] = &nasList[i]
		}
	}

	success := 0
	failed := 0
	var actionName string

	for _, sub := range subscribers {
		switch req.Action {
		case "renew":
			actionName = "Bulk renewed"
			// Check balance for resellers (using pre-loaded reseller)
			if user.UserType == models.UserTypeReseller && currentReseller != nil {
				if currentReseller.Balance < sub.Price {
					failed++
					continue
				}
				// Deduct balance
				database.DB.Model(currentReseller).Update("balance", gorm.Expr("balance - ?", sub.Price))
				currentReseller.Balance -= sub.Price // Update local copy for next iteration
				// Create transaction
				transaction := models.Transaction{
					Type:         models.TransactionTypeRenewal,
					Amount:       -sub.Price,
					ResellerID:   *user.ResellerID,
					SubscriberID: &sub.ID,
					Description:  fmt.Sprintf("Bulk Renewal: %s", sub.Username),
					IPAddress:    c.IP(),
					CreatedBy:    user.ID,
				}
				database.DB.Create(&transaction)
			}
			// Calculate new expiry
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
			// Reset FUP counters on renewal
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
				"monthly_quota_used":        0,
				"last_monthly_reset":        now,
				"cdn_monthly_download_used": 0,
				"cdn_monthly_upload_used":   0,
				"cdn_monthly_fup_level":     0,
			})
			// Update RADIUS expiration
			database.DB.Where("username = ? AND attribute = ?", sub.Username, "Expiration").Delete(&models.RadCheck{})
			database.DB.Create(&models.RadCheck{
				Username: sub.Username, Attribute: "Expiration", Op: ":=",
				Value: newExpiry.Format("Jan 02 2006 15:04:05"),
			})
			// Remove Auth-Type := Reject if exists (in case user was disabled)
			database.DB.Where("username = ? AND attribute = ? AND value = ?", sub.Username, "Auth-Type", "Reject").Delete(&models.RadCheck{})
			// Update RADIUS reply to reset rate limit to full speed
			// Delete existing rate limit and set to full speed
			// Speeds are already in kb (e.g., 2000 = 2000k), no conversion needed
			database.DB.Where("username = ? AND attribute = ?", sub.Username, "Mikrotik-Rate-Limit").Delete(&models.RadReply{})
			fullSpeedLimit := fmt.Sprintf("%dk/%dk", sub.Service.UploadSpeed, sub.Service.DownloadSpeed)
			database.DB.Create(&models.RadReply{
				Username:  sub.Username,
				Attribute: "Mikrotik-Rate-Limit",
				Op:        "=",
				Value:     fullSpeedLimit,
			})
			// Reset speed via RADIUS CoA (since FUP was reset)
			if sub.NasID != nil && *sub.NasID > 0 && sub.ServiceID > 0 {
				if nas, ok := nasMap[*sub.NasID]; ok && nas.IPAddress != "" {
					// Build rate limit string: upload/download format for MikroTik
					// Speeds are already in kb, no conversion needed
					rateLimit := fmt.Sprintf("%dk/%dk", sub.Service.UploadSpeed, sub.Service.DownloadSpeed)
					fmt.Printf("Renew: Resetting speed for %s via CoA to %s\n", sub.Username, rateLimit)

					// Use CoA to update rate limit
					coaClient := radius.NewCOAClient(nas.IPAddress, nas.CoAPort, nas.Secret)
					if err := coaClient.UpdateRateLimitViaRadclient(sub.Username, sub.SessionID, rateLimit); err != nil {
						fmt.Printf("Renew: CoA failed for %s: %v, trying MikroTik API\n", sub.Username, err)
						// Fallback to MikroTik API (speeds already in kb)
						client := mikrotik.NewClient(
							fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
							nas.APIUsername,
							nas.APIPassword,
						)
						if err := client.UpdateUserRateLimit(sub.Username, int(sub.Service.DownloadSpeed), int(sub.Service.UploadSpeed)); err != nil {
							fmt.Printf("Renew: MikroTik API also failed for %s: %v\n", sub.Username, err)
						}
						client.Close()
					} else {
						fmt.Printf("Renew: Successfully reset speed for %s via CoA\n", sub.Username)
					}
				}
			}
			success++

		case "disconnect":
			actionName = "Bulk disconnected"
			// Actually disconnect from MikroTik if NAS is configured (using pre-loaded nasMap)
			if sub.NasID != nil && *sub.NasID > 0 {
				if nas, ok := nasMap[*sub.NasID]; ok && nas.IPAddress != "" {
					client := mikrotik.NewClient(
						fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
						nas.APIUsername,
						nas.APIPassword,
					)
					if err := client.DisconnectUser(sub.Username); err != nil {
						fmt.Printf("MikroTik disconnect error for %s: %v\n", sub.Username, err)
					}
					client.Close()
				}
			}
			database.DB.Model(&sub).Updates(map[string]interface{}{
				"is_online":  false,
				"session_id": "",
			})
			success++

		case "enable":
			actionName = "Bulk enabled"
			database.DB.Model(&sub).Update("status", models.SubscriberStatusActive)
			// Remove Auth-Type := Reject from RADIUS if exists
			database.DB.Where("username = ? AND attribute = ? AND value = ?", sub.Username, "Auth-Type", "Reject").Delete(&models.RadCheck{})
			success++

		case "disable":
			actionName = "Bulk disabled"
			database.DB.Model(&sub).Update("status", models.SubscriberStatusInactive)
			// Add Auth-Type := Reject to RADIUS to block login
			database.DB.Where("username = ? AND attribute = ?", sub.Username, "Auth-Type").Delete(&models.RadCheck{})
			database.DB.Create(&models.RadCheck{
				Username: sub.Username, Attribute: "Auth-Type", Op: ":=", Value: "Reject",
			})
			// Disconnect from MikroTik if online (using pre-loaded nasMap)
			if sub.NasID != nil && *sub.NasID > 0 {
				if nas, ok := nasMap[*sub.NasID]; ok && nas.IPAddress != "" {
					client := mikrotik.NewClient(
						fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
						nas.APIUsername,
						nas.APIPassword,
					)
					client.DisconnectUser(sub.Username)
					client.Close()
				}
			}
			database.DB.Model(&sub).Updates(map[string]interface{}{
				"is_online":  false,
				"session_id": "",
			})
			success++

		case "reset_fup":
			actionName = "Bulk reset FUP"
			// Only reset DAILY FUP - monthly resets on renew
			updateResult := database.DB.Model(&sub).Updates(map[string]interface{}{
				"fup_level":               0,
				"daily_quota_used":         0,
				"daily_download_used":      0,
				"daily_upload_used":        0,
				"cdn_daily_download_used":  0,
				"cdn_daily_upload_used":    0,
				"cdn_fup_level":            0,
			})
			if updateResult.Error != nil {
				log.Printf("Reset FUP: DB update error for %s: %v", sub.Username, updateResult.Error)
			} else {
				log.Printf("Reset FUP: DB updated %d rows for %s (ID=%d)", updateResult.RowsAffected, sub.Username, sub.ID)
			}
			// Update RADIUS reply to reset rate limit to full speed (only if service exists)
			if sub.Service != nil {
				// Speeds are already in kb (e.g., 2000 = 2000k), no conversion needed
				database.DB.Where("username = ? AND attribute = ?", sub.Username, "Mikrotik-Rate-Limit").Delete(&models.RadReply{})
				fullSpeedLimit := fmt.Sprintf("%dk/%dk", sub.Service.UploadSpeed, sub.Service.DownloadSpeed)
				database.DB.Create(&models.RadReply{
					Username:  sub.Username,
					Attribute: "Mikrotik-Rate-Limit",
					Op:        "=",
					Value:     fullSpeedLimit,
				})
				// Reset speed via RADIUS CoA (using pre-loaded nasMap)
				if sub.NasID != nil && *sub.NasID > 0 && sub.ServiceID > 0 {
					if nas, ok := nasMap[*sub.NasID]; ok && nas.IPAddress != "" {
						// Build rate limit string: upload/download format for MikroTik
						// Speeds are already in kb, no conversion needed
						rateLimit := fmt.Sprintf("%dk/%dk", sub.Service.UploadSpeed, sub.Service.DownloadSpeed)
						fmt.Printf("Reset FUP: Resetting speed for %s via CoA to %s\n", sub.Username, rateLimit)

						// Use CoA to update rate limit
						coaClient := radius.NewCOAClient(nas.IPAddress, nas.CoAPort, nas.Secret)
						if err := coaClient.UpdateRateLimitViaRadclient(sub.Username, sub.SessionID, rateLimit); err != nil {
							fmt.Printf("Reset FUP: CoA failed for %s: %v, trying MikroTik API\n", sub.Username, err)
							// Fallback to MikroTik API (speeds already in kb)
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
			} else {
				log.Printf("Reset FUP: Skipping RADIUS update for %s - no service assigned", sub.Username)
			}
			success++

		case "delete":
			actionName = "Bulk deleted"
			// Disconnect from MikroTik if online (using pre-loaded nasMap)
			if sub.NasID != nil && *sub.NasID > 0 {
				if nas, ok := nasMap[*sub.NasID]; ok && nas.IPAddress != "" {
					client := mikrotik.NewClient(
						fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
						nas.APIUsername,
						nas.APIPassword,
					)
					client.DisconnectUser(sub.Username)
					client.Close()
				}
			}
			// Delete RADIUS entries
			database.DB.Where("username = ?", sub.Username).Delete(&models.RadCheck{})
			database.DB.Where("username = ?", sub.Username).Delete(&models.RadReply{})
			database.DB.Where("username = ?", sub.Username).Delete(&models.RadAcct{})
			// Soft delete + set deleted_by in one operation
			now := time.Now()
			if err := database.DB.Model(&sub).Updates(map[string]interface{}{
				"deleted_by_id":   user.ID,
				"deleted_by_name": user.Username,
				"deleted_at":      now,
			}).Error; err != nil {
				log.Printf("BulkAction: Failed to delete subscriber %d: %v", sub.ID, err)
				failed++
				continue
			}
			success++

		default:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Invalid action",
			})
		}
	}

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionUpdate,
		EntityType:  "subscriber",
		Description: fmt.Sprintf("%s %d subscribers (%d failed)", actionName, success, failed),
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	responseMessage := fmt.Sprintf("%s %d subscribers (%d failed)", actionName, success, failed)
	log.Printf("BulkAction response: actionName=%s, message=%s", actionName, responseMessage)

	return c.JSON(fiber.Map{
		"success": true,
		"message": responseMessage,
		"data": fiber.Map{
			"success": success,
			"failed":  failed,
		},
	})
}

// ChangeBulkRequest represents change bulk request with filters
type ChangeBulkRequest struct {
	ResellerID          uint               `json:"reseller_id"`           // 0 = All
	ServiceID           uint               `json:"service_id"`            // 0 = All
	NasID               uint               `json:"nas_id"`                // 0 = All
	StatusFilter        string             `json:"status_filter"`         // all, active, inactive, expired, active_inactive
	OnlineFilter        string             `json:"online_filter"`         // all, online, offline
	FUPLevelFilter      string             `json:"fup_level_filter"`      // all, 0, 1, 2, 3, 4, 5, 6
	IncludeSubResellers bool               `json:"include_sub_resellers"`
	Action              string             `json:"action"`
	ActionValue         string             `json:"action_value"`
	Filters             []ChangeBulkFilter `json:"filters"`
	Preview             bool               `json:"preview"`
}

// ChangeBulkFilter represents a custom filter
type ChangeBulkFilter struct {
	Field string `json:"field"` // username, expiry, name, address, price, phone, created, daily_usage, monthly_usage
	Rule  string `json:"rule"`  // equal, notequal, greater, less, like
	Value string `json:"value"`
}

// ChangeBulk performs bulk changes on subscribers based on filters
func (h *SubscriberHandler) ChangeBulk(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	// Only admins can use ChangeBulk
	if user.UserType != models.UserTypeAdmin {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"message": "Only administrators can use ChangeBulk",
		})
	}

	var req ChangeBulkRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Build query based on filters
	query := database.DB.Model(&models.Subscriber{})

	log.Printf("ChangeBulk: Received request - ResellerID=%d, ServiceID=%d, Status=%s, Preview=%v, Action=%s",
		req.ResellerID, req.ServiceID, req.StatusFilter, req.Preview, req.Action)

	// Filter by reseller
	if req.ResellerID > 0 {
		if req.IncludeSubResellers {
			query = query.Where("reseller_id IN (SELECT id FROM resellers WHERE id = ? OR parent_id = ?)", req.ResellerID, req.ResellerID)
		} else {
			query = query.Where("reseller_id = ?", req.ResellerID)
		}
	}

	// Filter by service
	if req.ServiceID > 0 {
		query = query.Where("service_id = ?", req.ServiceID)
	}

	// Filter by NAS
	if req.NasID > 0 {
		query = query.Where("nas_id = ?", req.NasID)
	}

	// Filter by status
	switch req.StatusFilter {
	case "active":
		query = query.Where("status = ?", models.SubscriberStatusActive)
	case "inactive":
		query = query.Where("status = ?", models.SubscriberStatusInactive)
	case "expired":
		query = query.Where("expiry_date < ?", time.Now())
	case "active_inactive":
		query = query.Where("status IN ?", []models.SubscriberStatus{models.SubscriberStatusActive, models.SubscriberStatusInactive})
	}

	// Filter by online status
	switch req.OnlineFilter {
	case "online":
		query = query.Where("is_online = ?", true)
	case "offline":
		query = query.Where("is_online = ?", false)
	}

	// Filter by FUP level
	switch req.FUPLevelFilter {
	case "0":
		query = query.Where("fup_level = ?", 0)
	case "1":
		query = query.Where("fup_level = ?", 1)
	case "2":
		query = query.Where("fup_level = ?", 2)
	case "3":
		query = query.Where("fup_level = ?", 3)
	case "4":
		query = query.Where("fup_level = ?", 4)
	case "5":
		query = query.Where("fup_level = ?", 5)
	case "6":
		query = query.Where("fup_level = ?", 6)
	}

	// Apply custom filters
	for _, filter := range req.Filters {
		var fieldName string
		switch filter.Field {
		case "username":
			fieldName = "username"
		case "expiry":
			fieldName = "expiry_date"
		case "name":
			fieldName = "full_name"
		case "address":
			fieldName = "address"
		case "price":
			fieldName = "price"
		case "phone":
			fieldName = "phone"
		case "created":
			fieldName = "created_at"
		case "daily_usage":
			fieldName = "daily_quota_used"
		case "monthly_usage":
			fieldName = "monthly_quota_used"
		default:
			continue
		}

		switch filter.Rule {
		case "equal":
			query = query.Where(fmt.Sprintf("%s = ?", fieldName), filter.Value)
		case "notequal":
			query = query.Where(fmt.Sprintf("%s != ?", fieldName), filter.Value)
		case "greater":
			query = query.Where(fmt.Sprintf("%s > ?", fieldName), filter.Value)
		case "less":
			query = query.Where(fmt.Sprintf("%s < ?", fieldName), filter.Value)
		case "like":
			query = query.Where(fmt.Sprintf("%s ILIKE ?", fieldName), "%"+filter.Value+"%")
		}
	}

	// Count affected subscribers
	var total int64
	query.Count(&total)

	log.Printf("ChangeBulk: Query returned %d subscribers", total)

	// If preview mode, return the list of affected subscribers
	if req.Preview {
		page, _ := strconv.Atoi(c.Query("page", "1"))
		limit, _ := strconv.Atoi(c.Query("limit", "50"))
		offset := (page - 1) * limit

		var subscribers []models.Subscriber
		query.Preload("Service").Preload("Reseller").Preload("Reseller.User").Preload("Nas").Offset(offset).Limit(limit).Find(&subscribers)

		return c.JSON(fiber.Map{
			"success": true,
			"data":    subscribers,
			"meta": fiber.Map{
				"total":      total,
				"page":       page,
				"limit":      limit,
				"totalPages": (total + int64(limit) - 1) / int64(limit),
			},
		})
	}

	// Execute action
	var subscribers []models.Subscriber
	if err := query.Find(&subscribers).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch subscribers",
		})
	}

	success := 0
	failed := 0
	var actionName string

	for _, sub := range subscribers {
		switch req.Action {
		case "set_expiry":
			actionName = "Set expiry date"
			expiryDate, err := time.Parse("2006-01-02", req.ActionValue)
			if err != nil {
				failed++
				continue
			}
			database.DB.Model(&sub).Update("expiry_date", expiryDate)
			database.DB.Where("username = ? AND attribute = ?", sub.Username, "Expiration").Delete(&models.RadCheck{})
			database.DB.Create(&models.RadCheck{
				Username: sub.Username, Attribute: "Expiration", Op: ":=",
				Value: expiryDate.Format("Jan 02 2006 15:04:05"),
			})
			success++

		case "set_service":
			actionName = "Set service"
			serviceID, err := strconv.ParseUint(req.ActionValue, 10, 32)
			if err != nil {
				failed++
				continue
			}
			var service models.Service
			if err := database.DB.First(&service, serviceID).Error; err != nil {
				failed++
				continue
			}
			database.DB.Model(&sub).Updates(map[string]interface{}{
				"service_id": serviceID,
				"price":      service.Price,
			})
			success++

		case "set_reseller":
			actionName = "Set reseller"
			resellerID, err := strconv.ParseUint(req.ActionValue, 10, 32)
			if err != nil {
				failed++
				continue
			}
			database.DB.Model(&sub).Update("reseller_id", resellerID)
			success++

		case "set_active":
			actionName = "Set active"
			database.DB.Model(&sub).Update("status", models.SubscriberStatusActive)
			success++

		case "set_inactive":
			actionName = "Set inactive"
			database.DB.Model(&sub).Update("status", models.SubscriberStatusInactive)
			success++

		case "set_monthly_quota":
			actionName = "Set monthly quota"
			quotaGB, err := strconv.ParseFloat(req.ActionValue, 64)
			if err != nil {
				failed++
				continue
			}
			quotaBytes := uint64(quotaGB * 1024 * 1024 * 1024) // Convert GB to bytes
			database.DB.Model(&sub).Update("monthly_quota", quotaBytes)
			success++

		case "set_daily_quota":
			actionName = "Set daily quota"
			quotaMB, err := strconv.ParseFloat(req.ActionValue, 64)
			if err != nil {
				failed++
				continue
			}
			quotaBytes := uint64(quotaMB * 1024 * 1024) // Convert MB to bytes
			database.DB.Model(&sub).Update("daily_quota", quotaBytes)
			success++

		case "set_price":
			actionName = "Set price"
			price, err := strconv.ParseFloat(req.ActionValue, 64)
			if err != nil {
				failed++
				continue
			}
			database.DB.Model(&sub).Update("price", price)
			success++

		case "reset_mac":
			actionName = "Reset MAC"
			database.DB.Model(&sub).Update("mac_address", "")
			database.DB.Where("username = ? AND attribute = ?", sub.Username, "Calling-Station-Id").Delete(&models.RadCheck{})
			success++

		case "disconnect":
			actionName = "Disconnect"
			// Try to disconnect via CoA or MikroTik API
			if sub.NasID != nil && sub.IPAddress != "" {
				var nas models.Nas
				if err := database.DB.First(&nas, *sub.NasID).Error; err == nil {
					// Try MikroTik API disconnect
					client := mikrotik.NewClient(
						fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
						nas.APIUsername,
						nas.APIPassword,
					)
					if err := client.DisconnectUser(sub.Username); err != nil {
						log.Printf("ChangeBulk: Failed to disconnect %s via MikroTik: %v", sub.Username, err)
					}
					client.Close()
				}
			}
			database.DB.Model(&sub).Update("is_online", false)
			success++

		case "renew":
			actionName = "Renew"
			days, err := strconv.Atoi(req.ActionValue)
			if err != nil || days <= 0 {
				days = 30 // Default 30 days
			}
			newExpiry := time.Now().AddDate(0, 0, days)
			database.DB.Model(&sub).Updates(map[string]interface{}{
				"expiry_date":        newExpiry,
				"status":             models.SubscriberStatusActive,
				"fup_level":          0,
				"daily_quota_used":   0,
				"daily_download_used": 0,
				"daily_upload_used":  0,
			})
			database.DB.Where("username = ? AND attribute = ?", sub.Username, "Expiration").Delete(&models.RadCheck{})
			database.DB.Create(&models.RadCheck{
				Username: sub.Username, Attribute: "Expiration", Op: ":=",
				Value: newExpiry.Format("Jan 02 2006 15:04:05"),
			})
			success++

		case "add_days":
			actionName = "Add Days"
			days, err := strconv.Atoi(req.ActionValue)
			if err != nil || days <= 0 {
				failed++
				continue
			}
			currentExpiry := sub.ExpiryDate
			if currentExpiry.Before(time.Now()) {
				currentExpiry = time.Now()
			}
			newExpiry := currentExpiry.AddDate(0, 0, days)
			database.DB.Model(&sub).Update("expiry_date", newExpiry)
			database.DB.Where("username = ? AND attribute = ?", sub.Username, "Expiration").Delete(&models.RadCheck{})
			database.DB.Create(&models.RadCheck{
				Username: sub.Username, Attribute: "Expiration", Op: ":=",
				Value: newExpiry.Format("Jan 02 2006 15:04:05"),
			})
			success++

		case "reset_fup":
			actionName = "Reset FUP"
			database.DB.Model(&sub).Updates(map[string]interface{}{
				"fup_level":               0,
				"daily_quota_used":         0,
				"daily_download_used":      0,
				"daily_upload_used":        0,
				"cdn_daily_download_used":  0,
				"cdn_daily_upload_used":    0,
				"cdn_fup_level":            0,
			})
			// Restore original speed in radreply
			if sub.ServiceID > 0 {
				var service models.Service
				if err := database.DB.First(&service, sub.ServiceID).Error; err == nil {
					rateLimit := fmt.Sprintf("%dk/%dk", service.UploadSpeed, service.DownloadSpeed)
					database.DB.Where("username = ? AND attribute = ?", sub.Username, "Mikrotik-Rate-Limit").Delete(&models.RadReply{})
					database.DB.Create(&models.RadReply{
						Username: sub.Username, Attribute: "Mikrotik-Rate-Limit", Op: ":=", Value: rateLimit,
					})
				}
			}
			success++

		case "reset_monthly_fup":
			actionName = "Reset Monthly FUP"
			database.DB.Model(&sub).Updates(map[string]interface{}{
				"monthly_fup_level":         0,
				"monthly_quota_used":        0,
				"monthly_download_used":     0,
				"monthly_upload_used":       0,
				"cdn_monthly_download_used": 0,
				"cdn_monthly_upload_used":   0,
				"cdn_monthly_fup_level":     0,
			})
			success++

		case "reset_all_counters":
			actionName = "Reset All Counters"
			database.DB.Model(&sub).Updates(map[string]interface{}{
				"fup_level":            0,
				"daily_quota_used":     0,
				"daily_download_used":  0,
				"daily_upload_used":    0,
				"monthly_fup_level":    0,
				"monthly_quota_used":   0,
				"monthly_download_used": 0,
				"monthly_upload_used":  0,
			})
			success++

		case "delete":
			actionName = "Delete"
			// Soft delete + set deleted_by in one operation
			now := time.Now()
			database.DB.Model(&sub).Updates(map[string]interface{}{
				"deleted_by_id":   user.ID,
				"deleted_by_name": user.Username,
				"deleted_at":      now,
			})
			// Remove from radcheck/radreply
			database.DB.Where("username = ?", sub.Username).Delete(&models.RadCheck{})
			database.DB.Where("username = ?", sub.Username).Delete(&models.RadReply{})
			success++

		case "set_nas":
			actionName = "Set NAS"
			nasID, err := strconv.ParseUint(req.ActionValue, 10, 32)
			if err != nil {
				failed++
				continue
			}
			nasIDUint := uint(nasID)
			database.DB.Model(&sub).Update("nas_id", nasIDUint)
			success++

		case "set_password":
			actionName = "Set Password"
			if req.ActionValue == "" {
				failed++
				continue
			}
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.ActionValue), bcrypt.DefaultCost)
			if err != nil {
				failed++
				continue
			}
			database.DB.Model(&sub).Update("password", string(hashedPassword))
			database.DB.Where("username = ? AND attribute = ?", sub.Username, "Cleartext-Password").Delete(&models.RadCheck{})
			database.DB.Create(&models.RadCheck{
				Username: sub.Username, Attribute: "Cleartext-Password", Op: ":=", Value: req.ActionValue,
			})
			success++

		case "set_static_ip":
			actionName = "Set Static IP"
			database.DB.Model(&sub).Update("static_ip", req.ActionValue)
			if req.ActionValue != "" {
				// Check if another user already has this IP in radreply
				var existingCount int64
				database.DB.Model(&models.RadReply{}).Where(
					"attribute = ? AND value = ? AND username != ?",
					"Framed-IP-Address", req.ActionValue, sub.Username,
				).Count(&existingCount)
				if existingCount > 0 {
					log.Printf("SetStaticIP: Skipped %s - IP %s already assigned to another user", sub.Username, req.ActionValue)
					failed++
					continue
				}
				database.DB.Where("username = ? AND attribute = ?", sub.Username, "Framed-IP-Address").Delete(&models.RadReply{})
				database.DB.Create(&models.RadReply{
					Username: sub.Username, Attribute: "Framed-IP-Address", Op: ":=", Value: req.ActionValue,
				})
			} else {
				database.DB.Where("username = ? AND attribute = ?", sub.Username, "Framed-IP-Address").Delete(&models.RadReply{})
			}
			success++

		default:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Invalid action: " + req.Action,
			})
		}
	}

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionUpdate,
		EntityType:  "subscriber",
		Description: fmt.Sprintf("ChangeBulk: %s for %d subscribers (%d failed)", actionName, success, failed),
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("%s applied to %d subscribers (%d failed)", actionName, success, failed),
		"data": fiber.Map{
			"success": success,
			"failed":  failed,
			"total":   total,
		},
	})
}

// ListArchived returns archived (soft-deleted) subscribers
func (h *SubscriberHandler) ListArchived(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "25"))
	search := c.Query("search", "")
	resellerIDFilter := c.Query("reseller_id", "")
	deletedByFilter := c.Query("deleted_by", "")
	fromDate := c.Query("from", "")
	toDate := c.Query("to", "")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 25
	}
	offset := (page - 1) * limit

	query := database.DB.Unscoped().Model(&models.Subscriber{}).
		Where("deleted_at IS NOT NULL")

	// Resellers always see only their own archived subscribers (no view_all bypass)
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		query = query.Where("reseller_id IN (SELECT id FROM resellers WHERE id = ? OR parent_id = ?)", *user.ResellerID, *user.ResellerID)
	}

	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("username ILIKE ? OR full_name ILIKE ?", searchPattern, searchPattern)
	}

	// Filter by reseller who owns the subscriber
	if resellerIDFilter != "" {
		if rid, err := strconv.Atoi(resellerIDFilter); err == nil {
			query = query.Where("reseller_id = ?", rid)
		}
	}

	// Filter by who deleted
	if deletedByFilter != "" {
		if dby, err := strconv.Atoi(deletedByFilter); err == nil {
			query = query.Where("deleted_by_id = ?", dby)
		}
	}

	// Date range filter on deleted_at
	if fromDate != "" {
		query = query.Where("deleted_at >= ?", fromDate)
	}
	if toDate != "" {
		query = query.Where("deleted_at < ?::date + interval '1 day'", toDate)
	}

	var total int64
	query.Count(&total)

	var subscribers []models.Subscriber
	if err := query.Preload("Service").Preload("Reseller").Preload("Reseller.User").Offset(offset).Limit(limit).Order("deleted_at DESC").Find(&subscribers).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch archived subscribers",
		})
	}

	// Stats queries (independent of search/filter params)
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startOfWeek := startOfDay.AddDate(0, 0, -int(now.Weekday()))
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var deletedToday, deletedThisWeek, deletedThisMonth int64

	statsQuery := func(since time.Time) int64 {
		var count int64
		q := database.DB.Unscoped().Model(&models.Subscriber{}).Where("deleted_at IS NOT NULL AND deleted_at >= ?", since)
		if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
			q = q.Where("reseller_id IN (SELECT id FROM resellers WHERE id = ? OR parent_id = ?)", *user.ResellerID, *user.ResellerID)
		}
		q.Count(&count)
		return count
	}
	deletedToday = statsQuery(startOfDay)
	deletedThisWeek = statsQuery(startOfWeek)
	deletedThisMonth = statsQuery(startOfMonth)

	// Top deleters this month
	type deleterStat struct {
		DeletedByID uint   `json:"deleted_by_id"`
		Name        string `json:"name"`
		Count       int64  `json:"count"`
	}
	var topDeleters []deleterStat

	topDeletersQuery := database.DB.Unscoped().Model(&models.Subscriber{}).
		Select("deleted_by_id, deleted_by_name as name, COUNT(*) as count").
		Where("deleted_at IS NOT NULL AND deleted_by_name IS NOT NULL AND deleted_by_name != '' AND deleted_at >= ?", startOfMonth).
		Group("deleted_by_id, deleted_by_name").
		Order("count DESC").
		Limit(10)

	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		topDeletersQuery = topDeletersQuery.Where("reseller_id IN (SELECT id FROM resellers WHERE id = ? OR parent_id = ?)", *user.ResellerID, *user.ResellerID)
	}

	topDeletersQuery.Find(&topDeleters)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    subscribers,
		"meta": fiber.Map{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
		"stats": fiber.Map{
			"deleted_today":      deletedToday,
			"deleted_this_week":  deletedThisWeek,
			"deleted_this_month": deletedThisMonth,
			"top_deleters":       topDeleters,
		},
	})
}

// Restore restores a soft-deleted subscriber
func (h *SubscriberHandler) Restore(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid subscriber ID",
		})
	}

	var subscriber models.Subscriber
	if err := database.DB.Unscoped().First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber not found",
		})
	}

	if subscriber.DeletedAt.Time.IsZero() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber is not archived",
		})
	}

	// Restore subscriber
	if err := database.DB.Unscoped().Model(&subscriber).Update("deleted_at", nil).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to restore subscriber",
		})
	}

	// Restore RADIUS attributes
	database.DB.Preload("Service").First(&subscriber, id)

	simultaneousSessions := subscriber.SimultaneousSessions
	if simultaneousSessions <= 0 {
		simultaneousSessions = 1
	}

	radCheck := []models.RadCheck{
		{Username: subscriber.Username, Attribute: "Cleartext-Password", Op: ":=", Value: subscriber.PasswordPlain},
		{Username: subscriber.Username, Attribute: "Expiration", Op: ":=", Value: subscriber.ExpiryDate.Format("Jan 02 2006 15:04:05")},
		{Username: subscriber.Username, Attribute: "Simultaneous-Use", Op: ":=", Value: fmt.Sprintf("%d", simultaneousSessions)},
	}
	database.DB.Create(&radCheck)

	if subscriber.Service.ID != 0 {
		radReply := []models.RadReply{
			{Username: subscriber.Username, Attribute: "Mikrotik-Rate-Limit", Op: "=", Value: fmt.Sprintf("%s/%s", subscriber.Service.UploadSpeedStr, subscriber.Service.DownloadSpeedStr)},
		}
		database.DB.Create(&radReply)
	}

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionUpdate,
		EntityType:  "subscriber",
		EntityID:    subscriber.ID,
		EntityName:  subscriber.Username,
		Description: "Restored archived subscriber",
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Subscriber restored successfully",
		"data":    subscriber,
	})
}

// PermanentDelete permanently deletes an archived subscriber
func (h *SubscriberHandler) PermanentDelete(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid subscriber ID",
		})
	}

	var subscriber models.Subscriber
	if err := database.DB.Unscoped().First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber not found",
		})
	}

	if subscriber.DeletedAt.Time.IsZero() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber must be archived before permanent deletion",
		})
	}

	// Permanently delete
	if err := database.DB.Unscoped().Delete(&subscriber).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to delete subscriber",
		})
	}

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionDelete,
		EntityType:  "subscriber",
		EntityID:    uint(id),
		EntityName:  subscriber.Username,
		Description: "Permanently deleted subscriber",
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Subscriber permanently deleted",
	})
}

// RenameRequest represents rename request
type RenameRequest struct {
	NewUsername string `json:"new_username"`
	Reason      string `json:"reason"`
}

// Rename changes subscriber's username
func (h *SubscriberHandler) Rename(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	var req RenameRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	if req.NewUsername == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "New username is required"})
	}

	var subscriber models.Subscriber
	if err := database.DB.First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	oldUsername := subscriber.Username

	// Check if new username already exists
	var count int64
	database.DB.Model(&models.Subscriber{}).Where("username = ? AND id != ?", req.NewUsername, id).Count(&count)
	if count > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": "Username already exists"})
	}

	// Update subscriber
	subscriber.Username = req.NewUsername
	database.DB.Save(&subscriber)

	// Update RADIUS entries
	database.DB.Model(&models.RadCheck{}).Where("username = ?", oldUsername).Update("username", req.NewUsername)
	database.DB.Model(&models.RadReply{}).Where("username = ?", oldUsername).Update("username", req.NewUsername)
	database.DB.Model(&models.RadUserGroup{}).Where("username = ?", oldUsername).Update("username", req.NewUsername)
	database.DB.Model(&models.RadAcct{}).Where("username = ?", oldUsername).Update("username", req.NewUsername)

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionUpdate,
		EntityType:  "subscriber",
		EntityID:    subscriber.ID,
		EntityName:  subscriber.Username,
		OldValue:    oldUsername,
		NewValue:    req.NewUsername,
		Description: fmt.Sprintf("Renamed from %s to %s. Reason: %s", oldUsername, req.NewUsername, req.Reason),
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Username changed successfully",
		"data": fiber.Map{
			"old_username": oldUsername,
			"new_username": req.NewUsername,
		},
	})
}

// AddDaysRequest represents add days request
type AddDaysRequest struct {
	Days   int    `json:"days"`
	Reason string `json:"reason"`
}

// AddDays adds or subtracts days from subscriber's expiry
func (h *SubscriberHandler) AddDays(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	var req AddDaysRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	if req.Days == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Days cannot be zero"})
	}

	var subscriber models.Subscriber
	if err := database.DB.First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	oldExpiry := subscriber.ExpiryDate
	newExpiry := subscriber.ExpiryDate.AddDate(0, 0, req.Days)

	// Update subscriber
	subscriber.ExpiryDate = newExpiry
	database.DB.Save(&subscriber)

	// Update RADIUS expiration
	database.DB.Where("username = ? AND attribute = ?", subscriber.Username, "Expiration").Delete(&models.RadCheck{})
	database.DB.Create(&models.RadCheck{
		Username:  subscriber.Username,
		Attribute: "Expiration",
		Op:        ":=",
		Value:     newExpiry.Format("Jan 02 2006 15:04:05"),
	})

	// Create audit log
	action := "Added"
	if req.Days < 0 {
		action = "Subtracted"
	}
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionUpdate,
		EntityType:  "subscriber",
		EntityID:    subscriber.ID,
		EntityName:  subscriber.Username,
		OldValue:    oldExpiry.Format("2006-01-02"),
		NewValue:    newExpiry.Format("2006-01-02"),
		Description: fmt.Sprintf("%s %d days. Reason: %s", action, abs(req.Days), req.Reason),
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("%s %d days successfully", action, abs(req.Days)),
		"data": fiber.Map{
			"old_expiry": oldExpiry,
			"new_expiry": newExpiry,
		},
	})
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// ChangeServiceRequest represents change service request
type ChangeServiceRequest struct {
	ServiceID        uint    `json:"service_id"`
	ExtendExpiry     bool    `json:"extend_expiry"`
	ResetFUP         bool    `json:"reset_fup"`
	ChargePrice      bool    `json:"charge_price"`
	ProratePrice     bool    `json:"prorate_price"`
	Reason           string  `json:"reason"`
}

// getSystemPreference retrieves a system preference value
func getSystemPreference(key string, defaultValue string) string {
	var pref models.SystemPreference
	if err := database.DB.Where("key = ?", key).First(&pref).Error; err != nil {
		return defaultValue
	}
	return pref.Value
}

// getSystemPreferenceFloat retrieves a system preference as float64
func getSystemPreferenceFloat(key string, defaultValue float64) float64 {
	val := getSystemPreference(key, "")
	if val == "" {
		return defaultValue
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return defaultValue
	}
	return f
}

// getSystemPreferenceBool retrieves a system preference as bool
func getSystemPreferenceBool(key string, defaultValue bool) bool {
	val := getSystemPreference(key, "")
	if val == "" {
		return defaultValue
	}
	return val == "true" || val == "1"
}

// ChangeServicePriceResponse represents the price calculation response
type ChangeServicePriceResponse struct {
	RemainingDays      int     `json:"remaining_days"`
	OldDayPrice        float64 `json:"old_day_price"`
	NewDayPrice        float64 `json:"new_day_price"`
	OldCredit          float64 `json:"old_credit"`           // Credit from remaining days on old service
	NewCost            float64 `json:"new_cost"`             // Cost for remaining days on new service
	PriceDifference    float64 `json:"price_difference"`     // NewCost - OldCredit
	ChangeFee          float64 `json:"change_fee"`           // Upgrade or downgrade fee
	TotalCharge        float64 `json:"total_charge"`         // Final amount to charge (can be negative for refund)
	IsUpgrade          bool    `json:"is_upgrade"`
	IsDowngrade        bool    `json:"is_downgrade"`
	DowngradeAllowed   bool    `json:"downgrade_allowed"`
	RefundEnabled      bool    `json:"refund_enabled"`
}

// CalculateChangeServicePrice calculates the price for changing service
func (h *SubscriberHandler) CalculateChangeServicePrice(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	subscriberID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	newServiceID, err := strconv.Atoi(c.Query("service_id"))
	if err != nil || newServiceID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Service ID is required"})
	}

	var subscriber models.Subscriber
	if err := database.DB.Preload("Service").First(&subscriber, subscriberID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	var newService models.Service
	if err := database.DB.First(&newService, newServiceID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Service not found"})
	}

	oldService := subscriber.Service

	// Get system preferences
	upgradeFee := getSystemPreferenceFloat("upgrade_change_service_fee", 0)
	downgradeFee := getSystemPreferenceFloat("downgrade_change_service_fee", 0)
	allowDowngrade := getSystemPreferenceBool("allow_downgrade", true)
	refundEnabled := getSystemPreferenceBool("downgrade_refund", false)

	// Calculate remaining days until expiry
	now := time.Now()
	remainingDays := 0
	if subscriber.ExpiryDate.After(now) {
		remainingDays = int(subscriber.ExpiryDate.Sub(now).Hours() / 24)
	}

	// Get day prices (use service's day_price if set, otherwise calculate from price)
	oldDayPrice := oldService.DayPrice
	if oldDayPrice == 0 && oldService.Price > 0 {
		// Calculate day price from monthly price (assuming 30 days)
		oldDayPrice = oldService.Price / 30.0
	}

	newDayPrice := newService.DayPrice
	if newDayPrice == 0 && newService.Price > 0 {
		newDayPrice = newService.Price / 30.0
	}

	// Calculate credits and costs
	oldCredit := oldDayPrice * float64(remainingDays)
	newCost := newDayPrice * float64(remainingDays)
	priceDifference := newCost - oldCredit

	// Determine if upgrade or downgrade
	isUpgrade := newService.Price > oldService.Price
	isDowngrade := newService.Price < oldService.Price

	// Calculate total charge
	var totalCharge float64
	var changeFee float64

	if isUpgrade {
		changeFee = upgradeFee
		totalCharge = priceDifference + changeFee
		if totalCharge < 0 {
			totalCharge = changeFee // At minimum charge the upgrade fee
		}
	} else if isDowngrade {
		changeFee = downgradeFee
		if refundEnabled {
			// Refund the difference minus downgrade fee
			totalCharge = priceDifference + changeFee // priceDifference is negative here
		} else {
			// No refund, just charge the downgrade fee
			totalCharge = changeFee
		}
	} else {
		// Same price, just charge upgrade fee if any
		changeFee = upgradeFee
		totalCharge = changeFee
	}

	// Round to 2 decimal places
	totalCharge = math.Round(totalCharge*100) / 100
	oldCredit = math.Round(oldCredit*100) / 100
	newCost = math.Round(newCost*100) / 100
	priceDifference = math.Round(priceDifference*100) / 100

	return c.JSON(fiber.Map{
		"success": true,
		"data": ChangeServicePriceResponse{
			RemainingDays:    remainingDays,
			OldDayPrice:      oldDayPrice,
			NewDayPrice:      newDayPrice,
			OldCredit:        oldCredit,
			NewCost:          newCost,
			PriceDifference:  priceDifference,
			ChangeFee:        changeFee,
			TotalCharge:      totalCharge,
			IsUpgrade:        isUpgrade,
			IsDowngrade:      isDowngrade,
			DowngradeAllowed: allowDowngrade,
			RefundEnabled:    refundEnabled,
		},
	})
}

// ChangeService changes subscriber's service plan
func (h *SubscriberHandler) ChangeService(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	var req ChangeServiceRequest
	if err := c.BodyParser(&req); err != nil {
		log.Printf("ChangeService: Failed to parse body: %v, body: %s", err, string(c.Body()))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	log.Printf("ChangeService: Subscriber %d, ServiceID: %d, Request: %+v", id, req.ServiceID, req)

	if req.ServiceID == 0 {
		log.Printf("ChangeService: ServiceID is 0, rejecting")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Service ID is required"})
	}

	var subscriber models.Subscriber
	if err := database.DB.Preload("Service").First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	var newService models.Service
	if err := database.DB.First(&newService, req.ServiceID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Service not found"})
	}

	oldService := subscriber.Service
	oldServiceID := subscriber.ServiceID

	// Check reseller per-service subscriber limit (only if changing to a different service)
	if subscriber.ResellerID > 0 && req.ServiceID != subscriber.ServiceID {
		if err := checkResellerServiceLimit(database.DB, subscriber.ResellerID, req.ServiceID); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": err.Error()})
		}
	}

	// Get system preferences
	upgradeFee := getSystemPreferenceFloat("upgrade_change_service_fee", 0)
	downgradeFee := getSystemPreferenceFloat("downgrade_change_service_fee", 0)
	allowDowngrade := getSystemPreferenceBool("allow_downgrade", true)
	refundEnabled := getSystemPreferenceBool("downgrade_refund", false)

	// Determine if upgrade or downgrade
	isUpgrade := newService.Price > oldService.Price
	isDowngrade := newService.Price < oldService.Price

	// Check if downgrade is allowed
	if isDowngrade && !allowDowngrade {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Service downgrade is not allowed"})
	}

	// Calculate prorate price
	var chargeAmount float64
	var priceDescription string

	if req.ProratePrice {
		// Calculate remaining days until expiry
		now := time.Now()
		remainingDays := 0
		if subscriber.ExpiryDate.After(now) {
			remainingDays = int(subscriber.ExpiryDate.Sub(now).Hours() / 24)
		}

		// Get day prices (use service's day_price if set, otherwise calculate from price)
		oldDayPrice := oldService.DayPrice
		if oldDayPrice == 0 && oldService.Price > 0 {
			oldDayPrice = oldService.Price / 30.0
		}

		newDayPrice := newService.DayPrice
		if newDayPrice == 0 && newService.Price > 0 {
			newDayPrice = newService.Price / 30.0
		}

		// Calculate credits and costs
		oldCredit := oldDayPrice * float64(remainingDays)
		newCost := newDayPrice * float64(remainingDays)
		priceDifference := newCost - oldCredit

		if isUpgrade {
			chargeAmount = priceDifference + upgradeFee
			if chargeAmount < upgradeFee {
				chargeAmount = upgradeFee
			}
			priceDescription = fmt.Sprintf("Prorate upgrade: %d days remaining, diff: %.2f + fee: %.2f", remainingDays, priceDifference, upgradeFee)
		} else if isDowngrade {
			if refundEnabled {
				chargeAmount = priceDifference + downgradeFee // priceDifference is negative
				priceDescription = fmt.Sprintf("Prorate downgrade with refund: %d days, diff: %.2f + fee: %.2f", remainingDays, priceDifference, downgradeFee)
			} else {
				chargeAmount = downgradeFee
				priceDescription = fmt.Sprintf("Downgrade fee (no refund): %.2f", downgradeFee)
			}
		} else {
			chargeAmount = upgradeFee
			priceDescription = fmt.Sprintf("Same price change, fee: %.2f", upgradeFee)
		}

		// Round to 2 decimal places
		chargeAmount = math.Round(chargeAmount*100) / 100
		log.Printf("ChangeService: Prorate calculation - %s, Total: %.2f", priceDescription, chargeAmount)
	} else if req.ChargePrice {
		// Full price charge (legacy behavior)
		chargeAmount = newService.Price
		priceDescription = fmt.Sprintf("Full service price: %.2f", newService.Price)
	}

	// Check reseller balance if charging
	if chargeAmount > 0 && user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		var reseller models.Reseller
		database.DB.First(&reseller, *user.ResellerID)
		if reseller.Balance < chargeAmount {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": fmt.Sprintf("Insufficient balance. Required: %.2f, Available: %.2f", chargeAmount, reseller.Balance),
			})
		}
	}

	// Update subscriber
	subscriber.ServiceID = req.ServiceID
	subscriber.Price = newService.Price

	if req.ExtendExpiry {
		if newService.ExpiryUnit == models.ExpiryUnitMonths {
			subscriber.ExpiryDate = subscriber.ExpiryDate.AddDate(0, newService.ExpiryValue, 0)
		} else {
			subscriber.ExpiryDate = subscriber.ExpiryDate.AddDate(0, 0, newService.ExpiryValue)
		}
	}

	if req.ResetFUP {
		subscriber.FUPLevel = 0
		subscriber.DailyQuotaUsed = 0
		subscriber.MonthlyQuotaUsed = 0
	}

	log.Printf("ChangeService: Updating subscriber %d from ServiceID %d to %d", subscriber.ID, oldServiceID, req.ServiceID)

	// Use direct update query to ensure the change is applied
	updateFields := map[string]interface{}{
		"service_id": req.ServiceID,
		"price":      newService.Price,
	}
	if req.ResetFUP {
		updateFields["fup_level"] = 0
		updateFields["daily_quota_used"] = 0
		updateFields["monthly_quota_used"] = 0
	}
	if req.ExtendExpiry {
		updateFields["expiry_date"] = subscriber.ExpiryDate
	}

	result := database.DB.Model(&models.Subscriber{}).Where("id = ?", subscriber.ID).Updates(updateFields)
	if result.Error != nil {
		log.Printf("ChangeService: Failed to update subscriber: %v", result.Error)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to update subscriber"})
	}
	log.Printf("ChangeService: Subscriber updated successfully, rows affected: %d", result.RowsAffected)

	// Update RADIUS rate limit
	database.DB.Where("username = ? AND attribute = ?", subscriber.Username, "Mikrotik-Rate-Limit").Delete(&models.RadReply{})
	database.DB.Create(&models.RadReply{
		Username:  subscriber.Username,
		Attribute: "Mikrotik-Rate-Limit",
		Op:        "=",
		Value:     fmt.Sprintf("%s/%s", newService.UploadSpeedStr, newService.DownloadSpeedStr),
	})

	// Remove old Framed-IP-Address so user gets new IP from correct pool on reconnect
	database.DB.Where("username = ? AND attribute = ?", subscriber.Username, "Framed-IP-Address").Delete(&models.RadReply{})
	ippool.ReleaseIPForUser(subscriber.Username)
	log.Printf("ChangeService: Cleared Framed-IP-Address for %s (will get new IP from pool %s)", subscriber.Username, newService.PoolName)

	if req.ExtendExpiry {
		database.DB.Where("username = ? AND attribute = ?", subscriber.Username, "Expiration").Delete(&models.RadCheck{})
		database.DB.Create(&models.RadCheck{
			Username:  subscriber.Username,
			Attribute: "Expiration",
			Op:        ":=",
			Value:     subscriber.ExpiryDate.Format("Jan 02 2006 15:04:05"),
		})
	}

	// Deduct balance if charging (prorate or full price)
	if (req.ChargePrice || req.ProratePrice) && chargeAmount != 0 && user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		if chargeAmount > 0 {
			// Charge reseller
			database.DB.Model(&models.Reseller{}).Where("id = ?", *user.ResellerID).Update("balance", gorm.Expr("balance - ?", chargeAmount))
		} else {
			// Refund reseller (chargeAmount is negative)
			database.DB.Model(&models.Reseller{}).Where("id = ?", *user.ResellerID).Update("balance", gorm.Expr("balance + ?", -chargeAmount))
		}

		// Create transaction
		transaction := models.Transaction{
			Type:           models.TransactionTypeChangeService,
			Amount:         -chargeAmount, // Negative for charge, positive for refund
			ResellerID:     *user.ResellerID,
			SubscriberID:   &subscriber.ID,
			OldServiceName: oldService.Name,
			NewServiceName: newService.Name,
			Description:    fmt.Sprintf("Service change: %s -> %s. %s", oldService.Name, newService.Name, priceDescription),
			IPAddress:      c.IP(),
			CreatedBy:      user.ID,
		}
		database.DB.Create(&transaction)
	}

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionUpdate,
		EntityType:  "subscriber",
		EntityID:    subscriber.ID,
		EntityName:  subscriber.Username,
		OldValue:    fmt.Sprintf("ServiceID: %d", oldServiceID),
		NewValue:    fmt.Sprintf("ServiceID: %d, Charge: %.2f", req.ServiceID, chargeAmount),
		Description: fmt.Sprintf("Changed service from %s to %s. %s. Reason: %s", oldService.Name, newService.Name, priceDescription, req.Reason),
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	// Auto-disconnect user so they reconnect with new service pool IP
	// Reload subscriber to get NasID
	database.DB.First(&subscriber, id)
	if subscriber.NasID != nil {
		go func(username string, nasID uint) {
			var nas models.Nas
			if err := database.DB.First(&nas, nasID).Error; err != nil {
				log.Printf("ChangeService: Failed to find NAS %d for disconnect: %v", nasID, err)
				return
			}

			log.Printf("ChangeService: Service changed for %s, disconnecting from NAS %s", username, nas.IPAddress)

			// Try MikroTik API first (most reliable for PPPoE)
			client := mikrotik.NewClient(
				fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
				nas.APIUsername,
				nas.APIPassword,
			)
			if err := client.DisconnectUser(username); err != nil {
				log.Printf("ChangeService: MikroTik API disconnect failed for %s: %v, trying CoA", username, err)

				// Fallback: try CoA Disconnect-Request
				coaClient := radius.NewCOAClient(nas.IPAddress, nas.CoAPort, nas.Secret)
				if err := coaClient.DisconnectUser(username, ""); err != nil {
					log.Printf("ChangeService: CoA disconnect also failed for %s: %v", username, err)
				} else {
					log.Printf("ChangeService: Disconnected %s via CoA", username)
				}
			} else {
				log.Printf("ChangeService: Disconnected %s via MikroTik API (will reconnect with new pool)", username)
			}
			client.Close()
		}(subscriber.Username, *subscriber.NasID)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Service changed successfully",
		"data": fiber.Map{
			"old_service":   oldService.Name,
			"new_service":   newService.Name,
			"charge_amount": chargeAmount,
			"is_upgrade":    isUpgrade,
			"is_downgrade":  isDowngrade,
		},
	})
}

// Activate activates a subscriber
func (h *SubscriberHandler) Activate(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	var subscriber models.Subscriber
	if err := database.DB.First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	oldStatus := subscriber.Status
	subscriber.Status = models.SubscriberStatusActive
	database.DB.Save(&subscriber)

	// Remove Auth-Type := Reject to allow login
	database.DB.Where("username = ? AND attribute = ? AND value = ?", subscriber.Username, "Auth-Type", "Reject").Delete(&models.RadCheck{})

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionUpdate,
		EntityType:  "subscriber",
		EntityID:    subscriber.ID,
		EntityName:  subscriber.Username,
		OldValue:    fmt.Sprintf("Status: %d", oldStatus),
		NewValue:    fmt.Sprintf("Status: %d", models.SubscriberStatusActive),
		Description: "Activated subscriber",
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Subscriber activated successfully",
	})
}

// Deactivate deactivates a subscriber
func (h *SubscriberHandler) Deactivate(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	var subscriber models.Subscriber
	if err := database.DB.Preload("Nas").First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	oldStatus := subscriber.Status
	subscriber.Status = models.SubscriberStatusInactive
	database.DB.Save(&subscriber)

	// Add Auth-Type := Reject to block re-login
	database.DB.Where("username = ? AND attribute = ?", subscriber.Username, "Auth-Type").Delete(&models.RadCheck{})
	database.DB.Create(&models.RadCheck{
		Username: subscriber.Username, Attribute: "Auth-Type", Op: ":=", Value: "Reject",
	})

	// Disconnect from MikroTik if online
	if subscriber.IsOnline && subscriber.Nas != nil && subscriber.Nas.IPAddress != "" {
		client := mikrotik.NewClient(
			fmt.Sprintf("%s:%d", subscriber.Nas.IPAddress, subscriber.Nas.APIPort),
			subscriber.Nas.APIUsername,
			subscriber.Nas.APIPassword,
		)
		if err := client.DisconnectUser(subscriber.Username); err != nil {
			log.Printf("Deactivate: MikroTik disconnect failed for %s: %v", subscriber.Username, err)
		}
		client.Close()
		subscriber.IsOnline = false
		subscriber.SessionID = ""
		database.DB.Model(&subscriber).Updates(map[string]interface{}{
			"is_online":  false,
			"session_id": "",
		})
	}

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionUpdate,
		EntityType:  "subscriber",
		EntityID:    subscriber.ID,
		EntityName:  subscriber.Username,
		OldValue:    fmt.Sprintf("Status: %d", oldStatus),
		NewValue:    fmt.Sprintf("Status: %d", models.SubscriberStatusInactive),
		Description: "Deactivated subscriber",
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Subscriber deactivated successfully",
	})
}

// AddBalanceRequest represents add balance request
type AddBalanceRequest struct {
	Amount float64 `json:"amount"`
	Reason string  `json:"reason"`
}

// AddBalance adds credit to subscriber's wallet balance
func (h *SubscriberHandler) AddBalance(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	var req AddBalanceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	if req.Amount <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Amount must be positive"})
	}

	var subscriber models.Subscriber
	if err := database.DB.First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	balanceBefore := subscriber.Balance

	// Reseller: deduct from reseller balance, add to subscriber
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		var reseller models.Reseller
		database.DB.First(&reseller, *user.ResellerID)
		if reseller.Balance < req.Amount {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Insufficient reseller balance"})
		}
		// Deduct from reseller atomically
		database.DB.Model(&reseller).Update("balance", gorm.Expr("balance - ?", req.Amount))

		// Create reseller deduction transaction
		database.DB.Create(&models.Transaction{
			Type:         models.TransactionTypeTransfer,
			Amount:       -req.Amount,
			BalanceBefore: reseller.Balance,
			BalanceAfter:  reseller.Balance - req.Amount,
			Description:  fmt.Sprintf("Add balance to subscriber %s", subscriber.Username),
			ResellerID:   *user.ResellerID,
			SubscriberID: &subscriber.ID,
			IPAddress:    c.IP(),
			CreatedBy:    user.ID,
		})
	}

	// Add to subscriber balance atomically
	database.DB.Model(&subscriber).Update("balance", gorm.Expr("balance + ?", req.Amount))

	// Create subscriber topup transaction
	resellerID := uint(0)
	if user.ResellerID != nil {
		resellerID = *user.ResellerID
	}
	database.DB.Create(&models.Transaction{
		Type:          models.TransactionTypeSubscriberTopup,
		Amount:        req.Amount,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceBefore + req.Amount,
		Description:   fmt.Sprintf("Balance added: %s", req.Reason),
		ResellerID:    resellerID,
		SubscriberID:  &subscriber.ID,
		IPAddress:     c.IP(),
		CreatedBy:     user.ID,
	})

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionUpdate,
		EntityType:  "subscriber",
		EntityID:    subscriber.ID,
		EntityName:  subscriber.Username,
		NewValue:    fmt.Sprintf("Balance: %.2f → %.2f", balanceBefore, balanceBefore+req.Amount),
		Description: fmt.Sprintf("Added $%.2f to wallet. Reason: %s", req.Amount, req.Reason),
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Added $%.2f to wallet successfully", req.Amount),
		"data": fiber.Map{
			"amount":         req.Amount,
			"balance_before": balanceBefore,
			"balance_after":  balanceBefore + req.Amount,
		},
	})
}

// Refill is an alias for AddBalance (backward compatibility)
func (h *SubscriberHandler) Refill(c *fiber.Ctx) error {
	return h.AddBalance(c)
}

// Ping pings subscriber's IP address via MikroTik router
func (h *SubscriberHandler) Ping(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	var subscriber models.Subscriber
	if err := database.DB.First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	// Get IP address
	ipAddress := subscriber.IPAddress
	if ipAddress == "" {
		ipAddress = subscriber.StaticIP
	}
	if ipAddress == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "No IP address available"})
	}

	// Get NAS to ping through MikroTik
	if subscriber.NasID == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "No NAS assigned to subscriber"})
	}

	var nas models.Nas
	if err := database.DB.First(&nas, *subscriber.NasID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "NAS not found"})
	}

	// Connect to MikroTik and ping
	client := mikrotik.NewClient(
		fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
		nas.APIUsername,
		nas.APIPassword,
	)
	defer client.Close()

	pingResult, err := client.Ping(ipAddress, 4, 0)

	// Format output in Windows-like style
	var output strings.Builder
	output.WriteString(fmt.Sprintf("\nPinging %s via %s:\n\n", ipAddress, nas.Name))

	if err != nil {
		output.WriteString(fmt.Sprintf("Ping failed: %v\n", err))
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Ping completed",
			"data": fiber.Map{
				"ip":      ipAddress,
				"online":  subscriber.IsOnline,
				"output":  output.String(),
				"success": false,
			},
		})
	}

	if pingResult.Received > 0 {
		for i := 0; i < pingResult.Received; i++ {
			output.WriteString(fmt.Sprintf("Reply from %s: bytes=32 time=%.1fms TTL=64\n", ipAddress, pingResult.AvgRTT))
		}
	}
	if pingResult.Sent > pingResult.Received {
		for i := 0; i < pingResult.Sent-pingResult.Received; i++ {
			output.WriteString("Request timed out.\n")
		}
	}

	output.WriteString(fmt.Sprintf("\nPing statistics for %s:\n", ipAddress))
	output.WriteString(fmt.Sprintf("    Packets: Sent = %d, Received = %d, Lost = %d (%d%% loss)\n",
		pingResult.Sent, pingResult.Received, pingResult.Sent-pingResult.Received, pingResult.PacketLoss))

	if pingResult.Received > 0 {
		output.WriteString(fmt.Sprintf("Approximate round trip times in milli-seconds:\n"))
		output.WriteString(fmt.Sprintf("    Minimum = %.1fms, Maximum = %.1fms, Average = %.1fms\n",
			pingResult.MinRTT, pingResult.MaxRTT, pingResult.AvgRTT))
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Ping completed",
		"data": fiber.Map{
			"ip":      ipAddress,
			"online":  subscriber.IsOnline,
			"output":  output.String(),
			"success": pingResult.Received > 0,
		},
	})
}

// PortCheck checks if a TCP port is open on subscriber's IP via MikroTik /tool/fetch
func (h *SubscriberHandler) PortCheck(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	var req struct {
		Port int `json:"port"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if req.Port < 1 || req.Port > 65535 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Port must be between 1 and 65535"})
	}

	var subscriber models.Subscriber
	if err := database.DB.First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	// Get IP address
	ipAddress := subscriber.IPAddress
	if ipAddress == "" {
		ipAddress = subscriber.StaticIP
	}
	if ipAddress == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "No IP address available"})
	}

	// Get NAS
	if subscriber.NasID == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "No NAS assigned to subscriber"})
	}

	var nas models.Nas
	if err := database.DB.First(&nas, *subscriber.NasID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "NAS not found"})
	}

	// Connect to MikroTik
	client := mikrotik.NewClient(
		fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
		nas.APIUsername,
		nas.APIPassword,
	)
	defer client.Close()

	result, err := client.PortCheck(ipAddress, req.Port, 3)
	if err != nil {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Port check completed",
			"data": fiber.Map{
				"ip":     ipAddress,
				"port":   req.Port,
				"status": "error",
				"error":  err.Error(),
			},
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Port check completed",
		"data":    result,
	})
}

// execPing executes ping command and returns output in Windows-like format
func execPing(ip string) (string, error) {
	// Execute real ping command with 10 packets, 200ms interval, 1s timeout (~3 sec total)
	cmd := exec.Command("ping", "-c", "10", "-i", "0.2", "-W", "1", ip)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// If ping fails, return the error output
		return string(output), err
	}

	// Parse Linux ping output and convert to Windows-like format
	lines := strings.Split(string(output), "\n")
	var result strings.Builder

	result.WriteString(fmt.Sprintf("\nPinging %s with 32 bytes of data:\n\n", ip))

	received := 0
	var times []string

	for _, line := range lines {
		// Parse lines like: "64 bytes from 192.168.1.1: icmp_seq=1 ttl=64 time=1.23 ms"
		if strings.Contains(line, "bytes from") && strings.Contains(line, "time=") {
			received++
			// Extract time value
			timeIdx := strings.Index(line, "time=")
			if timeIdx != -1 {
				timeStr := line[timeIdx+5:]
				// Remove "ms" and any trailing content
				timeStr = strings.Split(timeStr, " ")[0]
				timeStr = strings.TrimSuffix(timeStr, "ms")
				times = append(times, timeStr)
				result.WriteString(fmt.Sprintf("Reply from %s: bytes=32 time=%sms TTL=64\n", ip, timeStr))
			}
		} else if strings.Contains(line, "Request timeout") || strings.Contains(line, "100% packet loss") {
			result.WriteString(fmt.Sprintf("Request timed out.\n"))
		}
	}

	// Add statistics
	result.WriteString(fmt.Sprintf("\nPing statistics for %s:\n", ip))
	result.WriteString(fmt.Sprintf("    Packets: Sent = 10, Received = %d, Lost = %d (%d%% loss)\n",
		received, 10-received, (10-received)*10))

	// Calculate timing statistics if we have times
	if len(times) > 0 {
		var min, max, sum float64
		min = 999999
		for _, t := range times {
			if val, err := strconv.ParseFloat(t, 64); err == nil {
				sum += val
				if val < min {
					min = val
				}
				if val > max {
					max = val
				}
			}
		}
		avg := sum / float64(len(times))
		result.WriteString(fmt.Sprintf("Approximate round trip times in milli-seconds:\n"))
		result.WriteString(fmt.Sprintf("    Minimum = %.0fms, Maximum = %.0fms, Average = %.0fms\n", min, max, avg))
	}

	return result.String(), nil
}

// CDNBandwidth represents bandwidth for a specific CDN
type CDNBandwidth struct {
	CDNID   uint    `json:"cdn_id"`
	CDNName string  `json:"cdn_name"`
	Bytes   int64   `json:"bytes"`
	Color   string  `json:"color"`
}

// PortRuleBandwidth represents bandwidth for a specific CDN Port Rule
type PortRuleBandwidth struct {
	RuleID   uint   `json:"rule_id"`
	RuleName string `json:"rule_name"`
	Bytes    int64  `json:"bytes"`
	Color    string `json:"color"`
}

// BandwidthResponse represents real-time bandwidth data
type BandwidthResponse struct {
	Timestamp       int64               `json:"timestamp"`
	Download        float64             `json:"download"`    // Mbps
	Upload          float64             `json:"upload"`      // Mbps
	RxBytes         int64               `json:"rx_bytes"`
	TxBytes         int64               `json:"tx_bytes"`
	Uptime          string              `json:"uptime"`
	IPAddress       string              `json:"ip_address"`
	CallerID        string              `json:"caller_id"`
	CDNTraffic      []CDNBandwidth      `json:"cdn_traffic,omitempty"`
	CDNIsRate       bool                `json:"cdn_is_rate"` // true = Bytes field is bytes/sec rate (Torch), false = cumulative (delta needed)
	PortRuleTraffic []PortRuleBandwidth `json:"port_rule_traffic,omitempty"`
	PingMs          float64             `json:"ping_ms"`  // RTT in milliseconds (0 if not available)
	PingOk          bool                `json:"ping_ok"`  // false = timeout or error
}

// GetBandwidth returns real-time bandwidth data for a subscriber
func (h *SubscriberHandler) GetBandwidth(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	var subscriber models.Subscriber
	if err := database.DB.Preload("Nas").First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	// Check if subscriber is online
	if !subscriber.IsOnline {
		return c.JSON(fiber.Map{
			"success": false,
			"message": "Subscriber is offline",
			"data": BandwidthResponse{
				Timestamp: time.Now().UnixMilli(),
			},
		})
	}

	// Check if NAS is configured
	if subscriber.Nas == nil || subscriber.Nas.IPAddress == "" {
		return c.JSON(fiber.Map{
			"success": false,
			"message": "NAS not configured",
			"data": BandwidthResponse{
				Timestamp: time.Now().UnixMilli(),
			},
		})
	}

	// Connect to MikroTik and get session info
	client := mikrotik.NewClient(
		fmt.Sprintf("%s:%d", subscriber.Nas.IPAddress, subscriber.Nas.APIPort),
		subscriber.Nas.APIUsername,
		subscriber.Nas.APIPassword,
	)
	defer client.Close()

	session, err := client.GetActiveSession(subscriber.Username)
	if err != nil {
		return c.JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
			"data": BandwidthResponse{
				Timestamp: time.Now().UnixMilli(),
			},
		})
	}

	// Convert bytes/sec to Mbps (megabits per second)
	// TxRate = user download, RxRate = user upload (from interface/queue perspective)
	downloadMbps := float64(session.TxRate) * 8 / 1000000
	uploadMbps := float64(session.RxRate) * 8 / 1000000

	response := BandwidthResponse{
		Timestamp:  time.Now().UnixMilli(),
		Download:   downloadMbps,
		Upload:     uploadMbps,
		RxBytes:    session.RxBytes,
		TxBytes:    session.TxBytes,
		Uptime:     session.Uptime,
		IPAddress:  session.Address,
		CallerID:   session.CallerID,
	}

	// Run live ping using existing connection (reuse avoids second concurrent login which MikroTik may reject)
	var pingMs float64
	var pingOk bool
	if session.Address != "" {
		if result, err := client.Ping(session.Address, 1, 0); err == nil && result.Received > 0 {
			pingMs = result.AvgRTT
			pingOk = true
		}
	}

	// Get CDN and Port Rule traffic breakdown using a single Torch run
	if session.Address != "" {
		// Build CDN configs (from service CDNs with is_active=true)
		var cdnConfigs []mikrotik.CDNSubnetConfig
		cdnColorMap := make(map[uint]string)
		defaultCDNColor := "#EF4444"

		if subscriber.ServiceID > 0 {
			var serviceCDNs []models.ServiceCDN
			database.DB.Preload("CDN").Where("service_id = ? AND is_active = ?", subscriber.ServiceID, true).Find(&serviceCDNs)
			for _, cdn := range serviceCDNs {
				if cdn.CDN != nil && cdn.CDN.ID > 0 && cdn.CDN.Subnets != "" {
					cdnConfigs = append(cdnConfigs, mikrotik.CDNSubnetConfig{
						ID:      cdn.CDNID,
						Name:    cdn.CDN.Name,
						Subnets: cdn.CDN.Subnets,
					})
					color := cdn.CDN.Color
					if color == "" {
						color = defaultCDNColor
					}
					cdnColorMap[cdn.CDNID] = color
				}
			}
		}

		// Build Port Rule configs (rules with show_in_graph=true, port-based only)
		var portRuleConfigs []mikrotik.PortRuleTrafficConfig
		portRuleColorMap := make(map[uint]string)
		defaultPortRuleColor := "#8B5CF6"

		var activePortRules []models.CDNPortRule
		database.DB.Where("deleted_at IS NULL AND is_active = ? AND show_in_graph = ? AND direction != 'dscp' AND port != ''", true, true).Find(&activePortRules)
		for _, pr := range activePortRules {
			portRuleConfigs = append(portRuleConfigs, mikrotik.PortRuleTrafficConfig{
				ID:        pr.ID,
				Name:      pr.Name,
				Port:      pr.Port,
				Direction: pr.Direction,
			})
			color := pr.Color
			if color == "" {
				color = defaultPortRuleColor
			}
			portRuleColorMap[pr.ID] = color
		}

		// Only run Torch if there's something to measure
		if len(cdnConfigs) > 0 || len(portRuleConfigs) > 0 {
			cdnCounters, portRuleCounters, err := client.GetCombinedTrafficViaTorch(session.Address, cdnConfigs, portRuleConfigs)
			log.Printf("CombinedTorch for %s: CDNs=%d PortRules=%d err=%v", session.Address, len(cdnCounters), len(portRuleCounters), err)

			if err == nil {
				response.CDNIsRate = true // Bytes = bytes/sec rate from Torch (no delta needed)

				for _, counter := range cdnCounters {
					color := cdnColorMap[counter.CDNID]
					if color == "" {
						color = defaultCDNColor
					}
					response.CDNTraffic = append(response.CDNTraffic, CDNBandwidth{
						CDNID:   counter.CDNID,
						CDNName: counter.CDNName,
						Bytes:   counter.Bytes,
						Color:   color,
					})
				}

				for _, counter := range portRuleCounters {
					color := portRuleColorMap[counter.RuleID]
					if color == "" {
						color = defaultPortRuleColor
					}
					response.PortRuleTraffic = append(response.PortRuleTraffic, PortRuleBandwidth{
						RuleID:   counter.RuleID,
						RuleName: counter.RuleName,
						Bytes:    counter.Bytes,
						Color:    color,
					})
				}
			}
		}
	}

	response.PingMs = pingMs
	response.PingOk = pingOk

	return c.JSON(fiber.Map{
		"success": true,
		"data":    response,
	})
}

// TorchResponse represents the response for live torch data (like MikroTik Winbox torch)
type TorchResponse struct {
	Entries   []TorchEntry `json:"entries"`
	TotalTx   int64        `json:"total_tx"`   // Total TX bytes/sec
	TotalRx   int64        `json:"total_rx"`   // Total RX bytes/sec
	Interface string       `json:"interface"`
	FilterIP  string       `json:"filter_ip"`
	Duration  string       `json:"duration"`
}

type TorchEntry struct {
	SrcAddress  string `json:"src_address"`
	DstAddress  string `json:"dst_address"`
	SrcPort     int    `json:"src_port"`
	DstPort     int    `json:"dst_port"`
	Protocol    string `json:"protocol"`      // tcp, udp, icmp
	ProtoNum    int    `json:"proto_num"`     // 6, 17, 1
	MacProtocol string `json:"mac_protocol"`  // 800=IPv4, 86dd=IPv6
	VlanID      int    `json:"vlan_id"`
	DSCP        int    `json:"dscp"`
	TxRate      int64  `json:"tx_rate"`       // bytes/sec
	RxRate      int64  `json:"rx_rate"`       // bytes/sec
	TxPackets   int64  `json:"tx_packets"`
	RxPackets   int64  `json:"rx_packets"`
}

// GetTorch returns real-time traffic breakdown using MikroTik torch
func (h *SubscriberHandler) GetTorch(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	// Get duration from query (default 3 seconds)
	duration, _ := strconv.Atoi(c.Query("duration", "3"))
	if duration <= 0 {
		duration = 3
	}
	if duration > 10 {
		duration = 10
	}

	var subscriber models.Subscriber
	if err := database.DB.Preload("Nas").First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	// Check if subscriber is online
	if !subscriber.IsOnline {
		return c.JSON(fiber.Map{
			"success": false,
			"message": "Subscriber is offline",
		})
	}

	// Check if NAS is configured
	if subscriber.Nas == nil || subscriber.Nas.IPAddress == "" {
		return c.JSON(fiber.Map{
			"success": false,
			"message": "NAS not configured",
		})
	}

	// Get subscriber's current IP
	if subscriber.IPAddress == "" {
		return c.JSON(fiber.Map{
			"success": false,
			"message": "Subscriber has no IP address assigned",
		})
	}

	// Connect to MikroTik and run torch
	client := mikrotik.NewClient(
		fmt.Sprintf("%s:%d", subscriber.Nas.IPAddress, subscriber.Nas.APIPort),
		subscriber.Nas.APIUsername,
		subscriber.Nas.APIPassword,
	)
	defer client.Close()

	torchResult, err := client.GetLiveTorch(subscriber.IPAddress, duration)
	if err != nil {
		return c.JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	// Convert to response format
	response := TorchResponse{
		TotalTx:   torchResult.TotalTx,
		TotalRx:   torchResult.TotalRx,
		Interface: torchResult.Interface,
		FilterIP:  torchResult.FilterIP,
		Duration:  torchResult.Duration,
		Entries:   make([]TorchEntry, len(torchResult.Entries)),
	}

	for i, e := range torchResult.Entries {
		response.Entries[i] = TorchEntry{
			SrcAddress:  e.SrcAddress,
			DstAddress:  e.DstAddress,
			SrcPort:     e.SrcPort,
			DstPort:     e.DstPort,
			Protocol:    e.Protocol,
			ProtoNum:    e.ProtoNum,
			MacProtocol: e.MacProto,
			VlanID:      e.VlanID,
			DSCP:        e.DSCP,
			TxRate:      e.TxRate,
			RxRate:      e.RxRate,
			TxPackets:   e.TxPackets,
			RxPackets:   e.RxPackets,
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    response,
	})
}

// isIPInSubnets checks if an IP address is within any of the given subnets (comma-separated CIDR notation)
func isIPInSubnets(ipStr, subnets string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// Split subnets by comma or newline
	subnetList := strings.FieldsFunc(subnets, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})

	for _, subnet := range subnetList {
		subnet = strings.TrimSpace(subnet)
		if subnet == "" {
			continue
		}

		_, network, err := net.ParseCIDR(subnet)
		if err != nil {
			continue
		}

		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// ============================================
// Subscriber Bandwidth Rules Handlers
// ============================================

// GetBandwidthRules returns all bandwidth rules for a subscriber
func (h *SubscriberHandler) GetBandwidthRules(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	// Verify subscriber exists
	var subscriber models.Subscriber
	if err := database.DB.First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	var rules []models.SubscriberBandwidthRule
	database.DB.Where("subscriber_id = ?", id).Order("priority DESC, id ASC").Find(&rules)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    rules,
	})
}

// GetCDNUpgrades returns available CDN speed upgrades for a subscriber
func (h *SubscriberHandler) GetCDNUpgrades(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	// Get subscriber with service
	var subscriber models.Subscriber
	if err := database.DB.Preload("Service").First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	// Get current service's CDN configurations
	type ServiceCDN struct {
		ID         uint   `json:"id"`
		CDNID      uint   `json:"cdn_id"`
		CDNName    string `json:"cdn_name"`
		SpeedLimit int64  `json:"speed_limit"`
		ServiceID  uint   `json:"service_id"`
		ServiceName string `json:"service_name"`
	}

	var currentCDNs []ServiceCDN
	database.DB.Raw(`
		SELECT sc.id, sc.cdn_id, c.name as cdn_name, sc.speed_limit, sc.service_id, s.name as service_name
		FROM service_cdns sc
		JOIN cdns c ON sc.cdn_id = c.id
		JOIN services s ON sc.service_id = s.id
		WHERE sc.service_id = ? AND sc.is_active = true
	`, subscriber.ServiceID).Scan(&currentCDNs)

	// Build map of current CDN speeds
	currentSpeeds := make(map[uint]int64)
	for _, cdn := range currentCDNs {
		currentSpeeds[cdn.CDNID] = cdn.SpeedLimit
	}

	// Get all CDN upgrades (higher speeds from any service)
	var upgrades []ServiceCDN
	database.DB.Raw(`
		SELECT sc.id, sc.cdn_id, c.name as cdn_name, sc.speed_limit, sc.service_id, s.name as service_name
		FROM service_cdns sc
		JOIN cdns c ON sc.cdn_id = c.id
		JOIN services s ON sc.service_id = s.id
		WHERE sc.is_active = true AND s.is_active = true
		ORDER BY c.name, sc.speed_limit
	`).Scan(&upgrades)

	// Filter to only include upgrades (higher speeds than current)
	var availableUpgrades []map[string]interface{}
	for _, upgrade := range upgrades {
		currentSpeed, exists := currentSpeeds[upgrade.CDNID]
		// Include if: it's a higher speed, OR it's a CDN the user doesn't have yet
		if upgrade.SpeedLimit > currentSpeed || !exists {
			availableUpgrades = append(availableUpgrades, map[string]interface{}{
				"cdn_id":       upgrade.CDNID,
				"cdn_name":     upgrade.CDNName,
				"speed_limit":  upgrade.SpeedLimit,
				"service_id":   upgrade.ServiceID,
				"service_name": upgrade.ServiceName,
				"label":        fmt.Sprintf("%s - %dM (from %s)", upgrade.CDNName, upgrade.SpeedLimit, upgrade.ServiceName),
			})
		}
	}

	return c.JSON(fiber.Map{
		"success":        true,
		"current_cdns":   currentCDNs,
		"available_upgrades": availableUpgrades,
	})
}

// CreateBandwidthRule creates a new bandwidth rule for a subscriber
func (h *SubscriberHandler) CreateBandwidthRule(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	// Verify subscriber exists
	var subscriber models.Subscriber
	if err := database.DB.First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	var req struct {
		RuleType      string `json:"rule_type"`
		Enabled       bool   `json:"enabled"`
		DownloadSpeed int    `json:"download_speed"`
		UploadSpeed   int    `json:"upload_speed"`
		CDNID         uint   `json:"cdn_id"`
		Duration      string `json:"duration"` // "1h", "2h", "6h", "12h", "1d", "2d", "7d", "permanent"
		Priority      int    `json:"priority"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	// Validate rule type
	if req.RuleType != "internet" && req.RuleType != "cdn" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Rule type must be 'internet' or 'cdn'"})
	}

	// Look up CDN name if cdn_id is provided
	var cdnName string
	if req.CDNID > 0 {
		var cdn models.CDN
		if err := database.DB.First(&cdn, req.CDNID).Error; err == nil {
			cdnName = cdn.Name
		}
	}

	// Calculate expires_at from duration
	var expiresAt *time.Time
	if req.Duration != "" && req.Duration != "permanent" {
		expiry, err := parseDuration(req.Duration)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid duration format"})
		}
		expiresAt = &expiry
	}

	rule := models.SubscriberBandwidthRule{
		SubscriberID:  uint(id),
		RuleType:      models.SubscriberBandwidthRuleType(req.RuleType),
		Enabled:       req.Enabled,
		DownloadSpeed: req.DownloadSpeed,
		UploadSpeed:   req.UploadSpeed,
		CDNID:         req.CDNID,
		CDNName:       cdnName,
		Duration:      req.Duration,
		ExpiresAt:     expiresAt,
		Priority:      req.Priority,
	}

	if err := database.DB.Create(&rule).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to create rule"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Bandwidth rule created",
		"data":    rule,
	})
}

// parseDuration parses duration strings like "1h", "2h", "1d", "2d" and returns the expiry time
func parseDuration(duration string) (time.Time, error) {
	now := time.Now()

	// Handle days format (e.g., "1d", "2d", "7d")
	if strings.HasSuffix(duration, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(duration, "d"))
		if err != nil {
			return time.Time{}, err
		}
		return now.Add(time.Duration(days) * 24 * time.Hour), nil
	}

	// Handle hours format (e.g., "1h", "2h", "6h", "12h")
	if strings.HasSuffix(duration, "h") {
		hours, err := strconv.Atoi(strings.TrimSuffix(duration, "h"))
		if err != nil {
			return time.Time{}, err
		}
		return now.Add(time.Duration(hours) * time.Hour), nil
	}

	return time.Time{}, fmt.Errorf("invalid duration format: %s", duration)
}

// UpdateBandwidthRule updates a bandwidth rule
func (h *SubscriberHandler) UpdateBandwidthRule(c *fiber.Ctx) error {
	subscriberID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	ruleID, err := strconv.Atoi(c.Params("ruleId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid rule ID"})
	}

	var rule models.SubscriberBandwidthRule
	if err := database.DB.Where("id = ? AND subscriber_id = ?", ruleID, subscriberID).First(&rule).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Rule not found"})
	}

	var req struct {
		RuleType      string  `json:"rule_type"`
		Enabled       *bool   `json:"enabled"`
		DownloadSpeed *int    `json:"download_speed"`
		UploadSpeed   *int    `json:"upload_speed"`
		CDNID         *uint   `json:"cdn_id"`
		Duration      *string `json:"duration"` // "1h", "2h", "6h", "12h", "1d", "2d", "7d", "permanent"
		Priority      *int    `json:"priority"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	updates := make(map[string]interface{})

	if req.RuleType != "" {
		if req.RuleType != "internet" && req.RuleType != "cdn" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Rule type must be 'internet' or 'cdn'"})
		}
		updates["rule_type"] = req.RuleType
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.DownloadSpeed != nil {
		updates["download_speed"] = *req.DownloadSpeed
	}
	if req.UploadSpeed != nil {
		updates["upload_speed"] = *req.UploadSpeed
	}
	// Handle CDN ID update
	if req.CDNID != nil {
		updates["cdn_id"] = *req.CDNID
		if *req.CDNID > 0 {
			var cdn models.CDN
			if err := database.DB.First(&cdn, *req.CDNID).Error; err == nil {
				updates["cdn_name"] = cdn.Name
			}
		} else {
			updates["cdn_name"] = ""
		}
	}
	// Handle duration update
	if req.Duration != nil {
		updates["duration"] = *req.Duration
		if *req.Duration == "" || *req.Duration == "permanent" {
			updates["expires_at"] = nil
		} else {
			expiry, err := parseDuration(*req.Duration)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid duration format"})
			}
			updates["expires_at"] = expiry
		}
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}

	if err := database.DB.Model(&rule).Updates(updates).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to update rule"})
	}

	// Reload rule
	database.DB.First(&rule, ruleID)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Bandwidth rule updated",
		"data":    rule,
	})
}

// DeleteBandwidthRule deletes a bandwidth rule
func (h *SubscriberHandler) DeleteBandwidthRule(c *fiber.Ctx) error {
	subscriberID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	ruleID, err := strconv.Atoi(c.Params("ruleId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid rule ID"})
	}

	result := database.DB.Where("id = ? AND subscriber_id = ?", ruleID, subscriberID).Delete(&models.SubscriberBandwidthRule{})
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Rule not found"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Bandwidth rule deleted",
	})
}

// BulkImportExcelItem represents a single subscriber in Excel bulk import
type BulkImportExcelItem struct {
	Username    string `json:"username"`
	FullName    string `json:"full_name"`
	Password    string `json:"password"`
	Service     string `json:"service"`
	Expiry      string `json:"expiry"`
	Phone       string `json:"phone"`
	Address     string `json:"address"`
	Region      string `json:"region"`
	Building    string `json:"building"`
	Nationality string `json:"nationality"`
	Country     string `json:"country"`
	MACAddress  string `json:"mac_address"`
	Note        string `json:"note"`
	Reseller    string `json:"reseller"`
	Blocked     string `json:"blocked"`
}

// BulkImportExcelRequest represents the request for Excel bulk import
type BulkImportExcelRequest struct {
	Data  []BulkImportExcelItem `json:"data"`
	NasID uint                  `json:"nas_id"`
}

// BulkImportExcelResult represents result for each row in Excel import
type BulkImportExcelResult struct {
	Row      int    `json:"row"`
	Username string `json:"username"`
	Status   string `json:"status"`
	Message  string `json:"message"`
}

// BulkImportExcel imports multiple subscribers from Excel data (parsed by frontend)
func (h *SubscriberHandler) BulkImportExcel(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized",
		})
	}

	// Only admin can bulk import
	if user.UserType != models.UserTypeAdmin {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"message": "Only admin can bulk import",
		})
	}

	var req BulkImportExcelRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if len(req.Data) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "No data to import",
		})
	}

	// Check license limit
	var currentCount int64
	database.DB.Model(&models.Subscriber{}).Count(&currentCount)
	canAdd, _, maxAllowed, err := license.CanAddSubscriber(int(currentCount))
	if err != nil || !canAdd {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Cannot add subscribers. License limit is %d, current count is %d", maxAllowed, currentCount),
		})
	}
	// Check if we can add all the requested subscribers
	if int(currentCount)+len(req.Data) > maxAllowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Cannot add %d subscribers. License limit is %d, current count is %d", len(req.Data), maxAllowed, currentCount),
		})
	}

	// Cache services and resellers for lookup
	var services []models.Service
	database.DB.Find(&services)
	serviceMap := make(map[string]models.Service)
	for _, s := range services {
		serviceMap[strings.ToLower(s.Name)] = s
	}

	var resellers []models.Reseller
	database.DB.Find(&resellers)
	resellerMap := make(map[string]uint)
	for _, r := range resellers {
		resellerMap[strings.ToLower(r.Name)] = r.ID
	}

	results := make([]BulkImportExcelResult, 0, len(req.Data))
	successCount := 0
	failCount := 0

	for i, item := range req.Data {
		rowNum := i + 3 // Row 1 is header, Row 2 is description, data starts at Row 3
		result := BulkImportExcelResult{
			Row:      rowNum,
			Username: item.Username,
		}

		// Validate required fields
		if item.Username == "" {
			result.Status = "failed"
			result.Message = "Username is required"
			results = append(results, result)
			failCount++
			continue
		}

		if item.Password == "" {
			result.Status = "failed"
			result.Message = "Password is required"
			results = append(results, result)
			failCount++
			continue
		}

		if item.Service == "" {
			result.Status = "failed"
			result.Message = "Service is required"
			results = append(results, result)
			failCount++
			continue
		}

		// Find service
		service, ok := serviceMap[strings.ToLower(item.Service)]
		if !ok {
			result.Status = "failed"
			result.Message = fmt.Sprintf("Service '%s' not found", item.Service)
			results = append(results, result)
			failCount++
			continue
		}

		// Check if username exists
		var existingCount int64
		database.DB.Model(&models.Subscriber{}).Where("username = ?", item.Username).Count(&existingCount)
		if existingCount > 0 {
			result.Status = "failed"
			result.Message = "Username already exists"
			results = append(results, result)
			failCount++
			continue
		}

		// Parse expiry date
		var expiryDate time.Time
		if item.Expiry != "" {
			parsed, err := time.Parse("2006-01-02", item.Expiry)
			if err != nil {
				// Try other formats
				parsed, err = time.Parse("02/01/2006", item.Expiry)
				if err != nil {
					parsed, err = time.Parse("01/02/2006", item.Expiry)
					if err != nil {
						result.Status = "failed"
						result.Message = fmt.Sprintf("Invalid expiry date format: %s", item.Expiry)
						results = append(results, result)
						failCount++
						continue
					}
				}
			}
			expiryDate = parsed
		} else {
			// Default to 30 days from now
			expiryDate = time.Now().AddDate(0, 0, 30)
		}

		// Find reseller
		var resellerID uint = 0
		if item.Reseller != "" {
			if rid, ok := resellerMap[strings.ToLower(item.Reseller)]; ok {
				resellerID = rid
			}
		}

		// Determine status
		status := models.SubscriberStatusActive
		if item.Blocked == "1" || strings.ToLower(item.Blocked) == "true" || strings.ToLower(item.Blocked) == "yes" {
			status = models.SubscriberStatusStopped
		}

		// Encrypt password
		encryptedPassword := security.EncryptPassword(item.Password)

		// Create subscriber
		subscriber := models.Subscriber{
			Username:      item.Username,
			Password:      item.Password, // Plain for RADIUS PAP
			PasswordPlain: encryptedPassword,
			FullName:      item.FullName,
			Email:         "",
			Phone:         item.Phone,
			Address:       item.Address,
			Region:        item.Region,
			Building:      item.Building,
			Nationality:   item.Nationality,
			Country:       item.Country,
			Note:          item.Note,
			ServiceID:     service.ID,
			Status:        status,
			ExpiryDate:    expiryDate,
			Price:         service.Price,
			ResellerID:    resellerID,
			MACAddress:    strings.ToUpper(item.MACAddress),
			SaveMAC:       item.MACAddress != "",
			NasID:         &req.NasID,
		}

		if err := database.DB.Create(&subscriber).Error; err != nil {
			result.Status = "failed"
			result.Message = fmt.Sprintf("Database error: %v", err)
			results = append(results, result)
			failCount++
			continue
		}

		// Create RADIUS check attributes
		radCheck := []models.RadCheck{
			{Username: subscriber.Username, Attribute: "Cleartext-Password", Op: ":=", Value: item.Password},
			{Username: subscriber.Username, Attribute: "Expiration", Op: ":=", Value: expiryDate.Format("Jan 02 2006 15:04:05")},
			{Username: subscriber.Username, Attribute: "Simultaneous-Use", Op: ":=", Value: "1"},
		}
		database.DB.Create(&radCheck)

		// Create RADIUS reply attributes
		var radReply []models.RadReply
		uploadSpeed := service.UploadSpeedStr
		downloadSpeed := service.DownloadSpeedStr
		if uploadSpeed == "" && service.UploadSpeed > 0 {
			uploadSpeed = fmt.Sprintf("%dM", service.UploadSpeed)
		}
		if downloadSpeed == "" && service.DownloadSpeed > 0 {
			downloadSpeed = fmt.Sprintf("%dM", service.DownloadSpeed)
		}
		if uploadSpeed != "" || downloadSpeed != "" {
			rateLimit := fmt.Sprintf("%s/%s", uploadSpeed, downloadSpeed)
			radReply = append(radReply, models.RadReply{Username: subscriber.Username, Attribute: "Mikrotik-Rate-Limit", Op: "=", Value: rateLimit})
		}
		if service.PoolName != "" {
			radReply = append(radReply, models.RadReply{Username: subscriber.Username, Attribute: "Framed-Pool", Op: "=", Value: service.PoolName})
		}
		if len(radReply) > 0 {
			database.DB.Create(&radReply)
		}

		result.Status = "success"
		result.Message = "Imported successfully"
		results = append(results, result)
		successCount++
	}

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionCreate,
		EntityType:  "subscriber",
		EntityID:    0,
		EntityName:  "Bulk Import",
		Description: fmt.Sprintf("Bulk imported %d subscribers (%d success, %d failed)", len(req.Data), successCount, failCount),
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Import completed: %d success, %d failed", successCount, failCount),
		"data": fiber.Map{
			"total":   len(req.Data),
			"success": successCount,
			"failed":  failCount,
			"results": results,
		},
	})
}

// SkipWanCheck marks a subscriber's WAN check as skipped (admin override).
// If the subscriber was previously blocked, they need to reconnect to get
// normal speed (CoA rate-limit is session-based, not radreply-based).
func (h *SubscriberHandler) SkipWanCheck(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	var sub models.Subscriber
	if err := database.DB.First(&sub, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	if err := database.DB.Model(&sub).Update("wan_check_status", "skipped").Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to update status"})
	}

	// If subscriber is online and was blocked, disconnect them so they
	// reconnect with normal speed from radreply.
	if sub.IsOnline && sub.WanCheckStatus == "failed" && sub.NasID != nil {
		var nas models.Nas
		if err := database.DB.First(&nas, *sub.NasID).Error; err == nil {
			client := mikrotik.NewClient(
				fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
				nas.APIUsername,
				nas.APIPassword,
			)
			defer client.Close()
			if err := client.DisconnectUser(sub.Username); err != nil {
				log.Printf("WanCheck: Failed to disconnect %s after skip: %v", sub.Username, err)
			} else {
				log.Printf("WanCheck: Disconnected %s after skip (will reconnect with normal speed)", sub.Username)
			}
		}
	}

	log.Printf("WanCheck: Admin skipped WAN check for subscriber %s (ID=%d)", sub.Username, sub.ID)
	return c.JSON(fiber.Map{"success": true, "message": "WAN check skipped"})
}

// RecheckWan resets a subscriber's WAN check status to 'unchecked' so
// QuotaSync will re-check on the next cycle.
func (h *SubscriberHandler) RecheckWan(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid subscriber ID"})
	}

	var sub models.Subscriber
	if err := database.DB.First(&sub, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Subscriber not found"})
	}

	if err := database.DB.Model(&sub).Update("wan_check_status", "unchecked").Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to update status"})
	}

	log.Printf("WanCheck: Admin triggered re-check for subscriber %s (ID=%d)", sub.Username, sub.ID)
	return c.JSON(fiber.Map{"success": true, "message": "WAN check will re-run on next cycle"})
}
