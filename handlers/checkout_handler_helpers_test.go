package handlers

import (
	"testing"
	"time"

	providerops "ecommerce/internal/services/providerops"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestHandlerResponseCaptureErrorReason(t *testing.T) {
	var capture handlerResponseCapture

	assert.Equal(t, "request_rejected", capture.ErrorReason())

	capture.Respond(400, gin.H{"error": "Checkout snapshot has expired"})
	assert.Equal(t, "Checkout snapshot has expired", capture.ErrorReason())
}

func TestMapProviderConfigurationError(t *testing.T) {
	var capture handlerResponseCapture

	handled := mapProviderConfigurationError(providerops.ErrProviderCredentialWrongEnvironment, capture.Respond)
	require.True(t, handled)
	assert.Equal(t, 409, capture.Status())
	assert.Equal(t, gin.H{"error": "Provider credential is not configured for this environment"}, capture.Payload())

	capture = handlerResponseCapture{}
	handled = mapProviderConfigurationError(providerops.ErrUnsupportedProviderCurrency, capture.Respond)
	require.True(t, handled)
	assert.Equal(t, 400, capture.Status())
	assert.Equal(t, gin.H{"error": "Provider does not support the requested currency"}, capture.Payload())
}

func TestLoadCheckoutSnapshotForOrderMapsMissingSnapshot(t *testing.T) {
	db := newTestDB(t)

	order := models.Order{BaseModel: models.BaseModel{ID: 42}}
	var (
		capture   handlerResponseCapture
		handled   bool
		helperErr error
	)
	err := db.Transaction(func(tx *gorm.DB) error {
		_, handled, helperErr = loadCheckoutSnapshotForOrder(tx, 99, 123, &order, time.Now().UTC(), capture.Respond)
		return helperErr
	})
	require.NoError(t, err)
	require.True(t, handled)
	assert.Equal(t, 400, capture.Status())
	assert.Equal(t, gin.H{"error": "Checkout snapshot not found"}, capture.Payload())
}

func TestLoadCheckoutSnapshotForOrderMapsValidationError(t *testing.T) {
	db := newTestDB(t)

	snapshot := models.OrderCheckoutSnapshot{
		CheckoutSessionID:  7,
		Currency:           "USD",
		Subtotal:           models.MoneyFromFloat(10),
		ShippingAmount:     0,
		TaxAmount:          0,
		Total:              models.MoneyFromFloat(10),
		PaymentProviderID:  "payment-demo",
		ShippingProviderID: "shipping-demo",
		ExpiresAt:          time.Now().UTC().Add(time.Hour),
	}
	require.NoError(t, db.Create(&snapshot).Error)

	order := models.Order{
		BaseModel: models.BaseModel{ID: 55},
		Total:     models.MoneyFromFloat(25),
	}

	var (
		capture   handlerResponseCapture
		handled   bool
		helperErr error
	)
	err := db.Transaction(func(tx *gorm.DB) error {
		_, handled, helperErr = loadCheckoutSnapshotForOrder(tx, snapshot.CheckoutSessionID, snapshot.ID, &order, time.Now().UTC(), capture.Respond)
		return helperErr
	})
	require.NoError(t, err)
	require.True(t, handled)
	assert.Equal(t, 409, capture.Status())
	assert.Equal(t, gin.H{"error": "Checkout snapshot no longer matches the order"}, capture.Payload())
}
