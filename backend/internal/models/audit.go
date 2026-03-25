package models

import (
	"time"
)

// AuditAction represents the type of audit action
type AuditAction string

const (
	AuditActionCreate     AuditAction = "create"
	AuditActionUpdate     AuditAction = "update"
	AuditActionDelete     AuditAction = "delete"
	AuditActionLogin      AuditAction = "login"
	AuditActionLogout     AuditAction = "logout"
	AuditActionRenew      AuditAction = "renew"
	AuditActionDisconnect AuditAction = "disconnect"
	AuditActionResetFUP   AuditAction = "reset_fup"
	AuditActionResetMAC   AuditAction = "reset_mac"
	AuditActionTransfer   AuditAction = "transfer"
	AuditActionWithdraw   AuditAction = "withdraw"
)

// AuditLog represents an audit log entry
type AuditLog struct {
	ID          uint        `gorm:"column:id;primaryKey" json:"id"`
	UserID      uint        `gorm:"column:user_id;index" json:"user_id"`
	User        *User       `gorm:"-" json:"user,omitempty"`
	Username    string      `gorm:"column:username;size:100" json:"username"`
	UserType    UserType    `gorm:"column:user_type" json:"user_type"`
	Action      AuditAction `gorm:"column:action;size:50;not null;index" json:"action"`
	EntityType  string      `gorm:"column:entity_type;size:50;index" json:"entity_type"` // subscriber, reseller, service, etc.
	EntityID    uint        `gorm:"column:entity_id;index" json:"entity_id"`
	EntityName  string      `gorm:"column:entity_name;size:100" json:"entity_name"`
	OldValue    string      `gorm:"column:old_value;type:jsonb" json:"old_value"`
	NewValue    string      `gorm:"column:new_value;type:jsonb" json:"new_value"`
	Description string      `gorm:"column:description;size:500" json:"description"`
	IPAddress   string      `gorm:"column:ip_address;size:50" json:"ip_address"`
	UserAgent   string      `gorm:"column:user_agent;size:255" json:"user_agent"`
	CreatedAt   time.Time   `gorm:"column:created_at;index" json:"created_at"`
}

// Notification represents a system notification
type Notification struct {
	ID        uint       `gorm:"column:id;primaryKey" json:"id"`
	Title     string     `gorm:"column:title;size:255;not null" json:"title"`
	Message   string     `gorm:"column:message;type:text;not null" json:"message"`
	Type      string     `gorm:"column:type;size:50;default:info" json:"type"` // info, warning, error, success
	IsActive  bool       `gorm:"column:is_active;default:true" json:"is_active"`
	StartDate time.Time  `gorm:"column:start_date" json:"start_date"`
	EndDate   *time.Time `gorm:"column:end_date" json:"end_date"`
	CreatedAt time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

// Ticket represents a support ticket
type Ticket struct {
	ID           uint          `gorm:"column:id;primaryKey" json:"id"`
	TicketNumber string        `gorm:"column:ticket_number;size:20;uniqueIndex;not null" json:"ticket_number"`
	Subject      string        `gorm:"column:subject;size:255;not null" json:"subject"`
	Message      string        `gorm:"column:message;type:text;not null" json:"message"`
	Description  string        `gorm:"column:description;type:text;not null" json:"description"`
	Status       string        `gorm:"column:status;size:20;default:open;index" json:"status"` // open, pending, in_progress, resolved, closed
	Priority     string        `gorm:"column:priority;size:20;default:normal" json:"priority"` // low, normal, high, urgent
	Category     string        `gorm:"column:category;size:50" json:"category"`                // billing, technical, general, other

	// Creator (can be subscriber, reseller, or admin)
	CreatorType   string        `gorm:"column:creator_type;size:20" json:"creator_type"` // subscriber, reseller, admin
	SubscriberID  *uint         `gorm:"column:subscriber_id" json:"subscriber_id"`
	Subscriber    *Subscriber   `gorm:"foreignKey:SubscriberID;references:ID" json:"subscriber,omitempty"`
	ResellerID    *uint         `gorm:"column:reseller_id" json:"reseller_id"`
	Reseller      *Reseller     `gorm:"foreignKey:ResellerID;references:ID" json:"reseller,omitempty"`
	CreatedBy     *uint         `gorm:"column:created_by" json:"created_by"`
	CreatedByUser *User         `gorm:"foreignKey:CreatedBy;references:ID" json:"created_by_user,omitempty"`

	// Assignment
	AssignedTo   *uint         `gorm:"column:assigned_to" json:"assigned_to"`
	AssignedUser *User         `gorm:"foreignKey:AssignedTo;references:ID" json:"assigned_user,omitempty"`

	// Replies
	Replies      []TicketReply `gorm:"foreignKey:TicketID;references:ID" json:"replies"`

	CreatedAt    time.Time     `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time     `gorm:"column:updated_at" json:"updated_at"`
	ClosedAt     *time.Time    `gorm:"column:closed_at" json:"closed_at"`
}

// TicketReply represents a reply to a ticket
type TicketReply struct {
	ID         uint      `gorm:"column:id;primaryKey" json:"id"`
	TicketID   uint      `gorm:"column:ticket_id;not null;index" json:"ticket_id"`
	Message    string    `gorm:"column:message;type:text;not null" json:"message"`
	UserID     uint      `gorm:"column:user_id" json:"user_id"`
	User       *User     `gorm:"foreignKey:UserID;references:ID" json:"user,omitempty"`
	IsInternal bool      `gorm:"column:is_internal;default:false" json:"is_internal"` // Internal notes not visible to customer
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

// CommunicationTemplate represents a message template
type CommunicationTemplate struct {
	ID        uint      `gorm:"column:id;primaryKey" json:"id"`
	Name      string    `gorm:"column:name;size:100;not null" json:"name"`
	Type      string    `gorm:"column:type;size:20;not null" json:"type"` // sms, whatsapp, email
	Subject   string    `gorm:"column:subject;size:255" json:"subject"`
	Body      string    `gorm:"column:body;type:text;not null" json:"body"`
	Variables string    `gorm:"column:variables;type:text" json:"variables"` // JSON array of available variables
	IsActive  bool      `gorm:"column:is_active;default:true" json:"is_active"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// CommunicationRule represents an automation rule
type CommunicationRule struct {
	ID             uint      `gorm:"column:id;primaryKey" json:"id"`
	Name           string    `gorm:"column:name;size:100;not null" json:"name"`
	TriggerEvent   string    `gorm:"column:trigger_event;size:50;not null" json:"trigger_event"` // expiry_warning, expired, quota_warning, etc
	Channel        string    `gorm:"column:channel;size:20;not null;default:sms" json:"channel"` // sms, email, whatsapp
	DaysBefore     int       `gorm:"column:days_before;default:0" json:"days_before"`
	Template       string    `gorm:"column:template;type:text" json:"template"` // Message template with variables
	Enabled        bool      `gorm:"column:enabled;default:true" json:"enabled"`
	SendToReseller bool      `gorm:"column:send_to_reseller;default:false" json:"send_to_reseller"`
	ResellerID     *uint     `gorm:"column:reseller_id" json:"reseller_id"`    // nil = global
	FUPLevels      string    `gorm:"column:fup_levels;size:20;default:'1,2,3'" json:"fup_levels"` // comma-separated: "1", "2", "3", "1,2", etc.
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// CommunicationLog represents a sent message log
type CommunicationLog struct {
	ID           uint               `gorm:"column:id;primaryKey" json:"id"`
	Type         string             `gorm:"column:type;size:20;not null;index" json:"type"` // sms, whatsapp, email
	Recipient    string             `gorm:"column:recipient;size:255;not null" json:"recipient"`
	Subject      string             `gorm:"column:subject;size:255" json:"subject"`
	Message      string             `gorm:"column:message;type:text" json:"message"`
	Status       string             `gorm:"column:status;size:20;default:pending" json:"status"` // pending, sent, failed
	ErrorMessage string             `gorm:"column:error_message;size:500" json:"error_message"`
	SubscriberID *uint              `gorm:"column:subscriber_id;index" json:"subscriber_id"`
	Subscriber   *Subscriber        `gorm:"-" json:"subscriber,omitempty"`
	ResellerID   *uint              `gorm:"column:reseller_id;index" json:"reseller_id"`
	RuleID       *uint              `gorm:"column:rule_id" json:"rule_id"`
	Rule         *CommunicationRule `gorm:"-" json:"rule,omitempty"`
	CreatedAt    time.Time          `gorm:"column:created_at;index" json:"created_at"`
	SentAt       *time.Time         `gorm:"column:sent_at" json:"sent_at"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}

func (Notification) TableName() string {
	return "notifications"
}

func (Ticket) TableName() string {
	return "tickets"
}

func (TicketReply) TableName() string {
	return "ticket_replies"
}

func (CommunicationTemplate) TableName() string {
	return "communication_templates"
}

func (CommunicationRule) TableName() string {
	return "communication_rules"
}

func (CommunicationLog) TableName() string {
	return "communication_logs"
}

// MaintenanceWindow represents a scheduled maintenance period
type MaintenanceWindow struct {
	ID                uint       `gorm:"column:id;primaryKey" json:"id"`
	Title             string     `gorm:"column:title;size:200;not null" json:"title"`
	Message           string     `gorm:"column:message;type:text" json:"message"`
	StartTime         time.Time  `gorm:"column:start_time;not null" json:"start_time"`
	EndTime           time.Time  `gorm:"column:end_time;not null" json:"end_time"`
	IsActive          bool       `gorm:"column:is_active;default:true" json:"is_active"`
	NotifySubscribers bool       `gorm:"column:notify_subscribers;default:false" json:"notify_subscribers"`
	CreatedBy         uint       `gorm:"column:created_by" json:"created_by"`
	CreatedAt         time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (MaintenanceWindow) TableName() string {
	return "maintenance_windows"
}

// ChurnScore represents churn risk analysis for a subscriber
type ChurnScore struct {
	ID              uint      `gorm:"column:id;primaryKey" json:"id"`
	SubscriberID    uint      `gorm:"column:subscriber_id;uniqueIndex" json:"subscriber_id"`
	Score           int       `gorm:"column:score;default:0" json:"score"`
	RiskLevel       string    `gorm:"column:risk_level;size:20;default:low" json:"risk_level"`
	Factors         string    `gorm:"column:factors;type:text" json:"factors"`
	DaysUntilExpiry int       `gorm:"column:days_until_expiry;default:0" json:"days_until_expiry"`
	UsageTrend      string    `gorm:"column:usage_trend;size:20;default:stable" json:"usage_trend"`
	TicketCount     int       `gorm:"column:ticket_count;default:0" json:"ticket_count"`
	PaymentDelays   int       `gorm:"column:payment_delays;default:0" json:"payment_delays"`
	ComputedAt      time.Time `gorm:"column:computed_at" json:"computed_at"`
	CreatedAt       time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ChurnScore) TableName() string {
	return "churn_scores"
}

// RadiusLog represents a RADIUS event log entry (auth, accounting, CoA)
type RadiusLog struct {
	ID         uint      `gorm:"column:id;primaryKey" json:"id"`
	EventType  string    `gorm:"column:event_type;size:20;not null" json:"event_type"`
	Username   string    `gorm:"column:username;size:100" json:"username"`
	NasIP      string    `gorm:"column:nas_ip;size:50" json:"nas_ip"`
	ClientIP   string    `gorm:"column:client_ip;size:50" json:"client_ip"`
	MACAddress string    `gorm:"column:mac_address;size:50" json:"mac_address"`
	Reason     string    `gorm:"column:reason;size:200" json:"reason"`
	DurationMs int       `gorm:"column:duration_ms" json:"duration_ms"`
	SessionID  string    `gorm:"column:session_id;size:64" json:"session_id"`
	Details    string    `gorm:"column:details;type:text" json:"details"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

func (RadiusLog) TableName() string {
	return "radius_logs"
}

// SystemLog represents a system event log entry (API, background services)
type SystemLog struct {
	ID        uint      `gorm:"column:id;primaryKey" json:"id"`
	Level     string    `gorm:"column:level;size:10;not null" json:"level"`
	Module    string    `gorm:"column:module;size:50;not null" json:"module"`
	Message   string    `gorm:"column:message;type:text;not null" json:"message"`
	Details   string    `gorm:"column:details;type:text" json:"details"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (SystemLog) TableName() string {
	return "system_logs"
}
