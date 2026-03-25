package models

import (
	"time"

	"gorm.io/gorm"
)

// BackupSchedule represents a scheduled backup configuration
type BackupSchedule struct {
	ID          uint           `json:"id" gorm:"column:id;primaryKey"`
	Name        string         `json:"name" gorm:"column:name;size:100;not null"`
	IsEnabled   bool           `json:"is_enabled" gorm:"column:is_enabled;default:true"`
	BackupType  string         `json:"backup_type" gorm:"column:backup_type;size:20;default:full"` // full, data, config
	Frequency   string         `json:"frequency" gorm:"column:frequency;size:20;not null"`       // daily, weekly, monthly
	DayOfWeek   int            `json:"day_of_week" gorm:"column:day_of_week;default:0"`            // 0=Sunday, 1=Monday, etc. (for weekly)
	DayOfMonth  int            `json:"day_of_month" gorm:"column:day_of_month;default:1"`           // 1-28 (for monthly)
	TimeOfDay   string         `json:"time_of_day" gorm:"column:time_of_day;size:5;default:02:00"` // HH:MM format (24hr)
	Retention   int            `json:"retention" gorm:"column:retention;default:7"`              // Days to keep backups

	// Storage destinations
	StorageType string `json:"storage_type" gorm:"column:storage_type;size:20;default:local"` // local, ftp, both
	LocalPath   string `json:"local_path" gorm:"column:local_path;size:255"`

	// Cloud backup fields (ProxPanel Cloud)
	CloudEnabled bool `json:"cloud_enabled" gorm:"column:cloud_enabled;default:false"`

	// MikroTik config backup
	IncludeMikroTik bool `json:"include_mikrotik" gorm:"column:include_mikrotik;default:false"`

	// FTP settings
	FTPEnabled  bool   `json:"ftp_enabled" gorm:"column:ftp_enabled;default:false"`
	FTPHost     string `json:"ftp_host" gorm:"column:ftp_host;size:255"`
	FTPPort     int    `json:"ftp_port" gorm:"column:ftp_port;default:21"`
	FTPUsername string `json:"ftp_username" gorm:"column:ftp_username;size:100"`
	FTPPassword string `json:"ftp_password" gorm:"column:ftp_password;size:255"`
	FTPPath     string `json:"ftp_path" gorm:"column:ftp_path;size:255;default:/backups"`
	FTPPassive  bool   `json:"ftp_passive" gorm:"column:ftp_passive;default:true"`
	FTPTLS      bool   `json:"ftp_tls" gorm:"column:ftp_tls;default:false"`

	// Status tracking
	LastRunAt      *time.Time `json:"last_run_at" gorm:"column:last_run_at"`
	LastStatus     string     `json:"last_status" gorm:"column:last_status;size:20"`    // success, failed, running
	LastError      string     `json:"last_error" gorm:"column:last_error;size:500"`
	LastBackupFile string     `json:"last_backup_file" gorm:"column:last_backup_file;size:255"`
	NextRunAt      *time.Time `json:"next_run_at" gorm:"column:next_run_at"`

	CreatedAt time.Time      `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"column:deleted_at;index"`
}

// BackupLog represents a backup execution log
type BackupLog struct {
	ID             uint      `json:"id" gorm:"column:id;primaryKey"`
	ScheduleID     *uint     `json:"schedule_id" gorm:"column:schedule_id"`                           // null if manual backup
	ScheduleName   string    `json:"schedule_name" gorm:"column:schedule_name;size:100"`
	BackupType     string    `json:"backup_type" gorm:"column:backup_type;size:20"`
	Filename       string    `json:"filename" gorm:"column:filename;size:255"`
	FileSize       int64     `json:"file_size" gorm:"column:file_size"`
	StorageType    string    `json:"storage_type" gorm:"column:storage_type;size:20"`           // local, ftp
	StoragePath    string    `json:"storage_path" gorm:"column:storage_path;size:500"`
	Status         string    `json:"status" gorm:"column:status;size:20"`                 // success, failed
	ErrorMessage   string    `json:"error_message" gorm:"column:error_message;size:500"`
	Duration       int       `json:"duration" gorm:"column:duration"`                               // seconds
	StartedAt      time.Time `json:"started_at" gorm:"column:started_at"`
	CompletedAt    time.Time `json:"completed_at" gorm:"column:completed_at"`
	CreatedByID    *uint     `json:"created_by_id" gorm:"column:created_by_id"`
	CreatedByName  string    `json:"created_by_name" gorm:"column:created_by_name;size:100"`
}

// TableName specifies the table name for BackupSchedule
func (BackupSchedule) TableName() string {
	return "backup_schedules"
}

// TableName specifies the table name for BackupLog
func (BackupLog) TableName() string {
	return "backup_logs"
}
