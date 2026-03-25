package models

import (
	"time"
)

// RadCheck represents RADIUS check attributes
type RadCheck struct {
	ID        uint   `gorm:"column:id;primaryKey" json:"id"`
	Username  string `gorm:"column:username;size:64;not null;index" json:"username"`
	Attribute string `gorm:"column:attribute;size:64;not null" json:"attribute"`
	Op        string `gorm:"column:op;size:2;not null;default:':='" json:"op"`
	Value     string `gorm:"column:value;size:253;not null" json:"value"`
}

// RadReply represents RADIUS reply attributes
type RadReply struct {
	ID        uint   `gorm:"column:id;primaryKey" json:"id"`
	Username  string `gorm:"column:username;size:64;not null;index" json:"username"`
	Attribute string `gorm:"column:attribute;size:64;not null" json:"attribute"`
	Op        string `gorm:"column:op;size:2;not null;default:'='" json:"op"`
	Value     string `gorm:"column:value;size:253;not null" json:"value"`
}

// RadGroupCheck represents RADIUS group check attributes
type RadGroupCheck struct {
	ID        uint   `gorm:"column:id;primaryKey" json:"id"`
	GroupName string `gorm:"column:groupname;size:64;not null;index" json:"groupname"`
	Attribute string `gorm:"column:attribute;size:64;not null" json:"attribute"`
	Op        string `gorm:"column:op;size:2;not null;default:':='" json:"op"`
	Value     string `gorm:"column:value;size:253;not null" json:"value"`
}

// RadGroupReply represents RADIUS group reply attributes
type RadGroupReply struct {
	ID        uint   `gorm:"column:id;primaryKey" json:"id"`
	GroupName string `gorm:"column:groupname;size:64;not null;index" json:"groupname"`
	Attribute string `gorm:"column:attribute;size:64;not null" json:"attribute"`
	Op        string `gorm:"column:op;size:2;not null;default:'='" json:"op"`
	Value     string `gorm:"column:value;size:253;not null" json:"value"`
}

// RadUserGroup represents user to group mapping
type RadUserGroup struct {
	ID        uint   `gorm:"column:id;primaryKey" json:"id"`
	Username  string `gorm:"column:username;size:64;not null;index" json:"username"`
	GroupName string `gorm:"column:groupname;size:64;not null" json:"groupname"`
	Priority  int    `gorm:"column:priority;default:1" json:"priority"`
}

// RadAcct represents RADIUS accounting records
type RadAcct struct {
	ID                  uint       `gorm:"column:radacctid;primaryKey" json:"id"`
	AcctSessionID       string     `gorm:"column:acctsessionid;size:64;not null;index" json:"acctsessionid"`
	AcctUniqueID        string     `gorm:"column:acctuniqueid;size:32;uniqueIndex" json:"acctuniqueid"`
	Username            string     `gorm:"column:username;size:64;not null;index" json:"username"`
	Realm               string     `gorm:"column:realm;size:64" json:"realm"`
	NasIPAddress        string     `gorm:"column:nasipaddress;size:15;not null;index" json:"nasipaddress"`
	NasPortID           string     `gorm:"column:nasportid;size:50" json:"nasportid"`
	NasPortType         string     `gorm:"column:nasporttype;size:32" json:"nasporttype"`
	AcctStartTime       *time.Time `gorm:"column:acctstarttime;index" json:"acctstarttime"`
	AcctUpdateTime      *time.Time `gorm:"column:acctupdatetime" json:"acctupdatetime"`
	AcctStopTime        *time.Time `gorm:"column:acctstoptime;index" json:"acctstoptime"`
	AcctSessionTime     int        `gorm:"column:acctsessiontime;default:0" json:"acctsessiontime"`
	AcctAuthentic       string     `gorm:"column:acctauthentic;size:32" json:"acctauthentic"`
	ConnectInfoStart    string     `gorm:"column:connectinfo_start;size:50" json:"connectinfo_start"`
	ConnectInfoStop     string     `gorm:"column:connectinfo_stop;size:50" json:"connectinfo_stop"`
	AcctInputOctets     int64      `gorm:"column:acctinputoctets;default:0" json:"acctinputoctets"`
	AcctOutputOctets    int64      `gorm:"column:acctoutputoctets;default:0" json:"acctoutputoctets"`
	CalledStationID     string     `gorm:"column:calledstationid;size:50" json:"calledstationid"`
	CallingStationID    string     `gorm:"column:callingstationid;size:50;index" json:"callingstationid"` // MAC Address
	AcctTerminateCause  string     `gorm:"column:acctterminatecause;size:32" json:"acctterminatecause"`
	ServiceType         string     `gorm:"column:servicetype;size:32" json:"servicetype"`
	FramedProtocol      string     `gorm:"column:framedprotocol;size:32" json:"framedprotocol"`
	FramedIPAddress     string     `gorm:"column:framedipaddress;size:15;index" json:"framedipaddress"`
	FramedIPv6Address   string     `gorm:"column:framedipv6address;size:45" json:"framedipv6address"`
	FramedIPv6Prefix    string     `gorm:"column:framedipv6prefix;size:45" json:"framedipv6prefix"`
	FramedInterfaceID   string     `gorm:"column:framedinterfaceid;size:44" json:"framedinterfaceid"`
	DelegatedIPv6Prefix string     `gorm:"column:delegatedipv6prefix;size:45" json:"delegatedipv6prefix"`
}

// RadPostAuth represents post-authentication logs
type RadPostAuth struct {
	ID               uint      `gorm:"column:id;primaryKey" json:"id"`
	Username         string    `gorm:"column:username;size:64;not null;index" json:"username"`
	Pass             string    `gorm:"column:pass;size:64" json:"pass"`
	Reply            string    `gorm:"column:reply;size:32" json:"reply"`
	CallingStationID string    `gorm:"column:callingstationid;size:50" json:"callingstationid"`
	AuthDate         time.Time `gorm:"column:authdate;autoCreateTime;index" json:"authdate"`
}

func (RadCheck) TableName() string {
	return "radcheck"
}

func (RadReply) TableName() string {
	return "radreply"
}

func (RadGroupCheck) TableName() string {
	return "radgroupcheck"
}

func (RadGroupReply) TableName() string {
	return "radgroupreply"
}

func (RadUserGroup) TableName() string {
	return "radusergroup"
}

func (RadAcct) TableName() string {
	return "radacct"
}

func (RadPostAuth) TableName() string {
	return "radpostauth"
}
