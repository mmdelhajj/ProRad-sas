package models

import (
	"time"

	"gorm.io/gorm"
)

// SharingDetection stores detected sharing incidents
type SharingDetection struct {
	ID                 uint           `gorm:"primaryKey" json:"id"`
	SubscriberID       uint           `gorm:"index;not null" json:"subscriber_id"`
	Subscriber         *Subscriber    `gorm:"foreignKey:SubscriberID;references:ID" json:"subscriber,omitempty"`
	Username           string         `gorm:"size:100;index" json:"username"`
	FullName           string         `gorm:"size:200" json:"full_name"`
	IPAddress          string         `gorm:"size:45" json:"ip_address"`
	ServiceName        string         `gorm:"size:100" json:"service_name"`
	NasID              *uint          `json:"nas_id"`
	NasName            string         `gorm:"size:100" json:"nas_name"`
	ConnectionCount    int            `json:"connection_count"`
	UniqueDestinations int            `json:"unique_destinations"`
	TTLValues          string         `gorm:"type:text" json:"ttl_values"` // JSON array stored as string
	TTLStatus          string         `gorm:"size:50" json:"ttl_status"`   // normal, router_detected, multiple_os, double_router
	SuspicionLevel     string         `gorm:"size:20;index" json:"suspicion_level"` // low, medium, high
	ConfidenceScore    int            `json:"confidence_score"` // 0-100
	Reasons            string         `gorm:"type:text" json:"reasons"` // JSON array stored as string
	DetectedAt         time.Time      `gorm:"index" json:"detected_at"`
	ScanType              string         `gorm:"size:20;default:'automatic'" json:"scan_type"` // automatic, manual
	TrafficAnomalyFactor  int            `gorm:"default:0" json:"traffic_anomaly_factor"`
	MACDiversityFactor    int            `gorm:"default:0" json:"mac_diversity_factor"`
	ServiceSpeed          int            `gorm:"default:0" json:"service_speed"`
	CreatedAt             time.Time      `json:"created_at"`
	DeletedAt             gorm.DeletedAt `gorm:"index" json:"-"`
}

// SharingDetectionSetting stores sharing detection configuration
type SharingDetectionSetting struct {
	ID                    uint      `gorm:"primaryKey" json:"id"`
	Enabled               bool      `gorm:"default:true" json:"enabled"`
	ScanTime              string    `gorm:"size:5;default:'03:00'" json:"scan_time"` // HH:MM format
	RetentionDays         int       `gorm:"default:30" json:"retention_days"`
	MinSuspicionLevel     string    `gorm:"size:20;default:'medium'" json:"min_suspicion_level"` // Only save medium/high
	ConnectionThreshold   int       `gorm:"default:500" json:"connection_threshold"`
	NotifyOnHighRisk      bool      `gorm:"default:false" json:"notify_on_high_risk"`
	AutoSuspendRepeat     bool      `gorm:"default:false" json:"auto_suspend_repeat"` // Auto-suspend repeat offenders
	RepeatThreshold       int       `gorm:"default:5" json:"repeat_threshold"`        // How many detections before action

	// Automated actions
	AutoFlagEnabled            bool   `gorm:"default:false" json:"auto_flag_enabled"`
	AutoFlagThreshold          int    `gorm:"default:70" json:"auto_flag_threshold"`
	SpeedReductionEnabled      bool   `gorm:"default:false" json:"speed_reduction_enabled"`
	SpeedReductionThreshold    int    `gorm:"default:80" json:"speed_reduction_threshold"`
	SpeedReductionPercent      int    `gorm:"default:50" json:"speed_reduction_percent"`
	WhatsappNotifyEnabled      bool   `gorm:"default:false" json:"whatsapp_notify_enabled"`
	WhatsappNotifyThreshold    int    `gorm:"default:60" json:"whatsapp_notify_threshold"`
	WhatsappNotifyTemplate     string `gorm:"type:text" json:"whatsapp_notify_template"`
	ScoreEnabled               bool   `gorm:"default:true" json:"score_enabled"`

	UpdatedAt time.Time `json:"updated_at"`
}

// SubscriberSharingScore stores monthly sharing evaluation per subscriber
type SubscriberSharingScore struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	SubscriberID        uint      `gorm:"index;not null" json:"subscriber_id"`
	Month               string    `gorm:"size:7;not null" json:"month"` // YYYY-MM
	Score               int       `gorm:"default:0" json:"score"`       // 0-100
	Category            string    `gorm:"size:20;default:'good'" json:"category"` // good / warning / bad
	DetectionCount      int       `gorm:"default:0" json:"detection_count"`
	AvgConfidence       float64   `gorm:"default:0" json:"avg_confidence"`
	PeakConfidence      int       `gorm:"default:0" json:"peak_confidence"`
	Trend               string    `gorm:"size:20;default:'stable'" json:"trend"` // improving / stable / worsening
	PreviousScore       int       `gorm:"default:0" json:"previous_score"`
	TrafficAnomalyScore int       `gorm:"default:0" json:"traffic_anomaly_score"`
	MACDiversityScore   int       `gorm:"default:0" json:"mac_diversity_score"`
	FUPPatternScore     int       `gorm:"default:0" json:"fup_pattern_score"`
	ComputedAt          time.Time `json:"computed_at"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`

	// Virtual fields for joins
	Username    string `gorm:"-" json:"username,omitempty"`
	FullName    string `gorm:"-" json:"full_name,omitempty"`
	ServiceName string `gorm:"-" json:"service_name,omitempty"`
}

// SharingActionLog stores automated action history
type SharingActionLog struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	SubscriberID    uint      `gorm:"index;not null" json:"subscriber_id"`
	Username        string    `gorm:"size:100" json:"username"`
	ActionType      string    `gorm:"size:30;not null" json:"action_type"` // auto_flag / speed_reduction / whatsapp_notification / manual_whitelist
	TriggerScore    int       `gorm:"default:0" json:"trigger_score"`
	TriggerCategory string    `gorm:"size:20" json:"trigger_category"`
	Details         string    `gorm:"type:text" json:"details"`
	CreatedAt       time.Time `json:"created_at"`
}

func (SharingDetection) TableName() string {
	return "sharing_detections"
}

func (SharingDetectionSetting) TableName() string {
	return "sharing_detection_settings"
}

func (SubscriberSharingScore) TableName() string {
	return "subscriber_sharing_scores"
}

func (SharingActionLog) TableName() string {
	return "sharing_action_logs"
}
