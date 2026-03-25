package handlers

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/middleware"
	"github.com/proisp/backend/internal/mikrotik"
	"github.com/proisp/backend/internal/models"
	"github.com/proisp/backend/internal/radius"
	"github.com/proisp/backend/internal/services"
)

type NasHandler struct{}

func NewNasHandler() *NasHandler {
	return &NasHandler{}
}

// List returns all NAS devices (filtered by reseller assignment if user is reseller)
func (h *NasHandler) List(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	var nasList []models.Nas

	query := database.DB.Order("name ASC")

	// If user is a reseller, only show assigned NAS
	if user != nil && user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		// Get assigned NAS IDs
		var nasIDs []uint
		database.DB.Model(&models.ResellerNAS{}).
			Where("reseller_id = ?", *user.ResellerID).
			Pluck("nas_id", &nasIDs)

		if len(nasIDs) > 0 {
			query = query.Where("id IN ?", nasIDs)
		} else {
			// If no NAS assigned, return empty list
			return c.JSON(fiber.Map{
				"success": true,
				"data":    []models.Nas{},
			})
		}
	}

	if err := query.Find(&nasList).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch NAS devices",
		})
	}

	// Set computed fields for security indicators
	for i := range nasList {
		nasList[i].HasSecret = nasList[i].Secret != ""
		nasList[i].HasAPIPassword = nasList[i].APIPassword != ""
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    nasList,
	})
}

// Get returns a single NAS device
func (h *NasHandler) Get(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid NAS ID",
		})
	}

	var nas models.Nas
	if err := database.DB.First(&nas, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "NAS not found",
		})
	}

	// Get active sessions count
	var sessionCount int64
	database.DB.Model(&models.RadAcct{}).
		Where("nasipaddress = ? AND acctstoptime IS NULL", nas.IPAddress).
		Count(&sessionCount)

	// Set computed fields for security indicators
	nas.HasSecret = nas.Secret != ""
	nas.HasAPIPassword = nas.APIPassword != ""

	return c.JSON(fiber.Map{
		"success":        true,
		"data":           nas,
		"active_sessions": sessionCount,
	})
}

// CreateNasRequest represents create NAS request
type CreateNasRequest struct {
	Name        string `json:"name"`
	ShortName   string `json:"short_name"`
	IPAddress   string `json:"ip_address"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Secret      string `json:"secret"`
	AuthPort    int    `json:"auth_port"`
	AcctPort    int    `json:"acct_port"`
	CoAPort     int    `json:"coa_port"`
	APIUsername string `json:"api_username"`
	APIPassword string `json:"api_password"`
	APIPort     int    `json:"api_port"`
	APISSLPort  int    `json:"api_ssl_port"`
	UseSSL      bool   `json:"use_ssl"`
	FTPPort     int    `json:"ftp_port"`
}

// Create creates a new NAS device
func (h *NasHandler) Create(c *fiber.Ctx) error {
	var req CreateNasRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if req.Name == "" || req.IPAddress == "" || req.Secret == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Name, IP address, and secret are required",
		})
	}

	// Check if IP exists
	var existingCount int64
	database.DB.Model(&models.Nas{}).Where("ip_address = ?", req.IPAddress).Count(&existingCount)
	if existingCount > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "NAS with this IP address already exists",
		})
	}

	nas := models.Nas{
		Name:        req.Name,
		ShortName:   req.ShortName,
		IPAddress:   req.IPAddress,
		Type:        models.NasType(req.Type),
		Description: req.Description,
		Secret:      req.Secret,
		AuthPort:    req.AuthPort,
		AcctPort:    req.AcctPort,
		CoAPort:     req.CoAPort,
		APIUsername: req.APIUsername,
		APIPassword: req.APIPassword,
		APIPort:     req.APIPort,
		APISSLPort:  req.APISSLPort,
		UseSSL:      req.UseSSL,
		FTPPort:     req.FTPPort,
		IsActive:    true,
	}

	// Set defaults
	if nas.Type == "" {
		nas.Type = models.NasTypeMikrotik
	}
	if nas.AuthPort == 0 {
		nas.AuthPort = 1812
	}
	if nas.AcctPort == 0 {
		nas.AcctPort = 1813
	}
	if nas.CoAPort == 0 {
		nas.CoAPort = 1700
	}
	if nas.APIPort == 0 {
		nas.APIPort = 8728
	}
	if nas.APISSLPort == 0 {
		nas.APISSLPort = 8729
	}
	if nas.FTPPort == 0 {
		nas.FTPPort = 21
	}
	if nas.ShortName == "" {
		nas.ShortName = req.Name
	}

	if err := database.DB.Create(&nas).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create NAS",
		})
	}

	// Create audit log
	user := middleware.GetCurrentUser(c)
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionCreate,
		EntityType:  "nas",
		EntityID:    nas.ID,
		EntityName:  nas.Name,
		Description: "Created new NAS",
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	// Auto-import IP pools if API credentials are configured
	var ipPoolsImported int
	if nas.APIUsername != "" && nas.APIPassword != "" {
		log.Printf("NAS Create: Auto-importing IP pools from NAS %s (%s)", nas.Name, nas.IPAddress)
		go func(n models.Nas) {
			count, err := services.IPPool.ImportPoolsFromMikrotik(&n)
			if err != nil {
				log.Printf("NAS Create: Auto-import failed for %s: %v", n.Name, err)
			} else {
				log.Printf("NAS Create: Auto-imported %d IPs from %s", count, n.Name)
				// Also sync active sessions
				syncCount, err := services.IPPool.SyncActiveSessionsFromMikrotik(&n)
				if err != nil {
					log.Printf("NAS Create: Session sync failed for %s: %v", n.Name, err)
				} else {
					log.Printf("NAS Create: Synced %d active sessions from %s", syncCount, n.Name)
				}
			}
		}(nas)
		ipPoolsImported = -1 // -1 indicates async import started
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success":          true,
		"message":          "NAS created successfully",
		"data":             nas,
		"ip_pools_import":  ipPoolsImported != 0,
		"ip_pools_message": getIPPoolImportMessage(ipPoolsImported, nas.APIUsername != "" && nas.APIPassword != ""),
	})
}

// Update updates a NAS device
func (h *NasHandler) Update(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid NAS ID",
		})
	}

	var nas models.Nas
	if err := database.DB.First(&nas, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "NAS not found",
		})
	}

	var req map[string]interface{}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Map JSON field names to database column names (GORM snake_case)
	fieldMapping := map[string]string{
		"name":         "name",
		"short_name":   "short_name",
		"ip_address":   "ip_address",
		"type":         "type",
		"description":  "description",
		"secret":       "secret",
		"auth_port":    "auth_port",
		"acct_port":    "acct_port",
		"coa_port":     "coa_port",
		"api_username": "api_username",
		"api_password": "api_password",
		"api_port":     "api_port",
		"api_ssl_port": "api_ssl_port",
		"use_ssl":      "use_ssl",
		"ftp_port":     "ftp_port",
		"is_active":           "is_active",
		"cdn_torch_interface": "cdn_torch_interface",
	}

	updates := make(map[string]interface{})
	for jsonField, dbColumn := range fieldMapping {
		if val, ok := req[jsonField]; ok {
			updates[dbColumn] = val
		}
	}

	// Handle type field specially - convert string to NasType
	if typeVal, ok := updates["type"]; ok {
		if typeStr, ok := typeVal.(string); ok {
			updates["type"] = models.NasType(typeStr)
		}
	}

	if err := database.DB.Model(&nas).Updates(updates).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update NAS: " + err.Error(),
		})
	}

	// Create audit log
	user := middleware.GetCurrentUser(c)
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionUpdate,
		EntityType:  "nas",
		EntityID:    nas.ID,
		EntityName:  nas.Name,
		Description: "Updated NAS",
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	database.DB.First(&nas, id)

	// Auto-import IP pools if API credentials were just added
	var ipPoolsImported int
	apiUsernameUpdated := false
	apiPasswordUpdated := false
	if _, ok := updates["api_username"]; ok {
		apiUsernameUpdated = true
	}
	if _, ok := updates["api_password"]; ok {
		apiPasswordUpdated = true
	}

	// If API credentials were updated and both are now set, auto-import pools
	if (apiUsernameUpdated || apiPasswordUpdated) && nas.APIUsername != "" && nas.APIPassword != "" {
		log.Printf("NAS Update: Auto-importing IP pools from NAS %s (%s) after API credentials update", nas.Name, nas.IPAddress)
		go func(n models.Nas) {
			count, err := services.IPPool.ImportPoolsFromMikrotik(&n)
			if err != nil {
				log.Printf("NAS Update: Auto-import failed for %s: %v", n.Name, err)
			} else {
				log.Printf("NAS Update: Auto-imported %d IPs from %s", count, n.Name)
				// Also sync active sessions
				syncCount, err := services.IPPool.SyncActiveSessionsFromMikrotik(&n)
				if err != nil {
					log.Printf("NAS Update: Session sync failed for %s: %v", n.Name, err)
				} else {
					log.Printf("NAS Update: Synced %d active sessions from %s", syncCount, n.Name)
				}
			}
		}(nas)
		ipPoolsImported = -1 // -1 indicates async import started
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"message":          "NAS updated successfully",
		"data":             nas,
		"ip_pools_import":  ipPoolsImported != 0,
		"ip_pools_message": getIPPoolImportMessage(ipPoolsImported, (apiUsernameUpdated || apiPasswordUpdated) && nas.APIUsername != "" && nas.APIPassword != ""),
	})
}

// Delete deletes a NAS device
func (h *NasHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid NAS ID",
		})
	}

	var nas models.Nas
	if err := database.DB.First(&nas, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "NAS not found",
		})
	}

	// Check if NAS has subscribers
	var subscriberCount int64
	database.DB.Model(&models.Subscriber{}).Where("nas_id = ?", id).Count(&subscriberCount)
	if subscriberCount > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cannot delete NAS with assigned subscribers",
		})
	}

	if err := database.DB.Delete(&nas).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to delete NAS",
		})
	}

	// Create audit log
	user := middleware.GetCurrentUser(c)
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionDelete,
		EntityType:  "nas",
		EntityID:    nas.ID,
		EntityName:  nas.Name,
		Description: "Deleted NAS",
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "NAS deleted successfully",
	})
}

// Sync syncs NAS with Mikrotik
func (h *NasHandler) Sync(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid NAS ID",
		})
	}

	var nas models.Nas
	if err := database.DB.First(&nas, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "NAS not found",
		})
	}

	// TODO: Implement Mikrotik API sync
	// - Connect to Mikrotik
	// - Get active PPPoE sessions
	// - Sync with database

	return c.JSON(fiber.Map{
		"success": true,
		"message": "NAS sync initiated",
	})
}

// TestConnection tests NAS connectivity, API authentication, and RADIUS secret
func (h *NasHandler) TestConnection(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid NAS ID",
		})
	}

	var nas models.Nas
	if err := database.DB.First(&nas, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "NAS not found",
		})
	}

	// Test real MikroTik API authentication
	apiAddr := fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort)
	client := mikrotik.NewClient(apiAddr, nas.APIUsername, nas.APIPassword)
	apiResult := client.TestConnection()
	defer client.Close()

	// Test RADIUS secret
	radiusResult := radius.TestSecret(nas.IPAddress, nas.AuthPort, nas.Secret)

	// Update database based on REAL authentication results
	now := time.Now()
	updates := map[string]interface{}{
		"is_online":  apiResult.APIAuth, // Only online if API auth succeeded
		"last_seen":  &now,
	}
	database.DB.Model(&nas).Updates(updates)

	// Build response
	response := fiber.Map{
		"success":       true,
		"message":       "Connection test completed",
		"is_online":     apiResult.IsOnline,     // Port reachable
		"api_auth":      apiResult.APIAuth,      // API credentials valid
		"api_ok":        apiResult.APIAuth,      // For backwards compatibility
		"router_info":   apiResult.RouterInfo,
		"secret_valid":  radiusResult.SecretValid, // RADIUS secret valid
		"radius_ok":     radiusResult.SecretValid, // Alias
	}

	if apiResult.ErrorMsg != "" {
		response["api_error"] = apiResult.ErrorMsg
	}
	if radiusResult.ErrorMsg != "" {
		response["radius_error"] = radiusResult.ErrorMsg
	}

	// Build summary message
	var status []string
	if apiResult.APIAuth {
		status = append(status, "API: OK")
	} else if apiResult.IsOnline {
		status = append(status, "API: Auth Failed")
	} else {
		status = append(status, "API: Unreachable")
	}

	if radiusResult.SecretValid {
		status = append(status, "RADIUS: OK")
	} else {
		status = append(status, "RADIUS: Secret Invalid")
	}

	response["message"] = fmt.Sprintf("%s | %s", status[0], status[1])

	return c.JSON(response)
}

// GetIPPools fetches available IP pools from a NAS device
func (h *NasHandler) GetIPPools(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid NAS ID",
		})
	}

	var nas models.Nas
	if err := database.DB.First(&nas, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "NAS not found",
		})
	}

	// Connect to MikroTik and get IP pools
	client := mikrotik.NewClient(
		fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
		nas.APIUsername,
		nas.APIPassword,
	)
	if err := client.Connect(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to connect to NAS: %v", err),
		})
	}
	defer client.Close()

	pools, err := client.GetIPPools()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to get IP pools: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    pools,
	})
}

// GetInterfaces fetches all interfaces from a MikroTik NAS
func (h *NasHandler) GetInterfaces(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid NAS ID"})
	}

	var nas models.Nas
	if err := database.DB.First(&nas, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "NAS not found"})
	}

	client := mikrotik.NewClient(fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort), nas.APIUsername, nas.APIPassword)
	if err := client.Connect(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": fmt.Sprintf("Failed to connect to NAS: %v", err)})
	}
	defer client.Close()

	ifaces, err := client.GetInterfaces()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": fmt.Sprintf("Failed to get interfaces: %v", err)})
	}

	return c.JSON(fiber.Map{"success": true, "data": ifaces})
}

// UpdateSubscriberPools updates the subscriber pools for a NAS device
func (h *NasHandler) UpdateSubscriberPools(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid NAS ID",
		})
	}

	var nas models.Nas
	if err := database.DB.First(&nas, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "NAS not found",
		})
	}

	var input struct {
		SubscriberPools string `json:"subscriber_pools"`
	}

	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	nas.SubscriberPools = input.SubscriberPools
	if err := database.DB.Save(&nas).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update subscriber pools",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    nas,
		"message": "Subscriber pools updated",
	})
}

// getIPPoolImportMessage returns a message about IP pool import status
func getIPPoolImportMessage(count int, hasCredentials bool) string {
	if !hasCredentials {
		return "No API credentials configured - IP pools not imported"
	}
	if count == -1 {
		return "IP pool import started in background"
	}
	if count == 0 {
		return "No IP pools found to import"
	}
	return fmt.Sprintf("Imported %d IPs from MikroTik pools", count)
}

// GetNASDashboard returns aggregated stats for a specific NAS
func (h *NasHandler) GetNASDashboard(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid NAS ID"})
	}

	var nas models.Nas
	if err := database.DB.First(&nas, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "NAS not found"})
	}

	// Subscriber counts
	var totalSubs, onlineSubs int64
	database.DB.Model(&models.Subscriber{}).Where("nas_id = ? AND deleted_at IS NULL", id).Count(&totalSubs)
	database.DB.Model(&models.Subscriber{}).Where("nas_id = ? AND is_online = true AND deleted_at IS NULL", id).Count(&onlineSubs)

	// Total bandwidth from active sessions
	var totalDownload, totalUpload int64
	database.DB.Raw(`
		SELECT COALESCE(SUM(acctinputoctets), 0), COALESCE(SUM(acctoutputoctets), 0)
		FROM radacct WHERE acctstoptime IS NULL AND nasipaddress = ?`, nas.IPAddress).
		Row().Scan(&totalDownload, &totalUpload)

	// Top 10 bandwidth users
	type TopUser struct {
		Username string `json:"username"`
		Download int64  `json:"download"`
		Upload   int64  `json:"upload"`
	}
	var topUsers []TopUser
	database.DB.Raw(`
		SELECT r.username, r.acctinputoctets as download, r.acctoutputoctets as upload
		FROM radacct r WHERE r.acctstoptime IS NULL AND r.nasipaddress = ?
		ORDER BY (r.acctinputoctets + r.acctoutputoctets) DESC LIMIT 10`, nas.IPAddress).
		Scan(&topUsers)

	// Active sessions count over last 24h (hourly)
	type HourlySession struct {
		Hour  int   `json:"hour"`
		Count int64 `json:"count"`
	}
	var hourlySessions []HourlySession
	database.DB.Raw(`
		SELECT EXTRACT(HOUR FROM acctstarttime)::int as hour, COUNT(*) as count
		FROM radacct WHERE nasipaddress = ? AND acctstarttime > NOW() - INTERVAL '24 hours'
		GROUP BY hour ORDER BY hour`, nas.IPAddress).
		Scan(&hourlySessions)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"nas":              nas,
			"total_subscribers": totalSubs,
			"online_count":     onlineSubs,
			"offline_count":    totalSubs - onlineSubs,
			"total_download":   totalDownload,
			"total_upload":     totalUpload,
			"top_users":        topUsers,
			"hourly_sessions":  hourlySessions,
		},
	})
}

// GetNetworkMap returns all NAS devices with subscriber counts for network topology view
func (h *NasHandler) GetNetworkMap(c *fiber.Ctx) error {
	var nasList []models.Nas
	if err := database.DB.Where("deleted_at IS NULL").Find(&nasList).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to load NAS devices"})
	}

	type NASNode struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		IPAddress   string `json:"ip_address"`
		IsOnline    bool   `json:"is_online"`
		Subscribers int64  `json:"subscribers"`
		Online      int64  `json:"online"`
		LastSeen    *time.Time `json:"last_seen"`
	}

	var nodes []NASNode
	for _, nas := range nasList {
		node := NASNode{
			ID:        nas.ID,
			Name:      nas.Name,
			IPAddress: nas.IPAddress,
			IsOnline:  nas.IsOnline,
			LastSeen:  nas.LastSeen,
		}
		database.DB.Model(&models.Subscriber{}).Where("nas_id = ? AND deleted_at IS NULL", nas.ID).Count(&node.Subscribers)
		database.DB.Model(&models.Subscriber{}).Where("nas_id = ? AND is_online = true AND deleted_at IS NULL", nas.ID).Count(&node.Online)
		nodes = append(nodes, node)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    nodes,
	})
}
