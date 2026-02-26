package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

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

		var mediaObj models.MediaObject
		deadline := time.Now().Add(2 * time.Second)
		for {
			if err := db.First(&mediaObj, "id = ?", req.MediaID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) && time.Now().Before(deadline) {
					time.Sleep(150 * time.Millisecond)
					continue
				}
				if errors.Is(err, gorm.ErrRecordNotFound) {
					c.JSON(http.StatusNotFound, gin.H{"error": "Media not found"})
				} else {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load media"})
				}
				return
			}
			break
		}

		if mediaObj.Status != media.StatusReady || mediaObj.OriginalPath == "" {
			if mediaObj.Status == media.StatusFailed {
				c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Media processing failed"})
				return
			}

			processingDeadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(processingDeadline) {
				time.Sleep(150 * time.Millisecond)
				if err := db.First(&mediaObj, "id = ?", req.MediaID).Error; err != nil {
					c.JSON(http.StatusNotFound, gin.H{"error": "Media not found"})
					return
				}
				if mediaObj.Status == media.StatusReady && mediaObj.OriginalPath != "" {
					break
				}
				if mediaObj.Status == media.StatusFailed {
					c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Media processing failed"})
					return
				}
			}

			if mediaObj.Status != media.StatusReady || mediaObj.OriginalPath == "" {
				c.JSON(http.StatusConflict, gin.H{"error": "Media is still processing"})
				return
			}
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

		if err := db.Transaction(func(tx *gorm.DB) error {
			_, _, err := ensureProductDraft(tx, &product)
			return err
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize product draft"})
			return
		}

		var maxPosition int
		if err := db.Model(&models.MediaReference{}).
			Where("owner_type = ? AND owner_id = ? AND role = ?",
				media.OwnerTypeProduct, product.ID, media.RoleProductDraftImage).
			Select("COALESCE(MAX(position), 0)").
			Scan(&maxPosition).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load media order"})
			return
		}

		for _, mediaID := range req.MediaIDs {
			var mediaObj models.MediaObject
			mediaDeadline := time.Now().Add(2 * time.Second)
			for {
				if err := db.First(&mediaObj, "id = ?", mediaID).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) && time.Now().Before(mediaDeadline) {
						time.Sleep(150 * time.Millisecond)
						continue
					}
					if errors.Is(err, gorm.ErrRecordNotFound) {
						c.JSON(http.StatusNotFound, gin.H{"error": "Media not found: " + mediaID})
					} else {
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load media: " + mediaID})
					}
					return
				}
				break
			}
			if mediaObj.Status != media.StatusReady || mediaObj.OriginalPath == "" {
				if mediaObj.Status == media.StatusFailed {
					c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Media processing failed: " + mediaID})
					return
				}

				processingDeadline := time.Now().Add(2 * time.Second)
				for time.Now().Before(processingDeadline) {
					time.Sleep(150 * time.Millisecond)
					if err := db.First(&mediaObj, "id = ?", mediaID).Error; err != nil {
						c.JSON(http.StatusNotFound, gin.H{"error": "Media not found: " + mediaID})
						return
					}
					if mediaObj.Status == media.StatusReady && mediaObj.OriginalPath != "" {
						break
					}
					if mediaObj.Status == media.StatusFailed {
						c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Media processing failed: " + mediaID})
						return
					}
				}

				if mediaObj.Status != media.StatusReady || mediaObj.OriginalPath == "" {
					c.JSON(http.StatusConflict, gin.H{"error": "Media is still processing: " + mediaID})
					return
				}
			}

			maxPosition++
			ref := models.MediaReference{
				MediaID:   mediaID,
				OwnerType: media.OwnerTypeProduct,
				OwnerID:   product.ID,
				Role:      media.RoleProductDraftImage,
				Position:  maxPosition,
			}
			if err := db.Where("media_id = ? AND owner_type = ? AND owner_id = ? AND role = ?",
				ref.MediaID, ref.OwnerType, ref.OwnerID, ref.Role).FirstOrCreate(&ref).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to attach media"})
				return
			}
		}

		view, err := materializeAdminProduct(db, mediaService, product, true)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load updated product"})
			return
		}
		c.JSON(http.StatusOK, toContractProduct(view))
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
			_, _, err := ensureProductDraft(tx, &product)
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

		view, err := materializeAdminProduct(db, mediaService, product, true)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load updated product"})
			return
		}
		c.JSON(http.StatusOK, toContractProduct(view))
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
			_, _, err := ensureProductDraft(tx, &product)
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
		view, err := materializeAdminProduct(db, mediaService, product, true)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load updated product"})
			return
		}
		c.JSON(http.StatusOK, toContractProduct(view))
	}
}
