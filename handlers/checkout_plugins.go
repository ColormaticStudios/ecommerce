package handlers

import (
	"net/http"

	"ecommerce/internal/checkoutplugins"
	"ecommerce/models"

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

func cartSubtotal(cart *models.Cart) models.Money {
	var subtotal models.Money
	if cart == nil {
		return subtotal
	}
	for _, item := range cart.Items {
		if item.Quantity <= 0 {
			continue
		}
		subtotal += item.ProductVariant.Price.Mul(item.Quantity)
	}
	return subtotal
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

		quote := pluginManager.Quote(checkoutplugins.QuoteRequest{
			Subtotal:     cartSubtotal(requestCtx.Cart),
			PaymentID:    req.PaymentProviderID,
			ShippingID:   req.ShippingProviderID,
			TaxID:        req.TaxProviderID,
			PaymentData:  req.PaymentData,
			ShippingData: req.ShippingData,
			TaxData:      req.TaxData,
		})

		c.JSON(http.StatusOK, gin.H{
			"currency":        quote.Currency,
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
