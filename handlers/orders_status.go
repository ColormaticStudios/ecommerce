package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"ecommerce/internal/media"
	orderservice "ecommerce/internal/services/orders"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CancelUserOrder cancels a user's order and restores stock when needed.
func CancelUserOrder(db *gorm.DB, mediaServices ...*media.Service) gin.HandlerFunc {
	mediaService := resolveMediaService(mediaServices...)
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}

		var responseOrder models.Order
		errCannotCancel := errors.New("order cannot be cancelled")

		err = db.Transaction(func(tx *gorm.DB) error {
			var order models.Order
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ?", orderID, user.ID).First(&order).Error; err != nil {
				return err
			}

			if !models.IsUserCancelableOrderStatus(order.Status) {
				return errCannotCancel
			}

			if err := orderservice.ApplyStatusTransition(tx, &order, models.StatusCancelled); err != nil {
				return err
			}

			if err := tx.Preload("Items.Product").First(&responseOrder, order.ID).Error; err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			switch {
			case errors.Is(err, gorm.ErrRecordNotFound):
				c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			case errors.Is(err, errCannotCancel):
				c.JSON(http.StatusBadRequest, gin.H{"error": "Order cannot be cancelled"})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel order"})
			}
			return
		}

		applyOrderMediaToOrder(&responseOrder, mediaService)
		applyOrderCapabilities(&responseOrder, &user.ID)
		c.JSON(http.StatusOK, responseOrder)
	}
}

// UpdateOrderStatus updates the status of an order (for webhooks or admin).
func UpdateOrderStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}

		var req UpdateOrderStatusRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if !models.IsValidOrderStatus(req.Status) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status"})
			return
		}

		var order models.Order
		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, orderID).Error; err != nil {
				return err
			}
			return orderservice.ApplyStatusTransition(tx, &order, req.Status)
		})
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
				return
			}
			var stockErr *orderservice.InsufficientStockError
			if errors.As(err, &stockErr) {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":        "Insufficient stock",
					"product_id":   stockErr.ProductID,
					"product_name": stockErr.ProductName,
					"requested":    stockErr.Requested,
					"available":    stockErr.Available,
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
			return
		}

		db.Preload("Items.Product").First(&order, order.ID)
		applyOrderCapabilities(&order, nil)
		c.JSON(http.StatusOK, order)
	}
}
