package services

import (
	"log"
	"sync"
	"time"

	"github.com/proisp/backend/internal/database"
)

// StaleSessionCleanupService periodically closes stale radacct sessions
// that have no interim update for a configured threshold period.
// This prevents ghost sessions from accumulating when MikroTik doesn't send STOP packets.
type StaleSessionCleanupService struct {
	staleThreshold time.Duration // How old before session is considered stale
	checkInterval  time.Duration // How often to check
	stopChan       chan struct{}
	wg             sync.WaitGroup
	mu             sync.Mutex
	isRunning      bool
}

// NewStaleSessionCleanupService creates a new stale session cleanup service
func NewStaleSessionCleanupService(staleMinutes int) *StaleSessionCleanupService {
	if staleMinutes <= 0 {
		staleMinutes = 30 // Default: 30 minutes without interim update = stale
	}
	return &StaleSessionCleanupService{
		staleThreshold: time.Duration(staleMinutes) * time.Minute,
		checkInterval:  5 * time.Minute, // Check every 5 minutes
		stopChan:       make(chan struct{}),
	}
}

// Start begins the cleanup service
func (s *StaleSessionCleanupService) Start() {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.run()

	log.Printf("StaleSessionCleanupService started (threshold: %v, interval: %v)",
		s.staleThreshold, s.checkInterval)
}

// Stop stops the cleanup service
func (s *StaleSessionCleanupService) Stop() {
	s.mu.Lock()
	if !s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = false
	s.mu.Unlock()

	close(s.stopChan)
	s.wg.Wait()
	log.Println("StaleSessionCleanupService stopped")
}

func (s *StaleSessionCleanupService) run() {
	defer s.wg.Done()

	// Run first cleanup after a short delay (let system stabilize)
	select {
	case <-time.After(2 * time.Minute):
		s.cleanup()
	case <-s.stopChan:
		return
	}

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.cleanup()
		}
	}
}

func (s *StaleSessionCleanupService) cleanup() {
	if database.DB == nil {
		return
	}

	thresholdTime := time.Now().Add(-s.staleThreshold)

	// Close stale sessions: no interim update for threshold period
	result := database.DB.Exec(`
		UPDATE radacct
		SET acctstoptime = NOW(),
		    acctterminatecause = 'Stale-Session-Cleanup'
		WHERE acctstoptime IS NULL
		AND (acctupdatetime IS NULL OR acctupdatetime < $1)
		AND (acctstarttime < $1)
	`, thresholdTime)

	if result.Error != nil {
		log.Printf("StaleSessionCleanup: Error closing stale sessions: %v", result.Error)
		return
	}

	closedCount := result.RowsAffected
	if closedCount > 0 {
		log.Printf("StaleSessionCleanup: Closed %d stale sessions (no update since %v)", closedCount, thresholdTime)
	}

	// Sync subscriber is_online status: mark offline if no active session.
	// IMPORTANT: Use the same stale threshold for last_seen as for radacct sessions.
	// QuotaSync updates last_seen every 30s for active users (both via MikroTik API
	// and via RADIUS interim-update packets). If last_seen is within the threshold
	// (default 30 min), the user is considered active regardless of radacct status.
	// This prevents StaleSessionCleanup from fighting QuotaSync after a container
	// restart when radacct is empty but MikroTik sessions are still active.
	result = database.DB.Exec(`
		UPDATE subscribers SET is_online = false
		WHERE is_online = true
		AND deleted_at IS NULL
		AND (last_seen IS NULL OR last_seen < $1)
		AND username NOT IN (
			SELECT DISTINCT username FROM radacct WHERE acctstoptime IS NULL
		)
	`, thresholdTime)

	if result.Error != nil {
		log.Printf("StaleSessionCleanup: Error syncing is_online: %v", result.Error)
		return
	}

	offlineCount := result.RowsAffected
	if offlineCount > 0 {
		log.Printf("StaleSessionCleanup: Marked %d subscribers offline (no active session)", offlineCount)
	}
}
