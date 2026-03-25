package models

import "time"

// APIKey represents an API key for external API access
type APIKey struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	UserID     uint       `gorm:"column:user_id;not null" json:"user_id"`
	Name       string     `gorm:"column:name;size:100;not null;default:''" json:"name"`
	KeyPrefix  string     `gorm:"column:key_prefix;size:8;not null;default:''" json:"key_prefix"`
	KeyHash    string     `gorm:"column:key_hash;size:128;not null;default:''" json:"-"`
	Scopes     string     `gorm:"column:scopes;not null;default:'read'" json:"scopes"`
	IsActive   bool       `gorm:"column:is_active;default:true" json:"is_active"`
	LastUsedAt *time.Time `gorm:"column:last_used_at" json:"last_used_at"`
	ExpiresAt  *time.Time `gorm:"column:expires_at" json:"expires_at"`
	CreatedAt  time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (APIKey) TableName() string {
	return "api_keys"
}

// APIKeyLog represents a log entry for API key usage
type APIKeyLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	APIKeyID   uint      `gorm:"column:api_key_id;not null" json:"api_key_id"`
	Method     string    `gorm:"column:method;size:10;not null;default:''" json:"method"`
	Path       string    `gorm:"column:path;size:255;not null;default:''" json:"path"`
	StatusCode int       `gorm:"column:status_code;default:0" json:"status_code"`
	IPAddress  string    `gorm:"column:ip_address;size:50;default:''" json:"ip_address"`
	DurationMs int       `gorm:"column:duration_ms;default:0" json:"duration_ms"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

func (APIKeyLog) TableName() string {
	return "api_key_logs"
}
