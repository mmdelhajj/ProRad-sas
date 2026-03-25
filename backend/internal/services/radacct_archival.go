package services

import (
	"log"
	"sync"
	"time"

	"github.com/proisp/backend/internal/database"
)

// RadAcctArchivalService manages archival of old RADIUS accounting records
type RadAcctArchivalService struct {
	retentionDays int           // Days to keep in main table
	checkInterval time.Duration // How often to check for archival
	stopChan      chan struct{}
	wg            sync.WaitGroup
	mu            sync.Mutex
	isRunning     bool
}

// NewRadAcctArchivalService creates a new archival service
func NewRadAcctArchivalService(retentionDays int) *RadAcctArchivalService {
	if retentionDays <= 0 {
		retentionDays = 90 // Default: keep 90 days
	}
	return &RadAcctArchivalService{
		retentionDays: retentionDays,
		checkInterval: 24 * time.Hour, // Check daily
		stopChan:      make(chan struct{}),
	}
}

// Start begins the archival service
func (s *RadAcctArchivalService) Start() {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.run()

	log.Printf("RadAcctArchivalService started (retention: %d days, check interval: %v)",
		s.retentionDays, s.checkInterval)
}

// Stop stops the archival service
func (s *RadAcctArchivalService) Stop() {
	s.mu.Lock()
	if !s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = false
	s.mu.Unlock()

	close(s.stopChan)
	s.wg.Wait()
	log.Println("RadAcctArchivalService stopped")
}

func (s *RadAcctArchivalService) run() {
	defer s.wg.Done()

	// Run immediately on startup (at 3 AM to avoid peak hours)
	s.scheduleFirstRun()

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.archiveOldRecords()
		}
	}
}

// scheduleFirstRun waits until 3 AM to run first archival
func (s *RadAcctArchivalService) scheduleFirstRun() {
	now := time.Now()
	next3AM := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
	if now.After(next3AM) {
		next3AM = next3AM.Add(24 * time.Hour)
	}

	waitDuration := next3AM.Sub(now)
	if waitDuration > 0 && waitDuration < 24*time.Hour {
		log.Printf("RadAcctArchivalService: First run scheduled at %v", next3AM)
		select {
		case <-time.After(waitDuration):
			s.archiveOldRecords()
		case <-s.stopChan:
			return
		}
	}
}

// archiveOldRecords moves old records to archive table
func (s *RadAcctArchivalService) archiveOldRecords() {
	if database.DB == nil {
		return
	}

	cutoffDate := time.Now().AddDate(0, 0, -s.retentionDays)
	log.Printf("RadAcctArchivalService: Archiving records older than %v", cutoffDate)

	// Ensure archive table exists
	if err := s.ensureArchiveTable(); err != nil {
		log.Printf("RadAcctArchivalService: Failed to create archive table: %v", err)
		return
	}

	// Copy old records to archive table
	copySQL := `
		INSERT INTO radacct_archive
		SELECT * FROM radacct
		WHERE acctstoptime IS NOT NULL
		AND acctstoptime < $1
		ON CONFLICT (radacctid) DO NOTHING
	`

	result := database.DB.Exec(copySQL, cutoffDate)
	if result.Error != nil {
		log.Printf("RadAcctArchivalService: Failed to copy records to archive: %v", result.Error)
		return
	}

	copiedRows := result.RowsAffected
	if copiedRows > 0 {
		log.Printf("RadAcctArchivalService: Copied %d records to archive", copiedRows)
	}

	// Delete archived records from main table
	deleteSQL := `
		DELETE FROM radacct
		WHERE acctstoptime IS NOT NULL
		AND acctstoptime < $1
	`

	result = database.DB.Exec(deleteSQL, cutoffDate)
	if result.Error != nil {
		log.Printf("RadAcctArchivalService: Failed to delete archived records: %v", result.Error)
		return
	}

	deletedRows := result.RowsAffected
	if deletedRows > 0 {
		log.Printf("RadAcctArchivalService: Deleted %d archived records from main table", deletedRows)
	}

	// Clean up radpostauth (authentication logs) — keep 30 days
	postAuthCutoff := time.Now().AddDate(0, 0, -30)
	postAuthResult := database.DB.Exec("DELETE FROM radpostauth WHERE authdate < $1", postAuthCutoff)
	if postAuthResult.Error == nil && postAuthResult.RowsAffected > 0 {
		log.Printf("RadAcctArchivalService: Deleted %d old radpostauth entries (>30 days)", postAuthResult.RowsAffected)
	}

	// Clean up radius_logs — keep 30 days
	radiusLogResult := database.DB.Exec("DELETE FROM radius_logs WHERE created_at < $1", postAuthCutoff)
	if radiusLogResult.Error == nil && radiusLogResult.RowsAffected > 0 {
		log.Printf("RadAcctArchivalService: Deleted %d old radius_logs entries (>30 days)", radiusLogResult.RowsAffected)
	}

	// Run VACUUM ANALYZE to reclaim space (async, don't block)
	go func() {
		database.DB.Exec("VACUUM ANALYZE radacct")
		if postAuthResult.RowsAffected > 0 {
			database.DB.Exec("VACUUM ANALYZE radpostauth")
		}
		if radiusLogResult.RowsAffected > 0 {
			database.DB.Exec("VACUUM ANALYZE radius_logs")
		}
	}()

	log.Printf("RadAcctArchivalService: Archival complete (copied: %d, deleted: %d)", copiedRows, deletedRows)
}

// ensureArchiveTable creates the archive table if it doesn't exist
func (s *RadAcctArchivalService) ensureArchiveTable() error {
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS radacct_archive (
			radacctid BIGSERIAL PRIMARY KEY,
			acctsessionid VARCHAR(64) NOT NULL,
			acctuniqueid VARCHAR(32),
			username VARCHAR(253),
			groupname VARCHAR(253),
			realm VARCHAR(64),
			nasipaddress VARCHAR(15),
			nasportid VARCHAR(32),
			nasporttype VARCHAR(32),
			acctstarttime TIMESTAMP,
			acctupdatetime TIMESTAMP,
			acctstoptime TIMESTAMP,
			acctinterval INTEGER,
			acctsessiontime INTEGER,
			acctauthentic VARCHAR(32),
			connectinfo_start VARCHAR(50),
			connectinfo_stop VARCHAR(50),
			acctinputoctets BIGINT,
			acctoutputoctets BIGINT,
			calledstationid VARCHAR(50),
			callingstationid VARCHAR(50),
			acctterminatecause VARCHAR(32),
			servicetype VARCHAR(32),
			framedprotocol VARCHAR(32),
			framedipaddress VARCHAR(15),
			framedipv6address VARCHAR(45),
			framedipv6prefix VARCHAR(45),
			framedinterfaceid VARCHAR(44),
			delegatedipv6prefix VARCHAR(45),
			class VARCHAR(64)
		)
	`

	if err := database.DB.Exec(createTableSQL).Error; err != nil {
		return err
	}

	// Create indexes on archive table
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_radacct_archive_username ON radacct_archive(username)",
		"CREATE INDEX IF NOT EXISTS idx_radacct_archive_stoptime ON radacct_archive(acctstoptime)",
		"CREATE INDEX IF NOT EXISTS idx_radacct_archive_nasip ON radacct_archive(nasipaddress)",
	}

	for _, idx := range indexes {
		database.DB.Exec(idx)
	}

	return nil
}

// GetArchiveStats returns statistics about the archive
func (s *RadAcctArchivalService) GetArchiveStats() map[string]interface{} {
	stats := make(map[string]interface{})

	if database.DB == nil {
		return stats
	}

	// Count main table records
	var mainCount int64
	database.DB.Raw("SELECT COUNT(*) FROM radacct").Scan(&mainCount)
	stats["main_table_records"] = mainCount

	// Count archive table records
	var archiveCount int64
	database.DB.Raw("SELECT COUNT(*) FROM radacct_archive").Scan(&archiveCount)
	stats["archive_table_records"] = archiveCount

	// Get oldest record in main table
	var oldestDate time.Time
	database.DB.Raw("SELECT MIN(acctstarttime) FROM radacct").Scan(&oldestDate)
	stats["oldest_main_record"] = oldestDate

	// Get oldest record in archive
	database.DB.Raw("SELECT MIN(acctstarttime) FROM radacct_archive").Scan(&oldestDate)
	stats["oldest_archive_record"] = oldestDate

	stats["retention_days"] = s.retentionDays

	return stats
}

// ManualArchive triggers immediate archival (for admin use)
func (s *RadAcctArchivalService) ManualArchive() {
	go s.archiveOldRecords()
}
