package security

import (
	"log"
	"os"
	"time"
)

// buildMode is set at compile time
var buildMode string

// Initialize starts all security protections
func Initialize() {
	// Skip all security in dev mode
	if buildMode == "dev" || isDevMode() {
		log.Println("Security: Running in development mode - protections disabled")
		return
	}

	log.Println("Security: Initializing protection systems...")

	// Start anti-tamper checks
	InitAntiTamper()

	// Start anti-debugging
	InitAntiDebug()

	// Start integrity monitoring
	go integrityMonitor()

	log.Println("Security: All protection systems active")
}

// integrityMonitor continuously monitors for security violations
func integrityMonitor() {
	ticker := time.NewTicker(1 * time.Minute)
	violationCount := 0

	for range ticker.C {
		// Check for various security violations
		violations := 0

		// Check if being debugged
		if detectDebugger() {
			violations++
		}

		// Check if tampered
		if IsTampered() {
			violations++
		}

		// Check for suspicious environment variables
		if checkSuspiciousEnv() {
			violations++
		}

		if violations > 0 {
			violationCount++

			// After multiple violations, take action
			if violationCount > 3 {
				// Graceful degradation - corrupt responses
				setTamperDetected()
			}

			if violationCount > 5 {
				// Exit with misleading error
				time.Sleep(time.Duration(time.Now().UnixNano()%30) * time.Second)
				os.Exit(137) // Looks like OOM
			}
		} else {
			// Reset counter if clean
			if violationCount > 0 {
				violationCount--
			}
		}
	}
}

func checkSuspiciousEnv() bool {
	suspicious := []string{
		"LD_PRELOAD",      // Library injection
		"DYLD_INSERT",     // macOS injection
		"DEBUGGER",        // Debugger indicator
		"_JAVA_OPTIONS",   // Java debugging
		"GODEBUG",         // Go debugging
	}

	for _, env := range suspicious {
		if os.Getenv(env) != "" {
			return true
		}
	}
	return false
}

// GetHardwareID returns the hardware fingerprint for license binding
func GetHardwareID() string {
	return HardwareFingerprint()
}

// ValidateLicenseHardware validates the hardware binding
func ValidateLicenseHardware(registeredHardwareID string) bool {
	return ValidateHardwareBinding(registeredHardwareID)
}
