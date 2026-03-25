package models

import (
	"time"

	"gorm.io/gorm"
)

// NotificationBanner represents an in-app announcement banner
type NotificationBanner struct {
	ID            uint           `gorm:"column:id;primaryKey" json:"id"`
	Title         string         `gorm:"column:title;size:255;not null" json:"title"`
	Message       string         `gorm:"column:message;type:text;not null" json:"message"`
	BannerType    string         `gorm:"column:banner_type;size:20;default:'info'" json:"banner_type"`
	Target        string         `gorm:"column:target;size:20;default:'all'" json:"target"`
	TargetIDs     string         `gorm:"column:target_ids;type:text;default:''" json:"target_ids"`
	StartDate     time.Time      `gorm:"column:start_date;not null" json:"start_date"`
	EndDate       time.Time      `gorm:"column:end_date;not null" json:"end_date"`
	Dismissible   bool           `gorm:"column:dismissible;default:true" json:"dismissible"`
	Enabled       bool           `gorm:"column:enabled;default:true" json:"enabled"`
	CreatedByID   uint           `gorm:"column:created_by_id" json:"created_by_id"`
	CreatedByName string         `gorm:"column:created_by_name;size:100" json:"created_by_name"`
	ResellerID    uint           `gorm:"column:reseller_id;default:0" json:"reseller_id"`
	CreatedAt     time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"column:deleted_at;index" json:"deleted_at,omitempty"`
}

func (NotificationBanner) TableName() string {
	return "notification_banners"
}
