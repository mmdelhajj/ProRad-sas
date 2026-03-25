package models

import (
	"time"
)

// IPPoolStatus represents the status of an IP address in the pool
type IPPoolStatus string

const (
	IPPoolStatusAvailable IPPoolStatus = "available"
	IPPoolStatusInUse     IPPoolStatus = "in_use"
	IPPoolStatusReserved  IPPoolStatus = "reserved"
)

// IPPoolAssignment represents a single IP address assignment in the pool
type IPPoolAssignment struct {
	ID           uint         `gorm:"column:id;primaryKey" json:"id"`
	IPAddress    string       `gorm:"column:ip_address;size:15;not null;uniqueIndex" json:"ip_address"`
	PoolName     string       `gorm:"column:pool_name;size:64;not null;index" json:"pool_name"`
	Status       IPPoolStatus `gorm:"column:status;size:20;not null;default:available;index" json:"status"`
	Username     string       `gorm:"column:username;size:100;index" json:"username"`
	SubscriberID *uint        `gorm:"column:subscriber_id" json:"subscriber_id"`
	NasID        *uint        `gorm:"column:nas_id" json:"nas_id"`
	SessionID    string       `gorm:"column:session_id;size:100" json:"session_id"`
	AssignedAt   *time.Time   `gorm:"column:assigned_at" json:"assigned_at"`
	ReleasedAt   *time.Time   `gorm:"column:released_at" json:"released_at"`
	CreatedAt    time.Time    `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time    `gorm:"column:updated_at" json:"updated_at"`
}

func (IPPoolAssignment) TableName() string {
	return "ip_pool_assignments"
}

// IsAvailable returns true if the IP is available for assignment
func (ip *IPPoolAssignment) IsAvailable() bool {
	return ip.Status == IPPoolStatusAvailable
}

// IsInUse returns true if the IP is currently assigned to a user
func (ip *IPPoolAssignment) IsInUse() bool {
	return ip.Status == IPPoolStatusInUse
}
