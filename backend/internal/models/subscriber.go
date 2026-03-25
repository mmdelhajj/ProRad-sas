package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// SubscriberStatus represents the status of a subscriber
type SubscriberStatus int

const (
	SubscriberStatusActive   SubscriberStatus = 1
	SubscriberStatusInactive SubscriberStatus = 2
	SubscriberStatusExpired  SubscriberStatus = 3
	SubscriberStatusStopped  SubscriberStatus = 4
)

// Subscriber represents a PPPoE/Hotspot subscriber
type Subscriber struct {
	ID              uint             `gorm:"column:id;primaryKey" json:"id"`
	Username        string           `gorm:"column:username;index;size:100;not null" json:"username"`
	Password        string           `gorm:"column:password;size:255;not null" json:"-"`
	PasswordPlain   string           `gorm:"column:password_plain;size:255" json:"password_plain"` // Shown in edit form
	FullName        string           `gorm:"column:full_name;size:255" json:"full_name"`
	Email           string           `gorm:"column:email;size:255" json:"email"`
	Phone           string           `gorm:"column:phone;size:50" json:"phone"`
	Address         string           `gorm:"column:address;size:500" json:"address"`
	Region          string           `gorm:"column:region;size:100" json:"region"`
	Building        string           `gorm:"column:building;size:100" json:"building"`
	Nationality     string           `gorm:"column:nationality;size:100" json:"nationality"`
	Country         string           `gorm:"column:country;size:100" json:"country"`
	Note            string           `gorm:"column:note;type:text" json:"note"`

	// Service & Billing
	ServiceID       uint             `gorm:"column:service_id;not null" json:"service_id"`
	Service         *Service         `gorm:"foreignKey:ServiceID;references:ID" json:"service"`
	Status          SubscriberStatus `gorm:"column:status;default:1" json:"status"`
	ExpiryDate      time.Time        `gorm:"column:expiry_date" json:"expiry_date"`
	DueDate         *time.Time       `gorm:"column:due_date" json:"due_date"`
	Price           float64          `gorm:"column:price;type:decimal(15,2)" json:"price"`
	OverridePrice   bool             `gorm:"column:override_price;default:false" json:"override_price"`
	AutoRenew       bool             `gorm:"column:auto_renew;default:false" json:"auto_renew"`

	// Quota & FUP - stored in database for persistence
	DailyDownloadUsed   int64      `gorm:"column:daily_download_used;default:0" json:"daily_download_used"`
	DailyUploadUsed     int64      `gorm:"column:daily_upload_used;default:0" json:"daily_upload_used"`
	MonthlyDownloadUsed int64      `gorm:"column:monthly_download_used;default:0" json:"monthly_download_used"`
	MonthlyUploadUsed   int64      `gorm:"column:monthly_upload_used;default:0" json:"monthly_upload_used"`
	FUPLevel            int        `gorm:"column:fup_level;default:0" json:"fup_level"`
	MonthlyFUPLevel     int        `gorm:"column:monthly_fup_level;default:0" json:"monthly_fup_level"`
	LastDailyReset      *time.Time `gorm:"column:last_daily_reset" json:"last_daily_reset"`
	LastMonthlyReset    *time.Time `gorm:"column:last_monthly_reset" json:"last_monthly_reset"`
	LastSessionDownload int64      `gorm:"column:last_session_download;default:0" json:"last_session_download"`
	LastSessionUpload   int64      `gorm:"column:last_session_upload;default:0" json:"last_session_upload"`
	LastQuotaSync       *time.Time `gorm:"column:last_quota_sync" json:"last_quota_sync"`
	LastBypassCDNBytes  int64      `gorm:"column:last_bypass_cdn_bytes;default:0" json:"last_bypass_cdn_bytes"`
	// CDN FUP tracking
	CDNDailyDownloadUsed   int64 `gorm:"column:cdn_daily_download_used;default:0" json:"cdn_daily_download_used"`
	CDNDailyUploadUsed     int64 `gorm:"column:cdn_daily_upload_used;default:0" json:"cdn_daily_upload_used"`
	CDNMonthlyDownloadUsed int64 `gorm:"column:cdn_monthly_download_used;default:0" json:"cdn_monthly_download_used"`
	CDNMonthlyUploadUsed   int64 `gorm:"column:cdn_monthly_upload_used;default:0" json:"cdn_monthly_upload_used"`
	CDNFUPLevel            int   `gorm:"column:cdn_fup_level;default:0" json:"cdn_fup_level"`
	CDNMonthlyFUPLevel     int   `gorm:"column:cdn_monthly_fup_level;default:0" json:"cdn_monthly_fup_level"`
	DailyQuotaUsed   int64      `gorm:"column:daily_quota_used;default:0" json:"daily_quota_used"`
	MonthlyQuotaUsed int64      `gorm:"column:monthly_quota_used;default:0" json:"monthly_quota_used"`
	LastQuotaReset   *time.Time `gorm:"column:last_quota_reset" json:"last_quota_reset"`

	// Network
	MACAddress      string  `gorm:"column:mac_address;size:50;index" json:"mac_address"`
	IPAddress       string  `gorm:"column:ip_address;size:50" json:"ip_address"`
	StaticIP        string  `gorm:"column:static_ip;size:50" json:"static_ip"`
	SaveMAC         bool    `gorm:"column:save_mac;default:true" json:"save_mac"`
	NasID           *uint   `gorm:"column:nas_id" json:"nas_id"`
	Nas             *Nas    `gorm:"foreignKey:NasID;references:ID" json:"nas,omitempty"`

	// Location
	SwitchID        *uint   `gorm:"column:switch_id" json:"switch_id"`
	Switch          *Switch `gorm:"foreignKey:SwitchID;references:ID" json:"switch,omitempty"`
	Latitude        float64 `gorm:"column:latitude;type:decimal(10,8)" json:"latitude"`
	Longitude       float64 `gorm:"column:longitude;type:decimal(11,8)" json:"longitude"`

	// Ownership
	ResellerID      uint     `gorm:"column:reseller_id;not null;index" json:"reseller_id"`
	Reseller        *Reseller `gorm:"foreignKey:ResellerID;references:ID" json:"reseller"`
	CollectorID     *uint    `gorm:"column:collector_id" json:"collector_id"`

	// Session
	IsOnline             bool       `gorm:"column:is_online;default:false;index" json:"is_online"`
	LastSeen             *time.Time `gorm:"column:last_seen" json:"last_seen"`
	SessionID            string     `gorm:"column:session_id;size:100" json:"session_id"`
	SimultaneousSessions int        `gorm:"column:simultaneous_sessions;default:1" json:"simultaneous_sessions"`

	// Auto-recharge
	AutoRecharge     bool `gorm:"column:auto_recharge;default:false" json:"auto_recharge"`
	AutoRechargeDays int  `gorm:"column:auto_recharge_days;default:0" json:"auto_recharge_days"`
	AutoInvoice      bool `gorm:"column:auto_invoice;default:false" json:"auto_invoice"`

	// Notifications
	WhatsAppNotifications bool `gorm:"column:whatsapp_notifications;default:false" json:"whatsapp_notifications"`

	// WAN Management Check
	WanCheckStatus string `gorm:"column:wan_check_status;size:20;default:unchecked" json:"wan_check_status"`
	PortOpen       bool   `gorm:"column:port_open;default:false" json:"port_open"`

	// Sharing detection whitelist
	SharingWhitelisted     bool   `gorm:"column:sharing_whitelisted;default:false" json:"sharing_whitelisted"`
	SharingWhitelistReason string `gorm:"column:sharing_whitelist_reason;size:50" json:"sharing_whitelist_reason"`

	// Wallet Balance
	Balance float64 `gorm:"column:balance;type:decimal(15,2);default:0" json:"balance"`

	// Deletion tracking
	DeletedByID   *uint  `gorm:"column:deleted_by_id" json:"deleted_by_id"`
	DeletedByName string `gorm:"column:deleted_by_name;size:100" json:"deleted_by_name"`

	// Predictive FUP & Churn
	PredictedFupDate *time.Time `gorm:"column:predicted_fup_date" json:"predicted_fup_date"`
	UsageVelocity    float64    `gorm:"column:usage_velocity;default:0" json:"usage_velocity"`
	ChurnRisk        string     `gorm:"column:churn_risk;size:20;default:low" json:"churn_risk"`
	ChurnScore       int        `gorm:"column:churn_score;default:0" json:"churn_score"`

	// Timestamps
	CreatedAt       time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"column:deleted_at;index" json:"deleted_at"`
}

// Switch represents a network switch/location
type Switch struct {
	ID        uint           `gorm:"column:id;primaryKey" json:"id"`
	Name      string         `gorm:"column:name;size:100;not null" json:"name"`
	Location  string         `gorm:"column:location;size:255" json:"location"`
	ParentID  *uint          `gorm:"column:parent_id" json:"parent_id"`
	Parent    *Switch        `gorm:"foreignKey:ParentID;references:ID" json:"parent,omitempty"`
	CreatedAt time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// SubscriberBandwidthRuleType represents the type of bandwidth rule
type SubscriberBandwidthRuleType string

const (
	BandwidthRuleTypeInternet SubscriberBandwidthRuleType = "internet"
	BandwidthRuleTypeCDN      SubscriberBandwidthRuleType = "cdn"
)

// SubscriberBandwidthRule represents a per-subscriber bandwidth override rule
type SubscriberBandwidthRule struct {
	ID            uint                        `gorm:"column:id;primaryKey" json:"id"`
	SubscriberID  uint                        `gorm:"column:subscriber_id;not null;index" json:"subscriber_id"`
	Subscriber    *Subscriber                 `gorm:"foreignKey:SubscriberID;references:ID" json:"-"`
	RuleType      SubscriberBandwidthRuleType `gorm:"column:rule_type;size:20;not null" json:"rule_type"`
	Enabled       bool                        `gorm:"column:enabled;default:true" json:"enabled"`
	DownloadSpeed int                         `gorm:"column:download_speed;default:0" json:"download_speed"`
	UploadSpeed   int                         `gorm:"column:upload_speed;default:0" json:"upload_speed"`
	CDNID         uint                        `gorm:"column:cdn_id;default:0" json:"cdn_id"`
	CDNName       string                      `gorm:"column:cdn_name;size:100" json:"cdn_name"`
	Duration      string                      `gorm:"column:duration;size:20" json:"duration"`
	ExpiresAt     *time.Time                  `gorm:"column:expires_at" json:"expires_at"`
	Priority      int                         `gorm:"column:priority;default:0" json:"priority"`
	CreatedAt     time.Time                   `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time                   `gorm:"column:updated_at" json:"updated_at"`
}

func (SubscriberBandwidthRule) TableName() string {
	return "subscriber_bandwidth_rules"
}

// IsActiveNow checks if the rule is currently active (not expired)
func (r *SubscriberBandwidthRule) IsActiveNow() bool {
	if !r.Enabled {
		return false
	}

	// Check expiration if set
	if r.ExpiresAt != nil && time.Now().After(*r.ExpiresAt) {
		return false
	}

	return true
}

// IsExpired checks if the rule has expired
func (r *SubscriberBandwidthRule) IsExpired() bool {
	if r.ExpiresAt == nil {
		return false // Permanent rule
	}
	return time.Now().After(*r.ExpiresAt)
}

// TimeRemaining returns the remaining time as a human-readable string
func (r *SubscriberBandwidthRule) TimeRemaining() string {
	if r.ExpiresAt == nil {
		return "Permanent"
	}
	remaining := time.Until(*r.ExpiresAt)
	if remaining <= 0 {
		return "Expired"
	}
	if remaining >= 24*time.Hour {
		days := int(remaining.Hours() / 24)
		return fmt.Sprintf("%dd %dh", days, int(remaining.Hours())%24)
	}
	return fmt.Sprintf("%dh %dm", int(remaining.Hours()), int(remaining.Minutes())%60)
}

func (Subscriber) TableName() string {
	return "subscribers"
}

func (Switch) TableName() string {
	return "switches"
}

// IsExpired returns true if the subscriber's subscription has expired
func (s *Subscriber) IsExpired() bool {
	return time.Now().After(s.ExpiryDate)
}

// DaysRemaining returns the number of days remaining in subscription
func (s *Subscriber) DaysRemaining() int {
	if s.IsExpired() {
		return 0
	}
	return int(time.Until(s.ExpiryDate).Hours() / 24)
}
