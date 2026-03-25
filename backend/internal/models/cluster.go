package models

import (
	"time"

	"gorm.io/gorm"
)

// ClusterRole represents the role of a server in the cluster
type ClusterRole string

const (
	ClusterRoleStandalone ClusterRole = "standalone"
	ClusterRoleMain       ClusterRole = "main"
	ClusterRoleSecondary  ClusterRole = "secondary"
	ClusterRoleServer3    ClusterRole = "server3"
	ClusterRoleServer4    ClusterRole = "server4"
	ClusterRoleServer5    ClusterRole = "server5"
)

// ClusterStatus represents the sync status
type ClusterStatus string

const (
	ClusterStatusOffline     ClusterStatus = "offline"
	ClusterStatusOnline      ClusterStatus = "online"
	ClusterStatusSyncing     ClusterStatus = "syncing"
	ClusterStatusError       ClusterStatus = "error"
	ClusterStatusConnecting  ClusterStatus = "connecting"
)

// ClusterConfig stores the local server's cluster configuration
type ClusterConfig struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	ClusterID         string         `gorm:"size:50;uniqueIndex" json:"cluster_id"`        // Shared cluster identifier
	ClusterSecret     string         `gorm:"size:100" json:"cluster_secret,omitempty"`     // Secret key for joining cluster
	ServerRole        ClusterRole    `gorm:"size:20;default:standalone" json:"server_role"`
	ServerIP          string         `gorm:"size:45" json:"server_ip"`
	ServerName        string         `gorm:"size:100" json:"server_name"`
	MainServerIP      string         `gorm:"size:45" json:"main_server_ip"`                // IP of main server (for secondary/server3)
	MainServerPort    int            `gorm:"default:8080" json:"main_server_port"`

	// Database replication
	DBReplicationEnabled  bool       `gorm:"default:false" json:"db_replication_enabled"`
	DBReplicationStatus   ClusterStatus `gorm:"size:20;default:offline" json:"db_replication_status"`
	DBReplicationLag      int        `gorm:"default:0" json:"db_replication_lag"`          // Lag in seconds
	DBSyncPort            int        `gorm:"default:5433" json:"db_sync_port"`

	// Redis replication
	RedisReplicationEnabled bool     `gorm:"default:false" json:"redis_replication_enabled"`
	RedisReplicationStatus  ClusterStatus `gorm:"size:20;default:offline" json:"redis_replication_status"`

	// RADIUS
	RadiusEnabled         bool       `gorm:"default:true" json:"radius_enabled"`
	RadiusRole            string     `gorm:"size:20;default:primary" json:"radius_role"`   // primary, backup

	// API
	APIEnabled            bool       `gorm:"default:true" json:"api_enabled"`
	APIRole               string     `gorm:"size:20;default:active" json:"api_role"`       // active, standby, read-only

	// Failover
	AutoFailoverEnabled   bool       `gorm:"default:true" json:"auto_failover_enabled"`
	FailoverPriority      int        `gorm:"default:1" json:"failover_priority"`           // Lower = higher priority

	// Status
	IsActive              bool       `gorm:"default:false" json:"is_active"`
	LastHeartbeat         *time.Time `json:"last_heartbeat"`
	LastSyncTime          *time.Time `json:"last_sync_time"`

	// Unique identifiers for verification
	DatabaseID            string     `gorm:"size:100" json:"database_id"`                  // PostgreSQL cluster ID
	HardwareID            string     `gorm:"size:100" json:"hardware_id"`

	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// ClusterNode represents a node in the cluster (stored on main server)
type ClusterNode struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	ClusterID         string         `gorm:"size:50;index" json:"cluster_id"`
	HardwareID        string         `gorm:"size:100;uniqueIndex" json:"hardware_id"`
	ServerRole        ClusterRole    `gorm:"size:20" json:"server_role"`
	ServerName        string         `gorm:"size:100" json:"server_name"`
	ServerIP          string         `gorm:"size:45" json:"server_ip"`

	// Status
	Status            ClusterStatus  `gorm:"size:20;default:offline" json:"status"`
	DBSyncStatus      ClusterStatus  `gorm:"size:20;default:offline" json:"db_sync_status"`
	RedisSyncStatus   ClusterStatus  `gorm:"size:20;default:offline" json:"redis_sync_status"`
	DBReplicationLag  int            `gorm:"default:0" json:"db_replication_lag"`

	// Server specs (for capacity calculation)
	CPUCores          int            `gorm:"default:0" json:"cpu_cores"`
	RAMGB             int            `gorm:"default:0" json:"ram_gb"`

	// Health metrics
	CPUUsage          float64        `gorm:"default:0" json:"cpu_usage"`
	MemoryUsage       float64        `gorm:"default:0" json:"memory_usage"`
	DiskUsage         float64        `gorm:"default:0" json:"disk_usage"`

	// Version tracking
	Version           string         `gorm:"size:20" json:"version"`

	// Timing
	LastHeartbeat     *time.Time     `json:"last_heartbeat"`
	LastSyncTime      *time.Time     `json:"last_sync_time"`
	JoinedAt          time.Time      `json:"joined_at"`

	// Verification
	DatabaseID        string         `gorm:"size:100" json:"database_id"`

	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

// ClusterEvent stores cluster events for audit
type ClusterEvent struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ClusterID   string    `gorm:"size:50;index" json:"cluster_id"`
	EventType   string    `gorm:"size:50" json:"event_type"`   // node_joined, node_left, failover, sync_error, etc.
	NodeID      uint      `json:"node_id"`
	NodeIP      string    `gorm:"size:45" json:"node_ip"`
	NodeRole    string    `gorm:"size:20" json:"node_role"`
	Description string    `gorm:"size:500" json:"description"`
	Severity    string    `gorm:"size:20" json:"severity"`     // info, warning, error, critical
	CreatedAt   time.Time `json:"created_at"`
}

// ClusterHeartbeat for API communication between nodes
type ClusterHeartbeat struct {
	ClusterID         string        `json:"cluster_id"`
	ClusterSecret     string        `json:"cluster_secret"`
	HardwareID        string        `json:"hardware_id"`
	ServerRole        ClusterRole   `json:"server_role"`
	ServerName        string        `json:"server_name"`
	ServerIP          string        `json:"server_ip"`
	MainServerIP      string        `json:"main_server_ip"`
	DatabaseID        string        `json:"database_id"`
	DBReplicationLag  int           `json:"db_replication_lag"`
	CPUCores          int           `json:"cpu_cores"`         // Server CPU cores
	RAMGB             int           `json:"ram_gb"`            // Server RAM in GB
	CPUUsage          float64       `json:"cpu_usage"`
	MemoryUsage       float64       `json:"memory_usage"`
	DiskUsage         float64       `json:"disk_usage"`
	SubscriberCount   int           `json:"subscriber_count"`
	Version           string        `json:"version"`           // Software version
	Timestamp         time.Time     `json:"timestamp"`
}

// ClusterHeartbeatResponse returned from main server to secondary
type ClusterHeartbeatResponse struct {
	Success           bool          `json:"success"`
	Message           string        `json:"message"`
	MainVersion       string        `json:"main_version"`      // Main server's software version
	UpdateAvailable   bool          `json:"update_available"`  // True if secondary should update
}

// ClusterJoinRequest for secondary servers joining the cluster
type ClusterJoinRequest struct {
	ClusterSecret     string        `json:"cluster_secret"`
	HardwareID        string        `json:"hardware_id"`
	ServerName        string        `json:"server_name"`
	ServerIP          string        `json:"server_ip"`
	RequestedRole     ClusterRole   `json:"requested_role"`
	DatabaseID        string        `json:"database_id"`
}

// ClusterJoinResponse returned to joining server
type ClusterJoinResponse struct {
	Success           bool          `json:"success"`
	Message           string        `json:"message"`
	ClusterID         string        `json:"cluster_id"`
	AssignedRole      ClusterRole   `json:"assigned_role"`
	MainServerIP      string        `json:"main_server_ip"`
	DBSyncPort        int           `json:"db_sync_port"`
	RedisHost         string        `json:"redis_host"`
	RedisPort         int           `json:"redis_port"`
	RedisPassword     string        `json:"redis_password,omitempty"`
	DBHost            string        `json:"db_host"`
	DBPort            int           `json:"db_port"`
	DBUser            string        `json:"db_user"`
	DBPassword        string        `json:"db_password,omitempty"`
	DBName            string        `json:"db_name"`
	ReplicationSlot   string        `json:"replication_slot"`
}

// ClusterStatusResponse for status endpoint
type ClusterStatusResponse struct {
	ClusterID         string         `json:"cluster_id"`
	IsActive          bool           `json:"is_active"`
	ServerRole        ClusterRole    `json:"server_role"`
	TotalNodes        int            `json:"total_nodes"`
	OnlineNodes       int            `json:"online_nodes"`
	Nodes             []ClusterNode  `json:"nodes"`
	DBReplicationOK   bool           `json:"db_replication_ok"`
	RedisReplicationOK bool          `json:"redis_replication_ok"`
	LastSyncTime      *time.Time     `json:"last_sync_time"`
	Events            []ClusterEvent `json:"events,omitempty"`
}

func (ClusterConfig) TableName() string {
	return "cluster_config"
}

func (ClusterNode) TableName() string {
	return "cluster_nodes"
}

func (ClusterEvent) TableName() string {
	return "cluster_events"
}
