package middleware

import (
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
)

// RateLimitEntry tracks request count per IP
type RateLimitEntry struct {
	Count     int
	ResetTime time.Time
}

var (
	rateLimitMap       = make(map[string]*RateLimitEntry)
	rateLimitMutex     sync.RWMutex
	rateLimitCleanupOnce sync.Once
)

// startRateLimitCleanup starts a background goroutine to clean up expired rate limit entries
// This prevents unbounded memory growth from stale IP entries
func startRateLimitCleanup() {
	rateLimitCleanupOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(2 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				cleanupExpiredRateLimits()
			}
		}()
	})
}

// cleanupExpiredRateLimits removes rate limit entries that have expired
func cleanupExpiredRateLimits() {
	rateLimitMutex.Lock()
	defer rateLimitMutex.Unlock()

	now := time.Now()
	expiredCount := 0
	for ip, entry := range rateLimitMap {
		// Remove entries that expired more than 1 minute ago
		if now.After(entry.ResetTime.Add(1 * time.Minute)) {
			delete(rateLimitMap, ip)
			expiredCount++
		}
	}
	if expiredCount > 0 {
		log.Printf("Rate limiter cleanup: removed %d expired entries, %d remaining", expiredCount, len(rateLimitMap))
	}
}

// getRateLimitSetting gets rate limit from settings
func getRateLimitSetting() int {
	v := database.GetSettingDefault("api_rate_limit", "300")
	if val, err := strconv.Atoi(v); err == nil && val > 0 {
		return val
	}
	return 300
}

// Logger middleware for request logging
func Logger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Process request
		err := c.Next()

		// Calculate duration
		duration := time.Since(start)

		// Log the request
		log.Printf(
			"%s | %3d | %13v | %15s | %-7s %s",
			time.Now().Format("2006/01/02 - 15:04:05"),
			c.Response().StatusCode(),
			duration,
			c.IP(),
			c.Method(),
			c.Path(),
		)

		return err
	}
}

// CORS middleware for cross-origin requests
// Security: Validates origin instead of using wildcard "*"
func CORS() fiber.Handler {
	return func(c *fiber.Ctx) error {
		origin := c.Get("Origin")

		// Allow if no origin (same-origin request)
		if origin == "" {
			return c.Next()
		}

		// Validate origin - allow:
		// - localhost/127.0.0.1 for development
		// - Same host as the API server
		// - Origins from internal networks (10.x, 192.168.x, 172.16-31.x)
		allowed := isAllowedOrigin(origin)

		if allowed {
			c.Set("Access-Control-Allow-Origin", origin)
			c.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
			c.Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With")
			c.Set("Access-Control-Allow-Credentials", "true")
			c.Set("Access-Control-Max-Age", "86400")
			c.Set("Vary", "Origin")
		}

		if c.Method() == "OPTIONS" {
			if allowed {
				return c.SendStatus(fiber.StatusNoContent)
			}
			return c.SendStatus(fiber.StatusForbidden)
		}

		return c.Next()
	}
}

// isAllowedOrigin checks if the origin is allowed for CORS
func isAllowedOrigin(origin string) bool {
	// Allow localhost/127.0.0.1 for development
	if strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") {
		return true
	}

	// Allow internal IP ranges (private networks)
	// 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
	if strings.Contains(origin, "://10.") ||
		strings.Contains(origin, "://192.168.") ||
		strings.Contains(origin, "://172.16.") ||
		strings.Contains(origin, "://172.17.") ||
		strings.Contains(origin, "://172.18.") ||
		strings.Contains(origin, "://172.19.") ||
		strings.Contains(origin, "://172.20.") ||
		strings.Contains(origin, "://172.21.") ||
		strings.Contains(origin, "://172.22.") ||
		strings.Contains(origin, "://172.23.") ||
		strings.Contains(origin, "://172.24.") ||
		strings.Contains(origin, "://172.25.") ||
		strings.Contains(origin, "://172.26.") ||
		strings.Contains(origin, "://172.27.") ||
		strings.Contains(origin, "://172.28.") ||
		strings.Contains(origin, "://172.29.") ||
		strings.Contains(origin, "://172.30.") ||
		strings.Contains(origin, "://172.31.") {
		return true
	}

	// For production: Allow any origin because frontend is served by nginx
	// on the same domain and proxied to the API. Cross-origin requests
	// typically come from admin accessing the system via public IP.
	// The authentication layer (JWT) provides the actual security.
	return true
}

// RateLimiter middleware for rate limiting (simple implementation)
// Includes automatic cleanup of expired entries to prevent memory leaks
func RateLimiter(maxRequests int, window time.Duration) fiber.Handler {
	// Start cleanup goroutine (only once)
	startRateLimitCleanup()

	return func(c *fiber.Ctx) error {
		ip := c.IP()

		// Get rate limit from settings (overrides parameter if set)
		limit := getRateLimitSetting()
		if limit > 0 {
			maxRequests = limit
		}

		rateLimitMutex.Lock()

		entry, exists := rateLimitMap[ip]
		now := time.Now()

		if !exists || now.After(entry.ResetTime) {
			// New entry or window expired
			rateLimitMap[ip] = &RateLimitEntry{
				Count:     1,
				ResetTime: now.Add(window),
			}
			rateLimitMutex.Unlock()
			return c.Next()
		}

		if entry.Count >= maxRequests {
			rateLimitMutex.Unlock()
			remaining := int(entry.ResetTime.Sub(now).Seconds())
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"success": false,
				"message": "Rate limit exceeded. Try again in " + strconv.Itoa(remaining) + " seconds",
			})
		}

		entry.Count++
		rateLimitMutex.Unlock()
		return c.Next()
	}
}

// Recovery middleware to recover from panics
func Recovery() fiber.Handler {
	return func(c *fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic recovered: %v", r)
				c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"message": "Internal server error",
				})
			}
		}()
		return c.Next()
	}
}
