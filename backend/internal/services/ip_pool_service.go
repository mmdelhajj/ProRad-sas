package services

import (
	"fmt"
	"log"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/ippool"
	"github.com/proisp/backend/internal/mikrotik"
	"github.com/proisp/backend/internal/models"
)

// IPPoolService manages IP pool imports from MikroTik
type IPPoolService struct{}

// NewIPPoolService creates a new IP pool service
func NewIPPoolService() *IPPoolService {
	return &IPPoolService{}
}

// ImportPoolsFromMikrotik imports all IP pools from a MikroTik device
func (s *IPPoolService) ImportPoolsFromMikrotik(nas *models.Nas) (int, error) {
	log.Printf("IPPoolService: Importing pools from MikroTik %s (%s)", nas.Name, nas.IPAddress)

	// Connect to MikroTik
	client := mikrotik.NewClient(nas.IPAddress, nas.APIUsername, nas.APIPassword)
	if err := client.Connect(); err != nil {
		return 0, fmt.Errorf("failed to connect to MikroTik: %v", err)
	}
	defer client.Close()

	// Get all IP pools
	pools, err := client.GetIPPools()
	if err != nil {
		return 0, fmt.Errorf("failed to get IP pools: %v", err)
	}

	totalIPs := 0
	for _, pool := range pools {
		count, err := ippool.ImportPool(pool.Name, pool.Ranges, nas.ID)
		if err != nil {
			log.Printf("IPPoolService: Error importing pool %s: %v", pool.Name, err)
			continue
		}
		totalIPs += count
		log.Printf("IPPoolService: Imported %d IPs from pool %s", count, pool.Name)
	}

	log.Printf("IPPoolService: Total %d IPs imported from %s", totalIPs, nas.Name)
	return totalIPs, nil
}

// SyncActiveSessionsFromMikrotik marks IPs that are currently in use on MikroTik
func (s *IPPoolService) SyncActiveSessionsFromMikrotik(nas *models.Nas) (int, error) {
	log.Printf("IPPoolService: Syncing active sessions from MikroTik %s (%s)", nas.Name, nas.IPAddress)

	// Connect to MikroTik
	client := mikrotik.NewClient(nas.IPAddress, nas.APIUsername, nas.APIPassword)
	if err := client.Connect(); err != nil {
		return 0, fmt.Errorf("failed to connect to MikroTik: %v", err)
	}
	defer client.Close()

	// Get active PPPoE sessions
	sessions, err := client.GetAllActiveSessions()
	if err != nil {
		return 0, fmt.Errorf("failed to get active sessions: %v", err)
	}

	count := 0
	for _, session := range sessions {
		if session.Address == "" || session.Address == "<nil>" {
			continue
		}

		// Mark this IP as in use
		// ActiveSession uses: Name (username), SessionID (session ID), Address (IP)
		if err := ippool.MarkIPAsUsed(session.Address, session.Name, session.SessionID, nas.ID); err != nil {
			log.Printf("IPPoolService: Error marking IP %s: %v", session.Address, err)
			continue
		}
		count++
		log.Printf("IPPoolService: Marked IP %s as in use by %s", session.Address, session.Name)
	}

	log.Printf("IPPoolService: Synced %d active sessions from %s", count, nas.Name)
	return count, nil
}

// ImportAllPools imports pools from all active NAS devices
func (s *IPPoolService) ImportAllPools() (int, error) {
	var nasList []models.Nas
	if err := database.DB.Where("is_active = ?", true).Find(&nasList).Error; err != nil {
		return 0, fmt.Errorf("failed to get NAS list: %v", err)
	}

	totalIPs := 0
	for _, nas := range nasList {
		if nas.APIUsername == "" || nas.APIPassword == "" {
			log.Printf("IPPoolService: Skipping NAS %s - no API credentials", nas.Name)
			continue
		}

		count, err := s.ImportPoolsFromMikrotik(&nas)
		if err != nil {
			log.Printf("IPPoolService: Error importing from NAS %s: %v", nas.Name, err)
			continue
		}
		totalIPs += count
	}

	return totalIPs, nil
}

// SyncAllActiveSessions syncs active sessions from all active NAS devices
func (s *IPPoolService) SyncAllActiveSessions() (int, error) {
	var nasList []models.Nas
	if err := database.DB.Where("is_active = ?", true).Find(&nasList).Error; err != nil {
		return 0, fmt.Errorf("failed to get NAS list: %v", err)
	}

	totalSessions := 0
	for _, nas := range nasList {
		if nas.APIUsername == "" || nas.APIPassword == "" {
			log.Printf("IPPoolService: Skipping NAS %s - no API credentials", nas.Name)
			continue
		}

		count, err := s.SyncActiveSessionsFromMikrotik(&nas)
		if err != nil {
			log.Printf("IPPoolService: Error syncing from NAS %s: %v", nas.Name, err)
			continue
		}
		totalSessions += count
	}

	return totalSessions, nil
}

// GetAllPoolStats returns statistics for all pools
func (s *IPPoolService) GetAllPoolStats() ([]map[string]interface{}, error) {
	return ippool.Manager.GetAllPoolStats()
}

// Global instance
var IPPool = NewIPPoolService()

// Helper functions to be called from handlers

// AllocateIPForUser allocates an IP for a user from the specified pool
func AllocateIPForUser(poolName, username string, subscriberID uint, nasID uint, sessionID string) (string, error) {
	return ippool.AllocateIPForUser(poolName, username, subscriberID, nasID, sessionID)
}

// ReleaseIPForUser releases IPs assigned to a specific username
func ReleaseIPForUser(username string) error {
	return ippool.ReleaseIPForUser(username)
}

// ReleaseIP releases a specific IP back to the pool
func ReleaseIP(ipAddress string) error {
	return ippool.ReleaseIP(ipAddress)
}
