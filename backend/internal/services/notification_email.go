package services

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"os"
	"strings"

	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

// EmailService handles sending emails via SMTP
type EmailService struct{}

// NewEmailService creates a new email service
func NewEmailService() *EmailService {
	return &EmailService{}
}

// EmailConfig holds SMTP configuration
type EmailConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	FromName string
	FromAddr string
}

// GetConfig retrieves email configuration from database
func (s *EmailService) GetConfig() (*EmailConfig, error) {
	settings := make(map[string]string)
	keys := []string{"smtp_host", "smtp_port", "smtp_username", "smtp_password", "smtp_from_name", "smtp_from_email", "notification_email"}

	for _, key := range keys {
		var setting models.SystemPreference
		if err := database.DB.Where("key = ?", key).First(&setting).Error; err == nil {
			settings[key] = setting.Value
		}
	}

	// Fall back to environment variables if DB settings are empty
	if settings["smtp_host"] == "" {
		settings["smtp_host"] = os.Getenv("SMTP_HOST")
		settings["smtp_port"] = os.Getenv("SMTP_PORT")
		settings["smtp_username"] = os.Getenv("SMTP_USERNAME")
		settings["smtp_password"] = os.Getenv("SMTP_PASSWORD")
		settings["smtp_from_name"] = os.Getenv("SMTP_FROM_NAME")
		settings["smtp_from_email"] = os.Getenv("SMTP_FROM_EMAIL")
	}

	if settings["smtp_host"] == "" {
		return nil, fmt.Errorf("SMTP host not configured")
	}

	fromAddr := settings["smtp_from_email"]
	if fromAddr == "" {
		fromAddr = settings["smtp_username"]
	}
	if fromAddr == "" {
		fromAddr = settings["notification_email"]
	}

	return &EmailConfig{
		Host:     settings["smtp_host"],
		Port:     settings["smtp_port"],
		Username: settings["smtp_username"],
		Password: settings["smtp_password"],
		FromName: settings["smtp_from_name"],
		FromAddr: fromAddr,
	}, nil
}

// SendEmail sends an email
func (s *EmailService) SendEmail(to, subject, body string, isHTML bool) error {
	config, err := s.GetConfig()
	if err != nil {
		return err
	}

	return s.SendEmailWithConfig(config, to, subject, body, isHTML)
}

// SendEmailWithConfig sends an email with specific config (useful for testing)
func (s *EmailService) SendEmailWithConfig(config *EmailConfig, to, subject, body string, isHTML bool) error {
	if config.Host == "" || config.Port == "" {
		return fmt.Errorf("SMTP not configured")
	}

	// Build email headers
	from := config.FromAddr
	if config.FromName != "" {
		from = fmt.Sprintf("%s <%s>", config.FromName, config.FromAddr)
	}

	contentType := "text/plain"
	if isHTML {
		contentType = "text/html"
	}

	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: %s; charset=UTF-8\r\n"+
		"\r\n"+
		"%s", from, to, subject, contentType, body)

	addr := fmt.Sprintf("%s:%s", config.Host, config.Port)

	// Determine if we should use TLS
	port := config.Port
	useTLS := port == "465"
	useStartTLS := port == "587" || port == "25"

	var auth smtp.Auth
	if config.Username != "" && config.Password != "" {
		auth = smtp.PlainAuth("", config.Username, config.Password, config.Host)
	}

	if useTLS {
		// Direct TLS connection (port 465)
		return s.sendWithTLS(addr, config, auth, to, []byte(msg))
	} else if useStartTLS {
		// STARTTLS connection (port 587)
		return s.sendWithStartTLS(addr, config, auth, to, []byte(msg))
	} else {
		// Plain connection
		return smtp.SendMail(addr, auth, config.FromAddr, []string{to}, []byte(msg))
	}
}

// sendWithTLS sends email using direct TLS (port 465)
func (s *EmailService) sendWithTLS(addr string, config *EmailConfig, auth smtp.Auth, to string, msg []byte) error {
	tlsConfig := &tls.Config{
		ServerName: config.Host,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS dial failed: %v", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, config.Host)
	if err != nil {
		return fmt.Errorf("SMTP client failed: %v", err)
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %v", err)
		}
	}

	if err := client.Mail(config.FromAddr); err != nil {
		return fmt.Errorf("MAIL FROM failed: %v", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO failed: %v", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA failed: %v", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("Write failed: %v", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("Close failed: %v", err)
	}

	return client.Quit()
}

// sendWithStartTLS sends email using STARTTLS (port 587)
func (s *EmailService) sendWithStartTLS(addr string, config *EmailConfig, auth smtp.Auth, to string, msg []byte) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("SMTP dial failed: %v", err)
	}
	defer client.Close()

	// Say hello
	if err := client.Hello("localhost"); err != nil {
		return fmt.Errorf("HELLO failed: %v", err)
	}

	// Start TLS
	tlsConfig := &tls.Config{
		ServerName: config.Host,
	}
	if err := client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("STARTTLS failed: %v", err)
	}

	// Authenticate
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %v", err)
		}
	}

	// Send email
	if err := client.Mail(config.FromAddr); err != nil {
		return fmt.Errorf("MAIL FROM failed: %v", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO failed: %v", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA failed: %v", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("Write failed: %v", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("Close failed: %v", err)
	}

	return client.Quit()
}

// TestConnection tests the SMTP connection
func (s *EmailService) TestConnection(config *EmailConfig) error {
	if config.Host == "" {
		return fmt.Errorf("SMTP host is required")
	}
	if config.Port == "" {
		return fmt.Errorf("SMTP port is required")
	}

	addr := fmt.Sprintf("%s:%s", config.Host, config.Port)
	port := config.Port

	if port == "465" {
		// Test TLS connection
		tlsConfig := &tls.Config{
			ServerName: config.Host,
		}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("TLS connection failed: %v", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, config.Host)
		if err != nil {
			return fmt.Errorf("SMTP client failed: %v", err)
		}
		defer client.Close()

		if config.Username != "" && config.Password != "" {
			auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("Authentication failed: %v", err)
			}
		}

		return client.Quit()
	} else {
		// Test STARTTLS or plain connection
		client, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("Connection failed: %v", err)
		}
		defer client.Close()

		if port == "587" || port == "25" {
			tlsConfig := &tls.Config{
				ServerName: config.Host,
			}
			if err := client.StartTLS(tlsConfig); err != nil {
				return fmt.Errorf("STARTTLS failed: %v", err)
			}
		}

		if config.Username != "" && config.Password != "" {
			auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("Authentication failed: %v", err)
			}
		}

		return client.Quit()
	}
}

// SendTestEmail sends a test email
func (s *EmailService) SendTestEmail(config *EmailConfig, toEmail string) error {
	subject := "ProxPanel - Test Email"
	body := `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #3b82f6; color: white; padding: 20px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { background: #f9fafb; padding: 20px; border: 1px solid #e5e7eb; border-top: none; border-radius: 0 0 8px 8px; }
        .success { color: #059669; font-weight: bold; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>ProxPanel</h1>
        </div>
        <div class="content">
            <h2>SMTP Configuration Test</h2>
            <p class="success">✓ Your email configuration is working correctly!</p>
            <p>This is a test email sent from your ProxPanel system to verify that SMTP settings are configured properly.</p>
            <hr>
            <p><small>If you received this email, your notification system is ready to send automated messages.</small></p>
        </div>
    </div>
</body>
</html>
`
	return s.SendEmailWithConfig(config, toEmail, subject, strings.TrimSpace(body), true)
}
