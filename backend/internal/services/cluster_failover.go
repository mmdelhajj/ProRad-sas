package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

// ClusterFailover handles automatic and manual failover
type ClusterFailover struct {
	config              *models.ClusterConfig
	checkInterval       time.Duration
	failoverThreshold   time.Duration // How long main must be down before failover
	stopChan            chan struct{}
	wg                  sync.WaitGroup
	mu                  sync.Mutex
	isRunning           bool
	lastMainHeartbeat   time.Time
	failoverInProgress  bool
	httpClient          *http.Client
}

// NewClusterFailover creates a new failover manager
func NewClusterFailover() *ClusterFailover {
	return &ClusterFailover{
		checkInterval:     30 * time.Second,
		failoverThreshold: 2 * time.Minute, // Failover after 2 min no heartbeat
		stopChan:          make(chan struct{}),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Start begins failover monitoring (only on secondary servers)
func (cf *ClusterFailover) Start() {
	cf.mu.Lock()
	if cf.isRunning {
		cf.mu.Unlock()
		return
	}

	// Load config
	var config models.ClusterConfig
	if err := database.DB.First(&config).Error; err != nil {
		log.Println("ClusterFailover: No cluster config, not starting")
		cf.mu.Unlock()
		return
	}

	// Check if this database is a replica (in recovery mode)
	// If so, we're actually a secondary regardless of what the replicated config says
	var isRecovery bool
	if err := database.DB.Raw("SELECT pg_is_in_recovery()").Scan(&isRecovery).Error; err == nil && isRecovery {
		log.Println("ClusterFailover: Database is in recovery mode (replica) - treating as SECONDARY")
		config.ServerRole = models.ClusterRoleSecondary
	}

	// Only run on secondary servers
	if config.ServerRole != models.ClusterRoleSecondary {
		log.Printf("ClusterFailover: Not a secondary server (role: %s), not starting", config.ServerRole)
		cf.mu.Unlock()
		return
	}

	if !config.AutoFailoverEnabled {
		log.Println("ClusterFailover: Auto-failover disabled, not starting")
		cf.mu.Unlock()
		return
	}

	cf.config = &config
	cf.isRunning = true
	cf.lastMainHeartbeat = time.Now()
	cf.mu.Unlock()

	cf.wg.Add(1)
	go cf.monitorLoop()

	log.Printf("ClusterFailover: Started monitoring main server %s (threshold: %v)",
		config.MainServerIP, cf.failoverThreshold)
}

// Stop stops the failover monitor
func (cf *ClusterFailover) Stop() {
	cf.mu.Lock()
	if !cf.isRunning {
		cf.mu.Unlock()
		return
	}
	cf.isRunning = false
	cf.mu.Unlock()

	close(cf.stopChan)
	cf.wg.Wait()
	log.Println("ClusterFailover: Stopped")
}

// monitorLoop continuously checks main server health
func (cf *ClusterFailover) monitorLoop() {
	defer cf.wg.Done()

	ticker := time.NewTicker(cf.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-cf.stopChan:
			return
		case <-ticker.C:
			cf.checkMainServer()
		}
	}
}

// checkMainServer pings the main server
func (cf *ClusterFailover) checkMainServer() {
	if cf.config == nil || cf.config.MainServerIP == "" {
		return
	}

	// Try to reach main server API
	url := fmt.Sprintf("http://%s:%d/health",
		cf.config.MainServerIP, cf.config.MainServerPort)

	resp, err := cf.httpClient.Get(url)
	if err != nil {
		log.Printf("ClusterFailover: Main server unreachable: %v", err)
		cf.handleMainDown()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		cf.mu.Lock()
		cf.lastMainHeartbeat = time.Now()
		cf.mu.Unlock()
	} else {
		log.Printf("ClusterFailover: Main server returned status %d", resp.StatusCode)
		cf.handleMainDown()
	}
}

// handleMainDown is called when main server is not responding
func (cf *ClusterFailover) handleMainDown() {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	if cf.failoverInProgress {
		return
	}

	timeSinceLastHeartbeat := time.Since(cf.lastMainHeartbeat)

	if timeSinceLastHeartbeat >= cf.failoverThreshold {
		log.Printf("ClusterFailover: Main server down for %v, initiating failover!", timeSinceLastHeartbeat)
		cf.failoverInProgress = true
		go cf.performFailover()
	} else {
		log.Printf("ClusterFailover: Main server down for %v (threshold: %v)",
			timeSinceLastHeartbeat, cf.failoverThreshold)
	}
}

// performFailover executes the failover process
func (cf *ClusterFailover) performFailover() {
	log.Println("=== CLUSTER FAILOVER STARTING ===")

	// Log event
	cf.logEvent("failover_started", "Auto-failover initiated due to main server failure")

	// Step 1: Check replication status
	log.Println("ClusterFailover: Step 1 - Checking replication status...")
	lagSeconds := GetReplicationLagSeconds()
	if lagSeconds > 30 {
		log.Printf("ClusterFailover: WARNING - Replication lag is %d seconds, data loss possible", lagSeconds)
	}

	// Step 2: Promote PostgreSQL
	log.Println("ClusterFailover: Step 2 - Promoting PostgreSQL to primary...")
	pr := NewPostgresReplication(
		cf.config.MainServerIP,
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		5432,
	)

	if err := pr.PromoteToMain(); err != nil {
		log.Printf("ClusterFailover: ERROR promoting PostgreSQL: %v", err)
		cf.logEvent("failover_failed", fmt.Sprintf("PostgreSQL promotion failed: %v", err))
		cf.failoverInProgress = false
		return
	}

	// Step 3: Stop Redis replication
	log.Println("ClusterFailover: Step 3 - Stopping Redis replication...")
	StopRedisReplication(os.Getenv("REDIS_PASSWORD"))

	// Step 4: Update cluster config
	log.Println("ClusterFailover: Step 4 - Updating cluster configuration...")
	cf.config.ServerRole = models.ClusterRoleMain
	cf.config.APIRole = "active"
	cf.config.RadiusRole = "primary"
	cf.config.MainServerIP = cf.config.ServerIP
	now := time.Now()
	cf.config.LastHeartbeat = &now
	database.DB.Save(cf.config)

	// Step 5: Update node status in database
	log.Println("ClusterFailover: Step 5 - Updating node statuses...")
	// Mark old main as offline
	database.DB.Model(&models.ClusterNode{}).
		Where("server_ip = ?", cf.config.MainServerIP).
		Update("status", models.ClusterStatusOffline)

	// Mark self as main
	database.DB.Model(&models.ClusterNode{}).
		Where("hardware_id = ?", cf.config.HardwareID).
		Updates(map[string]interface{}{
			"server_role": models.ClusterRoleMain,
			"status":      models.ClusterStatusOnline,
		})

	// Step 6: Notify other nodes
	log.Println("ClusterFailover: Step 6 - Notifying other cluster nodes...")
	cf.notifyNodesOfNewMain()

	// Step 7: Update RADIUS configuration
	log.Println("ClusterFailover: Step 7 - Updating RADIUS configuration...")
	cf.updateRADIUSConfig()

	cf.logEvent("failover_completed", fmt.Sprintf("Failover complete. New main: %s", cf.config.ServerIP))

	log.Println("=== CLUSTER FAILOVER COMPLETE ===")
	log.Printf("This server (%s) is now the MAIN server", cf.config.ServerIP)

	cf.failoverInProgress = false
}

// notifyNodesOfNewMain informs other nodes about the new main server
func (cf *ClusterFailover) notifyNodesOfNewMain() {
	var nodes []models.ClusterNode
	database.DB.Where("cluster_id = ? AND hardware_id != ?",
		cf.config.ClusterID, cf.config.HardwareID).Find(&nodes)

	notification := map[string]interface{}{
		"event":         "new_main",
		"new_main_ip":   cf.config.ServerIP,
		"cluster_id":    cf.config.ClusterID,
		"cluster_secret": cf.config.ClusterSecret,
		"timestamp":     time.Now(),
	}

	jsonData, _ := json.Marshal(notification)

	for _, node := range nodes {
		url := fmt.Sprintf("http://%s:8080/api/cluster/notify", node.ServerIP)
		resp, err := cf.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("ClusterFailover: Failed to notify %s: %v", node.ServerIP, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			log.Printf("ClusterFailover: Notified %s of new main", node.ServerIP)
		}
	}
}

// updateRADIUSConfig updates RADIUS to work with new main server
func (cf *ClusterFailover) updateRADIUSConfig() {
	// Restart RADIUS service to pick up new configuration
	cmd := exec.Command("docker", "restart", "proisp-radius")
	if err := cmd.Run(); err != nil {
		log.Printf("ClusterFailover: Warning - failed to restart RADIUS: %v", err)
	} else {
		log.Println("ClusterFailover: RADIUS restarted successfully")
	}
}

// logEvent logs a failover event
func (cf *ClusterFailover) logEvent(eventType, description string) {
	event := models.ClusterEvent{
		ClusterID:   cf.config.ClusterID,
		EventType:   eventType,
		NodeIP:      cf.config.ServerIP,
		NodeRole:    string(cf.config.ServerRole),
		Description: description,
		Severity:    "critical",
	}
	database.DB.Create(&event)
}

// ManualFailover triggers failover to a specific node
func ManualFailover(targetNodeID uint) error {
	log.Printf("ManualFailover: Starting failover to node %d...", targetNodeID)

	var config models.ClusterConfig
	if err := database.DB.First(&config).Error; err != nil {
		return fmt.Errorf("cluster not configured")
	}

	if config.ServerRole != models.ClusterRoleMain {
		return fmt.Errorf("can only initiate failover from main server")
	}

	var targetNode models.ClusterNode
	if err := database.DB.First(&targetNode, targetNodeID).Error; err != nil {
		return fmt.Errorf("target node not found")
	}

	if targetNode.Status != models.ClusterStatusOnline {
		return fmt.Errorf("target node is not online")
	}

	// Notify target node to become main
	notification := map[string]interface{}{
		"event":          "promote_to_main",
		"current_main":   config.ServerIP,
		"cluster_id":     config.ClusterID,
		"cluster_secret": config.ClusterSecret,
	}

	jsonData, _ := json.Marshal(notification)
	url := fmt.Sprintf("http://%s:8080/api/cluster/promote", targetNode.ServerIP)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to contact target node: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("target node returned status %d", resp.StatusCode)
	}

	// Log event
	event := models.ClusterEvent{
		ClusterID:   config.ClusterID,
		EventType:   "manual_failover",
		NodeID:      targetNode.ID,
		NodeIP:      targetNode.ServerIP,
		NodeRole:    string(targetNode.ServerRole),
		Description: fmt.Sprintf("Manual failover from %s to %s", config.ServerIP, targetNode.ServerIP),
		Severity:    "warning",
	}
	database.DB.Create(&event)

	log.Printf("ManualFailover: Initiated failover to %s", targetNode.ServerIP)
	return nil
}

// HandlePromoteRequest handles a request to become the new main
func HandlePromoteRequest(clusterSecret, currentMainIP string) error {
	var config models.ClusterConfig
	if err := database.DB.First(&config).Error; err != nil {
		return fmt.Errorf("cluster not configured")
	}

	if config.ClusterSecret != clusterSecret {
		return fmt.Errorf("invalid cluster secret")
	}

	log.Printf("HandlePromoteRequest: Received promotion request from %s", currentMainIP)

	// Start failover process
	cf := NewClusterFailover()
	cf.config = &config
	cf.failoverInProgress = true

	go cf.performFailover()

	return nil
}

// SwitchoverToSecondary performs a planned switchover (no data loss)
func SwitchoverToSecondary(secondaryNodeID uint) error {
	log.Printf("Switchover: Starting planned switchover to node %d...", secondaryNodeID)

	var config models.ClusterConfig
	if err := database.DB.First(&config).Error; err != nil {
		return fmt.Errorf("cluster not configured")
	}

	if config.ServerRole != models.ClusterRoleMain {
		return fmt.Errorf("can only initiate switchover from main server")
	}

	var secondary models.ClusterNode
	if err := database.DB.First(&secondary, secondaryNodeID).Error; err != nil {
		return fmt.Errorf("secondary node not found")
	}

	// Step 1: Check replication is caught up
	var lagBytes int64
	database.DB.Raw(`
		SELECT pg_wal_lsn_diff(sent_lsn, replay_lsn)
		FROM pg_stat_replication
		WHERE client_addr = ?
	`, secondary.ServerIP).Scan(&lagBytes)

	if lagBytes > 1024*1024 { // 1MB lag
		return fmt.Errorf("replication lag too high (%d bytes), wait for sync", lagBytes)
	}

	// Step 2: Fence current main (stop writes)
	log.Println("Switchover: Fencing current main (stopping writes)...")
	database.DB.Exec("ALTER SYSTEM SET default_transaction_read_only = on")
	database.DB.Exec("SELECT pg_reload_conf()")

	// Step 3: Wait for replication to fully catch up
	log.Println("Switchover: Waiting for replication to sync...")
	time.Sleep(5 * time.Second)

	// Step 4: Notify secondary to promote
	notification := map[string]interface{}{
		"event":          "switchover",
		"current_main":   config.ServerIP,
		"cluster_id":     config.ClusterID,
		"cluster_secret": config.ClusterSecret,
	}

	jsonData, _ := json.Marshal(notification)
	url := fmt.Sprintf("http://%s:8080/api/cluster/promote", secondary.ServerIP)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		// Rollback fence
		database.DB.Exec("ALTER SYSTEM SET default_transaction_read_only = off")
		database.DB.Exec("SELECT pg_reload_conf()")
		return fmt.Errorf("failed to contact secondary: %v", err)
	}
	defer resp.Body.Close()

	// Step 5: Demote self to replica
	log.Println("Switchover: Demoting self to replica...")
	pr := NewPostgresReplication(
		secondary.ServerIP,
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		5432,
	)

	slotName := fmt.Sprintf("replica_%s", config.HardwareID[:16])
	pr.DemoteToReplica(secondary.ServerIP, slotName)

	// Update config
	config.ServerRole = models.ClusterRoleSecondary
	config.MainServerIP = secondary.ServerIP
	config.APIRole = "standby"
	config.RadiusRole = "backup"
	database.DB.Save(&config)

	log.Println("Switchover: Complete. This server is now a secondary.")
	return nil
}
