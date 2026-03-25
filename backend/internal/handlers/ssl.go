package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
)

type SSLHandler struct{}

func NewSSLHandler() *SSLHandler {
	return &SSLHandler{}
}

// ServeAcmeChallenge serves Let's Encrypt HTTP-01 challenge tokens
// This route must be public (no auth) so certbot can verify
func (h *SSLHandler) ServeAcmeChallenge(c *fiber.Ctx) error {
	token := c.Params("token")
	// Sanitize token - only allow alphanumeric and dash/underscore
	for _, ch := range token {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_') {
			return c.Status(404).SendString("not found")
		}
	}
	content, err := os.ReadFile("/opt/proxpanel/acme-challenge/" + token)
	if err != nil {
		return c.Status(404).SendString("not found")
	}
	c.Set("Content-Type", "text/plain")
	return c.SendString(strings.TrimSpace(string(content)))
}

// InstallSSL installs an SSL certificate for a custom domain (streaming NDJSON)
func (h *SSLHandler) InstallSSL(c *fiber.Ctx) error {
	domain := strings.TrimSpace(c.FormValue("domain"))
	email := strings.TrimSpace(c.FormValue("email"))

	if domain == "" || email == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Domain and email are required"})
	}

	// Basic domain validation
	if strings.Contains(domain, " ") || !strings.Contains(domain, ".") {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid domain name"})
	}

	c.Set("Content-Type", "application/x-ndjson")
	c.Set("Cache-Control", "no-cache")
	c.Set("X-Accel-Buffering", "no")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		send := func(msg string) {
			data, _ := json.Marshal(map[string]string{"msg": msg})
			w.Write(data)   //nolint
			w.WriteByte('\n') //nolint
			w.Flush()       //nolint
		}
		sendFinal := func(status, extra string) {
			data, _ := json.Marshal(map[string]string{"status": status, "domain": extra})
			w.Write(data)   //nolint
			w.WriteByte('\n') //nolint
			w.Flush()       //nolint
		}

		send("🔍 Checking DNS for " + domain + "...")

		// Check DNS resolves to this server's public IP
		serverIP := os.Getenv("SERVER_IP")
		addrs, err := net.LookupHost(domain)
		if err != nil {
			send("❌ DNS lookup failed: " + err.Error())
			sendFinal("error", "")
			return
		}
		dnsIP := addrs[0]
		if serverIP != "" && dnsIP != serverIP {
			send(fmt.Sprintf("⚠️  Warning: %s resolves to %s but server IP is %s", domain, dnsIP, serverIP))
			send("⚠️  Continuing anyway — make sure DNS points to this server's public IP")
		} else {
			send(fmt.Sprintf("✓ DNS OK: %s → %s", domain, dnsIP))
		}

		// Create acme challenge directory
		acmeDir := "/opt/proxpanel/acme-challenge"
		if err := os.MkdirAll(acmeDir, 0755); err != nil {
			send("❌ Failed to create acme-challenge directory: " + err.Error())
			sendFinal("error", "")
			return
		}
		send("✓ ACME challenge directory ready")

		// Update nginx.conf to proxy /.well-known/acme-challenge/ to the API
		send("⚙️  Configuring nginx for ACME challenge...")
		if err := h.addAcmeChallengeToNginx(); err != nil {
			send("⚠️  Nginx update warning: " + err.Error())
		} else {
			// Must restart (not just reload): writeFileInPlace preserves inode,
			// but if the file was previously replaced by os.WriteFile the inode changed.
			// Restart ensures Docker bind mount picks up the current path inode.
			exec.Command("docker", "restart", "proxpanel-frontend").Run() //nolint
			time.Sleep(3 * time.Second)
			send("✓ Nginx configured for ACME challenge")
		}

		// Install certbot on host via nsenter if not installed
		send("📦 Checking certbot installation...")
		checkCmd := exec.Command("nsenter", "-t", "1", "-m", "-u", "-n", "-i", "--",
			"bash", "-c", "command -v certbot >/dev/null 2>&1 || test -f /usr/local/bin/certbot || test -f /snap/bin/certbot")
		if err := checkCmd.Run(); err != nil {
			send("📦 Installing certbot (this may take a minute)...")
			installCmd := exec.Command("nsenter", "-t", "1", "-m", "-u", "-n", "-i", "--",
				"bash", "-c", "apt-get install -y certbot 2>&1 || snap install --classic certbot 2>&1 && ln -sf /snap/bin/certbot /usr/local/bin/certbot 2>/dev/null || true")
			out, err := installCmd.CombinedOutput()
			if err != nil {
				send("❌ Failed to install certbot: " + string(out))
				sendFinal("error", "")
				return
			}
			send("✓ Certbot installed")
		} else {
			send("✓ Certbot already installed")
		}

		// Create auth/cleanup hook scripts on the HOST via nsenter
		authHookContent := "#!/bin/bash\necho \"$CERTBOT_VALIDATION\" > /opt/proxpanel/acme-challenge/$CERTBOT_TOKEN\n"
		cleanupHookContent := "#!/bin/bash\nrm -f /opt/proxpanel/acme-challenge/$CERTBOT_TOKEN\n"
		writeHookCmd := exec.Command("nsenter", "-t", "1", "-m", "-u", "-n", "-i", "--",
			"bash", "-c", fmt.Sprintf(
				"cat > /opt/proxpanel/acme-auth-hook.sh << 'HOOKEOF'\n%sHOOKEOF\nchmod +x /opt/proxpanel/acme-auth-hook.sh\ncat > /opt/proxpanel/acme-cleanup-hook.sh << 'HOOKEOF'\n%sHOOKEOF\nchmod +x /opt/proxpanel/acme-cleanup-hook.sh",
				authHookContent, cleanupHookContent))
		if out, err := writeHookCmd.CombinedOutput(); err != nil {
			send("❌ Failed to create hook scripts: " + string(out))
			sendFinal("error", "")
			return
		}
		send("✓ Hook scripts created")

		// Run certbot
		send(fmt.Sprintf("🔐 Requesting SSL certificate for %s...", domain))
		send("⏳ Let's Encrypt will verify domain ownership via HTTP...")

		certbotCmd := exec.Command("nsenter", "-t", "1", "-m", "-u", "-n", "-i", "--",
			"certbot", "certonly",
			"--manual",
			"--non-interactive",
			"--agree-tos",
			"--email", email,
			"--preferred-challenges", "http",
			"--manual-auth-hook", "/opt/proxpanel/acme-auth-hook.sh",
			"--manual-cleanup-hook", "/opt/proxpanel/acme-cleanup-hook.sh",
			"--expand",
			"-d", domain,
		)
		certbotOut, err := certbotCmd.CombinedOutput()
		certbotStr := string(certbotOut)
		if err != nil {
			send("❌ Certbot failed:")
			for _, line := range strings.Split(certbotStr, "\n") {
				if strings.TrimSpace(line) != "" {
					send("   " + line)
				}
			}
			sendFinal("error", "")
			return
		}
		send("✓ SSL certificate issued successfully!")

		// Copy certs to /opt/proxpanel/certs/
		send("📋 Copying certificates...")
		certsDir := "/opt/proxpanel/certs"
		certSrcBase := fmt.Sprintf("/etc/letsencrypt/live/%s", domain)

		// Try reading from host /proc/1/root first, then via nsenter
		hostCertSrc := fmt.Sprintf("/proc/1/root/etc/letsencrypt/live/%s", domain)
		cpCmd := exec.Command("bash", "-c", fmt.Sprintf(
			"mkdir -p %s && cp %s/fullchain.pem %s/fullchain.pem && cp %s/privkey.pem %s/privkey.pem",
			certsDir, hostCertSrc, certsDir, hostCertSrc, certsDir))
		if out, err := cpCmd.CombinedOutput(); err != nil {
			cpCmd2 := exec.Command("nsenter", "-t", "1", "-m", "-u", "-n", "-i", "--",
				"bash", "-c", fmt.Sprintf("mkdir -p %s && cp %s/fullchain.pem %s/fullchain.pem && cp %s/privkey.pem %s/privkey.pem",
					certsDir, certSrcBase, certsDir, certSrcBase, certsDir))
			if out2, err2 := cpCmd2.CombinedOutput(); err2 != nil {
				send("❌ Failed to copy certs: " + string(out) + " / " + string(out2))
				sendFinal("error", "")
				return
			}
		}
		send("✓ Certificates copied to /opt/proxpanel/certs/")

		// Write SSL nginx.conf (in-place, preserves inode so container sees update)
		send("⚙️  Configuring nginx for HTTPS...")
		if err := h.writeSSLNginxConf(domain); err != nil {
			send("❌ Failed to write nginx SSL config: " + err.Error())
			sendFinal("error", "")
			return
		}
		// Also write to frontend/nginx.conf in case old installs mount that path
		h.writeSSLNginxConfTo(domain, "/opt/proxpanel/frontend/nginx.conf") //nolint
		send("✓ nginx.conf updated for HTTPS")

		// Update docker-compose.yml to expose port 443
		if err := h.updateDockerComposeForSSL(); err != nil {
			send(fmt.Sprintf("⚠️  Warning: Could not update docker-compose.yml for port 443: %v", err))
		} else {
			send("✓ docker-compose.yml updated with port 443")
		}

		// Restart frontend container (NOT docker compose up -d — that creates new networks
		// which steal routes to MikroTik IPs in the same subnet)
		send("🔄 Restarting nginx with SSL (brief downtime ~5 seconds)...")
		exec.Command("docker", "restart", "proxpanel-frontend").Run() //nolint
		time.Sleep(3 * time.Second)
		send("✓ nginx restarted with SSL")

		// Set up auto-renewal cron
		send("⏰ Setting up auto-renewal...")
		renewScript := "#!/bin/bash\n# ProxPanel SSL auto-renewal\n" +
			"nsenter -t 1 -m -u -n -i -- certbot renew --quiet --manual " +
			"--manual-auth-hook /opt/proxpanel/acme-auth-hook.sh " +
			"--manual-cleanup-hook /opt/proxpanel/acme-cleanup-hook.sh\n" +
			"cp /proc/1/root/etc/letsencrypt/live/" + domain + "/fullchain.pem /opt/proxpanel/certs/fullchain.pem 2>/dev/null || true\n" +
			"cp /proc/1/root/etc/letsencrypt/live/" + domain + "/privkey.pem /opt/proxpanel/certs/privkey.pem 2>/dev/null || true\n" +
			"docker exec proxpanel-frontend nginx -s reload 2>/dev/null || true\n"
		renewPath := "/opt/proxpanel/ssl-renew.sh"
		if err := os.WriteFile(renewPath, []byte(renewScript), 0755); err == nil {
			cronCmd := exec.Command("nsenter", "-t", "1", "-m", "-u", "-n", "-i", "--",
				"bash", "-c", fmt.Sprintf(`(crontab -l 2>/dev/null | grep -v proxpanel-ssl; echo "0 3 * * * %s >> /var/log/proxpanel-ssl-renew.log 2>&1") | crontab -`, renewPath))
			cronCmd.Run() //nolint
			send("✓ Auto-renewal cron configured (runs daily at 3 AM)")
		}

		// Save domain to system preferences via database
		send("💾 Saving configuration...")
		database.DB.Exec("INSERT INTO system_preferences (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = ?", "custom_domain", domain, domain)
		database.DB.Exec("INSERT INTO system_preferences (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = ?", "ssl_enabled", "true", "true")

		send(fmt.Sprintf("✅ SSL configured successfully! Panel at https://%s", domain))
		sendFinal("success", domain)
	})

	return nil
}

// addAcmeChallengeToNginx adds the ACME challenge proxy location to nginx.conf
func (h *SSLHandler) addAcmeChallengeToNginx() error {
	confPath := "/opt/proxpanel/nginx.conf"
	content, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}

	conf := string(content)
	// Only add if not already present
	if strings.Contains(conf, "acme-challenge") {
		return nil
	}

	acmeBlock := `
    # ACME challenge for Let's Encrypt SSL
    location /.well-known/acme-challenge/ {
        proxy_pass http://proxpanel-api:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
    }

`
	// Insert before the first location block (always present)
	insertBefore := "    location = /index.html"
	if !strings.Contains(conf, insertBefore) {
		insertBefore = "    location /api"
	}
	if !strings.Contains(conf, insertBefore) {
		insertBefore = "    location /"
	}
	conf = strings.Replace(conf, insertBefore, acmeBlock+insertBefore, 1)

	// Write in-place to preserve inode (Docker bind mounts track inode, not path)
	return writeFileInPlace(confPath, []byte(conf))
}

// writeSSLNginxConfTo writes the SSL nginx config to a specific path (for compatibility with old installs)
func (h *SSLHandler) writeSSLNginxConfTo(domain, path string) error {
	if _, err := os.Stat(path); err != nil {
		return err // skip if file doesn't exist
	}
	content, err := os.ReadFile("/opt/proxpanel/nginx.conf")
	if err != nil {
		return err
	}
	return writeFileInPlace(path, content)
}

// updateDockerComposeForSSL adds port 443 and cert volume to docker-compose.yml
func (h *SSLHandler) updateDockerComposeForSSL() error {
	composePath := "/opt/proxpanel/docker-compose.yml"
	content, err := os.ReadFile(composePath)
	if err != nil {
		return err
	}

	conf := string(content)

	// Add port 443 if not already present
	if !strings.Contains(conf, "443:443") {
		conf = strings.Replace(conf, `      - "80:80"`, "      - \"80:80\"\n      - \"443:443\"", 1)
	}

	// Add certs volume mount if not already present
	if !strings.Contains(conf, "proxpanel/certs") {
		conf = strings.Replace(conf,
			"      - ./frontend/nginx.conf:/etc/nginx/conf.d/default.conf:ro",
			"      - ./frontend/nginx.conf:/etc/nginx/conf.d/default.conf:ro\n      - /opt/proxpanel/certs:/etc/ssl/proxpanel:ro",
			1)
	}

	return os.WriteFile(composePath, []byte(conf), 0644)
}

// writeSSLNginxConf writes a new nginx.conf with SSL support
func (h *SSLHandler) writeSSLNginxConf(domain string) error {
	confPath := "/opt/proxpanel/nginx.conf"
	existing, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}

	// Backup HTTP config
	backupPath := filepath.Join(filepath.Dir(confPath), "nginx.conf.http-backup")
	os.WriteFile(backupPath, existing, 0644) //nolint

	sslConf := fmt.Sprintf(`server {
    listen 80;
    server_name %s _;

    # ACME challenge for SSL renewal
    location /.well-known/acme-challenge/ {
        proxy_pass http://proxpanel-api:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
    }

    # Redirect all other HTTP to HTTPS
    location / {
        return 301 https://%s$request_uri;
    }
}

server {
    listen 443 ssl;
    server_name %s;
    root /usr/share/nginx/html;
    index index.html;

    ssl_certificate /etc/ssl/proxpanel/fullchain.pem;
    ssl_certificate_key /etc/ssl/proxpanel/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;

    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_proxied any;
    gzip_comp_level 6;
    gzip_min_length 1024;
    gzip_types
        text/plain
        text/css
        text/javascript
        application/javascript
        application/json
        application/x-javascript
        image/svg+xml
        font/woff
        font/woff2;

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    server_tokens off;

    # ACME challenge
    location /.well-known/acme-challenge/ {
        proxy_pass http://proxpanel-api:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
    }

    # SSL install stream
    location /api/settings/ssl-stream {
        proxy_pass http://proxpanel-api:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 300s;
        chunked_transfer_encoding off;
    }

    # Streaming API endpoints (ping-stream)
    location /api/diagnostic/ping-stream {
        proxy_pass http://proxpanel-api:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 120s;
        chunked_transfer_encoding off;
    }

    # Proxy API requests to backend
    location /api/ {
        proxy_pass http://proxpanel-api:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    # Proxy uploads
    location ^~ /uploads/ {
        proxy_pass http://proxpanel-api:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # Health check
    location /health {
        proxy_pass http://proxpanel-api:8080;
        proxy_http_version 1.1;
    }

    # index.html - NO CACHE
    location = /index.html {
        add_header Cache-Control "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0";
        expires -1;
    }

    # SPA fallback
    location / {
        try_files $uri $uri/ /index.html;
        add_header Cache-Control "no-store, no-cache, must-revalidate" always;
    }

    # Cache static assets
    location ~* \.(?:js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
        try_files $uri =404;
    }
}
`, domain, domain, domain)

	return writeFileInPlace(confPath, []byte(sslConf))
}

// writeFileInPlace writes content to a file without changing its inode.
// Docker bind mounts track inodes, so os.WriteFile (which creates a new file) would
// cause the container to keep seeing the old content. This function truncates and
// rewrites the existing file, preserving the inode so the container sees the update.
func writeFileInPlace(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

// GetSSLStatus returns current SSL/domain configuration
func (h *SSLHandler) GetSSLStatus(c *fiber.Ctx) error {
	domain := ""
	sslEnabled := false

	// Read from DB via GORM
	var prefs []struct {
		Key   string
		Value string
	}
	database.DB.Raw("SELECT key, value FROM system_preferences WHERE key IN ('custom_domain', 'ssl_enabled')").Scan(&prefs)
	for _, p := range prefs {
		if p.Key == "custom_domain" {
			domain = p.Value
		}
		if p.Key == "ssl_enabled" && p.Value == "true" {
			sslEnabled = true
		}
	}

	// Check if cert actually exists
	certExists := false
	if _, err := os.Stat("/opt/proxpanel/certs/fullchain.pem"); err == nil {
		certExists = true
	}

	return c.JSON(fiber.Map{
		"success":     true,
		"domain":      domain,
		"ssl_enabled": sslEnabled,
		"cert_exists": certExists,
	})
}
