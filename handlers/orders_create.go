package handlers

import (
	"net/http"

	"ecommerce/internal/media"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CreateOrderRequest struct {
	Items []OrderItemRequest `json:"items" binding:"required"`
}

type OrderItemRequest struct {
	ProductVariantID uint `json:"product_variant_id" binding:"required"`
	Quantity         int  `json:"quantity" binding:"required,min=1"`
}

// CreateOrder creates a new order for the authenticated user.
func CreateOrder(db *gorm.DB, mediaServices ...*media.Service) gin.HandlerFunc {
	mediaService := resolveMediaService(mediaServices...)
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		var req CreateOrderRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if len(req.Items) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Order must contain at least one item"})
			return
		}

		requestedByVariant := make(map[uint]int, len(req.Items))
		orderedVariantIDs := make([]uint, 0, len(req.Items))
		for _, itemReq := range req.Items {
			if _, exists := requestedByVariant[itemReq.ProductVariantID]; !exists {
				orderedVariantIDs = append(orderedVariantIDs, itemReq.ProductVariantID)
			}
			requestedByVariant[itemReq.ProductVariantID] += itemReq.Quantity
		}

		var total models.Money
		orderItems := make([]models.OrderItem, 0, len(orderedVariantIDs))
		for _, variantID := range orderedVariantIDs {
			quantity := requestedByVariant[variantID]
			var variant models.ProductVariant
			if err := db.Preload("Product").First(&variant, variantID).Error; err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Product variant not found", "product_variant_id": variantID})
				return
			}
			if !variant.IsPublished || !productIsPubliclyVisible(variant.Product) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Product variant not found", "product_variant_id": variantID})
				return
			}

			if variant.Stock < quantity {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":              "Insufficient stock",
					"product_variant_id": variantID,
					"product_name":       variant.Product.Name,
					"requested":          quantity,
					"available":          variant.Stock,
				})
				return
			}

			total += variant.Price.Mul(quantity)
			orderItems = append(orderItems, models.OrderItem{
				ProductVariantID: variantID,
				VariantSKU:       variant.SKU,
				VariantTitle:     variant.Title,
				Quantity:         quantity,
				Price:            variant.Price,
			})
		}

		order := models.Order{
			UserID: user.ID,
			Total:  total,
			Status: models.StatusPending,
			Items:  orderItems,
		}

		if err := db.Create(&order).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
			return
		}

		db.Preload("Items.ProductVariant").Preload("Items.ProductVariant.Product").First(&order, order.ID)
		applyOrderMediaToOrder(&order, mediaService)
		applyOrderCapabilities(&order, &user.ID)
		response, err := buildOrderResponse(db, mediaService, order)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render order"})
			return
		}

		c.JSON(http.StatusCreated, response)
	}
}
