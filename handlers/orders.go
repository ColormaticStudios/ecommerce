package handlers

import (
	"errors"
	"net/http"
	"strconv"

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

// CreateOrder creates a new order for the authenticated user
func CreateOrder(db *gorm.DB) gin.HandlerFunc {
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
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		// Parse request
		var req CreateOrderRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if len(req.Items) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Order must contain at least one item"})
			return
		}

		// Calculate total and validate products
		var total float64
		var orderItems []models.OrderItem

		for _, itemReq := range req.Items {
			// Get product
			var product models.Product
			if err := db.First(&product, itemReq.ProductID).Error; err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Product not found", "product_id": itemReq.ProductID})
				return
			}

			// Check stock availability
			if product.Stock < itemReq.Quantity {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":        "Insufficient stock",
					"product_id":   itemReq.ProductID,
					"product_name": product.Name,
					"requested":    itemReq.Quantity,
					"available":    product.Stock,
				})
				return
			}

			// Calculate item total
			itemTotal := product.Price * float64(itemReq.Quantity)
			total += itemTotal

			// Create order item
			orderItem := models.OrderItem{
				ProductID: itemReq.ProductID,
				Quantity:  itemReq.Quantity,
				Price:     product.Price, // Snapshot price at time of order
			}
			orderItems = append(orderItems, orderItem)
		}

		// Create order
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

		// Preload related data for response
		db.Preload("Items.Product").First(&order, order.ID)

		c.JSON(http.StatusCreated, order)
	}
}

// GetUserOrders retrieves all orders for the authenticated user
func GetUserOrders(db *gorm.DB) gin.HandlerFunc {
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
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		// Get orders
		var orders []models.Order
		if err := db.Where("user_id = ?", user.ID).
			Preload("Items.Product").
			Order("created_at DESC").
			Find(&orders).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
			return
		}

		c.JSON(http.StatusOK, orders)
	}
}

// GetOrderByID retrieves a specific order by ID (only if it belongs to the user)
func GetOrderByID(db *gorm.DB) gin.HandlerFunc {
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
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		// Get order ID from URL
		orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}

		// Get order (only if it belongs to the user)
		var order models.Order
		if err := db.Where("id = ? AND user_id = ?", orderID, user.ID).
			Preload("Items.Product").
			First(&order).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}

		c.JSON(http.StatusOK, order)
	}
}

type UpdateOrderStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=PENDING PAID FAILED"`
}

// ProcessPayment processes payment for an order (mock implementation)
func ProcessPayment(db *gorm.DB) gin.HandlerFunc {
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
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}

		// Get order ID from URL
		orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}

		// Get order (only if it belongs to the user)
		var order models.Order
		if err := db.Where("id = ? AND user_id = ?", orderID, user.ID).
			Preload("Items.Product").
			First(&order).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}

		// Check if order is already paid
		if order.Status == models.StatusPaid {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Order is already paid"})
			return
		}

		// Mock payment processing - in a real app, this would call a payment provider
		// For now, we'll simulate a successful payment
		// In production, you'd validate payment details, process with Stripe/PayPal/etc.

		// Start transaction
		tx := db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		// Update order status
		order.Status = models.StatusPaid
		if err := tx.Save(&order).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
			return
		}

		// Deduct stock from products
		for _, item := range order.Items {
			var product models.Product
			if err := tx.First(&product, item.ProductID).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product stock"})
				return
			}

			// Check if stock is still available
			if product.Stock < item.Quantity {
				tx.Rollback()
				c.JSON(http.StatusBadRequest, gin.H{
					"error":        "Insufficient stock",
					"product_id":   item.ProductID,
					"product_name": product.Name,
					"requested":    item.Quantity,
					"available":    product.Stock,
				})
				return
			}

			// Deduct stock
			product.Stock -= item.Quantity
			if err := tx.Save(&product).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product stock"})
				return
			}
		}

		// Clear user's cart after successful payment
		var cart models.Cart
		if err := tx.Where("user_id = ?", user.ID).First(&cart).Error; err == nil {
			if err := tx.Where("cart_id = ?", cart.ID).Delete(&models.CartItem{}).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear cart"})
				return
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load cart"})
			return
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process payment"})
			return
		}

		// Reload order
		db.Preload("Items.Product").First(&order, order.ID)

		c.JSON(http.StatusOK, gin.H{
			"message": "Payment processed successfully",
			"order":   order,
		})
	}
}

// UpdateOrderStatus updates the status of an order (for webhooks or admin)
func UpdateOrderStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get order ID from URL
		orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}

		// Parse request
		var req UpdateOrderStatusRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Get order
		var order models.Order
		if err := db.First(&order, orderID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}

		// Update status
		order.Status = req.Status
		if err := db.Save(&order).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
			return
		}

		// If status is PAID, deduct stock
		if req.Status == models.StatusPaid && order.Status != models.StatusPaid {
			tx := db.Begin()
			defer func() {
				if r := recover(); r != nil {
					tx.Rollback()
				}
			}()

			// Load order items
			if err := tx.Preload("Items").First(&order, orderID).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load order items"})
				return
			}

			// Deduct stock
			for _, item := range order.Items {
				var product models.Product
				if err := tx.First(&product, item.ProductID).Error; err != nil {
					tx.Rollback()
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product stock"})
					return
				}

				if product.Stock >= item.Quantity {
					product.Stock -= item.Quantity
					if err := tx.Save(&product).Error; err != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product stock"})
						return
					}
				}
			}

			if err := tx.Commit().Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process stock update"})
				return
			}
		}

		// Reload order
		db.Preload("Items.Product").First(&order, order.ID)

		c.JSON(http.StatusOK, order)
	}
}

// GetAllOrders retrieves all orders (admin only)
func GetAllOrders(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var orders []models.Order
		if err := db.Preload("Items.Product").
			Preload("User").
			Order("created_at DESC").
			Find(&orders).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders"})
			return
		}

		c.JSON(http.StatusOK, orders)
	}
}

// GetAdminOrderByID retrieves any order by ID (admin only)
func GetAdminOrderByID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get order ID from URL
		orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}

		// Get order
		var order models.Order
		if err := db.Preload("Items.Product").
			Preload("User").
			First(&order, orderID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}

		c.JSON(http.StatusOK, order)
	}
}
