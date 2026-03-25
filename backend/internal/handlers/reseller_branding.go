package handlers

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/middleware"
	"github.com/proisp/backend/internal/models"
)

type ResellerBrandingHandler struct{}

func NewResellerBrandingHandler() *ResellerBrandingHandler {
	return &ResellerBrandingHandler{}
}

func (h *ResellerBrandingHandler) getReseller(c *fiber.Ctx) (*models.Reseller, error) {
	user := middleware.GetCurrentUser(c)
	if user == nil || user.ResellerID == nil {
		return nil, fmt.Errorf("not a reseller account")
	}
	var reseller models.Reseller
	if err := database.DB.First(&reseller, *user.ResellerID).Error; err != nil {
		return nil, fmt.Errorf("reseller not found")
	}
	if !reseller.RebrandEnabled {
		return nil, fmt.Errorf("rebranding not enabled for your account")
	}
	return &reseller, nil
}

// GetBranding returns the reseller's current branding settings
func (h *ResellerBrandingHandler) GetBranding(c *fiber.Ctx) error {
	reseller, err := h.getReseller(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	var branding models.ResellerBranding
	if err := database.DB.Where("reseller_id = ?", reseller.ID).First(&branding).Error; err != nil {
		// Return defaults if no branding set yet
		serverIP := os.Getenv("SERVER_IP")
		hasPublicIP := isPublicIP(serverIP)
		return c.JSON(fiber.Map{
			"success":              true,
			"company_name":         reseller.Name,
			"logo_path":            "",
			"primary_color":        "#2563eb",
			"footer_text":          "",
			"tagline":              "",
			"server_ip":            serverIP,
			"server_has_public_ip": hasPublicIP,
		})
	}

	logoURL := ""
	if branding.LogoPath != "" {
		logoURL = "/uploads/" + filepath.Base(branding.LogoPath)
	}

	// Detect server public IP
	serverIP := os.Getenv("SERVER_IP")
	hasPublicIP := isPublicIP(serverIP)

	return c.JSON(fiber.Map{
		"success":              true,
		"company_name":         branding.CompanyName,
		"logo_path":            logoURL,
		"primary_color":        branding.PrimaryColor,
		"footer_text":          branding.FooterText,
		"tagline":              branding.Tagline,
		"server_ip":            serverIP,
		"server_has_public_ip": hasPublicIP,
	})
}

// UpdateBranding updates the reseller's branding text settings
func (h *ResellerBrandingHandler) UpdateBranding(c *fiber.Ctx) error {
	reseller, err := h.getReseller(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	var req struct {
		CompanyName  string `json:"company_name"`
		PrimaryColor string `json:"primary_color"`
		FooterText   string `json:"footer_text"`
		Tagline      string `json:"tagline"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "invalid request"})
	}

	var branding models.ResellerBranding
	database.DB.Where("reseller_id = ?", reseller.ID).First(&branding)

	branding.ResellerID = reseller.ID
	if req.CompanyName != "" {
		branding.CompanyName = req.CompanyName
	}
	if req.PrimaryColor != "" {
		branding.PrimaryColor = req.PrimaryColor
	}
	branding.FooterText = req.FooterText
	branding.Tagline = req.Tagline

	database.DB.Save(&branding)

	return c.JSON(fiber.Map{"success": true, "message": "Branding updated"})
}

// UploadLogo uploads the reseller's logo
func (h *ResellerBrandingHandler) UploadLogo(c *fiber.Ctx) error {
	reseller, err := h.getReseller(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	file, err := c.FormFile("logo")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "No file uploaded"})
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".svg": true, ".webp": true}
	if !allowed[ext] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Only PNG, JPG, JPEG, SVG, WEBP allowed"})
	}
	if file.Size > 2*1024*1024 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "File too large (max 2MB)"})
	}

	uploadDir := "/app/uploads"
	os.MkdirAll(uploadDir, 0755)

	// Delete old logo
	var branding models.ResellerBranding
	database.DB.Where("reseller_id = ?", reseller.ID).First(&branding)
	if branding.LogoPath != "" {
		os.Remove(branding.LogoPath)
	}

	filename := fmt.Sprintf("reseller_%d_logo_%s%s", reseller.ID, uuid.New().String()[:8], ext)
	savePath := filepath.Join(uploadDir, filename)
	if err := c.SaveFile(file, savePath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to save file"})
	}

	branding.ResellerID = reseller.ID
	branding.LogoPath = savePath
	database.DB.Save(&branding)

	return c.JSON(fiber.Map{
		"success":  true,
		"logo_url": "/uploads/" + filename,
		"message":  "Logo uploaded",
	})
}

// DeleteLogo removes the reseller's logo
func (h *ResellerBrandingHandler) DeleteLogo(c *fiber.Ctx) error {
	reseller, err := h.getReseller(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	var branding models.ResellerBranding
	if err := database.DB.Where("reseller_id = ?", reseller.ID).First(&branding).Error; err != nil {
		return c.JSON(fiber.Map{"success": true})
	}

	if branding.LogoPath != "" {
		os.Remove(branding.LogoPath)
		database.DB.Model(&branding).Update("logo_path", "")
	}

	return c.JSON(fiber.Map{"success": true, "message": "Logo deleted"})
}

// RequestSSL issues a Let's Encrypt SSL certificate for the reseller's custom domain.
// Streams output so frontend can show progress.
func (h *ResellerBrandingHandler) RequestSSL(c *fiber.Ctx) error {
	reseller, err := h.getReseller(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	if reseller.CustomDomain == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "No custom domain configured. Set your domain first.",
		})
	}

	var req struct {
		Email string `json:"email"`
	}
	c.BodyParser(&req)
	if req.Email == "" {
		req.Email = "admin@" + reseller.CustomDomain
	}

	domain := reseller.CustomDomain

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")

	sendLine := func(msg string) {
		fmt.Fprintf(c, "data: %s\n\n", msg)
	}

	sendLine("🔍 Checking if domain " + domain + " points to this server...")

	// Step 1: Run certbot via nsenter (API container has privileged + pid:host access)
	// Use webroot mode - nginx serves .well-known/acme-challenge/ from /usr/share/nginx/html
	// On host, the webroot is /opt/proxpanel/frontend/dist
	sendLine("📦 Running certbot to obtain SSL certificate...")

	cmd := exec.Command("nsenter", "-t", "1", "-m", "-u", "-i", "-n", "-p", "--",
		"sh", "-c",
		fmt.Sprintf(
			"which certbot || apt-get install -y certbot > /dev/null 2>&1; "+
				"certbot certonly --webroot -w /opt/proxpanel/frontend/dist -d %s "+
				"--non-interactive --agree-tos --email %s 2>&1",
			domain, req.Email,
		),
	)

	out, certErr := cmd.CombinedOutput()
	for _, line := range strings.Split(string(out), "\n") {
		if line != "" {
			sendLine(line)
		}
	}

	if certErr != nil {
		sendLine("❌ certbot failed: " + certErr.Error())
		sendLine("💡 Make sure your domain's A record points to this server's IP and port 80 is reachable.")
		return nil
	}

	sendLine("✅ Certificate obtained!")

	// Step 2: Build nginx SSL server block
	sslBlock := fmt.Sprintf(`
server {
    listen 443 ssl;
    server_name %s;
    ssl_certificate /etc/letsencrypt/live/%s/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/%s/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    root /usr/share/nginx/html;
    index index.html;
    gzip on;
    location /api/ {
        proxy_pass http://proxpanel-api:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
    location ^~ /uploads/ {
        proxy_pass http://proxpanel-api:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
    location = /index.html {
        add_header Cache-Control "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0";
        expires -1;
    }
    location / {
        try_files $uri $uri/ /index.html;
    }
    location ~* \.(?:js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
        try_files $uri =404;
    }
}
`, domain, domain, domain)

	// Step 3: Update host nginx.conf to add SSL block + letsencrypt mount awareness
	nginxConfPath := "/opt/proxpanel/frontend/nginx.conf"
	sendLine("📝 Updating nginx configuration...")

	appendCmd := exec.Command("nsenter", "-t", "1", "-m", "-u", "-i", "-n", "-p", "--",
		"sh", "-c",
		fmt.Sprintf(
			// Remove any existing block for this domain, then append new block
			"grep -q 'server_name %s;' %s && "+
				"sed -i '/server {/{/server_name %s;/{:a;N;/}/!ba;d}}' %s; "+
				"echo '%s' >> %s",
			domain, nginxConfPath,
			domain, nginxConfPath,
			strings.ReplaceAll(sslBlock, "'", "'\"'\"'"),
			nginxConfPath,
		),
	)
	if appendErr := appendCmd.Run(); appendErr != nil {
		sendLine("⚠️  Could not update nginx.conf automatically: " + appendErr.Error())
	}

	// Step 4: Update docker-compose.yml to mount letsencrypt (if not already)
	sendLine("🐳 Checking docker-compose for letsencrypt mount...")
	mountCmd := exec.Command("nsenter", "-t", "1", "-m", "-u", "-i", "-n", "-p", "--",
		"sh", "-c",
		"grep -q '/etc/letsencrypt' /opt/proxpanel/docker-compose.yml || "+
			"sed -i '/proxpanel-frontend/,/proxpanel-/{/volumes:/a\\      - /etc/letsencrypt:/etc/letsencrypt:ro}' /opt/proxpanel/docker-compose.yml",
	)
	mountCmd.Run()

	// Step 5: Restart nginx container to apply changes
	sendLine("🔄 Restarting nginx to apply SSL configuration...")
	restartCmd := exec.Command("nsenter", "-t", "1", "-m", "-u", "-i", "-n", "-p", "--",
		"sh", "-c",
		"cd /opt/proxpanel && docker compose restart frontend 2>&1 || docker restart proxpanel-frontend 2>&1",
	)
	restartOut, _ := restartCmd.CombinedOutput()
	for _, line := range strings.Split(string(restartOut), "\n") {
		if line != "" {
			sendLine(line)
		}
	}

	sendLine("🎉 SSL certificate installed! Your domain " + domain + " now supports HTTPS.")
	sendLine("DONE")

	return nil
}

// UpdateDomain saves the reseller's custom domain
func (h *ResellerBrandingHandler) UpdateDomain(c *fiber.Ctx) error {
	reseller, err := h.getReseller(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	var req struct {
		CustomDomain string `json:"custom_domain"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "invalid request"})
	}

	domain := strings.TrimSpace(strings.ToLower(req.CustomDomain))
	// Remove protocol if provided
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimSuffix(domain, "/")

	if err := database.DB.Model(&models.Reseller{}).Where("id = ?", reseller.ID).
		Update("custom_domain", domain).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to save domain"})
	}

	return c.JSON(fiber.Map{"success": true, "message": "Domain saved", "custom_domain": domain})
}

// isPublicIP returns true if the IP is a routable public IP
func isPublicIP(ip string) bool {
	if ip == "" {
		return false
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	// Private ranges
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
	}
	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(parsed) {
			return false
		}
	}
	return true
}
