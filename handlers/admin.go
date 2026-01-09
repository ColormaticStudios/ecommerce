package handlers

import (
	"net/http"

	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func CreateProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var product models.Product
		if err := c.ShouldBindJSON(&product); err != nil {
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
		var updateData models.Product
		if err := c.ShouldBindJSON(&updateData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Validate price if provided
		if updateData.Price != 0 && updateData.Price <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Product price must be greater than 0"})
			return
		}

		// Update product
		if err := db.Model(&product).Updates(updateData).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product"})
			return
		}

		c.JSON(http.StatusOK, product)
	}
}

func DeleteProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var product models.Product

		// Find the product
		if err := db.First(&product, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		}

		// Delete the product
		if err := db.Delete(&product).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete product"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Product deleted successfully"})
	}
}
