package services

import (
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

// OverdueSuspendService handles automatic suspension of overdue subscribers
// and automatic archival of long-expired subscribers.
// Runs daily at 4:00 AM.
type OverdueSuspendService struct {
	stopChan  chan struct{}
	wg        sync.WaitGroup
	lastRunAt time.Time
}

func NewOverdueSuspendService() *OverdueSuspendService {
	return &OverdueSuspendService{
		stopChan: make(chan struct{}),
	}
}

func (s *OverdueSuspendService) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		log.Println("OverdueSuspendService started")

		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.checkAndRun()
			case <-s.stopChan:
				log.Println("OverdueSuspendService stopped")
				return
			}
		}
	}()
}

func (s *OverdueSuspendService) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

func (s *OverdueSuspendService) checkAndRun() {
	now := time.Now()

	// Run at 4:00 AM daily
	if now.Hour() != 4 || now.Minute() != 0 {
		return
	}

	// Prevent running more than once per day
	if now.Format("2006-01-02") == s.lastRunAt.Format("2006-01-02") {
		return
	}

	s.lastRunAt = now
	s.run()
}

func (s *OverdueSuspendService) run() {
	log.Println("OverdueSuspendService: Running daily overdue check...")

	// Feature 1: Overdue Auto-Suspend
	s.processOverdueSuspend()

	// Feature 2: Auto-Archive Expired
	s.processAutoArchive()
}

func (s *OverdueSuspendService) processOverdueSuspend() {
	enabled := getPreference("overdue_suspend_enabled")
	if enabled != "true" {
		return
	}

	daysStr := getPreference("overdue_suspend_days")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		days = 7
	}

	cutoff := time.Now().AddDate(0, 0, -days)

	// Find active subscribers whose expiry is past the cutoff
	var subscribers []models.Subscriber
	if err := database.DB.
		Where("status = ? AND expiry_date < ? AND deleted_at IS NULL", models.SubscriberStatusActive, cutoff).
		Find(&subscribers).Error; err != nil {
		log.Printf("OverdueSuspendService: Error querying overdue subscribers: %v", err)
		return
	}

	if len(subscribers) == 0 {
		return
	}

	count := 0
	for _, sub := range subscribers {
		note := fmt.Sprintf("Auto-suspended: %d days overdue (expired %s)", days, sub.ExpiryDate.Format("2006-01-02"))
		newNote := sub.Note
		if newNote != "" {
			newNote += "\n"
		}
		newNote += note

		if err := database.DB.Model(&models.Subscriber{}).Where("id = ?", sub.ID).Updates(map[string]interface{}{
			"status": models.SubscriberStatusStopped,
			"note":   newNote,
		}).Error; err != nil {
			log.Printf("OverdueSuspendService: Error suspending subscriber %d: %v", sub.ID, err)
			continue
		}
		count++
	}

	if count > 0 {
		log.Printf("OverdueSuspendService: Auto-suspended %d overdue subscribers (%d+ days past expiry)", count, days)
	}
}

func (s *OverdueSuspendService) processAutoArchive() {
	enabled := getPreference("auto_archive_enabled")
	if enabled != "true" {
		return
	}

	daysStr := getPreference("auto_archive_days")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		days = 30
	}

	cutoff := time.Now().AddDate(0, 0, -days)

	// Find suspended/expired subscribers past the archive cutoff
	var subscribers []models.Subscriber
	if err := database.DB.
		Where("status IN (?, ?) AND expiry_date < ? AND deleted_at IS NULL",
			models.SubscriberStatusStopped, models.SubscriberStatusExpired, cutoff).
		Find(&subscribers).Error; err != nil {
		log.Printf("OverdueSuspendService: Error querying archive candidates: %v", err)
		return
	}

	if len(subscribers) == 0 {
		return
	}

	count := 0
	now := time.Now()
	for _, sub := range subscribers {
		// Soft delete
		if err := database.DB.Model(&models.Subscriber{}).Where("id = ?", sub.ID).Updates(map[string]interface{}{
			"deleted_at":      now,
			"deleted_by_name": "System (Auto-Archive)",
			"note": func() string {
				n := sub.Note
				if n != "" {
					n += "\n"
				}
				return n + fmt.Sprintf("Auto-archived: %d days past expiry", days)
			}(),
		}).Error; err != nil {
			log.Printf("OverdueSuspendService: Error archiving subscriber %d: %v", sub.ID, err)
			continue
		}
		count++
	}

	if count > 0 {
		log.Printf("OverdueSuspendService: Auto-archived %d subscribers (%d+ days past expiry)", count, days)
	}
}

func getPreference(key string) string {
	var pref models.SystemPreference
	if err := database.DB.Where("key = ?", key).First(&pref).Error; err != nil {
		return ""
	}
	return pref.Value
}
