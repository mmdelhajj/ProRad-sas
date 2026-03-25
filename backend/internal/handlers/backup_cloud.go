package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// CloudBackupInfo represents a backup stored on the license server cloud
type CloudBackupInfo struct {
	ID               uint       `json:"id"`
	BackupID         string     `json:"backup_id"`
	Filename         string     `json:"filename"`
	SizeBytes        int64      `json:"size_bytes"`
	CreatedAt        time.Time  `json:"created_at"`
	DownloadCount    int        `json:"download_count"`
	LastDownloadedAt *time.Time `json:"last_downloaded_at"`
	Status           string     `json:"status"`
}

// CloudUsageInfo represents cloud storage usage stats
type CloudUsageInfo struct {
	UsedBytes   int64   `json:"used_bytes"`
	QuotaBytes  int64   `json:"quota_bytes"`
	BackupCount int     `json:"backup_count"`
	UsedPercent float64 `json:"used_percent"`
}

// getLicenseServerURL returns the license server base URL from env
func getLicenseServerURL() string {
	url := os.Getenv("LICENSE_SERVER")
	if url == "" {
		url = "https://license.proxrad.com"
	}
	return url
}

// getLicenseKey returns the license key from env
func getLicenseKey() string {
	return os.Getenv("LICENSE_KEY")
}

// cloudAPIRequest creates and executes an HTTP request to the license server cloud backup API.
// It automatically injects the X-License-Key header and respects the provided extra headers.
func (h *BackupHandler) cloudAPIRequest(method, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	url := getLicenseServerURL() + path

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Always inject license key
	req.Header.Set("X-License-Key", getLicenseKey())

	// Apply any additional headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Use a 5-minute timeout to accommodate large file uploads/downloads
	client := &http.Client{Timeout: 5 * time.Minute}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}

	return resp, nil
}

// isValidBackupID validates that a backup_id contains only alphanumeric characters
// and hyphens, and is no longer than 64 characters.
func isValidBackupID(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9\-]+$`, id)
	return matched
}

// CloudList returns the list of cloud backups stored on the license server.
// GET /api/backups/cloud/list
func (h *BackupHandler) CloudList(c *fiber.Ctx) error {
	resp, err := h.cloudAPIRequest("GET", "/api/v1/cloud-backup/list", nil, nil)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to reach cloud backup service: %v", err),
		})
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to read cloud backup list response",
		})
	}

	c.Set("Content-Type", "application/json")
	return c.Status(resp.StatusCode).Send(body)
}

// CloudUsage returns cloud storage usage stats from the license server.
// GET /api/backups/cloud/usage
func (h *BackupHandler) CloudUsage(c *fiber.Ctx) error {
	resp, err := h.cloudAPIRequest("GET", "/api/v1/cloud-backup/usage", nil, nil)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to reach cloud backup service: %v", err),
		})
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to read cloud usage response",
		})
	}

	c.Set("Content-Type", "application/json")
	return c.Status(resp.StatusCode).Send(body)
}

// CloudUpload uploads a local backup file to the license server cloud storage.
// POST /api/backups/:filename/cloud-upload
func (h *BackupHandler) CloudUpload(c *fiber.Ctx) error {
	filename := c.Params("filename")

	// Security: only allow .proisp.bak files
	if !strings.HasSuffix(filename, ".proisp.bak") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Only .proisp.bak backup files can be uploaded to cloud",
		})
	}

	// Sanitize filename to prevent path traversal
	// filepath.Base strips any directory components
	safeFilename := filepath.Base(filename)
	if safeFilename == "." || safeFilename == "/" || safeFilename != filename {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid filename",
		})
	}

	localPath := filepath.Join(h.backupDir, safeFilename)

	// Check the file exists locally
	info, err := os.Stat(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": fmt.Sprintf("Backup file not found: %s", safeFilename),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to stat backup file: %v", err),
		})
	}

	if info.IsDir() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Path is a directory, not a file",
		})
	}

	// Open the file for streaming upload
	file, err := os.Open(localPath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to open backup file: %v", err),
		})
	}
	defer file.Close()

	extraHeaders := map[string]string{
		"X-Filename":     safeFilename,
		"Content-Type":   "application/octet-stream",
		"Content-Length": fmt.Sprintf("%d", info.Size()),
	}

	resp, err := h.cloudAPIRequest("POST", "/api/v1/cloud-backup/upload", file, extraHeaders)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to upload to cloud: %v", err),
		})
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to read upload response",
		})
	}

	c.Set("Content-Type", "application/json")
	return c.Status(resp.StatusCode).Send(body)
}

// CloudDownload downloads a cloud backup from the license server and streams it to the browser.
// GET /api/backups/cloud/download/:backup_id
func (h *BackupHandler) CloudDownload(c *fiber.Ctx) error {
	backupID := c.Params("backup_id")

	if !isValidBackupID(backupID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid backup ID (only alphanumeric and hyphens allowed, max 64 chars)",
		})
	}

	resp, err := h.cloudAPIRequest(
		"GET",
		fmt.Sprintf("/api/v1/cloud-backup/download/%s", backupID),
		nil,
		nil,
	)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to reach cloud backup service: %v", err),
		})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Forward error body as JSON
		body, _ := io.ReadAll(resp.Body)
		c.Set("Content-Type", "application/json")
		return c.Status(resp.StatusCode).Send(body)
	}

	// Determine filename for Content-Disposition
	downloadFilename := resp.Header.Get("X-Filename")
	if downloadFilename == "" {
		// Build a sensible fallback name
		downloadFilename = fmt.Sprintf("cloud_backup_%s.proisp.bak", backupID)
	}
	// Sanitize the filename (strip any directory components from server response)
	downloadFilename = filepath.Base(downloadFilename)

	c.Set("Content-Type", "application/octet-stream")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, downloadFilename))

	// Forward Content-Length if available
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		c.Set("Content-Length", cl)
	}

	// Stream response body directly to client
	if _, err := io.Copy(c.Response().BodyWriter(), resp.Body); err != nil {
		// At this point headers are already sent; log only
		return nil
	}

	return nil
}

// CloudDelete deletes a cloud backup from the license server.
// DELETE /api/backups/cloud/:backup_id
func (h *BackupHandler) CloudDelete(c *fiber.Ctx) error {
	backupID := c.Params("backup_id")

	if !isValidBackupID(backupID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid backup ID (only alphanumeric and hyphens allowed, max 64 chars)",
		})
	}

	resp, err := h.cloudAPIRequest(
		"DELETE",
		fmt.Sprintf("/api/v1/cloud-backup/%s", backupID),
		nil,
		nil,
	)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to reach cloud backup service: %v", err),
		})
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to read delete response",
		})
	}

	c.Set("Content-Type", "application/json")
	return c.Status(resp.StatusCode).Send(body)
}
