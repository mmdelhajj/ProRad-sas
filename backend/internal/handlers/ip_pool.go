package handlers

import (
	"log"

	"github.com/gofiber/fiber/v2"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
	"github.com/proisp/backend/internal/services"
)

// GetIPPoolStats returns statistics for all IP pools
func GetIPPoolStats(c *fiber.Ctx) error {
	stats, err := services.IPPool.GetAllPoolStats()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get pool stats: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    stats,
	})
}

// ImportIPPools imports IP pools from all active NAS devices
func ImportIPPools(c *fiber.Ctx) error {
	log.Println("IPPoolHandler: Starting pool import from all NAS devices")

	count, err := services.IPPool.ImportAllPools()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to import pools: " + err.Error(),
		})
	}

	log.Printf("IPPoolHandler: Imported %d IPs from all NAS devices", count)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "IP pools imported successfully",
		"data": fiber.Map{
			"total_ips": count,
		},
	})
}

// ImportIPPoolsFromNAS imports IP pools from a specific NAS
func ImportIPPoolsFromNAS(c *fiber.Ctx) error {
	nasID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Invalid NAS ID",
		})
	}

	var nas models.Nas
	if err := database.DB.First(&nas, nasID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"message": "NAS not found",
		})
	}

	if nas.APIUsername == "" || nas.APIPassword == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "NAS has no API credentials configured",
		})
	}

	log.Printf("IPPoolHandler: Starting pool import from NAS %s (%s)", nas.Name, nas.IPAddress)

	count, err := services.IPPool.ImportPoolsFromMikrotik(&nas)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to import pools: " + err.Error(),
		})
	}

	log.Printf("IPPoolHandler: Imported %d IPs from NAS %s", count, nas.Name)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "IP pools imported successfully from " + nas.Name,
		"data": fiber.Map{
			"total_ips": count,
			"nas_name":  nas.Name,
		},
	})
}

// SyncActiveSessions syncs active sessions from all NAS devices
func SyncActiveSessions(c *fiber.Ctx) error {
	log.Println("IPPoolHandler: Starting active session sync from all NAS devices")

	count, err := services.IPPool.SyncAllActiveSessions()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to sync sessions: " + err.Error(),
		})
	}

	log.Printf("IPPoolHandler: Synced %d active sessions from all NAS devices", count)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Active sessions synced successfully",
		"data": fiber.Map{
			"total_sessions": count,
		},
	})
}

// SyncActiveSessionsFromNAS syncs active sessions from a specific NAS
func SyncActiveSessionsFromNAS(c *fiber.Ctx) error {
	nasID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "Invalid NAS ID",
		})
	}

	var nas models.Nas
	if err := database.DB.First(&nas, nasID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"message": "NAS not found",
		})
	}

	if nas.APIUsername == "" || nas.APIPassword == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"message": "NAS has no API credentials configured",
		})
	}

	log.Printf("IPPoolHandler: Starting session sync from NAS %s (%s)", nas.Name, nas.IPAddress)

	count, err := services.IPPool.SyncActiveSessionsFromMikrotik(&nas)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to sync sessions: " + err.Error(),
		})
	}

	log.Printf("IPPoolHandler: Synced %d active sessions from NAS %s", count, nas.Name)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Active sessions synced successfully from " + nas.Name,
		"data": fiber.Map{
			"total_sessions": count,
			"nas_name":       nas.Name,
		},
	})
}

// GetIPPoolAssignments returns list of IP assignments with filtering
func GetIPPoolAssignments(c *fiber.Ctx) error {
	poolName := c.Query("pool_name")
	status := c.Query("status")
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 100)

	offset := (page - 1) * limit

	query := database.DB.Model(&models.IPPoolAssignment{})

	if poolName != "" {
		query = query.Where("pool_name = ?", poolName)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var assignments []models.IPPoolAssignment
	if err := query.Offset(offset).Limit(limit).Order("pool_name, ip_address").Find(&assignments).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get assignments: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    assignments,
		"pagination": fiber.Map{
			"total":       total,
			"page":        page,
			"limit":       limit,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// EnableProISPIPManagement enables ProISP IP management (full import + sync)
func EnableProISPIPManagement(c *fiber.Ctx) error {
	log.Println("IPPoolHandler: Enabling ProISP IP Management...")

	// Step 1: Import all pools
	log.Println("IPPoolHandler: Step 1 - Importing IP pools from all NAS devices...")
	importCount, err := services.IPPool.ImportAllPools()
	if err != nil {
		log.Printf("IPPoolHandler: Error importing pools: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to import pools: " + err.Error(),
			"step":    "import_pools",
		})
	}
	log.Printf("IPPoolHandler: Imported %d IPs", importCount)

	// Step 2: Sync active sessions
	log.Println("IPPoolHandler: Step 2 - Syncing active sessions from all NAS devices...")
	syncCount, err := services.IPPool.SyncAllActiveSessions()
	if err != nil {
		log.Printf("IPPoolHandler: Error syncing sessions: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to sync sessions: " + err.Error(),
			"step":    "sync_sessions",
		})
	}
	log.Printf("IPPoolHandler: Synced %d active sessions", syncCount)

	// Step 3: Enable the feature flag in system preferences
	log.Println("IPPoolHandler: Step 3 - Enabling IP management feature flag...")
	if err := database.DB.Exec(`
		INSERT INTO system_preferences (key, value, value_type)
		VALUES ('proisp_ip_management', 'true', 'bool')
		ON CONFLICT (key) DO UPDATE SET value = 'true', updated_at = NOW()
	`).Error; err != nil {
		log.Printf("IPPoolHandler: Error setting feature flag: %v", err)
	}

	log.Println("IPPoolHandler: ProISP IP Management enabled successfully!")

	return c.JSON(fiber.Map{
		"success": true,
		"message": "ProISP IP Management enabled successfully",
		"data": fiber.Map{
			"total_ips":       importCount,
			"active_sessions": syncCount,
		},
	})
}

// DisableProISPIPManagement disables ProISP IP management
func DisableProISPIPManagement(c *fiber.Ctx) error {
	log.Println("IPPoolHandler: Disabling ProISP IP Management...")

	// Set the feature flag to false
	if err := database.DB.Exec(`
		INSERT INTO system_preferences (key, value, value_type)
		VALUES ('proisp_ip_management', 'false', 'bool')
		ON CONFLICT (key) DO UPDATE SET value = 'false', updated_at = NOW()
	`).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"message": "Failed to disable feature: " + err.Error(),
		})
	}

	log.Println("IPPoolHandler: ProISP IP Management disabled")

	return c.JSON(fiber.Map{
		"success": true,
		"message": "ProISP IP Management disabled. MikroTik will now manage IP assignments.",
	})
}

// GetIPManagementStatus returns the current IP management status
func GetIPManagementStatus(c *fiber.Ctx) error {
	// Check feature flag
	var pref models.SystemPreference
	enabled := false
	if err := database.DB.Where("key = ?", "proisp_ip_management").First(&pref).Error; err == nil {
		enabled = pref.Value == "true"
	}

	// Get pool stats
	stats, _ := services.IPPool.GetAllPoolStats()

	// Count totals
	var totalIPs, availableIPs, inUseIPs int64
	for _, s := range stats {
		totalIPs += s["total"].(int64)
		availableIPs += s["available"].(int64)
		inUseIPs += s["in_use"].(int64)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"enabled":       enabled,
			"total_ips":     totalIPs,
			"available_ips": availableIPs,
			"in_use_ips":    inUseIPs,
			"pools":         stats,
		},
	})
}
