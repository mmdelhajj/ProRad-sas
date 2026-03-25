package models

import (
	"time"

	"gorm.io/gorm"
)

// CDN represents a Content Delivery Network entry
type CDN struct {
	ID          uint           `gorm:"column:id;primaryKey" json:"id"`
	Name        string         `gorm:"column:name;size:100;not null;uniqueIndex" json:"name"`
	Description string         `gorm:"column:description;size:255" json:"description"`
	Subnets     string         `gorm:"column:subnets;type:text" json:"subnets"` // Comma-separated subnets e.g., "185.82.96.0/24, 185.82.97.0/24"
	Color       string         `gorm:"column:color;size:20;default:#EF4444" json:"color"` // Hex color for live graph (default red)
	NASIDs      string         `gorm:"column:nas_ids;size:500" json:"nas_ids"` // Comma-separated NAS IDs to sync to (empty = all NAS)
	IsActive    bool           `gorm:"column:is_active;default:true" json:"is_active"`
	CreatedAt   time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

// ServiceCDN represents the many-to-many relationship between Service and CDN
// with additional fields for speed limit and bypass settings
type ServiceCDN struct {
	ID            uint      `gorm:"column:id;primaryKey" json:"id"`
	ServiceID     uint      `gorm:"column:service_id;uniqueIndex:idx_service_cdn;not null" json:"service_id"`
	Service       *Service  `gorm:"foreignKey:ServiceID;references:ID" json:"service,omitempty"`
	CDNID         uint      `gorm:"column:cdn_id;uniqueIndex:idx_service_cdn;not null" json:"cdn_id"`
	CDN           *CDN      `gorm:"foreignKey:CDNID;references:ID" json:"cdn,omitempty"`
	SpeedLimit    int64     `gorm:"column:speed_limit;default:0" json:"speed_limit"`    // Speed limit in Mbps for this CDN (0 = no limit)
	BypassQuota   bool      `gorm:"column:bypass_quota;default:false" json:"bypass_quota"` // If true, traffic to this CDN doesn't count against quota
	PCQEnabled    bool      `gorm:"column:pcq_enabled;default:false" json:"pcq_enabled"`  // If true, use PCQ queue instead of per-customer queues
	PCQLimit      int       `gorm:"column:pcq_limit;default:50" json:"pcq_limit"`       // PCQ limit per connection in KiB (default 50)
	PCQTotalLimit int       `gorm:"column:pcq_total_limit;default:2000" json:"pcq_total_limit"` // PCQ total limit in KiB (default 2000)
	PCQNASID      *uint     `gorm:"column:pcq_nas_id;default:null" json:"pcq_nas_id"`    // NAS to apply PCQ rules to
	PCQTargetPools string   `gorm:"column:pcq_target_pools;size:500" json:"pcq_target_pools"`  // Comma-separated pool names or CIDRs for PCQ target
	IsActive      bool      `gorm:"column:is_active;default:true" json:"is_active"`
	// Time-based speed control for CDN (like night-time boost)
	TimeBasedSpeedEnabled bool `gorm:"column:time_based_speed_enabled;default:false" json:"time_based_speed_enabled"` // on/off toggle
	TimeFromHour   int  `gorm:"column:time_from_hour;default:0" json:"time_from_hour"`   // Start hour (0-23)
	TimeFromMinute int  `gorm:"column:time_from_minute;default:0" json:"time_from_minute"` // Start minute (0-59)
	TimeToHour     int  `gorm:"column:time_to_hour;default:0" json:"time_to_hour"`     // End hour (0-23)
	TimeToMinute   int  `gorm:"column:time_to_minute;default:0" json:"time_to_minute"`   // End minute (0-59)
	TimeSpeedRatio int  `gorm:"column:time_speed_ratio;default:100" json:"time_speed_ratio"` // Speed ratio in % (100=normal, 200=double, 50=half)
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (CDN) TableName() string {
	return "cdns"
}

func (ServiceCDN) TableName() string {
	return "service_cdns"
}

// CDNPortRule represents a port-based PCQ speed rule applied on MikroTik
type CDNPortRule struct {
	ID          uint           `gorm:"column:id;primaryKey" json:"id"`
	Name        string         `gorm:"column:name;size:100;not null" json:"name"`
	Port        string         `gorm:"column:port;size:50" json:"port"`                   // e.g. "8080" (empty for dscp direction)
	Direction   string         `gorm:"column:direction;size:10;default:'both'" json:"direction"` // src, dst, both, dscp
	DSCPValue   *int           `gorm:"column:dscp_value" json:"dscp_value"`               // DSCP value 0-63 (only for dscp direction)
	SpeedMbps   int64          `gorm:"column:speed_mbps;not null;default:5" json:"speed_mbps"` // Speed limit in Mbps
	NASID       *uint          `gorm:"column:nas_id" json:"nas_id"`
	IsActive    bool           `gorm:"column:is_active;default:true" json:"is_active"`
	ShowInGraph bool           `gorm:"column:show_in_graph;default:false" json:"show_in_graph"` // Show in subscriber live graph
	Color       string         `gorm:"column:color;size:20;default:#8B5CF6" json:"color"`       // Hex color for live graph (default purple)
	CreatedAt   time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

func (CDNPortRule) TableName() string {
	return "cdn_port_rules"
}
