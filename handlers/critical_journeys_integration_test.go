package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Critical backend integration coverage for test scenarios only.
// This file intentionally validates end-to-end handler/database behavior.

func performJSONRequest(t *testing.T, r *gin.Engine, method, path string, body any, bearerToken string) *httptest.ResponseRecorder {
	t.Helper()

	var reader *strings.Reader
	if body == nil {
		reader = strings.NewReader("")
	} else {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = strings.NewReader(string(raw))
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeJSON[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out
}

func TestCriticalJourney_RegisterCartOrderAndPayPersistsState(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.Product{},
		&models.Cart{},
		&models.CartItem{},
		&models.Order{},
		&models.OrderItem{},
		&models.SavedPaymentMethod{},
		&models.SavedAddress{},
	)
	product := seedProduct(t, db, "journey-sku-1", "Journey Product", 39.99, 8)

	registerResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"username": "journey-user",
		"email":    "journey-user@example.com",
		"password": "supersecret",
		"name":     "Journey User",
	}, "")
	require.Equal(t, http.StatusCreated, registerResp.Code)
	registered := decodeJSON[AuthResponse](t, registerResp)

	customerToken := issueBearerTokenWithRole(
		t,
		generatedTestJWTSecret,
		registered.User.Subject,
		"customer",
	)

	addCartResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/cart", map[string]any{
		"product_id": product.ID,
		"quantity":   2,
	}, customerToken)
	require.Equal(t, http.StatusOK, addCartResp.Code)
	cart := decodeJSON[models.Cart](t, addCartResp)
	require.Len(t, cart.Items, 1)
	assert.Equal(t, 2, cart.Items[0].Quantity)

	createOrderResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/orders", map[string]any{
		"items": []map[string]any{{
			"product_id": product.ID,
			"quantity":   2,
		}},
	}, customerToken)
	require.Equal(t, http.StatusCreated, createOrderResp.Code)
	order := decodeJSON[models.Order](t, createOrderResp)
	require.NotZero(t, order.ID)
	require.Equal(t, models.StatusPending, order.Status)

	createSavedPaymentResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/payment-methods", map[string]any{
		"cardholder_name": "Journey User",
		"card_number":     "4111111111111111",
		"exp_month":       12,
		"exp_year":        2040,
		"set_default":     true,
	}, customerToken)
	require.Equal(t, http.StatusCreated, createSavedPaymentResp.Code)
	savedPayment := decodeJSON[models.SavedPaymentMethod](t, createSavedPaymentResp)
	require.NotZero(t, savedPayment.ID)

	createSavedAddressResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/addresses", map[string]any{
		"full_name":   "Journey User",
		"line1":       "123 Integration Way",
		"city":        "Austin",
		"postal_code": "78701",
		"country":     "US",
		"set_default": true,
	}, customerToken)
	require.Equal(t, http.StatusCreated, createSavedAddressResp.Code)
	savedAddress := decodeJSON[models.SavedAddress](t, createSavedAddressResp)
	require.NotZero(t, savedAddress.ID)

	payResp := performJSONRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/me/orders/%d/pay", order.ID), map[string]any{
		"payment_method_id": savedPayment.ID,
		"address_id":        savedAddress.ID,
	}, customerToken)
	require.Equal(t, http.StatusOK, payResp.Code)
	payBody := decodeJSON[map[string]any](t, payResp)
	assert.Equal(t, "Payment processed successfully", payBody["message"])

	ordersResp := performJSONRequest(t, r, http.MethodGet, "/api/v1/me/orders?status=PAID&page=1&limit=20", nil, customerToken)
	require.Equal(t, http.StatusOK, ordersResp.Code)
	ordersBody := decodeJSON[map[string]any](t, ordersResp)
	ordersRaw, ok := ordersBody["data"].([]any)
	require.True(t, ok)
	require.Len(t, ordersRaw, 1)

	var reloadedOrder models.Order
	require.NoError(t, db.Preload("Items").First(&reloadedOrder, order.ID).Error)
	assert.Equal(t, models.StatusPaid, reloadedOrder.Status)
	assert.Contains(t, reloadedOrder.PaymentMethodDisplay, "1111")
	assert.Contains(t, reloadedOrder.ShippingAddressPretty, "Austin")

	var reloadedProduct models.Product
	require.NoError(t, db.First(&reloadedProduct, product.ID).Error)
	assert.Equal(t, 6, reloadedProduct.Stock)

	var cartItems int64
	require.NoError(t, db.Model(&models.CartItem{}).Count(&cartItems).Error)
	assert.EqualValues(t, 0, cartItems)
}

func TestCriticalJourney_InvalidPayloadsAreRejected(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.Product{},
		&models.Cart{},
		&models.CartItem{},
		&models.Order{},
		&models.OrderItem{},
		&models.SavedPaymentMethod{},
		&models.SavedAddress{},
	)
	product := seedProduct(t, db, "journey-sku-2", "Validation Product", 22.50, 3)

	registerResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"username": "invalid-flow-user",
		"email":    "invalid-flow-user@example.com",
		"password": "supersecret",
	}, "")
	require.Equal(t, http.StatusCreated, registerResp.Code)
	registered := decodeJSON[AuthResponse](t, registerResp)
	customerToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, registered.User.Subject, "customer")

	tooMuchStockResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/cart", map[string]any{
		"product_id": product.ID,
		"quantity":   10,
	}, customerToken)
	require.Equal(t, http.StatusBadRequest, tooMuchStockResp.Code)
	tooMuchStockBody := decodeJSON[map[string]any](t, tooMuchStockResp)
	assert.Equal(t, "Insufficient stock", tooMuchStockBody["error"])

	emptyOrderResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/orders", map[string]any{
		"items": []any{},
	}, customerToken)
	require.Equal(t, http.StatusBadRequest, emptyOrderResp.Code)
	assert.Contains(t, emptyOrderResp.Body.String(), "at least one item")

	createOrderResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/orders", map[string]any{
		"items": []map[string]any{{
			"product_id": product.ID,
			"quantity":   1,
		}},
	}, customerToken)
	require.Equal(t, http.StatusCreated, createOrderResp.Code)
	order := decodeJSON[models.Order](t, createOrderResp)

	invalidCardResp := performJSONRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/me/orders/%d/pay", order.ID), map[string]any{
		"payment_method": map[string]any{
			"cardholder_name": "Invalid Flow",
			"card_number":     "4242",
			"exp_month":       10,
			"exp_year":        2030,
		},
		"address": map[string]any{
			"full_name":   "Invalid Flow",
			"line1":       "1 Main St",
			"city":        "Austin",
			"postal_code": "78701",
			"country":     "US",
		},
	}, customerToken)
	require.Equal(t, http.StatusBadRequest, invalidCardResp.Code)
	assert.Contains(t, invalidCardResp.Body.String(), "card number")

	invalidCountryResp := performJSONRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/me/orders/%d/pay", order.ID), map[string]any{
		"payment_method": map[string]any{
			"cardholder_name": "Invalid Flow",
			"card_number":     "4111111111111111",
			"exp_month":       10,
			"exp_year":        2030,
		},
		"address": map[string]any{
			"full_name":   "Invalid Flow",
			"line1":       "1 Main St",
			"city":        "Austin",
			"postal_code": "78701",
			"country":     "USA",
		},
	}, customerToken)
	require.Equal(t, http.StatusBadRequest, invalidCountryResp.Code)
	assert.Contains(t, strings.ToLower(invalidCountryResp.Body.String()), "country")

	missingPaymentResp := performJSONRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/me/orders/%d/pay", order.ID), map[string]any{}, customerToken)
	require.Equal(t, http.StatusBadRequest, missingPaymentResp.Code)
	assert.Contains(t, missingPaymentResp.Body.String(), "payment method is required")
}

func TestCriticalJourney_OrderCannotBePaidByAnotherUser(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.Product{},
		&models.Order{},
		&models.OrderItem{},
	)
	product := seedProduct(t, db, "journey-sku-3", "Authorization Product", 11.99, 5)

	owner := seedUser(t, db, "owner-sub", "owner", "owner@example.com", "customer")
	other := seedUser(t, db, "other-sub", "other", "other@example.com", "customer")

	createOrderResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/orders", map[string]any{
		"items": []map[string]any{{
			"product_id": product.ID,
			"quantity":   1,
		}},
	}, issueBearerTokenWithRole(t, generatedTestJWTSecret, owner.Subject, owner.Role))
	require.Equal(t, http.StatusCreated, createOrderResp.Code)
	order := decodeJSON[models.Order](t, createOrderResp)

	payAsOtherResp := performJSONRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/me/orders/%d/pay", order.ID), map[string]any{
		"payment_method": map[string]any{
			"cardholder_name": "Other User",
			"card_number":     "4111111111111111",
			"exp_month":       12,
			"exp_year":        2032,
		},
		"address": map[string]any{
			"full_name":   "Other User",
			"line1":       "400 Denied Ave",
			"city":        "Denver",
			"postal_code": "80202",
			"country":     "US",
		},
	}, issueBearerTokenWithRole(t, generatedTestJWTSecret, other.Subject, other.Role))
	require.Equal(t, http.StatusNotFound, payAsOtherResp.Code)
	assert.Contains(t, payAsOtherResp.Body.String(), "Order not found")

	var reloadedOrder models.Order
	require.NoError(t, db.First(&reloadedOrder, order.ID).Error)
	assert.Equal(t, models.StatusPending, reloadedOrder.Status)
}
