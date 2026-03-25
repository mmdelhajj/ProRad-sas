package mikrotik

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/proisp/backend/internal/database"
)

// Client represents a MikroTik RouterOS API client
type Client struct {
	Address  string
	Username string
	Password string
	FTPPort  int
	conn     net.Conn
	timeout  time.Duration
}

// ConnectionResult contains the result of a connection test
type ConnectionResult struct {
	Success     bool
	IsOnline    bool
	APIAuth     bool
	ErrorMsg    string
	RouterInfo  map[string]string
}

// NewClient creates a new MikroTik client
func NewClient(address, username, password string) *Client {
	return &Client{
		Address:  address,
		Username: username,
		Password: password,
		timeout:  5 * time.Second,
	}
}

// TestConnection tests connectivity and authentication
func (c *Client) TestConnection() ConnectionResult {
	result := ConnectionResult{
		RouterInfo: make(map[string]string),
	}

	// Step 1: Check if port is reachable
	conn, err := net.DialTimeout("tcp", c.Address, c.timeout)
	if err != nil {
		result.ErrorMsg = fmt.Sprintf("Cannot reach router: %v", err)
		return result
	}
	defer conn.Close()

	result.IsOnline = true
	c.conn = conn
	conn.SetDeadline(time.Now().Add(c.timeout))

	// Step 2: Try to authenticate with RouterOS API
	// Send login command
	if err := c.sendWord("/login"); err != nil {
		result.ErrorMsg = fmt.Sprintf("Failed to send login: %v", err)
		return result
	}
	if err := c.sendWord("=name=" + c.Username); err != nil {
		result.ErrorMsg = fmt.Sprintf("Failed to send username: %v", err)
		return result
	}
	if err := c.sendWord("=password=" + c.Password); err != nil {
		result.ErrorMsg = fmt.Sprintf("Failed to send password: %v", err)
		return result
	}
	if err := c.sendWord(""); err != nil {
		result.ErrorMsg = fmt.Sprintf("Failed to send end: %v", err)
		return result
	}

	// Read response
	response, err := c.readResponse()
	if err != nil {
		result.ErrorMsg = fmt.Sprintf("Failed to read response: %v", err)
		return result
	}

	// Check if login was successful
	for _, word := range response {
		if word == "!done" {
			result.APIAuth = true
			result.Success = true
		}
		if strings.HasPrefix(word, "!trap") {
			result.ErrorMsg = "Authentication failed: Invalid username or password"
			return result
		}
		if strings.HasPrefix(word, "=ret=") {
			// Old style login - need challenge response
			challenge := strings.TrimPrefix(word, "=ret=")
			if err := c.challengeLogin(challenge); err != nil {
				result.ErrorMsg = fmt.Sprintf("Challenge login failed: %v", err)
				return result
			}
			result.APIAuth = true
			result.Success = true
		}
	}

	// If authenticated, try to get system identity
	if result.APIAuth {
		identity, err := c.getIdentity()
		if err == nil {
			result.RouterInfo["identity"] = identity
		}
	}

	return result
}

// challengeLogin performs the old-style MD5 challenge-response login
func (c *Client) challengeLogin(challenge string) error {
	// Decode challenge
	challengeBytes, err := hex.DecodeString(challenge)
	if err != nil {
		return err
	}

	// Create MD5 hash: 0x00 + password + challenge
	h := md5.New()
	h.Write([]byte{0})
	h.Write([]byte(c.Password))
	h.Write(challengeBytes)
	response := hex.EncodeToString(h.Sum(nil))

	// Send challenge response
	c.sendWord("/login")
	c.sendWord("=name=" + c.Username)
	c.sendWord("=response=00" + response)
	c.sendWord("")

	// Read response
	resp, err := c.readResponse()
	if err != nil {
		return err
	}

	for _, word := range resp {
		if word == "!done" {
			return nil
		}
		if strings.HasPrefix(word, "!trap") {
			return fmt.Errorf("authentication failed")
		}
	}

	return nil
}

// getIdentity retrieves the router's identity
func (c *Client) getIdentity() (string, error) {
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	c.sendWord("/system/identity/print")
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return "", err
	}

	for _, word := range response {
		if strings.HasPrefix(word, "=name=") {
			return strings.TrimPrefix(word, "=name="), nil
		}
	}

	return "", fmt.Errorf("identity not found")
}

// sendWord sends a word to the RouterOS API
func (c *Client) sendWord(word string) error {
	// Encode length
	length := len(word)
	var lenBytes []byte

	if length < 0x80 {
		lenBytes = []byte{byte(length)}
	} else if length < 0x4000 {
		lenBytes = []byte{byte((length >> 8) | 0x80), byte(length)}
	} else if length < 0x200000 {
		lenBytes = []byte{byte((length >> 16) | 0xC0), byte(length >> 8), byte(length)}
	} else if length < 0x10000000 {
		lenBytes = []byte{byte((length >> 24) | 0xE0), byte(length >> 16), byte(length >> 8), byte(length)}
	} else {
		lenBytes = []byte{0xF0, byte(length >> 24), byte(length >> 16), byte(length >> 8), byte(length)}
	}

	// Send length + word
	if _, err := c.conn.Write(lenBytes); err != nil {
		return err
	}
	if len(word) > 0 {
		if _, err := c.conn.Write([]byte(word)); err != nil {
			return err
		}
	}
	return nil
}

// readResponse reads a complete response from RouterOS
// Continues reading until !done is received
func (c *Client) readResponse() ([]string, error) {
	var words []string
	gotDone := false

	for {
		word, err := c.readWord()
		if err != nil {
			if err == io.EOF {
				break
			}
			return words, err
		}

		// Empty word means end of current sentence, but not end of response
		// Keep reading until we see !done
		if word == "" {
			if gotDone {
				break
			}
			continue
		}

		words = append(words, word)

		if word == "!done" {
			gotDone = true
		}
	}

	return words, nil
}

// readWord reads a single word from the connection
func (c *Client) readWord() (string, error) {
	// Read length
	length, err := c.readLength()
	if err != nil {
		return "", err
	}

	if length == 0 {
		return "", nil
	}

	// Read word
	word := make([]byte, length)
	_, err = io.ReadFull(c.conn, word)
	if err != nil {
		return "", err
	}

	return string(word), nil
}

// readLength reads the length encoding from RouterOS
func (c *Client) readLength() (int, error) {
	b := make([]byte, 1)
	_, err := c.conn.Read(b)
	if err != nil {
		return 0, err
	}

	first := b[0]

	if first < 0x80 {
		return int(first), nil
	} else if first < 0xC0 {
		_, err := c.conn.Read(b)
		if err != nil {
			return 0, err
		}
		return int(first&0x3F)<<8 | int(b[0]), nil
	} else if first < 0xE0 {
		extra := make([]byte, 2)
		_, err := io.ReadFull(c.conn, extra)
		if err != nil {
			return 0, err
		}
		return int(first&0x1F)<<16 | int(extra[0])<<8 | int(extra[1]), nil
	} else if first < 0xF0 {
		extra := make([]byte, 3)
		_, err := io.ReadFull(c.conn, extra)
		if err != nil {
			return 0, err
		}
		return int(first&0x0F)<<24 | int(extra[0])<<16 | int(extra[1])<<8 | int(extra[2]), nil
	} else {
		extra := make([]byte, 4)
		_, err := io.ReadFull(c.conn, extra)
		if err != nil {
			return 0, err
		}
		return int(extra[0])<<24 | int(extra[1])<<16 | int(extra[2])<<8 | int(extra[3]), nil
	}
}

// Close closes the connection
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// RunCommand runs an arbitrary MikroTik command and returns the raw response
func (c *Client) RunCommand(command string) ([]map[string]string, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	c.conn.SetDeadline(time.Now().Add(c.timeout))

	// Send command
	if err := c.sendWord(command); err != nil {
		return nil, fmt.Errorf("failed to send command: %v", err)
	}
	if err := c.sendWord(""); err != nil {
		return nil, fmt.Errorf("failed to send end: %v", err)
	}

	// Read response
	response, err := c.readResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// Parse response into map slices
	var results []map[string]string
	current := make(map[string]string)

	for _, word := range response {
		if word == "!re" {
			if len(current) > 0 {
				results = append(results, current)
				current = make(map[string]string)
			}
		} else if strings.HasPrefix(word, "=") {
			parts := strings.SplitN(word[1:], "=", 2)
			if len(parts) == 2 {
				current[parts[0]] = parts[1]
			} else if len(parts) == 1 {
				current[parts[0]] = ""
			}
		} else if word == "!done" {
			if len(current) > 0 {
				results = append(results, current)
			}
		}
	}

	return results, nil
}

// ExportConfig retrieves the full router configuration.
// It saves the export to a temp file on the router, downloads it via FTP,
// then cleans up. Returns the config as a string (RouterOS script format).
func (c *Client) ExportConfig() (string, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return "", err
		}
	}

	// The /export command doesn't return data via the API protocol.
	// We must: 1) save to file on router, 2) download via FTP, 3) cleanup.
	tempFileName := fmt.Sprintf("proisp_export_%d", time.Now().UnixNano())

	// Step 1: Export config to file on router
	c.conn.SetDeadline(time.Now().Add(30 * time.Second))
	if err := c.sendWord("/export"); err != nil {
		return "", fmt.Errorf("failed to send export command: %v", err)
	}
	if err := c.sendWord("=file=" + tempFileName); err != nil {
		return "", fmt.Errorf("failed to send file param: %v", err)
	}
	if err := c.sendWord(""); err != nil {
		return "", fmt.Errorf("failed to send end: %v", err)
	}

	response, err := c.readResponse()
	if err != nil {
		return "", fmt.Errorf("failed to read export response: %v", err)
	}
	for _, word := range response {
		if strings.HasPrefix(word, "!trap") {
			errMsg := "export command failed"
			for _, w := range response {
				if strings.HasPrefix(w, "=message=") {
					errMsg = strings.TrimPrefix(w, "=message=")
					break
				}
			}
			return "", fmt.Errorf(errMsg)
		}
	}

	// Wait for the file to be written
	time.Sleep(2 * time.Second)

	// Step 2: Download the file via FTP
	host := c.Address
	if idx := strings.LastIndex(host, ":"); idx > 0 {
		host = host[:idx]
	}

	ftpPort := c.FTPPort
	if ftpPort == 0 {
		ftpPort = 21
	}
	ftpAddr := fmt.Sprintf("%s:%d", host, ftpPort)
	ftpConn, err := net.DialTimeout("tcp", ftpAddr, 5*time.Second)
	if err != nil {
		// FTP not available - try to read via API /file/print contents (small files)
		c.removeFile(tempFileName + ".rsc")
		return "", fmt.Errorf("FTP connection failed (port %d): %v", ftpPort, err)
	}
	ftpConn.Close()

	configText, err := c.downloadFileViaFTP(host, tempFileName+".rsc")

	// Step 3: Cleanup - remove temp file from router
	c.removeFile(tempFileName + ".rsc")

	if err != nil {
		return "", fmt.Errorf("FTP download failed: %v", err)
	}

	return configText, nil
}

// downloadFileViaFTP downloads a file from MikroTik using FTP with the same credentials.
func (c *Client) downloadFileViaFTP(host, filename string) (string, error) {
	ftpPort := c.FTPPort
	if ftpPort == 0 {
		ftpPort = 21
	}
	ftpConn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, ftpPort), 5*time.Second)
	if err != nil {
		return "", err
	}

	// Simple FTP client - read welcome
	ftpConn.SetDeadline(time.Now().Add(15 * time.Second))
	buf := make([]byte, 1024)
	n, _ := ftpConn.Read(buf)
	welcome := string(buf[:n])
	if !strings.HasPrefix(welcome, "220") {
		ftpConn.Close()
		return "", fmt.Errorf("unexpected FTP welcome: %s", welcome)
	}

	// Login
	fmt.Fprintf(ftpConn, "USER %s\r\n", c.Username)
	n, _ = ftpConn.Read(buf)
	resp := string(buf[:n])
	if !strings.HasPrefix(resp, "331") {
		ftpConn.Close()
		return "", fmt.Errorf("FTP USER failed: %s", resp)
	}

	fmt.Fprintf(ftpConn, "PASS %s\r\n", c.Password)
	n, _ = ftpConn.Read(buf)
	resp = string(buf[:n])
	if !strings.HasPrefix(resp, "230") {
		ftpConn.Close()
		return "", fmt.Errorf("FTP login failed: %s", resp)
	}

	// Switch to passive mode
	fmt.Fprintf(ftpConn, "PASV\r\n")
	n, _ = ftpConn.Read(buf)
	resp = string(buf[:n])
	if !strings.HasPrefix(resp, "227") {
		ftpConn.Close()
		return "", fmt.Errorf("FTP PASV failed: %s", resp)
	}

	// Parse PASV response to get data port: 227 Entering Passive Mode (h1,h2,h3,h4,p1,p2)
	pasvStart := strings.Index(resp, "(")
	pasvEnd := strings.Index(resp, ")")
	if pasvStart < 0 || pasvEnd < 0 {
		ftpConn.Close()
		return "", fmt.Errorf("cannot parse PASV response: %s", resp)
	}
	parts := strings.Split(resp[pasvStart+1:pasvEnd], ",")
	if len(parts) != 6 {
		ftpConn.Close()
		return "", fmt.Errorf("invalid PASV parts: %s", resp)
	}
	p1, _ := strconv.Atoi(parts[4])
	p2, _ := strconv.Atoi(parts[5])
	dataPort := p1*256 + p2

	// Connect to data port
	dataConn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, dataPort), 5*time.Second)
	if err != nil {
		ftpConn.Close()
		return "", fmt.Errorf("FTP data connection failed: %v", err)
	}

	// Request file
	fmt.Fprintf(ftpConn, "RETR %s\r\n", filename)
	n, _ = ftpConn.Read(buf)
	resp = string(buf[:n])
	if !strings.HasPrefix(resp, "150") && !strings.HasPrefix(resp, "125") {
		dataConn.Close()
		ftpConn.Close()
		return "", fmt.Errorf("FTP RETR failed: %s", resp)
	}

	// Read file data
	dataConn.SetDeadline(time.Now().Add(30 * time.Second))
	var sb strings.Builder
	readBuf := make([]byte, 32768)
	for {
		n, err := dataConn.Read(readBuf)
		if n > 0 {
			sb.Write(readBuf[:n])
		}
		if err != nil {
			break
		}
	}
	dataConn.Close()

	// Read transfer complete response
	ftpConn.SetDeadline(time.Now().Add(5 * time.Second))
	n, _ = ftpConn.Read(buf)

	fmt.Fprintf(ftpConn, "QUIT\r\n")
	ftpConn.Close()

	return sb.String(), nil
}

// removeFile removes a file from the MikroTik router via API.
func (c *Client) removeFile(filename string) {
	if c.conn == nil {
		return
	}
	c.conn.SetDeadline(time.Now().Add(5 * time.Second))
	c.sendWord("/file/remove")
	c.sendWord("=.id=" + filename)
	c.sendWord("")
	c.readResponse()
}

// Connect establishes connection and authenticates
func (c *Client) Connect() error {
	conn, err := net.DialTimeout("tcp", c.Address, c.timeout)
	if err != nil {
		return fmt.Errorf("cannot connect: %v", err)
	}
	c.conn = conn
	conn.SetDeadline(time.Now().Add(c.timeout))

	// Send login
	c.sendWord("/login")
	c.sendWord("=name=" + c.Username)
	c.sendWord("=password=" + c.Password)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("login failed: %v", err)
	}

	for _, word := range response {
		if word == "!done" {
			return nil
		}
		if strings.HasPrefix(word, "=ret=") {
			// Old style login
			challenge := strings.TrimPrefix(word, "=ret=")
			return c.challengeLogin(challenge)
		}
		if strings.HasPrefix(word, "!trap") {
			return fmt.Errorf("authentication failed")
		}
	}
	return nil
}

// ActiveSession represents an active PPPoE session with bandwidth info
type ActiveSession struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Service    string `json:"service"`
	CallerID   string `json:"caller_id"`
	Address    string `json:"address"`
	Uptime     string `json:"uptime"`
	Encoding   string `json:"encoding"`
	SessionID  string `json:"session_id"`
	LimitBytesIn  int64 `json:"limit_bytes_in"`
	LimitBytesOut int64 `json:"limit_bytes_out"`
	RxBytes    int64  `json:"rx_bytes"`
	TxBytes    int64  `json:"tx_bytes"`
	RxRate     int64  `json:"rx_rate"`  // Current rx rate in bytes/sec
	TxRate     int64  `json:"tx_rate"`  // Current tx rate in bytes/sec
}

// GetActiveSessionMap fetches ALL active PPP sessions from MikroTik in a single API call.
// Returns a map of username -> *ActiveSession for O(1) lookup.
// This is much faster than calling GetActiveSession per-user when processing many subscribers.
func (c *Client) GetActiveSessionMap() (map[string]*ActiveSession, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout * 3)) // longer timeout for bulk fetch

	c.sendWord("/ppp/active/print")
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to query all sessions: %v", err)
	}

	sessions := make(map[string]*ActiveSession)
	var current *ActiveSession

	for _, word := range response {
		if word == "!re" {
			if current != nil && current.Name != "" {
				sessions[current.Name] = current
			}
			current = &ActiveSession{}
		} else if current != nil {
			if strings.HasPrefix(word, "=.id=") {
				current.ID = strings.TrimPrefix(word, "=.id=")
			} else if strings.HasPrefix(word, "=name=") {
				current.Name = strings.TrimPrefix(word, "=name=")
			} else if strings.HasPrefix(word, "=service=") {
				current.Service = strings.TrimPrefix(word, "=service=")
			} else if strings.HasPrefix(word, "=caller-id=") {
				current.CallerID = strings.TrimPrefix(word, "=caller-id=")
			} else if strings.HasPrefix(word, "=address=") {
				current.Address = strings.TrimPrefix(word, "=address=")
			} else if strings.HasPrefix(word, "=uptime=") {
				current.Uptime = strings.TrimPrefix(word, "=uptime=")
			} else if strings.HasPrefix(word, "=session-id=") {
				current.SessionID = strings.TrimPrefix(word, "=session-id=")
			}
		}
	}
	// Don't forget the last session
	if current != nil && current.Name != "" {
		sessions[current.Name] = current
	}

	log.Printf("MikroTik: Fetched %d active PPP sessions in bulk", len(sessions))
	return sessions, nil
}

// GetActiveSession gets bandwidth info for an active PPPoE session
func (c *Client) GetActiveSession(username string) (*ActiveSession, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	// Query active PPP session
	c.sendWord("/ppp/active/print")
	c.sendWord("?name=" + username)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to query session: %v", err)
	}

	session := &ActiveSession{}
	foundSession := false

	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			session.ID = strings.TrimPrefix(word, "=.id=")
			foundSession = true
		} else if strings.HasPrefix(word, "=name=") {
			session.Name = strings.TrimPrefix(word, "=name=")
		} else if strings.HasPrefix(word, "=service=") {
			session.Service = strings.TrimPrefix(word, "=service=")
		} else if strings.HasPrefix(word, "=caller-id=") {
			session.CallerID = strings.TrimPrefix(word, "=caller-id=")
		} else if strings.HasPrefix(word, "=address=") {
			session.Address = strings.TrimPrefix(word, "=address=")
		} else if strings.HasPrefix(word, "=uptime=") {
			session.Uptime = strings.TrimPrefix(word, "=uptime=")
		} else if strings.HasPrefix(word, "=session-id=") {
			session.SessionID = strings.TrimPrefix(word, "=session-id=")
		}
	}

	if !foundSession {
		return nil, fmt.Errorf("user not connected")
	}

	// Get session bytes from queue/simple (tracks total traffic)
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/queue/simple/print")
	c.sendWord("?target=" + session.Address + "/32")
	c.sendWord("")

	response, err = c.readResponse()
	if err == nil {
		for _, word := range response {
			// bytes format: "target-upload/target-download" where target is the client
			// So parts[0] = what client uploaded, parts[1] = what client downloaded
			if strings.HasPrefix(word, "=bytes=") {
				bytesStr := strings.TrimPrefix(word, "=bytes=")
				parts := strings.Split(bytesStr, "/")
				if len(parts) == 2 {
					session.TxBytes, _ = strconv.ParseInt(parts[1], 10, 64) // upload (client sends) - second value
					session.RxBytes, _ = strconv.ParseInt(parts[0], 10, 64) // download (client receives) - first value
				}
			}
		}
	}

	// Fallback: try interface stats with different name formats
	if session.RxBytes == 0 && session.TxBytes == 0 {
		fallbackInterfaceNames := []string{
			"<pppoe-" + username + ">",
			"<pppoe-" + username + "-1>",
			"<pppoe-" + username + "-2>",
		}
		for _, interfaceName := range fallbackInterfaceNames {
			c.conn.SetDeadline(time.Now().Add(c.timeout))
			c.sendWord("/interface/print")
			c.sendWord("?name=" + interfaceName)
			c.sendWord("")

			response, err = c.readResponse()
			if err == nil {
				for _, word := range response {
					if strings.HasPrefix(word, "=rx-byte=") {
						val := strings.TrimPrefix(word, "=rx-byte=")
						session.RxBytes, _ = strconv.ParseInt(val, 10, 64)
					} else if strings.HasPrefix(word, "=tx-byte=") {
						val := strings.TrimPrefix(word, "=tx-byte=")
						session.TxBytes, _ = strconv.ParseInt(val, 10, 64)
					}
				}
				if session.RxBytes > 0 || session.TxBytes > 0 {
					break // Found data, stop trying
				}
			}
		}
	}

	// Try different interface name formats for PPPoE
	// MikroTik creates dynamic interfaces with -1, -2, etc. suffixes
	interfaceNames := []string{
		"<pppoe-" + username + ">",
		"<pppoe-" + username + "-1>",
		"<pppoe-" + username + "-2>",
		"<pppoe-" + username + "-3>",
		"pppoe-" + username,
		"pppoe-" + username + "-1",
		"pppoe-" + username + "-2",
		username,
	}

	for _, interfaceName := range interfaceNames {
		// Get real-time traffic using /interface/monitor-traffic (single sample)
		c.conn.SetDeadline(time.Now().Add(c.timeout))
		c.sendWord("/interface/monitor-traffic")
		c.sendWord("=interface=" + interfaceName)
		c.sendWord("=once=")
		c.sendWord("")

		response, err = c.readResponse()
		if err == nil {
			for _, word := range response {
				if strings.HasPrefix(word, "=rx-bits-per-second=") {
					val := strings.TrimPrefix(word, "=rx-bits-per-second=")
					bits, _ := strconv.ParseInt(val, 10, 64)
					session.RxRate = bits / 8 // Convert to bytes/sec
					log.Printf("Found traffic on interface %s: rx=%d bits/sec", interfaceName, bits)
				} else if strings.HasPrefix(word, "=tx-bits-per-second=") {
					val := strings.TrimPrefix(word, "=tx-bits-per-second=")
					bits, _ := strconv.ParseInt(val, 10, 64)
					session.TxRate = bits / 8 // Convert to bytes/sec
					log.Printf("Found traffic on interface %s: tx=%d bits/sec", interfaceName, bits)
				}
			}
			// If we got real data, stop trying other names
			if session.RxRate > 0 || session.TxRate > 0 {
				break
			}
		}
	}

	// If still no rate data, try to get from queue rate
	if session.RxRate == 0 && session.TxRate == 0 && session.Address != "" {
		c.conn.SetDeadline(time.Now().Add(c.timeout))
		c.sendWord("/queue/simple/print")
		c.sendWord("?target=" + session.Address + "/32")
		c.sendWord("")

		response, err = c.readResponse()
		if err == nil {
			for _, word := range response {
				// rate format: "upload/download" in bits per second
				// upload = user sends = router receives, download = user gets = router transmits
				if strings.HasPrefix(word, "=rate=") {
					rateStr := strings.TrimPrefix(word, "=rate=")
					parts := strings.Split(rateStr, "/")
					if len(parts) == 2 {
						uploadBits, _ := strconv.ParseInt(parts[0], 10, 64)
						downloadBits, _ := strconv.ParseInt(parts[1], 10, 64)
						// Match interface convention: Rx=upload (received from user), Tx=download (sent to user)
						session.RxRate = uploadBits / 8   // user upload rate
						session.TxRate = downloadBits / 8 // user download rate
						log.Printf("Got rate from queue for %s: upload=%d download=%d bits/sec", username, uploadBits, downloadBits)
					}
				}
			}
		}
	}

	return session, nil
}

// GetAllActiveSessions gets all active PPPoE sessions
func (c *Client) GetAllActiveSessions() ([]ActiveSession, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	// Query all active PPP sessions
	c.sendWord("/ppp/active/print")
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %v", err)
	}

	var sessions []ActiveSession
	var current ActiveSession

	for _, word := range response {
		if word == "!re" {
			if current.Name != "" {
				sessions = append(sessions, current)
			}
			current = ActiveSession{}
		} else if strings.HasPrefix(word, "=.id=") {
			current.ID = strings.TrimPrefix(word, "=.id=")
		} else if strings.HasPrefix(word, "=name=") {
			current.Name = strings.TrimPrefix(word, "=name=")
		} else if strings.HasPrefix(word, "=service=") {
			current.Service = strings.TrimPrefix(word, "=service=")
		} else if strings.HasPrefix(word, "=caller-id=") {
			current.CallerID = strings.TrimPrefix(word, "=caller-id=")
		} else if strings.HasPrefix(word, "=address=") {
			current.Address = strings.TrimPrefix(word, "=address=")
		} else if strings.HasPrefix(word, "=uptime=") {
			current.Uptime = strings.TrimPrefix(word, "=uptime=")
		} else if strings.HasPrefix(word, "=session-id=") {
			current.SessionID = strings.TrimPrefix(word, "=session-id=")
		}
	}

	// Don't forget the last session
	if current.Name != "" {
		sessions = append(sessions, current)
	}

	return sessions, nil
}

// DisconnectUser disconnects a PPPoE user by username
func (c *Client) DisconnectUser(username string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	// Find active PPPoE session
	c.sendWord("/ppp/active/print")
	c.sendWord("?name=" + username)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to find session: %v", err)
	}

	// Extract session ID
	var sessionID string
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			sessionID = strings.TrimPrefix(word, "=.id=")
			break
		}
	}

	if sessionID == "" {
		return fmt.Errorf("user not connected")
	}

	// Remove the session
	c.sendWord("/ppp/active/remove")
	c.sendWord("=.id=" + sessionID)
	c.sendWord("")

	response, err = c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to disconnect: %v", err)
	}

	for _, word := range response {
		if strings.HasPrefix(word, "!trap") {
			return fmt.Errorf("disconnect failed")
		}
	}

	return nil
}

// UpdateUserRateLimit updates the rate limit for a PPPoE user
// downloadKbps and uploadKbps are in Kilobits per second (e.g., 700 = 700k)
// Updates the existing PPPoE queue created by RADIUS
func (c *Client) UpdateUserRateLimit(username string, downloadKbps, uploadKbps int) error {
	return c.UpdateUserRateLimitWithIP(username, "", downloadKbps, uploadKbps)
}

// UpdateUserRateLimitWithIP updates the rate limit for a PPPoE user
// Searches by interface name (<pppoe-username>) or IP address
func (c *Client) UpdateUserRateLimitWithIP(username, ipAddress string, downloadKbps, uploadKbps int) error {
	log.Printf("MikroTik: UpdateUserRateLimitWithIP called for %s, IP=%s, rate=%dk/%dk", username, ipAddress, downloadKbps, uploadKbps)

	// Reuse existing connection if available, only connect if needed
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			log.Printf("MikroTik: Connect failed: %v", err)
			return err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	// PPPoE queues can have target as:
	// 1. Interface name like "<pppoe-username>"
	// 2. IP address like "10.11.0.20/32"
	interfaceTarget := "<pppoe-" + username + ">"
	ipTarget := ""
	if ipAddress != "" {
		ipTarget = ipAddress + "/32"
	}

	// List all queues and find the one matching our target
	c.sendWord("/queue/simple/print")
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to list queues: %v", err)
	}

	// Parse response to find queue with matching target or name
	// We need to find the main PPPoE queue, not CDN queues (which have dst= set)
	var queueID string

	// First pass: collect all queue info
	type queueInfo struct {
		id      string
		name    string
		target  string
		dst     string
		dynamic string
	}
	var queues []queueInfo
	var current queueInfo

	for _, word := range response {
		if word == "!re" {
			// New queue entry - save previous if exists
			if current.id != "" {
				queues = append(queues, current)
			}
			current = queueInfo{}
		}
		if strings.HasPrefix(word, "=.id=") {
			current.id = strings.TrimPrefix(word, "=.id=")
		}
		if strings.HasPrefix(word, "=name=") {
			current.name = strings.TrimPrefix(word, "=name=")
		}
		if strings.HasPrefix(word, "=target=") {
			current.target = strings.TrimPrefix(word, "=target=")
		}
		if strings.HasPrefix(word, "=dst=") {
			current.dst = strings.TrimPrefix(word, "=dst=")
		}
		if strings.HasPrefix(word, "=dynamic=") {
			current.dynamic = strings.TrimPrefix(word, "=dynamic=")
		}
	}
	// Don't forget last queue
	if current.id != "" {
		queues = append(queues, current)
	}

	// Find the main PPPoE queue (matches target but has NO dst - not a CDN queue)
	// Also try matching with -1, -2, -3 suffixes for dynamic PPPoE queues
	// AND with @domain suffixes (e.g. <pppoe-user@domain.com>)
	interfaceTargets := []string{
		interfaceTarget,                              // <pppoe-username>
		"<pppoe-" + username + "-1>",                 // <pppoe-username-1>
		"<pppoe-" + username + "-2>",                 // <pppoe-username-2>
		"<pppoe-" + username + "-3>",                 // <pppoe-username-3>
	}
	// Prefix for domain-suffix matching: <pppoe-username@
	domainPrefix := "<pppoe-" + username + "@"
	for _, q := range queues {
		// Skip CDN queues (they have dst set)
		if q.dst != "" {
			continue
		}
		// Match by name or target against all possible interface formats
		for _, target := range interfaceTargets {
			if q.name == target || q.target == target {
				queueID = q.id
				log.Printf("MikroTik: Found main queue %s (name=%s, target=%s, dynamic=%s) for user %s", q.id, q.name, q.target, q.dynamic, username)
				break
			}
		}
		// Match by domain suffix: <pppoe-username@domain>
		if queueID == "" {
			if strings.HasPrefix(q.name, domainPrefix) || strings.HasPrefix(q.target, domainPrefix) {
				queueID = q.id
				log.Printf("MikroTik: Found main queue %s (name=%s, target=%s, dynamic=%s) for user %s via domain prefix", q.id, q.name, q.target, q.dynamic, username)
			}
		}
		// Also match by IP target — but ONLY dynamic queues (skip manual/static queues)
		if queueID == "" && ipTarget != "" && q.target == ipTarget {
			if q.dynamic == "true" {
				queueID = q.id
				log.Printf("MikroTik: Found main queue %s (name=%s, target=%s, dynamic=%s) for user %s via IP", q.id, q.name, q.target, q.dynamic, username)
			} else {
				log.Printf("MikroTik: Skipping manual queue %s (name=%s, target=%s, dynamic=%s) for user %s — not modifying manual queues", q.id, q.name, q.target, q.dynamic, username)
			}
		}
		if queueID != "" {
			break
		}
	}

	// Format rate limit: "upload/download" in Kbps (e.g., "700k/700k")
	maxLimit := fmt.Sprintf("%dk/%dk", uploadKbps, downloadKbps)

	if queueID == "" {
		log.Printf("MikroTik: Queue not found for user %s (interface=%s, ip=%s)", username, interfaceTarget, ipTarget)
		return fmt.Errorf("queue not found for user %s", username)
	}
	log.Printf("MikroTik: Found queue %s for user %s, setting max-limit=%s", queueID, username, maxLimit)

	// Try to update existing queue
	c.sendWord("/queue/simple/set")
	c.sendWord("=.id=" + queueID)
	c.sendWord("=max-limit=" + maxLimit)
	c.sendWord("")

	response, err = c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to update rate limit: %v", err)
	}
	log.Printf("MikroTik: Queue set response: %v", response)

	for _, word := range response {
		if strings.HasPrefix(word, "!trap") {
			// Check for "can't edit dynamic object" error
			for _, w := range response {
				if strings.HasPrefix(w, "=message=") {
					errMsg := strings.TrimPrefix(w, "=message=")
					log.Printf("MikroTik: Queue update error: %s", errMsg)
					if strings.Contains(errMsg, "dynamic") {
						// Dynamic queue - try to update via PPP secret instead
						log.Printf("MikroTik: Dynamic queue detected, trying PPP secret")
						return c.updatePPPRateLimit(username, maxLimit)
					}
					return fmt.Errorf("rate limit update failed: %s", errMsg)
				}
			}
			return fmt.Errorf("rate limit update failed")
		}
	}

	log.Printf("MikroTik: Queue update successful for %s to %s", username, maxLimit)
	return nil
}

// updatePPPRateLimit updates rate limit for a PPPoE user via PPP secret
func (c *Client) updatePPPRateLimit(username, rateLimit string) error {
	// Update the PPP secret's rate-limit, which will take effect after the dynamic queue is recreated
	// First check if secret exists
	c.sendWord("/ppp/secret/print")
	c.sendWord("?name=" + username)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to query PPP secret: %v", err)
	}

	var secretID string
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			secretID = strings.TrimPrefix(word, "=.id=")
			break
		}
	}

	if secretID == "" {
		// No local secret - user authenticates via RADIUS
		// Return error so caller can send CoA first, then call RemoveDynamicQueueForRecreation
		log.Printf("MikroTik: RADIUS user - no local PPP secret, CoA required to update session rate-limit")
		return fmt.Errorf("RADIUS user - CoA required first")
	}

	// Update rate limit on the PPP secret
	c.sendWord("/ppp/secret/set")
	c.sendWord("=.id=" + secretID)
	c.sendWord("=rate-limit=" + rateLimit)
	c.sendWord("")

	response, err = c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to set PPP secret rate limit: %v", err)
	}

	for _, word := range response {
		if strings.HasPrefix(word, "!trap") {
			for _, w := range response {
				if strings.HasPrefix(w, "=message=") {
					return fmt.Errorf("PPP secret rate limit update failed: %s", strings.TrimPrefix(w, "=message="))
				}
			}
			return fmt.Errorf("PPP secret rate limit update failed")
		}
	}

	// PPP secret updated, active session will get new rate on reconnect
	return nil
}

// RemoveDynamicQueueForRecreation removes the dynamic queue for a RADIUS user
// MikroTik will automatically recreate it with the rate-limit from the session
// This should be called AFTER CoA updates the session's rate-limit attribute
func (c *Client) RemoveDynamicQueueForRecreation(username string) error {
	interfaceTarget := "<pppoe-" + username + ">"

	// Find the dynamic queue
	c.sendWord("/queue/simple/print")
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to list queues: %v", err)
	}

	// Parse to find the dynamic queue (one without dst=, matching target)
	type queueInfo struct {
		id      string
		name    string
		target  string
		dst     string
		dynamic string
	}
	var queues []queueInfo
	var current queueInfo

	for _, word := range response {
		if word == "!re" {
			if current.id != "" {
				queues = append(queues, current)
			}
			current = queueInfo{}
		}
		if strings.HasPrefix(word, "=.id=") {
			current.id = strings.TrimPrefix(word, "=.id=")
		}
		if strings.HasPrefix(word, "=name=") {
			current.name = strings.TrimPrefix(word, "=name=")
		}
		if strings.HasPrefix(word, "=target=") {
			current.target = strings.TrimPrefix(word, "=target=")
		}
		if strings.HasPrefix(word, "=dst=") {
			current.dst = strings.TrimPrefix(word, "=dst=")
		}
		if strings.HasPrefix(word, "=dynamic=") {
			current.dynamic = strings.TrimPrefix(word, "=dynamic=")
		}
	}
	if current.id != "" {
		queues = append(queues, current)
	}

	// Find the main dynamic queue (no dst, matches target, must be dynamic)
	var queueID string
	for _, q := range queues {
		if q.dst != "" {
			continue // Skip CDN queues
		}
		if q.name == interfaceTarget || q.target == interfaceTarget {
			// Only remove dynamic queues — never touch manual/static queues
			if q.dynamic != "true" {
				log.Printf("MikroTik: Skipping non-dynamic queue %s (name=%s, dynamic=%s) — not removing manual queues", q.id, q.name, q.dynamic)
				continue
			}
			queueID = q.id
			log.Printf("MikroTik: Found dynamic queue %s (name=%s, dynamic=%s) to remove for recreation", q.id, q.name, q.dynamic)
			break
		}
	}

	if queueID == "" {
		return fmt.Errorf("dynamic queue not found for user %s", username)
	}

	// Remove the queue - MikroTik will recreate it with new rate from session
	c.sendWord("/queue/simple/remove")
	c.sendWord("=.id=" + queueID)
	c.sendWord("")

	response, err = c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to remove queue: %v", err)
	}

	for _, word := range response {
		if strings.HasPrefix(word, "!trap") {
			for _, w := range response {
				if strings.HasPrefix(w, "=message=") {
					errMsg := strings.TrimPrefix(w, "=message=")
					return fmt.Errorf("failed to remove queue: %s", errMsg)
				}
			}
			return fmt.Errorf("failed to remove queue")
		}
	}

	log.Printf("MikroTik: Removed dynamic queue for %s - MikroTik should recreate with new rate", username)
	return nil
}

// RemoveUserQueue restores the original speed for a user (when FUP is reset)
// This updates the PPPoE queue back to the original RADIUS speed
func (c *Client) RemoveUserQueue(username string) error {
	// This function now just logs that FUP was reset
	// The actual speed restoration happens via RADIUS when user reconnects
	// or via UpdateUserRateLimit with the original speed
	log.Printf("MikroTik: FUP reset for %s - speed will be restored from RADIUS on reconnect", username)
	return nil
}

// RestoreUserSpeed restores the original speed for a user
// downloadMbps and uploadMbps are in Megabits per second (from service)
func (c *Client) RestoreUserSpeed(username string, downloadMbps, uploadMbps int64) error {
	// Convert Mbps to Kbps for the API
	return c.UpdateUserRateLimit(username, int(downloadMbps*1000), int(uploadMbps*1000))
}

// RestoreUserSpeedWithIP restores the original speed for a user using IP for queue lookup
func (c *Client) RestoreUserSpeedWithIP(username, ipAddress string, downloadMbps, uploadMbps int64) error {
	// Convert Mbps to Kbps for the API
	return c.UpdateUserRateLimitWithIP(username, ipAddress, int(downloadMbps*1000), int(uploadMbps*1000))
}

// GetConnectionCount returns the number of active connections for an IP address
// This queries the connection tracking table
func (c *Client) GetConnectionCount(ipAddress string) (int, error) {
	if ipAddress == "" {
		return 0, fmt.Errorf("IP address is required")
	}

	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return 0, err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	// Query connection tracking for this IP
	c.sendWord("/ip/firewall/connection/print")
	c.sendWord("?src-address=" + ipAddress)
	c.sendWord("=count-only=")
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return 0, fmt.Errorf("failed to query connections: %v", err)
	}

	// Parse count from response
	count := 0
	for _, word := range response {
		if strings.HasPrefix(word, "=ret=") {
			countStr := strings.TrimPrefix(word, "=ret=")
			count, _ = strconv.Atoi(countStr)
			break
		}
	}

	return count, nil
}

// ConnectionStats holds connection statistics for an IP
type ConnectionStats struct {
	TotalConnections   int
	UniqueDestinations int
}

// GetAllConnectionStats returns connection stats for ALL source IPs in one query
// Returns both total connections and unique destination IPs per source
func (c *Client) GetAllConnectionStats() (map[string]*ConnectionStats, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout * 3)) // Longer timeout for bulk query

	// Query all connections
	c.sendWord("/ip/firewall/connection/print")
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to get connections: %v", err)
	}

	// Track connections and unique destinations per source IP
	stats := make(map[string]*ConnectionStats)
	destTracker := make(map[string]map[string]bool) // srcIP -> set of destIPs

	var currentSrcIP, currentDstIP string

	for _, word := range response {
		if strings.HasPrefix(word, "=src-address=") {
			// Extract source IP (format: IP:port)
			addrPort := strings.TrimPrefix(word, "=src-address=")
			parts := strings.Split(addrPort, ":")
			if len(parts) > 0 {
				currentSrcIP = parts[0]
			}
		} else if strings.HasPrefix(word, "=dst-address=") {
			// Extract destination IP (format: IP:port)
			addrPort := strings.TrimPrefix(word, "=dst-address=")
			parts := strings.Split(addrPort, ":")
			if len(parts) > 0 {
				currentDstIP = parts[0]
			}
		} else if strings.HasPrefix(word, "=.id=") && currentSrcIP != "" {
			// End of a connection entry - record stats
			if stats[currentSrcIP] == nil {
				stats[currentSrcIP] = &ConnectionStats{}
				destTracker[currentSrcIP] = make(map[string]bool)
			}
			stats[currentSrcIP].TotalConnections++
			if currentDstIP != "" {
				destTracker[currentSrcIP][currentDstIP] = true
			}
			currentDstIP = ""
		}
	}

	// Calculate unique destination counts
	for ip, dests := range destTracker {
		if stats[ip] != nil {
			stats[ip].UniqueDestinations = len(dests)
		}
	}

	return stats, nil
}

// GetAllConnectionCounts returns connection counts for ALL source IPs (backward compatible)
func (c *Client) GetAllConnectionCounts() (map[string]int, error) {
	stats, err := c.GetAllConnectionStats()
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int)
	for ip, s := range stats {
		counts[ip] = s.TotalConnections
	}
	return counts, nil
}

// GetAllTTLMarks returns TTL values for ALL source IPs that have TTL marks
// Requires pre-configured mangle rules that mark connections with TTL info
func (c *Client) GetAllTTLMarks() (map[string][]int, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout * 2))

	ttlMarks := make(map[string][]int)
	ttlMarkNames := []string{"ttl_127", "ttl_63", "ttl_128", "ttl_64", "ttl_126", "ttl_62"}
	ttlValues := []int{127, 63, 128, 64, 126, 62}

	for i, markName := range ttlMarkNames {
		c.conn.SetDeadline(time.Now().Add(c.timeout))
		c.sendWord("/ip/firewall/connection/print")
		c.sendWord("?connection-mark=" + markName)
		c.sendWord("")

		response, err := c.readResponse()
		if err != nil {
			continue
		}

		// Extract source IPs that have this TTL mark
		for _, word := range response {
			if strings.HasPrefix(word, "=src-address=") {
				addrPort := strings.TrimPrefix(word, "=src-address=")
				parts := strings.Split(addrPort, ":")
				if len(parts) > 0 {
					ip := parts[0]
					// Add TTL value if not already present
					found := false
					for _, v := range ttlMarks[ip] {
						if v == ttlValues[i] {
							found = true
							break
						}
					}
					if !found {
						ttlMarks[ip] = append(ttlMarks[ip], ttlValues[i])
					}
				}
			}
		}
	}

	return ttlMarks, nil
}

// GetTTLValues samples TTL values from recent connections for an IP
// Returns unique TTL values seen for this source IP
func (c *Client) GetTTLValues(ipAddress string) ([]int, error) {
	if ipAddress == "" {
		return nil, fmt.Errorf("IP address is required")
	}

	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout * 2)) // Longer timeout for this query

	// We need to get TTL from firewall mangle rules or torch
	// First, check if there's a mangle rule tracking TTL
	// If not, we'll try to get a sample from torch

	// Method 1: Check for marked connections with TTL info
	// This requires pre-configured mangle rules

	// Method 2: Use torch to sample packets (less reliable but works without config)
	// Torch shows real-time traffic including TTL

	// For now, let's query the connection table and see if we can get any TTL info
	// MikroTik connection table doesn't store TTL directly, so we'll need mangle rules

	// Fallback: Return empty if no TTL tracking is configured
	// In production, you'd want to add mangle rules to track TTL

	ttlValues := []int{}

	// Try to get from existing connections with reply-src-address
	// This is a workaround - real TTL detection needs mangle rules
	c.sendWord("/ip/firewall/connection/print")
	c.sendWord("?src-address=" + ipAddress)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return ttlValues, nil // Return empty on error
	}

	// Count connections to estimate if there might be sharing
	// Different reply-dst-address values might indicate different destinations
	connectionCount := 0
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			connectionCount++
		}
	}

	// Heuristic: if many connections, try to detect TTL via mangle marks
	// Check for TTL-marked connections
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/ip/firewall/connection/print")
	c.sendWord("?src-address=" + ipAddress)
	c.sendWord("?connection-mark=ttl_127")
	c.sendWord("=count-only=")
	c.sendWord("")

	response, err = c.readResponse()
	if err == nil {
		for _, word := range response {
			if strings.HasPrefix(word, "=ret=") {
				countStr := strings.TrimPrefix(word, "=ret=")
				count, _ := strconv.Atoi(countStr)
				if count > 0 {
					ttlValues = append(ttlValues, 127)
				}
				break
			}
		}
	}

	// Check for TTL=63 marked connections
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/ip/firewall/connection/print")
	c.sendWord("?src-address=" + ipAddress)
	c.sendWord("?connection-mark=ttl_63")
	c.sendWord("=count-only=")
	c.sendWord("")

	response, err = c.readResponse()
	if err == nil {
		for _, word := range response {
			if strings.HasPrefix(word, "=ret=") {
				countStr := strings.TrimPrefix(word, "=ret=")
				count, _ := strconv.Atoi(countStr)
				if count > 0 {
					ttlValues = append(ttlValues, 63)
				}
				break
			}
		}
	}

	// Check for TTL=128 marked connections (normal Windows)
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/ip/firewall/connection/print")
	c.sendWord("?src-address=" + ipAddress)
	c.sendWord("?connection-mark=ttl_128")
	c.sendWord("=count-only=")
	c.sendWord("")

	response, err = c.readResponse()
	if err == nil {
		for _, word := range response {
			if strings.HasPrefix(word, "=ret=") {
				countStr := strings.TrimPrefix(word, "=ret=")
				count, _ := strconv.Atoi(countStr)
				if count > 0 {
					ttlValues = append(ttlValues, 128)
				}
				break
			}
		}
	}

	// Check for TTL=64 marked connections (normal Linux/Android)
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/ip/firewall/connection/print")
	c.sendWord("?src-address=" + ipAddress)
	c.sendWord("?connection-mark=ttl_64")
	c.sendWord("=count-only=")
	c.sendWord("")

	response, err = c.readResponse()
	if err == nil {
		for _, word := range response {
			if strings.HasPrefix(word, "=ret=") {
				countStr := strings.TrimPrefix(word, "=ret=")
				count, _ := strconv.Atoi(countStr)
				if count > 0 {
					ttlValues = append(ttlValues, 64)
				}
				break
			}
		}
	}

	return ttlValues, nil
}

// CDNTraffic represents traffic from a specific source IP range
type CDNTraffic struct {
	SourceIP string `json:"source_ip"`
	Bytes    int64  `json:"bytes"`
	Packets  int64  `json:"packets"`
}

// GetTrafficBySourceIP returns traffic data grouped by source IP for a given destination IP (subscriber)
// This requires IP accounting to be enabled on MikroTik (/ip/accounting)
// Returns traffic from sources TO the subscriber (download direction)
func (c *Client) GetTrafficBySourceIP(subscriberIP string) ([]CDNTraffic, error) {
	if subscriberIP == "" {
		return nil, fmt.Errorf("subscriber IP is required")
	}

	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout * 3))

	// First, take a snapshot of accounting data
	c.sendWord("/ip/accounting/snapshot/take")
	c.sendWord("")

	_, err := c.readResponse()
	if err != nil {
		// Snapshot might already exist or accounting not enabled
		log.Printf("MikroTik: IP accounting snapshot take: %v", err)
	}

	// Query accounting snapshot for traffic TO the subscriber IP (download traffic)
	c.conn.SetDeadline(time.Now().Add(c.timeout * 3))
	c.sendWord("/ip/accounting/snapshot/print")
	c.sendWord("?dst-address=" + subscriberIP)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to query accounting: %v", err)
	}

	var results []CDNTraffic
	currentTraffic := CDNTraffic{}

	for _, word := range response {
		if word == "!re" {
			if currentTraffic.SourceIP != "" {
				results = append(results, currentTraffic)
			}
			currentTraffic = CDNTraffic{}
		} else if strings.HasPrefix(word, "=src-address=") {
			currentTraffic.SourceIP = strings.TrimPrefix(word, "=src-address=")
		} else if strings.HasPrefix(word, "=bytes=") {
			val := strings.TrimPrefix(word, "=bytes=")
			currentTraffic.Bytes, _ = strconv.ParseInt(val, 10, 64)
		} else if strings.HasPrefix(word, "=packets=") {
			val := strings.TrimPrefix(word, "=packets=")
			currentTraffic.Packets, _ = strconv.ParseInt(val, 10, 64)
		}
	}

	// Don't forget the last entry
	if currentTraffic.SourceIP != "" {
		results = append(results, currentTraffic)
	}

	return results, nil
}

// GetConnectionDetails returns detailed connection info for an IP
func (c *Client) GetConnectionDetails(ipAddress string) ([]map[string]string, error) {
	if ipAddress == "" {
		return nil, fmt.Errorf("IP address is required")
	}

	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout * 2))

	// Query connections - limit to first 50 for performance
	c.sendWord("/ip/firewall/connection/print")
	c.sendWord("?src-address=" + ipAddress)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to query connections: %v", err)
	}

	var connections []map[string]string
	currentConn := make(map[string]string)
	count := 0
	maxConnections := 50

	for _, word := range response {
		if word == "!re" {
			if len(currentConn) > 0 && count < maxConnections {
				connections = append(connections, currentConn)
				count++
			}
			currentConn = make(map[string]string)
		} else if strings.HasPrefix(word, "=") {
			parts := strings.SplitN(word[1:], "=", 2)
			if len(parts) == 2 {
				currentConn[parts[0]] = parts[1]
			}
		}
	}

	// Don't forget the last connection
	if len(currentConn) > 0 && count < maxConnections {
		connections = append(connections, currentConn)
	}

	return connections, nil
}

// CDNConfig represents a CDN configuration for MikroTik sync
type CDNConfig struct {
	ID          uint
	Name        string
	Subnets     []string // List of CIDR subnets
	CompanyName string   // Company name for branding in comments
}

// SyncCDNAddressList creates or updates an address-list for a CDN on MikroTik
func (c *Client) SyncCDNAddressList(cdn CDNConfig) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	// Use company name for branding in comments
	companyName := cdn.CompanyName
	if companyName == "" {
		companyName = database.GetCompanyName()
	}
	if companyName == "" {
		companyName = "ISP"
	}

	listName := fmt.Sprintf("CDN-%s", cdn.Name)

	// First, remove existing entries for this CDN list
	c.conn.SetDeadline(time.Now().Add(c.timeout * 2))
	c.sendWord("/ip/firewall/address-list/print")
	c.sendWord("?list=" + listName)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		log.Printf("MikroTik: Failed to query address-list: %v", err)
	}

	// Collect existing entry IDs to remove
	var existingIDs []string
	currentID := ""
	for _, word := range response {
		if word == "!re" {
			if currentID != "" {
				existingIDs = append(existingIDs, currentID)
			}
			currentID = ""
		} else if strings.HasPrefix(word, "=.id=") {
			currentID = strings.TrimPrefix(word, "=.id=")
		}
	}
	if currentID != "" {
		existingIDs = append(existingIDs, currentID)
	}

	// Remove existing entries
	for _, id := range existingIDs {
		c.conn.SetDeadline(time.Now().Add(c.timeout))
		c.sendWord("/ip/firewall/address-list/remove")
		c.sendWord("=.id=" + id)
		c.sendWord("")
		c.readResponse() // Ignore errors
	}

	// Add new entries for each subnet
	addedCount := 0
	for _, subnet := range cdn.Subnets {
		subnet = strings.TrimSpace(subnet)
		// Also remove any newlines that might be in the subnet
		subnet = strings.ReplaceAll(subnet, "\n", "")
		subnet = strings.ReplaceAll(subnet, "\r", "")
		if subnet == "" {
			continue
		}

		c.conn.SetDeadline(time.Now().Add(c.timeout))
		c.sendWord("/ip/firewall/address-list/add")
		c.sendWord("=list=" + listName)
		c.sendWord("=address=" + subnet)
		c.sendWord("=comment=" + companyName + "-CDN-" + cdn.Name)
		c.sendWord("")

		response, err := c.readResponse()
		if err != nil {
			log.Printf("MikroTik: Failed to add address-list entry %s: %v", subnet, err)
		} else {
			// Check for trap (error) in response
			hasError := false
			for _, word := range response {
				if strings.HasPrefix(word, "!trap") {
					hasError = true
					log.Printf("MikroTik: Error adding address-list entry %s: %v", subnet, response)
					break
				}
			}
			if !hasError {
				addedCount++
				log.Printf("MikroTik: Added address-list entry: list=%s address=%s", listName, subnet)
			}
		}
	}

	log.Printf("MikroTik: Synced CDN address-list %s with %d subnets (added %d entries)", listName, len(cdn.Subnets), addedCount)
	return nil
}

// SyncCDNMangleRule creates or updates a mangle rule to count CDN traffic
func (c *Client) SyncCDNMangleRule(cdn CDNConfig) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	// Use company name for branding in comments
	companyName := cdn.CompanyName
	if companyName == "" {
		companyName = database.GetCompanyName()
	}
	if companyName == "" {
		companyName = "ISP"
	}

	listName := fmt.Sprintf("CDN-%s", cdn.Name)
	comment := fmt.Sprintf("%s-CDN-%s-counter", companyName, cdn.Name)

	// Check if mangle rule already exists
	c.conn.SetDeadline(time.Now().Add(c.timeout * 2))
	c.sendWord("/ip/firewall/mangle/print")
	c.sendWord("?comment=" + comment)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to query mangle rules: %v", err)
	}

	// Check if rule exists
	ruleExists := false
	for _, word := range response {
		if word == "!re" {
			ruleExists = true
			break
		}
	}

	if !ruleExists {
		// Create new mangle rule
		c.conn.SetDeadline(time.Now().Add(c.timeout))
		c.sendWord("/ip/firewall/mangle/add")
		c.sendWord("=chain=forward")
		c.sendWord("=src-address-list=" + listName)
		c.sendWord("=action=passthrough")
		c.sendWord("=comment=" + comment)
		c.sendWord("")

		_, err := c.readResponse()
		if err != nil {
			return fmt.Errorf("failed to create mangle rule: %v", err)
		}
		log.Printf("MikroTik: Created mangle rule for CDN %s", cdn.Name)
	}

	return nil
}

// CDNTrafficCounter holds traffic counters for a CDN
type CDNTrafficCounter struct {
	CDNID   uint
	CDNName string
	Bytes   int64
	Packets int64
}

// GetCDNTrafficCounters retrieves traffic counters from mangle rules for all CDNs
func (c *Client) GetCDNTrafficCounters(cdnNames []string, companyName string) ([]CDNTrafficCounter, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	// Use company name for branding in comments
	if companyName == "" {
		companyName = database.GetCompanyName()
	}
	if companyName == "" {
		companyName = "ISP"
	}

	var results []CDNTrafficCounter

	for _, cdnName := range cdnNames {
		comment := fmt.Sprintf("%s-CDN-%s-counter", companyName, cdnName)

		c.conn.SetDeadline(time.Now().Add(c.timeout * 2))
		c.sendWord("/ip/firewall/mangle/print")
		c.sendWord("?comment=" + comment)
		c.sendWord("")

		response, err := c.readResponse()
		if err != nil {
			log.Printf("MikroTik: Failed to get mangle counters for %s: %v", cdnName, err)
			continue
		}

		counter := CDNTrafficCounter{CDNName: cdnName}
		for _, word := range response {
			if strings.HasPrefix(word, "=bytes=") {
				val := strings.TrimPrefix(word, "=bytes=")
				counter.Bytes, _ = strconv.ParseInt(val, 10, 64)
			} else if strings.HasPrefix(word, "=packets=") {
				val := strings.TrimPrefix(word, "=packets=")
				counter.Packets, _ = strconv.ParseInt(val, 10, 64)
			}
		}

		results = append(results, counter)
	}

	return results, nil
}

// CDNSubnetConfig holds CDN info with subnets for traffic matching
type CDNSubnetConfig struct {
	ID      uint
	Name    string
	Subnets string // Comma/newline separated CIDR subnets
}

// GetCDNTrafficForSubscriber gets CDN traffic counters for a specific subscriber
// Queries connection tracking and matches source IPs against provided CDN subnets
// NO MikroTik configuration required - just reads connection table
func (c *Client) GetCDNTrafficForSubscriber(subscriberIP string, cdns []CDNSubnetConfig) ([]CDNTrafficCounter, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	// Query connection tracking - get connections where dst-address contains subscriber IP
	// MikroTik stores dst-address with port like "14.12.12.12:443"
	c.conn.SetDeadline(time.Now().Add(c.timeout * 10))
	c.sendWord("/ip/firewall/connection/print")
	c.sendWord("=.proplist=src-address,dst-address,orig-bytes,repl-bytes")
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to get connections: %v", err)
	}

	// Parse all connections and filter by subscriber IP
	type connData struct {
		srcIP     string
		dstIP     string
		origBytes int64
		replBytes int64
	}
	var allConnections []connData
	current := connData{}

	for _, word := range response {
		if word == "!re" {
			if current.srcIP != "" {
				allConnections = append(allConnections, current)
			}
			current = connData{}
		} else if strings.HasPrefix(word, "=src-address=") {
			current.srcIP = strings.TrimPrefix(word, "=src-address=")
			// Remove port if present
			if idx := strings.Index(current.srcIP, ":"); idx > 0 {
				current.srcIP = current.srcIP[:idx]
			}
		} else if strings.HasPrefix(word, "=dst-address=") {
			current.dstIP = strings.TrimPrefix(word, "=dst-address=")
			// Remove port if present
			if idx := strings.Index(current.dstIP, ":"); idx > 0 {
				current.dstIP = current.dstIP[:idx]
			}
		} else if strings.HasPrefix(word, "=orig-bytes=") {
			val := strings.TrimPrefix(word, "=orig-bytes=")
			current.origBytes, _ = strconv.ParseInt(val, 10, 64)
		} else if strings.HasPrefix(word, "=repl-bytes=") {
			val := strings.TrimPrefix(word, "=repl-bytes=")
			current.replBytes, _ = strconv.ParseInt(val, 10, 64)
		}
	}
	// Don't forget last entry
	if current.srcIP != "" {
		allConnections = append(allConnections, current)
	}

	// Filter connections for this subscriber (check both src and dst)
	var connections []connData
	for _, conn := range allConnections {
		if conn.dstIP == subscriberIP || conn.srcIP == subscriberIP {
			connections = append(connections, conn)
		}
	}

	// Match connections against each CDN's subnets
	var results []CDNTrafficCounter
	for _, cdn := range cdns {
		var totalBytes int64
		subnets := parseSubnetList(cdn.Subnets)

		for _, conn := range connections {
			// Determine remote IP and download bytes based on direction
			var remoteIP string
			var downloadBytes int64

			if conn.srcIP == subscriberIP {
				// Outbound connection: subscriber -> remote
				// dstIP is the remote server (check if it's CDN)
				// repl-bytes = bytes coming back = download
				remoteIP = conn.dstIP
				downloadBytes = conn.replBytes
			} else {
				// Inbound connection: remote -> subscriber
				// srcIP is the remote server (check if it's CDN)
				// orig-bytes = bytes from source = download
				remoteIP = conn.srcIP
				downloadBytes = conn.origBytes
			}

			// Check if remote IP matches CDN subnet
			for _, subnet := range subnets {
				matched := isIPInCIDR(remoteIP, subnet)
				if matched {
					totalBytes += downloadBytes
					log.Printf("CDN Traffic: remote=%s matched CDN %s subnet %s, download=%d bytes", remoteIP, cdn.Name, subnet, downloadBytes)
					break
				}
			}
		}

		results = append(results, CDNTrafficCounter{
			CDNID:   cdn.ID,
			CDNName: cdn.Name,
			Bytes:   totalBytes,
		})
	}

	return results, nil
}

// GetCDNTrafficViaTorch gets per-CDN traffic rates using MikroTik Torch on the subscriber's
// PPPoE interface. Works even when connection tracking is disabled or NAT is on a different device.
// Returns CDNTrafficCounter with Bytes = bytes/sec rate (not cumulative).
func (c *Client) GetCDNTrafficViaTorch(subscriberIP string, cdns []CDNSubnetConfig) ([]CDNTrafficCounter, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	// Step 1: Find the PPPoE interface for this subscriber
	c.conn.SetDeadline(time.Now().Add(15 * time.Second))
	c.sendWord("/ppp/active/print")
	c.sendWord("?address=" + subscriberIP)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to find PPPoE session: %v", err)
	}

	var ifaceName string
	for _, word := range response {
		if strings.HasPrefix(word, "=name=") {
			username := strings.TrimPrefix(word, "=name=")
			ifaceName = "<pppoe-" + username + ">"
			break
		}
	}

	if ifaceName == "" {
		return nil, fmt.Errorf("subscriber not connected or interface not found")
	}

	log.Printf("CDN Torch: Interface %s for subscriber %s", ifaceName, subscriberIP)

	// Step 2: Pre-parse CDN subnets for fast matching
	type cdnEntry struct {
		config  CDNSubnetConfig
		subnets []string
	}
	cdnEntries := make([]cdnEntry, 0, len(cdns))
	cdnRates := make(map[uint]int64) // cdnID -> bytes/sec
	for _, cdn := range cdns {
		cdnEntries = append(cdnEntries, cdnEntry{
			config:  cdn,
			subnets: parseSubnetList(cdn.Subnets),
		})
		cdnRates[cdn.ID] = 0
	}

	matchCDN := func(ip string) *CDNSubnetConfig {
		for i := range cdnEntries {
			for _, subnet := range cdnEntries[i].subnets {
				if isIPInCIDR(ip, subnet) {
					return &cdnEntries[i].config
				}
			}
		}
		return nil
	}

	// Step 3: Run torch for 2 seconds on the subscriber's PPPoE interface
	c.conn.SetDeadline(time.Now().Add(20 * time.Second))
	c.sendWord("/tool/torch")
	c.sendWord("=interface=" + ifaceName)
	c.sendWord("=src-address=0.0.0.0/0")
	c.sendWord("=dst-address=0.0.0.0/0")
	c.sendWord("=port=any")
	c.sendWord("=ip-protocol=any")
	c.sendWord("=duration=2")
	c.sendWord("")

	// Step 4: Parse torch output and match against CDN subnets
	for {
		word, err := c.readWord()
		if err != nil || word == "!done" {
			break
		}

		if word == "!trap" {
			errMsg := ""
			for {
				attr, err := c.readWord()
				if err != nil || attr == "" {
					break
				}
				if strings.HasPrefix(attr, "=message=") {
					errMsg = strings.TrimPrefix(attr, "=message=")
				}
			}
			return nil, fmt.Errorf("torch error: %s", errMsg)
		}

		if word == "!re" {
			var srcAddr, dstAddr string
			var txRate int64

			for {
				attr, err := c.readWord()
				if err != nil || attr == "" {
					break
				}
				switch {
				case strings.HasPrefix(attr, "=src-address="):
					srcAddr = strings.TrimPrefix(attr, "=src-address=")
				case strings.HasPrefix(attr, "=dst-address="):
					dstAddr = strings.TrimPrefix(attr, "=dst-address=")
				case strings.HasPrefix(attr, "=tx="):
					bits, _ := strconv.ParseInt(strings.TrimPrefix(attr, "=tx="), 10, 64)
					txRate = bits / 8 // bps → bytes/sec
				case strings.HasPrefix(attr, "=rx="):
					// rxRate not used — txRate covers download in both directions
					_ = attr
				}
			}

			if srcAddr == "" || dstAddr == "" {
				continue
			}

			// On a PPPoE interface, txRate = bytes going TO the subscriber (download).
			// Subscribers INITIATE connections (src=subscriber, dst=CDN), so:
			//   - When src=subscriber → remote/CDN is the dst → download = txRate
			//   - When src=CDN → remote/CDN is the src → download = txRate (still going TO subscriber)
			// In both cases txRate = download. Pick the remote IP (not the subscriber's IP).
			remoteIP := dstAddr
			if srcAddr != subscriberIP {
				remoteIP = srcAddr
			}
			if cdn := matchCDN(remoteIP); cdn != nil {
				cdnRates[cdn.ID] += txRate
				log.Printf("CDN Torch: remote=%s CDN=%s dl=%d B/s", remoteIP, cdn.Name, txRate)
			}
		}
	}

	// Step 5: Build results — Bytes = bytes/sec rate (not cumulative)
	var results []CDNTrafficCounter
	for _, cdn := range cdns {
		results = append(results, CDNTrafficCounter{
			CDNID:   cdn.ID,
			CDNName: cdn.Name,
			Bytes:   cdnRates[cdn.ID],
		})
	}

	log.Printf("CDN Torch: Completed for %s: %v", subscriberIP, cdnRates)
	return results, nil
}

// PortRuleTrafficConfig holds the config for port-rule traffic matching in Torch
type PortRuleTrafficConfig struct {
	ID        uint
	Name      string
	Port      string // Port number as string e.g. "8080"
	Direction string // src, dst, both
}

// PortRuleTrafficCounter holds per-port-rule bytes/sec from Torch
type PortRuleTrafficCounter struct {
	RuleID   uint
	RuleName string
	Bytes    int64 // bytes/sec rate
}

// GetCombinedTrafficViaTorch runs ONE Torch session on the subscriber's PPPoE interface
// and matches traffic against both CDN subnets and port-based rules in a single pass.
// Returns CDN counters and port-rule counters. CDNIsRate=true (bytes/sec from Torch).
func (c *Client) GetCombinedTrafficViaTorch(subscriberIP string, cdns []CDNSubnetConfig, portRules []PortRuleTrafficConfig) ([]CDNTrafficCounter, []PortRuleTrafficCounter, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, nil, err
		}
	}

	// Step 1: Find the PPPoE interface for this subscriber
	c.conn.SetDeadline(time.Now().Add(15 * time.Second))
	c.sendWord("/ppp/active/print")
	c.sendWord("?address=" + subscriberIP)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find PPPoE session: %v", err)
	}

	var ifaceName string
	for _, word := range response {
		if strings.HasPrefix(word, "=name=") {
			username := strings.TrimPrefix(word, "=name=")
			ifaceName = "<pppoe-" + username + ">"
			break
		}
	}

	if ifaceName == "" {
		return nil, nil, fmt.Errorf("subscriber not connected or interface not found")
	}

	// Step 2: Pre-parse CDN subnets for fast matching
	type cdnEntry struct {
		config  CDNSubnetConfig
		subnets []string
	}
	cdnEntries := make([]cdnEntry, 0, len(cdns))
	cdnRates := make(map[uint]int64)
	for _, cdn := range cdns {
		cdnEntries = append(cdnEntries, cdnEntry{
			config:  cdn,
			subnets: parseSubnetList(cdn.Subnets),
		})
		cdnRates[cdn.ID] = 0
	}

	matchCDN := func(ip string) *CDNSubnetConfig {
		for i := range cdnEntries {
			for _, subnet := range cdnEntries[i].subnets {
				if isIPInCIDR(ip, subnet) {
					return &cdnEntries[i].config
				}
			}
		}
		return nil
	}

	// Step 3: Pre-parse port rules for fast matching
	portRuleRates := make(map[uint]int64)
	for _, pr := range portRules {
		portRuleRates[pr.ID] = 0
	}

	matchPortRule := func(srcPort, dstPort string) *PortRuleTrafficConfig {
		for i := range portRules {
			pr := &portRules[i]
			switch pr.Direction {
			case "src":
				if srcPort == pr.Port {
					return pr
				}
			case "dst":
				if dstPort == pr.Port {
					return pr
				}
			default: // "both"
				if srcPort == pr.Port || dstPort == pr.Port {
					return pr
				}
			}
		}
		return nil
	}

	// Step 4: Run torch for 2 seconds on the subscriber's PPPoE interface
	c.conn.SetDeadline(time.Now().Add(20 * time.Second))
	c.sendWord("/tool/torch")
	c.sendWord("=interface=" + ifaceName)
	c.sendWord("=src-address=0.0.0.0/0")
	c.sendWord("=dst-address=0.0.0.0/0")
	c.sendWord("=port=any")
	c.sendWord("=ip-protocol=any")
	c.sendWord("=duration=2")
	c.sendWord("")

	// Step 5: Parse torch output and match against CDN subnets and port rules
	for {
		word, err := c.readWord()
		if err != nil || word == "!done" {
			break
		}

		if word == "!trap" {
			errMsg := ""
			for {
				attr, err := c.readWord()
				if err != nil || attr == "" {
					break
				}
				if strings.HasPrefix(attr, "=message=") {
					errMsg = strings.TrimPrefix(attr, "=message=")
				}
			}
			return nil, nil, fmt.Errorf("torch error: %s", errMsg)
		}

		if word == "!re" {
			var srcAddr, dstAddr, srcPort, dstPort string
			var txRate int64

			for {
				attr, err := c.readWord()
				if err != nil || attr == "" {
					break
				}
				switch {
				case strings.HasPrefix(attr, "=src-address="):
					srcAddr = strings.TrimPrefix(attr, "=src-address=")
				case strings.HasPrefix(attr, "=dst-address="):
					dstAddr = strings.TrimPrefix(attr, "=dst-address=")
				case strings.HasPrefix(attr, "=src-port="):
					srcPort = strings.TrimPrefix(attr, "=src-port=")
				case strings.HasPrefix(attr, "=dst-port="):
					dstPort = strings.TrimPrefix(attr, "=dst-port=")
				case strings.HasPrefix(attr, "=tx="):
					bits, _ := strconv.ParseInt(strings.TrimPrefix(attr, "=tx="), 10, 64)
					txRate = bits / 8 // bps → bytes/sec
				}
			}

			if srcAddr == "" || dstAddr == "" {
				continue
			}

			// Match CDN by remote IP
			if len(cdns) > 0 {
				remoteIP := dstAddr
				if srcAddr != subscriberIP {
					remoteIP = srcAddr
				}
				if cdn := matchCDN(remoteIP); cdn != nil {
					cdnRates[cdn.ID] += txRate
				}
			}

			// Match port rules by src-port or dst-port
			if len(portRules) > 0 {
				if pr := matchPortRule(srcPort, dstPort); pr != nil {
					portRuleRates[pr.ID] += txRate
				}
			}
		}
	}

	// Step 6: Build CDN results
	var cdnResults []CDNTrafficCounter
	for _, cdn := range cdns {
		cdnResults = append(cdnResults, CDNTrafficCounter{
			CDNID:   cdn.ID,
			CDNName: cdn.Name,
			Bytes:   cdnRates[cdn.ID],
		})
	}

	// Step 7: Build port rule results
	var portRuleResults []PortRuleTrafficCounter
	for _, pr := range portRules {
		portRuleResults = append(portRuleResults, PortRuleTrafficCounter{
			RuleID:   pr.ID,
			RuleName: pr.Name,
			Bytes:    portRuleRates[pr.ID],
		})
	}

	log.Printf("CombinedTorch: Completed for %s: CDNs=%v PortRules=%v", subscriberIP, cdnRates, portRuleRates)
	return cdnResults, portRuleResults, nil
}

// parseSubnetList splits subnet string into slice
func parseSubnetList(subnets string) []string {
	var result []string
	for _, s := range strings.FieldsFunc(subnets, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';' || r == ' '
	}) {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

// isIPInAddressList checks if an IP belongs to a MikroTik address list
func (c *Client) isIPInAddressList(ip, listName string) bool {
	// Query address list
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/ip/firewall/address-list/print")
	c.sendWord("?list=" + listName)
	c.sendWord("")

	response, _ := c.readResponse()

	// Parse subnets from address list
	for _, word := range response {
		if strings.HasPrefix(word, "=address=") {
			subnet := strings.TrimPrefix(word, "=address=")
			if isIPInCIDR(ip, subnet) {
				return true
			}
		}
	}

	return false
}

// isIPInCIDR checks if an IP is within a CIDR range
func isIPInCIDR(ipStr, cidr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		// Try as single IP
		if cidr == ipStr {
			return true
		}
		return false
	}

	return network.Contains(ip)
}

// RemoveCDNConfig removes CDN address-list and mangle rule from MikroTik
func (c *Client) RemoveCDNConfig(cdnName string, companyName string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	// Use company name for branding in comments
	if companyName == "" {
		companyName = database.GetCompanyName()
	}
	if companyName == "" {
		companyName = "ISP"
	}

	listName := fmt.Sprintf("CDN-%s", cdnName)
	comment := fmt.Sprintf("%s-CDN-%s-counter", companyName, cdnName)

	// Remove mangle rule
	c.conn.SetDeadline(time.Now().Add(c.timeout * 2))
	c.sendWord("/ip/firewall/mangle/print")
	c.sendWord("?comment=" + comment)
	c.sendWord("")

	response, _ := c.readResponse()
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			id := strings.TrimPrefix(word, "=.id=")
			c.conn.SetDeadline(time.Now().Add(c.timeout))
			c.sendWord("/ip/firewall/mangle/remove")
			c.sendWord("=.id=" + id)
			c.sendWord("")
			c.readResponse()
		}
	}

	// Remove address-list entries
	c.conn.SetDeadline(time.Now().Add(c.timeout * 2))
	c.sendWord("/ip/firewall/address-list/print")
	c.sendWord("?list=" + listName)
	c.sendWord("")

	response, _ = c.readResponse()
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			id := strings.TrimPrefix(word, "=.id=")
			c.conn.SetDeadline(time.Now().Add(c.timeout))
			c.sendWord("/ip/firewall/address-list/remove")
			c.sendWord("=.id=" + id)
			c.sendWord("")
			c.readResponse()
		}
	}

	log.Printf("MikroTik: Removed CDN config for %s", cdnName)
	return nil
}

// SubscriberCDNConfig holds CDN queue configuration for a subscriber
type SubscriberCDNConfig struct {
	CDNName     string
	SpeedLimit  int64  // in Mbps
	Subnets     string // Comma/newline separated CIDR subnets
	CompanyName string // Company name for queue comment
}

// SyncSubscriberCDNQueues creates or updates queue rules to limit CDN traffic for a subscriber
// CDN queues target the PPPoE interface directly so they work and get removed when PPPoE disconnects
func (c *Client) SyncSubscriberCDNQueues(subscriberIP string, username string, cdnConfigs []SubscriberCDNConfig) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	// Target the PPPoE interface directly
	pppoeInterface := "<pppoe-" + username + ">"

	for _, cdn := range cdnConfigs {
		if cdn.SpeedLimit <= 0 || cdn.Subnets == "" {
			continue
		}

		// Parse subnets
		subnets := parseSubnetList(cdn.Subnets)
		if len(subnets) == 0 {
			continue
		}

		// Use first subnet for dst
		dstAddress := strings.Join(subnets, ",")

		queueName := fmt.Sprintf("cdn-%s-%s", username, cdn.CDNName)
		// Use company name for branding in comments
		companyName := cdn.CompanyName
		if companyName == "" {
			companyName = database.GetCompanyName()
		}
		if companyName == "" {
			companyName = "ISP"
		}
		comment := fmt.Sprintf("%s-CDN-Queue-%s", companyName, username)

		// Speed limit in format "10M/10M" (upload/download) - SpeedLimit is in Mbps
		speedLimit := fmt.Sprintf("%dM/%dM", cdn.SpeedLimit, cdn.SpeedLimit)

		// Check if queue already exists
		c.conn.SetDeadline(time.Now().Add(c.timeout * 2))
		c.sendWord("/queue/simple/print")
		c.sendWord("?name=" + queueName)
		c.sendWord("")

		response, err := c.readResponse()
		if err != nil {
			log.Printf("MikroTik: Failed to query queue: %v", err)
			continue
		}

		// Check if queue exists
		queueExists := false
		var queueID string
		for _, word := range response {
			if word == "!re" {
				queueExists = true
			}
			if strings.HasPrefix(word, "=.id=") {
				queueID = strings.TrimPrefix(word, "=.id=")
			}
		}

		if queueExists && queueID != "" {
			// Update existing queue - move to top
			c.conn.SetDeadline(time.Now().Add(c.timeout * 2))
			c.sendWord("/queue/simple/set")
			c.sendWord("=.id=" + queueID)
			c.sendWord("=target=" + pppoeInterface)
			c.sendWord("=dst=" + dstAddress)
			c.sendWord("=max-limit=" + speedLimit)
			c.sendWord("")
			resp, _ := c.readResponse()
			hasError := false
			for _, word := range resp {
				if strings.HasPrefix(word, "!trap") {
					hasError = true
					log.Printf("MikroTik: Error updating queue: %v", resp)
				}
			}
			if !hasError {
				// Move queue to top
				c.conn.SetDeadline(time.Now().Add(c.timeout))
				c.sendWord("/queue/simple/move")
				c.sendWord("=numbers=" + queueID)
				c.sendWord("=destination=0")
				c.sendWord("")
				c.readResponse()
				log.Printf("MikroTik: Updated CDN queue %s for %s: %s dst=%s (moved to top)", cdn.CDNName, username, speedLimit, dstAddress)
			}
		} else {
			// Create new queue targeting PPPoE interface, place at top
			c.conn.SetDeadline(time.Now().Add(c.timeout * 2))
			c.sendWord("/queue/simple/add")
			c.sendWord("=name=" + queueName)
			c.sendWord("=target=" + pppoeInterface)
			c.sendWord("=dst=" + dstAddress)
			c.sendWord("=max-limit=" + speedLimit)
			c.sendWord("=priority=1")
			c.sendWord("=comment=" + comment)
			c.sendWord("=place-before=*0")
			c.sendWord("")

			resp, err := c.readResponse()
			hasError := false
			for _, word := range resp {
				if strings.HasPrefix(word, "!trap") || strings.Contains(word, "=message=") {
					hasError = true
					log.Printf("MikroTik: Error creating CDN queue: %v", resp)
				}
			}
			if err != nil {
				log.Printf("MikroTik: Failed to create CDN queue for %s: %v", username, err)
			} else if !hasError {
				log.Printf("MikroTik: Created CDN queue %s for %s: %s dst=%s target=%s", cdn.CDNName, username, speedLimit, dstAddress, pppoeInterface)
			}
		}
	}

	return nil
}

// RemoveSubscriberCDNQueues removes all CDN queue rules for a subscriber
func (c *Client) RemoveSubscriberCDNQueues(username string, companyName string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	// Use company name for branding in comments
	if companyName == "" {
		companyName = database.GetCompanyName()
	}
	if companyName == "" {
		companyName = "ISP"
	}

	comment := fmt.Sprintf("%s-CDN-Queue-%s", companyName, username)

	// Find and remove queues with matching comment
	c.conn.SetDeadline(time.Now().Add(c.timeout * 2))
	c.sendWord("/queue/simple/print")
	c.sendWord("?comment=" + comment)
	c.sendWord("")

	response, _ := c.readResponse()
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			id := strings.TrimPrefix(word, "=.id=")
			c.conn.SetDeadline(time.Now().Add(c.timeout))
			c.sendWord("/queue/simple/remove")
			c.sendWord("=.id=" + id)
			c.sendWord("")
			c.readResponse()
		}
	}

	log.Printf("MikroTik: Removed CDN queues for %s", username)
	return nil
}

// CountTTLRules counts TTL detection mangle rules by comment
func (c *Client) CountTTLRules(comment string) (int, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return 0, err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout * 2))

	// Query all mangle rules and filter by comment containing our prefix
	c.sendWord("/ip/firewall/mangle/print")
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return 0, fmt.Errorf("failed to query mangle rules: %v", err)
	}

	// Count entries that have matching comment
	count := 0
	currentComment := ""
	for _, word := range response {
		if strings.HasPrefix(word, "=comment=") {
			currentComment = strings.TrimPrefix(word, "=comment=")
		}
		if word == "!re" {
			// Check if previous entry had matching comment
			if strings.Contains(currentComment, comment) {
				count++
			}
			currentComment = ""
		}
	}
	// Check last entry
	if strings.Contains(currentComment, comment) {
		count++
	}

	return count, nil
}

// CreateTTLMangleRule creates a mangle rule to mark connections by TTL value
func (c *Client) CreateTTLMangleRule(ttl int, mark string, comment string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout * 2))

	// First check if rule already exists
	c.sendWord("/ip/firewall/mangle/print")
	c.sendWord(fmt.Sprintf("?ttl=equal:%d", ttl))
	c.sendWord("?chain=prerouting")
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to check existing rules: %v", err)
	}

	// Check if rule exists
	for _, word := range response {
		if word == "!re" {
			log.Printf("MikroTik: TTL=%d rule already exists, skipping", ttl)
			return nil
		}
	}

	// Create the mangle rule
	c.conn.SetDeadline(time.Now().Add(c.timeout * 2))
	c.sendWord("/ip/firewall/mangle/add")
	c.sendWord("=chain=prerouting")
	c.sendWord(fmt.Sprintf("=ttl=equal:%d", ttl))
	c.sendWord("=action=mark-connection")
	c.sendWord("=new-connection-mark=" + mark)
	c.sendWord("=passthrough=yes")
	c.sendWord("=comment=" + comment)
	c.sendWord("")

	response, err = c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to create mangle rule: %v", err)
	}

	for _, word := range response {
		if strings.HasPrefix(word, "!trap") {
			for _, w := range response {
				if strings.HasPrefix(w, "=message=") {
					return fmt.Errorf("failed to create rule: %s", strings.TrimPrefix(w, "=message="))
				}
			}
			return fmt.Errorf("failed to create mangle rule")
		}
	}

	log.Printf("MikroTik: Created TTL=%d mangle rule with mark=%s", ttl, mark)
	return nil
}

// RemoveTTLRules removes all mangle rules with matching comment
func (c *Client) RemoveTTLRules(comment string) (int, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return 0, err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout * 2))

	// Find all rules with matching comment
	c.sendWord("/ip/firewall/mangle/print")
	c.sendWord("?comment~" + comment)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return 0, fmt.Errorf("failed to query mangle rules: %v", err)
	}

	// Collect IDs to remove
	var ids []string
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			ids = append(ids, strings.TrimPrefix(word, "=.id="))
		}
	}

	// Remove each rule
	removedCount := 0
	for _, id := range ids {
		c.conn.SetDeadline(time.Now().Add(c.timeout))
		c.sendWord("/ip/firewall/mangle/remove")
		c.sendWord("=.id=" + id)
		c.sendWord("")

		_, err := c.readResponse()
		if err == nil {
			removedCount++
		}
	}

	log.Printf("MikroTik: Removed %d TTL detection rules", removedCount)
	return removedCount, nil
}

// UpdateCDNQueueSpeed updates the speed limit of a CDN queue by name
func (c *Client) UpdateCDNQueueSpeed(queueName string, speedKbps int) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	// Find queue by name
	c.conn.SetDeadline(time.Now().Add(c.timeout * 2))
	c.sendWord("/queue/simple/print")
	c.sendWord("?name=" + queueName)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to query queue: %v", err)
	}

	// Get queue ID
	var queueID string
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			queueID = strings.TrimPrefix(word, "=.id=")
			break
		}
	}

	if queueID == "" {
		return fmt.Errorf("queue %s not found", queueName)
	}

	// Update queue speed
	speedLimit := fmt.Sprintf("%dk/%dk", speedKbps, speedKbps)
	c.conn.SetDeadline(time.Now().Add(c.timeout * 2))
	c.sendWord("/queue/simple/set")
	c.sendWord("=.id=" + queueID)
	c.sendWord("=max-limit=" + speedLimit)
	c.sendWord("")

	resp, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to update queue: %v", err)
	}

	for _, word := range resp {
		if strings.HasPrefix(word, "!trap") {
			return fmt.Errorf("error updating queue: %v", resp)
		}
	}

	log.Printf("MikroTik: Updated CDN queue %s to %s", queueName, speedLimit)
	return nil
}

// IPPool represents a MikroTik IP pool
type IPPool struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Ranges string `json:"ranges"`
}

// GetIPPools fetches all IP pools from MikroTik
func (c *Client) GetIPPools() ([]IPPool, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	c.sendWord("/ip/pool/print")
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to get IP pools: %v", err)
	}

	var pools []IPPool
	var current IPPool

	for _, word := range response {
		if word == "!re" {
			if current.Name != "" {
				pools = append(pools, current)
			}
			current = IPPool{}
		}
		if strings.HasPrefix(word, "=.id=") {
			current.ID = strings.TrimPrefix(word, "=.id=")
		}
		if strings.HasPrefix(word, "=name=") {
			current.Name = strings.TrimPrefix(word, "=name=")
		}
		if strings.HasPrefix(word, "=ranges=") {
			current.Ranges = strings.TrimPrefix(word, "=ranges=")
		}
	}
	// Don't forget last pool
	if current.Name != "" {
		pools = append(pools, current)
	}

	return pools, nil
}

// PCQConfig holds configuration for PCQ-based CDN setup
type PCQConfig struct {
	CDNName       string   // CDN name (e.g., "GGC")
	SpeedLimitM   int64    // Speed limit in Mbps (pcq-rate)
	PCQLimit      int      // PCQ limit per connection in KiB
	PCQTotalLimit int      // PCQ total limit in KiB
	TargetPools   string   // Comma-separated target pools/CIDRs
	CompanyName   string   // Company name for branding
	Subnets       []string // CDN subnets for address list
	ServiceName   string   // Service name (e.g., "SHAHID-8") for queue comment
}

// CreatePCQQueueType creates a PCQ queue type for a CDN with specific speed
func (c *Client) CreatePCQQueueType(config PCQConfig) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	// Queue type name includes speed and service (e.g., "SHAHID-8 SHAHID-8")
	queueTypeName := fmt.Sprintf("%s-%d", config.CDNName, config.SpeedLimitM)
	if config.ServiceName != "" {
		queueTypeName = fmt.Sprintf("%s-%d %s", config.CDNName, config.SpeedLimitM, config.ServiceName)
	}
	pcqRate := fmt.Sprintf("%dM", config.SpeedLimitM)

	// Check if queue type already exists
	c.sendWord("/queue/type/print")
	c.sendWord("?name=" + queueTypeName)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to check queue type: %v", err)
	}

	var existingID string
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			existingID = strings.TrimPrefix(word, "=.id=")
			break
		}
	}

	if existingID != "" {
		// Update existing queue type
		c.sendWord("/queue/type/set")
		c.sendWord("=.id=" + existingID)
		c.sendWord("=pcq-rate=" + pcqRate)
		c.sendWord(fmt.Sprintf("=pcq-limit=%dKiB", config.PCQLimit))
		c.sendWord(fmt.Sprintf("=pcq-total-limit=%dKiB", config.PCQTotalLimit))
		c.sendWord("")
	} else {
		// Create new queue type
		c.sendWord("/queue/type/add")
		c.sendWord("=name=" + queueTypeName)
		c.sendWord("=kind=pcq")
		c.sendWord("=pcq-rate=" + pcqRate)
		c.sendWord("=pcq-classifier=dst-address")
		c.sendWord(fmt.Sprintf("=pcq-limit=%dKiB", config.PCQLimit))
		c.sendWord(fmt.Sprintf("=pcq-total-limit=%dKiB", config.PCQTotalLimit))
		c.sendWord("=pcq-burst-rate=0")
		c.sendWord("=pcq-burst-threshold=0")
		c.sendWord("=pcq-burst-time=10s")
		c.sendWord("=pcq-src-address-mask=32")
		c.sendWord("=pcq-dst-address-mask=32")
		c.sendWord("")
	}

	response, err = c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to create/update queue type: %v", err)
	}

	for _, word := range response {
		if strings.HasPrefix(word, "!trap") {
			return fmt.Errorf("error creating queue type: %v", response)
		}
	}

	log.Printf("MikroTik: Created/updated PCQ queue type %s with rate %s", queueTypeName, pcqRate)
	return nil
}

// CreateCDNMangleRule creates a mangle rule to mark packets from a CDN
func (c *Client) CreateCDNMangleRule(config PCQConfig) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	// Use same address list name as SyncCDNAddressList: "CDN-{name}"
	addressListName := fmt.Sprintf("CDN-%s", config.CDNName)
	packetMark := fmt.Sprintf("CDN-%s", config.CDNName)
	comment := fmt.Sprintf("%s CDN %s packet mark", config.CompanyName, config.CDNName)

	// Check if mangle rule already exists
	c.sendWord("/ip/firewall/mangle/print")
	c.sendWord("?comment=" + comment)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to check mangle rule: %v", err)
	}

	var existingID string
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			existingID = strings.TrimPrefix(word, "=.id=")
			break
		}
	}

	if existingID != "" {
		// Delete existing rule - MikroTik doesn't allow changing chain with /set
		c.sendWord("/ip/firewall/mangle/remove")
		c.sendWord("=.id=" + existingID)
		c.sendWord("")
		c.readResponse()
		log.Printf("MikroTik: Removed old mangle rule for CDN %s to recreate in forward chain", config.CDNName)
	}

	// Create new mangle rule - MUST be in forward chain (before simple queues)
	// postrouting is too late - packets are already queued
	c.sendWord("/ip/firewall/mangle/add")
	c.sendWord("=chain=forward")
	c.sendWord("=action=mark-packet")
	c.sendWord("=new-packet-mark=" + packetMark)
	c.sendWord("=passthrough=no")
	c.sendWord("=src-address-list=" + addressListName)
	c.sendWord("=comment=" + comment)
	c.sendWord("")

	response, err = c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to create/update mangle rule: %v", err)
	}

	for _, word := range response {
		if strings.HasPrefix(word, "!trap") {
			return fmt.Errorf("error creating mangle rule: %v", response)
		}
	}

	log.Printf("MikroTik: Created/updated mangle rule for CDN %s", config.CDNName)
	return nil
}

// ipRangeToCIDR converts IP range (e.g., "14.12.12.10-14.12.12.254") to CIDR (e.g., "14.12.12.0/24")
func ipRangeToCIDR(ipRange string) string {
	// If already in CIDR format, return as is
	if strings.Contains(ipRange, "/") {
		return ipRange
	}

	// Split by dash for range format
	parts := strings.Split(ipRange, "-")
	if len(parts) != 2 {
		// Not a range, return as is (might be single IP)
		return ipRange
	}

	startIP := strings.TrimSpace(parts[0])
	endIP := strings.TrimSpace(parts[1])

	// Parse start IP
	startParts := strings.Split(startIP, ".")
	endParts := strings.Split(endIP, ".")
	if len(startParts) != 4 || len(endParts) != 4 {
		return ipRange
	}

	// Simple heuristic: find where they differ and calculate subnet
	// For typical pools like x.x.x.10-x.x.x.254, we assume /24
	// For x.x.0.1-x.x.255.254, we assume /16

	// Check if first 3 octets are same (typical /24)
	if startParts[0] == endParts[0] && startParts[1] == endParts[1] && startParts[2] == endParts[2] {
		return fmt.Sprintf("%s.%s.%s.0/24", startParts[0], startParts[1], startParts[2])
	}

	// Check if first 2 octets are same (could be /16 to /23)
	if startParts[0] == endParts[0] && startParts[1] == endParts[1] {
		// Approximate based on third octet range
		startOct3, _ := strconv.Atoi(startParts[2])
		endOct3, _ := strconv.Atoi(endParts[2])
		diff := endOct3 - startOct3 + 1

		if diff <= 2 {
			return fmt.Sprintf("%s.%s.%s.0/23", startParts[0], startParts[1], startParts[2])
		} else if diff <= 4 {
			return fmt.Sprintf("%s.%s.%d.0/22", startParts[0], startParts[1], startOct3&0xFC)
		} else if diff <= 8 {
			return fmt.Sprintf("%s.%s.%d.0/21", startParts[0], startParts[1], startOct3&0xF8)
		} else {
			return fmt.Sprintf("%s.%s.0.0/16", startParts[0], startParts[1])
		}
	}

	// Fallback: return as /16 based on first two octets
	return fmt.Sprintf("%s.%s.0.0/16", startParts[0], startParts[1])
}

// convertTargetPoolsToCIDR converts comma-separated IP ranges to CIDR format
func convertTargetPoolsToCIDR(pools string) string {
	var cidrs []string
	for _, pool := range strings.Split(pools, ",") {
		pool = strings.TrimSpace(pool)
		if pool != "" {
			cidrs = append(cidrs, ipRangeToCIDR(pool))
		}
	}
	return strings.Join(cidrs, ",")
}

// CreatePCQSimpleQueue creates a simple queue using PCQ for a CDN
func (c *Client) CreatePCQSimpleQueue(config PCQConfig) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	// Queue name format: "SHAHID-8M SHAHID-8" (CDN-Speed ServiceName)
	queueName := fmt.Sprintf("%s-%dM", config.CDNName, config.SpeedLimitM)
	// Queue type must match CreatePCQQueueType name
	queueTypeName := fmt.Sprintf("%s-%d", config.CDNName, config.SpeedLimitM)
	if config.ServiceName != "" {
		queueName = fmt.Sprintf("%s-%dM %s", config.CDNName, config.SpeedLimitM, config.ServiceName)
		queueTypeName = fmt.Sprintf("%s-%d %s", config.CDNName, config.SpeedLimitM, config.ServiceName)
	}
	// Packet mark matches mangle rule: "CDN-GGC"
	packetMark := fmt.Sprintf("CDN-%s", config.CDNName)
	// Convert IP ranges to CIDR format for MikroTik
	targetCIDR := convertTargetPoolsToCIDR(config.TargetPools)

	// Comment for identification - includes service name and speed
	queueComment := fmt.Sprintf("PCQ queue for CDN %s %dM %s", config.CDNName, config.SpeedLimitM, config.ServiceName)

	// Check if simple queue already exists (by name)
	c.sendWord("/queue/simple/print")
	c.sendWord("?name=" + queueName)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to check simple queue: %v", err)
	}

	var existingID string
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			existingID = strings.TrimPrefix(word, "=.id=")
			break
		}
	}

	// Queue type format: upload/download
	queueType := fmt.Sprintf("%s/%s", queueTypeName, queueTypeName)

	if existingID != "" {
		// Update existing simple queue (including name if speed changed)
		c.sendWord("/queue/simple/set")
		c.sendWord("=.id=" + existingID)
		c.sendWord("=name=" + queueName)
		c.sendWord("=target=" + targetCIDR)
		c.sendWord("=packet-marks=" + packetMark)
		c.sendWord("=queue=" + queueType)
		c.sendWord("=max-limit=1G/1G")
		c.sendWord("=comment=" + queueComment)
		c.sendWord("")
	} else {
		// Create new simple queue
		c.sendWord("/queue/simple/add")
		c.sendWord("=name=" + queueName)
		c.sendWord("=target=" + targetCIDR)
		c.sendWord("=packet-marks=" + packetMark)
		c.sendWord("=queue=" + queueType)
		c.sendWord("=max-limit=1G/1G")
		c.sendWord("=priority=8/8")
		c.sendWord("=comment=" + queueComment)
		c.sendWord("")
	}

	response, err = c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to create/update simple queue: %v", err)
	}

	for _, word := range response {
		if strings.HasPrefix(word, "!trap") {
			return fmt.Errorf("error creating simple queue: %v", response)
		}
	}

	// Move queue to top (position 0) so it's processed before user queues
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/queue/simple/print")
	c.sendWord("?comment=" + queueComment)
	c.sendWord("")

	response, err = c.readResponse()
	if err == nil {
		for _, word := range response {
			if strings.HasPrefix(word, "=.id=") {
				queueID := strings.TrimPrefix(word, "=.id=")
				// Move to top (destination 0)
				c.sendWord("/queue/simple/move")
				c.sendWord("=numbers=" + queueID)
				c.sendWord("=destination=0")
				c.sendWord("")
				c.readResponse()
				log.Printf("MikroTik: Moved PCQ queue %s to top", queueName)
				break
			}
		}
	}

	log.Printf("MikroTik: Created/updated PCQ simple queue %s for CDN %s (target=%s)", queueName, config.CDNName, targetCIDR)
	return nil
}

// SyncCDNPCQSetup creates the complete PCQ setup for a CDN (address list + queue type + mangle + simple queue)
func (c *Client) SyncCDNPCQSetup(config PCQConfig) error {
	// Step 0: Sync CDN address list (subnets) - required for mangle rule
	if len(config.Subnets) > 0 {
		cdnConfig := CDNConfig{
			Name:        config.CDNName,
			Subnets:     config.Subnets,
			CompanyName: config.CompanyName,
		}
		if err := c.SyncCDNAddressList(cdnConfig); err != nil {
			log.Printf("MikroTik: Warning - failed to sync address list for CDN %s: %v", config.CDNName, err)
			// Continue anyway, mangle rule will just not match any traffic
		}
	} else {
		log.Printf("MikroTik: Warning - no subnets configured for CDN %s, address list not synced", config.CDNName)
	}

	// Step 1: Create PCQ queue type
	if err := c.CreatePCQQueueType(config); err != nil {
		return fmt.Errorf("failed to create queue type: %v", err)
	}

	// Step 2: Create mangle rule (uses address list CDN-{name})
	if err := c.CreateCDNMangleRule(config); err != nil {
		return fmt.Errorf("failed to create mangle rule: %v", err)
	}

	// Step 3: Create simple queue with PCQ
	if err := c.CreatePCQSimpleQueue(config); err != nil {
		return fmt.Errorf("failed to create simple queue: %v", err)
	}

	log.Printf("MikroTik: Complete PCQ setup synced for CDN %s", config.CDNName)
	return nil
}

// RemoveCDNPCQSetup removes all PCQ components for a CDN with specific speed
// This removes: queue type, simple queue. Mangle rules and address lists are shared per CDN
// and should only be removed when no services use that CDN anymore.
func (c *Client) RemoveCDNPCQSetup(cdnName string, speedLimitM int64, companyName string, serviceName ...string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	// Queue type and simple queue names include service name if provided
	svcName := ""
	if len(serviceName) > 0 {
		svcName = serviceName[0]
	}
	queueTypeName := fmt.Sprintf("%s-%d", cdnName, speedLimitM)
	queueName := fmt.Sprintf("%s-%dM", cdnName, speedLimitM)
	if svcName != "" {
		queueTypeName = fmt.Sprintf("%s-%d %s", cdnName, speedLimitM, svcName)
		queueName = fmt.Sprintf("%s-%dM %s", cdnName, speedLimitM, svcName)
	}

	// Remove simple queue first (search by name)
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/queue/simple/print")
	c.sendWord("?name=" + queueName)
	c.sendWord("")

	response, err := c.readResponse()
	if err == nil {
		for _, word := range response {
			if strings.HasPrefix(word, "=.id=") {
				queueID := strings.TrimPrefix(word, "=.id=")
				c.sendWord("/queue/simple/remove")
				c.sendWord("=.id=" + queueID)
				c.sendWord("")
				c.readResponse()
				log.Printf("MikroTik: Removed simple queue for CDN %s %dM", cdnName, speedLimitM)
				break
			}
		}
	}

	// Remove queue type
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/queue/type/print")
	c.sendWord("?name=" + queueTypeName)
	c.sendWord("")

	response, err = c.readResponse()
	if err == nil {
		for _, word := range response {
			if strings.HasPrefix(word, "=.id=") {
				typeID := strings.TrimPrefix(word, "=.id=")
				c.sendWord("/queue/type/remove")
				c.sendWord("=.id=" + typeID)
				c.sendWord("")
				c.readResponse()
				log.Printf("MikroTik: Removed queue type %s", queueTypeName)
				break
			}
		}
	}

	log.Printf("MikroTik: Removed PCQ setup for CDN %s %dM", cdnName, speedLimitM)
	return nil
}

// RemoveCDNMangleAndAddressList removes mangle rules and address list for a CDN
// Only call this when NO services are using this CDN anymore
func (c *Client) RemoveCDNMangleAndAddressList(cdnName, companyName string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	// Comment used for mangle rule
	mangleComment := fmt.Sprintf("%s CDN %s packet mark", companyName, cdnName)
	// Address list name
	addressListName := fmt.Sprintf("CDN-%s", cdnName)

	// Remove mangle rule
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/ip/firewall/mangle/print")
	c.sendWord("?comment=" + mangleComment)
	c.sendWord("")

	response, err := c.readResponse()
	if err == nil {
		for _, word := range response {
			if strings.HasPrefix(word, "=.id=") {
				mangleID := strings.TrimPrefix(word, "=.id=")
				c.sendWord("/ip/firewall/mangle/remove")
				c.sendWord("=.id=" + mangleID)
				c.sendWord("")
				c.readResponse()
				log.Printf("MikroTik: Removed mangle rule for CDN %s", cdnName)
				break
			}
		}
	}

	// Remove address list entries
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/ip/firewall/address-list/print")
	c.sendWord("?list=" + addressListName)
	c.sendWord("")

	response, err = c.readResponse()
	if err == nil {
		// Collect all IDs first
		var ids []string
		for _, word := range response {
			if strings.HasPrefix(word, "=.id=") {
				ids = append(ids, strings.TrimPrefix(word, "=.id="))
			}
		}
		// Remove all entries
		for _, id := range ids {
			c.sendWord("/ip/firewall/address-list/remove")
			c.sendWord("=.id=" + id)
			c.sendWord("")
			c.readResponse()
		}
		if len(ids) > 0 {
			log.Printf("MikroTik: Removed %d address list entries for CDN %s", len(ids), cdnName)
		}
	}

	return nil
}

// SubscriberCDNOverrideConfig holds configuration for a subscriber's CDN override queue
type SubscriberCDNOverrideConfig struct {
	SubscriberIP string // Subscriber's current IP address
	Username     string // Subscriber username
	CDNName      string // CDN name (e.g., "GGC")
	SpeedLimitM  int64  // Speed limit in Mbps (to use the correct PCQ queue type)
	CompanyName  string // Company name for queue comment
}

// SyncSubscriberCDNOverrideQueue creates or updates a per-subscriber CDN override queue
// This allows a subscriber to use a different CDN speed than their service default
// The queue intercepts CDN traffic for this specific subscriber and applies the override speed
func (c *Client) SyncSubscriberCDNOverrideQueue(config SubscriberCDNOverrideConfig) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	// Queue name format: "CDN-Override-{username}"
	queueName := fmt.Sprintf("CDN-Override-%s", config.Username)
	// Queue type includes speed: "GGC-30" (must match CreatePCQQueueType)
	queueTypeName := fmt.Sprintf("%s-%d", config.CDNName, config.SpeedLimitM)
	// Packet mark matches mangle rule: "CDN-GGC"
	packetMark := fmt.Sprintf("CDN-%s", config.CDNName)
	// Target is subscriber's current IP
	target := config.SubscriberIP

	// Comment for identification
	queueComment := fmt.Sprintf("%s CDN Override for %s", config.CompanyName, config.Username)

	// Check if override queue already exists (by comment)
	c.sendWord("/queue/simple/print")
	c.sendWord("?comment=" + queueComment)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to check subscriber CDN override queue: %v", err)
	}

	var existingID string
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			existingID = strings.TrimPrefix(word, "=.id=")
			break
		}
	}

	// Queue type format: upload/download
	queueType := fmt.Sprintf("%s/%s", queueTypeName, queueTypeName)

	if existingID != "" {
		// Update existing override queue
		c.conn.SetDeadline(time.Now().Add(c.timeout))
		c.sendWord("/queue/simple/set")
		c.sendWord("=.id=" + existingID)
		c.sendWord("=name=" + queueName)
		c.sendWord("=target=" + target)
		c.sendWord("=packet-marks=" + packetMark)
		c.sendWord("=queue=" + queueType)
		c.sendWord("=max-limit=1G/1G")
		c.sendWord("=priority=1/1") // Higher priority than PCQ queues (which use 8/8)
		c.sendWord("")
	} else {
		// Create new override queue
		c.sendWord("/queue/simple/add")
		c.sendWord("=name=" + queueName)
		c.sendWord("=target=" + target)
		c.sendWord("=packet-marks=" + packetMark)
		c.sendWord("=queue=" + queueType)
		c.sendWord("=max-limit=1G/1G")
		c.sendWord("=priority=1/1") // Higher priority than PCQ queues
		c.sendWord("=comment=" + queueComment)
		c.sendWord("")
	}

	response, err = c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to create/update subscriber CDN override queue: %v", err)
	}

	for _, word := range response {
		if strings.HasPrefix(word, "!trap") {
			return fmt.Errorf("error creating subscriber CDN override queue: %v", response)
		}
	}

	// Move queue to top (position 0) so it's processed BEFORE PPPoE user queues
	// This is critical - MikroTik processes queues in order, and we need to match before the PPPoE queue
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/queue/simple/print")
	c.sendWord("?comment=" + queueComment)
	c.sendWord("")

	response, err = c.readResponse()
	if err == nil {
		for _, word := range response {
			if strings.HasPrefix(word, "=.id=") {
				queueID := strings.TrimPrefix(word, "=.id=")
				// Move to top (destination 0)
				c.conn.SetDeadline(time.Now().Add(c.timeout))
				c.sendWord("/queue/simple/move")
				c.sendWord("=numbers=" + queueID)
				c.sendWord("=destination=0")
				c.sendWord("")
				c.readResponse()
				log.Printf("MikroTik: Moved CDN override queue %s to top", queueName)
				break
			}
		}
	}

	log.Printf("MikroTik: Created/updated CDN override queue for %s -> %s at %dM", config.Username, config.CDNName, config.SpeedLimitM)
	return nil
}

// RemoveSubscriberCDNOverrideQueue removes the per-subscriber CDN override queue
func (c *Client) RemoveSubscriberCDNOverrideQueue(username string, companyName string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	// Comment for identification
	queueComment := fmt.Sprintf("%s CDN Override for %s", companyName, username)

	// Find and remove the queue
	c.sendWord("/queue/simple/print")
	c.sendWord("?comment=" + queueComment)
	c.sendWord("")

	response, _ := c.readResponse()
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			queueID := strings.TrimPrefix(word, "=.id=")
			c.conn.SetDeadline(time.Now().Add(c.timeout))
			c.sendWord("/queue/simple/remove")
			c.sendWord("=.id=" + queueID)
			c.sendWord("")
			c.readResponse()
			log.Printf("MikroTik: Removed CDN override queue for %s", username)
			return nil
		}
	}

	return nil // Queue didn't exist, nothing to remove
}

// AddStaticIPToAddressList adds a static IP to the STATIC-IPS address list on MikroTik
// This is used to reserve IPs so they won't be assigned from the pool
func (c *Client) AddStaticIPToAddressList(ip string, subscriberUsername string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	listName := "STATIC-IPS"
	comment := fmt.Sprintf("Static IP for %s", subscriberUsername)

	// Check if IP already exists in the list
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/ip/firewall/address-list/print")
	c.sendWord("?list=" + listName)
	c.sendWord("?address=" + ip)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to query address-list: %v", err)
	}

	// Check if entry already exists
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			// Already exists, update comment
			id := strings.TrimPrefix(word, "=.id=")
			c.conn.SetDeadline(time.Now().Add(c.timeout))
			c.sendWord("/ip/firewall/address-list/set")
			c.sendWord("=.id=" + id)
			c.sendWord("=comment=" + comment)
			c.sendWord("")
			c.readResponse()
			log.Printf("MikroTik: Updated static IP %s in address-list for %s", ip, subscriberUsername)
			return nil
		}
	}

	// Add new entry
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/ip/firewall/address-list/add")
	c.sendWord("=list=" + listName)
	c.sendWord("=address=" + ip)
	c.sendWord("=comment=" + comment)
	c.sendWord("")

	_, err = c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to add static IP to address-list: %v", err)
	}

	log.Printf("MikroTik: Added static IP %s to address-list STATIC-IPS for %s", ip, subscriberUsername)

	// Ensure the scheduler script exists to protect static IPs from pool assignment
	c.ensureStaticIPProtectionScript()

	return nil
}

// ensureStaticIPProtectionScript creates a scheduler script on MikroTik that protects static IPs
// from being assigned by the pool. This script runs every 30 seconds and removes static IPs
// from the pool's "used" list, making them unavailable for dynamic assignment.
func (c *Client) ensureStaticIPProtectionScript() {
	schedulerName := "protect-static-ips"

	// Check if scheduler already exists
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/system/scheduler/print")
	c.sendWord("?name=" + schedulerName)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		log.Printf("MikroTik: Failed to check scheduler: %v", err)
		return
	}

	// Check if scheduler exists
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			// Scheduler already exists
			log.Printf("MikroTik: Static IP protection scheduler already exists")
			return
		}
	}

	// Create the scheduler script
	// This script removes static IPs from the pool's used list every 30 seconds
	script := `:foreach entry in=[/ip firewall address-list find list=STATIC-IPS] do={:local addr [/ip firewall address-list get $entry address]; :foreach used in=[/ip pool used find address=$addr] do={/ip pool used remove $used}}`

	// Get company name for branding in comment
	companyName := database.GetCompanyName()
	if companyName == "" {
		companyName = "ISP"
	}

	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/system/scheduler/add")
	c.sendWord("=name=" + schedulerName)
	c.sendWord("=interval=30s")
	c.sendWord("=on-event=" + script)
	c.sendWord("=comment=" + companyName + ": Protects static IPs from pool assignment")
	c.sendWord("")

	_, err = c.readResponse()
	if err != nil {
		log.Printf("MikroTik: Failed to create static IP protection scheduler: %v", err)
		return
	}

	log.Printf("MikroTik: Created static IP protection scheduler")
}

// ReserveStaticIPInPPP creates a disabled PPP secret to reserve an IP from the pool
// This prevents MikroTik from assigning this IP to dynamic users
func (c *Client) ReserveStaticIPInPPP(ip string, subscriberUsername string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	// Use a unique name for the reservation
	secretName := fmt.Sprintf("static-%s", strings.ReplaceAll(ip, ".", "-"))
	comment := fmt.Sprintf("Reserved for %s (RADIUS static IP)", subscriberUsername)

	// Check if reservation already exists
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/ppp/secret/print")
	c.sendWord("?remote-address=" + ip)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to query PPP secrets: %v", err)
	}

	// If exists, update it
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			id := strings.TrimPrefix(word, "=.id=")
			c.conn.SetDeadline(time.Now().Add(c.timeout))
			c.sendWord("/ppp/secret/set")
			c.sendWord("=.id=" + id)
			c.sendWord("=comment=" + comment)
			c.sendWord("=disabled=yes")
			c.sendWord("")
			c.readResponse()
			log.Printf("MikroTik: Updated PPP reservation for static IP %s (%s)", ip, subscriberUsername)
			return nil
		}
	}

	// Create new disabled PPP secret to reserve the IP
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/ppp/secret/add")
	c.sendWord("=name=" + secretName)
	c.sendWord("=password=reserved")
	c.sendWord("=service=pppoe")
	c.sendWord("=remote-address=" + ip)
	c.sendWord("=disabled=yes")
	c.sendWord("=comment=" + comment)
	c.sendWord("")

	_, err = c.readResponse()
	if err != nil {
		// Might fail if name exists, try with different name
		log.Printf("MikroTik: Note - PPP reservation may already exist: %v", err)
	} else {
		log.Printf("MikroTik: Reserved static IP %s in PPP for %s", ip, subscriberUsername)
	}

	return nil
}

// RemoveStaticIPReservation removes a PPP secret reservation for a static IP
func (c *Client) RemoveStaticIPReservation(ip string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	// Find and remove PPP secret with this remote-address
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/ppp/secret/print")
	c.sendWord("?remote-address=" + ip)
	c.sendWord("?disabled=yes")
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to query PPP secrets: %v", err)
	}

	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			id := strings.TrimPrefix(word, "=.id=")
			c.conn.SetDeadline(time.Now().Add(c.timeout))
			c.sendWord("/ppp/secret/remove")
			c.sendWord("=.id=" + id)
			c.sendWord("")
			c.readResponse()
			log.Printf("MikroTik: Removed PPP reservation for static IP %s", ip)
			return nil
		}
	}

	log.Printf("MikroTik: No PPP reservation found for static IP %s", ip)
	return nil
}

// RemoveStaticIPFromAddressList removes a static IP from the STATIC-IPS address list on MikroTik
func (c *Client) RemoveStaticIPFromAddressList(ip string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	listName := "STATIC-IPS"

	// Find the entry
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/ip/firewall/address-list/print")
	c.sendWord("?list=" + listName)
	c.sendWord("?address=" + ip)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to query address-list: %v", err)
	}

	// Find and remove the entry
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			id := strings.TrimPrefix(word, "=.id=")
			c.conn.SetDeadline(time.Now().Add(c.timeout))
			c.sendWord("/ip/firewall/address-list/remove")
			c.sendWord("=.id=" + id)
			c.sendWord("")
			c.readResponse()
			log.Printf("MikroTik: Removed static IP %s from address-list STATIC-IPS", ip)
			return nil
		}
	}

	// Not found, that's okay
	log.Printf("MikroTik: Static IP %s not found in address-list STATIC-IPS (already removed)", ip)
	return nil
}

// TorchEntry represents a single torch traffic entry (like MikroTik Winbox torch)
type TorchEntry struct {
	SrcAddress string `json:"src_address"`
	DstAddress string `json:"dst_address"`
	SrcPort    int    `json:"src_port"`
	DstPort    int    `json:"dst_port"`
	Protocol   string `json:"protocol"`     // tcp, udp, icmp, etc.
	ProtoNum   int    `json:"proto_num"`    // 6=tcp, 17=udp, 1=icmp
	MacProto   string `json:"mac_protocol"` // 800=IPv4, 806=ARP, 86dd=IPv6
	VlanID     int    `json:"vlan_id"`
	DSCP       int    `json:"dscp"`
	TxRate     int64  `json:"tx_rate"`      // bytes per second
	RxRate     int64  `json:"rx_rate"`      // bytes per second
	TxPackets  int64  `json:"tx_packets"`
	RxPackets  int64  `json:"rx_packets"`
}

// TorchResult contains the result of a torch operation
type TorchResult struct {
	Entries   []TorchEntry `json:"entries"`
	TotalTx   int64        `json:"total_tx"`
	TotalRx   int64        `json:"total_rx"`
	Duration  string       `json:"duration"`
	Interface string       `json:"interface"`
	FilterIP  string       `json:"filter_ip"`
}

// GetLiveTorch runs torch on a PPPoE interface for a specific subscriber IP
// Returns real-time traffic breakdown by connection (like MikroTik Winbox torch)
func (c *Client) GetLiveTorch(subscriberIP string, durationSec int) (*TorchResult, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	if durationSec <= 0 {
		durationSec = 3
	}
	if durationSec > 10 {
		durationSec = 10 // Max 10 seconds to avoid timeout
	}

	// Set longer timeout for torch operation
	c.conn.SetDeadline(time.Now().Add(time.Duration(durationSec+10) * time.Second))

	// Find the PPPoE interface for this subscriber
	c.sendWord("/ppp/active/print")
	c.sendWord("?address=" + subscriberIP)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to find PPPoE session: %v", err)
	}

	// Find the interface name from PPP active session
	var ifaceName string
	var username string
	for _, word := range response {
		if strings.HasPrefix(word, "=name=") {
			username = strings.TrimPrefix(word, "=name=")
			// MikroTik PPPoE dynamic interfaces are named <pppoe-username>
			ifaceName = "<pppoe-" + username + ">"
			break
		}
	}

	if ifaceName == "" {
		return nil, fmt.Errorf("subscriber not connected or interface not found")
	}

	log.Printf("Torch: Found PPPoE session for IP %s, username=%s, interface=%s", subscriberIP, username, ifaceName)

	result := &TorchResult{
		Entries:   make([]TorchEntry, 0),
		Interface: ifaceName,
		FilterIP:  subscriberIP,
		Duration:  fmt.Sprintf("%ds", durationSec),
	}

	// Run torch command with full details
	// Based on MikroTik API docs: need interface, src-address, dst-address, port filters
	c.conn.SetDeadline(time.Now().Add(time.Duration(durationSec+10) * time.Second))
	c.sendWord("/tool/torch")
	c.sendWord("=interface=" + ifaceName)
	c.sendWord("=src-address=0.0.0.0/0")
	c.sendWord("=dst-address=0.0.0.0/0")
	c.sendWord("=port=any")
	c.sendWord("=ip-protocol=any")
	c.sendWord("=duration=" + strconv.Itoa(durationSec))
	c.sendWord("")

	// Read torch results
	entries := make(map[string]*TorchEntry)
	log.Printf("Torch: Starting to read responses for interface %s", ifaceName)
	responseCount := 0

	for {
		word, err := c.readWord()
		if err != nil {
			log.Printf("Torch: Read error: %v", err)
			break
		}

		if word == "!done" {
			log.Printf("Torch: Received !done after %d responses", responseCount)
			break
		}

		if word == "!re" {
			responseCount++
			entry := &TorchEntry{}
			key := ""

			// Read all attributes
			for {
				attr, err := c.readWord()
				if err != nil {
					break
				}
				if attr == "" {
					break
				}

				if strings.HasPrefix(attr, "=src-address=") {
					entry.SrcAddress = strings.TrimPrefix(attr, "=src-address=")
				} else if strings.HasPrefix(attr, "=dst-address=") {
					entry.DstAddress = strings.TrimPrefix(attr, "=dst-address=")
				} else if strings.HasPrefix(attr, "=ip-protocol=") {
					entry.ProtoNum, _ = strconv.Atoi(strings.TrimPrefix(attr, "=ip-protocol="))
					switch entry.ProtoNum {
					case 0:
						entry.Protocol = "" // HOPOPT or unspecified
					case 1:
						entry.Protocol = "icmp"
					case 6:
						entry.Protocol = "tcp"
					case 17:
						entry.Protocol = "udp"
					case 47:
						entry.Protocol = "gre"
					case 50:
						entry.Protocol = "esp"
					case 51:
						entry.Protocol = "ah"
					case 58:
						entry.Protocol = "icmpv6"
					default:
						entry.Protocol = strconv.Itoa(entry.ProtoNum)
					}
				} else if strings.HasPrefix(attr, "=src-port=") {
					entry.SrcPort, _ = strconv.Atoi(strings.TrimPrefix(attr, "=src-port="))
				} else if strings.HasPrefix(attr, "=dst-port=") {
					entry.DstPort, _ = strconv.Atoi(strings.TrimPrefix(attr, "=dst-port="))
				} else if strings.HasPrefix(attr, "=mac-protocol=") {
					entry.MacProto = strings.TrimPrefix(attr, "=mac-protocol=")
				} else if strings.HasPrefix(attr, "=vlan-id=") {
					entry.VlanID, _ = strconv.Atoi(strings.TrimPrefix(attr, "=vlan-id="))
				} else if strings.HasPrefix(attr, "=dscp=") {
					entry.DSCP, _ = strconv.Atoi(strings.TrimPrefix(attr, "=dscp="))
				} else if strings.HasPrefix(attr, "=tx=") {
					// MikroTik returns bits per second, convert to bytes for frontend
					bits, _ := strconv.ParseInt(strings.TrimPrefix(attr, "=tx="), 10, 64)
					entry.TxRate = bits / 8
				} else if strings.HasPrefix(attr, "=rx=") {
					// MikroTik returns bits per second, convert to bytes for frontend
					bits, _ := strconv.ParseInt(strings.TrimPrefix(attr, "=rx="), 10, 64)
					entry.RxRate = bits / 8
				} else if strings.HasPrefix(attr, "=tx-packets=") {
					entry.TxPackets, _ = strconv.ParseInt(strings.TrimPrefix(attr, "=tx-packets="), 10, 64)
				} else if strings.HasPrefix(attr, "=rx-packets=") {
					entry.RxPackets, _ = strconv.ParseInt(strings.TrimPrefix(attr, "=rx-packets="), 10, 64)
				}
			}

			// Skip aggregate/summary rows (those without valid addresses)
			// MikroTik torch returns summary rows with empty addresses
			if entry.SrcAddress == "" || entry.DstAddress == "" {
				continue
			}

			// If protocol wasn't detected but we have ports, infer the protocol
			// Most port-based traffic is TCP
			if entry.Protocol == "" && (entry.SrcPort > 0 || entry.DstPort > 0) {
				entry.Protocol = "tcp"
				entry.ProtoNum = 6
			}

			// Create unique key for this flow
			key = fmt.Sprintf("%s:%d-%s:%d-%s", entry.SrcAddress, entry.SrcPort, entry.DstAddress, entry.DstPort, entry.Protocol)

			// Aggregate or add new entry
			if existing, ok := entries[key]; ok {
				existing.TxRate = entry.TxRate
				existing.RxRate = entry.RxRate
				existing.TxPackets = entry.TxPackets
				existing.RxPackets = entry.RxPackets
			} else {
				entries[key] = entry
			}
		}

		if word == "!trap" {
			errMsg := ""
			for {
				attr, err := c.readWord()
				if err != nil || attr == "" {
					break
				}
				if strings.HasPrefix(attr, "=message=") {
					errMsg = strings.TrimPrefix(attr, "=message=")
				}
			}
			log.Printf("Torch: Error from router: %s", errMsg)
			return nil, fmt.Errorf("torch error: %s", errMsg)
		}
	}

	// Convert map to slice and calculate totals
	for _, entry := range entries {
		result.Entries = append(result.Entries, *entry)
		result.TotalTx += entry.TxRate
		result.TotalRx += entry.RxRate
	}

	log.Printf("Torch: Completed with %d unique flows, TotalTx=%d, TotalRx=%d", len(result.Entries), result.TotalTx, result.TotalRx)

	// Sort by TX rate descending (highest bandwidth first)
	for i := 0; i < len(result.Entries); i++ {
		for j := i + 1; j < len(result.Entries); j++ {
			if result.Entries[j].TxRate > result.Entries[i].TxRate {
				result.Entries[i], result.Entries[j] = result.Entries[j], result.Entries[i]
			}
		}
	}

	return result, nil
}

// PingEvent represents a single ping packet for live streaming
type PingEvent struct {
	Type string  `json:"type"` // "reply" or "timeout"
	Seq  int     `json:"seq"`
	Host string  `json:"host,omitempty"`
	Time float64 `json:"time,omitempty"` // milliseconds
	TTL  int     `json:"ttl,omitempty"`
	Size int     `json:"size,omitempty"`
}

// PingResult contains the result of a ping operation
type PingResult struct {
	Host       string    `json:"host"`
	Sent       int       `json:"sent"`
	Received   int       `json:"received"`
	PacketLoss int       `json:"packet_loss"`
	MinRTT     float64   `json:"min_rtt"`
	AvgRTT     float64   `json:"avg_rtt"`
	MaxRTT     float64   `json:"max_rtt"`
	RTTs       []float64 `json:"rtts"`
	Status     string    `json:"status"`
}

// Ping executes ping command on MikroTik to reach subscriber IP
func (c *Client) Ping(ip string, count int, size int) (*PingResult, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	if count <= 0 {
		count = 50
	}
	if count > 100 {
		count = 100
	}

	result := &PingResult{
		Host:   ip,
		Sent:   count,
		Status: "unknown",
	}

	// Set timeout for ping operation
	c.conn.SetDeadline(time.Now().Add(time.Duration(count*2+5) * time.Second))

	// Run ping command via MikroTik API with fast interval
	c.sendWord("/ping")
	c.sendWord("=address=" + ip)
	c.sendWord("=count=" + strconv.Itoa(count))
	c.sendWord("=interval=200ms")
	if size > 0 {
		c.sendWord("=size=" + strconv.Itoa(size))
	}
	c.sendWord("")

	// Read ping responses
	received := 0
	var rtts []float64

	for {
		word, err := c.readWord()
		if err != nil {
			break
		}

		if word == "!done" {
			for {
				attr, err := c.readWord()
				if err != nil || attr == "" {
					break
				}
			}
			break
		}

		if word == "!re" {
			var packetTime float64
			hasTime := false
			isTimeout := false

			for {
				attr, err := c.readWord()
				if err != nil || attr == "" {
					break
				}
				if strings.HasPrefix(attr, "=time=") {
					hasTime = true
					packetTime = parseMikrotikTime(strings.TrimPrefix(attr, "=time="))
				}
				if strings.HasPrefix(attr, "=timeout=") || attr == "=status=timeout" {
					isTimeout = true
				}
			}

			if hasTime && !isTimeout {
				rtts = append(rtts, packetTime)
				received++
			}
		}

		if word == "!trap" {
			for {
				attr, err := c.readWord()
				if err != nil || attr == "" {
					break
				}
			}
			result.Status = "error"
			return result, fmt.Errorf("ping failed")
		}
	}

	result.Received = received
	// Filter out timeout markers (-1) for the RTTs array
	var validRTTs []float64
	for _, r := range rtts {
		if r >= 0 {
			validRTTs = append(validRTTs, r)
		}
	}
	result.RTTs = validRTTs
	if count > 0 {
		result.PacketLoss = ((count - received) * 100) / count
	}

	if len(validRTTs) > 0 {
		var sum float64
		result.MinRTT = validRTTs[0]
		result.MaxRTT = validRTTs[0]
		for _, rtt := range validRTTs {
			sum += rtt
			if rtt < result.MinRTT {
				result.MinRTT = rtt
			}
			if rtt > result.MaxRTT {
				result.MaxRTT = rtt
			}
		}
		result.AvgRTT = sum / float64(len(validRTTs))
		result.Status = "success"
	} else {
		result.Status = "timeout"
	}

	return result, nil
}

// PortCheckResult contains the result of a port check operation
type PortCheckResult struct {
	Open         bool    `json:"open"`
	Status       string  `json:"status"`        // "open", "closed", "filtered"
	ResponseTime float64 `json:"response_time"`  // ms
	Error        string  `json:"error,omitempty"`
	IP           string  `json:"ip"`
	Port         int     `json:"port"`
}

// PortCheck checks if a TCP port is open on a remote IP via MikroTik /tool/fetch.
// Uses HTTP fetch to attempt TCP connection; parses result to determine open/closed/filtered.
func (c *Client) PortCheck(ip string, port int, timeoutSec int) (*PortCheckResult, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	if timeoutSec <= 0 {
		timeoutSec = 3
	}

	result := &PortCheckResult{
		IP:   ip,
		Port: port,
	}

	startTime := time.Now()

	// Set deadline for the entire operation
	c.conn.SetDeadline(time.Now().Add(time.Duration(timeoutSec+5) * time.Second))

	// Use /tool/fetch to check if port is open
	url := fmt.Sprintf("http://%s:%d/", ip, port)
	c.sendWord("/tool/fetch")
	c.sendWord("=url=" + url)
	c.sendWord("=mode=http")
	c.sendWord("=keep-result=no")
	c.sendWord(fmt.Sprintf("=duration=%ds", timeoutSec))
	c.sendWord("")

	// Read response — parse !re status attributes to determine if port is actually open.
	// MikroTik /tool/fetch sends !re progress updates (status=connecting, status=finished, etc.)
	// We must check the status value, not just assume !re means port is open.
	fetchStatus := ""     // last seen =status= value from !re
	fetchDownloaded := "" // =downloaded= value
	gotTrap := false
	for {
		word, err := c.readWord()
		if err != nil {
			result.Status = "filtered"
			result.ResponseTime = float64(time.Since(startTime).Milliseconds())
			return result, nil
		}

		if word == "!done" {
			result.ResponseTime = float64(time.Since(startTime).Milliseconds())
			if !gotTrap {
				// Determine result from fetch status
				statusLower := strings.ToLower(fetchStatus)
				if statusLower == "finished" || statusLower == "done" {
					// Fetch completed successfully — port is open
					result.Open = true
					result.Status = "open"
				} else if fetchDownloaded != "" && fetchDownloaded != "0" {
					// Got actual data — port is open
					result.Open = true
					result.Status = "open"
				} else {
					// Status still "connecting" or empty when duration expired — port not reachable
					result.Status = "filtered"
				}
			}
			for {
				attr, err := c.readWord()
				if err != nil || attr == "" {
					break
				}
			}
			break
		}

		if word == "!re" {
			// Parse attributes from progress reply
			for {
				attr, err := c.readWord()
				if err != nil || attr == "" {
					break
				}
				if strings.HasPrefix(attr, "=status=") {
					fetchStatus = strings.TrimPrefix(attr, "=status=")
				}
				if strings.HasPrefix(attr, "=downloaded=") {
					fetchDownloaded = strings.TrimPrefix(attr, "=downloaded=")
				}
			}
		}

		if word == "!trap" {
			gotTrap = true
			trapMsg := ""
			for {
				attr, err := c.readWord()
				if err != nil || attr == "" {
					break
				}
				if strings.HasPrefix(attr, "=message=") {
					trapMsg = strings.TrimPrefix(attr, "=message=")
				}
			}
			result.ResponseTime = float64(time.Since(startTime).Milliseconds())

			trapLower := strings.ToLower(trapMsg)
			if strings.Contains(trapLower, "connection refused") || strings.Contains(trapLower, "reset") {
				result.Status = "closed"
				result.Error = trapMsg
			} else if strings.Contains(trapLower, "connection timed out") || strings.Contains(trapLower, "timeout") {
				result.Status = "filtered"
				result.Error = trapMsg
			} else {
				// Any other error (HTTP 400, 401, 403, etc.) means TCP connected = port open
				result.Open = true
				result.Status = "open"
				result.Error = trapMsg
			}
			break
		}
	}

	return result, nil
}

// parseMikrotikTime parses MikroTik time formats into milliseconds
// Formats: "301us", "94ms514us", "1s200ms", "5ms", "0ms", "1s", plain number "5"
func parseMikrotikTime(timeStr string) float64 {
	timeStr = strings.TrimSpace(timeStr)
	if timeStr == "" {
		return 0
	}

	// Combined format: "94ms514us" or "1s200ms" or "1s200ms514us"
	if strings.Contains(timeStr, "s") && strings.Contains(timeStr, "ms") && strings.HasPrefix(timeStr, "") {
		// Format: Ns...ms... - extract seconds part
		sParts := strings.SplitN(timeStr, "s", 2)
		if len(sParts) == 2 && !strings.HasPrefix(sParts[0], "m") {
			// First part might be seconds
			sVal, err := strconv.ParseFloat(sParts[0], 64)
			if err == nil {
				// Remainder is ms...
				return sVal*1000.0 + parseMikrotikTime(sParts[1])
			}
		}
	}

	// Combined format: "94ms514us"
	if strings.Contains(timeStr, "ms") && strings.Contains(timeStr, "us") {
		parts := strings.Split(timeStr, "ms")
		if len(parts) == 2 {
			ms, _ := strconv.ParseFloat(parts[0], 64)
			usStr := strings.TrimSuffix(parts[1], "us")
			us, _ := strconv.ParseFloat(usStr, 64)
			return ms + us/1000.0
		}
	}

	// Microseconds only
	if strings.HasSuffix(timeStr, "us") {
		val, err := strconv.ParseFloat(strings.TrimSuffix(timeStr, "us"), 64)
		if err == nil {
			return val / 1000.0
		}
	}

	// Milliseconds only
	if strings.HasSuffix(timeStr, "ms") {
		val, err := strconv.ParseFloat(strings.TrimSuffix(timeStr, "ms"), 64)
		if err == nil {
			return val
		}
	}

	// Seconds only
	if strings.HasSuffix(timeStr, "s") {
		val, err := strconv.ParseFloat(strings.TrimSuffix(timeStr, "s"), 64)
		if err == nil {
			return val * 1000.0
		}
	}

	// Plain number - assume milliseconds
	val, err := strconv.ParseFloat(timeStr, 64)
	if err == nil {
		return val
	}

	return 0
}

// PingLive executes ping and calls onPacket for each received packet (for live streaming)
func (c *Client) PingLive(ip string, count int, size int, onPacket func(PingEvent)) (*PingResult, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	if count <= 0 {
		count = 50
	}
	if count > 100 {
		count = 100
	}

	result := &PingResult{
		Host:   ip,
		Sent:   count,
		Status: "unknown",
	}

	c.conn.SetDeadline(time.Now().Add(time.Duration(count*2+5) * time.Second))

	c.sendWord("/ping")
	c.sendWord("=address=" + ip)
	c.sendWord("=count=" + strconv.Itoa(count))
	c.sendWord("=interval=200ms")
	if size > 0 {
		c.sendWord("=size=" + strconv.Itoa(size))
	}
	c.sendWord("")

	received := 0
	var validRTTs []float64

	for {
		word, err := c.readWord()
		if err != nil {
			break
		}

		if word == "!done" {
			for {
				attr, err := c.readWord()
				if err != nil || attr == "" {
					break
				}
			}
			break
		}

		if word == "!re" {
			var packetTime float64
			hasTime := false
			isTimeout := false
			var seq, ttl, pktSize int
			var host string

			for {
				attr, err := c.readWord()
				if err != nil || attr == "" {
					break
				}
				if strings.HasPrefix(attr, "=time=") {
					hasTime = true
					packetTime = parseMikrotikTime(strings.TrimPrefix(attr, "=time="))
				}
				if strings.HasPrefix(attr, "=timeout=") || attr == "=status=timeout" {
					isTimeout = true
				}
				if strings.HasPrefix(attr, "=seq=") {
					seq, _ = strconv.Atoi(strings.TrimPrefix(attr, "=seq="))
				}
				if strings.HasPrefix(attr, "=host=") {
					host = strings.TrimPrefix(attr, "=host=")
				}
				if strings.HasPrefix(attr, "=ttl=") {
					ttl, _ = strconv.Atoi(strings.TrimPrefix(attr, "=ttl="))
				}
				if strings.HasPrefix(attr, "=size=") {
					pktSize, _ = strconv.Atoi(strings.TrimPrefix(attr, "=size="))
				}
			}

			if hasTime && !isTimeout {
				validRTTs = append(validRTTs, packetTime)
				received++
				if onPacket != nil {
					onPacket(PingEvent{Type: "reply", Seq: seq, Host: host, Time: packetTime, TTL: ttl, Size: pktSize})
				}
			} else if isTimeout {
				if onPacket != nil {
					onPacket(PingEvent{Type: "timeout", Seq: seq})
				}
			}
		}

		if word == "!trap" {
			var trapMsg string
			for {
				attr, err := c.readWord()
				if err != nil || attr == "" {
					break
				}
				trapMsg += attr + " "
			}
			result.Status = "error"
			return result, fmt.Errorf("ping failed: %s", trapMsg)
		}
	}

	result.Received = received
	result.RTTs = validRTTs
	if count > 0 {
		result.PacketLoss = ((count - received) * 100) / count
	}

	if len(validRTTs) > 0 {
		var sum float64
		result.MinRTT = validRTTs[0]
		result.MaxRTT = validRTTs[0]
		for _, rtt := range validRTTs {
			sum += rtt
			if rtt < result.MinRTT {
				result.MinRTT = rtt
			}
			if rtt > result.MaxRTT {
				result.MaxRTT = rtt
			}
		}
		result.AvgRTT = sum / float64(len(validRTTs))
		result.Status = "success"
	} else {
		result.Status = "timeout"
	}

	return result, nil
}

// TracerouteHop represents a single hop in a traceroute result
type TracerouteHop struct {
	Hop     int     `json:"hop"`
	Address string  `json:"address"`
	Loss    string  `json:"loss"`
	Last    float64 `json:"last"`
	Avg     float64 `json:"avg"`
	Best    float64 `json:"best"`
	Worst   float64 `json:"worst"`
	Status  string  `json:"status"`
}

// Traceroute executes traceroute command on MikroTik
func (c *Client) Traceroute(address string, timeout int) ([]TracerouteHop, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	if timeout <= 0 {
		timeout = 3
	}

	// Set deadline for traceroute operation (can take a while)
	c.conn.SetDeadline(time.Now().Add(60 * time.Second))

	// Run traceroute via MikroTik API
	c.sendWord("/tool/traceroute")
	c.sendWord("=address=" + address)
	c.sendWord("=count=1")
	c.sendWord("=timeout=" + strconv.Itoa(timeout) + "s")
	c.sendWord("")

	var hops []TracerouteHop
	hopMap := make(map[int]*TracerouteHop)

	for {
		word, err := c.readWord()
		if err != nil {
			break
		}

		if word == "!done" {
			break
		}

		if word == "!re" {
			hop := &TracerouteHop{}
			for {
				attr, err := c.readWord()
				if err != nil || attr == "" {
					break
				}

				if strings.HasPrefix(attr, "=.section=") {
					hopNum, _ := strconv.Atoi(strings.TrimPrefix(attr, "=.section="))
					hop.Hop = hopNum + 1
				} else if strings.HasPrefix(attr, "=address=") {
					hop.Address = strings.TrimPrefix(attr, "=address=")
				} else if strings.HasPrefix(attr, "=loss=") {
					hop.Loss = strings.TrimPrefix(attr, "=loss=")
				} else if strings.HasPrefix(attr, "=last=") {
					hop.Last = c.parseTracerouteTime(strings.TrimPrefix(attr, "=last="))
				} else if strings.HasPrefix(attr, "=avg=") {
					hop.Avg = c.parseTracerouteTime(strings.TrimPrefix(attr, "=avg="))
				} else if strings.HasPrefix(attr, "=best=") {
					hop.Best = c.parseTracerouteTime(strings.TrimPrefix(attr, "=best="))
				} else if strings.HasPrefix(attr, "=worst=") {
					hop.Worst = c.parseTracerouteTime(strings.TrimPrefix(attr, "=worst="))
				} else if strings.HasPrefix(attr, "=status=") {
					hop.Status = strings.TrimPrefix(attr, "=status=")
				}
			}

			if hop.Hop > 0 {
				if existing, ok := hopMap[hop.Hop]; ok {
					if hop.Address != "" {
						existing.Address = hop.Address
					}
					if hop.Loss != "" {
						existing.Loss = hop.Loss
					}
					if hop.Last > 0 {
						existing.Last = hop.Last
					}
					if hop.Avg > 0 {
						existing.Avg = hop.Avg
					}
					if hop.Best > 0 {
						existing.Best = hop.Best
					}
					if hop.Worst > 0 {
						existing.Worst = hop.Worst
					}
					if hop.Status != "" {
						existing.Status = hop.Status
					}
				} else {
					hopMap[hop.Hop] = hop
				}
			}
		}

		if word == "!trap" {
			for {
				attr, err := c.readWord()
				if err != nil || attr == "" {
					break
				}
			}
			return nil, fmt.Errorf("traceroute failed")
		}
	}

	// Convert map to sorted slice
	for i := 1; i <= len(hopMap); i++ {
		if h, ok := hopMap[i]; ok {
			hops = append(hops, *h)
		}
	}

	return hops, nil
}

// parseTracerouteTime parses MikroTik time format into milliseconds
func (c *Client) parseTracerouteTime(timeStr string) float64 {
	timeStr = strings.TrimSpace(timeStr)
	if timeStr == "" || timeStr == "timeout" {
		return 0
	}

	if strings.Contains(timeStr, "ms") && strings.Contains(timeStr, "us") {
		parts := strings.Split(timeStr, "ms")
		if len(parts) == 2 {
			ms, _ := strconv.ParseFloat(parts[0], 64)
			usStr := strings.TrimSuffix(parts[1], "us")
			us, _ := strconv.ParseFloat(usStr, 64)
			return ms + us/1000.0
		}
	} else if strings.HasSuffix(timeStr, "us") {
		val, _ := strconv.ParseFloat(strings.TrimSuffix(timeStr, "us"), 64)
		return val / 1000.0
	} else if strings.HasSuffix(timeStr, "ms") {
		val, _ := strconv.ParseFloat(strings.TrimSuffix(timeStr, "ms"), 64)
		return val
	} else if strings.HasSuffix(timeStr, "s") {
		val, _ := strconv.ParseFloat(strings.TrimSuffix(timeStr, "s"), 64)
		return val * 1000.0
	}

	val, _ := strconv.ParseFloat(timeStr, 64)
	return val
}

// PortRuleConfig holds configuration for port-based PCQ rules
type PortRuleConfig struct {
	Name        string // Rule name (e.g. "SP")
	Port        string // Port number (e.g. "8080") - empty for dscp direction
	Direction   string // "src", "dst", "both", or "dscp"
	DSCPValue   int    // DSCP value 0-63 (only for dscp direction)
	SpeedLimitM int64  // Speed in Mbps
	CompanyName string
}

// SyncPortRule creates PCQ queue type + mangle rule(s) + simple queue for port-based speed limiting
func (c *Client) SyncPortRule(config PortRuleConfig) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	queueTypeName := fmt.Sprintf("PORT-%s-%d", config.Name, config.SpeedLimitM)
	pcqRate := fmt.Sprintf("%dM", config.SpeedLimitM)
	packetMark := fmt.Sprintf("PORT-%s", config.Name)
	queueName := fmt.Sprintf("PORT-%s-%dM", config.Name, config.SpeedLimitM)
	queueComment := fmt.Sprintf("Port rule %s port %s %dM", config.Name, config.Port, config.SpeedLimitM)

	// Step 1: Create/update PCQ queue type
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/queue/type/print")
	c.sendWord("?name=" + queueTypeName)
	c.sendWord("")
	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to check queue type: %v", err)
	}
	var qtID string
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			qtID = strings.TrimPrefix(word, "=.id=")
			break
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	if qtID != "" {
		c.sendWord("/queue/type/set")
		c.sendWord("=.id=" + qtID)
		c.sendWord("=pcq-rate=" + pcqRate)
		c.sendWord("")
	} else {
		c.sendWord("/queue/type/add")
		c.sendWord("=name=" + queueTypeName)
		c.sendWord("=kind=pcq")
		c.sendWord("=pcq-rate=" + pcqRate)
		c.sendWord("=pcq-classifier=dst-address")
		c.sendWord("=pcq-limit=50KiB")
		c.sendWord("=pcq-total-limit=2000KiB")
		c.sendWord("=pcq-burst-rate=0")
		c.sendWord("=pcq-burst-threshold=0")
		c.sendWord("=pcq-burst-time=10s")
		c.sendWord("")
	}
	if _, err := c.readResponse(); err != nil {
		return fmt.Errorf("failed to create/update queue type: %v", err)
	}
	log.Printf("MikroTik: Created/updated port rule queue type %s rate=%s", queueTypeName, pcqRate)

	// Step 2: Create mangle rule(s) based on direction
	if config.Direction == "dscp" {
		// DSCP direction: single mangle rule matching by DSCP value, chain=postrouting
		dscpComment := fmt.Sprintf("%s CDN %s DSCP %d", config.CompanyName, config.Name, config.DSCPValue)
		c.conn.SetDeadline(time.Now().Add(c.timeout))
		c.sendWord("/ip/firewall/mangle/print")
		c.sendWord("?comment=" + dscpComment)
		c.sendWord("")
		response, err = c.readResponse()
		if err != nil {
			return fmt.Errorf("failed to check DSCP mangle rule: %v", err)
		}
		var dscpMangleID string
		for _, word := range response {
			if strings.HasPrefix(word, "=.id=") {
				dscpMangleID = strings.TrimPrefix(word, "=.id=")
				break
			}
		}
		if dscpMangleID != "" {
			c.conn.SetDeadline(time.Now().Add(c.timeout))
			c.sendWord("/ip/firewall/mangle/remove")
			c.sendWord("=.id=" + dscpMangleID)
			c.sendWord("")
			c.readResponse()
		}
		c.conn.SetDeadline(time.Now().Add(c.timeout))
		c.sendWord("/ip/firewall/mangle/add")
		c.sendWord("=action=mark-packet")
		c.sendWord("=chain=postrouting")
		c.sendWord(fmt.Sprintf("=dscp=%d", config.DSCPValue))
		c.sendWord("=new-packet-mark=" + packetMark)
		c.sendWord("=passthrough=no")
		c.sendWord("=comment=" + dscpComment)
		c.sendWord("")
		resp, err := c.readResponse()
		if err != nil {
			return fmt.Errorf("failed to create DSCP mangle rule: %v", err)
		}
		for _, word := range resp {
			if strings.HasPrefix(word, "!trap") {
				return fmt.Errorf("DSCP mangle rule error: %v", resp)
			}
		}
		log.Printf("MikroTik: Created DSCP mangle rule dscp=%d mark=%s", config.DSCPValue, packetMark)
	} else {
		type mangleRule struct {
			comment   string
			portField string // "src-port" or "dst-port"
		}
		var rules []mangleRule
		if config.Direction == "src" || config.Direction == "both" {
			rules = append(rules, mangleRule{
				comment:   fmt.Sprintf("%s PORT %s src-%s", config.CompanyName, config.Name, config.Port),
				portField: "src-port",
			})
		}
		if config.Direction == "dst" || config.Direction == "both" {
			rules = append(rules, mangleRule{
				comment:   fmt.Sprintf("%s PORT %s dst-%s", config.CompanyName, config.Name, config.Port),
				portField: "dst-port",
			})
		}

		for _, rule := range rules {
			c.conn.SetDeadline(time.Now().Add(c.timeout))
			c.sendWord("/ip/firewall/mangle/print")
			c.sendWord("?comment=" + rule.comment)
			c.sendWord("")
			response, err = c.readResponse()
			if err != nil {
				return fmt.Errorf("failed to check mangle rule: %v", err)
			}
			var mangleID string
			for _, word := range response {
				if strings.HasPrefix(word, "=.id=") {
					mangleID = strings.TrimPrefix(word, "=.id=")
					break
				}
			}
			// Remove existing rule if any (to ensure port value is updated)
			if mangleID != "" {
				c.conn.SetDeadline(time.Now().Add(c.timeout))
				c.sendWord("/ip/firewall/mangle/remove")
				c.sendWord("=.id=" + mangleID)
				c.sendWord("")
				c.readResponse()
			}
			// Create mangle rule
			c.conn.SetDeadline(time.Now().Add(c.timeout))
			c.sendWord("/ip/firewall/mangle/add")
			c.sendWord("=action=mark-packet")
			c.sendWord("=chain=forward")
			c.sendWord("=protocol=tcp")
			c.sendWord(fmt.Sprintf("=%s=%s", rule.portField, config.Port))
			c.sendWord("=new-packet-mark=" + packetMark)
			c.sendWord("=passthrough=no")
			c.sendWord("=comment=" + rule.comment)
			c.sendWord("")
			resp, err := c.readResponse()
			if err != nil {
				return fmt.Errorf("failed to create mangle rule: %v", err)
			}
			for _, word := range resp {
				if strings.HasPrefix(word, "!trap") {
					return fmt.Errorf("mangle rule error: %v", resp)
				}
			}
			log.Printf("MikroTik: Created port rule mangle %s=%s mark=%s", rule.portField, config.Port, packetMark)
		}
	}

	// Step 3: Create/update simple queue
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/queue/simple/print")
	c.sendWord("?comment=" + queueComment)
	c.sendWord("")
	response, err = c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to check simple queue: %v", err)
	}
	var sqID string
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			sqID = strings.TrimPrefix(word, "=.id=")
			break
		}
	}
	queueType := fmt.Sprintf("%s/%s", queueTypeName, queueTypeName)
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	if sqID != "" {
		c.sendWord("/queue/simple/set")
		c.sendWord("=.id=" + sqID)
		c.sendWord("=name=" + queueName)
		c.sendWord("=packet-marks=" + packetMark)
		c.sendWord("=queue=" + queueType)
		c.sendWord("=max-limit=1G/1G")
		c.sendWord("=comment=" + queueComment)
		c.sendWord("")
	} else {
		c.sendWord("/queue/simple/add")
		c.sendWord("=name=" + queueName)
		c.sendWord("=target=0.0.0.0/0")
		c.sendWord("=packet-marks=" + packetMark)
		c.sendWord("=queue=" + queueType)
		c.sendWord("=max-limit=1G/1G")
		c.sendWord("=priority=8/8")
		c.sendWord("=comment=" + queueComment)
		c.sendWord("")
	}
	resp, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to create/update simple queue: %v", err)
	}
	for _, word := range resp {
		if strings.HasPrefix(word, "!trap") {
			return fmt.Errorf("simple queue error: %v", resp)
		}
	}

	// Move queue to top (position 0) so it processes before user queues
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/queue/simple/print")
	c.sendWord("?comment=" + queueComment)
	c.sendWord("")
	response, err = c.readResponse()
	if err == nil {
		for _, word := range response {
			if strings.HasPrefix(word, "=.id=") {
				queueID := strings.TrimPrefix(word, "=.id=")
				c.conn.SetDeadline(time.Now().Add(c.timeout))
				c.sendWord("/queue/simple/move")
				c.sendWord("=numbers=" + queueID)
				c.sendWord("=destination=0")
				c.sendWord("")
				c.readResponse()
				log.Printf("MikroTik: Moved port rule queue %s to top", queueName)
				break
			}
		}
	}

	if config.Direction == "dscp" {
		log.Printf("MikroTik: Port rule %s synced (dscp=%d dir=%s speed=%dM)", config.Name, config.DSCPValue, config.Direction, config.SpeedLimitM)
	} else {
		log.Printf("MikroTik: Port rule %s synced (port=%s dir=%s speed=%dM)", config.Name, config.Port, config.Direction, config.SpeedLimitM)
	}
	return nil
}

// RemovePortRule removes all MikroTik rules for a port rule
func (c *Client) RemovePortRule(name string, speedLimitM int64, companyName string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	queueTypeName := fmt.Sprintf("PORT-%s-%d", name, speedLimitM)
	queueComment := fmt.Sprintf("Port rule %s", name)
	manglePrefix := fmt.Sprintf("%s PORT %s", companyName, name)

	// Remove simple queue(s) by comment prefix
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/queue/simple/print")
	c.sendWord("")
	response, _ := c.readResponse()
	var currentID string
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			currentID = strings.TrimPrefix(word, "=.id=")
		} else if strings.HasPrefix(word, "=comment=") {
			comment := strings.TrimPrefix(word, "=comment=")
			if strings.HasPrefix(comment, queueComment) && currentID != "" {
				c.conn.SetDeadline(time.Now().Add(c.timeout))
				c.sendWord("/queue/simple/remove")
				c.sendWord("=.id=" + currentID)
				c.sendWord("")
				c.readResponse()
				log.Printf("MikroTik: Removed port rule simple queue for %s", name)
			}
		}
	}

	// Remove mangle rules by comment prefix
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/ip/firewall/mangle/print")
	c.sendWord("")
	response, _ = c.readResponse()
	currentID = ""
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			currentID = strings.TrimPrefix(word, "=.id=")
		} else if strings.HasPrefix(word, "=comment=") {
			comment := strings.TrimPrefix(word, "=comment=")
			if strings.HasPrefix(comment, manglePrefix) && currentID != "" {
				c.conn.SetDeadline(time.Now().Add(c.timeout))
				c.sendWord("/ip/firewall/mangle/remove")
				c.sendWord("=.id=" + currentID)
				c.sendWord("")
				c.readResponse()
				log.Printf("MikroTik: Removed port rule mangle for %s", name)
			}
		}
	}

	// Remove queue type
	c.conn.SetDeadline(time.Now().Add(c.timeout))
	c.sendWord("/queue/type/print")
	c.sendWord("?name=" + queueTypeName)
	c.sendWord("")
	response, _ = c.readResponse()
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			qtID := strings.TrimPrefix(word, "=.id=")
			c.conn.SetDeadline(time.Now().Add(c.timeout))
			c.sendWord("/queue/type/remove")
			c.sendWord("=.id=" + qtID)
			c.sendWord("")
			c.readResponse()
			log.Printf("MikroTik: Removed port rule queue type %s", queueTypeName)
			break
		}
	}

	return nil
}

// BatchTorchResult represents a single flow from a batch torch run
type BatchTorchResult struct {
	SrcAddress string
	DstAddress string
	TxRate     int64 // bits per second
	RxRate     int64 // bits per second
}

// RunBatchCDNTorch runs a single Torch session on a NAS bridge/uplink interface
// for the specified duration and returns all observed flows.
func (c *Client) RunBatchCDNTorch(interfaceName string, duration int) ([]BatchTorchResult, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}

	if interfaceName == "" {
		return nil, fmt.Errorf("interface name is empty")
	}

	c.conn.SetDeadline(time.Now().Add(time.Duration(duration+10) * time.Second))
	c.sendWord("/tool/torch")
	c.sendWord("=interface=" + interfaceName)
	c.sendWord("=src-address=0.0.0.0/0")
	c.sendWord("=dst-address=0.0.0.0/0")
	c.sendWord("=port=any")
	c.sendWord("=ip-protocol=any")
	c.sendWord(fmt.Sprintf("=duration=%d", duration))
	c.sendWord("")

	var results []BatchTorchResult

	for {
		word, err := c.readWord()
		if err != nil || word == "!done" {
			break
		}

		if word == "!trap" {
			errMsg := ""
			for {
				attr, err := c.readWord()
				if err != nil || attr == "" {
					break
				}
				if strings.HasPrefix(attr, "=message=") {
					errMsg = strings.TrimPrefix(attr, "=message=")
				}
			}
			return nil, fmt.Errorf("torch error: %s", errMsg)
		}

		if word == "!re" {
			var srcAddr, dstAddr string
			var txRate, rxRate int64

			for {
				attr, err := c.readWord()
				if err != nil || attr == "" {
					break
				}
				switch {
				case strings.HasPrefix(attr, "=src-address="):
					srcAddr = strings.TrimPrefix(attr, "=src-address=")
				case strings.HasPrefix(attr, "=dst-address="):
					dstAddr = strings.TrimPrefix(attr, "=dst-address=")
				case strings.HasPrefix(attr, "=tx="):
					txRate, _ = strconv.ParseInt(strings.TrimPrefix(attr, "=tx="), 10, 64)
				case strings.HasPrefix(attr, "=rx="):
					rxRate, _ = strconv.ParseInt(strings.TrimPrefix(attr, "=rx="), 10, 64)
				}
			}

			if srcAddr != "" && dstAddr != "" && (txRate > 0 || rxRate > 0) {
				// Strip port suffix (e.g., "10.180.94.5:443" -> "10.180.94.5")
				if idx := strings.LastIndex(srcAddr, ":"); idx > 0 {
					srcAddr = srcAddr[:idx]
				}
				if idx := strings.LastIndex(dstAddr, ":"); idx > 0 {
					dstAddr = dstAddr[:idx]
				}
				results = append(results, BatchTorchResult{
					SrcAddress: srcAddr,
					DstAddress: dstAddr,
					TxRate:     txRate,
					RxRate:     rxRate,
				})
			}
		}
	}

	return results, nil
}

// AddCDNFUPQueue creates a simple queue that limits CDN-only traffic for a subscriber.
// The queue targets the subscriber's IP with dst matching CDN subnets.
func (c *Client) AddCDNFUPQueue(username, subscriberIP, cdnSubnets string, downloadKbps, uploadKbps int, serviceName string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	queueName := fmt.Sprintf("cdn-fup-%s", username)
	comment := fmt.Sprintf("CDN %s %dk/%dk %s", serviceName, downloadKbps, uploadKbps, username)

	// First check if queue already exists
	c.conn.SetDeadline(time.Now().Add(c.timeout * 2))
	c.sendWord("/queue/simple/print")
	c.sendWord("?name=" + queueName)
	c.sendWord("")

	response, _ := c.readResponse()
	existingID := ""
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			existingID = strings.TrimPrefix(word, "=.id=")
		}
	}

	if existingID != "" {
		// Update existing queue
		c.conn.SetDeadline(time.Now().Add(c.timeout))
		c.sendWord("/queue/simple/set")
		c.sendWord("=.id=" + existingID)
		c.sendWord(fmt.Sprintf("=max-limit=%dk/%dk", uploadKbps, downloadKbps))
		c.sendWord("=target=" + subscriberIP + "/32")
		c.sendWord("=dst=" + cdnSubnets)
		c.sendWord("=comment=" + comment)
		c.sendWord("")
		c.readResponse()
		log.Printf("MikroTik: Updated CDN FUP queue for %s: %dk/%dk", username, uploadKbps, downloadKbps)
	} else {
		// Create new queue
		c.conn.SetDeadline(time.Now().Add(c.timeout))
		c.sendWord("/queue/simple/add")
		c.sendWord("=name=" + queueName)
		c.sendWord("=target=" + subscriberIP + "/32")
		c.sendWord("=dst=" + cdnSubnets)
		c.sendWord(fmt.Sprintf("=max-limit=%dk/%dk", uploadKbps, downloadKbps))
		c.sendWord("=comment=" + comment)
		c.sendWord("=place-before=0")
		c.sendWord("")

		resp, err := c.readResponse()
		if err != nil {
			return fmt.Errorf("failed to create CDN FUP queue: %v", err)
		}
		for _, word := range resp {
			if strings.HasPrefix(word, "!trap") {
				return fmt.Errorf("MikroTik error creating CDN FUP queue for %s", username)
			}
		}
		log.Printf("MikroTik: Created CDN FUP queue for %s: %dk/%dk", username, uploadKbps, downloadKbps)
	}

	return nil
}

// UpdateCDNFUPQueue updates the speed of an existing CDN FUP queue
func (c *Client) UpdateCDNFUPQueue(username string, downloadKbps, uploadKbps int) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	queueName := fmt.Sprintf("cdn-fup-%s", username)

	c.conn.SetDeadline(time.Now().Add(c.timeout * 2))
	c.sendWord("/queue/simple/print")
	c.sendWord("?name=" + queueName)
	c.sendWord("")

	response, _ := c.readResponse()
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			id := strings.TrimPrefix(word, "=.id=")
			c.conn.SetDeadline(time.Now().Add(c.timeout))
			c.sendWord("/queue/simple/set")
			c.sendWord("=.id=" + id)
			c.sendWord(fmt.Sprintf("=max-limit=%dk/%dk", uploadKbps, downloadKbps))
			c.sendWord("")
			c.readResponse()
			log.Printf("MikroTik: Updated CDN FUP queue speed for %s: %dk/%dk", username, uploadKbps, downloadKbps)
			return nil
		}
	}

	return fmt.Errorf("CDN FUP queue not found for %s", username)
}

// RemoveCDNFUPQueue removes the CDN FUP queue for a subscriber
func (c *Client) RemoveCDNFUPQueue(username string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	queueName := fmt.Sprintf("cdn-fup-%s", username)

	c.conn.SetDeadline(time.Now().Add(c.timeout * 2))
	c.sendWord("/queue/simple/print")
	c.sendWord("?name=" + queueName)
	c.sendWord("")

	response, _ := c.readResponse()
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			id := strings.TrimPrefix(word, "=.id=")
			c.conn.SetDeadline(time.Now().Add(c.timeout))
			c.sendWord("/queue/simple/remove")
			c.sendWord("=.id=" + id)
			c.sendWord("")
			c.readResponse()
			log.Printf("MikroTik: Removed CDN FUP queue for %s", username)
			return nil
		}
	}

	return nil // Queue didn't exist
}

// RemoveAllCDNFUPQueues removes all CDN FUP queues (prefix "cdn-fup-") from the router
func (c *Client) RemoveAllCDNFUPQueues() error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}

	c.conn.SetDeadline(time.Now().Add(c.timeout * 3))
	c.sendWord("/queue/simple/print")
	c.sendWord("")

	response, _ := c.readResponse()

	var idsToRemove []string
	currentID := ""
	currentName := ""
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			currentID = strings.TrimPrefix(word, "=.id=")
		}
		if strings.HasPrefix(word, "=name=") {
			currentName = strings.TrimPrefix(word, "=name=")
		}
		if word == "!re" {
			if currentID != "" && strings.HasPrefix(currentName, "cdn-fup-") {
				idsToRemove = append(idsToRemove, currentID)
			}
			currentID = ""
			currentName = ""
		}
	}
	// Check last entry
	if currentID != "" && strings.HasPrefix(currentName, "cdn-fup-") {
		idsToRemove = append(idsToRemove, currentID)
	}

	for _, id := range idsToRemove {
		c.conn.SetDeadline(time.Now().Add(c.timeout))
		c.sendWord("/queue/simple/remove")
		c.sendWord("=.id=" + id)
		c.sendWord("")
		c.readResponse()
	}

	if len(idsToRemove) > 0 {
		log.Printf("MikroTik: Removed %d CDN FUP queues", len(idsToRemove))
	}

	return nil
}

// BandwidthTestResult contains the result of a MikroTik bandwidth test
type BandwidthTestResult struct {
	TxCurrent int64  `json:"tx_current"`
	RxCurrent int64  `json:"rx_current"`
	TxAverage int64  `json:"tx_average"`
	RxAverage int64  `json:"rx_average"`
	Status    string `json:"status"`
	Duration  int    `json:"duration"`
}

// BandwidthTest runs /tool/bandwidth-test on the MikroTik router
func (c *Client) BandwidthTest(target string, duration int, direction string) (*BandwidthTestResult, error) {
	if duration <= 0 {
		duration = 10
	}
	if duration > 30 {
		duration = 30
	}
	if direction == "" {
		direction = "both"
	}

	c.conn.SetDeadline(time.Now().Add(time.Duration(duration+10) * time.Second))
	c.sendWord("/tool/bandwidth-test")
	c.sendWord("=address=" + target)
	c.sendWord("=duration=" + strconv.Itoa(duration))
	c.sendWord("=direction=" + direction)
	c.sendWord("=protocol=tcp")
	c.sendWord("")

	result := &BandwidthTestResult{
		Duration: duration,
		Status:   "running",
	}

	// Read responses until done
	for {
		response, err := c.readResponse()
		if err != nil {
			result.Status = "error"
			return result, err
		}

		for _, line := range response {
			if strings.HasPrefix(line, "=tx-current=") {
				result.TxCurrent, _ = strconv.ParseInt(strings.TrimPrefix(line, "=tx-current="), 10, 64)
			} else if strings.HasPrefix(line, "=rx-current=") {
				result.RxCurrent, _ = strconv.ParseInt(strings.TrimPrefix(line, "=rx-current="), 10, 64)
			} else if strings.HasPrefix(line, "=tx-total-average=") {
				result.TxAverage, _ = strconv.ParseInt(strings.TrimPrefix(line, "=tx-total-average="), 10, 64)
			} else if strings.HasPrefix(line, "=rx-total-average=") {
				result.RxAverage, _ = strconv.ParseInt(strings.TrimPrefix(line, "=rx-total-average="), 10, 64)
			} else if strings.HasPrefix(line, "=status=") {
				result.Status = strings.TrimPrefix(line, "=status=")
			}
		}

		// Check if test is done
		for _, line := range response {
			if line == "!done" {
				result.Status = "done"
				return result, nil
			}
		}

		if result.Status == "done" || result.Status == "error" {
			break
		}
	}

	return result, nil
}

// ==========================================
// Bandwidth Manager - Simple Queue Functions
// ==========================================

// BWBurstConfig holds burst parameters for bandwidth manager queues
type BWBurstConfig struct {
	Enabled     bool
	BurstDl     int // kb
	BurstUl     int // kb
	ThresholdDl int // kb
	ThresholdUl int // kb
	BurstTime   int // seconds
}

// CreateBWSimpleQueue creates a simple queue for a bandwidth customer
func (c *Client) CreateBWSimpleQueue(name, targetIP string, dlKbps, ulKbps int, burst BWBurstConfig) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	maxLimit := fmt.Sprintf("%dk/%dk", ulKbps, dlKbps)

	c.sendWord("/queue/simple/add")
	c.sendWord("=name=" + name)
	c.sendWord("=target=" + targetIP + "/32")
	c.sendWord("=max-limit=" + maxLimit)

	if burst.Enabled && burst.BurstDl > 0 && burst.BurstUl > 0 {
		c.sendWord(fmt.Sprintf("=burst-limit=%dk/%dk", burst.BurstUl, burst.BurstDl))
		c.sendWord(fmt.Sprintf("=burst-threshold=%dk/%dk", burst.ThresholdUl, burst.ThresholdDl))
		c.sendWord(fmt.Sprintf("=burst-time=%d/%d", burst.BurstTime, burst.BurstTime))
	}

	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to create BW queue: %v", err)
	}

	for _, word := range response {
		if strings.HasPrefix(word, "!trap") {
			return fmt.Errorf("MikroTik error creating BW queue: %s", strings.Join(response, " "))
		}
	}

	log.Printf("MikroTik: Created BW queue %s target=%s max-limit=%s", name, targetIP, maxLimit)
	return nil
}

// UpdateBWSimpleQueue updates an existing bandwidth manager queue by name
func (c *Client) UpdateBWSimpleQueue(queueName string, dlKbps, ulKbps int, burst BWBurstConfig) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	// Find the queue by name
	c.sendWord("/queue/simple/print")
	c.sendWord("?name=" + queueName)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to find BW queue: %v", err)
	}

	var queueID string
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			queueID = strings.TrimPrefix(word, "=.id=")
			break
		}
	}

	if queueID == "" {
		return fmt.Errorf("BW queue %s not found", queueName)
	}

	maxLimit := fmt.Sprintf("%dk/%dk", ulKbps, dlKbps)

	c.sendWord("/queue/simple/set")
	c.sendWord("=.id=" + queueID)
	c.sendWord("=max-limit=" + maxLimit)

	if burst.Enabled && burst.BurstDl > 0 && burst.BurstUl > 0 {
		c.sendWord(fmt.Sprintf("=burst-limit=%dk/%dk", burst.BurstUl, burst.BurstDl))
		c.sendWord(fmt.Sprintf("=burst-threshold=%dk/%dk", burst.ThresholdUl, burst.ThresholdDl))
		c.sendWord(fmt.Sprintf("=burst-time=%d/%d", burst.BurstTime, burst.BurstTime))
	} else {
		c.sendWord("=burst-limit=0/0")
		c.sendWord("=burst-threshold=0/0")
		c.sendWord("=burst-time=0/0")
	}

	c.sendWord("")

	response, err = c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to update BW queue: %v", err)
	}

	for _, word := range response {
		if strings.HasPrefix(word, "!trap") {
			return fmt.Errorf("MikroTik error updating BW queue: %s", strings.Join(response, " "))
		}
	}

	log.Printf("MikroTik: Updated BW queue %s max-limit=%s", queueName, maxLimit)
	return nil
}

// DeleteBWSimpleQueue removes a bandwidth manager queue by name
func (c *Client) DeleteBWSimpleQueue(queueName string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	// Find the queue
	c.sendWord("/queue/simple/print")
	c.sendWord("?name=" + queueName)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to find BW queue for deletion: %v", err)
	}

	var queueID string
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			queueID = strings.TrimPrefix(word, "=.id=")
			break
		}
	}

	if queueID == "" {
		log.Printf("MikroTik: BW queue %s not found (already removed?)", queueName)
		return nil
	}

	c.sendWord("/queue/simple/remove")
	c.sendWord("=.id=" + queueID)
	c.sendWord("")

	response, err = c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to remove BW queue: %v", err)
	}

	for _, word := range response {
		if strings.HasPrefix(word, "!trap") {
			return fmt.Errorf("MikroTik error removing BW queue: %s", strings.Join(response, " "))
		}
	}

	log.Printf("MikroTik: Deleted BW queue %s", queueName)
	return nil
}

// BWQueueStats holds queue byte counters
type BWQueueStats struct {
	BytesIn  int64
	BytesOut int64
	Exists   bool
}

// GetBWQueueStats returns byte counters for a single queue by name
func (c *Client) GetBWQueueStats(queueName string) (*BWQueueStats, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	c.sendWord("/queue/simple/print")
	c.sendWord("?name=" + queueName)
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to get BW queue stats: %v", err)
	}

	stats := &BWQueueStats{}
	for _, word := range response {
		if strings.HasPrefix(word, "=bytes=") {
			// Format: "upload/download" e.g. "123456/789012"
			bytesStr := strings.TrimPrefix(word, "=bytes=")
			parts := strings.Split(bytesStr, "/")
			if len(parts) == 2 {
				stats.BytesIn, _ = strconv.ParseInt(parts[1], 10, 64)  // download = bytes-in
				stats.BytesOut, _ = strconv.ParseInt(parts[0], 10, 64) // upload = bytes-out
				stats.Exists = true
			}
		}
	}

	return stats, nil
}

// GetBWQueueStatsMap fetches ALL simple queues and returns a map of name→stats
func (c *Client) GetBWQueueStatsMap() (map[string]*BWQueueStats, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}
	c.conn.SetDeadline(time.Now().Add(15 * time.Second))

	c.sendWord("/queue/simple/print")
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to list all queues: %v", err)
	}

	statsMap := make(map[string]*BWQueueStats)
	var currentName string
	var currentStats *BWQueueStats

	for _, word := range response {
		if word == "!re" {
			if currentName != "" && currentStats != nil {
				currentStats.Exists = true
				statsMap[currentName] = currentStats
			}
			currentName = ""
			currentStats = &BWQueueStats{}
		} else if strings.HasPrefix(word, "=name=") {
			currentName = strings.TrimPrefix(word, "=name=")
		} else if strings.HasPrefix(word, "=bytes=") && currentStats != nil {
			bytesStr := strings.TrimPrefix(word, "=bytes=")
			parts := strings.Split(bytesStr, "/")
			if len(parts) == 2 {
				currentStats.BytesIn, _ = strconv.ParseInt(parts[1], 10, 64)
				currentStats.BytesOut, _ = strconv.ParseInt(parts[0], 10, 64)
			}
		}
	}
	if currentName != "" && currentStats != nil {
		currentStats.Exists = true
		statsMap[currentName] = currentStats
	}

	return statsMap, nil
}

// GetBWTrafficViaTorch runs torch filtered by IP for bandwidth customer live graph
func (c *Client) GetBWTrafficViaTorch(iface, customerIP string, durationSec int) (int64, int64, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return 0, 0, err
		}
	}

	if durationSec <= 0 {
		durationSec = 3
	}
	if durationSec > 10 {
		durationSec = 10
	}

	c.conn.SetDeadline(time.Now().Add(time.Duration(durationSec+10) * time.Second))

	torchIface := iface
	if torchIface == "" {
		torchIface = "all"
	}

	c.sendWord("/tool/torch")
	c.sendWord("=interface=" + torchIface)
	c.sendWord("=src-address=" + customerIP)
	c.sendWord(fmt.Sprintf("=duration=%d", durationSec))
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return 0, 0, fmt.Errorf("torch failed: %v", err)
	}

	var totalTx, totalRx int64
	for _, word := range response {
		if strings.HasPrefix(word, "=tx=") {
			val := strings.TrimPrefix(word, "=tx=")
			bits, _ := strconv.ParseInt(val, 10, 64)
			totalTx += bits / 8
		} else if strings.HasPrefix(word, "=rx=") {
			val := strings.TrimPrefix(word, "=rx=")
			bits, _ := strconv.ParseInt(val, 10, 64)
			totalRx += bits / 8
		}
	}

	return totalRx, totalTx, nil
}

// MikroTikInterface represents a MikroTik network interface
type MikroTikInterface struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Running string `json:"running"`
	Disabled string `json:"disabled"`
}

// GetInterfaces fetches all interfaces from MikroTik
func (c *Client) GetInterfaces() ([]MikroTikInterface, error) {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return nil, err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	c.sendWord("/interface/print")
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %v", err)
	}

	var ifaces []MikroTikInterface
	var current MikroTikInterface

	for _, word := range response {
		if word == "!re" {
			if current.Name != "" {
				ifaces = append(ifaces, current)
			}
			current = MikroTikInterface{}
		}
		if strings.HasPrefix(word, "=.id=") {
			current.ID = strings.TrimPrefix(word, "=.id=")
		}
		if strings.HasPrefix(word, "=name=") {
			current.Name = strings.TrimPrefix(word, "=name=")
		}
		if strings.HasPrefix(word, "=type=") {
			current.Type = strings.TrimPrefix(word, "=type=")
		}
		if strings.HasPrefix(word, "=running=") {
			current.Running = strings.TrimPrefix(word, "=running=")
		}
		if strings.HasPrefix(word, "=disabled=") {
			current.Disabled = strings.TrimPrefix(word, "=disabled=")
		}
	}
	if current.Name != "" {
		ifaces = append(ifaces, current)
	}

	return ifaces, nil
}

// CreateVLAN creates a VLAN interface on MikroTik
func (c *Client) CreateVLAN(name string, vlanID int, parentInterface string, comment string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	c.sendWord("/interface/vlan/add")
	c.sendWord(fmt.Sprintf("=name=%s", name))
	c.sendWord(fmt.Sprintf("=vlan-id=%d", vlanID))
	c.sendWord(fmt.Sprintf("=interface=%s", parentInterface))
	if comment != "" {
		c.sendWord(fmt.Sprintf("=comment=%s", comment))
	}
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to create VLAN: %v", err)
	}

	for _, word := range response {
		if strings.HasPrefix(word, "!trap") {
			for _, w := range response {
				if strings.HasPrefix(w, "=message=") {
					msg := strings.TrimPrefix(w, "=message=")
					// If VLAN already exists, that's OK
					if strings.Contains(msg, "already") {
						return nil
					}
					return fmt.Errorf("MikroTik error: %s", msg)
				}
			}
		}
	}

	return nil
}

// AddIPAddress adds an IP address to an interface on MikroTik
func (c *Client) AddIPAddress(address, iface, comment string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	c.sendWord("/ip/address/add")
	c.sendWord(fmt.Sprintf("=address=%s", address))
	c.sendWord(fmt.Sprintf("=interface=%s", iface))
	if comment != "" {
		c.sendWord(fmt.Sprintf("=comment=%s", comment))
	}
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to add IP address: %v", err)
	}

	for _, word := range response {
		if strings.HasPrefix(word, "!trap") {
			for _, w := range response {
				if strings.HasPrefix(w, "=message=") {
					msg := strings.TrimPrefix(w, "=message=")
					if strings.Contains(msg, "already have") {
						return nil
					}
					return fmt.Errorf("MikroTik error: %s", msg)
				}
			}
		}
	}

	return nil
}

// AddRoute adds a static route on MikroTik
func (c *Client) AddRoute(dstAddress, gateway, comment string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	c.sendWord("/ip/route/add")
	c.sendWord(fmt.Sprintf("=dst-address=%s", dstAddress))
	c.sendWord(fmt.Sprintf("=gateway=%s", gateway))
	if comment != "" {
		c.sendWord(fmt.Sprintf("=comment=%s", comment))
	}
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to add route: %v", err)
	}

	for _, word := range response {
		if strings.HasPrefix(word, "!trap") {
			for _, w := range response {
				if strings.HasPrefix(w, "=message=") {
					msg := strings.TrimPrefix(w, "=message=")
					if strings.Contains(msg, "already") {
						return nil
					}
					return fmt.Errorf("MikroTik error: %s", msg)
				}
			}
		}
	}

	return nil
}

// RemoveRoute removes a static route by destination address from MikroTik
func (c *Client) RemoveRoute(dstAddress string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	// Find route by dst-address
	c.sendWord("/ip/route/print")
	c.sendWord(fmt.Sprintf("?dst-address=%s", dstAddress))
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to find route: %v", err)
	}

	var routeID string
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			routeID = strings.TrimPrefix(word, "=.id=")
		}
	}

	if routeID == "" {
		return nil // Route doesn't exist, nothing to remove
	}

	c.sendWord("/ip/route/remove")
	c.sendWord(fmt.Sprintf("=.id=%s", routeID))
	c.sendWord("")

	_, err = c.readResponse()
	return err
}

// RemoveRoutesByComment removes all routes matching a comment
func (c *Client) RemoveRoutesByComment(comment string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	c.sendWord("/ip/route/print")
	c.sendWord(fmt.Sprintf("?comment=%s", comment))
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to find routes: %v", err)
	}

	var ids []string
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			ids = append(ids, strings.TrimPrefix(word, "=.id="))
		}
	}

	for _, id := range ids {
		c.conn.SetDeadline(time.Now().Add(c.timeout))
		c.sendWord("/ip/route/remove")
		c.sendWord(fmt.Sprintf("=.id=%s", id))
		c.sendWord("")
		c.readResponse()
	}
	return nil
}

// RemoveIPAddressByComment removes all IP addresses matching a comment
func (c *Client) RemoveIPAddressByComment(comment string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	c.sendWord("/ip/address/print")
	c.sendWord(fmt.Sprintf("?comment=%s", comment))
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to find IP addresses: %v", err)
	}

	var ids []string
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			ids = append(ids, strings.TrimPrefix(word, "=.id="))
		}
	}

	for _, id := range ids {
		c.conn.SetDeadline(time.Now().Add(c.timeout))
		c.sendWord("/ip/address/remove")
		c.sendWord(fmt.Sprintf("=.id=%s", id))
		c.sendWord("")
		c.readResponse()
	}
	return nil
}

// RemoveVLANByComment removes VLAN interfaces matching a comment
func (c *Client) RemoveVLANByComment(comment string) error {
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return err
		}
	}
	c.conn.SetDeadline(time.Now().Add(c.timeout))

	c.sendWord("/interface/vlan/print")
	c.sendWord(fmt.Sprintf("?comment=%s", comment))
	c.sendWord("")

	response, err := c.readResponse()
	if err != nil {
		return fmt.Errorf("failed to find VLANs: %v", err)
	}

	var ids []string
	for _, word := range response {
		if strings.HasPrefix(word, "=.id=") {
			ids = append(ids, strings.TrimPrefix(word, "=.id="))
		}
	}

	for _, id := range ids {
		c.conn.SetDeadline(time.Now().Add(c.timeout))
		c.sendWord("/interface/vlan/remove")
		c.sendWord(fmt.Sprintf("=.id=%s", id))
		c.sendWord("")
		c.readResponse()
	}
	return nil
}
