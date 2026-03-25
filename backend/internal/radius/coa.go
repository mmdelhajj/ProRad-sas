package radius

import (
	"fmt"
	"log"
	"net"
	"os/exec"
	"strings"
	"time"

	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2866"
)

// MikroTik vendor ID
const MikrotikVendorID = 14988

// MikroTik vendor-specific attribute types
const (
	MikrotikRateLimit = 8
)

// COAClient sends Change of Authorization packets to NAS
type COAClient struct {
	nasIP     string
	coaPort   int
	secret    string
	timeout   time.Duration
}

// NewCOAClient creates a new CoA client
func NewCOAClient(nasIP string, coaPort int, secret string) *COAClient {
	return &COAClient{
		nasIP:   nasIP,
		coaPort: coaPort,
		secret:  secret,
		timeout: 5 * time.Second,
	}
}

// UpdateRateLimit sends a CoA packet to update user's rate limit
func (c *COAClient) UpdateRateLimit(username, sessionID, rateLimit string) error {
	// Strip "0x" prefix from session ID if present and convert to lowercase
	// MikroTik requires lowercase session ID for CoA to work!
	cleanSessionID := sessionID
	if strings.HasPrefix(sessionID, "0x") || strings.HasPrefix(sessionID, "0X") {
		cleanSessionID = sessionID[2:]
	}
	cleanSessionID = strings.ToLower(cleanSessionID)

	log.Printf("CoA: Sending rate-limit change to %s:%d for user=%s, session=%s, rate=%s",
		c.nasIP, c.coaPort, username, cleanSessionID, rateLimit)

	packet := radius.New(radius.CodeCoARequest, []byte(c.secret))

	// Add User-Name attribute
	if err := rfc2865.UserName_SetString(packet, username); err != nil {
		return fmt.Errorf("failed to set User-Name: %v", err)
	}

	// Add Acct-Session-Id if provided
	if cleanSessionID != "" {
		if err := rfc2866.AcctSessionID_SetString(packet, cleanSessionID); err != nil {
			return fmt.Errorf("failed to set Acct-Session-Id: %v", err)
		}
	}

	// Add additional attributes that ProRadius sends (required by MikroTik)
	// Acct-Status-Type = 0x30 (48)
	rfc2866.AcctStatusType_Set(packet, 48)
	// Acct-Delay-Time = 0x30 (48)
	rfc2866.AcctDelayTime_Set(packet, 48)
	// Acct-Input-Octets = 0x30 (48)
	rfc2866.AcctInputOctets_Set(packet, 48)

	// Add MikroTik-Rate-Limit as Vendor-Specific Attribute
	// VSA format: Vendor-ID (4 bytes) + Vendor-Type (1 byte) + Vendor-Length (1 byte) + Value
	rateLimitBytes := []byte(rateLimit)
	vsaData := make([]byte, 4+1+1+len(rateLimitBytes))
	// Vendor-ID (MikroTik = 14988)
	vendorID := uint32(MikrotikVendorID)
	vsaData[0] = byte(vendorID >> 24)
	vsaData[1] = byte(vendorID >> 16)
	vsaData[2] = byte(vendorID >> 8)
	vsaData[3] = byte(vendorID)
	// Vendor-Type (Rate-Limit = 8)
	vsaData[4] = byte(MikrotikRateLimit)
	// Vendor-Length (type + length + data)
	vsaData[5] = byte(2 + len(rateLimitBytes))
	// Value
	copy(vsaData[6:], rateLimitBytes)
	packet.Add(rfc2865.VendorSpecific_Type, radius.Attribute(vsaData))

	// Send CoA request
	addr := fmt.Sprintf("%s:%d", c.nasIP, c.coaPort)
	conn, err := net.DialTimeout("udp", addr, c.timeout)
	if err != nil {
		return fmt.Errorf("failed to connect to NAS: %v", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(c.timeout))

	// Encode and send
	packetBytes, err := packet.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode packet: %v", err)
	}

	_, err = conn.Write(packetBytes)
	if err != nil {
		return fmt.Errorf("failed to send CoA: %v", err)
	}

	// Read response
	respBuf := make([]byte, 4096)
	n, err := conn.Read(respBuf)
	if err != nil {
		return fmt.Errorf("failed to read CoA response: %v", err)
	}

	response, err := radius.Parse(respBuf[:n], []byte(c.secret))
	if err != nil {
		return fmt.Errorf("failed to parse CoA response: %v", err)
	}

	switch response.Code {
	case radius.CodeCoAACK:
		log.Printf("CoA: Rate limit updated for %s to %s", username, rateLimit)
		return nil
	case radius.CodeCoANAK:
		return fmt.Errorf("CoA NAK received - NAS rejected the request")
	default:
		return fmt.Errorf("unexpected CoA response code: %d", response.Code)
	}
}

// UpdateRateLimitViaRadclient uses radclient command to send CoA (like old ProRadius)
// This approach is more compatible with MikroTik as it uses FreeRADIUS dictionaries
func (c *COAClient) UpdateRateLimitViaRadclient(username, sessionID, rateLimit string) error {
	// Strip "0x" prefix from session ID if present and convert to lowercase
	// MikroTik requires lowercase session ID for CoA to work!
	cleanSessionID := sessionID
	if strings.HasPrefix(sessionID, "0x") || strings.HasPrefix(sessionID, "0X") {
		cleanSessionID = sessionID[2:]
	}
	cleanSessionID = strings.ToLower(cleanSessionID)

	log.Printf("CoA (radclient): Sending rate-limit change to %s:%d for user=%s, session=%s, rate=%s",
		c.nasIP, c.coaPort, username, cleanSessionID, rateLimit)

	// Build radclient command with stdin input
	addr := fmt.Sprintf("%s:%d", c.nasIP, c.coaPort)
	cmd := exec.Command("radclient", "-x", addr, "coa", c.secret)

	// Write attributes to stdin (format: User-Name = "x", Acct-Session-Id = "y", Mikrotik-Rate-Limit = "z")
	content := fmt.Sprintf("User-Name = \"%s\"\nAcct-Session-Id = \"%s\"\nMikrotik-Rate-Limit = \"%s\"\n",
		username, cleanSessionID, rateLimit)
	cmd.Stdin = strings.NewReader(content)

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	log.Printf("CoA (radclient): Output: %s", outputStr)

	// Check for ACK in output (success case)
	if strings.Contains(outputStr, "CoA-ACK") {
		log.Printf("CoA (radclient): Rate limit updated for %s to %s", username, rateLimit)
		return nil
	}

	// Check for NAK response
	if strings.Contains(outputStr, "CoA-NAK") {
		// Extract error cause if present
		if strings.Contains(outputStr, "Session-Context-Not-Found") {
			return fmt.Errorf("CoA NAK: session not found on NAS")
		}
		if strings.Contains(outputStr, "Unsupported-Extension") {
			return fmt.Errorf("CoA NAK: session not found or unsupported (check session ID)")
		}
		return fmt.Errorf("CoA NAK received: %s", outputStr)
	}

	if err != nil {
		return fmt.Errorf("radclient failed: %v - %s", err, outputStr)
	}

	return fmt.Errorf("unexpected radclient output: %s", outputStr)
}

// DisconnectViaRadclient uses radclient command to send Disconnect-Request
func (c *COAClient) DisconnectViaRadclient(username, sessionID string) error {
	// Strip "0x" prefix and convert to lowercase (MikroTik requirement)
	cleanSessionID := sessionID
	if strings.HasPrefix(sessionID, "0x") || strings.HasPrefix(sessionID, "0X") {
		cleanSessionID = sessionID[2:]
	}
	cleanSessionID = strings.ToLower(cleanSessionID)

	log.Printf("CoA (radclient): Sending disconnect to %s:%d for user=%s, session=%s",
		c.nasIP, c.coaPort, username, cleanSessionID)

	// Build radclient command with stdin input
	addr := fmt.Sprintf("%s:%d", c.nasIP, c.coaPort)
	cmd := exec.Command("radclient", "-x", addr, "disconnect", c.secret)

	content := fmt.Sprintf("User-Name = \"%s\"\nAcct-Session-Id = \"%s\"\n", username, cleanSessionID)
	cmd.Stdin = strings.NewReader(content)

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	log.Printf("CoA (radclient): Disconnect output: %s", outputStr)

	// Check for ACK in output (success case)
	if strings.Contains(outputStr, "Disconnect-ACK") {
		log.Printf("CoA (radclient): User %s disconnected", username)
		return nil
	}

	// Check for NAK response
	if strings.Contains(outputStr, "Disconnect-NAK") {
		if strings.Contains(outputStr, "Session-Context-Not-Found") || strings.Contains(outputStr, "Unsupported-Extension") {
			return fmt.Errorf("Disconnect NAK: session not found on NAS")
		}
		return fmt.Errorf("Disconnect NAK received: %s", outputStr)
	}

	if err != nil {
		return fmt.Errorf("radclient failed: %v - %s", err, outputStr)
	}

	return fmt.Errorf("unexpected radclient output: %s", outputStr)
}

// DisconnectUser sends a Disconnect-Request to terminate user session
func (c *COAClient) DisconnectUser(username, sessionID string) error {
	log.Printf("CoA: Sending Disconnect-Request to %s:%d for user=%s, session=%s", c.nasIP, c.coaPort, username, sessionID)
	packet := radius.New(radius.CodeDisconnectRequest, []byte(c.secret))

	// Add User-Name attribute
	if err := rfc2865.UserName_SetString(packet, username); err != nil {
		return fmt.Errorf("failed to set User-Name: %v", err)
	}

	// Add Acct-Session-Id if provided
	if sessionID != "" {
		if err := rfc2866.AcctSessionID_SetString(packet, sessionID); err != nil {
			return fmt.Errorf("failed to set Acct-Session-Id: %v", err)
		}
	}

	// Send Disconnect request
	addr := fmt.Sprintf("%s:%d", c.nasIP, c.coaPort)
	conn, err := net.DialTimeout("udp", addr, c.timeout)
	if err != nil {
		return fmt.Errorf("failed to connect to NAS: %v", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(c.timeout))

	packetBytes, err := packet.Encode()
	if err != nil {
		return fmt.Errorf("failed to encode packet: %v", err)
	}

	_, err = conn.Write(packetBytes)
	if err != nil {
		return fmt.Errorf("failed to send Disconnect: %v", err)
	}

	// Read response
	respBuf := make([]byte, 4096)
	n, err := conn.Read(respBuf)
	if err != nil {
		return fmt.Errorf("failed to read Disconnect response: %v", err)
	}

	response, err := radius.Parse(respBuf[:n], []byte(c.secret))
	if err != nil {
		return fmt.Errorf("failed to parse Disconnect response: %v", err)
	}

	switch response.Code {
	case radius.CodeDisconnectACK:
		log.Printf("CoA: User %s disconnected", username)
		return nil
	case radius.CodeDisconnectNAK:
		return fmt.Errorf("Disconnect NAK received - NAS rejected the request")
	default:
		return fmt.Errorf("unexpected Disconnect response code: %d", response.Code)
	}
}
