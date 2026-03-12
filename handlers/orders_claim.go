package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"ecommerce/internal/media"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const orderAlreadyClaimedCode = "order_already_claimed"

type ClaimGuestOrderRequest struct {
	Email             string `json:"email"`
	ConfirmationToken string `json:"confirmation_token"`
}

func ClaimGuestOrder(db *gorm.DB, mediaServices ...*media.Service) gin.HandlerFunc {
	mediaService := resolveMediaService(mediaServices...)
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		var req ClaimGuestOrderRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		email, err := normalizeGuestEmail(req.Email)
		if err != nil || email == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email must be valid"})
			return
		}
		token := strings.TrimSpace(req.ConfirmationToken)
		if token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Confirmation token is required"})
			return
		}

		var order models.Order
		if err := db.Where(
			"confirmation_token = ? AND LOWER(COALESCE(guest_email, '')) = ?",
			token,
			*email,
		).
			Preload("Items.ProductVariant").
			Preload("Items.ProductVariant.Product").
			First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				log.Printf(
					"guest_order_claim result=failure user_id=%d email=%q token=%q reason=%q",
					user.ID,
					*email,
					token,
					"not_found",
				)
				c.JSON(http.StatusNotFound, gin.H{"error": "Guest order not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to claim guest order"})
			return
		}

		if order.UserID != nil {
			if *order.UserID != user.ID {
				log.Printf(
					"guest_order_claim result=failure user_id=%d order_id=%d email=%q reason=%q",
					user.ID,
					order.ID,
					*email,
					orderAlreadyClaimedCode,
				)
				c.JSON(http.StatusConflict, gin.H{
					"error": "Order has already been claimed",
					"code":  orderAlreadyClaimedCode,
				})
				return
			}

			response, err := buildCreatedOrderResponse(db, mediaService, order, &user.ID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render order"})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"message": "Order already linked to this account",
				"order":   response,
			})
			return
		}

		claimedAt := time.Now().UTC()
		err = db.Transaction(func(tx *gorm.DB) error {
			update := tx.Model(&models.Order{}).
				Where("id = ? AND user_id IS NULL", order.ID).
				Updates(map[string]any{
					"user_id":    user.ID,
					"claimed_at": claimedAt,
				})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected == 0 {
				var claimed models.Order
				if err := tx.Select("user_id").First(&claimed, order.ID).Error; err != nil {
					return err
				}
				if claimed.UserID != nil && *claimed.UserID == user.ID {
					return nil
				}
				return gorm.ErrDuplicatedKey
			}

			return tx.Model(&models.CheckoutSession{}).
				Where("id = ?", order.CheckoutSessionID).
				Updates(map[string]any{
					"user_id":      user.ID,
					"guest_email":  *email,
					"last_seen_at": claimedAt,
				}).Error
		})
		if err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				log.Printf(
					"guest_order_claim result=failure user_id=%d order_id=%d email=%q reason=%q",
					user.ID,
					order.ID,
					*email,
					orderAlreadyClaimedCode,
				)
				c.JSON(http.StatusConflict, gin.H{
					"error": "Order has already been claimed",
					"code":  orderAlreadyClaimedCode,
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to claim guest order"})
			return
		}

		if err := db.Preload("Items.ProductVariant").
			Preload("Items.ProductVariant.Product").
			First(&order, order.ID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load claimed order"})
			return
		}

		response, err := buildCreatedOrderResponse(db, mediaService, order, &user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render order"})
			return
		}

		log.Printf(
			"guest_order_claim result=success user_id=%d order_id=%d checkout_session_id=%d email=%q",
			user.ID,
			order.ID,
			order.CheckoutSessionID,
			*email,
		)
		c.JSON(http.StatusOK, gin.H{
			"message": "Order linked to your account",
			"order":   response,
		})
	}
}
