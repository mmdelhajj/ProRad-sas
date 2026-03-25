package database

import (
	"context"
	"encoding/json"
	"time"

	"github.com/proisp/backend/internal/models"
)

const (
	// Cache key prefixes
	CacheKeySettings       = "proisp:settings"
	CacheKeyTokenBlacklist = "proisp:token:blacklist:" // Token blacklist for logout
	CacheKeyDashboardStats = "proisp:dashboard:stats:" // Dashboard stats cache

	// Cache TTLs
	CacheTTLSettings       = 5 * time.Minute
	CacheTTLDashboardStats = 30 * time.Second // Dashboard stats refresh every 30s
)

// CacheGet retrieves a value from Redis cache and unmarshals it into dest
func CacheGet(key string, dest interface{}) error {
	ctx := context.Background()
	data, err := Redis.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// CacheSet stores a value in Redis cache with TTL
func CacheSet(key string, value interface{}, ttl time.Duration) error {
	ctx := context.Background()
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return Redis.Set(ctx, key, data, ttl).Err()
}

// CacheDelete removes a key from Redis cache
func CacheDelete(keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	ctx := context.Background()
	return Redis.Del(ctx, keys...).Err()
}

// CacheDeletePattern deletes all keys matching a pattern (use with caution)
func CacheDeletePattern(pattern string) error {
	ctx := context.Background()
	iter := Redis.Scan(ctx, 0, pattern, 0).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(keys) > 0 {
		return Redis.Del(ctx, keys...).Err()
	}
	return nil
}

// InvalidateSettingsCache clears settings cache
func InvalidateSettingsCache() {
	CacheDelete(CacheKeySettings)
}

// BlacklistToken adds a token to the blacklist (called on logout)
// The token stays blacklisted until its original expiry time
func BlacklistToken(token string, expiryDuration time.Duration) error {
	ctx := context.Background()
	key := CacheKeyTokenBlacklist + token
	return Redis.Set(ctx, key, "1", expiryDuration).Err()
}

// IsTokenBlacklisted checks if a token has been blacklisted (logged out)
func IsTokenBlacklisted(token string) bool {
	ctx := context.Background()
	key := CacheKeyTokenBlacklist + token
	exists, err := Redis.Exists(ctx, key).Result()
	if err != nil {
		return false // On error, allow token (fail open)
	}
	return exists > 0
}

// GetCompanyName retrieves the company name from system preferences for branding
// Returns empty string if not set (never returns "ProISP" as default)
func GetCompanyName() string {
	// Try cache first
	cacheKey := "proisp:branding:company_name"
	var cached string
	if err := CacheGet(cacheKey, &cached); err == nil {
		return cached
	}

	// Fetch from database
	var pref models.SystemPreference
	if err := DB.Where("key = ?", "company_name").First(&pref).Error; err != nil {
		return ""
	}

	// Cache for 5 minutes
	CacheSet(cacheKey, pref.Value, 5*time.Minute)
	return pref.Value
}
