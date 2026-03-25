package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
)

type PermissionHandler struct{}

func NewPermissionHandler() *PermissionHandler {
	return &PermissionHandler{}
}

// Permission Groups

// GroupWithResellers extends PermissionGroup with assigned resellers
type GroupWithResellers struct {
	models.PermissionGroup
	Resellers []ResellerInfo `json:"resellers"`
}

// ResellerInfo contains basic reseller info for display
type ResellerInfo struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	FullName string `json:"full_name"`
}

// ListGroups returns all permission groups with assigned resellers
func (h *PermissionHandler) ListGroups(c *fiber.Ctx) error {
	var groups []models.PermissionGroup
	database.DB.Order("name").Find(&groups)

	// Build response with resellers info
	result := make([]GroupWithResellers, len(groups))

	for i := range groups {
		// Load permissions
		var permissions []models.Permission
		database.DB.Table("permissions").
			Joins("JOIN permission_group_permissions pgp ON pgp.permission_id = permissions.id").
			Where("pgp.permission_group_id = ?", groups[i].ID).
			Find(&permissions)
		groups[i].Permissions = permissions

		// Load resellers assigned to this group (join with users table to get username)
		var resellers []ResellerInfo
		database.DB.Table("resellers").
			Select("resellers.id, users.username, resellers.name as full_name").
			Joins("JOIN users ON users.id = resellers.user_id").
			Where("resellers.permission_group = ?", groups[i].ID).
			Find(&resellers)

		result[i] = GroupWithResellers{
			PermissionGroup: groups[i],
			Resellers:       resellers,
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

// GetGroup returns a single permission group
func (h *PermissionHandler) GetGroup(c *fiber.Ctx) error {
	id := c.Params("id")

	var group models.PermissionGroup
	if err := database.DB.First(&group, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Permission group not found",
		})
	}

	// Manually load permissions (Preload doesn't work with gorm:"-")
	var permissions []models.Permission
	database.DB.Table("permissions").
		Joins("JOIN permission_group_permissions pgp ON pgp.permission_id = permissions.id").
		Where("pgp.permission_group_id = ?", group.ID).
		Find(&permissions)
	group.Permissions = permissions

	return c.JSON(fiber.Map{
		"success": true,
		"data":    group,
	})
}

// CreateGroup creates a new permission group
func (h *PermissionHandler) CreateGroup(c *fiber.Ctx) error {
	type CreateRequest struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		PermissionIDs []uint `json:"permission_ids"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Check if name exists
	var exists int64
	database.DB.Model(&models.PermissionGroup{}).Where("name = ?", req.Name).Count(&exists)
	if exists > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"success": false,
			"message": "Permission group name already exists",
		})
	}

	group := models.PermissionGroup{
		Name:        req.Name,
		Description: req.Description,
	}

	if err := database.DB.Create(&group).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create permission group",
		})
	}

	// Add permissions using junction table (Association doesn't work with gorm:"-")
	if len(req.PermissionIDs) > 0 {
		for _, permID := range req.PermissionIDs {
			database.DB.Exec("INSERT INTO permission_group_permissions (permission_group_id, permission_id) VALUES (?, ?)", group.ID, permID)
		}
	}

	// Load permissions manually for response (Preload doesn't work with gorm:"-")
	var permissions []models.Permission
	database.DB.Table("permissions").
		Joins("JOIN permission_group_permissions pgp ON pgp.permission_id = permissions.id").
		Where("pgp.permission_group_id = ?", group.ID).
		Find(&permissions)
	group.Permissions = permissions

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    group,
	})
}

// UpdateGroup updates a permission group
func (h *PermissionHandler) UpdateGroup(c *fiber.Ctx) error {
	id := c.Params("id")

	var group models.PermissionGroup
	if err := database.DB.First(&group, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Permission group not found",
		})
	}

	type UpdateRequest struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		PermissionIDs []uint `json:"permission_ids"`
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Check if name exists (excluding current group)
	if req.Name != "" && req.Name != group.Name {
		var exists int64
		database.DB.Model(&models.PermissionGroup{}).Where("name = ? AND id != ?", req.Name, id).Count(&exists)
		if exists > 0 {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"success": false,
				"message": "Permission group name already exists",
			})
		}
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}

	database.DB.Model(&group).Updates(updates)

	// Update permissions using junction table (Association doesn't work with gorm:"-")
	// First delete all existing permissions for this group
	database.DB.Exec("DELETE FROM permission_group_permissions WHERE permission_group_id = ?", group.ID)

	// Then insert new permissions
	if len(req.PermissionIDs) > 0 {
		for _, permID := range req.PermissionIDs {
			database.DB.Exec("INSERT INTO permission_group_permissions (permission_group_id, permission_id) VALUES (?, ?)", group.ID, permID)
		}
	}

	// Load permissions manually for response
	var permissions []models.Permission
	database.DB.Table("permissions").
		Joins("JOIN permission_group_permissions pgp ON pgp.permission_id = permissions.id").
		Where("pgp.permission_group_id = ?", group.ID).
		Find(&permissions)
	group.Permissions = permissions

	return c.JSON(fiber.Map{
		"success": true,
		"data":    group,
	})
}

// DeleteGroup deletes a permission group
func (h *PermissionHandler) DeleteGroup(c *fiber.Ctx) error {
	id := c.Params("id")

	var group models.PermissionGroup
	if err := database.DB.First(&group, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Permission group not found",
		})
	}

	// Check if group is in use
	var inUse int64
	database.DB.Model(&models.Reseller{}).Where("permission_group = ?", id).Count(&inUse)
	if inUse > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cannot delete permission group that is in use",
		})
	}

	// Clear permissions from junction table (Association doesn't work with gorm:"-")
	database.DB.Exec("DELETE FROM permission_group_permissions WHERE permission_group_id = ?", group.ID)
	database.DB.Delete(&group)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Permission group deleted",
	})
}

// Permissions

// ListPermissions returns all available permissions
func (h *PermissionHandler) ListPermissions(c *fiber.Ctx) error {
	var permissions []models.Permission
	database.DB.Order("name").Find(&permissions)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    permissions,
	})
}

// CreatePermission creates a new permission
func (h *PermissionHandler) CreatePermission(c *fiber.Ctx) error {
	var permission models.Permission
	if err := c.BodyParser(&permission); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if err := database.DB.Create(&permission).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create permission",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    permission,
	})
}

// DeletePermission deletes a permission
func (h *PermissionHandler) DeletePermission(c *fiber.Ctx) error {
	id := c.Params("id")

	result := database.DB.Delete(&models.Permission{}, id)
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Permission not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Permission deleted",
	})
}

// SeedDefaultPermissions creates default permissions (ProRadius-compatible comprehensive permissions)
func (h *PermissionHandler) SeedDefaultPermissions(c *fiber.Ctx) error {
	defaultPerms := []models.Permission{
		// ============ DASHBOARD ============
		{Name: "dashboard.view", Description: "View dashboard"},
		{Name: "dashboard.view_admin", Description: "View admin dashboard"},
		{Name: "dashboard.view_active_only", Description: "View only active users in dashboard"},
		{Name: "dashboard.stats", Description: "View dashboard statistics"},

		// ============ SUBSCRIBERS/USERS ============
		{Name: "subscribers.view", Description: "View subscribers"},
		{Name: "subscribers.view_all", Description: "View all subscribers"},
		{Name: "subscribers.create", Description: "Create subscribers"},
		{Name: "subscribers.edit", Description: "Edit subscribers"},
		{Name: "subscribers.edit_all", Description: "Edit all subscribers"},
		{Name: "subscribers.delete", Description: "Delete subscribers"},
		{Name: "subscribers.delete_all", Description: "Delete all subscribers"},
		{Name: "subscribers.delete_expired", Description: "Delete expired subscribers"},
		{Name: "subscribers.delete_all_expired", Description: "Delete all expired subscribers"},
		{Name: "subscribers.renew", Description: "Renew subscribers"},
		{Name: "subscribers.renew_all", Description: "Renew all subscribers"},
		{Name: "subscribers.disconnect", Description: "Disconnect subscribers"},
		{Name: "subscribers.disconnect_all", Description: "Disconnect all subscribers"},
		{Name: "subscribers.inactivate", Description: "Inactivate subscribers"},
		{Name: "subscribers.inactivate_all", Description: "Inactivate all subscribers"},
		{Name: "subscribers.rename", Description: "Rename subscribers"},
		{Name: "subscribers.rename_all", Description: "Rename all subscribers"},
		{Name: "subscribers.change_owner", Description: "Change subscriber owner"},
		{Name: "subscribers.change_owner_all", Description: "Change all subscribers owner"},
		{Name: "subscribers.change_service", Description: "Change subscriber service"},
		{Name: "subscribers.change_service_free", Description: "Change user service for free"},
		{Name: "subscribers.change_expiry", Description: "Change user expiry date"},
		{Name: "subscribers.change_service_money", Description: "Change service money for user"},
		{Name: "subscribers.change_service_money_all", Description: "Change service money for all users"},
		{Name: "subscribers.add_days", Description: "Add days to subscribers"},
		{Name: "subscribers.add_days_all", Description: "Add days to all subscribers"},
		{Name: "subscribers.add_days_overdue", Description: "Add overdue days to subscribers"},
		{Name: "subscribers.add_days_overdue_all", Description: "Add overdue days to all subscribers"},
		{Name: "subscribers.reset_fup", Description: "Reset FUP quota"},
		{Name: "subscribers.reset_fup_all", Description: "Reset FUP quota for all"},
		{Name: "subscribers.refill_quota", Description: "Refill monthly quota for user"},
		{Name: "subscribers.refill_quota_all", Description: "Refill monthly quota for all users"},
		{Name: "subscribers.reset_mac", Description: "Reset MAC address"},
		{Name: "subscribers.reset_mac_all", Description: "Reset MAC address for all"},
		{Name: "subscribers.unbind_mac", Description: "Unbind user MAC address"},
		{Name: "subscribers.queue_quota", Description: "Use queue quota"},
		{Name: "subscribers.ping", Description: "Ping subscriber"},
		{Name: "subscribers.ping_all", Description: "Ping all subscribers"},
		{Name: "subscribers.view_graph", Description: "View live user graph"},
		{Name: "subscribers.view_graph_all", Description: "View live user graph for all"},
		{Name: "subscribers.view_fup", Description: "View FUP level in list"},
		{Name: "subscribers.view_logs", Description: "View logs for subscribers"},
		{Name: "subscribers.view_logs_all", Description: "View logs for all subscribers"},
		{Name: "subscribers.bulk_import", Description: "Bulk import subscribers (CSV)"},
		{Name: "subscribers.bulk_add", Description: "Admin add bulk users"},
		{Name: "subscribers.bulk_update", Description: "Bulk update subscribers"},
		{Name: "subscribers.bulk_action", Description: "Bulk actions (renew/disconnect)"},
		{Name: "subscribers.change_bulk", Description: "ChangeBulk (admin bulk operations)"},
		{Name: "subscribers.export", Description: "Export/download list of subscribers"},
		{Name: "subscribers.export_all", Description: "Export/download list of all subscribers"},
		{Name: "subscribers.view_archived", Description: "View archived subscribers"},
		{Name: "subscribers.restore", Description: "Restore archived subscribers"},
		{Name: "subscribers.allow_refund", Description: "Allow refund for deleted subscribers"},
		{Name: "subscribers.refund_no_money", Description: "Stop connection without refund"},
		{Name: "subscribers.refund_no_money_all", Description: "Stop all connections without refund"},
		{Name: "subscribers.autorecharge", Description: "Auto recharge users"},

		// ============ SERVICES ============
		{Name: "services.view", Description: "View/list services"},
		{Name: "services.create", Description: "Create services"},
		{Name: "services.edit", Description: "Edit services"},
		{Name: "services.delete", Description: "Delete services"},

		// ============ NAS/ROUTERS ============
		{Name: "nas.view", Description: "View/list NAS devices"},
		{Name: "nas.create", Description: "Create NAS devices"},
		{Name: "nas.edit", Description: "Edit NAS devices"},
		{Name: "nas.delete", Description: "Delete NAS devices"},
		{Name: "nas.sync", Description: "Sync NAS devices"},
		{Name: "nas.test", Description: "Test NAS connection"},

		// ============ SESSIONS ============
		{Name: "sessions.view", Description: "View user sessions"},
		{Name: "sessions.view_all", Description: "View all users sessions"},
		{Name: "sessions.disconnect", Description: "Disconnect sessions"},
		{Name: "sessions.view_history", Description: "View session history"},

		// ============ RESELLERS ============
		{Name: "resellers.view", Description: "View resellers"},
		{Name: "resellers.view_all", Description: "View all resellers"},
		{Name: "resellers.create", Description: "Create resellers"},
		{Name: "resellers.edit", Description: "Edit resellers"},
		{Name: "resellers.edit_all", Description: "Edit all resellers"},
		{Name: "resellers.delete", Description: "Delete resellers"},
		{Name: "resellers.change_owner", Description: "Change reseller owner"},
		{Name: "resellers.change_owner_all", Description: "Change all reseller owner"},
		{Name: "resellers.add_money", Description: "Add money to resellers"},
		{Name: "resellers.add_money_all", Description: "Add money to all resellers"},
		{Name: "resellers.withdraw", Description: "Withdraw money from resellers"},
		{Name: "resellers.withdraw_all", Description: "Withdraw money from all resellers"},
		{Name: "resellers.view_subresellers", Description: "View sub-resellers"},
		{Name: "resellers.view_balance", Description: "View reseller balance"},
		{Name: "resellers.set_credit", Description: "Set reseller credit limit"},
		{Name: "resellers.add_support", Description: "Add/edit/list support users"},
		{Name: "resellers.add_collector", Description: "Add/edit/list collector users"},
		{Name: "resellers.notification", Description: "Send notification in user portal"},
		{Name: "resellers.recharge_code", Description: "Recharge voucher codes for users via API"},
		{Name: "resellers.recharge_code_all", Description: "Recharge voucher codes for all users via API"},
		{Name: "resellers.billing_add", Description: "Billing add reseller"},

		// ============ INVOICES ============
		{Name: "invoices.view", Description: "View invoices"},
		{Name: "invoices.create", Description: "Create invoices"},
		{Name: "invoices.edit", Description: "Edit invoices"},
		{Name: "invoices.delete", Description: "Delete invoices"},
		{Name: "invoices.print", Description: "Print invoices"},
		{Name: "invoices.email", Description: "Email invoices"},
		{Name: "invoices.mark_paid", Description: "Mark invoices as paid"},

		// ============ PREPAID/GENERATE CARDS ============
		{Name: "prepaid.view", Description: "View/list prepaid cards"},
		{Name: "prepaid.generate", Description: "Generate prepaid cards for users"},
		{Name: "prepaid.generate_all", Description: "Generate prepaid cards for all users"},
		{Name: "prepaid.delete", Description: "Delete prepaid cards"},
		{Name: "prepaid.disable", Description: "Disable prepaid cards"},
		{Name: "prepaid.print", Description: "Print prepaid cards"},
		{Name: "prepaid.export", Description: "Export prepaid cards"},
		{Name: "prepaid.hide_code", Description: "Hide generate card codes"},

		// ============ REPORTS ============
		{Name: "reports.view", Description: "View reports"},
		{Name: "reports.generate_all", Description: "Generate all type of reports"},
		{Name: "reports.subscribers", Description: "View subscriber reports"},
		{Name: "reports.revenue", Description: "View revenue reports"},
		{Name: "reports.services", Description: "View service reports"},
		{Name: "reports.usage", Description: "View usage reports"},
		{Name: "reports.resellers", Description: "View reseller reports"},
		{Name: "reports.export", Description: "Export reports"},

		// ============ COMMUNICATION ============
		{Name: "communication.access_module", Description: "Access communication module"},
		{Name: "communication.view_templates", Description: "View message templates"},
		{Name: "communication.create_templates", Description: "Create message templates"},
		{Name: "communication.edit_templates", Description: "Edit message templates"},
		{Name: "communication.delete_templates", Description: "Delete message templates"},
		{Name: "communication.view_rules", Description: "View automation rules"},
		{Name: "communication.create_rules", Description: "Create automation rules"},
		{Name: "communication.edit_rules", Description: "Edit automation rules"},
		{Name: "communication.delete_rules", Description: "Delete automation rules"},
		{Name: "communication.send_sms", Description: "Send SMS messages"},
		{Name: "communication.send_whatsapp", Description: "Send WhatsApp messages"},
		{Name: "communication.send_email", Description: "Send email messages"},
		{Name: "communication.view_logs", Description: "View communication logs"},

		// ============ BANDWIDTH/ADJUST RULES ============
		{Name: "bandwidth.view", Description: "View bandwidth/adjust rules"},
		{Name: "bandwidth.create", Description: "Create bandwidth/adjust rules"},
		{Name: "bandwidth.edit", Description: "Edit bandwidth/adjust rules"},
		{Name: "bandwidth.delete", Description: "Delete bandwidth/adjust rules"},
		{Name: "bandwidth.adjust", Description: "Adjust bandwidth"},

		// ============ FUP & COUNTERS ============
		{Name: "fup.view", Description: "View FUP policies"},
		{Name: "fup.create", Description: "Create FUP policies"},
		{Name: "fup.edit", Description: "Edit FUP policies"},
		{Name: "fup.delete", Description: "Delete FUP policies"},
		{Name: "counters.view", Description: "View counters"},
		{Name: "counters.view_all", Description: "View all users counters"},
		{Name: "counters.create", Description: "Create counters"},
		{Name: "counters.edit", Description: "Edit counters"},
		{Name: "counters.delete", Description: "Delete counters"},
		{Name: "counters.upload", Description: "Upload counters"},
		{Name: "counters.view_statistics", Description: "View counter statistics"},
		{Name: "counters.view_statistics_all", Description: "View counter statistics for all users"},

		// ============ STATIC IP ============
		{Name: "staticip.view", Description: "View static IPs"},
		{Name: "staticip.edit", Description: "Edit/price static IPs"},
		{Name: "staticip.rent", Description: "Rent static IP for user"},
		{Name: "staticip.rent_all", Description: "Rent static IP for all users"},

		// ============ TICKETS ============
		{Name: "tickets.view", Description: "View tickets"},
		{Name: "tickets.create", Description: "Create tickets"},
		{Name: "tickets.issue", Description: "Issue tickets for users"},
		{Name: "tickets.edit", Description: "Edit tickets"},
		{Name: "tickets.delete", Description: "Delete tickets"},
		{Name: "tickets.reply", Description: "Reply to tickets"},
		{Name: "tickets.assign", Description: "Assign tickets"},
		{Name: "tickets.close", Description: "Close tickets"},

		// ============ USERS & PERMISSIONS ============
		{Name: "users.view", Description: "View admin users"},
		{Name: "users.create", Description: "Create admin users"},
		{Name: "users.edit", Description: "Edit admin users"},
		{Name: "users.delete", Description: "Delete admin users"},
		{Name: "permissions.view", Description: "View permissions"},
		{Name: "permissions.manage", Description: "Manage permission groups"},

		// ============ BACKUPS ============
		{Name: "backups.view", Description: "View backups"},
		{Name: "backups.create", Description: "Create backups"},
		{Name: "backups.restore", Description: "Restore backups"},
		{Name: "backups.delete", Description: "Delete backups"},
		{Name: "backups.download", Description: "Download backups"},

		// ============ SETTINGS ============
		{Name: "settings.view", Description: "View settings"},
		{Name: "settings.edit", Description: "Edit settings"},
		{Name: "settings.system", Description: "System configuration"},
		{Name: "settings.radius", Description: "RADIUS configuration"},
		{Name: "settings.billing", Description: "Billing configuration"},
		{Name: "settings.notifications", Description: "Notification settings"},
		{Name: "settings.change_language", Description: "Change system language"},

		// ============ AUDIT ============
		{Name: "audit.view", Description: "View audit logs"},
		{Name: "audit.export", Description: "Export audit logs"},

		// ============ TRANSACTIONS ============
		{Name: "transactions.view", Description: "View user transactions"},
		{Name: "transactions.view_all", Description: "View all user transactions"},
		{Name: "transactions.create", Description: "Create transactions"},
		{Name: "transactions.export", Description: "Export transactions"},

		// ============ ITEMS ============
		{Name: "items.view", Description: "View items"},
		{Name: "items.edit", Description: "Add/edit/list items"},

		// ============ ADDONS ============
		{Name: "addons.view", Description: "View addon packages"},
		{Name: "addons.edit", Description: "Add/edit addons"},
		{Name: "addons.add_user", Description: "Add addon for user"},
		{Name: "addons.add_user_all", Description: "Add addon for all users"},

		// ============ ITV/IPTV ============
		{Name: "itv.view", Description: "View ITV packages"},
		{Name: "itv.edit", Description: "Edit ITV packages"},
		{Name: "itv.rent", Description: "Rent ITV for users"},

		// ============ ONLINE PAYMENTS ============
		{Name: "payments.view", Description: "View payments"},
		{Name: "payments.online_api", Description: "Access recharge online API"},
		{Name: "payments.user_recharge", Description: "Allow user to recharge online"},
		{Name: "payments.user_recharge_different", Description: "Allow user to recharge in different currency"},
		{Name: "payments.move_pending", Description: "Move pending refill for user"},
		{Name: "payments.move_pending_all", Description: "Move pending refill for all users"},
	}

	created := 0
	for _, perm := range defaultPerms {
		var exists int64
		database.DB.Model(&models.Permission{}).Where("name = ?", perm.Name).Count(&exists)
		if exists == 0 {
			database.DB.Create(&perm)
			created++
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Default permissions seeded",
		"data": fiber.Map{
			"created": created,
			"total":   len(defaultPerms),
		},
	})
}
