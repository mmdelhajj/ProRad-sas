package services

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

// DailyNotificationService fires communication rules for expiry_warning and expired events.
// Runs once per day at the configured notification send time (default 08:00).
type DailyNotificationService struct {
	stopChan  chan struct{}
	wg        sync.WaitGroup
	lastRunAt time.Time
}

// NewDailyNotificationService creates a new daily notification service
func NewDailyNotificationService() *DailyNotificationService {
	return &DailyNotificationService{
		stopChan: make(chan struct{}),
	}
}

// Start begins the daily notification scheduler
func (s *DailyNotificationService) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		log.Println("DailyNotificationService started")

		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.checkAndRun()
			case <-s.stopChan:
				log.Println("DailyNotificationService stopped")
				return
			}
		}
	}()
}

// Stop stops the daily notification service
func (s *DailyNotificationService) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

// getNotificationSendTime reads the configured notification send time from settings.
// Defaults to 08:00.
func getNotificationSendTime() (int, int) {
	var pref models.SystemPreference
	if err := database.DB.Where("key = ?", "notification_send_time").First(&pref).Error; err != nil {
		return 8, 0
	}

	parts := strings.Split(pref.Value, ":")
	if len(parts) != 2 {
		return 8, 0
	}

	var hour, minute int
	if _, err := fmt.Sscanf(parts[0], "%d", &hour); err != nil {
		return 8, 0
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &minute); err != nil {
		return 8, 0
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 8, 0
	}

	return hour, minute
}

// checkAndRun checks if it's the configured time and fires the notification rules once per day
func (s *DailyNotificationService) checkAndRun() {
	now := getNow()
	sendHour, sendMinute := getNotificationSendTime()

	if now.Hour() != sendHour || now.Minute() != sendMinute {
		return
	}

	// Prevent double-firing within the same minute
	todayRun := time.Date(now.Year(), now.Month(), now.Day(), sendHour, sendMinute, 0, 0, now.Location())
	if !s.lastRunAt.IsZero() && s.lastRunAt.After(todayRun.Add(-1*time.Minute)) {
		return
	}
	s.lastRunAt = now

	log.Printf("DailyNotificationService: Running at %02d:%02d", sendHour, sendMinute)
	s.fireExpiryWarningRules(now)
	s.fireExpiredRules(now)
}

// fireExpiryWarningRules sends notifications for subscribers expiring in exactly DaysBefore days
func (s *DailyNotificationService) fireExpiryWarningRules(now time.Time) {
	var rules []models.CommunicationRule
	if err := database.DB.Where("trigger_event = ? AND enabled = ?", "expiry_warning", true).Find(&rules).Error; err != nil {
		log.Printf("DailyNotif: Failed to query expiry_warning rules: %v", err)
		return
	}
	if len(rules) == 0 {
		return
	}

	for _, rule := range rules {
		if rule.Template == "" {
			continue
		}

		daysAhead := rule.DaysBefore
		if daysAhead <= 0 {
			daysAhead = 3
		}

		// Find subscribers expiring exactly daysAhead days from today
		targetDate := now.AddDate(0, 0, daysAhead)
		startOfDay := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, now.Location())
		endOfDay := startOfDay.Add(24 * time.Hour)

		var subscribers []models.Subscriber
		if err := database.DB.Preload("Service").Preload("Reseller.User").
			Where("expiry_date >= ? AND expiry_date < ? AND status = ?",
				startOfDay, endOfDay, models.SubscriberStatusActive).
			Find(&subscribers).Error; err != nil {
			log.Printf("DailyNotif[%s]: Query failed: %v", rule.Name, err)
			continue
		}

		if len(subscribers) == 0 {
			log.Printf("DailyNotif[%s]: No subscribers expiring in %d days", rule.Name, daysAhead)
			continue
		}

		log.Printf("DailyNotif[%s]: Sending expiry_warning to %d subscribers (expires in %d days)", rule.Name, len(subscribers), daysAhead)

		for _, sub := range subscribers {
			if dailyNotifAlreadySent(sub.ID, rule.ID, now) {
				log.Printf("DailyNotif[%s]: Skipping %s — already sent today", rule.Name, sub.Username)
				continue
			}
			dailySendNotification(rule, sub, daysAhead)
		}
	}
}

// fireExpiredRules sends notifications for subscribers whose account expired today
func (s *DailyNotificationService) fireExpiredRules(now time.Time) {
	var rules []models.CommunicationRule
	if err := database.DB.Where("trigger_event = ? AND enabled = ?", "expired", true).Find(&rules).Error; err != nil {
		log.Printf("DailyNotif: Failed to query expired rules: %v", err)
		return
	}
	if len(rules) == 0 {
		return
	}

	// Subscribers whose expiry_date falls in today
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	var subscribers []models.Subscriber
	if err := database.DB.Preload("Service").Preload("Reseller.User").
		Where("expiry_date >= ? AND expiry_date < ? AND deleted_at IS NULL",
			startOfDay, endOfDay).
		Find(&subscribers).Error; err != nil {
		log.Printf("DailyNotif: Failed to query expired subscribers: %v", err)
		return
	}

	if len(subscribers) == 0 {
		return
	}

	for _, rule := range rules {
		if rule.Template == "" {
			continue
		}

		log.Printf("DailyNotif[%s]: Sending 'expired' to %d subscribers", rule.Name, len(subscribers))

		for _, sub := range subscribers {
			if dailyNotifAlreadySent(sub.ID, rule.ID, now) {
				log.Printf("DailyNotif[%s]: Skipping %s — already sent today", rule.Name, sub.Username)
				continue
			}
			dailySendNotification(rule, sub, 0)
		}
	}
}

// dailyNotifAlreadySent returns true if we already sent a notification for this subscriber+rule today
func dailyNotifAlreadySent(subscriberID, ruleID uint, now time.Time) bool {
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var count int64
	database.DB.Model(&models.CommunicationLog{}).
		Where("subscriber_id = ? AND rule_id = ? AND created_at >= ?", subscriberID, ruleID, startOfDay).
		Count(&count)
	return count > 0
}

// dailySendNotification sends the notification and logs it
func dailySendNotification(rule models.CommunicationRule, sub models.Subscriber, daysRemaining int) {
	expiryDate := ""
	if !sub.ExpiryDate.IsZero() {
		expiryDate = sub.ExpiryDate.Format("2006-01-02")
	}
	serviceName := ""
	if sub.Service != nil {
		serviceName = sub.Service.Name
	}
	balance := fmt.Sprintf("%.2f", sub.Price)

	msg := rule.Template
	msg = strings.ReplaceAll(msg, "{username}", sub.Username)
	msg = strings.ReplaceAll(msg, "{full_name}", sub.FullName)
	msg = strings.ReplaceAll(msg, "{expiry_date}", expiryDate)
	msg = strings.ReplaceAll(msg, "{service_name}", serviceName)
	msg = strings.ReplaceAll(msg, "{balance}", balance)
	msg = strings.ReplaceAll(msg, "{days_before}", fmt.Sprintf("%d", daysRemaining))

	sent := false
	errMsg := ""

	switch rule.Channel {
	case "whatsapp":
		phone := getNotifPhone(rule, sub)
		if phone != "" {
			wa := NewWhatsAppService()
			if err := wa.SendMessageForSubscriber(sub, phone, msg); err != nil {
				log.Printf("DailyNotif[%s]: WhatsApp failed for %s: %v", rule.Name, sub.Username, err)
				errMsg = err.Error()
			} else {
				log.Printf("DailyNotif[%s]: WhatsApp sent to %s", rule.Name, sub.Username)
				sent = true
			}
		} else {
			log.Printf("DailyNotif[%s]: No phone for %s, skipping", rule.Name, sub.Username)
		}

	case "sms":
		phone := getNotifPhone(rule, sub)
		if phone != "" {
			sms := NewSMSService()
			if err := sms.SendSMS(phone, msg); err != nil {
				log.Printf("DailyNotif[%s]: SMS failed for %s: %v", rule.Name, sub.Username, err)
				errMsg = err.Error()
			} else {
				log.Printf("DailyNotif[%s]: SMS sent to %s", rule.Name, sub.Username)
				sent = true
			}
		} else {
			log.Printf("DailyNotif[%s]: No phone for %s, skipping", rule.Name, sub.Username)
		}

	case "email":
		email := getNotifEmail(rule, sub)
		if email != "" {
			emailSvc := NewEmailService()
			subject := fmt.Sprintf("Expiry Reminder - %s", sub.Username)
			if rule.TriggerEvent == "expired" {
				subject = fmt.Sprintf("Account Expired - %s", sub.Username)
			}
			if err := emailSvc.SendEmail(email, subject, msg, false); err != nil {
				log.Printf("DailyNotif[%s]: Email failed for %s: %v", rule.Name, sub.Username, err)
				errMsg = err.Error()
			} else {
				log.Printf("DailyNotif[%s]: Email sent to %s", rule.Name, sub.Username)
				sent = true
			}
		} else {
			log.Printf("DailyNotif[%s]: No email for %s, skipping", rule.Name, sub.Username)
		}
	}

	// Log the attempt (sent or failed) for dedup tracking
	logDailyNotif(rule, sub, msg, sent, errMsg)
}

// getNotifPhone returns the phone to notify (subscriber or reseller depending on rule)
func getNotifPhone(rule models.CommunicationRule, sub models.Subscriber) string {
	if rule.SendToReseller && sub.Reseller != nil && sub.Reseller.User != nil && sub.Reseller.User.Phone != "" {
		return sub.Reseller.User.Phone
	}
	return sub.Phone
}

// getNotifEmail returns the email to notify (subscriber or reseller depending on rule)
func getNotifEmail(rule models.CommunicationRule, sub models.Subscriber) string {
	if rule.SendToReseller && sub.Reseller != nil && sub.Reseller.User != nil && sub.Reseller.User.Email != "" {
		return sub.Reseller.User.Email
	}
	return sub.Email
}

// logDailyNotif records a CommunicationLog entry for dedup and audit
func logDailyNotif(rule models.CommunicationRule, sub models.Subscriber, message string, sent bool, errMsg string) {
	now := time.Now()
	subID := sub.ID
	ruleID := rule.ID

	status := "sent"
	if !sent {
		status = "failed"
	}

	recipient := sub.Phone
	if rule.Channel == "email" {
		recipient = sub.Email
	}

	commLog := models.CommunicationLog{
		Type:         rule.Channel,
		Recipient:    recipient,
		Subject:      fmt.Sprintf("%s - %s", rule.TriggerEvent, sub.Username),
		Message:      message,
		Status:       status,
		ErrorMessage: errMsg,
		SubscriberID: &subID,
		RuleID:       &ruleID,
		CreatedAt:    now,
		SentAt:       &now,
	}
	database.DB.Create(&commLog)
}
