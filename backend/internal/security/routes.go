package security

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// RouteObfuscation provides obfuscated API route generation
// This makes it harder to understand the API structure

// routeSalt is set at compile time for route obfuscation
var routeSalt = "proisp-route-v1-2024"

// ObfuscateRoute generates an obfuscated route name
// Use this for internal/sensitive API endpoints
func ObfuscateRoute(originalRoute string) string {
	// Hash the route with salt
	data := routeSalt + originalRoute
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8]) // 16 char route
}

// RouteMap maps human-readable routes to obfuscated ones
// This allows keeping readable code while deploying obfuscated routes
type RouteMap struct {
	routes map[string]string
}

// NewRouteMap creates a new route mapping
func NewRouteMap() *RouteMap {
	return &RouteMap{
		routes: make(map[string]string),
	}
}

// Register adds a route to the map
func (rm *RouteMap) Register(readable string) string {
	obfuscated := ObfuscateRoute(readable)
	rm.routes[readable] = obfuscated
	rm.routes[obfuscated] = readable // Reverse lookup
	return obfuscated
}

// Get returns the obfuscated route for a readable one
func (rm *RouteMap) Get(readable string) string {
	if obf, ok := rm.routes[readable]; ok {
		return obf
	}
	return readable // Fallback to original
}

// InternalRoutes returns obfuscated routes for sensitive endpoints
// These are the routes that should be harder to discover
var InternalRoutes = map[string]string{
	// License endpoints
	"/api/license/validate": "/api/x/" + ObfuscateRoute("license-validate"),
	"/api/license/activate": "/api/x/" + ObfuscateRoute("license-activate"),
	"/api/license/status":   "/api/x/" + ObfuscateRoute("license-status"),

	// Admin endpoints
	"/api/admin/users":    "/api/x/" + ObfuscateRoute("admin-users"),
	"/api/admin/settings": "/api/x/" + ObfuscateRoute("admin-settings"),
	"/api/admin/audit":    "/api/x/" + ObfuscateRoute("admin-audit"),

	// System endpoints
	"/api/system/update":  "/api/x/" + ObfuscateRoute("system-update"),
	"/api/system/backup":  "/api/x/" + ObfuscateRoute("system-backup"),
	"/api/system/restore": "/api/x/" + ObfuscateRoute("system-restore"),
}

// GetObfuscatedRoute returns the obfuscated version of a route
func GetObfuscatedRoute(original string) string {
	if obf, ok := InternalRoutes[original]; ok {
		return obf
	}
	return original
}

// IsInternalRoute checks if a route should be protected
func IsInternalRoute(route string) bool {
	route = strings.TrimSuffix(route, "/")
	for orig, obf := range InternalRoutes {
		if route == orig || route == obf {
			return true
		}
	}
	return false
}
