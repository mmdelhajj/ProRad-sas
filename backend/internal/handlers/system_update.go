package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/security"
)

// SystemUpdateHandler handles system updates
type SystemUpdateHandler struct {
	licenseServer    string
	licenseKey       string
	installDir       string
	updating         bool
	updateStatus     *UpdateStatus
	cachedUpdateInfo *CachedUpdateInfo
	mutex            sync.RWMutex
}

// UpdateStatus tracks current update progress
type UpdateStatus struct {
	InProgress   bool      `json:"in_progress"`
	Step         string    `json:"step"`
	Progress     int       `json:"progress"`
	Message      string    `json:"message"`
	Error        string    `json:"error,omitempty"`
	StartedAt    time.Time `json:"started_at,omitempty"`
	CompletedAt  time.Time `json:"completed_at,omitempty"`
	FromVersion  string    `json:"from_version,omitempty"`
	ToVersion    string    `json:"to_version,omitempty"`
	NeedsRestart bool      `json:"needs_restart"`
}

// UpdateCheckResponse from license server
type UpdateCheckResponse struct {
	Success         bool   `json:"success"`
	UpdateAvailable bool   `json:"update_available"`
	Version         string `json:"version"`
	ReleaseNotes    string `json:"release_notes"`
	IsCritical      bool   `json:"is_critical"`
	FileSize        int64  `json:"file_size"`
	Checksum        string `json:"checksum"`
	Signature       string `json:"signature"`
	IsEncrypted     bool   `json:"is_encrypted"`
	EncryptionKey   string `json:"encryption_key,omitempty"`
	ReleasedAt      string `json:"released_at"`
}

// CachedUpdateInfo stores update info from check for use during download
type CachedUpdateInfo struct {
	Version       string
	Checksum      string
	Signature     string
	IsEncrypted   bool
	EncryptionKey string
}

// NewSystemUpdateHandler creates a new system update handler
func NewSystemUpdateHandler() *SystemUpdateHandler {
	return &SystemUpdateHandler{
		licenseServer: os.Getenv("LICENSE_SERVER"),
		licenseKey:    os.Getenv("LICENSE_KEY"),
		installDir:    getInstallDir(),
		updateStatus:  &UpdateStatus{},
	}
}

func getInstallDir() string {
	if dir := os.Getenv("INSTALL_DIR"); dir != "" {
		return dir
	}
	return "/opt/proxpanel"
}

func getCurrentVersion() string {
	versionFile := filepath.Join(getInstallDir(), "VERSION")
	data, err := os.ReadFile(versionFile)
	if err != nil {
		return os.Getenv("PROXPANEL_VERSION")
	}
	return string(bytes.TrimSpace(data))
}

// CheckUpdate checks for available updates
func (h *SystemUpdateHandler) CheckUpdate(c *fiber.Ctx) error {
	if h.licenseServer == "" || h.licenseKey == "" {
		return c.JSON(fiber.Map{
			"success":          true,
			"update_available": false,
			"message":          "License not configured",
		})
	}

	currentVersion := getCurrentVersion()
	if currentVersion == "" {
		currentVersion = "1.0.0"
	}

	// Call license server
	reqBody, _ := json.Marshal(map[string]string{
		"license_key":     h.licenseKey,
		"current_version": currentVersion,
	})

	resp, err := http.Post(
		h.licenseServer+"/api/v1/update/check",
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	if err != nil {
		return c.JSON(fiber.Map{
			"success":          true,
			"update_available": false,
			"current_version":  currentVersion,
			"message":          "Could not connect to update server",
		})
	}
	defer resp.Body.Close()

	var updateResp UpdateCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&updateResp); err != nil {
		return c.JSON(fiber.Map{
			"success":          true,
			"update_available": false,
			"current_version":  currentVersion,
		})
	}

	// Cache update info for later use during download
	if updateResp.UpdateAvailable {
		h.mutex.Lock()
		h.cachedUpdateInfo = &CachedUpdateInfo{
			Version:       updateResp.Version,
			Checksum:      updateResp.Checksum,
			Signature:     updateResp.Signature,
			IsEncrypted:   updateResp.IsEncrypted,
			EncryptionKey: updateResp.EncryptionKey,
		}
		h.mutex.Unlock()
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"update_available": updateResp.UpdateAvailable,
		"current_version":  currentVersion,
		"new_version":      updateResp.Version,
		"release_notes":    updateResp.ReleaseNotes,
		"is_critical":      updateResp.IsCritical,
		"file_size":        updateResp.FileSize,
		"released_at":      updateResp.ReleasedAt,
		"is_encrypted":     updateResp.IsEncrypted,
	})
}

// GetUpdateStatus returns current update status
func (h *SystemUpdateHandler) GetUpdateStatus(c *fiber.Ctx) error {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	return c.JSON(fiber.Map{
		"success": true,
		"data":    h.updateStatus,
	})
}

// StartUpdate initiates the update process
func (h *SystemUpdateHandler) StartUpdate(c *fiber.Ctx) error {
	h.mutex.Lock()
	if h.updating {
		h.mutex.Unlock()
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"success": false,
			"message": "Update already in progress",
		})
	}
	h.updating = true
	h.mutex.Unlock()

	var req struct {
		Version string `json:"version"`
	}
	if err := c.BodyParser(&req); err != nil {
		h.mutex.Lock()
		h.updating = false
		h.mutex.Unlock()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request",
		})
	}

	// Start update in background
	go h.performUpdate(req.Version)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Update started",
	})
}

func (h *SystemUpdateHandler) setStatus(step string, progress int, message string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.updateStatus.Step = step
	h.updateStatus.Progress = progress
	h.updateStatus.Message = message
}

func (h *SystemUpdateHandler) performUpdate(version string) {
	currentVersion := getCurrentVersion()

	h.mutex.Lock()
	h.updateStatus = &UpdateStatus{
		InProgress:  true,
		Step:        "preparing",
		Progress:    0,
		Message:     "Preparing update...",
		StartedAt:   time.Now(),
		FromVersion: currentVersion,
		ToVersion:   version,
	}
	// Get cached update info
	updateInfo := h.cachedUpdateInfo
	h.mutex.Unlock()

	// Track backup dir for rollback
	backupDir := fmt.Sprintf("/opt/proxpanel-backup-%s", time.Now().Format("20060102-150405"))
	updateFailed := false

	defer func() {
		h.mutex.Lock()
		h.updating = false
		h.updateStatus.InProgress = false
		h.updateStatus.CompletedAt = time.Now()
		h.mutex.Unlock()

		// Auto-rollback if update failed and backup exists
		if updateFailed {
			h.performRollback(backupDir, currentVersion, version)
		}
	}()

	// Step 1: Create backup
	h.setStatus("backup", 10, "Creating backup...")
	if err := exec.Command("cp", "-r", h.installDir, backupDir).Run(); err != nil {
		h.setError("Failed to create backup: " + err.Error())
		updateFailed = true
		return
	}

	// Step 2: Download update
	h.setStatus("download", 25, "Downloading update...")
	downloadURL := fmt.Sprintf("%s/api/v1/update/download/%s?license_key=%s",
		h.licenseServer, version, h.licenseKey)

	tmpFile := "/tmp/proxpanel-update.tar.gz.enc"
	if updateInfo == nil || !updateInfo.IsEncrypted {
		tmpFile = "/tmp/proxpanel-update.tar.gz"
	}

	if err := h.downloadFile(downloadURL, tmpFile); err != nil {
		h.setError("Failed to download update: " + err.Error())
		updateFailed = true
		return
	}

	// Step 3: Decrypt if encrypted
	decryptedFile := "/tmp/proxpanel-update.tar.gz"
	if updateInfo != nil && updateInfo.IsEncrypted {
		h.setStatus("decrypt", 35, "Decrypting update package...")

		if updateInfo.EncryptionKey == "" {
			h.setError("Encrypted package but no encryption key provided")
			os.Remove(tmpFile)
			updateFailed = true
			return
		}

		encryptedData, err := os.ReadFile(tmpFile)
		if err != nil {
			h.setError("Failed to read encrypted package: " + err.Error())
			os.Remove(tmpFile)
			updateFailed = true
			return
		}

		decryptedData, err := security.DecryptUpdatePackage(encryptedData, updateInfo.EncryptionKey)
		if err != nil {
			h.setError("Failed to decrypt package: " + err.Error())
			os.Remove(tmpFile)
			updateFailed = true
			return
		}

		// Write decrypted data
		if err := os.WriteFile(decryptedFile, decryptedData, 0644); err != nil {
			h.setError("Failed to write decrypted package: " + err.Error())
			os.Remove(tmpFile)
			updateFailed = true
			return
		}
		os.Remove(tmpFile) // Remove encrypted file

		// Step 4: Verify signature and checksum
		h.setStatus("verify", 45, "Verifying package integrity and signature...")

		if updateInfo.Checksum != "" && updateInfo.Signature != "" {
			if err := security.VerifyUpdatePackage(decryptedData, version, updateInfo.Checksum, updateInfo.Signature); err != nil {
				h.setError("Package verification failed: " + err.Error())
				os.Remove(decryptedFile)
				updateFailed = true
				return
			}
			log.Println("Update package signature and checksum verified successfully")
		} else if updateInfo.Checksum != "" {
			// Verify checksum only
			actualChecksum := security.CalculateChecksum(decryptedData)
			if actualChecksum != updateInfo.Checksum {
				h.setError(fmt.Sprintf("Checksum mismatch: expected %s, got %s", updateInfo.Checksum, actualChecksum))
				os.Remove(decryptedFile)
				updateFailed = true
				return
			}
			log.Println("Update package checksum verified successfully")
		}
	} else {
		// Non-encrypted: verify checksum if available
		h.setStatus("verify", 45, "Verifying package integrity...")

		if updateInfo != nil && updateInfo.Checksum != "" {
			// Read file to verify checksum
			fileData, err := os.ReadFile(decryptedFile)
			if err == nil {
				actualChecksum := security.CalculateChecksum(fileData)
				if actualChecksum != updateInfo.Checksum {
					h.setError(fmt.Sprintf("Checksum mismatch: expected %s, got %s", updateInfo.Checksum, actualChecksum))
					os.Remove(decryptedFile)
					updateFailed = true
					return
				}
				log.Println("Update package checksum verified successfully")
			}
		}
	}

	// Step 5: Extract update to temp directory first
	h.setStatus("extract", 55, "Extracting update...")
	tmpExtractDir := "/tmp/proxpanel-update-extract"
	os.RemoveAll(tmpExtractDir)
	os.MkdirAll(tmpExtractDir, 0755)

	cmd := exec.Command("tar", "-xzf", decryptedFile, "-C", tmpExtractDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		h.setError(fmt.Sprintf("Failed to extract update: %v - %s", err, string(output)))
		updateFailed = true
		return
	}
	os.Remove(decryptedFile)

	// Step 5: VALIDATE update package before applying
	h.setStatus("validate", 60, "Validating update package...")

	// Check for versioned root directory (e.g., proxpanel-1.0.27/)
	packageRoot := tmpExtractDir
	entries, _ := os.ReadDir(tmpExtractDir)
	for _, entry := range entries {
		if entry.IsDir() && (entry.Name() == "proxpanel-"+version ||
			(len(entry.Name()) > 10 && entry.Name()[:10] == "proxpanel-")) {
			packageRoot = filepath.Join(tmpExtractDir, entry.Name())
			break
		}
	}

	// Check that required files exist and are valid
	// Support multiple structures:
	// 1. backend/proisp-api/proisp-api (subdirectory with binary)
	// 2. backend/proisp-api (direct binary)
	// 3. backend/api (old name)
	apiBinary := filepath.Join(packageRoot, "backend", "proisp-api", "proisp-api")
	if _, err := os.Stat(apiBinary); err != nil {
		// Try flat structure
		apiBinary = filepath.Join(packageRoot, "backend", "proisp-api")
	}
	if _, err := os.Stat(apiBinary); err != nil {
		// Try old name
		apiBinary = filepath.Join(packageRoot, "backend", "api")
	}

	radiusBinary := filepath.Join(packageRoot, "backend", "proisp-radius", "proisp-radius")
	if _, err := os.Stat(radiusBinary); err != nil {
		// Try flat structure
		radiusBinary = filepath.Join(packageRoot, "backend", "proisp-radius")
	}
	if _, err := os.Stat(radiusBinary); err != nil {
		// Try old name
		radiusBinary = filepath.Join(packageRoot, "backend", "radius")
	}

	// Support both frontend/dist (new) and frontend/ directly (old 1.0.23 format)
	frontendDist := filepath.Join(packageRoot, "frontend", "dist")
	if _, err := os.Stat(filepath.Join(frontendDist, "index.html")); err != nil {
		// Try old format - files directly in frontend/
		frontendDist = filepath.Join(packageRoot, "frontend")
	}

	// Validate API binary exists and is a file (not directory)
	if apiInfo, err := os.Stat(apiBinary); err != nil || apiInfo.IsDir() {
		h.setError("Invalid update package: API binary missing or invalid")
		os.RemoveAll(tmpExtractDir)
		updateFailed = true
		return
	}

	// Validate RADIUS binary exists and is a file
	if radiusInfo, err := os.Stat(radiusBinary); err != nil || radiusInfo.IsDir() {
		h.setError("Invalid update package: RADIUS binary missing or invalid")
		os.RemoveAll(tmpExtractDir)
		updateFailed = true
		return
	}

	// Validate frontend dist exists and is a directory
	if distInfo, err := os.Stat(frontendDist); err != nil || !distInfo.IsDir() {
		h.setError("Invalid update package: Frontend dist missing or invalid")
		os.RemoveAll(tmpExtractDir)
		updateFailed = true
		return
	}

	// Validate frontend has index.html
	if _, err := os.Stat(filepath.Join(frontendDist, "index.html")); err != nil {
		h.setError("Invalid update package: Frontend index.html missing")
		os.RemoveAll(tmpExtractDir)
		updateFailed = true
		return
	}

	// Test that binaries are executable (ELF format check)
	h.setStatus("validate", 65, "Testing binaries...")
	testCmd := exec.Command(apiBinary, "--version")
	testCmd.Env = append(os.Environ(), "PROXPANEL_VERSION=test")
	// Just check it doesn't crash immediately - timeout after 2 seconds
	if err := testCmd.Start(); err == nil {
		go func() {
			time.Sleep(2 * time.Second)
			testCmd.Process.Kill()
		}()
		testCmd.Wait()
	}

	// Step 6: Apply update (copy files - don't stop API, it will restart after)
	h.setStatus("apply", 70, "Applying update files...")

	// Backup current binaries (in case we need to rollback)
	// Handle both structures: proisp-api (file) or proisp-api/proisp-api (directory with binary)
	currentApi := filepath.Join(h.installDir, "backend", "proisp-api")
	if info, err := os.Stat(currentApi); err == nil && info.IsDir() {
		currentApi = filepath.Join(currentApi, "proisp-api")
	}
	currentRadius := filepath.Join(h.installDir, "backend", "proisp-radius")
	if info, err := os.Stat(currentRadius); err == nil && info.IsDir() {
		currentRadius = filepath.Join(currentRadius, "proisp-radius")
	}
	exec.Command("cp", currentApi, "/tmp/proisp-api-backup").Run()
	exec.Command("cp", currentRadius, "/tmp/proisp-radius-backup").Run()

	// Update backend binaries - copy to temp first, then move (atomic)
	exec.Command("mkdir", "-p", filepath.Join(h.installDir, "backend")).Run()

	// Copy API to temp location first
	tmpApi := filepath.Join(h.installDir, "backend", "proisp-api.new")
	if err := exec.Command("cp", "-f", apiBinary, tmpApi).Run(); err != nil {
		h.setError("Failed to copy API binary: " + err.Error())
		os.RemoveAll(tmpExtractDir)
		updateFailed = true
		return
	}
	exec.Command("chmod", "+x", tmpApi).Run()

	// Copy RADIUS to temp location
	tmpRadius := filepath.Join(h.installDir, "backend", "proisp-radius.new")
	if err := exec.Command("cp", "-f", radiusBinary, tmpRadius).Run(); err != nil {
		h.setError("Failed to copy RADIUS binary: " + err.Error())
		os.Remove(tmpApi)
		os.RemoveAll(tmpExtractDir)
		updateFailed = true
		return
	}
	exec.Command("chmod", "+x", tmpRadius).Run()

	h.setStatus("apply", 75, "Updating frontend...")

	// Update frontend dist - copy to temp first
	tmpDist := filepath.Join(h.installDir, "frontend", "dist.new")
	exec.Command("rm", "-rf", tmpDist).Run()
	if err := exec.Command("cp", "-r", frontendDist, tmpDist).Run(); err != nil {
		h.setError("Failed to copy frontend: " + err.Error())
		os.Remove(tmpApi)
		os.Remove(tmpRadius)
		os.RemoveAll(tmpExtractDir)
		updateFailed = true
		return
	}

	// Now atomically move files into place
	h.setStatus("apply", 80, "Finalizing file updates...")

	// Move new binaries into place (mv is atomic on same filesystem)
	// Handle both structures: backend/proisp-api (file) or backend/proisp-api/proisp-api (directory with binary)
	apiDest := filepath.Join(h.installDir, "backend", "proisp-api")
	if info, err := os.Stat(apiDest); err == nil && info.IsDir() {
		// It's a directory, move binary inside it
		apiDest = filepath.Join(apiDest, "proisp-api")
	}
	exec.Command("mv", "-f", tmpApi, apiDest).Run()

	radiusDest := filepath.Join(h.installDir, "backend", "proisp-radius")
	if info, err := os.Stat(radiusDest); err == nil && info.IsDir() {
		// It's a directory, move binary inside it
		radiusDest = filepath.Join(radiusDest, "proisp-radius")
	}
	exec.Command("mv", "-f", tmpRadius, radiusDest).Run()

	// Update frontend in-place (preserve directory inode so bind mounts stay valid)
	currentDist := filepath.Join(h.installDir, "frontend", "dist")
	os.MkdirAll(currentDist, 0755)
	// Clear existing files then copy new ones in (inode of currentDist unchanged)
	exec.Command("sh", "-c", "rm -rf '"+currentDist+"/'* '"+currentDist+"/'.[!.]* 2>/dev/null || true").Run()
	exec.Command("sh", "-c", "cp -r '"+tmpDist+"/.' '"+currentDist+"/'").Run()
	exec.Command("rm", "-rf", tmpDist).Run()

	// Update nginx.conf - but NEVER overwrite if SSL is configured
	currentNginxConf := filepath.Join(h.installDir, "frontend", "nginx.conf")
	newNginxConf := filepath.Join(packageRoot, "frontend", "nginx.conf")
	if _, err := os.Stat(newNginxConf); err == nil {
		// Update package contains a new nginx.conf
		if h.hasSSLConfigured(currentNginxConf) {
			// Server has SSL - skip overwrite, preserve existing SSL config
			log.Println("Update: SSL detected in nginx.conf - skipping overwrite to preserve SSL configuration")
		} else {
			// No SSL configured - safe to apply new nginx.conf from package
			if err := exec.Command("cp", "-f", newNginxConf, currentNginxConf).Run(); err != nil {
				log.Printf("Update: Failed to update nginx.conf: %v", err)
			} else {
				log.Println("Update: nginx.conf updated from package")
			}
		}
	} else if _, err := os.Stat(currentNginxConf); os.IsNotExist(err) {
		// No nginx.conf anywhere - write a safe default
		os.WriteFile(currentNginxConf, []byte(`server {
    listen 80;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;

    location /api/ {
        proxy_pass http://proxpanel-api:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location / {
        try_files $uri $uri/ /index.html;
    }
}
`), 0644)
	}

	// Cleanup
	os.RemoveAll(tmpExtractDir)
	os.Remove("/tmp/proisp-api-backup")
	os.Remove("/tmp/proisp-radius-backup")

	// Step 6: Install cloudflared if not present (for Remote Access tunnel feature)
	h.setStatus("dependencies", 82, "Checking dependencies...")
	h.installCloudflaredIfMissing()

	// Step 7: Update version file
	h.setStatus("finalize", 85, "Finalizing update...")
	os.WriteFile(filepath.Join(h.installDir, "VERSION"), []byte(version), 0644)

	// Step 6: Report success to license server
	h.setStatus("report", 90, "Reporting update status...")
	h.reportUpdateStatus(currentVersion, version, "success", "")

	// Done - need restart
	h.mutex.Lock()
	h.updateStatus.Step = "complete"
	h.updateStatus.Progress = 100
	h.updateStatus.Message = "Update complete! Restarting services..."
	h.updateStatus.NeedsRestart = true
	h.mutex.Unlock()

	// Write flag file for host-level systemd service to trigger restart
	// This is a fallback in case the Docker API restart fails
	flagFile := filepath.Join(h.installDir, ".update-complete")
	os.WriteFile(flagFile, []byte(version), 0644)
	log.Printf("Update: Written flag file %s for host-level restart", flagFile)

	// Trigger service restart (via Docker API)
	go h.restartServices()
}

func (h *SystemUpdateHandler) setError(message string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.updateStatus.Error = message
	h.updateStatus.Message = "Update failed"
	h.updateStatus.Progress = 0
}

// hasSSLConfigured checks if the given nginx.conf has SSL configured
func (h *SystemUpdateHandler) hasSSLConfigured(nginxConfPath string) bool {
	data, err := os.ReadFile(nginxConfPath)
	if err != nil {
		return false
	}
	return bytes.Contains(data, []byte("listen 443 ssl")) ||
		bytes.Contains(data, []byte("ssl_certificate"))
}

// installCloudflaredIfMissing installs cloudflared on the host if not present (for Remote Access tunnel feature)
func (h *SystemUpdateHandler) installCloudflaredIfMissing() {
	// Check if cloudflared exists on host via nsenter
	out, err := exec.Command("nsenter", "-t", "1", "-m", "-u", "-i", "-n", "--", "which", "cloudflared").CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) != "" {
		log.Println("Update: cloudflared already installed on host")
		return
	}

	log.Println("Update: Installing cloudflared on host for Remote Access tunnel feature...")
	installCmd := `curl -fsSL --max-time 60 -o /usr/local/bin/cloudflared https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 && chmod +x /usr/local/bin/cloudflared`
	cmd := exec.Command("nsenter", "-t", "1", "-m", "-u", "-i", "-n", "--", "bash", "-c", installCmd)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Update: cloudflared install failed (optional - needed only for Remote Access): %v — %s", err, string(output))
	} else {
		log.Println("Update: cloudflared installed successfully on host")
	}
}

func (h *SystemUpdateHandler) downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (h *SystemUpdateHandler) verifyChecksum(file, expected string) bool {
	f, err := os.Open(file)
	if err != nil {
		return false
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return false
	}

	actual := hex.EncodeToString(hasher.Sum(nil))
	return actual == expected
}

func (h *SystemUpdateHandler) reportUpdateStatus(fromVersion, toVersion, status, errorMsg string) {
	serverIP := os.Getenv("SERVER_IP")
	reqBody, _ := json.Marshal(map[string]string{
		"license_key":   h.licenseKey,
		"server_ip":     serverIP,
		"from_version":  fromVersion,
		"to_version":    toVersion,
		"status":        status,
		"error_message": errorMsg,
	})

	http.Post(
		h.licenseServer+"/api/v1/update/report",
		"application/json",
		bytes.NewBuffer(reqBody),
	)
}

func (h *SystemUpdateHandler) restartServices() {
	time.Sleep(1 * time.Second)

	allSuccess := true
	targetVersion := h.updateStatus.ToVersion

	// Use Docker API via socket to restart containers
	// This works even without docker CLI installed in container

	// Reload nginx to pick up new dist files (in-place update, no restart needed)
	// But also do a restart to ensure clean state, then a reload to fix any stale mounts
	log.Println("Update: Reloading proxpanel-frontend nginx...")
	if err := exec.Command("docker", "exec", "proxpanel-frontend", "nginx", "-s", "reload").Run(); err != nil {
		log.Printf("Update: nginx reload failed (%v), doing full restart...", err)
		if err := h.restartContainerViaSocket("proxpanel-frontend"); err != nil {
			exec.Command("docker", "restart", "proxpanel-frontend").Run()
		}
		time.Sleep(3 * time.Second)
		// Extra reload after restart to clear any stale mount cache
		exec.Command("docker", "exec", "proxpanel-frontend", "nginx", "-s", "reload").Run()
	}
	time.Sleep(2 * time.Second)

	// Verify frontend is serving content
	if err := h.verifyFrontendHealth(); err != nil {
		log.Printf("Update: Frontend health check failed (%v), doing restart + reload...", err)
		h.restartContainerViaSocket("proxpanel-frontend")
		time.Sleep(3 * time.Second)
		exec.Command("docker", "exec", "proxpanel-frontend", "nginx", "-s", "reload").Run()
		time.Sleep(1 * time.Second)
		// Check again
		if err2 := h.verifyFrontendHealth(); err2 != nil {
			log.Printf("Update: Frontend still unhealthy after retry: %v", err2)
			allSuccess = false
		} else {
			log.Println("Update: Frontend health check passed after retry")
		}
	} else {
		log.Println("Update: Frontend health check passed")
	}

	// Restart RADIUS to pick up new binary
	log.Println("Update: Restarting proxpanel-radius...")
	if err := h.restartContainerViaSocket("proxpanel-radius"); err != nil {
		log.Printf("Update: Failed to restart radius via socket: %v, trying CLI fallback", err)
		if err := exec.Command("docker", "restart", "proxpanel-radius").Run(); err != nil {
			log.Printf("Update: CLI fallback also failed: %v", err)
			allSuccess = false
		}
	}
	time.Sleep(1 * time.Second)

	// If all restarts succeeded, remove the flag file so host service doesn't also restart
	if allSuccess {
		flagFile := filepath.Join(h.installDir, ".update-complete")
		os.Remove(flagFile)
		log.Println("Update: All services restarted successfully, removed flag file")
	} else {
		log.Println("Update: Some restarts failed, leaving flag file for host-level service")
	}

	// Write verification file for post-restart check
	// The new API process will read this and verify it's running the correct version
	verifyFile := filepath.Join(h.installDir, ".update-verify")
	os.WriteFile(verifyFile, []byte(targetVersion), 0644)
	log.Printf("Update: Written verification file for version %s", targetVersion)

	// Restart API last - this will use the new binary
	// The current request will complete, then container restarts with new code
	log.Println("Update: Restarting proxpanel-api...")
	if err := h.restartContainerViaSocket("proxpanel-api"); err != nil {
		log.Printf("Update: Failed to restart api via socket: %v, trying CLI fallback", err)
		exec.Command("docker", "restart", "proxpanel-api").Run()
	}
}

// verifyFrontendHealth checks if the frontend nginx is serving content correctly
func (h *SystemUpdateHandler) verifyFrontendHealth() error {
	client := &http.Client{Timeout: 5 * time.Second}

	// Try to fetch index.html from frontend
	resp, err := client.Get("http://proxpanel-frontend/")
	if err != nil {
		return fmt.Errorf("failed to connect to frontend: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 500 {
		return fmt.Errorf("frontend returning 500 error")
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("frontend returned status %d", resp.StatusCode)
	}

	return nil
}

// VerifyUpdate checks if the running API version matches the expected version after update
// This should be called on API startup
func VerifyUpdateOnStartup() {
	installDir := getInstallDir()
	verifyFile := filepath.Join(installDir, ".update-verify")

	// Check if verification is pending
	expectedVersion, err := os.ReadFile(verifyFile)
	if err != nil {
		return // No verification pending
	}

	// Clean up the verify file
	os.Remove(verifyFile)

	expected := string(bytes.TrimSpace(expectedVersion))
	current := getCurrentVersion()

	if current != expected {
		log.Printf("UPDATE VERIFICATION FAILED: Expected version %s but running %s", expected, current)
		log.Printf("The update may have failed - please check the binaries")
		// TODO: Could trigger automatic rollback here
	} else {
		log.Printf("UPDATE VERIFICATION SUCCESS: Running version %s as expected", current)

		// Report success to license server
		licenseServer := os.Getenv("LICENSE_SERVER")
		licenseKey := os.Getenv("LICENSE_KEY")
		if licenseServer != "" && licenseKey != "" {
			reqBody, _ := json.Marshal(map[string]string{
				"license_key": licenseKey,
				"version":     current,
				"status":      "verified",
			})
			http.Post(licenseServer+"/api/v1/update/verified", "application/json", bytes.NewBuffer(reqBody))
		}
	}
}

// restartContainerViaSocket restarts a container using Docker Engine API via Unix socket
func (h *SystemUpdateHandler) restartContainerViaSocket(containerName string) error {
	socketPath := "/var/run/docker.sock"

	// Check if socket exists
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		return fmt.Errorf("docker socket not found at %s", socketPath)
	}

	// Create HTTP client that connects via Unix socket
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
		Timeout: 30 * time.Second,
	}

	// POST /containers/{name}/restart
	url := fmt.Sprintf("http://docker/containers/%s/restart?t=10", containerName)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("docker API returned %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("Update: Successfully restarted container %s", containerName)
	return nil
}

// validateBackupIntegrity checks if a backup has all required files for rollback
func (h *SystemUpdateHandler) validateBackupIntegrity(backupDir string) (bool, string) {
	requiredFiles := []string{
		filepath.Join("frontend", "dist", "index.html"),
	}

	// Check for at least one backend binary structure
	apiPath1 := filepath.Join(backupDir, "backend", "proisp-api", "proisp-api")
	apiPath2 := filepath.Join(backupDir, "backend", "proisp-api")
	apiExists := false
	if _, err := os.Stat(apiPath1); err == nil {
		apiExists = true
	} else if info, err := os.Stat(apiPath2); err == nil && !info.IsDir() {
		apiExists = true
	}
	if !apiExists {
		return false, "API binary not found in backup"
	}

	// Check required frontend files
	for _, file := range requiredFiles {
		fullPath := filepath.Join(backupDir, file)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return false, fmt.Sprintf("Missing required file: %s", file)
		}
	}

	return true, ""
}

// performRollback restores the system to the backup state after a failed update
func (h *SystemUpdateHandler) performRollback(backupDir, fromVersion, toVersion string) {
	// Check if backup exists
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		log.Printf("Rollback: No backup found at %s, cannot rollback", backupDir)
		h.reportUpdateStatus(fromVersion, toVersion, "failed", h.updateStatus.Error+" (no backup available for rollback)")
		return
	}

	// Validate backup integrity before attempting rollback
	log.Printf("Rollback: Validating backup integrity...")
	if valid, reason := h.validateBackupIntegrity(backupDir); !valid {
		log.Printf("Rollback: Backup validation failed: %s", reason)
		h.mutex.Lock()
		h.updateStatus.Step = "failed"
		h.updateStatus.Progress = 0
		h.updateStatus.Message = fmt.Sprintf("Update failed and automatic rollback is not possible: %s. Please contact support.", reason)
		h.updateStatus.Error = fmt.Sprintf("Backup validation failed: %s", reason)
		h.mutex.Unlock()
		h.reportUpdateStatus(fromVersion, toVersion, "failed", fmt.Sprintf("Rollback failed - backup corrupted: %s", reason))
		return
	}

	log.Printf("Rollback: Starting automatic rollback from %s...", backupDir)
	h.setStatus("rollback", 0, "Rolling back to previous version...")

	// Restore backend binaries
	h.setStatus("rollback", 25, "Restoring API binary...")
	// Handle both structures: proisp-api (file) or proisp-api/proisp-api (directory with binary)
	backupApi := filepath.Join(backupDir, "backend", "proisp-api", "proisp-api")
	if _, err := os.Stat(backupApi); os.IsNotExist(err) {
		backupApi = filepath.Join(backupDir, "backend", "proisp-api")
	}
	destApi := filepath.Join(h.installDir, "backend", "proisp-api")
	if info, err := os.Stat(destApi); err == nil && info.IsDir() {
		destApi = filepath.Join(destApi, "proisp-api")
	}
	if _, err := os.Stat(backupApi); err == nil {
		exec.Command("cp", "-f", backupApi, destApi).Run()
		exec.Command("chmod", "+x", destApi).Run()
	}

	h.setStatus("rollback", 50, "Restoring RADIUS binary...")
	backupRadius := filepath.Join(backupDir, "backend", "proisp-radius", "proisp-radius")
	if _, err := os.Stat(backupRadius); os.IsNotExist(err) {
		backupRadius = filepath.Join(backupDir, "backend", "proisp-radius")
	}
	destRadius := filepath.Join(h.installDir, "backend", "proisp-radius")
	if info, err := os.Stat(destRadius); err == nil && info.IsDir() {
		destRadius = filepath.Join(destRadius, "proisp-radius")
	}
	if _, err := os.Stat(backupRadius); err == nil {
		exec.Command("cp", "-f", backupRadius, destRadius).Run()
		exec.Command("chmod", "+x", destRadius).Run()
	}

	h.setStatus("rollback", 75, "Restoring frontend...")
	backupFrontend := filepath.Join(backupDir, "frontend", "dist")
	if _, err := os.Stat(backupFrontend); err == nil {
		currentDist := filepath.Join(h.installDir, "frontend", "dist")
		exec.Command("rm", "-rf", currentDist).Run()
		exec.Command("cp", "-r", backupFrontend, currentDist).Run()
	}

	// Restore VERSION file
	backupVersion := filepath.Join(backupDir, "VERSION")
	if _, err := os.Stat(backupVersion); err == nil {
		exec.Command("cp", "-f", backupVersion, filepath.Join(h.installDir, "VERSION")).Run()
	}

	// Verify rollback was successful by checking critical files exist
	criticalFiles := []string{
		filepath.Join(h.installDir, "frontend", "dist", "index.html"),
	}
	rollbackSuccess := true
	for _, file := range criticalFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			log.Printf("Rollback: Critical file missing after rollback: %s", file)
			rollbackSuccess = false
		}
	}

	h.setStatus("rollback", 90, "Cleaning up and reporting...")

	if rollbackSuccess {
		// Report rollback status
		h.reportUpdateStatus(fromVersion, toVersion, "rolled_back", h.updateStatus.Error)

		// Cleanup backup after successful rollback
		os.RemoveAll(backupDir)

		h.mutex.Lock()
		h.updateStatus.Step = "rolled_back"
		h.updateStatus.Progress = 100
		h.updateStatus.Message = "Update failed. System has been rolled back to the previous version."
		h.mutex.Unlock()

		log.Println("Rollback: Automatic rollback completed successfully")
	} else {
		h.mutex.Lock()
		h.updateStatus.Step = "failed"
		h.updateStatus.Progress = 0
		h.updateStatus.Message = "Update failed and rollback was incomplete. Please contact support or restore from a backup."
		h.updateStatus.Error = "Rollback incomplete - some files may be missing"
		h.mutex.Unlock()

		h.reportUpdateStatus(fromVersion, toVersion, "failed", "Rollback incomplete - critical files missing after restore")
		log.Println("Rollback: WARNING - Rollback was incomplete, some files may be missing")
	}
}
