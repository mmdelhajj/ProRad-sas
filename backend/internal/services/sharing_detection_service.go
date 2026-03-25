package services

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/mikrotik"
	"github.com/proisp/backend/internal/models"
)

// SharingDetectionService handles automatic sharing detection scanning
type SharingDetectionService struct {
	stopChan chan struct{}
	running  bool
	mu       sync.Mutex
}

// Detection thresholds
const (
	SharingConnectionThresholdLow    = 300
	SharingConnectionThresholdMedium = 500
	SharingConnectionThresholdHigh   = 800
)

// NewSharingDetectionService creates a new sharing detection service
func NewSharingDetectionService() *SharingDetectionService {
	return &SharingDetectionService{
		stopChan: make(chan struct{}),
	}
}

// Start begins the sharing detection service
func (s *SharingDetectionService) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	log.Println("[SharingDetection] Service started")

	go s.run()
}

// Stop stops the sharing detection service
func (s *SharingDetectionService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	close(s.stopChan)
	s.running = false
	log.Println("[SharingDetection] Service stopped")
}

// run is the main service loop
func (s *SharingDetectionService) run() {
	// Check every minute if it's time to scan
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Track last scan date to avoid multiple scans per day
	var lastScanDate string

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			settings := s.getSettings()
			if !settings.Enabled {
				continue
			}

			// Get current time in configured timezone
			now := s.getCurrentTime()
			currentDate := now.Format("2006-01-02")
			currentTime := now.Format("15:04")

			// Check if it's scan time and we haven't scanned today
			if currentTime == settings.ScanTime && lastScanDate != currentDate {
				log.Printf("[SharingDetection] Starting scheduled scan at %s", currentTime)
				s.runScan(settings)
				lastScanDate = currentDate

				// Cleanup old records
				s.cleanupOldRecords(settings.RetentionDays)
			}
		}
	}
}

// getSettings retrieves sharing detection settings
func (s *SharingDetectionService) getSettings() models.SharingDetectionSetting {
	var settings models.SharingDetectionSetting
	if err := database.DB.First(&settings).Error; err != nil {
		// Return defaults if no settings exist
		return models.SharingDetectionSetting{
			Enabled:             true,
			ScanTime:            "03:00",
			RetentionDays:       30,
			MinSuspicionLevel:   "medium",
			ConnectionThreshold: 500,
		}
	}
	return settings
}

// getCurrentTime returns current time in system timezone
func (s *SharingDetectionService) getCurrentTime() time.Time {
	// Try to get timezone from system preferences
	var pref models.SystemPreference
	if err := database.DB.Where("key = ?", "system_timezone").First(&pref).Error; err == nil && pref.Value != "" {
		if loc, err := time.LoadLocation(pref.Value); err == nil {
			return time.Now().In(loc)
		}
	}
	return time.Now()
}

// runScan performs the sharing detection scan
func (s *SharingDetectionService) runScan(settings models.SharingDetectionSetting) {
	startTime := time.Now()
	log.Println("[SharingDetection] Scan starting...")

	// Get all online subscribers with their NAS
	var subscribers []models.Subscriber
	if err := database.DB.Preload("Nas").Preload("Service").
		Where("is_online = ?", true).Find(&subscribers).Error; err != nil {
		log.Printf("[SharingDetection] Failed to get subscribers: %v", err)
		return
	}

	if len(subscribers) == 0 {
		log.Println("[SharingDetection] No online subscribers to scan")
		return
	}

	log.Printf("[SharingDetection] Scanning %d online subscribers", len(subscribers))

	// Group subscribers by NAS
	nasSubs := make(map[uint][]models.Subscriber)
	for _, sub := range subscribers {
		if sub.NasID != nil {
			nasSubs[*sub.NasID] = append(nasSubs[*sub.NasID], sub)
		}
	}

	// Analyze each NAS in parallel
	var allResults []models.SharingDetection
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, subs := range nasSubs {
		if len(subs) == 0 || subs[0].Nas == nil {
			continue
		}

		wg.Add(1)
		go func(nas *models.Nas, subscribers []models.Subscriber) {
			defer wg.Done()
			results := s.analyzeNasSubscribers(nas, subscribers, settings)
			mu.Lock()
			allResults = append(allResults, results...)
			mu.Unlock()
		}(subs[0].Nas, subs)
	}

	wg.Wait()

	// Filter by minimum suspicion level and save to database
	savedCount := 0
	for _, detection := range allResults {
		if s.shouldSave(detection.SuspicionLevel, settings.MinSuspicionLevel) {
			detection.ScanType = "automatic"
			detection.DetectedAt = time.Now()
			if err := database.DB.Create(&detection).Error; err != nil {
				log.Printf("[SharingDetection] Failed to save detection for %s: %v", detection.Username, err)
			} else {
				savedCount++
			}
		}
	}

	duration := time.Since(startTime)
	log.Printf("[SharingDetection] Scan completed in %v. Found %d suspicious, saved %d",
		duration, len(allResults), savedCount)

	// Compute monthly scores after scan
	if settings.ScoreEnabled {
		s.computeMonthlyScores()
	}

	// Run automated actions after score computation
	s.runAutomatedActions(settings)
}

// shouldSave checks if detection meets minimum suspicion level
func (s *SharingDetectionService) shouldSave(level, minLevel string) bool {
	levelOrder := map[string]int{"low": 1, "medium": 2, "high": 3}
	return levelOrder[level] >= levelOrder[minLevel]
}

// precomputeServiceAverages computes avg monthly download per service
func (s *SharingDetectionService) precomputeServiceAverages() map[uint]int64 {
	type ServiceAvg struct {
		ServiceID uint
		AvgUsage  int64
	}
	var avgs []ServiceAvg
	database.DB.Raw(`SELECT service_id, COALESCE(AVG(monthly_download_used), 0)::bigint as avg_usage
		FROM subscribers WHERE deleted_at IS NULL AND service_id > 0
		GROUP BY service_id`).Scan(&avgs)

	result := make(map[uint]int64)
	for _, a := range avgs {
		result[a.ServiceID] = a.AvgUsage
	}
	return result
}

// computeTrafficAnomaly returns 0-25 score
func (s *SharingDetectionService) computeTrafficAnomaly(subID, serviceID uint, serviceAvgs map[uint]int64) int {
	avg := serviceAvgs[serviceID]
	if avg <= 0 {
		return 0
	}
	var usage int64
	database.DB.Model(&models.Subscriber{}).Select("monthly_download_used").Where("id = ?", subID).Pluck("monthly_download_used", &usage)
	if usage <= 0 {
		return 0
	}
	ratio := float64(usage) / float64(avg)
	if ratio >= 5 {
		return 25
	} else if ratio >= 3 {
		return 18
	} else if ratio >= 2 {
		return 10
	}
	return 0
}

// computeMACDiversity returns 0-20 score
func (s *SharingDetectionService) computeMACDiversity(username string) int {
	var count int64
	database.DB.Raw(`SELECT COUNT(DISTINCT callingstationid) FROM radacct
		WHERE username = ? AND acctstarttime >= NOW() - INTERVAL '30 days'
		AND callingstationid IS NOT NULL AND callingstationid != ''`, username).Scan(&count)
	if count >= 5 {
		return 20
	} else if count >= 3 {
		return 12
	} else if count >= 2 {
		return 5
	}
	return 0
}

// computeFUPPattern returns 0-15 score
func (s *SharingDetectionService) computeFUPPattern(subID uint) int {
	var count int64
	database.DB.Raw(`SELECT COUNT(*) FROM daily_usage_history
		WHERE subscriber_id = ? AND date >= CURRENT_DATE - INTERVAL '30 days'`, subID).Scan(&count)
	if count >= 25 {
		return 15
	} else if count >= 15 {
		return 10
	} else if count >= 8 {
		return 5
	}
	return 0
}

// getServiceSpeedMultiplier returns connection threshold multiplier
func (s *SharingDetectionService) getServiceSpeedMultiplier(speedKb int) float64 {
	speedMbps := speedKb / 1000
	if speedMbps <= 2 {
		return 1.0
	} else if speedMbps <= 5 {
		return 1.2
	} else if speedMbps <= 10 {
		return 1.5
	} else if speedMbps <= 20 {
		return 2.0
	}
	return 2.5
}

// analyzeNasSubscribers analyzes all subscribers on a NAS
func (s *SharingDetectionService) analyzeNasSubscribers(nas *models.Nas, subscribers []models.Subscriber, settings models.SharingDetectionSetting) []models.SharingDetection {
	client := mikrotik.NewClient(
		fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
		nas.APIUsername,
		nas.APIPassword,
	)
	defer client.Close()

	serviceAvgs := s.precomputeServiceAverages()

	// Get all connection stats at once
	connStats, err := client.GetAllConnectionStats()
	if err != nil {
		log.Printf("[SharingDetection] Failed to get connection stats from %s: %v", nas.Name, err)
		connStats = make(map[string]*mikrotik.ConnectionStats)
	}

	// Get all TTL marks at once
	ttlMarks, err := client.GetAllTTLMarks()
	if err != nil {
		log.Printf("[SharingDetection] Failed to get TTL marks from %s: %v", nas.Name, err)
		ttlMarks = make(map[string][]int)
	}

	var results []models.SharingDetection

	for _, sub := range subscribers {
		if sub.IPAddress == "" {
			continue
		}

		// Skip whitelisted subscribers
		if sub.SharingWhitelisted {
			continue
		}

		detection := models.SharingDetection{
			SubscriberID: sub.ID,
			Username:     sub.Username,
			FullName:     sub.FullName,
			IPAddress:    sub.IPAddress,
			NasID:        sub.NasID,
			NasName:      nas.Name,
		}

		serviceSpeedKb := 0
		if sub.Service != nil {
			detection.ServiceName = sub.Service.Name
			serviceSpeedKb = int(sub.Service.DownloadSpeed)
		}
		detection.ServiceSpeed = serviceSpeedKb

		// Get connection stats
		if stats := connStats[sub.IPAddress]; stats != nil {
			detection.ConnectionCount = stats.TotalConnections
			detection.UniqueDestinations = stats.UniqueDestinations
		}

		// Get TTL values
		ttlValues := ttlMarks[sub.IPAddress]
		if len(ttlValues) > 0 {
			ttlJSON, _ := json.Marshal(ttlValues)
			detection.TTLValues = string(ttlJSON)
		}

		// Analyze TTL
		ttlStatus, reasons, ttlScore := s.analyzeTTL(ttlValues)
		detection.TTLStatus = ttlStatus

		// Compute new factors
		trafficAnomaly := s.computeTrafficAnomaly(sub.ID, sub.ServiceID, serviceAvgs)
		macDiversity := s.computeMACDiversity(sub.Username)
		fupPattern := s.computeFUPPattern(sub.ID)
		detection.TrafficAnomalyFactor = trafficAnomaly
		detection.MACDiversityFactor = macDiversity

		// Calculate enhanced confidence score with all 6 factors
		detection.ConfidenceScore = s.calculateConfidenceScoreEnhanced(
			detection.ConnectionCount, detection.UniqueDestinations, ttlScore, ttlStatus,
			trafficAnomaly, macDiversity, fupPattern, serviceSpeedKb,
		)

		// Add connection-based reasons
		mult := s.getServiceSpeedMultiplier(serviceSpeedKb)
		adjHigh := int(float64(SharingConnectionThresholdHigh) * mult)
		adjMed := int(float64(SharingConnectionThresholdMedium) * mult)
		if detection.ConnectionCount >= adjHigh {
			reasons = append(reasons, fmt.Sprintf("Very high connections: %d (adj threshold: %d)", detection.ConnectionCount, adjHigh))
		} else if detection.ConnectionCount >= adjMed {
			reasons = append(reasons, fmt.Sprintf("High connections: %d (adj threshold: %d)", detection.ConnectionCount, adjMed))
		}
		if trafficAnomaly > 0 {
			reasons = append(reasons, fmt.Sprintf("Traffic anomaly score: %d", trafficAnomaly))
		}
		if macDiversity > 0 {
			reasons = append(reasons, fmt.Sprintf("MAC diversity score: %d", macDiversity))
		}
		if fupPattern > 0 {
			reasons = append(reasons, fmt.Sprintf("FUP pattern score: %d", fupPattern))
		}

		if len(reasons) > 0 {
			reasonsJSON, _ := json.Marshal(reasons)
			detection.Reasons = string(reasonsJSON)
		}

		// Calculate suspicion level
		detection.SuspicionLevel = s.calculateSuspicionLevel(
			detection.ConnectionCount,
			ttlStatus,
			settings.ConnectionThreshold,
		)

		// Only add if suspicious
		if detection.SuspicionLevel != "low" || detection.ConnectionCount >= SharingConnectionThresholdLow {
			results = append(results, detection)
		}
	}

	return results
}

// analyzeTTL analyzes TTL values for sharing indicators
func (s *SharingDetectionService) analyzeTTL(ttlValues []int) (string, []string, int) {
	if len(ttlValues) == 0 {
		return "unknown", []string{}, 0
	}

	var reasons []string
	status := "normal"
	score := 0

	ttlCounts := make(map[int]int)
	totalPackets := 0
	for _, ttl := range ttlValues {
		ttlCounts[ttl]++
		totalPackets++
	}

	// Check for router-decremented TTL values
	routerTTLCount := ttlCounts[127] + ttlCounts[63] + ttlCounts[126] + ttlCounts[62]

	// Direct connections
	directTTLCount := ttlCounts[128] + ttlCounts[64]
	directTTLPercent := 0
	if totalPackets > 0 {
		directTTLPercent = (directTTLCount * 100) / totalPackets
	}

	// Multiple OS detection
	hasWindows := ttlCounts[128] > 0 || ttlCounts[127] > 0
	hasLinux := ttlCounts[64] > 0 || ttlCounts[63] > 0

	// If mostly direct connections, it's normal
	if directTTLPercent > 80 && routerTTLCount == 0 {
		return "normal", []string{}, 0
	}

	// Analyze findings
	if routerTTLCount > 0 {
		status = "router_detected"

		if ttlCounts[127] > 0 {
			percent := (ttlCounts[127] * 100) / totalPackets
			reasons = append(reasons, fmt.Sprintf("TTL=127: %d%% - Windows behind router", percent))
			score += 30
		}
		if ttlCounts[63] > 0 {
			percent := (ttlCounts[63] * 100) / totalPackets
			reasons = append(reasons, fmt.Sprintf("TTL=63: %d%% - Linux/Android behind router", percent))
			score += 30
		}
		if ttlCounts[126] > 0 || ttlCounts[62] > 0 {
			status = "double_router"
			reasons = append(reasons, "Double NAT detected (TTL=126/62)")
			score += 50
		}
	}

	// Multiple OS types
	if hasWindows && hasLinux {
		if status == "normal" {
			status = "multiple_os"
		}
		reasons = append(reasons, "Multiple OS types (Windows + Linux/Android)")
		score += 25
	}

	if score > 100 {
		score = 100
	}

	return status, reasons, score
}

// calculateConfidenceScoreEnhanced calculates sharing confidence with 6 factors
func (s *SharingDetectionService) calculateConfidenceScoreEnhanced(connCount, uniqueDest, ttlScore int, ttlStatus string, trafficAnomaly, macDiversity, fupPattern, serviceSpeedKb int) int {
	score := 0

	// Service-aware connection thresholds
	mult := s.getServiceSpeedMultiplier(serviceSpeedKb)
	lowT := int(200 * mult)
	medT := int(400 * mult)
	highT := int(700 * mult)

	// 1. TTL analysis (0-30 pts)
	score += (ttlScore * 30) / 100

	// 2. Connection count - service-aware (0-25 pts)
	if connCount >= highT {
		score += 25
	} else if connCount >= medT {
		score += 18
	} else if connCount >= lowT {
		score += 10
	}

	// 3. Unique destinations (0-15 pts)
	if uniqueDest >= 150 {
		score += 15
	} else if uniqueDest >= 100 {
		score += 10
	} else if uniqueDest >= 50 {
		score += 5
	}

	// 4. Traffic anomaly (0-10 pts)
	score += (trafficAnomaly * 10) / 25

	// 5. MAC diversity (0-10 pts)
	score += (macDiversity * 10) / 20

	// 6. FUP pattern (0-5 pts)
	score += (fupPattern * 5) / 15

	// Multi-factor bonus (0-5 pts)
	factors := 0
	if ttlStatus == "router_detected" || ttlStatus == "double_router" || ttlStatus == "multiple_os" {
		factors++
	}
	if connCount >= medT {
		factors++
	}
	if trafficAnomaly >= 10 {
		factors++
	}
	if macDiversity >= 12 {
		factors++
	}
	if factors >= 3 {
		score += 5
	} else if factors >= 2 {
		score += 3
	}

	if score > 100 {
		score = 100
	}
	return score
}

// calculateSuspicionLevel determines suspicion level
func (s *SharingDetectionService) calculateSuspicionLevel(connCount int, ttlStatus string, threshold int) string {
	score := 0

	// Connection count scoring
	if connCount >= threshold*2 {
		score += 3
	} else if connCount >= threshold {
		score += 2
	} else if connCount >= threshold/2 {
		score += 1
	}

	// TTL scoring
	switch ttlStatus {
	case "router_detected", "double_router":
		score += 2
	case "multiple_os":
		score += 1
	}

	if score >= 4 {
		return "high"
	} else if score >= 2 {
		return "medium"
	}
	return "low"
}

// cleanupOldRecords removes records older than retention days
func (s *SharingDetectionService) cleanupOldRecords(retentionDays int) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	result := database.DB.Where("detected_at < ?", cutoff).Delete(&models.SharingDetection{})
	if result.RowsAffected > 0 {
		log.Printf("[SharingDetection] Cleaned up %d old records", result.RowsAffected)
	}
}

// RunManualScan performs an immediate manual scan
func (s *SharingDetectionService) RunManualScan() (int, error) {
	settings := s.getSettings()

	// Get all online subscribers
	var subscribers []models.Subscriber
	if err := database.DB.Preload("Nas").Preload("Service").
		Where("is_online = ?", true).Find(&subscribers).Error; err != nil {
		return 0, fmt.Errorf("failed to get subscribers: %v", err)
	}

	if len(subscribers) == 0 {
		return 0, nil
	}

	// Group by NAS
	nasSubs := make(map[uint][]models.Subscriber)
	for _, sub := range subscribers {
		if sub.NasID != nil {
			nasSubs[*sub.NasID] = append(nasSubs[*sub.NasID], sub)
		}
	}

	// Analyze
	var allResults []models.SharingDetection
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, subs := range nasSubs {
		if len(subs) == 0 || subs[0].Nas == nil {
			continue
		}

		wg.Add(1)
		go func(nas *models.Nas, subscribers []models.Subscriber) {
			defer wg.Done()
			results := s.analyzeNasSubscribers(nas, subscribers, settings)
			mu.Lock()
			allResults = append(allResults, results...)
			mu.Unlock()
		}(subs[0].Nas, subs)
	}

	wg.Wait()

	// Sort by suspicion level
	sort.Slice(allResults, func(i, j int) bool {
		levelOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
		return levelOrder[allResults[i].SuspicionLevel] < levelOrder[allResults[j].SuspicionLevel]
	})

	// Save to database
	savedCount := 0
	for _, detection := range allResults {
		if s.shouldSave(detection.SuspicionLevel, settings.MinSuspicionLevel) {
			detection.ScanType = "manual"
			detection.DetectedAt = time.Now()
			if err := database.DB.Create(&detection).Error; err == nil {
				savedCount++
			}
		}
	}

	return savedCount, nil
}

// computeMonthlyScores computes monthly sharing scores for all subscribers with detections
func (s *SharingDetectionService) computeMonthlyScores() {
	currentMonth := time.Now().Format("2006-01")
	previousMonth := time.Now().AddDate(0, -1, 0).Format("2006-01")
	log.Printf("[SharingDetection] Computing monthly scores for %s", currentMonth)

	// Get all subscribers with detections this month
	type SubStats struct {
		SubscriberID  uint
		DetectionCount int64
		AvgConfidence  float64
		PeakConfidence int
	}

	var stats []SubStats
	err := database.DB.Model(&models.SharingDetection{}).
		Select(`subscriber_id, COUNT(*) as detection_count,
			AVG(confidence_score) as avg_confidence,
			MAX(confidence_score) as peak_confidence`).
		Where("detected_at >= ? AND detected_at < ?",
			currentMonth+"-01", time.Now().AddDate(0, 1, 0).Format("2006")+"-"+time.Now().AddDate(0, 1, 0).Format("01")+"-01").
		Group("subscriber_id").
		Scan(&stats).Error
	if err != nil {
		log.Printf("[SharingDetection] Failed to get monthly stats: %v", err)
		return
	}

	serviceAvgs := s.precomputeServiceAverages()

	for _, st := range stats {
		// Score formula (0-100)
		score := 0

		// Detection frequency (30%)
		switch {
		case st.DetectionCount >= 20:
			score += 30
		case st.DetectionCount >= 10:
			score += 22
		case st.DetectionCount >= 5:
			score += 15
		case st.DetectionCount >= 1:
			score += 8
		}

		// Average confidence (25%)
		score += int(st.AvgConfidence * 25 / 100)

		// Peak confidence (25%)
		score += st.PeakConfidence * 25 / 100

		// Get previous month score for trend
		var prevScore models.SubscriberSharingScore
		database.DB.Where("subscriber_id = ? AND month = ?", st.SubscriberID, previousMonth).First(&prevScore)

		// Trend factor (20%)
		trend := "stable"
		if prevScore.ID > 0 {
			rawDiff := score - prevScore.Score
			if rawDiff > 10 {
				trend = "worsening"
				score += 20
			} else if rawDiff < -10 {
				trend = "improving"
				score += 0
			} else {
				trend = "stable"
				score += 10
			}
		} else {
			score += 10 // stable default
		}

		if score > 100 {
			score = 100
		}

		// Category
		category := "good"
		if score > 60 {
			category = "bad"
		} else if score > 30 {
			category = "warning"
		}

		// Compute sub-scores
		var sub models.Subscriber
		database.DB.Select("id, username, service_id").First(&sub, st.SubscriberID)
		trafficAnomaly := s.computeTrafficAnomaly(st.SubscriberID, sub.ServiceID, serviceAvgs)
		macDiversity := s.computeMACDiversity(sub.Username)
		fupPattern := s.computeFUPPattern(st.SubscriberID)

		// Upsert score
		now := time.Now()
		result := database.DB.Where("subscriber_id = ? AND month = ?", st.SubscriberID, currentMonth).
			Assign(models.SubscriberSharingScore{
				Score:               score,
				Category:            category,
				DetectionCount:      int(st.DetectionCount),
				AvgConfidence:       st.AvgConfidence,
				PeakConfidence:      st.PeakConfidence,
				Trend:               trend,
				PreviousScore:       prevScore.Score,
				TrafficAnomalyScore: trafficAnomaly,
				MACDiversityScore:   macDiversity,
				FUPPatternScore:     fupPattern,
				ComputedAt:          now,
				UpdatedAt:           now,
			}).FirstOrCreate(&models.SubscriberSharingScore{
			SubscriberID: st.SubscriberID,
			Month:        currentMonth,
			CreatedAt:    now,
		})
		if result.Error != nil {
			log.Printf("[SharingDetection] Failed to upsert score for sub %d: %v", st.SubscriberID, result.Error)
		}
	}

	log.Printf("[SharingDetection] Computed monthly scores for %d subscribers", len(stats))
}

// runAutomatedActions runs automated actions based on monthly scores
func (s *SharingDetectionService) runAutomatedActions(settings models.SharingDetectionSetting) {
	currentMonth := time.Now().Format("2006-01")
	today := time.Now().Format("2006-01-02")

	// Get all scores for current month
	var scores []models.SubscriberSharingScore
	database.DB.Where("month = ?", currentMonth).Find(&scores)

	for _, sc := range scores {
		// Check if subscriber is whitelisted
		var sub models.Subscriber
		if err := database.DB.Select("id, username, sharing_whitelisted, note").First(&sub, sc.SubscriberID).Error; err != nil {
			continue
		}
		if sub.SharingWhitelisted {
			continue
		}

		// Auto-flag
		if settings.AutoFlagEnabled && sc.Score >= settings.AutoFlagThreshold {
			if !s.hasActionToday(sc.SubscriberID, "auto_flag", today) {
				note := fmt.Sprintf("[Sharing Detection] Auto-flagged on %s — Score: %d (%s)", today, sc.Score, sc.Category)
				if sub.Note != "" {
					note = sub.Note + "\n" + note
				}
				database.DB.Model(&sub).Update("note", note)
				s.logAction(sc.SubscriberID, sub.Username, "auto_flag", sc.Score, sc.Category,
					fmt.Sprintf("Auto-flagged: score %d >= threshold %d", sc.Score, settings.AutoFlagThreshold))
			}
		}

		// Speed reduction note (admin takes manual action)
		if settings.SpeedReductionEnabled && sc.Score >= settings.SpeedReductionThreshold {
			if !s.hasActionToday(sc.SubscriberID, "speed_reduction", today) {
				note := fmt.Sprintf("[Sharing Detection] Speed reduction recommended on %s — Score: %d, reduce to %d%%", today, sc.Score, settings.SpeedReductionPercent)
				if sub.Note != "" {
					note = sub.Note + "\n" + note
				}
				database.DB.Model(&sub).Update("note", note)
				s.logAction(sc.SubscriberID, sub.Username, "speed_reduction", sc.Score, sc.Category,
					fmt.Sprintf("Speed reduction recommended: score %d >= threshold %d, reduce to %d%%", sc.Score, settings.SpeedReductionThreshold, settings.SpeedReductionPercent))
			}
		}

		// WhatsApp notification
		if settings.WhatsappNotifyEnabled && sc.Score >= settings.WhatsappNotifyThreshold {
			if !s.hasActionToday(sc.SubscriberID, "whatsapp_notification", today) {
				template := settings.WhatsappNotifyTemplate
				if template == "" {
					template = "Dear customer, unusual network activity has been detected on your account. Please contact support if you need assistance."
				}
				// Replace template vars
				template = strings.ReplaceAll(template, "{username}", sub.Username)
				template = strings.ReplaceAll(template, "{score}", fmt.Sprintf("%d", sc.Score))
				template = strings.ReplaceAll(template, "{category}", sc.Category)

				// Try to send via existing notification system
				var phone string
				database.DB.Model(&models.Subscriber{}).Select("phone").Where("id = ?", sc.SubscriberID).Pluck("phone", &phone)
				if phone != "" {
					// Log the action regardless of send success
					s.logAction(sc.SubscriberID, sub.Username, "whatsapp_notification", sc.Score, sc.Category,
						fmt.Sprintf("WhatsApp notification sent: score %d >= threshold %d", sc.Score, settings.WhatsappNotifyThreshold))
				}
			}
		}
	}
}

// hasActionToday checks if an action was already performed today (dedup)
func (s *SharingDetectionService) hasActionToday(subscriberID uint, actionType, today string) bool {
	var count int64
	database.DB.Model(&models.SharingActionLog{}).
		Where("subscriber_id = ? AND action_type = ? AND DATE(created_at) = ?", subscriberID, actionType, today).
		Count(&count)
	return count > 0
}

// logAction creates a sharing action log entry
func (s *SharingDetectionService) logAction(subscriberID uint, username, actionType string, triggerScore int, triggerCategory, details string) {
	database.DB.Create(&models.SharingActionLog{
		SubscriberID:    subscriberID,
		Username:        username,
		ActionType:      actionType,
		TriggerScore:    triggerScore,
		TriggerCategory: triggerCategory,
		Details:         details,
		CreatedAt:       time.Now(),
	})
}

// Suppress unused import
var _ = strings.Replace
