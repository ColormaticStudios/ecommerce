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

type MediaOrderRequest struct {
	MediaIDs []string `json:"media_ids" binding:"required"`
}

func SetProfilePhoto(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUserWithNotFound(db, c)
		if !ok {
			return
		}

		var req MediaAttachRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		mediaObj, err := mediaService.WaitUntilReady(req.MediaID, mediaReadyTimeout)
		if err != nil {
			if writeStatusError(c, mediaLookupStatusError(err, "Media not found", "Failed to load media", "Media processing failed", "Media is still processing")) {
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load media"})
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
		user, ok := getAuthenticatedUserWithNotFound(db, c)
		if !ok {
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
		if err := db.Preload("Related").First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		var req MediaAttachManyRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := AttachProductMediaToDraft(db, mediaService, &product, req.MediaIDs); err != nil {
			if writeStatusError(c, err) {
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to attach media"})
			return
		}

		respondAdminProduct(c, db, mediaService, product.ID)
	}
}

func UpdateProductMediaOrder(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var product models.Product
		if err := db.Preload("Related").First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		var req MediaOrderRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if len(req.MediaIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Media IDs required"})
			return
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			_, err := ensureProductCatalogDraft(tx, &product)
			return err
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize product draft"})
			return
		}

		var refs []models.MediaReference
		if err := db.Where("owner_type = ? AND owner_id = ? AND role = ?",
			media.OwnerTypeProduct, product.ID, media.RoleProductDraftImage).
			Find(&refs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load product media"})
			return
		}

		refByID := make(map[string]models.MediaReference, len(refs))
		for _, ref := range refs {
			refByID[ref.MediaID] = ref
		}

		for _, mediaID := range req.MediaIDs {
			if _, ok := refByID[mediaID]; !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Media not attached: " + mediaID})
				return
			}
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			for index, mediaID := range req.MediaIDs {
				if err := tx.Model(&models.MediaReference{}).
					Where("media_id = ? AND owner_type = ? AND owner_id = ? AND role = ?",
						mediaID, media.OwnerTypeProduct, product.ID, media.RoleProductDraftImage).
					Update("position", index+1).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update media order"})
			return
		}

		respondAdminProduct(c, db, mediaService, product.ID)
	}
}

func DetachProductMedia(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		mediaID := c.Param("mediaId")

		var product models.Product
		if err := db.Preload("Related").First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			_, err := ensureProductCatalogDraft(tx, &product)
			return err
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize product draft"})
			return
		}

		if err := db.Where("media_id = ? AND owner_type = ? AND owner_id = ? AND role = ?",
			mediaID, media.OwnerTypeProduct, product.ID, media.RoleProductDraftImage).Delete(&models.MediaReference{}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to detach media"})
			return
		}

		_ = mediaService.DeleteIfOrphan(mediaID)
		respondAdminProduct(c, db, mediaService, product.ID)
	}
}
