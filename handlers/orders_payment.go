package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"ecommerce/internal/checkoutplugins"
	"ecommerce/internal/media"
	checkoutservice "ecommerce/internal/services/checkout"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func hasPluginCheckoutSelection(req ProcessPaymentRequest) bool {
	return checkoutservice.HasProviderSelection(checkoutservice.ProviderSelection{
		PaymentProviderID:  req.PaymentProviderID,
		ShippingProviderID: req.ShippingProviderID,
		TaxProviderID:      req.TaxProviderID,
	})
}

// ProcessPayment processes payment for an order (mock implementation).
func ProcessPayment(db *gorm.DB, pluginManager *checkoutplugins.Manager, mediaServices ...*media.Service) gin.HandlerFunc {
	mediaService := resolveMediaService(mediaServices...)
	return func(c *gin.Context) {
		user, ok := getAuthenticatedUser(db, c)
		if !ok {
			return
		}

		var req ProcessPaymentRequest
		if err := bindStrictJSON(c, &req); err != nil && !errors.Is(err, io.EOF) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}

		var order models.Order
		if err := db.Where("id = ? AND user_id = ?", orderID, user.ID).Preload("Items.Product").First(&order).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}

		if order.Status == models.StatusPaid {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Order is already paid"})
			return
		}
		if strings.TrimSpace(order.PaymentMethodDisplay) != "" || strings.TrimSpace(order.ShippingAddressPretty) != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Order payment already submitted"})
			return
		}

		paymentDisplay := ""
		shippingAddress := ""
		if hasPluginCheckoutSelection(req) {
			resolved, err := checkoutservice.ResolveProviderSelection(pluginManager, order.Total, checkoutservice.ProviderSelection{
				PaymentProviderID:  req.PaymentProviderID,
				ShippingProviderID: req.ShippingProviderID,
				TaxProviderID:      req.TaxProviderID,
				PaymentData:        req.PaymentData,
				ShippingData:       req.ShippingData,
				TaxData:            req.TaxData,
			})
			if err != nil {
				if err.Error() == "checkout plugins unavailable" {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			paymentDisplay = resolved.PaymentDisplay
			shippingAddress = resolved.ShippingAddress
			order.Total = resolved.Total
		} else {
			paymentDisplay, err = resolvePaymentDisplayForOrder(db, user.ID, req)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			shippingAddress, err = resolveShippingAddressForOrder(db, user.ID, req)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}

		tx := db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		order.Status = models.StatusPending
		order.PaymentMethodDisplay = paymentDisplay
		order.ShippingAddressPretty = shippingAddress
		if err := tx.Save(&order).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
			return
		}

		if err := checkoutservice.ClearOrderedItemsFromCart(tx, user.ID, order.Items); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cart"})
			return
		}

		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process payment"})
			return
		}

		db.Preload("Items.Product").First(&order, order.ID)
		applyOrderMediaToOrder(&order, mediaService)
		applyOrderCapabilities(&order, &user.ID)

		c.JSON(http.StatusOK, gin.H{
			"message": "Order submitted and pending confirmation",
			"order":   order,
		})
	}
}

func resolvePaymentDisplayForOrder(db *gorm.DB, userID uint, req ProcessPaymentRequest) (string, error) {
	if req.PaymentMethodID != nil {
		var method models.SavedPaymentMethod
		if err := db.Where("id = ? AND user_id = ?", *req.PaymentMethodID, userID).First(&method).Error; err != nil {
			return "", fmt.Errorf("payment method not found")
		}
		return paymentMethodDisplay(method.Brand, method.Last4), nil
	}

	if req.PaymentMethod != nil {
		cardDigits := digitsOnly(req.PaymentMethod.CardNumber)
		if len(cardDigits) < 12 || len(cardDigits) > 19 {
			return "", fmt.Errorf("card number must be 12 to 19 digits")
		}
		brand := detectCardBrand(cardDigits)
		last4 := cardDigits[len(cardDigits)-4:]
		return paymentMethodDisplay(brand, last4), nil
	}

	var method models.SavedPaymentMethod
	if err := db.Where("user_id = ? AND is_default = ?", userID, true).First(&method).Error; err == nil {
		return paymentMethodDisplay(method.Brand, method.Last4), nil
	}

	return "", fmt.Errorf("payment method is required")
}

func resolveShippingAddressForOrder(db *gorm.DB, userID uint, req ProcessPaymentRequest) (string, error) {
	if req.AddressID != nil {
		var address models.SavedAddress
		if err := db.Where("id = ? AND user_id = ?", *req.AddressID, userID).First(&address).Error; err != nil {
			return "", fmt.Errorf("address not found")
		}
		return addressPretty(address), nil
	}

	if req.Address != nil {
		country := strings.ToUpper(strings.TrimSpace(req.Address.Country))
		if len(country) != 2 {
			return "", fmt.Errorf("country must be a 2-letter code")
		}
		address := models.SavedAddress{
			FullName:   strings.TrimSpace(req.Address.FullName),
			Line1:      strings.TrimSpace(req.Address.Line1),
			Line2:      strings.TrimSpace(req.Address.Line2),
			City:       strings.TrimSpace(req.Address.City),
			State:      strings.TrimSpace(req.Address.State),
			PostalCode: strings.TrimSpace(req.Address.PostalCode),
			Country:    country,
		}
		return addressPretty(address), nil
	}

	var address models.SavedAddress
	if err := db.Where("user_id = ? AND is_default = ?", userID, true).First(&address).Error; err == nil {
		return addressPretty(address), nil
	}

	return "", fmt.Errorf("shipping address is required")
}
