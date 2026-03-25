package models

import "time"

type ResellerBranding struct {
	ID           uint      `gorm:"column:id;primaryKey" json:"id"`
	ResellerID   uint      `gorm:"column:reseller_id;not null;uniqueIndex" json:"reseller_id"`
	CompanyName  string    `gorm:"column:company_name;size:255" json:"company_name"`
	LogoPath     string    `gorm:"column:logo_path;size:500" json:"logo_path"`
	PrimaryColor string    `gorm:"column:primary_color;size:20" json:"primary_color"`
	FooterText   string    `gorm:"column:footer_text;size:500" json:"footer_text"`
	Tagline      string    `gorm:"column:tagline;size:500" json:"tagline"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}
