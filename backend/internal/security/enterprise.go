package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"math/rand"
	"os"
	"sync"
	"time"
)

// Enterprise security features based on industry best practices
// References: PACE Anti-Piracy, Microcosm, LicenseSpring

// Response signing key - derived from license key at runtime
var responseSigningKey []byte
var signingKeyOnce sync.Once

// Timing anomaly detection
var (
	lastCheckTime    time.Time
	checkIntervals   []time.Duration
	anomalyCount     int
	anomalyMux       sync.Mutex
)

// InitEnterpriseSecurity initializes enterprise security features
func InitEnterpriseSecurity(licenseKey string) {
	signingKeyOnce.Do(func() {
		// Derive signing key from license key
		h := sha256.Sum256([]byte("HMAC-" + licenseKey + "-SIGN"))
		responseSigningKey = h[:]
	})

	// Start random delayed checks
	go randomDelayedChecks()

	// Start timing anomaly detection
	go timingAnomalyDetector()
}

// VerifyResponseSignature verifies HMAC signature of license server response
func VerifyResponseSignature(data []byte, signature string) bool {
	if len(responseSigningKey) == 0 {
		return true // Not initialized yet
	}

	mac := hmac.New(sha256.New, responseSigningKey)
	mac.Write(data)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedSig), []byte(signature))
}

// RecordCheckTime records when a license check occurred for anomaly detection
func RecordCheckTime() {
	anomalyMux.Lock()
	defer anomalyMux.Unlock()

	if !lastCheckTime.IsZero() {
		interval := time.Since(lastCheckTime)
		checkIntervals = append(checkIntervals, interval)

		// Keep only last 10 intervals
		if len(checkIntervals) > 10 {
			checkIntervals = checkIntervals[1:]
		}

		// Detect anomaly: if interval is way too fast (< 5 seconds), someone might be fuzzing
		if interval < 5*time.Second && len(checkIntervals) > 3 {
			anomalyCount++
			if anomalyCount > 5 {
				// Too many rapid checks - possible tampering
				go func() {
					time.Sleep(time.Duration(rand.Intn(60)) * time.Second)
					os.Exit(1)
				}()
			}
		} else {
			anomalyCount = 0
		}
	}
	lastCheckTime = time.Now()
}

// randomDelayedChecks performs additional license checks at random intervals
// This makes it harder to find and disable all license checks
func randomDelayedChecks() {
	for {
		// Random delay between 2-10 minutes
		delay := time.Duration(120+rand.Intn(480)) * time.Second
		time.Sleep(delay)

		// Perform a stealth check
		stealthLicenseCheck()
	}
}

// stealthLicenseCheck is a hidden license check that runs at random times
var stealthCheckFunc func() bool

// RegisterStealthCheck registers the stealth check function
func RegisterStealthCheck(f func() bool) {
	stealthCheckFunc = f
}

func stealthLicenseCheck() {
	if stealthCheckFunc != nil {
		valid := stealthCheckFunc()
		if !valid {
			// Random delay before exit to make debugging harder
			time.Sleep(time.Duration(10+rand.Intn(50)) * time.Second)
			os.Exit(1)
		}
	}
}

// timingAnomalyDetector monitors for timing-based attacks
func timingAnomalyDetector() {
	ticker := time.NewTicker(30 * time.Second)
	var lastTick time.Time
	var anomalies int

	for range ticker.C {
		if !lastTick.IsZero() {
			elapsed := time.Since(lastTick)
			// If ticker is significantly delayed (>60s instead of 30s), system might be paused for debugging
			if elapsed > 60*time.Second {
				anomalies++
				if anomalies > 3 {
					// System was paused multiple times - possible debugging
					go func() {
						time.Sleep(time.Duration(rand.Intn(120)) * time.Second)
						os.Exit(1)
					}()
				}
			} else {
				anomalies = 0
			}
		}
		lastTick = time.Now()
	}
}

// ObfuscatedStrings - runtime decrypted strings (manual obfuscation)
// These strings are XOR encrypted and decrypted at runtime

var stringKey = []byte{0x4C, 0x69, 0x63, 0x65, 0x6E, 0x73, 0x65} // "License"

// DecryptString decrypts an XOR-encrypted string
func DecryptString(encrypted []byte) string {
	result := make([]byte, len(encrypted))
	for i := range encrypted {
		result[i] = encrypted[i] ^ stringKey[i%len(stringKey)]
	}
	return string(result)
}

// EncryptString encrypts a string with XOR (for storing in code)
func EncryptString(plain string) []byte {
	data := []byte(plain)
	result := make([]byte, len(data))
	for i := range data {
		result[i] = data[i] ^ stringKey[i%len(stringKey)]
	}
	return result
}

// Pre-encrypted sensitive strings (XOR with "License" key)
var (
	// "invalid" encrypted
	EncInvalid = []byte{0x25, 0x07, 0x17, 0x00, 0x02, 0x06, 0x2B}
	// "blocked" encrypted
	EncBlocked = []byte{0x2E, 0x05, 0x0C, 0x08, 0x0B, 0x04, 0x2B}
	// "terminated" encrypted
	EncTerminated = []byte{0x38, 0x0C, 0x15, 0x0A, 0x03, 0x07, 0x25, 0x38, 0x0C, 0x0B}
)

// WatermarkResponse adds invisible watermark to responses for tracking leaks
func WatermarkResponse(data []byte, customerID string) []byte {
	// Add invisible watermark as base64 comment that looks like formatting
	watermark := base64.StdEncoding.EncodeToString([]byte("WM:" + customerID))
	// Embed watermark in response (could be expanded for JSON/HTML)
	_ = watermark // Reserved for future implementation
	return data
}

// DecoyCheck1 - fake license check that does nothing (confuses reverse engineers)
func DecoyCheck1() bool {
	time.Sleep(time.Microsecond)
	return true
}

// DecoyCheck2 - another fake check
func DecoyCheck2() bool {
	_ = sha256.Sum256([]byte("check"))
	return true
}

// DecoyCheck3 - reads random file (looks like important check)
func DecoyCheck3() bool {
	os.ReadFile("/dev/null")
	return true
}

// MultiPointValidation validates license at multiple code points
var validationPoints = make(map[string]bool)
var validationMux sync.RWMutex

// SetValidationPoint marks a validation point as checked
func SetValidationPoint(point string, valid bool) {
	validationMux.Lock()
	defer validationMux.Unlock()
	validationPoints[point] = valid
}

// CheckAllValidationPoints ensures all validation points passed
func CheckAllValidationPoints() bool {
	validationMux.RLock()
	defer validationMux.RUnlock()

	// Must have at least 3 validation points
	if len(validationPoints) < 3 {
		return false
	}

	for _, valid := range validationPoints {
		if !valid {
			return false
		}
	}
	return true
}
