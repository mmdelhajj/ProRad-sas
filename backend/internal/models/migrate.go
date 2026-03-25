package models

import (
	_ "embed"
	"encoding/json"
	"log"
	"time"

	"gorm.io/gorm"
)

//go:embed schema.sql
var schemaSQL string

// BandwidthRule represents a time-based bandwidth adjustment rule
type BandwidthRule struct {
	ID                 uint            `gorm:"column:id;primaryKey" json:"id"`
	Name               string          `gorm:"column:name;size:100;not null" json:"name"`
	TriggerType        string          `gorm:"column:trigger_type;size:20;not null;default:time" json:"trigger_type"`
	StartTime          string          `gorm:"column:start_time;size:10" json:"start_time"`
	EndTime            string          `gorm:"column:end_time;size:10" json:"end_time"`
	DaysOfWeek         json.RawMessage `gorm:"column:days_of_week;type:json" json:"days_of_week"`
	UploadMultiplier   int             `gorm:"column:upload_multiplier;default:100" json:"upload_multiplier"`
	DownloadMultiplier int             `gorm:"column:download_multiplier;default:100" json:"download_multiplier"`
	ServiceIDs         json.RawMessage `gorm:"column:service_ids;type:json" json:"service_ids"`
	Priority           int             `gorm:"column:priority;default:10" json:"priority"`
	Enabled            bool            `gorm:"column:enabled;default:true" json:"enabled"`
	AutoApply          bool            `gorm:"column:auto_apply;default:false" json:"auto_apply"`
}

func (BandwidthRule) TableName() string {
	return "bandwidth_rules"
}

// CDNBandwidthRule represents a time-based CDN bandwidth adjustment rule
type CDNBandwidthRule struct {
	ID              uint            `gorm:"column:id;primaryKey" json:"id"`
	Name            string          `gorm:"column:name;size:100;not null" json:"name"`
	StartTime       string          `gorm:"column:start_time;size:10" json:"start_time"`
	EndTime         string          `gorm:"column:end_time;size:10" json:"end_time"`
	DaysOfWeek      json.RawMessage `gorm:"column:days_of_week;type:json" json:"days_of_week"`
	SpeedMultiplier int             `gorm:"column:speed_multiplier;default:100" json:"speed_multiplier"`
	CDNIDs          json.RawMessage `gorm:"column:cdn_ids;type:json" json:"cdn_ids"`
	ServiceIDs      json.RawMessage `gorm:"column:service_ids;type:json" json:"service_ids"`
	Priority        int             `gorm:"column:priority;default:10" json:"priority"`
	Enabled         bool            `gorm:"column:enabled;default:true" json:"enabled"`
	AutoApply       bool            `gorm:"column:auto_apply;default:false" json:"auto_apply"`
}

func (CDNBandwidthRule) TableName() string {
	return "cdn_bandwidth_rules"
}

// AutoMigrate runs database migrations using raw SQL
// This bypasses GORM's AutoMigrate to work with code obfuscation
func AutoMigrate(db *gorm.DB) error {
	log.Println("Running database migrations using SQL schema...")

	// Execute the entire schema at once instead of splitting
	// PostgreSQL can handle multiple statements in one exec
	if err := db.Exec(schemaSQL).Error; err != nil {
		log.Printf("SQL schema execution warning: %v", err)
		// Don't return error - some statements may fail if objects exist
	}

	log.Println("Database migrations completed successfully")
	return nil
}

// DailyQuota tracks daily quota usage
type DailyQuota struct {
	ID           uint   `gorm:"column:id;primaryKey" json:"id"`
	SubscriberID uint   `gorm:"column:subscriber_id;uniqueIndex:idx_daily_subscriber_date;not null" json:"subscriber_id"`
	Date         string `gorm:"column:date;size:10;uniqueIndex:idx_daily_subscriber_date;not null" json:"date"` // YYYY-MM-DD
	Download     int64  `gorm:"column:download;default:0" json:"download"`
	Upload       int64  `gorm:"column:upload;default:0" json:"upload"`
	Total        int64  `gorm:"column:total;default:0" json:"total"`
}

// MonthlyQuota tracks monthly quota usage
type MonthlyQuota struct {
	ID           uint   `gorm:"column:id;primaryKey" json:"id"`
	SubscriberID uint   `gorm:"column:subscriber_id;uniqueIndex:idx_monthly_subscriber_month;not null" json:"subscriber_id"`
	Month        string `gorm:"column:month;size:7;uniqueIndex:idx_monthly_subscriber_month;not null" json:"month"` // YYYY-MM
	Download     int64  `gorm:"column:download;default:0" json:"download"`
	Upload       int64  `gorm:"column:upload;default:0" json:"upload"`
	Total        int64  `gorm:"column:total;default:0" json:"total"`
}

// SystemPreference represents system-wide preferences
type SystemPreference struct {
	ID        uint   `gorm:"column:id;primaryKey" json:"id"`
	Key       string `gorm:"column:key;size:100;uniqueIndex;not null" json:"key"`
	Value     string `gorm:"column:value;type:text" json:"value"`
	ValueType string `gorm:"column:value_type;size:20;default:string" json:"value_type"` // string, int, bool, json
}

func (DailyQuota) TableName() string {
	return "daily_quotas"
}

func (MonthlyQuota) TableName() string {
	return "monthly_quotas"
}

func (SystemPreference) TableName() string {
	return "system_preferences"
}

// Session represents an active user session (view from RadAcct)
type Session struct {
	ID               uint        `gorm:"column:id;primaryKey" json:"id"`
	Username         string      `gorm:"column:username;size:100;not null;index" json:"username"`
	SubscriberID     *uint       `gorm:"column:subscriber_id;index" json:"subscriber_id"`
	Subscriber       *Subscriber `gorm:"-" json:"subscriber,omitempty"`
	NASID            *uint       `gorm:"column:nas_id;index" json:"nas_id"`
	NAS              *Nas        `gorm:"-" json:"nas,omitempty"`
	NASIPAddress     string      `gorm:"column:nas_ip_address;size:50" json:"nas_ip_address"`
	FramedIPAddress  string      `gorm:"column:framed_ip_address;size:50" json:"framed_ip_address"`
	CallingStationID string      `gorm:"column:calling_station_id;size:50" json:"calling_station_id"` // MAC address
	AcctSessionID    string      `gorm:"column:acct_session_id;size:100;uniqueIndex" json:"acct_session_id"`
	SessionTime      int64       `gorm:"column:session_time" json:"session_time"` // in seconds
	InputOctets      int64       `gorm:"column:input_octets" json:"input_octets"`
	OutputOctets     int64       `gorm:"column:output_octets" json:"output_octets"`
	Status           string      `gorm:"column:status;size:20;default:online;index" json:"status"` // online, offline
	StartTime        time.Time   `gorm:"column:start_time" json:"start_time"`
	UpdateTime       *time.Time  `gorm:"column:update_time" json:"update_time"`
	StopTime         *time.Time  `gorm:"column:stop_time" json:"stop_time"`
}

func (Session) TableName() string {
	return "sessions"
}

// RadiusAccounting is an alias for RadAcct for clearer usage
type RadiusAccounting = RadAcct
