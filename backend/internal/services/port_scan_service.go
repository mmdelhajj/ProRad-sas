package services

import (
	"log"
	"sync"
	"time"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

// PortScanService periodically checks if the configured WAN port is open
// on all online subscribers via MikroTik /tool/fetch, and updates port_open field.
type PortScanService struct {
	stopChan chan struct{}
	wg       sync.WaitGroup
}

func NewPortScanService() *PortScanService {
	return &PortScanService{
		stopChan: make(chan struct{}),
	}
}

func (s *PortScanService) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		log.Println("PortScanService started, scanning every 2 minutes")

		// Wait 30s after startup before first scan (let QuotaSync settle)
		select {
		case <-time.After(30 * time.Second):
		case <-s.stopChan:
			return
		}

		s.scanAll()

		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.scanAll()
			case <-s.stopChan:
				log.Println("PortScanService stopped")
				return
			}
		}
	}()
}

func (s *PortScanService) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

func (s *PortScanService) scanAll() {
	port := getWanCheckPort()

	// Get all online subscribers grouped by NAS
	var subscribers []models.Subscriber
	if err := database.DB.Preload("Nas").
		Where("is_online = ? AND nas_id IS NOT NULL", true).
		Find(&subscribers).Error; err != nil {
		log.Printf("PortScan: Failed to get subscribers: %v", err)
		return
	}

	if len(subscribers) == 0 {
		return
	}

	// Group by NAS
	nasSubs := make(map[uint][]models.Subscriber)
	nasMap := make(map[uint]*models.Nas)
	for _, sub := range subscribers {
		if sub.NasID != nil && sub.Nas != nil {
			nasSubs[*sub.NasID] = append(nasSubs[*sub.NasID], sub)
			nasMap[*sub.NasID] = sub.Nas
		}
	}

	var totalOpen, totalClosed int

	// Scan each NAS group
	for nasID, subs := range nasSubs {
		nas := nasMap[nasID]
		open, closed := s.scanNas(nas, subs, port)
		totalOpen += open
		totalClosed += closed
	}

	log.Printf("PortScan: Scanned %d subscribers — %d open, %d closed (port %d)",
		len(subscribers), totalOpen, totalClosed, port)

	// Mark all offline subscribers as port_open = false
	database.DB.Model(&models.Subscriber{}).
		Where("is_online = ? AND port_open = ?", false, true).
		Update("port_open", false)
}

func (s *PortScanService) scanNas(nas *models.Nas, subs []models.Subscriber, port int) (open, closed int) {
	for _, sub := range subs {
		ip := sub.IPAddress
		if ip == "" {
			ip = sub.StaticIP
		}
		if ip == "" {
			continue
		}

		// Use direct TCP check from API server (MikroTik /tool/fetch can't reach its own PPPoE clients)
		portOpen := checkPortDirect(ip, port, 2)

		// Only update DB if value changed
		if sub.PortOpen != portOpen {
			database.DB.Model(&models.Subscriber{}).
				Where("id = ?", sub.ID).
				Update("port_open", portOpen)
		}

		if portOpen {
			open++
		} else {
			closed++
		}
	}

	return open, closed
}
