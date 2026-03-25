package models

import (
	"time"

	"gorm.io/gorm"
)

// BandwidthCustomer represents an IP-based bandwidth customer (business/tower/hotspot)
type BandwidthCustomer struct {
	ID            uint   `gorm:"column:id;primaryKey" json:"id"`
	Name          string `gorm:"column:name;size:200;not null" json:"name"`
	ContactPerson string `gorm:"column:contact_person;size:200" json:"contact_person"`
	Phone         string `gorm:"column:phone;size:50" json:"phone"`
	Email         string `gorm:"column:email;size:200" json:"email"`
	Address       string `gorm:"column:address;type:text" json:"address"`
	Notes         string `gorm:"column:notes;type:text" json:"notes"`

	// Connection
	IPAddress  string `gorm:"column:ip_address;size:45;not null;index" json:"ip_address"`
	SubnetMask string `gorm:"column:subnet_mask;size:20;default:/24" json:"subnet_mask"`
	Gateway    string `gorm:"column:gateway;size:45" json:"gateway"`
	NasID      *uint  `gorm:"column:nas_id;index" json:"nas_id"`
	Nas        *Nas   `gorm:"foreignKey:NasID;references:ID" json:"nas,omitempty"`
	Interface  string `gorm:"column:interface;size:50" json:"interface"`
	VlanID     int    `gorm:"column:vlan_id;default:0" json:"vlan_id"`
	QueueName  string `gorm:"column:queue_name;size:100" json:"queue_name"`

	// Public IP
	PublicIP      string `gorm:"column:public_ip;size:500" json:"public_ip"` // comma-separated for multiple IPs
	PublicSubnet  string `gorm:"column:public_subnet;size:5" json:"public_subnet"`
	PublicGateway string `gorm:"column:public_gateway;size:45" json:"public_gateway"`

	// IP Block (IPAM)
	IPBlockID      *uint `gorm:"column:ip_block_id" json:"ip_block_id"`
	IPAllocationID *uint `gorm:"column:ip_allocation_id" json:"ip_allocation_id"`

	// Speed (in kb)
	DownloadSpeed    int    `gorm:"column:download_speed;not null;default:0" json:"download_speed"`
	UploadSpeed      int    `gorm:"column:upload_speed;not null;default:0" json:"upload_speed"`
	CDNDownloadSpeed int    `gorm:"column:cdn_download_speed;default:0" json:"cdn_download_speed"`
	CDNUploadSpeed   int    `gorm:"column:cdn_upload_speed;default:0" json:"cdn_upload_speed"`
	SpeedSource      string `gorm:"column:speed_source;size:20;default:queue" json:"speed_source"` // 'queue' or 'radius'

	// Burst
	BurstEnabled     bool `gorm:"column:burst_enabled;default:false" json:"burst_enabled"`
	BurstDownload    int  `gorm:"column:burst_download;default:0" json:"burst_download"`
	BurstUpload      int  `gorm:"column:burst_upload;default:0" json:"burst_upload"`
	BurstThresholdDl int  `gorm:"column:burst_threshold_dl;default:0" json:"burst_threshold_dl"`
	BurstThresholdUl int  `gorm:"column:burst_threshold_ul;default:0" json:"burst_threshold_ul"`
	BurstTime        int  `gorm:"column:burst_time;default:10" json:"burst_time"`

	// FUP
	FUPEnabled    bool  `gorm:"column:fup_enabled;default:false" json:"fup_enabled"`
	DailyQuota    int64 `gorm:"column:daily_quota;default:0" json:"daily_quota"`           // bytes
	FUP1Threshold int64 `gorm:"column:fup1_threshold;default:0" json:"fup1_threshold"`     // bytes
	FUP1Speed     int   `gorm:"column:fup1_speed;default:0" json:"fup1_speed"`             // kb
	FUP2Threshold int64 `gorm:"column:fup2_threshold;default:0" json:"fup2_threshold"`
	FUP2Speed     int   `gorm:"column:fup2_speed;default:0" json:"fup2_speed"`
	FUP3Threshold int64 `gorm:"column:fup3_threshold;default:0" json:"fup3_threshold"`
	FUP3Speed     int   `gorm:"column:fup3_speed;default:0" json:"fup3_speed"`

	// Monthly FUP
	MonthlyQuota          int64 `gorm:"column:monthly_quota;default:0" json:"monthly_quota"`
	MonthlyFUP1Threshold  int64 `gorm:"column:monthly_fup1_threshold;default:0" json:"monthly_fup1_threshold"`
	MonthlyFUP1Speed      int   `gorm:"column:monthly_fup1_speed;default:0" json:"monthly_fup1_speed"`
	MonthlyFUP2Threshold  int64 `gorm:"column:monthly_fup2_threshold;default:0" json:"monthly_fup2_threshold"`
	MonthlyFUP2Speed      int   `gorm:"column:monthly_fup2_speed;default:0" json:"monthly_fup2_speed"`
	MonthlyFUP3Threshold  int64 `gorm:"column:monthly_fup3_threshold;default:0" json:"monthly_fup3_threshold"`
	MonthlyFUP3Speed      int   `gorm:"column:monthly_fup3_speed;default:0" json:"monthly_fup3_speed"`

	// FUP State
	FUPLevel        int `gorm:"column:fup_level;default:0" json:"fup_level"`
	MonthlyFUPLevel int `gorm:"column:monthly_fup_level;default:0" json:"monthly_fup_level"`

	// Usage tracking
	DailyDownloadUsed   int64 `gorm:"column:daily_download_used;default:0" json:"daily_download_used"`
	DailyUploadUsed     int64 `gorm:"column:daily_upload_used;default:0" json:"daily_upload_used"`
	MonthlyDownloadUsed int64 `gorm:"column:monthly_download_used;default:0" json:"monthly_download_used"`
	MonthlyUploadUsed   int64 `gorm:"column:monthly_upload_used;default:0" json:"monthly_upload_used"`
	LastQueueBytesIn    int64 `gorm:"column:last_queue_bytes_in;default:0" json:"last_queue_bytes_in"`
	LastQueueBytesOut   int64 `gorm:"column:last_queue_bytes_out;default:0" json:"last_queue_bytes_out"`
	TotalDownload       int64 `gorm:"column:total_download;default:0" json:"total_download"`
	TotalUpload         int64 `gorm:"column:total_upload;default:0" json:"total_upload"`

	// Billing
	Price        float64    `gorm:"column:price;type:decimal(10,2);default:0" json:"price"`
	BillingCycle string     `gorm:"column:billing_cycle;size:20;default:monthly" json:"billing_cycle"`
	StartDate    *time.Time `gorm:"column:start_date;type:date" json:"start_date"`
	ExpiryDate   *time.Time `gorm:"column:expiry_date;type:date" json:"expiry_date"`
	AutoRenew    bool       `gorm:"column:auto_renew;default:true" json:"auto_renew"`

	// Status
	Status   string     `gorm:"column:status;size:20;default:active;index" json:"status"` // active, suspended, expired
	IsOnline bool       `gorm:"column:is_online;default:false;index" json:"is_online"`
	LastSeen *time.Time `gorm:"column:last_seen" json:"last_seen"`

	// Timestamps
	CreatedAt        time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt        time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"column:deleted_at;index" json:"deleted_at"`
	LastDailyReset   *time.Time     `gorm:"column:last_daily_reset" json:"last_daily_reset"`
	LastMonthlyReset *time.Time     `gorm:"column:last_monthly_reset" json:"last_monthly_reset"`
}

func (BandwidthCustomer) TableName() string {
	return "bandwidth_customers"
}

// BwDailyUsageHistory stores daily usage snapshots for bandwidth customers
type BwDailyUsageHistory struct {
	ID            uint      `gorm:"column:id;primaryKey" json:"id"`
	CustomerID    uint      `gorm:"column:customer_id;not null;index" json:"customer_id"`
	Date          time.Time `gorm:"column:date;type:date;not null" json:"date"`
	DownloadBytes int64     `gorm:"column:download_bytes;default:0" json:"download_bytes"`
	UploadBytes   int64     `gorm:"column:upload_bytes;default:0" json:"upload_bytes"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
}

func (BwDailyUsageHistory) TableName() string {
	return "bw_daily_usage_history"
}

// BwIPBlock represents a CIDR IP block for bandwidth customer IPAM
type BwIPBlock struct {
	ID          uint           `gorm:"column:id;primaryKey" json:"id"`
	Name        string         `gorm:"column:name;size:100;not null" json:"name"`
	CIDR        string         `gorm:"column:cidr;size:50;not null" json:"cidr"`
	Gateway     string         `gorm:"column:gateway;size:45" json:"gateway"`
	SubnetMask  string         `gorm:"column:subnet_mask;size:20" json:"subnet_mask"`
	Description string         `gorm:"column:description;type:text" json:"description"`
	TotalIPs    int            `gorm:"column:total_ips;default:0" json:"total_ips"`
	UsedIPs     int            `gorm:"column:used_ips;default:0" json:"used_ips"`
	IsActive    bool           `gorm:"column:is_active;default:true" json:"is_active"`
	CreatedAt   time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;index" json:"deleted_at"`
}

func (BwIPBlock) TableName() string {
	return "bw_ip_blocks"
}

// BwIPAllocation represents an individual IP from a block
type BwIPAllocation struct {
	ID         uint       `gorm:"column:id;primaryKey" json:"id"`
	BlockID    uint       `gorm:"column:block_id;not null;index" json:"block_id"`
	CustomerID *uint      `gorm:"column:customer_id;index" json:"customer_id"`
	IPAddress  string     `gorm:"column:ip_address;size:45;not null;uniqueIndex" json:"ip_address"`
	Status     string     `gorm:"column:status;size:20;default:available;index" json:"status"` // available, assigned, reserved, gateway
	Notes      string     `gorm:"column:notes;type:text" json:"notes"`
	AssignedAt *time.Time `gorm:"column:assigned_at" json:"assigned_at"`
	CreatedAt  time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (BwIPAllocation) TableName() string {
	return "bw_ip_allocations"
}
