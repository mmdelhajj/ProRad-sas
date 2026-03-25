package models

import (
	"time"

	"gorm.io/gorm"
)

// PublicIPPool represents a pool of public IP addresses (CIDR range)
type PublicIPPool struct {
	ID           uint           `gorm:"column:id;primaryKey" json:"id"`
	Name         string         `gorm:"column:name;size:100;not null" json:"name"`
	CIDR         string         `gorm:"column:cidr;size:50;not null" json:"cidr"`
	IPVersion    int            `gorm:"column:ip_version;not null;default:4" json:"ip_version"`
	Gateway      string         `gorm:"column:gateway;size:50" json:"gateway"`
	MonthlyPrice float64        `gorm:"column:monthly_price;type:decimal(10,2);default:0" json:"monthly_price"`
	Description  string         `gorm:"column:description;type:text" json:"description"`
	IsActive     bool           `gorm:"column:is_active;default:true" json:"is_active"`
	TotalIPs     int            `gorm:"column:total_ips;default:0" json:"total_ips"`
	UsedIPs      int            `gorm:"column:used_ips;default:0" json:"used_ips"`
	CreatedAt    time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at;index" json:"deleted_at"`
}

func (PublicIPPool) TableName() string {
	return "public_ip_pools"
}

// PublicIPAssignmentStatus represents assignment status
type PublicIPAssignmentStatus string

const (
	PublicIPStatusActive    PublicIPAssignmentStatus = "active"
	PublicIPStatusReleased  PublicIPAssignmentStatus = "released"
	PublicIPStatusSuspended PublicIPAssignmentStatus = "suspended"
	PublicIPStatusReserved  PublicIPAssignmentStatus = "reserved"
)

// PublicIPAssignment represents an IP assigned to a subscriber or bandwidth customer
type PublicIPAssignment struct {
	ID                  uint                    `gorm:"column:id;primaryKey" json:"id"`
	PoolID              uint                    `gorm:"column:pool_id;index" json:"pool_id"`
	Pool                *PublicIPPool           `gorm:"foreignKey:PoolID;references:ID" json:"pool,omitempty"`
	SubscriberID        *uint                   `gorm:"column:subscriber_id;index" json:"subscriber_id"`
	Subscriber          *Subscriber             `gorm:"foreignKey:SubscriberID;references:ID" json:"subscriber,omitempty"`
	BandwidthCustomerID *uint                   `gorm:"column:bandwidth_customer_id;index" json:"bandwidth_customer_id"`
	BandwidthCustomer   *BandwidthCustomer      `gorm:"foreignKey:BandwidthCustomerID;references:ID" json:"bandwidth_customer,omitempty"`
	IPAddress    string                  `gorm:"column:ip_address;size:50;not null" json:"ip_address"`
	IPVersion    int                     `gorm:"column:ip_version;not null;default:4" json:"ip_version"`
	Status       PublicIPAssignmentStatus `gorm:"column:status;size:20;default:active" json:"status"`
	AssignedAt   time.Time               `gorm:"column:assigned_at" json:"assigned_at"`
	ReleasedAt   *time.Time              `gorm:"column:released_at" json:"released_at"`
	LastBilledAt *time.Time              `gorm:"column:last_billed_at" json:"last_billed_at"`
	NextBillingAt *time.Time             `gorm:"column:next_billing_at" json:"next_billing_at"`
	MonthlyPrice float64                 `gorm:"column:monthly_price;type:decimal(10,2);default:0" json:"monthly_price"`
	Notes        string                  `gorm:"column:notes;type:text" json:"notes"`
	CreatedAt    time.Time               `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time               `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt    gorm.DeletedAt          `gorm:"column:deleted_at;index" json:"deleted_at"`
}

func (PublicIPAssignment) TableName() string {
	return "public_ip_assignments"
}
