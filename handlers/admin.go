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

type UpdateProductRequest struct {
	SKU         *string   `json:"sku"`
	Name        *string   `json:"name"`
	Description *string   `json:"description"`
	Price       *float64  `json:"price"`
	Stock       *int      `json:"stock"`
	Images      *[]string `json:"images"`
}

func CreateProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var product models.Product
		if err := bindStrictJSON(c, &product); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Validation
		if product.SKU == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Product SKU is required"})
			return
		}
		if product.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Product name is required"})
			return
		}
		if product.Price <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Product price must be greater than 0"})
			return
		}

		// Check SKU uniqueness
		var existingProduct models.Product
		if err := db.Where("sku = ?", product.SKU).First(&existingProduct).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Product with this SKU already exists"})
			return
		} else if err != gorm.ErrRecordNotFound {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check SKU uniqueness"})
			return
		}

		// Set default stock if not provided
		if product.Stock < 0 {
			product.Stock = 0
		}

		if err := db.Create(&product).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product"})
			return
		}

		c.JSON(http.StatusCreated, product)
	}
}

func UpdateProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var product models.Product

		// Find the product
		if err := db.First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		// Bind update data
		var req UpdateProductRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		updates := make(map[string]any)

		if req.SKU != nil {
			sku := strings.TrimSpace(*req.SKU)
			if sku == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Product SKU is required"})
				return
			}
			if sku != product.SKU {
				var existingProduct models.Product
				if err := db.Where("sku = ? AND id <> ?", sku, product.ID).First(&existingProduct).Error; err == nil {
					c.JSON(http.StatusConflict, gin.H{"error": "Product with this SKU already exists"})
					return
				} else if !errors.Is(err, gorm.ErrRecordNotFound) {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check SKU uniqueness"})
					return
				}
			}
			updates["sku"] = sku
		}

		if req.Name != nil {
			name := strings.TrimSpace(*req.Name)
			if name == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Product name is required"})
				return
			}
			updates["name"] = name
		}

		if req.Description != nil {
			updates["description"] = strings.TrimSpace(*req.Description)
		}

		if req.Price != nil {
			if *req.Price <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Product price must be greater than 0"})
				return
			}
			updates["price"] = models.MoneyFromFloat(*req.Price)
		}

		if req.Stock != nil {
			stock := *req.Stock
			if stock < 0 {
				stock = 0
			}
			updates["stock"] = stock
		}

		if req.Images != nil {
			updates["images"] = *req.Images
		}

		if len(updates) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
			return
		}

		// Update product
		if err := db.Model(&product).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product"})
			return
		}
		if err := db.First(&product, id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load updated product"})
			return
		}

		c.JSON(http.StatusOK, product)
	}
}

type UpdateRelatedRequest struct {
	RelatedIDs []uint `json:"related_ids"`
}

func UpdateProductRelated(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var product models.Product

		if err := db.Preload("Related").First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		var req UpdateRelatedRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		for _, relatedID := range req.RelatedIDs {
			if relatedID == product.ID {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Product cannot be related to itself"})
				return
			}
		}

		var related []models.Product
		if len(req.RelatedIDs) > 0 {
			if err := db.Where("id IN ?", req.RelatedIDs).Find(&related).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load related products"})
				return
			}
		}

		if err := db.Model(&product).Association("Related").Replace(related); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update related products"})
			return
		}

		product.Related = related
		if mediaService != nil {
			mediaURLs, err := mediaService.ProductMediaURLs(product.ID)
			if err == nil && len(mediaURLs) > 0 {
				product.Images = mediaURLs
			}
		}

		c.JSON(http.StatusOK, product)
	}
}

func DeleteProduct(db *gorm.DB, mediaService *media.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var product models.Product

		// Find the product
		if err := db.First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		var refs []models.MediaReference
		if mediaService != nil {
			if err := db.Where("owner_type = ? AND owner_id = ? AND role = ?",
				media.OwnerTypeProduct, product.ID, media.RoleProductImage).Find(&refs).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load product media"})
				return
			}
		}

		// Delete the product
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Delete(&product).Error; err != nil {
				return err
			}
			if mediaService != nil {
				if err := tx.Where("owner_type = ? AND owner_id = ? AND role = ?",
					media.OwnerTypeProduct, product.ID, media.RoleProductImage).Delete(&models.MediaReference{}).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete product"})
			return
		}

		if mediaService != nil {
			for _, ref := range refs {
				_ = mediaService.DeleteIfOrphan(ref.MediaID)
			}
		}

		c.JSON(http.StatusOK, gin.H{"message": "Product deleted successfully"})
	}
}
