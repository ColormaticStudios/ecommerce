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
	ProductID uint `json:"product_id" binding:"required"`
	Quantity  int  `json:"quantity" binding:"required,min=1"`
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

		requestedByProduct := make(map[uint]int, len(req.Items))
		orderedProductIDs := make([]uint, 0, len(req.Items))
		for _, itemReq := range req.Items {
			if _, exists := requestedByProduct[itemReq.ProductID]; !exists {
				orderedProductIDs = append(orderedProductIDs, itemReq.ProductID)
			}
			requestedByProduct[itemReq.ProductID] += itemReq.Quantity
		}

		var total models.Money
		orderItems := make([]models.OrderItem, 0, len(orderedProductIDs))
		for _, productID := range orderedProductIDs {
			quantity := requestedByProduct[productID]
			var product models.Product
			if err := db.First(&product, productID).Error; err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Product not found", "product_id": productID})
				return
			}
			if !productIsPubliclyVisible(product) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Product not found", "product_id": productID})
				return
			}

			if product.Stock < quantity {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":        "Insufficient stock",
					"product_id":   productID,
					"product_name": product.Name,
					"requested":    quantity,
					"available":    product.Stock,
				})
				return
			}

			total += product.Price.Mul(quantity)
			orderItems = append(orderItems, models.OrderItem{
				ProductID: productID,
				Quantity:  quantity,
				Price:     product.Price,
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

		db.Preload("Items.Product").First(&order, order.ID)
		applyOrderMediaToOrder(&order, mediaService)
		applyOrderCapabilities(&order, &user.ID)

		c.JSON(http.StatusCreated, order)
	}
}
