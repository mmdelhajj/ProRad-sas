package ippool

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

// IPPoolManager manages IP pool assignments
type IPPoolManager struct {
	mu sync.Mutex
}

// Manager is the global IP pool manager instance
var Manager = &IPPoolManager{}

// AllocateIP finds and allocates an available IP from the specified pool
// IMPORTANT: Also checks radreply table to ensure IP is not statically assigned to another user
func (m *IPPoolManager) AllocateIP(poolName, username string, subscriberID uint, nasID uint, sessionID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// Find an available IP from the pool that is NOT already assigned in radreply
	// This query joins ip_pool_assignments with radreply to exclude IPs that are statically assigned
	var assignment models.IPPoolAssignment
	result := database.DB.Raw(`
		SELECT ipa.* FROM ip_pool_assignments ipa
		WHERE ipa.pool_name = ?
		AND ipa.status = 'available'
		AND NOT EXISTS (
			SELECT 1 FROM radreply r
			WHERE r.attribute = 'Framed-IP-Address'
			AND r.value = ipa.ip_address
			AND r.username != ?
		)
		ORDER BY ipa.id
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`, poolName, username).Scan(&assignment)

	if result.Error != nil {
		return "", fmt.Errorf("failed to find available IP: %v", result.Error)
	}

	if assignment.ID == 0 {
		return "", fmt.Errorf("no available IPs in pool %s (all IPs are either in use or statically assigned in radreply)", poolName)
	}

	// Mark as in use
	subID := subscriberID
	nID := nasID
	if err := database.DB.Model(&models.IPPoolAssignment{}).
		Where("id = ?", assignment.ID).
		Updates(map[string]interface{}{
			"status":        models.IPPoolStatusInUse,
			"username":      username,
			"subscriber_id": &subID,
			"nas_id":        &nID,
			"session_id":    sessionID,
			"assigned_at":   now,
			"updated_at":    now,
		}).Error; err != nil {
		return "", fmt.Errorf("failed to allocate IP: %v", err)
	}

	log.Printf("IPPool: Allocated IP %s from pool %s to %s", assignment.IPAddress, poolName, username)
	return assignment.IPAddress, nil
}

// ReleaseIP releases an IP back to the pool
func (m *IPPoolManager) ReleaseIP(ipAddress string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	result := database.DB.Model(&models.IPPoolAssignment{}).
		Where("ip_address = ?", ipAddress).
		Updates(map[string]interface{}{
			"status":        models.IPPoolStatusAvailable,
			"username":      "",
			"subscriber_id": nil,
			"session_id":    "",
			"released_at":   now,
			"updated_at":    now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to release IP: %v", result.Error)
	}

	if result.RowsAffected > 0 {
		log.Printf("IPPool: Released IP %s back to pool", ipAddress)
	}

	return nil
}

// ReleaseIPByUsername releases IPs assigned to a specific username
func (m *IPPoolManager) ReleaseIPByUsername(username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	result := database.DB.Model(&models.IPPoolAssignment{}).
		Where("username = ? AND status = ?", username, models.IPPoolStatusInUse).
		Updates(map[string]interface{}{
			"status":        models.IPPoolStatusAvailable,
			"username":      "",
			"subscriber_id": nil,
			"session_id":    "",
			"released_at":   now,
			"updated_at":    now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to release IP for user: %v", result.Error)
	}

	if result.RowsAffected > 0 {
		log.Printf("IPPool: Released %d IPs for user %s", result.RowsAffected, username)
	}

	return nil
}

// UpdateSessionID updates the session ID for an IP assignment
func (m *IPPoolManager) UpdateSessionID(ipAddress, username, sessionID string) error {
	return database.DB.Model(&models.IPPoolAssignment{}).
		Where("ip_address = ? AND username = ?", ipAddress, username).
		Update("session_id", sessionID).Error
}

// GetIPByUsername returns the currently assigned IP for a username
func (m *IPPoolManager) GetIPByUsername(username string) (string, error) {
	var assignment models.IPPoolAssignment
	if err := database.DB.Where("username = ? AND status = ?", username, models.IPPoolStatusInUse).First(&assignment).Error; err != nil {
		return "", err
	}
	return assignment.IPAddress, nil
}

// IsIPAvailable checks if an IP is available in the pool
func (m *IPPoolManager) IsIPAvailable(ipAddress string) bool {
	var count int64
	database.DB.Model(&models.IPPoolAssignment{}).
		Where("ip_address = ? AND status = ?", ipAddress, models.IPPoolStatusAvailable).
		Count(&count)
	return count > 0
}

// GetPoolStats returns statistics for a pool
func (m *IPPoolManager) GetPoolStats(poolName string) (total int64, available int64, inUse int64, err error) {
	database.DB.Model(&models.IPPoolAssignment{}).Where("pool_name = ?", poolName).Count(&total)
	database.DB.Model(&models.IPPoolAssignment{}).Where("pool_name = ? AND status = ?", poolName, models.IPPoolStatusAvailable).Count(&available)
	database.DB.Model(&models.IPPoolAssignment{}).Where("pool_name = ? AND status = ?", poolName, models.IPPoolStatusInUse).Count(&inUse)
	return
}

// GetAllPoolStats returns statistics for all pools
func (m *IPPoolManager) GetAllPoolStats() ([]map[string]interface{}, error) {
	var results []struct {
		PoolName  string
		Total     int64
		Available int64
		InUse     int64
	}

	err := database.DB.Raw(`
		SELECT
			pool_name,
			COUNT(*) as total,
			SUM(CASE WHEN status = 'available' THEN 1 ELSE 0 END) as available,
			SUM(CASE WHEN status = 'in_use' THEN 1 ELSE 0 END) as in_use
		FROM ip_pool_assignments
		GROUP BY pool_name
		ORDER BY pool_name
	`).Scan(&results).Error

	if err != nil {
		return nil, err
	}

	var stats []map[string]interface{}
	for _, r := range results {
		stats = append(stats, map[string]interface{}{
			"pool_name": r.PoolName,
			"total":     r.Total,
			"available": r.Available,
			"in_use":    r.InUse,
		})
	}

	return stats, nil
}

// Helper functions for direct calls

// AllocateIPForUser allocates an IP for a user from the specified pool
func AllocateIPForUser(poolName, username string, subscriberID uint, nasID uint, sessionID string) (string, error) {
	return Manager.AllocateIP(poolName, username, subscriberID, nasID, sessionID)
}

// ReleaseIPForUser releases IPs assigned to a specific username
func ReleaseIPForUser(username string) error {
	return Manager.ReleaseIPByUsername(username)
}

// ReleaseIP releases a specific IP back to the pool
func ReleaseIP(ipAddress string) error {
	return Manager.ReleaseIP(ipAddress)
}

// UpdateSessionID updates the session ID for an IP assignment
func UpdateSessionID(ipAddress, username, sessionID string) error {
	return Manager.UpdateSessionID(ipAddress, username, sessionID)
}

// ImportPool imports a single pool's IP range into the database
func ImportPool(poolName, ranges string, nasID uint) (int, error) {
	// Parse ranges (can be comma-separated, e.g., "10.0.0.1-10.0.0.254,10.0.1.1-10.0.1.254")
	rangeList := strings.Split(ranges, ",")
	count := 0

	for _, r := range rangeList {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}

		ips, err := parseIPRange(r)
		if err != nil {
			log.Printf("IPPool: Error parsing range %s: %v", r, err)
			continue
		}

		for _, ip := range ips {
			// Insert or update IP
			result := database.DB.Exec(`
				INSERT INTO ip_pool_assignments (ip_address, pool_name, status, nas_id, created_at, updated_at)
				VALUES (?, ?, ?, ?, NOW(), NOW())
				ON CONFLICT (ip_address) DO UPDATE SET pool_name = ?, nas_id = ?, updated_at = NOW()
			`, ip, poolName, models.IPPoolStatusAvailable, nasID, poolName, nasID)

			if result.Error != nil {
				log.Printf("IPPool: Error inserting IP %s: %v", ip, result.Error)
				continue
			}
			count++
		}
	}

	return count, nil
}

// parseIPRange parses an IP range string like "10.0.0.1-10.0.0.254" into a list of IPs
func parseIPRange(rangeStr string) ([]string, error) {
	// Check if it's a range (contains "-")
	if strings.Contains(rangeStr, "-") {
		parts := strings.SplitN(rangeStr, "-", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid range format: %s", rangeStr)
		}

		startIP := net.ParseIP(strings.TrimSpace(parts[0]))
		endIP := net.ParseIP(strings.TrimSpace(parts[1]))

		if startIP == nil || endIP == nil {
			return nil, fmt.Errorf("invalid IP in range: %s", rangeStr)
		}

		return expandIPRange(startIP, endIP), nil
	}

	// Single IP
	ip := net.ParseIP(strings.TrimSpace(rangeStr))
	if ip == nil {
		return nil, fmt.Errorf("invalid IP: %s", rangeStr)
	}
	return []string{ip.String()}, nil
}

// expandIPRange expands an IP range into individual IPs
func expandIPRange(start, end net.IP) []string {
	var ips []string

	// Convert to 4-byte representation
	start = start.To4()
	end = end.To4()

	if start == nil || end == nil {
		return ips
	}

	// Convert to uint32 for easy comparison
	startInt := ipToUint32(start)
	endInt := ipToUint32(end)

	// Limit to prevent memory issues (max 65536 IPs per range)
	if endInt-startInt > 65536 {
		log.Printf("IPPool: Range too large, limiting to 65536 IPs")
		endInt = startInt + 65536
	}

	for i := startInt; i <= endInt; i++ {
		ips = append(ips, uint32ToIP(i).String())
	}

	return ips
}

// ipToUint32 converts an IP address to uint32
func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

// uint32ToIP converts uint32 to IP address
func uint32ToIP(n uint32) net.IP {
	return net.IPv4(byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
}

// MarkIPAsUsed marks an IP as in use (called when syncing active sessions)
func MarkIPAsUsed(ipAddress, username, sessionID string, nasID uint) error {
	now := time.Now()
	return database.DB.Model(&models.IPPoolAssignment{}).
		Where("ip_address = ?", ipAddress).
		Updates(map[string]interface{}{
			"status":      models.IPPoolStatusInUse,
			"username":    username,
			"session_id":  sessionID,
			"nas_id":      nasID,
			"assigned_at": now,
			"updated_at":  now,
		}).Error
}

// SyncWithRadreply ensures ip_pool_assignments is consistent with radreply table
// This marks IPs as "in_use" if they have a Framed-IP-Address entry in radreply
// Call this periodically or after bulk IP assignments to prevent duplicates
func SyncWithRadreply() (int64, error) {
	now := time.Now()

	// Find all IPs in radreply that are marked as "available" in ip_pool_assignments
	// and mark them as "in_use" with the username from radreply
	result := database.DB.Exec(`
		UPDATE ip_pool_assignments ipa
		SET status = 'in_use',
			username = r.username,
			updated_at = ?
		FROM radreply r
		WHERE r.attribute = 'Framed-IP-Address'
		AND r.value = ipa.ip_address
		AND ipa.status = 'available'
	`, now)

	if result.Error != nil {
		return 0, fmt.Errorf("failed to sync with radreply: %v", result.Error)
	}

	if result.RowsAffected > 0 {
		log.Printf("IPPool: Synced %d IPs from radreply (marked as in_use)", result.RowsAffected)
	}

	return result.RowsAffected, nil
}
