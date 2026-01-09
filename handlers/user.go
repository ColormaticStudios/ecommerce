package handlers

import (
	"net/http"

	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetProfile retrieves the authenticated user's profile
func GetProfile(db *gorm.DB) gin.HandlerFunc {
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
		var users []models.User
		if err := db.Find(&users).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
			return
		}

		c.JSON(http.StatusOK, users)
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
