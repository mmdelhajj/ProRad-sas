package middleware

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

// AuditLogger middleware logs API actions to audit log
func AuditLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip non-modifying requests
		method := c.Method()
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			return c.Next()
		}

		// Skip certain paths
		path := c.Path()
		skipPaths := []string{"/api/auth/login", "/api/auth/refresh", "/health"}
		for _, skip := range skipPaths {
			if strings.HasPrefix(path, skip) {
				return c.Next()
			}
		}

		// Get user before executing (context is valid here)
		user := GetCurrentUser(c)
		ip := c.IP()
		userAgent := c.Get("User-Agent")

		// Capture request body for POST/PUT (to get entity name)
		var requestBody []byte
		if method == "POST" || method == "PUT" || method == "PATCH" {
			requestBody = c.Body()
		}

		// For DELETE, capture entity name BEFORE deletion
		var entityNameBeforeDelete string
		if method == "DELETE" {
			entityType := getEntityTypeFromPath(path)
			entityID := extractIDFromPath(path)
			if entityID != "" {
				entityNameBeforeDelete = getEntityName(entityType, entityID)
			}
		}

		// Execute the request
		err := c.Next()

		// Read optional custom audit details injected by the handler
		customDesc, _ := c.Locals("audit_description").(string)
		customEntityID, _ := c.Locals("audit_entity_id").(uint)
		customEntityName, _ := c.Locals("audit_entity_name").(string)

		// Only log successful responses
		statusCode := c.Response().StatusCode()
		if statusCode >= 200 && statusCode < 400 && user != nil {
			logAuditEntry(user, method, path, ip, userAgent, requestBody, entityNameBeforeDelete, customDesc, customEntityID, customEntityName)
		}

		return err
	}
}

// extractIDFromPath gets the numeric ID from URL path
func extractIDFromPath(path string) string {
	idRegex := regexp.MustCompile(`/(\d+)(?:/|$)`)
	matches := idRegex.FindStringSubmatch(path)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func logAuditEntry(user *models.User, method, path, ip, userAgent string, requestBody []byte, preDeleteName, customDesc string, customEntityID uint, customEntityName string) {
	if user == nil {
		return
	}

	// Determine action based on method
	var action models.AuditAction
	switch method {
	case "POST":
		action = models.AuditActionCreate
	case "PUT", "PATCH":
		action = models.AuditActionUpdate
	case "DELETE":
		action = models.AuditActionDelete
	default:
		return
	}

	// Determine entity type from path
	entityType := getEntityTypeFromPath(path)
	if entityType == "" {
		return
	}

	// Use handler-provided description if available, otherwise generate generic one
	var description string
	if customDesc != "" {
		description = customDesc
	} else {
		description = generateDescription(action, entityType, path, requestBody, preDeleteName)
	}

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      action,
		EntityType:  entityType,
		EntityID:    customEntityID,
		EntityName:  customEntityName,
		Description: description,
		IPAddress:   ip,
		UserAgent:   userAgent,
		OldValue:    "{}",
		NewValue:    "{}",
	}
	database.DB.Create(&auditLog)
}

// generateDescription creates a human-readable description for audit logs
func generateDescription(action models.AuditAction, entityType, path string, requestBody []byte, preDeleteName string) string {
	// Extract ID from path if present
	entityID := extractIDFromPath(path)

	// Get entity name based on action type
	var entityName string
	if action == models.AuditActionDelete && preDeleteName != "" {
		// For deletes, use the pre-captured name
		entityName = preDeleteName
	} else if action == models.AuditActionCreate && len(requestBody) > 0 {
		// For creates, get name from request body
		entityName = getNameFromRequestBody(requestBody)
	} else if entityID != "" {
		// For updates, get from database
		entityName = getEntityName(entityType, entityID)
	}

	// Action verbs
	actionVerbs := map[models.AuditAction]string{
		models.AuditActionCreate: "Created",
		models.AuditActionUpdate: "Updated",
		models.AuditActionDelete: "Deleted",
	}
	verb := actionVerbs[action]

	// Handle special paths (bulk actions, etc.)
	if strings.Contains(path, "/bulk") {
		return verb + " multiple " + entityType + "s (bulk action)"
	}
	if strings.Contains(path, "/change-service") {
		return "Changed service for " + entityName
	}
	if strings.Contains(path, "/renew") {
		return "Renewed " + entityName
	}
	if strings.Contains(path, "/reset-fup") {
		return "Reset FUP for " + entityName
	}
	if strings.Contains(path, "/reset-mac") {
		return "Reset MAC for " + entityName
	}
	if strings.Contains(path, "/disconnect") {
		return "Disconnected " + entityName
	}
	if strings.Contains(path, "/activate") || strings.Contains(path, "/toggle") {
		return "Toggled status for " + entityName
	}
	if strings.Contains(path, "/impersonate") {
		return "Impersonated " + entityName
	}
	if strings.Contains(path, "/add-days") {
		return "Added days to " + entityName
	}
	if strings.Contains(path, "/refill") {
		return "Refilled quota for " + entityName
	}
	if strings.Contains(path, "/groups") && entityType == "permission" {
		return verb + " permission group" + formatEntityName(entityName)
	}

	// Default description
	if entityName != "" {
		return verb + " " + entityType + " \"" + entityName + "\""
	}
	return verb + " " + entityType
}

// getNameFromRequestBody extracts name/username from JSON request body
func getNameFromRequestBody(body []byte) string {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return ""
	}

	// Try common name fields in order of preference
	nameFields := []string{"name", "username", "full_name", "title", "display_name"}
	for _, field := range nameFields {
		if val, ok := data[field]; ok {
			if strVal, ok := val.(string); ok && strVal != "" {
				return strVal
			}
		}
	}
	return ""
}

// getEntityName looks up the entity name from database
func getEntityName(entityType, entityID string) string {
	if entityID == "" {
		return ""
	}

	switch entityType {
	case "subscriber":
		var sub models.Subscriber
		if database.DB.Select("username").First(&sub, entityID).Error == nil {
			return sub.Username
		}
	case "service":
		var svc models.Service
		if database.DB.Select("name").First(&svc, entityID).Error == nil {
			return svc.Name
		}
	case "reseller":
		var user models.User
		if database.DB.Select("username").First(&user, entityID).Error == nil {
			return user.Username
		}
	case "user":
		var user models.User
		if database.DB.Select("username").First(&user, entityID).Error == nil {
			return user.Username
		}
	case "nas":
		var nas models.Nas
		if database.DB.Select("name").First(&nas, entityID).Error == nil {
			return nas.Name
		}
	case "permission":
		var group models.PermissionGroup
		if database.DB.Select("name").First(&group, entityID).Error == nil {
			return group.Name
		}
	case "prepaid":
		return "card #" + entityID
	case "invoice":
		return "invoice #" + entityID
	case "ticket":
		return "ticket #" + entityID
	case "backup":
		return "backup #" + entityID
	case "cdn":
		var cdn models.CDN
		if database.DB.Select("name").First(&cdn, entityID).Error == nil {
			return cdn.Name
		}
	case "cdn-bandwidth":
		return "CDN bandwidth rule #" + entityID
	case "sharing":
		return "sharing detection #" + entityID
	case "communication":
		return "communication rule #" + entityID
	case "bandwidth":
		return "bandwidth rule #" + entityID
	}
	return "#" + entityID
}

// formatEntityName adds quotes around non-empty names
func formatEntityName(name string) string {
	if name == "" || strings.HasPrefix(name, "#") {
		return ""
	}
	return " \"" + name + "\""
}

func getEntityTypeFromPath(path string) string {
	// Direct path matching for common routes (most reliable)
	if strings.Contains(path, "/resellers") {
		return "reseller"
	}
	if strings.Contains(path, "/subscribers") {
		return "subscriber"
	}
	if strings.Contains(path, "/services") {
		return "service"
	}
	if strings.Contains(path, "/users") {
		return "user"
	}
	if strings.Contains(path, "/cdns") {
		return "cdn"
	}
	if strings.Contains(path, "/permissions") {
		return "permission"
	}
	if strings.Contains(path, "/nas") {
		return "nas"
	}

	// Fallback to path parsing
	parts := strings.Split(strings.TrimPrefix(path, "/api/"), "/")
	if len(parts) == 0 {
		return ""
	}

	// Map paths to entity types
	entityMap := map[string]string{
		"subscribers":        "subscriber",
		"services":           "service",
		"nas":                "nas",
		"resellers":          "reseller",
		"sessions":           "session",
		"settings":           "settings",
		"users":              "user",
		"communication":      "communication",
		"prepaid":            "prepaid",
		"invoices":           "invoice",
		"tickets":            "ticket",
		"permissions":        "permission",
		"bandwidth":          "bandwidth",
		"fup":                "fup",
		"backups":            "backup",
		"cdns":               "cdn",
		"cdn-bandwidth-rules": "cdn-bandwidth",
		"sharing":            "sharing",
		"reports":            "report",
		"notifications":      "notification",
		"system":             "system",
		"license":            "license",
	}

	if entity, ok := entityMap[parts[0]]; ok {
		return entity
	}
	return ""
}
