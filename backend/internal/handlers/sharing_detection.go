package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/mikrotik"
	"github.com/proisp/backend/internal/models"
)

type SharingDetectionHandler struct{}

func NewSharingDetectionHandler() *SharingDetectionHandler {
	return &SharingDetectionHandler{}
}

// SuspiciousAccount represents an account suspected of sharing
type SuspiciousAccount struct {
	SubscriberID       uint     `json:"subscriber_id"`
	Username           string   `json:"username"`
	FullName           string   `json:"full_name"`
	IPAddress          string   `json:"ip_address"`
	ServiceName        string   `json:"service_name"`
	ConnectionCount    int      `json:"connection_count"`
	UniqueDestinations int      `json:"unique_destinations"` // Number of unique destination IPs
	TTLValues          []int    `json:"ttl_values"`
	TTLStatus          string   `json:"ttl_status"`      // "normal", "router_detected", "multiple_os", "double_router"
	SuspicionLevel     string   `json:"suspicion_level"` // "normal", "low", "medium", "high", "critical"
	ConfidenceScore    int      `json:"confidence_score"` // 0-100% confidence that sharing is happening
	Reasons            []string `json:"reasons"`
	NASName            string   `json:"nas_name"`
	NASIPAddress       string   `json:"nas_ip_address"`
}

// SharingStats represents overall sharing detection statistics
type SharingStats struct {
	TotalOnline       int `json:"total_online"`
	SuspiciousCount   int `json:"suspicious_count"`
	HighRiskCount     int `json:"high_risk_count"`
	RouterDetected    int `json:"router_detected"`
	HighConnections   int `json:"high_connections"`
}

// Thresholds for detection - IMPROVED for better accuracy
const (
	// Connection thresholds (higher = more accurate, fewer false positives)
	ConnectionThresholdLow    = 300  // Low suspicion - could be heavy user
	ConnectionThresholdMedium = 500  // Medium suspicion - likely sharing
	ConnectionThresholdHigh   = 800  // High suspicion - definitely sharing

	// Unique destination thresholds (many destinations = more devices)
	DestinationThresholdLow    = 50   // Normal browsing
	DestinationThresholdMedium = 100  // Multiple devices likely
	DestinationThresholdHigh   = 150  // Definitely multiple devices

	// TTL values
	NormalTTLWindows = 128 // Windows default
	NormalTTLLinux   = 64  // Linux/Android/iOS default
)

// List returns all online users with sharing detection analysis
func (h *SharingDetectionHandler) List(c *fiber.Ctx) error {
	// Get thresholds from query params (allow customization)
	connThresholdMedium := c.QueryInt("conn_threshold_medium", ConnectionThresholdMedium)
	connThresholdHigh := c.QueryInt("conn_threshold_high", ConnectionThresholdHigh)

	// Get all online subscribers with their NAS and Service
	var subscribers []models.Subscriber
	if err := database.DB.Preload("Nas").Preload("Service").
		Where("is_online = ?", true).Find(&subscribers).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get online subscribers",
		})
	}

	if len(subscribers) == 0 {
		return c.JSON(fiber.Map{
			"success": true,
			"data":    []SuspiciousAccount{},
			"stats": SharingStats{
				TotalOnline: 0,
			},
		})
	}

	// Group subscribers by NAS
	nasSubs := make(map[uint][]models.Subscriber)
	for _, sub := range subscribers {
		if sub.NasID != nil {
			nasSubs[*sub.NasID] = append(nasSubs[*sub.NasID], sub)
		}
	}

	// Analyze each NAS in parallel with timeout
	var allResults []SuspiciousAccount
	var mu sync.Mutex
	var wg sync.WaitGroup

	for nasID, subs := range nasSubs {
		if len(subs) == 0 || subs[0].Nas == nil {
			continue
		}

		wg.Add(1)
		go func(nas *models.Nas, subscribers []models.Subscriber) {
			defer wg.Done()
			results := analyzeNasSubscribers(nas, subscribers, connThresholdMedium, connThresholdHigh)
			mu.Lock()
			allResults = append(allResults, results...)
			mu.Unlock()
		}(subs[0].Nas, subs)
		_ = nasID // avoid unused variable
	}

	// Wait with 30-second timeout to prevent infinite loading
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
		// All NAS analyzed successfully
	case <-time.After(30 * time.Second):
		log.Println("SharingDetection: Timeout waiting for NAS analysis (30s)")
	}

	// Sort by suspicion level (high first) then by connection count
	sort.Slice(allResults, func(i, j int) bool {
		levelOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
		if levelOrder[allResults[i].SuspicionLevel] != levelOrder[allResults[j].SuspicionLevel] {
			return levelOrder[allResults[i].SuspicionLevel] < levelOrder[allResults[j].SuspicionLevel]
		}
		return allResults[i].ConnectionCount > allResults[j].ConnectionCount
	})

	// Calculate stats
	stats := SharingStats{
		TotalOnline: len(subscribers),
	}
	for _, r := range allResults {
		if r.SuspicionLevel == "medium" || r.SuspicionLevel == "high" {
			stats.SuspiciousCount++
		}
		if r.SuspicionLevel == "high" {
			stats.HighRiskCount++
		}
		if r.TTLStatus == "router_detected" {
			stats.RouterDetected++
		}
		if r.ConnectionCount >= connThresholdMedium {
			stats.HighConnections++
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    allResults,
		"stats":   stats,
	})
}

// GetSubscriberDetails returns detailed sharing analysis for a specific subscriber
func (h *SharingDetectionHandler) GetSubscriberDetails(c *fiber.Ctx) error {
	id := c.Params("id")

	var subscriber models.Subscriber
	if err := database.DB.Preload("Nas").Preload("Service").First(&subscriber, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber not found",
		})
	}

	if !subscriber.IsOnline || subscriber.Nas == nil {
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"subscriber_id": subscriber.ID,
				"username":      subscriber.Username,
				"is_online":     false,
				"message":       "Subscriber is offline",
			},
		})
	}

	// Get detailed connection info from MikroTik
	client := mikrotik.NewClient(
		fmt.Sprintf("%s:%d", subscriber.Nas.IPAddress, subscriber.Nas.APIPort),
		subscriber.Nas.APIUsername,
		subscriber.Nas.APIPassword,
	)
	defer client.Close()

	// Get connection count
	connCount, err := client.GetConnectionCount(subscriber.IPAddress)
	if err != nil {
		log.Printf("SharingDetection: Failed to get connection count for %s: %v", subscriber.Username, err)
		connCount = 0
	}

	// Get TTL values
	ttlValues, err := client.GetTTLValues(subscriber.IPAddress)
	if err != nil {
		log.Printf("SharingDetection: Failed to get TTL values for %s: %v", subscriber.Username, err)
		ttlValues = []int{}
	}

	// Get connection details
	connections, err := client.GetConnectionDetails(subscriber.IPAddress)
	if err != nil {
		log.Printf("SharingDetection: Failed to get connection details for %s: %v", subscriber.Username, err)
		connections = []map[string]string{}
	}

	// Analyze
	ttlStatus, ttlReasons := analyzeTTL(ttlValues)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"subscriber_id":    subscriber.ID,
			"username":         subscriber.Username,
			"full_name":        subscriber.FullName,
			"ip_address":       subscriber.IPAddress,
			"service_name":     subscriber.Service.Name,
			"connection_count": connCount,
			"ttl_values":       ttlValues,
			"ttl_status":       ttlStatus,
			"ttl_analysis":     ttlReasons,
			"connections":      connections,
			"nas_name":         subscriber.Nas.Name,
		},
	})
}

// analyzeNasSubscribers analyzes all subscribers on a NAS using batch queries
func analyzeNasSubscribers(nas *models.Nas, subscribers []models.Subscriber, connThresholdMedium, connThresholdHigh int) []SuspiciousAccount {
	client := mikrotik.NewClient(
		fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
		nas.APIUsername,
		nas.APIPassword,
	)
	defer client.Close()

	// Pre-compute service averages for traffic anomaly
	serviceAvgs := precomputeServiceAverages()

	// Build IP to subscriber map for fast lookup
	ipToSub := make(map[string]*models.Subscriber)
	for i := range subscribers {
		if subscribers[i].IPAddress != "" {
			ipToSub[subscribers[i].IPAddress] = &subscribers[i]
		}
	}

	// BATCH QUERY: Get all connection stats at once (connections + unique destinations)
	log.Printf("SharingDetection: Getting connection stats for %d IPs on NAS %s", len(ipToSub), nas.Name)
	connStats, err := client.GetAllConnectionStats()
	if err != nil {
		log.Printf("SharingDetection: Failed to get batch connection stats: %v", err)
		connStats = make(map[string]*mikrotik.ConnectionStats)
	}

	// BATCH QUERY: Get all TTL marks at once
	ttlMarks, err := client.GetAllTTLMarks()
	if err != nil {
		log.Printf("SharingDetection: Failed to get batch TTL marks: %v", err)
		ttlMarks = make(map[string][]int)
	}

	var results []SuspiciousAccount

	for _, sub := range subscribers {
		if sub.IPAddress == "" {
			continue
		}

		// Skip whitelisted subscribers
		if sub.SharingWhitelisted {
			continue
		}

		result := SuspiciousAccount{
			SubscriberID: sub.ID,
			Username:     sub.Username,
			FullName:     sub.FullName,
			IPAddress:    sub.IPAddress,
			NASName:      nas.Name,
			NASIPAddress: nas.IPAddress,
			Reasons:      []string{},
		}

		serviceSpeedKb := 0
		if sub.Service != nil && sub.Service.ID > 0 {
			result.ServiceName = sub.Service.Name
			serviceSpeedKb = int(sub.Service.DownloadSpeed)
		}

		// Get connection stats from batch result
		if stats := connStats[sub.IPAddress]; stats != nil {
			result.ConnectionCount = stats.TotalConnections
			result.UniqueDestinations = stats.UniqueDestinations
		}

		// Get TTL values from batch result
		result.TTLValues = ttlMarks[sub.IPAddress]

		// Analyze TTL with improved detection
		ttlStatus, ttlReasons, ttlScore := analyzeTTLImproved(result.TTLValues)
		result.TTLStatus = ttlStatus
		result.Reasons = append(result.Reasons, ttlReasons...)

		// Compute new factors
		trafficAnomaly := computeTrafficAnomalyScore(sub.ID, sub.ServiceID, serviceAvgs)
		macDiversity := computeMACDiversityScore(sub.Username)
		fupPattern := computeFUPPatternScore(sub.ID)

		if trafficAnomaly > 0 {
			result.Reasons = append(result.Reasons, fmt.Sprintf("Traffic anomaly: usage significantly above service average (score: %d)", trafficAnomaly))
		}
		if macDiversity > 0 {
			result.Reasons = append(result.Reasons, fmt.Sprintf("MAC diversity: multiple devices detected in last 30 days (score: %d)", macDiversity))
		}
		if fupPattern > 0 {
			result.Reasons = append(result.Reasons, fmt.Sprintf("FUP pattern: frequently hitting daily quota (score: %d)", fupPattern))
		}

		// Calculate confidence score with all 6 factors
		confidenceScore := calculateConfidenceScoreEnhanced(
			result.ConnectionCount, result.UniqueDestinations, ttlScore, ttlStatus,
			trafficAnomaly, macDiversity, fupPattern, serviceSpeedKb,
		)
		result.ConfidenceScore = confidenceScore

		// Add connection reasons with service-aware thresholds
		lowT, medT, highT := getServiceAwareThresholds(serviceSpeedKb)
		if result.ConnectionCount >= highT {
			result.Reasons = append(result.Reasons, fmt.Sprintf("Very high connections: %d (threshold: %d for this service speed)", result.ConnectionCount, highT))
		} else if result.ConnectionCount >= medT {
			result.Reasons = append(result.Reasons, fmt.Sprintf("High connections: %d (threshold: %d)", result.ConnectionCount, medT))
		} else if result.ConnectionCount >= lowT {
			result.Reasons = append(result.Reasons, fmt.Sprintf("Elevated connections: %d", result.ConnectionCount))
		}

		// Determine suspicion level
		result.SuspicionLevel = calculateSuspicionLevel(result.ConnectionCount, ttlStatus, connThresholdMedium, connThresholdHigh)

		results = append(results, result)
	}

	log.Printf("SharingDetection: Analyzed %d subscribers on NAS %s", len(results), nas.Name)
	return results
}

// analyzeTTL analyzes TTL values to detect sharing
func analyzeTTL(ttlValues []int) (string, []string) {
	if len(ttlValues) == 0 {
		return "unknown", []string{}
	}

	var reasons []string
	status := "normal"

	// Check for router-decremented TTL values
	hasRouterTTL := false
	hasMultipleOS := false

	ttlCounts := make(map[int]int)
	for _, ttl := range ttlValues {
		ttlCounts[ttl]++

		// TTL 127 = Windows behind router (128-1)
		// TTL 63 = Linux/Android behind router (64-1)
		// TTL 126 or 62 = Two routers
		if ttl == 127 || ttl == 63 || ttl == 126 || ttl == 62 {
			hasRouterTTL = true
		}
	}

	// Check for multiple OS types
	hasWindows := ttlCounts[128] > 0 || ttlCounts[127] > 0
	hasLinux := ttlCounts[64] > 0 || ttlCounts[63] > 0
	if hasWindows && hasLinux {
		hasMultipleOS = true
	}

	if hasRouterTTL {
		status = "router_detected"
		if ttlCounts[127] > 0 {
			reasons = append(reasons, fmt.Sprintf("TTL=127 detected (%d packets) - Windows device behind router", ttlCounts[127]))
		}
		if ttlCounts[63] > 0 {
			reasons = append(reasons, fmt.Sprintf("TTL=63 detected (%d packets) - Linux/Android device behind router", ttlCounts[63]))
		}
		if ttlCounts[126] > 0 || ttlCounts[62] > 0 {
			reasons = append(reasons, "TTL indicates multiple routers in chain")
		}
	}

	if hasMultipleOS {
		if status == "normal" {
			status = "multiple_os"
		}
		reasons = append(reasons, "Multiple OS types detected (Windows + Linux/Android)")
	}

	return status, reasons
}

// analyzeTTLImproved analyzes TTL values with improved accuracy and returns a score
func analyzeTTLImproved(ttlValues []int) (string, []string, int) {
	if len(ttlValues) == 0 {
		return "unknown", []string{}, 0
	}

	var reasons []string
	status := "normal"
	score := 0 // 0-100 score for confidence calculation

	// Count occurrences of each TTL
	ttlCounts := make(map[int]int)
	totalPackets := 0
	for _, ttl := range ttlValues {
		ttlCounts[ttl]++
		totalPackets++
	}

	// Check for router-decremented TTL values (strongest indicator)
	// TTL 127 = Windows behind router (128-1)
	// TTL 63 = Linux/Android behind router (64-1)
	// TTL 126 or 62 = Two routers (rare but definitive)
	// TTL 125 or 61 = Three routers (very rare)

	routerTTLCount := ttlCounts[127] + ttlCounts[63] + ttlCounts[126] + ttlCounts[62] + ttlCounts[125] + ttlCounts[61]
	routerTTLPercent := 0
	if totalPackets > 0 {
		routerTTLPercent = (routerTTLCount * 100) / totalPackets
	}

	// Check for direct connections (normal)
	directTTLCount := ttlCounts[128] + ttlCounts[64]
	directTTLPercent := 0
	if totalPackets > 0 {
		directTTLPercent = (directTTLCount * 100) / totalPackets
	}

	// Multiple OS detection
	hasWindows := ttlCounts[128] > 0 || ttlCounts[127] > 0
	hasLinux := ttlCounts[64] > 0 || ttlCounts[63] > 0

	// If majority is direct TTL (normal connections), reduce suspicion
	if directTTLPercent > 80 && routerTTLPercent == 0 {
		return "normal", []string{"Direct connections only (TTL=128/64)"}, 0
	}

	// Analyze findings
	if routerTTLPercent > 0 {
		status = "router_detected"

		if ttlCounts[127] > 0 {
			percent := (ttlCounts[127] * 100) / totalPackets
			reasons = append(reasons, fmt.Sprintf("TTL=127: %d packets (%d%%) - Windows device behind router", ttlCounts[127], percent))
			score += 30 // Strong indicator
		}
		if ttlCounts[63] > 0 {
			percent := (ttlCounts[63] * 100) / totalPackets
			reasons = append(reasons, fmt.Sprintf("TTL=63: %d packets (%d%%) - Linux/Android device behind router", ttlCounts[63], percent))
			score += 30
		}
		if ttlCounts[126] > 0 || ttlCounts[62] > 0 {
			status = "double_router"
			reasons = append(reasons, "TTL=126/62: Multiple routers detected (double NAT)")
			score += 50 // Very strong indicator
		}
		if ttlCounts[125] > 0 || ttlCounts[61] > 0 {
			reasons = append(reasons, "TTL=125/61: Triple router chain detected")
			score += 60
		}
	}

	// Multiple OS types is suspicious (different devices)
	if hasWindows && hasLinux {
		if status == "normal" {
			status = "multiple_os"
		}
		reasons = append(reasons, "Multiple OS types detected (Windows + Linux/Android devices)")
		score += 25
	}

	// TTL diversity (many different TTL values = many hops/devices)
	uniqueTTLs := len(ttlCounts)
	if uniqueTTLs > 4 {
		reasons = append(reasons, fmt.Sprintf("%d different TTL values detected (unusual diversity)", uniqueTTLs))
		score += 15
	}

	// Cap score at 100
	if score > 100 {
		score = 100
	}

	return status, reasons, score
}

// getServiceSpeedMultiplier returns connection threshold multiplier based on service speed (in kb)
func getServiceSpeedMultiplier(speedKb int) float64 {
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

// getServiceAwareThresholds returns connection thresholds adjusted for service speed
func getServiceAwareThresholds(serviceSpeedKb int) (int, int, int) {
	mult := getServiceSpeedMultiplier(serviceSpeedKb)
	return int(200 * mult), int(400 * mult), int(700 * mult)
}

// calculateConfidenceScoreEnhanced calculates overall confidence (0-100%) with 6 factors
func calculateConfidenceScoreEnhanced(connCount, uniqueDest, ttlScore int, ttlStatus string, trafficAnomaly, macDiversity, fupPattern, serviceSpeedKb int) int {
	score := 0

	// Get service-aware thresholds
	lowThresh, medThresh, highThresh := getServiceAwareThresholds(serviceSpeedKb)

	// 1. TTL analysis factor (0-30 points) - most reliable
	score += (ttlScore * 30) / 100

	// 2. Connection count factor - service-aware (0-25 points)
	if connCount >= highThresh {
		score += 25
	} else if connCount >= medThresh {
		score += 18
	} else if connCount >= lowThresh {
		score += 10
	}

	// 3. Unique destinations factor (0-15 points)
	if uniqueDest >= DestinationThresholdHigh {
		score += 15
	} else if uniqueDest >= DestinationThresholdMedium {
		score += 10
	} else if uniqueDest >= DestinationThresholdLow {
		score += 5
	}

	// 4. Traffic anomaly factor (0-10 points)
	score += (trafficAnomaly * 10) / 25

	// 5. MAC diversity factor (0-10 points)
	score += (macDiversity * 10) / 20

	// 6. FUP pattern factor (0-5 points)
	score += (fupPattern * 5) / 15

	// Multi-factor bonus (0-5 points)
	factors := 0
	if ttlStatus == "router_detected" || ttlStatus == "double_router" || ttlStatus == "multiple_os" {
		factors++
	}
	if connCount >= medThresh {
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

// calculateConfidenceScore - backward compatible wrapper
func calculateConfidenceScore(connCount, uniqueDest, ttlScore int, ttlStatus string) int {
	return calculateConfidenceScoreEnhanced(connCount, uniqueDest, ttlScore, ttlStatus, 0, 0, 0, 2000)
}

// computeTrafficAnomalyScore computes traffic anomaly score for a subscriber
// serviceAvgs maps service_id -> avg monthly bytes for that service
func computeTrafficAnomalyScore(subscriberID uint, serviceID uint, serviceAvgs map[uint]int64) int {
	avg := serviceAvgs[serviceID]
	if avg <= 0 {
		return 0
	}

	// Get subscriber's monthly usage
	var sub struct {
		MonthlyDownloadUsed int64
	}
	database.DB.Model(&models.Subscriber{}).Select("monthly_download_used").Where("id = ?", subscriberID).Scan(&sub)

	if sub.MonthlyDownloadUsed <= 0 {
		return 0
	}

	ratio := float64(sub.MonthlyDownloadUsed) / float64(avg)
	if ratio >= 5 {
		return 25
	} else if ratio >= 3 {
		return 18
	} else if ratio >= 2 {
		return 10
	}
	return 0
}

// computeMACDiversityScore counts unique MACs in radacct over 30 days
func computeMACDiversityScore(username string) int {
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

// computeFUPPatternScore counts days hitting 80%+ of daily quota in last 30 days
func computeFUPPatternScore(subscriberID uint) int {
	var count int64
	database.DB.Raw(`SELECT COUNT(*) FROM daily_usage_history
		WHERE subscriber_id = ? AND date >= CURRENT_DATE - INTERVAL '30 days'`, subscriberID).Scan(&count)

	// Use daily_usage_history presence as a proxy for heavy usage days
	// (entries are created when daily quota is reset, meaning they hit quota)
	if count >= 25 {
		return 15
	} else if count >= 15 {
		return 10
	} else if count >= 8 {
		return 5
	}
	return 0
}

// precomputeServiceAverages computes average monthly download per service (single query)
func precomputeServiceAverages() map[uint]int64 {
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

// calculateSuspicionLevel determines overall suspicion level
func calculateSuspicionLevel(connCount int, ttlStatus string, thresholdMedium, thresholdHigh int) string {
	score := 0

	// Connection count scoring
	if connCount >= thresholdHigh {
		score += 3
	} else if connCount >= thresholdMedium {
		score += 2
	} else if connCount >= thresholdMedium/2 {
		score += 1
	}

	// TTL scoring
	switch ttlStatus {
	case "router_detected":
		score += 2
	case "multiple_os":
		score += 1
	}

	// Determine level
	if score >= 4 {
		return "high"
	} else if score >= 2 {
		return "medium"
	}
	return "low"
}

// GetStats returns overall sharing detection statistics
func (h *SharingDetectionHandler) GetStats(c *fiber.Ctx) error {
	// Quick stats without full analysis
	var onlineCount int64
	database.DB.Model(&models.Subscriber{}).Where("is_online = ?", true).Count(&onlineCount)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"total_online": onlineCount,
			"message":      "Use /sharing/list for full analysis",
		},
	})
}

// NASRuleStatus represents TTL rule status for a NAS
type NASRuleStatus struct {
	NASID          uint   `json:"nas_id"`
	NASName        string `json:"nas_name"`
	NASIPAddress   string `json:"nas_ip_address"`
	RulesConfigured bool   `json:"rules_configured"`
	RuleCount      int    `json:"rule_count"`
	Error          string `json:"error,omitempty"`
}

// getSharingCompanyName retrieves company name from settings for sharing detection branding
func getSharingCompanyName() string {
	name := database.GetCompanyName()
	if name == "" {
		return "ISP"
	}
	return name
}

// getTTLRuleComment returns the TTL rule comment with company branding
func getTTLRuleComment() string {
	return getSharingCompanyName() + "-TTL-Detection"
}

// ListNASRuleStatus returns TTL rule status for all NAS devices
func (h *SharingDetectionHandler) ListNASRuleStatus(c *fiber.Ctx) error {
	var nasList []models.Nas
	if err := database.DB.Where("is_active = ?", true).Find(&nasList).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get NAS list",
		})
	}

	// Return empty if no NAS devices
	if len(nasList) == 0 {
		return c.JSON(fiber.Map{
			"success": true,
			"data":    []NASRuleStatus{},
		})
	}

	results := make([]NASRuleStatus, len(nasList))
	resultChan := make(chan struct {
		idx    int
		status NASRuleStatus
	}, len(nasList))

	// Start goroutines with individual timeouts
	for i, nas := range nasList {
		go func(idx int, n models.Nas) {
			status := NASRuleStatus{
				NASID:        n.ID,
				NASName:      n.Name,
				NASIPAddress: n.IPAddress,
			}

			// Create a channel for this specific NAS check
			checkDone := make(chan bool, 1)
			go func() {
				client := mikrotik.NewClient(
					fmt.Sprintf("%s:%d", n.IPAddress, n.APIPort),
					n.APIUsername,
					n.APIPassword,
				)
				defer client.Close()

				count, err := client.CountTTLRules(getTTLRuleComment())
				if err != nil {
					status.Error = err.Error()
				} else {
					status.RuleCount = count
					status.RulesConfigured = count >= 4
				}
				checkDone <- true
			}()

			// Wait max 5 seconds per NAS
			select {
			case <-checkDone:
				// Completed
			case <-time.After(5 * time.Second):
				status.Error = "Connection timeout"
			}

			resultChan <- struct {
				idx    int
				status NASRuleStatus
			}{idx, status}
		}(i, nas)
	}

	// Collect results with overall 10 second timeout
	timeout := time.After(10 * time.Second)
	collected := 0
	for collected < len(nasList) {
		select {
		case result := <-resultChan:
			results[result.idx] = result.status
			collected++
		case <-timeout:
			// Fill remaining with timeout error
			for i := range results {
				if results[i].NASID == 0 {
					results[i] = NASRuleStatus{
						NASID:        nasList[i].ID,
						NASName:      nasList[i].Name,
						NASIPAddress: nasList[i].IPAddress,
						Error:        "Connection timeout",
					}
				}
			}
			log.Printf("SharingDetection: ListNASRuleStatus timed out, collected %d/%d", collected, len(nasList))
			return c.JSON(fiber.Map{
				"success": true,
				"data":    results,
			})
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    results,
	})
}

// GenerateTTLRules creates TTL detection mangle rules on a NAS
func (h *SharingDetectionHandler) GenerateTTLRules(c *fiber.Ctx) error {
	nasID := c.Params("nas_id")

	var nas models.Nas
	if err := database.DB.First(&nas, nasID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "NAS not found",
		})
	}

	client := mikrotik.NewClient(
		fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
		nas.APIUsername,
		nas.APIPassword,
	)
	defer client.Close()

	// Create the TTL detection rules
	rules := []struct {
		TTL  int
		Mark string
		Desc string
	}{
		{127, "ttl_127", "Windows behind router"},
		{63, "ttl_63", "Linux/Android behind router"},
		{128, "ttl_128", "Direct Windows"},
		{64, "ttl_64", "Direct Linux/Android"},
	}

	createdCount := 0
	var errors []string

	for _, rule := range rules {
		err := client.CreateTTLMangleRule(rule.TTL, rule.Mark, getTTLRuleComment()+" - "+rule.Desc)
		if err != nil {
			errors = append(errors, fmt.Sprintf("TTL=%d: %s", rule.TTL, err.Error()))
		} else {
			createdCount++
		}
	}

	if createdCount == 0 {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create any rules",
			"errors":  errors,
		})
	}

	return c.JSON(fiber.Map{
		"success":       true,
		"message":       fmt.Sprintf("Created %d TTL detection rules on %s", createdCount, nas.Name),
		"created_count": createdCount,
		"errors":        errors,
	})
}

// RemoveTTLRules removes TTL detection rules from a NAS
func (h *SharingDetectionHandler) RemoveTTLRules(c *fiber.Ctx) error {
	nasID := c.Params("nas_id")

	var nas models.Nas
	if err := database.DB.First(&nas, nasID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "NAS not found",
		})
	}

	client := mikrotik.NewClient(
		fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
		nas.APIUsername,
		nas.APIPassword,
	)
	defer client.Close()

	removedCount, err := client.RemoveTTLRules(getTTLRuleComment())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to remove rules: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success":       true,
		"message":       fmt.Sprintf("Removed %d TTL detection rules from %s", removedCount, nas.Name),
		"removed_count": removedCount,
	})
}

// GetHistory returns historical sharing detections
func (h *SharingDetectionHandler) GetHistory(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	suspicionLevel := c.Query("suspicion_level", "")
	username := c.Query("username", "")
	days := c.QueryInt("days", 7)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	offset := (page - 1) * limit

	// Build query
	query := database.DB.Model(&models.SharingDetection{})

	// Filter by date range
	cutoff := time.Now().AddDate(0, 0, -days)
	query = query.Where("detected_at >= ?", cutoff)

	// Filter by suspicion level
	if suspicionLevel != "" {
		query = query.Where("suspicion_level = ?", suspicionLevel)
	}

	// Filter by username
	if username != "" {
		query = query.Where("username ILIKE ?", "%"+username+"%")
	}

	// Get total count
	var total int64
	query.Count(&total)

	// Get records
	var detections []models.SharingDetection
	if err := query.Order("detected_at DESC").Offset(offset).Limit(limit).Find(&detections).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get history",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    detections,
		"meta": fiber.Map{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// GetRepeatOffenders returns subscribers detected multiple times
func (h *SharingDetectionHandler) GetRepeatOffenders(c *fiber.Ctx) error {
	days := c.QueryInt("days", 30)
	minCount := c.QueryInt("min_count", 3)

	cutoff := time.Now().AddDate(0, 0, -days)

	type RepeatOffender struct {
		SubscriberID    uint    `json:"subscriber_id"`
		Username        string  `json:"username"`
		FullName        string  `json:"full_name"`
		DetectionCount  int     `json:"detection_count"`
		AvgConfidence   float64 `json:"avg_confidence"`
		HighRiskCount   int     `json:"high_risk_count"`
		LastDetectedAt  time.Time `json:"last_detected_at"`
		ServiceName     string  `json:"service_name"`
	}

	var offenders []RepeatOffender
	err := database.DB.Model(&models.SharingDetection{}).
		Select(`
			subscriber_id,
			username,
			MAX(full_name) as full_name,
			COUNT(*) as detection_count,
			AVG(confidence_score) as avg_confidence,
			SUM(CASE WHEN suspicion_level = 'high' THEN 1 ELSE 0 END) as high_risk_count,
			MAX(detected_at) as last_detected_at,
			MAX(service_name) as service_name
		`).
		Where("detected_at >= ?", cutoff).
		Group("subscriber_id, username").
		Having("COUNT(*) >= ?", minCount).
		Order("detection_count DESC").
		Scan(&offenders).Error

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get repeat offenders",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    offenders,
	})
}

// GetTrends returns sharing detection trends over time
func (h *SharingDetectionHandler) GetTrends(c *fiber.Ctx) error {
	days := c.QueryInt("days", 7)

	type DailyTrend struct {
		Date           string `json:"date"`
		TotalDetected  int    `json:"total_detected"`
		HighRiskCount  int    `json:"high_risk_count"`
		MediumRiskCount int   `json:"medium_risk_count"`
		AvgConfidence  float64 `json:"avg_confidence"`
	}

	var trends []DailyTrend
	cutoff := time.Now().AddDate(0, 0, -days)

	err := database.DB.Model(&models.SharingDetection{}).
		Select(`
			DATE(detected_at) as date,
			COUNT(*) as total_detected,
			SUM(CASE WHEN suspicion_level = 'high' THEN 1 ELSE 0 END) as high_risk_count,
			SUM(CASE WHEN suspicion_level = 'medium' THEN 1 ELSE 0 END) as medium_risk_count,
			AVG(confidence_score) as avg_confidence
		`).
		Where("detected_at >= ?", cutoff).
		Group("DATE(detected_at)").
		Order("date DESC").
		Scan(&trends).Error

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get trends",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    trends,
	})
}

// GetSettings returns sharing detection settings
func (h *SharingDetectionHandler) GetSettings(c *fiber.Ctx) error {
	var settings models.SharingDetectionSetting
	if err := database.DB.First(&settings).Error; err != nil {
		// Return defaults
		settings = models.SharingDetectionSetting{
			Enabled:             true,
			ScanTime:            "03:00",
			RetentionDays:       30,
			MinSuspicionLevel:   "medium",
			ConnectionThreshold: 500,
			NotifyOnHighRisk:    false,
			AutoSuspendRepeat:   false,
			RepeatThreshold:     5,
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    settings,
	})
}

// UpdateSettings updates sharing detection settings
func (h *SharingDetectionHandler) UpdateSettings(c *fiber.Ctx) error {
	var req struct {
		Enabled                 *bool   `json:"enabled"`
		ScanTime                string  `json:"scan_time"`
		RetentionDays           int     `json:"retention_days"`
		MinSuspicionLevel       string  `json:"min_suspicion_level"`
		ConnectionThreshold     int     `json:"connection_threshold"`
		NotifyOnHighRisk        *bool   `json:"notify_on_high_risk"`
		AutoSuspendRepeat       *bool   `json:"auto_suspend_repeat"`
		RepeatThreshold         int     `json:"repeat_threshold"`
		AutoFlagEnabled         *bool   `json:"auto_flag_enabled"`
		AutoFlagThreshold       int     `json:"auto_flag_threshold"`
		SpeedReductionEnabled   *bool   `json:"speed_reduction_enabled"`
		SpeedReductionThreshold int     `json:"speed_reduction_threshold"`
		SpeedReductionPercent   int     `json:"speed_reduction_percent"`
		WhatsappNotifyEnabled   *bool   `json:"whatsapp_notify_enabled"`
		WhatsappNotifyThreshold int     `json:"whatsapp_notify_threshold"`
		WhatsappNotifyTemplate  string  `json:"whatsapp_notify_template"`
		ScoreEnabled            *bool   `json:"score_enabled"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Get or create settings
	var settings models.SharingDetectionSetting
	if err := database.DB.First(&settings).Error; err != nil {
		settings = models.SharingDetectionSetting{}
	}

	// Update fields
	if req.Enabled != nil {
		settings.Enabled = *req.Enabled
	}
	if req.ScanTime != "" {
		settings.ScanTime = req.ScanTime
	}
	if req.RetentionDays > 0 {
		settings.RetentionDays = req.RetentionDays
	}
	if req.MinSuspicionLevel != "" {
		settings.MinSuspicionLevel = req.MinSuspicionLevel
	}
	if req.ConnectionThreshold > 0 {
		settings.ConnectionThreshold = req.ConnectionThreshold
	}
	if req.NotifyOnHighRisk != nil {
		settings.NotifyOnHighRisk = *req.NotifyOnHighRisk
	}
	if req.AutoSuspendRepeat != nil {
		settings.AutoSuspendRepeat = *req.AutoSuspendRepeat
	}
	if req.RepeatThreshold > 0 {
		settings.RepeatThreshold = req.RepeatThreshold
	}
	// Automated actions
	if req.AutoFlagEnabled != nil {
		settings.AutoFlagEnabled = *req.AutoFlagEnabled
	}
	if req.AutoFlagThreshold > 0 {
		settings.AutoFlagThreshold = req.AutoFlagThreshold
	}
	if req.SpeedReductionEnabled != nil {
		settings.SpeedReductionEnabled = *req.SpeedReductionEnabled
	}
	if req.SpeedReductionThreshold > 0 {
		settings.SpeedReductionThreshold = req.SpeedReductionThreshold
	}
	if req.SpeedReductionPercent > 0 {
		settings.SpeedReductionPercent = req.SpeedReductionPercent
	}
	if req.WhatsappNotifyEnabled != nil {
		settings.WhatsappNotifyEnabled = *req.WhatsappNotifyEnabled
	}
	if req.WhatsappNotifyThreshold > 0 {
		settings.WhatsappNotifyThreshold = req.WhatsappNotifyThreshold
	}
	if req.WhatsappNotifyTemplate != "" {
		settings.WhatsappNotifyTemplate = req.WhatsappNotifyTemplate
	}
	if req.ScoreEnabled != nil {
		settings.ScoreEnabled = *req.ScoreEnabled
	}
	settings.UpdatedAt = time.Now()

	if err := database.DB.Save(&settings).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to save settings",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Settings updated",
		"data":    settings,
	})
}

// RunManualScan triggers an immediate scan
func (h *SharingDetectionHandler) RunManualScan(c *fiber.Ctx) error {
	// Import service here to avoid circular dependency
	// The actual scan will be done inline since we can't import services package

	// Get settings
	var settings models.SharingDetectionSetting
	if err := database.DB.First(&settings).Error; err != nil {
		settings = models.SharingDetectionSetting{
			MinSuspicionLevel:   "medium",
			ConnectionThreshold: 500,
		}
	}

	// Get all online subscribers
	var subscribers []models.Subscriber
	if err := database.DB.Preload("Nas").Preload("Service").
		Where("is_online = ?", true).Find(&subscribers).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get subscribers",
		})
	}

	if len(subscribers) == 0 {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "No online subscribers to scan",
			"saved":   0,
		})
	}

	// Group by NAS
	nasSubs := make(map[uint][]models.Subscriber)
	for _, sub := range subscribers {
		if sub.NasID != nil {
			nasSubs[*sub.NasID] = append(nasSubs[*sub.NasID], sub)
		}
	}

	// Analyze each NAS
	var allDetections []models.SharingDetection
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, subs := range nasSubs {
		if len(subs) == 0 || subs[0].Nas == nil {
			continue
		}

		wg.Add(1)
		go func(nas *models.Nas, subscribers []models.Subscriber) {
			defer wg.Done()

			client := mikrotik.NewClient(
				fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
				nas.APIUsername,
				nas.APIPassword,
			)
			defer client.Close()

			connStats, _ := client.GetAllConnectionStats()
			if connStats == nil {
				connStats = make(map[string]*mikrotik.ConnectionStats)
			}

			ttlMarks, _ := client.GetAllTTLMarks()
			if ttlMarks == nil {
				ttlMarks = make(map[string][]int)
			}

			for _, sub := range subscribers {
				if sub.IPAddress == "" {
					continue
				}

				detection := models.SharingDetection{
					SubscriberID: sub.ID,
					Username:     sub.Username,
					FullName:     sub.FullName,
					IPAddress:    sub.IPAddress,
					NasID:        sub.NasID,
					NasName:      nas.Name,
					ScanType:     "manual",
					DetectedAt:   time.Now(),
				}

				if sub.Service != nil {
					detection.ServiceName = sub.Service.Name
				}

				if stats := connStats[sub.IPAddress]; stats != nil {
					detection.ConnectionCount = stats.TotalConnections
					detection.UniqueDestinations = stats.UniqueDestinations
				}

				// Analyze TTL
				ttlValues := ttlMarks[sub.IPAddress]
				ttlStatus, reasons := analyzeTTL(ttlValues)
				detection.TTLStatus = ttlStatus

				// Calculate suspicion level
				detection.SuspicionLevel = calculateSuspicionLevel(
					detection.ConnectionCount,
					ttlStatus,
					settings.ConnectionThreshold,
					settings.ConnectionThreshold*2,
				)

				// Calculate confidence
				detection.ConfidenceScore = calculateConfidenceScore(
					detection.ConnectionCount,
					detection.UniqueDestinations,
					0,
					ttlStatus,
				)

				// Only save medium/high
				levelOrder := map[string]int{"low": 1, "medium": 2, "high": 3}
				if levelOrder[detection.SuspicionLevel] >= levelOrder[settings.MinSuspicionLevel] {
					if len(reasons) > 0 {
						reasonsJSON, _ := json.Marshal(reasons)
						detection.Reasons = string(reasonsJSON)
					}
					mu.Lock()
					allDetections = append(allDetections, detection)
					mu.Unlock()
				}
			}
		}(subs[0].Nas, subs)
	}

	wg.Wait()

	// Save to database
	savedCount := 0
	for _, detection := range allDetections {
		if err := database.DB.Create(&detection).Error; err == nil {
			savedCount++
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Manual scan completed. Found %d suspicious accounts.", savedCount),
		"saved":   savedCount,
		"scanned": len(subscribers),
	})
}

// GetMonthlyScores returns paginated monthly sharing scores
func (h *SharingDetectionHandler) GetMonthlyScores(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	month := c.Query("month", time.Now().Format("2006-01"))
	category := c.Query("category", "")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	offset := (page - 1) * limit

	// Build query
	query := database.DB.Table("subscriber_sharing_scores ss").
		Select(`ss.*, s.username, s.full_name, sv.name as service_name`).
		Joins("LEFT JOIN subscribers s ON s.id = ss.subscriber_id").
		Joins("LEFT JOIN services sv ON sv.id = s.service_id").
		Where("ss.month = ?", month)

	if category != "" {
		query = query.Where("ss.category = ?", category)
	}

	var total int64
	query.Count(&total)

	type ScoreRow struct {
		models.SubscriberSharingScore
		Username    string `json:"username"`
		FullName    string `json:"full_name"`
		ServiceName string `json:"service_name"`
	}

	var scores []ScoreRow
	if err := query.Order("ss.score DESC").Offset(offset).Limit(limit).Scan(&scores).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get scores",
		})
	}

	// Summary counts
	type SumRow struct {
		Category string
		Count    int64
	}
	var summary []SumRow
	database.DB.Table("subscriber_sharing_scores").
		Select("category, COUNT(*) as count").
		Where("month = ?", month).
		Group("category").Scan(&summary)

	summaryMap := map[string]int64{"good": 0, "warning": 0, "bad": 0}
	for _, s := range summary {
		summaryMap[s.Category] = s.Count
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    scores,
		"summary": summaryMap,
		"meta": fiber.Map{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// GetSubscriberScoreHistory returns last 12 months of scores for one subscriber
func (h *SharingDetectionHandler) GetSubscriberScoreHistory(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid subscriber ID",
		})
	}

	var scores []models.SubscriberSharingScore
	if err := database.DB.Where("subscriber_id = ?", id).
		Order("month DESC").Limit(12).Find(&scores).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get score history",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    scores,
	})
}

// ToggleWhitelist adds/removes a subscriber from sharing detection whitelist
func (h *SharingDetectionHandler) ToggleWhitelist(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid subscriber ID",
		})
	}

	var req struct {
		Whitelisted bool   `json:"whitelisted"`
		Reason      string `json:"reason"` // business / family / server / other
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	var sub models.Subscriber
	if err := database.DB.First(&sub, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Subscriber not found",
		})
	}

	sub.SharingWhitelisted = req.Whitelisted
	if req.Whitelisted {
		sub.SharingWhitelistReason = req.Reason
	} else {
		sub.SharingWhitelistReason = ""
	}

	if err := database.DB.Model(&sub).Updates(map[string]interface{}{
		"sharing_whitelisted":      sub.SharingWhitelisted,
		"sharing_whitelist_reason": sub.SharingWhitelistReason,
	}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update whitelist",
		})
	}

	// Log the action
	actionType := "manual_whitelist"
	details := fmt.Sprintf("Whitelisted with reason: %s", req.Reason)
	if !req.Whitelisted {
		actionType = "manual_whitelist"
		details = "Removed from whitelist"
	}
	database.DB.Create(&models.SharingActionLog{
		SubscriberID:    sub.ID,
		Username:        sub.Username,
		ActionType:      actionType,
		TriggerCategory: req.Reason,
		Details:         details,
		CreatedAt:       time.Now(),
	})

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Whitelist updated",
	})
}

// GetWhitelistedSubscribers returns all whitelisted subscribers
func (h *SharingDetectionHandler) GetWhitelistedSubscribers(c *fiber.Ctx) error {
	type WhitelistedSub struct {
		ID       uint   `json:"id"`
		Username string `json:"username"`
		FullName string `json:"full_name"`
		Reason   string `json:"reason"`
	}

	var subs []WhitelistedSub
	if err := database.DB.Model(&models.Subscriber{}).
		Select("id, username, full_name, sharing_whitelist_reason as reason").
		Where("sharing_whitelisted = ?", true).
		Scan(&subs).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get whitelisted subscribers",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    subs,
	})
}

// GetActionLogs returns paginated sharing action logs
func (h *SharingDetectionHandler) GetActionLogs(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	days := c.QueryInt("days", 30)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	offset := (page - 1) * limit

	cutoff := time.Now().AddDate(0, 0, -days)

	var total int64
	database.DB.Model(&models.SharingActionLog{}).Where("created_at >= ?", cutoff).Count(&total)

	var logs []models.SharingActionLog
	if err := database.DB.Where("created_at >= ?", cutoff).
		Order("created_at DESC").Offset(offset).Limit(limit).
		Find(&logs).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to get action logs",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    logs,
		"meta": fiber.Map{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// Suppress unused import warnings
var _ = math.Max
var _ = strconv.Itoa
