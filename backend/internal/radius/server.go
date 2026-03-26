package radius

import (
	"bytes"
	"context"
	"crypto/des"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/ippool"
	"github.com/proisp/backend/internal/models"
	"github.com/proisp/backend/internal/security"
	"golang.org/x/crypto/md4"
	"gorm.io/gorm"
	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2866"
	"layeh.com/radius/rfc2869"
)

// staticIPConflictTracker tracks users who keep getting IPs that conflict with static IPs
// Key: "username:conflicting_ip", Value: {count, lastTime}
type conflictEntry struct {
	count    int
	lastTime time.Time
}

// logRadiusEvent writes a RADIUS event to the radius_logs table in a goroutine (non-blocking)
func logRadiusEvent(eventType, username, nasIP, clientIP, mac, reason, sessionID, details string, durationMs int) {
	go func() {
		if database.DB == nil {
			return
		}
		entry := models.RadiusLog{
			EventType:  eventType,
			Username:   username,
			NasIP:      nasIP,
			ClientIP:   clientIP,
			MACAddress: mac,
			Reason:     reason,
			DurationMs: durationMs,
			SessionID:  sessionID,
			Details:    details,
		}
		database.DB.Create(&entry)
	}()
}

var staticIPConflicts sync.Map

// normalizeSpeedForMikrotik converts all speeds to kb format
// Examples: "1.2M" -> "1200k", "2M" -> "2000k", "1200k" -> "1200k"
func normalizeSpeedForMikrotik(speed string) string {
	if speed == "" {
		return ""
	}

	speed = strings.TrimSpace(speed)
	lowerSpeed := strings.ToLower(speed)

	// Already in k format - return as-is
	if strings.HasSuffix(lowerSpeed, "k") {
		return speed
	}

	// Convert M to k
	if strings.HasSuffix(lowerSpeed, "m") {
		numPart := speed[:len(speed)-1]
		val, err := strconv.ParseFloat(numPart, 64)
		if err == nil {
			return fmt.Sprintf("%dk", int(val*1000))
		}
		return speed
	}

	// Plain number - add k suffix
	return speed + "k"
}

// normalizeRateLimitString normalizes a full rate limit string
// Format: "upload/download" or "upload/download burst_up/burst_down threshold_up/threshold_down time_up/time_down"
// Each speed value is normalized to handle decimal M values
func normalizeRateLimitString(rateLimit string) string {
	if rateLimit == "" {
		return ""
	}

	// Split by space for burst parameters
	parts := strings.Split(rateLimit, " ")
	if len(parts) == 0 {
		return rateLimit
	}

	// Normalize the first part (upload/download)
	speeds := strings.Split(parts[0], "/")
	if len(speeds) >= 2 {
		speeds[0] = normalizeSpeedForMikrotik(speeds[0])
		speeds[1] = normalizeSpeedForMikrotik(speeds[1])
		parts[0] = strings.Join(speeds, "/")
	} else if len(speeds) == 1 {
		parts[0] = normalizeSpeedForMikrotik(speeds[0])
	}

	// If there are burst parameters, normalize them too
	if len(parts) >= 2 {
		// Burst rate (parts[1])
		burstSpeeds := strings.Split(parts[1], "/")
		if len(burstSpeeds) >= 2 {
			burstSpeeds[0] = normalizeSpeedForMikrotik(burstSpeeds[0])
			burstSpeeds[1] = normalizeSpeedForMikrotik(burstSpeeds[1])
			parts[1] = strings.Join(burstSpeeds, "/")
		}
	}

	return strings.Join(parts, " ")
}

// getSettingInt retrieves an integer setting from database with default fallback
func getSettingInt(key string, defaultVal int) int {
	var pref models.SystemPreference
	if err := database.DB.Where("key = ?", key).First(&pref).Error; err != nil {
		return defaultVal
	}
	if val, err := strconv.Atoi(pref.Value); err == nil {
		return val
	}
	return defaultVal
}

// getSettingBool retrieves a boolean setting from database with default fallback
func getSettingBool(key string, defaultVal bool) bool {
	var pref models.SystemPreference
	if err := database.DB.Where("key = ?", key).First(&pref).Error; err != nil {
		return defaultVal
	}
	return pref.Value == "true" || pref.Value == "1"
}

// findAvailableIP finds an available IP in the same /24 subnet that's not used by any online user
// or assigned as a static IP to anyone
func findAvailableIP(conflictIP string) string {
	// Parse the conflict IP to get the subnet
	parts := strings.Split(conflictIP, ".")
	if len(parts) != 4 {
		return ""
	}

	// Get the /24 subnet base (e.g., "10.180.96")
	subnetBase := fmt.Sprintf("%s.%s.%s", parts[0], parts[1], parts[2])

	// Get all IPs currently in use by online users in this subnet
	var usedIPs []string
	database.DB.Model(&models.Subscriber{}).
		Where("ip_address LIKE ? AND is_online = ?", subnetBase+".%", true).
		Pluck("ip_address", &usedIPs)

	// Get all static IPs in this subnet
	var staticIPs []string
	database.DB.Model(&models.Subscriber{}).
		Where("static_ip LIKE ?", subnetBase+".%").
		Pluck("static_ip", &staticIPs)

	// Get all IPs already assigned in radreply (Framed-IP-Address)
	var radreplyIPs []string
	database.DB.Model(&models.RadReply{}).
		Where("attribute = ? AND value LIKE ?", "Framed-IP-Address", subnetBase+".%").
		Pluck("value", &radreplyIPs)

	// Create a set of used IPs
	usedSet := make(map[string]bool)
	for _, ip := range usedIPs {
		usedSet[ip] = true
	}
	for _, ip := range staticIPs {
		usedSet[ip] = true
	}
	for _, ip := range radreplyIPs {
		usedSet[ip] = true
	}

	// Find an available IP (start from .10 to avoid gateway/network addresses)
	for i := 10; i < 250; i++ {
		candidateIP := fmt.Sprintf("%s.%d", subnetBase, i)
		if !usedSet[candidateIP] {
			return candidateIP
		}
	}

	return ""
}

// Server represents a RADIUS server
// saasNASInfo holds tenant info for a NAS IP in SaaS mode
type saasNASInfo struct {
	TenantID   uint
	SchemaName string
}

type Server struct {
	authAddr  string
	acctAddr  string
	secretsMu sync.RWMutex
	secrets   map[string][]byte // NAS IP -> Secret

	// SaaS mode fields
	saasMode           bool
	saasNASMap         sync.Map // NAS IP (string) -> saasNASInfo
	saasMapLoaded      time.Time
	saasSharedSecret   []byte // Shared RADIUS secret for all SaaS tenants (unknown IPs)
}

// NewServer creates a new RADIUS server
func NewServer(authPort, acctPort int) *Server {
	s := &Server{
		authAddr: fmt.Sprintf(":%d", authPort),
		acctAddr: fmt.Sprintf(":%d", acctPort),
		secrets:  make(map[string][]byte),
	}
	// Check SaaS mode from environment
	if os.Getenv("SAAS_MODE") == "true" {
		s.saasMode = true
		log.Println("RADIUS: SaaS mode enabled - will route by NAS IP to tenant schemas")

		// Load shared RADIUS secret for unknown NAS IPs
		sharedSecret := os.Getenv("SAAS_RADIUS_SECRET")
		if sharedSecret == "" {
			sharedSecret = "ProxPanel-SaaS-2026"
		}
		s.saasSharedSecret = []byte(sharedSecret)
		log.Println("RADIUS SaaS: Shared secret loaded for unknown NAS IPs")
	}
	return s
}

// LoadSecrets loads NAS secrets from database
func (s *Server) LoadSecrets() error {
	newSecrets := make(map[string][]byte)

	if s.saasMode {
		// SaaS mode: load secrets from admin.tenants table
		// Each tenant's WireGuard client IP is the NAS IP, and radius_secret is the shared secret
		var tenants []models.Tenant
		if err := database.DB.Where("status IN ?", []string{"active", "trial"}).Find(&tenants).Error; err != nil {
			return fmt.Errorf("failed to load tenants for RADIUS: %w", err)
		}
		for _, t := range tenants {
			if t.WGClientIP != "" && t.RadiusSecret != "" {
				newSecrets[t.WGClientIP] = []byte(t.RadiusSecret)
				// Also update the NAS→tenant mapping
				s.saasNASMap.Store(t.WGClientIP, saasNASInfo{
					TenantID:   t.ID,
					SchemaName: t.SchemaName,
				})
			}
		}

		// Also load from admin.nas_tenant_map for explicit mappings
		var nasMappings []models.NasTenantMap
		if err := database.DB.Preload("Tenant").Find(&nasMappings).Error; err == nil {
			for _, m := range nasMappings {
				if m.Tenant != nil && m.Tenant.RadiusSecret != "" {
					newSecrets[m.NasIP] = []byte(m.Tenant.RadiusSecret)
					s.saasNASMap.Store(m.NasIP, saasNASInfo{
						TenantID:   m.TenantID,
						SchemaName: m.Tenant.SchemaName,
					})
				}
			}
		}
		s.saasMapLoaded = time.Now()
		log.Printf("RADIUS SaaS: Loaded %d tenant NAS secrets", len(newSecrets))
	} else {
		// Standard mode: load from nas_devices table
		var nasList []models.Nas
		if err := database.DB.Where("is_active = ?", true).Find(&nasList).Error; err != nil {
			return err
		}
		for _, nas := range nasList {
			newSecrets[nas.IPAddress] = []byte(nas.Secret)
		}
	}

	s.secretsMu.Lock()
	s.secrets = newSecrets
	s.secretsMu.Unlock()

	log.Printf("Loaded %d NAS secrets", len(newSecrets))
	return nil
}

// getDBForNAS returns a tenant-scoped DB session for SaaS mode, or the global DB for standard mode.
// In SaaS mode, it looks up the NAS IP in the tenant map and sets the search_path accordingly.
func (s *Server) getDBForNAS(nasIP string) *gorm.DB {
	if !s.saasMode {
		return database.DB
	}

	// Refresh tenant map every 60 seconds
	if time.Since(s.saasMapLoaded) > 60*time.Second {
		go s.LoadSecrets() // Refresh in background
	}

	if info, ok := s.saasNASMap.Load(nasIP); ok {
		tenantInfo := info.(saasNASInfo)
		return database.GetTenantDB(tenantInfo.SchemaName)
	}

	// Fallback to global DB if no mapping found
	log.Printf("RADIUS SaaS: No tenant mapping for NAS %s, using global DB", nasIP)
	return database.DB
}

// getDBForNASWithUsername resolves tenant DB for unknown NAS IPs by searching subscriber across tenants.
// Returns the tenant-scoped DB and true if a tenant was found via subscriber search.
func (s *Server) getDBForNASWithUsername(nasIP string, username string) (*gorm.DB, bool) {
	if !s.saasMode {
		return database.DB, false
	}

	// Refresh tenant map every 60 seconds
	if time.Since(s.saasMapLoaded) > 60*time.Second {
		go s.LoadSecrets()
	}

	// Fast path: known NAS
	if info, ok := s.saasNASMap.Load(nasIP); ok {
		tenantInfo := info.(saasNASInfo)
		return database.GetTenantDB(tenantInfo.SchemaName), false
	}

	// Slow path: unknown NAS — search subscriber across all tenant schemas
	if len(s.saasSharedSecret) > 0 && username != "" {
		tenantDB, tenantInfo := s.findTenantBySubscriber(username)
		if tenantDB != nil {
			// Auto-register this NAS IP → tenant mapping
			s.autoRegisterNAS(nasIP, tenantInfo)
			return tenantDB, true
		}
	}

	log.Printf("RADIUS SaaS: No tenant mapping for NAS %s, using global DB", nasIP)
	return database.DB, false
}

// findTenantBySubscriber searches for a subscriber username across all active/trial tenant schemas.
func (s *Server) findTenantBySubscriber(username string) (*gorm.DB, saasNASInfo) {
	var tenants []models.Tenant
	database.DB.Where("status IN ?", []string{"active", "trial"}).Find(&tenants)

	for _, t := range tenants {
		tDB := database.GetTenantDB(t.SchemaName)
		var count int64
		tDB.Model(&models.Subscriber{}).Where("username = ? AND is_active = true", username).Count(&count)
		if count > 0 {
			log.Printf("RADIUS SaaS: Found subscriber %s in tenant %s", username, t.SchemaName)
			return tDB, saasNASInfo{TenantID: t.ID, SchemaName: t.SchemaName}
		}
	}
	return nil, saasNASInfo{}
}

// autoRegisterNAS persists the NAS IP → tenant mapping for fast subsequent lookups.
func (s *Server) autoRegisterNAS(nasIP string, info saasNASInfo) {
	// 1. Store in memory map (immediate effect)
	s.saasNASMap.Store(nasIP, info)
	s.secretsMu.Lock()
	s.secrets[nasIP] = s.saasSharedSecret
	s.secretsMu.Unlock()

	// 2. Persist to nas_tenant_map in DB
	database.DB.Exec(
		"INSERT INTO admin.nas_tenant_map (nas_ip, tenant_id) VALUES (?, ?) ON CONFLICT (nas_ip) DO UPDATE SET tenant_id = ?",
		nasIP, info.TenantID, info.TenantID,
	)

	// 3. Update NAS ip_address in tenant schema (replace 0.0.0.0 placeholder)
	tDB := database.GetTenantDB(info.SchemaName)
	tDB.Model(&models.Nas{}).Where("ip_address = ?", "0.0.0.0").Update("ip_address", nasIP)

	log.Printf("RADIUS SaaS: Auto-registered NAS %s → tenant %s", nasIP, info.SchemaName)
}

// GetSecret returns the secret for a NAS IP
func (s *Server) GetSecret(remoteAddr net.Addr) ([]byte, error) {
	host, _, err := net.SplitHostPort(remoteAddr.String())
	if err != nil {
		return nil, err
	}

	// Fast path: known NAS
	s.secretsMu.RLock()
	secret, ok := s.secrets[host]
	s.secretsMu.RUnlock()
	if ok {
		return secret, nil
	}

	// SaaS mode: return shared secret for unknown IPs
	if s.saasMode && len(s.saasSharedSecret) > 0 {
		log.Printf("RADIUS SaaS: Using shared secret for unknown NAS %s", host)
		return s.saasSharedSecret, nil
	}

	return nil, fmt.Errorf("unknown NAS: %s", host)
}

// SecretSource implements the radius.SecretSource interface
type SecretSource struct {
	server *Server
}

func (ss SecretSource) RADIUSSecret(ctx context.Context, remoteAddr net.Addr) ([]byte, error) {
	return ss.server.GetSecret(remoteAddr)
}

// Start starts the RADIUS server
func (s *Server) Start() error {
	// Load NAS secrets
	if err := s.LoadSecrets(); err != nil {
		return fmt.Errorf("failed to load secrets: %w", err)
	}

	secretSource := SecretSource{server: s}

	// Start authentication server
	go func() {
		authServer := radius.PacketServer{
			Addr:         s.authAddr,
			Network:      "udp",
			SecretSource: secretSource,
			Handler:      radius.HandlerFunc(s.handleAuth),
		}

		log.Printf("Starting RADIUS auth server on %s", s.authAddr)
		if err := authServer.ListenAndServe(); err != nil {
			log.Printf("Auth server error: %v", err)
		}
	}()

	// Start accounting server
	go func() {
		acctServer := radius.PacketServer{
			Addr:         s.acctAddr,
			Network:      "udp",
			SecretSource: secretSource,
			Handler:      radius.HandlerFunc(s.handleAcct),
		}

		log.Printf("Starting RADIUS acct server on %s", s.acctAddr)
		if err := acctServer.ListenAndServe(); err != nil {
			log.Printf("Acct server error: %v", err)
		}
	}()

	return nil
}

// handleAuth handles authentication requests
func (s *Server) handleAuth(w radius.ResponseWriter, r *radius.Request) {
	username := rfc2865.UserName_GetString(r.Packet)
	originalUsername := username // Keep original for logging
	nasIP := rfc2865.NASIPAddress_Get(r.Packet)
	callingStationID := rfc2865.CallingStationID_GetString(r.Packet)

	log.Printf("Auth request: user=%s, nas=%s, mac=%s", username, nasIP, callingStationID)

	// Start timing
	startTime := time.Now()

	// SaaS mode: resolve tenant DB from NAS IP (with subscriber search for unknown IPs)
	db, _ := s.getDBForNASWithUsername(nasIP.String(), username)

	// Handle realm stripping based on NAS configuration
	username = s.stripRealmIfAllowed(username, nasIP.String())
	if username != originalUsername {
		log.Printf("Realm stripped: %s -> %s", originalUsername, username)
	}

	// Get subscriber from cache or database
	subscriber, err := s.getSubscriberWithDB(db, username)
	if err != nil {
		log.Printf("Auth reject (user not found): %s", username)
		logRadiusEvent("auth_reject", username, nasIP.String(), "", callingStationID, "not_found", "", "", 0)
		s.logPostAuth(username, callingStationID, "Access-Reject")
		w.Write(r.Response(radius.CodeAccessReject))
		return
	}

	// Check if subscriber is active
	if subscriber.Status != models.SubscriberStatusActive {
		log.Printf("Auth reject (inactive): %s", username)
		logRadiusEvent("auth_reject", username, nasIP.String(), "", callingStationID, "inactive", "", "", 0)
		s.logPostAuth(username, callingStationID, "Access-Reject")
		w.Write(r.Response(radius.CodeAccessReject))
		return
	}

	// Check expiry
	if subscriber.IsExpired() {
		log.Printf("Auth reject (expired): %s", username)
		logRadiusEvent("auth_reject", username, nasIP.String(), "", callingStationID, "expired", "", "", 0)
		s.logPostAuth(username, callingStationID, "Access-Reject")
		w.Write(r.Response(radius.CodeAccessReject))
		return
	}

	// Get password from radcheck table (Cleartext-Password)
	var radcheck models.RadCheck
	var plainPassword string
	if err := db.Where("username = ? AND attribute = ?", username, "Cleartext-Password").First(&radcheck).Error; err == nil {
		plainPassword = radcheck.Value
	} else {
		// Fallback: try to decrypt from subscriber.PasswordPlain
		plainPassword = security.DecryptPassword(subscriber.PasswordPlain)
	}
	if plainPassword == "" {
		log.Printf("Auth reject (password not found): %s", username)
		logRadiusEvent("auth_reject", username, nasIP.String(), "", callingStationID, "password_not_found", "", "", 0)
		s.logPostAuth(username, callingStationID, "Access-Reject")
		w.Write(r.Response(radius.CodeAccessReject))
		return
	}

	// Try MS-CHAPv2 first (preferred for PPPoE)
	mschapChallenge := getMSCHAPChallenge(r.Packet)
	mschap2Response := getMSCHAP2Response(r.Packet)

	var authSuccess bool
	var mschap2SuccessResponse []byte

	if len(mschapChallenge) > 0 && len(mschap2Response) >= 50 {
		// MS-CHAPv2 authentication - use originalUsername for hash calculation (client uses full username with realm)
		authSuccess, mschap2SuccessResponse = verifyMSCHAP2(originalUsername, plainPassword, mschapChallenge, mschap2Response)
		if !authSuccess {
			log.Printf("Auth reject (MS-CHAPv2 failed): %s", username)
			logRadiusEvent("auth_reject", username, nasIP.String(), "", callingStationID, "wrong_password", "", "MS-CHAPv2", 0)
			s.logPostAuth(username, callingStationID, "Access-Reject")
			w.Write(r.Response(radius.CodeAccessReject))
			return
		}
		log.Printf("MS-CHAPv2 auth success for: %s", username)
	} else {
		// Fall back to PAP authentication
		password := rfc2865.UserPassword_GetString(r.Packet)
		if plainPassword != password {
			log.Printf("Auth reject (wrong password - PAP): %s", username)
			logRadiusEvent("auth_reject", username, nasIP.String(), "", callingStationID, "wrong_password", "", "PAP", 0)
			s.logPostAuth(username, callingStationID, "Access-Reject")
			w.Write(r.Response(radius.CodeAccessReject))
			return
		}
		authSuccess = true
		log.Printf("PAP auth success for: %s", username)
	}

	// Check MAC binding
	if subscriber.SaveMAC && subscriber.MACAddress != "" {
		normalizedMAC := strings.ToUpper(strings.ReplaceAll(callingStationID, "-", ":"))
		normalizedSavedMAC := strings.ToUpper(strings.ReplaceAll(subscriber.MACAddress, "-", ":"))
		if normalizedMAC != normalizedSavedMAC {
			log.Printf("Auth reject (MAC mismatch): %s, expected=%s, got=%s", username, subscriber.MACAddress, callingStationID)
			logRadiusEvent("auth_reject", username, nasIP.String(), "", callingStationID, "mac_mismatch", "", fmt.Sprintf("expected=%s got=%s", subscriber.MACAddress, callingStationID), 0)
			s.logPostAuth(username, callingStationID, "Access-Reject")
			w.Write(r.Response(radius.CodeAccessReject))
			return
		}
	}

	// Build response
	response := r.Response(radius.CodeAccessAccept)

	// Add MS-CHAP2-Success if MS-CHAPv2 was used
	if len(mschap2SuccessResponse) > 0 {
		// MS-CHAP2-Success is Microsoft VSA (Vendor 311, Attribute 26)
		vsaData := buildMicrosoftVSA(26, mschap2SuccessResponse)
		response.Add(26, vsaData)
	}

	// Add Mikrotik rate limit
	// Priority: 1) Per-subscriber bandwidth rule, 2) radreply (FUP), 3) service default
	var radReply models.RadReply
	var rateLimit string

	// First check for active per-subscriber bandwidth rule (highest priority)
	var subscriberRule models.SubscriberBandwidthRule
	if err := db.Where("subscriber_id = ? AND rule_type = ? AND enabled = ?",
		subscriber.ID, models.BandwidthRuleTypeInternet, true).
		Order("priority DESC").First(&subscriberRule).Error; err == nil {
		// Check if rule is active (not expired)
		if subscriberRule.IsActiveNow() {
			rateLimit = fmt.Sprintf("%dk/%dk", subscriberRule.UploadSpeed, subscriberRule.DownloadSpeed)
			log.Printf("Using per-subscriber bandwidth rule for %s: %s (rule_id=%d, remaining=%s)",
				username, rateLimit, subscriberRule.ID, subscriberRule.TimeRemaining())
		}
	}

	// If no subscriber rule, check radreply for custom rate limit (FUP speeds)
	if rateLimit == "" {
		if err := db.Where("username = ? AND attribute = ?", username, "Mikrotik-Rate-Limit").First(&radReply).Error; err == nil && radReply.Value != "" {
			// Use rate limit from radreply (FUP or custom speed)
			// Normalize the rate limit value to handle decimal M values
			rateLimit = normalizeRateLimitString(radReply.Value)
			log.Printf("Using radreply rate limit for %s: %s", username, rateLimit)
		}
	}

	// If still no rate limit, fall back to service default speeds
	if rateLimit == "" && subscriber.Service != nil {
		// Fall back to service default speeds
		// Normalize speed values to convert decimal M to k (e.g., 1.2M -> 1200k)
		uploadSpeed := normalizeSpeedForMikrotik(subscriber.Service.UploadSpeedStr)
		downloadSpeed := normalizeSpeedForMikrotik(subscriber.Service.DownloadSpeedStr)
		if uploadSpeed == "" && subscriber.Service.UploadSpeed > 0 {
			uploadSpeed = fmt.Sprintf("%dM", subscriber.Service.UploadSpeed)
		}
		if downloadSpeed == "" && subscriber.Service.DownloadSpeed > 0 {
			downloadSpeed = fmt.Sprintf("%dM", subscriber.Service.DownloadSpeed)
		}

		if uploadSpeed != "" || downloadSpeed != "" {
			rateLimit = fmt.Sprintf("%s/%s", uploadSpeed, downloadSpeed)

			// Add burst if configured
			if subscriber.Service.BurstUpload > 0 || subscriber.Service.BurstDownload > 0 {
				burstUp := subscriber.Service.BurstUpload
				burstDown := subscriber.Service.BurstDownload
				threshold := subscriber.Service.BurstThreshold
				burstTime := subscriber.Service.BurstTime

				rateLimit = fmt.Sprintf("%s/%s %d/%d %d/%d %d/%d",
					uploadSpeed, downloadSpeed,
					burstUp, burstDown,
					threshold, threshold,
					burstTime, burstTime)
			}
		}
	}

	// WAN Management Check: all users start at 1k/1k until port check passes
	if getSettingBool("wan_check_enabled", false) && subscriber.WanCheckStatus != "ok" && subscriber.WanCheckStatus != "skipped" {
		rateLimit = "1k/1k"
		log.Printf("WanCheck: Enforcing 1k/1k for %s (wan_check_status=%s, pending verification)", username, subscriber.WanCheckStatus)
	}

	if rateLimit != "" {
		// Set as vendor-specific attribute (Mikrotik-Rate-Limit)
		// Vendor ID: 14988, Attribute: 8
		vsaData := buildMikrotikVSA(8, []byte(rateLimit))
		response.Add(26, vsaData)
		log.Printf("Sending rate limit for %s: %s", username, rateLimit)
	}

	// Check if ProISP IP management is enabled
	proispIPManagement := getSettingBool("proisp_ip_management", false)

	// Determine the IP to send
	var framedIPToSend string

	// Priority 1: Static IP from subscriber record
	if subscriber.StaticIP != "" {
		framedIPToSend = subscriber.StaticIP
		log.Printf("Using static IP=%s for %s", framedIPToSend, username)
	}

	// Priority 2: Check radreply table for Framed-IP-Address (set by conflict resolution)
	if framedIPToSend == "" {
		var radreply models.RadReply
		if err := db.Where("username = ? AND attribute = ?", username, "Framed-IP-Address").First(&radreply).Error; err == nil {
			framedIPToSend = radreply.Value
			log.Printf("Found radreply Framed-IP-Address=%s for %s (conflict resolution)", framedIPToSend, username)
		}
	}

	// Priority 3: ProISP IP management - allocate from pool
	if framedIPToSend == "" && proispIPManagement && subscriber.Service != nil && subscriber.Service.PoolName != "" {
		// Get NAS ID for the allocation
		var nasID uint
		var nas models.Nas
		if err := db.Where("ip_address = ?", rfc2865.NASIPAddress_Get(r.Packet).String()).First(&nas).Error; err == nil {
			nasID = nas.ID
		}

		// Allocate IP from the pool
		allocatedIP, err := ippool.AllocateIPForUser(
			subscriber.Service.PoolName,
			username,
			subscriber.ID,
			nasID,
			"", // Session ID not available yet during auth
		)
		if err != nil {
			log.Printf("ProISP IP Management: Failed to allocate IP for %s from pool %s: %v",
				username, subscriber.Service.PoolName, err)
			// Fall back to Framed-Pool for MikroTik to assign
		} else {
			framedIPToSend = allocatedIP
			log.Printf("ProISP IP Management: Allocated IP=%s for %s from pool %s",
				framedIPToSend, username, subscriber.Service.PoolName)

			// Save to radreply so this IP persists across reconnections
			db.Where("username = ? AND attribute = ?", username, "Framed-IP-Address").Delete(&models.RadReply{})
			db.Create(&models.RadReply{
				Username:  username,
				Attribute: "Framed-IP-Address",
				Op:        ":=",
				Value:     allocatedIP,
			})
			log.Printf("ProISP IP Management: Saved Framed-IP-Address=%s to radreply for %s", allocatedIP, username)
		}
	}

	// Send IP to MikroTik
	if framedIPToSend != "" {
		ip := net.ParseIP(framedIPToSend)
		if ip != nil {
			rfc2865.FramedIPAddress_Set(response, ip)
			log.Printf("Sending Framed-IP-Address=%s for %s", framedIPToSend, username)
		}
	} else if subscriber.Service != nil && subscriber.Service.PoolName != "" {
		// No specific IP - send Framed-Pool for MikroTik to assign (legacy mode)
		rfc2869.FramedPool_SetString(response, subscriber.Service.PoolName)
		log.Printf("Sending Framed-Pool=%s for %s (MikroTik will assign IP)", subscriber.Service.PoolName, username)
	}

	// Add session timeout - use minimum of (time until expiry, default_session_timeout)
	defaultSessionTimeout := getSettingInt("default_session_timeout", 86400)
	remainingSeconds := int(time.Until(subscriber.ExpiryDate).Seconds())
	sessionTimeout := defaultSessionTimeout
	if remainingSeconds > 0 && remainingSeconds < defaultSessionTimeout {
		sessionTimeout = remainingSeconds
	}
	if sessionTimeout > 0 {
		rfc2865.SessionTimeout_Set(response, rfc2865.SessionTimeout(sessionTimeout))
	}

	// Add idle timeout from settings
	idleTimeout := getSettingInt("idle_timeout", 0)
	if idleTimeout > 0 {
		rfc2865.IdleTimeout_Set(response, rfc2865.IdleTimeout(idleTimeout))
	}

	// Add simultaneous use limit from subscriber settings
	simultaneousUse := subscriber.SimultaneousSessions
	if simultaneousUse <= 0 {
		simultaneousUse = 1
	}
	// Check global setting - if Allow Simultaneous Use is OFF, force to 1
	if !getSettingBool("simultaneous_use", false) && simultaneousUse > 1 {
		simultaneousUse = 1
	}
	// Add as Vendor-Specific Attribute for Mikrotik (Mikrotik-Recv-Limit, id=1)
	// Or use standard Session-Limit if supported
	// For now, we rely on radcheck Simultaneous-Use attribute which is set per-user

	// Update subscriber MAC if not saved
	if !subscriber.SaveMAC || subscriber.MACAddress == "" {
		go func() {
			db.Model(&models.Subscriber{}).Where("id = ?", subscriber.ID).Update("mac_address", callingStationID)
		}()
	}

	// Log successful auth
	s.logPostAuth(username, callingStationID, "Access-Accept")

	duration := time.Since(startTime)
	durationMs := int(duration.Milliseconds())
	log.Printf("Auth accept: %s (%.2fms)", username, float64(duration.Microseconds())/1000)
	logRadiusEvent("auth_accept", username, nasIP.String(), "", callingStationID, "", "", "", durationMs)

	w.Write(response)
}

// handleAcct handles accounting requests
func (s *Server) handleAcct(w radius.ResponseWriter, r *radius.Request) {
	username := rfc2865.UserName_GetString(r.Packet)
	acctStatusType := rfc2866.AcctStatusType_Get(r.Packet)
	sessionID := rfc2866.AcctSessionID_GetString(r.Packet)
	nasIP := rfc2865.NASIPAddress_Get(r.Packet)
	framedIP := rfc2865.FramedIPAddress_Get(r.Packet)
	callingStationID := rfc2865.CallingStationID_GetString(r.Packet)
	sessionTime := rfc2866.AcctSessionTime_Get(r.Packet)
	inputOctets := rfc2866.AcctInputOctets_Get(r.Packet)
	outputOctets := rfc2866.AcctOutputOctets_Get(r.Packet)
	terminateCause := rfc2866.AcctTerminateCause_Get(r.Packet)

	log.Printf("Acct request: user=%s, type=%d, session=%s", username, acctStatusType, sessionID)

	// SaaS mode: resolve tenant DB from NAS IP (with subscriber search for unknown IPs)
	db, _ := s.getDBForNASWithUsername(nasIP.String(), username)

	now := time.Now()

	// Log accounting event type
	switch acctStatusType {
	case rfc2866.AcctStatusType_Value_Start:
		logRadiusEvent("acct_start", username, nasIP.String(), framedIP.String(), callingStationID, "", sessionID, "", 0)
		// DUPLICATE IP PROTECTION: Close any existing session with this IP (different user)
		// BUT: Don't close session of a user who has this IP as their STATIC IP
		ipStr := framedIP.String()
		if ipStr != "" && ipStr != "<nil>" && ipStr != "0.0.0.0" {
			// Find other users with this IP
			var oldSessions []models.Subscriber
			db.Where("ip_address = ? AND is_online = ? AND username != ?", ipStr, true, username).Find(&oldSessions)
			for _, oldSub := range oldSessions {
				// IMPORTANT: If old user has this as their STATIC IP, they have priority
				// We need to kick the NEW user (current session), not the static IP user
				if oldSub.StaticIP == ipStr {
					// Track how many times this user has been kicked for this conflict
					conflictKey := fmt.Sprintf("%s:%s", username, ipStr)
					var entry conflictEntry
					if val, ok := staticIPConflicts.Load(conflictKey); ok {
						entry = val.(conflictEntry)
					}

					// Reset counter if last attempt was more than 5 minutes ago
					if time.Since(entry.lastTime) > 5*time.Minute {
						entry.count = 0
					}
					entry.count++
					entry.lastTime = time.Now()
					staticIPConflicts.Store(conflictKey, entry)

					// After 3 attempts, assign the user a different IP via radreply Framed-IP-Address
					// This forces MikroTik to give them a specific IP instead of picking from pool
					if entry.count >= 3 {
						log.Printf("STATIC IP CONFLICT - ASSIGNING NEW IP: %s has been kicked %d times for IP %s (static IP of %s). Finding available IP...",
							username, entry.count, ipStr, oldSub.Username)

						// Find an available IP from the same subnet that's not used
						newIP := findAvailableIP(ipStr)
						if newIP != "" {
							// Add radreply entry to force this IP on next connection
							db.Where("username = ? AND attribute = ?", username, "Framed-IP-Address").Delete(&models.RadReply{})
							db.Create(&models.RadReply{
								Username:  username,
								Attribute: "Framed-IP-Address",
								Op:        "=",
								Value:     newIP,
							})
							log.Printf("STATIC IP CONFLICT RESOLVED: Assigned %s to %s via radreply. They will get this IP on reconnect.",
								newIP, username)

							// Disconnect and let them reconnect with new IP
							var nas models.Nas
							if err := db.Where("ip_address = ?", nasIP.String()).First(&nas).Error; err == nil {
								go func(newUsername, newSessionID string, nasInfo models.Nas, assignedIP string) {
									time.Sleep(500 * time.Millisecond)
									coaClient := NewCOAClient(nasInfo.IPAddress, nasInfo.CoAPort, nasInfo.Secret)
									coaClient.DisconnectViaRadclient(newUsername, newSessionID)
									log.Printf("Disconnected %s to apply new IP %s", newUsername, assignedIP)
								}(username, sessionID, nas, newIP)
							}
						} else {
							log.Printf("STATIC IP CONFLICT - NO AVAILABLE IP: Could not find available IP for %s. Allowing duplicate.", username)
						}
						staticIPConflicts.Delete(conflictKey)
						continue
					}

					log.Printf("DUPLICATE IP CONFLICT (%d/3): %s has static IP %s, but %s got assigned same IP from pool. Kicking new user.",
						entry.count, oldSub.Username, ipStr, username)

					// Get NAS info to send CoA disconnect to the NEW user
					var nas models.Nas
					if err := db.Where("ip_address = ?", nasIP.String()).First(&nas).Error; err == nil {
						go func(newUsername, newSessionID string, nasInfo models.Nas, conflictIP string, staticIPOwner string) {
							// Wait a moment for session to be fully established
							time.Sleep(500 * time.Millisecond)
							coaClient := NewCOAClient(nasInfo.IPAddress, nasInfo.CoAPort, nasInfo.Secret)
							if err := coaClient.DisconnectViaRadclient(newUsername, newSessionID); err != nil {
								log.Printf("CoA disconnect failed for new user %s: %v", newUsername, err)
							} else {
								log.Printf("CoA disconnected new user %s who had static IP %s belonging to %s",
									newUsername, conflictIP, staticIPOwner)
							}
						}(username, sessionID, nas, ipStr, oldSub.Username)
					}

					// Don't process this session - the user will reconnect and hopefully get a different IP
					continue
				}

				log.Printf("DUPLICATE IP DETECTED: %s has IP %s, but %s is starting new session with same IP. Disconnecting old user.",
					oldSub.Username, ipStr, username)

				// Send CoA disconnect to kick the old user from MikroTik
				if oldSub.NasID != nil && oldSub.SessionID != "" {
					go func(subUsername, subSessionID string, subNasID uint) {
						var nas models.Nas
						if err := db.First(&nas, subNasID).Error; err == nil {
							coaClient := NewCOAClient(nas.IPAddress, nas.CoAPort, nas.Secret)
							if err := coaClient.DisconnectViaRadclient(subUsername, subSessionID); err != nil {
								log.Printf("CoA disconnect failed for %s: %v", subUsername, err)
							} else {
								log.Printf("CoA disconnected %s (duplicate IP cleanup)", subUsername)
							}
						}
					}(oldSub.Username, oldSub.SessionID, *oldSub.NasID)
				}

				// Mark old user as offline
				db.Model(&models.Subscriber{}).Where("id = ?", oldSub.ID).Updates(map[string]interface{}{
					"is_online":  false,
					"session_id": "",
				})
				// Close old radacct record
				db.Model(&models.RadAcct{}).Where("username = ? AND framedipaddress = ? AND acctstoptime IS NULL",
					oldSub.Username, ipStr).Updates(map[string]interface{}{
					"acctstoptime":       now,
					"acctterminatecause": "Duplicate-IP-Cleanup",
				})
			}
		}

		// Session start - use shorter unique ID (sessionID + timestamp hex)
		uniqueID := fmt.Sprintf("%s-%x", sessionID, now.Unix())
		if len(uniqueID) > 32 {
			uniqueID = uniqueID[:32]
		}
		acct := models.RadAcct{
			AcctSessionID:    sessionID,
			AcctUniqueID:     uniqueID,
			Username:         username,
			NasIPAddress:     nasIP.String(),
			AcctStartTime:    &now,
			CallingStationID: callingStationID,
			FramedIPAddress:  framedIP.String(),
		}
		if err := db.Create(&acct).Error; err != nil {
			log.Printf("Acct: Failed to create radacct record for %s: %v", username, err)
		} else {
			log.Printf("Acct: Created radacct record for %s, session=%s", username, sessionID)
		}

		// Look up NAS by IP to get nas_id
		var nas models.Nas
		var nasID *uint
		if err := db.Where("ip_address = ?", nasIP.String()).First(&nas).Error; err == nil {
			nasID = &nas.ID
		}

		// Update IP pool assignment with session ID (if ProISP IP management is enabled)
		if getSettingBool("proisp_ip_management", false) {
			ipStr := framedIP.String()
			if ipStr != "" && ipStr != "<nil>" && ipStr != "0.0.0.0" {
				go func(ip, user, sessID string) {
					db.Model(&models.IPPoolAssignment{}).
						Where("ip_address = ? AND username = ?", ip, user).
						Update("session_id", sessID)
				}(ipStr, username, sessionID)
			}
		}

		// Update subscriber online status with nas_id
		go func(nasIDPtr *uint) {
			updates := map[string]interface{}{
				"is_online":   true,
				"ip_address":  framedIP.String(),
				"session_id":  sessionID,
				"last_seen":   now,
				"mac_address": callingStationID,
			}
			if nasIDPtr != nil {
				updates["nas_id"] = *nasIDPtr
			}
			db.Model(&models.Subscriber{}).Where("username = ?", username).Updates(updates)

			// WAN Management Check: if enabled and subscriber is unchecked,
			// log that QuotaSync will perform the check on the next cycle.
			if getSettingBool("wan_check_enabled", false) {
				var wanStatus string
				db.Model(&models.Subscriber{}).
					Select("wan_check_status").
					Where("username = ?", username).
					Pluck("wan_check_status", &wanStatus)
				if wanStatus == "" || wanStatus == "unchecked" {
					log.Printf("WanCheck: New session for %s — will be checked on next QuotaSync cycle", username)
				}
			}
		}(nasID)

	case rfc2866.AcctStatusType_Value_Stop:
		logRadiusEvent("acct_stop", username, nasIP.String(), framedIP.String(), callingStationID, "", sessionID, fmt.Sprintf("duration=%ds", sessionTime), 0)
		// Session stop
		var cause string
		if terminateCause > 0 {
			cause = fmt.Sprintf("%d", terminateCause)
		}

		db.Model(&models.RadAcct{}).Where("acctsessionid = ? AND username = ? AND acctstoptime IS NULL", sessionID, username).Updates(map[string]interface{}{
			"acctstoptime":       now,
			"acctsessiontime":    sessionTime,
			"acctinputoctets":    inputOctets,
			"acctoutputoctets":   outputOctets,
			"acctterminatecause": cause,
		})

		// Release IP if ProISP IP management is enabled
		if getSettingBool("proisp_ip_management", false) {
			ipStr := framedIP.String()
			if ipStr != "" && ipStr != "<nil>" && ipStr != "0.0.0.0" {
				go func(ip, user string) {
					if err := ippool.ReleaseIP(ip); err != nil {
						log.Printf("ProISP IP Management: Failed to release IP %s for %s: %v", ip, user, err)
					} else {
						log.Printf("ProISP IP Management: Released IP %s for %s", ip, user)
					}
				}(ipStr, username)
			}
		}

		// Update subscriber status
		go func() {
			db.Model(&models.Subscriber{}).Where("username = ?", username).Updates(map[string]interface{}{
				"is_online":  false,
				"session_id": "",
				"last_seen":  now,
			})

			// Update quota
			s.updateQuota(username, int64(inputOctets), int64(outputOctets))
		}()

	case rfc2866.AcctStatusType_Value_InterimUpdate:
		logRadiusEvent("acct_update", username, nasIP.String(), framedIP.String(), callingStationID, "", sessionID, fmt.Sprintf("duration=%ds", sessionTime), 0)
		// Interim update
		updateResult := db.Model(&models.RadAcct{}).Where("acctsessionid = ? AND username = ? AND acctstoptime IS NULL", sessionID, username).Updates(map[string]interface{}{
			"acctupdatetime":   now,
			"acctsessiontime":  sessionTime,
			"acctinputoctets":  inputOctets,
			"acctoutputoctets": outputOctets,
		})

		// If no existing session record (e.g., after server restart where only interim-updates arrive),
		// insert a new radacct row so StaleSessionCleanup and billing queries work correctly.
		// Security: verify username belongs to a real subscriber before inserting,
		// to prevent fake session injection from a compromised NAS device.
		if updateResult.RowsAffected == 0 {
			var subCount int64
			db.Model(&models.Subscriber{}).Where("username = ? AND deleted_at IS NULL", username).Count(&subCount)
			if subCount == 0 {
				log.Printf("Acct: InterimUpdate - ignoring unknown username %s (no subscriber record)", username)
				break
			}
			estimatedStart := now.Add(-time.Duration(sessionTime) * time.Second)
			uniqueID := fmt.Sprintf("%s-i%x", sessionID, now.Unix())
			if len(uniqueID) > 32 {
				uniqueID = uniqueID[:32]
			}
			acct := models.RadAcct{
				AcctSessionID:    sessionID,
				AcctUniqueID:     uniqueID,
				Username:         username,
				NasIPAddress:     nasIP.String(),
				AcctStartTime:    &estimatedStart,
				AcctUpdateTime:   &now,
				AcctSessionTime:  int(sessionTime),
				AcctInputOctets:  int64(inputOctets),
				AcctOutputOctets: int64(outputOctets),
				CallingStationID: callingStationID,
				FramedIPAddress:  framedIP.String(),
			}
			if err := db.Create(&acct).Error; err != nil {
				log.Printf("Acct: InterimUpdate - failed to insert missing session for %s: %v", username, err)
			} else {
				log.Printf("Acct: InterimUpdate - created missing radacct row for %s (session=%s, uptime=%ds)", username, sessionID, sessionTime)
			}
		}

		// Update last seen AND ensure is_online is true
		// This fixes the case where RADIUS restarts and misses the Start packet
		go func() {
			db.Model(&models.Subscriber{}).Where("username = ?", username).Updates(map[string]interface{}{
				"last_seen":  now,
				"is_online":  true,
				"session_id": sessionID,
				"ip_address": framedIP.String(),
			})

			// Update quota
			s.updateQuota(username, int64(inputOctets), int64(outputOctets))
		}()
	}

	// Always respond with Accounting-Response
	w.Write(r.Response(radius.CodeAccountingResponse))
}

// getSubscriber gets subscriber from database with caching (uses global DB)
func (s *Server) getSubscriber(username string) (*models.Subscriber, error) {
	return s.getSubscriberWithDB(database.DB, username)
}

// getSubscriberWithDB gets subscriber from a specific DB session (for SaaS tenant routing)
func (s *Server) getSubscriberWithDB(db *gorm.DB, username string) (*models.Subscriber, error) {
	// Try Redis cache first
	ctx := context.Background()
	cacheKey := fmt.Sprintf("subscriber:%s", username)

	// For now, always query database (add caching later)
	var subscriber models.Subscriber
	if err := db.Preload("Service").Where("username = ?", username).First(&subscriber).Error; err != nil {
		return nil, err
	}

	// Cache in Redis
	go func() {
		database.Redis.Set(ctx, cacheKey, subscriber.ID, 5*time.Minute)
	}()

	return &subscriber, nil
}

// stripRealmIfAllowed strips the realm from username if it's in the NAS's allowed realms list
func (s *Server) stripRealmIfAllowed(username, nasIP string) string {
	// Check if username contains a realm (@domain)
	if !strings.Contains(username, "@") {
		return username
	}

	parts := strings.SplitN(username, "@", 2)
	if len(parts) != 2 {
		return username
	}

	user := parts[0]
	realm := strings.ToLower(parts[1])

	// Get NAS from database to check allowed realms
	var nas models.Nas
	if err := database.DB.Where("ip_address = ?", nasIP).First(&nas).Error; err != nil {
		// NAS not found, don't strip realm
		log.Printf("NAS not found for IP %s, keeping realm", nasIP)
		return username
	}

	// Check if NAS has allowed realms configured
	if nas.AllowedRealms == "" {
		// No realms configured, don't strip (require explicit configuration)
		return username
	}

	// Check if the realm is in the allowed list
	allowedRealms := strings.Split(nas.AllowedRealms, ",")
	for _, allowed := range allowedRealms {
		allowed = strings.TrimSpace(strings.ToLower(allowed))
		if allowed == realm {
			// Realm is allowed, strip it
			log.Printf("Realm '%s' is allowed for NAS %s, stripping from username", realm, nas.Name)
			return user
		}
	}

	// Realm not in allowed list, keep original username
	log.Printf("Realm '%s' not in allowed list for NAS %s", realm, nas.Name)
	return username
}

// logPostAuth logs authentication attempt
func (s *Server) logPostAuth(username, callingStationID, reply string) {
	log := models.RadPostAuth{
		Username:         username,
		CallingStationID: callingStationID,
		Reply:            reply,
	}
	database.DB.Create(&log)
}

// isWithinTimeWindow checks if the current time falls within the service's time-based speed window (FREE time)
func isWithinTimeWindow(service *models.Service, now time.Time) bool {
	// Skip if ratios are both 0 (no boost) or time window not configured
	// Ratio is a BOOST percentage: 100% = double speed, 200% = triple speed, 0% = no change
	if service.TimeDownloadRatio == 0 && service.TimeUploadRatio == 0 {
		return false
	}
	if service.TimeFromHour == 0 && service.TimeFromMinute == 0 &&
		service.TimeToHour == 0 && service.TimeToMinute == 0 {
		return false
	}

	currentHour := now.Hour()
	currentMinute := now.Minute()
	currentTimeMinutes := currentHour*60 + currentMinute

	fromMinutes := service.TimeFromHour*60 + service.TimeFromMinute
	toMinutes := service.TimeToHour*60 + service.TimeToMinute

	// Handle time ranges that might cross midnight
	if fromMinutes <= toMinutes {
		// Normal range (e.g., 00:00 to 06:00 or 16:00 to 23:00)
		return currentTimeMinutes >= fromMinutes && currentTimeMinutes < toMinutes
	}
	// Crosses midnight (e.g., 22:00 to 06:00)
	return currentTimeMinutes >= fromMinutes || currentTimeMinutes < toMinutes
}

// updateQuota is called by RADIUS accounting but does NOT update subscriber quota fields.
// All quota tracking is handled by QuotaSyncService which uses delta-based calculation.
// This function only logs the accounting event for debugging purposes.
func (s *Server) updateQuota(username string, input, output int64) {
	// Quota tracking is handled entirely by QuotaSyncService
	// RADIUS accounting only updates radacct records (done in handleAcct)
	// We don't update subscriber quota fields here to avoid conflicts with QuotaSync
}

// checkFUP checks and applies Fair Usage Policy
// FUP is now handled by quota_sync service with multi-tier direct speeds
func (s *Server) checkFUP(subscriber *models.Subscriber) {
	// FUP is handled by QuotaSyncService which runs periodically
	// It checks thresholds and applies direct speeds (FUP1, FUP2, FUP3)
	// This function is kept for accounting updates but doesn't change FUP level
	// The QuotaSyncService will handle FUP enforcement based on:
	// - FUP1Threshold/FUP1DownloadSpeed/FUP1UploadSpeed
	// - FUP2Threshold/FUP2DownloadSpeed/FUP2UploadSpeed
	// - FUP3Threshold/FUP3DownloadSpeed/FUP3UploadSpeed
}

// MS-CHAPv2 Helper Functions

// getMSCHAPChallenge extracts MS-CHAP-Challenge from RADIUS packet (VSA 311:11)
func getMSCHAPChallenge(p *radius.Packet) []byte {
	for _, attr := range p.Attributes {
		if attr.Type == 26 { // Vendor-Specific
			if len(attr.Attribute) < 6 {
				continue
			}
			vendorID := binary.BigEndian.Uint32(attr.Attribute[0:4])
			if vendorID == 311 { // Microsoft
				vsaType := attr.Attribute[4]
				vsaLen := attr.Attribute[5]
				if vsaType == 11 && int(vsaLen) <= len(attr.Attribute)-4 { // MS-CHAP-Challenge
					return attr.Attribute[6 : 6+vsaLen-2]
				}
			}
		}
	}
	return nil
}

// getMSCHAP2Response extracts MS-CHAP2-Response from RADIUS packet (VSA 311:25)
func getMSCHAP2Response(p *radius.Packet) []byte {
	for _, attr := range p.Attributes {
		if attr.Type == 26 { // Vendor-Specific
			if len(attr.Attribute) < 6 {
				continue
			}
			vendorID := binary.BigEndian.Uint32(attr.Attribute[0:4])
			if vendorID == 311 { // Microsoft
				vsaType := attr.Attribute[4]
				vsaLen := attr.Attribute[5]
				if vsaType == 25 && int(vsaLen) <= len(attr.Attribute)-4 { // MS-CHAP2-Response
					return attr.Attribute[6 : 6+vsaLen-2]
				}
			}
		}
	}
	return nil
}

// verifyMSCHAP2 verifies MS-CHAPv2 authentication
func verifyMSCHAP2(username, password string, challenge, response []byte) (bool, []byte) {
	if len(response) < 50 {
		return false, nil
	}

	// MS-CHAPv2 Response format:
	// Ident (1) + Flags (1) + PeerChallenge (16) + Reserved (8) + NTResponse (24)
	peerChallenge := response[2:18]
	ntResponse := response[26:50]

	// Calculate expected NT Response
	expectedNT := generateNTResponse(challenge, peerChallenge, username, password)

	if !bytes.Equal(ntResponse, expectedNT) {
		return false, nil
	}

	// Generate authenticator response for MS-CHAP2-Success
	authResponse := generateAuthenticatorResponse(password, ntResponse, peerChallenge, challenge, username)

	// Build MS-CHAP2-Success (Ident + "S=" + 40 hex chars)
	ident := response[0]
	successStr := fmt.Sprintf("%c%s", ident, authResponse)

	return true, []byte(successStr)
}

// generateNTResponse generates the NT-Response for MS-CHAPv2
func generateNTResponse(authChallenge, peerChallenge []byte, username, password string) []byte {
	challenge := challengeHash(peerChallenge, authChallenge, username)
	passwordHash := ntPasswordHash(password)
	return challengeResponse(challenge, passwordHash)
}

// challengeHash creates the 8-byte challenge from peer and auth challenges
func challengeHash(peerChallenge, authChallenge []byte, username string) []byte {
	h := sha1.New()
	h.Write(peerChallenge)
	h.Write(authChallenge)
	h.Write([]byte(username))
	return h.Sum(nil)[:8]
}

// ntPasswordHash creates NT password hash using MD4
func ntPasswordHash(password string) []byte {
	// Convert password to UTF-16LE
	unicodePassword := make([]byte, len(password)*2)
	for i, r := range password {
		unicodePassword[i*2] = byte(r)
		unicodePassword[i*2+1] = byte(r >> 8)
	}

	h := md4.New()
	h.Write(unicodePassword)
	return h.Sum(nil)
}

// challengeResponse generates DES-encrypted response
func challengeResponse(challenge, passwordHash []byte) []byte {
	// Pad password hash to 21 bytes
	paddedHash := make([]byte, 21)
	copy(paddedHash, passwordHash)

	response := make([]byte, 24)
	desEncrypt(paddedHash[0:7], challenge, response[0:8])
	desEncrypt(paddedHash[7:14], challenge, response[8:16])
	desEncrypt(paddedHash[14:21], challenge, response[16:24])

	return response
}

// desEncrypt performs DES encryption for MS-CHAP
func desEncrypt(key, clear, cipher []byte) {
	// Expand 7-byte key to 8-byte DES key with parity bits
	desKey := make([]byte, 8)
	desKey[0] = key[0]
	desKey[1] = (key[0] << 7) | (key[1] >> 1)
	desKey[2] = (key[1] << 6) | (key[2] >> 2)
	desKey[3] = (key[2] << 5) | (key[3] >> 3)
	desKey[4] = (key[3] << 4) | (key[4] >> 4)
	desKey[5] = (key[4] << 3) | (key[5] >> 5)
	desKey[6] = (key[5] << 2) | (key[6] >> 6)
	desKey[7] = key[6] << 1

	// Set parity bits
	for i := range desKey {
		desKey[i] = setParityBit(desKey[i])
	}

	block, err := des.NewCipher(desKey)
	if err != nil {
		return
	}
	block.Encrypt(cipher, clear)
}

// setParityBit sets the parity bit for DES key byte
func setParityBit(b byte) byte {
	parity := byte(0)
	for i := 0; i < 7; i++ {
		parity ^= (b >> i) & 1
	}
	return (b & 0xFE) | (parity ^ 1)
}

// generateAuthenticatorResponse generates the authenticator response string
func generateAuthenticatorResponse(password string, ntResponse, peerChallenge, authChallenge []byte, username string) string {
	passwordHash := ntPasswordHash(password)
	passwordHashHash := md4Hash(passwordHash)

	h := sha1.New()
	h.Write(passwordHashHash)
	h.Write(ntResponse)
	h.Write([]byte("Magic server to client signing constant"))
	digest := h.Sum(nil)

	challenge := challengeHash(peerChallenge, authChallenge, username)

	h2 := sha1.New()
	h2.Write(digest)
	h2.Write(challenge)
	h2.Write([]byte("Pad to make it do more than one iteration"))
	finalDigest := h2.Sum(nil)

	return fmt.Sprintf("S=%X", finalDigest)
}

// md4Hash computes MD4 hash
func md4Hash(data []byte) []byte {
	h := md4.New()
	h.Write(data)
	return h.Sum(nil)
}

// buildMicrosoftVSA builds a Microsoft Vendor-Specific Attribute
func buildMicrosoftVSA(attrType byte, value []byte) []byte {
	// VSA format: Vendor-ID (4) + VSA-Type (1) + VSA-Length (1) + Value
	vsaLen := byte(len(value) + 2) // +2 for type and length bytes
	result := make([]byte, 4+2+len(value))

	// Microsoft Vendor ID = 311
	binary.BigEndian.PutUint32(result[0:4], 311)
	result[4] = attrType
	result[5] = vsaLen
	copy(result[6:], value)

	return result
}

// buildMikrotikVSA builds a Mikrotik Vendor-Specific Attribute
func buildMikrotikVSA(attrType byte, value []byte) []byte {
	// VSA format: Vendor-ID (4) + VSA-Type (1) + VSA-Length (1) + Value
	vsaLen := byte(len(value) + 2) // +2 for type and length bytes
	result := make([]byte, 4+2+len(value))

	// Mikrotik Vendor ID = 14988
	binary.BigEndian.PutUint32(result[0:4], 14988)
	result[4] = attrType
	result[5] = vsaLen
	copy(result[6:], value)

	return result
}
