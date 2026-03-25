package license

import (
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// HeartbeatService handles periodic heartbeat sending
type HeartbeatService struct {
	interval        time.Duration
	stopChan        chan struct{}
	subscriberCount func() int
	onlineCount     func() int
}

// NewHeartbeatService creates a new heartbeat service
func NewHeartbeatService(interval time.Duration, subscriberCount, onlineCount func() int) *HeartbeatService {
	return &HeartbeatService{
		interval:        interval,
		stopChan:        make(chan struct{}),
		subscriberCount: subscriberCount,
		onlineCount:     onlineCount,
	}
}

// Start begins the heartbeat loop
func (h *HeartbeatService) Start() {
	go h.run()
}

// Stop gracefully stops the heartbeat service
func (h *HeartbeatService) Stop() {
	close(h.stopChan)
}

func (h *HeartbeatService) run() {
	// Send initial heartbeat
	h.sendHeartbeat()

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopChan:
			return
		case <-ticker.C:
			h.sendHeartbeat()
		}
	}
}

func (h *HeartbeatService) sendHeartbeat() {
	cpuUsage := getCPUUsage()
	memUsage := getMemoryUsage()
	diskUsage := getDiskUsage()

	subCount := 0
	if h.subscriberCount != nil {
		subCount = h.subscriberCount()
	}

	onlineCount := 0
	if h.onlineCount != nil {
		onlineCount = h.onlineCount()
	}

	if err := SendHeartbeat(subCount, onlineCount, cpuUsage, memUsage, diskUsage); err != nil {
		log.Printf("Failed to send heartbeat: %v", err)
	}
}

func getCPUUsage() float64 {
	// Read from /proc/stat on Linux
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return 0
	}

	// Parse first line (cpu total)
	fields := strings.Fields(lines[0])
	if len(fields) < 5 {
		return 0
	}

	var total, idle float64
	for i := 1; i < len(fields); i++ {
		val, _ := strconv.ParseFloat(fields[i], 64)
		total += val
		if i == 4 { // idle is 4th value
			idle = val
		}
	}

	if total == 0 {
		return 0
	}

	return 100.0 * (1.0 - idle/total)
}

func getMemoryUsage() float64 {
	// Try reading from /proc/meminfo first
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		// Fallback to runtime memory stats
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return float64(m.Alloc) / float64(m.Sys) * 100
	}

	var total, free, buffers, cached uint64
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			total = val
		case "MemFree:":
			free = val
		case "Buffers:":
			buffers = val
		case "Cached:":
			cached = val
		}
	}

	if total == 0 {
		return 0
	}

	used := total - free - buffers - cached
	return float64(used) / float64(total) * 100
}

func getDiskUsage() float64 {
	// Read from /proc/mounts and statfs
	// Simplified - just return 0 if we can't determine
	// In production, use syscall.Statfs
	return 0
}
