package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/config"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/middleware"
	"github.com/proisp/backend/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type ResellerHandler struct {
	cfg *config.Config
}

func NewResellerHandler(cfg *config.Config) *ResellerHandler {
	return &ResellerHandler{cfg: cfg}
}

// List returns all resellers
func (h *ResellerHandler) List(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "25"))
	search := c.Query("search", "")

	if page < 1 {
		page = 1
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	query := database.DB.Model(&models.Reseller{}).Preload("User").Preload("Parent.User")

	// Filter by parent for resellers
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		query = query.Where("parent_id = ?", *user.ResellerID)
	}

	// Search filter
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Joins("JOIN users ON users.id = resellers.user_id").
			Where("resellers.name ILIKE ? OR users.username ILIKE ? OR users.email ILIKE ?",
				searchPattern, searchPattern, searchPattern)
	}

	var total int64
	query.Count(&total)

	var resellers []models.Reseller
	query.Order("name ASC").Offset(offset).Limit(limit).Find(&resellers)

	// Get subscriber counts for all resellers in a single query (fix N+1)
	subscriberCounts := make(map[uint]int64)
	if len(resellers) > 0 {
		resellerIDs := make([]uint, len(resellers))
		for i, r := range resellers {
			resellerIDs[i] = r.ID
		}

		type countResult struct {
			ResellerID uint  `gorm:"column:reseller_id"`
			Count      int64 `gorm:"column:count"`
		}
		var counts []countResult
		database.DB.Model(&models.Subscriber{}).
			Select("reseller_id, COUNT(*) as count").
			Where("reseller_id IN ?", resellerIDs).
			Group("reseller_id").
			Scan(&counts)

		for _, c := range counts {
			subscriberCounts[c.ResellerID] = c.Count
		}
	}

	// Build response with subscriber counts
	type ResellerWithCount struct {
		models.Reseller
		SubscriberCount int64 `json:"subscriber_count"`
	}
	resellersWithCounts := make([]ResellerWithCount, len(resellers))
	for i, r := range resellers {
		resellersWithCounts[i] = ResellerWithCount{
			Reseller:        r,
			SubscriberCount: subscriberCounts[r.ID],
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    resellersWithCounts,
		"meta": fiber.Map{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// Get returns a single reseller
func (h *ResellerHandler) Get(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid reseller ID",
		})
	}

	var reseller models.Reseller
	if err := database.DB.Preload("User").Preload("Parent.User").Preload("Children.User").First(&reseller, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Reseller not found",
		})
	}

	// Get stats
	var stats struct {
		TotalSubscribers  int64   `json:"total_subscribers"`
		ActiveSubscribers int64   `json:"active_subscribers"`
		TotalRevenue      float64 `json:"total_revenue"`
	}

	database.DB.Model(&models.Subscriber{}).Where("reseller_id = ?", id).Count(&stats.TotalSubscribers)
	database.DB.Model(&models.Subscriber{}).Where("reseller_id = ? AND status = ?", id, models.SubscriberStatusActive).Count(&stats.ActiveSubscribers)
	database.DB.Model(&models.Transaction{}).Where("reseller_id = ? AND type IN (?, ?)", id, models.TransactionTypeNew, models.TransactionTypeRenewal).
		Select("COALESCE(SUM(ABS(amount)), 0)").Scan(&stats.TotalRevenue)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    reseller,
		"stats":   stats,
	})
}

// CreateResellerRequest represents create reseller request
type CreateResellerRequest struct {
	Username        string  `json:"username"`
	Password        string  `json:"password"`
	Email           string  `json:"email"`
	Phone           string  `json:"phone"`
	Name            string  `json:"name"`
	FullName        string  `json:"fullname"`
	Company         string  `json:"company"`
	Address         string  `json:"address"`
	Credit          float64 `json:"credit"`
	CreditLimit     float64 `json:"credit_limit"`
	Balance         float64 `json:"balance"`
	ParentID        *uint   `json:"parent_id"`
	PermissionGroup *uint   `json:"permission_group"`
	IsActive        *bool   `json:"is_active"`
	WanCheckEnabled *bool   `json:"wan_check_enabled"`
	WanCheckICMP    *bool   `json:"wan_check_icmp"`
	WanCheckPort    *bool   `json:"wan_check_port"`
}

// Create creates a new reseller
func (h *ResellerHandler) Create(c *fiber.Ctx) error {
	var req CreateResellerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Validate required fields - accept name from any of these fields
	resellerName := req.Name
	if resellerName == "" {
		resellerName = req.FullName
	}
	if resellerName == "" {
		resellerName = req.Company
	}
	if resellerName == "" {
		resellerName = req.Username // Use username as last resort
	}

	if req.Username == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Username and password are required",
		})
	}

	// Use the determined name
	req.Name = resellerName

	// Check if username exists (including soft-deleted to prevent conflicts)
	var existingCount int64
	database.DB.Unscoped().Model(&models.User{}).Where("username = ?", req.Username).Count(&existingCount)
	if existingCount > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Username already exists",
		})
	}

	// Hash password
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

	// Determine full name - use FullName if provided, otherwise use Name
	fullName := req.Name
	if req.FullName != "" {
		fullName = req.FullName
	}

	// Determine company name - use Company if provided, otherwise use Name
	companyName := req.Name
	if req.Company != "" {
		companyName = req.Company
	}

	// Create user
	user := models.User{
		Username:      req.Username,
		Password:      string(hashedPassword),
		PasswordPlain: req.Password, // Store plain text for admin visibility
		Email:         req.Email,
		Phone:         req.Phone,
		FullName:      fullName,
		UserType:      models.UserTypeReseller,
		IsActive:      true,
	}

	if err := database.DB.Create(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create user: " + err.Error(),
		})
	}

	// Set parent reseller
	currentUser := middleware.GetCurrentUser(c)
	parentID := req.ParentID
	if currentUser.UserType == models.UserTypeReseller && currentUser.ResellerID != nil {
		parentID = currentUser.ResellerID
	}

	// Determine credit - use CreditLimit if provided, otherwise use Credit
	credit := req.Credit
	if req.CreditLimit > 0 {
		credit = req.CreditLimit
	}

	// Determine is_active
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	// Determine WAN check settings — default to OFF for new resellers
	wanEnabled := false
	wanCheckEnabled := &wanEnabled
	if req.WanCheckEnabled != nil {
		wanCheckEnabled = req.WanCheckEnabled
	}
	wanCheckICMP := true
	if req.WanCheckICMP != nil {
		wanCheckICMP = *req.WanCheckICMP
	}
	wanCheckPort := true
	if req.WanCheckPort != nil {
		wanCheckPort = *req.WanCheckPort
	}

	// Create reseller
	reseller := models.Reseller{
		UserID:          user.ID,
		Name:            companyName,
		Address:         req.Address,
		Balance:         req.Balance,
		Credit:          credit,
		ParentID:        parentID,
		PermissionGroup: req.PermissionGroup,
		IsActive:        isActive,
		WanCheckEnabled: wanCheckEnabled,
		WanCheckICMP:    wanCheckICMP,
		WanCheckPort:    wanCheckPort,
	}

	if err := database.DB.Create(&reseller).Error; err != nil {
		// Rollback user creation
		database.DB.Delete(&user)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create reseller: " + err.Error(),
		})
	}

	// Update user with reseller ID
	database.DB.Model(&user).Update("reseller_id", reseller.ID)

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      currentUser.ID,
		Username:    currentUser.Username,
		UserType:    currentUser.UserType,
		Action:      models.AuditActionCreate,
		EntityType:  "reseller",
		EntityID:    reseller.ID,
		EntityName:  reseller.Name,
		Description: "Created new reseller",
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	database.DB.Preload("User").First(&reseller, reseller.ID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Reseller created successfully",
		"data":    reseller,
	})
}

// Update updates a reseller
func (h *ResellerHandler) Update(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid reseller ID",
		})
	}

	var reseller models.Reseller
	if err := database.DB.Preload("User").First(&reseller, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Reseller not found",
		})
	}

	var req map[string]interface{}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Update reseller fields
	resellerUpdates := make(map[string]interface{})
	if val, ok := req["company"]; ok {
		resellerUpdates["name"] = val
	}
	if val, ok := req["name"]; ok {
		resellerUpdates["name"] = val
	}
	if val, ok := req["address"]; ok {
		resellerUpdates["address"] = val
	}
	if val, ok := req["credit_limit"]; ok {
		resellerUpdates["credit"] = val
	}
	if val, ok := req["credit"]; ok {
		resellerUpdates["credit"] = val
	}
	if val, ok := req["parent_id"]; ok {
		resellerUpdates["parent_id"] = val
	}
	if val, ok := req["is_active"]; ok {
		resellerUpdates["is_active"] = val
		// Also sync user account active status so existing JWT sessions are blocked
		if reseller.User != nil {
			database.DB.Model(&models.User{}).Where("id = ?", reseller.User.ID).Update("is_active", val)
		}
	}
	if val, ok := req["permission_group"]; ok {
		// Handle null/empty case
		if val == nil || val == "" {
			resellerUpdates["permission_group"] = nil
		} else if floatVal, ok := val.(float64); ok {
			// JSON numbers come as float64, convert to int
			resellerUpdates["permission_group"] = int(floatVal)
		} else {
			resellerUpdates["permission_group"] = val
		}
	}
	if val, ok := req["rebrand_enabled"]; ok {
		if v, ok := val.(bool); ok {
			resellerUpdates["rebrand_enabled"] = v
		}
	}
	if val, ok := req["custom_domain"]; ok {
		if v, ok := val.(string); ok {
			resellerUpdates["custom_domain"] = strings.TrimSpace(strings.ToLower(v))
		}
	}
	if val, ok := req["wan_check_enabled"]; ok {
		if val == nil {
			resellerUpdates["wan_check_enabled"] = nil
		} else if v, ok := val.(bool); ok {
			resellerUpdates["wan_check_enabled"] = v
		}
	}
	if val, ok := req["wan_check_icmp"]; ok {
		if v, ok := val.(bool); ok {
			resellerUpdates["wan_check_icmp"] = v
		}
	}
	if val, ok := req["wan_check_port"]; ok {
		if v, ok := val.(bool); ok {
			resellerUpdates["wan_check_port"] = v
		}
	}
	if len(resellerUpdates) > 0 {
		// Use Table() to explicitly target resellers table (avoids GORM confusion with preloaded User)
		database.DB.Table("resellers").Where("id = ?", reseller.ID).Updates(resellerUpdates)
	}

	// Update user fields
	userUpdates := make(map[string]interface{})
	if val, ok := req["email"]; ok {
		userUpdates["email"] = val
	}
	if val, ok := req["phone"]; ok {
		userUpdates["phone"] = val
	}
	if val, ok := req["fullname"]; ok {
		userUpdates["full_name"] = val
	}
	if val, ok := req["full_name"]; ok {
		userUpdates["full_name"] = val
	}
	if password, ok := req["password"].(string); ok && password != "" {
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		userUpdates["password"] = string(hashedPassword)
		userUpdates["password_plain"] = password // Store plain text for admin visibility
	}
	// Allow username change - check if new username is unique
	if reseller.User != nil {
		if newUsername, ok := req["username"].(string); ok && newUsername != "" && newUsername != reseller.User.Username {
			var existingCount int64
			database.DB.Model(&models.User{}).Where("username = ? AND id != ?", newUsername, reseller.User.ID).Count(&existingCount)
			if existingCount > 0 {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": "Username already exists",
				})
			}
			userUpdates["username"] = newUsername
		}
	}
	if len(userUpdates) > 0 {
		database.DB.Model(&models.User{}).Where("id = ?", reseller.UserID).Updates(userUpdates)
	}

	// Create audit log
	user := middleware.GetCurrentUser(c)
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionUpdate,
		EntityType:  "reseller",
		EntityID:    reseller.ID,
		EntityName:  reseller.Name,
		Description: "Updated reseller",
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	database.DB.Preload("User").First(&reseller, id)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Reseller updated successfully",
		"data":    reseller,
	})
}

// Delete deletes a reseller
func (h *ResellerHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid reseller ID",
		})
	}

	var reseller models.Reseller
	if err := database.DB.First(&reseller, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Reseller not found",
		})
	}

	// Check if reseller has subscribers
	var subscriberCount int64
	database.DB.Model(&models.Subscriber{}).Where("reseller_id = ?", id).Count(&subscriberCount)
	if subscriberCount > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cannot delete reseller with subscribers",
		})
	}

	// Check if reseller has sub-resellers
	var childCount int64
	database.DB.Model(&models.Reseller{}).Where("parent_id = ?", id).Count(&childCount)
	if childCount > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cannot delete reseller with sub-resellers",
		})
	}

	// Delete reseller and user
	database.DB.Delete(&reseller)
	database.DB.Delete(&models.User{}, reseller.UserID)

	// Create audit log
	user := middleware.GetCurrentUser(c)
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionDelete,
		EntityType:  "reseller",
		EntityID:    reseller.ID,
		EntityName:  reseller.Name,
		Description: "Deleted reseller",
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Reseller deleted successfully",
	})
}

// PermanentDelete permanently removes a reseller from database (cannot be undone)
func (h *ResellerHandler) PermanentDelete(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid reseller ID",
		})
	}

	// Use Unscoped to find even soft-deleted resellers
	var reseller models.Reseller
	if err := database.DB.Unscoped().First(&reseller, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Reseller not found",
		})
	}

	// Check if reseller has subscribers (including soft-deleted)
	var subscriberCount int64
	database.DB.Unscoped().Model(&models.Subscriber{}).Where("reseller_id = ?", id).Count(&subscriberCount)
	if subscriberCount > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cannot permanently delete reseller with subscribers. Delete subscribers first.",
		})
	}

	// Check if reseller has sub-resellers (including soft-deleted)
	var childCount int64
	database.DB.Unscoped().Model(&models.Reseller{}).Where("parent_id = ?", id).Count(&childCount)
	if childCount > 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cannot permanently delete reseller with sub-resellers. Delete sub-resellers first.",
		})
	}

	// Get the username before permanent deletion for audit
	var user models.User
	database.DB.Unscoped().First(&user, reseller.UserID)
	username := user.Username

	// Permanently delete reseller and user (Unscoped + Delete = hard delete)
	database.DB.Unscoped().Delete(&reseller)
	database.DB.Unscoped().Delete(&models.User{}, reseller.UserID)

	// Create audit log
	currentUser := middleware.GetCurrentUser(c)
	auditLog := models.AuditLog{
		UserID:      currentUser.ID,
		Username:    currentUser.Username,
		UserType:    currentUser.UserType,
		Action:      models.AuditActionDelete,
		EntityType:  "reseller",
		EntityID:    reseller.ID,
		EntityName:  reseller.Name,
		Description: "Permanently deleted reseller \"" + username + "\" (username can be reused)",
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Reseller permanently deleted. Username can now be reused.",
	})
}

// Transfer transfers money to reseller
func (h *ResellerHandler) Transfer(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid reseller ID",
		})
	}

	var req struct {
		Amount float64 `json:"amount"`
		Note   string  `json:"note"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if req.Amount <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Amount must be positive",
		})
	}

	var reseller models.Reseller
	if err := database.DB.First(&reseller, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Reseller not found",
		})
	}

	user := middleware.GetCurrentUser(c)

	// Check if admin or parent reseller
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		var sourceReseller models.Reseller
		database.DB.First(&sourceReseller, *user.ResellerID)

		if sourceReseller.Balance < req.Amount {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Insufficient balance",
			})
		}

		// Deduct from source
		database.DB.Model(&sourceReseller).Update("balance", database.DB.Raw("balance - ?", req.Amount))

		// Create transaction for source
		database.DB.Create(&models.Transaction{
			Type:             models.TransactionTypeTransfer,
			Amount:           -req.Amount,
			BalanceBefore:    sourceReseller.Balance,
			BalanceAfter:     sourceReseller.Balance - req.Amount,
			ResellerID:       sourceReseller.ID,
			TargetResellerID: &reseller.ID,
			Description:      fmt.Sprintf("Transfer to %s: %s", reseller.Name, req.Note),
			IPAddress:        c.IP(),
			CreatedBy:        user.ID,
		})
	}

	// Add to target
	database.DB.Model(&reseller).Update("balance", database.DB.Raw("balance + ?", req.Amount))

	// Create transaction for target
	database.DB.Create(&models.Transaction{
		Type:          models.TransactionTypeTransfer,
		Amount:        req.Amount,
		BalanceBefore: reseller.Balance,
		BalanceAfter:  reseller.Balance + req.Amount,
		ResellerID:    reseller.ID,
		Description:   fmt.Sprintf("Transfer received: %s", req.Note),
		IPAddress:     c.IP(),
		CreatedBy:     user.ID,
	})

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionTransfer,
		EntityType:  "reseller",
		EntityID:    reseller.ID,
		EntityName:  reseller.Name,
		Description: fmt.Sprintf("Transferred $%.2f", req.Amount),
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Transfer successful",
	})
}

// Withdraw withdraws money from reseller
func (h *ResellerHandler) Withdraw(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid reseller ID",
		})
	}

	var req struct {
		Amount float64 `json:"amount"`
		Note   string  `json:"note"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	if req.Amount <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Amount must be positive",
		})
	}

	var reseller models.Reseller
	if err := database.DB.First(&reseller, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Reseller not found",
		})
	}

	if reseller.Balance < req.Amount {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Insufficient balance",
		})
	}

	user := middleware.GetCurrentUser(c)

	// Deduct from reseller
	database.DB.Model(&reseller).Update("balance", database.DB.Raw("balance - ?", req.Amount))

	// Create transaction
	database.DB.Create(&models.Transaction{
		Type:          models.TransactionTypeWithdraw,
		Amount:        -req.Amount,
		BalanceBefore: reseller.Balance,
		BalanceAfter:  reseller.Balance - req.Amount,
		ResellerID:    reseller.ID,
		Description:   fmt.Sprintf("Withdrawal: %s", req.Note),
		IPAddress:     c.IP(),
		CreatedBy:     user.ID,
	})

	// If parent reseller, add to parent
	if user.UserType == models.UserTypeReseller && user.ResellerID != nil {
		var sourceReseller models.Reseller
		database.DB.First(&sourceReseller, *user.ResellerID)

		database.DB.Model(&sourceReseller).Update("balance", database.DB.Raw("balance + ?", req.Amount))

		database.DB.Create(&models.Transaction{
			Type:             models.TransactionTypeWithdraw,
			Amount:           req.Amount,
			BalanceBefore:    sourceReseller.Balance,
			BalanceAfter:     sourceReseller.Balance + req.Amount,
			ResellerID:       sourceReseller.ID,
			TargetResellerID: &reseller.ID,
			Description:      fmt.Sprintf("Withdrawal from %s: %s", reseller.Name, req.Note),
			IPAddress:        c.IP(),
			CreatedBy:        user.ID,
		})
	}

	// Create audit log
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionWithdraw,
		EntityType:  "reseller",
		EntityID:    reseller.ID,
		EntityName:  reseller.Name,
		Description: fmt.Sprintf("Withdrew $%.2f", req.Amount),
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Withdrawal successful",
	})
}

// Impersonate generates a login token for the reseller
func (h *ResellerHandler) Impersonate(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid reseller ID",
		})
	}

	currentUser := middleware.GetCurrentUser(c)
	if currentUser == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized",
		})
	}

	// Only admins can impersonate
	if currentUser.UserType != models.UserTypeAdmin {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"message": "Only admins can impersonate resellers",
		})
	}

	var reseller models.Reseller
	if err := database.DB.Preload("User").First(&reseller, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Reseller not found",
		})
	}

	// Generate token for the reseller's user
	token, err := middleware.GenerateToken(reseller.User, h.cfg)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate token",
		})
	}

	// Get permissions for reseller from junction table
	var permissions []string
	if reseller.PermissionGroup != nil {
		database.DB.Table("permissions").
			Joins("JOIN permission_group_permissions pgp ON pgp.permission_id = permissions.id").
			Where("pgp.permission_group_id = ?", *reseller.PermissionGroup).
			Pluck("name", &permissions)
	}

	// Create audit log
	resellerUsername := reseller.Name
	if reseller.User != nil {
		resellerUsername = reseller.User.Username
	}
	auditLog := models.AuditLog{
		UserID:      currentUser.ID,
		Username:    currentUser.Username,
		UserType:    currentUser.UserType,
		Action:      models.AuditActionLogin,
		EntityType:  "reseller",
		EntityID:    reseller.ID,
		EntityName:  reseller.Name,
		Description: fmt.Sprintf("Admin impersonated reseller %s", resellerUsername),
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	// Convert user_type to string for frontend
	userTypeStr := "reseller"
	switch reseller.User.UserType {
	case models.UserTypeAdmin:
		userTypeStr = "admin"
	case models.UserTypeSupport:
		userTypeStr = "support"
	case models.UserTypeSubscriber:
		userTypeStr = "subscriber"
	}

	// Build user response with permissions
	userResponse := fiber.Map{
		"id":                    reseller.User.ID,
		"username":              reseller.User.Username,
		"email":                 reseller.User.Email,
		"phone":                 reseller.User.Phone,
		"full_name":             reseller.User.FullName,
		"user_type":             userTypeStr,
		"is_active":             reseller.User.IsActive,
		"reseller_id":           reseller.ID,
		"permissions":           permissions,
		"force_password_change": reseller.User.ForcePasswordChange,
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Impersonation successful",
		"data": fiber.Map{
			"token":    token,
			"user":     userResponse,
			"reseller": reseller,
		},
	})
}

// GetAssignedNAS returns the NAS devices assigned to a reseller
func (h *ResellerHandler) GetAssignedNAS(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid reseller ID",
		})
	}

	// Get all NAS devices
	var allNAS []models.Nas
	database.DB.Order("name ASC").Find(&allNAS)

	// Get assigned NAS IDs
	var assignedIDs []uint
	database.DB.Model(&models.ResellerNAS{}).
		Where("reseller_id = ?", id).
		Pluck("nas_id", &assignedIDs)

	// Create a map for quick lookup
	assignedMap := make(map[uint]bool)
	for _, nasID := range assignedIDs {
		assignedMap[nasID] = true
	}

	// Build response with assigned flag
	type NASWithAssignment struct {
		models.Nas
		Assigned bool `json:"assigned"`
	}

	result := make([]NASWithAssignment, len(allNAS))
	for i, nas := range allNAS {
		result[i] = NASWithAssignment{
			Nas:      nas,
			Assigned: assignedMap[nas.ID],
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

// UpdateAssignedNASRequest represents update assigned NAS request
type UpdateAssignedNASRequest struct {
	NASIDs []uint `json:"nas_ids"`
}

// UpdateAssignedNAS updates the NAS devices assigned to a reseller
func (h *ResellerHandler) UpdateAssignedNAS(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid reseller ID",
		})
	}

	var req UpdateAssignedNASRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Delete existing assignments
	database.DB.Where("reseller_id = ?", id).Delete(&models.ResellerNAS{})

	// Create new assignments
	for _, nasID := range req.NASIDs {
		database.DB.Create(&models.ResellerNAS{
			ResellerID: uint(id),
			NASID:      nasID,
		})
	}

	// Create audit log
	user := middleware.GetCurrentUser(c)
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionUpdate,
		EntityType:  "reseller",
		EntityID:    uint(id),
		Description: fmt.Sprintf("Updated NAS assignments: %d NAS devices", len(req.NASIDs)),
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "NAS assignments updated successfully",
	})
}

// GetAssignedServices returns the services assigned to a reseller with pricing
func (h *ResellerHandler) GetAssignedServices(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid reseller ID",
		})
	}

	// Get all services
	var allServices []models.Service
	database.DB.Order("sort_order ASC, name ASC").Find(&allServices)

	// Get assigned services with pricing
	var resellerServices []models.ResellerService
	database.DB.Where("reseller_id = ?", id).Find(&resellerServices)

	// Create a map for quick lookup
	serviceMap := make(map[uint]models.ResellerService)
	for _, rs := range resellerServices {
		serviceMap[rs.ServiceID] = rs
	}

	// Build response with assignment and pricing info
	type ServiceWithAssignment struct {
		ID              uint     `json:"id"`
		Name            string   `json:"name"`
		DefaultPrice    float64  `json:"default_price"`
		DefaultDayPrice float64  `json:"default_day_price"`
		Assigned        bool     `json:"assigned"`
		CustomPrice     *float64 `json:"custom_price"`
		CustomDayPrice  *float64 `json:"custom_day_price"`
		IsEnabled       bool     `json:"is_enabled"`
	}

	result := make([]ServiceWithAssignment, len(allServices))
	for i, svc := range allServices {
		rs, assigned := serviceMap[svc.ID]
		result[i] = ServiceWithAssignment{
			ID:              svc.ID,
			Name:            svc.Name,
			DefaultPrice:    svc.Price,
			DefaultDayPrice: svc.DayPrice,
			Assigned:        assigned,
			IsEnabled:       assigned && rs.IsEnabled,
		}
		if assigned {
			// Return actual custom prices (may differ from default)
			if rs.Price != svc.Price {
				result[i].CustomPrice = &rs.Price
			}
			if rs.DayPrice != svc.DayPrice {
				result[i].CustomDayPrice = &rs.DayPrice
			}
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

// UpdateAssignedServicesRequest represents update assigned services request
type UpdateAssignedServicesRequest struct {
	Services []struct {
		ServiceID      uint     `json:"service_id"`
		Price          *float64 `json:"price"`
		DayPrice       *float64 `json:"day_price"`
		CustomPrice    *float64 `json:"custom_price"`
		CustomDayPrice *float64 `json:"custom_day_price"`
		IsEnabled      bool     `json:"is_enabled"`
	} `json:"services"`
}

// UpdateAssignedServices updates the services assigned to a reseller with pricing
func (h *ResellerHandler) UpdateAssignedServices(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid reseller ID",
		})
	}

	var req UpdateAssignedServicesRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Get all services for default prices
	var allServices []models.Service
	database.DB.Find(&allServices)
	serviceDefaultPrices := make(map[uint]models.Service)
	for _, s := range allServices {
		serviceDefaultPrices[s.ID] = s
	}

	// Delete existing assignments
	database.DB.Where("reseller_id = ?", id).Delete(&models.ResellerService{})

	// Create new assignments (only for enabled services)
	enabledCount := 0
	for _, svc := range req.Services {
		// Determine the price to use (custom_price takes precedence, then price, then default)
		var price, dayPrice float64
		defaultSvc := serviceDefaultPrices[svc.ServiceID]

		if svc.CustomPrice != nil {
			price = *svc.CustomPrice
		} else if svc.Price != nil {
			price = *svc.Price
		} else {
			price = defaultSvc.Price
		}

		if svc.CustomDayPrice != nil {
			dayPrice = *svc.CustomDayPrice
		} else if svc.DayPrice != nil {
			dayPrice = *svc.DayPrice
		} else {
			dayPrice = defaultSvc.DayPrice
		}

		database.DB.Create(&models.ResellerService{
			ResellerID: uint(id),
			ServiceID:  svc.ServiceID,
			Price:      price,
			DayPrice:   dayPrice,
			IsEnabled:  true,
		})
		enabledCount++
	}

	// Create audit log
	user := middleware.GetCurrentUser(c)
	auditLog := models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionUpdate,
		EntityType:  "reseller",
		EntityID:    uint(id),
		Description: fmt.Sprintf("Updated service assignments: %d services enabled", enabledCount),
		IPAddress:   c.IP(),
	}
	database.DB.Create(&auditLog)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Service assignments updated successfully",
	})
}

// GetServiceLimits returns per-service subscriber limits for a reseller
func (h *ResellerHandler) GetServiceLimits(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid reseller ID"})
	}

	type LimitWithService struct {
		models.ResellerServiceLimit
		ServiceName  string `json:"service_name"`
		CurrentCount int64  `json:"current_count"`
	}

	var limits []models.ResellerServiceLimit
	database.DB.Where("reseller_id = ?", id).Find(&limits)

	// Get all services
	var services []models.Service
	database.DB.Order("sort_order ASC, name ASC").Find(&services)

	// Get subscriber counts per service for this reseller
	type countResult struct {
		ServiceID uint  `gorm:"column:service_id"`
		Count     int64 `gorm:"column:count"`
	}
	var counts []countResult
	database.DB.Model(&models.Subscriber{}).
		Select("service_id, COUNT(*) as count").
		Where("reseller_id = ? AND status != 'deleted'", id).
		Group("service_id").
		Scan(&counts)

	countMap := make(map[uint]int64)
	for _, c := range counts {
		countMap[c.ServiceID] = c.Count
	}

	limitMap := make(map[uint]models.ResellerServiceLimit)
	for _, l := range limits {
		limitMap[l.ServiceID] = l
	}

	// Build response with all services
	result := make([]fiber.Map, len(services))
	for i, svc := range services {
		limit, hasLimit := limitMap[svc.ID]
		maxSubs := 0
		if hasLimit {
			maxSubs = limit.MaxSubscribers
		}
		result[i] = fiber.Map{
			"service_id":      svc.ID,
			"service_name":    svc.Name,
			"max_subscribers": maxSubs,
			"current_count":   countMap[svc.ID],
		}
	}

	return c.JSON(fiber.Map{"success": true, "data": result})
}

// SetServiceLimits sets per-service subscriber limits for a reseller
func (h *ResellerHandler) SetServiceLimits(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid reseller ID"})
	}

	var req struct {
		Limits []struct {
			ServiceID      uint `json:"service_id"`
			MaxSubscribers int  `json:"max_subscribers"`
		} `json:"limits"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	// Delete all existing limits for this reseller
	database.DB.Where("reseller_id = ?", id).Delete(&models.ResellerServiceLimit{})

	// Insert new limits (skip 0 = unlimited)
	for _, l := range req.Limits {
		if l.MaxSubscribers > 0 {
			database.DB.Create(&models.ResellerServiceLimit{
				ResellerID:     uint(id),
				ServiceID:      l.ServiceID,
				MaxSubscribers: l.MaxSubscribers,
			})
		}
	}

	// Audit log
	user := middleware.GetCurrentUser(c)
	database.DB.Create(&models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionUpdate,
		EntityType:  "reseller",
		EntityID:    uint(id),
		Description: "Updated service subscriber limits",
		IPAddress:   c.IP(),
	})

	return c.JSON(fiber.Map{"success": true, "message": "Service limits updated successfully"})
}

// GetSelfWanSettings returns the current reseller's own WAN check settings
func (h *ResellerHandler) GetSelfWanSettings(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil || user.ResellerID == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}
	if !checkUserPermission(user, "settings.wan_check") {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": "Permission denied"})
	}

	var reseller models.Reseller
	if err := database.DB.Select("id, wan_check_enabled, wan_check_icmp, wan_check_port").
		First(&reseller, *user.ResellerID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Reseller not found"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"wan_check_enabled": reseller.WanCheckEnabled,
			"wan_check_icmp":    reseller.WanCheckICMP,
			"wan_check_port":    reseller.WanCheckPort,
		},
	})
}

// UpdateSelfWanSettings allows a reseller to update their own WAN check settings
func (h *ResellerHandler) UpdateSelfWanSettings(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil || user.ResellerID == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}
	if !checkUserPermission(user, "settings.wan_check") {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": "Permission denied"})
	}

	var req map[string]interface{}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	updates := make(map[string]interface{})
	if val, ok := req["wan_check_enabled"]; ok {
		if val == nil {
			updates["wan_check_enabled"] = nil
		} else if v, ok := val.(bool); ok {
			updates["wan_check_enabled"] = v
		}
	}
	if val, ok := req["wan_check_icmp"]; ok {
		if v, ok := val.(bool); ok {
			updates["wan_check_icmp"] = v
		}
	}
	if val, ok := req["wan_check_port"]; ok {
		if v, ok := val.(bool); ok {
			updates["wan_check_port"] = v
		}
	}

	if len(updates) > 0 {
		database.DB.Table("resellers").Where("id = ?", *user.ResellerID).Updates(updates)
	}

	return c.JSON(fiber.Map{"success": true, "message": "WAN check settings updated"})
}

// GetSubResellerServiceLimits returns per-service subscriber limits for a sub-reseller (reseller context)
func (h *ResellerHandler) GetSubResellerServiceLimits(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil || user.ResellerID == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	subID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid reseller ID"})
	}

	// Verify sub-reseller belongs to this reseller
	var subReseller models.Reseller
	if err := database.DB.First(&subReseller, subID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Sub-reseller not found"})
	}
	if subReseller.ParentID == nil || *subReseller.ParentID != *user.ResellerID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": "Not your sub-reseller"})
	}

	// Reuse admin handler logic with the sub-reseller ID
	c.Locals("override_id", subID)
	return h.GetServiceLimits(c)
}

// SetSubResellerServiceLimits sets per-service subscriber limits for a sub-reseller (reseller context)
func (h *ResellerHandler) SetSubResellerServiceLimits(c *fiber.Ctx) error {
	user := middleware.GetCurrentUser(c)
	if user == nil || user.ResellerID == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Unauthorized"})
	}

	subID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid reseller ID"})
	}

	// Verify sub-reseller belongs to this reseller
	var subReseller models.Reseller
	if err := database.DB.First(&subReseller, subID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "message": "Sub-reseller not found"})
	}
	if subReseller.ParentID == nil || *subReseller.ParentID != *user.ResellerID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": "Not your sub-reseller"})
	}

	var req struct {
		Limits []struct {
			ServiceID      uint `json:"service_id"`
			MaxSubscribers int  `json:"max_subscribers"`
		} `json:"limits"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	// Load parent reseller's limits to enforce ceiling
	var parentLimits []models.ResellerServiceLimit
	database.DB.Where("reseller_id = ?", *user.ResellerID).Find(&parentLimits)
	parentLimitMap := make(map[uint]int)
	for _, pl := range parentLimits {
		parentLimitMap[pl.ServiceID] = pl.MaxSubscribers
	}

	// Validate: sub-reseller limit cannot exceed parent's limit
	for _, l := range req.Limits {
		if l.MaxSubscribers > 0 {
			parentMax, hasParentLimit := parentLimitMap[l.ServiceID]
			if hasParentLimit && parentMax > 0 && l.MaxSubscribers > parentMax {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"message": fmt.Sprintf("Cannot set limit higher than your own limit (%d) for this service", parentMax),
				})
			}
		}
	}

	// Delete all existing limits for the sub-reseller
	database.DB.Where("reseller_id = ?", subID).Delete(&models.ResellerServiceLimit{})

	// Insert new limits
	for _, l := range req.Limits {
		if l.MaxSubscribers > 0 {
			database.DB.Create(&models.ResellerServiceLimit{
				ResellerID:     uint(subID),
				ServiceID:      l.ServiceID,
				MaxSubscribers: l.MaxSubscribers,
			})
		}
	}

	// Audit log
	database.DB.Create(&models.AuditLog{
		UserID:      user.ID,
		Username:    user.Username,
		UserType:    user.UserType,
		Action:      models.AuditActionUpdate,
		EntityType:  "reseller",
		EntityID:    uint(subID),
		Description: "Updated sub-reseller service subscriber limits",
		IPAddress:   c.IP(),
	})

	return c.JSON(fiber.Map{"success": true, "message": "Service limits updated successfully"})
}
