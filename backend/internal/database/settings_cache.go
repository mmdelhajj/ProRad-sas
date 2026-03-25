package database

import (
	"log"
	"sync"
	"time"

	"github.com/proisp/backend/internal/models"
)

// settingsCache is an in-memory cache for system_preferences table.
// Eliminates tens of millions of sequential scans per day by serving
// settings reads from memory instead of hitting PostgreSQL every time.
var settingsCache struct {
	mu       sync.RWMutex
	data     map[string]string
	loaded   bool
	stopChan chan struct{}
}

// InitSettingsCache loads all system_preferences into memory and starts
// a background goroutine to refresh every 60 seconds.
func InitSettingsCache() {
	settingsCache.stopChan = make(chan struct{})
	refreshSettingsCache()

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				refreshSettingsCache()
			case <-settingsCache.stopChan:
				return
			}
		}
	}()
}

// StopSettingsCache stops the background refresh goroutine.
func StopSettingsCache() {
	if settingsCache.stopChan != nil {
		close(settingsCache.stopChan)
	}
}

// refreshSettingsCache loads all rows from system_preferences into memory.
func refreshSettingsCache() {
	var prefs []models.SystemPreference
	if err := DB.Find(&prefs).Error; err != nil {
		log.Printf("SettingsCache: Failed to refresh: %v", err)
		return
	}

	m := make(map[string]string, len(prefs))
	for _, p := range prefs {
		m[p.Key] = p.Value
	}

	settingsCache.mu.Lock()
	settingsCache.data = m
	settingsCache.loaded = true
	settingsCache.mu.Unlock()
}

// GetSetting returns a system_preference value from the in-memory cache.
// Returns empty string and false if not found.
func GetSetting(key string) (string, bool) {
	settingsCache.mu.RLock()
	defer settingsCache.mu.RUnlock()

	if !settingsCache.loaded {
		return "", false
	}
	v, ok := settingsCache.data[key]
	return v, ok
}

// GetSettingDefault returns a system_preference value or a default if not found.
func GetSettingDefault(key, defaultVal string) string {
	if v, ok := GetSetting(key); ok && v != "" {
		return v
	}
	return defaultVal
}

// InvalidateSettingsCacheNow forces an immediate refresh of the settings cache.
// Call this after updating system_preferences via the API.
func InvalidateSettingsCacheNow() {
	refreshSettingsCache()
}
