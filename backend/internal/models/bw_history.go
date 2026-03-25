package models

import "time"

// BwHourlyUsage stores hourly bandwidth aggregation for bandwidth customers
type BwHourlyUsage struct {
	ID               uint      `gorm:"column:id;primaryKey" json:"id"`
	CustomerID       uint      `gorm:"column:customer_id;not null" json:"customer_id"`
	Hour             time.Time `gorm:"column:hour;not null" json:"hour"`
	DownloadBytes    int64     `gorm:"column:download_bytes;default:0" json:"download_bytes"`
	UploadBytes      int64     `gorm:"column:upload_bytes;default:0" json:"upload_bytes"`
	PeakDownloadKbps int64     `gorm:"column:peak_download_kbps;default:0" json:"peak_download_kbps"`
	PeakUploadKbps   int64     `gorm:"column:peak_upload_kbps;default:0" json:"peak_upload_kbps"`
	SampleCount      int       `gorm:"column:sample_count;default:0" json:"sample_count"`
	CreatedAt        time.Time `gorm:"column:created_at" json:"created_at"`
}

func (BwHourlyUsage) TableName() string {
	return "bw_hourly_usage"
}

// BwSession tracks online/offline session periods for bandwidth customers
type BwSession struct {
	ID            uint       `gorm:"column:id;primaryKey" json:"id"`
	CustomerID    uint       `gorm:"column:customer_id;not null;index" json:"customer_id"`
	StartedAt     time.Time  `gorm:"column:started_at;not null" json:"started_at"`
	EndedAt       *time.Time `gorm:"column:ended_at" json:"ended_at"`
	DurationSec   int        `gorm:"column:duration_sec;default:0" json:"duration_sec"`
	DownloadBytes int64      `gorm:"column:download_bytes;default:0" json:"download_bytes"`
	UploadBytes   int64      `gorm:"column:upload_bytes;default:0" json:"upload_bytes"`
}

func (BwSession) TableName() string {
	return "bw_sessions"
}
