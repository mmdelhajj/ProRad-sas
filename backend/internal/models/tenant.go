package models

import "time"

// Tenant represents an ISP customer in SaaS mode
type Tenant struct {
	ID        uint      `gorm:"column:id;primaryKey" json:"id"`
	Name      string    `gorm:"column:name;size:255;not null" json:"name"`
	Subdomain string    `gorm:"column:subdomain;size:100;uniqueIndex;not null" json:"subdomain"`
	CustomDomain string `gorm:"column:custom_domain;size:255" json:"custom_domain,omitempty"`
	SchemaName string   `gorm:"column:schema_name;size:100;uniqueIndex;not null" json:"schema_name"`
	Status     string   `gorm:"column:status;size:20;default:active" json:"status"`
	Plan       string   `gorm:"column:plan;size:50;default:free" json:"plan"`
	MaxSubscribers int  `gorm:"column:max_subscribers;default:50" json:"max_subscribers"`
	MaxRouters     int  `gorm:"column:max_routers;default:1" json:"max_routers"`

	// WireGuard
	WGServerPrivateKey string `gorm:"column:wg_server_private_key;type:text" json:"-"`
	WGServerPublicKey  string `gorm:"column:wg_server_public_key;type:text" json:"wg_server_public_key,omitempty"`
	WGClientPrivateKey string `gorm:"column:wg_client_private_key;type:text" json:"-"`
	WGClientPublicKey  string `gorm:"column:wg_client_public_key;type:text" json:"wg_client_public_key,omitempty"`
	WGSubnet           string `gorm:"column:wg_subnet;size:20" json:"wg_subnet,omitempty"`
	WGServerIP         string `gorm:"column:wg_server_ip;size:15" json:"wg_server_ip,omitempty"`
	WGClientIP         string `gorm:"column:wg_client_ip;size:15" json:"wg_client_ip,omitempty"`

	// MikroTik
	MikrotikAPIIP       string `gorm:"column:mikrotik_api_ip;size:15" json:"mikrotik_api_ip,omitempty"`
	MikrotikAPIPort     int    `gorm:"column:mikrotik_api_port;default:8728" json:"mikrotik_api_port"`
	MikrotikAPIUser     string `gorm:"column:mikrotik_api_user;size:100" json:"mikrotik_api_user,omitempty"`
	MikrotikAPIPassword string `gorm:"column:mikrotik_api_password;size:255" json:"-"`
	RadiusSecret        string `gorm:"column:radius_secret;size:100" json:"-"`

	// Admin user
	AdminUsername     string `gorm:"column:admin_username;size:100;not null" json:"admin_username"`
	AdminPasswordHash string `gorm:"column:admin_password_hash;size:255;not null" json:"-"`
	AdminEmail        string `gorm:"column:admin_email;size:255" json:"admin_email,omitempty"`

	// Billing
	StripeCustomerID       string     `gorm:"column:stripe_customer_id;size:255" json:"stripe_customer_id,omitempty"`
	TrialEndsAt            *time.Time `gorm:"column:trial_ends_at" json:"trial_ends_at,omitempty"`
	CurrentSubscriberCount int        `gorm:"column:current_subscriber_count;default:0" json:"current_subscriber_count"`

	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName overrides the table name to use admin schema
func (Tenant) TableName() string {
	return "admin.tenants"
}

// NasTenantMap maps NAS IP addresses to tenants for RADIUS routing
type NasTenantMap struct {
	NasIP    string  `gorm:"column:nas_ip;primaryKey;size:15" json:"nas_ip"`
	TenantID uint    `gorm:"column:tenant_id" json:"tenant_id"`
	WGSubnet string  `gorm:"column:wg_subnet;size:20" json:"wg_subnet,omitempty"`
	Tenant   *Tenant `gorm:"foreignKey:TenantID;references:ID" json:"tenant,omitempty"`
}

// TableName overrides the table name to use admin schema
func (NasTenantMap) TableName() string {
	return "admin.nas_tenant_map"
}

// SuperAdmin represents a super-admin user for the SaaS platform
type SuperAdmin struct {
	ID           uint      `gorm:"column:id;primaryKey" json:"id"`
	Username     string    `gorm:"column:username;size:100;uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"column:password_hash;size:255;not null" json:"-"`
	Email        string    `gorm:"column:email;size:255" json:"email,omitempty"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (SuperAdmin) TableName() string {
	return "admin.super_admins"
}
