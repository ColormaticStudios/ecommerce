package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"ecommerce/internal/media"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetProfile retrieves the authenticated user's profile
func GetProfile(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// The userID is the OIDC Subject claim stored in the middleware
		subject := c.GetString("userID")
		if subject == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
			return
		}

		var user models.User
		if err := db.Where("subject = ?", subject).First(&user).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User profile not found"})
			return
		}

		if mediaService != nil {
			profileURL, err := mediaService.UserProfilePhotoURL(user.ID)
			if err == nil {
				user.ProfilePhoto = profileURL
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				mediaService.Logger.Printf("[WARN] Failed to load profile photo: %v", err)
			}
		}

		c.JSON(http.StatusOK, user)
	}
}

type UpdateProfileRequest struct {
	Name         string `json:"name"`
	Currency     string `json:"currency" binding:"omitempty,len=3"`
	ProfilePhoto string `json:"profile_photo_url"`
}

// UpdateProfile updates the authenticated user's profile
func UpdateProfile(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user subject from middleware
		subject := c.GetString("userID")
		if subject == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
			return
		}

		// Find user by subject
		var user models.User
		if err := db.Where("subject = ?", subject).First(&user).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User profile not found"})
			return
		}

		// Parse request
		var req UpdateProfileRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Validate currency code if provided
		validCurrencies := map[string]bool{
			"USD": true, "EUR": true, "GBP": true, "JPY": true, "CAD": true,
			"AUD": true, "CHF": true, "CNY": true, "INR": true, "BRL": true,
		}
		if req.Currency != "" && !validCurrencies[req.Currency] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid currency code"})
			return
		}

		// Update fields
		if req.Name != "" {
			user.Name = req.Name
		}
		if req.Currency != "" {
			user.Currency = req.Currency
		}
		if req.ProfilePhoto != "" {
			user.ProfilePhoto = req.ProfilePhoto
		}

		// Save changes
		if err := db.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
			return
		}

		c.JSON(http.StatusOK, user)
	}
}

// GetAllUsers retrieves all users (admin only)
func GetAllUsers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		if page < 1 {
			page = 1
		}
		if limit < 1 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}
		offset := (page - 1) * limit

		query := db.Model(&models.User{})
		var total int64
		if err := query.Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
			return
		}

		var users []models.User
		if err := query.Offset(offset).Limit(limit).Find(&users).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
			return
		}

		totalPages := int(total) / limit
		if int(total)%limit > 0 {
			totalPages++
		}

		c.JSON(http.StatusOK, gin.H{
			"data": users,
			"pagination": gin.H{
				"page":        page,
				"limit":       limit,
				"total":       total,
				"total_pages": totalPages,
			},
		})
	}
}

type UpdateUserRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=admin customer"`
}

// UpdateUserRole updates a user's role (admin only)
func UpdateUserRole(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("id")

		// Find user
		var user models.User
		if err := db.First(&user, userID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		// Parse request
		var req UpdateUserRoleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Update role
		user.Role = req.Role
		if err := db.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user role"})
			return
		}

		c.JSON(http.StatusOK, user)
	}
}
