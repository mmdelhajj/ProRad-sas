package handlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/mikrotik"
	"github.com/proisp/backend/internal/models"
)

type CDNHandler struct{}

func NewCDNHandler() *CDNHandler {
	return &CDNHandler{}
}

// parseSubnets parses a subnet string that may be comma or newline separated
func parseSubnets(subnetsStr string) []string {
	var subnets []string
	if subnetsStr == "" {
		return subnets
	}
	// Replace newlines with commas, then split by comma
	subnetsStr = strings.ReplaceAll(subnetsStr, "\r\n", ",")
	subnetsStr = strings.ReplaceAll(subnetsStr, "\n", ",")
	subnetsStr = strings.ReplaceAll(subnetsStr, "\r", ",")
	for _, subnet := range strings.Split(subnetsStr, ",") {
		subnet = strings.TrimSpace(subnet)
		if subnet != "" {
			subnets = append(subnets, subnet)
		}
	}
	return subnets
}

// parseNASIDs parses a comma-separated string of NAS IDs into a slice of uints
func parseNASIDs(nasIDsStr string) []uint {
	var nasIDs []uint
	if nasIDsStr == "" {
		return nasIDs
	}
	for _, idStr := range strings.Split(nasIDsStr, ",") {
		idStr = strings.TrimSpace(idStr)
		if idStr != "" {
			if id, err := strconv.ParseUint(idStr, 10, 32); err == nil {
				nasIDs = append(nasIDs, uint(id))
			}
		}
	}
	return nasIDs
}

// findRemovedNASIDs finds NAS IDs that were in oldIDs but not in newIDs
func findRemovedNASIDs(oldIDs, newIDs []uint) []uint {
	newSet := make(map[uint]bool)
	for _, id := range newIDs {
		newSet[id] = true
	}
	var removed []uint
	for _, id := range oldIDs {
		if !newSet[id] {
			removed = append(removed, id)
		}
	}
	return removed
}

// getCDNCompanyName retrieves company name from settings for CDN branding
func getCDNCompanyName() string {
	name := database.GetCompanyName()
	if name == "" {
		return "ISP"
	}
	return name
}

// syncCDNToAllNAS syncs a CDN configuration to selected NAS devices (or all if none specified)
func syncCDNToAllNAS(cdn models.CDN) {
	// Get company name for branding
	companyName := getCDNCompanyName()

	// Parse NAS IDs if specified
	selectedNASIDs := parseNASIDs(cdn.NASIDs)

	// Get NAS devices (filtered or all)
	var nasList []models.Nas
	if len(selectedNASIDs) > 0 {
		database.DB.Where("is_active = ? AND id IN ?", true, selectedNASIDs).Find(&nasList)
	} else {
		database.DB.Where("is_active = ?", true).Find(&nasList)
	}

	if len(nasList) == 0 {
		log.Printf("CDN Sync: No active NAS devices found")
		return
	}

	// Parse subnets from CDN
	subnets := parseSubnets(cdn.Subnets)
	if len(subnets) == 0 {
		log.Printf("CDN Sync: No valid subnets for CDN %s", cdn.Name)
		return
	}

	cdnConfig := mikrotik.CDNConfig{
		ID:          cdn.ID,
		Name:        cdn.Name,
		Subnets:     subnets,
		CompanyName: companyName,
	}

	// Sync to each NAS
	for _, nas := range nasList {
		go func(nas models.Nas) {
			client := mikrotik.NewClient(
				fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
				nas.APIUsername,
				nas.APIPassword,
			)
			defer client.Close()

			// Sync address list
			if err := client.SyncCDNAddressList(cdnConfig); err != nil {
				log.Printf("CDN Sync: Failed to sync address-list to NAS %s: %v", nas.Name, err)
				return
			}

			// Create mangle rule for counting
			if err := client.SyncCDNMangleRule(cdnConfig); err != nil {
				log.Printf("CDN Sync: Failed to sync mangle rule to NAS %s: %v", nas.Name, err)
				return
			}

			log.Printf("CDN Sync: Successfully synced CDN %s to NAS %s", cdn.Name, nas.Name)
		}(nas)
	}
}

// removeCDNFromAllNAS removes a CDN configuration from all NAS devices
func removeCDNFromAllNAS(cdnName string) {
	removeCDNFromNASList(cdnName, nil)
}

// removeCDNFromNASList removes a CDN configuration from specific NAS devices (or all if nasIDs is nil)
func removeCDNFromNASList(cdnName string, nasIDs []uint) {
	// Get company name for branding
	companyName := getCDNCompanyName()

	var nasList []models.Nas
	if len(nasIDs) > 0 {
		database.DB.Where("is_active = ? AND id IN ?", true, nasIDs).Find(&nasList)
	} else {
		database.DB.Where("is_active = ?", true).Find(&nasList)
	}

	for _, nas := range nasList {
		go func(nas models.Nas) {
			client := mikrotik.NewClient(
				fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
				nas.APIUsername,
				nas.APIPassword,
			)
			defer client.Close()

			if err := client.RemoveCDNConfig(cdnName, companyName); err != nil {
				log.Printf("CDN Sync: Failed to remove CDN %s from NAS %s: %v", cdnName, nas.Name, err)
			}
		}(nas)
	}
}

// syncCDNPCQToNAS syncs PCQ setup for a CDN to a specific NAS device
// This creates: queue type (PCQ), mangle rule (packet mark), simple queue with PCQ
func syncCDNPCQToNAS(cdnName string, speedLimitM int64, pcqLimit, pcqTotalLimit int, nasID uint, targetPools string, serviceName string) {
	companyName := getCDNCompanyName()

	var nas models.Nas
	if err := database.DB.First(&nas, nasID).Error; err != nil {
		log.Printf("CDN PCQ Sync: NAS with ID %d not found", nasID)
		return
	}

	if !nas.IsActive {
		log.Printf("CDN PCQ Sync: NAS %s is not active, skipping PCQ setup", nas.Name)
		return
	}

	// Look up CDN subnets from database
	var cdn models.CDN
	var subnets []string
	if err := database.DB.Where("name = ?", cdnName).First(&cdn).Error; err == nil {
		if cdn.Subnets != "" {
			subnets = parseSubnets(cdn.Subnets)
		}
	}

	client := mikrotik.NewClient(
		fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
		nas.APIUsername,
		nas.APIPassword,
	)
	defer client.Close()

	pcqConfig := mikrotik.PCQConfig{
		CDNName:       cdnName,
		SpeedLimitM:   speedLimitM,
		PCQLimit:      pcqLimit,
		PCQTotalLimit: pcqTotalLimit,
		TargetPools:   targetPools,
		CompanyName:   companyName,
		Subnets:       subnets,
		ServiceName:   serviceName,
	}

	if err := client.SyncCDNPCQSetup(pcqConfig); err != nil {
		log.Printf("CDN PCQ Sync: Failed to sync PCQ for CDN %s to NAS %s: %v", cdnName, nas.Name, err)
		return
	}

	log.Printf("CDN PCQ Sync: Successfully synced PCQ for CDN %s to NAS %s with pools: %s", cdnName, nas.Name, targetPools)
}

// syncCDNPCQToNASWithSubnets syncs PCQ setup for a CDN to a specific NAS device with subnets passed directly
// This is used when enabling PCQ from service page where we already have the CDN data
func syncCDNPCQToNASWithSubnets(cdnName string, speedLimitM int64, pcqLimit, pcqTotalLimit int, nasID uint, targetPools string, subnets []string, serviceName string) {
	companyName := getCDNCompanyName()

	var nas models.Nas
	if err := database.DB.First(&nas, nasID).Error; err != nil {
		log.Printf("CDN PCQ Sync: NAS with ID %d not found", nasID)
		return
	}

	if !nas.IsActive {
		log.Printf("CDN PCQ Sync: NAS %s is not active, skipping PCQ setup", nas.Name)
		return
	}

	client := mikrotik.NewClient(
		fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
		nas.APIUsername,
		nas.APIPassword,
	)
	defer client.Close()

	pcqConfig := mikrotik.PCQConfig{
		CDNName:       cdnName,
		SpeedLimitM:   speedLimitM,
		PCQLimit:      pcqLimit,
		PCQTotalLimit: pcqTotalLimit,
		TargetPools:   targetPools,
		CompanyName:   companyName,
		Subnets:       subnets,
		ServiceName:   serviceName,
	}

	if err := client.SyncCDNPCQSetup(pcqConfig); err != nil {
		log.Printf("CDN PCQ Sync: Failed to sync PCQ for CDN %s to NAS %s: %v", cdnName, nas.Name, err)
		return
	}

	log.Printf("CDN PCQ Sync: Successfully synced PCQ for CDN %s to NAS %s with pools: %s, subnets: %v", cdnName, nas.Name, targetPools, subnets)
}

// cleanupPCQFromNAS removes PCQ setup for a CDN from a specific NAS device when PCQ is disabled
func cleanupPCQFromNAS(cdnName string, speedLimitM int64, nasID uint, serviceName ...string) {
	companyName := getCDNCompanyName()

	var nas models.Nas
	if err := database.DB.First(&nas, nasID).Error; err != nil {
		log.Printf("CDN PCQ Cleanup: NAS with ID %d not found", nasID)
		return
	}

	if !nas.IsActive {
		log.Printf("CDN PCQ Cleanup: NAS %s is not active, skipping cleanup", nas.Name)
		return
	}

	client := mikrotik.NewClient(
		fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
		nas.APIUsername,
		nas.APIPassword,
	)
	defer client.Close()

	if err := client.RemoveCDNPCQSetup(cdnName, speedLimitM, companyName, serviceName...); err != nil {
		log.Printf("CDN PCQ Cleanup: Failed to remove PCQ for CDN %s %dM from NAS %s: %v", cdnName, speedLimitM, nas.Name, err)
		return
	}

	// Check if any other services still use this CDN on this NAS
	// If not, remove the mangle rule and address list too
	var count int64
	database.DB.Model(&models.ServiceCDN{}).
		Where("cdn_id = (SELECT id FROM cdns WHERE name = ?) AND pcq_enabled = ? AND pcqnas_id = ?", cdnName, true, nasID).
		Count(&count)

	if count == 0 {
		// No other services use this CDN, remove mangle and address list
		if err := client.RemoveCDNMangleAndAddressList(cdnName, companyName); err != nil {
			log.Printf("CDN PCQ Cleanup: Failed to remove mangle/address list for CDN %s from NAS %s: %v", cdnName, nas.Name, err)
		} else {
			log.Printf("CDN PCQ Cleanup: Removed mangle rule and address list for CDN %s from NAS %s (no more services using it)", cdnName, nas.Name)
		}
	}

	log.Printf("CDN PCQ Cleanup: Successfully removed PCQ for CDN %s %dM from NAS %s", cdnName, speedLimitM, nas.Name)
}

// syncCDNPCQToAllNAS syncs PCQ setup for a CDN to all active NAS devices (legacy, now uses per-service config)
func syncCDNPCQToAllNAS(cdnName string, speedLimitM int64, pcqLimit, pcqTotalLimit int, serviceName string) {
	companyName := getCDNCompanyName()

	// Look up CDN subnets from database
	var cdn models.CDN
	var subnets []string
	if err := database.DB.Where("name = ?", cdnName).First(&cdn).Error; err == nil {
		if cdn.Subnets != "" {
			subnets = parseSubnets(cdn.Subnets)
		}
	}

	var nasList []models.Nas
	database.DB.Where("is_active = ?", true).Find(&nasList)

	if len(nasList) == 0 {
		log.Printf("CDN PCQ Sync: No active NAS devices found")
		return
	}

	for _, nas := range nasList {
		go func(nas models.Nas, subnets []string) {
			// Skip NAS without subscriber pools configured
			if nas.SubscriberPools == "" {
				log.Printf("CDN PCQ Sync: NAS %s has no subscriber pools configured, skipping PCQ setup", nas.Name)
				return
			}

			client := mikrotik.NewClient(
				fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
				nas.APIUsername,
				nas.APIPassword,
			)
			defer client.Close()

			pcqConfig := mikrotik.PCQConfig{
				CDNName:       cdnName,
				SpeedLimitM:   speedLimitM,
				PCQLimit:      pcqLimit,
				PCQTotalLimit: pcqTotalLimit,
				TargetPools:   nas.SubscriberPools,
				CompanyName:   companyName,
				Subnets:       subnets,
				ServiceName:   serviceName,
			}

			if err := client.SyncCDNPCQSetup(pcqConfig); err != nil {
				log.Printf("CDN PCQ Sync: Failed to sync PCQ for CDN %s to NAS %s: %v", cdnName, nas.Name, err)
				return
			}

			log.Printf("CDN PCQ Sync: Successfully synced PCQ for CDN %s to NAS %s", cdnName, nas.Name)
		}(nas, subnets)
	}
}

// removeCDNPCQFromAllNAS removes ALL PCQ setups for a CDN from all NAS devices (all speeds)
// This is a legacy function - use cleanupPCQFromNAS for per-speed removal
func removeCDNPCQFromAllNAS(cdnName string) {
	// Get all service CDN configs for this CDN to find all speeds
	var cdn models.CDN
	if err := database.DB.Where("name = ?", cdnName).First(&cdn).Error; err != nil {
		log.Printf("CDN PCQ Cleanup: CDN %s not found", cdnName)
		return
	}

	var serviceCDNs []models.ServiceCDN
	database.DB.Preload("Service").Where("cdn_id = ?", cdn.ID).Find(&serviceCDNs)

	// Collect unique NAS+Speed combinations
	type nasSpeed struct {
		nasID       uint
		speed       int64
		serviceName string
	}
	toRemove := make(map[string]nasSpeed)
	for _, sc := range serviceCDNs {
		if sc.PCQNASID != nil && sc.SpeedLimit > 0 {
			key := fmt.Sprintf("%d-%d", *sc.PCQNASID, sc.SpeedLimit)
			sName := ""
			if sc.Service != nil {
				sName = sc.Service.Name
			}
			toRemove[key] = nasSpeed{*sc.PCQNASID, sc.SpeedLimit, sName}
		}
	}

	for _, ns := range toRemove {
		go cleanupPCQFromNAS(cdnName, ns.speed, ns.nasID, ns.serviceName)
	}
}

// List returns all CDNs
func (h *CDNHandler) List(c *fiber.Ctx) error {
	var cdns []models.CDN
	query := database.DB.Model(&models.CDN{})

	// Filter by active status if provided
	if active := c.Query("active"); active != "" {
		if active == "true" {
			query = query.Where("is_active = ?", true)
		} else if active == "false" {
			query = query.Where("is_active = ?", false)
		}
	}

	query.Order("name ASC").Find(&cdns)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    cdns,
	})
}

// Get returns a single CDN
func (h *CDNHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")

	var cdn models.CDN
	if err := database.DB.First(&cdn, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "CDN not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    cdn,
	})
}

// GetCDNSpeeds returns all available speeds for each CDN from service configurations
func (h *CDNHandler) GetCDNSpeeds(c *fiber.Ctx) error {
	type CDNSpeed struct {
		CDNID       uint   `json:"cdn_id"`
		CDNName     string `json:"cdn_name"`
		NASIDs      string `json:"nas_ids"`
		SpeedLimit  int64  `json:"speed_limit"`
		ServiceID   uint   `json:"service_id"`
		ServiceName string `json:"service_name"`
	}

	var speeds []CDNSpeed
	database.DB.Raw(`
		SELECT DISTINCT sc.cdn_id, c.name as cdn_name, COALESCE(c.nas_ids, '') as nas_ids, sc.speed_limit, sc.service_id, s.name as service_name
		FROM service_cdns sc
		JOIN cdns c ON sc.cdn_id = c.id
		JOIN services s ON sc.service_id = s.id
		WHERE sc.is_active = true
		ORDER BY c.name, sc.speed_limit
	`).Scan(&speeds)

	// Group by CDN
	cdnSpeedsMap := make(map[uint]map[string]interface{})
	for _, speed := range speeds {
		if _, ok := cdnSpeedsMap[speed.CDNID]; !ok {
			cdnSpeedsMap[speed.CDNID] = map[string]interface{}{
				"cdn_id":   speed.CDNID,
				"cdn_name": speed.CDNName,
				"nas_ids":  speed.NASIDs,
				"speeds":   []map[string]interface{}{},
			}
		}
		speedsList := cdnSpeedsMap[speed.CDNID]["speeds"].([]map[string]interface{})
		cdnSpeedsMap[speed.CDNID]["speeds"] = append(speedsList, map[string]interface{}{
			"speed_limit":  speed.SpeedLimit,
			"service_id":   speed.ServiceID,
			"service_name": speed.ServiceName,
		})
	}

	// Convert to slice
	var result []map[string]interface{}
	for _, v := range cdnSpeedsMap {
		result = append(result, v)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

// Create creates a new CDN
func (h *CDNHandler) Create(c *fiber.Ctx) error {
	var cdn models.CDN
	if err := c.BodyParser(&cdn); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Validate required fields
	if cdn.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "CDN name is required",
		})
	}

	// Check if name already exists
	var existingCount int64
	database.DB.Model(&models.CDN{}).Where("name = ?", cdn.Name).Count(&existingCount)
	if existingCount > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "CDN with this name already exists",
		})
	}

	if err := database.DB.Create(&cdn).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create CDN",
		})
	}

	// Sync to all NAS devices in background
	if cdn.IsActive && cdn.Subnets != "" {
		go syncCDNToAllNAS(cdn)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    cdn,
	})
}

// Update updates a CDN
func (h *CDNHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	var cdn models.CDN
	if err := database.DB.First(&cdn, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "CDN not found",
		})
	}

	var updates models.CDN
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Check if name already exists (if changing name)
	if updates.Name != "" && updates.Name != cdn.Name {
		var existingCount int64
		database.DB.Model(&models.CDN{}).Where("name = ? AND id != ?", updates.Name, id).Count(&existingCount)
		if existingCount > 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "CDN with this name already exists",
			})
		}
	}

	oldName := cdn.Name
	oldNASIDs := parseNASIDs(cdn.NASIDs)

	database.DB.Model(&cdn).Updates(map[string]interface{}{
		"name":        updates.Name,
		"description": updates.Description,
		"subnets":     updates.Subnets,
		"color":       updates.Color,
		"nas_ids":     updates.NASIDs,
		"is_active":   updates.IsActive,
	})

	// Reload to get updated data
	database.DB.First(&cdn, id)

	newNASIDs := parseNASIDs(cdn.NASIDs)

	// If name changed, remove old config from ALL NAS first (in case NAS selection changed too)
	if oldName != cdn.Name {
		go removeCDNFromAllNAS(oldName)
	} else {
		// Name didn't change, but NAS selection might have changed
		// Find NAS IDs that were removed (in old but not in new)
		removedNASIDs := findRemovedNASIDs(oldNASIDs, newNASIDs)
		if len(removedNASIDs) > 0 {
			go removeCDNFromNASList(cdn.Name, removedNASIDs)
		}
	}

	// Sync to selected NAS devices in background
	if cdn.IsActive && cdn.Subnets != "" {
		go syncCDNToAllNAS(cdn)
	} else if !cdn.IsActive {
		// If deactivated, remove from all NAS (not just selected ones)
		go removeCDNFromAllNAS(cdn.Name)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    cdn,
	})
}

// Delete deletes a CDN
func (h *CDNHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")

	// Get CDN name before deleting (for MikroTik cleanup)
	var cdn models.CDN
	if err := database.DB.First(&cdn, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "CDN not found",
		})
	}

	// Check if CDN is used in any services
	var usageCount int64
	database.DB.Model(&models.ServiceCDN{}).Where("cdn_id = ?", id).Count(&usageCount)
	if usageCount > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cannot delete CDN that is assigned to services",
		})
	}

	result := database.DB.Delete(&models.CDN{}, id)
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "CDN not found",
		})
	}

	// Remove CDN address list and mangle rules from all NAS devices
	go removeCDNFromAllNAS(cdn.Name)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "CDN deleted",
	})
}

// ServiceCDN handlers

// ListServiceCDNs returns all CDN configurations for a service
func (h *CDNHandler) ListServiceCDNs(c *fiber.Ctx) error {
	serviceID := c.Params("id")

	var serviceCDNs []models.ServiceCDN
	database.DB.Preload("CDN").Where("service_id = ?", serviceID).Find(&serviceCDNs)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    serviceCDNs,
	})
}

// UpdateServiceCDNs updates CDN configurations for a service (bulk update)
func (h *CDNHandler) UpdateServiceCDNs(c *fiber.Ctx) error {
	serviceID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid service ID",
		})
	}

	// Check if service exists
	var service models.Service
	if err := database.DB.First(&service, serviceID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Service not found",
		})
	}

	var input struct {
		CDNs []struct {
			CDNID          uint   `json:"cdn_id"`
			SpeedLimit     int64  `json:"speed_limit"`
			BypassQuota    bool   `json:"bypass_quota"`
			PCQEnabled     bool   `json:"pcq_enabled"`
			PCQLimit       int    `json:"pcq_limit"`
			PCQTotalLimit  int    `json:"pcq_total_limit"`
			PCQNASID       *uint  `json:"pcq_nas_id"`
			PCQTargetPools string `json:"pcq_target_pools"`
			IsActive       bool   `json:"is_active"`
			TimeFromHour   int    `json:"time_from_hour"`
			TimeFromMinute int    `json:"time_from_minute"`
			TimeToHour     int    `json:"time_to_hour"`
			TimeToMinute   int    `json:"time_to_minute"`
			TimeSpeedRatio int    `json:"time_speed_ratio"`
		} `json:"cdns"`
	}

	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Debug: Log received CDN configs
	for i, cdn := range input.CDNs {
		log.Printf("UpdateServiceCDNs: Received CDN[%d] cdn_id=%d is_active=%v", i, cdn.CDNID, cdn.IsActive)
	}

	// Fetch old configs with PCQ enabled BEFORE deleting (for cleanup)
	var oldConfigs []models.ServiceCDN
	database.DB.Preload("CDN").Preload("Service").Where("service_id = ? AND pcq_enabled = ?", serviceID, true).Find(&oldConfigs)

	// Build map of old PCQ configs: key = "cdn_id-speed-nas_id"
	oldPCQConfigs := make(map[string]models.ServiceCDN)
	for _, old := range oldConfigs {
		if old.PCQNASID != nil && old.SpeedLimit > 0 {
			key := fmt.Sprintf("%d-%d-%d", old.CDNID, old.SpeedLimit, *old.PCQNASID)
			oldPCQConfigs[key] = old
		}
	}

	// Delete existing service CDN configs
	database.DB.Where("service_id = ?", serviceID).Delete(&models.ServiceCDN{})

	// Track new PCQ configs for comparison
	newPCQKeys := make(map[string]bool)

	// Create new configs
	for _, cdnConfig := range input.CDNs {
		// Default time speed ratio to 100 if not set
		timeSpeedRatio := cdnConfig.TimeSpeedRatio
		if timeSpeedRatio == 0 {
			timeSpeedRatio = 100
		}
		// Default PCQ limits if not set
		pcqLimit := cdnConfig.PCQLimit
		if pcqLimit == 0 {
			pcqLimit = 50
		}
		pcqTotalLimit := cdnConfig.PCQTotalLimit
		if pcqTotalLimit == 0 {
			pcqTotalLimit = 2000
		}
		serviceCDN := models.ServiceCDN{
			ServiceID:      uint(serviceID),
			CDNID:          cdnConfig.CDNID,
			SpeedLimit:     cdnConfig.SpeedLimit,
			BypassQuota:    cdnConfig.BypassQuota,
			PCQEnabled:     cdnConfig.PCQEnabled,
			PCQLimit:       pcqLimit,
			PCQTotalLimit:  pcqTotalLimit,
			PCQNASID:       cdnConfig.PCQNASID,
			PCQTargetPools: cdnConfig.PCQTargetPools,
			IsActive:       cdnConfig.IsActive,
			TimeFromHour:   cdnConfig.TimeFromHour,
			TimeFromMinute: cdnConfig.TimeFromMinute,
			TimeToHour:     cdnConfig.TimeToHour,
			TimeToMinute:   cdnConfig.TimeToMinute,
			TimeSpeedRatio: timeSpeedRatio,
		}
		database.DB.Create(&serviceCDN)
		// Explicitly update boolean fields (GORM ignores false due to default:true in model)
		database.DB.Model(&serviceCDN).Updates(map[string]interface{}{
			"is_active":    cdnConfig.IsActive,
			"bypass_quota": cdnConfig.BypassQuota,
			"pcq_enabled":  cdnConfig.PCQEnabled,
		})

		// If PCQ enabled, sync PCQ setup to the specific NAS
		if cdnConfig.PCQEnabled && cdnConfig.IsActive && cdnConfig.SpeedLimit > 0 && cdnConfig.PCQNASID != nil && cdnConfig.PCQTargetPools != "" {
			// Track this as a new PCQ config
			key := fmt.Sprintf("%d-%d-%d", cdnConfig.CDNID, cdnConfig.SpeedLimit, *cdnConfig.PCQNASID)
			newPCQKeys[key] = true

			// Get CDN name and subnets
			var cdn models.CDN
			if err := database.DB.First(&cdn, cdnConfig.CDNID).Error; err == nil {
				// Parse subnets from CDN (handles both comma and newline separators)
				subnets := parseSubnets(cdn.Subnets)
				go syncCDNPCQToNASWithSubnets(cdn.Name, cdnConfig.SpeedLimit, pcqLimit, pcqTotalLimit, *cdnConfig.PCQNASID, cdnConfig.PCQTargetPools, subnets, service.Name)
			}
		}
	}

	// Clean up PCQ configs that were removed or disabled
	for key, oldConfig := range oldPCQConfigs {
		if !newPCQKeys[key] && oldConfig.CDN != nil {
			// This PCQ config was removed or disabled, clean up on MikroTik
			oldSvcName := ""
			if oldConfig.Service != nil {
				oldSvcName = oldConfig.Service.Name
			}
			go cleanupPCQFromNAS(oldConfig.CDN.Name, oldConfig.SpeedLimit, *oldConfig.PCQNASID, oldSvcName)
		}
	}

	// Return updated list
	var serviceCDNs []models.ServiceCDN
	database.DB.Preload("CDN").Where("service_id = ?", serviceID).Find(&serviceCDNs)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    serviceCDNs,
	})
}

// AddServiceCDN adds a CDN configuration to a service
func (h *CDNHandler) AddServiceCDN(c *fiber.Ctx) error {
	serviceID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid service ID",
		})
	}

	var serviceCDN models.ServiceCDN
	if err := c.BodyParser(&serviceCDN); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	serviceCDN.ServiceID = uint(serviceID)

	// Check if already exists
	var existingCount int64
	database.DB.Model(&models.ServiceCDN{}).Where("service_id = ? AND cdn_id = ?", serviceID, serviceCDN.CDNID).Count(&existingCount)
	if existingCount > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "CDN already assigned to this service",
		})
	}

	if err := database.DB.Create(&serviceCDN).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to add CDN to service",
		})
	}

	// Reload with CDN data
	database.DB.Preload("CDN").First(&serviceCDN, serviceCDN.ID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    serviceCDN,
	})
}

// DeleteServiceCDN removes a CDN configuration from a service
func (h *CDNHandler) DeleteServiceCDN(c *fiber.Ctx) error {
	serviceID := c.Params("id")
	cdnID := c.Params("cdnId")

	result := database.DB.Where("service_id = ? AND cdn_id = ?", serviceID, cdnID).Delete(&models.ServiceCDN{})
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "CDN configuration not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "CDN removed from service",
	})
}

// SyncToNAS manually syncs a CDN to all active NAS devices
func (h *CDNHandler) SyncToNAS(c *fiber.Ctx) error {
	id := c.Params("id")

	var cdn models.CDN
	if err := database.DB.First(&cdn, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "CDN not found",
		})
	}

	if !cdn.IsActive {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cannot sync inactive CDN",
		})
	}

	if cdn.Subnets == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "CDN has no subnets configured",
		})
	}

	// Sync to all NAS devices
	go syncCDNToAllNAS(cdn)

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Syncing CDN '%s' to all NAS devices", cdn.Name),
	})
}

// SyncAllToNAS syncs all active CDNs to all NAS devices
func (h *CDNHandler) SyncAllToNAS(c *fiber.Ctx) error {
	var cdns []models.CDN
	database.DB.Where("is_active = ? AND subnets != ''", true).Find(&cdns)

	if len(cdns) == 0 {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "No active CDNs to sync",
		})
	}

	// Sync each CDN
	for _, cdn := range cdns {
		go syncCDNToAllNAS(cdn)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Syncing %d CDNs to all NAS devices", len(cdns)),
	})
}

// SyncPCQToNAS manually syncs PCQ setup for a CDN to all NAS devices
func (h *CDNHandler) SyncPCQToNAS(c *fiber.Ctx) error {
	id := c.Params("id")

	var cdn models.CDN
	if err := database.DB.First(&cdn, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "CDN not found",
		})
	}

	// Find any ServiceCDN with PCQ enabled for this CDN
	var serviceCDN models.ServiceCDN
	if err := database.DB.Where("cdn_id = ? AND pcq_enabled = ? AND is_active = ?", cdn.ID, true, true).First(&serviceCDN).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "No PCQ-enabled configuration found for this CDN",
		})
	}

	if serviceCDN.SpeedLimit == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Speed limit must be configured for PCQ",
		})
	}

	// Get service name for queue comment
	svcName := ""
	var svc models.Service
	if err := database.DB.First(&svc, serviceCDN.ServiceID).Error; err == nil {
		svcName = svc.Name
	}

	// Sync PCQ to all NAS devices
	go syncCDNPCQToAllNAS(cdn.Name, serviceCDN.SpeedLimit, serviceCDN.PCQLimit, serviceCDN.PCQTotalLimit, svcName)

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Syncing PCQ for CDN '%s' to all NAS devices", cdn.Name),
	})
}

// SyncAllPCQToNAS syncs PCQ setup for all PCQ-enabled CDNs to all NAS devices
func (h *CDNHandler) SyncAllPCQToNAS(c *fiber.Ctx) error {
	// Find all ServiceCDNs with PCQ enabled
	var serviceCDNs []models.ServiceCDN
	database.DB.Preload("CDN").Preload("Service").Where("pcq_enabled = ? AND is_active = ?", true, true).Find(&serviceCDNs)

	if len(serviceCDNs) == 0 {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "No PCQ-enabled CDN configurations found",
		})
	}

	// Track which CDNs we've synced (to avoid duplicates)
	syncedCDNs := make(map[uint]bool)
	syncCount := 0

	for _, sc := range serviceCDNs {
		if syncedCDNs[sc.CDNID] || sc.SpeedLimit == 0 || sc.CDN == nil {
			continue
		}
		syncedCDNs[sc.CDNID] = true
		syncCount++
		sName := ""
		if sc.Service != nil {
			sName = sc.Service.Name
		}
		go syncCDNPCQToAllNAS(sc.CDN.Name, sc.SpeedLimit, sc.PCQLimit, sc.PCQTotalLimit, sName)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Syncing PCQ for %d CDNs to all NAS devices", syncCount),
	})
}
