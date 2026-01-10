package handlers

import (
	"net/http"
	"strings"

	"ecommerce/internal/media"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MediaAttachRequest struct {
	MediaID string `json:"media_id" binding:"required"`
}

type MediaAttachManyRequest struct {
	MediaIDs []string `json:"media_ids" binding:"required"`
}

func SetProfilePhoto(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
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

		var req MediaAttachRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var mediaObj models.MediaObject
		if err := db.First(&mediaObj, "id = ?", req.MediaID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Media not found"})
			return
		}

		if mediaObj.Status != media.StatusReady {
			c.JSON(http.StatusConflict, gin.H{"error": "Media is still processing"})
			return
		}

		if !strings.HasPrefix(mediaObj.MimeType, "image/") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Profile photo must be an image"})
			return
		}

		if mediaObj.SizeBytes > media.DefaultProfilePhotoMaxBytes {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "Profile photo is too large"})
			return
		}

		var existing []models.MediaReference
		if err := db.Where("owner_type = ? AND owner_id = ? AND role = ?",
			media.OwnerTypeUser, user.ID, media.RoleProfilePhoto).Find(&existing).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read existing profile photo"})
			return
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?",
				media.OwnerTypeUser, user.ID, media.RoleProfilePhoto).Delete(&models.MediaReference{}).Error; err != nil {
				return err
			}

			return tx.Create(&models.MediaReference{
				MediaID:   req.MediaID,
				OwnerType: media.OwnerTypeUser,
				OwnerID:   user.ID,
				Role:      media.RoleProfilePhoto,
			}).Error
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile photo"})
			return
		}

		for _, ref := range existing {
			if ref.MediaID == req.MediaID {
				continue
			}
			_ = mediaService.DeleteIfOrphan(ref.MediaID)
		}

		profileURL, err := mediaService.UserProfilePhotoURL(user.ID)
		if err == nil {
			user.ProfilePhoto = profileURL
		}
		c.JSON(http.StatusOK, user)
	}
}

func DeleteProfilePhoto(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
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

		var refs []models.MediaReference
		if err := db.Where("owner_type = ? AND owner_id = ? AND role = ?",
			media.OwnerTypeUser, user.ID, media.RoleProfilePhoto).Find(&refs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read profile photo"})
			return
		}

		if err := db.Where("owner_type = ? AND owner_id = ? AND role = ?",
			media.OwnerTypeUser, user.ID, media.RoleProfilePhoto).Delete(&models.MediaReference{}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove profile photo"})
			return
		}

		for _, ref := range refs {
			_ = mediaService.DeleteIfOrphan(ref.MediaID)
		}

		user.ProfilePhoto = ""
		c.JSON(http.StatusOK, user)
	}
}

func AttachProductMedia(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var product models.Product
		if err := db.First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		var req MediaAttachManyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		for _, mediaID := range req.MediaIDs {
			var mediaObj models.MediaObject
			if err := db.First(&mediaObj, "id = ?", mediaID).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Media not found: " + mediaID})
				return
			}
			if mediaObj.Status != media.StatusReady {
				c.JSON(http.StatusConflict, gin.H{"error": "Media is still processing: " + mediaID})
				return
			}

			ref := models.MediaReference{
				MediaID:   mediaID,
				OwnerType: media.OwnerTypeProduct,
				OwnerID:   product.ID,
				Role:      media.RoleProductImage,
			}
			if err := db.Where("media_id = ? AND owner_type = ? AND owner_id = ? AND role = ?",
				ref.MediaID, ref.OwnerType, ref.OwnerID, ref.Role).FirstOrCreate(&ref).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to attach media"})
				return
			}
		}

		productImages, _ := mediaService.ProductMediaURLs(product.ID)
		product.Images = productImages
		c.JSON(http.StatusOK, product)
	}
}

func DetachProductMedia(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		mediaID := c.Param("mediaId")

		var product models.Product
		if err := db.First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		if err := db.Where("media_id = ? AND owner_type = ? AND owner_id = ? AND role = ?",
			mediaID, media.OwnerTypeProduct, product.ID, media.RoleProductImage).Delete(&models.MediaReference{}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to detach media"})
			return
		}

		_ = mediaService.DeleteIfOrphan(mediaID)
		productImages, _ := mediaService.ProductMediaURLs(product.ID)
		product.Images = productImages
		c.JSON(http.StatusOK, product)
	}
}
