package services

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/mikrotik"
	"github.com/proisp/backend/internal/models"
	"github.com/proisp/backend/internal/radius"
)

// getConfiguredTimezone returns the system timezone from settings
// Falls back to UTC if not configured or invalid
func getConfiguredTimezone() *time.Location {
	v := database.GetSettingDefault("system_timezone", "")
	if v == "" {
		return time.UTC
	}

	loc, err := time.LoadLocation(v)
	if err != nil {
		return time.UTC
	}

	return loc
}

// getNow returns current time in the configured timezone
func getNow() time.Time {
	return time.Now().In(getConfiguredTimezone())
}

// getDailyQuotaResetTime gets the configured daily quota reset time from settings
// Returns hour and minute. Defaults to 00:00 (midnight)
func getDailyQuotaResetTime() (int, int) {
	v := database.GetSettingDefault("daily_quota_reset_time", "")
	if v == "" {
		return 0, 0 // Default to midnight
	}

	// Parse time in HH:MM format
	parts := strings.Split(v, ":")
	if len(parts) != 2 {
		return 0, 0
	}

	hour, err1 := strconv.Atoi(parts[0])
	minute, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0
	}

	return hour, minute
}

// getQuotaSyncCompanyName retrieves company name from settings for quota sync branding
func getQuotaSyncCompanyName() string {
	name := database.GetCompanyName()
	if name == "" {
		return "ISP"
	}
	return name
}

// isBlockOnDailyQuotaExceeded checks if internet should be blocked when daily quota is exceeded
func isBlockOnDailyQuotaExceeded() bool {
	v := database.GetSettingDefault("block_on_daily_quota_exceeded", "")
	return v == "true" || v == "1"
}

// isBlockOnMonthlyQuotaExceeded checks if internet should be blocked when monthly quota is exceeded
func isBlockOnMonthlyQuotaExceeded() bool {
	v := database.GetSettingDefault("block_on_monthly_quota_exceeded", "")
	return v == "true" || v == "1"
}

// shouldResetDailyQuota checks if daily quota should be reset based on configured time
func shouldResetDailyQuota(lastReset *time.Time, now time.Time) bool {
	resetHour, resetMinute := getDailyQuotaResetTime()

	// Calculate the reset time for today
	todayReset := time.Date(now.Year(), now.Month(), now.Day(), resetHour, resetMinute, 0, 0, now.Location())

	// If current time is before today's reset time, the effective reset time is yesterday's
	if now.Before(todayReset) {
		todayReset = todayReset.AddDate(0, 0, -1)
	}

	// If no previous reset, or last reset was before the effective reset time
	if lastReset == nil || lastReset.Before(todayReset) {
		return true
	}

	return false
}

// DailyQuotaResetService handles scheduled daily quota reset for ALL users
type DailyQuotaResetService struct {
	stopChan    chan struct{}
	wg          sync.WaitGroup
	lastResetAt time.Time
}

// NewDailyQuotaResetService creates a new daily quota reset service
func NewDailyQuotaResetService() *DailyQuotaResetService {
	return &DailyQuotaResetService{
		stopChan: make(chan struct{}),
	}
}

// Start begins the daily quota reset scheduler
func (s *DailyQuotaResetService) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		log.Println("DailyQuotaResetService started")

		// Check every minute
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.checkAndReset()
			case <-s.stopChan:
				log.Println("DailyQuotaResetService stopped")
				return
			}
		}
	}()
}

// Stop stops the daily quota reset service
func (s *DailyQuotaResetService) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

// checkAndReset checks if it's time to reset and performs the reset
func (s *DailyQuotaResetService) checkAndReset() {
	now := getNow()
	resetHour, resetMinute := getDailyQuotaResetTime()

	// Check if current time matches reset time (within the current minute)
	if now.Hour() != resetHour || now.Minute() != resetMinute {
		return
	}

	// Check if we already reset today
	todayReset := time.Date(now.Year(), now.Month(), now.Day(), resetHour, resetMinute, 0, 0, now.Location())
	if !s.lastResetAt.IsZero() && s.lastResetAt.After(todayReset.Add(-1*time.Minute)) {
		return // Already reset in this time window
	}

	log.Printf("DailyQuotaResetService: Running scheduled reset at %02d:%02d", resetHour, resetMinute)

	// Save daily usage to history BEFORE resetting counters.
	// This is the source of truth for the per-day chart — radacct groups by session
	// start time so sessions spanning midnight get attributed to the wrong day.
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	saveResult := database.DB.Exec(`
		INSERT INTO daily_usage_history (subscriber_id, date, download_bytes, upload_bytes)
		SELECT id, ?::date, daily_download_used, daily_upload_used
		FROM subscribers
		WHERE deleted_at IS NULL AND (daily_download_used > 0 OR daily_upload_used > 0)
		ON CONFLICT (subscriber_id, date) DO UPDATE SET
			download_bytes = GREATEST(daily_usage_history.download_bytes, EXCLUDED.download_bytes),
			upload_bytes   = GREATEST(daily_usage_history.upload_bytes,   EXCLUDED.upload_bytes)
	`, yesterday)
	if saveResult.Error != nil {
		log.Printf("DailyQuotaResetService: Failed to save usage history: %v", saveResult.Error)
	} else {
		log.Printf("DailyQuotaResetService: Saved daily usage history for %d subscribers (date: %s)", saveResult.RowsAffected, yesterday)
	}

	// Reset daily quotas for ALL subscribers (online and offline)
	result := database.DB.Model(&models.Subscriber{}).
		Where("deleted_at IS NULL").
		Updates(map[string]interface{}{
			"daily_quota_used":         0,
			"daily_download_used":      0,
			"daily_upload_used":        0,
			"fup_level":                0,
			"cdn_daily_download_used":  0,
			"cdn_daily_upload_used":    0,
			"cdn_fup_level":            0,
			"last_daily_reset":         now,
		})

	if result.Error != nil {
		log.Printf("DailyQuotaResetService: Failed to reset quotas: %v", result.Error)
		return
	}

	s.lastResetAt = now
	log.Printf("DailyQuotaResetService: Reset daily quotas for %d subscribers", result.RowsAffected)

	// Remove all CDN FUP queues from NAS devices (they'll be re-created if needed)
	var nasDevices []models.Nas
	database.DB.Where("is_active = ? AND cdn_torch_interface != ''", true).Find(&nasDevices)
	for _, nas := range nasDevices {
		go func(n models.Nas) {
			client := mikrotik.NewClient(
				fmt.Sprintf("%s:%d", n.IPAddress, n.APIPort),
				n.APIUsername,
				n.APIPassword,
			)
			defer client.Close()
			if err := client.RemoveAllCDNFUPQueues(); err != nil {
				log.Printf("DailyQuotaResetService: Failed to remove CDN FUP queues from NAS %s: %v", n.Name, err)
			}
		}(nas)
	}

	// Also reset daily quotas for bandwidth manager customers
	ResetDailyQuotasForBWCustomers()
}

// TimeSpeedState tracks time-based speed state for a user session
type TimeSpeedState struct {
	Applied   bool   // whether time-based speed is currently applied
	SessionID string // the session ID when speed was applied
}

// resellerWanConfig holds per-reseller WAN check settings
type resellerWanConfig struct {
	WanCheckEnabled *bool
	WanCheckICMP    bool
	WanCheckPort    bool
}

// QuotaSyncService handles periodic quota synchronization from MikroTik to database
type QuotaSyncService struct {
	interval              time.Duration
	stopChan              chan struct{}
	wg                    sync.WaitGroup
	timeBasedSpeedState   map[string]*TimeSpeedState // username -> time-based speed state
	wanCheckCooldown      map[uint]time.Time         // subscriber ID -> last WAN check time
	wanCheckWasEnabled    bool                       // tracks previous state to detect enable transition
	resellerWanMap        map[uint]resellerWanConfig // reseller ID -> WAN check config
	mu                    sync.RWMutex
	syncMu                sync.Mutex // prevents concurrent syncAllQuotas runs
}

// NewQuotaSyncService creates a new quota sync service
func NewQuotaSyncService(interval time.Duration) *QuotaSyncService {
	return &QuotaSyncService{
		interval:            interval,
		stopChan:            make(chan struct{}),
		timeBasedSpeedState: make(map[string]*TimeSpeedState),
		wanCheckCooldown:    make(map[uint]time.Time),
	}
}

// Start begins the quota sync background job
func (s *QuotaSyncService) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		log.Printf("QuotaSyncService started, syncing every %v", s.interval)

		// Run immediately on start
		s.syncAllQuotas()

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.syncAllQuotas()
			case <-s.stopChan:
				log.Println("QuotaSyncService stopped")
				return
			}
		}
	}()
}

// Stop stops the quota sync service
func (s *QuotaSyncService) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

// syncAllQuotas syncs quota for all online subscribers
func (s *QuotaSyncService) syncAllQuotas() {
	// Prevent concurrent runs: if previous cycle is still running, skip this one
	if !s.syncMu.TryLock() {
		log.Println("QuotaSync: previous cycle still running, skipping this tick")
		return
	}
	defer s.syncMu.Unlock()

	// Detect and resolve static IP conflicts first
	// This kicks dynamic users who got a static IP from the pool
	s.detectAndResolveStaticIPConflicts()

	// WAN Management Check is handled per-subscriber in checkWanManagement()

	// Load reseller WAN check settings for per-reseller overrides
	var resellersForWan []models.Reseller
	database.DB.Select("id, wan_check_enabled, wan_check_icmp, wan_check_port").Find(&resellersForWan)
	s.resellerWanMap = make(map[uint]resellerWanConfig)
	for _, r := range resellersForWan {
		s.resellerWanMap[r.ID] = resellerWanConfig{
			WanCheckEnabled: r.WanCheckEnabled,
			WanCheckICMP:    r.WanCheckICMP,
			WanCheckPort:    r.WanCheckPort,
		}
	}

	// Get all online subscribers with their NAS and Service
	var subscribers []models.Subscriber
	if err := database.DB.Preload("Nas").Preload("Service").
		Where("is_online = ?", true).Find(&subscribers).Error; err != nil {
		log.Printf("QuotaSync: Failed to get online subscribers: %v", err)
		return
	}

	if len(subscribers) == 0 {
		return
	}

	log.Printf("QuotaSync: Syncing %d online subscribers", len(subscribers))

	// Group subscribers by NAS to minimize connections
	nasSubs := make(map[uint][]models.Subscriber)
	for _, sub := range subscribers {
		if sub.NasID != nil {
			nasSubs[*sub.NasID] = append(nasSubs[*sub.NasID], sub)
		}
	}

	// Process each NAS
	for _, subs := range nasSubs {
		if len(subs) == 0 || subs[0].Nas == nil {
			continue
		}
		nas := subs[0].Nas
		s.syncNasSubscribers(nas, subs)
	}

	// Every 10 minutes, snapshot current daily usage to history.
	// This ensures history is saved even if the daily reset is missed (API restart, etc.)
	now := getNow()
	if now.Minute()%10 == 0 {
		today := now.Format("2006-01-02")
		database.DB.Exec(`
			INSERT INTO daily_usage_history (subscriber_id, date, download_bytes, upload_bytes)
			SELECT id, ?::date, daily_download_used, daily_upload_used
			FROM subscribers
			WHERE deleted_at IS NULL AND (daily_download_used > 0 OR daily_upload_used > 0)
			ON CONFLICT (subscriber_id, date) DO UPDATE SET
				download_bytes = GREATEST(daily_usage_history.download_bytes, EXCLUDED.download_bytes),
				upload_bytes   = GREATEST(daily_usage_history.upload_bytes,   EXCLUDED.upload_bytes)
		`, today)
	}
}

// syncNasSubscribers syncs quota for subscribers on a specific NAS
func (s *QuotaSyncService) syncNasSubscribers(nas *models.Nas, subscribers []models.Subscriber) {
	client := mikrotik.NewClient(
		fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
		nas.APIUsername,
		nas.APIPassword,
	)
	defer client.Close()

	now := getNow()

	// Batch fetch ALL active sessions from MikroTik in ONE API call
	allSessions, err := client.GetActiveSessionMap()
	if err != nil {
		log.Printf("QuotaSync: Failed to batch-fetch sessions from NAS %s: %v, falling back to individual queries", nas.Name, err)
		allSessions = nil // will fall back to individual GetActiveSession calls
	} else {
		log.Printf("QuotaSync: Batch-fetched %d active sessions from NAS %s for %d DB subscribers", len(allSessions), nas.Name, len(subscribers))
	}

	// CDN FUP: Run batch torch on NAS bridge interface to track CDN usage per subscriber
	var cdnTorchByIP map[string]cdnBytesResult
	if nas.CDNTorchInterface != "" {
		cdnTorchByIP = s.runBatchCDNTorch(client, nas)
	}

	for _, sub := range subscribers {
		// Step 1: Check if user is connected (fast batch lookup or individual check)
		isConnected := false
		if allSessions != nil {
			// Fast O(1) lookup from batch result
			isConnected = allSessions[sub.Username] != nil
		} else {
			// Fallback: individual query
			s, err := client.GetActiveSession(sub.Username)
			isConnected = err == nil && s != nil
		}

		if !isConnected {
			// Session ended - mark user as offline and clear IP
			log.Printf("QuotaSync: No active session for %s on NAS %s - marking offline", sub.Username, nas.Name)
			result := database.DB.Model(&models.Subscriber{}).Where("id = ?", sub.ID).Updates(map[string]interface{}{
				"is_online":             false,
				"ip_address":            nil, // Clear IP so it doesn't show as duplicate
				"last_quota_sync":       getNow(),
				"last_bypass_cdn_bytes": 0, // Reset bypass tracking for next session
			})
			if result.Error != nil {
				log.Printf("QuotaSync: Failed to mark %s offline: %v", sub.Username, result.Error)
			} else {
				log.Printf("QuotaSync: Marked %s as offline (rows affected: %d)", sub.Username, result.RowsAffected)
			}
			// Remove CDN queues when user disconnects
			if err := client.RemoveSubscriberCDNQueues(sub.Username, getQuotaSyncCompanyName()); err != nil {
				log.Printf("QuotaSync: Failed to remove CDN queues for %s: %v", sub.Username, err)
			} else {
				log.Printf("QuotaSync: Removed CDN queues for %s", sub.Username)
			}
			// Also remove CDN override queue if exists
			if err := client.RemoveSubscriberCDNOverrideQueue(sub.Username, getQuotaSyncCompanyName()); err != nil {
				log.Printf("QuotaSync: Failed to remove CDN override queue for %s: %v", sub.Username, err)
			}
			continue
		}
		// Step 2: User is connected — get full session with traffic bytes
		session, err := client.GetActiveSession(sub.Username)
		if err != nil || session == nil {
			// Race condition: user disconnected between batch check and individual query
			log.Printf("QuotaSync: %s connected in batch but GetActiveSession failed: %v - skipping", sub.Username, err)
			continue
		}
		log.Printf("QuotaSync: Got session for %s: IP=%s, SessionID=%s, ID=%s", sub.Username, session.Address, session.SessionID, session.ID)

		// Get current session bytes from MikroTik
		// After MikroTik parsing: TxBytes = client download, RxBytes = client upload
		currentDownload := session.TxBytes
		currentUpload := session.RxBytes

		// Re-read fresh subscriber data FIRST to handle race conditions with FUP reset
		// This ensures we use the updated last_session values set during reset
		var freshSub models.Subscriber
		if err := database.DB.First(&freshSub, sub.ID).Error; err != nil {
			log.Printf("QuotaSync: Failed to re-read subscriber %s: %v", sub.Username, err)
			continue
		}

		// Check if a reset happened since we started (last_daily_reset > last_quota_sync)
		wasReset := false
		if freshSub.LastDailyReset != nil && freshSub.LastQuotaSync != nil {
			if freshSub.LastDailyReset.After(*freshSub.LastQuotaSync) {
				wasReset = true
				log.Printf("QuotaSync: Detected FUP reset for %s, using fresh baseline data", sub.Username)
			}
		}

		// Calculate delta since last sync using FRESH subscriber data
		// This is critical after FUP reset which sets last_session values to current MikroTik bytes
		var deltaDownload, deltaUpload int64

		// If current bytes are less than last known, session must have restarted
		if currentDownload >= freshSub.LastSessionDownload {
			deltaDownload = currentDownload - freshSub.LastSessionDownload
		} else {
			// Session restarted, all current bytes are new
			deltaDownload = currentDownload
		}

		if currentUpload >= freshSub.LastSessionUpload {
			deltaUpload = currentUpload - freshSub.LastSessionUpload
		} else {
			// Session restarted, all current bytes are new
			deltaUpload = currentUpload
		}

		// Check for CDNs with bypass_quota=true and subtract their traffic from quota
		var bypassCDNDelta int64
		var currentBypassBytes int64
		if sub.ServiceID > 0 {
			var bypassServiceCDNs []models.ServiceCDN
			database.DB.Preload("CDN").Where("service_id = ? AND is_active = ? AND bypass_quota = ?", sub.ServiceID, true, true).Find(&bypassServiceCDNs)

			if len(bypassServiceCDNs) > 0 {
				// Build CDN config list for bypass CDNs only
				var bypassCDNConfigs []mikrotik.CDNSubnetConfig
				for _, sc := range bypassServiceCDNs {
					if sc.CDN != nil && sc.CDN.ID > 0 && sc.CDN.Subnets != "" {
						bypassCDNConfigs = append(bypassCDNConfigs, mikrotik.CDNSubnetConfig{
							ID:      sc.CDNID,
							Name:    sc.CDN.Name,
							Subnets: sc.CDN.Subnets,
						})
					}
				}

				if len(bypassCDNConfigs) > 0 {
					// Get current bypass CDN traffic from connection tracking
					cdnCounters, err := client.GetCDNTrafficForSubscriber(session.Address, bypassCDNConfigs)
					if err == nil {
						for _, counter := range cdnCounters {
							currentBypassBytes += counter.Bytes
						}

						// Calculate delta bypass bytes
						if currentBypassBytes >= freshSub.LastBypassCDNBytes {
							bypassCDNDelta = currentBypassBytes - freshSub.LastBypassCDNBytes
						} else {
							// Connections reset/closed, use current as new baseline
							bypassCDNDelta = currentBypassBytes
						}

						if bypassCDNDelta > 0 {
							// Subtract bypass CDN traffic from download delta (CDN is mostly download)
							if bypassCDNDelta <= deltaDownload {
								deltaDownload -= bypassCDNDelta
								log.Printf("QuotaSync: %s - bypass CDN traffic: %.2fMB subtracted from quota",
									sub.Username, float64(bypassCDNDelta)/1024/1024)
							} else {
								// If bypass exceeds download delta, set download to 0
								deltaDownload = 0
								log.Printf("QuotaSync: %s - bypass CDN traffic: %.2fMB (exceeds delta, quota zeroed)",
									sub.Username, float64(bypassCDNDelta)/1024/1024)
							}
						}
					}
				}
			}
		}

		// Build single update map for atomic operation
		updates := map[string]interface{}{
			"last_session_download":  currentDownload,
			"last_session_upload":    currentUpload,
			"last_quota_sync":        now,
			"last_bypass_cdn_bytes":  currentBypassBytes,
		}

		// Check if we're in the free hours window and calculate quota discount
		// freePercent = how much of usage is FREE (0=none, 70=70% free, 100=completely free)
		freePercent := int64(0)
		if sub.Service != nil && sub.Service.ID > 0 {
			if isWithinTimeWindow(sub.Service, now) {
				// Use TimeDownloadRatio as the free percentage (0-100)
				freePercent = int64(sub.Service.TimeDownloadRatio)
				if freePercent > 100 {
					freePercent = 100
				}
				if freePercent < 0 {
					freePercent = 0
				}
				log.Printf("QuotaSync: %s is in FREE HOURS window (%02d:%02d-%02d:%02d) - %d%% free (only %d%% of usage counted)",
					sub.Username,
					sub.Service.TimeFromHour, sub.Service.TimeFromMinute,
					sub.Service.TimeToHour, sub.Service.TimeToMinute,
					freePercent, 100-freePercent,
				)
			}
		}

		// Apply quota discount: counted = delta * (100 - freePercent) / 100
		// 100% free = 0 counted, 70% free = 30% counted, 0% free = 100% counted
		discountedDownload := deltaDownload * (100 - freePercent) / 100
		discountedUpload := deltaUpload * (100 - freePercent) / 100
		isFreeTime := freePercent == 100 // fully free (for monthly skip optimization)

		// Calculate new daily quota values
		var newDailyDownload, newDailyUpload int64
		if shouldResetDailyQuota(freshSub.LastDailyReset, now) {
			// Reset time has passed - reset daily counters
			newDailyDownload = discountedDownload
			newDailyUpload = discountedUpload
			updates["last_daily_reset"] = now
		} else if wasReset {
			// FUP was reset - use discounted delta only (fresh counters are 0)
			newDailyDownload = discountedDownload
			newDailyUpload = discountedUpload
		} else {
			// Same day - add discounted delta to existing
			newDailyDownload = freshSub.DailyDownloadUsed + discountedDownload
			newDailyUpload = freshSub.DailyUploadUsed + discountedUpload
		}
		updates["daily_download_used"] = newDailyDownload
		updates["daily_upload_used"] = newDailyUpload

		// Calculate new monthly quota values
		// Monthly is NOT affected by daily FUP reset - it continues to accumulate
		var newMonthlyDownload, newMonthlyUpload int64
		thisMonth := now.Format("2006-01")

		// During 100% FREE TIME, skip monthly update entirely for efficiency
		if isFreeTime {
			newMonthlyDownload = freshSub.MonthlyDownloadUsed
			newMonthlyUpload = freshSub.MonthlyUploadUsed
			log.Printf("QuotaSync: %s FREE HOURS 100%% - monthly frozen at %.2f GB",
				sub.Username,
				float64(freshSub.MonthlyDownloadUsed+freshSub.MonthlyUploadUsed)/1024/1024/1024)
		} else if freshSub.LastMonthlyReset == nil || freshSub.LastMonthlyReset.Format("2006-01") != thisMonth {
			// New month - reset monthly counters
			newMonthlyDownload = discountedDownload
			newMonthlyUpload = discountedUpload
			updates["last_monthly_reset"] = now
			updates["monthly_download_used"] = newMonthlyDownload
			updates["monthly_upload_used"] = newMonthlyUpload
			updates["monthly_quota_used"] = newMonthlyDownload + newMonthlyUpload
		} else {
			// Same month - add discounted delta to existing
			newMonthlyDownload = freshSub.MonthlyDownloadUsed + discountedDownload
			newMonthlyUpload = freshSub.MonthlyUploadUsed + discountedUpload
			updates["monthly_download_used"] = newMonthlyDownload
			updates["monthly_upload_used"] = newMonthlyUpload
			updates["monthly_quota_used"] = newMonthlyDownload + newMonthlyUpload
		}

		// Always update daily (legacy field)
		updates["daily_quota_used"] = newDailyDownload + newDailyUpload

		// Perform single atomic update
		if err := database.DB.Model(&models.Subscriber{}).Where("id = ?", sub.ID).Updates(updates).Error; err != nil {
			log.Printf("QuotaSync: Failed to update %s: %v", sub.Username, err)
			continue
		}

		if deltaDownload > 0 || deltaUpload > 0 {
			log.Printf("QuotaSync: %s - delta: dl=%.2fMB ul=%.2fMB, daily: %.2fMB, monthly: %.2fGB (free=%d%%)",
				sub.Username,
				float64(deltaDownload)/1024/1024,
				float64(deltaUpload)/1024/1024,
				float64(newDailyDownload+newDailyUpload)/1024/1024,
				float64(newMonthlyDownload+newMonthlyUpload)/1024/1024/1024,
				freePercent,
			)
		}

		// WAN Management Check (highest priority - before FUP)
		freshSub.Service = sub.Service // Copy service from original (not preloaded in fresh)
		if s.checkWanManagement(client, nas, &freshSub, session.Address, session.SessionID) {
			continue // blocked by WAN check, skip all speed processing
		}

		// CDN FUP: Update CDN usage counters from batch torch results
		// Apply same Free Hours discount as regular quota (freePercent from line ~533)
		if cdnTorchByIP != nil && session.Address != "" {
			if cdnBytes, ok := cdnTorchByIP[session.Address]; ok && (cdnBytes.Download > 0 || cdnBytes.Upload > 0) {
				// Apply quota discount: counted = bytes * (100 - freePercent) / 100
				cdnDownloadCounted := cdnBytes.Download * (100 - freePercent) / 100
				cdnUploadCounted := cdnBytes.Upload * (100 - freePercent) / 100
				if cdnDownloadCounted > 0 || cdnUploadCounted > 0 {
					database.DB.Model(&models.Subscriber{}).Where("id = ?", sub.ID).Updates(map[string]interface{}{
						"cdn_daily_download_used":   gorm.Expr("cdn_daily_download_used + ?", cdnDownloadCounted),
						"cdn_daily_upload_used":     gorm.Expr("cdn_daily_upload_used + ?", cdnUploadCounted),
						"cdn_monthly_download_used": gorm.Expr("cdn_monthly_download_used + ?", cdnDownloadCounted),
						"cdn_monthly_upload_used":   gorm.Expr("cdn_monthly_upload_used + ?", cdnUploadCounted),
					})
					freshSub.CDNDailyDownloadUsed += cdnDownloadCounted
					freshSub.CDNMonthlyDownloadUsed += cdnDownloadCounted
					freshSub.CDNDailyUploadUsed += cdnUploadCounted
					freshSub.CDNMonthlyUploadUsed += cdnUploadCounted
				}
			}
		}

		// Check and enforce FUP (Fair Usage Policy)
		// Use freshSub to get current FUP level (may have been reset)
		newTotalDaily := newDailyDownload + newDailyUpload
		newTotalMonthly := newMonthlyDownload + newMonthlyUpload
		s.checkAndEnforceFUP(client, nas, &freshSub, session.Address, session.SessionID, newTotalDaily, newTotalMonthly)

		// Check and enforce CDN FUP (independent of regular FUP)
		if nas.CDNTorchInterface != "" && freshSub.Service != nil && freshSub.Service.CDNFUPEnabled {
			s.checkAndEnforceCDNFUP(client, nas, &freshSub, session.Address)
		}

		// Fire quota_warning rules if monthly quota threshold reached (only when data was used this cycle)
		if (deltaDownload > 0 || deltaUpload > 0) && freshSub.Service != nil {
			go fireQuotaWarningRules(freshSub, newTotalMonthly)
		}

		// Check and apply time-based speed control from Service settings
		// Re-read fresh FUP levels to check if FUP was applied
		var latestSub models.Subscriber
		if err := database.DB.First(&latestSub, sub.ID).Error; err == nil {
			latestSub.Service = sub.Service
			s.checkAndApplyTimeBasedSpeed(client, nas, &latestSub, session.Address, session.SessionID)
		}

		// Per-subscriber CDN queues disabled - now using PCQ mode for CDN queues
		// s.syncSubscriberCDNQueues(client, &sub, session.Address)

		// Check and apply per-subscriber CDN bandwidth rules (overrides PCQ queue)
		s.syncSubscriberCDNOverride(client, &sub, session.Address)
	}
}

// syncSubscriberCDNOverride checks for active CDN bandwidth rules and applies overrides
// When a subscriber has an active CDN rule, it creates a per-subscriber queue that
// uses a different PCQ queue type (different speed) than their service default
func (s *QuotaSyncService) syncSubscriberCDNOverride(client *mikrotik.Client, sub *models.Subscriber, subscriberIP string) {
	if sub.ID == 0 || subscriberIP == "" {
		return
	}

	companyName := getQuotaSyncCompanyName()

	// Get active CDN bandwidth rule for this subscriber
	cdnRule := getActiveSubscriberBandwidthRule(sub.ID, models.BandwidthRuleTypeCDN)
	if cdnRule == nil {
		// No active CDN rule - remove any existing override queue
		if err := client.RemoveSubscriberCDNOverrideQueue(sub.Username, companyName); err != nil {
			log.Printf("CDNOverride: Failed to remove override queue for %s: %v", sub.Username, err)
		}
		return
	}

	// Get the CDN name from the rule
	if cdnRule.CDNName == "" || cdnRule.CDNID == 0 {
		log.Printf("CDNOverride: Rule for %s has no CDN configured", sub.Username)
		return
	}

	// Get the speed from the rule (stored in kbps, need to convert to Mbps for PCQ queue type)
	// The DownloadSpeed in CDN rules represents the CDN speed limit
	speedMbps := int64(cdnRule.DownloadSpeed / 1000)
	if speedMbps <= 0 {
		// If no speed specified, try to get it from service CDN config
		var serviceCDN models.ServiceCDN
		if err := database.DB.Where("service_id = ? AND cdn_id = ? AND is_active = ?",
			sub.ServiceID, cdnRule.CDNID, true).First(&serviceCDN).Error; err == nil {
			speedMbps = serviceCDN.SpeedLimit
		}
	}

	if speedMbps <= 0 {
		log.Printf("CDNOverride: No valid speed for CDN rule %d, subscriber %s", cdnRule.ID, sub.Username)
		return
	}

	// Create/update the override queue
	config := mikrotik.SubscriberCDNOverrideConfig{
		SubscriberIP: subscriberIP,
		Username:     sub.Username,
		CDNName:      cdnRule.CDNName,
		SpeedLimitM:  speedMbps,
		CompanyName:  companyName,
	}

	if err := client.SyncSubscriberCDNOverrideQueue(config); err != nil {
		log.Printf("CDNOverride: Failed to apply override for %s to %s at %dM: %v",
			sub.Username, cdnRule.CDNName, speedMbps, err)
	} else {
		log.Printf("CDNOverride: Applied CDN rule for %s: %s at %dM (rule_id=%d, remaining=%s)",
			sub.Username, cdnRule.CDNName, speedMbps, cdnRule.ID, cdnRule.TimeRemaining())
	}
}

// syncSubscriberCDNQueues creates/updates CDN queue rules for a subscriber
// Skips CDNs that have PCQ enabled (those use shared PCQ queues instead)
func (s *QuotaSyncService) syncSubscriberCDNQueues(client *mikrotik.Client, sub *models.Subscriber, subscriberIP string) {
	if sub.ServiceID == 0 || subscriberIP == "" {
		return
	}

	// Get service CDN configurations
	var serviceCDNs []models.ServiceCDN
	database.DB.Preload("CDN").Where("service_id = ? AND is_active = ?", sub.ServiceID, true).Find(&serviceCDNs)

	if len(serviceCDNs) == 0 {
		return
	}

	// Get company name from system preferences
	companyName := getQuotaSyncCompanyName()

	now := getNow()

	// Get active CDN bandwidth rules
	activeCDNBandwidthRules := getActiveCDNBandwidthRules(now)

	// Build CDN config list - skip PCQ-enabled CDNs (they use shared queues)
	var cdnConfigs []mikrotik.SubscriberCDNConfig
	for _, sc := range serviceCDNs {
		// Skip if CDN not loaded
		if sc.CDN == nil {
			continue
		}
		// Skip PCQ-enabled CDNs - they use shared PCQ queues, not per-subscriber queues
		if sc.PCQEnabled {
			log.Printf("CDN: Skipping individual queue for %s/%s - PCQ mode enabled", sub.Username, sc.CDN.Name)
			continue
		}

		if sc.CDN.ID > 0 && sc.SpeedLimit > 0 && sc.CDN.Subnets != "" {
			// Calculate effective speed limit based on time window
			effectiveSpeedLimit := sc.SpeedLimit

			// Check if time-based speed is configured and currently active
			if sc.TimeSpeedRatio > 0 && sc.TimeSpeedRatio != 100 {
				if isCDNWithinTimeWindow(&sc, now) {
					// Apply time-based ratio
					effectiveSpeedLimit = sc.SpeedLimit * int64(sc.TimeSpeedRatio) / 100
					log.Printf("CDN TimeSpeed: %s for %s - applying %d%% ratio (%dM -> %dM) during %02d:%02d-%02d:%02d",
						sc.CDN.Name, sub.Username, sc.TimeSpeedRatio,
						sc.SpeedLimit, effectiveSpeedLimit,
						sc.TimeFromHour, sc.TimeFromMinute, sc.TimeToHour, sc.TimeToMinute)
				}
			}

			// Apply CDN Bandwidth Rule multiplier if active
			if multiplier := getCDNBandwidthMultiplier(activeCDNBandwidthRules, sc.CDNID); multiplier != 100 {
				originalSpeed := effectiveSpeedLimit
				effectiveSpeedLimit = effectiveSpeedLimit * int64(multiplier) / 100
				log.Printf("CDN BandwidthRule: %s for %s - applying %d%% multiplier (%dM -> %dM)",
					sc.CDN.Name, sub.Username, multiplier, originalSpeed, effectiveSpeedLimit)
			}

			cdnConfigs = append(cdnConfigs, mikrotik.SubscriberCDNConfig{
				CDNName:     sc.CDN.Name,
				SpeedLimit:  effectiveSpeedLimit,
				Subnets:     sc.CDN.Subnets,
				CompanyName: companyName,
			})
		}
	}

	if len(cdnConfigs) == 0 {
		return
	}

	// Sync CDN queues to MikroTik
	if err := client.SyncSubscriberCDNQueues(subscriberIP, sub.Username, cdnConfigs); err != nil {
		log.Printf("QuotaSync: Failed to sync CDN queues for %s: %v", sub.Username, err)
	}
}

// isCDNWithinTimeWindow checks if current time is within the CDN's time-based speed window
func isCDNWithinTimeWindow(sc *models.ServiceCDN, now time.Time) bool {
	// Skip if time-based speed is disabled
	if !sc.TimeBasedSpeedEnabled {
		return false
	}
	// Skip if time window not configured (all zeros means no time window)
	if sc.TimeFromHour == 0 && sc.TimeFromMinute == 0 &&
		sc.TimeToHour == 0 && sc.TimeToMinute == 0 {
		return false
	}

	currentHour := now.Hour()
	currentMinute := now.Minute()
	currentTimeMinutes := currentHour*60 + currentMinute

	fromMinutes := sc.TimeFromHour*60 + sc.TimeFromMinute
	toMinutes := sc.TimeToHour*60 + sc.TimeToMinute

	// Handle time ranges that might cross midnight
	if fromMinutes <= toMinutes {
		// Normal range (e.g., 00:00 to 06:00 or 16:00 to 23:00)
		return currentTimeMinutes >= fromMinutes && currentTimeMinutes < toMinutes
	} else {
		// Crosses midnight (e.g., 22:00 to 06:00)
		return currentTimeMinutes >= fromMinutes || currentTimeMinutes < toMinutes
	}
}

// getActiveSubscriberBandwidthRule returns the active bandwidth rule for a subscriber if one exists
// Returns nil if no active rule is found
func getActiveSubscriberBandwidthRule(subscriberID uint, ruleType models.SubscriberBandwidthRuleType) *models.SubscriberBandwidthRule {
	var rules []models.SubscriberBandwidthRule
	database.DB.Where("subscriber_id = ? AND rule_type = ? AND enabled = ?", subscriberID, ruleType, true).
		Order("priority DESC").Find(&rules)

	for _, rule := range rules {
		if rule.IsActiveNow() {
			return &rule
		}
	}
	return nil
}

// getActiveBandwidthRuleForService returns the active time-based bandwidth rule multiplier for a service
// Returns downloadMultiplier, uploadMultiplier (100 = no change, 200 = double speed)
func getActiveBandwidthRuleForService(serviceID uint) (int, int) {
	now := getNow()
	currentTime := now.Format("15:04")
	currentWeekday := int(now.Weekday())

	// Get all enabled bandwidth rules with auto_apply enabled
	var rules []models.BandwidthRule
	if err := database.DB.Where("enabled = ? AND auto_apply = ?", true, true).Order("priority ASC").Find(&rules).Error; err != nil {
		return 100, 100
	}

	for _, rule := range rules {
		// Check if rule applies to this service
		var serviceIDs []uint
		if len(rule.ServiceIDs) > 0 {
			if err := json.Unmarshal(rule.ServiceIDs, &serviceIDs); err != nil {
				continue
			}
		}
		if len(serviceIDs) == 0 {
			continue
		}

		serviceMatches := false
		for _, svcID := range serviceIDs {
			if svcID == serviceID {
				serviceMatches = true
				break
			}
		}
		if !serviceMatches {
			continue
		}

		// Check days of week
		if len(rule.DaysOfWeek) > 0 {
			var days []int
			if err := json.Unmarshal(rule.DaysOfWeek, &days); err == nil && len(days) > 0 {
				dayMatch := false
				for _, day := range days {
					if day == currentWeekday {
						dayMatch = true
						break
					}
				}
				if !dayMatch {
					continue
				}
			}
		}

		// Check time range
		if rule.StartTime == "" || rule.EndTime == "" {
			continue
		}

		isActive := false
		if rule.StartTime <= rule.EndTime {
			isActive = currentTime >= rule.StartTime && currentTime < rule.EndTime
		} else {
			isActive = currentTime >= rule.StartTime || currentTime < rule.EndTime
		}

		if isActive {
			log.Printf("BandwidthRuleMultiplier: Found active rule '%s' for service %d (dl=%d%%, ul=%d%%)",
				rule.Name, serviceID, rule.DownloadMultiplier, rule.UploadMultiplier)
			return rule.DownloadMultiplier, rule.UploadMultiplier
		}
	}

	return 100, 100 // No active rule
}

// applySubscriberBandwidthRule applies a per-subscriber bandwidth rule if active
// Returns true if a rule was applied, false otherwise
// Now also applies time-based bandwidth rule multiplier on top of subscriber rule
func (s *QuotaSyncService) applySubscriberBandwidthRule(client *mikrotik.Client, nas *models.Nas, sub *models.Subscriber, sessionIP, sessionID string) bool {
	rule := getActiveSubscriberBandwidthRule(sub.ID, models.BandwidthRuleTypeInternet)
	if rule == nil {
		// No active rule - check if we need to restore original service speed
		s.restoreOriginalSpeedIfNeeded(client, nas, sub, sessionIP, sessionID)
		return false
	}

	// Start with subscriber's custom bandwidth rule speeds as BASE
	baseDownloadK := int64(rule.DownloadSpeed)
	baseUploadK := int64(rule.UploadSpeed)

	// Check for active time-based bandwidth rules for this subscriber's service
	// If active, multiply the subscriber rule speeds by the bandwidth rule multiplier
	downloadMultiplier, uploadMultiplier := getActiveBandwidthRuleForService(sub.ServiceID)

	// Apply multiplier: FINAL = BASE × (Multiplier / 100)
	// 100% = same speed, 200% = double speed, 150% = 1.5x speed
	finalDownloadK := baseDownloadK * int64(downloadMultiplier) / 100
	finalUploadK := baseUploadK * int64(uploadMultiplier) / 100

	// Format rate limit for MikroTik (upload/download)
	rateLimit := fmt.Sprintf("%dk/%dk", finalUploadK, finalDownloadK)

	// Check if radreply already has the correct speed — skip API/CoA if unchanged
	// This prevents constant MikroTik API + CoA bombardment every 30s cycle
	var radReply models.RadReply
	if err := database.DB.Where("username = ? AND attribute = ?", sub.Username, "Mikrotik-Rate-Limit").First(&radReply).Error; err == nil {
		if radReply.Value == rateLimit {
			// Speed already correct in RADIUS — no need to touch MikroTik
			return true
		}
	}

	if downloadMultiplier != 100 || uploadMultiplier != 100 {
		log.Printf("SubscriberRule: Applying custom bandwidth for %s: base=%dk/%dk × %d%%/%d%% = %s (rule_id=%d)",
			sub.Username, baseUploadK, baseDownloadK, uploadMultiplier, downloadMultiplier, rateLimit, rule.ID)
	} else {
		log.Printf("SubscriberRule: Applying custom bandwidth for %s: %s (rule_id=%d, duration=%s, remaining=%s)",
			sub.Username, rateLimit, rule.ID, rule.Duration, rule.TimeRemaining())
	}

	// Update radreply to ensure RADIUS has correct speed for future reconnects
	result := database.DB.Exec("UPDATE radreply SET value = ? WHERE username = ? AND attribute = ?",
		rateLimit, sub.Username, "Mikrotik-Rate-Limit")
	if result.Error != nil {
		log.Printf("SubscriberRule: Failed to update radreply for %s: %v", sub.Username, result.Error)
	} else {
		log.Printf("SubscriberRule: Updated radreply for %s to %s (rows=%d)", sub.Username, rateLimit, result.RowsAffected)
	}

	// Apply speed change - try MikroTik API first, then CoA
	coaClient := radius.NewCOAClient(nas.IPAddress, nas.CoAPort, nas.Secret)

	if err := client.UpdateUserRateLimitWithIP(sub.Username, sessionIP, int(finalDownloadK), int(finalUploadK)); err != nil {
		log.Printf("SubscriberRule: MikroTik API failed for %s: %v, trying CoA", sub.Username, err)
		if err := coaClient.UpdateRateLimitViaRadclient(sub.Username, sessionID, rateLimit); err != nil {
			log.Printf("SubscriberRule: CoA also failed for %s: %v", sub.Username, err)
		}
	}

	return true
}

// restoreOriginalSpeedIfNeeded checks if speed needs to be restored to original service speed
// Now also considers active time-based bandwidth rules to avoid restoring during NIGHT hours etc.
func (s *QuotaSyncService) restoreOriginalSpeedIfNeeded(client *mikrotik.Client, nas *models.Nas, sub *models.Subscriber, sessionIP, sessionID string) {
	// Don't restore if subscriber is in FUP - the reduced speed is intentional
	if sub.FUPLevel > 0 || sub.MonthlyFUPLevel > 0 {
		return
	}

	// Calculate expected original service speed
	if sub.Service == nil {
		return
	}
	service := sub.Service
	// Speeds are already in kb (e.g., 2000 = 2000k), no conversion needed
	downloadSpeed := service.DownloadSpeed
	uploadSpeed := service.UploadSpeed

	// Check if there's an active time-based bandwidth rule
	// If yes, apply the multiplier to get the expected speed
	downloadMultiplier, uploadMultiplier := getActiveBandwidthRuleForService(sub.ServiceID)
	if downloadMultiplier != 100 || uploadMultiplier != 100 {
		// Apply multiplier to service speeds
		downloadSpeed = downloadSpeed * int64(downloadMultiplier) / 100
		uploadSpeed = uploadSpeed * int64(uploadMultiplier) / 100
	}

	// Use string speeds if available, otherwise use numeric speeds
	var expectedRateLimit string
	if downloadMultiplier == 100 && uploadMultiplier == 100 && (service.DownloadSpeedStr != "" || service.UploadSpeedStr != "") {
		expectedRateLimit = fmt.Sprintf("%s/%s", service.UploadSpeedStr, service.DownloadSpeedStr)
	} else if downloadSpeed > 0 || uploadSpeed > 0 {
		expectedRateLimit = fmt.Sprintf("%dk/%dk", uploadSpeed, downloadSpeed)
	} else {
		return // No speed configured
	}

	// Check current radreply value
	var radReply models.RadReply
	if err := database.DB.Where("username = ? AND attribute = ?", sub.Username, "Mikrotik-Rate-Limit").First(&radReply).Error; err != nil {
		return // No radreply entry
	}

	// If current value matches expected, no need to restore
	if radReply.Value == expectedRateLimit {
		return // Already at correct speed
	}

	// Check if user has any bandwidth rules (active or not)
	var ruleCount int64
	database.DB.Model(&models.SubscriberBandwidthRule{}).
		Where("subscriber_id = ?", sub.ID).
		Count(&ruleCount)

	// If no rules exist, check if current speed looks like it was set by a subscriber rule
	// (i.e., it doesn't match service speed and it's a simple kbps format like "6000k/6000k")
	if ruleCount == 0 {
		// If current speed matches expected, nothing to do
		// If not, it might be FUP or other - we only restore if the value looks like a subscriber rule format
		// Subscriber rules always use format like "6000k/6000k" (kbps)
		// FUP usually uses same format, but we can't distinguish, so let's just restore anyway
		// since we know there are no rules, the speed should be the original service speed
	}

	log.Printf("SubscriberRule: Restoring original speed for %s: %s (current: %s)", sub.Username, expectedRateLimit, radReply.Value)

	// Update radreply with original speed
	result := database.DB.Exec("UPDATE radreply SET value = ? WHERE username = ? AND attribute = ?",
		expectedRateLimit, sub.Username, "Mikrotik-Rate-Limit")
	if result.Error != nil {
		log.Printf("SubscriberRule: Failed to update radreply for %s: %v", sub.Username, result.Error)
	} else {
		log.Printf("SubscriberRule: Updated radreply for %s to %s (rows=%d)", sub.Username, expectedRateLimit, result.RowsAffected)
	}

	// Apply via MikroTik API
	coaClient := radius.NewCOAClient(nas.IPAddress, nas.CoAPort, nas.Secret)
	if err := client.UpdateUserRateLimitWithIP(sub.Username, sessionIP, int(downloadSpeed), int(uploadSpeed)); err != nil {
		log.Printf("SubscriberRule: Failed to restore speed via API for %s: %v, trying CoA", sub.Username, err)
		if err := coaClient.UpdateRateLimitViaRadclient(sub.Username, sessionID, expectedRateLimit); err != nil {
			log.Printf("SubscriberRule: CoA also failed for %s: %v", sub.Username, err)
		}
	}
}

// checkAndEnforceFUP checks quota and enforces multi-tier FUP by changing rate limit
// Supports both Daily FUP (resets daily) and Monthly FUP (resets on renew)
// FUP Levels: 0=Normal, 1=FUP1, 2=FUP2, 3=FUP3, 4=FUP4, 5=FUP5, 6=FUP6
// The effective FUP is the HIGHEST of daily and monthly FUP levels
// Speeds are stored directly in service as Kbps (e.g., 700 = 700k)
// Uses RADIUS CoA to push rate limit changes to active sessions
func (s *QuotaSyncService) checkAndEnforceFUP(client *mikrotik.Client, nas *models.Nas, sub *models.Subscriber, sessionIP, sessionID string, dailyUsed, monthlyUsed int64) {
	if sub.ServiceID == 0 {
		return
	}

	// Check for per-subscriber bandwidth rules first (highest priority)
	if s.applySubscriberBandwidthRule(client, nas, sub, sessionIP, sessionID) {
		// Subscriber has active custom bandwidth rule, skip normal FUP logic
		return
	}

	if sub.Service == nil {
		return
	}
	service := sub.Service

	// Calculate Daily FUP level based on daily usage
	var dailyFUPLevel int
	var dailyFUPDownload, dailyFUPUpload int64

	if service.FUP6Threshold > 0 && dailyUsed >= service.FUP6Threshold && service.FUP6DownloadSpeed > 0 {
		dailyFUPLevel = 6
		dailyFUPDownload = service.FUP6DownloadSpeed
		dailyFUPUpload = service.FUP6UploadSpeed
	} else if service.FUP5Threshold > 0 && dailyUsed >= service.FUP5Threshold && service.FUP5DownloadSpeed > 0 {
		dailyFUPLevel = 5
		dailyFUPDownload = service.FUP5DownloadSpeed
		dailyFUPUpload = service.FUP5UploadSpeed
	} else if service.FUP4Threshold > 0 && dailyUsed >= service.FUP4Threshold && service.FUP4DownloadSpeed > 0 {
		dailyFUPLevel = 4
		dailyFUPDownload = service.FUP4DownloadSpeed
		dailyFUPUpload = service.FUP4UploadSpeed
	} else if service.FUP3Threshold > 0 && dailyUsed >= service.FUP3Threshold && service.FUP3DownloadSpeed > 0 {
		dailyFUPLevel = 3
		dailyFUPDownload = service.FUP3DownloadSpeed
		dailyFUPUpload = service.FUP3UploadSpeed
	} else if service.FUP2Threshold > 0 && dailyUsed >= service.FUP2Threshold && service.FUP2DownloadSpeed > 0 {
		dailyFUPLevel = 2
		dailyFUPDownload = service.FUP2DownloadSpeed
		dailyFUPUpload = service.FUP2UploadSpeed
	} else if service.FUP1Threshold > 0 && dailyUsed >= service.FUP1Threshold && service.FUP1DownloadSpeed > 0 {
		dailyFUPLevel = 1
		dailyFUPDownload = service.FUP1DownloadSpeed
		dailyFUPUpload = service.FUP1UploadSpeed
	}

	// Calculate Monthly FUP level based on monthly usage
	var monthlyFUPLevel int
	var monthlyFUPDownload, monthlyFUPUpload int64

	if service.MonthlyFUP6Threshold > 0 && monthlyUsed >= service.MonthlyFUP6Threshold && service.MonthlyFUP6DownloadSpeed > 0 {
		monthlyFUPLevel = 6
		monthlyFUPDownload = service.MonthlyFUP6DownloadSpeed
		monthlyFUPUpload = service.MonthlyFUP6UploadSpeed
	} else if service.MonthlyFUP5Threshold > 0 && monthlyUsed >= service.MonthlyFUP5Threshold && service.MonthlyFUP5DownloadSpeed > 0 {
		monthlyFUPLevel = 5
		monthlyFUPDownload = service.MonthlyFUP5DownloadSpeed
		monthlyFUPUpload = service.MonthlyFUP5UploadSpeed
	} else if service.MonthlyFUP4Threshold > 0 && monthlyUsed >= service.MonthlyFUP4Threshold && service.MonthlyFUP4DownloadSpeed > 0 {
		monthlyFUPLevel = 4
		monthlyFUPDownload = service.MonthlyFUP4DownloadSpeed
		monthlyFUPUpload = service.MonthlyFUP4UploadSpeed
	} else if service.MonthlyFUP3Threshold > 0 && monthlyUsed >= service.MonthlyFUP3Threshold && service.MonthlyFUP3DownloadSpeed > 0 {
		monthlyFUPLevel = 3
		monthlyFUPDownload = service.MonthlyFUP3DownloadSpeed
		monthlyFUPUpload = service.MonthlyFUP3UploadSpeed
	} else if service.MonthlyFUP2Threshold > 0 && monthlyUsed >= service.MonthlyFUP2Threshold && service.MonthlyFUP2DownloadSpeed > 0 {
		monthlyFUPLevel = 2
		monthlyFUPDownload = service.MonthlyFUP2DownloadSpeed
		monthlyFUPUpload = service.MonthlyFUP2UploadSpeed
	} else if service.MonthlyFUP1Threshold > 0 && monthlyUsed >= service.MonthlyFUP1Threshold && service.MonthlyFUP1DownloadSpeed > 0 {
		monthlyFUPLevel = 1
		monthlyFUPDownload = service.MonthlyFUP1DownloadSpeed
		monthlyFUPUpload = service.MonthlyFUP1UploadSpeed
	}

	// Determine effective FUP: pick the SLOWER speed (more restrictive)
	var targetFUPLevel int
	var fupDownload, fupUpload int64
	var fupSource string

	if dailyFUPLevel > 0 && monthlyFUPLevel > 0 {
		// Both daily and monthly FUP active — pick the SLOWER speed
		if monthlyFUPDownload > 0 && monthlyFUPDownload < dailyFUPDownload {
			// Monthly FUP is slower — use it
			fupDownload = monthlyFUPDownload
			fupUpload = monthlyFUPUpload
			fupSource = "monthly"
		} else {
			// Daily FUP is slower (or equal) — use it
			fupDownload = dailyFUPDownload
			fupUpload = dailyFUPUpload
			fupSource = "daily"
		}
		// Use the higher level number for DB tracking
		if monthlyFUPLevel > dailyFUPLevel {
			targetFUPLevel = monthlyFUPLevel
		} else {
			targetFUPLevel = dailyFUPLevel
		}
	} else if dailyFUPLevel > 0 {
		targetFUPLevel = dailyFUPLevel
		fupDownload = dailyFUPDownload
		fupUpload = dailyFUPUpload
		fupSource = "daily"
	} else if monthlyFUPLevel > 0 {
		targetFUPLevel = monthlyFUPLevel
		fupDownload = monthlyFUPDownload
		fupUpload = monthlyFUPUpload
		fupSource = "monthly"
	}

	// Check if block mode is enabled for the FUP source
	// If enabled, set speed to 1k/1k to effectively block internet
	shouldBlock := false
	if fupSource == "daily" && isBlockOnDailyQuotaExceeded() {
		shouldBlock = true
	} else if fupSource == "monthly" && isBlockOnMonthlyQuotaExceeded() {
		shouldBlock = true
	}

	if shouldBlock && targetFUPLevel > 0 {
		log.Printf("FUP: Block mode enabled for %s - blocking internet for %s", fupSource, sub.Username)
		fupDownload = 1 // 1 kbps - effectively no internet
		fupUpload = 1   // 1 kbps - effectively no internet
	}

	// Get OLD effective level before any updates (max of stored daily and monthly)
	oldEffectiveLevel := sub.FUPLevel
	if sub.MonthlyFUPLevel > oldEffectiveLevel {
		oldEffectiveLevel = sub.MonthlyFUPLevel
	}

	// Update individual FUP levels in database if changed
	if dailyFUPLevel != sub.FUPLevel {
		database.DB.Model(sub).Update("fup_level", dailyFUPLevel)
	}
	if monthlyFUPLevel != sub.MonthlyFUPLevel {
		database.DB.Model(sub).Update("monthly_fup_level", monthlyFUPLevel)
	}

	// Check if effective FUP level needs to change (compare NEW target vs OLD effective)
	if targetFUPLevel != oldEffectiveLevel {
		if targetFUPLevel > 0 && fupDownload > 0 {
			// Apply FUP
			if fupSource == "daily" {
				log.Printf("FUP: %s daily usage %.2f GB, applying Daily FUP%d",
					sub.Username,
					float64(dailyUsed)/1024/1024/1024,
					targetFUPLevel,
				)
			} else {
				log.Printf("FUP: %s monthly usage %.2f GB, applying Monthly FUP%d",
					sub.Username,
					float64(monthlyUsed)/1024/1024/1024,
					targetFUPLevel,
				)
			}

			// Apply FUP rate limit (speeds are in Kbps, format as "upload/download" for MikroTik rx/tx)
			fupRateLimit := fmt.Sprintf("%dk/%dk", fupUpload, fupDownload)

			// Update RADIUS radreply table
			database.DB.Model(&models.RadReply{}).
				Where("username = ? AND attribute = ?", sub.Username, "Mikrotik-Rate-Limit").
				Update("value", fupRateLimit)

			// Try multiple methods to change speed without disconnect
			coaClient := radius.NewCOAClient(nas.IPAddress, nas.CoAPort, nas.Secret)
			speedChanged := false

			// Method 1: Try MikroTik API to update queue directly (works for static queues)
			if err := client.UpdateUserRateLimitWithIP(sub.Username, sessionIP, int(fupDownload), int(fupUpload)); err != nil {
				log.Printf("FUP: MikroTik API queue update failed for %s: %v", sub.Username, err)

				// Method 2: Send CoA to update session's rate-limit attribute
				// Don't remove queue or disconnect - user keeps current speed until reconnect
				log.Printf("FUP: Trying CoA for %s", sub.Username)
				if err := coaClient.UpdateRateLimitViaRadclient(sub.Username, sessionID, fupRateLimit); err != nil {
					log.Printf("FUP: CoA failed for %s: %v - speed will apply on reconnect", sub.Username, err)
				} else {
					log.Printf("FUP: CoA sent successfully for %s - speed will apply on reconnect", sub.Username)
					speedChanged = true
				}
			} else {
				log.Printf("FUP: Changed %s speed via MikroTik API queue", sub.Username)
				speedChanged = true
			}

			// Don't disconnect users - they keep current speed until reconnect
			// radreply is already updated, so next login will get FUP speed
			if !speedChanged {
				log.Printf("FUP: Speed update pending for %s - will apply on reconnect", sub.Username)
			}
			log.Printf("FUP: Applied %s to %s (%s FUP%d)", fupRateLimit, sub.Username, fupSource, targetFUPLevel)
		// Fire communication rules with trigger_event='fup_applied'
		{
			var quotaUsed, quotaTotal int64
			if fupSource == "daily" {
				quotaUsed = dailyUsed
				switch targetFUPLevel {
				case 1:
					quotaTotal = service.FUP1Threshold
				case 2:
					quotaTotal = service.FUP2Threshold
				case 3:
					quotaTotal = service.FUP3Threshold
				case 4:
					quotaTotal = service.FUP4Threshold
				case 5:
					quotaTotal = service.FUP5Threshold
				case 6:
					quotaTotal = service.FUP6Threshold
				}
			} else {
				quotaUsed = monthlyUsed
				switch targetFUPLevel {
				case 1:
					quotaTotal = service.MonthlyFUP1Threshold
				case 2:
					quotaTotal = service.MonthlyFUP2Threshold
				case 3:
					quotaTotal = service.MonthlyFUP3Threshold
				case 4:
					quotaTotal = service.MonthlyFUP4Threshold
				case 5:
					quotaTotal = service.MonthlyFUP5Threshold
				case 6:
					quotaTotal = service.MonthlyFUP6Threshold
				}
			}
			go fireFUPAppliedRules(sub, targetFUPLevel, quotaUsed, quotaTotal)
		}
		} else if targetFUPLevel == 0 && oldEffectiveLevel > 0 {
			// Restore original speed (both daily and monthly FUP cleared)
			log.Printf("FUP: %s all FUP cleared, restoring original speed", sub.Username)

			// Restore original speed in RADIUS radreply table (kb format, format: upload/download)
			// Speeds are already in kb (e.g., 2000 = 2000k), no conversion needed
			originalRateLimitK := fmt.Sprintf("%dk/%dk", service.UploadSpeed, service.DownloadSpeed)
			database.DB.Model(&models.RadReply{}).
				Where("username = ? AND attribute = ?", sub.Username, "Mikrotik-Rate-Limit").
				Update("value", originalRateLimitK)

			// CoA rate limit in kbps format (format: upload/download)

			// Try multiple methods to restore speed without disconnect
			coaClient := radius.NewCOAClient(nas.IPAddress, nas.CoAPort, nas.Secret)
			speedRestored := false

			// Method 1: Try MikroTik API to restore queue directly
			if err := client.RestoreUserSpeedWithIP(sub.Username, sessionIP, service.DownloadSpeed, service.UploadSpeed); err != nil {
				log.Printf("FUP: MikroTik API queue restore failed for %s: %v", sub.Username, err)

				// Method 2: Send CoA to update session's rate-limit attribute
				// Don't remove queue or disconnect - user keeps current speed until reconnect
				log.Printf("FUP: Trying CoA for %s", sub.Username)
				if err := coaClient.UpdateRateLimitViaRadclient(sub.Username, sessionID, originalRateLimitK); err != nil {
					log.Printf("FUP: CoA restore failed for %s: %v - speed will apply on reconnect", sub.Username, err)
				} else {
					log.Printf("FUP: CoA sent successfully for %s - speed will apply on reconnect", sub.Username)
					speedRestored = true
				}
			} else {
				log.Printf("FUP: Restored %s speed via MikroTik API queue", sub.Username)
				speedRestored = true
			}

			// Don't disconnect users - they keep current speed until reconnect
			// radreply is already updated, so next login will get original speed
			if !speedRestored {
				log.Printf("FUP: Speed restore pending for %s - will apply on reconnect", sub.Username)
			}
			log.Printf("FUP: Restored speed for %s to %s", sub.Username, originalRateLimitK)
		}
	} else if targetFUPLevel > 0 && fupDownload > 0 && targetFUPLevel == oldEffectiveLevel {
		// FUP level unchanged but verify radreply still has the correct FUP speed
		expectedFUPRate := fmt.Sprintf("%dk/%dk", fupUpload, fupDownload)
		var radReply models.RadReply
		if err := database.DB.Where("username = ? AND attribute = ?", sub.Username, "Mikrotik-Rate-Limit").First(&radReply).Error; err == nil {
			if radReply.Value != expectedFUPRate {
				log.Printf("FUP: Re-enforcing %s for %s (radreply was %s)", expectedFUPRate, sub.Username, radReply.Value)
				database.DB.Model(&models.RadReply{}).
					Where("username = ? AND attribute = ?", sub.Username, "Mikrotik-Rate-Limit").
					Update("value", expectedFUPRate)
				coaClient := radius.NewCOAClient(nas.IPAddress, nas.CoAPort, nas.Secret)
				if err := client.UpdateUserRateLimitWithIP(sub.Username, sessionIP, int(fupDownload), int(fupUpload)); err != nil {
					log.Printf("FUP: Re-enforce API failed for %s: %v, trying CoA", sub.Username, err)
					coaClient.UpdateRateLimitViaRadclient(sub.Username, sessionID, expectedFUPRate)
				}
			}
		}
	}
}

// SyncSubscriberQuota syncs quota for a single subscriber (called from API)
func SyncSubscriberQuota(subscriber *models.Subscriber) error {
	if subscriber.Nas == nil || !subscriber.IsOnline {
		return nil
	}

	client := mikrotik.NewClient(
		fmt.Sprintf("%s:%d", subscriber.Nas.IPAddress, subscriber.Nas.APIPort),
		subscriber.Nas.APIUsername,
		subscriber.Nas.APIPassword,
	)
	defer client.Close()

	session, err := client.GetActiveSession(subscriber.Username)
	if err != nil {
		return err
	}

	now := getNow()
	thisMonth := now.Format("2006-01")

	// After MikroTik parsing: TxBytes = client download, RxBytes = client upload
	currentDownload := session.TxBytes
	currentUpload := session.RxBytes

	// Calculate delta
	var deltaDownload, deltaUpload int64
	if currentDownload >= subscriber.LastSessionDownload {
		deltaDownload = currentDownload - subscriber.LastSessionDownload
	} else {
		deltaDownload = currentDownload
	}
	if currentUpload >= subscriber.LastSessionUpload {
		deltaUpload = currentUpload - subscriber.LastSessionUpload
	} else {
		deltaUpload = currentUpload
	}

	// Update daily quota
	if shouldResetDailyQuota(subscriber.LastDailyReset, now) {
		subscriber.DailyDownloadUsed = deltaDownload
		subscriber.DailyUploadUsed = deltaUpload
		subscriber.LastDailyReset = &now
	} else {
		subscriber.DailyDownloadUsed += deltaDownload
		subscriber.DailyUploadUsed += deltaUpload
	}

	// Update monthly quota
	if subscriber.LastMonthlyReset == nil || subscriber.LastMonthlyReset.Format("2006-01") != thisMonth {
		subscriber.MonthlyDownloadUsed = deltaDownload
		subscriber.MonthlyUploadUsed = deltaUpload
		subscriber.LastMonthlyReset = &now
	} else {
		subscriber.MonthlyDownloadUsed += deltaDownload
		subscriber.MonthlyUploadUsed += deltaUpload
	}

	// Update session tracking
	subscriber.LastSessionDownload = currentDownload
	subscriber.LastSessionUpload = currentUpload
	subscriber.LastQuotaSync = &now
	subscriber.DailyQuotaUsed = subscriber.DailyDownloadUsed + subscriber.DailyUploadUsed
	subscriber.MonthlyQuotaUsed = subscriber.MonthlyDownloadUsed + subscriber.MonthlyUploadUsed

	return database.DB.Save(subscriber).Error
}

// isWithinTimeWindow checks if the current time falls within the service's free hours window
func isWithinTimeWindow(service *models.Service, now time.Time) bool {
	// Skip if free hours is disabled
	if !service.TimeBasedSpeedEnabled {
		return false
	}
	// Skip if time window not configured (both from and to are 00:00)
	if service.TimeFromHour == 0 && service.TimeFromMinute == 0 &&
		service.TimeToHour == 0 && service.TimeToMinute == 0 {
		return false
	}

	currentHour := now.Hour()
	currentMinute := now.Minute()
	currentTimeMinutes := currentHour*60 + currentMinute

	fromMinutes := service.TimeFromHour*60 + service.TimeFromMinute
	toMinutes := service.TimeToHour*60 + service.TimeToMinute

	// Handle time ranges that might cross midnight
	if fromMinutes <= toMinutes {
		// Normal range (e.g., 00:00 to 06:00 or 16:00 to 23:00)
		return currentTimeMinutes >= fromMinutes && currentTimeMinutes < toMinutes
	} else {
		// Crosses midnight (e.g., 22:00 to 06:00)
		return currentTimeMinutes >= fromMinutes || currentTimeMinutes < toMinutes
	}
}

// checkAndApplyTimeBasedSpeed tracks time-based speed state for Free Hours.
// Since v1.0.246, the time-based ratio represents "Free Hours — Quota Discount"
// (how much quota is FREE during the time window), NOT a speed boost.
// The actual quota discount is applied in the delta calculation (freePercent logic).
// Speed boosts should use the separate Bandwidth Rules feature.
func (s *QuotaSyncService) checkAndApplyTimeBasedSpeed(client *mikrotik.Client, nas *models.Nas, sub *models.Subscriber, sessionIP, sessionID string) {
	if sub.ServiceID == 0 || sub.Service == nil {
		return
	}

	service := sub.Service
	now := getNow()
	inTimeWindow := isWithinTimeWindow(service, now)

	// Only track state — no speed changes. The quota discount logic in the
	// main sync loop uses isWithinTimeWindow() directly, so this state tracking
	// is just for consistency. No MikroTik API/CoA/queue operations needed.
	s.mu.Lock()
	if inTimeWindow {
		s.timeBasedSpeedState[sub.Username] = &TimeSpeedState{
			Applied:   true,
			SessionID: sessionID,
		}
	} else {
		delete(s.timeBasedSpeedState, sub.Username)
	}
	s.mu.Unlock()
	return

	// === LEGACY CODE BELOW (kept for reference, unreachable) ===

	// Check if subscriber has a subscriber bandwidth rule
	// If yes, the speed is already set correctly (with global bandwidth rule multiplier applied)
	// Service TimeSpeed should use the subscriber rule speed as base, not service speed
	subscriberRule := getActiveSubscriberBandwidthRule(sub.ID, models.BandwidthRuleTypeInternet)
	hasSubscriberRule := subscriberRule != nil

	_ = subscriberRule
	_ = hasSubscriberRule
	now = getNow()
	inTimeWindow = isWithinTimeWindow(service, now)

	// Get current state and check if session changed
	s.mu.RLock()
	state := s.timeBasedSpeedState[sub.Username]
	s.mu.RUnlock()

	wasApplied := false
	sessionChanged := false
	if state != nil {
		wasApplied = state.Applied
		// If session ID changed, treat as new session - need to reapply speed
		if state.SessionID != sessionID {
			sessionChanged = true
			wasApplied = false // Force reapply for new session
			log.Printf("TimeSpeed: Session changed for %s (old=%s, new=%s), will reapply speed",
				sub.Username, state.SessionID, sessionID)
		}
	}

	// Calculate effective FUP level
	effectiveFUPLevel := sub.FUPLevel
	if sub.MonthlyFUPLevel > effectiveFUPLevel {
		effectiveFUPLevel = sub.MonthlyFUPLevel
	}

	// Calculate base speed
	// Priority: 1. Subscriber Bandwidth Rule, 2. FUP speed, 3. Service normal speed
	var baseDownloadK, baseUploadK int64
	if hasSubscriberRule {
		// Use subscriber's custom rule as base
		// Check if there's an active global bandwidth rule
		downloadMultiplier, uploadMultiplier := getActiveBandwidthRuleForService(sub.ServiceID)
		if downloadMultiplier != 100 || uploadMultiplier != 100 {
			// Global bandwidth rule is active - skip Service TimeSpeed for this subscriber
			// because applySubscriberBandwidthRule() already applied the multiplier
			// This prevents double-boosting
			log.Printf("TimeSpeed: Skipping for %s - global bandwidth rule already applied (dl=%d%%, ul=%d%%)",
				sub.Username, downloadMultiplier, uploadMultiplier)
			s.mu.Lock()
			s.timeBasedSpeedState[sub.Username] = &TimeSpeedState{
				Applied:   true, // Mark as applied to prevent repeated processing
				SessionID: sessionID,
			}
			s.mu.Unlock()
			return
		}
		// No global bandwidth rule - use subscriber rule as base for Service TimeSpeed
		baseDownloadK = int64(subscriberRule.DownloadSpeed)
		baseUploadK = int64(subscriberRule.UploadSpeed)
	} else if effectiveFUPLevel > 0 {
		// Use FUP speed as base
		switch effectiveFUPLevel {
		case 1:
			baseDownloadK = service.FUP1DownloadSpeed
			baseUploadK = service.FUP1UploadSpeed
		case 2:
			baseDownloadK = service.FUP2DownloadSpeed
			baseUploadK = service.FUP2UploadSpeed
		case 3:
			baseDownloadK = service.FUP3DownloadSpeed
			baseUploadK = service.FUP3UploadSpeed
		case 4:
			baseDownloadK = service.FUP4DownloadSpeed
			baseUploadK = service.FUP4UploadSpeed
		case 5:
			baseDownloadK = service.FUP5DownloadSpeed
			baseUploadK = service.FUP5UploadSpeed
		case 6:
			baseDownloadK = service.FUP6DownloadSpeed
			baseUploadK = service.FUP6UploadSpeed
		default:
			// Speeds are already in kb, no conversion needed
			baseDownloadK = int64(service.DownloadSpeed)
			baseUploadK = int64(service.UploadSpeed)
		}
	} else {
		// Normal speed (already in kb)
		baseDownloadK = int64(service.DownloadSpeed)
		baseUploadK = int64(service.UploadSpeed)
	}

	if inTimeWindow && !wasApplied {
		// Apply time-based speed BOOST to base speed
		// Ratio is a BOOST percentage: 100% = double speed, 200% = triple, 0% = no change
		// Formula: base_speed * (100 + boost_percent) / 100
		downloadK := baseDownloadK * (100 + int64(service.TimeDownloadRatio)) / 100
		uploadK := baseUploadK * (100 + int64(service.TimeUploadRatio)) / 100
		rateLimit := fmt.Sprintf("%dk/%dk", uploadK, downloadK)

		log.Printf("TimeSpeed: Applying time-based speed BOOST for %s (%02d:%02d-%02d:%02d, boost: dl=+%d%% ul=+%d%%, FUP=%d) -> %s",
			sub.Username,
			service.TimeFromHour, service.TimeFromMinute,
			service.TimeToHour, service.TimeToMinute,
			service.TimeDownloadRatio, service.TimeUploadRatio,
			effectiveFUPLevel,
			rateLimit,
		)

		// Update radreply to ensure RADIUS has correct speed for future reconnects
		database.DB.Model(&models.RadReply{}).
			Where("username = ? AND attribute = ?", sub.Username, "Mikrotik-Rate-Limit").
			Update("value", rateLimit)

		// Apply speed change - try MikroTik API first, then CoA with queue recreation
		speedChanged := false
		coaClient := radius.NewCOAClient(nas.IPAddress, nas.CoAPort, nas.Secret)

		// Method 1: MikroTik API (directly updates the queue - works for static queues)
		if err := client.UpdateUserRateLimitWithIP(sub.Username, sessionIP, int(downloadK), int(uploadK)); err != nil {
			log.Printf("TimeSpeed: MikroTik API failed for %s: %v", sub.Username, err)

			// Method 2: CoA + queue recreation for RADIUS users with dynamic queues
			log.Printf("TimeSpeed: Trying CoA + queue recreation for %s", sub.Username)
			if err := coaClient.UpdateRateLimitViaRadclient(sub.Username, sessionID, rateLimit); err != nil {
				log.Printf("TimeSpeed: CoA also failed for %s: %v", sub.Username, err)
			} else {
				log.Printf("TimeSpeed: CoA sent successfully for %s", sub.Username)
				// Remove dynamic queue so MikroTik recreates with new rate
				if err := client.RemoveDynamicQueueForRecreation(sub.Username); err != nil {
					log.Printf("TimeSpeed: Queue removal failed for %s: %v (speed may take effect on reconnect)", sub.Username, err)
				} else {
					log.Printf("TimeSpeed: Dynamic queue removed, MikroTik will recreate with new rate")
				}
				speedChanged = true
			}
		} else {
			log.Printf("TimeSpeed: Applied %s speed via MikroTik API", sub.Username)
			speedChanged = true
		}

		// Method 3: Disconnect as last resort
		if !speedChanged {
			log.Printf("TimeSpeed: All rate-limit methods failed for %s, disconnecting user", sub.Username)
			if err := coaClient.DisconnectViaRadclient(sub.Username, sessionID); err != nil {
				log.Printf("TimeSpeed: Radclient disconnect failed for %s: %v, trying MikroTik API", sub.Username, err)
				if err := client.DisconnectUser(sub.Username); err != nil {
					log.Printf("TimeSpeed: All disconnect methods failed for %s: %v", sub.Username, err)
					return
				}
				log.Printf("TimeSpeed: Disconnected %s via MikroTik API", sub.Username)
			} else {
				log.Printf("TimeSpeed: Disconnected %s via radclient", sub.Username)
			}
		}

		s.mu.Lock()
		s.timeBasedSpeedState[sub.Username] = &TimeSpeedState{
			Applied:   true,
			SessionID: sessionID,
		}
		s.mu.Unlock()
	} else if !inTimeWindow && (wasApplied || sessionChanged) {
		// Restore base speed (FUP or normal) - format: upload/download for MikroTik rx/tx
		rateLimit := fmt.Sprintf("%dk/%dk", baseUploadK, baseDownloadK)

		log.Printf("TimeSpeed: Restoring speed for %s (time window ended, FUP=%d) -> %s",
			sub.Username, effectiveFUPLevel, rateLimit,
		)

		// Update radreply to ensure RADIUS has correct speed for future reconnects
		database.DB.Model(&models.RadReply{}).
			Where("username = ? AND attribute = ?", sub.Username, "Mikrotik-Rate-Limit").
			Update("value", rateLimit)

		// Restore speed - try MikroTik API first, then CoA with queue recreation
		speedRestored := false
		coaClient := radius.NewCOAClient(nas.IPAddress, nas.CoAPort, nas.Secret)

		// Method 1: MikroTik API (directly updates the queue - works for static queues)
		if err := client.UpdateUserRateLimitWithIP(sub.Username, sessionIP, int(baseDownloadK), int(baseUploadK)); err != nil {
			log.Printf("TimeSpeed: MikroTik API restore failed for %s: %v", sub.Username, err)

			// Method 2: CoA + queue recreation for RADIUS users with dynamic queues
			log.Printf("TimeSpeed: Trying CoA + queue recreation for %s", sub.Username)
			if err := coaClient.UpdateRateLimitViaRadclient(sub.Username, sessionID, rateLimit); err != nil {
				log.Printf("TimeSpeed: CoA restore also failed for %s: %v", sub.Username, err)
			} else {
				log.Printf("TimeSpeed: CoA sent successfully for %s", sub.Username)
				// Remove dynamic queue so MikroTik recreates with new rate
				if err := client.RemoveDynamicQueueForRecreation(sub.Username); err != nil {
					log.Printf("TimeSpeed: Queue removal failed for %s: %v (speed may take effect on reconnect)", sub.Username, err)
				} else {
					log.Printf("TimeSpeed: Dynamic queue removed, MikroTik will recreate with restored rate")
				}
				speedRestored = true
			}
		} else {
			log.Printf("TimeSpeed: Restored %s speed via MikroTik API", sub.Username)
			speedRestored = true
		}

		// Method 3: Disconnect as last resort
		if !speedRestored {
			log.Printf("TimeSpeed: All restore methods failed for %s, disconnecting user", sub.Username)
			if err := coaClient.DisconnectViaRadclient(sub.Username, sessionID); err != nil {
				log.Printf("TimeSpeed: Radclient disconnect failed for %s: %v, trying MikroTik API", sub.Username, err)
				if err := client.DisconnectUser(sub.Username); err != nil {
					log.Printf("TimeSpeed: All disconnect methods failed for %s: %v", sub.Username, err)
					return
				}
				log.Printf("TimeSpeed: Disconnected %s via MikroTik API", sub.Username)
			} else {
				log.Printf("TimeSpeed: Disconnected %s via radclient", sub.Username)
			}
		}

		s.mu.Lock()
		s.timeBasedSpeedState[sub.Username] = &TimeSpeedState{
			Applied:   false,
			SessionID: sessionID,
		}
		s.mu.Unlock()
	} else if state == nil || state.SessionID != sessionID {
		// New session but not in time window and no previous state - just track it
		s.mu.Lock()
		s.timeBasedSpeedState[sub.Username] = &TimeSpeedState{
			Applied:   false,
			SessionID: sessionID,
		}
		s.mu.Unlock()
	}
}

// getActiveCDNBandwidthRules returns all currently active CDN bandwidth rules
func getActiveCDNBandwidthRules(now time.Time) []CDNBandwidthRule {
	currentTime := now.Format("15:04")
	currentWeekday := int(now.Weekday())

	var rules []CDNBandwidthRule
	if err := database.DB.Where("enabled = ? AND auto_apply = ?", true, true).Order("priority ASC").Find(&rules).Error; err != nil {
		return nil
	}

	var activeRules []CDNBandwidthRule
	for _, rule := range rules {
		if isCDNBandwidthRuleActive(&rule, currentTime, currentWeekday) {
			activeRules = append(activeRules, rule)
		}
	}

	return activeRules
}

// isCDNBandwidthRuleActive checks if a CDN bandwidth rule should be active at the current time
func isCDNBandwidthRuleActive(rule *CDNBandwidthRule, currentTime string, currentWeekday int) bool {
	// Check days of week
	if len(rule.DaysOfWeek) > 0 {
		var days []int
		if err := json.Unmarshal(rule.DaysOfWeek, &days); err == nil && len(days) > 0 {
			dayMatch := false
			for _, day := range days {
				if day == currentWeekday {
					dayMatch = true
					break
				}
			}
			if !dayMatch {
				return false
			}
		}
	}

	// Check time range
	if rule.StartTime == "" || rule.EndTime == "" {
		return false
	}

	// Handle time ranges that might cross midnight
	if rule.StartTime <= rule.EndTime {
		return currentTime >= rule.StartTime && currentTime < rule.EndTime
	} else {
		return currentTime >= rule.StartTime || currentTime < rule.EndTime
	}
}

// getCDNBandwidthMultiplier returns the speed multiplier for a CDN from active bandwidth rules
// Returns 100 if no active rule affects this CDN
func getCDNBandwidthMultiplier(activeRules []CDNBandwidthRule, cdnID uint) int {
	for _, rule := range activeRules {
		// Check if this rule applies to this CDN
		var cdnIDs []uint
		if len(rule.CDNIDs) > 0 {
			if err := json.Unmarshal(rule.CDNIDs, &cdnIDs); err != nil {
				continue
			}
		}

		// If no CDNs specified, skip (we require CDN selection now)
		if len(cdnIDs) == 0 {
			continue
		}

		// Check if this CDN is in the rule's CDN list
		for _, id := range cdnIDs {
			if id == cdnID {
				return rule.SpeedMultiplier
			}
		}
	}

	return 100 // No active rule, return normal speed
}

// detectAndResolveStaticIPConflicts checks for users who have an IP that belongs to someone else's static_ip
// and disconnects them so they can reconnect and get a different IP from the pool
func (s *QuotaSyncService) detectAndResolveStaticIPConflicts() {
	// Get all static IPs
	var staticIPUsers []models.Subscriber
	if err := database.DB.Where("static_ip IS NOT NULL AND static_ip != '' AND deleted_at IS NULL").
		Find(&staticIPUsers).Error; err != nil {
		log.Printf("StaticIPConflict: Failed to get static IP users: %v", err)
		return
	}

	if len(staticIPUsers) == 0 {
		return
	}

	// Build a map of static_ip -> owner username
	staticIPOwners := make(map[string]string)
	for _, user := range staticIPUsers {
		if user.StaticIP != "" {
			staticIPOwners[user.StaticIP] = user.Username
		}
	}

	log.Printf("StaticIPConflict: Checking %d static IPs for conflicts", len(staticIPOwners))

	// Get all online users with their current IP (don't use Select with Preload)
	var onlineUsers []models.Subscriber
	if err := database.DB.Preload("Nas").
		Where("is_online = ? AND ip_address IS NOT NULL AND ip_address != '' AND deleted_at IS NULL", true).
		Find(&onlineUsers).Error; err != nil {
		log.Printf("StaticIPConflict: Failed to get online users: %v", err)
		return
	}

	log.Printf("StaticIPConflict: Checking %d online users", len(onlineUsers))

	// Check each online user
	for _, user := range onlineUsers {
		// Skip if user has no IP
		if user.IPAddress == "" {
			continue
		}

		// Check if this IP is someone else's static IP
		owner, isStaticIP := staticIPOwners[user.IPAddress]
		if !isStaticIP {
			continue // This IP is not a static IP, no conflict
		}

		// If the owner is this user, no conflict
		if owner == user.Username {
			continue
		}

		// CONFLICT DETECTED: This user has an IP that belongs to someone else
		log.Printf("StaticIPConflict: User %s has IP %s which is static IP of %s - disconnecting",
			user.Username, user.IPAddress, owner)

		// Get NAS info for disconnect
		if user.Nas == nil || user.NasID == nil {
			// Try to load NAS
			var nas models.Nas
			if user.NasID != nil {
				if err := database.DB.First(&nas, *user.NasID).Error; err != nil {
					log.Printf("StaticIPConflict: Failed to get NAS for user %s: %v", user.Username, err)
					continue
				}
				user.Nas = &nas
			} else {
				log.Printf("StaticIPConflict: User %s has no NAS assigned", user.Username)
				continue
			}
		}

		// Disconnect the user via CoA
		coaClient := radius.NewCOAClient(user.Nas.IPAddress, user.Nas.CoAPort, user.Nas.Secret)
		if err := coaClient.DisconnectViaRadclient(user.Username, ""); err != nil {
			log.Printf("StaticIPConflict: CoA disconnect failed for %s: %v, trying MikroTik API", user.Username, err)

			// Fallback to MikroTik API
			client := mikrotik.NewClient(
				fmt.Sprintf("%s:%d", user.Nas.IPAddress, user.Nas.APIPort),
				user.Nas.APIUsername,
				user.Nas.APIPassword,
			)
			if err := client.DisconnectUser(user.Username); err != nil {
				log.Printf("StaticIPConflict: MikroTik disconnect also failed for %s: %v", user.Username, err)
			} else {
				log.Printf("StaticIPConflict: Disconnected %s via MikroTik API", user.Username)
			}
			client.Close()
		} else {
			log.Printf("StaticIPConflict: Disconnected %s via CoA", user.Username)
		}

		// Clear the IP address in database so it doesn't show as duplicate
		database.DB.Model(&models.Subscriber{}).Where("id = ?", user.ID).Update("ip_address", nil)
	}
}

// fireFUPAppliedRules fires communication rules with trigger_event='fup_applied'.
// Called in a goroutine when a subscriber's FUP level increases.
// quotaUsed/quotaTotal are in bytes; they are formatted as human-readable strings.
func fireFUPAppliedRules(sub *models.Subscriber, fupLevel int, quotaUsed, quotaTotal int64) {
	if sub == nil {
		return
	}

	var rules []models.CommunicationRule
	if err := database.DB.Where("trigger_event = ? AND enabled = ?", "fup_applied", true).Find(&rules).Error; err != nil {
		log.Printf("FUPRule: Failed to query communication rules: %v", err)
		return
	}
	if len(rules) == 0 {
		return
	}

	// Prepare template variables
	expiryDate := ""
	if !sub.ExpiryDate.IsZero() {
		expiryDate = sub.ExpiryDate.Format("2006-01-02")
	}
	serviceName := ""
	if sub.Service != nil {
		serviceName = sub.Service.Name
	}
	balance := fmt.Sprintf("%.2f", sub.Price)
	quotaUsedStr := formatFUPQuota(quotaUsed)
	quotaTotalStr := formatFUPQuota(quotaTotal)

	levelStr := strconv.Itoa(fupLevel)

	for _, rule := range rules {
		if rule.Template == "" {
			continue
		}

		// Filter by configured FUP levels (e.g. "2" means only fire on FUP2)
		if rule.FUPLevels != "" && !strings.Contains(rule.FUPLevels, levelStr) {
			log.Printf("FUPRule[%s]: Skipping for FUP%d (rule configured for levels: %s)", rule.Name, fupLevel, rule.FUPLevels)
			continue
		}

		msg := rule.Template
		msg = strings.ReplaceAll(msg, "{username}", sub.Username)
		msg = strings.ReplaceAll(msg, "{full_name}", sub.FullName)
		msg = strings.ReplaceAll(msg, "{expiry_date}", expiryDate)
		msg = strings.ReplaceAll(msg, "{service_name}", serviceName)
		msg = strings.ReplaceAll(msg, "{balance}", balance)
		msg = strings.ReplaceAll(msg, "{quota_used}", quotaUsedStr)
		msg = strings.ReplaceAll(msg, "{quota_total}", quotaTotalStr)
		msg = strings.ReplaceAll(msg, "{fup_level}", fmt.Sprintf("%d", fupLevel))

		switch rule.Channel {
		case "whatsapp":
			if sub.Phone != "" {
				wa := NewWhatsAppService()
				if err := wa.SendMessageForSubscriber(*sub, sub.Phone, msg); err != nil {
					log.Printf("FUPRule[%s]: WhatsApp send failed for %s: %v", rule.Name, sub.Username, err)
				} else {
					log.Printf("FUPRule[%s]: WhatsApp sent to %s", rule.Name, sub.Username)
				}
			}
		case "sms":
			if sub.Phone != "" {
				sms := NewSMSService()
				if err := sms.SendSMS(sub.Phone, msg); err != nil {
					log.Printf("FUPRule[%s]: SMS send failed for %s: %v", rule.Name, sub.Username, err)
				} else {
					log.Printf("FUPRule[%s]: SMS sent to %s", rule.Name, sub.Username)
				}
			}
		case "email":
			if sub.Email != "" {
				emailSvc := NewEmailService()
				subject := fmt.Sprintf("FUP Level %d Applied - %s", fupLevel, sub.Username)
				if err := emailSvc.SendEmail(sub.Email, subject, msg, false); err != nil {
					log.Printf("FUPRule[%s]: Email send failed for %s: %v", rule.Name, sub.Username, err)
				} else {
					log.Printf("FUPRule[%s]: Email sent to %s", rule.Name, sub.Username)
				}
			}
		}
	}
}

// fireQuotaWarningRules sends notifications when a subscriber's monthly quota reaches a threshold.
// The rule's days_before field is repurposed as the threshold percentage (e.g. 80 = fire at 80% used).
// Deduplication: only fires once per rule per subscriber per month.
func fireQuotaWarningRules(sub models.Subscriber, monthlyUsed int64) {
	if sub.Service == nil {
		return
	}

	// Find the total monthly quota: use the highest non-zero monthly FUP threshold
	var quotaTotal int64
	for _, t := range []int64{sub.Service.MonthlyFUP6Threshold, sub.Service.MonthlyFUP5Threshold, sub.Service.MonthlyFUP4Threshold, sub.Service.MonthlyFUP3Threshold, sub.Service.MonthlyFUP2Threshold, sub.Service.MonthlyFUP1Threshold} {
		if t > 0 {
			quotaTotal = t
			break
		}
	}
	if quotaTotal == 0 || monthlyUsed == 0 {
		return // No monthly quota defined or no usage yet
	}

	usedPercent := monthlyUsed * 100 / quotaTotal

	var rules []models.CommunicationRule
	if err := database.DB.Where("trigger_event = ? AND enabled = ?", "quota_warning", true).Find(&rules).Error; err != nil {
		return
	}
	if len(rules) == 0 {
		return
	}

	now := getNow()
	for _, rule := range rules {
		if rule.Template == "" {
			continue
		}
		threshold := int64(rule.DaysBefore) // days_before repurposed as threshold %
		if threshold <= 0 || threshold > 99 {
			threshold = 80
		}
		if usedPercent < threshold {
			continue
		}
		// Per-month dedup: only send once per rule per subscriber this month
		if quotaWarningAlreadySentThisMonth(sub.ID, rule.ID, now) {
			continue
		}
		log.Printf("QuotaWarning[%s]: %s used %d%% of monthly quota (threshold=%d%%), sending notification",
			rule.Name, sub.Username, usedPercent, threshold)

		serviceName := sub.Service.Name
		balance := fmt.Sprintf("%.2f", sub.Price)
		usedStr := formatFUPQuota(monthlyUsed)
		totalStr := formatFUPQuota(quotaTotal)

		msg := rule.Template
		msg = strings.ReplaceAll(msg, "{username}", sub.Username)
		msg = strings.ReplaceAll(msg, "{full_name}", sub.FullName)
		msg = strings.ReplaceAll(msg, "{service_name}", serviceName)
		msg = strings.ReplaceAll(msg, "{balance}", balance)
		msg = strings.ReplaceAll(msg, "{quota_used}", usedStr)
		msg = strings.ReplaceAll(msg, "{quota_total}", totalStr)
		msg = strings.ReplaceAll(msg, "{quota_percent}", fmt.Sprintf("%d", usedPercent))

		sent := false
		errMsg := ""

		phone := sub.Phone
		if rule.SendToReseller && sub.Reseller != nil && sub.Reseller.User != nil && sub.Reseller.User.Phone != "" {
			phone = sub.Reseller.User.Phone
		}
		email := sub.Email
		if rule.SendToReseller && sub.Reseller != nil && sub.Reseller.User != nil && sub.Reseller.User.Email != "" {
			email = sub.Reseller.User.Email
		}

		switch rule.Channel {
		case "whatsapp":
			if phone != "" {
				wa := NewWhatsAppService()
				if err := wa.SendMessageForSubscriber(sub, phone, msg); err != nil {
					log.Printf("QuotaWarning[%s]: WhatsApp failed for %s: %v", rule.Name, sub.Username, err)
					errMsg = err.Error()
				} else {
					log.Printf("QuotaWarning[%s]: WhatsApp sent to %s", rule.Name, sub.Username)
					sent = true
				}
			}
		case "sms":
			if phone != "" {
				sms := NewSMSService()
				if err := sms.SendSMS(phone, msg); err != nil {
					log.Printf("QuotaWarning[%s]: SMS failed for %s: %v", rule.Name, sub.Username, err)
					errMsg = err.Error()
				} else {
					log.Printf("QuotaWarning[%s]: SMS sent to %s", rule.Name, sub.Username)
					sent = true
				}
			}
		case "email":
			if email != "" {
				emailSvc := NewEmailService()
				subject := fmt.Sprintf("Quota Warning (%d%% used) - %s", usedPercent, sub.Username)
				if err := emailSvc.SendEmail(email, subject, msg, false); err != nil {
					log.Printf("QuotaWarning[%s]: Email failed for %s: %v", rule.Name, sub.Username, err)
					errMsg = err.Error()
				} else {
					log.Printf("QuotaWarning[%s]: Email sent to %s", rule.Name, sub.Username)
					sent = true
				}
			}
		}

		// Log for per-month dedup
		logDailyNotif(rule, sub, msg, sent, errMsg)
	}
}

// quotaWarningAlreadySentThisMonth returns true if quota_warning was already sent for this subscriber+rule this month.
func quotaWarningAlreadySentThisMonth(subscriberID, ruleID uint, now time.Time) bool {
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	var count int64
	database.DB.Model(&models.CommunicationLog{}).
		Where("subscriber_id = ? AND rule_id = ? AND created_at >= ?", subscriberID, ruleID, startOfMonth).
		Count(&count)
	return count > 0
}

// formatFUPQuota formats a byte count as a human-readable string.
func formatFUPQuota(bytes int64) string {
	if bytes <= 0 {
		return "0 MB"
	}
	if bytes >= 1024*1024*1024 {
		return fmt.Sprintf("%.2f GB", float64(bytes)/1024/1024/1024)
	}
	if bytes >= 1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(bytes)/1024/1024)
	}
	return fmt.Sprintf("%d KB", bytes/1024)
}

// ============================================================
// WAN Management Check — blocks new subscribers until router
// has ICMP + management port enabled
// ============================================================

// isWanCheckEnabled reads the wan_check_enabled setting
func isWanCheckEnabled() bool {
	v := database.GetSettingDefault("wan_check_enabled", "")
	return v == "true" || v == "1"
}

// isWanCheckICMPEnabled reads the global wan_check_icmp_enabled setting (default true)
func isWanCheckICMPEnabled() bool {
	v, ok := database.GetSetting("wan_check_icmp_enabled")
	if !ok {
		return true // default: ICMP enabled
	}
	return v != "false" && v != "0"
}

// isWanCheckPortEnabled reads the global wan_check_port_enabled setting (default true)
func isWanCheckPortEnabled() bool {
	v, ok := database.GetSetting("wan_check_port_enabled")
	if !ok {
		return true // default: port check enabled
	}
	return v != "false" && v != "0"
}

// getWanCheckPort reads the wan_check_port setting (default 8084)
func getWanCheckPort() int {
	v := database.GetSettingDefault("wan_check_port", "8084")
	port, err := strconv.Atoi(v)
	if err != nil || port <= 0 || port > 65535 {
		return 8084
	}
	return port
}

// grandfatherExistingSubscribers marks all subscribers that still have
// the default 'unchecked' status as 'ok' (one-time migration when feature
// is first enabled). Only subscribers who are currently online get
// grandfathered; newly connecting ones will be checked normally.
func grandfatherExistingSubscribers() {
	result := database.DB.Model(&models.Subscriber{}).
		Where("wan_check_status = ? AND is_online = ?", "unchecked", true).
		Update("wan_check_status", "ok")
	if result.RowsAffected > 0 {
		log.Printf("WanCheck: Grandfathered %d existing online subscribers as 'ok'", result.RowsAffected)
	}
}

// checkWanManagement performs the WAN management check for a subscriber.
// Returns true if the subscriber is BLOCKED (caller should skip FUP/speed).
// Returns false if check passed or feature is disabled.
//
// Logic: RADIUS starts ALL new users at 1k/1k. This function checks the port
// and RESTORES full speed when it's open. No temp speed restore needed for
// first check (user is already at 1k/1k from RADIUS).
func (s *QuotaSyncService) checkWanManagement(client *mikrotik.Client, nas *models.Nas, sub *models.Subscriber, sessionIP, sessionID string) bool {
	enabled := isWanCheckEnabled()

	// Start with global ICMP/port settings
	runICMP := isWanCheckICMPEnabled()
	runPort := isWanCheckPortEnabled()

	// Per-reseller override: check if reseller has its own WAN check setting
	if sub.ResellerID > 0 {
		if cfg, ok := s.resellerWanMap[sub.ResellerID]; ok {
			if cfg.WanCheckEnabled != nil {
				if !*cfg.WanCheckEnabled {
					// Reseller explicitly disabled WAN check — skip
					if sub.WanCheckStatus != "skipped" {
						database.DB.Model(&models.Subscriber{}).Where("id = ?", sub.ID).
							Updates(map[string]interface{}{"wan_check_status": "skipped", "port_open": false})
					}
					return false
				}
				// Reseller explicitly enabled — override global
				enabled = true
			}
			// Per-reseller ICMP/port toggles override global
			runICMP = cfg.WanCheckICMP
			runPort = cfg.WanCheckPort
		}
	}

	if !enabled {
		s.wanCheckWasEnabled = false
		return false
	}

	// If both ICMP and port are disabled but WAN check is enabled — treat as passed
	if !runICMP && !runPort {
		if sub.WanCheckStatus != "ok" {
			database.DB.Model(&models.Subscriber{}).Where("id = ?", sub.ID).
				Updates(map[string]interface{}{"wan_check_status": "ok", "port_open": false})
			s.restoreOriginalSpeedIfNeeded(client, nas, sub, sessionIP, sessionID)
		}
		return false
	}

	// Detect disabled → enabled transition: grandfather all currently-online
	// subscribers as "ok" so they skip the check. Without this, ALL online
	// subscribers would be checked simultaneously, creating a MikroTik API/CoA
	// storm that can overwhelm the router and destroy manual queues.
	if !s.wanCheckWasEnabled {
		s.wanCheckWasEnabled = true
		grandfatherExistingSubscribers()
		// Skip ALL WAN checks this cycle — subscribers were loaded before
		// grandfathering updated the DB, so their in-memory status is stale.
		// Next cycle (30s) will load fresh "ok" statuses from DB.
		return false
	}

	status := sub.WanCheckStatus

	// Already passed or skipped — nothing to do
	if status == "ok" || status == "skipped" {
		return false
	}

	// For already-failed users, re-check every 1 minute (not every 30s)
	if status == "failed" {
		if lastCheck, ok := s.wanCheckCooldown[sub.ID]; ok {
			if time.Since(lastCheck) < 1*time.Minute {
				return true // still blocked, skip re-check
			}
		}
	}

	// ----- Perform the actual check -----
	port := getWanCheckPort()

	// For failed users being re-checked: temporarily restore speed so
	// /tool/fetch can get through the 1k/1k throttle to test the port.
	// For "unchecked" users: RADIUS already set 1k/1k but /tool/fetch runs
	// from MikroTik (not through the user's connection), so no restore needed.
	if status == "failed" {
		if sub.Service != nil {
			tempRate := fmt.Sprintf("%dk/%dk", sub.Service.UploadSpeed, sub.Service.DownloadSpeed)
			coaClient := radius.NewCOAClient(nas.IPAddress, nas.CoAPort, nas.Secret)
			_ = coaClient.UpdateRateLimitViaRadclient(sub.Username, sessionID, tempRate)
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Step 1: Ping (3 packets) — only if ICMP check enabled
	if runICMP {
		pingResult, err := client.Ping(sessionIP, 3, 0)
		if err != nil || pingResult == nil || pingResult.Received == 0 {
			s.applyWanBlock(client, nas, sub, sessionIP, sessionID, "ICMP unreachable")
			s.wanCheckCooldown[sub.ID] = time.Now()
			return true
		}
	}

	// Step 2: Check management port — only if port check enabled
	// Use direct TCP from API server (MikroTik /tool/fetch can't reach its own PPPoE clients)
	if runPort {
		if !checkPortDirect(sessionIP, port, 3) {
			s.applyWanBlock(client, nas, sub, sessionIP, sessionID, fmt.Sprintf("port %d closed", port))
			s.wanCheckCooldown[sub.ID] = time.Now()
			return true
		}
	}

	// All enabled checks passed — mark as OK and set port_open for icon display
	database.DB.Model(&models.Subscriber{}).Where("id = ?", sub.ID).
		Updates(map[string]interface{}{"wan_check_status": "ok", "port_open": runPort})
	log.Printf("WanCheck: %s PASSED (icmp=%v, port=%v) — restoring full speed", sub.Username, runICMP, runPort)

	// Clean up cooldown entry
	delete(s.wanCheckCooldown, sub.ID)

	// Restore original speed (user was at 1k/1k from RADIUS or from previous block)
	s.restoreOriginalSpeedIfNeeded(client, nas, sub, sessionIP, sessionID)

	return false
}

// checkPortDirect tests if a TCP port is open by connecting from the host network namespace.
// Uses nsenter to escape the Docker container network and reach PPPoE client IPs.
// This replaces MikroTik's /tool/fetch which cannot reach its own PPPoE clients.
func checkPortDirect(ip string, port int, timeoutSec int) bool {
	// First try direct TCP from the container (works if host network or proper routing exists)
	addr := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.DialTimeout("tcp", addr, time.Duration(timeoutSec)*time.Second)
	if err == nil {
		conn.Close()
		return true
	}

	// Fallback: use nsenter to check from host network namespace (for Docker bridge networks)
	// nsenter -t 1 -n runs the command in PID 1's (host init) network namespace
	cmd := exec.Command("nsenter", "-t", "1", "-n", "--",
		"bash", "-c", fmt.Sprintf("timeout %d bash -c 'echo > /dev/tcp/%s/%d' 2>/dev/null", timeoutSec, ip, port))
	if err := cmd.Run(); err == nil {
		return true
	}
	return false
}

// applyWanBlock rate-limits a subscriber to 1k/1k and marks them as failed.
func (s *QuotaSyncService) applyWanBlock(client *mikrotik.Client, nas *models.Nas, sub *models.Subscriber, sessionIP, sessionID, reason string) {
	// Only log on first failure or status change
	if sub.WanCheckStatus != "failed" {
		log.Printf("WanCheck: BLOCKING %s — %s (applying 1k/1k)", sub.Username, reason)
	}

	database.DB.Model(&models.Subscriber{}).Where("id = ?", sub.ID).
		Updates(map[string]interface{}{"wan_check_status": "failed", "port_open": false})

	// Apply 1k/1k via MikroTik API, fallback to CoA
	blockRate := "1k/1k" // upload/download
	if err := client.UpdateUserRateLimitWithIP(sub.Username, sessionIP, 1, 1); err != nil {
		log.Printf("WanCheck: API rate-limit failed for %s: %v, trying CoA", sub.Username, err)
		coaClient := radius.NewCOAClient(nas.IPAddress, nas.CoAPort, nas.Secret)
		if err := coaClient.UpdateRateLimitViaRadclient(sub.Username, sessionID, blockRate); err != nil {
			log.Printf("WanCheck: CoA also failed for %s: %v", sub.Username, err)
		}
	}
}

// --- CDN FUP (Fair Usage Policy for CDN-only traffic) ---

// cdnBytesResult holds estimated CDN bytes for a subscriber from batch torch
type cdnBytesResult struct {
	Download int64
	Upload   int64
}

// runBatchCDNTorch runs a batch torch on the NAS bridge interface, matches flows against
// CDN subnets, and returns estimated CDN bytes per subscriber IP.
func (s *QuotaSyncService) runBatchCDNTorch(client *mikrotik.Client, nas *models.Nas) map[string]cdnBytesResult {
	const torchDuration = 3 // seconds
	const syncInterval = 30 // QuotaSync runs every 30s

	results, err := client.RunBatchCDNTorch(nas.CDNTorchInterface, torchDuration)
	if err != nil {
		log.Printf("CDN-FUP: Batch torch failed on NAS %s interface %s: %v", nas.Name, nas.CDNTorchInterface, err)
		return nil
	}

	if len(results) == 0 {
		return nil
	}

	// Load all active CDN subnets from DB
	var cdns []models.CDN
	database.DB.Where("is_active = ?", true).Find(&cdns)
	if len(cdns) == 0 {
		return nil
	}

	// Pre-parse CDN subnets into net.IPNet for fast matching
	type parsedCDN struct {
		nets []*net.IPNet
	}
	var parsedCDNs []parsedCDN
	for _, cdn := range cdns {
		var nets []*net.IPNet
		for _, s := range strings.FieldsFunc(cdn.Subnets, func(r rune) bool {
			return r == ',' || r == '\n' || r == ';' || r == ' '
		}) {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			_, ipNet, err := net.ParseCIDR(s)
			if err != nil {
				continue
			}
			nets = append(nets, ipNet)
		}
		parsedCDNs = append(parsedCDNs, parsedCDN{nets: nets})
	}

	isCDNIP := func(ipStr string) bool {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			return false
		}
		for _, pcdn := range parsedCDNs {
			for _, ipNet := range pcdn.nets {
				if ipNet.Contains(ip) {
					return true
				}
			}
		}
		return false
	}

	// Also need to know which IPs are subscriber IPs (to determine direction)
	// On the bridge, subscriber traffic appears as src=subscriber for upload, dst=subscriber for download
	// We match the OTHER side against CDN subnets
	byIP := make(map[string]cdnBytesResult)

	for _, flow := range results {
		// For each flow, one side is the subscriber and the other is remote
		// tx = bits going out of the interface (download TO subscriber when matched by subscriber IP)
		srcIsCDN := isCDNIP(flow.SrcAddress)
		dstIsCDN := isCDNIP(flow.DstAddress)

		if srcIsCDN && !dstIsCDN {
			// CDN -> subscriber: flow.DstAddress is subscriber, this is download
			entry := byIP[flow.DstAddress]
			entry.Download += flow.TxRate * syncInterval / 8 // bits/s * interval / 8 = bytes
			byIP[flow.DstAddress] = entry
		} else if dstIsCDN && !srcIsCDN {
			// subscriber -> CDN: flow.SrcAddress is subscriber, this is upload
			entry := byIP[flow.SrcAddress]
			entry.Upload += flow.RxRate * syncInterval / 8
			byIP[flow.SrcAddress] = entry
		}
	}

	if len(byIP) > 0 {
		log.Printf("CDN-FUP: Torch on NAS %s: %d flows, %d subscribers with CDN traffic", nas.Name, len(results), len(byIP))
	}

	return byIP
}

// checkAndEnforceCDNFUP checks CDN usage against CDN FUP thresholds and creates/updates/removes
// CDN-specific simple queues on MikroTik. Independent of regular FUP.
func (s *QuotaSyncService) checkAndEnforceCDNFUP(client *mikrotik.Client, nas *models.Nas, sub *models.Subscriber, sessionIP string) {
	service := sub.Service
	if service == nil || !service.CDNFUPEnabled {
		return
	}

	cdnDailyUsed := sub.CDNDailyDownloadUsed
	cdnMonthlyUsed := sub.CDNMonthlyDownloadUsed

	// Calculate CDN Daily FUP level (3 tiers)
	var dailyLevel int
	var dailyDownload, dailyUpload int64

	if service.CDNFUP3Threshold > 0 && cdnDailyUsed >= service.CDNFUP3Threshold && service.CDNFUP3DownloadSpeed > 0 {
		dailyLevel = 3
		dailyDownload = service.CDNFUP3DownloadSpeed
		dailyUpload = service.CDNFUP3UploadSpeed
	} else if service.CDNFUP2Threshold > 0 && cdnDailyUsed >= service.CDNFUP2Threshold && service.CDNFUP2DownloadSpeed > 0 {
		dailyLevel = 2
		dailyDownload = service.CDNFUP2DownloadSpeed
		dailyUpload = service.CDNFUP2UploadSpeed
	} else if service.CDNFUP1Threshold > 0 && cdnDailyUsed >= service.CDNFUP1Threshold && service.CDNFUP1DownloadSpeed > 0 {
		dailyLevel = 1
		dailyDownload = service.CDNFUP1DownloadSpeed
		dailyUpload = service.CDNFUP1UploadSpeed
	}

	// Calculate CDN Monthly FUP level (3 tiers)
	var monthlyLevel int
	var monthlyDownload, monthlyUpload int64

	if service.CDNMonthlyFUP3Threshold > 0 && cdnMonthlyUsed >= service.CDNMonthlyFUP3Threshold && service.CDNMonthlyFUP3DownloadSpeed > 0 {
		monthlyLevel = 3
		monthlyDownload = service.CDNMonthlyFUP3DownloadSpeed
		monthlyUpload = service.CDNMonthlyFUP3UploadSpeed
	} else if service.CDNMonthlyFUP2Threshold > 0 && cdnMonthlyUsed >= service.CDNMonthlyFUP2Threshold && service.CDNMonthlyFUP2DownloadSpeed > 0 {
		monthlyLevel = 2
		monthlyDownload = service.CDNMonthlyFUP2DownloadSpeed
		monthlyUpload = service.CDNMonthlyFUP2UploadSpeed
	} else if service.CDNMonthlyFUP1Threshold > 0 && cdnMonthlyUsed >= service.CDNMonthlyFUP1Threshold && service.CDNMonthlyFUP1DownloadSpeed > 0 {
		monthlyLevel = 1
		monthlyDownload = service.CDNMonthlyFUP1DownloadSpeed
		monthlyUpload = service.CDNMonthlyFUP1UploadSpeed
	}

	// Determine effective CDN FUP: pick the SLOWER speed (more restrictive)
	var effectiveLevel int
	var fupDownload, fupUpload int64

	if dailyLevel > 0 && monthlyLevel > 0 {
		if monthlyDownload > 0 && monthlyDownload < dailyDownload {
			fupDownload = monthlyDownload
			fupUpload = monthlyUpload
		} else {
			fupDownload = dailyDownload
			fupUpload = dailyUpload
		}
		if monthlyLevel > dailyLevel {
			effectiveLevel = monthlyLevel
		} else {
			effectiveLevel = dailyLevel
		}
	} else if dailyLevel > 0 {
		effectiveLevel = dailyLevel
		fupDownload = dailyDownload
		fupUpload = dailyUpload
	} else if monthlyLevel > 0 {
		effectiveLevel = monthlyLevel
		fupDownload = monthlyDownload
		fupUpload = monthlyUpload
	}

	// Check if levels changed
	oldDailyLevel := sub.CDNFUPLevel
	oldMonthlyLevel := sub.CDNMonthlyFUPLevel
	oldEffective := oldDailyLevel
	if oldMonthlyLevel > oldEffective {
		oldEffective = oldMonthlyLevel
	}

	// Update individual levels in DB if changed
	dbUpdates := map[string]interface{}{}
	if dailyLevel != oldDailyLevel {
		dbUpdates["cdn_fup_level"] = dailyLevel
	}
	if monthlyLevel != oldMonthlyLevel {
		dbUpdates["cdn_monthly_fup_level"] = monthlyLevel
	}
	if len(dbUpdates) > 0 {
		database.DB.Model(&models.Subscriber{}).Where("id = ?", sub.ID).Updates(dbUpdates)
	}

	// Apply or remove CDN FUP queue only if effective level changed
	if effectiveLevel != oldEffective {
		if effectiveLevel > 0 && fupDownload > 0 {
			// Build comma-separated CDN subnet list
			var cdns []models.CDN
			database.DB.Where("is_active = ?", true).Find(&cdns)
			var subnets []string
			for _, cdn := range cdns {
				for _, s := range strings.FieldsFunc(cdn.Subnets, func(r rune) bool {
					return r == ',' || r == '\n' || r == ';' || r == ' '
				}) {
					s = strings.TrimSpace(s)
					if s != "" {
						subnets = append(subnets, s)
					}
				}
			}
			cdnSubnetStr := strings.Join(subnets, ",")

			if cdnSubnetStr != "" {
				log.Printf("CDN-FUP: %s CDN daily=%.2fGB monthly=%.2fGB → applying CDN FUP level %d (%dk/%dk)",
					sub.Username,
					float64(cdnDailyUsed)/1024/1024/1024,
					float64(cdnMonthlyUsed)/1024/1024/1024,
					effectiveLevel, fupUpload, fupDownload)

				serviceName := ""
				if sub.Service != nil {
					serviceName = sub.Service.Name
				}
				if err := client.AddCDNFUPQueue(sub.Username, sessionIP, cdnSubnetStr, int(fupDownload), int(fupUpload), serviceName); err != nil {
					log.Printf("CDN-FUP: Failed to create/update queue for %s: %v", sub.Username, err)
				}
			}
		} else {
			// Remove CDN FUP queue
			log.Printf("CDN-FUP: %s CDN FUP cleared (was level %d)", sub.Username, oldEffective)
			if err := client.RemoveCDNFUPQueue(sub.Username); err != nil {
				log.Printf("CDN-FUP: Failed to remove queue for %s: %v", sub.Username, err)
			}
		}
	}
}
