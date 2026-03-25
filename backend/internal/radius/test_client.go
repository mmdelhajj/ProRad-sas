package radius

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

const (
	CodeCoARequest = 43 // Change-of-Authorization Request
	CodeCoAAck     = 44 // CoA-ACK
	CodeCoANak     = 45 // CoA-NAK
)

// TestResult contains the result of a RADIUS secret test
type TestResult struct {
	Success     bool
	SecretValid bool
	SecretSet   bool
	ErrorMsg    string
}

// TestSecret verifies the RADIUS secret by sending a CoA packet to the NAS
// MikroTik listens on CoA port (default 3799) and will respond if secret is correct
func TestSecret(nasIP string, authPort int, secret string) TestResult {
	result := TestResult{}

	// Check if secret is set
	if secret == "" {
		result.ErrorMsg = "RADIUS secret not configured"
		return result
	}
	result.SecretSet = true

	// Try CoA port (3799 is standard, some use 1700)
	coaPorts := []int{3799, 1700}

	for _, port := range coaPorts {
		testResult := sendCoATest(nasIP, port, secret)
		if testResult.Success {
			result.Success = true
			result.SecretValid = true
			result.ErrorMsg = fmt.Sprintf("Secret valid (CoA port %d)", port)
			return result
		}
		// If we got a response but NAK, secret is still valid
		if testResult.ErrorMsg == "CoA-NAK received (secret valid, no matching session)" {
			result.Success = true
			result.SecretValid = true
			result.ErrorMsg = testResult.ErrorMsg
			return result
		}
	}

	// No response on any port
	result.ErrorMsg = "No CoA response - secret may be wrong or CoA not enabled"
	return result
}

func sendCoATest(nasIP string, coaPort int, secret string) TestResult {
	result := TestResult{}

	addr := fmt.Sprintf("%s:%d", nasIP, coaPort)
	conn, err := net.DialTimeout("udp", addr, 2*time.Second)
	if err != nil {
		result.ErrorMsg = fmt.Sprintf("Cannot connect to CoA port %d", coaPort)
		return result
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(2 * time.Second))

	// Build CoA-Request packet
	packet := buildCoAPacket(secret)

	// Send packet
	_, err = conn.Write(packet)
	if err != nil {
		result.ErrorMsg = fmt.Sprintf("Failed to send CoA: %v", err)
		return result
	}

	// Wait for response
	response := make([]byte, 4096)
	n, err := conn.Read(response)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			result.ErrorMsg = "Timeout - no response"
			return result
		}
		result.ErrorMsg = fmt.Sprintf("Read error: %v", err)
		return result
	}

	// Got a response!
	if n >= 20 {
		code := response[0]
		switch code {
		case CodeCoAAck:
			result.Success = true
			result.SecretValid = true
			result.ErrorMsg = "CoA-ACK received (secret valid)"
		case CodeCoANak:
			// NAK means secret was correct but request was rejected (e.g., no matching session)
			result.Success = true
			result.SecretValid = true
			result.ErrorMsg = "CoA-NAK received (secret valid, no matching session)"
		default:
			result.ErrorMsg = fmt.Sprintf("Unknown response code: %d", code)
		}
	}

	return result
}

// buildCoAPacket creates a minimal CoA-Request packet
func buildCoAPacket(secret string) []byte {
	packet := make([]byte, 0, 64)

	// Code: CoA-Request (43)
	packet = append(packet, CodeCoARequest)

	// Identifier
	packet = append(packet, 0x01)

	// Length: placeholder
	packet = append(packet, 0x00, 0x00)

	// Authenticator: 16 bytes (will be computed)
	authenticator := make([]byte, 16)
	packet = append(packet, authenticator...)

	// Add a dummy attribute (NAS-IP-Address = 0.0.0.0)
	// Type=4, Length=6, Value=0.0.0.0
	packet = append(packet, 4, 6, 0, 0, 0, 0)

	// Update length
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))

	// Compute authenticator: MD5(Code + ID + Length + 16 zero bytes + Attributes + Secret)
	h := md5.New()
	h.Write(packet)
	h.Write([]byte(secret))
	auth := h.Sum(nil)
	copy(packet[4:20], auth)

	return packet
}
