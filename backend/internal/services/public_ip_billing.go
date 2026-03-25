package services

import (
	"fmt"
	"log"
	"time"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

// PublicIPBillingService handles monthly billing for public IP assignments
type PublicIPBillingService struct {
	stopChan chan struct{}
}

func NewPublicIPBillingService() *PublicIPBillingService {
	return &PublicIPBillingService{
		stopChan: make(chan struct{}),
	}
}

func (s *PublicIPBillingService) Start() {
	go s.run()
	log.Println("PublicIPBillingService: Started (runs daily at 6:00 AM)")
}

func (s *PublicIPBillingService) Stop() {
	close(s.stopChan)
}

func (s *PublicIPBillingService) run() {
	// Wait for initial delay then check every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// Run once on startup if past 6 AM
	s.checkAndBill()

	for {
		select {
		case <-ticker.C:
			s.checkAndBill()
		case <-s.stopChan:
			log.Println("PublicIPBillingService: Stopped")
			return
		}
	}
}

func (s *PublicIPBillingService) checkAndBill() {
	// Get configured billing time (default 06:00)
	billingTimeStr := "06:00"
	var pref models.SystemPreference
	if err := database.DB.Where("key = ?", "public_ip_billing_time").First(&pref).Error; err == nil {
		billingTimeStr = pref.Value
	}

	// Get timezone
	loc := getSystemTimezone()
	now := time.Now().In(loc)

	// Parse billing time
	hour, min := 6, 0
	fmt.Sscanf(billingTimeStr, "%d:%d", &hour, &min)

	// Only run within the billing hour
	if now.Hour() != hour || now.Minute() > min+59 {
		return
	}

	// Prevent running more than once per day using a state key
	todayStr := now.Format("2006-01-02")
	var lastRun models.SystemPreference
	if err := database.DB.Where("key = ?", "public_ip_billing_last_run").First(&lastRun).Error; err == nil {
		if lastRun.Value == todayStr {
			return // Already ran today
		}
	}

	log.Println("PublicIPBillingService: Starting daily billing run")

	s.processBilling(now)

	// Mark as run for today
	database.DB.Exec(
		"INSERT INTO system_preferences (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = ?",
		"public_ip_billing_last_run", todayStr, todayStr,
	)
}

func (s *PublicIPBillingService) processBilling(now time.Time) {
	// Find all active assignments where next_billing_at <= now
	var assignments []models.PublicIPAssignment
	if err := database.DB.
		Where("status = ? AND next_billing_at IS NOT NULL AND next_billing_at <= ? AND deleted_at IS NULL",
			models.PublicIPStatusActive, now).
		Find(&assignments).Error; err != nil {
		log.Printf("PublicIPBillingService: Error querying assignments: %v", err)
		return
	}

	if len(assignments) == 0 {
		return
	}

	log.Printf("PublicIPBillingService: Processing %d assignments due for billing", len(assignments))

	billed := 0
	suspended := 0

	for _, assignment := range assignments {
		if assignment.MonthlyPrice <= 0 {
			// Free IP — just advance billing date
			nextBilling := now.Add(30 * 24 * time.Hour)
			database.DB.Model(&assignment).Updates(map[string]interface{}{
				"last_billed_at":  now,
				"next_billing_at": nextBilling,
			})
			continue
		}

		// Get subscriber to find reseller
		var subscriber models.Subscriber
		if err := database.DB.First(&subscriber, assignment.SubscriberID).Error; err != nil {
			log.Printf("PublicIPBillingService: Subscriber %d not found for assignment %d", assignment.SubscriberID, assignment.ID)
			continue
		}

		// Get reseller
		var reseller models.Reseller
		if err := database.DB.First(&reseller, subscriber.ResellerID).Error; err != nil {
			log.Printf("PublicIPBillingService: Reseller %d not found for subscriber %d", subscriber.ResellerID, subscriber.ID)
			continue
		}

		// Check balance
		if reseller.Balance < assignment.MonthlyPrice {
			// Insufficient balance — suspend the assignment
			log.Printf("PublicIPBillingService: Insufficient balance for reseller %d (%.2f < %.2f), suspending IP %s",
				reseller.ID, reseller.Balance, assignment.MonthlyPrice, assignment.IPAddress)

			database.DB.Model(&assignment).Update("status", models.PublicIPStatusSuspended)

			// Remove from radreply
			attribute := "Framed-IP-Address"
			if assignment.IPVersion == 6 {
				attribute = "Framed-IPv6-Prefix"
			}
			database.DB.Exec("DELETE FROM radreply WHERE username = ? AND attribute = ?",
				subscriber.Username, attribute)

			suspended++
			continue
		}

		// Deduct from reseller balance
		database.DB.Model(&reseller).Update("balance", database.DB.Raw("balance - ?", assignment.MonthlyPrice))

		// Create transaction
		database.DB.Create(&models.Transaction{
			Type:          models.TransactionTypeAddon,
			Amount:        -assignment.MonthlyPrice,
			BalanceBefore: reseller.Balance,
			BalanceAfter:  reseller.Balance - assignment.MonthlyPrice,
			Description:   fmt.Sprintf("Public IP monthly billing: %s for %s", assignment.IPAddress, subscriber.Username),
			ResellerID:    subscriber.ResellerID,
			SubscriberID:  &subscriber.ID,
		})

		// Advance billing date
		nextBilling := now.Add(30 * 24 * time.Hour)
		database.DB.Model(&assignment).Updates(map[string]interface{}{
			"last_billed_at":  now,
			"next_billing_at": nextBilling,
		})

		billed++
	}

	log.Printf("PublicIPBillingService: Billing complete — %d billed, %d suspended", billed, suspended)
}

// getSystemTimezone returns the configured system timezone
func getSystemTimezone() *time.Location {
	var pref models.SystemPreference
	if err := database.DB.Where("key = ?", "system_timezone").First(&pref).Error; err == nil {
		if loc, err := time.LoadLocation(pref.Value); err == nil {
			return loc
		}
	}
	return time.UTC
}
