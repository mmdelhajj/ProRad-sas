package database

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

const (
	subscriberCachePrefix = "proisp:subscriber:"
	subscriberCacheTTL    = 5 * time.Minute // Cache subscriber data for 5 minutes
)

// CachedSubscriber contains frequently accessed subscriber data for RADIUS
type CachedSubscriber struct {
	ID               uint      `json:"id"`
	Username         string    `json:"username"`
	PasswordPlain    string    `json:"password_plain"`
	ServiceID        uint      `json:"service_id"`
	Status           int       `json:"status"`
	IsOnline         bool      `json:"is_online"`
	ExpiresAt        time.Time `json:"expires_at"`
	MACAddress       string    `json:"mac_address"`
	StaticIP         string    `json:"static_ip"`
	DownloadSpeed    int       `json:"download_speed"`
	UploadSpeed      int       `json:"upload_speed"`
	DownloadSpeedStr string    `json:"download_speed_str"`
	UploadSpeedStr   string    `json:"upload_speed_str"`
	FUPLevel         int       `json:"fup_level"`
	NasID            uint      `json:"nas_id"`
}

// GetCachedSubscriber retrieves subscriber from cache or returns nil
func GetCachedSubscriber(username string) *CachedSubscriber {
	if Redis == nil {
		return nil
	}

	ctx := context.Background()
	key := subscriberCachePrefix + username

	data, err := Redis.Get(ctx, key).Bytes()
	if err != nil {
		return nil // Cache miss
	}

	var sub CachedSubscriber
	if err := json.Unmarshal(data, &sub); err != nil {
		return nil
	}

	return &sub
}

// SetCachedSubscriber stores subscriber in cache
func SetCachedSubscriber(sub *CachedSubscriber) {
	if Redis == nil || sub == nil {
		return
	}

	ctx := context.Background()
	key := subscriberCachePrefix + sub.Username

	data, err := json.Marshal(sub)
	if err != nil {
		log.Printf("Failed to marshal subscriber for cache: %v", err)
		return
	}

	Redis.Set(ctx, key, data, subscriberCacheTTL)
}

// InvalidateSubscriberCache removes subscriber from cache (call on update)
func InvalidateSubscriberCache(username string) {
	if Redis == nil {
		return
	}

	ctx := context.Background()
	key := subscriberCachePrefix + username
	Redis.Del(ctx, key)
}

// InvalidateSubscriberCacheByID removes subscriber from cache by ID
// This requires a lookup first, so use username version when possible
func InvalidateSubscriberCacheByID(id uint) {
	if DB == nil || Redis == nil {
		return
	}

	var username string
	DB.Raw("SELECT username FROM subscribers WHERE id = ?", id).Scan(&username)
	if username != "" {
		InvalidateSubscriberCache(username)
	}
}

// GetSubscriberCacheStats returns cache statistics
func GetSubscriberCacheStats() map[string]interface{} {
	stats := make(map[string]interface{})

	if Redis == nil {
		stats["status"] = "Redis not connected"
		return stats
	}

	ctx := context.Background()

	// Count cached subscribers
	keys, _ := Redis.Keys(ctx, subscriberCachePrefix+"*").Result()
	stats["cached_subscribers"] = len(keys)
	stats["cache_ttl_seconds"] = int(subscriberCacheTTL.Seconds())
	stats["cache_prefix"] = subscriberCachePrefix

	return stats
}

// WarmupSubscriberCache pre-loads frequently used subscribers
func WarmupSubscriberCache() {
	if DB == nil || Redis == nil {
		return
	}

	log.Println("Warming up subscriber cache for online users...")

	// Load online subscribers into cache
	rows, err := DB.Raw(`
		SELECT s.id, s.username, s.password_plain, s.service_id, s.status,
			   s.is_online, s.expiry_date, s.mac_address, s.static_ip,
			   COALESCE(sv.download_speed, 0) as download_speed,
			   COALESCE(sv.upload_speed, 0) as upload_speed,
			   COALESCE(sv.download_speed_str, '') as download_speed_str,
			   COALESCE(sv.upload_speed_str, '') as upload_speed_str,
			   s.fup_level, s.nas_id
		FROM subscribers s
		LEFT JOIN services sv ON s.service_id = sv.id
		WHERE s.is_online = true
		LIMIT 10000
	`).Rows()

	if err != nil {
		log.Printf("Failed to warmup subscriber cache: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var sub CachedSubscriber
		if err := rows.Scan(
			&sub.ID, &sub.Username, &sub.PasswordPlain, &sub.ServiceID, &sub.Status,
			&sub.IsOnline, &sub.ExpiresAt, &sub.MACAddress, &sub.StaticIP,
			&sub.DownloadSpeed, &sub.UploadSpeed, &sub.DownloadSpeedStr, &sub.UploadSpeedStr,
			&sub.FUPLevel, &sub.NasID,
		); err != nil {
			continue
		}
		SetCachedSubscriber(&sub)
		count++
	}

	log.Printf("Subscriber cache warmup complete: %d subscribers cached", count)
}
