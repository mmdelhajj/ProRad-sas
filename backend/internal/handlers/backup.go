package handlers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/config"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/middleware"
	"github.com/proisp/backend/internal/models"
	"github.com/proisp/backend/internal/services"
)

// Download tokens for secure file downloads
var (
	downloadTokens     = make(map[string]downloadTokenInfo)
	downloadTokenMutex sync.RWMutex
)

type downloadTokenInfo struct {
	Filename  string
	ExpiresAt time.Time
	UserID    uint
}

type BackupHandler struct {
	backupDir string
	cfg       *config.Config
}

func NewBackupHandler(cfg *config.Config) *BackupHandler {
	backupDir := "/var/backups/proisp"
	os.MkdirAll(backupDir, 0755)
	return &BackupHandler{
		backupDir: backupDir,
		cfg:       cfg,
	}
}

// BackupInfo represents a backup file info
type BackupInfo struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	Type      string    `json:"type"`
	Encrypted bool      `json:"encrypted"`
}

// List returns all backups
func (h *BackupHandler) List(c *fiber.Ctx) error {
	files, err := os.ReadDir(h.backupDir)
	if err != nil {
		return c.JSON(fiber.Map{
			"success": true,
			"data":    []BackupInfo{},
		})
	}

	backups := []BackupInfo{}
	for i, file := range files {
		if file.IsDir() {
			continue
		}

		// Show .proisp.bak files (encrypted backups), legacy .sql files, and mikrotik .tar.gz files
		name := file.Name()
		isBak := strings.HasSuffix(name, ".proisp.bak") || strings.HasSuffix(name, ".sql")
		isMikroTik := strings.HasPrefix(name, "mikrotik_") && strings.HasSuffix(name, ".tar.gz")
		if !isBak && !isMikroTik {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		backupType := "full"
		if isMikroTik {
			backupType = "mikrotik"
		} else if strings.Contains(name, "_config") {
			backupType = "config"
		} else if strings.Contains(name, "_data") {
			backupType = "data"
		}

		// Mark encrypted backups (mikrotik tar.gz files are not encrypted)
		encrypted := strings.HasSuffix(name, ".proisp.bak")

		backups = append(backups, BackupInfo{
			ID:        strconv.Itoa(i + 1),
			Filename:  file.Name(),
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
			Type:      backupType,
			Encrypted: encrypted,
		})
	}

	// Sort by date descending
	for i := 0; i < len(backups)-1; i++ {
		for j := i + 1; j < len(backups); j++ {
			if backups[i].CreatedAt.Before(backups[j].CreatedAt) {
				backups[i], backups[j] = backups[j], backups[i]
			}
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    backups,
	})
}

// CreateBackupRequest represents create backup request
type CreateBackupRequest struct {
	Type        string `json:"type"` // full, data, config
	Description string `json:"description"`
}

// CreateMikroTikBackupRequest represents MikroTik backup request
type CreateMikroTikBackupRequest struct {
	NasIDs []uint `json:"nas_ids"`
}

// Create creates a new encrypted backup
func (h *BackupHandler) Create(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	var req CreateBackupRequest
	if err := c.BodyParser(&req); err != nil {
		req.Type = "full"
	}

	if req.Type == "" {
		req.Type = "full"
	}

	timestamp := time.Now().Format("20060102_150405")
	// Create temp file for pg_dump output (custom format)
	tempFile := filepath.Join(h.backupDir, fmt.Sprintf(".temp_%s_%s.dump", req.Type, timestamp))
	// Final encrypted backup file
	filename := fmt.Sprintf("proisp_%s_%s.proisp.bak", req.Type, timestamp)
	finalPath := filepath.Join(h.backupDir, filename)

	// Build pg_dump command with custom format (-Fc for compressed binary)
	cmd := exec.Command("pg_dump",
		"-h", h.cfg.DBHost,
		"-p", strconv.Itoa(h.cfg.DBPort),
		"-U", h.cfg.DBUser,
		"-d", h.cfg.DBName,
		"-Fc", // Custom format (compressed, binary)
		"-f", tempFile,
		"--no-owner",
		"--no-acl",
	)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", h.cfg.DBPassword))

	// Add table filters based on type
	if req.Type == "data" {
		// Only data tables (exclude settings, permissions)
		cmd.Args = append(cmd.Args,
			"--table=subscribers",
			"--table=services",
			"--table=nas",
			"--table=resellers",
			"--table=transactions",
			"--table=invoices",
			"--table=prepaid_cards",
			"--table=sessions",
			"--table=radcheck",
			"--table=radreply",
			"--table=radacct",
		)
	} else if req.Type == "config" {
		// Only config tables
		cmd.Args = append(cmd.Args,
			"--table=users",
			"--table=settings",
			"--table=permissions",
			"--table=permission_groups",
			"--table=communication_templates",
			"--table=communication_rules",
			"--table=bandwidth_rules",
		)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(tempFile) // Clean up temp file
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to create backup: %s", string(output)),
		})
	}

	// Encrypt the backup file
	if err := h.encryptBackup(tempFile, finalPath); err != nil {
		os.Remove(tempFile)
		os.Remove(finalPath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to encrypt backup: %v", err),
		})
	}

	// Remove temp file
	os.Remove(tempFile)

	// Get file info
	info, _ := os.Stat(finalPath)

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionCreate,
		EntityType:  "backup",
		EntityName:  filename,
		Description: fmt.Sprintf("Created encrypted %s backup", req.Type),
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Encrypted backup created successfully",
		"data": BackupInfo{
			ID:        filename,
			Filename:  filename,
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
			Type:      req.Type,
			Encrypted: true,
		},
	})
}

// CreateMikroTikBackup creates a MikroTik config backup for selected NAS devices
func (h *BackupHandler) CreateMikroTikBackup(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	var req CreateMikroTikBackupRequest
	if err := c.BodyParser(&req); err != nil {
		// Allow empty body (export all NAS)
		req.NasIDs = nil
	}

	timestamp := time.Now().Format("20060102_150405")

	svc := services.NewBackupSchedulerService(h.cfg)

	var mikrotikPath string
	var err error
	if len(req.NasIDs) > 0 {
		mikrotikPath, err = svc.ExportMikroTikConfigs(timestamp, req.NasIDs)
	} else {
		mikrotikPath, err = svc.ExportMikroTikConfigs(timestamp)
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("MikroTik export failed: %v", err),
		})
	}

	if mikrotikPath == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "No NAS devices with API credentials found",
		})
	}

	info, err := os.Stat(mikrotikPath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": fmt.Sprintf("Failed to stat backup file: %v", err),
		})
	}

	filename := filepath.Base(mikrotikPath)

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionCreate,
		EntityType:  "backup",
		EntityName:  filename,
		Description: fmt.Sprintf("Created MikroTik config backup (%d NAS devices)", len(req.NasIDs)),
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "MikroTik config backup created successfully",
		"data": BackupInfo{
			ID:        filename,
			Filename:  filename,
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
			Type:      "mikrotik",
			Encrypted: false,
		},
	})
}

// deriveEncryptionKey derives a 32-byte AES-256 key from the license key
func (h *BackupHandler) deriveEncryptionKey() []byte {
	// Use database password + fixed salt to derive encryption key
	// This must match backup_scheduler.go's deriveEncryptionKey() exactly
	salt := "ProxPanel-Backup-Encryption-2024"
	combined := h.cfg.DBPassword + salt
	hash := sha256.Sum256([]byte(combined))
	return hash[:]
}

// encryptBackup encrypts a backup file using AES-256-GCM
func (h *BackupHandler) encryptBackup(inputPath, outputPath string) error {
	// Read input file
	plaintext, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %v", err)
	}

	// Get encryption key
	key := h.deriveEncryptionKey()

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %v", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %v", err)
	}

	// Create nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to create nonce: %v", err)
	}

	// Encrypt and prepend nonce
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// Write encrypted file with magic header V2 (includes license key)
	licenseKey := os.Getenv("LICENSE_KEY")
	if licenseKey == "" {
		licenseKey = "UNKNOWN"
	}

	header := fmt.Sprintf("PROXPANEL_ENCRYPTED_BACKUP_V2\nLICENSE_KEY=%s\n", licenseKey)
	output := append([]byte(header), ciphertext...)

	if err := os.WriteFile(outputPath, output, 0600); err != nil {
		return fmt.Errorf("failed to write encrypted file: %v", err)
	}

	return nil
}

// decryptBackup decrypts an encrypted backup file
func (h *BackupHandler) decryptBackup(inputPath, outputPath string) error {
	// Read encrypted file
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read encrypted file: %v", err)
	}

	// Check and remove magic header
	header := []byte("PROXPANEL_ENCRYPTED_BACKUP_V1\n")
	if len(data) < len(header) || string(data[:len(header)]) != string(header) {
		return fmt.Errorf("invalid encrypted backup format - this file may be corrupted or not a ProxPanel backup")
	}
	ciphertext := data[len(header):]

	// Get decryption key
	key := h.deriveEncryptionKey()

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %v", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %v", err)
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("decryption failed - this backup may be from a different installation")
	}

	// Write decrypted file
	if err := os.WriteFile(outputPath, plaintext, 0600); err != nil {
		return fmt.Errorf("failed to write decrypted file: %v", err)
	}

	return nil
}

// Download downloads a backup file (authenticated)
func (h *BackupHandler) Download(c *fiber.Ctx) error {
	filename := c.Params("filename")
	if filename == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Filename is required",
		})
	}

	// Sanitize filename to prevent path traversal
	filename = filepath.Base(filename)
	filePath := filepath.Join(h.backupDir, filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Backup not found",
		})
	}

	return c.Download(filePath, filename)
}

// GetDownloadToken generates a temporary download token
func (h *BackupHandler) GetDownloadToken(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	filename := c.Params("filename")
	if filename == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Filename is required",
		})
	}

	// Sanitize filename
	filename = filepath.Base(filename)
	filePath := filepath.Join(h.backupDir, filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Backup not found",
		})
	}

	// Generate random token
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	// Store token with 5 minute expiry
	downloadTokenMutex.Lock()
	downloadTokens[token] = downloadTokenInfo{
		Filename:  filename,
		ExpiresAt: time.Now().Add(5 * time.Minute),
		UserID:    user.ID,
	}
	downloadTokenMutex.Unlock()

	// Clean up expired tokens
	go cleanupExpiredTokens()

	return c.JSON(fiber.Map{
		"success": true,
		"token":   token,
		"url":     fmt.Sprintf("/api/backups/public-download/%s", token),
	})
}

// PublicDownload downloads a backup using a temporary token (no auth required)
func (h *BackupHandler) PublicDownload(c *fiber.Ctx) error {
	token := c.Params("token")
	if token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Token is required",
		})
	}

	// Look up token
	downloadTokenMutex.RLock()
	info, exists := downloadTokens[token]
	downloadTokenMutex.RUnlock()

	if !exists {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Invalid or expired download token",
		})
	}

	// Check expiry
	if time.Now().After(info.ExpiresAt) {
		downloadTokenMutex.Lock()
		delete(downloadTokens, token)
		downloadTokenMutex.Unlock()
		return c.Status(fiber.StatusGone).JSON(fiber.Map{
			"success": false,
			"message": "Download token has expired",
		})
	}

	// Delete token after use (one-time use)
	downloadTokenMutex.Lock()
	delete(downloadTokens, token)
	downloadTokenMutex.Unlock()

	// Serve the file
	filePath := filepath.Join(h.backupDir, info.Filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Backup file not found",
		})
	}

	return c.Download(filePath, info.Filename)
}

// cleanupExpiredTokens removes expired download tokens
func cleanupExpiredTokens() {
	downloadTokenMutex.Lock()
	defer downloadTokenMutex.Unlock()

	now := time.Now()
	for token, info := range downloadTokens {
		if now.After(info.ExpiresAt) {
			delete(downloadTokens, token)
		}
	}
}

// ValidateBackup validates a backup file without restoring it
// Returns detailed validation info so user can decide whether to proceed
func (h *BackupHandler) ValidateBackup(c *fiber.Ctx) error {
	filename := c.Params("filename")
	if filename == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Filename is required",
		})
	}

	// Sanitize filename
	filename = filepath.Base(filename)
	filePath := filepath.Join(h.backupDir, filename)

	// Check file exists
	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Backup file not found",
			"valid":   false,
		})
	}

	validationResult := fiber.Map{
		"filename":   filename,
		"size":       fileInfo.Size(),
		"created_at": fileInfo.ModTime(),
		"encrypted":  strings.HasSuffix(filename, ".proisp.bak"),
	}

	var tempDecrypted string
	var fileToValidate string

	// Check if this is an encrypted backup (.proisp.bak)
	if strings.HasSuffix(filename, ".proisp.bak") {
		// Try to decrypt using local password first, then license server password
		tempDecrypted = filepath.Join(h.backupDir, fmt.Sprintf(".validate_temp_%d.dump", time.Now().UnixNano()))
		var validateDecryptErr error
		passwordsToTry := []string{h.cfg.DBPassword}
		if fetchedPwd, err := h.fetchDBPasswordFromLicenseServerWithKey(os.Getenv("LICENSE_KEY")); err == nil && fetchedPwd != "" && fetchedPwd != h.cfg.DBPassword {
			passwordsToTry = append(passwordsToTry, fetchedPwd)
		}
		for _, pwd := range passwordsToTry {
			if err := h.decryptBackupWithPassword(filePath, tempDecrypted, pwd); err == nil {
				validateDecryptErr = nil
				break
			} else {
				validateDecryptErr = err
			}
		}
		if validateDecryptErr != nil {
			return c.JSON(fiber.Map{
				"success": true,
				"valid":   false,
				"message": fmt.Sprintf("Backup decryption failed: %v. This backup cannot be restored on this server.", validateDecryptErr),
				"data":    validationResult,
			})
		}
		defer os.Remove(tempDecrypted)
		fileToValidate = tempDecrypted
		validationResult["decryption"] = "success"
	} else {
		fileToValidate = filePath
	}

	// Validate the backup format using pg_restore --list (dry run)
	cmd := exec.Command("pg_restore", "--list", fileToValidate)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if it might be a SQL file
		if !strings.HasSuffix(filename, ".proisp.bak") {
			// For SQL files, try to validate syntax
			cmd = exec.Command("psql",
				"-h", h.cfg.DBHost,
				"-p", strconv.Itoa(h.cfg.DBPort),
				"-U", h.cfg.DBUser,
				"-d", h.cfg.DBName,
				"-f", filePath,
				"--set", "ON_ERROR_STOP=1",
				"-c", "\\q", // Just parse, don't execute
			)
			cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", h.cfg.DBPassword))
			_, sqlErr := cmd.CombinedOutput()
			if sqlErr != nil {
				return c.JSON(fiber.Map{
					"success": true,
					"valid":   false,
					"message": "Backup format validation failed. The file may be corrupted or not a valid backup.",
					"data":    validationResult,
				})
			}
			validationResult["format"] = "sql"
		} else {
			return c.JSON(fiber.Map{
				"success": true,
				"valid":   false,
				"message": "Backup format validation failed. The backup file appears to be corrupted.",
				"data":    validationResult,
			})
		}
	} else {
		validationResult["format"] = "pg_dump_custom"
		// Count objects in backup
		lines := strings.Split(string(output), "\n")
		tableCount := 0
		dataCount := 0
		for _, line := range lines {
			if strings.Contains(line, " TABLE ") {
				tableCount++
			}
			if strings.Contains(line, " TABLE DATA ") {
				dataCount++
			}
		}
		validationResult["table_count"] = tableCount
		validationResult["data_sections"] = dataCount
	}

	return c.JSON(fiber.Map{
		"success": true,
		"valid":   true,
		"message": "Backup validation successful. This backup can be safely restored.",
		"data":    validationResult,
	})
}

// Restore restores from a backup
func (h *BackupHandler) Restore(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	filename := c.Params("filename")
	if filename == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Filename is required",
		})
	}

	// Parse request body for optional source license key
	var reqBody struct {
		SourceLicenseKey string `json:"source_license_key"`
	}
	c.BodyParser(&reqBody) // Ignore errors, it's optional

	// Sanitize filename
	filename = filepath.Base(filename)
	filePath := filepath.Join(h.backupDir, filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Backup not found",
		})
	}

	var cmd *exec.Cmd
	var tempDecrypted string

	// Check if this is an encrypted backup (.proisp.bak)
	if strings.HasSuffix(filename, ".proisp.bak") {
		// Try to extract license key from backup file header (V2 format)
		sourceLicenseKey := reqBody.SourceLicenseKey
		if sourceLicenseKey == "" {
			// Read license key from backup file
			if extractedKey, err := h.extractLicenseKeyFromBackup(filePath); err == nil && extractedKey != "" {
				sourceLicenseKey = extractedKey
				log.Printf("Restore: Found license key in backup file: %s", sourceLicenseKey)
			} else {
				// Fallback to current server's license key for V1 backups
				sourceLicenseKey = os.Getenv("LICENSE_KEY")
				log.Printf("Restore: No license key in backup (V1 format), using current server's license")
			}
		}

		// Build list of passwords to try, in order of priority
		var passwordsToTry []string

		// 1. Try fetched password from license server first (for cross-server restores)
		if fetchedPassword, err := h.fetchDBPasswordFromLicenseServerWithKey(sourceLicenseKey); err == nil && fetchedPassword != "" {
			log.Printf("Restore: Got DB password from license server (license: %s)", sourceLicenseKey)
			passwordsToTry = append(passwordsToTry, fetchedPassword)
		} else {
			log.Printf("Restore: License server fetch failed (%v), will try local password only", err)
		}

		// 2. Always also try local DB password as fallback (backup may have been encrypted
		//    before Option-2 secrets were configured, or license server password differs from .env)
		if h.cfg.DBPassword != "" && (len(passwordsToTry) == 0 || passwordsToTry[0] != h.cfg.DBPassword) {
			passwordsToTry = append(passwordsToTry, h.cfg.DBPassword)
		}

		// Decrypt the backup — try each password until one succeeds
		tempDecrypted = filepath.Join(h.backupDir, fmt.Sprintf(".restore_temp_%d.dump", time.Now().UnixNano()))
		var decryptErr error
		for i, pwd := range passwordsToTry {
			if err := h.decryptBackupWithPassword(filePath, tempDecrypted, pwd); err == nil {
				if i > 0 {
					log.Printf("Restore: Decryption succeeded using fallback password #%d", i+1)
				} else {
					log.Printf("Restore: Decryption succeeded")
				}
				decryptErr = nil
				break
			} else {
				log.Printf("Restore: Password #%d failed: %v", i+1, err)
				decryptErr = err
			}
		}
		if decryptErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": fmt.Sprintf("Failed to decrypt backup: %v. This backup cannot be restored on this server - it may be from a different installation.", decryptErr),
			})
		}
		defer os.Remove(tempDecrypted) // Clean up temp file after restore

		// Validate the backup format before restoring
		validateCmd := exec.Command("pg_restore", "--list", tempDecrypted)
		if _, err := validateCmd.CombinedOutput(); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Backup validation failed. The backup file appears to be corrupted and cannot be restored safely.",
			})
		}

		// Use pg_restore for custom format backups
		cmd = exec.Command("pg_restore",
			"-h", h.cfg.DBHost,
			"-p", strconv.Itoa(h.cfg.DBPort),
			"-U", h.cfg.DBUser,
			"-d", h.cfg.DBName,
			"--clean",              // Clean (drop) database objects before recreating
			"--if-exists",          // Don't error if objects don't exist
			"--no-owner",           // Don't set ownership
			"--no-acl",             // Don't restore access privileges
			"--single-transaction", // Restore as single transaction
			tempDecrypted,
		)
	} else {
		// Legacy SQL file - use psql
		cmd = exec.Command("psql",
			"-h", h.cfg.DBHost,
			"-p", strconv.Itoa(h.cfg.DBPort),
			"-U", h.cfg.DBUser,
			"-d", h.cfg.DBName,
			"-f", filePath,
		)
	}

	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", h.cfg.DBPassword))

	output, err := cmd.CombinedOutput()
	if err != nil {
		// For pg_restore, some errors are warnings - check if it's a real failure
		if strings.Contains(string(output), "FATAL") || strings.Contains(string(output), "could not connect") {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": fmt.Sprintf("Failed to restore backup: %s", string(output)),
			})
		}
		// pg_restore may return non-zero even with warnings, so log but continue
	}

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionUpdate,
		EntityType:  "backup",
		EntityName:  filename,
		Description: "Restored from backup",
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Backup restored successfully",
	})
}

// Delete deletes a backup file
func (h *BackupHandler) Delete(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	filename := c.Params("filename")
	if filename == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Filename is required",
		})
	}

	// Sanitize filename
	filename = filepath.Base(filename)
	filePath := filepath.Join(h.backupDir, filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Backup not found",
		})
	}

	if err := os.Remove(filePath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to delete backup",
		})
	}

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionDelete,
		EntityType:  "backup",
		EntityName:  filename,
		Description: "Deleted backup",
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Backup deleted successfully",
	})
}

// Upload uploads a backup file
func (h *BackupHandler) Upload(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "No file uploaded",
		})
	}

	// Validate file extension - accept both encrypted (.proisp.bak) and legacy (.sql)
	isEncrypted := strings.HasSuffix(file.Filename, ".proisp.bak")
	isLegacy := strings.HasSuffix(file.Filename, ".sql")
	if !isEncrypted && !isLegacy {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Only .proisp.bak or .sql files are allowed",
		})
	}

	// Sanitize filename
	filename := filepath.Base(file.Filename)
	destPath := filepath.Join(h.backupDir, filename)

	// Open source file
	src, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to open uploaded file",
		})
	}
	defer src.Close()

	// Create destination file
	dst, err := os.Create(destPath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to save file",
		})
	}
	defer dst.Close()

	// Copy content
	if _, err := io.Copy(dst, src); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to save file",
		})
	}

	// Validate encrypted backup format if it's a .proisp.bak file
	if isEncrypted {
		// Read first 35 bytes to check header (V1=30 chars, V2=30 chars)
		f, err := os.Open(destPath)
		if err == nil {
			header := make([]byte, 35)
			n, _ := f.Read(header)
			f.Close()
			headerStr := string(header[:n])
			v1Header := "PROXPANEL_ENCRYPTED_BACKUP_V1\n"
			v2Header := "PROXPANEL_ENCRYPTED_BACKUP_V2\n"
			if !strings.HasPrefix(headerStr, v1Header) && !strings.HasPrefix(headerStr, v2Header) {
				os.Remove(destPath)
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": "Invalid backup file format - this is not a valid ProxPanel encrypted backup",
				})
			}
		}
	}

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionCreate,
		EntityType:  "backup",
		EntityName:  filename,
		Description: fmt.Sprintf("Uploaded backup file (%s)", map[bool]string{true: "encrypted", false: "legacy SQL"}[isEncrypted]),
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Backup uploaded successfully",
		"data": fiber.Map{
			"filename":  filename,
			"encrypted": isEncrypted,
		},
	})
}

// ========== Backup Schedule Management ==========

// ListSchedules returns all backup schedules
func (h *BackupHandler) ListSchedules(c *fiber.Ctx) error {
	var schedules []models.BackupSchedule
	if err := database.DB.Order("created_at DESC").Find(&schedules).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch schedules",
		})
	}

	// Hide FTP passwords in response
	for i := range schedules {
		if schedules[i].FTPPassword != "" {
			schedules[i].FTPPassword = "********"
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    schedules,
	})
}

// GetSchedule returns a single backup schedule
func (h *BackupHandler) GetSchedule(c *fiber.Ctx) error {
	id := c.Params("id")

	var schedule models.BackupSchedule
	if err := database.DB.First(&schedule, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Schedule not found",
		})
	}

	// Hide FTP password
	if schedule.FTPPassword != "" {
		schedule.FTPPassword = "********"
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    schedule,
	})
}

// CreateSchedule creates a new backup schedule
func (h *BackupHandler) CreateSchedule(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	var schedule models.BackupSchedule
	if err := c.BodyParser(&schedule); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Validate required fields
	if schedule.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Schedule name is required",
		})
	}

	if schedule.Frequency == "" {
		schedule.Frequency = "daily"
	}

	if schedule.BackupType == "" {
		schedule.BackupType = "full"
	}

	if schedule.TimeOfDay == "" {
		schedule.TimeOfDay = "02:00"
	}

	if schedule.Retention == 0 {
		schedule.Retention = 7
	}

	if schedule.LocalPath == "" {
		schedule.LocalPath = h.backupDir
	}

	// Set ID to 0 for new record
	schedule.ID = 0

	// Calculate next run time using configured timezone
	nextRun := services.CalculateNextRunForSchedule(&schedule)
	schedule.NextRunAt = &nextRun

	if err := database.DB.Create(&schedule).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create schedule",
		})
	}

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionCreate,
		EntityType:  "backup_schedule",
		EntityID:    schedule.ID,
		EntityName:  schedule.Name,
		Description: fmt.Sprintf("Created backup schedule: %s (%s)", schedule.Name, schedule.Frequency),
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Schedule created successfully",
		"data":    schedule,
	})
}

// UpdateSchedule updates a backup schedule
func (h *BackupHandler) UpdateSchedule(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	id := c.Params("id")

	var existing models.BackupSchedule
	if err := database.DB.First(&existing, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Schedule not found",
		})
	}

	var updates models.BackupSchedule
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Don't update password if it's masked
	if updates.FTPPassword == "********" || updates.FTPPassword == "" {
		updates.FTPPassword = existing.FTPPassword
	}

	// Update fields
	existing.Name = updates.Name
	existing.IsEnabled = updates.IsEnabled
	existing.BackupType = updates.BackupType
	existing.Frequency = updates.Frequency
	existing.DayOfWeek = updates.DayOfWeek
	existing.DayOfMonth = updates.DayOfMonth
	existing.TimeOfDay = updates.TimeOfDay
	existing.Retention = updates.Retention
	existing.StorageType = updates.StorageType
	existing.LocalPath = updates.LocalPath
	existing.FTPEnabled = updates.FTPEnabled
	existing.FTPHost = updates.FTPHost
	existing.FTPPort = updates.FTPPort
	existing.FTPUsername = updates.FTPUsername
	existing.FTPPassword = updates.FTPPassword
	existing.FTPPath = updates.FTPPath
	existing.FTPPassive = updates.FTPPassive
	existing.FTPTLS = updates.FTPTLS
	existing.IncludeMikroTik = updates.IncludeMikroTik
	existing.CloudEnabled = updates.CloudEnabled

	// Recalculate next run time using configured timezone
	nextRun := services.CalculateNextRunForSchedule(&existing)
	existing.NextRunAt = &nextRun

	if err := database.DB.Save(&existing).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update schedule",
		})
	}

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionUpdate,
		EntityType:  "backup_schedule",
		EntityID:    existing.ID,
		EntityName:  existing.Name,
		Description: fmt.Sprintf("Updated backup schedule: %s", existing.Name),
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Schedule updated successfully",
		"data":    existing,
	})
}

// DeleteSchedule deletes a backup schedule
func (h *BackupHandler) DeleteSchedule(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	id := c.Params("id")

	var schedule models.BackupSchedule
	if err := database.DB.First(&schedule, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Schedule not found",
		})
	}

	if err := database.DB.Delete(&schedule).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to delete schedule",
		})
	}

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionDelete,
		EntityType:  "backup_schedule",
		EntityID:    schedule.ID,
		EntityName:  schedule.Name,
		Description: fmt.Sprintf("Deleted backup schedule: %s", schedule.Name),
		IPAddress:   c.IP(),
		UserAgent:   c.Get("User-Agent"),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Schedule deleted successfully",
	})
}

// ToggleSchedule enables/disables a backup schedule
func (h *BackupHandler) ToggleSchedule(c *fiber.Ctx) error {
	id := c.Params("id")

	var schedule models.BackupSchedule
	if err := database.DB.First(&schedule, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Schedule not found",
		})
	}

	schedule.IsEnabled = !schedule.IsEnabled
	database.DB.Save(&schedule)

	status := "disabled"
	if schedule.IsEnabled {
		status = "enabled"
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Schedule %s", status),
		"data":    schedule,
	})
}

// TestFTP tests FTP connection
func (h *BackupHandler) TestFTP(c *fiber.Ctx) error {
	var req struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
		Path     string `json:"path"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request",
		})
	}

	if req.Port == 0 {
		req.Port = 21
	}

	err := services.TestFTPConnection(req.Host, req.Port, req.Username, req.Password, req.Path)
	if err != nil {
		return c.JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "FTP connection successful",
	})
}

// extractLicenseKeyFromBackup reads the license key from a V2 backup file header
func (h *BackupHandler) extractLicenseKeyFromBackup(filePath string) (string, error) {
	// Read first 200 bytes to check header
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	headerBytes := make([]byte, 200)
	n, err := file.Read(headerBytes)
	if err != nil && err != io.EOF {
		return "", err
	}

	header := string(headerBytes[:n])

	// Check if V2 format
	if !strings.HasPrefix(header, "PROXPANEL_ENCRYPTED_BACKUP_V2\n") {
		return "", fmt.Errorf("not a V2 backup (no license key stored)")
	}

	// Extract license key
	lines := strings.Split(header, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "LICENSE_KEY=") {
			return strings.TrimPrefix(line, "LICENSE_KEY="), nil
		}
	}

	return "", fmt.Errorf("LICENSE_KEY not found in V2 header")
}

// fetchDBPasswordFromLicenseServerWithKey fetches the DB password from license server using a specific license key
// This is used for cross-server backup restores
func (h *BackupHandler) fetchDBPasswordFromLicenseServerWithKey(licenseKey string) (string, error) {
	if licenseKey == "" {
		return "", fmt.Errorf("license key is empty")
	}

	licenseServer := os.Getenv("LICENSE_SERVER")
	if licenseServer == "" {
		licenseServer = "https://license.proxrad.com"
	}

	url := fmt.Sprintf("%s/api/v1/license/backup-password", licenseServer)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	// CRITICAL: Add the X-License-Key header
	req.Header.Set("X-License-Key", licenseKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch DB password: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("license server returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Success    bool   `json:"success"`
		DBPassword string `json:"db_password"`
		Message    string `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if !result.Success {
		return "", fmt.Errorf("%s", result.Message)
	}

	return result.DBPassword, nil
}

// decryptBackupWithPassword decrypts a backup file using a specific password
func (h *BackupHandler) decryptBackupWithPassword(inputPath, outputPath, password string) error {
	// Read encrypted file
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read encrypted file: %v", err)
	}

	// Detect version and extract ciphertext
	var ciphertext []byte

	// Check for V2 format (read first 100 bytes safely for header detection)
	v2Header := "PROXPANEL_ENCRYPTED_BACKUP_V2\n"
	v1Header := "PROXPANEL_ENCRYPTED_BACKUP_V1\n"

	headerCheckLen := 100
	if len(data) < headerCheckLen {
		headerCheckLen = len(data)
	}
	headerCheck := string(data[:headerCheckLen])

	if strings.HasPrefix(headerCheck, v2Header) {
		// V2 format: PROXPANEL_ENCRYPTED_BACKUP_V2\nLICENSE_KEY=xxx\n[ciphertext]
		// Find the second newline (after LICENSE_KEY line)
		offset := len(v2Header)
		secondNewline := -1
		for i := offset; i < len(data) && i < 200; i++ {
			if data[i] == '\n' {
				secondNewline = i
				break
			}
		}

		if secondNewline == -1 {
			return fmt.Errorf("invalid V2 backup format - LICENSE_KEY line not found")
		}

		// Ciphertext starts after the second newline
		ciphertext = data[secondNewline+1:]

	} else if strings.HasPrefix(headerCheck, v1Header) {
		// V1 format: Simple header
		ciphertext = data[len(v1Header):]
	} else {
		return fmt.Errorf("invalid encrypted backup format - this file may be corrupted or not a ProxPanel backup")
	}

	// Derive decryption key from provided password
	salt := "ProxPanel-Backup-Encryption-2024"
	combined := password + salt
	hash := sha256.Sum256([]byte(combined))
	key := hash[:]

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %v", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %v", err)
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("decryption failed - this backup may be from a different installation")
	}

	// Write decrypted file
	if err := os.WriteFile(outputPath, plaintext, 0600); err != nil {
		return fmt.Errorf("failed to write decrypted file: %v", err)
	}

	return nil
}

// ListBackupLogs returns backup execution logs
func (h *BackupHandler) ListBackupLogs(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	if limit < 1 || limit > 200 {
		limit = 50
	}
	offset := (page - 1) * limit

	var logs []models.BackupLog
	var total int64

	query := database.DB.Model(&models.BackupLog{})

	// Filter by schedule if specified
	if scheduleID := c.Query("schedule_id"); scheduleID != "" {
		query = query.Where("schedule_id = ?", scheduleID)
	}

	// Filter by status if specified
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)
	query.Order("started_at DESC").Offset(offset).Limit(limit).Find(&logs)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    logs,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

// RunScheduleNow manually triggers a scheduled backup
func (h *BackupHandler) RunScheduleNow(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	id := c.Params("id")

	var schedule models.BackupSchedule
	if err := database.DB.First(&schedule, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Schedule not found",
		})
	}

	// Run backup in background with panic recovery
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("BackupScheduler: PANIC in RunScheduleNow goroutine: %v", r)
			}
		}()
		svc := services.NewBackupSchedulerService(h.cfg)
		if schedule.BackupType == "mikrotik" {
			// MikroTik-only backup
			log.Printf("BackupScheduler: RunScheduleNow triggering MikroTik-only backup for schedule '%s'", schedule.Name)
			svc.RunMikroTikBackupNow(&schedule, user.ID, user.Username)
		} else {
			// DB backup (with optional MikroTik include)
			log.Printf("BackupScheduler: RunScheduleNow triggering %s backup for schedule '%s' (includeMikroTik=%v, cloudEnabled=%v)", schedule.BackupType, schedule.Name, schedule.IncludeMikroTik, schedule.CloudEnabled)
			svc.RunManualBackup(schedule.BackupType, &schedule, user.ID, user.Username, schedule.IncludeMikroTik)
		}
	}()

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Backup started",
	})
}
