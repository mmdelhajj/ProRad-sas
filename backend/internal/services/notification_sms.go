package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

// SMSService handles sending SMS via various providers
type SMSService struct {
	client *http.Client
}

// NewSMSService creates a new SMS service
func NewSMSService() *SMSService {
	return &SMSService{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SMSProvider represents supported SMS providers
type SMSProvider string

const (
	SMSProviderTwilio  SMSProvider = "twilio"
	SMSProviderVonage  SMSProvider = "vonage"
	SMSProviderCustom  SMSProvider = "custom"
)

// SMSConfig holds SMS configuration
type SMSConfig struct {
	Provider    SMSProvider
	// Twilio
	TwilioSID    string
	TwilioToken  string
	TwilioFrom   string
	// Vonage
	VonageKey    string
	VonageSecret string
	VonageFrom   string
	// Custom API
	CustomURL     string
	CustomMethod  string // GET or POST
	CustomHeaders map[string]string
	CustomBody    string // Template with {{to}}, {{message}} placeholders
	CustomParams  string // URL params template for GET requests
}

// GetConfig retrieves SMS configuration from database
func (s *SMSService) GetConfig() (*SMSConfig, error) {
	settings := make(map[string]string)
	keys := []string{
		"sms_provider",
		"sms_twilio_sid", "sms_twilio_token", "sms_twilio_from",
		"sms_vonage_key", "sms_vonage_secret", "sms_vonage_from",
		"sms_custom_url", "sms_custom_method", "sms_custom_headers", "sms_custom_body", "sms_custom_params",
		"sms_api_key", // Legacy field
	}

	for _, key := range keys {
		var setting models.SystemPreference
		if err := database.DB.Where("key = ?", key).First(&setting).Error; err == nil {
			settings[key] = setting.Value
		}
	}

	provider := SMSProvider(settings["sms_provider"])
	if provider == "" {
		return nil, fmt.Errorf("SMS provider not configured")
	}

	config := &SMSConfig{
		Provider:      provider,
		TwilioSID:     settings["sms_twilio_sid"],
		TwilioToken:   settings["sms_twilio_token"],
		TwilioFrom:    settings["sms_twilio_from"],
		VonageKey:     settings["sms_vonage_key"],
		VonageSecret:  settings["sms_vonage_secret"],
		VonageFrom:    settings["sms_vonage_from"],
		CustomURL:     settings["sms_custom_url"],
		CustomMethod:  settings["sms_custom_method"],
		CustomBody:    settings["sms_custom_body"],
		CustomParams:  settings["sms_custom_params"],
	}

	// Parse custom headers if present
	if settings["sms_custom_headers"] != "" {
		config.CustomHeaders = make(map[string]string)
		json.Unmarshal([]byte(settings["sms_custom_headers"]), &config.CustomHeaders)
	}

	return config, nil
}

// SendSMS sends an SMS message
func (s *SMSService) SendSMS(to, message string) error {
	config, err := s.GetConfig()
	if err != nil {
		return err
	}

	return s.SendSMSWithConfig(config, to, message)
}

// SendSMSWithConfig sends an SMS with specific config
func (s *SMSService) SendSMSWithConfig(config *SMSConfig, to, message string) error {
	switch config.Provider {
	case SMSProviderTwilio:
		return s.sendViaTwilio(config, to, message)
	case SMSProviderVonage:
		return s.sendViaVonage(config, to, message)
	case SMSProviderCustom:
		return s.sendViaCustom(config, to, message)
	default:
		return fmt.Errorf("unsupported SMS provider: %s", config.Provider)
	}
}

// sendViaTwilio sends SMS via Twilio
func (s *SMSService) sendViaTwilio(config *SMSConfig, to, message string) error {
	if config.TwilioSID == "" || config.TwilioToken == "" {
		return fmt.Errorf("Twilio credentials not configured")
	}

	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", config.TwilioSID)

	data := url.Values{}
	data.Set("To", to)
	data.Set("From", config.TwilioFrom)
	data.Set("Body", message)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.SetBasicAuth(config.TwilioSID, config.TwilioToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Twilio error (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// sendViaVonage sends SMS via Vonage (Nexmo)
func (s *SMSService) sendViaVonage(config *SMSConfig, to, message string) error {
	if config.VonageKey == "" || config.VonageSecret == "" {
		return fmt.Errorf("Vonage credentials not configured")
	}

	apiURL := "https://rest.nexmo.com/sms/json"

	payload := map[string]string{
		"api_key":    config.VonageKey,
		"api_secret": config.VonageSecret,
		"to":         to,
		"from":       config.VonageFrom,
		"text":       message,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Vonage error (%d): %s", resp.StatusCode, string(body))
	}

	// Check Vonage response for errors
	var vonageResp struct {
		Messages []struct {
			Status    string `json:"status"`
			ErrorText string `json:"error-text"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &vonageResp); err == nil {
		if len(vonageResp.Messages) > 0 && vonageResp.Messages[0].Status != "0" {
			return fmt.Errorf("Vonage error: %s", vonageResp.Messages[0].ErrorText)
		}
	}

	return nil
}

// sendViaCustom sends SMS via custom API
func (s *SMSService) sendViaCustom(config *SMSConfig, to, message string) error {
	if config.CustomURL == "" {
		return fmt.Errorf("custom SMS URL not configured")
	}

	method := config.CustomMethod
	if method == "" {
		method = "POST"
	}

	// Replace placeholders in URL
	apiURL := strings.ReplaceAll(config.CustomURL, "{{to}}", url.QueryEscape(to))
	apiURL = strings.ReplaceAll(apiURL, "{{message}}", url.QueryEscape(message))

	var reqBody io.Reader
	if method == "POST" && config.CustomBody != "" {
		body := strings.ReplaceAll(config.CustomBody, "{{to}}", to)
		body = strings.ReplaceAll(body, "{{message}}", message)
		reqBody = strings.NewReader(body)
	} else if method == "GET" && config.CustomParams != "" {
		params := strings.ReplaceAll(config.CustomParams, "{{to}}", url.QueryEscape(to))
		params = strings.ReplaceAll(params, "{{message}}", url.QueryEscape(message))
		if !strings.Contains(apiURL, "?") {
			apiURL += "?" + params
		} else {
			apiURL += "&" + params
		}
	}

	req, err := http.NewRequest(method, apiURL, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Add custom headers
	for key, value := range config.CustomHeaders {
		req.Header.Set(key, value)
	}

	if method == "POST" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SMS API error (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// TestConnection tests the SMS connection
func (s *SMSService) TestConnection(config *SMSConfig) error {
	switch config.Provider {
	case SMSProviderTwilio:
		if config.TwilioSID == "" || config.TwilioToken == "" {
			return fmt.Errorf("Twilio SID and Token are required")
		}
		// Test by fetching account info
		apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s.json", config.TwilioSID)
		req, _ := http.NewRequest("GET", apiURL, nil)
		req.SetBasicAuth(config.TwilioSID, config.TwilioToken)
		resp, err := s.client.Do(req)
		if err != nil {
			return fmt.Errorf("connection failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == 401 {
			return fmt.Errorf("authentication failed: invalid credentials")
		}
		if resp.StatusCode >= 400 {
			return fmt.Errorf("Twilio error: status %d", resp.StatusCode)
		}
		return nil

	case SMSProviderVonage:
		if config.VonageKey == "" || config.VonageSecret == "" {
			return fmt.Errorf("Vonage API Key and Secret are required")
		}
		// Test by checking account balance
		apiURL := fmt.Sprintf("https://rest.nexmo.com/account/get-balance?api_key=%s&api_secret=%s",
			config.VonageKey, config.VonageSecret)
		resp, err := s.client.Get(apiURL)
		if err != nil {
			return fmt.Errorf("connection failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return fmt.Errorf("Vonage error: invalid credentials")
		}
		return nil

	case SMSProviderCustom:
		if config.CustomURL == "" {
			return fmt.Errorf("custom API URL is required")
		}
		// Just verify URL is reachable
		testURL := strings.Split(config.CustomURL, "?")[0]
		req, _ := http.NewRequest("HEAD", testURL, nil)
		resp, err := s.client.Do(req)
		if err != nil {
			return fmt.Errorf("connection failed: %v", err)
		}
		defer resp.Body.Close()
		return nil

	default:
		return fmt.Errorf("unsupported provider: %s", config.Provider)
	}
}

// SendTestSMS sends a test SMS
func (s *SMSService) SendTestSMS(config *SMSConfig, toPhone string) error {
	message := "ProxPanel Test: Your SMS configuration is working correctly!"
	return s.SendSMSWithConfig(config, toPhone, message)
}

// Helper for basic auth header
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
