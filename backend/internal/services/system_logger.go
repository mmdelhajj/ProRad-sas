package services

import (
	"log"
	"time"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

// LogInfo writes an info-level system log entry
func LogInfo(module, message string) {
	go writeSystemLog("info", module, message, "")
}

// LogWarning writes a warning-level system log entry
func LogWarning(module, message, details string) {
	go writeSystemLog("warning", module, message, details)
}

// LogError writes an error-level system log entry
func LogError(module, message, details string) {
	go writeSystemLog("error", module, message, details)
}

func writeSystemLog(level, module, message, details string) {
	if database.DB == nil {
		return
	}
	entry := models.SystemLog{
		Level:   level,
		Module:  module,
		Message: message,
		Details: details,
	}
	if err := database.DB.Create(&entry).Error; err != nil {
		log.Printf("Failed to write system log: %v", err)
	}
}

// CleanupOldLogs deletes radius_logs and system_logs older than 30 days
func CleanupOldLogs() {
	if database.DB == nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -30)

	result := database.DB.Where("created_at < ?", cutoff).Delete(&models.RadiusLog{})
	if result.RowsAffected > 0 {
		log.Printf("LogCleanup: Deleted %d old radius_logs entries", result.RowsAffected)
	}

	result = database.DB.Where("created_at < ?", cutoff).Delete(&models.SystemLog{})
	if result.RowsAffected > 0 {
		log.Printf("LogCleanup: Deleted %d old system_logs entries", result.RowsAffected)
	}
}

// StartLogCleanupService runs log cleanup on startup and then daily
func StartLogCleanupService() {
	// Run cleanup immediately on startup
	go func() {
		time.Sleep(10 * time.Second) // Wait for DB to be ready
		CleanupOldLogs()
		LogInfo("api", "API server started")
	}()

	// Run cleanup daily
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			CleanupOldLogs()
		}
	}()
}
