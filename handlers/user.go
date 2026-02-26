package handlers

import (
	"errors"
	"net/http"
	"strings"

	"ecommerce/internal/media"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetProfile retrieves the authenticated user's profile
func GetProfile(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUserWithNotFound(db, c)
		if !ok {
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
		user, ok := getAuthenticatedUserWithNotFound(db, c)
		if !ok {
			return
		}

		// Parse request
		var req UpdateProfileRequest
		if err := bindStrictJSON(c, &req); err != nil {
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
		if err := db.Save(user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
			return
		}

		c.JSON(http.StatusOK, user)
	}
}

// GetAllUsers retrieves all users (admin only)
func GetAllUsers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, limit, offset := parsePagination(c, 10)
		searchTerm := strings.TrimSpace(c.Query("q"))

		query := db.Model(&models.User{})
		if searchTerm != "" {
			like := "%" + strings.ToLower(searchTerm) + "%"
			query = query.Where(
				`CAST(id AS TEXT) LIKE ? OR
				 LOWER(username) LIKE ? OR
				 LOWER(email) LIKE ? OR
				 LOWER(COALESCE(name, '')) LIKE ? OR
				 LOWER(subject) LIKE ? OR
				 LOWER(role) LIKE ?`,
				like, like, like, like, like, like,
			)
		}
		var total int64
		if err := query.Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
			return
		}

		var users []models.User
		if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&users).Error; err != nil {
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
		if err := bindStrictJSON(c, &req); err != nil {
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
