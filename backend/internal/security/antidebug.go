package security

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// InitAntiDebug starts anti-debugging protection
func InitAntiDebug() {
	if isDevMode() {
		return
	}

	go antiDebugLoop()
}

func antiDebugLoop() {
	for {
		if detectDebugger() {
			// Don't exit immediately - makes it harder to find the check
			time.Sleep(time.Duration(10+time.Now().UnixNano()%20) * time.Second)
			os.Exit(1)
		}
		time.Sleep(3 * time.Second)
	}
}

func detectDebugger() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	// Check 1: TracerPid in /proc/self/status
	if checkTracerPid() {
		return true
	}

	// Check 2: Check for common debugger processes
	if checkDebuggerProcesses() {
		return true
	}

	// Check 3: Timing check (debuggers slow down execution)
	if checkTimingAnomaly() {
		return true
	}

	// Check 4: Check ptrace
	if checkPtrace() {
		return true
	}

	return false
}

func checkTracerPid() bool {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return false
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "TracerPid:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				pid, _ := strconv.Atoi(parts[1])
				return pid != 0
			}
		}
	}
	return false
}

func checkDebuggerProcesses() bool {
	debuggers := []string{"gdb", "lldb", "strace", "ltrace", "ida", "radare2", "r2", "ghidra"}

	// Check parent process
	ppidData, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return false
	}

	fields := strings.Fields(string(ppidData))
	if len(fields) < 4 {
		return false
	}

	ppid := fields[3]
	cmdline, err := os.ReadFile("/proc/" + ppid + "/cmdline")
	if err != nil {
		return false
	}

	cmdLower := strings.ToLower(string(cmdline))
	for _, dbg := range debuggers {
		if strings.Contains(cmdLower, dbg) {
			return true
		}
	}

	return false
}

var lastTimingCheck time.Time
var timingCheckCount int

func checkTimingAnomaly() bool {
	if lastTimingCheck.IsZero() {
		lastTimingCheck = time.Now()
		return false
	}

	elapsed := time.Since(lastTimingCheck)
	lastTimingCheck = time.Now()

	// If timing check interval is way off, might be debugging
	// Normal interval should be ~3 seconds
	if elapsed > 30*time.Second {
		timingCheckCount++
		if timingCheckCount > 3 {
			return true
		}
	} else {
		timingCheckCount = 0
	}

	return false
}

func checkPtrace() bool {
	// Skip ptrace check in Docker containers (no SYS_PTRACE capability)
	if isRunningInDocker() {
		return false
	}
	// Try to ptrace ourselves - if already being traced, this fails
	err := syscall.PtraceAttach(os.Getpid())
	if err == nil {
		syscall.PtraceDetach(os.Getpid())
		return false
	}
	// EPERM means we're already being traced
	return err == syscall.EPERM
}

func isRunningInDocker() bool {
	// Check for .dockerenv file
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	// Check cgroup for docker
	data, err := os.ReadFile("/proc/1/cgroup")
	if err == nil && strings.Contains(string(data), "docker") {
		return true
	}
	return false
}
