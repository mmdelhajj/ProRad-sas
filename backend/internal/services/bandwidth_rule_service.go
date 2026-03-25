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
	"github.com/proisp/backend/internal/radius"
)

// BandwidthRule is an alias to models.BandwidthRule
type BandwidthRule = models.BandwidthRule

// BandwidthRuleService handles periodic bandwidth rule enforcement
type BandwidthRuleService struct {
	interval         time.Duration
	stopChan         chan struct{}
	wg               sync.WaitGroup
	mu               sync.RWMutex
	activeRules      map[uint]bool                    // ruleID -> is currently active
	appliedToUsers   map[string]*AppliedBandwidthRule // username -> applied rule info
}

// AppliedBandwidthRule tracks which bandwidth rule is applied to a user
type AppliedBandwidthRule struct {
	RuleID             uint
	AppliedDownloadK   int64 // Applied speed in Kbps
	AppliedUploadK     int64
	OriginalDownloadM  int64 // Original service speed in Mbps
	OriginalUploadM    int64
	FUPLevel           int // User's FUP level at time of application
}

// NewBandwidthRuleService creates a new bandwidth rule service
func NewBandwidthRuleService(interval time.Duration) *BandwidthRuleService {
	return &BandwidthRuleService{
		interval:       interval,
		stopChan:       make(chan struct{}),
		activeRules:    make(map[uint]bool),
		appliedToUsers: make(map[string]*AppliedBandwidthRule),
	}
}

// Start begins the bandwidth rule enforcement background job
func (s *BandwidthRuleService) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		log.Printf("BandwidthRuleService started, checking every %v", s.interval)

		// Run immediately on start
		s.enforceRules()

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.enforceRules()
			case <-s.stopChan:
				log.Println("BandwidthRuleService stopped")
				return
			}
		}
	}()
}

// Stop stops the bandwidth rule service
func (s *BandwidthRuleService) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

// enforceRules checks all bandwidth rules and applies/removes them as needed
func (s *BandwidthRuleService) enforceRules() {
	now := getNow()
	currentTime := now.Format("15:04")
	currentWeekday := int(now.Weekday()) // 0=Sunday, 1=Monday, etc.

	// Get all enabled bandwidth rules with auto_apply enabled
	var rules []BandwidthRule
	if err := database.DB.Where("enabled = ? AND auto_apply = ?", true, true).Order("priority ASC").Find(&rules).Error; err != nil {
		log.Printf("BandwidthRule: Failed to get rules: %v", err)
		return
	}

	// Determine which rules are currently active
	currentlyActiveRules := make(map[uint]*BandwidthRule)
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
			log.Printf("BandwidthRule: Rule '%s' (ID=%d) is now ACTIVE", rule.Name, ruleID)
			s.activeRules[ruleID] = true
			// Apply this rule to affected subscribers
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
			log.Printf("BandwidthRule: Rule '%s' (ID=%d) is now INACTIVE", ruleName, ruleID)
			delete(s.activeRules, ruleID)
			// Restore speeds for subscribers affected by this rule
			go s.restoreSpeedsForRule(ruleID)
		}
	}
	s.mu.Unlock()
}

// isRuleActive checks if a rule should be active at the current time
func (s *BandwidthRuleService) isRuleActive(rule *BandwidthRule, currentTime string, currentWeekday int) bool {
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

// applyRule applies a bandwidth rule to all affected online subscribers
func (s *BandwidthRuleService) applyRule(rule *BandwidthRule) {
	// Parse service IDs
	var serviceIDs []uint
	if len(rule.ServiceIDs) > 0 {
		if err := json.Unmarshal(rule.ServiceIDs, &serviceIDs); err != nil {
			log.Printf("BandwidthRule: Failed to parse service IDs for rule %d: %v", rule.ID, err)
			return
		}
	}

	if len(serviceIDs) == 0 {
		log.Printf("BandwidthRule: Rule %d has no service IDs, skipping", rule.ID)
		return
	}

	// Get all online subscribers using these services
	var subscribers []models.Subscriber
	if err := database.DB.Preload("Nas").Preload("Service").
		Where("is_online = ? AND service_id IN ?", true, serviceIDs).
		Find(&subscribers).Error; err != nil {
		log.Printf("BandwidthRule: Failed to get subscribers for rule %d: %v", rule.ID, err)
		return
	}

	log.Printf("BandwidthRule: Applying rule '%s' to %d online subscribers", rule.Name, len(subscribers))

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
		s.applyRuleToNasSubscribers(rule, nas, subs)
	}
}

// getActiveSubscriberBandwidthRuleInternet returns the active internet bandwidth rule for a subscriber
func getActiveSubscriberBandwidthRuleInternet(subscriberID uint) *models.SubscriberBandwidthRule {
	var rules []models.SubscriberBandwidthRule
	database.DB.Where("subscriber_id = ? AND rule_type = ? AND enabled = ?", subscriberID, models.BandwidthRuleTypeInternet, true).
		Order("priority DESC").Find(&rules)

	for _, rule := range rules {
		if rule.IsActiveNow() {
			return &rule
		}
	}
	return nil
}

// applyRuleToNasSubscribers applies a bandwidth rule to subscribers on a specific NAS
func (s *BandwidthRuleService) applyRuleToNasSubscribers(rule *BandwidthRule, nas *models.Nas, subscribers []models.Subscriber) {
	client := mikrotik.NewClient(
		fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
		nas.APIUsername,
		nas.APIPassword,
	)
	defer client.Close()

	coaClient := radius.NewCOAClient(nas.IPAddress, nas.CoAPort, nas.Secret)

	for _, sub := range subscribers {
		if sub.Service.ID == 0 {
			continue
		}

		// Skip users blocked by WAN Management Check — don't override their 1k/1k
		if sub.WanCheckStatus == "failed" {
			continue
		}

		// Skip if user already has a higher priority rule applied
		s.mu.RLock()
		if applied, exists := s.appliedToUsers[sub.Username]; exists {
			// Check if existing rule has higher priority (lower number = higher priority)
			var existingRule BandwidthRule
			if database.DB.First(&existingRule, applied.RuleID).Error == nil {
				if existingRule.Priority < rule.Priority {
					s.mu.RUnlock()
					continue
				}
			}
		}
		s.mu.RUnlock()

		// Calculate adjusted speed based on multiplier
		// Priority: 1. Subscriber Bandwidth Rule, 2. FUP speed, 3. Service normal speed
		var baseDownload, baseUpload int64
		var usingSubscriberRule bool

		// Check if subscriber has a custom bandwidth rule
		subscriberRule := getActiveSubscriberBandwidthRuleInternet(sub.ID)
		if subscriberRule != nil {
			// Use subscriber's custom rule as base speed
			baseDownload = int64(subscriberRule.DownloadSpeed)
			baseUpload = int64(subscriberRule.UploadSpeed)
			usingSubscriberRule = true
			log.Printf("BandwidthRule: Using subscriber rule as base for %s: %dk/%dk", sub.Username, baseUpload, baseDownload)
		} else if sub.FUPLevel > 0 {
			// Use FUP speed as base
			switch sub.FUPLevel {
			case 1:
				baseDownload = sub.Service.FUP1DownloadSpeed // Already in Kbps
				baseUpload = sub.Service.FUP1UploadSpeed
			case 2:
				baseDownload = sub.Service.FUP2DownloadSpeed
				baseUpload = sub.Service.FUP2UploadSpeed
			case 3:
				baseDownload = sub.Service.FUP3DownloadSpeed
				baseUpload = sub.Service.FUP3UploadSpeed
			case 4:
				baseDownload = sub.Service.FUP4DownloadSpeed
				baseUpload = sub.Service.FUP4UploadSpeed
			case 5:
				baseDownload = sub.Service.FUP5DownloadSpeed
				baseUpload = sub.Service.FUP5UploadSpeed
			case 6:
				baseDownload = sub.Service.FUP6DownloadSpeed
				baseUpload = sub.Service.FUP6UploadSpeed
			}
		} else {
			// Normal speed - already in Kbps
			baseDownload = int64(sub.Service.DownloadSpeed)
			baseUpload = int64(sub.Service.UploadSpeed)
		}
		_ = usingSubscriberRule // Used in logging above

		// Apply multiplier percentage (100% = same speed, 200% = double speed)
		// Formula: base * multiplier / 100
		newDownloadK := baseDownload * int64(rule.DownloadMultiplier) / 100
		newUploadK := baseUpload * int64(rule.UploadMultiplier) / 100

		// Format rate limit (upload/download for MikroTik rx/tx)
		rateLimit := fmt.Sprintf("%dk/%dk", newUploadK, newDownloadK)

		// Get session info
		session, err := client.GetActiveSession(sub.Username)
		if err != nil {
			log.Printf("BandwidthRule: Failed to get session for %s: %v", sub.Username, err)
			continue
		}

		// Apply via CoA
		if err := coaClient.UpdateRateLimitViaRadclient(sub.Username, session.SessionID, rateLimit); err != nil {
			log.Printf("BandwidthRule: CoA failed for %s: %v, trying MikroTik API", sub.Username, err)
			// Try MikroTik API as fallback
			if err := client.UpdateUserRateLimitWithIP(sub.Username, session.Address, int(newDownloadK), int(newUploadK)); err != nil {
				log.Printf("BandwidthRule: MikroTik API also failed for %s: %v", sub.Username, err)
				continue
			}
		}

		// Track applied rule
		s.mu.Lock()
		s.appliedToUsers[sub.Username] = &AppliedBandwidthRule{
			RuleID:            rule.ID,
			AppliedDownloadK:  newDownloadK,
			AppliedUploadK:    newUploadK,
			OriginalDownloadM: int64(sub.Service.DownloadSpeed),
			OriginalUploadM:   int64(sub.Service.UploadSpeed),
			FUPLevel:          sub.FUPLevel,
		}
		s.mu.Unlock()

		log.Printf("BandwidthRule: Applied %s to %s (rule: %s, %d%% -> %dk/%dk)",
			rateLimit, sub.Username, rule.Name, rule.DownloadMultiplier, newDownloadK, newUploadK)
	}
}

// restoreSpeedsForRule restores original speeds for subscribers affected by a rule
func (s *BandwidthRuleService) restoreSpeedsForRule(ruleID uint) {
	// Find all users with this rule applied
	s.mu.Lock()
	usersToRestore := make(map[string]*AppliedBandwidthRule)
	for username, applied := range s.appliedToUsers {
		if applied.RuleID == ruleID {
			usersToRestore[username] = applied
			delete(s.appliedToUsers, username)
		}
	}
	s.mu.Unlock()

	if len(usersToRestore) == 0 {
		return
	}

	log.Printf("BandwidthRule: Restoring speeds for %d users (rule %d ended)", len(usersToRestore), ruleID)

	// Get subscriber details for restoration
	for username, applied := range usersToRestore {
		var subscriber models.Subscriber
		if err := database.DB.Preload("Nas").Preload("Service").
			Where("username = ?", username).First(&subscriber).Error; err != nil {
			log.Printf("BandwidthRule: Failed to get subscriber %s: %v", username, err)
			continue
		}

		if subscriber.Nas == nil || !subscriber.IsOnline {
			continue
		}

		// Determine restore speed (considering subscriber bandwidth rule + FUP level)
		var restoreDownloadK, restoreUploadK int64

		// Check if subscriber has an active per-subscriber bandwidth rule
		subRule := getActiveSubscriberBandwidthRuleInternet(subscriber.ID)
		if subRule != nil {
			// Restore to subscriber's custom speed (not service speed)
			restoreDownloadK = int64(subRule.DownloadSpeed)
			restoreUploadK = int64(subRule.UploadSpeed)
			log.Printf("BandwidthRule: Restoring %s to subscriber rule speed %dk/%dk (rule_id=%d)", username, restoreUploadK, restoreDownloadK, subRule.ID)
		} else if subscriber.FUPLevel > 0 {
			// Restore to FUP speed
			switch subscriber.FUPLevel {
			case 1:
				restoreDownloadK = subscriber.Service.FUP1DownloadSpeed
				restoreUploadK = subscriber.Service.FUP1UploadSpeed
			case 2:
				restoreDownloadK = subscriber.Service.FUP2DownloadSpeed
				restoreUploadK = subscriber.Service.FUP2UploadSpeed
			case 3:
				restoreDownloadK = subscriber.Service.FUP3DownloadSpeed
				restoreUploadK = subscriber.Service.FUP3UploadSpeed
			case 4:
				restoreDownloadK = subscriber.Service.FUP4DownloadSpeed
				restoreUploadK = subscriber.Service.FUP4UploadSpeed
			case 5:
				restoreDownloadK = subscriber.Service.FUP5DownloadSpeed
				restoreUploadK = subscriber.Service.FUP5UploadSpeed
			case 6:
				restoreDownloadK = subscriber.Service.FUP6DownloadSpeed
				restoreUploadK = subscriber.Service.FUP6UploadSpeed
			}
		} else {
			// Restore to normal service speed (already in Kbps)
			restoreDownloadK = applied.OriginalDownloadM
			restoreUploadK = applied.OriginalUploadM
		}

		rateLimit := fmt.Sprintf("%dk/%dk", restoreUploadK, restoreDownloadK)

		// Get MikroTik client
		client := mikrotik.NewClient(
			fmt.Sprintf("%s:%d", subscriber.Nas.IPAddress, subscriber.Nas.APIPort),
			subscriber.Nas.APIUsername,
			subscriber.Nas.APIPassword,
		)

		session, err := client.GetActiveSession(subscriber.Username)
		if err != nil {
			client.Close()
			log.Printf("BandwidthRule: Failed to get session for restore %s: %v", username, err)
			continue
		}

		// Restore via CoA
		coaClient := radius.NewCOAClient(subscriber.Nas.IPAddress, subscriber.Nas.CoAPort, subscriber.Nas.Secret)
		if err := coaClient.UpdateRateLimitViaRadclient(subscriber.Username, session.SessionID, rateLimit); err != nil {
			log.Printf("BandwidthRule: CoA restore failed for %s: %v, trying MikroTik API", username, err)
			if err := client.UpdateUserRateLimitWithIP(subscriber.Username, session.Address, int(restoreDownloadK), int(restoreUploadK)); err != nil {
				log.Printf("BandwidthRule: MikroTik API restore also failed for %s: %v", username, err)
			}
		}

		client.Close()
		log.Printf("BandwidthRule: Restored %s speed to %s (FUP%d)", username, rateLimit, subscriber.FUPLevel)
	}
}

// ApplyRuleNow manually applies a bandwidth rule immediately (for "Apply Now" button)
func (s *BandwidthRuleService) ApplyRuleNow(ruleID uint) (int, error) {
	var rule BandwidthRule
	if err := database.DB.First(&rule, ruleID).Error; err != nil {
		return 0, err
	}

	// Parse service IDs
	var serviceIDs []uint
	if len(rule.ServiceIDs) > 0 {
		if err := json.Unmarshal(rule.ServiceIDs, &serviceIDs); err != nil {
			return 0, fmt.Errorf("failed to parse service IDs: %v", err)
		}
	}

	if len(serviceIDs) == 0 {
		return 0, fmt.Errorf("rule has no service IDs")
	}

	// Get all online subscribers using these services
	var subscribers []models.Subscriber
	if err := database.DB.Preload("Nas").Preload("Service").
		Where("is_online = ? AND service_id IN ?", true, serviceIDs).
		Find(&subscribers).Error; err != nil {
		return 0, fmt.Errorf("failed to get subscribers: %v", err)
	}

	log.Printf("BandwidthRule: Manually applying rule '%s' to %d online subscribers", rule.Name, len(subscribers))

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
		applied := s.applyRuleToNasSubscribersCount(&rule, nas, subs)
		appliedCount += applied
	}

	return appliedCount, nil
}

// applyRuleToNasSubscribersCount applies a rule and returns count of successful applications
func (s *BandwidthRuleService) applyRuleToNasSubscribersCount(rule *BandwidthRule, nas *models.Nas, subscribers []models.Subscriber) int {
	client := mikrotik.NewClient(
		fmt.Sprintf("%s:%d", nas.IPAddress, nas.APIPort),
		nas.APIUsername,
		nas.APIPassword,
	)
	defer client.Close()

	coaClient := radius.NewCOAClient(nas.IPAddress, nas.CoAPort, nas.Secret)
	appliedCount := 0

	for _, sub := range subscribers {
		if sub.Service.ID == 0 {
			continue
		}

		// Skip users blocked by WAN Management Check — don't override their 1k/1k
		if sub.WanCheckStatus == "failed" {
			continue
		}

		// Calculate adjusted speed based on multiplier
		// Priority: 1. Subscriber Bandwidth Rule, 2. FUP speed, 3. Service normal speed
		var baseDownload, baseUpload int64

		// Check if subscriber has a custom bandwidth rule
		subscriberRule := getActiveSubscriberBandwidthRuleInternet(sub.ID)
		if subscriberRule != nil {
			// Use subscriber's custom rule as base speed
			baseDownload = int64(subscriberRule.DownloadSpeed)
			baseUpload = int64(subscriberRule.UploadSpeed)
			log.Printf("BandwidthRule: Using subscriber rule as base for %s: %dk/%dk", sub.Username, baseUpload, baseDownload)
		} else if sub.FUPLevel > 0 {
			switch sub.FUPLevel {
			case 1:
				baseDownload = sub.Service.FUP1DownloadSpeed
				baseUpload = sub.Service.FUP1UploadSpeed
			case 2:
				baseDownload = sub.Service.FUP2DownloadSpeed
				baseUpload = sub.Service.FUP2UploadSpeed
			case 3:
				baseDownload = sub.Service.FUP3DownloadSpeed
				baseUpload = sub.Service.FUP3UploadSpeed
			case 4:
				baseDownload = sub.Service.FUP4DownloadSpeed
				baseUpload = sub.Service.FUP4UploadSpeed
			case 5:
				baseDownload = sub.Service.FUP5DownloadSpeed
				baseUpload = sub.Service.FUP5UploadSpeed
			case 6:
				baseDownload = sub.Service.FUP6DownloadSpeed
				baseUpload = sub.Service.FUP6UploadSpeed
			}
		} else {
			// Normal speed - already in Kbps
			baseDownload = int64(sub.Service.DownloadSpeed)
			baseUpload = int64(sub.Service.UploadSpeed)
		}

		// Apply multiplier percentage (100% = same speed, 200% = double speed)
		newDownloadK := baseDownload * int64(rule.DownloadMultiplier) / 100
		newUploadK := baseUpload * int64(rule.UploadMultiplier) / 100
		rateLimit := fmt.Sprintf("%dk/%dk", newUploadK, newDownloadK)

		session, err := client.GetActiveSession(sub.Username)
		if err != nil {
			log.Printf("BandwidthRule: Failed to get session for %s: %v", sub.Username, err)
			continue
		}

		if err := coaClient.UpdateRateLimitViaRadclient(sub.Username, session.SessionID, rateLimit); err != nil {
			log.Printf("BandwidthRule: CoA failed for %s: %v, trying MikroTik API", sub.Username, err)
			if err := client.UpdateUserRateLimitWithIP(sub.Username, session.Address, int(newDownloadK), int(newUploadK)); err != nil {
				log.Printf("BandwidthRule: MikroTik API also failed for %s: %v", sub.Username, err)
				continue
			}
		}

		appliedCount++
		log.Printf("BandwidthRule: Applied %s to %s (rule: %s)", rateLimit, sub.Username, rule.Name)
	}

	return appliedCount
}

// ApplyNewSubscriber checks and applies active bandwidth rules to a newly connected subscriber
func (s *BandwidthRuleService) ApplyNewSubscriber(subscriber *models.Subscriber) {
	if subscriber == nil || subscriber.Nas == nil || subscriber.ServiceID == 0 {
		return
	}

	s.mu.RLock()
	activeRuleIDs := make([]uint, 0, len(s.activeRules))
	for ruleID := range s.activeRules {
		activeRuleIDs = append(activeRuleIDs, ruleID)
	}
	s.mu.RUnlock()

	if len(activeRuleIDs) == 0 {
		return
	}

	// Check if any active rule applies to this subscriber's service
	for _, ruleID := range activeRuleIDs {
		var rule BandwidthRule
		if err := database.DB.First(&rule, ruleID).Error; err != nil {
			continue
		}

		var serviceIDs []uint
		if err := json.Unmarshal(rule.ServiceIDs, &serviceIDs); err != nil {
			continue
		}

		for _, svcID := range serviceIDs {
			if svcID == subscriber.ServiceID {
				// This rule applies to this subscriber
				go s.applyRuleToNasSubscribers(&rule, subscriber.Nas, []models.Subscriber{*subscriber})
				return // Apply first matching rule
			}
		}
	}
}
