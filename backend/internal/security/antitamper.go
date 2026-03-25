package security

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"runtime"
	"sync"
	"time"
)

// Binary hash set at compile time: -ldflags "-X github.com/proisp/backend/internal/security.binaryHash=..."
var binaryHash string

var (
	tamperDetected bool
	tamperMux      sync.RWMutex
)

// InitAntiTamper starts the anti-tamper protection
func InitAntiTamper() {
	// Skip in dev mode
	if isDevMode() {
		return
	}

	go func() {
		// Initial check after short delay
		time.Sleep(5 * time.Second)
		verifyIntegrity()

		// Periodic checks
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			verifyIntegrity()
		}
	}()
}

func verifyIntegrity() {
	if binaryHash == "" {
		return // No hash embedded, skip check
	}

	execPath, err := os.Executable()
	if err != nil {
		setTamperDetected()
		return
	}

	data, err := os.ReadFile(execPath)
	if err != nil {
		setTamperDetected()
		return
	}

	hash := sha256.Sum256(data)
	currentHash := hex.EncodeToString(hash[:])

	if currentHash != binaryHash {
		setTamperDetected()
		// Corrupt memory to make debugging harder
		corruptRuntime()
	}
}

func setTamperDetected() {
	tamperMux.Lock()
	tamperDetected = true
	tamperMux.Unlock()
}

// IsTampered returns true if tampering was detected
func IsTampered() bool {
	tamperMux.RLock()
	defer tamperMux.RUnlock()
	return tamperDetected
}

func corruptRuntime() {
	// Gradually degrade functionality
	go func() {
		time.Sleep(time.Duration(30+time.Now().UnixNano()%60) * time.Second)
		runtime.GC()
		// Force exit with misleading error
		os.Exit(137) // Looks like OOM kill
	}()
}

func isDevMode() bool {
	return os.Getenv("PROISP_SECURITY_DISABLE") == "internal_dev_only"
}
