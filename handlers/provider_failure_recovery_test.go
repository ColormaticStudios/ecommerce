package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	paymentservice "ecommerce/internal/services/payments"
	providerops "ecommerce/internal/services/providerops"
	shippingservice "ecommerce/internal/services/shipping"
	taxservice "ecommerce/internal/services/tax"
	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type testPaymentRegistry struct {
	provider paymentservice.PaymentProvider
}

func (r testPaymentRegistry) Provider(providerID string) (paymentservice.PaymentProvider, error) {
	if strings.TrimSpace(providerID) != "dummy-card" {
		return nil, fmt.Errorf("%w: %s", paymentservice.ErrUnknownPaymentProvider, providerID)
	}
	return r.provider, nil
}

type testShippingRegistry struct {
	provider shippingservice.ShippingProvider
}

func (r testShippingRegistry) Provider(providerID string) (shippingservice.ShippingProvider, error) {
	if strings.TrimSpace(providerID) != "dummy-ground" {
		return nil, fmt.Errorf("%w: %s", shippingservice.ErrUnknownShippingProvider, providerID)
	}
	return r.provider, nil
}

type interceptingPaymentProvider struct {
	paymentservice.PaymentProvider
	authorizeStarted *atomic.Int32
}

func (p interceptingPaymentProvider) Authorize(ctx context.Context, req paymentservice.AuthorizeRequest) (paymentservice.ProviderOperationResult, error) {
	p.authorizeStarted.Store(1)
	return p.PaymentProvider.Authorize(ctx, req)
}

type interceptingShippingProvider struct {
	shippingservice.ShippingProvider
	buyLabelStarted *atomic.Int32
}

func (p interceptingShippingProvider) BuyLabel(ctx context.Context, req shippingservice.BuyLabelRequest) (shippingservice.ProviderShipment, error) {
	p.buyLabelStarted.Store(1)
	return p.ShippingProvider.BuyLabel(ctx, req)
}

func TestCheckoutPaymentAuthorizationPersistsIntentWhenOrderFinalizeFails(t *testing.T) {
	resetCheckoutProtectionForTest(t)

	baseRegistry := paymentservice.NewDefaultProviderRegistry()
	baseProvider, err := baseRegistry.Provider("dummy-card")
	require.NoError(t, err)

	var authorizeStarted atomic.Int32
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{
		ProviderRuntime: providerops.NewRuntime(nil, providerops.RuntimeConfig{
			Environment:       "sandbox",
			PaymentProviders:  testPaymentRegistry{provider: interceptingPaymentProvider{PaymentProvider: baseProvider, authorizeStarted: &authorizeStarted}},
			ShippingProviders: shippingservice.NewDefaultProviderRegistry(),
			TaxProviders:      taxservice.NewDefaultProviderRegistry(),
		}),
	},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
		&models.IdempotencyKey{},
		&models.OrderCheckoutSnapshot{},
		&models.OrderCheckoutSnapshotItem{},
		&models.PaymentIntent{},
		&models.PaymentTransaction{},
	)

	var failedOnce atomic.Int32
	callbackName := "test:fail-order-update-after-authorize"
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if authorizeStarted.Load() == 0 || failedOnce.Load() != 0 {
			return
		}
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "orders" && failedOnce.CompareAndSwap(0, 1) {
			tx.AddError(errors.New("forced order finalize failure"))
		}
	}))
	t.Cleanup(func() {
		_ = db.Callback().Update().Remove(callbackName)
	})

	product := seedProduct(t, db, "sku-provider-auth-failure", "Provider Auth Failure Product", 19.5, 8)
	variantID := requireDefaultVariantID(t, product)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/checkout/cart", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)
	require.Equal(t, http.StatusOK, getW.Code)
	checkoutToken := cookieValueByName(t, getW, checkoutSessionCookieName)
	csrfToken := cookieValueByName(t, getW, csrfCookieName)

	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/cart/items", strings.NewReader(`{"product_variant_id":`+strconv.Itoa(int(variantID))+`,"quantity":1}`))
	addReq.Header.Set("Content-Type", "application/json")
	addReq.Header.Set("X-CSRF-Token", csrfToken)
	addReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	addReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	addW := httptest.NewRecorder()
	r.ServeHTTP(addW, addReq)
	require.Equal(t, http.StatusOK, addW.Code)

	order := createGuestCheckoutOrder(t, r, checkoutToken, csrfToken, "provider-auth-failure@example.com")
	quote := quoteCheckoutWithDummyProviders(t, r, checkoutToken, csrfToken)
	require.NotNil(t, quote.SnapshotID)

	authorizeW := authorizeCheckoutWithSnapshot(t, r, order.Id, *quote.SnapshotID, checkoutToken, csrfToken, "provider-auth-failure")
	require.Equal(t, http.StatusInternalServerError, authorizeW.Code)
	assert.Contains(t, authorizeW.Body.String(), "Failed to process payment")
	assert.EqualValues(t, 1, authorizeStarted.Load())

	var intents []models.PaymentIntent
	require.NoError(t, db.Preload("Transactions", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at ASC, id ASC")
	}).Order("id ASC").Find(&intents).Error)
	require.Len(t, intents, 1)
	assert.Equal(t, models.PaymentIntentStatusAuthorized, intents[0].Status)
	require.Len(t, intents[0].Transactions, 1)
	assert.Equal(t, models.PaymentTransactionOperationAuthorize, intents[0].Transactions[0].Operation)
	assert.Equal(t, models.PaymentTransactionStatusSucceeded, intents[0].Transactions[0].Status)
	assert.NotEmpty(t, intents[0].Transactions[0].ProviderTxnID)

	var storedOrder models.Order
	require.NoError(t, db.First(&storedOrder, order.Id).Error)
	assert.Equal(t, models.StatusPending, storedOrder.Status)
	assert.Empty(t, strings.TrimSpace(storedOrder.PaymentMethodDisplay))
	assert.Empty(t, strings.TrimSpace(storedOrder.ShippingAddressPretty))
}

func TestShippingLabelPurchasePersistsDraftShipmentWhenFinalizeFails(t *testing.T) {
	resetCheckoutProtectionForTest(t)

	baseRegistry := shippingservice.NewDefaultProviderRegistry()
	baseProvider, err := baseRegistry.Provider("dummy-ground")
	require.NoError(t, err)

	var buyLabelStarted atomic.Int32
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{
		ProviderRuntime: providerops.NewRuntime(nil, providerops.RuntimeConfig{
			Environment:       "sandbox",
			PaymentProviders:  paymentservice.NewDefaultProviderRegistry(),
			ShippingProviders: testShippingRegistry{provider: interceptingShippingProvider{ShippingProvider: baseProvider, buyLabelStarted: &buyLabelStarted}},
			TaxProviders:      taxservice.NewDefaultProviderRegistry(),
		}),
	},
		&models.User{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
		&models.IdempotencyKey{},
		&models.OrderCheckoutSnapshot{},
		&models.OrderCheckoutSnapshotItem{},
		&models.ShipmentRate{},
		&models.Shipment{},
		&models.ShipmentPackage{},
	)

	var failedOnce atomic.Int32
	callbackName := "test:fail-shipment-update-after-buy-label"
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if buyLabelStarted.Load() == 0 || failedOnce.Load() != 0 {
			return
		}
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "shipments" && failedOnce.CompareAndSwap(0, 1) {
			tx.AddError(errors.New("forced shipment finalize failure"))
		}
	}))
	t.Cleanup(func() {
		_ = db.Callback().Update().Remove(callbackName)
	})

	admin := seedUser(t, db, "sub-admin-shipment-failure", "admin-shipment-failure", "admin-shipment-failure@example.com", "admin")
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)
	product := seedProduct(t, db, "sku-shipping-failure", "Shipping Failure Product", 22, 5)
	variantID := requireDefaultVariantID(t, product)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/checkout/cart", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)
	require.Equal(t, http.StatusOK, getW.Code)
	checkoutToken := cookieValueByName(t, getW, checkoutSessionCookieName)
	csrfToken := cookieValueByName(t, getW, csrfCookieName)

	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/cart/items", strings.NewReader(`{"product_variant_id":`+strconv.Itoa(int(variantID))+`,"quantity":1}`))
	addReq.Header.Set("Content-Type", "application/json")
	addReq.Header.Set("X-CSRF-Token", csrfToken)
	addReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	addReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	addW := httptest.NewRecorder()
	r.ServeHTTP(addW, addReq)
	require.Equal(t, http.StatusOK, addW.Code)

	order := createGuestCheckoutOrder(t, r, checkoutToken, csrfToken, "shipping-failure@example.com")
	quote := quoteCheckoutWithDummyProviders(t, r, checkoutToken, csrfToken)
	require.NotNil(t, quote.SnapshotID)

	ratesW := quoteShippingRatesWithSnapshot(t, r, order.Id, *quote.SnapshotID, checkoutToken, csrfToken, "shipping-failure-rates")
	require.Equal(t, http.StatusOK, ratesW.Code)
	rates := decodeJSON[checkoutShippingRatesTestResponse](t, ratesW)
	require.NotEmpty(t, rates.Rates)

	selectedRateID := rates.Rates[0].ID
	for _, rate := range rates.Rates {
		if rate.Selected {
			selectedRateID = rate.ID
			break
		}
	}

	labelPath := fmt.Sprintf("/api/v1/admin/orders/%d/shipping/labels", order.Id)
	labelW := adminLifecycleRequest(
		t,
		r,
		http.MethodPost,
		labelPath,
		fmt.Sprintf(`{"rate_id":%d,"package":{"reference":"draft-box","weight_grams":500}}`, selectedRateID),
		adminToken,
		"shipping-finalize-failure",
	)
	require.Equal(t, http.StatusInternalServerError, labelW.Code)
	assert.Contains(t, labelW.Body.String(), "Failed to create shipping label")
	assert.EqualValues(t, 1, buyLabelStarted.Load())

	var shipments []models.Shipment
	require.NoError(t, db.Order("id ASC").Find(&shipments).Error)
	require.Len(t, shipments, 1)
	assert.Equal(t, models.ShipmentStatusQuoted, shipments[0].Status)
	assert.Nil(t, shipments[0].FinalizedAt)
	assert.Nil(t, shipments[0].PurchasedAt)
	assert.Contains(t, shipments[0].ProviderShipmentID, "pending:")
	assert.Equal(t, selectedRateID, shipments[0].ShipmentRateID)

	var rate models.ShipmentRate
	require.NoError(t, db.First(&rate, selectedRateID).Error)
	require.NotNil(t, rate.ShipmentID)
	assert.Equal(t, shipments[0].ID, *rate.ShipmentID)
	assert.True(t, rate.Selected)

	var packages []models.ShipmentPackage
	require.NoError(t, db.Where("shipment_id = ?", shipments[0].ID).Order("id ASC").Find(&packages).Error)
	require.Len(t, packages, 1)
	assert.Equal(t, "draft-box", packages[0].Reference)
	assert.Equal(t, 500, packages[0].WeightGrams)
}
