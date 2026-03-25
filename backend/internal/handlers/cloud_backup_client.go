package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/config"
)

// CloudBackupClientHandler communicates with the license server cloud backup API
type CloudBackupClientHandler struct {
	cfg *config.Config
}

// NewCloudBackupClientHandler creates a new CloudBackupClientHandler
func NewCloudBackupClientHandler(cfg *config.Config) *CloudBackupClientHandler {
	return &CloudBackupClientHandler{cfg: cfg}
}

// licenseServerURL returns the license server base URL from env with fallback
func (h *CloudBackupClientHandler) licenseServerURL() string {
	server := os.Getenv("LICENSE_SERVER")
	if server == "" {
		server = "https://license.proxrad.com"
	}
	return server
}

// licenseKey returns the license key from env
func (h *CloudBackupClientHandler) licenseKey() string {
	return os.Getenv("LICENSE_KEY")
}

// newHTTPClient returns an HTTP client with the given timeout
func (h *CloudBackupClientHandler) newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

// doRequest performs a generic HTTP request to the license server with the
// X-License-Key header set, reads the response body, and returns it along
// with the HTTP status code.
func (h *CloudBackupClientHandler) doRequest(method, path string, body io.Reader, contentType string, timeout time.Duration) (int, []byte, error) {
	url := h.licenseServerURL() + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-License-Key", h.licenseKey())
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	client := h.newHTTPClient(timeout)
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return resp.StatusCode, respBody, nil
}

// CloudUsageData represents the storage usage returned by the license server
type CloudUsageData struct {
	UsedBytes     int64   `json:"used_bytes"`
	QuotaBytes    int64   `json:"quota_bytes"`
	Tier          string  `json:"tier"`
	BackupCount   int     `json:"backup_count"`
	UsagePercent  float64 `json:"usage_percent"`
}

// GetUsage calls GET {LICENSE_SERVER}/api/v1/cloud-backup/usage and returns usage data.
// License server returns the usage under the "usage" key; we re-map it to "data" for the frontend.
func (h *CloudBackupClientHandler) GetUsage(c *fiber.Ctx) error {
	statusCode, body, err := h.doRequest("GET", "/api/v1/cloud-backup/usage", nil, "", 30*time.Second)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to reach cloud backup service: %v", err),
		})
	}

	if statusCode != http.StatusOK {
		return c.Status(statusCode).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Cloud backup service returned status %d", statusCode),
		})
	}

	// License server returns { "success": true, "usage": { ... } }
	var result struct {
		Success bool           `json:"success"`
		Usage   CloudUsageData `json:"usage"`
		Message string         `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to parse cloud backup usage response",
		})
	}

	if !result.Success {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"success": false,
			"message": result.Message,
		})
	}

	// Frontend expects { "success": true, "data": { ... } }
	return c.JSON(fiber.Map{
		"success": true,
		"data":    result.Usage,
	})
}

// List calls GET {LICENSE_SERVER}/api/v1/cloud-backup/list and returns the list of cloud backups.
// License server returns { "success": true, "backups": [...] }; we re-map to "data" for the frontend.
func (h *CloudBackupClientHandler) List(c *fiber.Ctx) error {
	statusCode, body, err := h.doRequest("GET", "/api/v1/cloud-backup/list", nil, "", 30*time.Second)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to reach cloud backup service: %v", err),
		})
	}

	if statusCode != http.StatusOK {
		return c.Status(statusCode).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Cloud backup service returned status %d", statusCode),
		})
	}

	// License server returns { "success": true, "backups": [...] }
	var result struct {
		Success bool                     `json:"success"`
		Backups []map[string]interface{} `json:"backups"`
		Message string                   `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to parse cloud backup list response",
		})
	}

	if !result.Success {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"success": false,
			"message": result.Message,
		})
	}

	// Frontend expects { "success": true, "data": [...] }
	backups := result.Backups
	if backups == nil {
		backups = []map[string]interface{}{}
	}

	// Remap size_bytes → size so the frontend formatBytes() function works
	for i := range backups {
		if sizeBytes, ok := backups[i]["size_bytes"]; ok {
			backups[i]["size"] = sizeBytes
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    backups,
	})
}

// Upload reads a local backup file and streams it to the license server as raw
// application/octet-stream. The license server expects:
//   - X-License-Key header
//   - X-Filename header  (basename of the file)
//   - Content-Length header
//   - Raw binary body
func (h *CloudBackupClientHandler) Upload(c *fiber.Ctx) error {
	filename := c.Params("filename")
	if filename == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Filename parameter is required",
		})
	}

	// Sanitize: only allow simple filenames, no path traversal
	if filename != filepath.Base(filename) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid filename",
		})
	}

	localPath := filepath.Join("/var/backups/proisp", filename)
	f, err := os.Open(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": fmt.Sprintf("Backup file not found: %s", filename),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to open backup file: %v", err),
		})
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to stat backup file: %v", err),
		})
	}

	url := h.licenseServerURL() + "/api/v1/cloud-backup/upload"
	req, err := http.NewRequest("POST", url, f)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to create upload request: %v", err),
		})
	}

	req.Header.Set("X-License-Key", h.licenseKey())
	req.Header.Set("X-Filename", filename)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = info.Size()

	// 30-minute timeout for large backup uploads
	client := h.newHTTPClient(30 * time.Minute)
	resp, err := client.Do(req)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Upload to cloud backup service failed: %v", err),
		})
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to read upload response",
		})
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// Try to extract message from license server response
		var errResult struct {
			Message string `json:"message"`
		}
		if jsonErr := json.Unmarshal(respBody, &errResult); jsonErr == nil && errResult.Message != "" {
			return c.Status(resp.StatusCode).JSON(fiber.Map{
				"success": false,
				"message": errResult.Message,
			})
		}
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Cloud backup service returned status %d", resp.StatusCode),
		})
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to parse upload response",
		})
	}

	return c.JSON(result)
}

// Delete calls DELETE {LICENSE_SERVER}/api/v1/cloud-backup/{backup_id}.
// The backup_id is taken from the :backup_id URL parameter.
func (h *CloudBackupClientHandler) Delete(c *fiber.Ctx) error {
	backupID := c.Params("backup_id")
	if backupID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "backup_id parameter is required",
		})
	}

	path := fmt.Sprintf("/api/v1/cloud-backup/%s", backupID)
	statusCode, body, err := h.doRequest("DELETE", path, nil, "", 30*time.Second)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to reach cloud backup service: %v", err),
		})
	}

	if statusCode != http.StatusOK && statusCode != http.StatusNoContent {
		return c.Status(statusCode).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Cloud backup service returned status %d", statusCode),
		})
	}

	// For 204 No Content the body will be empty
	if len(body) == 0 {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Cloud backup deleted successfully",
		})
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		// Non-JSON body but successful status code — treat as success
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Cloud backup deleted successfully",
		})
	}

	return c.JSON(result)
}

// Download streams a cloud backup from the license server to a local temp file,
// then issues a one-time download token (reusing the existing token system from
// backup.go) so the frontend can download it via the public-download endpoint.
func (h *CloudBackupClientHandler) Download(c *fiber.Ctx) error {
	backupID := c.Params("backup_id")
	if backupID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "backup_id parameter is required",
		})
	}

	// Stream the file from the license server
	url := h.licenseServerURL() + fmt.Sprintf("/api/v1/cloud-backup/download/%s", backupID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to create download request: %v", err),
		})
	}
	req.Header.Set("X-License-Key", h.licenseKey())

	client := h.newHTTPClient(5 * time.Minute)
	resp, err := client.Do(req)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to download from cloud backup service: %v", err),
		})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Cloud backup service returned status %d: %s", resp.StatusCode, string(errBody)),
		})
	}

	// Determine filename from Content-Disposition header or use backup_id
	filename := backupID
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		// Simple parse: look for filename=
		const prefix = "filename="
		if idx := len(cd) - len(prefix); idx > 0 {
			for i := 0; i+len(prefix) <= len(cd); i++ {
				if cd[i:i+len(prefix)] == prefix {
					name := cd[i+len(prefix):]
					// Strip surrounding quotes if present
					if len(name) >= 2 && name[0] == '"' && name[len(name)-1] == '"' {
						name = name[1 : len(name)-1]
					}
					if name != "" {
						filename = name
					}
					break
				}
			}
		}
	}

	// Save to local temp directory (reuse backup dir)
	backupDir := "/var/backups/proisp"
	os.MkdirAll(backupDir, 0755)
	tempFilename := fmt.Sprintf("cloud_download_%s_%s", backupID, filename)
	tempPath := filepath.Join(backupDir, tempFilename)

	outFile, err := os.Create(tempPath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to create temp file: %v", err),
		})
	}

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		outFile.Close()
		os.Remove(tempPath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to save downloaded backup: %v", err),
		})
	}
	outFile.Close()

	// Issue a one-time download token using the existing token system
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	downloadTokenMutex.Lock()
	downloadTokens[token] = downloadTokenInfo{
		Filename:  tempFilename,
		ExpiresAt: time.Now().Add(5 * time.Minute),
		UserID:    0, // cloud download tokens are not user-bound
	}
	downloadTokenMutex.Unlock()

	go cleanupExpiredTokens()

	return c.JSON(fiber.Map{
		"success":  true,
		"token":    token,
		"url":      fmt.Sprintf("/api/backups/public-download/%s", token),
		"filename": filename,
	})
}

// TestConnection verifies connectivity to the license server cloud backup API
// by calling GetUsage internally and returning the usage data on success.
func (h *CloudBackupClientHandler) TestConnection(c *fiber.Ctx) error {
	statusCode, body, err := h.doRequest("GET", "/api/v1/cloud-backup/usage", nil, "", 30*time.Second)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Cloud backup service is unreachable: %v", err),
		})
	}

	if statusCode != http.StatusOK {
		// Try to extract a message from the response body
		var errResult struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &errResult)
		msg := errResult.Message
		if msg == "" {
			msg = fmt.Sprintf("Cloud backup service returned HTTP %d", statusCode)
		}
		return c.Status(statusCode).JSON(fiber.Map{
			"success": false,
			"message": msg,
		})
	}

	var result struct {
		Success bool           `json:"success"`
		Data    CloudUsageData `json:"data"`
		Message string         `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Connected but failed to parse cloud backup response",
		})
	}

	if !result.Success {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"success": false,
			"message": result.Message,
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Cloud backup connected",
		"data":    result.Data,
	})
}

// cloudBackupBuildRequest is a helper used internally to build an *http.Request
// with JSON body for methods that need it (e.g. future PATCH calls).
func cloudBackupBuildRequest(method, url, licenseKey string, payload interface{}) (*http.Request, error) {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to encode request payload: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-License-Key", licenseKey)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}
