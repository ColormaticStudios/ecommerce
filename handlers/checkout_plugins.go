package handlers

import (
	"net/http"
	"time"

	"ecommerce/internal/checkoutplugins"
	checkoutservice "ecommerce/internal/services/checkout"
	paymentservice "ecommerce/internal/services/payments"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CheckoutQuoteRequest struct {
	PaymentProviderID  string            `json:"payment_provider_id" binding:"required"`
	ShippingProviderID string            `json:"shipping_provider_id" binding:"required"`
	TaxProviderID      string            `json:"tax_provider_id"`
	PaymentData        map[string]string `json:"payment_data"`
	ShippingData       map[string]string `json:"shipping_data"`
	TaxData            map[string]string `json:"tax_data"`
}

func ListCheckoutPlugins(pluginManager *checkoutplugins.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		payments, shippings, taxes := pluginManager.List()
		c.JSON(http.StatusOK, gin.H{
			"payment":  payments,
			"shipping": shippings,
			"tax":      taxes,
		})
	}
}

func ListCheckoutPluginsWithAccess(db *gorm.DB, pluginManager *checkoutplugins.Manager, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := requireCheckoutAccess(db, c, jwtSecret); !ok {
			return
		}
		ListCheckoutPlugins(pluginManager)(c)
	}
}

func QuoteCheckout(db *gorm.DB, pluginManager *checkoutplugins.Manager, jwtSecret string, cookieCfg AuthCookieConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestCtx, ok := resolveCheckoutRequestContext(db, c, jwtSecret, cookieCfg)
		if !ok {
			return
		}

		var req CheckoutQuoteRequest
		if err := bindStrictJSON(c, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		discounts, err := evaluateCartDiscounts(db, requestCtx.Cart)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to evaluate discounts"})
			return
		}

		quote := pluginManager.Quote(checkoutplugins.QuoteRequest{
			Subtotal:     discounts.FinalSubtotal,
			PaymentID:    req.PaymentProviderID,
			ShippingID:   req.ShippingProviderID,
			TaxID:        req.TaxProviderID,
			PaymentData:  req.PaymentData,
			ShippingData: req.ShippingData,
			TaxData:      req.TaxData,
		})

		var snapshotID *uint
		var expiresAt *time.Time
		if quote.Valid {
			resolved, err := checkoutservice.ResolveProviderSelection(
				pluginManager,
				discounts.FinalSubtotal,
				checkoutProviderSelectionFromPaymentRequest(ProcessPaymentRequest{
					PaymentProviderID:  req.PaymentProviderID,
					ShippingProviderID: req.ShippingProviderID,
					TaxProviderID:      req.TaxProviderID,
					PaymentData:        req.PaymentData,
					ShippingData:       req.ShippingData,
					TaxData:            req.TaxData,
				}),
			)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			snapshotItems, err := buildSnapshotItemsFromCart(db, requestCtx.Cart)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to evaluate discounts"})
				return
			}

			snapshot, err := paymentservice.CreateCheckoutSnapshot(db, paymentservice.CreateCheckoutSnapshotInput{
				CheckoutSessionID:     requestCtx.Session.ID,
				Currency:              quote.Currency,
				Subtotal:              quote.Subtotal,
				ShippingAmount:        quote.Shipping,
				TaxAmount:             quote.Tax,
				Total:                 quote.Total,
				PaymentProviderID:     req.PaymentProviderID,
				ShippingProviderID:    req.ShippingProviderID,
				TaxProviderID:         req.TaxProviderID,
				PaymentData:           req.PaymentData,
				ShippingData:          req.ShippingData,
				TaxData:               req.TaxData,
				PaymentMethodDisplay:  resolved.PaymentDisplay,
				ShippingAddressPretty: resolved.ShippingAddress,
				Items:                 snapshotItems,
				Now:                   time.Now().UTC(),
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create checkout snapshot"})
				return
			}
			snapshotID = &snapshot.ID
			expiresAt = &snapshot.ExpiresAt
		}

		c.JSON(http.StatusOK, gin.H{
			"snapshot_id":     snapshotID,
			"expires_at":      expiresAt,
			"currency":        quote.Currency,
			"discount_total":  discounts.DiscountTotal.Float64(),
			"subtotal":        quote.Subtotal,
			"shipping":        quote.Shipping,
			"tax":             quote.Tax,
			"total":           quote.Total,
			"valid":           quote.Valid,
			"payment_states":  quote.PaymentStates,
			"shipping_states": quote.ShippingStates,
			"tax_states":      quote.TaxStates,
		})
	}
}
