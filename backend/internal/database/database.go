package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/proisp/backend/internal/config"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	DB    *gorm.DB
	Redis *redis.Client
)

func Connect(cfg *config.Config) error {
	// PostgreSQL connection with retry logic
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable TimeZone=UTC",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName,
	)

	var err error
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
			NowFunc: func() time.Time {
				return time.Now().UTC()
			},
			DisableForeignKeyConstraintWhenMigrating: true,
		})
		if err == nil {
			break
		}
		log.Printf("Database connection attempt %d/%d failed: %v. Retrying in 2 seconds...", i+1, maxRetries, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
	}

	// Configure connection pool
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	// Connection pool optimized for 30,000+ users
	// MaxOpenConns: 1500 for high concurrency (requires PostgreSQL max_connections = 2000)
	// MaxIdleConns: Keep 100 connections ready for burst traffic
	// ConnMaxLifetime: Recycle connections every 30 minutes to prevent stale connections
	// ConnMaxIdleTime: Close idle connections after 5 minutes to free resources
	sqlDB.SetMaxIdleConns(100)
	sqlDB.SetMaxOpenConns(1500)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	log.Println("Database connected successfully")

	// Redis connection
	Redis = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := Redis.Ping(ctx).Result(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Println("Redis connected successfully")

	return nil
}

func Close() {
	if sqlDB, err := DB.DB(); err == nil {
		sqlDB.Close()
	}
	if Redis != nil {
		Redis.Close()
	}
}

// EnsureIndexes creates performance indexes on frequently queried columns
// This should be called after AutoMigrate
func EnsureIndexes() {
	indexes := []string{
		// Schema migrations
		"ALTER TABLE subscribers ADD COLUMN IF NOT EXISTS whatsapp_notifications BOOLEAN DEFAULT false",
		"ALTER TABLE resellers ADD COLUMN IF NOT EXISTS rebrand_enabled BOOLEAN DEFAULT false",
		"ALTER TABLE resellers ADD COLUMN IF NOT EXISTS custom_domain VARCHAR(255)",
		"CREATE TABLE IF NOT EXISTS reseller_brandings (id SERIAL PRIMARY KEY, reseller_id INTEGER NOT NULL UNIQUE REFERENCES resellers(id) ON DELETE CASCADE, company_name VARCHAR(255), logo_path VARCHAR(500), primary_color VARCHAR(20) DEFAULT '#2563eb', footer_text VARCHAR(500), tagline VARCHAR(500), created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)",
		// Daily usage history - saved before daily reset so chart shows correct per-day values
		"CREATE TABLE IF NOT EXISTS daily_usage_history (id BIGSERIAL PRIMARY KEY, subscriber_id INTEGER NOT NULL, date DATE NOT NULL, download_bytes BIGINT DEFAULT 0, upload_bytes BIGINT DEFAULT 0, created_at TIMESTAMPTZ DEFAULT NOW(), UNIQUE(subscriber_id, date))",
		"CREATE INDEX IF NOT EXISTS idx_daily_usage_history_sub_date ON daily_usage_history(subscriber_id, date)",

		// Fix: Replace UNIQUE constraint with partial unique index (allow soft-deleted duplicates)
		"ALTER TABLE subscribers DROP CONSTRAINT IF EXISTS subscribers_username_key",
		"CREATE UNIQUE INDEX IF NOT EXISTS subscribers_username_key ON subscribers (username) WHERE deleted_at IS NULL",

		// Subscribers - most frequently queried table
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subscribers_reseller_id ON subscribers(reseller_id)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subscribers_service_id ON subscribers(service_id)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subscribers_nas_id ON subscribers(nas_id)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subscribers_username ON subscribers(username)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subscribers_status ON subscribers(status)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subscribers_is_online ON subscribers(is_online)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subscribers_expiry_date ON subscribers(expiry_date)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subscribers_created_at ON subscribers(created_at)",

		// Composite index for common query patterns
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subscribers_reseller_status ON subscribers(reseller_id, status)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subscribers_online_nas ON subscribers(is_online, nas_id) WHERE is_online = true",

		// Transactions
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_transactions_reseller_id ON transactions(reseller_id)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_transactions_subscriber_id ON transactions(subscriber_id)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_transactions_type ON transactions(type)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_transactions_created_at ON transactions(created_at)",

		// RADIUS accounting
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_radacct_username ON radacct(username)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_radacct_nasipaddress ON radacct(nasipaddress)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_radacct_acctstarttime ON radacct(acctstarttime)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_radacct_acctstoptime ON radacct(acctstoptime)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_radacct_acctsessionid ON radacct(acctsessionid)",

		// Active sessions index for QuotaSync
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_radacct_active ON radacct(username, acctstarttime) WHERE acctstoptime IS NULL",

		// Services
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_services_name ON services(name)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_services_is_active ON services(is_active)",

		// NAS devices
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_nas_devices_ip_address ON nas_devices(ip_address)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_nas_devices_is_active ON nas_devices(is_active)",

		// Audit logs
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_audit_logs_entity_type ON audit_logs(entity_type)",

		// Resellers
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_resellers_user_id ON resellers(user_id)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_resellers_parent_id ON resellers(parent_id)",

		// Prepaid cards
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_prepaid_cards_code ON prepaid_cards(code)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_prepaid_cards_is_used ON prepaid_cards(is_used)",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_prepaid_cards_reseller_id ON prepaid_cards(reseller_id)",

		// Bandwidth Manager IPAM + History tables
		"CREATE TABLE IF NOT EXISTS bw_ip_blocks (id SERIAL PRIMARY KEY, name VARCHAR(100) NOT NULL, cidr VARCHAR(50) NOT NULL, gateway VARCHAR(45), subnet_mask VARCHAR(20), description TEXT, total_ips INTEGER DEFAULT 0, used_ips INTEGER DEFAULT 0, is_active BOOLEAN DEFAULT true, created_at TIMESTAMPTZ DEFAULT NOW(), updated_at TIMESTAMPTZ DEFAULT NOW(), deleted_at TIMESTAMPTZ)",
		"CREATE TABLE IF NOT EXISTS bw_ip_allocations (id SERIAL PRIMARY KEY, block_id INTEGER NOT NULL REFERENCES bw_ip_blocks(id), customer_id INTEGER REFERENCES bandwidth_customers(id), ip_address VARCHAR(45) NOT NULL, status VARCHAR(20) DEFAULT 'available', notes TEXT, assigned_at TIMESTAMPTZ, created_at TIMESTAMPTZ DEFAULT NOW(), updated_at TIMESTAMPTZ DEFAULT NOW())",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_bw_ip_alloc_ip ON bw_ip_allocations(ip_address)",
		"CREATE INDEX IF NOT EXISTS idx_bw_ip_alloc_block ON bw_ip_allocations(block_id)",
		"ALTER TABLE bandwidth_customers ADD COLUMN IF NOT EXISTS ip_block_id INTEGER",
		"ALTER TABLE bandwidth_customers ADD COLUMN IF NOT EXISTS ip_allocation_id INTEGER",
		"CREATE TABLE IF NOT EXISTS bw_hourly_usage (id SERIAL PRIMARY KEY, customer_id INTEGER NOT NULL REFERENCES bandwidth_customers(id), hour TIMESTAMPTZ NOT NULL, download_bytes BIGINT DEFAULT 0, upload_bytes BIGINT DEFAULT 0, peak_download_kbps BIGINT DEFAULT 0, peak_upload_kbps BIGINT DEFAULT 0, sample_count INTEGER DEFAULT 0, created_at TIMESTAMPTZ DEFAULT NOW())",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_bw_hourly_customer_hour ON bw_hourly_usage(customer_id, hour)",
		"CREATE TABLE IF NOT EXISTS bw_sessions (id SERIAL PRIMARY KEY, customer_id INTEGER NOT NULL REFERENCES bandwidth_customers(id), started_at TIMESTAMPTZ NOT NULL, ended_at TIMESTAMPTZ, duration_sec INTEGER DEFAULT 0, download_bytes BIGINT DEFAULT 0, upload_bytes BIGINT DEFAULT 0)",
		"CREATE INDEX IF NOT EXISTS idx_bw_sessions_customer ON bw_sessions(customer_id)",
	}

	for _, indexSQL := range indexes {
		// Execute each index creation - errors are ignored as index may already exist
		if err := DB.Exec(indexSQL).Error; err != nil {
			// Log but don't fail - CONCURRENTLY indexes may fail in transaction
			log.Printf("Index creation skipped (may already exist): %v", err)
		}
	}

	log.Println("Database indexes ensured")
}
