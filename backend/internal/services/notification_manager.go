package services

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

// NotificationManager orchestrates all notification channels
type NotificationManager struct {
	email    *EmailService
	sms      *SMSService
	whatsapp *WhatsAppService
}

// NewNotificationManager creates a new notification manager
func NewNotificationManager() *NotificationManager {
	return &NotificationManager{
		email:    NewEmailService(),
		sms:      NewSMSService(),
		whatsapp: NewWhatsAppService(),
	}
}

// NotificationType represents different notification events
type NotificationType string

const (
	NotifyWelcome           NotificationType = "welcome"
	NotifyExpiryReminder    NotificationType = "expiry_reminder"
	NotifyExpired           NotificationType = "expired"
	NotifyRenewal           NotificationType = "renewal"
	NotifyQuotaLow          NotificationType = "quota_low"
	NotifyQuotaExhausted    NotificationType = "quota_exhausted"
	NotifyDisconnect        NotificationType = "disconnect"
	NotifyServiceChange     NotificationType = "service_change"
	NotifyPaymentReceived   NotificationType = "payment_received"
	NotifyInvoice           NotificationType = "invoice"
)

// NotificationChannel represents notification delivery channel
type NotificationChannel string

const (
	ChannelEmail    NotificationChannel = "email"
	ChannelSMS      NotificationChannel = "sms"
	ChannelWhatsApp NotificationChannel = "whatsapp"
)

// NotificationData holds data for template replacement
type NotificationData struct {
	SubscriberID   uint
	Username       string
	FullName       string
	Email          string
	Phone          string
	ServiceName    string
	ExpiryDate     string
	DaysRemaining  int
	QuotaUsed      string
	QuotaRemaining string
	Amount         string
	InvoiceNumber  string
	CompanyName    string
	CustomData     map[string]string
}

// SendNotification sends a notification through enabled channels
func (m *NotificationManager) SendNotification(notifType NotificationType, data *NotificationData) error {
	// Get notification settings
	settings := m.getNotificationSettings(notifType)
	if settings == nil {
		return nil // Notification type not configured
	}

	// Get company name for templates
	data.CompanyName = m.getCompanySetting("company_name", "ProxPanel")

	var errors []string

	// Send via enabled channels
	if settings.EmailEnabled && data.Email != "" {
		if err := m.sendEmailNotification(notifType, data); err != nil {
			errors = append(errors, fmt.Sprintf("email: %v", err))
			log.Printf("Email notification failed for %s: %v", data.Username, err)
		}
	}

	if settings.SMSEnabled && data.Phone != "" {
		if err := m.sendSMSNotification(notifType, data); err != nil {
			errors = append(errors, fmt.Sprintf("sms: %v", err))
			log.Printf("SMS notification failed for %s: %v", data.Username, err)
		}
	}

	if settings.WhatsAppEnabled && data.Phone != "" {
		if err := m.sendWhatsAppNotification(notifType, data); err != nil {
			errors = append(errors, fmt.Sprintf("whatsapp: %v", err))
			log.Printf("WhatsApp notification failed for %s: %v", data.Username, err)
		}
	}

	// Log notification
	m.logNotification(notifType, data, errors)

	if len(errors) > 0 {
		return fmt.Errorf("notification errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// NotificationSettings holds settings for a notification type
type NotificationSettings struct {
	EmailEnabled    bool
	SMSEnabled      bool
	WhatsAppEnabled bool
	EmailTemplate   string
	SMSTemplate     string
	WhatsAppTemplate string
}

func (m *NotificationManager) getNotificationSettings(notifType NotificationType) *NotificationSettings {
	prefix := fmt.Sprintf("notif_%s_", notifType)

	settings := &NotificationSettings{}

	// Check if notification type is enabled
	enabledKey := prefix + "enabled"
	var enabledSetting models.SystemPreference
	if database.DB.Where("key = ?", enabledKey).First(&enabledSetting).Error != nil {
		// Default settings if not configured
		return m.getDefaultSettings(notifType)
	}

	if enabledSetting.Value != "true" && enabledSetting.Value != "1" {
		return nil
	}

	// Get channel settings
	settings.EmailEnabled = m.getSettingBool(prefix + "email")
	settings.SMSEnabled = m.getSettingBool(prefix + "sms")
	settings.WhatsAppEnabled = m.getSettingBool(prefix + "whatsapp")

	// Get templates
	settings.EmailTemplate = m.getSettingString(prefix + "email_template")
	settings.SMSTemplate = m.getSettingString(prefix + "sms_template")
	settings.WhatsAppTemplate = m.getSettingString(prefix + "whatsapp_template")

	return settings
}

func (m *NotificationManager) getDefaultSettings(notifType NotificationType) *NotificationSettings {
	// Return default templates if no custom settings
	return &NotificationSettings{
		EmailEnabled:     true,
		SMSEnabled:       true,
		WhatsAppEnabled:  true,
		EmailTemplate:    m.getDefaultEmailTemplate(notifType),
		SMSTemplate:      m.getDefaultSMSTemplate(notifType),
		WhatsAppTemplate: m.getDefaultWhatsAppTemplate(notifType),
	}
}

func (m *NotificationManager) getDefaultEmailTemplate(notifType NotificationType) string {
	templates := map[NotificationType]string{
		NotifyWelcome: `
<h2>Welcome to {{company_name}}!</h2>
<p>Dear {{full_name}},</p>
<p>Your account has been created successfully.</p>
<p><strong>Username:</strong> {{username}}<br>
<strong>Service:</strong> {{service_name}}</p>
<p>Thank you for choosing us!</p>
`,
		NotifyExpiryReminder: `
<h2>Account Expiry Reminder</h2>
<p>Dear {{full_name}},</p>
<p>Your internet service will expire in <strong>{{days_remaining}} days</strong> on {{expiry_date}}.</p>
<p>Please renew your subscription to avoid service interruption.</p>
`,
		NotifyExpired: `
<h2>Account Expired</h2>
<p>Dear {{full_name}},</p>
<p>Your internet service has expired on {{expiry_date}}.</p>
<p>Please renew your subscription to restore service.</p>
`,
		NotifyRenewal: `
<h2>Subscription Renewed</h2>
<p>Dear {{full_name}},</p>
<p>Your subscription has been renewed successfully!</p>
<p><strong>New expiry date:</strong> {{expiry_date}}</p>
<p>Thank you for your continued trust.</p>
`,
		NotifyQuotaLow: `
<h2>Low Quota Warning</h2>
<p>Dear {{full_name}},</p>
<p>Your data quota is running low.</p>
<p><strong>Used:</strong> {{quota_used}}<br>
<strong>Remaining:</strong> {{quota_remaining}}</p>
`,
		NotifyPaymentReceived: `
<h2>Payment Received</h2>
<p>Dear {{full_name}},</p>
<p>We have received your payment of <strong>{{amount}}</strong>.</p>
<p>Thank you!</p>
`,
		NotifyInvoice: `
<h2>Invoice {{invoice_number}}</h2>
<p>Dear {{full_name}},</p>
<p>A new invoice has been generated for your internet service.</p>
<p><strong>Amount:</strong> {{amount}}<br>
<strong>Due Date:</strong> {{expiry_date}}<br>
<strong>Service:</strong> {{service_name}}</p>
<p>Please make the payment before the due date to avoid service interruption.</p>
`,
	}

	if template, ok := templates[notifType]; ok {
		return template
	}
	return ""
}

func (m *NotificationManager) getDefaultSMSTemplate(notifType NotificationType) string {
	templates := map[NotificationType]string{
		NotifyWelcome:        "Welcome to {{company_name}}! Your account {{username}} is now active.",
		NotifyExpiryReminder: "{{company_name}}: Your service expires in {{days_remaining}} days on {{expiry_date}}. Please renew.",
		NotifyExpired:        "{{company_name}}: Your service has expired. Please renew to restore internet access.",
		NotifyRenewal:        "{{company_name}}: Your subscription renewed! New expiry: {{expiry_date}}",
		NotifyQuotaLow:       "{{company_name}}: Low quota warning! Remaining: {{quota_remaining}}",
		NotifyPaymentReceived: "{{company_name}}: Payment of {{amount}} received. Thank you!",
		NotifyInvoice:         "{{company_name}}: Invoice {{invoice_number}} for {{amount}} due {{expiry_date}}. Please pay to avoid interruption.",
	}

	if template, ok := templates[notifType]; ok {
		return template
	}
	return ""
}

func (m *NotificationManager) getDefaultWhatsAppTemplate(notifType NotificationType) string {
	templates := map[NotificationType]string{
		NotifyWelcome: `🎉 *Welcome to {{company_name}}!*

Dear {{full_name}},

Your account has been created:
• *Username:* {{username}}
• *Service:* {{service_name}}

Thank you for choosing us!`,
		NotifyExpiryReminder: `⚠️ *Expiry Reminder*

Dear {{full_name}},

Your service expires in *{{days_remaining}} days* ({{expiry_date}}).

Please renew to avoid disconnection.`,
		NotifyExpired: `🔴 *Service Expired*

Dear {{full_name}},

Your internet service has expired.

Please renew your subscription to restore access.`,
		NotifyRenewal: `✅ *Subscription Renewed*

Dear {{full_name}},

Your subscription is renewed!
• *New expiry:* {{expiry_date}}

Thank you! 🙏`,
		NotifyQuotaLow: `⚠️ *Low Quota Warning*

Your data quota is running low:
• *Used:* {{quota_used}}
• *Remaining:* {{quota_remaining}}`,
		NotifyPaymentReceived: `✅ *Payment Received*

Amount: *{{amount}}*

Thank you for your payment!`,
		NotifyInvoice: `📄 *Invoice {{invoice_number}}*

Dear {{full_name}},

A new invoice has been generated:
• *Amount:* {{amount}}
• *Due Date:* {{expiry_date}}
• *Service:* {{service_name}}

Please pay before the due date to avoid disconnection.`,
	}

	if template, ok := templates[notifType]; ok {
		return template
	}
	return ""
}

func (m *NotificationManager) sendEmailNotification(notifType NotificationType, data *NotificationData) error {
	settings := m.getNotificationSettings(notifType)
	if settings == nil || !settings.EmailEnabled {
		return nil
	}

	template := settings.EmailTemplate
	if template == "" {
		template = m.getDefaultEmailTemplate(notifType)
	}

	subject := m.getNotificationSubject(notifType, data)
	body := m.replaceTemplateVars(template, data)

	return m.email.SendEmail(data.Email, subject, body, true)
}

func (m *NotificationManager) sendSMSNotification(notifType NotificationType, data *NotificationData) error {
	settings := m.getNotificationSettings(notifType)
	if settings == nil || !settings.SMSEnabled {
		return nil
	}

	template := settings.SMSTemplate
	if template == "" {
		template = m.getDefaultSMSTemplate(notifType)
	}

	message := m.replaceTemplateVars(template, data)
	return m.sms.SendSMS(data.Phone, message)
}

func (m *NotificationManager) sendWhatsAppNotification(notifType NotificationType, data *NotificationData) error {
	settings := m.getNotificationSettings(notifType)
	if settings == nil || !settings.WhatsAppEnabled {
		return nil
	}

	template := settings.WhatsAppTemplate
	if template == "" {
		template = m.getDefaultWhatsAppTemplate(notifType)
	}

	message := m.replaceTemplateVars(template, data)
	return m.whatsapp.SendMessage(data.Phone, message)
}

func (m *NotificationManager) getNotificationSubject(notifType NotificationType, data *NotificationData) string {
	subjects := map[NotificationType]string{
		NotifyWelcome:        "Welcome to %s!",
		NotifyExpiryReminder: "%s - Service Expiry Reminder",
		NotifyExpired:        "%s - Service Expired",
		NotifyRenewal:        "%s - Subscription Renewed",
		NotifyQuotaLow:       "%s - Low Quota Warning",
		NotifyQuotaExhausted: "%s - Quota Exhausted",
		NotifyPaymentReceived: "%s - Payment Received",
		NotifyInvoice:        "%s - Invoice #%s",
	}

	if subject, ok := subjects[notifType]; ok {
		if notifType == NotifyInvoice {
			return fmt.Sprintf(subject, data.CompanyName, data.InvoiceNumber)
		}
		return fmt.Sprintf(subject, data.CompanyName)
	}
	return fmt.Sprintf("%s - Notification", data.CompanyName)
}

func (m *NotificationManager) replaceTemplateVars(template string, data *NotificationData) string {
	replacements := map[string]string{
		"{{username}}":        data.Username,
		"{{full_name}}":       data.FullName,
		"{{email}}":           data.Email,
		"{{phone}}":           data.Phone,
		"{{service_name}}":    data.ServiceName,
		"{{expiry_date}}":     data.ExpiryDate,
		"{{days_remaining}}":  fmt.Sprintf("%d", data.DaysRemaining),
		"{{quota_used}}":      data.QuotaUsed,
		"{{quota_remaining}}": data.QuotaRemaining,
		"{{amount}}":          data.Amount,
		"{{invoice_number}}":  data.InvoiceNumber,
		"{{company_name}}":    data.CompanyName,
	}

	result := template
	for key, value := range replacements {
		result = strings.ReplaceAll(result, key, value)
	}

	// Replace custom data
	for key, value := range data.CustomData {
		result = strings.ReplaceAll(result, "{{"+key+"}}", value)
	}

	return result
}

func (m *NotificationManager) logNotification(notifType NotificationType, data *NotificationData, errors []string) {
	status := "sent"
	errorMsg := ""
	if len(errors) > 0 {
		status = "failed"
		errorMsg = strings.Join(errors, "; ")
	}

	now := time.Now()

	// Log to database using CommunicationLog
	commLog := models.CommunicationLog{
		Type:         string(notifType),
		Recipient:    data.Email,
		Subject:      m.getNotificationSubject(notifType, data),
		Status:       status,
		ErrorMessage: errorMsg,
		SubscriberID: &data.SubscriberID,
		CreatedAt:    now,
		SentAt:       &now,
	}
	database.DB.Create(&commLog)
}

func (m *NotificationManager) getSettingBool(key string) bool {
	var setting models.SystemPreference
	if database.DB.Where("key = ?", key).First(&setting).Error != nil {
		return false
	}
	return setting.Value == "true" || setting.Value == "1"
}

func (m *NotificationManager) getSettingString(key string) string {
	var setting models.SystemPreference
	if database.DB.Where("key = ?", key).First(&setting).Error != nil {
		return ""
	}
	return setting.Value
}

func (m *NotificationManager) getCompanySetting(key, defaultValue string) string {
	value := m.getSettingString(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// SendExpiryReminders sends reminders to subscribers expiring soon
func (m *NotificationManager) SendExpiryReminders(daysBeforeExpiry int) error {
	targetDate := time.Now().AddDate(0, 0, daysBeforeExpiry)
	startOfDay := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, targetDate.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	var subscribers []models.Subscriber
	if err := database.DB.Preload("Service").
		Where("expiry_date >= ? AND expiry_date < ? AND is_active = ?", startOfDay, endOfDay, true).
		Find(&subscribers).Error; err != nil {
		return err
	}

	for _, sub := range subscribers {
		data := &NotificationData{
			SubscriberID:  sub.ID,
			Username:      sub.Username,
			FullName:      sub.FullName,
			Email:         sub.Email,
			Phone:         sub.Phone,
			ServiceName:   sub.Service.Name,
			ExpiryDate:    sub.ExpiryDate.Format("2006-01-02"),
			DaysRemaining: daysBeforeExpiry,
		}
		m.SendNotification(NotifyExpiryReminder, data)
	}

	return nil
}

// GetEmailService returns the email service
func (m *NotificationManager) GetEmailService() *EmailService {
	return m.email
}

// GetSMSService returns the SMS service
func (m *NotificationManager) GetSMSService() *SMSService {
	return m.sms
}

// GetWhatsAppService returns the WhatsApp service
func (m *NotificationManager) GetWhatsAppService() *WhatsAppService {
	return m.whatsapp
}
