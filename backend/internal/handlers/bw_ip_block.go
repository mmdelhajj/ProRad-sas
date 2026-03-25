package handlers

import (
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

type BwIPBlockHandler struct{}

func NewBwIPBlockHandler() *BwIPBlockHandler {
	return &BwIPBlockHandler{}
}

// ListBlocks returns all IP blocks with utilization stats
func (h *BwIPBlockHandler) ListBlocks(c *fiber.Ctx) error {
	var blocks []models.BwIPBlock
	if err := database.DB.Order("created_at DESC").Find(&blocks).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to fetch blocks"})
	}

	// Update used_ips count for each block
	for i := range blocks {
		var count int64
		database.DB.Model(&models.BwIPAllocation{}).Where("block_id = ? AND status = ?", blocks[i].ID, "assigned").Count(&count)
		blocks[i].UsedIPs = int(count)
	}

	return c.JSON(fiber.Map{"success": true, "data": blocks})
}

// CreateBlock creates a new IP block and auto-generates allocations
func (h *BwIPBlockHandler) CreateBlock(c *fiber.Ctx) error {
	var req struct {
		Name        string `json:"name"`
		CIDR        string `json:"cidr"`
		Gateway     string `json:"gateway"`
		Description string `json:"description"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request"})
	}

	if req.Name == "" || req.CIDR == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Name and CIDR are required"})
	}

	// Parse CIDR
	ip, ipNet, err := net.ParseCIDR(req.CIDR)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid CIDR: " + err.Error()})
	}

	// Only support IPv4
	if ip.To4() == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Only IPv4 CIDR blocks are supported"})
	}

	// Check overlap with existing blocks
	var existingBlocks []models.BwIPBlock
	database.DB.Find(&existingBlocks)
	for _, existing := range existingBlocks {
		_, existingNet, err := net.ParseCIDR(existing.CIDR)
		if err != nil {
			continue
		}
		if ipNet.Contains(existingNet.IP) || existingNet.Contains(ipNet.IP) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"success": false,
				"message": fmt.Sprintf("CIDR %s overlaps with existing block %s (%s)", req.CIDR, existing.Name, existing.CIDR),
			})
		}
	}

	// Calculate subnet mask from CIDR
	ones, bits := ipNet.Mask.Size()
	subnetMask := net.IP(ipNet.Mask).String()

	// Calculate usable IPs (total - network - broadcast)
	totalHosts := int(math.Pow(2, float64(bits-ones)))
	usableIPs := totalHosts - 2 // minus network and broadcast
	if usableIPs < 1 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "CIDR block too small"})
	}

	// Auto-detect gateway if not provided
	gateway := req.Gateway
	if gateway == "" {
		// Default: first usable IP (network + 1)
		networkIP := ipToUint32(ipNet.IP.To4())
		gateway = uint32ToIP(networkIP + 1).String()
	}

	// If gateway is in the block, subtract it from usable
	gwIP := net.ParseIP(gateway)
	gatewayInBlock := gwIP != nil && ipNet.Contains(gwIP)
	if gatewayInBlock {
		usableIPs-- // gateway is not assignable
	}

	block := models.BwIPBlock{
		Name:       req.Name,
		CIDR:       req.CIDR,
		Gateway:    gateway,
		SubnetMask: subnetMask,
		Description: req.Description,
		TotalIPs:   usableIPs,
		UsedIPs:    0,
		IsActive:   true,
	}

	if err := database.DB.Create(&block).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to create block: " + err.Error()})
	}

	// Generate IP allocations
	networkIP := ipToUint32(ipNet.IP.To4())
	broadcastIP := networkIP + uint32(totalHosts) - 1

	var allocations []models.BwIPAllocation
	for ipUint := networkIP + 1; ipUint < broadcastIP; ipUint++ {
		ipAddr := uint32ToIP(ipUint).String()
		status := "available"
		notes := ""

		if gatewayInBlock && ipAddr == gateway {
			status = "gateway"
			notes = "Gateway"
		}

		allocations = append(allocations, models.BwIPAllocation{
			BlockID:   block.ID,
			IPAddress: ipAddr,
			Status:    status,
			Notes:     notes,
		})
	}

	if len(allocations) > 0 {
		database.DB.CreateInBatches(allocations, 100)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    block,
		"message": fmt.Sprintf("Block created with %d usable IPs", usableIPs),
	})
}

// GetBlock returns a single block with all allocations
func (h *BwIPBlockHandler) GetBlock(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	var block models.BwIPBlock
	if err := database.DB.First(&block, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Block not found"})
	}

	var allocations []models.BwIPAllocation
	database.DB.Where("block_id = ?", id).Order("ip_address ASC").Find(&allocations)

	// Count used
	var usedCount int64
	database.DB.Model(&models.BwIPAllocation{}).Where("block_id = ? AND status = ?", id, "assigned").Count(&usedCount)
	block.UsedIPs = int(usedCount)

	return c.JSON(fiber.Map{
		"success":     true,
		"data":        block,
		"allocations": allocations,
	})
}

// DeleteBlock deletes a block only if no IPs are assigned
func (h *BwIPBlockHandler) DeleteBlock(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	var block models.BwIPBlock
	if err := database.DB.First(&block, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Block not found"})
	}

	// Check for assigned IPs
	var assignedCount int64
	database.DB.Model(&models.BwIPAllocation{}).Where("block_id = ? AND status = ?", id, "assigned").Count(&assignedCount)
	if assignedCount > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Cannot delete block with %d assigned IPs. Release all IPs first.", assignedCount),
		})
	}

	// Delete allocations first, then block
	database.DB.Where("block_id = ?", id).Delete(&models.BwIPAllocation{})
	database.DB.Delete(&block)

	return c.JSON(fiber.Map{"success": true, "message": "Block deleted"})
}

// GetAvailableIPs returns available IPs in a block
func (h *BwIPBlockHandler) GetAvailableIPs(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid ID"})
	}

	var allocations []models.BwIPAllocation
	database.DB.Where("block_id = ? AND status = ?", id, "available").Order("ip_address ASC").Find(&allocations)

	return c.JSON(fiber.Map{"success": true, "data": allocations})
}

// AssignIP assigns an IP from a block to a customer
func (h *BwIPBlockHandler) AssignIP(c *fiber.Ctx) error {
	blockID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid block ID"})
	}

	var req struct {
		CustomerID uint   `json:"customer_id"`
		IPAddress  string `json:"ip_address"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request"})
	}

	var alloc models.BwIPAllocation
	if err := database.DB.Where("block_id = ? AND ip_address = ? AND status = ?", blockID, req.IPAddress, "available").First(&alloc).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "IP not available"})
	}

	now := time.Now()
	alloc.Status = "assigned"
	alloc.CustomerID = &req.CustomerID
	alloc.AssignedAt = &now
	database.DB.Save(&alloc)

	// Update customer's IP block reference
	database.DB.Model(&models.BandwidthCustomer{}).Where("id = ?", req.CustomerID).Updates(map[string]interface{}{
		"ip_block_id":      blockID,
		"ip_allocation_id": alloc.ID,
		"public_ip":        req.IPAddress,
	})

	// Update block used count
	h.updateBlockUsedCount(uint(blockID))

	return c.JSON(fiber.Map{"success": true, "data": alloc, "message": "IP assigned"})
}

// ReleaseIP releases an assigned IP
func (h *BwIPBlockHandler) ReleaseIP(c *fiber.Ctx) error {
	allocID, err := strconv.Atoi(c.Params("allocId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid allocation ID"})
	}

	var alloc models.BwIPAllocation
	if err := database.DB.First(&alloc, allocID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Allocation not found"})
	}

	if alloc.Status != "assigned" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "IP is not assigned"})
	}

	// Clear customer reference
	if alloc.CustomerID != nil {
		database.DB.Model(&models.BandwidthCustomer{}).Where("id = ?", *alloc.CustomerID).Updates(map[string]interface{}{
			"ip_block_id":      nil,
			"ip_allocation_id": nil,
		})
	}

	alloc.Status = "available"
	alloc.CustomerID = nil
	alloc.AssignedAt = nil
	database.DB.Save(&alloc)

	h.updateBlockUsedCount(alloc.BlockID)

	return c.JSON(fiber.Map{"success": true, "message": "IP released"})
}

func (h *BwIPBlockHandler) updateBlockUsedCount(blockID uint) {
	var count int64
	database.DB.Model(&models.BwIPAllocation{}).Where("block_id = ? AND status = ?", blockID, "assigned").Count(&count)
	database.DB.Model(&models.BwIPBlock{}).Where("id = ?", blockID).Update("used_ips", count)
}

// Helper: convert net.IP to uint32
func ipToUint32(ip net.IP) uint32 {
	return binary.BigEndian.Uint32(ip.To4())
}

// Helper: convert uint32 to net.IP
func uint32ToIP(n uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, n)
	return ip
}
