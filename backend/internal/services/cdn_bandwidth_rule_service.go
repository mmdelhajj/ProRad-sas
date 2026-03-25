package services

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/mikrotik"
	"github.com/proisp/backend/internal/models"
)

// CDNBandwidthRule is an alias to models.CDNBandwidthRule
type CDNBandwidthRule = models.CDNBandwidthRule

// CDNBandwidthRuleService handles periodic CDN bandwidth rule enforcement
type CDNBandwidthRuleService struct {
	interval         time.Duration
	stopChan         chan struct{}
	wg               sync.WaitGroup
	mu               sync.RWMutex
	activeRules      map[uint]bool                         // ruleID -> is currently active
	appliedToQueues  map[string]*AppliedCDNBandwidthRule   // queueKey (nasID-username-cdnID) -> applied rule info
}

// AppliedCDNBandwidthRule tracks which CDN bandwidth rule is applied to a queue
type AppliedCDNBandwidthRule struct {
	RuleID         uint
	AppliedSpeedK  int64 // Applied speed in Kbps
	OriginalSpeedK int64 // Original CDN speed in Kbps
	NasID          uint
	Username       string
	CDNID          uint
}

// NewCDNBandwidthRuleService creates a new CDN bandwidth rule service
func NewCDNBandwidthRuleService(interval time.Duration) *CDNBandwidthRuleService {
	return &CDNBandwidthRuleService{
		interval:        interval,
		stopChan:        make(chan struct{}),
		activeRules:     make(map[uint]bool),
		appliedToQueues: make(map[string]*AppliedCDNBandwidthRule),
	}
}

// Start begins the CDN bandwidth rule enforcement background job
func (s *CDNBandwidthRuleService) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		log.Printf("CDNBandwidthRuleService started, checking every %v", s.interval)

		// Run immediately on start
		s.enforceRules()

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.enforceRules()
			case <-s.stopChan:
				log.Println("CDNBandwidthRuleService stopped")
				return
			}
		}
	}()
}

// Stop stops the CDN bandwidth rule service
func (s *CDNBandwidthRuleService) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

// enforceRules checks all CDN bandwidth rules and applies/removes them as needed
func (s *CDNBandwidthRuleService) enforceRules() {
	now := getNow()
	currentTime := now.Format("15:04")
	currentWeekday := int(now.Weekday()) // 0=Sunday, 1=Monday, etc.

	// Get all enabled CDN bandwidth rules with auto_apply enabled
	var rules []CDNBandwidthRule
	if err := database.DB.Where("enabled = ? AND auto_apply = ?", true, true).Order("priority ASC").Find(&rules).Error; err != nil {
		log.Printf("CDNBandwidthRule: Failed to get rules: %v", err)
		return
	}

	// Determine which rules are currently active
	currentlyActiveRules := make(map[uint]*CDNBandwidthRule)
	for i := range rules {
		rule := &rules[i]
		if s.isRuleActive(rule, currentTime, currentWeekday) {
			currentlyActiveRules[rule.ID] = rule
		}
	}

	// Find rules that just became active
	s.mu.Lock()
	for ruleID, rule := range currentlyActiveRules {
		if !s.activeRules[ruleID] {
			log.Printf("CDNBandwidthRule: Rule '%s' (ID=%d) is now ACTIVE", rule.Name, ruleID)
			s.activeRules[ruleID] = true
			// Apply this rule to affected CDN queues
			go s.applyRule(rule)
		}
	}

	// Find rules that just became inactive
	for ruleID := range s.activeRules {
		if _, stillActive := currentlyActiveRules[ruleID]; !stillActive {
			// Find the rule name for logging
			var ruleName string
			for _, r := range rules {
				if r.ID == ruleID {
					ruleName = r.Name
					break
				}
			}
			log.Printf("CDNBandwidthRule: Rule '%s' (ID=%d) is now INACTIVE", ruleName, ruleID)
			delete(s.activeRules, ruleID)
			// Restore CDN speeds for queues affected by this rule
			go s.restoreSpeedsForRule(ruleID)
		}
	}
	s.mu.Unlock()
}

// isRuleActive checks if a CDN rule should be active at the current time
func (s *CDNBandwidthRuleService) isRuleActive(rule *CDNBandwidthRule, currentTime string, currentWeekday int) bool {
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
		// Normal range (e.g., 16:00 to 23:00)
		return currentTime >= rule.StartTime && currentTime < rule.EndTime
	} else {
		// Crosses midnight (e.g., 22:00 to 06:00)
		return currentTime >= rule.StartTime || currentTime < rule.EndTime
	}
}

// applyRule applies a CDN bandwidth rule to all affected online subscribers' CDN queues
func (s *CDNBandwidthRuleService) applyRule(rule *CDNBandwidthRule) {
	// Parse CDN IDs (empty = rule doesn't apply to any CDN)
	var cdnIDs []uint
	if len(rule.CDNIDs) > 0 {
		if err := json.Unmarshal(rule.CDNIDs, &cdnIDs); err != nil {
			log.Printf("CDNBandwidthRule: Failed to parse CDN IDs for rule %d: %v", rule.ID, err)
		}
	}

	// If no CDNs selected, skip this rule
	if len(cdnIDs) == 0 {
		log.Printf("CDNBandwidthRule: Rule '%s' has no CDNs selected, skipping", rule.Name)
		return
	}

	// Parse service IDs (empty = all services)
	var serviceIDs []uint
	if len(rule.ServiceIDs) > 0 {
		if err := json.Unmarshal(rule.ServiceIDs, &serviceIDs); err != nil {
			log.Printf("CDNBandwidthRule: Failed to parse service IDs for rule %d: %v", rule.ID, err)
		}
	}

	// Get all online subscribers
	query := database.DB.Preload("Nas").Preload("Service").Where("is_online = ?", true)
	if len(serviceIDs) > 0 {
		query = query.Where("service_id IN ?", serviceIDs)
	}

	var subscribers []models.Subscriber
	if err := query.Find(&subscribers).Error; err != nil {
		log.Printf("CDNBandwidthRule: Failed to get subscribers for rule %d: %v", rule.ID, err)
		return
	}

	log.Printf("CDNBandwidthRule: Applying rule '%s' (multiplier %d%%) to CDN queues for %d online subscribers",
		rule.Name, rule.SpeedMultiplier, len(subscribers))

	// Group by NAS for efficiency
	nasSubs := make(map[uint][]models.Subscriber)
	for _, sub := range subscribers {
		if sub.NasID != nil {
			nasSubs[*sub.NasID] = append(nasSubs[*sub.NasID], sub)
		}
	}

	// Apply to each NAS
	for _, subs := range nasSubs {
		if len(subs) == 0 || subs[0].Nas == nil {
			continue
		}
		nas := subs[0].Nas
		s.applyRuleToNasSubscribers(rule, nas, subs, cdnIDs)
	}
}

// applyRuleToNasSubscribers applies a CDN bandwidth rule to subscribers on a specific NAS
func (s *CDNBandwidthRuleService) applyRuleToNasSubscribers(rule *CDNBandwidthRule, nas *models.Nas, subscribers []models.Subscriber, cdnIDs []uint) {
	client := mikrotik.NewClient(
		fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
		nas.APIUsername,
		nas.APIPassword,
	)
	defer client.Close()

	for _, sub := range subscribers {
		// Get service CDN configurations for this subscriber
		var serviceCDNs []models.ServiceCDN
		cdnQuery := database.DB.Preload("CDN").Where("service_id = ? AND is_active = ?", sub.ServiceID, true)
		if len(cdnIDs) > 0 {
			cdnQuery = cdnQuery.Where("cdn_id IN ?", cdnIDs)
		}
		cdnQuery.Find(&serviceCDNs)

		if len(serviceCDNs) == 0 {
			continue
		}

		// Apply rule to each CDN queue
		for _, scdm := range serviceCDNs {
			if scdm.CDN == nil || scdm.CDN.ID == 0 || !scdm.CDN.IsActive {
				continue
			}

			// SpeedLimit is already stored in Kbps
			originalSpeedK := scdm.SpeedLimit
			if originalSpeedK <= 0 {
				// No specific limit, skip
				continue
			}

			// Apply multiplier percentage (100% = same speed, 200% = double speed)
			newSpeedK := originalSpeedK * int64(rule.SpeedMultiplier) / 100

			// Create queue key for tracking
			queueKey := fmt.Sprintf("%d-%s-%d", nas.ID, sub.Username, scdm.CDNID)

			// Skip if already has higher priority rule
			s.mu.RLock()
			if applied, exists := s.appliedToQueues[queueKey]; exists {
				var existingRule CDNBandwidthRule
				if database.DB.First(&existingRule, applied.RuleID).Error == nil {
					if existingRule.Priority < rule.Priority {
						s.mu.RUnlock()
						continue
					}
				}
			}
			s.mu.RUnlock()

			// Update CDN queue on MikroTik
			queueName := fmt.Sprintf("cdn-%s-%s", sub.Username, scdm.CDN.Name)
			if err := client.UpdateCDNQueueSpeed(queueName, int(newSpeedK)); err != nil {
				log.Printf("CDNBandwidthRule: Failed to update queue %s: %v", queueName, err)
				continue
			}

			// Track applied rule
			s.mu.Lock()
			s.appliedToQueues[queueKey] = &AppliedCDNBandwidthRule{
				RuleID:         rule.ID,
				AppliedSpeedK:  newSpeedK,
				OriginalSpeedK: originalSpeedK,
				NasID:          nas.ID,
				Username:       sub.Username,
				CDNID:          scdm.CDNID,
			}
			s.mu.Unlock()

			log.Printf("CDNBandwidthRule: Applied %s to %s (CDN: %s, %d%% -> %dK)",
				rule.Name, sub.Username, scdm.CDN.Name, rule.SpeedMultiplier, newSpeedK)
		}
	}
}

// restoreSpeedsForRule restores original CDN speeds for queues affected by a rule
func (s *CDNBandwidthRuleService) restoreSpeedsForRule(ruleID uint) {
	// Find all queues with this rule applied
	s.mu.Lock()
	queuesToRestore := make(map[string]*AppliedCDNBandwidthRule)
	for queueKey, applied := range s.appliedToQueues {
		if applied.RuleID == ruleID {
			queuesToRestore[queueKey] = applied
			delete(s.appliedToQueues, queueKey)
		}
	}
	s.mu.Unlock()

	if len(queuesToRestore) == 0 {
		return
	}

	log.Printf("CDNBandwidthRule: Restoring CDN speeds for %d queues (rule %d ended)", len(queuesToRestore), ruleID)

	// Group by NAS for efficiency
	nasQueues := make(map[uint][]*AppliedCDNBandwidthRule)
	for _, applied := range queuesToRestore {
		nasQueues[applied.NasID] = append(nasQueues[applied.NasID], applied)
	}

	// Restore for each NAS
	for nasID, queues := range nasQueues {
		var nas models.Nas
		if err := database.DB.First(&nas, nasID).Error; err != nil {
			log.Printf("CDNBandwidthRule: Failed to get NAS %d: %v", nasID, err)
			continue
		}

		client := mikrotik.NewClient(
			fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
			nas.APIUsername,
			nas.APIPassword,
		)

		for _, applied := range queues {
			// Get CDN name
			var cdn models.CDN
			if err := database.DB.First(&cdn, applied.CDNID).Error; err != nil {
				continue
			}

			queueName := fmt.Sprintf("cdn-%s-%s", applied.Username, cdn.Name)
			if err := client.UpdateCDNQueueSpeed(queueName, int(applied.OriginalSpeedK)); err != nil {
				log.Printf("CDNBandwidthRule: Failed to restore queue %s: %v", queueName, err)
				continue
			}

			log.Printf("CDNBandwidthRule: Restored %s CDN queue to %dK", applied.Username, applied.OriginalSpeedK)
		}

		client.Close()
	}
}

// ApplyRuleNow manually applies a CDN bandwidth rule immediately (for "Apply Now" button)
func (s *CDNBandwidthRuleService) ApplyRuleNow(ruleID uint) (int, error) {
	var rule CDNBandwidthRule
	if err := database.DB.First(&rule, ruleID).Error; err != nil {
		return 0, err
	}

	// Parse CDN IDs
	var cdnIDs []uint
	if len(rule.CDNIDs) > 0 {
		if err := json.Unmarshal(rule.CDNIDs, &cdnIDs); err != nil {
			return 0, fmt.Errorf("failed to parse CDN IDs: %v", err)
		}
	}

	// If no CDNs selected, return error
	if len(cdnIDs) == 0 {
		return 0, fmt.Errorf("no CDNs selected for this rule")
	}

	// Parse service IDs
	var serviceIDs []uint
	if len(rule.ServiceIDs) > 0 {
		if err := json.Unmarshal(rule.ServiceIDs, &serviceIDs); err != nil {
			return 0, fmt.Errorf("failed to parse service IDs: %v", err)
		}
	}

	// Get all online subscribers
	query := database.DB.Preload("Nas").Preload("Service").Where("is_online = ?", true)
	if len(serviceIDs) > 0 {
		query = query.Where("service_id IN ?", serviceIDs)
	}

	var subscribers []models.Subscriber
	if err := query.Find(&subscribers).Error; err != nil {
		return 0, fmt.Errorf("failed to get subscribers: %v", err)
	}

	log.Printf("CDNBandwidthRule: Manually applying rule '%s' to CDN queues for %d online subscribers",
		rule.Name, len(subscribers))

	// Group by NAS for efficiency
	nasSubs := make(map[uint][]models.Subscriber)
	for _, sub := range subscribers {
		if sub.NasID != nil {
			nasSubs[*sub.NasID] = append(nasSubs[*sub.NasID], sub)
		}
	}

	// Apply to each NAS
	appliedCount := 0
	for _, subs := range nasSubs {
		if len(subs) == 0 || subs[0].Nas == nil {
			continue
		}
		nas := subs[0].Nas
		applied := s.applyRuleToNasSubscribersCount(&rule, nas, subs, cdnIDs)
		appliedCount += applied
	}

	return appliedCount, nil
}

// applyRuleToNasSubscribersCount applies a rule and returns count of successful applications
func (s *CDNBandwidthRuleService) applyRuleToNasSubscribersCount(rule *CDNBandwidthRule, nas *models.Nas, subscribers []models.Subscriber, cdnIDs []uint) int {
	client := mikrotik.NewClient(
		fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
		nas.APIUsername,
		nas.APIPassword,
	)
	defer client.Close()

	appliedCount := 0

	for _, sub := range subscribers {
		// Get service CDN configurations for this subscriber
		var serviceCDNs []models.ServiceCDN
		cdnQuery := database.DB.Preload("CDN").Where("service_id = ? AND is_active = ?", sub.ServiceID, true)
		if len(cdnIDs) > 0 {
			cdnQuery = cdnQuery.Where("cdn_id IN ?", cdnIDs)
		}
		cdnQuery.Find(&serviceCDNs)

		if len(serviceCDNs) == 0 {
			continue
		}

		for _, scdm := range serviceCDNs {
			if scdm.CDN == nil || scdm.CDN.ID == 0 || !scdm.CDN.IsActive {
				continue
			}

			// SpeedLimit is already stored in Kbps
			originalSpeedK := scdm.SpeedLimit
			if originalSpeedK <= 0 {
				continue
			}

			// Apply multiplier percentage (100% = same speed, 200% = double speed)
			newSpeedK := originalSpeedK * int64(rule.SpeedMultiplier) / 100
			queueName := fmt.Sprintf("cdn-%s-%s", sub.Username, scdm.CDN.Name)

			if err := client.UpdateCDNQueueSpeed(queueName, int(newSpeedK)); err != nil {
				log.Printf("CDNBandwidthRule: Failed to update queue %s: %v", queueName, err)
				continue
			}

			appliedCount++
			log.Printf("CDNBandwidthRule: Applied %s to %s (CDN: %s, %d%% -> %dK)",
				rule.Name, sub.Username, scdm.CDN.Name, rule.SpeedMultiplier, newSpeedK)
		}
	}

	return appliedCount
}
