package services

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

// TenantWorkerManager orchestrates per-tenant background services (QuotaSync, etc.)
type TenantWorkerManager struct {
	mu      sync.RWMutex
	workers map[uint]*TenantWorker
	ctx     context.Context
	cancel  context.CancelFunc
}

// TenantWorker holds the services running for a single tenant
type TenantWorker struct {
	TenantID   uint
	SchemaName string
	cancel     context.CancelFunc
	running    bool
}

// NewTenantWorkerManager creates a new manager
func NewTenantWorkerManager() *TenantWorkerManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &TenantWorkerManager{
		workers: make(map[uint]*TenantWorker),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start begins the worker manager loop
func (m *TenantWorkerManager) Start() {
	log.Println("SaaS: TenantWorkerManager starting...")

	// Initial sync
	m.syncWorkers()

	// Monitor for tenant changes every 60 seconds
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-m.ctx.Done():
				log.Println("SaaS: TenantWorkerManager stopped")
				return
			case <-ticker.C:
				m.syncWorkers()
			}
		}
	}()
}

// Stop shuts down all workers
func (m *TenantWorkerManager) Stop() {
	m.cancel()
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, worker := range m.workers {
		worker.cancel()
		log.Printf("SaaS: Stopped worker for tenant %d", id)
	}
	m.workers = make(map[uint]*TenantWorker)
}

// syncWorkers loads active tenants and starts/stops workers as needed
func (m *TenantWorkerManager) syncWorkers() {
	var tenants []models.Tenant
	if err := database.DB.Where("status IN ('active', 'trial')").Find(&tenants).Error; err != nil {
		log.Printf("SaaS: Failed to load tenants: %v", err)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Track which tenants are active
	activeTenantIDs := make(map[uint]bool)
	for _, tenant := range tenants {
		activeTenantIDs[tenant.ID] = true

		// Start worker if not already running
		if _, exists := m.workers[tenant.ID]; !exists {
			m.startWorker(tenant)
		}
	}

	// Stop workers for removed/suspended tenants
	for id, worker := range m.workers {
		if !activeTenantIDs[id] {
			worker.cancel()
			delete(m.workers, id)
			log.Printf("SaaS: Stopped worker for tenant %d (no longer active)", id)
		}
	}
}

// startWorker launches background services for a tenant
func (m *TenantWorkerManager) startWorker(tenant models.Tenant) {
	ctx, cancel := context.WithCancel(m.ctx)

	worker := &TenantWorker{
		TenantID:   tenant.ID,
		SchemaName: tenant.SchemaName,
		cancel:     cancel,
		running:    true,
	}

	m.workers[tenant.ID] = worker

	// Start per-tenant QuotaSync goroutine
	go func() {
		log.Printf("SaaS: Starting QuotaSync worker for tenant %d (%s)", tenant.ID, tenant.Name)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.runTenantQuotaSync(tenant)
			}
		}
	}()

	// Start per-tenant StaleSessionCleanup
	go func() {
		log.Printf("SaaS: Starting StaleSessionCleanup for tenant %d (%s)", tenant.ID, tenant.Name)
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.runTenantStaleCleanup(tenant)
			}
		}
	}()

	log.Printf("SaaS: Started worker for tenant %d (%s, schema: %s)", tenant.ID, tenant.Name, tenant.SchemaName)
}

// runTenantQuotaSync runs a single QuotaSync cycle for a tenant
func (m *TenantWorkerManager) runTenantQuotaSync(tenant models.Tenant) {
	tenantDB := database.GetTenantDB(tenant.SchemaName)

	// Get online subscribers for this tenant
	var subscribers []models.Subscriber
	if err := tenantDB.Where("is_online = true").
		Preload("Service").
		Find(&subscribers).Error; err != nil {
		log.Printf("SaaS: QuotaSync tenant %d: failed to load subscribers: %v", tenant.ID, err)
		return
	}

	if len(subscribers) == 0 {
		return
	}

	// Connect to tenant's MikroTik via VPN IP
	if tenant.MikrotikAPIIP == "" || tenant.MikrotikAPIUser == "" {
		return // MikroTik not configured yet
	}

	// TODO: Use MikroTik client to sync bandwidth
	// This will use the existing QuotaSyncService logic but with tenant-scoped DB
	log.Printf("SaaS: QuotaSync tenant %d: %d online subscribers", tenant.ID, len(subscribers))
}

// runTenantStaleCleanup cleans up stale sessions for a tenant
func (m *TenantWorkerManager) runTenantStaleCleanup(tenant models.Tenant) {
	tenantDB := database.GetTenantDB(tenant.SchemaName)

	thresholdTime := time.Now().Add(-30 * time.Minute)

	// Close stale radacct sessions
	result := tenantDB.Exec(`
		UPDATE radacct SET
			acctstoptime = NOW(),
			acctterminatecause = 'Stale-Session'
		WHERE acctstoptime IS NULL
		AND acctinputoctets = 0
		AND acctoutputoctets = 0
		AND acctstarttime < ?
	`, thresholdTime)

	if result.RowsAffected > 0 {
		log.Printf("SaaS: StaleCleanup tenant %d: closed %d stale sessions", tenant.ID, result.RowsAffected)
	}

	// Sync is_online status
	tenantDB.Exec(`
		UPDATE subscribers SET is_online = false
		WHERE is_online = true
		AND id NOT IN (
			SELECT DISTINCT ON (username) s.id
			FROM subscribers s
			JOIN radacct r ON r.username = s.username
			WHERE r.acctstoptime IS NULL
		)
	`)
}

// GetWorkerCount returns the number of active workers
func (m *TenantWorkerManager) GetWorkerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.workers)
}

// GetWorkerTenantIDs returns the list of tenant IDs with active workers
func (m *TenantWorkerManager) GetWorkerTenantIDs() []uint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]uint, 0, len(m.workers))
	for id := range m.workers {
		ids = append(ids, id)
	}
	return ids
}
