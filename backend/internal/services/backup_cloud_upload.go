package services

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// uploadToCloud uploads an encrypted backup file to the ProxPanel license server cloud storage.
// This is called from runBackup() when storage_type is "cloud" or "local+cloud".
func (s *BackupSchedulerService) uploadToCloud(localFilePath, filename string) error {
	licenseServer := os.Getenv("LICENSE_SERVER")
	if licenseServer == "" {
		licenseServer = "https://license.proxrad.com"
	}
	licenseKey := os.Getenv("LICENSE_KEY")
	if licenseKey == "" {
		return fmt.Errorf("LICENSE_KEY not set, cannot upload to cloud")
	}

	// Open file
	file, err := os.Open(localFilePath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %v", err)
	}
	defer file.Close()

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat backup file: %v", err)
	}

	log.Printf("BackupCloud: Uploading %s (%.1f MB) to cloud storage...", filename, float64(stat.Size())/1024/1024)

	// Create HTTP request
	url := fmt.Sprintf("%s/api/v1/cloud-backup/upload", licenseServer)
	req, err := http.NewRequest("POST", url, file)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("X-License-Key", licenseKey)
	req.Header.Set("X-Filename", filename)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = stat.Size()

	// Use client with timeout for large files
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cloud upload request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 402 {
		return fmt.Errorf("cloud storage quota exceeded: %s", string(body))
	}
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return fmt.Errorf("cloud upload failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	log.Printf("BackupCloud: Successfully uploaded %s to cloud storage", filename)
	return nil
}
