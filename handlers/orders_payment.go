package handlers

import (
	"errors"
	"fmt"
	"io"
	"log"
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
		if err := db.Where("id = ? AND user_id = ?", orderID, user.ID).
			Preload("Items.ProductVariant").
			Preload("Items.ProductVariant.Product").
			First(&order).Error; err != nil {
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
			paymentDisplay, err = resolvePaymentDisplayForOrder(db, &user.ID, req)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			shippingAddress, err = resolveShippingAddressForOrder(db, &user.ID, req)
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

		if err := checkoutservice.ClearOrderedItemsFromCart(tx, order.CheckoutSessionID, order.Items); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update cart"})
			return
		}
		if err := tx.Model(&models.CheckoutSession{}).
			Where("id = ?", order.CheckoutSessionID).
			Updates(map[string]any{
				"status":      models.CheckoutSessionStatusConverted,
				"guest_email": order.GuestEmail,
			}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update checkout session"})
			return
		}

		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process payment"})
			return
		}

		db.Preload("Items.ProductVariant").Preload("Items.ProductVariant.Product").First(&order, order.ID)
		applyOrderMediaToOrder(&order, mediaService)
		applyOrderCapabilities(&order, &user.ID)
		responseOrder, err := buildOrderResponse(db, mediaService, order)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render order"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Order submitted and pending confirmation",
			"order":   responseOrder,
		})
	}
}

func resolvePaymentDisplayForOrder(db *gorm.DB, userID *uint, req ProcessPaymentRequest) (string, error) {
	if req.PaymentMethodID != nil {
		if userID == nil {
			return "", fmt.Errorf("payment method not found")
		}
		var method models.SavedPaymentMethod
		if err := db.Where("id = ? AND user_id = ?", *req.PaymentMethodID, *userID).First(&method).Error; err != nil {
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

	if userID != nil {
		var method models.SavedPaymentMethod
		if err := db.Where("user_id = ? AND is_default = ?", *userID, true).First(&method).Error; err == nil {
			return paymentMethodDisplay(method.Brand, method.Last4), nil
		}
	}

	return "", fmt.Errorf("payment method is required")
}

func resolveShippingAddressForOrder(db *gorm.DB, userID *uint, req ProcessPaymentRequest) (string, error) {
	if req.AddressID != nil {
		if userID == nil {
			return "", fmt.Errorf("address not found")
		}
		var address models.SavedAddress
		if err := db.Where("id = ? AND user_id = ?", *req.AddressID, *userID).First(&address).Error; err != nil {
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

	if userID != nil {
		var address models.SavedAddress
		if err := db.Where("user_id = ? AND is_default = ?", *userID, true).First(&address).Error; err == nil {
			return addressPretty(address), nil
		}
	}

	return "", fmt.Errorf("shipping address is required")
}

// AuthorizeCheckoutOrderPayment processes payment for an order owned by the current checkout session.
func AuthorizeCheckoutOrderPayment(
	db *gorm.DB,
	pluginManager *checkoutplugins.Manager,
	jwtSecret string,
	cookieCfg AuthCookieConfig,
	mediaServices ...*media.Service,
) gin.HandlerFunc {
	mediaService := resolveMediaService(mediaServices...)
	return func(c *gin.Context) {
		requestCtx, ok := resolveCheckoutOrderRequestContext(db, c, jwtSecret, cookieCfg)
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
		if err := db.Where("id = ? AND checkout_session_id = ?", orderID, requestCtx.Session.ID).
			Preload("Items.ProductVariant").
			Preload("Items.ProductVariant.Product").
			First(&order).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}

		handled, err := replayCheckoutIdempotency(
			db,
			c,
			requestCtx.Session,
			fmt.Sprintf("checkout_order_payment_authorize:%d", order.ID),
			req,
		)
		if err != nil {
			log.Printf(
				"checkout_order_payment_authorize result=failure mode=%s session_id=%d user_id=%v guest_email=%q order_id=%d reason=%q",
				checkoutMode(requestCtx.User),
				requestCtx.Session.ID,
				checkoutUserID(requestCtx.User),
				checkoutGuestEmail(order.GuestEmail),
				order.ID,
				err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process checkout request"})
			return
		}
		if handled {
			return
		}

		if !enforceCheckoutSubmissionRateLimit(c, requestCtx.Session, "authorize_payment") {
			log.Printf(
				"checkout_order_payment_authorize result=failure mode=%s session_id=%d user_id=%v guest_email=%q order_id=%d reason=%q",
				checkoutMode(requestCtx.User),
				requestCtx.Session.ID,
				checkoutUserID(requestCtx.User),
				checkoutGuestEmail(order.GuestEmail),
				order.ID,
				checkoutRateLimitedCode,
			)
			return
		}

		idempotencyRecord, handled, err := beginCheckoutIdempotency(
			db,
			c,
			requestCtx.Session,
			fmt.Sprintf("checkout_order_payment_authorize:%d", order.ID),
			req,
		)
		if err != nil {
			log.Printf(
				"checkout_order_payment_authorize result=failure mode=%s session_id=%d user_id=%v guest_email=%q order_id=%d reason=%q",
				checkoutMode(requestCtx.User),
				requestCtx.Session.ID,
				checkoutUserID(requestCtx.User),
				checkoutGuestEmail(order.GuestEmail),
				order.ID,
				err.Error(),
			)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process checkout request"})
			return
		}
		if handled {
			return
		}

		respond := func(status int, payload any) {
			writeCheckoutJSON(db, c, idempotencyRecord, status, payload)
		}

		if requestCtx.User == nil && (order.GuestEmail == nil || strings.TrimSpace(*order.GuestEmail) == "") {
			log.Printf(
				"checkout_order_payment_authorize result=failure mode=%s session_id=%d user_id=%v guest_email=%q order_id=%d reason=%q",
				checkoutMode(requestCtx.User),
				requestCtx.Session.ID,
				checkoutUserID(requestCtx.User),
				checkoutGuestEmail(order.GuestEmail),
				order.ID,
				"Guest email is required",
			)
			respond(http.StatusBadRequest, gin.H{"error": "Guest email is required"})
			return
		}
		if requestCtx.Session.Status == models.CheckoutSessionStatusConverted && !checkoutOrderPaymentSubmitted(order) {
			respond(http.StatusConflict, gin.H{"error": "Checkout session has already been converted"})
			return
		}
		if order.Status == models.StatusPaid {
			respond(http.StatusBadRequest, gin.H{"error": "Order is already paid"})
			return
		}
		if checkoutOrderPaymentSubmitted(order) {
			respond(http.StatusBadRequest, gin.H{"error": "Order payment already submitted"})
			return
		}
		currentOpenOrder, err := findCurrentCheckoutOpenOrder(db, requestCtx.Session.ID)
		if err != nil {
			respond(http.StatusInternalServerError, gin.H{"error": "Failed to load checkout order"})
			return
		}
		if currentOpenOrder != nil && currentOpenOrder.ID != order.ID {
			respond(http.StatusConflict, gin.H{"error": "Checkout order is no longer payable"})
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
					respond(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				respond(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			paymentDisplay = resolved.PaymentDisplay
			shippingAddress = resolved.ShippingAddress
			order.Total = resolved.Total
		} else {
			var userID *uint
			if requestCtx.User != nil {
				userID = &requestCtx.User.ID
			}
			paymentDisplay, err = resolvePaymentDisplayForOrder(db, userID, req)
			if err != nil {
				respond(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			shippingAddress, err = resolveShippingAddressForOrder(db, userID, req)
			if err != nil {
				respond(http.StatusBadRequest, gin.H{"error": err.Error()})
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
			respond(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
			return
		}
		if err := checkoutservice.ClearOrderedItemsFromCart(tx, order.CheckoutSessionID, order.Items); err != nil {
			tx.Rollback()
			respond(http.StatusInternalServerError, gin.H{"error": "Failed to update cart"})
			return
		}
		if err := tx.Model(&models.CheckoutSession{}).
			Where("id = ?", order.CheckoutSessionID).
			Updates(map[string]any{
				"status":      models.CheckoutSessionStatusConverted,
				"guest_email": order.GuestEmail,
			}).Error; err != nil {
			tx.Rollback()
			respond(http.StatusInternalServerError, gin.H{"error": "Failed to update checkout session"})
			return
		}
		if err := tx.Commit().Error; err != nil {
			respond(http.StatusInternalServerError, gin.H{"error": "Failed to process payment"})
			return
		}

		if err := db.Preload("Items.ProductVariant").
			Preload("Items.ProductVariant.Product").
			First(&order, order.ID).Error; err != nil {
			respond(http.StatusInternalServerError, gin.H{"error": "Failed to load order"})
			return
		}
		var userID *uint
		if requestCtx.User != nil {
			userID = &requestCtx.User.ID
		}
		applyOrderMediaToOrder(&order, mediaService)
		applyOrderCapabilities(&order, userID)
		responseOrder, err := buildOrderResponse(db, mediaService, order)
		if err != nil {
			respond(http.StatusInternalServerError, gin.H{"error": "Failed to render order"})
			return
		}

		log.Printf(
			"checkout_order_payment_authorize result=success mode=%s session_id=%d user_id=%v guest_email=%q order_id=%d",
			checkoutMode(requestCtx.User),
			requestCtx.Session.ID,
			checkoutUserID(requestCtx.User),
			checkoutGuestEmail(order.GuestEmail),
			order.ID,
		)
		respond(http.StatusOK, gin.H{
			"message": "Order submitted and pending confirmation",
			"order":   responseOrder,
		})
	}
}
