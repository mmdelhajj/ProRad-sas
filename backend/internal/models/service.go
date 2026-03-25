package models

import (
	"time"

	"gorm.io/gorm"
)

// ExpiryUnit represents the unit for expiry calculation
type ExpiryUnit int

const (
	ExpiryUnitDays   ExpiryUnit = 1
	ExpiryUnitMonths ExpiryUnit = 2
)

// Service represents a service plan
type Service struct {
	ID              uint           `gorm:"column:id;primaryKey" json:"id"`
	Name            string         `gorm:"column:name;size:100;not null" json:"name"`
	CommercialName  string         `gorm:"column:commercial_name;size:100" json:"commercial_name"`
	Description     string         `gorm:"column:description;type:text" json:"description"`

	// Speed
	DownloadSpeed   int64  `gorm:"column:download_speed;not null" json:"download_speed"`      // bits per second
	UploadSpeed     int64  `gorm:"column:upload_speed;not null" json:"upload_speed"`        // bits per second
	DownloadSpeedStr string `gorm:"column:download_speed_str;size:50" json:"download_speed_str"`  // e.g., "10M"
	UploadSpeedStr  string `gorm:"column:upload_speed_str;size:50" json:"upload_speed_str"`     // e.g., "5M"

	// Burst
	BurstDownload   int64 `gorm:"column:burst_download;default:0" json:"burst_download"`
	BurstUpload     int64 `gorm:"column:burst_upload;default:0" json:"burst_upload"`
	BurstThreshold  int64 `gorm:"column:burst_threshold;default:0" json:"burst_threshold"`
	BurstTime       int   `gorm:"column:burst_time;default:0" json:"burst_time"`

	// Quota
	DailyQuota      int64 `gorm:"column:daily_quota;default:0" json:"daily_quota"`         // bytes, 0 = unlimited
	MonthlyQuota    int64 `gorm:"column:monthly_quota;default:0" json:"monthly_quota"`       // bytes, 0 = unlimited
	TimeQuota       int   `gorm:"column:time_quota;default:0" json:"time_quota"`          // minutes per day

	// Daily FUP (Fair Usage Policy) - resets every day at midnight
	// FUP1 - First daily threshold
	FUP1Threshold      int64 `gorm:"column:fup1_threshold;default:0" json:"fup1_threshold"`        // bytes, 0 = disabled
	FUP1DownloadSpeed  int64 `gorm:"column:fup1_download_speed;default:0" json:"fup1_download_speed"`   // Kbps (e.g., 700 = 700k)
	FUP1UploadSpeed    int64 `gorm:"column:fup1_upload_speed;default:0" json:"fup1_upload_speed"`     // Kbps

	// FUP2 - Second daily threshold
	FUP2Threshold      int64 `gorm:"column:fup2_threshold;default:0" json:"fup2_threshold"`        // bytes, 0 = disabled
	FUP2DownloadSpeed  int64 `gorm:"column:fup2_download_speed;default:0" json:"fup2_download_speed"`   // Kbps
	FUP2UploadSpeed    int64 `gorm:"column:fup2_upload_speed;default:0" json:"fup2_upload_speed"`     // Kbps

	// FUP3 - Third daily threshold
	FUP3Threshold      int64 `gorm:"column:fup3_threshold;default:0" json:"fup3_threshold"`        // bytes, 0 = disabled
	FUP3DownloadSpeed  int64 `gorm:"column:fup3_download_speed;default:0" json:"fup3_download_speed"`   // Kbps
	FUP3UploadSpeed    int64 `gorm:"column:fup3_upload_speed;default:0" json:"fup3_upload_speed"`     // Kbps

	// FUP4 - Fourth daily threshold
	FUP4Threshold      int64 `gorm:"column:fup4_threshold;default:0" json:"fup4_threshold"`
	FUP4DownloadSpeed  int64 `gorm:"column:fup4_download_speed;default:0" json:"fup4_download_speed"`
	FUP4UploadSpeed    int64 `gorm:"column:fup4_upload_speed;default:0" json:"fup4_upload_speed"`

	// FUP5 - Fifth daily threshold
	FUP5Threshold      int64 `gorm:"column:fup5_threshold;default:0" json:"fup5_threshold"`
	FUP5DownloadSpeed  int64 `gorm:"column:fup5_download_speed;default:0" json:"fup5_download_speed"`
	FUP5UploadSpeed    int64 `gorm:"column:fup5_upload_speed;default:0" json:"fup5_upload_speed"`

	// FUP6 - Sixth daily threshold
	FUP6Threshold      int64 `gorm:"column:fup6_threshold;default:0" json:"fup6_threshold"`
	FUP6DownloadSpeed  int64 `gorm:"column:fup6_download_speed;default:0" json:"fup6_download_speed"`
	FUP6UploadSpeed    int64 `gorm:"column:fup6_upload_speed;default:0" json:"fup6_upload_speed"`

	// Monthly FUP - resets when user renews subscription
	// MonthlyFUP1 - First monthly threshold
	MonthlyFUP1Threshold     int64 `gorm:"column:monthly_fup1_threshold;default:0" json:"monthly_fup1_threshold"`      // bytes, 0 = disabled
	MonthlyFUP1DownloadSpeed int64 `gorm:"column:monthly_fup1_download_speed;default:0" json:"monthly_fup1_download_speed"` // Kbps
	MonthlyFUP1UploadSpeed   int64 `gorm:"column:monthly_fup1_upload_speed;default:0" json:"monthly_fup1_upload_speed"`   // Kbps

	// MonthlyFUP2 - Second monthly threshold
	MonthlyFUP2Threshold     int64 `gorm:"column:monthly_fup2_threshold;default:0" json:"monthly_fup2_threshold"`      // bytes, 0 = disabled
	MonthlyFUP2DownloadSpeed int64 `gorm:"column:monthly_fup2_download_speed;default:0" json:"monthly_fup2_download_speed"` // Kbps
	MonthlyFUP2UploadSpeed   int64 `gorm:"column:monthly_fup2_upload_speed;default:0" json:"monthly_fup2_upload_speed"`   // Kbps

	// MonthlyFUP3 - Third monthly threshold
	MonthlyFUP3Threshold     int64 `gorm:"column:monthly_fup3_threshold;default:0" json:"monthly_fup3_threshold"`      // bytes, 0 = disabled
	MonthlyFUP3DownloadSpeed int64 `gorm:"column:monthly_fup3_download_speed;default:0" json:"monthly_fup3_download_speed"` // Kbps
	MonthlyFUP3UploadSpeed   int64 `gorm:"column:monthly_fup3_upload_speed;default:0" json:"monthly_fup3_upload_speed"`   // Kbps

	// MonthlyFUP4 - Fourth monthly threshold
	MonthlyFUP4Threshold     int64 `gorm:"column:monthly_fup4_threshold;default:0" json:"monthly_fup4_threshold"`
	MonthlyFUP4DownloadSpeed int64 `gorm:"column:monthly_fup4_download_speed;default:0" json:"monthly_fup4_download_speed"`
	MonthlyFUP4UploadSpeed   int64 `gorm:"column:monthly_fup4_upload_speed;default:0" json:"monthly_fup4_upload_speed"`

	// MonthlyFUP5 - Fifth monthly threshold
	MonthlyFUP5Threshold     int64 `gorm:"column:monthly_fup5_threshold;default:0" json:"monthly_fup5_threshold"`
	MonthlyFUP5DownloadSpeed int64 `gorm:"column:monthly_fup5_download_speed;default:0" json:"monthly_fup5_download_speed"`
	MonthlyFUP5UploadSpeed   int64 `gorm:"column:monthly_fup5_upload_speed;default:0" json:"monthly_fup5_upload_speed"`

	// MonthlyFUP6 - Sixth monthly threshold
	MonthlyFUP6Threshold     int64 `gorm:"column:monthly_fup6_threshold;default:0" json:"monthly_fup6_threshold"`
	MonthlyFUP6DownloadSpeed int64 `gorm:"column:monthly_fup6_download_speed;default:0" json:"monthly_fup6_download_speed"`
	MonthlyFUP6UploadSpeed   int64 `gorm:"column:monthly_fup6_upload_speed;default:0" json:"monthly_fup6_upload_speed"`

	// CDN FUP (Fair Usage Policy) - limits CDN-only traffic speed
	CDNFUPEnabled          bool  `gorm:"column:cdn_fup_enabled;default:false" json:"cdn_fup_enabled"`
	// CDN Daily FUP
	CDNFUP1Threshold       int64 `gorm:"column:cdn_fup1_threshold;default:0" json:"cdn_fup1_threshold"`
	CDNFUP1DownloadSpeed   int64 `gorm:"column:cdn_fup1_download_speed;default:0" json:"cdn_fup1_download_speed"`
	CDNFUP1UploadSpeed     int64 `gorm:"column:cdn_fup1_upload_speed;default:0" json:"cdn_fup1_upload_speed"`
	CDNFUP2Threshold       int64 `gorm:"column:cdn_fup2_threshold;default:0" json:"cdn_fup2_threshold"`
	CDNFUP2DownloadSpeed   int64 `gorm:"column:cdn_fup2_download_speed;default:0" json:"cdn_fup2_download_speed"`
	CDNFUP2UploadSpeed     int64 `gorm:"column:cdn_fup2_upload_speed;default:0" json:"cdn_fup2_upload_speed"`
	CDNFUP3Threshold       int64 `gorm:"column:cdn_fup3_threshold;default:0" json:"cdn_fup3_threshold"`
	CDNFUP3DownloadSpeed   int64 `gorm:"column:cdn_fup3_download_speed;default:0" json:"cdn_fup3_download_speed"`
	CDNFUP3UploadSpeed     int64 `gorm:"column:cdn_fup3_upload_speed;default:0" json:"cdn_fup3_upload_speed"`
	// CDN Monthly FUP
	CDNMonthlyFUP1Threshold     int64 `gorm:"column:cdn_monthly_fup1_threshold;default:0" json:"cdn_monthly_fup1_threshold"`
	CDNMonthlyFUP1DownloadSpeed int64 `gorm:"column:cdn_monthly_fup1_download_speed;default:0" json:"cdn_monthly_fup1_download_speed"`
	CDNMonthlyFUP1UploadSpeed   int64 `gorm:"column:cdn_monthly_fup1_upload_speed;default:0" json:"cdn_monthly_fup1_upload_speed"`
	CDNMonthlyFUP2Threshold     int64 `gorm:"column:cdn_monthly_fup2_threshold;default:0" json:"cdn_monthly_fup2_threshold"`
	CDNMonthlyFUP2DownloadSpeed int64 `gorm:"column:cdn_monthly_fup2_download_speed;default:0" json:"cdn_monthly_fup2_download_speed"`
	CDNMonthlyFUP2UploadSpeed   int64 `gorm:"column:cdn_monthly_fup2_upload_speed;default:0" json:"cdn_monthly_fup2_upload_speed"`
	CDNMonthlyFUP3Threshold     int64 `gorm:"column:cdn_monthly_fup3_threshold;default:0" json:"cdn_monthly_fup3_threshold"`
	CDNMonthlyFUP3DownloadSpeed int64 `gorm:"column:cdn_monthly_fup3_download_speed;default:0" json:"cdn_monthly_fup3_download_speed"`
	CDNMonthlyFUP3UploadSpeed   int64 `gorm:"column:cdn_monthly_fup3_upload_speed;default:0" json:"cdn_monthly_fup3_upload_speed"`

	// Pricing
	Price           float64 `gorm:"column:price;type:decimal(15,2);not null" json:"price"`
	DayPrice        float64 `gorm:"column:day_price;type:decimal(15,2);default:0" json:"day_price"`
	ResetPrice      float64 `gorm:"column:reset_price;type:decimal(15,2);default:0" json:"reset_price"`

	// Expiry
	ExpiryValue     int        `gorm:"column:expiry_value;default:30" json:"expiry_value"`
	ExpiryUnit      ExpiryUnit `gorm:"column:expiry_unit;default:1" json:"expiry_unit"`
	EntireMonth     bool       `gorm:"column:entire_month;default:false" json:"entire_month"`
	MonthlyAccount  bool       `gorm:"column:monthly_account;default:false" json:"monthly_account"`

	// Time-based speed control (automatic speed change during specified hours)
	TimeBasedSpeedEnabled bool `gorm:"column:time_based_speed_enabled;default:false" json:"time_based_speed_enabled"` // on/off toggle
	TimeFromHour          int  `gorm:"column:time_from_hour;default:0" json:"time_from_hour"`                         // 0-23
	TimeFromMinute        int  `gorm:"column:time_from_minute;default:0" json:"time_from_minute"`                     // 0-59
	TimeToHour            int  `gorm:"column:time_to_hour;default:0" json:"time_to_hour"`                             // 0-23
	TimeToMinute          int  `gorm:"column:time_to_minute;default:0" json:"time_to_minute"`                         // 0-59
	TimeDownloadRatio     int  `gorm:"column:time_download_ratio;default:100" json:"time_download_ratio"`             // percentage (100 = full, 200 = double)
	TimeUploadRatio       int  `gorm:"column:time_upload_ratio;default:100" json:"time_upload_ratio"`                 // percentage (100 = full, 200 = double)

	// Mikrotik
	NasID           *uint  `gorm:"column:nas_id" json:"nas_id"`
	PoolName        string `gorm:"column:pool_name;size:100" json:"pool_name"`
	AddressListIn   string `gorm:"column:address_list_in;size:100" json:"address_list_in"`
	AddressListOut  string `gorm:"column:address_list_out;size:100" json:"address_list_out"`
	QueueType       string `gorm:"column:queue_type;size:50;default:simple" json:"queue_type"`

	// Status
	IsActive        bool      `gorm:"column:is_active;default:true" json:"is_active"`
	SortOrder       int       `gorm:"column:sort_order;default:0" json:"sort_order"`

	// Timestamps
	CreatedAt       time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// ResellerService represents pricing for reseller
type ResellerService struct {
	ID         uint      `gorm:"column:id;primaryKey" json:"id"`
	ResellerID uint      `gorm:"column:reseller_id;uniqueIndex:idx_reseller_service;not null" json:"reseller_id"`
	Reseller   Reseller  `gorm:"-" json:"reseller"`
	ServiceID  uint      `gorm:"column:service_id;uniqueIndex:idx_reseller_service;not null" json:"service_id"`
	Service    Service   `gorm:"-" json:"service"`
	Price      float64   `gorm:"column:price;type:decimal(15,2);not null" json:"price"`
	DayPrice   float64   `gorm:"column:day_price;type:decimal(15,2);default:0" json:"day_price"`
	IsEnabled  bool      `gorm:"column:is_enabled;default:true" json:"is_enabled"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Service) TableName() string {
	return "services"
}

func (ResellerService) TableName() string {
	return "reseller_services"
}

