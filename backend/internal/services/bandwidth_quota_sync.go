package services

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/mikrotik"
	"github.com/proisp/backend/internal/models"
)

// BandwidthQuotaSyncService syncs queue byte counters for bandwidth customers
type BandwidthQuotaSyncService struct {
	stopChan chan struct{}
	wg       sync.WaitGroup
	interval time.Duration
	mu       sync.Mutex
}

func NewBandwidthQuotaSyncService(interval time.Duration) *BandwidthQuotaSyncService {
	return &BandwidthQuotaSyncService{
		stopChan: make(chan struct{}),
		interval: interval,
	}
}

func (s *BandwidthQuotaSyncService) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		log.Println("BandwidthQuotaSyncService: Started (interval:", s.interval, ")")

		for {
			select {
			case <-ticker.C:
				if !s.mu.TryLock() {
					continue // Skip if previous sync still running
				}
				s.syncQuotas()
				s.mu.Unlock()
			case <-s.stopChan:
				return
			}
		}
	}()
}

func (s *BandwidthQuotaSyncService) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

func (s *BandwidthQuotaSyncService) syncQuotas() {
	// Get all active bandwidth customers grouped by NAS
	var customers []models.BandwidthCustomer
	if err := database.DB.Where("status = ? AND deleted_at IS NULL", "active").Find(&customers).Error; err != nil {
		log.Printf("BWQuotaSync: Failed to load customers: %v", err)
		return
	}

	if len(customers) == 0 {
		return
	}

	// Group by NAS
	nasCustMap := make(map[uint][]*models.BandwidthCustomer)
	for i := range customers {
		if customers[i].NasID != nil {
			nasCustMap[*customers[i].NasID] = append(nasCustMap[*customers[i].NasID], &customers[i])
		}
	}

	for nasID, custList := range nasCustMap {
		s.syncNAS(nasID, custList)
	}
}

func (s *BandwidthQuotaSyncService) syncNAS(nasID uint, customers []*models.BandwidthCustomer) {
	var nas models.Nas
	if err := database.DB.First(&nas, nasID).Error; err != nil {
		log.Printf("BWQuotaSync: NAS %d not found", nasID)
		return
	}

	if nas.APIUsername == "" || nas.APIPassword == "" {
		return
	}

	client := mikrotik.NewClient(fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort), nas.APIUsername, nas.APIPassword)

	// Batch fetch all queue stats in one API call
	statsMap, err := client.GetBWQueueStatsMap()
	if err != nil {
		log.Printf("BWQuotaSync: Failed to get queue stats from NAS %s: %v", nas.Name, err)
		return
	}

	now := time.Now()

	for _, cust := range customers {
		stats, found := statsMap[cust.QueueName]
		if !found || !stats.Exists {
			// Queue doesn't exist — mark offline
			if cust.IsOnline {
				database.DB.Model(cust).Updates(map[string]interface{}{
					"is_online": false,
				})
				// Close open session
				s.closeSession(cust.ID, now)
			}
			continue
		}

		// Calculate delta bytes
		deltaIn := stats.BytesIn - cust.LastQueueBytesIn
		deltaOut := stats.BytesOut - cust.LastQueueBytesOut

		// Counter reset detection (MikroTik reboot)
		if stats.BytesIn < cust.LastQueueBytesIn {
			deltaIn = stats.BytesIn
		}
		if stats.BytesOut < cust.LastQueueBytesOut {
			deltaOut = stats.BytesOut
		}

		if deltaIn < 0 {
			deltaIn = 0
		}
		if deltaOut < 0 {
			deltaOut = 0
		}

		// Update counters
		updates := map[string]interface{}{
			"daily_download_used":   cust.DailyDownloadUsed + deltaIn,
			"daily_upload_used":     cust.DailyUploadUsed + deltaOut,
			"monthly_download_used": cust.MonthlyDownloadUsed + deltaIn,
			"monthly_upload_used":   cust.MonthlyUploadUsed + deltaOut,
			"total_download":        cust.TotalDownload + deltaIn,
			"total_upload":          cust.TotalUpload + deltaOut,
			"last_queue_bytes_in":   stats.BytesIn,
			"last_queue_bytes_out":  stats.BytesOut,
			"is_online":            true,
			"last_seen":            now,
		}

		// Check FUP thresholds if enabled
		if cust.FUPEnabled {
			newDailyDl := cust.DailyDownloadUsed + deltaIn
			newMonthlyDl := cust.MonthlyDownloadUsed + deltaIn

			newDailyFUP := s.calculateFUPLevel(newDailyDl, cust.FUP1Threshold, cust.FUP2Threshold, cust.FUP3Threshold)
			newMonthlyFUP := s.calculateFUPLevel(newMonthlyDl, cust.MonthlyFUP1Threshold, cust.MonthlyFUP2Threshold, cust.MonthlyFUP3Threshold)

			// Pick the slower FUP level (higher number = slower)
			effectiveFUP := newDailyFUP
			if newMonthlyFUP > effectiveFUP {
				effectiveFUP = newMonthlyFUP
			}

			updates["fup_level"] = newDailyFUP
			updates["monthly_fup_level"] = newMonthlyFUP

			// If FUP level changed, apply new speed
			oldEffective := cust.FUPLevel
			if cust.MonthlyFUPLevel > oldEffective {
				oldEffective = cust.MonthlyFUPLevel
			}

			if effectiveFUP != oldEffective {
				go s.applyFUPSpeed(cust, &nas, effectiveFUP, newDailyFUP, newMonthlyFUP)
			}
		}

		database.DB.Model(cust).Updates(updates)

		// Accumulate hourly bandwidth data
		if deltaIn > 0 || deltaOut > 0 {
			s.accumulateHourlyUsage(cust.ID, now, deltaIn, deltaOut)
		}

		// Track session: if customer just came online, open a new session
		if !cust.IsOnline {
			s.openSession(cust.ID, now)
		}
	}
}

// accumulateHourlyUsage UPSERTs bandwidth data into hourly buckets
func (s *BandwidthQuotaSyncService) accumulateHourlyUsage(customerID uint, now time.Time, deltaIn, deltaOut int64) {
	// Calculate peak rate in kbps (delta bytes over 30s interval → kbps)
	peakDlKbps := deltaIn * 8 / 1000 / 30 // bytes to kbits, divided by interval
	peakUlKbps := deltaOut * 8 / 1000 / 30

	database.DB.Exec(`
		INSERT INTO bw_hourly_usage (customer_id, hour, download_bytes, upload_bytes, peak_download_kbps, peak_upload_kbps, sample_count, created_at)
		VALUES (?, date_trunc('hour', ?::timestamptz), ?, ?, ?, ?, 1, NOW())
		ON CONFLICT (customer_id, hour) DO UPDATE SET
			download_bytes = bw_hourly_usage.download_bytes + EXCLUDED.download_bytes,
			upload_bytes = bw_hourly_usage.upload_bytes + EXCLUDED.upload_bytes,
			peak_download_kbps = GREATEST(bw_hourly_usage.peak_download_kbps, EXCLUDED.peak_download_kbps),
			peak_upload_kbps = GREATEST(bw_hourly_usage.peak_upload_kbps, EXCLUDED.peak_upload_kbps),
			sample_count = bw_hourly_usage.sample_count + 1
	`, customerID, now, deltaIn, deltaOut, peakDlKbps, peakUlKbps)
}

// openSession creates a new session record when a customer comes online
func (s *BandwidthQuotaSyncService) openSession(customerID uint, now time.Time) {
	// Check if there's already an open session
	var count int64
	database.DB.Model(&models.BwSession{}).Where("customer_id = ? AND ended_at IS NULL", customerID).Count(&count)
	if count > 0 {
		return // already has an open session
	}

	session := models.BwSession{
		CustomerID: customerID,
		StartedAt:  now,
	}
	database.DB.Create(&session)
}

// closeSession closes the open session for a customer going offline
func (s *BandwidthQuotaSyncService) closeSession(customerID uint, now time.Time) {
	var session models.BwSession
	if err := database.DB.Where("customer_id = ? AND ended_at IS NULL", customerID).First(&session).Error; err != nil {
		return // no open session
	}

	durationSec := int(now.Sub(session.StartedAt).Seconds())

	database.DB.Model(&session).Updates(map[string]interface{}{
		"ended_at":     now,
		"duration_sec": durationSec,
	})
}

func (s *BandwidthQuotaSyncService) calculateFUPLevel(usedBytes, t1, t2, t3 int64) int {
	if t3 > 0 && usedBytes >= t3 {
		return 3
	}
	if t2 > 0 && usedBytes >= t2 {
		return 2
	}
	if t1 > 0 && usedBytes >= t1 {
		return 1
	}
	return 0
}

func (s *BandwidthQuotaSyncService) applyFUPSpeed(cust *models.BandwidthCustomer, nas *models.Nas, effectiveLevel, dailyFUP, monthlyFUP int) {
	// Determine speed based on FUP level
	dlSpeed := cust.DownloadSpeed
	ulSpeed := cust.UploadSpeed

	switch effectiveLevel {
	case 1:
		if dailyFUP >= 1 && cust.FUP1Speed > 0 {
			dlSpeed = cust.FUP1Speed
			ulSpeed = cust.FUP1Speed
		}
		if monthlyFUP >= 1 && cust.MonthlyFUP1Speed > 0 && cust.MonthlyFUP1Speed < dlSpeed {
			dlSpeed = cust.MonthlyFUP1Speed
			ulSpeed = cust.MonthlyFUP1Speed
		}
	case 2:
		if dailyFUP >= 2 && cust.FUP2Speed > 0 {
			dlSpeed = cust.FUP2Speed
			ulSpeed = cust.FUP2Speed
		}
		if monthlyFUP >= 2 && cust.MonthlyFUP2Speed > 0 && cust.MonthlyFUP2Speed < dlSpeed {
			dlSpeed = cust.MonthlyFUP2Speed
			ulSpeed = cust.MonthlyFUP2Speed
		}
	case 3:
		if dailyFUP >= 3 && cust.FUP3Speed > 0 {
			dlSpeed = cust.FUP3Speed
			ulSpeed = cust.FUP3Speed
		}
		if monthlyFUP >= 3 && cust.MonthlyFUP3Speed > 0 && cust.MonthlyFUP3Speed < dlSpeed {
			dlSpeed = cust.MonthlyFUP3Speed
			ulSpeed = cust.MonthlyFUP3Speed
		}
	}

	if effectiveLevel == 0 {
		// Restore original speed
		dlSpeed = cust.DownloadSpeed
		ulSpeed = cust.UploadSpeed
	}

	client := mikrotik.NewClient(fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort), nas.APIUsername, nas.APIPassword)
	burst := mikrotik.BWBurstConfig{} // No burst during FUP
	if effectiveLevel == 0 {
		burst = mikrotik.BWBurstConfig{
			Enabled: cust.BurstEnabled, BurstDl: cust.BurstDownload, BurstUl: cust.BurstUpload,
			ThresholdDl: cust.BurstThresholdDl, ThresholdUl: cust.BurstThresholdUl, BurstTime: cust.BurstTime,
		}
	}

	if err := client.UpdateBWSimpleQueue(cust.QueueName, dlSpeed, ulSpeed, burst); err != nil {
		log.Printf("BWQuotaSync: Failed to apply FUP level %d for %s: %v", effectiveLevel, cust.Name, err)
	} else {
		log.Printf("BWQuotaSync: Applied FUP level %d for %s: %dk/%dk", effectiveLevel, cust.Name, dlSpeed, ulSpeed)
	}
}

// ResetDailyQuotasForBWCustomers resets daily counters for bandwidth customers
// Called by DailyQuotaResetService
func ResetDailyQuotasForBWCustomers() {
	// Save daily usage history BEFORE reset (otherwise counters are already zeroed)
	database.DB.Exec(`
		INSERT INTO bw_daily_usage_history (customer_id, date, download_bytes, upload_bytes)
		SELECT id, CURRENT_DATE - INTERVAL '1 day',
			GREATEST(daily_download_used, 0), GREATEST(daily_upload_used, 0)
		FROM bandwidth_customers
		WHERE deleted_at IS NULL AND (daily_download_used > 0 OR daily_upload_used > 0)
		ON CONFLICT DO NOTHING
	`)

	now := time.Now()
	result := database.DB.Model(&models.BandwidthCustomer{}).
		Where("deleted_at IS NULL").
		Updates(map[string]interface{}{
			"daily_download_used": 0,
			"daily_upload_used":   0,
			"fup_level":           0,
			"last_daily_reset":    now,
		})

	if result.Error != nil {
		log.Printf("BWQuotaSync: Failed to reset daily quotas: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("BWQuotaSync: Reset daily quotas for %d bandwidth customers", result.RowsAffected)
	}
}
