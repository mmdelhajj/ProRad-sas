package models

import (
	"time"

	"gorm.io/gorm"
)

// UpdateNotification represents a notification sent to customers about updates
type UpdateNotification struct {
	ID              uint           `gorm:"column:id;primaryKey" json:"id"`
	UpdateID        uint           `gorm:"column:update_id;not null" json:"update_id"`
	LicenseID       uint           `gorm:"column:license_id;not null" json:"license_id"`
	CustomerID      uint           `gorm:"column:customer_id;not null" json:"customer_id"`
	NotificationType string        `gorm:"column:notification_type;size:20;not null" json:"notification_type"` // email, sms, in-app
	Status          string         `gorm:"column:status;size:20;not null;default:'pending'" json:"status"` // pending, sent, failed, read
	SentAt          *time.Time     `gorm:"column:sent_at" json:"sent_at"`
	ReadAt          *time.Time     `gorm:"column:read_at" json:"read_at"`
	ErrorMessage    string         `gorm:"column:error_message;type:text" json:"error_message"`
	CreatedAt       time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"column:deleted_at;index" json:"deleted_at,omitempty"`
}

// TableName specifies the table name for UpdateNotification
func (UpdateNotification) TableName() string {
	return "update_notifications"
}

// PendingNotification represents an in-app notification for customer dashboard
type PendingNotification struct {
	ID          uint       `json:"id"`
	UpdateID    uint       `json:"update_id"`
	Version     string     `json:"version"`
	Priority    string     `json:"priority"` // critical, important, info
	Title       string     `json:"title"`
	Message     string     `json:"message"`
	ReleaseDate time.Time  `json:"release_date"`
	SentAt      time.Time  `json:"sent_at"`
	ReadAt      *time.Time `json:"read_at"`
}
