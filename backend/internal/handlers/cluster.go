package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
	"github.com/proisp/backend/internal/services"
)

// ClusterHandler handles HA cluster operations
type ClusterHandler struct{}

// NewClusterHandler creates a new cluster handler
func NewClusterHandler() *ClusterHandler {
	return &ClusterHandler{}
}

// GetConfig returns the current cluster configuration
func (h *ClusterHandler) GetConfig(c *fiber.Ctx) error {
	var config models.ClusterConfig
	result := database.DB.First(&config)

	if result.Error != nil {
		// Return default standalone config if not configured
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"server_role":        "standalone",
				"is_active":          false,
				"cluster_id":         "",
				"server_ip":          getLocalIP(),
				"hardware_id":        getHardwareID(),
				"database_id":        getDatabaseID(),
			},
		})
	}

	// Don't expose secret in response
	config.ClusterSecret = ""
	if config.ServerRole == models.ClusterRoleMain {
		// Main server can see the secret (for sharing with secondary)
		var fullConfig models.ClusterConfig
		database.DB.First(&fullConfig)
		config.ClusterSecret = fullConfig.ClusterSecret
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    config,
	})
}

// SetupMain configures this server as the main server
func (h *ClusterHandler) SetupMain(c *fiber.Ctx) error {
	var req struct {
		ServerName string `json:"server_name"`
		ServerIP   string `json:"server_ip"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request",
		})
	}

	if req.ServerIP == "" {
		req.ServerIP = getLocalIP()
	}

	if req.ServerName == "" {
		req.ServerName = "Main Server"
	}

	// Generate cluster ID and secret
	clusterID := generateClusterID()
	clusterSecret := generateClusterSecret()

	// Check if config already exists
	var existingConfig models.ClusterConfig
	if database.DB.First(&existingConfig).Error == nil {
		// Update existing config
		existingConfig.ClusterID = clusterID
		existingConfig.ClusterSecret = clusterSecret
		existingConfig.ServerRole = models.ClusterRoleMain
		existingConfig.ServerName = req.ServerName
		existingConfig.ServerIP = req.ServerIP
		existingConfig.MainServerIP = req.ServerIP
		existingConfig.HardwareID = getHardwareID()
		existingConfig.DatabaseID = getDatabaseID()
		existingConfig.IsActive = true
		existingConfig.DBReplicationEnabled = true
		existingConfig.RedisReplicationEnabled = true
		existingConfig.RadiusEnabled = true
		existingConfig.RadiusRole = "primary"
		existingConfig.APIEnabled = true
		existingConfig.APIRole = "active"
		now := time.Now()
		existingConfig.LastHeartbeat = &now

		if err := database.DB.Save(&existingConfig).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to save config: " + err.Error(),
			})
		}
	} else {
		// Create new config
		now := time.Now()
		config := models.ClusterConfig{
			ClusterID:              clusterID,
			ClusterSecret:          clusterSecret,
			ServerRole:             models.ClusterRoleMain,
			ServerName:             req.ServerName,
			ServerIP:               req.ServerIP,
			MainServerIP:           req.ServerIP,
			HardwareID:             getHardwareID(),
			DatabaseID:             getDatabaseID(),
			IsActive:               true,
			DBReplicationEnabled:   true,
			DBSyncPort:             5433,
			RedisReplicationEnabled: true,
			RadiusEnabled:          true,
			RadiusRole:             "primary",
			APIEnabled:             true,
			APIRole:                "active",
			AutoFailoverEnabled:    true,
			FailoverPriority:       1,
			LastHeartbeat:          &now,
		}

		if err := database.DB.Create(&config).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to create config: " + err.Error(),
			})
		}
	}

	// Register self as first node
	h.registerNode(clusterID, getHardwareID(), models.ClusterRoleMain, req.ServerName, req.ServerIP)

	// Log event
	h.logEvent(clusterID, "cluster_created", 0, req.ServerIP, "main", "HA Cluster created, Main server configured", "info")

	// Setup PostgreSQL for replication
	go h.setupPostgreSQLMain()

	return c.JSON(fiber.Map{
		"success":        true,
		"message":        "Main server configured successfully",
		"cluster_id":     clusterID,
		"cluster_secret": clusterSecret,
		"server_ip":      req.ServerIP,
	})
}

// SetupSecondary configures this server as a secondary server
func (h *ClusterHandler) SetupSecondary(c *fiber.Ctx) error {
	var req struct {
		MainServerIP  string `json:"main_server_ip"`
		ClusterSecret string `json:"cluster_secret"`
		ServerName    string `json:"server_name"`
		ServerIP      string `json:"server_ip"`
		ServerRole    string `json:"server_role"` // secondary, server3, etc.
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request",
		})
	}

	if req.MainServerIP == "" || req.ClusterSecret == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Main server IP and cluster secret are required",
		})
	}

	if req.ServerIP == "" {
		req.ServerIP = getLocalIP()
	}

	if req.ServerName == "" {
		req.ServerName = "Secondary Server"
	}

	role := models.ClusterRoleSecondary
	if req.ServerRole == "server3" {
		role = models.ClusterRoleServer3
	} else if req.ServerRole == "server4" {
		role = models.ClusterRoleServer4
	} else if req.ServerRole == "server5" {
		role = models.ClusterRoleServer5
	}

	// Join the cluster on main server
	joinReq := models.ClusterJoinRequest{
		ClusterSecret: req.ClusterSecret,
		HardwareID:    getHardwareID(),
		ServerName:    req.ServerName,
		ServerIP:      req.ServerIP,
		RequestedRole: role,
		DatabaseID:    getDatabaseID(),
	}

	joinResp, err := h.joinCluster(req.MainServerIP, joinReq)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Failed to join cluster: " + err.Error(),
		})
	}

	if !joinResp.Success {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": joinResp.Message,
		})
	}

	// Save local config
	var existingConfig models.ClusterConfig
	if database.DB.First(&existingConfig).Error == nil {
		existingConfig.ClusterID = joinResp.ClusterID
		existingConfig.ClusterSecret = req.ClusterSecret
		existingConfig.ServerRole = joinResp.AssignedRole
		existingConfig.ServerName = req.ServerName
		existingConfig.ServerIP = req.ServerIP
		existingConfig.MainServerIP = joinResp.MainServerIP
		existingConfig.HardwareID = getHardwareID()
		existingConfig.DatabaseID = getDatabaseID()
		existingConfig.IsActive = true
		existingConfig.DBReplicationEnabled = true
		existingConfig.DBReplicationStatus = models.ClusterStatusSyncing
		existingConfig.RedisReplicationEnabled = true
		existingConfig.RadiusEnabled = role == models.ClusterRoleSecondary
		existingConfig.RadiusRole = "backup"
		existingConfig.APIEnabled = true
		existingConfig.APIRole = "standby"
		now := time.Now()
		existingConfig.LastHeartbeat = &now

		database.DB.Save(&existingConfig)
	} else {
		now := time.Now()
		config := models.ClusterConfig{
			ClusterID:              joinResp.ClusterID,
			ClusterSecret:          req.ClusterSecret,
			ServerRole:             joinResp.AssignedRole,
			ServerName:             req.ServerName,
			ServerIP:               req.ServerIP,
			MainServerIP:           joinResp.MainServerIP,
			MainServerPort:         80,
			HardwareID:             getHardwareID(),
			DatabaseID:             getDatabaseID(),
			IsActive:               true,
			DBReplicationEnabled:   true,
			DBReplicationStatus:    models.ClusterStatusSyncing,
			DBSyncPort:             joinResp.DBSyncPort,
			RedisReplicationEnabled: true,
			RadiusEnabled:          role == models.ClusterRoleSecondary,
			RadiusRole:             "backup",
			APIEnabled:             true,
			APIRole:                "standby",
			AutoFailoverEnabled:    true,
			FailoverPriority:       2,
			LastHeartbeat:          &now,
		}

		database.DB.Create(&config)
	}

	// Setup PostgreSQL replication from main
	go h.setupPostgreSQLReplica(joinResp)

	// Setup Redis replication
	go h.setupRedisReplica(joinResp)

	// Sync uploads (logo, favicon, etc.) from main server
	go h.syncUploadsFromMain(joinResp.MainServerIP, req.ClusterSecret)

	return c.JSON(fiber.Map{
		"success":       true,
		"message":       "Successfully joined cluster as " + string(joinResp.AssignedRole),
		"cluster_id":    joinResp.ClusterID,
		"assigned_role": joinResp.AssignedRole,
		"main_server":   joinResp.MainServerIP,
	})
}

// JoinCluster handles requests from secondary servers to join
func (h *ClusterHandler) JoinCluster(c *fiber.Ctx) error {
	var req models.ClusterJoinRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request",
		})
	}

	// Get our config
	var config models.ClusterConfig
	if database.DB.First(&config).Error != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "This server is not configured as main",
		})
	}

	// Verify we are main
	if config.ServerRole != models.ClusterRoleMain {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "This server is not the main server",
		})
	}

	// Verify secret
	if req.ClusterSecret != config.ClusterSecret {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Invalid cluster secret",
		})
	}

	// Check if already in cluster
	var existingNode models.ClusterNode
	if database.DB.Where("hardware_id = ?", req.HardwareID).First(&existingNode).Error == nil {
		// Update existing node
		existingNode.ServerName = req.ServerName
		existingNode.ServerIP = req.ServerIP
		existingNode.ServerRole = req.RequestedRole
		existingNode.Status = models.ClusterStatusOnline
		now := time.Now()
		existingNode.LastHeartbeat = &now
		database.DB.Save(&existingNode)

		h.logEvent(config.ClusterID, "node_rejoined", existingNode.ID, req.ServerIP, string(req.RequestedRole),
			fmt.Sprintf("Node %s rejoined cluster", req.ServerName), "info")
	} else {
		// Register new node
		h.registerNode(config.ClusterID, req.HardwareID, req.RequestedRole, req.ServerName, req.ServerIP)

		h.logEvent(config.ClusterID, "node_joined", 0, req.ServerIP, string(req.RequestedRole),
			fmt.Sprintf("Node %s joined cluster as %s", req.ServerName, req.RequestedRole), "info")
	}

	// Get database credentials
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" || dbHost == "db" || dbHost == "localhost" {
		dbHost = config.ServerIP // Use main server IP for external access
	}
	dbPort, _ := strconv.Atoi(os.Getenv("DB_PORT"))
	if dbPort == 0 {
		dbPort = 5432
	}
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" || redisHost == "redis" || redisHost == "localhost" {
		redisHost = config.ServerIP
	}
	redisPort, _ := strconv.Atoi(os.Getenv("REDIS_PORT"))
	if redisPort == 0 {
		redisPort = 6379
	}
	redisPass := os.Getenv("REDIS_PASSWORD")

	// Create replication slot name
	slotName := fmt.Sprintf("replica_%s", strings.ReplaceAll(req.HardwareID, "-", "_")[:16])

	return c.JSON(models.ClusterJoinResponse{
		Success:         true,
		Message:         "Successfully joined cluster",
		ClusterID:       config.ClusterID,
		AssignedRole:    req.RequestedRole,
		MainServerIP:    config.ServerIP,
		DBSyncPort:      config.DBSyncPort,
		DBHost:          dbHost,
		DBPort:          dbPort,
		DBUser:          dbUser,
		DBPassword:      dbPass,
		DBName:          dbName,
		RedisHost:       redisHost,
		RedisPort:       redisPort,
		RedisPassword:   redisPass,
		ReplicationSlot: slotName,
	})
}

// Heartbeat receives heartbeat from cluster nodes
func (h *ClusterHandler) Heartbeat(c *fiber.Ctx) error {
	var req models.ClusterHeartbeat

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request",
		})
	}

	// Verify cluster secret
	var config models.ClusterConfig
	if database.DB.First(&config).Error != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cluster not configured",
		})
	}

	if req.ClusterSecret != config.ClusterSecret {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Invalid cluster secret",
		})
	}

	// Update node status
	var node models.ClusterNode
	if database.DB.Where("hardware_id = ?", req.HardwareID).First(&node).Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Node not found in cluster",
		})
	}

	now := time.Now()
	node.Status = models.ClusterStatusOnline
	node.LastHeartbeat = &now
	node.CPUCores = req.CPUCores
	node.RAMGB = req.RAMGB
	node.CPUUsage = req.CPUUsage
	node.MemoryUsage = req.MemoryUsage
	node.DiskUsage = req.DiskUsage
	node.DBReplicationLag = req.DBReplicationLag
	node.Version = req.Version

	// Update sync status based on replication lag
	if req.DBReplicationLag < 5 {
		node.DBSyncStatus = models.ClusterStatusOnline
	} else if req.DBReplicationLag < 30 {
		node.DBSyncStatus = models.ClusterStatusSyncing
	} else {
		node.DBSyncStatus = models.ClusterStatusError
	}

	database.DB.Save(&node)

	// Get main server's version for auto-update feature
	mainVersion := getMainVersion()
	updateAvailable := mainVersion != "" && req.Version != "" && mainVersion != req.Version

	return c.JSON(models.ClusterHeartbeatResponse{
		Success:         true,
		Message:         "Heartbeat received",
		MainVersion:     mainVersion,
		UpdateAvailable: updateAvailable,
	})
}

// getMainVersion returns the current version of this server
func getMainVersion() string {
	// Try /opt/proxpanel/VERSION first
	if data, err := os.ReadFile("/opt/proxpanel/VERSION"); err == nil {
		return strings.TrimSpace(string(data))
	}
	// Try /app/VERSION (inside container)
	if data, err := os.ReadFile("/app/VERSION"); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

// GetStatus returns the cluster status
func (h *ClusterHandler) GetStatus(c *fiber.Ctx) error {
	var config models.ClusterConfig
	if database.DB.First(&config).Error != nil {
		return c.JSON(fiber.Map{
			"success": true,
			"data": models.ClusterStatusResponse{
				IsActive:   false,
				ServerRole: models.ClusterRoleStandalone,
			},
		})
	}

	// Get all nodes
	var nodes []models.ClusterNode
	database.DB.Where("cluster_id = ?", config.ClusterID).Find(&nodes)

	// Count online nodes
	onlineCount := 0
	dbOK := true
	redisOK := true
	for _, node := range nodes {
		if node.Status == models.ClusterStatusOnline {
			onlineCount++
		}
		if node.DBSyncStatus != models.ClusterStatusOnline {
			dbOK = false
		}
		if node.RedisSyncStatus != models.ClusterStatusOnline {
			redisOK = false
		}
	}

	// Get recent events
	var events []models.ClusterEvent
	database.DB.Where("cluster_id = ?", config.ClusterID).
		Order("created_at DESC").
		Limit(10).
		Find(&events)

	return c.JSON(fiber.Map{
		"success": true,
		"data": models.ClusterStatusResponse{
			ClusterID:          config.ClusterID,
			IsActive:           config.IsActive,
			ServerRole:         config.ServerRole,
			TotalNodes:         len(nodes),
			OnlineNodes:        onlineCount,
			Nodes:              nodes,
			DBReplicationOK:    dbOK,
			RedisReplicationOK: redisOK,
			LastSyncTime:       config.LastSyncTime,
			Events:             events,
		},
	})
}

// LeaveCluster removes this server from the cluster
func (h *ClusterHandler) LeaveCluster(c *fiber.Ctx) error {
	var config models.ClusterConfig
	if database.DB.First(&config).Error != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Not part of a cluster",
		})
	}

	if config.ServerRole == models.ClusterRoleMain {
		// Main server leaving - need to promote secondary or dissolve
		var nodes []models.ClusterNode
		database.DB.Where("cluster_id = ? AND server_role != ?", config.ClusterID, "main").Find(&nodes)

		if len(nodes) > 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Cannot leave cluster while secondary servers are connected. Remove them first or promote one to main.",
			})
		}

		// Delete all cluster data
		database.DB.Where("cluster_id = ?", config.ClusterID).Delete(&models.ClusterNode{})
		database.DB.Where("cluster_id = ?", config.ClusterID).Delete(&models.ClusterEvent{})
	} else {
		// Secondary leaving - notify main
		go h.notifyMainOfLeave(config)

		// Delete local node from main's database (if reachable)
		database.DB.Where("hardware_id = ?", config.HardwareID).Delete(&models.ClusterNode{})
	}

	// Reset local config to standalone
	config.ServerRole = models.ClusterRoleStandalone
	config.IsActive = false
	config.ClusterID = ""
	config.ClusterSecret = ""
	config.MainServerIP = ""
	config.DBReplicationEnabled = false
	config.RedisReplicationEnabled = false
	database.DB.Save(&config)

	// Stop replication
	go h.stopReplication()

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Left cluster successfully",
	})
}

// RemoveNode removes a node from the cluster (main only)
func (h *ClusterHandler) RemoveNode(c *fiber.Ctx) error {
	nodeID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid node ID",
		})
	}

	var config models.ClusterConfig
	if database.DB.First(&config).Error != nil || config.ServerRole != models.ClusterRoleMain {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"message": "Only main server can remove nodes",
		})
	}

	var node models.ClusterNode
	if database.DB.First(&node, nodeID).Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Node not found",
		})
	}

	if node.ServerRole == models.ClusterRoleMain {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cannot remove main server",
		})
	}

	h.logEvent(config.ClusterID, "node_removed", node.ID, node.ServerIP, string(node.ServerRole),
		fmt.Sprintf("Node %s removed from cluster", node.ServerName), "warning")

	database.DB.Delete(&node)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Node removed from cluster",
	})
}

// ManualFailover triggers manual failover to secondary
func (h *ClusterHandler) ManualFailover(c *fiber.Ctx) error {
	var req struct {
		TargetNodeID uint `json:"target_node_id"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request",
		})
	}

	var config models.ClusterConfig
	if database.DB.First(&config).Error != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cluster not configured",
		})
	}

	var targetNode models.ClusterNode
	if database.DB.First(&targetNode, req.TargetNodeID).Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Target node not found",
		})
	}

	if targetNode.Status != models.ClusterStatusOnline {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Target node is not online",
		})
	}

	h.logEvent(config.ClusterID, "manual_failover", targetNode.ID, targetNode.ServerIP, string(targetNode.ServerRole),
		fmt.Sprintf("Manual failover initiated to %s", targetNode.ServerName), "warning")

	// TODO: Implement actual failover logic
	// 1. Promote target PostgreSQL to primary
	// 2. Update all nodes to point to new main
	// 3. Demote current main to secondary

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Failover initiated to " + targetNode.ServerName,
	})
}

// TestConnection tests connection to main server
func (h *ClusterHandler) TestConnection(c *fiber.Ctx) error {
	var req struct {
		MainServerIP  string `json:"main_server_ip"`
		ClusterSecret string `json:"cluster_secret"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request",
		})
	}

	// Test TCP connection to API port
	conn, err := net.DialTimeout("tcp", req.MainServerIP+":80", 5*time.Second)
	if err != nil {
		return c.JSON(fiber.Map{
			"success":     false,
			"message":     "Cannot connect to main server: " + err.Error(),
			"api_ok":      false,
			"db_ok":       false,
			"redis_ok":    false,
		})
	}
	conn.Close()

	// Test DB port
	dbOK := false
	conn, err = net.DialTimeout("tcp", req.MainServerIP+":5432", 5*time.Second)
	if err == nil {
		dbOK = true
		conn.Close()
	}

	// Test Redis port
	redisOK := false
	conn, err = net.DialTimeout("tcp", req.MainServerIP+":6379", 5*time.Second)
	if err == nil {
		redisOK = true
		conn.Close()
	}

	return c.JSON(fiber.Map{
		"success":  true,
		"message":  "Connection successful",
		"api_ok":   true,
		"db_ok":    dbOK,
		"redis_ok": redisOK,
	})
}

// Helper functions

func generateClusterID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "CL-" + hex.EncodeToString(bytes)
}

func generateClusterSecret() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	s := hex.EncodeToString(bytes)
	return fmt.Sprintf("%s-%s-%s-%s", s[0:4], s[4:8], s[8:12], s[12:16])
}

func getLocalIP() string {
	// Try to get from environment first
	if ip := os.Getenv("SERVER_IP"); ip != "" {
		return ip
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

func getHardwareID() string {
	// Try to get MAC address
	if mac := os.Getenv("SERVER_MAC"); mac != "" {
		return mac
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return "unknown"
	}

	for _, iface := range interfaces {
		if iface.HardwareAddr != nil && len(iface.HardwareAddr) > 0 {
			if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
				return iface.HardwareAddr.String()
			}
		}
	}
	return "unknown"
}

func getDatabaseID() string {
	var result struct {
		ID string
	}
	database.DB.Raw("SELECT system_identifier::text as id FROM pg_control_system()").Scan(&result)
	if result.ID != "" {
		return result.ID
	}
	return "unknown"
}

func (h *ClusterHandler) registerNode(clusterID, hardwareID string, role models.ClusterRole, name, ip string) {
	now := time.Now()
	node := models.ClusterNode{
		ClusterID:       clusterID,
		HardwareID:      hardwareID,
		ServerRole:      role,
		ServerName:      name,
		ServerIP:        ip,
		Status:          models.ClusterStatusOnline,
		DBSyncStatus:    models.ClusterStatusOnline,
		RedisSyncStatus: models.ClusterStatusOnline,
		Version:         getMainVersion(),
		LastHeartbeat:   &now,
		JoinedAt:        now,
		DatabaseID:      getDatabaseID(),
	}
	database.DB.Create(&node)
}

func (h *ClusterHandler) logEvent(clusterID, eventType string, nodeID uint, nodeIP, nodeRole, description, severity string) {
	event := models.ClusterEvent{
		ClusterID:   clusterID,
		EventType:   eventType,
		NodeID:      nodeID,
		NodeIP:      nodeIP,
		NodeRole:    nodeRole,
		Description: description,
		Severity:    severity,
	}
	database.DB.Create(&event)
}

func (h *ClusterHandler) joinCluster(mainIP string, req models.ClusterJoinRequest) (*models.ClusterJoinResponse, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Marshal request to JSON
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Make POST request to main server
	url := fmt.Sprintf("http://%s/api/cluster/join", mainIP)
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to main server: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}
		if json.Unmarshal(body, &errResp) == nil && errResp.Message != "" {
			return nil, fmt.Errorf("%s", errResp.Message)
		}
		return nil, fmt.Errorf("main server returned status %d", resp.StatusCode)
	}

	// Parse response
	var joinResp models.ClusterJoinResponse
	if err := json.Unmarshal(body, &joinResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	if !joinResp.Success {
		return nil, fmt.Errorf("%s", joinResp.Message)
	}

	return &joinResp, nil
}

func (h *ClusterHandler) notifyMainOfLeave(config models.ClusterConfig) {
	// Notify main server that this node is leaving
	log.Printf("Notifying main server %s of cluster leave", config.MainServerIP)
}

func (h *ClusterHandler) setupPostgreSQLMain() {
	log.Println("Setting up PostgreSQL for replication (main)...")

	// Enable replication in postgresql.conf
	// This would require modifying PostgreSQL config and restarting
	// In Docker, this is usually done via environment variables

	// Commands to run on PostgreSQL:
	// ALTER SYSTEM SET wal_level = replica;
	// ALTER SYSTEM SET max_wal_senders = 10;
	// ALTER SYSTEM SET max_replication_slots = 10;
	// SELECT pg_reload_conf();
}

func (h *ClusterHandler) setupPostgreSQLReplica(config *models.ClusterJoinResponse) {
	log.Printf("Setting up PostgreSQL replication from %s...", config.MainServerIP)

	// This would:
	// 1. Stop local PostgreSQL
	// 2. Clear data directory
	// 3. Run pg_basebackup from main
	// 4. Configure recovery.conf / standby.signal
	// 5. Start PostgreSQL in standby mode
}

func (h *ClusterHandler) setupRedisReplica(config *models.ClusterJoinResponse) {
	log.Printf("Setting up Redis replication from %s...", config.MainServerIP)

	// Configure Redis as replica:
	// REPLICAOF main_ip 6379
	cmd := exec.Command("redis-cli", "-a", config.RedisPassword,
		"REPLICAOF", config.RedisHost, fmt.Sprintf("%d", config.RedisPort))
	cmd.Run()
}

func (h *ClusterHandler) stopReplication() {
	log.Println("Stopping replication...")

	// Stop PostgreSQL replication
	// Remove standby.signal
	// Promote to standalone

	// Stop Redis replication
	cmd := exec.Command("redis-cli", "REPLICAOF", "NO", "ONE")
	cmd.Run()
}

// HandlePromote handles a request to promote this server to main (called during failover)
func (h *ClusterHandler) HandlePromote(c *fiber.Ctx) error {
	var req struct {
		ClusterSecret string `json:"cluster_secret"`
		CurrentMainIP string `json:"current_main"`
		ClusterID     string `json:"cluster_id"`
		Event         string `json:"event"` // "promote_to_main" or "switchover"
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request",
		})
	}

	// Get our config
	var config models.ClusterConfig
	if database.DB.First(&config).Error != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cluster not configured on this server",
		})
	}

	// Verify cluster secret
	if req.ClusterSecret != config.ClusterSecret {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Invalid cluster secret",
		})
	}

	// Verify we are a secondary server
	if config.ServerRole != models.ClusterRoleSecondary {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "This server is not a secondary - cannot be promoted",
		})
	}

	log.Printf("Received promotion request from %s (event: %s)", req.CurrentMainIP, req.Event)

	h.logEvent(config.ClusterID, "promotion_received", 0, config.ServerIP, string(config.ServerRole),
		fmt.Sprintf("Promotion request received from %s", req.CurrentMainIP), "warning")

	// Use the services package to perform the actual failover
	go func() {
		if err := services.HandlePromoteRequest(req.ClusterSecret, req.CurrentMainIP); err != nil {
			log.Printf("Promotion failed: %v", err)
			h.logEvent(config.ClusterID, "promotion_failed", 0, config.ServerIP, string(config.ServerRole),
				fmt.Sprintf("Promotion failed: %v", err), "critical")
		}
	}()

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Promotion initiated",
	})
}

// HandleNotify handles cluster notifications (new main, config changes, etc.)
func (h *ClusterHandler) HandleNotify(c *fiber.Ctx) error {
	var req struct {
		Event         string    `json:"event"`          // "new_main", "config_update", etc.
		NewMainIP     string    `json:"new_main_ip"`
		ClusterID     string    `json:"cluster_id"`
		ClusterSecret string    `json:"cluster_secret"`
		Timestamp     time.Time `json:"timestamp"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request",
		})
	}

	// Get our config
	var config models.ClusterConfig
	if database.DB.First(&config).Error != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cluster not configured on this server",
		})
	}

	// Verify cluster secret
	if req.ClusterSecret != config.ClusterSecret {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Invalid cluster secret",
		})
	}

	log.Printf("Received cluster notification: %s", req.Event)

	switch req.Event {
	case "new_main":
		// Update our config to point to new main server
		config.MainServerIP = req.NewMainIP
		database.DB.Save(&config)

		h.logEvent(config.ClusterID, "new_main_acknowledged", 0, config.ServerIP, string(config.ServerRole),
			fmt.Sprintf("Acknowledged new main server: %s", req.NewMainIP), "info")

		// Reconfigure replication to new main
		if config.ServerRole != models.ClusterRoleMain {
			go h.reconfigureReplication(req.NewMainIP)
		}

	case "config_update":
		// Refresh config from main server
		log.Println("Config update notification received")

	default:
		log.Printf("Unknown cluster event: %s", req.Event)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Notification processed",
	})
}

// GetReplicationStatus returns PostgreSQL replication status
func (h *ClusterHandler) GetReplicationStatus(c *fiber.Ctx) error {
	pr := services.NewPostgresReplication(
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		5432,
	)

	status, err := pr.CheckReplicationStatus()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to check replication status: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    status,
	})
}

// reconfigureReplication points this replica to a new main server
func (h *ClusterHandler) reconfigureReplication(newMainIP string) {
	log.Printf("Reconfiguring replication to new main: %s", newMainIP)

	// This would need to:
	// 1. Update PostgreSQL primary_conninfo to point to new main
	// 2. Restart PostgreSQL to apply changes
	// 3. Update Redis REPLICAOF to new main

	// For Redis:
	redisPassword := os.Getenv("REDIS_PASSWORD")
	if redisPassword != "" {
		cmd := exec.Command("redis-cli", "-a", redisPassword, "REPLICAOF", newMainIP, "6379")
		cmd.Run()
	} else {
		cmd := exec.Command("redis-cli", "REPLICAOF", newMainIP, "6379")
		cmd.Run()
	}

	log.Println("Replication reconfigured to new main")
}

// GetUploads returns list of upload files for cluster sync
func (h *ClusterHandler) GetUploads(c *fiber.Ctx) error {
	var req struct {
		ClusterSecret string `json:"cluster_secret"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request",
		})
	}

	// Verify cluster secret
	var config models.ClusterConfig
	if database.DB.First(&config).Error != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cluster not configured",
		})
	}

	if req.ClusterSecret != config.ClusterSecret {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Invalid cluster secret",
		})
	}

	// Read uploads directory
	uploadsDir := "/app/uploads"
	if _, err := os.Stat(uploadsDir); os.IsNotExist(err) {
		uploadsDir = "/opt/proxpanel/uploads"
	}

	files := []fiber.Map{}
	entries, err := os.ReadDir(uploadsDir)
	if err != nil {
		return c.JSON(fiber.Map{
			"success": true,
			"data":    files,
		})
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filePath := uploadsDir + "/" + entry.Name()
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		// Base64 encode file content
		encoded := hex.EncodeToString(content)
		files = append(files, fiber.Map{
			"name":    entry.Name(),
			"content": encoded,
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    files,
	})
}

// PromoteToMain promotes this secondary server to become the main server (one-click failover)
func (h *ClusterHandler) PromoteToMain(c *fiber.Ctx) error {
	// Check if we're in recovery mode (read-only replica)
	var isInRecovery bool
	database.DB.Raw("SELECT pg_is_in_recovery()").Scan(&isInRecovery)

	if !isInRecovery {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "This server is already the primary database server",
		})
	}

	log.Println("Starting one-click promotion to main server...")

	// Step 1: Promote PostgreSQL to primary
	log.Println("Step 1: Promoting PostgreSQL to primary...")
	var promoted bool
	if err := database.DB.Raw("SELECT pg_promote()").Scan(&promoted).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to promote PostgreSQL: " + err.Error(),
		})
	}

	// Wait for promotion to complete
	time.Sleep(2 * time.Second)

	// Verify promotion succeeded
	database.DB.Raw("SELECT pg_is_in_recovery()").Scan(&isInRecovery)
	if isInRecovery {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "PostgreSQL promotion failed - still in recovery mode",
		})
	}

	log.Println("Step 2: Updating cluster configuration...")

	// Step 2: Update local cluster config to main role
	var config models.ClusterConfig
	if database.DB.First(&config).Error == nil {
		oldMainIP := config.MainServerIP
		config.ServerRole = models.ClusterRoleMain
		config.MainServerIP = config.ServerIP // We are now the main
		config.APIRole = "active"
		config.RadiusRole = "primary"
		now := time.Now()
		config.LastHeartbeat = &now
		database.DB.Save(&config)

		// Log the failover event
		h.logEvent(config.ClusterID, "failover_complete", 0, config.ServerIP, "main",
			fmt.Sprintf("Server promoted to main (previous main: %s)", oldMainIP), "warning")

		// Update cluster_nodes table - mark old main as offline, this server as main
		database.DB.Model(&models.ClusterNode{}).
			Where("server_ip = ?", oldMainIP).
			Updates(map[string]interface{}{
				"status": models.ClusterStatusOffline,
			})

		database.DB.Model(&models.ClusterNode{}).
			Where("server_ip = ?", config.ServerIP).
			Updates(map[string]interface{}{
				"server_role":   models.ClusterRoleMain,
				"status":        models.ClusterStatusOnline,
				"last_heartbeat": now,
			})
	}

	// Step 3: Stop Redis replication (become standalone)
	log.Println("Step 3: Stopping Redis replication...")
	exec.Command("docker", "exec", "proxpanel-redis", "redis-cli", "REPLICAOF", "NO", "ONE").Run()

	log.Println("Promotion complete! This server is now the main server.")

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Successfully promoted to main server. This server is now the primary.",
		"data": fiber.Map{
			"new_role":     "main",
			"database":     "primary (read-write)",
			"redis":        "standalone",
			"next_steps":   "Update MikroTik RADIUS to point to this server's IP",
		},
	})
}

// CheckMainStatus checks if the main server is reachable
func (h *ClusterHandler) CheckMainStatus(c *fiber.Ctx) error {
	var config models.ClusterConfig
	if database.DB.First(&config).Error != nil {
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"is_configured": false,
				"server_role":   "standalone",
			},
		})
	}

	// Check if we are the main server
	if config.ServerRole == models.ClusterRoleMain {
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"is_configured":  true,
				"server_role":    "main",
				"is_main_online": true,
			},
		})
	}

	// We are secondary - check if main is reachable
	mainOnline := false
	mainLastSeen := ""

	// Try to connect to main server API
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s/health", config.MainServerIP))
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			mainOnline = true
		}
	}

	// Get last heartbeat from cluster_nodes
	var mainNode models.ClusterNode
	if database.DB.Where("server_role = ?", "main").First(&mainNode).Error == nil {
		if mainNode.LastHeartbeat != nil {
			mainLastSeen = mainNode.LastHeartbeat.Format("2006-01-02 15:04:05")
		}
	}

	// Check how long main has been offline
	offlineMinutes := 0
	if !mainOnline && mainNode.LastHeartbeat != nil {
		offlineMinutes = int(time.Since(*mainNode.LastHeartbeat).Minutes())
	}

	// Check if we're a read-only replica
	var isReplica bool
	database.DB.Raw("SELECT pg_is_in_recovery()").Scan(&isReplica)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"is_configured":    true,
			"server_role":      config.ServerRole,
			"main_server_ip":   config.MainServerIP,
			"is_main_online":   mainOnline,
			"main_last_seen":   mainLastSeen,
			"offline_minutes":  offlineMinutes,
			"is_replica":       isReplica,
			"can_promote":      isReplica && !mainOnline,
			"cluster_id":       config.ClusterID,
		},
	})
}

// RecoverFromServer recovers database from an existing server (for new installations)
func (h *ClusterHandler) RecoverFromServer(c *fiber.Ctx) error {
	var req struct {
		SourceServerIP string `json:"source_server_ip"`
		RootPassword   string `json:"root_password"`
		BecomeMain     bool   `json:"become_main"` // If true, become main; if false, become secondary
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request",
		})
	}

	if req.SourceServerIP == "" || req.RootPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Source server IP and password are required",
		})
	}

	log.Printf("Starting recovery from server %s...", req.SourceServerIP)

	// Step 1: Test SSH connection to source server
	log.Println("Step 1: Testing SSH connection...")
	testCmd := exec.Command("sshpass", "-p", req.RootPassword,
		"ssh", "-o", "StrictHostKeyChecking=no", "-o", "ConnectTimeout=10",
		fmt.Sprintf("root@%s", req.SourceServerIP), "echo connected")
	if output, err := testCmd.CombinedOutput(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cannot connect to source server: " + string(output),
		})
	}

	// Step 2: Check if source server has ProISP installed
	log.Println("Step 2: Checking source server...")
	checkCmd := exec.Command("sshpass", "-p", req.RootPassword,
		"ssh", "-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("root@%s", req.SourceServerIP),
		"docker exec proxpanel-db psql -U proxpanel -d proxpanel -c 'SELECT COUNT(*) FROM subscribers'")
	if _, err := checkCmd.CombinedOutput(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Source server does not have a valid ProISP database",
		})
	}

	// Step 3: Create backup on source server
	log.Println("Step 3: Creating backup on source server...")
	backupCmd := exec.Command("sshpass", "-p", req.RootPassword,
		"ssh", "-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("root@%s", req.SourceServerIP),
		"docker exec proxpanel-db pg_dump -U proxpanel -d proxpanel -F c -f /tmp/recovery_backup.dump && docker cp proxpanel-db:/tmp/recovery_backup.dump /tmp/")
	if output, err := backupCmd.CombinedOutput(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create backup on source server: " + string(output),
		})
	}

	// Step 4: Download backup to this server
	log.Println("Step 4: Downloading backup...")
	downloadCmd := exec.Command("sshpass", "-p", req.RootPassword,
		"scp", "-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("root@%s:/tmp/recovery_backup.dump", req.SourceServerIP),
		"/tmp/recovery_backup.dump")
	if output, err := downloadCmd.CombinedOutput(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to download backup: " + string(output),
		})
	}

	// Step 5: Restore backup to local database
	log.Println("Step 5: Restoring backup...")

	// Copy backup into container
	exec.Command("docker", "cp", "/tmp/recovery_backup.dump", "proxpanel-db:/tmp/recovery_backup.dump").Run()

	// Drop and recreate database
	exec.Command("docker", "exec", "proxpanel-db", "psql", "-U", "proxpanel", "-d", "postgres", "-c", "DROP DATABASE IF EXISTS proxpanel_old").Run()
	exec.Command("docker", "exec", "proxpanel-db", "psql", "-U", "proxpanel", "-d", "postgres", "-c", "ALTER DATABASE proxpanel RENAME TO proxpanel_old").Run()
	exec.Command("docker", "exec", "proxpanel-db", "psql", "-U", "proxpanel", "-d", "postgres", "-c", "CREATE DATABASE proxpanel OWNER proxpanel").Run()

	// Restore
	restoreCmd := exec.Command("docker", "exec", "proxpanel-db", "pg_restore", "-U", "proxpanel", "-d", "proxpanel", "/tmp/recovery_backup.dump")
	if output, err := restoreCmd.CombinedOutput(); err != nil {
		// pg_restore may return non-zero even on success due to warnings
		log.Printf("pg_restore output: %s", string(output))
	}

	// Drop old database
	exec.Command("docker", "exec", "proxpanel-db", "psql", "-U", "proxpanel", "-d", "postgres", "-c", "DROP DATABASE IF EXISTS proxpanel_old").Run()

	// Step 6: Sync uploads (logo, favicon, etc.)
	log.Println("Step 6: Syncing uploads...")
	syncUploadsCmd := exec.Command("sshpass", "-p", req.RootPassword,
		"scp", "-o", "StrictHostKeyChecking=no", "-r",
		fmt.Sprintf("root@%s:/opt/proxpanel/uploads/*", req.SourceServerIP),
		"/opt/proxpanel/uploads/")
	syncUploadsCmd.Run() // Ignore errors if no uploads exist

	// Step 7: Update cluster config
	log.Println("Step 7: Updating configuration...")

	localIP := getLocalIP()
	now := time.Now()

	if req.BecomeMain {
		// This server becomes the new main
		var config models.ClusterConfig
		if database.DB.First(&config).Error == nil {
			config.ServerRole = models.ClusterRoleMain
			config.ServerIP = localIP
			config.MainServerIP = localIP
			config.IsActive = true
			config.LastHeartbeat = &now
			database.DB.Save(&config)
		}
	}

	// Step 8: Restart services to pick up new database
	log.Println("Step 8: Restarting services...")
	exec.Command("docker", "restart", "proxpanel-api").Run()
	time.Sleep(3 * time.Second)

	// Cleanup
	os.Remove("/tmp/recovery_backup.dump")
	exec.Command("sshpass", "-p", req.RootPassword,
		"ssh", "-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("root@%s", req.SourceServerIP),
		"rm -f /tmp/recovery_backup.dump").Run()

	log.Println("Recovery complete!")

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Successfully recovered data from source server",
		"data": fiber.Map{
			"source_server":     req.SourceServerIP,
			"recovered_to":      localIP,
			"role":              map[bool]string{true: "main", false: "secondary"}[req.BecomeMain],
			"next_steps":        "Refresh the page. All your data has been restored.",
		},
	})
}

// TestSourceConnection tests connection to a source server for recovery
func (h *ClusterHandler) TestSourceConnection(c *fiber.Ctx) error {
	var req struct {
		SourceServerIP string `json:"source_server_ip"`
		RootPassword   string `json:"root_password"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request",
		})
	}

	results := fiber.Map{
		"ssh_ok":      false,
		"database_ok": false,
		"has_data":    false,
		"subscribers": 0,
	}

	// Test SSH connection
	testCmd := exec.Command("sshpass", "-p", req.RootPassword,
		"ssh", "-o", "StrictHostKeyChecking=no", "-o", "ConnectTimeout=10",
		fmt.Sprintf("root@%s", req.SourceServerIP), "echo connected")
	if _, err := testCmd.CombinedOutput(); err != nil {
		return c.JSON(fiber.Map{
			"success": false,
			"message": "SSH connection failed - check IP and password",
			"data":    results,
		})
	}
	results["ssh_ok"] = true

	// Test database
	dbCmd := exec.Command("sshpass", "-p", req.RootPassword,
		"ssh", "-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("root@%s", req.SourceServerIP),
		"docker exec proxpanel-db psql -U proxpanel -d proxpanel -t -c 'SELECT COUNT(*) FROM subscribers'")
	output, err := dbCmd.CombinedOutput()
	if err != nil {
		return c.JSON(fiber.Map{
			"success": false,
			"message": "Database not accessible on source server",
			"data":    results,
		})
	}
	results["database_ok"] = true

	// Parse subscriber count
	countStr := strings.TrimSpace(string(output))
	if count, err := strconv.Atoi(countStr); err == nil {
		results["subscribers"] = count
		results["has_data"] = count > 0
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Connection successful",
		"data":    results,
	})
}

// syncUploadsFromMain downloads uploads from main server
func (h *ClusterHandler) syncUploadsFromMain(mainIP, clusterSecret string) {
	log.Printf("Syncing uploads from main server %s...", mainIP)

	// Create HTTP client
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	// Request uploads from main
	reqBody, _ := json.Marshal(map[string]string{
		"cluster_secret": clusterSecret,
	})

	url := fmt.Sprintf("http://%s/api/cluster/uploads", mainIP)
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("Failed to fetch uploads from main: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Main server returned status %d for uploads", resp.StatusCode)
		return
	}

	// Parse response
	var result struct {
		Success bool `json:"success"`
		Data    []struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		} `json:"data"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("Failed to parse uploads response: %v", err)
		return
	}

	if !result.Success || len(result.Data) == 0 {
		log.Println("No uploads to sync from main server")
		return
	}

	// Ensure uploads directory exists
	uploadsDir := "/app/uploads"
	if _, err := os.Stat(uploadsDir); os.IsNotExist(err) {
		uploadsDir = "/opt/proxpanel/uploads"
	}
	os.MkdirAll(uploadsDir, 0755)

	// Save each file
	for _, file := range result.Data {
		content, err := hex.DecodeString(file.Content)
		if err != nil {
			log.Printf("Failed to decode file %s: %v", file.Name, err)
			continue
		}

		filePath := uploadsDir + "/" + file.Name
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			log.Printf("Failed to save file %s: %v", file.Name, err)
			continue
		}

		log.Printf("Synced upload: %s", file.Name)
	}

	log.Printf("Successfully synced %d uploads from main server", len(result.Data))
}
