package services

import (
	"encoding/json"
	"log"
	"math"
	"sync"
	"time"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

// ChurnPredictionService computes churn risk scores for subscribers
// and predicts FUP dates based on usage velocity.
// Runs daily at 5:00 AM.
type ChurnPredictionService struct {
	stopChan  chan struct{}
	wg        sync.WaitGroup
	lastRunAt time.Time
}

func NewChurnPredictionService() *ChurnPredictionService {
	return &ChurnPredictionService{
		stopChan: make(chan struct{}),
	}
}

func (s *ChurnPredictionService) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		log.Println("ChurnPredictionService started")

		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.checkAndRun()
			case <-s.stopChan:
				log.Println("ChurnPredictionService stopped")
				return
			}
		}
	}()
}

func (s *ChurnPredictionService) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

func (s *ChurnPredictionService) checkAndRun() {
	now := time.Now()

	if now.Hour() != 5 || now.Minute() != 0 {
		return
	}

	if now.Format("2006-01-02") == s.lastRunAt.Format("2006-01-02") {
		return
	}

	s.lastRunAt = now
	s.run()
}

func (s *ChurnPredictionService) run() {
	log.Println("ChurnPredictionService: Computing churn scores...")

	var subscribers []models.Subscriber
	if err := database.DB.
		Where("status = ? AND deleted_at IS NULL", models.SubscriberStatusActive).
		Preload("Service").
		Find(&subscribers).Error; err != nil {
		log.Printf("ChurnPredictionService: Error loading subscribers: %v", err)
		return
	}

	now := time.Now()
	updated := 0

	for _, sub := range subscribers {
		score, factors := s.computeChurnScore(sub, now)
		riskLevel := churnRiskLevel(score)

		factorsJSON, _ := json.Marshal(factors)

		churnScore := models.ChurnScore{
			SubscriberID:    sub.ID,
			Score:           score,
			RiskLevel:       riskLevel,
			Factors:         string(factorsJSON),
			DaysUntilExpiry: sub.DaysRemaining(),
			UsageTrend:      s.getUsageTrend(sub.ID, now),
			TicketCount:     s.getTicketCount(sub.ID, now),
			PaymentDelays:   s.getPaymentDelays(sub.ID, now),
			ComputedAt:      now,
			CreatedAt:       now,
		}

		// Upsert
		if err := database.DB.Exec(`
			INSERT INTO churn_scores (subscriber_id, score, risk_level, factors, days_until_expiry, usage_trend, ticket_count, payment_delays, computed_at, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (subscriber_id) DO UPDATE SET
				score = EXCLUDED.score,
				risk_level = EXCLUDED.risk_level,
				factors = EXCLUDED.factors,
				days_until_expiry = EXCLUDED.days_until_expiry,
				usage_trend = EXCLUDED.usage_trend,
				ticket_count = EXCLUDED.ticket_count,
				payment_delays = EXCLUDED.payment_delays,
				computed_at = EXCLUDED.computed_at`,
			churnScore.SubscriberID, churnScore.Score, churnScore.RiskLevel, churnScore.Factors,
			churnScore.DaysUntilExpiry, churnScore.UsageTrend, churnScore.TicketCount, churnScore.PaymentDelays,
			churnScore.ComputedAt, churnScore.CreatedAt).Error; err != nil {
			log.Printf("ChurnPredictionService: Error upserting churn score for subscriber %d: %v", sub.ID, err)
			continue
		}

		// Update subscriber fields
		database.DB.Model(&models.Subscriber{}).Where("id = ?", sub.ID).Updates(map[string]interface{}{
			"churn_risk":  riskLevel,
			"churn_score": score,
		})

		// Predictive FUP
		s.computePredictiveFUP(sub, now)

		updated++
	}

	log.Printf("ChurnPredictionService: Updated churn scores for %d subscribers", updated)
}

type churnFactor struct {
	Name   string `json:"name"`
	Points int    `json:"points"`
	Detail string `json:"detail"`
}

func (s *ChurnPredictionService) computeChurnScore(sub models.Subscriber, now time.Time) (int, []churnFactor) {
	score := 0
	var factors []churnFactor

	// 1. Days until expiry (25 points)
	daysLeft := sub.DaysRemaining()
	expiryPoints := 0
	if sub.IsExpired() {
		expiryPoints = 25
	} else if daysLeft < 3 {
		expiryPoints = 20
	} else if daysLeft < 7 {
		expiryPoints = 15
	} else if daysLeft < 14 {
		expiryPoints = 10
	} else if daysLeft < 30 {
		expiryPoints = 5
	}
	if expiryPoints > 0 {
		score += expiryPoints
		factors = append(factors, churnFactor{"expiry", expiryPoints, formatDaysLeft(daysLeft, sub.IsExpired())})
	}

	// 2. Usage trend (25 points)
	trend := s.getUsageTrend(sub.ID, now)
	usagePoints := 0
	switch trend {
	case "declining_heavy":
		usagePoints = 25
	case "declining":
		usagePoints = 15
	case "declining_slight":
		usagePoints = 8
	}
	if usagePoints > 0 {
		score += usagePoints
		factors = append(factors, churnFactor{"usage_trend", usagePoints, trend})
	}

	// 3. Ticket frequency (20 points)
	ticketCount := s.getTicketCount(sub.ID, now)
	ticketPoints := 0
	if ticketCount >= 3 {
		ticketPoints = 20
	} else if ticketCount == 2 {
		ticketPoints = 12
	} else if ticketCount == 1 {
		ticketPoints = 6
	}
	if ticketPoints > 0 {
		score += ticketPoints
		factors = append(factors, churnFactor{"tickets", ticketPoints, formatTickets(ticketCount)})
	}

	// 4. Payment delays (20 points)
	delays := s.getPaymentDelays(sub.ID, now)
	paymentPoints := 0
	if delays >= 3 {
		paymentPoints = 20
	} else if delays == 2 {
		paymentPoints = 12
	} else if delays == 1 {
		paymentPoints = 6
	}
	if paymentPoints > 0 {
		score += paymentPoints
		factors = append(factors, churnFactor{"payment_delays", paymentPoints, formatDelays(delays)})
	}

	// 5. FUP hits (10 points) - 0 hits = not using service = risk
	var fupHits int64
	database.DB.Model(&models.CommunicationLog{}).
		Where("subscriber_id = ? AND type = 'whatsapp' AND created_at > ? AND message LIKE '%FUP%'",
			sub.ID, now.AddDate(0, 0, -30)).
		Count(&fupHits)
	if fupHits == 0 && sub.DailyDownloadUsed == 0 && sub.MonthlyDownloadUsed == 0 {
		score += 10
		factors = append(factors, churnFactor{"no_usage", 10, "No usage detected"})
	}

	if score > 100 {
		score = 100
	}

	return score, factors
}

func (s *ChurnPredictionService) getUsageTrend(subscriberID uint, now time.Time) string {
	// Compare last 7 days avg vs previous 7 days avg
	var recentAvg, previousAvg float64

	database.DB.Raw(`
		SELECT COALESCE(AVG(acctinputoctets + acctoutputoctets), 0) FROM radacct
		WHERE username = (SELECT username FROM subscribers WHERE id = ?)
		AND acctstarttime > ? AND acctstarttime <= ?`,
		subscriberID, now.AddDate(0, 0, -7), now).Scan(&recentAvg)

	database.DB.Raw(`
		SELECT COALESCE(AVG(acctinputoctets + acctoutputoctets), 0) FROM radacct
		WHERE username = (SELECT username FROM subscribers WHERE id = ?)
		AND acctstarttime > ? AND acctstarttime <= ?`,
		subscriberID, now.AddDate(0, 0, -14), now.AddDate(0, 0, -7)).Scan(&previousAvg)

	if previousAvg == 0 {
		return "stable"
	}

	change := (recentAvg - previousAvg) / previousAvg
	if change < -0.5 {
		return "declining_heavy"
	} else if change < -0.25 {
		return "declining"
	} else if change < -0.1 {
		return "declining_slight"
	} else if change > 0.1 {
		return "growing"
	}
	return "stable"
}

func (s *ChurnPredictionService) getTicketCount(subscriberID uint, now time.Time) int {
	var count int64
	database.DB.Model(&models.Ticket{}).
		Where("subscriber_id = ? AND created_at > ?", subscriberID, now.AddDate(0, 0, -30)).
		Count(&count)
	return int(count)
}

func (s *ChurnPredictionService) getPaymentDelays(subscriberID uint, now time.Time) int {
	// Count invoices that were paid after due date in last 90 days
	var count int64
	database.DB.Model(&models.Invoice{}).
		Where("subscriber_id = ? AND created_at > ? AND paid_date IS NOT NULL AND paid_date > due_date",
			subscriberID, now.AddDate(0, 0, -90)).
		Count(&count)
	return int(count)
}

func (s *ChurnPredictionService) computePredictiveFUP(sub models.Subscriber, now time.Time) {
	if sub.Service == nil {
		return
	}

	dailyQuotaBytes := sub.Service.DailyQuota
	if dailyQuotaBytes <= 0 {
		return
	}

	// Calculate average daily usage over last 7 days
	var totalBytes float64
	database.DB.Raw(`
		SELECT COALESCE(SUM(acctinputoctets + acctoutputoctets), 0) FROM radacct
		WHERE username = ? AND acctstarttime > ?`,
		sub.Username, now.AddDate(0, 0, -7)).Scan(&totalBytes)

	velocityBytesPerDay := totalBytes / 7.0
	if velocityBytesPerDay <= 0 {
		return
	}

	remainingBytes := dailyQuotaBytes - sub.DailyDownloadUsed
	if remainingBytes <= 0 {
		// Already hit FUP
		database.DB.Model(&models.Subscriber{}).Where("id = ?", sub.ID).Updates(map[string]interface{}{
			"usage_velocity":    velocityBytesPerDay,
			"predicted_fup_date": now,
		})
		return
	}

	daysUntilFUP := math.Ceil(float64(remainingBytes) / velocityBytesPerDay)
	if daysUntilFUP > 365 {
		daysUntilFUP = 365
	}
	predictedDate := now.AddDate(0, 0, int(daysUntilFUP))

	database.DB.Model(&models.Subscriber{}).Where("id = ?", sub.ID).Updates(map[string]interface{}{
		"usage_velocity":    velocityBytesPerDay,
		"predicted_fup_date": predictedDate,
	})
}

func churnRiskLevel(score int) string {
	switch {
	case score >= 76:
		return "critical"
	case score >= 51:
		return "high"
	case score >= 26:
		return "medium"
	default:
		return "low"
	}
}

func formatDaysLeft(days int, expired bool) string {
	if expired {
		return "Already expired"
	}
	return time.Now().AddDate(0, 0, days).Format("2006-01-02")
}

func formatTickets(count int) string {
	if count == 1 {
		return "1 ticket in last 30 days"
	}
	return time.Now().Format("") // placeholder
}

func formatDelays(count int) string {
	if count == 1 {
		return "1 late payment in last 90 days"
	}
	return ""
}
