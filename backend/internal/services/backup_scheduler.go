package services

import (
	"archive/tar"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/proisp/backend/internal/config"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/mikrotik"
	"github.com/proisp/backend/internal/models"
)

// BackupSchedulerService handles scheduled backups
type BackupSchedulerService struct {
	cfg       *config.Config
	backupDir string
	stopChan  chan struct{}
}

// NewBackupSchedulerService creates a new backup scheduler service
func NewBackupSchedulerService(cfg *config.Config) *BackupSchedulerService {
	backupDir := "/var/backups/proisp"
	os.MkdirAll(backupDir, 0755)
	return &BackupSchedulerService{
		cfg:       cfg,
		backupDir: backupDir,
		stopChan:  make(chan struct{}),
	}
}

// Start starts the backup scheduler
func (s *BackupSchedulerService) Start() {
	log.Println("BackupSchedulerService started, checking every 1 minute")

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Initial check
	s.checkSchedules()

	for {
		select {
		case <-s.stopChan:
			log.Println("BackupSchedulerService stopped")
			return
		case <-ticker.C:
			s.checkSchedules()
		}
	}
}

// Stop stops the backup scheduler
func (s *BackupSchedulerService) Stop() {
	close(s.stopChan)
}

// checkSchedules checks all schedules and runs due backups
func (s *BackupSchedulerService) checkSchedules() {
	var schedules []models.BackupSchedule
	if err := database.DB.Where("is_enabled = ?", true).Find(&schedules).Error; err != nil {
		log.Printf("BackupScheduler: Failed to load schedules: %v", err)
		return
	}

	// Use configured timezone for time comparison
	tz := getConfiguredTimezone()
	now := time.Now().In(tz)

	for _, schedule := range schedules {
		if s.isDue(&schedule, now) {
			go s.runBackup(&schedule)
		}
	}
}

// isDue checks if a schedule is due to run
func (s *BackupSchedulerService) isDue(schedule *models.BackupSchedule, now time.Time) bool {
	// Parse time of day
	hour, minute := 2, 0 // default 02:00
	if schedule.TimeOfDay != "" {
		fmt.Sscanf(schedule.TimeOfDay, "%d:%d", &hour, &minute)
	}

	// Check if it's the right time (within 1 minute window)
	if now.Hour() != hour || now.Minute() != minute {
		return false
	}

	// Check frequency
	switch schedule.Frequency {
	case "daily":
		// Runs every day at specified time
		return true
	case "weekly":
		// Runs on specified day of week
		return int(now.Weekday()) == schedule.DayOfWeek
	case "monthly":
		// Runs on specified day of month
		return now.Day() == schedule.DayOfMonth
	}

	return false
}

// runBackup executes a scheduled backup
func (s *BackupSchedulerService) runBackup(schedule *models.BackupSchedule) {
	startTime := time.Now()

	// Update status to running
	database.DB.Model(schedule).Updates(map[string]interface{}{
		"last_status": "running",
		"last_run_at": startTime,
	})

	// MikroTik-only scheduled backup
	if schedule.BackupType == "mikrotik" {
		s.runMikroTikBackup(schedule, startTime)
		return
	}

	// Create backup log entry
	backupLog := models.BackupLog{
		ScheduleID:   &schedule.ID,
		ScheduleName: schedule.Name,
		BackupType:   schedule.BackupType,
		Status:       "running",
		StartedAt:    startTime,
	}
	database.DB.Create(&backupLog)

	// Generate filenames
	timestamp := startTime.Format("20060102_150405")
	tempFile := filepath.Join(s.backupDir, fmt.Sprintf(".temp_%s_%s_scheduled.dump", schedule.BackupType, timestamp))
	filename := fmt.Sprintf("proisp_%s_%s_scheduled.proisp.bak", schedule.BackupType, timestamp)
	localPath := filepath.Join(s.backupDir, filename)

	// Run pg_dump with custom format
	err := s.createDatabaseBackupCustomFormat(schedule.BackupType, tempFile)
	if err != nil {
		s.handleBackupError(schedule, &backupLog, err, startTime)
		return
	}

	// Encrypt the backup
	err = s.EncryptFile(tempFile, localPath)
	os.Remove(tempFile) // Clean up temp file regardless
	if err != nil {
		s.handleBackupError(schedule, &backupLog, fmt.Errorf("encryption failed: %v", err), startTime)
		return
	}

	// Get file info
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		s.handleBackupError(schedule, &backupLog, err, startTime)
		return
	}

	backupLog.Filename = filename
	backupLog.FileSize = fileInfo.Size()
	backupLog.StoragePath = localPath

	// Export MikroTik configs if enabled
	var mikrotikExportPath string
	if schedule.IncludeMikroTik {
		mikrotikPath, err := s.ExportMikroTikConfigs(timestamp)
		if err != nil {
			log.Printf("BackupScheduler: MikroTik export failed for %s: %v (continuing with DB backup)", schedule.Name, err)
		} else if mikrotikPath != "" {
			mikrotikExportPath = mikrotikPath
			log.Printf("BackupScheduler: MikroTik configs saved to %s", filepath.Base(mikrotikPath))
		}
	}

	// Upload to FTP if enabled
	if schedule.FTPEnabled && (schedule.StorageType == "ftp" || schedule.StorageType == "both") {
		err = s.uploadToFTP(schedule, localPath, filename)
		if err != nil {
			log.Printf("BackupScheduler: FTP upload failed for %s: %v", schedule.Name, err)
			// Don't fail the whole backup if FTP fails but local succeeded
			if schedule.StorageType == "ftp" {
				s.handleBackupError(schedule, &backupLog, fmt.Errorf("FTP upload failed: %v", err), startTime)
				return
			}
		} else {
			backupLog.StorageType = "both"
			backupLog.StoragePath = fmt.Sprintf("local:%s, ftp:%s/%s", localPath, schedule.FTPPath, filename)
		}
	}

	// Upload to ProxPanel Cloud if enabled (legacy CloudEnabled flag)
	if schedule.CloudEnabled {
		if err := s.uploadToProxPanelCloud(localPath, filename); err != nil {
			log.Printf("BackupScheduler: Cloud upload failed for %s: %v", schedule.Name, err)
		} else {
			log.Printf("BackupScheduler: Successfully uploaded %s to ProxPanel Cloud", filename)
		}
		// Also upload MikroTik backup to cloud if it was exported
		if mikrotikExportPath != "" {
			mtFilename := filepath.Base(mikrotikExportPath)
			if err := s.uploadToProxPanelCloud(mikrotikExportPath, mtFilename); err != nil {
				log.Printf("BackupScheduler: Cloud upload failed for MikroTik %s: %v", mtFilename, err)
			} else {
				log.Printf("BackupScheduler: Successfully uploaded MikroTik %s to ProxPanel Cloud", mtFilename)
			}
		}
	}

	// Upload to ProxPanel cloud if storage type includes cloud
	if schedule.StorageType == "cloud" || schedule.StorageType == "local+cloud" {
		if err := s.uploadToCloud(localPath, filename); err != nil {
			log.Printf("BackupScheduler: Cloud upload failed for schedule '%s': %v", schedule.Name, err)
			// If cloud-only, this is a failure
			if schedule.StorageType == "cloud" {
				// Mark as failed
				backupLog.Status = "failed"
				backupLog.ErrorMessage = fmt.Sprintf("Cloud upload failed: %v", err)
				database.DB.Save(&backupLog)
				return
			}
			// For local+cloud, log warning but don't fail
		} else {
			if backupLog.StorageType == "local" {
				backupLog.StorageType = "local+cloud"
			} else {
				backupLog.StorageType = "cloud"
			}
		}
	}

	// If cloud-only storage, remove local file after successful upload to save disk space
	if schedule.StorageType == "cloud" && backupLog.StorageType == "cloud" {
		os.Remove(localPath)
		log.Printf("BackupScheduler: Removed local copy of %s (cloud-only mode)", filename)
	}

	// Delete old backups based on retention policy
	if schedule.Retention > 0 {
		s.cleanOldBackups(schedule)
	}

	// Update schedule status
	nextRun := s.calculateNextRun(schedule)
	database.DB.Model(schedule).Updates(map[string]interface{}{
		"last_status":      "success",
		"last_error":       "",
		"last_backup_file": filename,
		"next_run_at":      nextRun,
	})

	// Complete backup log
	completedAt := time.Now()
	backupLog.Status = "success"
	backupLog.CompletedAt = completedAt
	backupLog.Duration = int(completedAt.Sub(startTime).Seconds())
	if backupLog.StorageType == "" {
		backupLog.StorageType = "local"
	}
	database.DB.Save(&backupLog)

	log.Printf("BackupScheduler: Backup completed for %s (%s, %d bytes)",
		schedule.Name, filename, fileInfo.Size())
}

// runMikroTikBackup executes a MikroTik-only scheduled backup
func (s *BackupSchedulerService) runMikroTikBackup(schedule *models.BackupSchedule, startTime time.Time) {
	timestamp := startTime.Format("20060102_150405")

	backupLog := models.BackupLog{
		ScheduleID:   &schedule.ID,
		ScheduleName: schedule.Name,
		BackupType:   "mikrotik",
		Status:       "running",
		StartedAt:    startTime,
		StorageType:  "local",
	}
	database.DB.Create(&backupLog)

	mikrotikPath, err := s.ExportMikroTikConfigs(timestamp)
	if err != nil {
		s.handleBackupError(schedule, &backupLog, fmt.Errorf("MikroTik export failed: %v", err), startTime)
		return
	}
	if mikrotikPath == "" {
		s.handleBackupError(schedule, &backupLog, fmt.Errorf("no NAS devices with API credentials found"), startTime)
		return
	}

	fileInfo, err := os.Stat(mikrotikPath)
	if err != nil {
		s.handleBackupError(schedule, &backupLog, err, startTime)
		return
	}

	filename := filepath.Base(mikrotikPath)
	backupLog.Filename = filename
	backupLog.FileSize = fileInfo.Size()
	backupLog.StoragePath = mikrotikPath

	// Upload to ProxPanel Cloud if enabled
	if schedule.CloudEnabled {
		if err := s.uploadToProxPanelCloud(mikrotikPath, filename); err != nil {
			log.Printf("BackupScheduler: Cloud upload failed for MikroTik backup %s: %v", schedule.Name, err)
		} else {
			log.Printf("BackupScheduler: Successfully uploaded MikroTik backup %s to ProxPanel Cloud", filename)
			backupLog.StorageType = "local+cloud"
		}
	}

	// Update schedule status
	nextRun := s.calculateNextRun(schedule)
	database.DB.Model(schedule).Updates(map[string]interface{}{
		"last_status":      "success",
		"last_error":       "",
		"last_backup_file": filename,
		"next_run_at":      nextRun,
	})

	completedAt := time.Now()
	backupLog.Status = "success"
	backupLog.CompletedAt = completedAt
	backupLog.Duration = int(completedAt.Sub(startTime).Seconds())
	database.DB.Save(&backupLog)

	log.Printf("BackupScheduler: MikroTik backup completed for %s (%s, %d bytes)",
		schedule.Name, filename, fileInfo.Size())
}

// RunMikroTikBackupNow runs a MikroTik-only backup manually (called by RunScheduleNow handler)
func (s *BackupSchedulerService) RunMikroTikBackupNow(schedule *models.BackupSchedule, userID uint, username string) (*models.BackupLog, error) {
	startTime := time.Now()
	timestamp := startTime.Format("20060102_150405")

	backupLog := models.BackupLog{
		ScheduleID:    &schedule.ID,
		ScheduleName:  schedule.Name,
		BackupType:    "mikrotik",
		Status:        "running",
		StartedAt:     startTime,
		StorageType:   "local",
		CreatedByID:   &userID,
		CreatedByName: username,
	}
	database.DB.Create(&backupLog)

	mikrotikPath, err := s.ExportMikroTikConfigs(timestamp)
	if err != nil {
		backupLog.Status = "failed"
		backupLog.ErrorMessage = fmt.Sprintf("MikroTik export failed: %v", err)
		backupLog.CompletedAt = time.Now()
		database.DB.Save(&backupLog)
		return &backupLog, err
	}
	if mikrotikPath == "" {
		errMsg := "no NAS devices with API credentials found"
		backupLog.Status = "failed"
		backupLog.ErrorMessage = errMsg
		backupLog.CompletedAt = time.Now()
		database.DB.Save(&backupLog)
		return &backupLog, fmt.Errorf(errMsg)
	}

	fileInfo, err := os.Stat(mikrotikPath)
	if err != nil {
		backupLog.Status = "failed"
		backupLog.ErrorMessage = err.Error()
		backupLog.CompletedAt = time.Now()
		database.DB.Save(&backupLog)
		return &backupLog, err
	}

	filename := filepath.Base(mikrotikPath)
	backupLog.Filename = filename
	backupLog.FileSize = fileInfo.Size()
	backupLog.StoragePath = mikrotikPath

	// Upload to ProxPanel Cloud if enabled
	if schedule.CloudEnabled {
		if err := s.uploadToProxPanelCloud(mikrotikPath, filename); err != nil {
			log.Printf("BackupScheduler: Cloud upload failed for MikroTik manual backup: %v", err)
		} else {
			log.Printf("BackupScheduler: Successfully uploaded MikroTik backup %s to ProxPanel Cloud", filename)
			backupLog.StorageType = "local+cloud"
		}
	}

	completedAt := time.Now()
	backupLog.Status = "success"
	backupLog.CompletedAt = completedAt
	backupLog.Duration = int(completedAt.Sub(startTime).Seconds())
	database.DB.Save(&backupLog)

	log.Printf("BackupScheduler: MikroTik manual backup completed (%s, %d bytes)", filename, fileInfo.Size())
	return &backupLog, nil
}

// createDatabaseBackup creates a database backup (legacy SQL format)
func (s *BackupSchedulerService) createDatabaseBackup(backupType, destPath string) error {
	cmd := exec.Command("pg_dump",
		"-h", s.cfg.DBHost,
		"-p", strconv.Itoa(s.cfg.DBPort),
		"-U", s.cfg.DBUser,
		"-d", s.cfg.DBName,
		"-f", destPath,
		"--no-owner",
		"--no-acl",
	)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", s.cfg.DBPassword))

	// Add table filters based on type
	if backupType == "data" {
		cmd.Args = append(cmd.Args,
			"--table=subscribers",
			"--table=services",
			"--table=nas",
			"--table=resellers",
			"--table=transactions",
			"--table=invoices",
			"--table=prepaid_cards",
			"--table=sessions",
			"--table=radcheck",
			"--table=radreply",
			"--table=radacct",
		)
	} else if backupType == "config" {
		cmd.Args = append(cmd.Args,
			"--table=users",
			"--table=settings",
			"--table=permissions",
			"--table=permission_groups",
			"--table=communication_templates",
			"--table=communication_rules",
			"--table=bandwidth_rules",
		)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err.Error(), string(output))
	}
	return nil
}

// createDatabaseBackupCustomFormat creates a database backup in custom format (compressed binary)
func (s *BackupSchedulerService) createDatabaseBackupCustomFormat(backupType, destPath string) error {
	cmd := exec.Command("pg_dump",
		"-h", s.cfg.DBHost,
		"-p", strconv.Itoa(s.cfg.DBPort),
		"-U", s.cfg.DBUser,
		"-d", s.cfg.DBName,
		"-Fc", // Custom format (compressed, binary)
		"-f", destPath,
		"--no-owner",
		"--no-acl",
	)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", s.cfg.DBPassword))

	// Add table filters based on type
	if backupType == "data" {
		cmd.Args = append(cmd.Args,
			"--table=subscribers",
			"--table=services",
			"--table=nas",
			"--table=resellers",
			"--table=transactions",
			"--table=invoices",
			"--table=prepaid_cards",
			"--table=sessions",
			"--table=radcheck",
			"--table=radreply",
			"--table=radacct",
		)
	} else if backupType == "config" {
		cmd.Args = append(cmd.Args,
			"--table=users",
			"--table=settings",
			"--table=permissions",
			"--table=permission_groups",
			"--table=communication_templates",
			"--table=communication_rules",
			"--table=bandwidth_rules",
		)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err.Error(), string(output))
	}
	return nil
}

// uploadToFTP uploads a file to FTP server
func (s *BackupSchedulerService) uploadToFTP(schedule *models.BackupSchedule, localPath, filename string) error {
	// Connect to FTP
	addr := fmt.Sprintf("%s:%d", schedule.FTPHost, schedule.FTPPort)
	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(30*time.Second))
	if err != nil {
		return fmt.Errorf("FTP connection failed: %v", err)
	}
	defer conn.Quit()

	// Login
	err = conn.Login(schedule.FTPUsername, schedule.FTPPassword)
	if err != nil {
		return fmt.Errorf("FTP login failed: %v", err)
	}

	// Change to backup directory (create if needed)
	if schedule.FTPPath != "" && schedule.FTPPath != "/" {
		// Try to change to directory, create if doesn't exist
		err = conn.ChangeDir(schedule.FTPPath)
		if err != nil {
			// Try to create directory
			conn.MakeDir(schedule.FTPPath)
			err = conn.ChangeDir(schedule.FTPPath)
			if err != nil {
				return fmt.Errorf("FTP directory change failed: %v", err)
			}
		}
	}

	// Open local file
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer file.Close()

	// Upload file
	err = conn.Stor(filename, file)
	if err != nil {
		return fmt.Errorf("FTP upload failed: %v", err)
	}

	log.Printf("BackupScheduler: Uploaded %s to FTP %s", filename, schedule.FTPHost)
	return nil
}

// uploadToProxPanelCloud uploads a backup file to ProxPanel Cloud via the license server
// Uses raw streaming with X-Filename header (matches cloud_backup_client.go Upload handler)
func (s *BackupSchedulerService) uploadToProxPanelCloud(filePath, filename string) error {
	licenseServer := os.Getenv("LICENSE_SERVER")
	if licenseServer == "" {
		licenseServer = "https://license.proxrad.com"
	}
	licenseKey := os.Getenv("LICENSE_KEY")
	if licenseKey == "" {
		return fmt.Errorf("LICENSE_KEY not set")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %v", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat backup file: %v", err)
	}

	req, err := http.NewRequest("POST", licenseServer+"/api/v1/cloud-backup/upload", file)
	if err != nil {
		return err
	}
	req.Header.Set("X-License-Key", licenseKey)
	req.Header.Set("X-Filename", filename)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = fileInfo.Size()

	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("upload request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// cleanOldBackups removes backups older than retention period (local + cloud + FTP)
func (s *BackupSchedulerService) cleanOldBackups(schedule *models.BackupSchedule) {
	cutoff := time.Now().AddDate(0, 0, -schedule.Retention)
	log.Printf("BackupScheduler: Cleaning backups older than %d days (before %s)", schedule.Retention, cutoff.Format("2006-01-02"))

	// Clean local backups
	files, err := os.ReadDir(s.backupDir)
	if err == nil {
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			info, err := file.Info()
			if err != nil {
				continue
			}
			name := file.Name()
			isBackup := strings.HasSuffix(name, ".proisp.bak") || strings.HasSuffix(name, ".sql") ||
				(strings.HasPrefix(name, "mikrotik_") && strings.HasSuffix(name, ".tar.gz"))
			if info.ModTime().Before(cutoff) && isBackup && len(name) > 10 {
				os.Remove(filepath.Join(s.backupDir, name))
				log.Printf("BackupScheduler: Deleted old local backup %s", name)
			}
		}
	}

	// Clean cloud backups
	s.cleanOldCloudBackups(cutoff)

	// Clean FTP backups if enabled
	if schedule.FTPEnabled {
		s.cleanOldFTPBackups(schedule, cutoff)
	}

	// Clean old backup_logs entries from DB
	result := database.DB.Where("completed_at < ? AND completed_at > '2000-01-01'", cutoff).Delete(&models.BackupLog{})
	if result.RowsAffected > 0 {
		log.Printf("BackupScheduler: Cleaned %d old backup log entries", result.RowsAffected)
	}
}

// cleanOldCloudBackups removes old cloud backups from the license server
func (s *BackupSchedulerService) cleanOldCloudBackups(cutoff time.Time) {
	licenseServer := os.Getenv("LICENSE_SERVER")
	if licenseServer == "" {
		licenseServer = "https://license.proxrad.com"
	}
	licenseKey := os.Getenv("LICENSE_KEY")
	if licenseKey == "" {
		return
	}

	// List cloud backups
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", licenseServer+"/api/v1/cloud-backup/list", nil)
	if err != nil {
		return
	}
	req.Header.Set("X-License-Key", licenseKey)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("BackupScheduler: Failed to list cloud backups: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return
	}

	var listResult struct {
		Success bool `json:"success"`
		Backups []struct {
			BackupID  string    `json:"backup_id"`
			Filename  string    `json:"filename"`
			CreatedAt time.Time `json:"created_at"`
		} `json:"backups"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if err := json.Unmarshal(body, &listResult); err != nil {
		return
	}

	if !listResult.Success || len(listResult.Backups) == 0 {
		return
	}

	// Delete cloud backups older than cutoff
	deletedCount := 0
	for _, backup := range listResult.Backups {
		if backup.CreatedAt.Before(cutoff) && backup.BackupID != "" {
			delReq, err := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/cloud-backup/%s", licenseServer, backup.BackupID), nil)
			if err != nil {
				continue
			}
			delReq.Header.Set("X-License-Key", licenseKey)

			delResp, err := client.Do(delReq)
			if err != nil {
				log.Printf("BackupScheduler: Failed to delete cloud backup %s: %v", backup.Filename, err)
				continue
			}
			delResp.Body.Close()

			if delResp.StatusCode == 200 || delResp.StatusCode == 204 {
				deletedCount++
				log.Printf("BackupScheduler: Deleted old cloud backup %s (%s)", backup.Filename, backup.BackupID)
			}
		}
	}
	if deletedCount > 0 {
		log.Printf("BackupScheduler: Cleaned %d old cloud backups", deletedCount)
	}
}

// cleanOldFTPBackups removes old backups from FTP server
func (s *BackupSchedulerService) cleanOldFTPBackups(schedule *models.BackupSchedule, cutoff time.Time) {
	addr := fmt.Sprintf("%s:%d", schedule.FTPHost, schedule.FTPPort)
	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(30*time.Second))
	if err != nil {
		return
	}
	defer conn.Quit()

	err = conn.Login(schedule.FTPUsername, schedule.FTPPassword)
	if err != nil {
		return
	}

	if schedule.FTPPath != "" && schedule.FTPPath != "/" {
		conn.ChangeDir(schedule.FTPPath)
	}

	entries, err := conn.List("")
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.Type == ftp.EntryTypeFile && entry.Time.Before(cutoff) {
			name := entry.Name
			isBackup := strings.HasSuffix(name, ".proisp.bak") || strings.HasSuffix(name, ".sql") ||
				(strings.HasPrefix(name, "mikrotik_") && strings.HasSuffix(name, ".tar.gz"))
			if isBackup {
				conn.Delete(name)
				log.Printf("BackupScheduler: Deleted old FTP backup %s", name)
			}
		}
	}
}

// calculateNextRun calculates the next run time for a schedule
func (s *BackupSchedulerService) calculateNextRun(schedule *models.BackupSchedule) time.Time {
	// Use configured timezone
	tz := getConfiguredTimezone()
	now := time.Now().In(tz)

	hour, minute := 2, 0
	if schedule.TimeOfDay != "" {
		fmt.Sscanf(schedule.TimeOfDay, "%d:%d", &hour, &minute)
	}

	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, tz)

	switch schedule.Frequency {
	case "daily":
		if next.Before(now) || next.Equal(now) {
			next = next.AddDate(0, 0, 1)
		}
	case "weekly":
		daysUntil := (schedule.DayOfWeek - int(now.Weekday()) + 7) % 7
		if daysUntil == 0 && (next.Before(now) || next.Equal(now)) {
			daysUntil = 7
		}
		next = next.AddDate(0, 0, daysUntil)
	case "monthly":
		next = time.Date(now.Year(), now.Month(), schedule.DayOfMonth, hour, minute, 0, 0, tz)
		if next.Before(now) || next.Equal(now) {
			next = next.AddDate(0, 1, 0)
		}
	}

	// Store as UTC so timestamp without time zone column works correctly
	return next.UTC()
}

// CalculateNextRunForSchedule calculates and updates next_run_at for a schedule (exported for use by handlers)
func CalculateNextRunForSchedule(schedule *models.BackupSchedule) time.Time {
	// Use configured timezone
	tz := getConfiguredTimezone()
	now := time.Now().In(tz)

	hour, minute := 2, 0
	if schedule.TimeOfDay != "" {
		fmt.Sscanf(schedule.TimeOfDay, "%d:%d", &hour, &minute)
	}

	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, tz)

	switch schedule.Frequency {
	case "daily":
		if next.Before(now) || next.Equal(now) {
			next = next.AddDate(0, 0, 1)
		}
	case "weekly":
		daysUntil := (schedule.DayOfWeek - int(now.Weekday()) + 7) % 7
		if daysUntil == 0 && (next.Before(now) || next.Equal(now)) {
			daysUntil = 7
		}
		next = next.AddDate(0, 0, daysUntil)
	case "monthly":
		next = time.Date(now.Year(), now.Month(), schedule.DayOfMonth, hour, minute, 0, 0, tz)
		if next.Before(now) || next.Equal(now) {
			next = next.AddDate(0, 1, 0)
		}
	}

	// Store as UTC so timestamp without time zone column works correctly
	return next.UTC()
}

// handleBackupError handles backup errors
func (s *BackupSchedulerService) handleBackupError(schedule *models.BackupSchedule, backupLog *models.BackupLog, err error, startTime time.Time) {
	log.Printf("BackupScheduler: Backup failed for %s: %v", schedule.Name, err)

	completedAt := time.Now()

	// Update schedule
	database.DB.Model(schedule).Updates(map[string]interface{}{
		"last_status": "failed",
		"last_error":  err.Error(),
	})

	// Update backup log
	backupLog.Status = "failed"
	backupLog.ErrorMessage = err.Error()
	backupLog.CompletedAt = completedAt
	backupLog.Duration = int(completedAt.Sub(startTime).Seconds())
	database.DB.Save(backupLog)
}

// TestFTPConnection tests FTP connection with given credentials
func TestFTPConnection(host string, port int, username, password, path string) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(10*time.Second))
	if err != nil {
		return fmt.Errorf("connection failed: %v", err)
	}
	defer conn.Quit()

	err = conn.Login(username, password)
	if err != nil {
		return fmt.Errorf("login failed: %v", err)
	}

	if path != "" && path != "/" {
		err = conn.ChangeDir(path)
		if err != nil {
			// Try to create directory
			err = conn.MakeDir(path)
			if err != nil {
				return fmt.Errorf("cannot access or create directory %s: %v", path, err)
			}
		}
	}

	return nil
}

// RunManualBackup runs a manual backup with optional FTP upload and MikroTik config export
func (s *BackupSchedulerService) RunManualBackup(backupType string, ftpConfig *models.BackupSchedule, userID uint, username string, includeMikroTik ...bool) (*models.BackupLog, error) {
	startTime := time.Now()
	doMikroTik := len(includeMikroTik) > 0 && includeMikroTik[0]
	log.Printf("BackupScheduler: RunManualBackup started (type=%s, includeMikroTik=%v, cloudEnabled=%v)", backupType, doMikroTik, ftpConfig != nil && ftpConfig.CloudEnabled)

	// Create backup log entry
	backupLog := models.BackupLog{
		BackupType:    backupType,
		Status:        "running",
		StartedAt:     startTime,
		CreatedByID:   &userID,
		CreatedByName: username,
	}
	database.DB.Create(&backupLog)

	// Generate filenames
	timestamp := startTime.Format("20060102_150405")
	tempFile := filepath.Join(s.backupDir, fmt.Sprintf(".temp_%s_%s.dump", backupType, timestamp))
	filename := fmt.Sprintf("proisp_%s_%s.proisp.bak", backupType, timestamp)
	localPath := filepath.Join(s.backupDir, filename)

	// Run pg_dump with custom format
	err := s.createDatabaseBackupCustomFormat(backupType, tempFile)
	if err != nil {
		backupLog.Status = "failed"
		backupLog.ErrorMessage = err.Error()
		backupLog.CompletedAt = time.Now()
		database.DB.Save(&backupLog)
		return &backupLog, err
	}

	// Encrypt the backup
	err = s.EncryptFile(tempFile, localPath)
	os.Remove(tempFile) // Clean up temp file regardless
	if err != nil {
		backupLog.Status = "failed"
		backupLog.ErrorMessage = fmt.Sprintf("Encryption failed: %v", err)
		backupLog.CompletedAt = time.Now()
		database.DB.Save(&backupLog)
		return &backupLog, err
	}

	// Get file info
	fileInfo, _ := os.Stat(localPath)
	backupLog.Filename = filename
	backupLog.FileSize = fileInfo.Size()
	backupLog.StoragePath = localPath
	backupLog.StorageType = "local"

	// Export MikroTik configs if requested
	var mikrotikExportPath string
	if doMikroTik {
		log.Printf("BackupScheduler: Starting MikroTik config export...")
		mikrotikPath, err := s.ExportMikroTikConfigs(timestamp)
		if err != nil {
			log.Printf("BackupScheduler: MikroTik export failed for manual backup: %v (continuing with DB backup)", err)
		} else if mikrotikPath != "" {
			mikrotikExportPath = mikrotikPath
			log.Printf("BackupScheduler: MikroTik configs saved to %s", filepath.Base(mikrotikPath))
		} else {
			log.Printf("BackupScheduler: MikroTik export returned empty path (no NAS devices with API credentials)")
		}
	}

	// Upload to FTP if configured
	if ftpConfig != nil && ftpConfig.FTPEnabled {
		err = s.uploadToFTP(ftpConfig, localPath, filename)
		if err != nil {
			// FTP failed but local succeeded
			backupLog.ErrorMessage = fmt.Sprintf("Local backup succeeded, FTP failed: %v", err)
		} else {
			backupLog.StorageType = "both"
		}
	}

	// Upload to ProxPanel Cloud if enabled
	if ftpConfig != nil && ftpConfig.CloudEnabled {
		if err := s.uploadToProxPanelCloud(localPath, filename); err != nil {
			log.Printf("BackupScheduler: Cloud upload failed for manual backup: %v", err)
		} else {
			log.Printf("BackupScheduler: Successfully uploaded %s to ProxPanel Cloud", filename)
			if backupLog.StorageType == "local" {
				backupLog.StorageType = "local+cloud"
			}
		}
		// Also upload MikroTik backup to cloud if it was exported
		if mikrotikExportPath != "" {
			mtFilename := filepath.Base(mikrotikExportPath)
			if err := s.uploadToProxPanelCloud(mikrotikExportPath, mtFilename); err != nil {
				log.Printf("BackupScheduler: Cloud upload failed for MikroTik %s: %v", mtFilename, err)
			} else {
				log.Printf("BackupScheduler: Successfully uploaded MikroTik %s to ProxPanel Cloud", mtFilename)
			}
		}
	}

	// Upload to ProxPanel cloud if storage type includes cloud
	if ftpConfig != nil && (ftpConfig.StorageType == "cloud" || ftpConfig.StorageType == "local+cloud") {
		if err := s.uploadToCloud(localPath, filename); err != nil {
			log.Printf("BackupScheduler: Cloud upload failed for manual backup: %v", err)
		} else {
			if backupLog.StorageType == "local" {
				backupLog.StorageType = "local+cloud"
			}
		}
	}

	completedAt := time.Now()
	backupLog.Status = "success"
	backupLog.CompletedAt = completedAt
	backupLog.Duration = int(completedAt.Sub(startTime).Seconds())
	database.DB.Save(&backupLog)

	log.Printf("BackupScheduler: RunManualBackup completed (file=%s, size=%d, storage=%s, mikrotik=%s, duration=%ds)",
		backupLog.Filename, backupLog.FileSize, backupLog.StorageType,
		func() string {
			if mikrotikExportPath != "" {
				return filepath.Base(mikrotikExportPath)
			}
			return "none"
		}(), backupLog.Duration)
	return &backupLog, nil
}

// deriveEncryptionKey derives a 32-byte key from the database password and a salt
func (s *BackupSchedulerService) deriveEncryptionKey() []byte {
	// Use database password + fixed salt to derive encryption key
	// This ensures backups can only be decrypted with knowledge of the DB password
	salt := "ProxPanel-Backup-Encryption-2024"
	combined := s.cfg.DBPassword + salt
	hash := sha256.Sum256([]byte(combined))
	return hash[:]
}

// EncryptFile encrypts a file using AES-256-GCM
func (s *BackupSchedulerService) EncryptFile(inputPath, outputPath string) error {
	// Read input file
	plaintext, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %v", err)
	}

	// Get encryption key
	key := s.deriveEncryptionKey()

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %v", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %v", err)
	}

	// Create nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to create nonce: %v", err)
	}

	// Encrypt and prepend nonce
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// Write encrypted file with V2 header (embeds license key for cross-server restore)
	licenseKey := os.Getenv("LICENSE_KEY")
	if licenseKey == "" {
		licenseKey = "UNKNOWN"
	}
	header := fmt.Sprintf("PROXPANEL_ENCRYPTED_BACKUP_V2\nLICENSE_KEY=%s\n", licenseKey)
	output := append([]byte(header), ciphertext...)

	if err := os.WriteFile(outputPath, output, 0600); err != nil {
		return fmt.Errorf("failed to write encrypted file: %v", err)
	}

	return nil
}

// DecryptFile decrypts a file encrypted with EncryptFile
func (s *BackupSchedulerService) DecryptFile(inputPath, outputPath string) error {
	// Read encrypted file
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read encrypted file: %v", err)
	}

	// Check and remove magic header
	header := []byte("PROXPANEL_ENCRYPTED_BACKUP_V1\n")
	if len(data) < len(header) || string(data[:len(header)]) != string(header) {
		return fmt.Errorf("invalid encrypted file format")
	}
	ciphertext := data[len(header):]

	// Get decryption key
	key := s.deriveEncryptionKey()

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %v", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %v", err)
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("decryption failed: %v", err)
	}

	// Write decrypted file
	if err := os.WriteFile(outputPath, plaintext, 0600); err != nil {
		return fmt.Errorf("failed to write decrypted file: %v", err)
	}

	return nil
}

// IsEncrypted checks if a backup file is encrypted
func IsEncrypted(filePath string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	header := make([]byte, 30) // Length of "PROXPANEL_ENCRYPTED_BACKUP_V1\n"
	n, err := file.Read(header)
	if err != nil || n < 30 {
		return false
	}

	return string(header) == "PROXPANEL_ENCRYPTED_BACKUP_V1\n"
}

// GetEncryptionKeyHash returns a hash of the encryption key for verification
func (s *BackupSchedulerService) GetEncryptionKeyHash() string {
	key := s.deriveEncryptionKey()
	hash := sha256.Sum256(key)
	return hex.EncodeToString(hash[:8]) // Return first 8 bytes as hex
}

// ExportMikroTikConfigs exports config from NAS devices with API credentials
// and bundles them into a tar.gz file. Returns the path to the tar.gz or empty string if none.
// If nasIDs is provided and non-empty, only export from those specific NAS devices.
func (s *BackupSchedulerService) ExportMikroTikConfigs(timestamp string, nasIDs ...[]uint) (string, error) {
	// Find NAS devices with API credentials
	var nasDevices []models.Nas
	query := database.DB.Where("is_active = ? AND api_username != '' AND api_password != ''", true)
	if len(nasIDs) > 0 && len(nasIDs[0]) > 0 {
		query = query.Where("id IN ?", nasIDs[0])
	}
	if err := query.Find(&nasDevices).Error; err != nil {
		return "", fmt.Errorf("failed to query NAS devices: %v", err)
	}

	if len(nasDevices) == 0 {
		log.Println("BackupScheduler: No NAS devices with API credentials found, skipping MikroTik backup")
		return "", nil
	}

	// Create temp directory for individual exports
	tmpDir := filepath.Join(s.backupDir, fmt.Sprintf(".mikrotik_export_%s", timestamp))
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	var exportedFiles []string

	for _, nas := range nasDevices {
		apiAddr := fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort)
		client := mikrotik.NewClient(apiAddr, nas.APIUsername, nas.APIPassword)
		client.FTPPort = nas.FTPPort

		configText, err := client.ExportConfig()
		client.Close()

		if err != nil {
			log.Printf("BackupScheduler: Failed to export config from NAS %s (%s): %v", nas.Name, nas.IPAddress, err)
			// Write an error marker file so user knows this NAS failed
			errContent := fmt.Sprintf("# Export failed for %s (%s) at %s\n# Error: %v\n", nas.Name, nas.IPAddress, time.Now().Format(time.RFC3339), err)
			safeName := strings.ReplaceAll(strings.ReplaceAll(nas.Name, "/", "_"), " ", "_")
			errFile := filepath.Join(tmpDir, fmt.Sprintf("nas_%s_%s.rsc.ERROR", safeName, nas.IPAddress))
			os.WriteFile(errFile, []byte(errContent), 0600)
			exportedFiles = append(exportedFiles, errFile)
			continue
		}

		// Save config as .rsc file
		safeName := strings.ReplaceAll(strings.ReplaceAll(nas.Name, "/", "_"), " ", "_")
		rscFile := filepath.Join(tmpDir, fmt.Sprintf("nas_%s_%s.rsc", safeName, nas.IPAddress))
		if err := os.WriteFile(rscFile, []byte(configText), 0600); err != nil {
			log.Printf("BackupScheduler: Failed to write config file for NAS %s: %v", nas.Name, err)
			continue
		}

		exportedFiles = append(exportedFiles, rscFile)
		log.Printf("BackupScheduler: Exported config from NAS %s (%s), %d bytes", nas.Name, nas.IPAddress, len(configText))
	}

	if len(exportedFiles) == 0 {
		return "", nil
	}

	// Bundle into tar.gz
	tarGzPath := filepath.Join(s.backupDir, fmt.Sprintf("mikrotik_%s.tar.gz", timestamp))
	if err := createTarGz(tarGzPath, exportedFiles); err != nil {
		return "", fmt.Errorf("failed to create tar.gz: %v", err)
	}

	log.Printf("BackupScheduler: Created MikroTik backup %s with %d configs", filepath.Base(tarGzPath), len(exportedFiles))
	return tarGzPath, nil
}

// createTarGz creates a tar.gz archive from a list of files
func createTarGz(outputPath string, files []string) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	for _, filePath := range files {
		info, err := os.Stat(filePath)
		if err != nil {
			continue
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			continue
		}
		// Use just the base filename in the archive
		header.Name = filepath.Base(filePath)

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		f, err := os.Open(filePath)
		if err != nil {
			continue
		}
		if _, err := io.Copy(tarWriter, f); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}

	return nil
}
