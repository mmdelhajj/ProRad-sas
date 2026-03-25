package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/proisp/backend/internal/config"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/license"
	"github.com/proisp/backend/internal/models"
	"github.com/proisp/backend/internal/radius"
)

// Build date - set at compile time: -ldflags "-X main.buildDate=2026-01-25"
var buildDate string

// Maximum days binary can run without update
const maxBinaryAgeDays = 30

func main() {
	log.Println("Starting ProISP RADIUS Server...")

	// Check binary expiry
	if err := checkBinaryExpiry(); err != nil {
		log.Fatalf("Binary expired: %v - Please update to latest version", err)
	}

	// Load configuration
	cfg := config.Load()

	// Initialize license client - RADIUS MUST have valid license (unless SKIP_LICENSE=true for SaaS/dev)
	skipLicense := os.Getenv("SKIP_LICENSE") == "true"
	if !skipLicense {
		licenseServer := os.Getenv("LICENSE_SERVER")
		licenseKey := os.Getenv("LICENSE_KEY")

		if licenseServer == "" || licenseKey == "" {
			log.Fatalf("LICENSE_SERVER and LICENSE_KEY environment variables are required")
		}

		if err := license.Initialize(licenseServer, licenseKey); err != nil {
			log.Fatalf("License validation failed: %v - RADIUS cannot start without valid license", err)
		}
		log.Println("License validated successfully")
	} else {
		log.Println("License validation skipped (SKIP_LICENSE=true)")
	}

	// Connect to database
	if err := database.Connect(cfg); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Run migrations (skip in SaaS mode — tenant schemas managed separately)
	if os.Getenv("SAAS_MODE") != "true" {
		if err := models.AutoMigrate(database.DB); err != nil {
			log.Fatalf("Failed to run migrations: %v", err)
		}
	} else {
		log.Println("SaaS mode: Skipping AutoMigrate (tenant schemas managed by API)")
	}

	// Create and start RADIUS server
	server := radius.NewServer(cfg.RadiusAuthPort, cfg.RadiusAcctPort)
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start RADIUS server: %v", err)
	}

	log.Printf("RADIUS server started (auth: %d, acct: %d)", cfg.RadiusAuthPort, cfg.RadiusAcctPort)

	// Create stop channel for graceful shutdown of background goroutines
	stopChan := make(chan struct{})

	// Periodically reload NAS secrets - with proper cleanup
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop() // Prevent goroutine leak
		for {
			select {
			case <-ticker.C:
				if err := server.LoadSecrets(); err != nil {
					log.Printf("Failed to reload secrets: %v", err)
				}
			case <-stopChan:
				log.Println("Secrets reload goroutine stopped")
				return
			}
		}
	}()

	// Periodic license check - every hour - with proper cleanup
	if !skipLicense {
		go func() {
			ticker := time.NewTicker(1 * time.Hour)
			defer ticker.Stop() // Prevent goroutine leak
			for {
				select {
				case <-ticker.C:
					if err := license.Revalidate(); err != nil {
						log.Printf("License revalidation failed: %v", err)
					}
					if !license.IsValid() {
						log.Fatalf("License invalid - RADIUS shutting down")
					}
					// Also check binary expiry
					if err := checkBinaryExpiry(); err != nil {
						log.Fatalf("Binary expired: %v - Please update to latest version", err)
					}
				case <-stopChan:
					log.Println("License check goroutine stopped")
					return
				}
			}
		}()
	}

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down RADIUS server...")

	// Signal all background goroutines to stop
	close(stopChan)

	// Give goroutines time to cleanup
	time.Sleep(100 * time.Millisecond)

	if !skipLicense {
		license.Stop()
	}
}

// checkBinaryExpiry checks if the binary has expired
func checkBinaryExpiry() error {
	if buildDate == "" {
		// No build date set - allow in dev mode
		return nil
	}

	built, err := time.Parse("2006-01-02", buildDate)
	if err != nil {
		return nil // Invalid date format - allow
	}

	daysSinceBuild := int(time.Since(built).Hours() / 24)
	if daysSinceBuild > maxBinaryAgeDays {
		return &BinaryExpiredError{
			BuildDate:   buildDate,
			DaysExpired: daysSinceBuild - maxBinaryAgeDays,
		}
	}

	return nil
}

// BinaryExpiredError indicates the binary has exceeded its maximum age
type BinaryExpiredError struct {
	BuildDate   string
	DaysExpired int
}

func (e *BinaryExpiredError) Error() string {
	return "binary built on " + e.BuildDate + " has expired by " + string(rune(e.DaysExpired+'0')) + " days"
}
