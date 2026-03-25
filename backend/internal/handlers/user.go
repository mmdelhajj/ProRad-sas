package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/proisp/backend/internal/database"
	"github.com/proisp/backend/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// getMinPasswordLength gets minimum password length from settings
func getMinPasswordLength() int {
	var pref models.SystemPreference
	if err := database.DB.Where("key = ?", "password_min_length").First(&pref).Error; err != nil {
		return 8 // Default
	}
	if val, err := strconv.Atoi(pref.Value); err == nil && val > 0 {
		return val
	}
	return 8
}

type UserHandler struct{}

func NewUserHandler() *UserHandler {
	return &UserHandler{}
}

// List returns all admin users (non-subscribers)
func (h *UserHandler) List(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 25)
	search := c.Query("search", "")
	userType := c.QueryInt("user_type", 0)

	if page < 1 {
		page = 1
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	query := database.DB.Model(&models.User{}).Where("user_type >= ?", models.UserTypeReseller)

	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("username ILIKE ? OR email ILIKE ? OR full_name ILIKE ?",
			searchPattern, searchPattern, searchPattern)
	}

	if userType > 0 {
		query = query.Where("user_type = ?", userType)
	}

	var total int64
	query.Count(&total)

	var users []models.User
	query.Preload("Reseller").Order("created_at DESC").Offset(offset).Limit(limit).Find(&users)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    users,
		"meta": fiber.Map{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// Get returns a single user
func (h *UserHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")

	var user models.User
	if err := database.DB.Preload("Reseller").First(&user, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "User not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    user,
	})
}

// Create creates a new admin user
func (h *UserHandler) Create(c *fiber.Ctx) error {
	type CreateRequest struct {
		Username        string           `json:"username"`
		Password        string           `json:"password"`
		Email           string           `json:"email"`
		Phone           string           `json:"phone"`
		FullName        string           `json:"full_name"`
		UserType        models.UserType  `json:"user_type"`
		IsActive        bool             `json:"is_active"`
		PermissionGroup *uint            `json:"permission_group"`
	}

	var req CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Validate user type
	if req.UserType < models.UserTypeReseller {
		req.UserType = models.UserTypeSupport
	}

	// Validate password length
	minLen := getMinPasswordLength()
	if len(req.Password) < minLen {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Password must be at least " + strconv.Itoa(minLen) + " characters",
		})
	}

	// Check if username exists (including soft-deleted to prevent conflicts)
	var exists int64
	database.DB.Unscoped().Model(&models.User{}).Where("username = ?", req.Username).Count(&exists)
	if exists > 0 {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"success": false,
			"message": "Username already exists",
		})
	}

	// Hash password
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

	user := models.User{
		Username:        req.Username,
		Password:        string(hashedPassword),
		Email:           req.Email,
		Phone:           req.Phone,
		FullName:        req.FullName,
		UserType:        req.UserType,
		IsActive:        req.IsActive,
		PermissionGroup: req.PermissionGroup,
	}

	if err := database.DB.Create(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create user",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    user,
	})
}

// Update updates an admin user
func (h *UserHandler) Update(c *fiber.Ctx) error {
	id := c.Params("id")

	var user models.User
	if err := database.DB.First(&user, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "User not found",
		})
	}

	type UpdateRequest struct {
		Password        string           `json:"password"`
		Email           string           `json:"email"`
		Phone           string           `json:"phone"`
		FullName        string           `json:"full_name"`
		UserType        models.UserType  `json:"user_type"`
		IsActive        bool             `json:"is_active"`
		PermissionGroup *uint            `json:"permission_group"`
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	updates := map[string]interface{}{
		"email":            req.Email,
		"phone":            req.Phone,
		"full_name":        req.FullName,
		"user_type":        req.UserType,
		"is_active":        req.IsActive,
		"permission_group": req.PermissionGroup,
	}

	if req.Password != "" {
		minLen := getMinPasswordLength()
		if len(req.Password) < minLen {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "Password must be at least " + strconv.Itoa(minLen) + " characters",
			})
		}
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		updates["password"] = string(hashedPassword)
	}

	database.DB.Model(&user).Updates(updates)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    user,
	})
}

// Delete deletes an admin user
func (h *UserHandler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")

	var user models.User
	if err := database.DB.First(&user, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "User not found",
		})
	}

	// Don't allow deleting self or last admin
	var adminCount int64
	database.DB.Model(&models.User{}).Where("user_type = ?", models.UserTypeAdmin).Count(&adminCount)
	if user.UserType == models.UserTypeAdmin && adminCount <= 1 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Cannot delete the last admin user",
		})
	}

	database.DB.Delete(&user)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "User deleted",
	})
}

// UpdateLastLogin updates user's last login time
func UpdateLastLogin(userID uint) {
	now := time.Now()
	database.DB.Model(&models.User{}).Where("id = ?", userID).Update("last_login", &now)
}
