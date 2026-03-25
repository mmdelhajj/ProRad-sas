package mikrotik

import (
	"fmt"
	"log"
	"strings"
)

// PooledClient provides MikroTik operations using the connection pool
type PooledClient struct {
	address  string
	username string
	password string
	pool     *ConnectionPool
}

// NewPooledClient creates a new pooled client for a NAS device
func NewPooledClient(address, username, password string) *PooledClient {
	return &PooledClient{
		address:  address,
		username: username,
		password: password,
		pool:     GetPool(),
	}
}

// Execute runs a command and returns the result
func (pc *PooledClient) Execute(command string, args ...string) ([]map[string]string, error) {
	conn, err := pc.pool.Get(pc.address, pc.username, pc.password)
	if err != nil {
		return nil, err
	}

	result, err := conn.Execute(command, args...)
	if err != nil {
		// Connection might be broken, remove it
		pc.pool.Remove(conn)
		return nil, err
	}

	// Return connection to pool
	pc.pool.Put(conn)
	return result, nil
}

// GetActiveSessions gets all active PPPoE sessions
func (pc *PooledClient) GetActiveSessions() ([]ActiveSession, error) {
	results, err := pc.Execute("/ppp/active/print")
	if err != nil {
		return nil, err
	}

	var sessions []ActiveSession
	for _, r := range results {
		session := ActiveSession{
			ID:            r[".id"],
			Name:          r["name"],
			Service:       r["service"],
			CallerID:      r["caller-id"],
			Address:       r["address"],
			Uptime:        r["uptime"],
			Encoding:      r["encoding"],
			SessionID:     r["session-id"],
			LimitBytesIn:  parseInt64(r["limit-bytes-in"]),
			LimitBytesOut: parseInt64(r["limit-bytes-out"]),
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// GetActiveSession gets a specific user's session
func (pc *PooledClient) GetActiveSession(username string) (*ActiveSession, error) {
	sessions, err := pc.GetActiveSessions()
	if err != nil {
		return nil, err
	}

	for _, s := range sessions {
		if s.Name == username {
			return &s, nil
		}
	}

	return nil, fmt.Errorf("session not found for %s", username)
}

// DisconnectUser disconnects a PPPoE user
func (pc *PooledClient) DisconnectUser(username string) error {
	// First find the session
	sessions, err := pc.GetActiveSessions()
	if err != nil {
		return err
	}

	for _, s := range sessions {
		if s.Name == username {
			// Remove the session by ID
			_, err := pc.Execute("/ppp/active/remove", "=.id="+s.ID)
			return err
		}
	}

	return fmt.Errorf("user %s not found in active sessions", username)
}

// UpdateUserRateLimit updates the rate limit for an active user
func (pc *PooledClient) UpdateUserRateLimit(username string, downloadKbps, uploadKbps int) error {
	rateLimit := fmt.Sprintf("%dk/%dk", uploadKbps, downloadKbps)

	// Find the active session
	sessions, err := pc.GetActiveSessions()
	if err != nil {
		return err
	}

	for _, s := range sessions {
		if s.Name == username {
			// Update the session's rate limit
			_, err := pc.Execute("/ppp/active/set",
				"=.id="+s.ID,
				"=rate-limit="+rateLimit,
			)
			if err != nil {
				// Try queue-based approach
				return pc.updateQueueRateLimit(username, s.Address, downloadKbps, uploadKbps)
			}
			return nil
		}
	}

	return fmt.Errorf("user %s not found", username)
}

// updateQueueRateLimit updates rate limit via queue
func (pc *PooledClient) updateQueueRateLimit(username, ipAddress string, downloadKbps, uploadKbps int) error {
	rateLimit := fmt.Sprintf("%dk/%dk", uploadKbps, downloadKbps)

	// Find queue by name (username)
	results, err := pc.Execute("/queue/simple/print", "?name="+username)
	if err != nil {
		return err
	}

	if len(results) > 0 {
		queueID := results[0][".id"]
		_, err := pc.Execute("/queue/simple/set",
			"=.id="+queueID,
			"=max-limit="+rateLimit,
		)
		return err
	}

	// Try finding queue by target IP
	if ipAddress != "" {
		results, err = pc.Execute("/queue/simple/print", "?target="+ipAddress+"/32")
		if err == nil && len(results) > 0 {
			queueID := results[0][".id"]
			_, err := pc.Execute("/queue/simple/set",
				"=.id="+queueID,
				"=max-limit="+rateLimit,
			)
			return err
		}
	}

	return fmt.Errorf("queue not found for %s", username)
}

// Note: Ping and GetLiveTorch use complex result structures
// For these operations, use the original Client which handles the full response format

// GetSystemResource gets CPU, memory, disk usage
func (pc *PooledClient) GetSystemResource() (*PooledSystemResource, error) {
	results, err := pc.Execute("/system/resource/print")
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no resource data")
	}

	r := results[0]
	return &PooledSystemResource{
		Uptime:         r["uptime"],
		Version:        r["version"],
		BuildTime:      r["build-time"],
		FactorySoftware: r["factory-software"],
		FreeMemory:     parseInt64(r["free-memory"]),
		TotalMemory:    parseInt64(r["total-memory"]),
		CPUCount:       parseInt(r["cpu-count"]),
		CPUFrequency:   parseInt(r["cpu-frequency"]),
		CPULoad:        parseInt(r["cpu-load"]),
		FreeHDD:        parseInt64(r["free-hdd-space"]),
		TotalHDD:       parseInt64(r["total-hdd-space"]),
		Architecture:   r["architecture-name"],
		BoardName:      r["board-name"],
		Platform:       r["platform"],
	}, nil
}

// GetIdentity gets the router's identity
func (pc *PooledClient) GetIdentity() (string, error) {
	results, err := pc.Execute("/system/identity/print")
	if err != nil {
		return "", err
	}

	if len(results) > 0 {
		return results[0]["name"], nil
	}

	return "", fmt.Errorf("identity not found")
}

// GetAllConnectionCounts gets connection counts per source IP (simplified)
func (pc *PooledClient) GetAllConnectionCounts() (map[string]int, error) {
	results, err := pc.Execute("/ip/firewall/connection/print")
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int)

	for _, r := range results {
		srcAddr := r["src-address"]
		if srcAddr == "" {
			continue
		}

		// Extract IP from ip:port format
		if idx := strings.Index(srcAddr, ":"); idx > 0 {
			srcAddr = srcAddr[:idx]
		}

		counts[srcAddr]++
	}

	return counts, nil
}

// PooledSystemResource for GetSystemResource return type
type PooledSystemResource struct {
	Uptime          string `json:"uptime"`
	Version         string `json:"version"`
	BuildTime       string `json:"build_time"`
	FactorySoftware string `json:"factory_software"`
	FreeMemory      int64  `json:"free_memory"`
	TotalMemory     int64  `json:"total_memory"`
	CPUCount        int    `json:"cpu_count"`
	CPUFrequency    int    `json:"cpu_frequency"`
	CPULoad         int    `json:"cpu_load"`
	FreeHDD         int64  `json:"free_hdd"`
	TotalHDD        int64  `json:"total_hdd"`
	Architecture    string `json:"architecture"`
	BoardName       string `json:"board_name"`
	Platform        string `json:"platform"`
}

// Helper functions

func parseInt(s string) int {
	var i int
	fmt.Sscanf(s, "%d", &i)
	return i
}

func parseInt64(s string) int64 {
	var i int64
	fmt.Sscanf(s, "%d", &i)
	return i
}

func parseBytes(s string) int64 {
	// Parse bytes/bits from MikroTik format (e.g., "1234", "1.2kbps", "500Mbps")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	var value float64
	var unit string

	n, _ := fmt.Sscanf(s, "%f%s", &value, &unit)
	if n == 0 {
		return 0
	}

	unit = strings.ToLower(unit)
	multiplier := int64(1)

	switch {
	case strings.Contains(unit, "g"):
		multiplier = 1000000000
	case strings.Contains(unit, "m"):
		multiplier = 1000000
	case strings.Contains(unit, "k"):
		multiplier = 1000
	}

	// If it's bits per second, convert to bytes
	if strings.Contains(unit, "bps") || strings.Contains(unit, "bit") {
		return int64(value) * multiplier / 8
	}

	return int64(value) * multiplier
}

// InitializePool should be called at application startup
func InitializePool() {
	pool := GetPool()
	log.Printf("MikroTik connection pool initialized with config: MaxConns=%d, IdleTimeout=%v",
		pool.config.MaxConnections, pool.config.IdleTimeout)
}

// ShutdownPool should be called at application shutdown
func ShutdownPool() {
	if globalPool != nil {
		globalPool.Stop()
	}
}
