package mikrotik

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// PoolConfig holds configuration for the connection pool
type PoolConfig struct {
	MaxConnections    int           // Max connections per NAS device
	IdleTimeout       time.Duration // Close idle connections after this
	ConnectTimeout    time.Duration // Timeout for new connections
	MaxAge            time.Duration // Max age of a connection before recycling
	CleanupInterval   time.Duration // How often to cleanup dead connections
}

// DefaultPoolConfig returns default pool configuration
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxConnections:  10,
		IdleTimeout:     5 * time.Minute,
		ConnectTimeout:  5 * time.Second,
		MaxAge:          30 * time.Minute,
		CleanupInterval: 1 * time.Minute,
	}
}

// PooledConnection represents a reusable connection
type PooledConnection struct {
	conn       net.Conn
	address    string
	username   string
	password   string
	createdAt  time.Time
	lastUsedAt time.Time
	inUse      bool
	mu         sync.Mutex
}

// ConnectionPool manages connections to multiple NAS devices
type ConnectionPool struct {
	config    *PoolConfig
	pools     map[string]*nasPool // keyed by address
	mu        sync.RWMutex
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

// nasPool is a pool for a single NAS device
type nasPool struct {
	address     string
	username    string
	password    string
	connections []*PooledConnection
	mu          sync.Mutex
	waiting     int // Number of goroutines waiting for a connection
}

var (
	globalPool     *ConnectionPool
	globalPoolOnce sync.Once
)

// GetPool returns the global connection pool (singleton)
func GetPool() *ConnectionPool {
	globalPoolOnce.Do(func() {
		globalPool = NewConnectionPool(DefaultPoolConfig())
		globalPool.Start()
	})
	return globalPool
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(config *PoolConfig) *ConnectionPool {
	if config == nil {
		config = DefaultPoolConfig()
	}
	return &ConnectionPool{
		config:   config,
		pools:    make(map[string]*nasPool),
		stopChan: make(chan struct{}),
	}
}

// Start begins the cleanup goroutine
func (p *ConnectionPool) Start() {
	p.wg.Add(1)
	go p.cleanupLoop()
	log.Println("MikroTik connection pool started")
}

// Stop shuts down the pool and closes all connections
func (p *ConnectionPool) Stop() {
	close(p.stopChan)
	p.wg.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, np := range p.pools {
		np.mu.Lock()
		for _, pc := range np.connections {
			if pc.conn != nil {
				pc.conn.Close()
			}
		}
		np.mu.Unlock()
	}

	log.Println("MikroTik connection pool stopped")
}

// cleanupLoop periodically removes stale connections
func (p *ConnectionPool) cleanupLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.cleanup()
		}
	}
}

// cleanup removes idle and old connections
func (p *ConnectionPool) cleanup() {
	p.mu.RLock()
	pools := make([]*nasPool, 0, len(p.pools))
	for _, np := range p.pools {
		pools = append(pools, np)
	}
	p.mu.RUnlock()

	now := time.Now()

	for _, np := range pools {
		np.mu.Lock()

		// Remove stale connections
		alive := make([]*PooledConnection, 0, len(np.connections))
		for _, pc := range np.connections {
			pc.mu.Lock()

			shouldRemove := false

			// Remove if: idle too long, too old, or not in use
			if !pc.inUse {
				if now.Sub(pc.lastUsedAt) > p.config.IdleTimeout {
					shouldRemove = true
				}
				if now.Sub(pc.createdAt) > p.config.MaxAge {
					shouldRemove = true
				}
			}

			if shouldRemove {
				if pc.conn != nil {
					pc.conn.Close()
				}
				pc.mu.Unlock()
				continue
			}

			pc.mu.Unlock()
			alive = append(alive, pc)
		}

		np.connections = alive
		np.mu.Unlock()
	}
}

// Get retrieves a connection from the pool, creating one if necessary
func (p *ConnectionPool) Get(address, username, password string) (*PooledConnection, error) {
	np := p.getNasPool(address, username, password)

	// Try to get an existing connection
	np.mu.Lock()
	for _, pc := range np.connections {
		pc.mu.Lock()
		if !pc.inUse && pc.conn != nil {
			// Check if connection is still alive
			if p.isConnectionAlive(pc) {
				pc.inUse = true
				pc.lastUsedAt = time.Now()
				pc.mu.Unlock()
				np.mu.Unlock()
				return pc, nil
			}
			// Connection dead, close it
			pc.conn.Close()
			pc.conn = nil
		}
		pc.mu.Unlock()
	}

	// Check if we can create a new connection
	if len(np.connections) < p.config.MaxConnections {
		np.mu.Unlock()
		return p.createConnection(np)
	}

	np.mu.Unlock()

	// Wait for a connection to become available (with timeout)
	return p.waitForConnection(np)
}

// getNasPool gets or creates a pool for a specific NAS
func (p *ConnectionPool) getNasPool(address, username, password string) *nasPool {
	p.mu.RLock()
	np, ok := p.pools[address]
	p.mu.RUnlock()

	if ok {
		return np
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	np, ok = p.pools[address]
	if ok {
		return np
	}

	np = &nasPool{
		address:     address,
		username:    username,
		password:    password,
		connections: make([]*PooledConnection, 0, p.config.MaxConnections),
	}
	p.pools[address] = np
	return np
}

// createConnection creates a new authenticated connection
func (p *ConnectionPool) createConnection(np *nasPool) (*PooledConnection, error) {
	conn, err := net.DialTimeout("tcp", np.address, p.config.ConnectTimeout)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %v", err)
	}

	pc := &PooledConnection{
		conn:       conn,
		address:    np.address,
		username:   np.username,
		password:   np.password,
		createdAt:  time.Now(),
		lastUsedAt: time.Now(),
		inUse:      true,
	}

	// Authenticate
	if err := pc.authenticate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("authentication failed: %v", err)
	}

	// Add to pool
	np.mu.Lock()
	np.connections = append(np.connections, pc)
	np.mu.Unlock()

	return pc, nil
}

// waitForConnection waits for an available connection
func (p *ConnectionPool) waitForConnection(np *nasPool) (*PooledConnection, error) {
	deadline := time.Now().Add(p.config.ConnectTimeout * 2)

	for time.Now().Before(deadline) {
		np.mu.Lock()
		for _, pc := range np.connections {
			pc.mu.Lock()
			if !pc.inUse && pc.conn != nil {
				pc.inUse = true
				pc.lastUsedAt = time.Now()
				pc.mu.Unlock()
				np.mu.Unlock()
				return pc, nil
			}
			pc.mu.Unlock()
		}
		np.mu.Unlock()

		time.Sleep(100 * time.Millisecond)
	}

	return nil, fmt.Errorf("timeout waiting for connection to %s", np.address)
}

// Put returns a connection to the pool
func (p *ConnectionPool) Put(pc *PooledConnection) {
	if pc == nil {
		return
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.inUse = false
	pc.lastUsedAt = time.Now()
}

// Remove removes and closes a connection (use when connection is broken)
func (p *ConnectionPool) Remove(pc *PooledConnection) {
	if pc == nil {
		return
	}

	pc.mu.Lock()
	if pc.conn != nil {
		pc.conn.Close()
		pc.conn = nil
	}
	pc.mu.Unlock()

	// Remove from pool
	p.mu.RLock()
	np, ok := p.pools[pc.address]
	p.mu.RUnlock()

	if !ok {
		return
	}

	np.mu.Lock()
	defer np.mu.Unlock()

	for i, conn := range np.connections {
		if conn == pc {
			np.connections = append(np.connections[:i], np.connections[i+1:]...)
			break
		}
	}
}

// isConnectionAlive checks if a connection is still usable
func (p *ConnectionPool) isConnectionAlive(pc *PooledConnection) bool {
	// Set a very short deadline to test
	pc.conn.SetDeadline(time.Now().Add(100 * time.Millisecond))

	// Try a simple read to see if connection is alive
	// This is a lightweight check
	one := make([]byte, 1)
	pc.conn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
	_, err := pc.conn.Read(one)

	// Reset deadline
	pc.conn.SetDeadline(time.Time{})

	// Timeout is expected (no data), other errors mean dead connection
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return true // Timeout is OK, connection is alive
		}
		if err == io.EOF {
			return false // Connection closed
		}
	}

	return true
}

// authenticate performs RouterOS API authentication
func (pc *PooledConnection) authenticate() error {
	pc.conn.SetDeadline(time.Now().Add(5 * time.Second))

	// Send login command
	if err := pc.sendWord("/login"); err != nil {
		return err
	}
	if err := pc.sendWord("=name=" + pc.username); err != nil {
		return err
	}
	if err := pc.sendWord("=password=" + pc.password); err != nil {
		return err
	}
	if err := pc.sendWord(""); err != nil {
		return err
	}

	// Read response
	response, err := pc.readResponse()
	if err != nil {
		return err
	}

	for _, word := range response {
		if word == "!done" {
			return nil
		}
		if strings.HasPrefix(word, "!trap") {
			return fmt.Errorf("authentication failed")
		}
		if strings.HasPrefix(word, "=ret=") {
			// Old style login with challenge
			challenge := strings.TrimPrefix(word, "=ret=")
			return pc.challengeLogin(challenge)
		}
	}

	return nil
}

// challengeLogin performs MD5 challenge-response authentication
func (pc *PooledConnection) challengeLogin(challenge string) error {
	challengeBytes, err := hex.DecodeString(challenge)
	if err != nil {
		return err
	}

	h := md5.New()
	h.Write([]byte{0})
	h.Write([]byte(pc.password))
	h.Write(challengeBytes)
	response := hex.EncodeToString(h.Sum(nil))

	pc.sendWord("/login")
	pc.sendWord("=name=" + pc.username)
	pc.sendWord("=response=00" + response)
	pc.sendWord("")

	resp, err := pc.readResponse()
	if err != nil {
		return err
	}

	for _, word := range resp {
		if word == "!done" {
			return nil
		}
		if strings.HasPrefix(word, "!trap") {
			return fmt.Errorf("challenge authentication failed")
		}
	}

	return nil
}

// sendWord sends a word to RouterOS API
func (pc *PooledConnection) sendWord(word string) error {
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

	if _, err := pc.conn.Write(lenBytes); err != nil {
		return err
	}
	if len(word) > 0 {
		if _, err := pc.conn.Write([]byte(word)); err != nil {
			return err
		}
	}
	return nil
}

// readResponse reads a complete response from RouterOS
func (pc *PooledConnection) readResponse() ([]string, error) {
	var words []string
	gotDone := false

	for {
		word, err := pc.readWord()
		if err != nil {
			if err == io.EOF {
				break
			}
			return words, err
		}

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
func (pc *PooledConnection) readWord() (string, error) {
	length, err := pc.readLength()
	if err != nil {
		return "", err
	}

	if length == 0 {
		return "", nil
	}

	word := make([]byte, length)
	_, err = io.ReadFull(pc.conn, word)
	if err != nil {
		return "", err
	}

	return string(word), nil
}

// readLength reads the length encoding from RouterOS
func (pc *PooledConnection) readLength() (int, error) {
	b := make([]byte, 1)
	_, err := pc.conn.Read(b)
	if err != nil {
		return 0, err
	}

	first := b[0]

	if first < 0x80 {
		return int(first), nil
	} else if first < 0xC0 {
		_, err := pc.conn.Read(b)
		if err != nil {
			return 0, err
		}
		return int(first&0x3F)<<8 | int(b[0]), nil
	} else if first < 0xE0 {
		extra := make([]byte, 2)
		_, err := io.ReadFull(pc.conn, extra)
		if err != nil {
			return 0, err
		}
		return int(first&0x1F)<<16 | int(extra[0])<<8 | int(extra[1]), nil
	} else if first < 0xF0 {
		extra := make([]byte, 3)
		_, err := io.ReadFull(pc.conn, extra)
		if err != nil {
			return 0, err
		}
		return int(first&0x0F)<<24 | int(extra[0])<<16 | int(extra[1])<<8 | int(extra[2]), nil
	} else {
		extra := make([]byte, 4)
		_, err := io.ReadFull(pc.conn, extra)
		if err != nil {
			return 0, err
		}
		return int(extra[0])<<24 | int(extra[1])<<16 | int(extra[2])<<8 | int(extra[3]), nil
	}
}

// Execute runs a command on the pooled connection and returns the result
func (pc *PooledConnection) Execute(command string, args ...string) ([]map[string]string, error) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.conn.SetDeadline(time.Now().Add(10 * time.Second))

	// Send command
	if err := pc.sendWord(command); err != nil {
		return nil, err
	}

	// Send arguments
	for _, arg := range args {
		if err := pc.sendWord(arg); err != nil {
			return nil, err
		}
	}

	// Send end of command
	if err := pc.sendWord(""); err != nil {
		return nil, err
	}

	// Read response
	response, err := pc.readResponse()
	if err != nil {
		return nil, err
	}

	// Parse response into maps
	var results []map[string]string
	current := make(map[string]string)

	for _, word := range response {
		if word == "!re" {
			if len(current) > 0 {
				results = append(results, current)
				current = make(map[string]string)
			}
		} else if word == "!done" {
			if len(current) > 0 {
				results = append(results, current)
			}
			break
		} else if word == "!trap" {
			// Error response
			continue
		} else if strings.HasPrefix(word, "=") {
			// Parse key=value
			parts := strings.SplitN(word[1:], "=", 2)
			if len(parts) == 2 {
				current[parts[0]] = parts[1]
			}
		} else if strings.HasPrefix(word, "!trap") {
			// Extract error message
			if strings.Contains(word, "=message=") {
				return nil, fmt.Errorf("MikroTik error: %s", word)
			}
		}
	}

	return results, nil
}

// GetStats returns pool statistics
func (p *ConnectionPool) GetStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_pools"] = len(p.pools)

	totalConns := 0
	activeConns := 0

	for addr, np := range p.pools {
		np.mu.Lock()
		poolStats := map[string]int{
			"total":  len(np.connections),
			"active": 0,
		}
		for _, pc := range np.connections {
			pc.mu.Lock()
			if pc.inUse {
				poolStats["active"]++
				activeConns++
			}
			pc.mu.Unlock()
		}
		totalConns += len(np.connections)
		np.mu.Unlock()

		stats["pool_"+addr] = poolStats
	}

	stats["total_connections"] = totalConns
	stats["active_connections"] = activeConns

	return stats
}
