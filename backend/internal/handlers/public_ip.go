package handlers

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"math/big"
	"net"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

type PublicIPHandler struct{}

func NewPublicIPHandler() *PublicIPHandler {
	return &PublicIPHandler{}
}

// ---- Pool CRUD ----

type CreatePoolRequest struct {
	Name         string  `json:"name"`
	CIDR         string  `json:"cidr"`
	Gateway      string  `json:"gateway"`
	MonthlyPrice float64 `json:"monthly_price"`
	Description  string  `json:"description"`
	IsActive     bool    `json:"is_active"`
}

func (h *PublicIPHandler) ListPools(c *fiber.Ctx) error {
	var pools []models.PublicIPPool
	query := database.DB.Where("deleted_at IS NULL")

	search := c.Query("search", "")
	if search != "" {
		pattern := "%" + search + "%"
		query = query.Where("name ILIKE ? OR cidr ILIKE ?", pattern, pattern)
	}

	if err := query.Order("created_at DESC").Find(&pools).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch pools",
		})
	}

	// Recalculate used_ips for each pool (active + reserved)
	for i := range pools {
		var count int64
		database.DB.Model(&models.PublicIPAssignment{}).
			Where("pool_id = ? AND status IN (?, ?) AND deleted_at IS NULL", pools[i].ID, models.PublicIPStatusActive, models.PublicIPStatusReserved).
			Count(&count)
		pools[i].UsedIPs = int(count)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    pools,
	})
}

func (h *PublicIPHandler) CreatePool(c *fiber.Ctx) error {
	var req CreatePoolRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if req.Name == "" || req.CIDR == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Name and CIDR are required",
		})
	}

	// Parse and validate CIDR
	_, ipNet, err := net.ParseCIDR(req.CIDR)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Invalid CIDR: %v", err),
		})
	}

	// Detect IP version
	ipVersion := 4
	if ipNet.IP.To4() == nil {
		ipVersion = 6
	}

	// Calculate total usable IPs from prefix size
	ones, bits := ipNet.Mask.Size()
	size := 1 << uint(bits-ones)
	totalIPs := size - 2 // exclude network + broadcast
	if totalIPs < 1 {
		totalIPs = 1
	}

	pool := models.PublicIPPool{
		Name:         req.Name,
		CIDR:         req.CIDR, // Store as entered — user's starting IP matters
		IPVersion:    ipVersion,
		Gateway:      req.Gateway,
		MonthlyPrice: req.MonthlyPrice,
		Description:  req.Description,
		IsActive:     req.IsActive,
		TotalIPs:     totalIPs,
		UsedIPs:      0,
	}

	if err := database.DB.Create(&pool).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create pool",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    pool,
		"message": "Pool created successfully",
	})
}

func (h *PublicIPHandler) UpdatePool(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid pool ID",
		})
	}

	var pool models.PublicIPPool
	if err := database.DB.First(&pool, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Pool not found",
		})
	}

	var req CreatePoolRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	updates := map[string]interface{}{
		"name":          req.Name,
		"gateway":       req.Gateway,
		"monthly_price": req.MonthlyPrice,
		"description":   req.Description,
		"is_active":     req.IsActive,
	}

	if err := database.DB.Model(&pool).Updates(updates).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update pool",
		})
	}

	database.DB.First(&pool, id)
	return c.JSON(fiber.Map{
		"success": true,
		"data":    pool,
		"message": "Pool updated successfully",
	})
}

func (h *PublicIPHandler) DeletePool(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid pool ID",
		})
	}

	// Check for active assignments
	var activeCount int64
	database.DB.Model(&models.PublicIPAssignment{}).
		Where("pool_id = ? AND status = ? AND deleted_at IS NULL", id, models.PublicIPStatusActive).
		Count(&activeCount)

	if activeCount > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Cannot delete pool with %d active assignments", activeCount),
		})
	}

	if err := database.DB.Delete(&models.PublicIPPool{}, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to delete pool",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Pool deleted successfully",
	})
}

// ---- Assignments ----

func (h *PublicIPHandler) ListAssignments(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "25"))
	if page < 1 {
		page = 1
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	query := database.DB.Model(&models.PublicIPAssignment{}).Where("public_ip_assignments.deleted_at IS NULL")

	// Filters
	if poolID := c.Query("pool_id"); poolID != "" {
		query = query.Where("pool_id = ?", poolID)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if search := c.Query("search"); search != "" {
		pattern := "%" + search + "%"
		query = query.Where("ip_address ILIKE ?", pattern)
	}

	var total int64
	query.Count(&total)

	var assignments []models.PublicIPAssignment
	if err := query.
		Preload("Pool").
		Preload("Subscriber").
		Preload("BandwidthCustomer").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&assignments).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch assignments",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    assignments,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

type AssignIPRequest struct {
	PoolID       uint   `json:"pool_id"`
	SubscriberID uint   `json:"subscriber_id"`
	IPAddress    string `json:"ip_address"` // optional: specific IP
	Notes        string `json:"notes"`
}

func (h *PublicIPHandler) AssignIP(c *fiber.Ctx) error {
	var req AssignIPRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if req.PoolID == 0 || req.SubscriberID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Pool ID and Subscriber ID are required",
		})
	}

	// Check subscriber exists
	var subscriber models.Subscriber
	if err := database.DB.First(&subscriber, req.SubscriberID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber not found",
		})
	}

	// Check subscriber doesn't already have an active public IP
	var existingCount int64
	database.DB.Model(&models.PublicIPAssignment{}).
		Where("subscriber_id = ? AND status = ? AND deleted_at IS NULL", req.SubscriberID, models.PublicIPStatusActive).
		Count(&existingCount)
	if existingCount > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber already has an active public IP assignment",
		})
	}

	// Get pool
	var pool models.PublicIPPool
	if err := database.DB.First(&pool, req.PoolID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Pool not found",
		})
	}

	if !pool.IsActive {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Pool is not active",
		})
	}

	// Find available IP
	var ipAddress string
	if req.IPAddress != "" {
		// Specific IP requested — validate it's in the CIDR and not assigned
		if !isIPInCIDR(req.IPAddress, pool.CIDR) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "IP address is not in the pool's CIDR range",
			})
		}
		var usedCount int64
		database.DB.Model(&models.PublicIPAssignment{}).
			Where("ip_address = ? AND status = ? AND deleted_at IS NULL", req.IPAddress, models.PublicIPStatusActive).
			Count(&usedCount)
		if usedCount > 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "IP address is already assigned",
			})
		}
		ipAddress = req.IPAddress
	} else {
		// Auto-allocate next available IP
		var err error
		ipAddress, err = findNextAvailablePublicIP(pool)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": err.Error(),
			})
		}
	}

	now := time.Now()
	assignment := models.PublicIPAssignment{
		PoolID:       pool.ID,
		SubscriberID: &req.SubscriberID,
		IPAddress:    ipAddress,
		IPVersion:    pool.IPVersion,
		Status:       models.PublicIPStatusActive,
		AssignedAt:   now,
		MonthlyPrice: pool.MonthlyPrice,
		Notes:        req.Notes,
	}

	if err := database.DB.Create(&assignment).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create assignment",
		})
	}

	// Update pool used count
	database.DB.Model(&pool).Update("used_ips", database.DB.Raw("used_ips + 1"))

	// Write to radreply for RADIUS delivery
	writePublicIPToRadreply(subscriber.Username, ipAddress, pool.IPVersion)

	// Send CoA disconnect to force reconnect with new IP
	go disconnectSubscriberByCoA(&subscriber)

	// Reload with relations
	database.DB.Preload("Pool").Preload("Subscriber").First(&assignment, assignment.ID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    assignment,
		"message": fmt.Sprintf("Public IP %s assigned successfully", ipAddress),
	})
}

func (h *PublicIPHandler) ReleaseIP(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid assignment ID",
		})
	}

	var assignment models.PublicIPAssignment
	if err := database.DB.Preload("Subscriber").First(&assignment, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Assignment not found",
		})
	}

	if assignment.Status != models.PublicIPStatusActive && assignment.Status != models.PublicIPStatusReserved {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Assignment is not active or reserved",
		})
	}

	// Save original status before updating
	wasActive := assignment.Status == models.PublicIPStatusActive

	now := time.Now()
	database.DB.Model(&assignment).Updates(map[string]interface{}{
		"status":      models.PublicIPStatusReleased,
		"released_at": now,
	})

	// Decrement pool used count
	database.DB.Model(&models.PublicIPPool{}).Where("id = ?", assignment.PoolID).
		Update("used_ips", database.DB.Raw("GREATEST(used_ips - 1, 0)"))

	// Remove from radreply and disconnect (only if it was active with a subscriber)
	if wasActive && assignment.Subscriber != nil {
		removePublicIPFromRadreply(assignment.Subscriber.Username, assignment.IPVersion)
		go disconnectSubscriberByCoA(assignment.Subscriber)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Public IP %s released successfully", assignment.IPAddress),
	})
}

// ---- Reserve IP ----

type ReserveIPRequest struct {
	PoolID    uint   `json:"pool_id"`
	IPAddress string `json:"ip_address"`
	Notes     string `json:"notes"`
}

func (h *PublicIPHandler) ReserveIP(c *fiber.Ctx) error {
	var req ReserveIPRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if req.PoolID == 0 || req.IPAddress == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Pool ID and IP Address are required",
		})
	}

	// Get pool
	var pool models.PublicIPPool
	if err := database.DB.First(&pool, req.PoolID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Pool not found",
		})
	}

	// Validate IP is in CIDR
	if !isIPInCIDR(req.IPAddress, pool.CIDR) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "IP address is not in the pool's CIDR range",
		})
	}

	// Check not already assigned or reserved
	var existingCount int64
	database.DB.Model(&models.PublicIPAssignment{}).
		Where("ip_address = ? AND status IN (?, ?) AND deleted_at IS NULL", req.IPAddress, models.PublicIPStatusActive, models.PublicIPStatusReserved).
		Count(&existingCount)
	if existingCount > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "IP address is already assigned or reserved",
		})
	}

	now := time.Now()
	assignment := models.PublicIPAssignment{
		PoolID:    pool.ID,
		IPAddress: req.IPAddress,
		IPVersion: pool.IPVersion,
		Status:    models.PublicIPStatusReserved,
		AssignedAt: now,
		Notes:     req.Notes,
	}

	if err := database.DB.Create(&assignment).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to reserve IP",
		})
	}

	// Update pool used count
	database.DB.Model(&pool).Update("used_ips", database.DB.Raw("used_ips + 1"))

	database.DB.Preload("Pool").First(&assignment, assignment.ID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    assignment,
		"message": fmt.Sprintf("IP %s reserved successfully", req.IPAddress),
	})
}

// ---- Admin: Get subscriber's public IP ----

func (h *PublicIPHandler) GetSubscriberPublicIP(c *fiber.Ctx) error {
	subID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid subscriber ID",
		})
	}

	var assignment models.PublicIPAssignment
	if err := database.DB.Preload("Pool").
		Where("subscriber_id = ? AND status = ? AND deleted_at IS NULL", subID, models.PublicIPStatusActive).
		First(&assignment).Error; err != nil {
		// No assignment found — return success with null
		return c.JSON(fiber.Map{
			"success":    true,
			"assignment": nil,
		})
	}

	return c.JSON(fiber.Map{
		"success":    true,
		"assignment": assignment,
	})
}

// ---- Customer Portal Endpoints ----

func (h *PublicIPHandler) GetCustomerPublicIP(c *fiber.Ctx) error {
	sub := getCustomerSubscriber(c)
	if sub == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized",
		})
	}

	// Get current assignment
	var assignment models.PublicIPAssignment
	hasAssignment := false
	if err := database.DB.Preload("Pool").
		Where("subscriber_id = ? AND status = ? AND deleted_at IS NULL", sub.ID, models.PublicIPStatusActive).
		First(&assignment).Error; err == nil {
		hasAssignment = true
	}

	// Get available pools
	var pools []models.PublicIPPool
	database.DB.Where("is_active = true AND deleted_at IS NULL").Find(&pools)

	// Calculate available IPs for each pool
	type PoolInfo struct {
		models.PublicIPPool
		AvailableIPs int `json:"available_ips"`
	}
	var poolInfos []PoolInfo
	for _, p := range pools {
		info := PoolInfo{
			PublicIPPool: p,
			AvailableIPs: p.TotalIPs - p.UsedIPs,
		}
		poolInfos = append(poolInfos, info)
	}

	result := fiber.Map{
		"success":        true,
		"has_assignment": hasAssignment,
		"pools":          poolInfos,
	}
	if hasAssignment {
		result["assignment"] = assignment
	}

	return c.JSON(result)
}

type BuyPublicIPRequest struct {
	PoolID uint `json:"pool_id"`
}

func (h *PublicIPHandler) BuyPublicIP(c *fiber.Ctx) error {
	sub := getCustomerSubscriber(c)
	if sub == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized",
		})
	}

	var req BuyPublicIPRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Check subscriber doesn't already have one
	var existingCount int64
	database.DB.Model(&models.PublicIPAssignment{}).
		Where("subscriber_id = ? AND status = ? AND deleted_at IS NULL", sub.ID, models.PublicIPStatusActive).
		Count(&existingCount)
	if existingCount > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "You already have a public IP assigned",
		})
	}

	// Get pool
	var pool models.PublicIPPool
	if err := database.DB.First(&pool, req.PoolID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Pool not found",
		})
	}

	if !pool.IsActive {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Pool is not available",
		})
	}

	// Check subscriber wallet balance if price > 0
	if pool.MonthlyPrice > 0 {
		if sub.Balance < pool.MonthlyPrice {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": fmt.Sprintf("Insufficient wallet balance. You need $%.2f but have $%.2f", pool.MonthlyPrice, sub.Balance),
			})
		}

		balanceBefore := sub.Balance
		// Deduct from subscriber wallet atomically
		database.DB.Model(sub).Update("balance", database.DB.Raw("balance - ?", pool.MonthlyPrice))

		// Create subscriber purchase transaction
		database.DB.Create(&models.Transaction{
			Type:          models.TransactionTypeSubscriberPurchase,
			Amount:        -pool.MonthlyPrice,
			BalanceBefore: balanceBefore,
			BalanceAfter:  balanceBefore - pool.MonthlyPrice,
			Description:   fmt.Sprintf("Public IP purchase (pool: %s)", pool.Name),
			ResellerID:    sub.ResellerID,
			SubscriberID:  &sub.ID,
		})
	}

	// Allocate IP
	ipAddress, err := findNextAvailablePublicIP(pool)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	now := time.Now()
	nextBilling := now.Add(30 * 24 * time.Hour)
	assignment := models.PublicIPAssignment{
		PoolID:        pool.ID,
		SubscriberID:  &sub.ID,
		IPAddress:     ipAddress,
		IPVersion:     pool.IPVersion,
		Status:        models.PublicIPStatusActive,
		AssignedAt:    now,
		LastBilledAt:  &now,
		NextBillingAt: &nextBilling,
		MonthlyPrice:  pool.MonthlyPrice,
	}

	if err := database.DB.Create(&assignment).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to assign IP",
		})
	}

	// Update pool used count
	database.DB.Model(&pool).Update("used_ips", database.DB.Raw("used_ips + 1"))

	// Write to radreply
	writePublicIPToRadreply(sub.Username, ipAddress, pool.IPVersion)

	// CoA disconnect to pick up new IP
	go disconnectSubscriberByCoA(sub)

	database.DB.Preload("Pool").First(&assignment, assignment.ID)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    assignment,
		"message": fmt.Sprintf("Public IP %s purchased successfully", ipAddress),
	})
}

func (h *PublicIPHandler) ReleaseCustomerIP(c *fiber.Ctx) error {
	sub := getCustomerSubscriber(c)
	if sub == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized",
		})
	}

	var assignment models.PublicIPAssignment
	if err := database.DB.
		Where("subscriber_id = ? AND status = ? AND deleted_at IS NULL", sub.ID, models.PublicIPStatusActive).
		First(&assignment).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "No active public IP assignment found",
		})
	}

	now := time.Now()
	database.DB.Model(&assignment).Updates(map[string]interface{}{
		"status":      models.PublicIPStatusReleased,
		"released_at": now,
	})

	// Decrement pool used count
	database.DB.Model(&models.PublicIPPool{}).Where("id = ?", assignment.PoolID).
		Update("used_ips", database.DB.Raw("GREATEST(used_ips - 1, 0)"))

	// Remove from radreply
	removePublicIPFromRadreply(sub.Username, assignment.IPVersion)

	// CoA disconnect
	go disconnectSubscriberByCoA(sub)

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Public IP %s released", assignment.IPAddress),
	})
}

// GetAvailableIPs returns all available (unassigned) IPs in a pool
func (h *PublicIPHandler) GetAvailableIPs(c *fiber.Ctx) error {
	poolID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid pool ID"})
	}

	var pool models.PublicIPPool
	if err := database.DB.First(&pool, poolID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Pool not found"})
	}

	// Parse the user-entered IP (before the /) as the START of the range
	userIP, ipNet, err := net.ParseCIDR(pool.CIDR)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Invalid CIDR in pool"})
	}

	// Get all assigned/reserved IPs
	var assignedIPs []string
	database.DB.Model(&models.PublicIPAssignment{}).
		Where("pool_id = ? AND status IN (?, ?) AND deleted_at IS NULL", pool.ID, models.PublicIPStatusActive, models.PublicIPStatusReserved).
		Pluck("ip_address", &assignedIPs)

	assignedSet := make(map[string]bool, len(assignedIPs))
	for _, ip := range assignedIPs {
		assignedSet[ip] = true
	}

	type AvailableIP struct {
		IPAddress string `json:"ip_address"`
	}

	var available []AvailableIP

	if pool.IPVersion == 4 {
		ip4 := userIP.To4()
		if ip4 != nil {
			// Use the user's entered IP as the start (e.g. .8), not the normalized network (.0)
			start := binary.BigEndian.Uint32(ip4)
			ones, bits := ipNet.Mask.Size()
			size := uint32(1) << uint(bits-ones)
			end := start + size - 1 // last IP = broadcast

			// Generate IPs from start+1 (skip network) to end-1 (skip broadcast)
			for addr := start + 1; addr < end; addr++ {
				ipBytes := make(net.IP, 4)
				binary.BigEndian.PutUint32(ipBytes, addr)
				ipStr := ipBytes.String()
				if ipStr == pool.Gateway || assignedSet[ipStr] {
					continue
				}
				available = append(available, AvailableIP{IPAddress: ipStr})
			}
		}
	}

	return c.JSON(fiber.Map{"success": true, "data": available})
}

// ---- Helper functions ----

func calculateUsableIPs(ipNet *net.IPNet, ipVersion int) int {
	ones, bits := ipNet.Mask.Size()
	if ipVersion == 4 {
		total := int(math.Pow(2, float64(bits-ones)))
		if total <= 2 {
			return 0
		}
		return total - 2 // Subtract network and broadcast
	}
	// IPv6: just report the prefix size (not enumerating)
	hostBits := bits - ones
	if hostBits > 20 {
		return 1048576 // Cap at 1M for display
	}
	return int(math.Pow(2, float64(hostBits)))
}

func findNextAvailablePublicIP(pool models.PublicIPPool) (string, error) {
	userIP, ipNet, err := net.ParseCIDR(pool.CIDR)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR: %v", err)
	}

	// Get all assigned + reserved IPs in this pool
	var assignedIPs []string
	database.DB.Model(&models.PublicIPAssignment{}).
		Where("pool_id = ? AND status IN (?, ?) AND deleted_at IS NULL", pool.ID, models.PublicIPStatusActive, models.PublicIPStatusReserved).
		Pluck("ip_address", &assignedIPs)

	assignedSet := make(map[string]bool, len(assignedIPs))
	for _, ip := range assignedIPs {
		assignedSet[ip] = true
	}

	if pool.IPVersion == 4 {
		return findNextIPv4FromUserIP(userIP, ipNet, assignedSet, pool.Gateway)
	}
	return findNextIPv6(ipNet, assignedSet)
}

func findNextIPv4(ipNet *net.IPNet, assigned map[string]bool, gateway string) (string, error) {
	ip := ipNet.IP.To4()
	if ip == nil {
		return "", fmt.Errorf("not an IPv4 network")
	}

	// Convert to uint32 for iteration
	start := binary.BigEndian.Uint32(ip)
	ones, bits := ipNet.Mask.Size()
	size := uint32(1) << uint(bits-ones)
	end := start + size - 1 // broadcast

	// Skip network address (start) and broadcast (end)
	for addr := start + 1; addr < end; addr++ {
		ipBytes := make(net.IP, 4)
		binary.BigEndian.PutUint32(ipBytes, addr)
		ipStr := ipBytes.String()

		// Skip gateway
		if ipStr == gateway {
			continue
		}

		// Skip if already assigned
		if assigned[ipStr] {
			continue
		}

		// Also check radreply to avoid conflicts
		var radreplyCount int64
		database.DB.Table("radreply").
			Where("attribute = ? AND value = ?", "Framed-IP-Address", ipStr).
			Count(&radreplyCount)
		if radreplyCount > 0 {
			continue
		}

		return ipStr, nil
	}

	return "", fmt.Errorf("no available IPs in pool")
}

// findNextIPv4FromUserIP uses the user-entered IP (before CIDR normalization) as the start
// of the range. For example, if CIDR is "109.110.185.8/27", userIP is .8 and range is .9-.38.
func findNextIPv4FromUserIP(userIP net.IP, ipNet *net.IPNet, assigned map[string]bool, gateway string) (string, error) {
	ip4 := userIP.To4()
	if ip4 == nil {
		return "", fmt.Errorf("not an IPv4 address")
	}

	start := binary.BigEndian.Uint32(ip4)
	ones, bits := ipNet.Mask.Size()
	size := uint32(1) << uint(bits-ones)
	end := start + size - 1 // last IP in range

	// Skip first IP (network/start) and last IP (broadcast)
	for addr := start + 1; addr < end; addr++ {
		ipBytes := make(net.IP, 4)
		binary.BigEndian.PutUint32(ipBytes, addr)
		ipStr := ipBytes.String()

		if ipStr == gateway {
			continue
		}
		if assigned[ipStr] {
			continue
		}

		var radreplyCount int64
		database.DB.Table("radreply").
			Where("attribute = ? AND value = ?", "Framed-IP-Address", ipStr).
			Count(&radreplyCount)
		if radreplyCount > 0 {
			continue
		}

		return ipStr, nil
	}

	return "", fmt.Errorf("no available IPs in pool")
}

func findNextIPv6(ipNet *net.IPNet, assigned map[string]bool) (string, error) {
	ip := ipNet.IP.To16()
	if ip == nil {
		return "", fmt.Errorf("not an IPv6 network")
	}

	// Try first 10000 addresses in the range
	ipInt := new(big.Int).SetBytes(ip)
	one := big.NewInt(1)

	for i := 0; i < 10000; i++ {
		ipInt.Add(ipInt, one)
		candidate := make(net.IP, 16)
		b := ipInt.Bytes()
		copy(candidate[16-len(b):], b)

		if !ipNet.Contains(candidate) {
			break
		}

		ipStr := candidate.String()
		if !assigned[ipStr] {
			return ipStr, nil
		}
	}

	return "", fmt.Errorf("no available IPs in pool")
}

func isIPInCIDR(ipStr, cidr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	return ipNet.Contains(ip)
}

func writePublicIPToRadreply(username, ipAddress string, ipVersion int) {
	attribute := "Framed-IP-Address"
	if ipVersion == 6 {
		attribute = "Framed-IPv6-Prefix"
	}

	// Delete existing entry first
	database.DB.Exec(
		"DELETE FROM radreply WHERE username = ? AND attribute = ?",
		username, attribute,
	)

	// Insert new entry
	database.DB.Exec(
		"INSERT INTO radreply (username, attribute, op, value) VALUES (?, ?, ?, ?)",
		username, attribute, ":=", ipAddress,
	)

	log.Printf("PublicIP: Wrote %s = %s for user %s", attribute, ipAddress, username)
}

func removePublicIPFromRadreply(username string, ipVersion int) {
	attribute := "Framed-IP-Address"
	if ipVersion == 6 {
		attribute = "Framed-IPv6-Prefix"
	}

	database.DB.Exec(
		"DELETE FROM radreply WHERE username = ? AND attribute = ?",
		username, attribute,
	)

	log.Printf("PublicIP: Removed %s for user %s", attribute, username)
}

// getCustomerSubscriber extracts the subscriber from customer JWT context
func getCustomerSubscriber(c *fiber.Ctx) *models.Subscriber {
	username, ok := c.Locals("customer_username").(string)
	if !ok || username == "" {
		return nil
	}
	var sub models.Subscriber
	if err := database.DB.Where("username = ?", username).First(&sub).Error; err != nil {
		return nil
	}
	return &sub
}

