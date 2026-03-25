package models

import (
	"time"

	"gorm.io/gorm"
)

// NasType represents the type of NAS device
type NasType string

const (
	NasTypeMikrotik  NasType = "mikrotik"
	NasTypeCisco     NasType = "cisco"
	NasTypeHuawei    NasType = "huawei"
	NasTypeUbiquiti  NasType = "ubiquiti"
	NasTypeOther     NasType = "other"
)

// Nas represents a NAS/Router device
type Nas struct {
	ID              uint           `gorm:"column:id;primaryKey" json:"id"`
	Name            string         `gorm:"column:name;size:100;not null" json:"name"`
	ShortName       string         `gorm:"column:short_name;size:50;uniqueIndex" json:"short_name"`
	IPAddress       string         `gorm:"column:ip_address;size:50;not null;uniqueIndex" json:"ip_address"`
	Type            NasType        `gorm:"column:type;size:50;default:mikrotik" json:"type"`
	Description     string         `gorm:"column:description;size:255" json:"description"`

	// RADIUS
	Secret          string         `gorm:"column:secret;size:100;not null" json:"-"` // Hidden from API responses for security
	HasSecret       bool           `gorm:"-" json:"has_secret"`                       // Computed field to indicate if secret is set
	AuthPort        int            `gorm:"column:auth_port;default:1812" json:"auth_port"`
	AcctPort        int            `gorm:"column:acct_port;default:1813" json:"acct_port"`
	CoAPort         int            `gorm:"column:coa_port;default:1700" json:"coa_port"`

	// Mikrotik API
	APIUsername     string         `gorm:"column:api_username;size:100" json:"api_username"`
	APIPassword     string         `gorm:"column:api_password;size:255" json:"-"` // Hidden from API responses for security
	HasAPIPassword  bool           `gorm:"-" json:"has_api_password"`             // Computed field to indicate if API password is set
	APIPort         int            `gorm:"column:api_port;default:8728" json:"api_port"`
	APISSLPort      int            `gorm:"column:api_ssl_port;default:8729" json:"api_ssl_port"`
	UseSSL          bool           `gorm:"column:use_ssl;default:false" json:"use_ssl"`
	FTPPort         int            `gorm:"column:ftp_port;default:21" json:"ftp_port"`

	// PCQ/CDN Settings
	SubscriberPools    string `gorm:"column:subscriber_pools;size:500" json:"subscriber_pools"` // Comma-separated pool names or CIDRs for PCQ target
	CDNTorchInterface  string `gorm:"column:cdn_torch_interface;size:100" json:"cdn_torch_interface"` // Bridge/interface for batch CDN torch (e.g., "bridge1")

	// Realm Settings (for RADIUS authentication)
	AllowedRealms   string         `gorm:"column:allowed_realms;size:500" json:"allowed_realms"` // Comma-separated list of allowed realms (e.g., "test.mes.net.lb,other.domain.com")

	// Status
	IsActive        bool           `gorm:"column:is_active;default:true" json:"is_active"`
	IsOnline        bool           `gorm:"column:is_online;default:false" json:"is_online"`
	LastSeen        *time.Time     `gorm:"column:last_seen" json:"last_seen"`
	Version         string         `gorm:"column:version;size:50" json:"version"`

	// Stats
	ActiveSessions  int            `gorm:"column:active_sessions;default:0" json:"active_sessions"`
	TotalUsers      int            `gorm:"column:total_users;default:0" json:"total_users"`

	// Timestamps
	CreatedAt       time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

func (Nas) TableName() string {
	return "nas_devices"
}

// GetSecretForRADIUS returns the RADIUS shared secret
func (n *Nas) GetSecretForRADIUS() []byte {
	return []byte(n.Secret)
}
