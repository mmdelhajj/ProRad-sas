package handlers

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/middleware"
	"github.com/proisp/backend/internal/mikrotik"
	"github.com/proisp/backend/internal/models"
)

type BandwidthCustomerHandler struct{}

func NewBandwidthCustomerHandler() *BandwidthCustomerHandler {
	return &BandwidthCustomerHandler{}
}

// sanitizeName generates a queue name from customer name
func sanitizeName(name string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9-]`)
	sanitized := re.ReplaceAllString(strings.TrimSpace(name), "-")
	if len(sanitized) > 60 {
		sanitized = sanitized[:60]
	}
	return "bw-" + sanitized
}

// List returns bandwidth customers with pagination, search, filters
func (h *BandwidthCustomerHandler) List(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "25"))
	search := c.Query("search", "")
	status := c.Query("status", "")
	nasID, _ := strconv.Atoi(c.Query("nas_id", "0"))
	sortBy := c.Query("sort_by", "name")
	sortDir := c.Query("sort_dir", "asc")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 25
	}

	query := database.DB.Model(&models.BandwidthCustomer{})

	if search != "" {
		pattern := "%" + search + "%"
		query = query.Where("name ILIKE ? OR ip_address ILIKE ? OR contact_person ILIKE ? OR phone ILIKE ?",
			pattern, pattern, pattern, pattern)
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if nasID > 0 {
		query = query.Where("nas_id = ?", nasID)
	}

	var total int64
	query.Count(&total)

	// Sorting
	allowedSorts := map[string]bool{"name": true, "ip_address": true, "status": true, "price": true, "created_at": true, "download_speed": true}
	if !allowedSorts[sortBy] {
		sortBy = "name"
	}
	if sortDir != "desc" {
		sortDir = "asc"
	}

	offset := (page - 1) * limit
	var customers []models.BandwidthCustomer
	if err := query.Order(sortBy + " " + sortDir).Offset(offset).Limit(limit).Find(&customers).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to fetch customers"})
	}

	// Load NAS info manually (to avoid garble issues with Preload)
	nasIDs := make([]uint, 0)
	for _, cust := range customers {
		if cust.NasID != nil {
			nasIDs = append(nasIDs, *cust.NasID)
		}
	}
	nasMap := make(map[uint]*models.Nas)
	if len(nasIDs) > 0 {
		var nasList []models.Nas
		database.DB.Where("id IN ?", nasIDs).Find(&nasList)
		for i := range nasList {
			nasList[i].HasSecret = nasList[i].Secret != ""
			nasList[i].HasAPIPassword = nasList[i].APIPassword != ""
			nasMap[nasList[i].ID] = &nasList[i]
		}
		for i := range customers {
			if customers[i].NasID != nil {
				customers[i].Nas = nasMap[*customers[i].NasID]
			}
		}
	}

	// Stats
	var stats struct {
		Total     int64   `json:"total"`
		Active    int64   `json:"active"`
		Suspended int64   `json:"suspended"`
		Expired   int64   `json:"expired"`
		Online    int64   `json:"online"`
		Revenue   float64 `json:"revenue"`
	}
	database.DB.Model(&models.BandwidthCustomer{}).Count(&stats.Total)
	database.DB.Model(&models.BandwidthCustomer{}).Where("status = ?", "active").Count(&stats.Active)
	database.DB.Model(&models.BandwidthCustomer{}).Where("status = ?", "suspended").Count(&stats.Suspended)
	database.DB.Model(&models.BandwidthCustomer{}).Where("status = ?", "expired").Count(&stats.Expired)
	database.DB.Model(&models.BandwidthCustomer{}).Where("is_online = ?", true).Count(&stats.Online)
	database.DB.Model(&models.BandwidthCustomer{}).Where("status = ?", "active").Select("COALESCE(SUM(price), 0)").Scan(&stats.Revenue)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    customers,
		"total":   total,
		"page":    page,
		"limit":   limit,
		"stats":   stats,
	})
}

// Get returns a single bandwidth customer by ID
func (h *BandwidthCustomerHandler) Get(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	var customer models.BandwidthCustomer
	if err := database.DB.First(&customer, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Customer not found"})
	}

	// Load NAS
	if customer.NasID != nil {
		var nas models.Nas
		if err := database.DB.First(&nas, *customer.NasID).Error; err == nil {
			nas.HasSecret = nas.Secret != ""
			nas.HasAPIPassword = nas.APIPassword != ""
			customer.Nas = &nas
		}
	}

	return c.JSON(fiber.Map{"success": true, "data": customer})
}

// Create creates a new bandwidth customer
func (h *BandwidthCustomerHandler) Create(c *fiber.Ctx) error {
	var customer models.BandwidthCustomer
	if err := c.BodyParser(&customer); err != nil {
		log.Printf("BW Create BodyParser error: %v | Body: %s", err, string(c.Body()))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body: " + err.Error()})
	}

	log.Printf("BW Create: name=%s public_ip=%s public_subnet=%s public_gateway=%s ip_block_id=%v assignedIPs_count=%d",
		customer.Name, customer.PublicIP, customer.PublicSubnet, customer.PublicGateway, customer.IPBlockID, len(strings.Split(customer.PublicIP, ",")))

	if customer.Name == "" || customer.IPAddress == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Name and IP address are required"})
	}

	if customer.DownloadSpeed <= 0 || customer.UploadSpeed <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Download and upload speed are required"})
	}

	// Check IP uniqueness in both tables
	var count int64
	database.DB.Model(&models.BandwidthCustomer{}).Where("ip_address = ?", customer.IPAddress).Count(&count)
	if count > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": "IP address already used by another bandwidth customer"})
	}
	database.DB.Model(&models.Subscriber{}).Where("ip_address = ? AND is_online = true", customer.IPAddress).Count(&count)
	if count > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": "IP address currently in use by a PPPoE subscriber"})
	}

	// Generate queue name
	customer.QueueName = sanitizeName(customer.Name)
	customer.Status = "active"

	if customer.SpeedSource == "" {
		customer.SpeedSource = "queue"
	}

	if err := database.DB.Create(&customer).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to create customer: " + err.Error()})
	}

	// Mark public IPs as assigned in the pool
	if customer.PublicIP != "" && customer.IPBlockID != nil {
		h.assignPublicIPs(&customer)
	}

	// Create MikroTik queue + VLAN + route
	if customer.NasID != nil {
		go h.syncToNAS(&customer)
		go h.setupNetworkOnNAS(&customer)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": customer, "message": "Customer created successfully"})
}

// Update updates a bandwidth customer
func (h *BandwidthCustomerHandler) Update(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	var existing models.BandwidthCustomer
	if err := database.DB.First(&existing, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Customer not found"})
	}

	var input models.BandwidthCustomer
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	// Check IP uniqueness if changed
	if input.IPAddress != "" && input.IPAddress != existing.IPAddress {
		var count int64
		database.DB.Model(&models.BandwidthCustomer{}).Where("ip_address = ? AND id != ?", input.IPAddress, id).Count(&count)
		if count > 0 {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"success": false, "message": "IP address already in use"})
		}
	}

	// Allowed fields for update
	updates := map[string]interface{}{
		"name": input.Name, "contact_person": input.ContactPerson, "phone": input.Phone,
		"email": input.Email, "address": input.Address, "notes": input.Notes,
		"ip_address": input.IPAddress, "subnet_mask": input.SubnetMask, "gateway": input.Gateway,
		"nas_id": input.NasID, "interface": input.Interface, "vlan_id": input.VlanID,
		"public_ip": input.PublicIP, "public_subnet": input.PublicSubnet, "public_gateway": input.PublicGateway,
		"download_speed": input.DownloadSpeed, "upload_speed": input.UploadSpeed,
		"cdn_download_speed": input.CDNDownloadSpeed, "cdn_upload_speed": input.CDNUploadSpeed,
		"speed_source": input.SpeedSource,
		"burst_enabled": input.BurstEnabled, "burst_download": input.BurstDownload, "burst_upload": input.BurstUpload,
		"burst_threshold_dl": input.BurstThresholdDl, "burst_threshold_ul": input.BurstThresholdUl, "burst_time": input.BurstTime,
		"fup_enabled": input.FUPEnabled, "daily_quota": input.DailyQuota,
		"fup1_threshold": input.FUP1Threshold, "fup1_speed": input.FUP1Speed,
		"fup2_threshold": input.FUP2Threshold, "fup2_speed": input.FUP2Speed,
		"fup3_threshold": input.FUP3Threshold, "fup3_speed": input.FUP3Speed,
		"monthly_quota": input.MonthlyQuota,
		"monthly_fup1_threshold": input.MonthlyFUP1Threshold, "monthly_fup1_speed": input.MonthlyFUP1Speed,
		"monthly_fup2_threshold": input.MonthlyFUP2Threshold, "monthly_fup2_speed": input.MonthlyFUP2Speed,
		"monthly_fup3_threshold": input.MonthlyFUP3Threshold, "monthly_fup3_speed": input.MonthlyFUP3Speed,
		"price": input.Price, "billing_cycle": input.BillingCycle,
		"start_date": input.StartDate, "expiry_date": input.ExpiryDate, "auto_renew": input.AutoRenew,
		"status": input.Status,
	}

	// Update queue name if name changed
	if input.Name != "" && input.Name != existing.Name {
		updates["queue_name"] = sanitizeName(input.Name)
	}

	if err := database.DB.Model(&existing).Updates(updates).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to update customer"})
	}

	// Reload
	database.DB.First(&existing, id)

	// If speed changed, update MikroTik queue
	speedChanged := input.DownloadSpeed != existing.DownloadSpeed || input.UploadSpeed != existing.UploadSpeed
	if speedChanged && existing.NasID != nil {
		go h.syncToNAS(&existing)
	}

	// If network config changed, setup VLAN/IP/route on MikroTik
	if existing.NasID != nil {
		go h.setupNetworkOnNAS(&existing)
	}

	return c.JSON(fiber.Map{"success": true, "data": existing, "message": "Customer updated successfully"})
}

// Delete soft-deletes a bandwidth customer and removes MikroTik queue
func (h *BandwidthCustomerHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	var customer models.BandwidthCustomer
	if err := database.DB.First(&customer, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Customer not found"})
	}

	// Remove MikroTik queue
	if customer.NasID != nil && customer.QueueName != "" {
		go h.removeFromNAS(&customer)
	}

	// Remove MikroTik config (routes, IP address, VLAN)
	if customer.NasID != nil {
		go h.cleanupNASConfig(&customer)
	}

	// Release public IP assignments for this bandwidth customer
	h.releasePublicIPs(&customer)

	if err := database.DB.Delete(&customer).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to delete customer"})
	}

	return c.JSON(fiber.Map{"success": true, "message": "Customer deleted successfully"})
}

// Suspend sets a bandwidth customer to suspended with 1k/1k speed
func (h *BandwidthCustomerHandler) Suspend(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	var customer models.BandwidthCustomer
	if err := database.DB.First(&customer, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Customer not found"})
	}

	database.DB.Model(&customer).Update("status", "suspended")

	// Set queue to 1k/1k on MikroTik
	if customer.NasID != nil {
		go func() {
			log.Printf("BW Suspend: Throttling queue %s to 1k/1k for customer %s", customer.QueueName, customer.Name)
			var nas models.Nas
			if err := database.DB.First(&nas, *customer.NasID).Error; err != nil {
				log.Printf("BW Suspend: NAS %d not found: %v", *customer.NasID, err)
				return
			}
			addr := fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort)
			log.Printf("BW Suspend: Connecting to NAS %s at %s", nas.Name, addr)
			client := mikrotik.NewClient(addr, nas.APIUsername, nas.APIPassword)
			burst := mikrotik.BWBurstConfig{}
			if err := client.UpdateBWSimpleQueue(customer.QueueName, 1, 1, burst); err != nil {
				log.Printf("BW Suspend: Failed to throttle queue %s: %v", customer.QueueName, err)
			} else {
				log.Printf("BW Suspend: Successfully throttled queue %s to 1k/1k", customer.QueueName)
			}
		}()
	} else {
		log.Printf("BW Suspend: Customer %s has no NAS configured, skipping queue throttle", customer.Name)
	}

	return c.JSON(fiber.Map{"success": true, "message": "Customer suspended"})
}

// Unsuspend restores a suspended customer to active with original speed
func (h *BandwidthCustomerHandler) Unsuspend(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	var customer models.BandwidthCustomer
	if err := database.DB.First(&customer, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Customer not found"})
	}

	database.DB.Model(&customer).Update("status", "active")

	// Restore speed
	if customer.NasID != nil {
		go h.syncToNAS(&customer)
	}

	return c.JSON(fiber.Map{"success": true, "message": "Customer unsuspended"})
}

// ResetFUP resets daily FUP counters and restores original speed
func (h *BandwidthCustomerHandler) ResetFUP(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	var customer models.BandwidthCustomer
	if err := database.DB.First(&customer, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Customer not found"})
	}

	now := time.Now()
	database.DB.Model(&customer).Updates(map[string]interface{}{
		"daily_download_used": 0,
		"daily_upload_used":   0,
		"fup_level":           0,
		"last_daily_reset":    now,
	})

	// Reset baseline bytes so sync doesn't re-add old usage
	if customer.NasID != nil {
		go func() {
			var nas models.Nas
			if err := database.DB.First(&nas, *customer.NasID).Error; err != nil {
				return
			}
			client := mikrotik.NewClient(fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort), nas.APIUsername, nas.APIPassword)
			stats, err := client.GetBWQueueStats(customer.QueueName)
			if err == nil && stats.Exists {
				database.DB.Model(&customer).Updates(map[string]interface{}{
					"last_queue_bytes_in":  stats.BytesIn,
					"last_queue_bytes_out": stats.BytesOut,
				})
			}
			// Restore original speed
			burst := mikrotik.BWBurstConfig{
				Enabled: customer.BurstEnabled, BurstDl: customer.BurstDownload, BurstUl: customer.BurstUpload,
				ThresholdDl: customer.BurstThresholdDl, ThresholdUl: customer.BurstThresholdUl, BurstTime: customer.BurstTime,
			}
			client.UpdateBWSimpleQueue(customer.QueueName, customer.DownloadSpeed, customer.UploadSpeed, burst)
		}()
	}

	return c.JSON(fiber.Map{"success": true, "message": "FUP reset successfully"})
}

// ChangeSpeed updates the customer's speed
func (h *BandwidthCustomerHandler) ChangeSpeed(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	var req struct {
		DownloadSpeed int `json:"download_speed"`
		UploadSpeed   int `json:"upload_speed"`
	}
	if err := c.BodyParser(&req); err != nil || req.DownloadSpeed <= 0 || req.UploadSpeed <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Valid download_speed and upload_speed required"})
	}

	var customer models.BandwidthCustomer
	if err := database.DB.First(&customer, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Customer not found"})
	}

	database.DB.Model(&customer).Updates(map[string]interface{}{
		"download_speed": req.DownloadSpeed,
		"upload_speed":   req.UploadSpeed,
	})

	customer.DownloadSpeed = req.DownloadSpeed
	customer.UploadSpeed = req.UploadSpeed

	if customer.NasID != nil {
		go h.syncToNAS(&customer)
	}

	return c.JSON(fiber.Map{"success": true, "message": fmt.Sprintf("Speed changed to %dk/%dk", req.DownloadSpeed, req.UploadSpeed)})
}

// GetBandwidth runs MikroTik torch for live bandwidth graph
func (h *BandwidthCustomerHandler) GetBandwidth(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	var customer models.BandwidthCustomer
	if err := database.DB.First(&customer, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Customer not found"})
	}

	if customer.NasID == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "No NAS configured"})
	}

	var nas models.Nas
	if err := database.DB.First(&nas, *customer.NasID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "NAS not found"})
	}

	client := mikrotik.NewClient(fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort), nas.APIUsername, nas.APIPassword)
	download, upload, err := client.GetBWTrafficViaTorch(customer.Interface, customer.IPAddress, 3)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Torch failed: " + err.Error()})
	}

	return c.JSON(fiber.Map{
		"success":  true,
		"download": download,
		"upload":   upload,
	})
}

// GetUsage returns daily usage history for charts
func (h *BandwidthCustomerHandler) GetUsage(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	days, _ := strconv.Atoi(c.Query("days", "30"))
	if days < 1 || days > 90 {
		days = 30
	}

	var history []models.BwDailyUsageHistory
	database.DB.Where("customer_id = ? AND date >= CURRENT_DATE - ? * INTERVAL '1 day'", id, days).
		Order("date ASC").Find(&history)

	return c.JSON(fiber.Map{"success": true, "data": history})
}

// GetStats returns aggregate stats for the dashboard
func (h *BandwidthCustomerHandler) GetStats(c *fiber.Ctx) error {
	var stats struct {
		Total     int64   `json:"total"`
		Active    int64   `json:"active"`
		Suspended int64   `json:"suspended"`
		Expired   int64   `json:"expired"`
		Online    int64   `json:"online"`
		Revenue   float64 `json:"revenue"`
	}

	database.DB.Model(&models.BandwidthCustomer{}).Count(&stats.Total)
	database.DB.Model(&models.BandwidthCustomer{}).Where("status = ?", "active").Count(&stats.Active)
	database.DB.Model(&models.BandwidthCustomer{}).Where("status = ?", "suspended").Count(&stats.Suspended)
	database.DB.Model(&models.BandwidthCustomer{}).Where("status = ?", "expired").Count(&stats.Expired)
	database.DB.Model(&models.BandwidthCustomer{}).Where("is_online = ?", true).Count(&stats.Online)
	database.DB.Model(&models.BandwidthCustomer{}).Where("status = ?", "active").Select("COALESCE(SUM(price), 0)").Scan(&stats.Revenue)

	return c.JSON(fiber.Map{"success": true, "data": stats})
}

// syncToNAS creates or updates MikroTik queue for the customer
func (h *BandwidthCustomerHandler) syncToNAS(customer *models.BandwidthCustomer) {
	if customer.NasID == nil {
		return
	}

	var nas models.Nas
	if err := database.DB.First(&nas, *customer.NasID).Error; err != nil {
		log.Printf("BW syncToNAS: NAS %d not found: %v", *customer.NasID, err)
		return
	}

	if nas.APIUsername == "" || nas.APIPassword == "" {
		log.Printf("BW syncToNAS: NAS %s has no API credentials", nas.Name)
		return
	}

	// Auto-generate queue name if empty
	if customer.QueueName == "" {
		customer.QueueName = fmt.Sprintf("BW-%s", customer.Name)
		database.DB.Model(customer).Update("queue_name", customer.QueueName)
	}

	if customer.IPAddress == "" {
		log.Printf("BW syncToNAS: Customer %s has no IP address, skipping queue", customer.Name)
		return
	}

	if customer.DownloadSpeed == 0 && customer.UploadSpeed == 0 {
		log.Printf("BW syncToNAS: Customer %s has no speed set, skipping queue", customer.Name)
		return
	}

	log.Printf("BW syncToNAS: Creating queue %s for %s (IP=%s, DL=%dk, UL=%dk) on NAS %s:%d",
		customer.QueueName, customer.Name, customer.IPAddress, customer.DownloadSpeed, customer.UploadSpeed, nas.IPAddress, nas.APIPort)

	client := mikrotik.NewClient(fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort), nas.APIUsername, nas.APIPassword)

	burst := mikrotik.BWBurstConfig{
		Enabled:     customer.BurstEnabled,
		BurstDl:     customer.BurstDownload,
		BurstUl:     customer.BurstUpload,
		ThresholdDl: customer.BurstThresholdDl,
		ThresholdUl: customer.BurstThresholdUl,
		BurstTime:   customer.BurstTime,
	}

	// Try update first, create if not found
	err := client.UpdateBWSimpleQueue(customer.QueueName, customer.DownloadSpeed, customer.UploadSpeed, burst)
	if err != nil {
		log.Printf("BW syncToNAS: Queue %s not found, creating new: %v", customer.QueueName, err)
		// Queue doesn't exist, create it
		if err := client.CreateBWSimpleQueue(customer.QueueName, customer.IPAddress, customer.DownloadSpeed, customer.UploadSpeed, burst); err != nil {
			log.Printf("BW syncToNAS: Failed to create queue for %s: %v", customer.Name, err)
		} else {
			log.Printf("BW syncToNAS: Created queue %s for %s", customer.QueueName, customer.Name)
		}
	} else {
		log.Printf("BW syncToNAS: Updated queue %s for %s", customer.QueueName, customer.Name)
	}
}

// removeFromNAS removes MikroTik queue for the customer
func (h *BandwidthCustomerHandler) removeFromNAS(customer *models.BandwidthCustomer) {
	if customer.NasID == nil {
		return
	}

	var nas models.Nas
	if err := database.DB.First(&nas, *customer.NasID).Error; err != nil {
		return
	}

	log.Printf("BW removeFromNAS: Removing queue %s from NAS %s:%d", customer.QueueName, nas.IPAddress, nas.APIPort)
	client := mikrotik.NewClient(fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort), nas.APIUsername, nas.APIPassword)
	if err := client.DeleteBWSimpleQueue(customer.QueueName); err != nil {
		log.Printf("BW removeFromNAS: Failed to delete queue %s: %v", customer.QueueName, err)
	}
}

// setupNetworkOnNAS creates VLAN, adds IP address, and adds route for public IPs on MikroTik
func (h *BandwidthCustomerHandler) setupNetworkOnNAS(customer *models.BandwidthCustomer) {
	if customer.NasID == nil {
		return
	}

	var nas models.Nas
	if err := database.DB.First(&nas, *customer.NasID).Error; err != nil {
		log.Printf("BW setupNetwork: NAS %d not found: %v", *customer.NasID, err)
		return
	}

	if nas.APIUsername == "" || nas.APIPassword == "" {
		log.Printf("BW setupNetwork: NAS %s has no API credentials, skipping", nas.Name)
		return
	}

	log.Printf("BW setupNetwork: Connecting to NAS %s (%s:%d) for customer %s", nas.Name, nas.IPAddress, nas.APIPort, customer.Name)
	client := mikrotik.NewClient(fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort), nas.APIUsername, nas.APIPassword)
	if err := client.Connect(); err != nil {
		log.Printf("BW setupNetwork: Failed to connect to NAS %s: %v", nas.Name, err)
		return
	}
	defer client.Close()

	iface := customer.Interface
	comment := fmt.Sprintf("BW:%s", customer.Name)
	log.Printf("BW setupNetwork: Customer=%s Interface=%s VlanID=%d Gateway=%s PublicIP=%s PrivateIP=%s", customer.Name, iface, customer.VlanID, customer.Gateway, customer.PublicIP, customer.IPAddress)

	// 1. Create VLAN if VLAN ID is set
	if customer.VlanID > 0 && iface != "" {
		vlanName := fmt.Sprintf("vlan%d", customer.VlanID)
		if err := client.CreateVLAN(vlanName, customer.VlanID, iface, comment); err != nil {
			log.Printf("BW setupNetwork: Failed to create VLAN %d on %s: %v", customer.VlanID, iface, err)
		} else {
			log.Printf("BW setupNetwork: Created VLAN %d on %s for %s", customer.VlanID, iface, customer.Name)
		}
		// Use VLAN interface for IP assignment
		iface = vlanName
	}

	// 2. Add gateway IP to the interface (customer-facing link)
	if customer.Gateway != "" && iface != "" {
		mask := customer.SubnetMask
		if mask == "" {
			mask = "255.255.255.0"
		}
		// Convert subnet mask to CIDR prefix
		prefix := subnetToCIDR(mask)
		addr := fmt.Sprintf("%s/%d", customer.Gateway, prefix)
		if err := client.AddIPAddress(addr, iface, comment); err != nil {
			log.Printf("BW setupNetwork: Failed to add IP %s to %s: %v", addr, iface, err)
		} else {
			log.Printf("BW setupNetwork: Added IP %s to %s for %s", addr, iface, customer.Name)
		}
	}

	// 3. Add route for public IPs → customer private IP
	if customer.PublicIP != "" && customer.IPAddress != "" {
		publicIPs := strings.Split(customer.PublicIP, ",")
		prefix := customer.PublicSubnet
		if prefix == "" {
			prefix = "/32"
		}
		if !strings.HasPrefix(prefix, "/") {
			prefix = "/" + prefix
		}

		if prefix != "/32" && len(publicIPs) > 1 {
			// Subnet route: calculate network address and create ONE route for the block
			firstIP := strings.TrimSpace(publicIPs[0])
			networkAddr := calcNetworkAddress(firstIP, prefix)
			dst := networkAddr + prefix
			if err := client.AddRoute(dst, customer.IPAddress, comment); err != nil {
				log.Printf("BW setupNetwork: Failed to add route %s -> %s: %v", dst, customer.IPAddress, err)
			} else {
				log.Printf("BW setupNetwork: Added subnet route %s -> %s for %s", dst, customer.IPAddress, customer.Name)
			}
		} else {
			// Single IP — /32 route
			for _, pip := range publicIPs {
				pip = strings.TrimSpace(pip)
				if pip == "" {
					continue
				}
				dst := pip + "/32"
				if err := client.AddRoute(dst, customer.IPAddress, comment); err != nil {
					log.Printf("BW setupNetwork: Failed to add route %s -> %s: %v", dst, customer.IPAddress, err)
				} else {
					log.Printf("BW setupNetwork: Added route %s -> %s for %s", dst, customer.IPAddress, customer.Name)
				}
			}
		}
	}
}

// cleanupNASConfig removes all MikroTik config for a bandwidth customer (routes, IP, VLAN)
func (h *BandwidthCustomerHandler) cleanupNASConfig(customer *models.BandwidthCustomer) {
	if customer.NasID == nil {
		return
	}

	var nas models.Nas
	if err := database.DB.First(&nas, *customer.NasID).Error; err != nil {
		log.Printf("BW cleanup: NAS %d not found: %v", *customer.NasID, err)
		return
	}

	if nas.APIUsername == "" || nas.APIPassword == "" {
		return
	}

	client := mikrotik.NewClient(fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort), nas.APIUsername, nas.APIPassword)
	if err := client.Connect(); err != nil {
		log.Printf("BW cleanup: Failed to connect to NAS %s: %v", nas.Name, err)
		return
	}
	defer client.Close()

	comment := fmt.Sprintf("BW:%s", customer.Name)
	log.Printf("BW cleanup: Removing all config for %s (comment=%s)", customer.Name, comment)

	// 1. Remove routes by comment
	if err := client.RemoveRoutesByComment(comment); err != nil {
		log.Printf("BW cleanup: Failed to remove routes: %v", err)
	} else {
		log.Printf("BW cleanup: Removed routes for %s", customer.Name)
	}

	// 2. Remove IP addresses by comment
	if err := client.RemoveIPAddressByComment(comment); err != nil {
		log.Printf("BW cleanup: Failed to remove IP addresses: %v", err)
	} else {
		log.Printf("BW cleanup: Removed IP addresses for %s", customer.Name)
	}

	// 3. Remove VLAN by comment
	if err := client.RemoveVLANByComment(comment); err != nil {
		log.Printf("BW cleanup: Failed to remove VLAN: %v", err)
	} else {
		log.Printf("BW cleanup: Removed VLAN for %s", customer.Name)
	}
}

// calcNetworkAddress calculates the network address from an IP and CIDR prefix
func calcNetworkAddress(ip string, prefix string) string {
	prefixLen := 24
	fmt.Sscanf(prefix, "/%d", &prefixLen)

	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return ip
	}

	var octets [4]uint32
	for i, p := range parts {
		val, _ := strconv.ParseUint(p, 10, 32)
		octets[i] = uint32(val)
	}

	ipNum := (octets[0] << 24) | (octets[1] << 16) | (octets[2] << 8) | octets[3]
	mask := uint32(0xFFFFFFFF) << (32 - prefixLen)
	network := ipNum & mask

	return fmt.Sprintf("%d.%d.%d.%d", (network>>24)&0xFF, (network>>16)&0xFF, (network>>8)&0xFF, network&0xFF)
}

// subnetToCIDR converts subnet mask to CIDR prefix length
func subnetToCIDR(mask string) int {
	switch mask {
	case "255.255.255.252", "/30":
		return 30
	case "255.255.255.248", "/29":
		return 29
	case "255.255.255.240", "/28":
		return 28
	case "255.255.255.224", "/27":
		return 27
	case "255.255.255.192", "/26":
		return 26
	case "255.255.255.128", "/25":
		return 25
	case "255.255.255.0", "/24":
		return 24
	default:
		return 24
	}
}

// assignPublicIPs marks IPs as assigned in public_ip_assignments for a bandwidth customer
func (h *BandwidthCustomerHandler) assignPublicIPs(customer *models.BandwidthCustomer) {
	if customer.PublicIP == "" || customer.IPBlockID == nil {
		return
	}

	poolID := *customer.IPBlockID
	publicIPs := strings.Split(customer.PublicIP, ",")
	now := time.Now()

	for _, ip := range publicIPs {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		assignment := models.PublicIPAssignment{
			PoolID:              poolID,
			BandwidthCustomerID: &customer.ID,
			IPAddress:           ip,
			IPVersion:           4,
			Status:              models.PublicIPStatusActive,
			AssignedAt:          now,
			Notes:               fmt.Sprintf("BW Customer: %s", customer.Name),
		}
		if err := database.DB.Create(&assignment).Error; err != nil {
			log.Printf("BW assignPublicIPs: Failed to assign IP %s: %v", ip, err)
		}
	}

	// Update used_ips count on pool
	var usedCount int64
	database.DB.Model(&models.PublicIPAssignment{}).
		Where("pool_id = ? AND status = ? AND deleted_at IS NULL", poolID, models.PublicIPStatusActive).
		Count(&usedCount)
	database.DB.Model(&models.PublicIPPool{}).Where("id = ?", poolID).Update("used_ips", usedCount)

	log.Printf("BW assignPublicIPs: Assigned %d IPs from pool %d for %s", len(publicIPs), poolID, customer.Name)
}

// releasePublicIPs releases all public IP assignments for a bandwidth customer
func (h *BandwidthCustomerHandler) releasePublicIPs(customer *models.BandwidthCustomer) {
	now := time.Now()
	result := database.DB.Model(&models.PublicIPAssignment{}).
		Where("bandwidth_customer_id = ? AND status = ? AND deleted_at IS NULL", customer.ID, models.PublicIPStatusActive).
		Updates(map[string]interface{}{
			"status":      models.PublicIPStatusReleased,
			"released_at": now,
		})

	if result.RowsAffected > 0 {
		log.Printf("BW releasePublicIPs: Released %d IPs for %s", result.RowsAffected, customer.Name)
		// Update used_ips count on pool
		if customer.IPBlockID != nil {
			var usedCount int64
			database.DB.Model(&models.PublicIPAssignment{}).
				Where("pool_id = ? AND status = ? AND deleted_at IS NULL", *customer.IPBlockID, models.PublicIPStatusActive).
				Count(&usedCount)
			database.DB.Model(&models.PublicIPPool{}).Where("id = ?", *customer.IPBlockID).Update("used_ips", usedCount)
		}
	}
}

// GetHourlyUsage returns hourly bandwidth data for historical charts
func (h *BandwidthCustomerHandler) GetHourlyUsage(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	days, _ := strconv.Atoi(c.Query("days", "7"))
	if days < 1 || days > 90 {
		days = 7
	}

	var data []models.BwHourlyUsage
	database.DB.Where("customer_id = ? AND hour >= NOW() - ? * INTERVAL '1 day'", id, days).
		Order("hour ASC").Find(&data)

	return c.JSON(fiber.Map{"success": true, "data": data})
}

// GetSessions returns session history for timeline visualization
func (h *BandwidthCustomerHandler) GetSessions(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	days, _ := strconv.Atoi(c.Query("days", "30"))
	if days < 1 || days > 90 {
		days = 30
	}

	var sessions []models.BwSession
	database.DB.Where("customer_id = ? AND started_at >= NOW() - ? * INTERVAL '1 day'", id, days).
		Order("started_at DESC").Find(&sessions)

	return c.JSON(fiber.Map{"success": true, "data": sessions})
}

// GetHeatmap returns day-of-week x hour-of-day aggregated bandwidth
func (h *BandwidthCustomerHandler) GetHeatmap(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	days, _ := strconv.Atoi(c.Query("days", "30"))
	if days < 1 || days > 90 {
		days = 30
	}

	type HeatmapCell struct {
		Day           int   `json:"day"`
		Hour          int   `json:"hour"`
		DownloadBytes int64 `json:"download_bytes"`
		UploadBytes   int64 `json:"upload_bytes"`
	}

	var cells []HeatmapCell
	database.DB.Raw(`
		SELECT
			EXTRACT(dow FROM hour)::int AS day,
			EXTRACT(hour FROM hour)::int AS hour,
			SUM(download_bytes) AS download_bytes,
			SUM(upload_bytes) AS upload_bytes
		FROM bw_hourly_usage
		WHERE customer_id = ? AND hour >= NOW() - ? * INTERVAL '1 day'
		GROUP BY EXTRACT(dow FROM hour), EXTRACT(hour FROM hour)
		ORDER BY day, hour
	`, id, days).Scan(&cells)

	return c.JSON(fiber.Map{"success": true, "data": cells})
}
