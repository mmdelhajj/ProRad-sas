package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

// proxRadAPIBase is the base URL for the proxsms.com API
const proxRadAPIBase = "http://proxsms.com/api"

// getProxRadAPISecret returns the proxsms.com API key from env (PROXRAD_API_SECRET).
// Falls back to the legacy hardcoded value if the env var is not set.
func getProxRadAPISecret() string {
	if v := os.Getenv("PROXRAD_API_SECRET"); v != "" {
		return v
	}
	return "ad893b18ecad5d1fb751523420a2aaf04ac7b05e"
}

// WhatsAppService handles sending WhatsApp messages via Ultramsg or ProxRad
type WhatsAppService struct {
	client *http.Client
}

// NewWhatsAppService creates a new WhatsApp service
func NewWhatsAppService() *WhatsAppService {
	return &WhatsAppService{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// WhatsAppConfig holds WhatsApp configuration
type WhatsAppConfig struct {
	InstanceID string
	Token      string
}

// ProxRadConfig holds ProxRad (proxsms.com) WhatsApp configuration
type ProxRadConfig struct {
	APISecret     string
	AccountUnique string
	APIBase       string
}

// getProvider reads the configured WhatsApp provider from DB
func (s *WhatsAppService) getProvider() string {
	var setting models.SystemPreference
	if err := database.DB.Where("key = ?", "whatsapp_provider").First(&setting).Error; err == nil {
		return setting.Value
	}
	return "ultramsg"
}

// GetProxRadConfig retrieves ProxRad configuration from database
func (s *WhatsAppService) GetProxRadConfig() (*ProxRadConfig, error) {
	var setting models.SystemPreference
	accountUnique := ""
	if err := database.DB.Where("key = ?", "proxrad_account_unique").First(&setting).Error; err == nil {
		accountUnique = setting.Value
	}
	if accountUnique == "" {
		return nil, fmt.Errorf("ProxRad WhatsApp account not linked yet")
	}
	return &ProxRadConfig{
		APISecret:     getProxRadAPISecret(),
		AccountUnique: accountUnique,
		APIBase:       proxRadAPIBase,
	}, nil
}

// GetConfig retrieves WhatsApp configuration from database
func (s *WhatsAppService) GetConfig() (*WhatsAppConfig, error) {
	settings := make(map[string]string)
	keys := []string{"whatsapp_instance_id", "whatsapp_token", "whatsapp_api_key"}

	for _, key := range keys {
		var setting models.SystemPreference
		if err := database.DB.Where("key = ?", key).First(&setting).Error; err == nil {
			settings[key] = setting.Value
		}
	}

	instanceID := settings["whatsapp_instance_id"]
	token := settings["whatsapp_token"]
	if token == "" {
		token = settings["whatsapp_api_key"] // Legacy field
	}

	if instanceID == "" || token == "" {
		return nil, fmt.Errorf("WhatsApp not configured")
	}

	return &WhatsAppConfig{
		InstanceID: instanceID,
		Token:      token,
	}, nil
}

// ProxRadWAAccount represents a WhatsApp account from proxsms.com
type ProxRadWAAccount struct {
	ID     int    `json:"id"`
	Phone  string `json:"phone"`
	Unique string `json:"unique"`
	Status string `json:"status"`
}

// ProxRadLinkResult holds the QR code data from create/wa.link
type ProxRadLinkResult struct {
	QRString    string `json:"qrstring"`
	QRImageLink string `json:"qrimagelink"`
	InfoLink    string `json:"infolink"`
}

// ProxRadLinkInfo holds the connection status from get/wa.info
type ProxRadLinkInfo struct {
	Status string `json:"status"`
	Unique string `json:"unique"`
	Phone  string `json:"phone"`
}

// CreateProxRadLink creates a new WhatsApp link and returns QR code data
func (s *WhatsAppService) CreateProxRadLink(sid int) (*ProxRadLinkResult, error) {
	if sid <= 0 {
		sid = 1
	}
	reqURL := fmt.Sprintf("%s/create/wa.link?secret=%s&sid=%d", proxRadAPIBase, url.QueryEscape(getProxRadAPISecret()), sid)
	resp, err := s.client.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Status  int             `json:"status"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}
	if result.Status != 200 {
		return nil, fmt.Errorf("proxsms error: %s", result.Message)
	}
	var linkResult ProxRadLinkResult
	if err := json.Unmarshal(result.Data, &linkResult); err != nil {
		return nil, fmt.Errorf("proxsms returned unexpected data: %s", string(result.Data))
	}
	if linkResult.QRImageLink == "" {
		return nil, fmt.Errorf("no QR code returned from proxsms")
	}
	return &linkResult, nil
}

// GetProxRadLinkStatus checks the connection status via the info URL
func (s *WhatsAppService) GetProxRadLinkStatus(infoURL string) (*ProxRadLinkInfo, error) {
	resp, err := s.client.Get(infoURL)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Status  int             `json:"status"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	// Parse data as object (may be false/bool on error)
	var info ProxRadLinkInfo
	if err := json.Unmarshal(result.Data, &info); err != nil {
		// data might be false (bool) — return pending status
		return &ProxRadLinkInfo{Status: "pending"}, nil
	}
	return &info, nil
}

// GetProxRadAccounts lists all WhatsApp accounts from proxsms.com
func (s *WhatsAppService) GetProxRadAccounts() ([]ProxRadWAAccount, error) {
	reqURL := fmt.Sprintf("%s/get/wa.accounts?secret=%s", proxRadAPIBase, url.QueryEscape(getProxRadAPISecret()))
	resp, err := s.client.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Status  int              `json:"status"`
		Message string           `json:"message"`
		Data    json.RawMessage  `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}
	if result.Status != 200 {
		return nil, fmt.Errorf("proxsms error: %s", result.Message)
	}
	var accounts []ProxRadWAAccount
	if err := json.Unmarshal(result.Data, &accounts); err != nil {
		return nil, fmt.Errorf("failed to parse accounts: %v", err)
	}
	return accounts, nil
}

// proxRadAccessCache caches the license server subscription check result
var proxRadAccessCache struct {
	sync.Mutex
	result    *ProxRadAccessResult
	fetchedAt time.Time
}

// ProxRadAccessResult holds the result of a ProxRad access check
type ProxRadAccessResult struct {
	Allowed    bool
	Type       string // "subscribed", "trial", "expired"
	ExpiresAt  *time.Time
	TrialEnds  *time.Time
}

// CheckProxRadAccess checks if ProxRad WhatsApp is allowed for this installation.
// Order of checks:
//  1. License server says "subscribed" → allow
//  2. License server says "expired" (subscription was set but lapsed) → block
//  3. License server says "trial" (no subscription) → check local 2-day trial
func (s *WhatsAppService) CheckProxRadAccess() *ProxRadAccessResult {
	proxRadAccessCache.Lock()
	defer proxRadAccessCache.Unlock()

	// Use cached result for 5 minutes
	if proxRadAccessCache.result != nil && time.Since(proxRadAccessCache.fetchedAt) < 5*time.Minute {
		return proxRadAccessCache.result
	}

	result := s.fetchProxRadAccess()
	proxRadAccessCache.result = result
	proxRadAccessCache.fetchedAt = time.Now()
	return result
}

func (s *WhatsAppService) fetchProxRadAccess() *ProxRadAccessResult {
	licenseKey := os.Getenv("LICENSE_KEY")
	licenseServer := os.Getenv("LICENSE_SERVER")
	if licenseServer == "" {
		licenseServer = "https://license.proxrad.com"
	}

	// Call license server
	req, err := http.NewRequest("GET", licenseServer+"/api/v1/license/proxrad-status", nil)
	if err == nil {
		req.Header.Set("X-License-Key", licenseKey)
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == 200 {
			defer resp.Body.Close()
			var lsResult struct {
				Success   bool       `json:"success"`
				Type      string     `json:"type"`
				Active    bool       `json:"active"`
				ExpiresAt *time.Time `json:"expires_at"`
			}
			if json.NewDecoder(resp.Body).Decode(&lsResult) == nil {
				if lsResult.Type == "subscribed" {
					return &ProxRadAccessResult{Allowed: true, Type: "subscribed", ExpiresAt: lsResult.ExpiresAt}
				}
				if lsResult.Type == "expired" {
					return &ProxRadAccessResult{Allowed: false, Type: "expired", ExpiresAt: lsResult.ExpiresAt}
				}
			}
		}
	}

	// License server unavailable or "trial" → check local 2-day trial
	return s.checkLocalTrial()
}

func (s *WhatsAppService) checkLocalTrial() *ProxRadAccessResult {
	var setting models.SystemPreference
	if err := database.DB.Where("key = ?", "proxrad_trial_start").First(&setting).Error; err != nil {
		// No trial started yet — not connected → allow (trial starts on first connect)
		return &ProxRadAccessResult{Allowed: true, Type: "trial"}
	}

	trialStart, err := time.Parse(time.RFC3339, setting.Value)
	if err != nil {
		return &ProxRadAccessResult{Allowed: true, Type: "trial"}
	}

	trialEnd := trialStart.Add(48 * time.Hour)
	if time.Now().Before(trialEnd) {
		return &ProxRadAccessResult{Allowed: true, Type: "trial", TrialEnds: &trialEnd}
	}
	return &ProxRadAccessResult{Allowed: false, Type: "expired", TrialEnds: &trialEnd}
}

// InvalidateProxRadAccessCache clears the cached access result
func (s *WhatsAppService) InvalidateProxRadAccessCache() {
	proxRadAccessCache.Lock()
	proxRadAccessCache.result = nil
	proxRadAccessCache.Unlock()
}

// DisconnectProxRadAccount calls proxsms.com to delete/disconnect the given account unique ID
func (s *WhatsAppService) DisconnectProxRadAccount(unique string) error {
	reqURL := fmt.Sprintf("%s/delete/wa.account?secret=%s&unique=%s",
		proxRadAPIBase, url.QueryEscape(getProxRadAPISecret()), url.QueryEscape(unique))

	resp, err := s.client.Get(reqURL)
	if err != nil {
		log.Printf("DisconnectProxRad: HTTP error: %v", err)
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	log.Printf("DisconnectProxRad: response for %s: %s", unique, string(body))

	var result struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &result) == nil && result.Status != 200 {
		return fmt.Errorf("proxsms disconnect error: %s", result.Message)
	}
	return nil
}

// SendMessageViaProxRad sends a WhatsApp message using proxsms.com API
func (s *WhatsAppService) SendMessageViaProxRad(config *ProxRadConfig, to, message string) error {
	// Check subscription / trial access
	access := s.CheckProxRadAccess()
	if !access.Allowed {
		if access.Type == "expired" && access.ExpiresAt != nil {
			return fmt.Errorf("ProxRad subscription expired on %s — contact your provider to renew", access.ExpiresAt.Format("2006-01-02"))
		}
		return fmt.Errorf("ProxRad 2-day trial has expired — contact your provider to subscribe")
	}

	reqURL := fmt.Sprintf("%s/send/whatsapp", config.APIBase)

	formData := url.Values{}
	formData.Set("secret", config.APISecret)
	formData.Set("account", config.AccountUnique)
	formData.Set("recipient", to)
	formData.Set("type", "text")
	formData.Set("message", message)
	formData.Set("priority", "1")

	req, err := http.NewRequest("POST", reqURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err == nil && result.Status != 200 {
		return fmt.Errorf("proxsms error: %s", result.Message)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("proxsms HTTP error (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// SendMessageWithAccountUnique sends a WhatsApp message using a specific reseller account_unique
// Used by per-reseller WhatsApp feature
func (s *WhatsAppService) SendMessageWithAccountUnique(accountUnique, to, message string) error {
	config := &ProxRadConfig{
		APISecret:     getProxRadAPISecret(),
		AccountUnique: accountUnique,
		APIBase:       proxRadAPIBase,
	}
	return s.SendMessageViaProxRad(config, to, message)
}

// SendMessage sends a WhatsApp text message (routes to correct provider)
func (s *WhatsAppService) SendMessage(to, message string) error {
	provider := s.getProvider()
	if provider == "proxrad" {
		config, err := s.GetProxRadConfig()
		if err != nil {
			return err
		}
		return s.SendMessageViaProxRad(config, to, message)
	}
	// Default: Ultramsg
	config, err := s.GetConfig()
	if err != nil {
		return err
	}
	return s.SendMessageWithConfig(config, to, message)
}

// SendMessageForSubscriber sends WhatsApp using the subscriber's reseller account if connected,
// otherwise falls back to the admin's configured WhatsApp account.
func (s *WhatsAppService) SendMessageForSubscriber(sub models.Subscriber, to, message string) error {
	// Only send if subscriber has WhatsApp notifications opted in
	if !sub.WhatsAppNotifications {
		return nil // subscriber not opted in for WhatsApp notifications
	}
	// Try reseller's WhatsApp account first
	if sub.ResellerID > 0 {
		reseller := sub.Reseller
		if reseller == nil || reseller.ID == 0 {
			// Load reseller from DB if not preloaded
			var r models.Reseller
			if err := database.DB.First(&r, sub.ResellerID).Error; err == nil {
				reseller = &r
			}
		}
		if reseller != nil && reseller.WhatsAppEnabled && reseller.WhatsAppAccountUnique != "" {
			log.Printf("CommRule: Sending WhatsApp via reseller '%s' account for subscriber %s", reseller.Name, sub.Username)
			return s.SendMessageWithAccountUnique(reseller.WhatsAppAccountUnique, to, message)
		}
		// Reseller has no WhatsApp configured - skip notification (don't use admin's account)
		log.Printf("CommRule: Reseller has no WhatsApp configured - skipping for %s", sub.Username)
		return nil
	}
	// No reseller (admin's own subscriber) - use admin's configured WhatsApp account
	return s.SendMessage(to, message)
}

// SendMessageWithConfig sends a WhatsApp message with specific config
func (s *WhatsAppService) SendMessageWithConfig(config *WhatsAppConfig, to, message string) error {
	apiURL := fmt.Sprintf("https://api.ultramsg.com/%s/messages/chat", config.InstanceID)

	// Format phone number (remove + if present, ensure it has country code)
	to = strings.TrimPrefix(to, "+")

	data := url.Values{}
	data.Set("token", config.Token)
	data.Set("to", to)
	data.Set("body", message)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Ultramsg error (%d): %s", resp.StatusCode, string(body))
	}

	// Check response for errors
	var ultramsgResp struct {
		Sent   string `json:"sent"`
		Error  string `json:"error"`
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &ultramsgResp); err == nil {
		if ultramsgResp.Error != "" {
			return fmt.Errorf("Ultramsg error: %s", ultramsgResp.Error)
		}
		if ultramsgResp.Sent == "false" {
			return fmt.Errorf("message not sent: %s", string(body))
		}
	}

	return nil
}

// SendImage sends a WhatsApp image message
func (s *WhatsAppService) SendImage(to, imageURL, caption string) error {
	config, err := s.GetConfig()
	if err != nil {
		return err
	}

	apiURL := fmt.Sprintf("https://api.ultramsg.com/%s/messages/image", config.InstanceID)

	to = strings.TrimPrefix(to, "+")

	data := url.Values{}
	data.Set("token", config.Token)
	data.Set("to", to)
	data.Set("image", imageURL)
	if caption != "" {
		data.Set("caption", caption)
	}

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Ultramsg error (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// SendDocument sends a WhatsApp document
func (s *WhatsAppService) SendDocument(to, documentURL, filename string) error {
	config, err := s.GetConfig()
	if err != nil {
		return err
	}

	apiURL := fmt.Sprintf("https://api.ultramsg.com/%s/messages/document", config.InstanceID)

	to = strings.TrimPrefix(to, "+")

	data := url.Values{}
	data.Set("token", config.Token)
	data.Set("to", to)
	data.Set("document", documentURL)
	if filename != "" {
		data.Set("filename", filename)
	}

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Ultramsg error (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// TestConnection tests the WhatsApp connection
func (s *WhatsAppService) TestConnection(config *WhatsAppConfig) error {
	if config.InstanceID == "" {
		return fmt.Errorf("Instance ID is required")
	}
	if config.Token == "" {
		return fmt.Errorf("Token is required")
	}

	// Test by getting instance status
	apiURL := fmt.Sprintf("https://api.ultramsg.com/%s/instance/status?token=%s",
		config.InstanceID, config.Token)

	resp, err := s.client.Get(apiURL)
	if err != nil {
		return fmt.Errorf("connection failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Ultramsg error (%d): %s", resp.StatusCode, string(body))
	}

	// Check response
	var statusResp struct {
		Status struct {
			AccountStatus struct {
				Status string `json:"status"`
			} `json:"accountStatus"`
		} `json:"status"`
		Error string `json:"error"`
	}

	if err := json.Unmarshal(body, &statusResp); err == nil {
		if statusResp.Error != "" {
			return fmt.Errorf("Ultramsg error: %s", statusResp.Error)
		}
	}

	return nil
}

// GetInstanceStatus gets the WhatsApp instance status
func (s *WhatsAppService) GetInstanceStatus(config *WhatsAppConfig) (map[string]interface{}, error) {
	apiURL := fmt.Sprintf("https://api.ultramsg.com/%s/instance/status?token=%s",
		config.InstanceID, config.Token)

	resp, err := s.client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return result, nil
}

// SendTestMessage sends a test WhatsApp message
func (s *WhatsAppService) SendTestMessage(config *WhatsAppConfig, toPhone string) error {
	message := "✅ *ProxPanel Test*\n\nYour WhatsApp configuration is working correctly!\n\nYou can now receive automated notifications."
	return s.SendMessageWithConfig(config, toPhone, message)
}

// SendTemplateMessage sends a formatted message using a template
func (s *WhatsAppService) SendTemplateMessage(to string, template string, data map[string]string) error {
	message := template
	for key, value := range data {
		message = strings.ReplaceAll(message, "{{"+key+"}}", value)
	}
	return s.SendMessage(to, message)
}

// BulkSendMessage sends messages to multiple recipients
func (s *WhatsAppService) BulkSendMessage(recipients []string, message string) ([]error, error) {
	config, err := s.GetConfig()
	if err != nil {
		return nil, err
	}

	errors := make([]error, len(recipients))
	for i, to := range recipients {
		errors[i] = s.SendMessageWithConfig(config, to, message)
		// Add delay to stay within WhatsApp rate limit (1 message/second minimum)
		time.Sleep(1000 * time.Millisecond)
	}

	return errors, nil
}

// UltramsgWebhookPayload represents incoming webhook from Ultramsg
type UltramsgWebhookPayload struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Body      string `json:"body"`
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Ack       string `json:"ack"`
}

// ParseWebhook parses incoming webhook payload
func (s *WhatsAppService) ParseWebhook(body []byte) (*UltramsgWebhookPayload, error) {
	var payload UltramsgWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse webhook: %v", err)
	}
	return &payload, nil
}
