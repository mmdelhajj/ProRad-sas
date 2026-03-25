package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

// ClusterService manages HA cluster operations
type ClusterService struct {
	stopChan           chan struct{}
	wg                 sync.WaitGroup
	isRunning          bool
	mu                 sync.Mutex
	config             *models.ClusterConfig
	httpClient         *http.Client
	currentVersion     string
	mainVersion        string
	updatePending      bool
	updateDetectedAt   time.Time
}

// NewClusterService creates a new cluster service
func NewClusterService() *ClusterService {
	// Load current version
	version := "unknown"
	if data, err := os.ReadFile("/opt/proxpanel/VERSION"); err == nil {
		version = strings.TrimSpace(string(data))
	} else if data, err := os.ReadFile("/app/VERSION"); err == nil {
		version = strings.TrimSpace(string(data))
	}

	return &ClusterService{
		stopChan:       make(chan struct{}),
		currentVersion: version,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Start begins the cluster service
func (s *ClusterService) Start() {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = true
	s.mu.Unlock()

	// Load config
	var config models.ClusterConfig
	if err := database.DB.First(&config).Error; err != nil {
		log.Println("ClusterService: No cluster config found, running in standalone mode")
		return
	}

	if !config.IsActive || config.ServerRole == models.ClusterRoleStandalone {
		log.Println("ClusterService: Cluster not active, running in standalone mode")
		return
	}

	// Check if this database is a replica (in recovery mode)
	// If so, override the config to act as secondary regardless of what config says
	var isRecovery bool
	if err := database.DB.Raw("SELECT pg_is_in_recovery()").Scan(&isRecovery).Error; err == nil && isRecovery {
		log.Println("ClusterService: Database is in recovery mode (replica) - forcing SECONDARY role")
		config.ServerRole = models.ClusterRoleSecondary

		// IMPORTANT: The config is replicated from the main server, so hardware_id and server_ip
		// are the MAIN server's values. We need to get THIS server's real values.
		realHardwareID := getLocalHardwareID()
		realServerIP := getLocalServerIP()

		log.Printf("ClusterService: Overriding replicated config - hardware_id: %s -> %s, server_ip: %s -> %s",
			config.HardwareID, realHardwareID, config.ServerIP, realServerIP)

		config.HardwareID = realHardwareID
		config.ServerIP = realServerIP

		// Main server IP should be the original main (from replicated config)
		// Only override if it's pointing to this server (which would be wrong)
		if config.MainServerIP == "" || config.MainServerIP == realServerIP {
			// Try to get the actual main server from environment
			if mainIP := os.Getenv("MAIN_SERVER_IP"); mainIP != "" {
				config.MainServerIP = mainIP
			}
		}
	}

	s.config = &config
	log.Printf("ClusterService: Starting in %s mode", config.ServerRole)

	s.wg.Add(1)
	go s.run()
}

// Stop stops the cluster service
func (s *ClusterService) Stop() {
	s.mu.Lock()
	if !s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = false
	s.mu.Unlock()

	close(s.stopChan)
	s.wg.Wait()
	log.Println("ClusterService: Stopped")
}

// run is the main loop
func (s *ClusterService) run() {
	defer s.wg.Done()

	// Different behavior based on role
	if s.config.ServerRole == models.ClusterRoleMain {
		s.runMain()
	} else {
		s.runSecondary()
	}
}

// runMain handles main server responsibilities
func (s *ClusterService) runMain() {
	log.Println("ClusterService: Running as MAIN server")

	// Check node health every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.checkNodeHealth()
		}
	}
}

// runSecondary handles secondary server responsibilities
func (s *ClusterService) runSecondary() {
	log.Println("ClusterService: Running as SECONDARY server")

	// Send heartbeat every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Send initial heartbeat
	s.sendHeartbeat()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.sendHeartbeat()
		}
	}
}

// sendHeartbeat sends heartbeat to main server
func (s *ClusterService) sendHeartbeat() {
	if s.config == nil || s.config.MainServerIP == "" {
		return
	}

	// Gather metrics
	cpuUsage := getCPUUsage()
	memUsage := getMemoryUsage()
	diskUsage := getDiskUsage()
	dbLag := s.getReplicationLag()

	// Get server specs for capacity calculation
	cpuCores := getServerCPUCores()
	ramGB := getServerRAMGB()

	// Count subscribers
	var subscriberCount int64
	database.DB.Model(&models.Subscriber{}).Count(&subscriberCount)

	heartbeat := models.ClusterHeartbeat{
		ClusterID:        s.config.ClusterID,
		ClusterSecret:    s.config.ClusterSecret,
		HardwareID:       s.config.HardwareID,
		ServerRole:       s.config.ServerRole,
		ServerName:       s.config.ServerName,
		ServerIP:         s.config.ServerIP,
		MainServerIP:     s.config.MainServerIP,
		DatabaseID:       s.config.DatabaseID,
		DBReplicationLag: dbLag,
		CPUCores:         cpuCores,
		RAMGB:            ramGB,
		CPUUsage:         cpuUsage,
		MemoryUsage:      memUsage,
		DiskUsage:        diskUsage,
		SubscriberCount:  int(subscriberCount),
		Version:          s.currentVersion,
		Timestamp:        time.Now(),
	}

	// Send to main server
	url := fmt.Sprintf("http://%s:%d/api/cluster/heartbeat",
		s.config.MainServerIP, s.config.MainServerPort)

	jsonData, err := json.Marshal(heartbeat)
	if err != nil {
		log.Printf("ClusterService: Failed to marshal heartbeat: %v", err)
		return
	}

	resp, err := s.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("ClusterService: Failed to send heartbeat to %s: %v", s.config.MainServerIP, err)
		s.updateLocalStatus(models.ClusterStatusError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("ClusterService: Heartbeat rejected by main: %s", string(body))
		return
	}

	// Parse heartbeat response
	var heartbeatResp models.ClusterHeartbeatResponse
	if err := json.NewDecoder(resp.Body).Decode(&heartbeatResp); err == nil {
		// Check if main server has newer version
		if heartbeatResp.MainVersion != "" && heartbeatResp.MainVersion != s.currentVersion {
			if !s.updatePending {
				log.Printf("ClusterService: Main server has newer version %s (we have %s) - scheduling update",
					heartbeatResp.MainVersion, s.currentVersion)
				s.mainVersion = heartbeatResp.MainVersion
				s.updatePending = true
				s.updateDetectedAt = time.Now()
			} else {
				// Check if enough time has passed (5 minutes) for stability
				if time.Since(s.updateDetectedAt) >= 5*time.Minute {
					log.Printf("ClusterService: 5 minute stability window passed - triggering auto-update to %s", s.mainVersion)
					go s.performAutoUpdate()
				} else {
					remaining := 5*time.Minute - time.Since(s.updateDetectedAt)
					log.Printf("ClusterService: Update pending - waiting %.0f more seconds for stability", remaining.Seconds())
				}
			}
		} else if s.updatePending && heartbeatResp.MainVersion == s.currentVersion {
			// Main rolled back or we caught up somehow
			log.Printf("ClusterService: Main server version now matches ours - canceling pending update")
			s.updatePending = false
		}
	}

	// Update local status
	s.updateLocalStatus(models.ClusterStatusOnline)
	log.Printf("ClusterService: Heartbeat sent successfully (lag: %ds, version: %s)", dbLag, s.currentVersion)
}

// checkNodeHealth checks health of all nodes (main server only)
func (s *ClusterService) checkNodeHealth() {
	var nodes []models.ClusterNode
	database.DB.Where("cluster_id = ?", s.config.ClusterID).Find(&nodes)

	offlineThreshold := time.Now().Add(-2 * time.Minute)

	for _, node := range nodes {
		if node.ServerRole == models.ClusterRoleMain {
			continue // Don't check self
		}

		if node.LastHeartbeat == nil || node.LastHeartbeat.Before(offlineThreshold) {
			if node.Status != models.ClusterStatusOffline {
				log.Printf("ClusterService: Node %s (%s) is OFFLINE", node.ServerName, node.ServerIP)
				node.Status = models.ClusterStatusOffline
				database.DB.Save(&node)

				// Log event
				s.logEvent("node_offline", node.ID, node.ServerIP, string(node.ServerRole),
					fmt.Sprintf("Node %s went offline (no heartbeat for 2 minutes)", node.ServerName), "warning")

				// Check if auto-failover needed
				if s.config.AutoFailoverEnabled && node.ServerRole == models.ClusterRoleSecondary {
					// For now, just log - actual failover would need more complex logic
					log.Printf("ClusterService: Secondary node offline - failover may be needed if this was active")
				}
			}
		}
	}
}

// getReplicationLag gets PostgreSQL replication lag in seconds
func (s *ClusterService) getReplicationLag() int {
	if s.config.ServerRole == models.ClusterRoleMain {
		return 0
	}

	// Query PostgreSQL for replication lag
	var lag float64
	err := database.DB.Raw(`
		SELECT COALESCE(
			EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp()))::integer,
			0
		) as lag
	`).Scan(&lag).Error

	if err != nil {
		log.Printf("ClusterService: Failed to get replication lag: %v", err)
		return -1
	}

	return int(lag)
}

// updateLocalStatus updates local cluster config status
func (s *ClusterService) updateLocalStatus(status models.ClusterStatus) {
	now := time.Now()
	database.DB.Model(&models.ClusterConfig{}).
		Where("id = ?", s.config.ID).
		Updates(map[string]interface{}{
			"db_replication_status": status,
			"last_heartbeat":        now,
		})
}

// logEvent logs a cluster event
func (s *ClusterService) logEvent(eventType string, nodeID uint, nodeIP, nodeRole, description, severity string) {
	event := models.ClusterEvent{
		ClusterID:   s.config.ClusterID,
		EventType:   eventType,
		NodeID:      nodeID,
		NodeIP:      nodeIP,
		NodeRole:    nodeRole,
		Description: description,
		Severity:    severity,
	}
	database.DB.Create(&event)
}

// SetupPostgreSQLReplication sets up PostgreSQL streaming replication
func SetupPostgreSQLReplication(mainIP string, dbUser, dbPass, dbName string, slotName string) error {
	log.Println("ClusterService: Setting up PostgreSQL replication...")

	// This is a simplified version - in production you'd need:
	// 1. Stop PostgreSQL on replica
	// 2. Clear data directory
	// 3. Run pg_basebackup
	// 4. Configure standby.signal
	// 5. Start PostgreSQL

	// For Docker deployments, we need to:
	// 1. Configure the primary to allow replication
	// 2. Create replication slot on primary
	// 3. Configure replica to connect to primary

	// Create replication user and slot on primary (run on main server)
	createSlotSQL := fmt.Sprintf(`
		DO $$
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_replication_slots WHERE slot_name = '%s') THEN
				PERFORM pg_create_physical_replication_slot('%s');
			END IF;
		END $$;
	`, slotName, slotName)

	if err := database.DB.Exec(createSlotSQL).Error; err != nil {
		log.Printf("ClusterService: Warning creating replication slot: %v", err)
	}

	// For the replica, we need to run pg_basebackup
	// This should be done outside the Go application, typically via shell script

	log.Println("ClusterService: PostgreSQL replication slot created")
	log.Println("ClusterService: To complete setup, run on replica:")
	log.Printf("  pg_basebackup -h %s -U %s -D /var/lib/postgresql/data -Fp -Xs -P -R", mainIP, dbUser)

	return nil
}

// SetupRedisReplication sets up Redis replication
func SetupRedisReplication(mainIP string, redisPort int, redisPass string) error {
	log.Printf("ClusterService: Setting up Redis replication from %s:%d", mainIP, redisPort)

	// Build redis-cli command
	args := []string{}
	if redisPass != "" {
		args = append(args, "-a", redisPass)
	}
	args = append(args, "REPLICAOF", mainIP, strconv.Itoa(redisPort))

	cmd := exec.Command("redis-cli", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("ClusterService: Redis replication setup failed: %v - %s", err, string(output))
		return err
	}

	log.Printf("ClusterService: Redis replication setup complete: %s", strings.TrimSpace(string(output)))
	return nil
}

// StopRedisReplication stops Redis replication
func StopRedisReplication(redisPass string) error {
	log.Println("ClusterService: Stopping Redis replication")

	args := []string{}
	if redisPass != "" {
		args = append(args, "-a", redisPass)
	}
	args = append(args, "REPLICAOF", "NO", "ONE")

	cmd := exec.Command("redis-cli", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("ClusterService: Redis replication stop failed: %v - %s", err, string(output))
		return err
	}

	log.Printf("ClusterService: Redis replication stopped: %s", strings.TrimSpace(string(output)))
	return nil
}

// GetClusterStatus returns current cluster status for API
func GetClusterStatus() (*models.ClusterStatusResponse, error) {
	var config models.ClusterConfig
	if err := database.DB.First(&config).Error; err != nil {
		return &models.ClusterStatusResponse{
			IsActive:   false,
			ServerRole: models.ClusterRoleStandalone,
		}, nil
	}

	if !config.IsActive {
		return &models.ClusterStatusResponse{
			IsActive:   false,
			ServerRole: config.ServerRole,
		}, nil
	}

	// Get nodes
	var nodes []models.ClusterNode
	database.DB.Where("cluster_id = ?", config.ClusterID).Find(&nodes)

	// Count online
	onlineCount := 0
	dbOK := true
	redisOK := true
	for _, node := range nodes {
		if node.Status == models.ClusterStatusOnline {
			onlineCount++
		}
		if node.DBSyncStatus != models.ClusterStatusOnline && node.ServerRole != models.ClusterRoleMain {
			dbOK = false
		}
		if node.RedisSyncStatus != models.ClusterStatusOnline && node.ServerRole != models.ClusterRoleMain {
			redisOK = false
		}
	}

	// Get recent events
	var events []models.ClusterEvent
	database.DB.Where("cluster_id = ?", config.ClusterID).
		Order("created_at DESC").
		Limit(10).
		Find(&events)

	return &models.ClusterStatusResponse{
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
	}, nil
}

// Helper functions for system metrics

func getCPUUsage() float64 {
	// Read /proc/stat for CPU usage (Linux)
	if runtime.GOOS != "linux" {
		return 0
	}

	// First reading
	idle1, total1 := readCPUStat()
	time.Sleep(200 * time.Millisecond)
	// Second reading
	idle2, total2 := readCPUStat()

	idleDelta := float64(idle2 - idle1)
	totalDelta := float64(total2 - total1)

	if totalDelta == 0 {
		return 0
	}

	return (1.0 - idleDelta/totalDelta) * 100
}

func readCPUStat() (idle, total uint64) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				return 0, 0
			}

			for i := 1; i < len(fields); i++ {
				val, _ := strconv.ParseUint(fields[i], 10, 64)
				total += val
				if i == 4 { // idle is the 4th field
					idle = val
				}
			}
			return idle, total
		}
	}
	return 0, 0
}

func getMemoryUsage() float64 {
	if runtime.GOOS != "linux" {
		return 0
	}

	// Try host proc first (for Docker)
	data, err := os.ReadFile("/host/proc/meminfo")
	if err != nil {
		data, err = os.ReadFile("/proc/meminfo")
		if err != nil {
			return 0
		}
	}

	var memTotal, memAvailable uint64
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, _ := strconv.ParseUint(fields[1], 10, 64)

		switch fields[0] {
		case "MemTotal:":
			memTotal = value
		case "MemAvailable:":
			memAvailable = value
		}
	}

	if memTotal == 0 {
		return 0
	}

	return float64(memTotal-memAvailable) / float64(memTotal) * 100
}

func getDiskUsage() float64 {
	// Use df command for disk usage
	cmd := exec.Command("df", "/")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return 0
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return 0
	}

	// Usage percentage is in field 4, remove the % sign
	usageStr := strings.TrimSuffix(fields[4], "%")
	usage, _ := strconv.ParseFloat(usageStr, 64)
	return usage
}

// getLocalHardwareID returns this server's actual hardware ID (MAC address)
// This is needed because on a replica, the cluster_config is replicated from the main
// server and contains the main server's hardware_id, not this server's.
func getLocalHardwareID() string {
	// Try SERVER_MAC env var first
	if mac := os.Getenv("SERVER_MAC"); mac != "" {
		return mac
	}

	// Get MAC from first non-loopback interface
	interfaces, err := net.Interfaces()
	if err != nil {
		return "unknown"
	}

	for _, iface := range interfaces {
		if iface.HardwareAddr != nil && len(iface.HardwareAddr) > 0 {
			// Skip loopback, docker, and virtual interfaces
			if iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			if strings.HasPrefix(iface.Name, "docker") ||
				strings.HasPrefix(iface.Name, "veth") ||
				strings.HasPrefix(iface.Name, "br-") ||
				strings.HasPrefix(iface.Name, "lo") {
				continue
			}
			if iface.Flags&net.FlagUp != 0 {
				return iface.HardwareAddr.String()
			}
		}
	}
	return "unknown"
}

// getLocalServerIP returns this server's actual IP address
// This is needed because on a replica, the cluster_config contains the main server's IP.
func getLocalServerIP() string {
	// Try SERVER_IP env var first
	if ip := os.Getenv("SERVER_IP"); ip != "" {
		return ip
	}

	// Get IP from first non-loopback interface
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		// Skip loopback, docker, and virtual interfaces
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if strings.HasPrefix(iface.Name, "docker") ||
			strings.HasPrefix(iface.Name, "veth") ||
			strings.HasPrefix(iface.Name, "br-") ||
			strings.HasPrefix(iface.Name, "lo") {
			continue
		}
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Skip IPv6 and loopback
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			return ip.String()
		}
	}
	return ""
}

// performAutoUpdate downloads and installs update from license server
func (s *ClusterService) performAutoUpdate() {
	s.mu.Lock()
	if !s.updatePending {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	log.Printf("ClusterService: Starting auto-update to version %s", s.mainVersion)

	// Get license key from environment or database
	licenseKey := os.Getenv("LICENSE_KEY")
	if licenseKey == "" {
		var pref struct {
			Value string
		}
		if err := database.DB.Table("system_preferences").
			Where("key = ?", "license_key").
			First(&pref).Error; err == nil {
			licenseKey = pref.Value
		}
	}

	if licenseKey == "" {
		log.Println("ClusterService: Auto-update failed - no license key found")
		s.updatePending = false
		return
	}

	// Get license server URL
	licenseServer := os.Getenv("LICENSE_SERVER")
	if licenseServer == "" {
		licenseServer = "https://license.proxrad.com"
	}

	// Check for update from license server (POST request)
	checkURL := fmt.Sprintf("%s/api/v1/update/check", licenseServer)
	checkBody := map[string]string{
		"license_key":     licenseKey,
		"current_version": s.currentVersion,
	}
	checkBodyJSON, _ := json.Marshal(checkBody)

	resp, err := s.httpClient.Post(checkURL, "application/json", bytes.NewBuffer(checkBodyJSON))
	if err != nil {
		log.Printf("ClusterService: Auto-update failed - cannot reach license server: %v", err)
		s.updatePending = false
		return
	}
	defer resp.Body.Close()

	var updateInfo struct {
		UpdateAvailable bool   `json:"update_available"`
		Version         string `json:"version"`
		DownloadURL     string `json:"download_url"`
		Checksum        string `json:"checksum"`
		FileName        string `json:"file_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&updateInfo); err != nil {
		log.Printf("ClusterService: Auto-update failed - invalid response: %v", err)
		s.updatePending = false
		return
	}

	if !updateInfo.UpdateAvailable {
		log.Println("ClusterService: No update available from license server")
		s.updatePending = false
		return
	}

	// Build download URL if not provided
	if updateInfo.DownloadURL == "" {
		updateInfo.DownloadURL = fmt.Sprintf("%s/api/v1/update/download/%s?license_key=%s",
			licenseServer, updateInfo.Version, licenseKey)
	}

	log.Printf("ClusterService: Downloading update %s from %s", updateInfo.Version, updateInfo.DownloadURL)

	// Download update
	downloadResp, err := http.Get(updateInfo.DownloadURL)
	if err != nil {
		log.Printf("ClusterService: Auto-update failed - download error: %v", err)
		s.updatePending = false
		return
	}
	defer downloadResp.Body.Close()

	// Save to temp file
	tmpFile := fmt.Sprintf("/tmp/proxpanel-update-%s.tar.gz", updateInfo.Version)
	outFile, err := os.Create(tmpFile)
	if err != nil {
		log.Printf("ClusterService: Auto-update failed - cannot create temp file: %v", err)
		s.updatePending = false
		return
	}

	_, err = io.Copy(outFile, downloadResp.Body)
	outFile.Close()
	if err != nil {
		log.Printf("ClusterService: Auto-update failed - download incomplete: %v", err)
		os.Remove(tmpFile)
		s.updatePending = false
		return
	}

	log.Printf("ClusterService: Update downloaded, extracting...")

	// Extract update
	extractDir := "/tmp/proxpanel-update-extract"
	os.RemoveAll(extractDir)
	os.MkdirAll(extractDir, 0755)

	cmd := exec.Command("tar", "-xzf", tmpFile, "-C", extractDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("ClusterService: Auto-update failed - extract error: %v - %s", err, string(output))
		os.Remove(tmpFile)
		s.updatePending = false
		return
	}

	// Find the extracted directory
	entries, _ := os.ReadDir(extractDir)
	if len(entries) == 0 {
		log.Println("ClusterService: Auto-update failed - empty archive")
		os.Remove(tmpFile)
		os.RemoveAll(extractDir)
		s.updatePending = false
		return
	}

	updateDir := fmt.Sprintf("%s/%s", extractDir, entries[0].Name())

	// Copy files to /opt/proxpanel
	log.Println("ClusterService: Installing update...")

	// Copy API binary
	apiSrc := fmt.Sprintf("%s/backend/proisp-api/proisp-api", updateDir)
	apiDst := "/opt/proxpanel/backend/proisp-api/proisp-api"
	if _, err := os.Stat(apiSrc); err == nil {
		exec.Command("cp", "-f", apiSrc, apiDst).Run()
		exec.Command("chmod", "+x", apiDst).Run()
	}

	// Copy RADIUS binary
	radiusSrc := fmt.Sprintf("%s/backend/proisp-radius/proisp-radius", updateDir)
	radiusDst := "/opt/proxpanel/backend/proisp-radius/proisp-radius"
	if _, err := os.Stat(radiusSrc); err == nil {
		exec.Command("cp", "-f", radiusSrc, radiusDst).Run()
		exec.Command("chmod", "+x", radiusDst).Run()
	}

	// Copy frontend - clear contents but keep directory (Docker bind mount fix)
	frontendSrc := fmt.Sprintf("%s/frontend/dist", updateDir)
	if _, err := os.Stat(frontendSrc); err == nil {
		// Don't delete the directory - just clear its contents
		// This preserves the Docker bind mount inode
		exec.Command("sh", "-c", "rm -rf /opt/proxpanel/frontend/dist/*").Run()
		exec.Command("sh", "-c", fmt.Sprintf("cp -r %s/* /opt/proxpanel/frontend/dist/", frontendSrc)).Run()
	}

	// Copy nginx.conf
	nginxSrc := fmt.Sprintf("%s/frontend/nginx.conf", updateDir)
	nginxDst := "/opt/proxpanel/frontend/nginx.conf"
	if _, err := os.Stat(nginxSrc); err == nil {
		exec.Command("cp", "-f", nginxSrc, nginxDst).Run()
	}

	// Update VERSION file
	versionFile := fmt.Sprintf("%s/VERSION", updateDir)
	if _, err := os.Stat(versionFile); err == nil {
		exec.Command("cp", "-f", versionFile, "/opt/proxpanel/VERSION").Run()
	}

	// Cleanup
	os.Remove(tmpFile)
	os.RemoveAll(extractDir)

	log.Printf("ClusterService: Update installed successfully! Restarting services...")

	// Log cluster event
	s.logEvent("cluster_update", 0, s.config.ServerIP, string(s.config.ServerRole),
		fmt.Sprintf("Auto-updated from %s to %s (synced from main)", s.currentVersion, updateInfo.Version), "info")

	// Restart containers via Docker API
	s.restartContainers()

	s.updatePending = false
}

// restartContainers restarts Docker containers after update
func (s *ClusterService) restartContainers() {
	// Try Docker socket API first
	socketPath := "/var/run/docker.sock"
	if _, err := os.Stat(socketPath); err == nil {
		containers := []string{"proxpanel-api", "proxpanel-radius", "proxpanel-frontend"}

		for _, container := range containers {
			url := fmt.Sprintf("http://localhost/containers/%s/restart?t=5", container)
			req, _ := http.NewRequest("POST", url, nil)

			client := &http.Client{
				Transport: &http.Transport{
					DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
						return net.Dial("unix", socketPath)
					},
				},
				Timeout: 30 * time.Second,
			}

			resp, err := client.Do(req)
			if err != nil {
				log.Printf("ClusterService: Failed to restart %s via Docker API: %v", container, err)
			} else {
				resp.Body.Close()
				if resp.StatusCode == 204 || resp.StatusCode == 200 {
					log.Printf("ClusterService: Restarted %s", container)
				}
			}
		}

		// Wait for frontend to be ready, then reload nginx
		time.Sleep(3 * time.Second)
		s.reloadNginx()
		return
	}

	// Fallback to docker-compose
	log.Println("ClusterService: Docker socket not available, trying docker-compose...")
	cmd := exec.Command("docker-compose", "-f", "/opt/proxpanel/docker-compose.yml", "restart")
	cmd.Dir = "/opt/proxpanel"
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("ClusterService: docker-compose restart failed: %v - %s", err, string(output))

		// Final fallback - create flag file for systemd watcher
		os.WriteFile("/opt/proxpanel/.update-complete", []byte(time.Now().String()), 0644)
		log.Println("ClusterService: Created .update-complete flag for systemd watcher")
	}

	// Reload nginx after docker-compose restart
	time.Sleep(3 * time.Second)
	s.reloadNginx()
}

// reloadNginx reloads nginx config inside frontend container to pick up new files
func (s *ClusterService) reloadNginx() {
	socketPath := "/var/run/docker.sock"
	if _, err := os.Stat(socketPath); err != nil {
		return
	}

	// Execute nginx reload inside frontend container via Docker API
	execURL := "http://localhost/containers/proxpanel-frontend/exec"
	execBody := map[string]interface{}{
		"Cmd":          []string{"nginx", "-s", "reload"},
		"AttachStdout": false,
		"AttachStderr": false,
		"Detach":       true,
	}
	execJSON, _ := json.Marshal(execBody)

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
		Timeout: 10 * time.Second,
	}

	req, _ := http.NewRequest("POST", execURL, bytes.NewBuffer(execJSON))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("ClusterService: Failed to create nginx reload exec: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		log.Printf("ClusterService: Nginx reload exec creation failed with status %d", resp.StatusCode)
		return
	}

	// Parse exec ID
	var execResp struct {
		ID string `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&execResp); err != nil {
		log.Printf("ClusterService: Failed to parse exec response: %v", err)
		return
	}

	// Start the exec
	startURL := fmt.Sprintf("http://localhost/exec/%s/start", execResp.ID)
	startBody := map[string]bool{"Detach": true}
	startJSON, _ := json.Marshal(startBody)

	startReq, _ := http.NewRequest("POST", startURL, bytes.NewBuffer(startJSON))
	startReq.Header.Set("Content-Type", "application/json")

	startResp, err := client.Do(startReq)
	if err != nil {
		log.Printf("ClusterService: Failed to start nginx reload: %v", err)
		return
	}
	startResp.Body.Close()

	log.Println("ClusterService: Nginx reloaded successfully")
}

// getServerCPUCores returns the number of CPU cores
func getServerCPUCores() int {
	// Try host's cpuinfo first (for Docker containers)
	cpuinfoPath := "/host/proc/cpuinfo"
	if _, err := os.Stat(cpuinfoPath); os.IsNotExist(err) {
		cpuinfoPath = "/proc/cpuinfo"
	}

	data, err := os.ReadFile(cpuinfoPath)
	if err != nil {
		return 4 // Default fallback
	}

	cores := 0
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "processor") {
			cores++
		}
	}

	if cores == 0 {
		return 4 // Default fallback
	}
	return cores
}

// getServerRAMGB returns total RAM in gigabytes
func getServerRAMGB() int {
	// Try host's meminfo first (for Docker containers)
	meminfoPath := "/host/proc/meminfo"
	if _, err := os.Stat(meminfoPath); os.IsNotExist(err) {
		meminfoPath = "/proc/meminfo"
	}

	data, err := os.ReadFile(meminfoPath)
	if err != nil {
		return 8 // Default fallback
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseUint(fields[1], 10, 64)
				return int(kb / 1024 / 1024) // Convert KB to GB
			}
		}
	}

	return 8 // Default fallback
}
