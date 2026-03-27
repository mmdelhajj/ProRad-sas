package middleware

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
	"gorm.io/gorm"
)

// tenantCache caches subdomain/domain → tenant lookups
var tenantCache sync.Map
var tenantCacheExpiry sync.Map
const tenantCacheTTL = 60 * time.Second

type cachedTenant struct {
	ID         uint
	SchemaName string
	Status     string
}

// TenantMiddleware extracts tenant from subdomain or custom domain and sets context
func TenantMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip for super-admin routes
		if strings.HasPrefix(c.Path(), "/api/saas/") {
			return c.Next()
		}

		// Try to get tenant from JWT claims first (authenticated requests)
		if tenantID, ok := c.Locals("tenant_id").(uint); ok && tenantID > 0 {
			schemaName, _ := c.Locals("tenant_schema").(string)
			if schemaName != "" {
				tenantDB := database.GetTenantDB(schemaName)
				c.Locals("tenant_db", tenantDB)
				return c.Next()
			}
		}

		// Extract from hostname
		host := c.Hostname()
		host = strings.Split(host, ":")[0] // Remove port

		tenant := resolveTenant(host)
		if tenant == nil {
			// If no tenant found and it's not a tenant subdomain,
			// allow the request through without tenant context.
			// Regular auth endpoints will handle it normally (or fail gracefully).
			return c.Next()
		}

		if tenant.Status == "suspended" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": "This account has been suspended",
			})
		}

		// Set tenant context
		c.Locals("tenant_id", tenant.ID)
		c.Locals("tenant_schema", tenant.SchemaName)
		tenantDB := database.GetTenantDB(tenant.SchemaName)
		c.Locals("tenant_db", tenantDB)

		return c.Next()
	}
}

// resolveTenant looks up tenant by hostname (subdomain or custom domain)
func resolveTenant(host string) *cachedTenant {
	// Check cache
	if cached, ok := tenantCache.Load(host); ok {
		if expiry, ok := tenantCacheExpiry.Load(host); ok {
			if time.Now().Before(expiry.(time.Time)) {
				return cached.(*cachedTenant)
			}
		}
	}

	var tenant models.Tenant

	// Try subdomain match: acme.saas.proxrad.com → subdomain = "acme"
	if strings.HasSuffix(host, ".saas.proxrad.com") {
		subdomain := strings.TrimSuffix(host, ".saas.proxrad.com")
		if err := database.DB.Where("subdomain = ? AND status != 'deleted'", subdomain).First(&tenant).Error; err != nil {
			log.Printf("SaaS: Unknown subdomain: %s", subdomain)
			return nil
		}
	} else {
		// Try custom domain match
		if err := database.DB.Where("custom_domain = ? AND status != 'deleted'", host).First(&tenant).Error; err != nil {
			log.Printf("SaaS: Unknown domain: %s", host)
			return nil
		}
	}

	cached := &cachedTenant{
		ID:         tenant.ID,
		SchemaName: tenant.SchemaName,
		Status:     tenant.Status,
	}

	// Cache the result
	tenantCache.Store(host, cached)
	tenantCacheExpiry.Store(host, time.Now().Add(tenantCacheTTL))

	return cached
}

// InvalidateTenantCache clears the tenant cache for a specific host
func InvalidateTenantCache(host string) {
	tenantCache.Delete(host)
	tenantCacheExpiry.Delete(host)
}

// InvalidateAllTenantCache clears the entire tenant cache
func InvalidateAllTenantCache() {
	tenantCache = sync.Map{}
	tenantCacheExpiry = sync.Map{}
}

// GetTenantDBFromCtx retrieves the tenant-scoped DB from fiber context.
// In SaaS mode, returns the tenant-scoped DB. In standalone mode, returns global DB.
func GetTenantDBFromCtx(c *fiber.Ctx) *gorm.DB {
	if db, ok := c.Locals("tenant_db").(*gorm.DB); ok {
		return db
	}
	return database.DB
}

// TenantRequired ensures the request has a valid tenant context
func TenantRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		tenantID, _ := c.Locals("tenant_id").(uint)
		if tenantID == 0 {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": "Tenant context required",
			})
		}
		return c.Next()
	}
}

// SuperAdminOnly restricts access to super-admin users only
func SuperAdminOnly() fiber.Handler {
	return func(c *fiber.Ctx) error {
		isSuperAdmin, ok := c.Locals("is_super_admin").(bool)
		if !ok || !isSuperAdmin {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": "Super-admin access required",
			})
		}
		return c.Next()
	}
}
