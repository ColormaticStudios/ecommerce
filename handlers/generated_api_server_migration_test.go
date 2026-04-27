package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/media"
	paymentservice "ecommerce/internal/services/payments"
	webhookservice "ecommerce/internal/services/webhooks"
	"ecommerce/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupGeneratedRouterWithConfig(t *testing.T, cfg GeneratedAPIServerConfig, migrateModels ...any) (*gin.Engine, *gorm.DB) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	db := newTestDB(t, migrateModels...)
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = generatedTestJWTSecret
	}

	r := gin.New()
	server, err := NewGeneratedAPIServer(db, nil, cfg)
	require.NoError(t, err)
	apicontract.RegisterHandlers(r, server)
	return r, db
}

func seedCheckoutSession(t *testing.T, db *gorm.DB, userID *uint) models.CheckoutSession {
	t.Helper()
	session := models.CheckoutSession{
		PublicToken: "test-session-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		Status:      models.CheckoutSessionStatusActive,
		ExpiresAt:   time.Now().Add(24 * time.Hour).UTC(),
		LastSeenAt:  time.Now().UTC(),
	}
	if userID != nil {
		session.UserID = userID
	}
	require.NoError(t, db.Create(&session).Error)
	return session
}

func seedCartForUser(t *testing.T, db *gorm.DB, userID uint) models.Cart {
	t.Helper()
	session := seedCheckoutSession(t, db, &userID)
	cart := models.Cart{CheckoutSessionID: session.ID}
	require.NoError(t, db.Create(&cart).Error)
	return cart
}

func cookieValueByName(t *testing.T, w *httptest.ResponseRecorder, name string) string {
	t.Helper()
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	require.FailNow(t, "cookie not found", "missing cookie %s", name)
	return ""
}

func cookieValueIfPresent(w *httptest.ResponseRecorder, name string) string {
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

type checkoutQuoteTestResponse struct {
	SnapshotID *uint `json:"snapshot_id"`
	Valid      bool  `json:"valid"`
}

type checkoutShippingRatesTestResponse struct {
	OrderID    uint   `json:"order_id"`
	SnapshotID uint   `json:"snapshot_id"`
	Provider   string `json:"provider"`
	Rates      []struct {
		ID          uint    `json:"id"`
		ServiceCode string  `json:"service_code"`
		Selected    bool    `json:"selected"`
		Amount      float64 `json:"amount"`
	} `json:"rates"`
}

type adminShippingLabelTestResponse struct {
	Message  string `json:"message"`
	Shipment struct {
		ID                 uint   `json:"id"`
		Status             string `json:"status"`
		ShipmentRateID     uint   `json:"shipment_rate_id"`
		ProviderShipmentID string `json:"provider_shipment_id"`
		TrackingNumber     string `json:"tracking_number"`
	} `json:"shipment"`
}

type checkoutTrackingTestResponse struct {
	OrderID   uint `json:"order_id"`
	Shipments []struct {
		ID                 uint   `json:"id"`
		Status             string `json:"status"`
		ProviderShipmentID string `json:"provider_shipment_id"`
		TrackingEvents     []struct {
			ProviderEventID string `json:"provider_event_id"`
			Status          string `json:"status"`
		} `json:"tracking_events"`
	} `json:"shipments"`
}

type checkoutTaxFinalizeTestResponse struct {
	Message          string  `json:"message"`
	OrderID          uint    `json:"order_id"`
	SnapshotID       uint    `json:"snapshot_id"`
	Provider         string  `json:"provider"`
	InclusivePricing bool    `json:"inclusive_pricing"`
	TotalTax         float64 `json:"total_tax"`
	Lines            []struct {
		LineType       string  `json:"line_type"`
		SnapshotItemID *uint   `json:"snapshot_item_id"`
		TaxAmount      float64 `json:"tax_amount"`
	} `json:"lines"`
}

func quoteCheckoutWithDummyProviders(
	t *testing.T,
	r *gin.Engine,
	checkoutToken, csrfToken string,
) checkoutQuoteTestResponse {
	t.Helper()

	quoteBody := `{"payment_provider_id":"dummy-card","shipping_provider_id":"dummy-ground","tax_provider_id":"dummy-us-tax","payment_data":{"cardholder_name":"Guest Buyer","card_number":"4111111111111111","exp_month":"12","exp_year":"2030"},"shipping_data":{"full_name":"Guest Buyer","line1":"1 Guest Way","city":"Austin","state":"TX","postal_code":"78701","country":"US","service_level":"standard"},"tax_data":{"state":"TX"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/quote", strings.NewReader(quoteBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	req.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	return decodeJSON[checkoutQuoteTestResponse](t, w)
}

func authorizeCheckoutWithSnapshot(
	t *testing.T,
	r *gin.Engine,
	orderID int,
	snapshotID uint,
	checkoutToken, csrfToken, idempotencyKey string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/v1/checkout/orders/%d/payments/authorize", orderID),
		strings.NewReader(fmt.Sprintf(`{"snapshot_id":%d}`, snapshotID)),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	if strings.TrimSpace(idempotencyKey) != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	req.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func quoteShippingRatesWithSnapshot(
	t *testing.T,
	r *gin.Engine,
	orderID int,
	snapshotID uint,
	checkoutToken, csrfToken, idempotencyKey string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/v1/checkout/orders/%d/shipping/rates", orderID),
		strings.NewReader(fmt.Sprintf(`{"snapshot_id":%d}`, snapshotID)),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	if strings.TrimSpace(idempotencyKey) != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	req.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func finalizeCheckoutTaxWithSnapshot(
	t *testing.T,
	r *gin.Engine,
	orderID int,
	snapshotID uint,
	checkoutToken, csrfToken, idempotencyKey string,
	bodySuffix string,
) *httptest.ResponseRecorder {
	t.Helper()

	body := fmt.Sprintf(`{"snapshot_id":%d%s}`, snapshotID, bodySuffix)
	req := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/v1/checkout/orders/%d/tax/finalize", orderID),
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	if strings.TrimSpace(idempotencyKey) != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	req.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func checkoutTrackingRequest(
	t *testing.T,
	r *gin.Engine,
	orderID int,
	checkoutToken string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/checkout/orders/%d/shipping/tracking", orderID), nil)
	req.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func adminLifecycleRequest(
	t *testing.T,
	r *gin.Engine,
	method, path string,
	body string,
	adminToken string,
	idempotencyKey string,
) *httptest.ResponseRecorder {
	t.Helper()

	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(idempotencyKey) != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("X-CSRF-Token", "admin-csrf")
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "admin-csrf"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func webhookRequest(
	t *testing.T,
	r *gin.Engine,
	provider string,
	body string,
	signature string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/"+provider, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(signature) != "" {
		req.Header.Set("X-Dummy-Signature", signature)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func createGuestCheckoutOrder(
	t *testing.T,
	r *gin.Engine,
	checkoutToken, csrfToken, guestEmail string,
) apicontract.Order {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/orders", strings.NewReader(fmt.Sprintf(`{"guest_email":%q}`, guestEmail)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	req.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)
	return decodeJSON[apicontract.Order](t, w)
}

func waitForWebhookEvent(
	t *testing.T,
	db *gorm.DB,
	eventID uint,
	check func(models.WebhookEvent) bool,
) models.WebhookEvent {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var event models.WebhookEvent
		if err := db.First(&event, eventID).Error; err == nil && check(event) {
			return event
		}
		time.Sleep(10 * time.Millisecond)
	}

	var event models.WebhookEvent
	require.NoError(t, db.First(&event, eventID).Error)
	require.True(t, check(event), "webhook event %d did not reach expected state: %+v", eventID, event)
	return event
}

func resetCheckoutProtectionForTest(t *testing.T) {
	t.Helper()

	originalLimit := checkoutSubmissionRateLimit.Limit
	originalWindow := checkoutSubmissionRateLimit.Window
	checkoutSubmissionLimiter.reset()

	t.Cleanup(func() {
		checkoutSubmissionRateLimit.Limit = originalLimit
		checkoutSubmissionRateLimit.Window = originalWindow
		checkoutSubmissionLimiter.reset()
	})
}

func issueBearerTokenWithRole(t *testing.T, secret, subject, role string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":   subject,
		"email": subject + "@example.com",
		"role":  role,
		"exp":   time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return signed
}

func seedUser(t *testing.T, db *gorm.DB, subject, username, email, role string) models.User {
	t.Helper()
	user := models.User{
		Subject:  subject,
		Username: username,
		Email:    email,
		Role:     role,
		Currency: "USD",
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func seedProduct(t *testing.T, db *gorm.DB, sku, name string, price float64, stock int) models.Product {
	t.Helper()
	product := models.Product{
		SKU:         sku,
		Name:        name,
		Description: name + " description",
		Price:       models.MoneyFromFloat(price),
		Stock:       stock,
	}
	require.NoError(t, db.Create(&product).Error)
	variant := models.ProductVariant{
		ProductID:   product.ID,
		SKU:         sku + "-default",
		Title:       name,
		Price:       models.MoneyFromFloat(price),
		Stock:       stock,
		Position:    1,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&variant).Error)
	require.NoError(t, db.Model(&product).Update("default_variant_id", variant.ID).Error)
	product.DefaultVariantID = &variant.ID
	return product
}

func requireDefaultVariantID(t *testing.T, product models.Product) uint {
	t.Helper()
	require.NotNil(t, product.DefaultVariantID)
	return *product.DefaultVariantID
}

func singleVariantProductUpsertPayload(sku, name, description string, price float64, stock int) map[string]any {
	return map[string]any{
		"sku":                 sku,
		"name":                name,
		"description":         description,
		"images":              []string{},
		"related_product_ids": []int{},
		"options":             []map[string]any{},
		"variants": []map[string]any{
			{
				"sku":          sku,
				"title":        name,
				"price":        price,
				"stock":        stock,
				"is_published": true,
				"selections":   []map[string]any{},
			},
		},
		"attributes": []map[string]any{},
		"seo":        map[string]any{},
	}
}

func TestGeneratedAdminAuthParity(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{})
	_ = seedUser(t, db, "sub-admin", "admin", "admin@example.com", "admin")
	_ = seedUser(t, db, "sub-customer", "cust", "cust@example.com", "customer")

	unauthReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=1&limit=20", nil)
	unauthW := httptest.NewRecorder()
	r.ServeHTTP(unauthW, unauthReq)
	assert.Equal(t, http.StatusUnauthorized, unauthW.Code)

	customerReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=1&limit=20", nil)
	customerReq.Header.Set("Authorization", "Bearer "+issueBearerTokenWithRole(t, generatedTestJWTSecret, "sub-customer", "customer"))
	customerW := httptest.NewRecorder()
	r.ServeHTTP(customerW, customerReq)
	assert.Equal(t, http.StatusForbidden, customerW.Code)

	adminReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=1&limit=20", nil)
	adminReq.Header.Set("Authorization", "Bearer "+issueBearerTokenWithRole(t, generatedTestJWTSecret, "sub-admin", "admin"))
	adminW := httptest.NewRecorder()
	r.ServeHTTP(adminW, adminReq)
	assert.Equal(t, http.StatusOK, adminW.Code)
}

func TestAdminCheckoutPluginManagement(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{}, &models.CheckoutProviderSetting{})
	admin := seedUser(t, db, "sub-admin-plugins", "admin-plugins", "admin-plugins@example.com", "admin")
	customer := seedUser(t, db, "sub-customer-plugins", "customer-plugins", "customer-plugins@example.com", "customer")

	unauthList := performJSONRequest(t, r, http.MethodGet, "/api/v1/admin/checkout/plugins", nil, "")
	require.Equal(t, http.StatusUnauthorized, unauthList.Code)

	customerList := performJSONRequest(t, r, http.MethodGet, "/api/v1/admin/checkout/plugins", nil, issueBearerTokenWithRole(t, generatedTestJWTSecret, customer.Subject, customer.Role))
	require.Equal(t, http.StatusForbidden, customerList.Code)

	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)
	adminList := performJSONRequest(t, r, http.MethodGet, "/api/v1/admin/checkout/plugins", nil, adminToken)
	require.Equal(t, http.StatusOK, adminList.Code)

	catalog := decodeJSON[apicontract.CheckoutPluginCatalog](t, adminList)
	activeTaxes := 0
	disabledTaxID := ""
	activeTaxID := ""
	for _, tax := range catalog.Tax {
		if tax.Enabled {
			activeTaxes++
			activeTaxID = tax.Id
		} else {
			disabledTaxID = tax.Id
		}
	}
	require.Equal(t, 1, activeTaxes)
	require.NotEmpty(t, disabledTaxID)

	activateResp := performJSONRequest(
		t,
		r,
		http.MethodPatch,
		fmt.Sprintf("/api/v1/admin/checkout/plugins/tax/%s", disabledTaxID),
		map[string]any{"enabled": true},
		adminToken,
	)
	require.Equal(t, http.StatusOK, activateResp.Code)
	updatedCatalog := decodeJSON[apicontract.CheckoutPluginCatalog](t, activateResp)

	activeTaxes = 0
	for _, tax := range updatedCatalog.Tax {
		if tax.Enabled {
			activeTaxes++
			require.Equal(t, disabledTaxID, tax.Id)
		}
	}
	require.Equal(t, 1, activeTaxes)

	disableActiveResp := performJSONRequest(
		t,
		r,
		http.MethodPatch,
		fmt.Sprintf("/api/v1/admin/checkout/plugins/tax/%s", disabledTaxID),
		map[string]any{"enabled": false},
		adminToken,
	)
	require.Equal(t, http.StatusBadRequest, disableActiveResp.Code)

	disableOldActiveResp := performJSONRequest(
		t,
		r,
		http.MethodPatch,
		fmt.Sprintf("/api/v1/admin/checkout/plugins/tax/%s", activeTaxID),
		map[string]any{"enabled": false},
		adminToken,
	)
	require.Equal(t, http.StatusOK, disableOldActiveResp.Code)
}

func TestGeneratedMeAuthParity(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{}, &models.Cart{}, &models.CartItem{}, &models.Product{})
	_ = seedUser(t, db, "sub-me", "me-user", "me@example.com", "customer")

	unauthReq := httptest.NewRequest(http.MethodGet, "/api/v1/me/cart", nil)
	unauthW := httptest.NewRecorder()
	r.ServeHTTP(unauthW, unauthReq)
	assert.Equal(t, http.StatusUnauthorized, unauthW.Code)

	authReq := httptest.NewRequest(http.MethodGet, "/api/v1/me/cart", nil)
	authReq.Header.Set("Authorization", "Bearer "+issueBearerTokenWithRole(t, generatedTestJWTSecret, "sub-me", "customer"))
	authW := httptest.NewRecorder()
	r.ServeHTTP(authW, authReq)
	assert.Equal(t, http.StatusOK, authW.Code)

	invalidReq := httptest.NewRequest(http.MethodGet, "/api/v1/me/cart", nil)
	invalidReq.Header.Set("Authorization", "Bearer invalid.token.value")
	invalidW := httptest.NewRecorder()
	r.ServeHTTP(invalidW, invalidReq)
	assert.Equal(t, http.StatusUnauthorized, invalidW.Code)
}

func TestGeneratedCSRFMiddlewareMatrix(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{}, &models.Product{}, &models.ProductVariant{}, &models.Cart{}, &models.CartItem{})
	user := seedUser(t, db, "sub-csrf", "csrf-user", "csrf@example.com", "customer")
	product := seedProduct(t, db, "sku-csrf", "CSRF Product", 12.99, 10)
	variantID := requireDefaultVariantID(t, product)
	token := issueBearerTokenWithRole(t, generatedTestJWTSecret, user.Subject, user.Role)

	bearerReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart", strings.NewReader(`{"product_variant_id":`+strconv.Itoa(int(variantID))+`,"quantity":1}`))
	bearerReq.Header.Set("Authorization", "Bearer "+token)
	bearerReq.Header.Set("Content-Type", "application/json")
	bearerW := httptest.NewRecorder()
	r.ServeHTTP(bearerW, bearerReq)
	assert.Equal(t, http.StatusOK, bearerW.Code)

	sessionNoCsrfReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart", strings.NewReader(`{"product_variant_id":`+strconv.Itoa(int(variantID))+`,"quantity":1}`))
	sessionNoCsrfReq.Header.Set("Content-Type", "application/json")
	sessionNoCsrfReq.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	sessionNoCsrfW := httptest.NewRecorder()
	r.ServeHTTP(sessionNoCsrfW, sessionNoCsrfReq)
	assert.Equal(t, http.StatusForbidden, sessionNoCsrfW.Code)

	sessionCsrfReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart", strings.NewReader(`{"product_variant_id":`+strconv.Itoa(int(variantID))+`,"quantity":2}`))
	sessionCsrfReq.Header.Set("Content-Type", "application/json")
	sessionCsrfReq.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	sessionCsrfReq.AddCookie(&http.Cookie{Name: "csrf_token", Value: "csrf-123"})
	sessionCsrfReq.Header.Set("X-CSRF-Token", "csrf-123")
	sessionCsrfW := httptest.NewRecorder()
	r.ServeHTTP(sessionCsrfW, sessionCsrfReq)
	assert.Equal(t, http.StatusOK, sessionCsrfW.Code)

	invalidDataReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart", strings.NewReader(`{"product_variant_id":`+strconv.Itoa(int(variantID))+`,"quantity":0}`))
	invalidDataReq.Header.Set("Content-Type", "application/json")
	invalidDataReq.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	invalidDataReq.AddCookie(&http.Cookie{Name: "csrf_token", Value: "csrf-123"})
	invalidDataReq.Header.Set("X-CSRF-Token", "csrf-123")
	invalidDataW := httptest.NewRecorder()
	r.ServeHTTP(invalidDataW, invalidDataReq)
	assert.Equal(t, http.StatusBadRequest, invalidDataW.Code)
}

func TestCheckoutCartGuestSessionFlow(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.Product{}, &models.ProductVariant{}, &models.Cart{}, &models.CartItem{}, &models.CheckoutSession{})
	product := seedProduct(t, db, "sku-guest-cart", "Guest Cart Product", 18.25, 10)
	variantID := requireDefaultVariantID(t, product)

	firstPostReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/cart/items", strings.NewReader(`{"product_variant_id":`+strconv.Itoa(int(variantID))+`,"quantity":1}`))
	firstPostReq.Header.Set("Content-Type", "application/json")
	firstPostW := httptest.NewRecorder()
	r.ServeHTTP(firstPostW, firstPostReq)
	require.Equal(t, http.StatusOK, firstPostW.Code)
	firstCart := decodeJSON[cartResponse](t, firstPostW)
	require.Len(t, firstCart.Items, 1)
	assert.Equal(t, 1, firstCart.Items[0].Quantity)
	firstCheckoutToken := cookieValueByName(t, firstPostW, checkoutSessionCookieName)
	firstCSRFToken := cookieValueByName(t, firstPostW, csrfCookieName)
	assert.NotEmpty(t, firstCheckoutToken)
	assert.NotEmpty(t, firstCSRFToken)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/checkout/cart", nil)
	getReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: firstCheckoutToken})
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)
	require.Equal(t, http.StatusOK, getW.Code)
	csrfToken := firstCSRFToken

	postNoCSRFReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/cart/items", strings.NewReader(`{"product_variant_id":`+strconv.Itoa(int(variantID))+`,"quantity":1}`))
	postNoCSRFReq.Header.Set("Content-Type", "application/json")
	postNoCSRFReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: firstCheckoutToken})
	postNoCSRFW := httptest.NewRecorder()
	r.ServeHTTP(postNoCSRFW, postNoCSRFReq)
	require.Equal(t, http.StatusForbidden, postNoCSRFW.Code)

	postReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/cart/items", strings.NewReader(`{"product_variant_id":`+strconv.Itoa(int(variantID))+`,"quantity":2}`))
	postReq.Header.Set("Content-Type", "application/json")
	postReq.Header.Set("X-CSRF-Token", csrfToken)
	postReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: firstCheckoutToken})
	postReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	postW := httptest.NewRecorder()
	r.ServeHTTP(postW, postReq)
	require.Equal(t, http.StatusOK, postW.Code)
	cart := decodeJSON[cartResponse](t, postW)
	require.Len(t, cart.Items, 1)
	assert.Equal(t, 3, cart.Items[0].Quantity)
	assert.Equal(t, 0, cart.UserID)

	reloadReq := httptest.NewRequest(http.MethodGet, "/api/v1/checkout/cart", nil)
	reloadReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: firstCheckoutToken})
	reloadW := httptest.NewRecorder()
	r.ServeHTTP(reloadW, reloadReq)
	require.Equal(t, http.StatusOK, reloadW.Code)
	reloaded := decodeJSON[cartResponse](t, reloadW)
	require.Len(t, reloaded.Items, 1)
	assert.Equal(t, 3, reloaded.Items[0].Quantity)
}

func TestCheckoutCartInvalidTokenRotatesSession(t *testing.T) {
	r, _ := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.Cart{}, &models.CartItem{}, &models.CheckoutSession{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/checkout/cart", nil)
	req.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: "invalid-session-token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.NotEqual(t, "invalid-session-token", cookieValueByName(t, w, checkoutSessionCookieName))
}

func TestCheckoutCartMutationInvalidTokenRotatesSession(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.Product{}, &models.ProductVariant{}, &models.Cart{}, &models.CartItem{}, &models.CheckoutSession{})
	product := seedProduct(t, db, "sku-invalid-mutation", "Invalid Mutation Product", 14.75, 10)
	variantID := requireDefaultVariantID(t, product)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/cart/items", strings.NewReader(`{"product_variant_id":`+strconv.Itoa(int(variantID))+`,"quantity":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "csrf-rotate")
	req.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: "invalid-session-token"})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-rotate"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.NotEqual(t, "invalid-session-token", cookieValueByName(t, w, checkoutSessionCookieName))

	cart := decodeJSON[cartResponse](t, w)
	require.Len(t, cart.Items, 1)
	assert.Equal(t, 1, cart.Items[0].Quantity)

	var sessionCount int64
	require.NoError(t, db.Model(&models.CheckoutSession{}).Count(&sessionCount).Error)
	assert.EqualValues(t, 1, sessionCount)
}

func TestCheckoutGuestToggleBlocksGuestsButAllowsAuthenticatedCheckout(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{}, &models.WebsiteSettings{}, &models.Cart{}, &models.CartItem{}, &models.CheckoutSession{})
	user := seedUser(t, db, "sub-guest-toggle", "guest-toggle", "guest-toggle@example.com", "customer")
	require.NoError(t, db.Select("*").Save(&models.WebsiteSettings{
		ID:                 models.WebsiteSettingsSingletonID,
		AllowGuestCheckout: false,
	}).Error)

	guestReq := httptest.NewRequest(http.MethodGet, "/api/v1/checkout/cart", nil)
	guestW := httptest.NewRecorder()
	r.ServeHTTP(guestW, guestReq)
	require.Equal(t, http.StatusForbidden, guestW.Code)
	guestBody := decodeJSON[map[string]any](t, guestW)
	assert.Equal(t, guestCheckoutDisabledCode, guestBody["code"])

	authReq := httptest.NewRequest(http.MethodGet, "/api/v1/checkout/cart", nil)
	authReq.Header.Set("Authorization", "Bearer "+issueBearerTokenWithRole(t, generatedTestJWTSecret, user.Subject, user.Role))
	authW := httptest.NewRecorder()
	r.ServeHTTP(authW, authReq)
	require.Equal(t, http.StatusOK, authW.Code)
}

func TestCheckoutCartAuthenticatedWithoutCookieReusesLinkedSession(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{}, &models.Product{}, &models.ProductVariant{}, &models.Cart{}, &models.CartItem{}, &models.CheckoutSession{})
	user := seedUser(t, db, "sub-checkout-link", "checkout-link", "checkout-link@example.com", "customer")
	product := seedProduct(t, db, "sku-checkout-link", "Checkout Link Product", 16.5, 10)
	variantID := requireDefaultVariantID(t, product)
	token := issueBearerTokenWithRole(t, generatedTestJWTSecret, user.Subject, user.Role)

	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/cart/items", strings.NewReader(`{"product_variant_id":`+strconv.Itoa(int(variantID))+`,"quantity":1}`))
	addReq.Header.Set("Authorization", "Bearer "+token)
	addReq.Header.Set("Content-Type", "application/json")
	addW := httptest.NewRecorder()
	r.ServeHTTP(addW, addReq)
	require.Equal(t, http.StatusOK, addW.Code)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/checkout/cart", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)
	require.Equal(t, http.StatusOK, getW.Code)
	cart := decodeJSON[cartResponse](t, getW)
	require.Len(t, cart.Items, 1)
	assert.Equal(t, int(user.ID), cart.UserID)

	var session models.CheckoutSession
	require.NoError(t, db.Where("user_id = ?", user.ID).First(&session).Error)
	assert.NotEmpty(t, session.PublicToken)
}

func TestCheckoutCartAuthenticatedPrefersLinkedSessionOverGuestCookie(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{}, &models.Product{}, &models.ProductVariant{}, &models.Cart{}, &models.CartItem{}, &models.CheckoutSession{})
	user := seedUser(t, db, "sub-checkout-prefers-user", "checkout-prefers-user", "checkout-prefers-user@example.com", "customer")
	product := seedProduct(t, db, "sku-checkout-prefers-user", "Checkout Prefers User Product", 11.5, 10)
	variantID := requireDefaultVariantID(t, product)
	token := issueBearerTokenWithRole(t, generatedTestJWTSecret, user.Subject, user.Role)

	userSession := seedCheckoutSession(t, db, &user.ID)
	userCart := models.Cart{CheckoutSessionID: userSession.ID}
	require.NoError(t, db.Create(&userCart).Error)
	require.NoError(t, db.Create(&models.CartItem{
		CartID:           userCart.ID,
		ProductVariantID: variantID,
		Quantity:         1,
	}).Error)

	guestSession := seedCheckoutSession(t, db, nil)
	guestCart := models.Cart{CheckoutSessionID: guestSession.ID}
	require.NoError(t, db.Create(&guestCart).Error)
	require.NoError(t, db.Create(&models.CartItem{
		CartID:           guestCart.ID,
		ProductVariantID: variantID,
		Quantity:         3,
	}).Error)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/checkout/cart", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: guestSession.PublicToken})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	cart := decodeJSON[cartResponse](t, w)
	require.Len(t, cart.Items, 1)
	assert.Equal(t, 1, cart.Items[0].Quantity)
	assert.Equal(t, int(user.ID), cart.UserID)
	assert.Equal(t, userSession.PublicToken, cookieValueByName(t, w, checkoutSessionCookieName))

	var reloadedGuestSession models.CheckoutSession
	require.NoError(t, db.First(&reloadedGuestSession, guestSession.ID).Error)
	assert.Nil(t, reloadedGuestSession.UserID)
}

func TestCheckoutCartSummaryDoesNotCreateGuestState(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.Cart{}, &models.CartItem{}, &models.CheckoutSession{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/checkout/cart/summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	body := decodeJSON[struct {
		ItemCount int `json:"item_count"`
	}](t, w)
	assert.Equal(t, 0, body.ItemCount)
	assert.Empty(t, cookieValueIfPresent(w, checkoutSessionCookieName))
	assert.Empty(t, cookieValueIfPresent(w, csrfCookieName))

	var sessionCount int64
	require.NoError(t, db.Model(&models.CheckoutSession{}).Count(&sessionCount).Error)
	assert.EqualValues(t, 0, sessionCount)

	var cartCount int64
	require.NoError(t, db.Model(&models.Cart{}).Count(&cartCount).Error)
	assert.EqualValues(t, 0, cartCount)
}

func TestCheckoutOrderGuestFlowRequiresEmailAndConvertsSession(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
	)
	product := seedProduct(t, db, "sku-guest-order", "Guest Order Product", 24.5, 10)
	variantID := requireDefaultVariantID(t, product)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/checkout/cart", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)
	require.Equal(t, http.StatusOK, getW.Code)

	checkoutToken := cookieValueByName(t, getW, checkoutSessionCookieName)
	csrfToken := cookieValueByName(t, getW, csrfCookieName)

	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/cart/items", strings.NewReader(`{"product_variant_id":`+strconv.Itoa(int(variantID))+`,"quantity":2}`))
	addReq.Header.Set("Content-Type", "application/json")
	addReq.Header.Set("X-CSRF-Token", csrfToken)
	addReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	addReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	addW := httptest.NewRecorder()
	r.ServeHTTP(addW, addReq)
	require.Equal(t, http.StatusOK, addW.Code)

	missingEmailReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/orders", strings.NewReader(`{}`))
	missingEmailReq.Header.Set("Content-Type", "application/json")
	missingEmailReq.Header.Set("X-CSRF-Token", csrfToken)
	missingEmailReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	missingEmailReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	missingEmailW := httptest.NewRecorder()
	r.ServeHTTP(missingEmailW, missingEmailReq)
	require.Equal(t, http.StatusBadRequest, missingEmailW.Code)
	assert.Contains(t, missingEmailW.Body.String(), "Guest email is required")

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/orders", strings.NewReader(`{"guest_email":"guest@example.com"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-CSRF-Token", csrfToken)
	createReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	createReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code)
	order := decodeJSON[apicontract.Order](t, createW)
	require.Nil(t, order.UserId)
	require.NotNil(t, order.GuestEmail)
	require.Equal(t, "guest@example.com", string(*order.GuestEmail))
	require.NotNil(t, order.ConfirmationToken)
	require.NotZero(t, order.CheckoutSessionId)

	quote := quoteCheckoutWithDummyProviders(t, r, checkoutToken, csrfToken)
	require.True(t, quote.Valid)
	require.NotNil(t, quote.SnapshotID)

	payW := authorizeCheckoutWithSnapshot(t, r, order.Id, *quote.SnapshotID, checkoutToken, csrfToken, "")
	require.Equal(t, http.StatusOK, payW.Code)

	var payBody struct {
		Message string            `json:"message"`
		Order   apicontract.Order `json:"order"`
	}
	require.NoError(t, json.Unmarshal(payW.Body.Bytes(), &payBody))
	assert.Equal(t, "Order submitted and pending confirmation", payBody.Message)
	require.Nil(t, payBody.Order.UserId)
	require.NotNil(t, payBody.Order.GuestEmail)
	assert.Equal(t, "guest@example.com", string(*payBody.Order.GuestEmail))

	var cartItems int64
	require.NoError(t, db.Model(&models.CartItem{}).Count(&cartItems).Error)
	assert.EqualValues(t, 0, cartItems)

	var session models.CheckoutSession
	require.NoError(t, db.First(&session, order.CheckoutSessionId).Error)
	assert.Equal(t, models.CheckoutSessionStatusConverted, session.Status)
	require.NotNil(t, session.GuestEmail)
	assert.Equal(t, "guest@example.com", *session.GuestEmail)
}

func TestAdminOrdersListSupportsGuestOrders(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
	)
	admin := seedUser(t, db, "sub-admin-guest-orders", "admin-guest-orders", "admin-guest-orders@example.com", "admin")
	email := "guest-order@example.com"
	session := seedCheckoutSession(t, db, nil)
	order := models.Order{
		CheckoutSessionID: session.ID,
		GuestEmail:        &email,
		ConfirmationToken: func() *string { value := "confirm-guest"; return &value }(),
		Status:            models.StatusPending,
		Total:             models.MoneyFromFloat(17.25),
	}
	require.NoError(t, db.Create(&order).Error)

	resp := performJSONRequest(t, r, http.MethodGet, "/api/v1/admin/orders?q=guest-order@example.com", nil, issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role))
	require.Equal(t, http.StatusOK, resp.Code)

	var payload struct {
		Data []apicontract.Order `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
	require.Len(t, payload.Data, 1)
	assert.Nil(t, payload.Data[0].UserId)
	require.NotNil(t, payload.Data[0].GuestEmail)
	assert.Equal(t, email, string(*payload.Data[0].GuestEmail))
}

func TestCheckoutOrderCreateIdempotencyReplaysResponse(t *testing.T) {
	resetCheckoutProtectionForTest(t)

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
		&models.IdempotencyKey{},
	)
	product := seedProduct(t, db, "sku-idempotent-create", "Idempotent Create Product", 12.5, 10)
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

	body := `{"guest_email":"idempotent@example.com"}`
	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/orders", strings.NewReader(body))
	firstReq.Header.Set("Content-Type", "application/json")
	firstReq.Header.Set("X-CSRF-Token", csrfToken)
	firstReq.Header.Set("Idempotency-Key", "create-order-key")
	firstReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	firstReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	firstW := httptest.NewRecorder()
	r.ServeHTTP(firstW, firstReq)
	require.Equal(t, http.StatusCreated, firstW.Code)
	firstOrder := decodeJSON[apicontract.Order](t, firstW)

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/orders", strings.NewReader(body))
	secondReq.Header.Set("Content-Type", "application/json")
	secondReq.Header.Set("X-CSRF-Token", csrfToken)
	secondReq.Header.Set("Idempotency-Key", "create-order-key")
	secondReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	secondReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	secondW := httptest.NewRecorder()
	r.ServeHTTP(secondW, secondReq)
	require.Equal(t, http.StatusCreated, secondW.Code)
	secondOrder := decodeJSON[apicontract.Order](t, secondW)

	assert.Equal(t, firstOrder.Id, secondOrder.Id)

	var orderCount int64
	require.NoError(t, db.Model(&models.Order{}).Count(&orderCount).Error)
	assert.EqualValues(t, 1, orderCount)
}

func TestCheckoutOrderCreateReusesExistingOpenOrderWithoutIdempotencyKey(t *testing.T) {
	resetCheckoutProtectionForTest(t)

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
	)
	product := seedProduct(t, db, "sku-reuse-open-order", "Reuse Open Order Product", 15.5, 10)
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

	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/orders", strings.NewReader(`{"guest_email":"first@example.com"}`))
	firstReq.Header.Set("Content-Type", "application/json")
	firstReq.Header.Set("X-CSRF-Token", csrfToken)
	firstReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	firstReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	firstW := httptest.NewRecorder()
	r.ServeHTTP(firstW, firstReq)
	require.Equal(t, http.StatusCreated, firstW.Code)
	firstOrder := decodeJSON[apicontract.Order](t, firstW)

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/orders", strings.NewReader(`{"guest_email":"second@example.com"}`))
	secondReq.Header.Set("Content-Type", "application/json")
	secondReq.Header.Set("X-CSRF-Token", csrfToken)
	secondReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	secondReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	secondW := httptest.NewRecorder()
	r.ServeHTTP(secondW, secondReq)
	require.Equal(t, http.StatusOK, secondW.Code)
	secondOrder := decodeJSON[apicontract.Order](t, secondW)

	assert.Equal(t, firstOrder.Id, secondOrder.Id)
	require.NotNil(t, secondOrder.GuestEmail)
	assert.Equal(t, "second@example.com", string(*secondOrder.GuestEmail))

	var orderCount int64
	require.NoError(t, db.Model(&models.Order{}).Count(&orderCount).Error)
	assert.EqualValues(t, 1, orderCount)

	var storedOrder models.Order
	require.NoError(t, db.First(&storedOrder, firstOrder.Id).Error)
	require.NotNil(t, storedOrder.GuestEmail)
	assert.Equal(t, "second@example.com", *storedOrder.GuestEmail)
}

func TestClaimGuestOrderLinksOrderToAuthenticatedUser(t *testing.T) {
	resetCheckoutProtectionForTest(t)

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
	)
	user := seedUser(t, db, "sub-claim-order", "claim-user", "claim-user@example.com", "customer")
	session := seedCheckoutSession(t, db, nil)
	email := "claim-me@example.com"
	token := "claim-token"
	order := models.Order{
		CheckoutSessionID: session.ID,
		GuestEmail:        &email,
		ConfirmationToken: &token,
		Status:            models.StatusPending,
		Total:             models.MoneyFromFloat(18.75),
	}
	require.NoError(t, db.Create(&order).Error)

	resp := performJSONRequest(
		t,
		r,
		http.MethodPost,
		"/api/v1/me/orders/claim",
		map[string]any{
			"email":              email,
			"confirmation_token": token,
		},
		issueBearerTokenWithRole(t, generatedTestJWTSecret, user.Subject, user.Role),
	)
	require.Equal(t, http.StatusOK, resp.Code)

	var claimBody struct {
		Message string            `json:"message"`
		Order   apicontract.Order `json:"order"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &claimBody))
	require.NotNil(t, claimBody.Order.UserId)
	assert.Equal(t, int(user.ID), *claimBody.Order.UserId)

	var reloaded models.Order
	require.NoError(t, db.First(&reloaded, order.ID).Error)
	require.NotNil(t, reloaded.UserID)
	assert.Equal(t, user.ID, *reloaded.UserID)
	assert.True(t, reloaded.ClaimedAt.Valid())

	var reloadedSession models.CheckoutSession
	require.NoError(t, db.First(&reloadedSession, session.ID).Error)
	require.NotNil(t, reloadedSession.UserID)
	assert.Equal(t, user.ID, *reloadedSession.UserID)
}

func TestCheckoutOrderCreateRejectsIdempotencyPayloadMismatch(t *testing.T) {
	resetCheckoutProtectionForTest(t)

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
		&models.IdempotencyKey{},
	)
	product := seedProduct(t, db, "sku-idempotent-conflict", "Idempotent Conflict Product", 8.5, 10)
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

	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/orders", strings.NewReader(`{"guest_email":"first@example.com"}`))
	firstReq.Header.Set("Content-Type", "application/json")
	firstReq.Header.Set("X-CSRF-Token", csrfToken)
	firstReq.Header.Set("Idempotency-Key", "create-order-conflict")
	firstReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	firstReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	firstW := httptest.NewRecorder()
	r.ServeHTTP(firstW, firstReq)
	require.Equal(t, http.StatusCreated, firstW.Code)

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/orders", strings.NewReader(`{"guest_email":"second@example.com"}`))
	secondReq.Header.Set("Content-Type", "application/json")
	secondReq.Header.Set("X-CSRF-Token", csrfToken)
	secondReq.Header.Set("Idempotency-Key", "create-order-conflict")
	secondReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	secondReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	secondW := httptest.NewRecorder()
	r.ServeHTTP(secondW, secondReq)
	require.Equal(t, http.StatusConflict, secondW.Code)

	body := decodeJSON[map[string]any](t, secondW)
	assert.Equal(t, idempotencyConflictCode, body["code"])
}

func TestCheckoutOrderPaymentIdempotencyReplaysAfterSessionConversion(t *testing.T) {
	resetCheckoutProtectionForTest(t)

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
		&models.IdempotencyKey{},
	)
	product := seedProduct(t, db, "sku-idempotent-pay", "Idempotent Pay Product", 21, 10)
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

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/orders", strings.NewReader(`{"guest_email":"pay-idempotent@example.com"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-CSRF-Token", csrfToken)
	createReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	createReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code)
	order := decodeJSON[apicontract.Order](t, createW)

	quote := quoteCheckoutWithDummyProviders(t, r, checkoutToken, csrfToken)
	require.True(t, quote.Valid)
	require.NotNil(t, quote.SnapshotID)

	firstPayW := authorizeCheckoutWithSnapshot(t, r, order.Id, *quote.SnapshotID, checkoutToken, csrfToken, "payment-key")
	require.Equal(t, http.StatusOK, firstPayW.Code)

	secondPayW := authorizeCheckoutWithSnapshot(t, r, order.Id, *quote.SnapshotID, checkoutToken, csrfToken, "payment-key")
	require.Equal(t, http.StatusOK, secondPayW.Code)

	var firstPayload struct {
		Order apicontract.Order `json:"order"`
	}
	var secondPayload struct {
		Order apicontract.Order `json:"order"`
	}
	require.NoError(t, json.Unmarshal(firstPayW.Body.Bytes(), &firstPayload))
	require.NoError(t, json.Unmarshal(secondPayW.Body.Bytes(), &secondPayload))
	assert.Equal(t, firstPayload.Order.Id, secondPayload.Order.Id)

	var sessionCount int64
	require.NoError(t, db.Model(&models.CheckoutSession{}).Count(&sessionCount).Error)
	assert.EqualValues(t, 1, sessionCount)

	var intentCount int64
	require.NoError(t, db.Model(&models.PaymentIntent{}).Count(&intentCount).Error)
	assert.EqualValues(t, 1, intentCount)

	var txnCount int64
	require.NoError(t, db.Model(&models.PaymentTransaction{}).Count(&txnCount).Error)
	assert.EqualValues(t, 1, txnCount)
}

func TestCheckoutOrderPaymentRejectsExpiredSnapshot(t *testing.T) {
	resetCheckoutProtectionForTest(t)

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
	)
	product := seedProduct(t, db, "sku-expired-snapshot", "Expired Snapshot Product", 12.5, 10)
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

	quote := quoteCheckoutWithDummyProviders(t, r, checkoutToken, csrfToken)
	require.True(t, quote.Valid)
	require.NotNil(t, quote.SnapshotID)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/orders", strings.NewReader(`{"guest_email":"expired-snapshot@example.com"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-CSRF-Token", csrfToken)
	createReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	createReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code)
	order := decodeJSON[apicontract.Order](t, createW)

	require.NoError(t, db.Model(&models.OrderCheckoutSnapshot{}).
		Where("id = ?", *quote.SnapshotID).
		Update("expires_at", time.Now().Add(-time.Minute).UTC()).Error)

	payW := authorizeCheckoutWithSnapshot(t, r, order.Id, *quote.SnapshotID, checkoutToken, csrfToken, "")
	require.Equal(t, http.StatusBadRequest, payW.Code)
	assert.Contains(t, payW.Body.String(), "snapshot has expired")
}

func TestCheckoutOrderPaymentRejectsChangedOrderTotals(t *testing.T) {
	resetCheckoutProtectionForTest(t)

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
	)
	product := seedProduct(t, db, "sku-mismatch-snapshot", "Mismatch Snapshot Product", 16.25, 10)
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

	quote := quoteCheckoutWithDummyProviders(t, r, checkoutToken, csrfToken)
	require.True(t, quote.Valid)
	require.NotNil(t, quote.SnapshotID)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/orders", strings.NewReader(`{"guest_email":"snapshot-mismatch@example.com"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-CSRF-Token", csrfToken)
	createReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	createReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code)
	order := decodeJSON[apicontract.Order](t, createW)

	require.NoError(t, db.Model(&models.Order{}).
		Where("id = ?", order.Id).
		Update("total", models.MoneyFromFloat(99.99)).Error)

	payW := authorizeCheckoutWithSnapshot(t, r, order.Id, *quote.SnapshotID, checkoutToken, csrfToken, "")
	require.Equal(t, http.StatusConflict, payW.Code)
	assert.Contains(t, payW.Body.String(), "snapshot no longer matches the order")
}

func TestCheckoutShippingRatesRejectAlreadyBoundSnapshot(t *testing.T) {
	resetCheckoutProtectionForTest(t)

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
		&models.IdempotencyKey{},
	)
	product := seedProduct(t, db, "sku-bound-shipping-snapshot", "Bound Shipping Snapshot Product", 19.50, 5)
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

	order := createGuestCheckoutOrder(t, r, checkoutToken, csrfToken, "bound-shipping-snapshot@example.com")
	quote := quoteCheckoutWithDummyProviders(t, r, checkoutToken, csrfToken)
	require.NotNil(t, quote.SnapshotID)

	var storedOrder models.Order
	require.NoError(t, db.First(&storedOrder, order.Id).Error)

	boundOrder := models.Order{
		CheckoutSessionID: storedOrder.CheckoutSessionID,
		Total:             storedOrder.Total,
		Status:            models.StatusPending,
	}
	require.NoError(t, db.Create(&boundOrder).Error)
	require.NoError(t, db.Model(&models.OrderCheckoutSnapshot{}).
		Where("id = ?", *quote.SnapshotID).
		Update("order_id", boundOrder.ID).Error)

	ratesW := quoteShippingRatesWithSnapshot(t, r, order.Id, *quote.SnapshotID, checkoutToken, csrfToken, "bound-shipping-rates-key")
	require.Equal(t, http.StatusConflict, ratesW.Code)
	assert.Contains(t, ratesW.Body.String(), "snapshot is already bound to another order")
}

func TestAdminCapturePaymentReplaysAndBlocksDoubleCapture(t *testing.T) {
	resetCheckoutProtectionForTest(t)

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
		&models.IdempotencyKey{},
	)
	admin := seedUser(t, db, "sub-admin-capture", "admin-capture", "admin-capture@example.com", "admin")
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)
	product := seedProduct(t, db, "sku-admin-capture", "Admin Capture Product", 24.5, 12)
	variantID := requireDefaultVariantID(t, product)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/checkout/cart", nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)
	require.Equal(t, http.StatusOK, getW.Code)

	checkoutToken := cookieValueByName(t, getW, checkoutSessionCookieName)
	csrfToken := cookieValueByName(t, getW, csrfCookieName)

	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/cart/items", strings.NewReader(`{"product_variant_id":`+strconv.Itoa(int(variantID))+`,"quantity":2}`))
	addReq.Header.Set("Content-Type", "application/json")
	addReq.Header.Set("X-CSRF-Token", csrfToken)
	addReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	addReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	addW := httptest.NewRecorder()
	r.ServeHTTP(addW, addReq)
	require.Equal(t, http.StatusOK, addW.Code)

	order := createGuestCheckoutOrder(t, r, checkoutToken, csrfToken, "capture-admin@example.com")
	quote := quoteCheckoutWithDummyProviders(t, r, checkoutToken, csrfToken)
	require.NotNil(t, quote.SnapshotID)
	authorizeW := authorizeCheckoutWithSnapshot(t, r, order.Id, *quote.SnapshotID, checkoutToken, csrfToken, "authorize-admin-capture")
	require.Equal(t, http.StatusOK, authorizeW.Code)

	var intent models.PaymentIntent
	require.NoError(t, db.First(&intent).Error)

	capturePath := fmt.Sprintf("/api/v1/admin/orders/%d/payments/%d/capture", order.Id, intent.ID)
	firstCaptureW := adminLifecycleRequest(t, r, http.MethodPost, capturePath, "", adminToken, "capture-key")
	require.Equal(t, http.StatusOK, firstCaptureW.Code)
	firstCapture := decodeJSON[apicontract.AdminOrderPaymentLifecycleResponse](t, firstCaptureW)
	assert.Equal(t, "Payment captured", firstCapture.Message)
	assert.Equal(t, string(models.StatusPaid), string(firstCapture.Order.Status))
	assert.Equal(t, string(models.PaymentIntentStatusCaptured), string(firstCapture.PaymentIntent.Status))
	assert.Equal(t, "CAPTURE", string(firstCapture.Transaction.Operation))

	secondCaptureW := adminLifecycleRequest(t, r, http.MethodPost, capturePath, "", adminToken, "capture-key")
	require.Equal(t, http.StatusOK, secondCaptureW.Code)
	secondCapture := decodeJSON[apicontract.AdminOrderPaymentLifecycleResponse](t, secondCaptureW)
	assert.Equal(t, firstCapture.Transaction.Id, secondCapture.Transaction.Id)

	duplicateCaptureW := adminLifecycleRequest(t, r, http.MethodPost, capturePath, "", adminToken, "capture-key-2")
	require.Equal(t, http.StatusConflict, duplicateCaptureW.Code)
	assert.Contains(t, duplicateCaptureW.Body.String(), paymentservice.ErrCaptureNotAllowed.Error())

	var refreshedOrder models.Order
	require.NoError(t, db.First(&refreshedOrder, order.Id).Error)
	assert.Equal(t, models.StatusPaid, refreshedOrder.Status)

	var refreshedIntent models.PaymentIntent
	require.NoError(t, db.First(&refreshedIntent, intent.ID).Error)
	assert.Equal(t, models.PaymentIntentStatusCaptured, refreshedIntent.Status)

	ledgerW := adminLifecycleRequest(t, r, http.MethodGet, fmt.Sprintf("/api/v1/admin/orders/%d/payments", order.Id), "", adminToken, "")
	require.Equal(t, http.StatusOK, ledgerW.Code)
	ledger := decodeJSON[apicontract.OrderPaymentLedger](t, ledgerW)
	require.Len(t, ledger.Intents, 1)
	require.Len(t, ledger.Intents[0].Transactions, 2)
	assert.Equal(t, "AUTHORIZE", string(ledger.Intents[0].Transactions[0].Operation))
	assert.Equal(t, "CAPTURE", string(ledger.Intents[0].Transactions[1].Operation))
}

func TestAdminRefundPaymentReplaysAndBlocksDuplicateRefund(t *testing.T) {
	resetCheckoutProtectionForTest(t)

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
		&models.IdempotencyKey{},
	)
	admin := seedUser(t, db, "sub-admin-refund", "admin-refund", "admin-refund@example.com", "admin")
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)
	product := seedProduct(t, db, "sku-admin-refund", "Admin Refund Product", 30, 6)
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

	order := createGuestCheckoutOrder(t, r, checkoutToken, csrfToken, "refund-admin@example.com")
	quote := quoteCheckoutWithDummyProviders(t, r, checkoutToken, csrfToken)
	require.NotNil(t, quote.SnapshotID)
	authorizeW := authorizeCheckoutWithSnapshot(t, r, order.Id, *quote.SnapshotID, checkoutToken, csrfToken, "authorize-admin-refund")
	require.Equal(t, http.StatusOK, authorizeW.Code)

	var intent models.PaymentIntent
	require.NoError(t, db.First(&intent).Error)
	capturePath := fmt.Sprintf("/api/v1/admin/orders/%d/payments/%d/capture", order.Id, intent.ID)
	captureW := adminLifecycleRequest(t, r, http.MethodPost, capturePath, "", adminToken, "capture-before-refund")
	require.Equal(t, http.StatusOK, captureW.Code)

	refundPath := fmt.Sprintf("/api/v1/admin/orders/%d/payments/%d/refund", order.Id, intent.ID)
	firstRefundW := adminLifecycleRequest(t, r, http.MethodPost, refundPath, "", adminToken, "refund-key")
	require.Equal(t, http.StatusOK, firstRefundW.Code)
	firstRefund := decodeJSON[apicontract.AdminOrderPaymentLifecycleResponse](t, firstRefundW)
	assert.Equal(t, "Payment refunded", firstRefund.Message)
	assert.Equal(t, string(models.StatusRefunded), string(firstRefund.Order.Status))
	assert.Equal(t, string(models.PaymentIntentStatusRefunded), string(firstRefund.PaymentIntent.Status))

	secondRefundW := adminLifecycleRequest(t, r, http.MethodPost, refundPath, "", adminToken, "refund-key")
	require.Equal(t, http.StatusOK, secondRefundW.Code)
	secondRefund := decodeJSON[apicontract.AdminOrderPaymentLifecycleResponse](t, secondRefundW)
	assert.Equal(t, firstRefund.Transaction.Id, secondRefund.Transaction.Id)

	duplicateRefundW := adminLifecycleRequest(t, r, http.MethodPost, refundPath, "", adminToken, "refund-key-2")
	require.Equal(t, http.StatusConflict, duplicateRefundW.Code)
	assert.Contains(t, duplicateRefundW.Body.String(), paymentservice.ErrAmountExceedsAvailable.Error())

	ledgerW := adminLifecycleRequest(t, r, http.MethodGet, fmt.Sprintf("/api/v1/admin/orders/%d/payments", order.Id), "", adminToken, "")
	require.Equal(t, http.StatusOK, ledgerW.Code)
	ledger := decodeJSON[apicontract.OrderPaymentLedger](t, ledgerW)
	require.Len(t, ledger.Intents, 1)
	require.Len(t, ledger.Intents[0].Transactions, 3)
	assert.Equal(t, "AUTHORIZE", string(ledger.Intents[0].Transactions[0].Operation))
	assert.Equal(t, "CAPTURE", string(ledger.Intents[0].Transactions[1].Operation))
	assert.Equal(t, "REFUND", string(ledger.Intents[0].Transactions[2].Operation))
}

func TestAdminVoidPaymentRequiresIdempotencyKey(t *testing.T) {
	resetCheckoutProtectionForTest(t)

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
		&models.IdempotencyKey{},
	)
	admin := seedUser(t, db, "sub-admin-void", "admin-void", "admin-void@example.com", "admin")
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)
	product := seedProduct(t, db, "sku-admin-void", "Admin Void Product", 18, 6)
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

	order := createGuestCheckoutOrder(t, r, checkoutToken, csrfToken, "void-admin@example.com")
	quote := quoteCheckoutWithDummyProviders(t, r, checkoutToken, csrfToken)
	require.NotNil(t, quote.SnapshotID)
	authorizeW := authorizeCheckoutWithSnapshot(t, r, order.Id, *quote.SnapshotID, checkoutToken, csrfToken, "authorize-admin-void")
	require.Equal(t, http.StatusOK, authorizeW.Code)

	var intent models.PaymentIntent
	require.NoError(t, db.First(&intent).Error)

	voidPath := fmt.Sprintf("/api/v1/admin/orders/%d/payments/%d/void", order.Id, intent.ID)
	voidW := adminLifecycleRequest(t, r, http.MethodPost, voidPath, "", adminToken, "")
	require.Equal(t, http.StatusBadRequest, voidW.Code)
	assert.Contains(t, voidW.Body.String(), "Header parameter Idempotency-Key is required")
}

func TestWebhookRejectsInvalidSignature(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.WebhookEvent{},
	)

	resp := webhookRequest(
		t,
		r,
		"dummy-card",
		`{"id":"evt-invalid-signature","type":"payment.captured","data":{"provider_txn_id":"dummy"}}`,
		"invalid",
	)
	require.Equal(t, http.StatusUnauthorized, resp.Code)
	assert.Contains(t, resp.Body.String(), "Invalid webhook signature")

	var events []models.WebhookEvent
	require.NoError(t, db.Order("id ASC").Find(&events).Error)
	require.Len(t, events, 1)
	assert.Equal(t, "dummy-card", events[0].Provider)
	assert.Contains(t, events[0].ProviderEventID, "signature.invalid.")
	assert.Equal(t, "signature.invalid", events[0].EventType)
	assert.False(t, events[0].SignatureValid)
	assert.Equal(t, 1, events[0].AttemptCount)
	assert.Contains(t, events[0].LastError, "invalid webhook signature")
}

func TestWebhookReplayIsNoOp(t *testing.T) {
	resetCheckoutProtectionForTest(t)

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
		&models.IdempotencyKey{},
		&models.WebhookEvent{},
	)
	admin := seedUser(t, db, "sub-admin-webhook-replay", "admin-webhook-replay", "admin-webhook-replay@example.com", "admin")
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)
	product := seedProduct(t, db, "sku-webhook-replay", "Webhook Replay Product", 19.5, 8)
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

	order := createGuestCheckoutOrder(t, r, checkoutToken, csrfToken, "webhook-replay@example.com")
	quote := quoteCheckoutWithDummyProviders(t, r, checkoutToken, csrfToken)
	require.NotNil(t, quote.SnapshotID)
	authorizeW := authorizeCheckoutWithSnapshot(t, r, order.Id, *quote.SnapshotID, checkoutToken, csrfToken, "authorize-webhook-replay")
	require.Equal(t, http.StatusOK, authorizeW.Code)

	var intent models.PaymentIntent
	require.NoError(t, db.First(&intent).Error)
	capturePath := fmt.Sprintf("/api/v1/admin/orders/%d/payments/%d/capture", order.Id, intent.ID)
	captureW := adminLifecycleRequest(t, r, http.MethodPost, capturePath, "", adminToken, "capture-webhook-replay")
	require.Equal(t, http.StatusOK, captureW.Code)

	var captureTxn models.PaymentTransaction
	require.NoError(t, db.Where("payment_intent_id = ? AND operation = ?", intent.ID, models.PaymentTransactionOperationCapture).First(&captureTxn).Error)

	firstWebhook := webhookRequest(
		t,
		r,
		"dummy-card",
		fmt.Sprintf(`{"id":"evt-webhook-replay","type":"payment.captured","data":{"provider_txn_id":%q}}`, captureTxn.ProviderTxnID),
		"valid",
	)
	require.Equal(t, http.StatusOK, firstWebhook.Code)

	firstPayload := decodeJSON[map[string]any](t, firstWebhook)
	eventID := uint(firstPayload["event_id"].(float64))
	event := waitForWebhookEvent(t, db, eventID, func(event models.WebhookEvent) bool {
		return event.ProcessedAt != nil && event.AttemptCount == 1
	})
	assert.Equal(t, "evt-webhook-replay", event.ProviderEventID)

	replayWebhook := webhookRequest(
		t,
		r,
		"dummy-card",
		fmt.Sprintf(`{"id":"evt-webhook-replay","type":"payment.captured","data":{"provider_txn_id":%q}}`, captureTxn.ProviderTxnID),
		"valid",
	)
	require.Equal(t, http.StatusOK, replayWebhook.Code)
	replayPayload := decodeJSON[map[string]any](t, replayWebhook)
	assert.Equal(t, true, replayPayload["duplicate"])

	var count int64
	require.NoError(t, db.Model(&models.WebhookEvent{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestPoisonWebhookEventVisibleInAdminInspection(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.WebhookEvent{},
	)
	admin := seedUser(t, db, "sub-admin-webhook-poison", "admin-webhook-poison", "admin-webhook-poison@example.com", "admin")
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)

	resp := webhookRequest(
		t,
		r,
		"dummy-card",
		`{"id":"evt-poison","type":"payment.captured","data":{"provider_txn_id":"missing-provider-transaction"}}`,
		"valid",
	)
	require.Equal(t, http.StatusOK, resp.Code)
	payload := decodeJSON[map[string]any](t, resp)
	eventID := uint(payload["event_id"].(float64))

	event := waitForWebhookEvent(t, db, eventID, func(event models.WebhookEvent) bool {
		return event.ProcessedAt == nil &&
			event.AttemptCount >= webhookservice.DefaultMaxAttempts &&
			strings.Contains(event.LastError, "provider transaction not found")
	})
	assert.Equal(t, "evt-poison", event.ProviderEventID)

	listW := adminLifecycleRequest(t, r, http.MethodGet, "/api/v1/admin/webhooks/events?status=DEAD_LETTER&page=1&limit=20", "", adminToken, "")
	require.Equal(t, http.StatusOK, listW.Code)

	var page struct {
		Data []struct {
			ProviderEventID string `json:"provider_event_id"`
			Status          string `json:"status"`
			AttemptCount    int    `json:"attempt_count"`
			LastError       string `json:"last_error"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listW.Body.Bytes(), &page))
	require.Len(t, page.Data, 1)
	assert.Equal(t, "evt-poison", page.Data[0].ProviderEventID)
	assert.Equal(t, webhookservice.EventStatusDeadLetter, page.Data[0].Status)
	assert.Equal(t, webhookservice.DefaultMaxAttempts, page.Data[0].AttemptCount)
	assert.Contains(t, page.Data[0].LastError, "provider transaction not found")
}

func TestRejectedWebhookEventVisibleInAdminInspection(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.WebhookEvent{},
	)
	admin := seedUser(t, db, "sub-admin-webhook-rejected", "admin-webhook-rejected", "admin-webhook-rejected@example.com", "admin")
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)

	resp := webhookRequest(
		t,
		r,
		"dummy-card",
		`{"id":"evt-invalid-signature","type":"payment.captured","data":{"provider_txn_id":"missing-provider-transaction"}}`,
		"invalid",
	)
	require.Equal(t, http.StatusUnauthorized, resp.Code)

	secondResp := webhookRequest(
		t,
		r,
		"dummy-card",
		`{"id":"evt-invalid-signature-2","type":"payment.captured","data":{"provider_txn_id":"missing-provider-transaction-2"}}`,
		"invalid",
	)
	require.Equal(t, http.StatusUnauthorized, secondResp.Code)

	listW := adminLifecycleRequest(t, r, http.MethodGet, "/api/v1/admin/webhooks/events?status=REJECTED&page=1&limit=20", "", adminToken, "")
	require.Equal(t, http.StatusOK, listW.Code)

	var page struct {
		Data []struct {
			ProviderEventID string `json:"provider_event_id"`
			EventType       string `json:"event_type"`
			SignatureValid  bool   `json:"signature_valid"`
			Status          string `json:"status"`
			LastError       string `json:"last_error"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listW.Body.Bytes(), &page))
	require.Len(t, page.Data, 2)
	assert.NotEqual(t, page.Data[0].ProviderEventID, page.Data[1].ProviderEventID)
	for _, event := range page.Data {
		assert.Contains(t, event.ProviderEventID, "signature.invalid.")
		assert.Equal(t, "signature.invalid", event.EventType)
		assert.False(t, event.SignatureValid)
		assert.Equal(t, webhookservice.EventStatusRejected, event.Status)
		assert.Contains(t, event.LastError, "invalid webhook signature")
	}
}

func TestAdminShippingLabelKeepsChosenServiceImmutable(t *testing.T) {
	resetCheckoutProtectionForTest(t)

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
		&models.IdempotencyKey{},
		&models.ShipmentRate{},
		&models.Shipment{},
	)
	admin := seedUser(t, db, "sub-admin-shipping-label", "admin-shipping-label", "admin-shipping-label@example.com", "admin")
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)
	product := seedProduct(t, db, "sku-shipping-label", "Shipping Label Product", 22, 5)
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

	order := createGuestCheckoutOrder(t, r, checkoutToken, csrfToken, "shipping-label@example.com")
	quote := quoteCheckoutWithDummyProviders(t, r, checkoutToken, csrfToken)
	require.NotNil(t, quote.SnapshotID)

	ratesW := quoteShippingRatesWithSnapshot(t, r, order.Id, *quote.SnapshotID, checkoutToken, csrfToken, "shipping-rates-key")
	require.Equal(t, http.StatusOK, ratesW.Code)
	rates := decodeJSON[checkoutShippingRatesTestResponse](t, ratesW)
	require.Len(t, rates.Rates, 2)

	selectedRateID := rates.Rates[0].ID
	alternateRateID := rates.Rates[1].ID
	for _, rate := range rates.Rates {
		if rate.Selected {
			selectedRateID = rate.ID
		} else {
			alternateRateID = rate.ID
		}
	}

	labelPath := fmt.Sprintf("/api/v1/admin/orders/%d/shipping/labels", order.Id)
	firstLabelW := adminLifecycleRequest(
		t,
		r,
		http.MethodPost,
		labelPath,
		fmt.Sprintf(`{"rate_id":%d,"package":{"reference":"box-1","weight_grams":500}}`, selectedRateID),
		adminToken,
		"shipping-label-key",
	)
	require.Equal(t, http.StatusOK, firstLabelW.Code)
	firstLabel := decodeJSON[adminShippingLabelTestResponse](t, firstLabelW)
	assert.Equal(t, models.ShipmentStatusLabelPurchased, firstLabel.Shipment.Status)

	replayLabelW := adminLifecycleRequest(
		t,
		r,
		http.MethodPost,
		labelPath,
		fmt.Sprintf(`{"rate_id":%d,"package":{"reference":"box-1","weight_grams":500}}`, selectedRateID),
		adminToken,
		"shipping-label-key",
	)
	require.Equal(t, http.StatusOK, replayLabelW.Code)
	replayLabel := decodeJSON[adminShippingLabelTestResponse](t, replayLabelW)
	assert.Equal(t, firstLabel.Shipment.ID, replayLabel.Shipment.ID)

	conflictLabelW := adminLifecycleRequest(
		t,
		r,
		http.MethodPost,
		labelPath,
		fmt.Sprintf(`{"rate_id":%d}`, alternateRateID),
		adminToken,
		"shipping-label-conflict",
	)
	require.Equal(t, http.StatusConflict, conflictLabelW.Code)
	assert.Contains(t, conflictLabelW.Body.String(), "immutable")
}

func TestShippingTrackingWebhookUpdatesShipmentStateIdempotently(t *testing.T) {
	resetCheckoutProtectionForTest(t)

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
		&models.IdempotencyKey{},
		&models.ShipmentRate{},
		&models.Shipment{},
		&models.TrackingEvent{},
		&models.WebhookEvent{},
	)
	admin := seedUser(t, db, "sub-admin-tracking", "admin-tracking", "admin-tracking@example.com", "admin")
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)
	product := seedProduct(t, db, "sku-tracking", "Tracking Product", 24, 5)
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

	order := createGuestCheckoutOrder(t, r, checkoutToken, csrfToken, "tracking@example.com")
	quote := quoteCheckoutWithDummyProviders(t, r, checkoutToken, csrfToken)
	require.NotNil(t, quote.SnapshotID)

	ratesW := quoteShippingRatesWithSnapshot(t, r, order.Id, *quote.SnapshotID, checkoutToken, csrfToken, "tracking-rates-key")
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
		fmt.Sprintf(`{"rate_id":%d}`, selectedRateID),
		adminToken,
		"tracking-label-key",
	)
	require.Equal(t, http.StatusOK, labelW.Code)
	label := decodeJSON[adminShippingLabelTestResponse](t, labelW)

	webhookBody := fmt.Sprintf(
		`{"id":"evt-tracking-in-transit","type":"tracking.in_transit","data":{"provider_shipment_id":%q,"tracking_number":%q,"status":"IN_TRANSIT","location":"Austin, TX","description":"Accepted by carrier","occurred_at":"2026-03-20T12:00:00Z"}}`,
		label.Shipment.ProviderShipmentID,
		label.Shipment.TrackingNumber,
	)
	firstWebhook := webhookRequest(t, r, "dummy-ground", webhookBody, "valid")
	require.Equal(t, http.StatusOK, firstWebhook.Code)
	firstPayload := decodeJSON[map[string]any](t, firstWebhook)
	eventID := uint(firstPayload["event_id"].(float64))
	waitForWebhookEvent(t, db, eventID, func(event models.WebhookEvent) bool {
		return event.ProcessedAt != nil && event.AttemptCount == 1
	})

	trackingW := checkoutTrackingRequest(t, r, order.Id, checkoutToken)
	require.Equal(t, http.StatusOK, trackingW.Code)
	tracking := decodeJSON[checkoutTrackingTestResponse](t, trackingW)
	require.Len(t, tracking.Shipments, 1)
	assert.Equal(t, models.ShipmentStatusInTransit, tracking.Shipments[0].Status)
	require.Len(t, tracking.Shipments[0].TrackingEvents, 1)
	assert.Equal(t, "evt-tracking-in-transit", tracking.Shipments[0].TrackingEvents[0].ProviderEventID)

	replayWebhook := webhookRequest(t, r, "dummy-ground", webhookBody, "valid")
	require.Equal(t, http.StatusOK, replayWebhook.Code)
	replayPayload := decodeJSON[map[string]any](t, replayWebhook)
	assert.Equal(t, true, replayPayload["duplicate"])

	var trackingCount int64
	require.NoError(t, db.Model(&models.TrackingEvent{}).Count(&trackingCount).Error)
	assert.EqualValues(t, 1, trackingCount)

	var refreshedOrder models.Order
	require.NoError(t, db.First(&refreshedOrder, order.Id).Error)
	assert.Equal(t, models.StatusShipped, refreshedOrder.Status)
}

func TestCheckoutOrderTrackingUsesUserOwnershipForAuthenticatedHistoricalOrders(t *testing.T) {
	resetCheckoutProtectionForTest(t)

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.ShipmentRate{},
		&models.Shipment{},
		&models.ShipmentPackage{},
		&models.TrackingEvent{},
	)

	customer := seedUser(
		t,
		db,
		"sub-customer-tracking-history",
		"customer-tracking-history",
		"customer-tracking-history@example.com",
		"customer",
	)
	customerToken := issueBearerTokenWithRole(
		t,
		generatedTestJWTSecret,
		customer.Subject,
		customer.Role,
	)

	now := time.Now().UTC()
	olderSession := models.CheckoutSession{
		PublicToken: "historical-tracking-session-old",
		UserID:      &customer.ID,
		Status:      models.CheckoutSessionStatusConverted,
		ExpiresAt:   now.Add(24 * time.Hour),
		LastSeenAt:  now.Add(-2 * time.Hour),
	}
	newerSession := models.CheckoutSession{
		PublicToken: "historical-tracking-session-new",
		UserID:      &customer.ID,
		Status:      models.CheckoutSessionStatusConverted,
		ExpiresAt:   now.Add(24 * time.Hour),
		LastSeenAt:  now.Add(-1 * time.Hour),
	}
	require.NoError(t, db.Create(&olderSession).Error)
	require.NoError(t, db.Create(&newerSession).Error)

	historicalOrder := models.Order{
		UserID:            &customer.ID,
		CheckoutSessionID: olderSession.ID,
		Status:            models.StatusPending,
		Total:             models.MoneyFromFloat(19.99),
	}
	require.NoError(t, db.Create(&historicalOrder).Error)

	req := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/v1/checkout/orders/%d/shipping/tracking", historicalOrder.ID),
		nil,
	)
	req.Header.Set("Authorization", "Bearer "+customerToken)
	req.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: newerSession.PublicToken})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	tracking := decodeJSON[checkoutTrackingTestResponse](t, w)
	assert.Equal(t, historicalOrder.ID, tracking.OrderID)
	assert.Len(t, tracking.Shipments, 0)

	guestOwnerSession := seedCheckoutSession(t, db, nil)
	otherGuestSession := seedCheckoutSession(t, db, nil)
	guestOrder := models.Order{
		CheckoutSessionID: guestOwnerSession.ID,
		Status:            models.StatusPending,
		Total:             models.MoneyFromFloat(9.99),
	}
	require.NoError(t, db.Create(&guestOrder).Error)

	guestReq := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/v1/checkout/orders/%d/shipping/tracking", guestOrder.ID),
		nil,
	)
	guestReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: otherGuestSession.PublicToken})

	guestW := httptest.NewRecorder()
	r.ServeHTTP(guestW, guestReq)

	require.Equal(t, http.StatusNotFound, guestW.Code)
	assert.Contains(t, guestW.Body.String(), "Order not found")
}

func TestCheckoutTaxFinalizePersistsLineLevelSnapshotResults(t *testing.T) {
	resetCheckoutProtectionForTest(t)

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
		&models.IdempotencyKey{},
		&models.OrderTaxLine{},
	)
	product := seedProduct(t, db, "sku-tax-finalize", "Tax Finalize Product", 30, 8)
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

	order := createGuestCheckoutOrder(t, r, checkoutToken, csrfToken, "tax-finalize@example.com")
	quote := quoteCheckoutWithDummyProviders(t, r, checkoutToken, csrfToken)
	require.NotNil(t, quote.SnapshotID)

	finalizeW := finalizeCheckoutTaxWithSnapshot(t, r, order.Id, *quote.SnapshotID, checkoutToken, csrfToken, "tax-finalize-key", "")
	require.Equal(t, http.StatusOK, finalizeW.Code)
	finalizeResp := decodeJSON[checkoutTaxFinalizeTestResponse](t, finalizeW)
	assert.Equal(t, "dummy-us-tax", finalizeResp.Provider)
	assert.False(t, finalizeResp.InclusivePricing)
	require.Len(t, finalizeResp.Lines, 2)

	lineTypes := []string{finalizeResp.Lines[0].LineType, finalizeResp.Lines[1].LineType}
	assert.Contains(t, lineTypes, models.TaxLineTypeItem)
	assert.Contains(t, lineTypes, models.TaxLineTypeShipping)
	assert.NotZero(t, finalizeResp.TotalTax)

	var storedLines []models.OrderTaxLine
	require.NoError(t, db.Where("order_id = ? AND snapshot_id = ?", order.Id, *quote.SnapshotID).Order("id ASC").Find(&storedLines).Error)
	require.Len(t, storedLines, 2)
	for _, line := range storedLines {
		assert.Equal(t, uint(order.Id), line.OrderID)
		assert.Equal(t, *quote.SnapshotID, line.SnapshotID)
		assert.Equal(t, "dummy-us-tax", line.TaxProviderID)
	}
}

func TestCheckoutTaxFinalizeRejectsAlreadyBoundSnapshot(t *testing.T) {
	resetCheckoutProtectionForTest(t)

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
		&models.IdempotencyKey{},
		&models.OrderTaxLine{},
	)
	product := seedProduct(t, db, "sku-bound-tax-snapshot", "Bound Tax Snapshot Product", 24.25, 8)
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

	order := createGuestCheckoutOrder(t, r, checkoutToken, csrfToken, "bound-tax-snapshot@example.com")
	quote := quoteCheckoutWithDummyProviders(t, r, checkoutToken, csrfToken)
	require.NotNil(t, quote.SnapshotID)

	var storedOrder models.Order
	require.NoError(t, db.First(&storedOrder, order.Id).Error)

	boundOrder := models.Order{
		CheckoutSessionID: storedOrder.CheckoutSessionID,
		Total:             storedOrder.Total,
		Status:            models.StatusPending,
	}
	require.NoError(t, db.Create(&boundOrder).Error)
	require.NoError(t, db.Model(&models.OrderCheckoutSnapshot{}).
		Where("id = ?", *quote.SnapshotID).
		Update("order_id", boundOrder.ID).Error)

	finalizeW := finalizeCheckoutTaxWithSnapshot(t, r, order.Id, *quote.SnapshotID, checkoutToken, csrfToken, "bound-tax-finalize-key", "")
	require.Equal(t, http.StatusConflict, finalizeW.Code)
	assert.Contains(t, finalizeW.Body.String(), "snapshot is already bound to another order")
}

func TestCheckoutOrderPaymentRejectsStaleDuplicateOrderAfterSessionConversion(t *testing.T) {
	resetCheckoutProtectionForTest(t)

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
	)
	product := seedProduct(t, db, "sku-stale-order", "Stale Order Product", 18.75, 10)
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

	emailA := "stale-a@example.com"
	tokenA := "confirm-a"
	emailB := "stale-b@example.com"
	tokenB := "confirm-b"
	var staleOrderID uint
	var currentOrderID uint
	var sessionID uint
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		var session models.CheckoutSession
		if err := tx.Where("public_token = ?", checkoutToken).First(&session).Error; err != nil {
			return err
		}
		sessionID = session.ID

		staleOrder := models.Order{
			CheckoutSessionID: session.ID,
			GuestEmail:        &emailA,
			ConfirmationToken: &tokenA,
			Status:            models.StatusPending,
			Total:             models.MoneyFromFloat(18.75),
			Items: []models.OrderItem{{
				ProductVariantID: variantID,
				VariantSKU:       "stale-a",
				VariantTitle:     "Stale A",
				Quantity:         1,
				Price:            models.MoneyFromFloat(18.75),
			}},
		}
		if err := tx.Create(&staleOrder).Error; err != nil {
			return err
		}
		currentOrder := models.Order{
			CheckoutSessionID: session.ID,
			GuestEmail:        &emailB,
			ConfirmationToken: &tokenB,
			Status:            models.StatusPending,
			Total:             models.MoneyFromFloat(18.75),
			Items: []models.OrderItem{{
				ProductVariantID: variantID,
				VariantSKU:       "stale-b",
				VariantTitle:     "Stale B",
				Quantity:         1,
				Price:            models.MoneyFromFloat(18.75),
			}},
		}
		if err := tx.Create(&currentOrder).Error; err != nil {
			return err
		}
		staleOrderID = staleOrder.ID
		currentOrderID = currentOrder.ID
		return nil
	}))

	quote := quoteCheckoutWithDummyProviders(t, r, checkoutToken, csrfToken)
	require.True(t, quote.Valid)
	require.NotNil(t, quote.SnapshotID)

	stalePayW := authorizeCheckoutWithSnapshot(t, r, int(staleOrderID), *quote.SnapshotID, checkoutToken, csrfToken, "")
	require.Equal(t, http.StatusConflict, stalePayW.Code)
	assert.Contains(t, stalePayW.Body.String(), "no longer payable")

	currentPayW := authorizeCheckoutWithSnapshot(t, r, int(currentOrderID), *quote.SnapshotID, checkoutToken, csrfToken, "")
	require.Equal(t, http.StatusOK, currentPayW.Code)

	replayStalePayW := authorizeCheckoutWithSnapshot(t, r, int(staleOrderID), *quote.SnapshotID, checkoutToken, csrfToken, "")
	require.Equal(t, http.StatusConflict, replayStalePayW.Code)
	assert.Contains(t, replayStalePayW.Body.String(), "already been converted")

	var session models.CheckoutSession
	require.NoError(t, db.First(&session, sessionID).Error)
	assert.Equal(t, models.CheckoutSessionStatusConverted, session.Status)
}

func TestCheckoutOrderCreateRateLimitReturnsTooManyRequests(t *testing.T) {
	resetCheckoutProtectionForTest(t)
	checkoutSubmissionRateLimit.Limit = 1
	checkoutSubmissionRateLimit.Window = time.Hour

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
	)
	product := seedProduct(t, db, "sku-rate-limit", "Rate Limit Product", 9.25, 10)
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

	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/orders", strings.NewReader(`{"guest_email":"rate-limit@example.com"}`))
	firstReq.Header.Set("Content-Type", "application/json")
	firstReq.Header.Set("X-CSRF-Token", csrfToken)
	firstReq.Header.Set("Idempotency-Key", "rate-limit-first")
	firstReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	firstReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	firstW := httptest.NewRecorder()
	r.ServeHTTP(firstW, firstReq)
	require.Equal(t, http.StatusCreated, firstW.Code)

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/orders", strings.NewReader(`{"guest_email":"rate-limit@example.com"}`))
	secondReq.Header.Set("Content-Type", "application/json")
	secondReq.Header.Set("X-CSRF-Token", csrfToken)
	secondReq.Header.Set("Idempotency-Key", "rate-limit-second")
	secondReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	secondReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	secondW := httptest.NewRecorder()
	r.ServeHTTP(secondW, secondReq)
	require.Equal(t, http.StatusTooManyRequests, secondW.Code)

	body := decodeJSON[map[string]any](t, secondW)
	assert.Equal(t, checkoutRateLimitedCode, body["code"])
}

func TestCheckoutOrderCreateIdempotencyReplayBypassesRateLimit(t *testing.T) {
	resetCheckoutProtectionForTest(t)
	checkoutSubmissionRateLimit.Limit = 1
	checkoutSubmissionRateLimit.Window = time.Hour

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
		&models.IdempotencyKey{},
	)
	product := seedProduct(t, db, "sku-rate-limit-replay", "Rate Limit Replay Product", 13.5, 10)
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

	body := `{"guest_email":"rate-limit-replay@example.com"}`
	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/orders", strings.NewReader(body))
	firstReq.Header.Set("Content-Type", "application/json")
	firstReq.Header.Set("X-CSRF-Token", csrfToken)
	firstReq.Header.Set("Idempotency-Key", "rate-limit-replay")
	firstReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	firstReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	firstW := httptest.NewRecorder()
	r.ServeHTTP(firstW, firstReq)
	require.Equal(t, http.StatusCreated, firstW.Code)
	firstOrder := decodeJSON[apicontract.Order](t, firstW)

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/orders", strings.NewReader(body))
	secondReq.Header.Set("Content-Type", "application/json")
	secondReq.Header.Set("X-CSRF-Token", csrfToken)
	secondReq.Header.Set("Idempotency-Key", "rate-limit-replay")
	secondReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	secondReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	secondW := httptest.NewRecorder()
	r.ServeHTTP(secondW, secondReq)
	require.Equal(t, http.StatusCreated, secondW.Code)
	secondOrder := decodeJSON[apicontract.Order](t, secondW)

	assert.Equal(t, firstOrder.Id, secondOrder.Id)
}

func TestCheckoutOrderPaymentIdempotencyReplayBypassesRateLimit(t *testing.T) {
	resetCheckoutProtectionForTest(t)
	checkoutSubmissionRateLimit.Limit = 1
	checkoutSubmissionRateLimit.Window = time.Hour

	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Cart{},
		&models.CartItem{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.OrderItem{},
		&models.IdempotencyKey{},
	)
	product := seedProduct(t, db, "sku-pay-rate-limit-replay", "Payment Rate Limit Replay Product", 19.99, 10)
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

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/checkout/orders", strings.NewReader(`{"guest_email":"pay-rate-limit-replay@example.com"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-CSRF-Token", csrfToken)
	createReq.AddCookie(&http.Cookie{Name: checkoutSessionCookieName, Value: checkoutToken})
	createReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code)
	order := decodeJSON[apicontract.Order](t, createW)

	quote := quoteCheckoutWithDummyProviders(t, r, checkoutToken, csrfToken)
	require.True(t, quote.Valid)
	require.NotNil(t, quote.SnapshotID)

	firstPayW := authorizeCheckoutWithSnapshot(
		t,
		r,
		order.Id,
		*quote.SnapshotID,
		checkoutToken,
		csrfToken,
		"payment-rate-limit-replay",
	)
	require.Equal(t, http.StatusOK, firstPayW.Code)

	secondPayW := authorizeCheckoutWithSnapshot(
		t,
		r,
		order.Id,
		*quote.SnapshotID,
		checkoutToken,
		csrfToken,
		"payment-rate-limit-replay",
	)
	require.Equal(t, http.StatusOK, secondPayW.Code)

	var firstPayload struct {
		Order apicontract.Order `json:"order"`
	}
	var secondPayload struct {
		Order apicontract.Order `json:"order"`
	}
	require.NoError(t, json.Unmarshal(firstPayW.Body.Bytes(), &firstPayload))
	require.NoError(t, json.Unmarshal(secondPayW.Body.Bytes(), &secondPayload))
	assert.Equal(t, firstPayload.Order.Id, secondPayload.Order.Id)
}

func TestGeneratedDisableLocalSignInAndAuthValidation(t *testing.T) {
	rDisabled, _ := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{
		DisableLocalSignIn: true,
	}, &models.User{})

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/v1/auth/register", body: `{"username":"u1","email":"u1@example.com","password":"supersecret"}`},
		{method: http.MethodPost, path: "/api/v1/auth/register", body: `{"username":"u1"}`},
		{method: http.MethodPost, path: "/api/v1/auth/login", body: `{"email":"u1@example.com","password":"supersecret"}`},
		{method: http.MethodPost, path: "/api/v1/auth/login", body: `{"email":"bad"}`},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		rDisabled.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	}

	authConfigReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/config", nil)
	authConfigW := httptest.NewRecorder()
	rDisabled.ServeHTTP(authConfigW, authConfigReq)
	require.Equal(t, http.StatusOK, authConfigW.Code)
	var disabledConfig AuthConfigResponse
	require.NoError(t, json.Unmarshal(authConfigW.Body.Bytes(), &disabledConfig))
	assert.Equal(t, AuthConfigResponse{
		LocalSignInEnabled: false,
		OIDCEnabled:        false,
	}, disabledConfig)

	for _, path := range []string{"/api/v1/auth/oidc/login", "/api/v1/auth/oidc/callback?state=x&code=y"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		rDisabled.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	}

	rEnabled, _ := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{})

	registerInvalidReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"username":"onlyname"}`))
	registerInvalidReq.Header.Set("Content-Type", "application/json")
	registerInvalidW := httptest.NewRecorder()
	rEnabled.ServeHTTP(registerInvalidW, registerInvalidReq)
	assert.Equal(t, http.StatusBadRequest, registerInvalidW.Code)

	registerValidReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"username":"gooduser","email":"good@example.com","password":"supersecret"}`))
	registerValidReq.Header.Set("Content-Type", "application/json")
	registerValidW := httptest.NewRecorder()
	rEnabled.ServeHTTP(registerValidW, registerValidReq)
	assert.Equal(t, http.StatusCreated, registerValidW.Code)

	loginInvalidReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"good@example.com","password":"wrongpass"}`))
	loginInvalidReq.Header.Set("Content-Type", "application/json")
	loginInvalidW := httptest.NewRecorder()
	rEnabled.ServeHTTP(loginInvalidW, loginInvalidReq)
	assert.Equal(t, http.StatusUnauthorized, loginInvalidW.Code)

	loginValidReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"good@example.com","password":"supersecret"}`))
	loginValidReq.Header.Set("Content-Type", "application/json")
	loginValidW := httptest.NewRecorder()
	rEnabled.ServeHTTP(loginValidW, loginValidReq)
	assert.Equal(t, http.StatusOK, loginValidW.Code)

	authConfigEnabledReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/config", nil)
	authConfigEnabledW := httptest.NewRecorder()
	rEnabled.ServeHTTP(authConfigEnabledW, authConfigEnabledReq)
	require.Equal(t, http.StatusOK, authConfigEnabledW.Code)
	var enabledConfig AuthConfigResponse
	require.NoError(t, json.Unmarshal(authConfigEnabledW.Body.Bytes(), &enabledConfig))
	assert.Equal(t, AuthConfigResponse{
		LocalSignInEnabled: true,
		OIDCEnabled:        false,
	}, enabledConfig)
}

func TestGeneratedMediaUploadProtectionAndSuccess(t *testing.T) {
	var uploadCalls int32
	uploadHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&uploadCalls, 1)
		w.WriteHeader(http.StatusNoContent)
	})

	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{
		MediaUploads: uploadHandler,
	}, &models.User{})
	user := seedUser(t, db, "sub-media", "media-user", "media@example.com", "customer")
	token := issueBearerTokenWithRole(t, generatedTestJWTSecret, user.Subject, user.Role)

	unauthReq := httptest.NewRequest(http.MethodPost, "/api/v1/media/uploads", nil)
	unauthW := httptest.NewRecorder()
	r.ServeHTTP(unauthW, unauthReq)
	assert.Equal(t, http.StatusUnauthorized, unauthW.Code)

	sessionNoCsrfReq := httptest.NewRequest(http.MethodPost, "/api/v1/media/uploads", nil)
	sessionNoCsrfReq.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	sessionNoCsrfW := httptest.NewRecorder()
	r.ServeHTTP(sessionNoCsrfW, sessionNoCsrfReq)
	assert.Equal(t, http.StatusForbidden, sessionNoCsrfW.Code)

	invalidHeaderReq := httptest.NewRequest(http.MethodPost, "/api/v1/media/uploads", nil)
	invalidHeaderReq.Header.Set("Authorization", "Bearer "+token)
	invalidHeaderReq.Header.Set("Upload-Length", "abc")
	invalidHeaderW := httptest.NewRecorder()
	r.ServeHTTP(invalidHeaderW, invalidHeaderReq)
	assert.Equal(t, http.StatusBadRequest, invalidHeaderW.Code)

	validPostReq := httptest.NewRequest(http.MethodPost, "/api/v1/media/uploads", nil)
	validPostReq.Header.Set("Authorization", "Bearer "+token)
	validPostW := httptest.NewRecorder()
	r.ServeHTTP(validPostW, validPostReq)
	assert.Equal(t, http.StatusNoContent, validPostW.Code)

	validPatchReq := httptest.NewRequest(http.MethodPatch, "/api/v1/media/uploads/path-1", nil)
	validPatchReq.Header.Set("Authorization", "Bearer "+token)
	validPatchW := httptest.NewRecorder()
	r.ServeHTTP(validPatchW, validPatchReq)
	assert.Equal(t, http.StatusNoContent, validPatchW.Code)

	validHeadReq := httptest.NewRequest(http.MethodHead, "/api/v1/media/uploads/path-1", nil)
	validHeadReq.Header.Set("Authorization", "Bearer "+token)
	validHeadW := httptest.NewRecorder()
	r.ServeHTTP(validHeadW, validHeadReq)
	assert.Equal(t, http.StatusNoContent, validHeadW.Code)

	assert.GreaterOrEqual(t, atomic.LoadInt32(&uploadCalls), int32(3))
}

func TestGeneratedProtectedRouteValidation(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{}, &models.Cart{}, &models.CartItem{}, &models.Product{})
	_ = seedUser(t, db, "sub-admin-2", "admin2", "admin2@example.com", "admin")
	_ = seedUser(t, db, "sub-user-2", "user2", "user2@example.com", "customer")
	_ = seedProduct(t, db, "sku-val-1", "Validation Product", 19.99, 5)

	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, "sub-admin-2", "admin")
	userToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, "sub-user-2", "customer")

	adminInvalidReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=oops", nil)
	adminInvalidReq.Header.Set("Authorization", "Bearer "+adminToken)
	adminInvalidW := httptest.NewRecorder()
	r.ServeHTTP(adminInvalidW, adminInvalidReq)
	assert.Equal(t, http.StatusBadRequest, adminInvalidW.Code)

	adminValidReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=1&limit=10", nil)
	adminValidReq.Header.Set("Authorization", "Bearer "+adminToken)
	adminValidW := httptest.NewRecorder()
	r.ServeHTTP(adminValidW, adminValidReq)
	assert.Equal(t, http.StatusOK, adminValidW.Code)

	cartInvalidPathReq := httptest.NewRequest(http.MethodPatch, "/api/v1/me/cart/not-an-int", strings.NewReader(`{"quantity":1}`))
	cartInvalidPathReq.Header.Set("Authorization", "Bearer "+userToken)
	cartInvalidPathReq.Header.Set("Content-Type", "application/json")
	cartInvalidPathW := httptest.NewRecorder()
	r.ServeHTTP(cartInvalidPathW, cartInvalidPathReq)
	assert.Equal(t, http.StatusBadRequest, cartInvalidPathW.Code)

	ordersInvalidQueryReq := httptest.NewRequest(http.MethodGet, "/api/v1/me/orders?page=bad", nil)
	ordersInvalidQueryReq.Header.Set("Authorization", "Bearer "+userToken)
	ordersInvalidQueryW := httptest.NewRecorder()
	r.ServeHTTP(ordersInvalidQueryW, ordersInvalidQueryReq)
	assert.Equal(t, http.StatusBadRequest, ordersInvalidQueryW.Code)
}

func TestGeneratedSmokeRegisterCartOrderFlow(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{}, &models.Product{}, &models.ProductVariant{}, &models.Cart{}, &models.CartItem{}, &models.Order{}, &models.OrderItem{})
	product := seedProduct(t, db, "sku-smoke-1", "Smoke Product", 15.50, 10)
	variantID := requireDefaultVariantID(t, product)

	registerReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"username":"smoke-user","email":"smoke@example.com","password":"supersecret"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerW := httptest.NewRecorder()
	r.ServeHTTP(registerW, registerReq)
	assert.Equal(t, http.StatusCreated, registerW.Code)

	var authResp AuthResponse
	require.NoError(t, json.Unmarshal(registerW.Body.Bytes(), &authResp))
	require.NotEmpty(t, authResp.User.Subject)

	token := issueBearerTokenWithRole(t, generatedTestJWTSecret, authResp.User.Subject, "customer")

	addCartReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart", strings.NewReader(`{"product_variant_id":`+strconv.Itoa(int(variantID))+`,"quantity":2}`))
	addCartReq.Header.Set("Authorization", "Bearer "+token)
	addCartReq.Header.Set("Content-Type", "application/json")
	addCartW := httptest.NewRecorder()
	r.ServeHTTP(addCartW, addCartReq)
	assert.Equal(t, http.StatusOK, addCartW.Code)

	createInvalidOrderReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/orders", strings.NewReader(`{"items":[]}`))
	createInvalidOrderReq.Header.Set("Authorization", "Bearer "+token)
	createInvalidOrderReq.Header.Set("Content-Type", "application/json")
	createInvalidOrderW := httptest.NewRecorder()
	r.ServeHTTP(createInvalidOrderW, createInvalidOrderReq)
	assert.Equal(t, http.StatusBadRequest, createInvalidOrderW.Code)

	createValidOrderReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/orders", strings.NewReader(`{"items":[{"product_variant_id":`+strconv.Itoa(int(variantID))+`,"quantity":2}]}`))
	createValidOrderReq.Header.Set("Authorization", "Bearer "+token)
	createValidOrderReq.Header.Set("Content-Type", "application/json")
	createValidOrderW := httptest.NewRecorder()
	r.ServeHTTP(createValidOrderW, createValidOrderReq)
	assert.Equal(t, http.StatusCreated, createValidOrderW.Code)

	var orderCount int64
	require.NoError(t, db.Model(&models.Order{}).Count(&orderCount).Error)
	assert.EqualValues(t, 1, orderCount)
}

func setupGeneratedRouterForAccountData(t *testing.T) (*gin.Engine, *models.User) {
	t.Helper()
	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.SavedPaymentMethod{},
		&models.SavedAddress{},
	)
	user := seedUser(t, db, "sub-account", "account-user", "account@example.com", "customer")
	return r, &user
}

func TestSavedPaymentMethodDefaultTransitions(t *testing.T) {
	r, user := setupGeneratedRouterForAccountData(t)
	token := issueBearerTokenWithRole(t, generatedTestJWTSecret, user.Subject, user.Role)

	firstResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/payment-methods", map[string]any{
		"cardholder_name": "User One",
		"card_number":     "4111111111111111",
		"exp_month":       12,
		"exp_year":        2040,
	}, token)
	require.Equal(t, http.StatusCreated, firstResp.Code)
	first := decodeJSON[models.SavedPaymentMethod](t, firstResp)
	assert.True(t, first.IsDefault)

	secondResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/payment-methods", map[string]any{
		"cardholder_name": "User One",
		"card_number":     "5555555555554444",
		"exp_month":       10,
		"exp_year":        2039,
		"set_default":     true,
	}, token)
	require.Equal(t, http.StatusCreated, secondResp.Code)
	second := decodeJSON[models.SavedPaymentMethod](t, secondResp)
	assert.True(t, second.IsDefault)

	listResp := performJSONRequest(t, r, http.MethodGet, "/api/v1/me/payment-methods", nil, token)
	require.Equal(t, http.StatusOK, listResp.Code)
	methods := decodeJSON[[]models.SavedPaymentMethod](t, listResp)
	require.Len(t, methods, 2)
	assert.Equal(t, second.ID, methods[0].ID)
	assert.True(t, methods[0].IsDefault)

	setDefaultResp := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/me/payment-methods/%d/default", first.ID), nil, token)
	require.Equal(t, http.StatusOK, setDefaultResp.Code)
	setDefaultBody := decodeJSON[models.SavedPaymentMethod](t, setDefaultResp)
	assert.True(t, setDefaultBody.IsDefault)

	deleteResp := performJSONRequest(t, r, http.MethodDelete, fmt.Sprintf("/api/v1/me/payment-methods/%d", first.ID), nil, token)
	require.Equal(t, http.StatusOK, deleteResp.Code)

	listAfterDeleteResp := performJSONRequest(t, r, http.MethodGet, "/api/v1/me/payment-methods", nil, token)
	require.Equal(t, http.StatusOK, listAfterDeleteResp.Code)
	remaining := decodeJSON[[]models.SavedPaymentMethod](t, listAfterDeleteResp)
	require.Len(t, remaining, 1)
	assert.Equal(t, second.ID, remaining[0].ID)
	assert.True(t, remaining[0].IsDefault)
}

func TestSavedAddressDefaultTransitions(t *testing.T) {
	r, user := setupGeneratedRouterForAccountData(t)
	token := issueBearerTokenWithRole(t, generatedTestJWTSecret, user.Subject, user.Role)

	firstResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/addresses", map[string]any{
		"full_name":   "Address User",
		"line1":       "100 Main",
		"city":        "Austin",
		"postal_code": "78701",
		"country":     "US",
	}, token)
	require.Equal(t, http.StatusCreated, firstResp.Code)
	first := decodeJSON[models.SavedAddress](t, firstResp)
	assert.True(t, first.IsDefault)

	secondResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/addresses", map[string]any{
		"full_name":   "Address User",
		"line1":       "200 Main",
		"city":        "Austin",
		"postal_code": "78702",
		"country":     "US",
		"set_default": true,
	}, token)
	require.Equal(t, http.StatusCreated, secondResp.Code)
	second := decodeJSON[models.SavedAddress](t, secondResp)
	assert.True(t, second.IsDefault)

	listResp := performJSONRequest(t, r, http.MethodGet, "/api/v1/me/addresses", nil, token)
	require.Equal(t, http.StatusOK, listResp.Code)
	addresses := decodeJSON[[]models.SavedAddress](t, listResp)
	require.Len(t, addresses, 2)
	assert.Equal(t, second.ID, addresses[0].ID)
	assert.True(t, addresses[0].IsDefault)

	setDefaultResp := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/me/addresses/%d/default", first.ID), nil, token)
	require.Equal(t, http.StatusOK, setDefaultResp.Code)
	setDefaultBody := decodeJSON[models.SavedAddress](t, setDefaultResp)
	assert.True(t, setDefaultBody.IsDefault)

	deleteResp := performJSONRequest(t, r, http.MethodDelete, fmt.Sprintf("/api/v1/me/addresses/%d", first.ID), nil, token)
	require.Equal(t, http.StatusOK, deleteResp.Code)

	listAfterDeleteResp := performJSONRequest(t, r, http.MethodGet, "/api/v1/me/addresses", nil, token)
	require.Equal(t, http.StatusOK, listAfterDeleteResp.Code)
	remaining := decodeJSON[[]models.SavedAddress](t, listAfterDeleteResp)
	require.Len(t, remaining, 1)
	assert.Equal(t, second.ID, remaining[0].ID)
	assert.True(t, remaining[0].IsDefault)
}

func TestProfileEndpointsAndUserList(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{})
	admin := seedUser(t, db, "sub-admin-profile", "admin-profile", "admin-profile@example.com", "admin")
	customer := seedUser(t, db, "sub-customer-profile", "customer-profile", "customer-profile@example.com", "customer")

	customerToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, customer.Subject, customer.Role)
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)

	getProfileResp := performJSONRequest(t, r, http.MethodGet, "/api/v1/me/", nil, customerToken)
	require.Equal(t, http.StatusOK, getProfileResp.Code)
	profile := decodeJSON[models.User](t, getProfileResp)
	assert.Equal(t, customer.Subject, profile.Subject)

	badUpdateResp := performJSONRequest(t, r, http.MethodPatch, "/api/v1/me/", map[string]any{"currency": "XYZ"}, customerToken)
	require.Equal(t, http.StatusBadRequest, badUpdateResp.Code)

	goodUpdateResp := performJSONRequest(t, r, http.MethodPatch, "/api/v1/me/", map[string]any{"name": "Updated Name", "currency": "EUR"}, customerToken)
	require.Equal(t, http.StatusOK, goodUpdateResp.Code)
	updated := decodeJSON[models.User](t, goodUpdateResp)
	assert.Equal(t, "Updated Name", updated.Name)
	assert.Equal(t, "EUR", updated.Currency)

	listUsersResp := performJSONRequest(t, r, http.MethodGet, "/api/v1/admin/users?page=1&limit=1", nil, adminToken)
	require.Equal(t, http.StatusOK, listUsersResp.Code)
	page := decodeJSON[apicontract.UserPage](t, listUsersResp)
	assert.Equal(t, 1, page.Pagination.Page)
	assert.Equal(t, 1, page.Pagination.Limit)
	assert.GreaterOrEqual(t, page.Pagination.Total, 2)
}

func setupMediaRouter(t *testing.T, customerSubject, adminSubject string) (*gin.Engine, *models.User, *models.User, *models.Product, *models.MediaObject, *models.MediaObject, *models.MediaObject, *models.MediaObject, *models.MediaObject, *models.MediaObject, *gorm.DB) {
	t.Helper()
	db := newTestDB(t, &models.User{}, &models.Product{}, &models.MediaObject{}, &models.MediaVariant{}, &models.MediaReference{})
	mediaService := media.NewService(db, t.TempDir(), "http://localhost:3000/media", nil)
	require.NoError(t, mediaService.EnsureDirs())

	customer := seedUser(t, db, customerSubject, "media-customer", "media-customer@example.com", "customer")
	admin := seedUser(t, db, adminSubject, "media-admin", "media-admin@example.com", "admin")
	product := seedProduct(t, db, "media-prod-1", "Media Product", 19.99, 9)

	nonImage := models.MediaObject{ID: "profile-non-image", OriginalPath: "files/non-image.pdf", MimeType: "application/pdf", SizeBytes: 120, Status: media.StatusReady}
	image := models.MediaObject{ID: "profile-image", OriginalPath: "images/profile.webp", MimeType: "image/webp", SizeBytes: 120, Status: media.StatusReady}
	prodA := models.MediaObject{ID: "product-media-a", OriginalPath: "a/original.webp", MimeType: "image/webp", SizeBytes: 10, Status: media.StatusReady}
	prodB := models.MediaObject{ID: "product-media-b", OriginalPath: "b/original.webp", MimeType: "image/webp", SizeBytes: 10, Status: media.StatusReady}
	processing := models.MediaObject{ID: "processing-media", OriginalPath: "proc/file.webp", MimeType: "image/webp", SizeBytes: 10, Status: media.StatusProcessing}
	failed := models.MediaObject{ID: "failed-media", OriginalPath: "failed/file.webp", MimeType: "image/webp", SizeBytes: 10, Status: media.StatusFailed}

	for _, obj := range []models.MediaObject{nonImage, image, prodA, prodB, processing, failed} {
		require.NoError(t, db.Create(&obj).Error)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	server, err := NewGeneratedAPIServer(db, mediaService, GeneratedAPIServerConfig{JWTSecret: generatedTestJWTSecret})
	require.NoError(t, err)
	apicontract.RegisterHandlers(r, server)

	return r, &customer, &admin, &product, &nonImage, &image, &prodA, &prodB, &processing, &failed, db
}

func TestProfilePhotoLifecycleAndValidation(t *testing.T) {
	r, customer, _, _, nonImage, image, _, _, _, _, db := setupMediaRouter(t, "sub-media-customer-1", "sub-media-admin-1")
	token := issueBearerTokenWithRole(t, generatedTestJWTSecret, customer.Subject, customer.Role)

	badResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/profile-photo", map[string]any{"media_id": nonImage.ID}, token)
	require.Equal(t, http.StatusBadRequest, badResp.Code)

	setResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/profile-photo", map[string]any{"media_id": image.ID}, token)
	require.Equal(t, http.StatusOK, setResp.Code)
	setBody := decodeJSON[models.User](t, setResp)
	assert.Contains(t, setBody.ProfilePhoto, "images/profile.webp")

	deleteResp := performJSONRequest(t, r, http.MethodDelete, "/api/v1/me/profile-photo", nil, token)
	require.Equal(t, http.StatusOK, deleteResp.Code)
	deleted := decodeJSON[models.User](t, deleteResp)
	assert.Equal(t, "", deleted.ProfilePhoto)

	var refs int64
	require.NoError(t, db.Model(&models.MediaReference{}).Where("owner_type = ? AND role = ?", media.OwnerTypeUser, media.RoleProfilePhoto).Count(&refs).Error)
	assert.EqualValues(t, 0, refs)
}

func TestAdminProductMediaAttachReorderDetachAndProcessingRejection(t *testing.T) {
	r, _, admin, product, _, _, prodA, prodB, processing, failed, db := setupMediaRouter(t, "sub-media-customer-2", "sub-media-admin-2")
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)

	attachResp := performJSONRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/admin/products/%d/media", product.ID), map[string]any{"media_ids": []string{prodA.ID, prodB.ID}}, adminToken)
	require.Equal(t, http.StatusOK, attachResp.Code)

	processingResp := performJSONRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/admin/products/%d/media", product.ID), map[string]any{"media_ids": []string{processing.ID}}, adminToken)
	require.Equal(t, http.StatusConflict, processingResp.Code)

	failedResp := performJSONRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/admin/products/%d/media", product.ID), map[string]any{"media_ids": []string{failed.ID}}, adminToken)
	require.Equal(t, http.StatusUnprocessableEntity, failedResp.Code)

	badOrderResp := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/admin/products/%d/media/order", product.ID), map[string]any{"media_ids": []string{"missing-id"}}, adminToken)
	require.Equal(t, http.StatusBadRequest, badOrderResp.Code)

	goodOrderResp := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/admin/products/%d/media/order", product.ID), map[string]any{"media_ids": []string{prodB.ID, prodA.ID}}, adminToken)
	require.Equal(t, http.StatusOK, goodOrderResp.Code)

	var refs []models.MediaReference
	require.NoError(t, db.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeProduct, product.ID, media.RoleProductDraftImage).Order("position asc").Find(&refs).Error)
	require.Len(t, refs, 2)
	assert.Equal(t, prodB.ID, refs[0].MediaID)
	assert.Equal(t, prodA.ID, refs[1].MediaID)

	detachResp := performJSONRequest(t, r, http.MethodDelete, fmt.Sprintf("/api/v1/admin/products/%d/media/%s", product.ID, prodA.ID), nil, adminToken)
	require.Equal(t, http.StatusOK, detachResp.Code)

	var count int64
	require.NoError(t, db.Model(&models.MediaReference{}).Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeProduct, product.ID, media.RoleProductDraftImage).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestAdminProductDraftIsolationAndPublish(t *testing.T) {
	r, _, admin, product, _, _, _, _, _, _, _ := setupMediaRouter(t, "sub-media-customer-iso", "sub-media-admin-iso")
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)

	updateResp := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/admin/products/%d", product.ID), singleVariantProductUpsertPayload(
		product.SKU,
		"Draft Name",
		product.Description,
		product.Price.Float64(),
		0,
	), adminToken)
	require.Equal(t, http.StatusOK, updateResp.Code)
	updated := decodeJSON[apicontract.Product](t, updateResp)
	assert.Equal(t, "Draft Name", updated.Name)
	assert.Equal(t, 0, updated.Stock)
	require.NotNil(t, updated.HasDraftChanges)
	assert.True(t, *updated.HasDraftChanges)

	publicBefore := performJSONRequest(t, r, http.MethodGet, fmt.Sprintf("/api/v1/products/%d", product.ID), nil, "")
	require.Equal(t, http.StatusOK, publicBefore.Code)
	publicProductBefore := decodeJSON[apicontract.Product](t, publicBefore)
	assert.Equal(t, "Media Product", publicProductBefore.Name)
	assert.Equal(t, 9, publicProductBefore.Stock)

	adminView := performJSONRequest(t, r, http.MethodGet, fmt.Sprintf("/api/v1/admin/products/%d", product.ID), nil, adminToken)
	require.Equal(t, http.StatusOK, adminView.Code)
	adminProduct := decodeJSON[apicontract.Product](t, adminView)
	assert.Equal(t, "Draft Name", adminProduct.Name)
	assert.Equal(t, 0, adminProduct.Stock)

	publishResp := performJSONRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/admin/products/%d/publish", product.ID), nil, adminToken)
	require.Equal(t, http.StatusOK, publishResp.Code)

	publicAfter := performJSONRequest(t, r, http.MethodGet, fmt.Sprintf("/api/v1/products/%d", product.ID), nil, "")
	require.Equal(t, http.StatusOK, publicAfter.Code)
	publicProductAfter := decodeJSON[apicontract.Product](t, publicAfter)
	assert.Equal(t, "Draft Name", publicProductAfter.Name)
	assert.Equal(t, 0, publicProductAfter.Stock)
}

func TestAdminCreateProductStaysUnpublishedUntilPublish(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.Product{})
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, "sub-admin-create-draft", "admin")

	createResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/admin/products", singleVariantProductUpsertPayload(
		"draft-create-sku",
		"Draft Create Product",
		"Draft Create Product description",
		19.99,
		7,
	), adminToken)
	require.Equal(t, http.StatusCreated, createResp.Code)

	created := decodeJSON[apicontract.Product](t, createResp)
	require.NotNil(t, created.IsPublished)
	assert.False(t, *created.IsPublished)
	require.NotNil(t, created.HasDraftChanges)
	assert.True(t, *created.HasDraftChanges)

	publicByIDResp := performJSONRequest(t, r, http.MethodGet, fmt.Sprintf("/api/v1/products/%d", created.Id), nil, "")
	assert.Equal(t, http.StatusNotFound, publicByIDResp.Code)

	publicListResp := performJSONRequest(t, r, http.MethodGet, "/api/v1/products", nil, "")
	require.Equal(t, http.StatusOK, publicListResp.Code)
	publicList := decodeJSON[apicontract.ProductPage](t, publicListResp)
	for _, product := range publicList.Data {
		assert.NotEqual(t, created.Id, product.Id)
	}

	publishResp := performJSONRequest(
		t,
		r,
		http.MethodPost,
		fmt.Sprintf("/api/v1/admin/products/%d/publish", created.Id),
		nil,
		adminToken,
	)
	require.Equal(t, http.StatusOK, publishResp.Code)

	var stored models.Product
	require.NoError(t, db.First(&stored, created.Id).Error)
	assert.True(t, stored.IsPublished)

	publicAfterResp := performJSONRequest(t, r, http.MethodGet, fmt.Sprintf("/api/v1/products/%d", created.Id), nil, "")
	assert.Equal(t, http.StatusOK, publicAfterResp.Code)
}

func TestAdminCanUnpublishProductWithoutDeleting(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.Product{})
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, "sub-admin-unpublish", "admin")

	product := seedProduct(t, db, "unpublish-sku", "Unpublish Product", 29.99, 5)

	publicBefore := performJSONRequest(t, r, http.MethodGet, fmt.Sprintf("/api/v1/products/%d", product.ID), nil, "")
	require.Equal(t, http.StatusOK, publicBefore.Code)

	unpublishResp := performJSONRequest(
		t,
		r,
		http.MethodPost,
		fmt.Sprintf("/api/v1/admin/products/%d/unpublish", product.ID),
		nil,
		adminToken,
	)
	require.Equal(t, http.StatusOK, unpublishResp.Code)
	unpublished := decodeJSON[apicontract.Product](t, unpublishResp)
	require.NotNil(t, unpublished.IsPublished)
	assert.False(t, *unpublished.IsPublished)
	require.NotNil(t, unpublished.HasDraftChanges)
	assert.True(t, *unpublished.HasDraftChanges)

	var stored models.Product
	require.NoError(t, db.First(&stored, product.ID).Error)
	assert.False(t, stored.IsPublished)
	assert.NotNil(t, stored.DraftUpdatedAt)
	var draftCount int64
	require.NoError(t, db.Model(&models.ProductDraft{}).Where("product_id = ?", product.ID).Count(&draftCount).Error)
	assert.EqualValues(t, 1, draftCount)

	publicAfter := performJSONRequest(t, r, http.MethodGet, fmt.Sprintf("/api/v1/products/%d", product.ID), nil, "")
	assert.Equal(t, http.StatusNotFound, publicAfter.Code)

	adminView := performJSONRequest(t, r, http.MethodGet, fmt.Sprintf("/api/v1/admin/products/%d", product.ID), nil, adminToken)
	assert.Equal(t, http.StatusOK, adminView.Code)
}

func TestAdminProductDraftMediaPromotesOnPublish(t *testing.T) {
	r, _, admin, product, _, _, prodA, prodB, _, _, db := setupMediaRouter(t, "sub-media-customer-publish", "sub-media-admin-publish")
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)

	require.NoError(t, db.Create(&models.MediaReference{
		MediaID:   prodA.ID,
		OwnerType: media.OwnerTypeProduct,
		OwnerID:   product.ID,
		Role:      media.RoleProductImage,
		Position:  0,
	}).Error)

	attachResp := performJSONRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/admin/products/%d/media", product.ID), map[string]any{
		"media_ids": []string{prodB.ID},
	}, adminToken)
	require.Equal(t, http.StatusOK, attachResp.Code)

	var liveRefsBefore []models.MediaReference
	require.NoError(t, db.Where("owner_type = ? AND owner_id = ? AND role = ?",
		media.OwnerTypeProduct, product.ID, media.RoleProductImage).
		Order("position asc").Find(&liveRefsBefore).Error)
	require.Len(t, liveRefsBefore, 1)
	assert.Equal(t, prodA.ID, liveRefsBefore[0].MediaID)

	var draftRefsBefore []models.MediaReference
	require.NoError(t, db.Where("owner_type = ? AND owner_id = ? AND role = ?",
		media.OwnerTypeProduct, product.ID, media.RoleProductDraftImage).
		Order("position asc").Find(&draftRefsBefore).Error)
	require.Len(t, draftRefsBefore, 2)

	publishResp := performJSONRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/admin/products/%d/publish", product.ID), nil, adminToken)
	require.Equal(t, http.StatusOK, publishResp.Code)

	var liveRefsAfter []models.MediaReference
	require.NoError(t, db.Where("owner_type = ? AND owner_id = ? AND role = ?",
		media.OwnerTypeProduct, product.ID, media.RoleProductImage).
		Order("position asc").Find(&liveRefsAfter).Error)
	require.Len(t, liveRefsAfter, 2)

	var draftCount int64
	require.NoError(t, db.Model(&models.MediaReference{}).Where("owner_type = ? AND owner_id = ? AND role = ?",
		media.OwnerTypeProduct, product.ID, media.RoleProductDraftImage).Count(&draftCount).Error)
	assert.EqualValues(t, 0, draftCount)
}

func TestCartUpdateDeleteOwnershipIsolation(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{}, &models.Product{}, &models.ProductVariant{}, &models.Cart{}, &models.CartItem{})
	owner := seedUser(t, db, "sub-owner", "owner", "owner@example.com", "customer")
	other := seedUser(t, db, "sub-other", "other", "other@example.com", "customer")
	product := seedProduct(t, db, "sku-cart-gap", "Cart Gap Product", 10, 5)
	variantID := requireDefaultVariantID(t, product)

	ownerToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, owner.Subject, owner.Role)
	otherToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, other.Subject, other.Role)

	addResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/cart", map[string]any{"product_variant_id": variantID, "quantity": 1}, ownerToken)
	require.Equal(t, http.StatusOK, addResp.Code)
	cart := decodeJSON[cartResponse](t, addResp)
	require.Len(t, cart.Items, 1)
	itemID := cart.Items[0].ID

	patchOther := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/me/cart/%d", itemID), map[string]any{"quantity": 2}, otherToken)
	require.Equal(t, http.StatusNotFound, patchOther.Code)

	deleteOther := performJSONRequest(t, r, http.MethodDelete, fmt.Sprintf("/api/v1/me/cart/%d", itemID), nil, otherToken)
	require.Equal(t, http.StatusNotFound, deleteOther.Code)

	patchOwnerBad := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/me/cart/%d", itemID), map[string]any{"quantity": 10}, ownerToken)
	require.Equal(t, http.StatusBadRequest, patchOwnerBad.Code)

	patchOwnerGood := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/me/cart/%d", itemID), map[string]any{"quantity": 2}, ownerToken)
	require.Equal(t, http.StatusOK, patchOwnerGood.Code)

	deleteOwner := performJSONRequest(t, r, http.MethodDelete, fmt.Sprintf("/api/v1/me/cart/%d", itemID), nil, ownerToken)
	require.Equal(t, http.StatusOK, deleteOwner.Code)
}

func TestOrdersDateFilterAndGetOrderValidation(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{}, &models.Product{}, &models.ProductVariant{}, &models.Order{}, &models.OrderItem{})
	user := seedUser(t, db, "sub-order-gap", "order-gap", "order-gap@example.com", "customer")
	product := seedProduct(t, db, "sku-order-gap", "Order Gap Product", 30, 8)
	variantID := requireDefaultVariantID(t, product)
	token := issueBearerTokenWithRole(t, generatedTestJWTSecret, user.Subject, user.Role)

	createOrderResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/orders", map[string]any{
		"items": []map[string]any{{"product_variant_id": variantID, "quantity": 1}},
	}, token)
	require.Equal(t, http.StatusCreated, createOrderResp.Code)
	createdOrder := decodeJSON[apicontract.Order](t, createOrderResp)
	require.True(t, createdOrder.CanCancel)

	shippedOrder := models.Order{
		UserID:            &user.ID,
		CheckoutSessionID: seedCheckoutSession(t, db, &user.ID).ID,
		Status:            models.StatusShipped,
		Total:             models.MoneyFromFloat(30),
	}
	require.NoError(t, db.Create(&shippedOrder).Error)

	badStart := performJSONRequest(t, r, http.MethodGet, "/api/v1/me/orders?start_date=nope", nil, token)
	require.Equal(t, http.StatusBadRequest, badStart.Code)

	badRange := performJSONRequest(t, r, http.MethodGet, "/api/v1/me/orders?start_date=2024-01-02&end_date=2024-01-01", nil, token)
	require.Equal(t, http.StatusBadRequest, badRange.Code)

	badStatus := performJSONRequest(t, r, http.MethodGet, "/api/v1/me/orders?status=INVALID", nil, token)
	require.Equal(t, http.StatusBadRequest, badStatus.Code)

	invalidIDReq := httptest.NewRequest(http.MethodGet, "/api/v1/me/orders/not-an-int", nil)
	invalidIDReq.Header.Set("Authorization", "Bearer "+token)
	invalidIDW := httptest.NewRecorder()
	r.ServeHTTP(invalidIDW, invalidIDReq)
	require.Equal(t, http.StatusBadRequest, invalidIDW.Code)

	missingResp := performJSONRequest(t, r, http.MethodGet, "/api/v1/me/orders/999999", nil, token)
	require.Equal(t, http.StatusNotFound, missingResp.Code)

	getOrderResp := performJSONRequest(t, r, http.MethodGet, fmt.Sprintf("/api/v1/me/orders/%d", createdOrder.Id), nil, token)
	require.Equal(t, http.StatusOK, getOrderResp.Code)
	gotOrder := decodeJSON[models.Order](t, getOrderResp)
	require.True(t, gotOrder.CanCancel)

	listOrdersResp := performJSONRequest(t, r, http.MethodGet, "/api/v1/me/orders", nil, token)
	require.Equal(t, http.StatusOK, listOrdersResp.Code)
	var listPayload struct {
		Data []apicontract.Order `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listOrdersResp.Body.Bytes(), &listPayload))

	canCancelByID := make(map[int]bool, len(listPayload.Data))
	for _, order := range listPayload.Data {
		canCancelByID[order.Id] = order.CanCancel
	}

	require.True(t, canCancelByID[createdOrder.Id])
	require.False(t, canCancelByID[int(shippedOrder.ID)])
}

func TestAdminUpdateOrderStatusDeductsStockOnceAndRollbackOnFailure(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{}, &models.Product{}, &models.ProductVariant{}, &models.Order{}, &models.OrderItem{})
	admin := seedUser(t, db, "sub-admin-order", "admin-order", "admin-order@example.com", "admin")
	customer := seedUser(t, db, "sub-customer-order", "customer-order", "customer-order@example.com", "customer")
	product := seedProduct(t, db, "sku-stock-gap", "Stock Gap Product", 12.5, 5)
	productVariantID := requireDefaultVariantID(t, product)

	order := models.Order{
		UserID:            &customer.ID,
		CheckoutSessionID: seedCheckoutSession(t, db, &customer.ID).ID,
		Status:            models.StatusPending,
		Total:             models.MoneyFromFloat(25),
	}
	require.NoError(t, db.Create(&order).Error)
	require.NoError(t, db.Create(&models.OrderItem{
		OrderID:          order.ID,
		ProductVariantID: productVariantID,
		VariantSKU:       "sku-stock-gap-default",
		VariantTitle:     "Stock Gap Product",
		Quantity:         2,
		Price:            models.MoneyFromFloat(12.5),
	}).Error)

	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)

	firstPay := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/admin/orders/%d/status", order.ID), map[string]any{"status": models.StatusPaid}, adminToken)
	require.Equal(t, http.StatusOK, firstPay.Code)

	var reloaded models.ProductVariant
	require.NoError(t, db.First(&reloaded, productVariantID).Error)
	assert.Equal(t, 3, reloaded.Stock)

	secondPay := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/admin/orders/%d/status", order.ID), map[string]any{"status": models.StatusPaid}, adminToken)
	require.Equal(t, http.StatusOK, secondPay.Code)
	require.NoError(t, db.First(&reloaded, productVariantID).Error)
	assert.Equal(t, 3, reloaded.Stock)

	ship := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/admin/orders/%d/status", order.ID), map[string]any{"status": models.StatusShipped}, adminToken)
	require.Equal(t, http.StatusOK, ship.Code)
	require.NoError(t, db.First(&reloaded, productVariantID).Error)
	assert.Equal(t, 3, reloaded.Stock)

	deliver := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/admin/orders/%d/status", order.ID), map[string]any{"status": models.StatusDelivered}, adminToken)
	require.Equal(t, http.StatusOK, deliver.Code)
	require.NoError(t, db.First(&reloaded, productVariantID).Error)
	assert.Equal(t, 3, reloaded.Stock)

	reverseToFailed := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/admin/orders/%d/status", order.ID), map[string]any{"status": models.StatusFailed}, adminToken)
	require.Equal(t, http.StatusOK, reverseToFailed.Code)
	require.NoError(t, db.First(&reloaded, productVariantID).Error)
	assert.Equal(t, 5, reloaded.Stock)

	reapplyPaid := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/admin/orders/%d/status", order.ID), map[string]any{"status": models.StatusPaid}, adminToken)
	require.Equal(t, http.StatusOK, reapplyPaid.Code)
	require.NoError(t, db.First(&reloaded, productVariantID).Error)
	assert.Equal(t, 3, reloaded.Stock)

	lowStockProduct := seedProduct(t, db, "sku-low-stock", "Low Stock Product", 9.99, 1)
	lowStockVariantID := requireDefaultVariantID(t, lowStockProduct)
	failingOrder := models.Order{
		UserID:            &customer.ID,
		CheckoutSessionID: seedCheckoutSession(t, db, &customer.ID).ID,
		Status:            models.StatusPending,
		Total:             models.MoneyFromFloat(29.97),
	}
	require.NoError(t, db.Create(&failingOrder).Error)
	require.NoError(t, db.Create(&models.OrderItem{
		OrderID:          failingOrder.ID,
		ProductVariantID: lowStockVariantID,
		VariantSKU:       "sku-low-stock-default",
		VariantTitle:     "Low Stock Product",
		Quantity:         2,
		Price:            models.MoneyFromFloat(9.99),
	}).Error)

	failPay := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/admin/orders/%d/status", failingOrder.ID), map[string]any{"status": models.StatusPaid}, adminToken)
	require.Equal(t, http.StatusBadRequest, failPay.Code)
	assert.Contains(t, strings.ToLower(failPay.Body.String()), "insufficient stock")

	var orderAfter models.Order
	require.NoError(t, db.First(&orderAfter, failingOrder.ID).Error)
	assert.Equal(t, models.StatusPending, orderAfter.Status)
}

func TestUserCancelOrderRefundsAndRestocks(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{}, &models.Product{}, &models.ProductVariant{}, &models.Order{}, &models.OrderItem{})
	customer := seedUser(t, db, "sub-customer-cancel", "customer-cancel", "customer-cancel@example.com", "customer")
	product := seedProduct(t, db, "sku-cancel-flow", "Cancelable Product", 19.99, 5)
	productVariantID := requireDefaultVariantID(t, product)

	order := models.Order{
		UserID:                &customer.ID,
		CheckoutSessionID:     seedCheckoutSession(t, db, &customer.ID).ID,
		Status:                models.StatusPaid,
		Total:                 models.MoneyFromFloat(39.98),
		PaymentMethodDisplay:  "Visa •••• 4242",
		ShippingAddressPretty: "123 Main St, New York, NY, 10001, US",
	}
	require.NoError(t, db.Create(&order).Error)
	require.NoError(t, db.Create(&models.OrderItem{
		OrderID:          order.ID,
		ProductVariantID: productVariantID,
		VariantSKU:       "sku-cancel-flow-default",
		VariantTitle:     "Cancelable Product",
		Quantity:         2,
		Price:            models.MoneyFromFloat(19.99),
	}).Error)
	require.NoError(t, db.Model(&models.ProductVariant{}).Where("id = ?", productVariantID).Update("stock", 3).Error)

	customerToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, customer.Subject, customer.Role)
	cancelResp := performJSONRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/me/orders/%d/cancel", order.ID), nil, customerToken)
	require.Equal(t, http.StatusOK, cancelResp.Code)

	var cancelled apicontract.Order
	require.NoError(t, json.Unmarshal(cancelResp.Body.Bytes(), &cancelled))
	assert.Equal(t, string(models.StatusCancelled), string(cancelled.Status))

	var reloadedProduct models.ProductVariant
	require.NoError(t, db.First(&reloadedProduct, productVariantID).Error)
	assert.Equal(t, 5, reloadedProduct.Stock)

	shippedOrder := models.Order{
		UserID:                &customer.ID,
		CheckoutSessionID:     seedCheckoutSession(t, db, &customer.ID).ID,
		Status:                models.StatusShipped,
		Total:                 models.MoneyFromFloat(19.99),
		PaymentMethodDisplay:  "Visa •••• 0005",
		ShippingAddressPretty: "In transit",
	}
	require.NoError(t, db.Create(&shippedOrder).Error)
	require.NoError(t, db.Create(&models.OrderItem{
		OrderID:          shippedOrder.ID,
		ProductVariantID: productVariantID,
		VariantSKU:       "sku-cancel-flow-default",
		VariantTitle:     "Cancelable Product",
		Quantity:         1,
		Price:            models.MoneyFromFloat(19.99),
	}).Error)

	cancelShippedResp := performJSONRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/me/orders/%d/cancel", shippedOrder.ID), nil, customerToken)
	require.Equal(t, http.StatusBadRequest, cancelShippedResp.Code)
	assert.Contains(t, cancelShippedResp.Body.String(), "Order cannot be cancelled")

	deliveredOrder := models.Order{
		UserID:                &customer.ID,
		CheckoutSessionID:     seedCheckoutSession(t, db, &customer.ID).ID,
		Status:                models.StatusDelivered,
		Total:                 models.MoneyFromFloat(19.99),
		PaymentMethodDisplay:  "Visa •••• 1111",
		ShippingAddressPretty: "Completed delivery",
	}
	require.NoError(t, db.Create(&deliveredOrder).Error)
	require.NoError(t, db.Create(&models.OrderItem{
		OrderID:          deliveredOrder.ID,
		ProductVariantID: productVariantID,
		VariantSKU:       "sku-cancel-flow-default",
		VariantTitle:     "Cancelable Product",
		Quantity:         1,
		Price:            models.MoneyFromFloat(19.99),
	}).Error)

	cancelDeliveredResp := performJSONRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/me/orders/%d/cancel", deliveredOrder.ID), nil, customerToken)
	require.Equal(t, http.StatusBadRequest, cancelDeliveredResp.Code)
	assert.Contains(t, cancelDeliveredResp.Body.String(), "Order cannot be cancelled")
}

func TestStorefrontDisabledSectionsAndHeroMediaValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t, &models.StorefrontSettings{}, &models.MediaObject{}, &models.MediaReference{})
	mediaService := media.NewService(db, t.TempDir(), "http://localhost:3000/media", nil)
	require.NoError(t, mediaService.EnsureDirs())

	settings := defaultStorefrontSettings()
	require.NotEmpty(t, settings.HomepageSections)
	disabledID := settings.HomepageSections[0].ID
	settings.HomepageSections[0].Enabled = false

	raw, err := json.Marshal(settings)
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.StorefrontSettings{ID: models.StorefrontSettingsSingletonID, ConfigJSON: string(raw)}).Error)

	r := gin.New()
	r.GET("/public", GetStorefrontSettings(db, mediaService))
	r.GET("/admin", GetAdminStorefrontSettings(db, mediaService))
	r.PUT("/admin", UpsertStorefrontSettings(db, mediaService))

	publicReq := httptest.NewRequest(http.MethodGet, "/public", nil)
	publicW := httptest.NewRecorder()
	r.ServeHTTP(publicW, publicReq)
	require.Equal(t, http.StatusOK, publicW.Code)
	publicResp := decodeJSON[StorefrontSettingsResponse](t, publicW)
	for _, section := range publicResp.Settings.HomepageSections {
		assert.NotEqual(t, disabledID, section.ID)
	}

	adminReq := httptest.NewRequest(http.MethodGet, "/admin", nil)
	adminW := httptest.NewRecorder()
	r.ServeHTTP(adminW, adminReq)
	require.Equal(t, http.StatusOK, adminW.Code)
	adminResp := decodeJSON[StorefrontSettingsResponse](t, adminW)
	foundDisabled := false
	for _, section := range adminResp.Settings.HomepageSections {
		if section.ID == disabledID {
			foundDisabled = true
			break
		}
	}
	assert.True(t, foundDisabled)

	require.NoError(t, db.Create(&models.MediaObject{ID: "hero-not-image", OriginalPath: "hero/file.pdf", MimeType: "application/pdf", SizeBytes: 12, Status: media.StatusReady}).Error)
	updated := defaultStorefrontSettings()
	for i := range updated.HomepageSections {
		if updated.HomepageSections[i].Type == "hero" && updated.HomepageSections[i].Hero != nil {
			updated.HomepageSections[i].Hero.BackgroundImageMediaID = "hero-not-image"
			updated.HomepageSections[i].Hero.BackgroundImageUrl = ""
			break
		}
	}
	body, err := json.Marshal(UpsertStorefrontSettingsRequest{Settings: updated})
	require.NoError(t, err)

	upsertReq := httptest.NewRequest(http.MethodPut, "/admin", strings.NewReader(string(body)))
	upsertReq.Header.Set("Content-Type", "application/json")
	upsertW := httptest.NewRecorder()
	r.ServeHTTP(upsertW, upsertReq)
	require.Equal(t, http.StatusBadRequest, upsertW.Code)
	assert.Contains(t, strings.ToLower(upsertW.Body.String()), "media must be an image")
}

func TestAdminPreviewSessionAndPublicDraftRendering(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t, &models.User{}, &models.Product{}, &models.StorefrontSettings{}, &models.MediaObject{}, &models.MediaReference{})
	mediaService := media.NewService(db, t.TempDir(), "http://localhost:3000/media", nil)
	require.NoError(t, mediaService.EnsureDirs())

	admin := seedUser(t, db, "sub-preview-admin", "preview-admin", "preview-admin@example.com", "admin")
	customer := seedUser(t, db, "sub-preview-customer", "preview-customer", "preview-customer@example.com", "customer")

	require.NoError(t, db.Create(&models.MediaObject{
		ID:           "hero-live",
		OriginalPath: "hero/live.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    12,
		Status:       media.StatusReady,
	}).Error)
	require.NoError(t, db.Create(&models.MediaObject{
		ID:           "hero-draft",
		OriginalPath: "hero/draft.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    12,
		Status:       media.StatusReady,
	}).Error)
	require.NoError(t, db.Create(&models.MediaObject{
		ID:           "product-live",
		OriginalPath: "products/live.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    12,
		Status:       media.StatusReady,
	}).Error)
	require.NoError(t, db.Create(&models.MediaObject{
		ID:           "product-draft",
		OriginalPath: "products/draft.jpg",
		MimeType:     "image/jpeg",
		SizeBytes:    12,
		Status:       media.StatusReady,
	}).Error)

	publishedProduct := seedProduct(t, db, "sku-preview-live", "Live Product", 9.99, 4)
	require.NoError(t, saveNormalizedProductDraft(db, publishedProduct, productCatalogDraft{
		SKU:         publishedProduct.SKU,
		Name:        publishedProduct.Name,
		Description: publishedProduct.Description,
		Price:       publishedProduct.Price.Float64(),
		Stock:       publishedProduct.Stock,
		Images:      []string{"products/draft.jpg"},
		Variants: []productVariantDraftData{
			{
				SKU:         publishedProduct.SKU + "-draft",
				Title:       publishedProduct.Name,
				Price:       publishedProduct.Price.Float64(),
				Stock:       publishedProduct.Stock,
				IsPublished: true,
			},
		},
		DefaultVariantSKU: publishedProduct.SKU + "-draft",
	}, time.Now().UTC()))
	require.NoError(t, db.Model(&publishedProduct).Update("draft_updated_at", ptrTimeNow()).Error)
	require.NoError(t, db.Create(&models.MediaReference{
		MediaID:   "product-live",
		OwnerType: media.OwnerTypeProduct,
		OwnerID:   publishedProduct.ID,
		Role:      media.RoleProductImage,
		Position:  0,
	}).Error)
	require.NoError(t, db.Create(&models.MediaReference{
		MediaID:   "product-draft",
		OwnerType: media.OwnerTypeProduct,
		OwnerID:   publishedProduct.ID,
		Role:      media.RoleProductDraftImage,
		Position:  0,
	}).Error)

	unpublishedDraft := models.Product{
		SKU:         "sku-preview-draft",
		Name:        "Draft Product Base",
		Description: "draft base",
		Price:       models.MoneyFromFloat(11.50),
		Stock:       3,
		IsPublished: false,
	}
	require.NoError(t, db.Select("*").Create(&unpublishedDraft).Error)
	require.NoError(t, saveNormalizedProductDraft(db, unpublishedDraft, productCatalogDraft{
		SKU:         "sku-preview-draft",
		Name:        "Draft Product",
		Description: "draft description",
		Price:       13.25,
		Stock:       2,
		Variants: []productVariantDraftData{
			{
				SKU:         "sku-preview-draft-default",
				Title:       "Draft Product",
				Price:       13.25,
				Stock:       2,
				IsPublished: true,
			},
		},
		DefaultVariantSKU: "sku-preview-draft-default",
	}, time.Now().UTC()))
	require.NoError(t, db.Model(&unpublishedDraft).Updates(map[string]any{
		"is_published":     false,
		"draft_updated_at": ptrTimeNow(),
	}).Error)

	publishedStorefront := defaultStorefrontSettings()
	draftStorefront := defaultStorefrontSettings()
	publishedStorefront.SiteTitle = "Live Storefront"
	draftStorefront.SiteTitle = "Draft Storefront"
	for i := range publishedStorefront.HomepageSections {
		if publishedStorefront.HomepageSections[i].Type == "hero" && publishedStorefront.HomepageSections[i].Hero != nil {
			publishedStorefront.HomepageSections[i].Hero.BackgroundImageMediaID = "hero-live"
			publishedStorefront.HomepageSections[i].Hero.BackgroundImageUrl = ""
			break
		}
	}
	for i := range draftStorefront.HomepageSections {
		if draftStorefront.HomepageSections[i].Type == "hero" && draftStorefront.HomepageSections[i].Hero != nil {
			draftStorefront.HomepageSections[i].Hero.BackgroundImageMediaID = "hero-draft"
			draftStorefront.HomepageSections[i].Hero.BackgroundImageUrl = ""
			break
		}
	}
	publishedStorefrontRaw, err := json.Marshal(publishedStorefront)
	require.NoError(t, err)
	draftStorefrontRaw, err := json.Marshal(draftStorefront)
	require.NoError(t, err)
	draftStorefrontRawStr := string(draftStorefrontRaw)
	require.NoError(t, db.Create(&models.StorefrontSettings{
		ID:               models.StorefrontSettingsSingletonID,
		ConfigJSON:       string(publishedStorefrontRaw),
		DraftConfigJSON:  &draftStorefrontRawStr,
		DraftUpdatedAt:   ptrTimeNow(),
		PublishedUpdated: time.Now(),
	}).Error)
	require.NoError(t, db.Create(&models.MediaReference{
		MediaID:   "hero-live",
		OwnerType: media.OwnerTypeStorefront,
		OwnerID:   models.StorefrontSettingsSingletonID,
		Role:      media.RoleStorefrontHero,
		Position:  0,
	}).Error)
	require.NoError(t, db.Create(&models.MediaReference{
		MediaID:   "hero-draft",
		OwnerType: media.OwnerTypeStorefront,
		OwnerID:   models.StorefrontSettingsSingletonID,
		Role:      media.RoleStorefrontHeroDraft,
		Position:  0,
	}).Error)

	r := gin.New()
	server, err := NewGeneratedAPIServer(db, mediaService, GeneratedAPIServerConfig{
		JWTSecret: generatedTestJWTSecret,
	})
	require.NoError(t, err)
	apicontract.RegisterHandlers(r, server)

	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)
	customerToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, customer.Subject, customer.Role)

	statusBeforeReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/preview", nil)
	statusBeforeReq.Header.Set("Authorization", "Bearer "+adminToken)
	statusBeforeW := httptest.NewRecorder()
	r.ServeHTTP(statusBeforeW, statusBeforeReq)
	require.Equal(t, http.StatusOK, statusBeforeW.Code)
	statusBefore := decodeJSON[DraftPreviewSessionResponse](t, statusBeforeW)
	assert.False(t, statusBefore.Active)

	unauthStartReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/preview/start", nil)
	unauthStartW := httptest.NewRecorder()
	r.ServeHTTP(unauthStartW, unauthStartReq)
	assert.Equal(t, http.StatusUnauthorized, unauthStartW.Code)

	customerStartReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/preview/start", nil)
	customerStartReq.Header.Set("Authorization", "Bearer "+customerToken)
	customerStartW := httptest.NewRecorder()
	r.ServeHTTP(customerStartW, customerStartReq)
	assert.Equal(t, http.StatusForbidden, customerStartW.Code)

	startReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/preview/start", nil)
	startReq.Header.Set("Authorization", "Bearer "+adminToken)
	startW := httptest.NewRecorder()
	r.ServeHTTP(startW, startReq)
	require.Equal(t, http.StatusOK, startW.Code)
	startBody := decodeJSON[DraftPreviewSessionResponse](t, startW)
	require.True(t, startBody.Active)
	require.NotNil(t, startBody.ExpiresAt)

	var previewCookie *http.Cookie
	for _, cookie := range startW.Result().Cookies() {
		if cookie.Name == draftPreviewCookieName {
			previewCookie = cookie
			break
		}
	}
	require.NotNil(t, previewCookie)
	require.NotEmpty(t, previewCookie.Value)

	statusReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/preview", nil)
	statusReq.Header.Set("Authorization", "Bearer "+adminToken)
	statusReq.AddCookie(previewCookie)
	statusW := httptest.NewRecorder()
	r.ServeHTTP(statusW, statusReq)
	require.Equal(t, http.StatusOK, statusW.Code)
	statusBody := decodeJSON[DraftPreviewSessionResponse](t, statusW)
	assert.True(t, statusBody.Active)
	require.NotNil(t, statusBody.ExpiresAt)

	publicLiveReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/products/%d", unpublishedDraft.ID), nil)
	publicLiveW := httptest.NewRecorder()
	r.ServeHTTP(publicLiveW, publicLiveReq)
	require.Equal(t, http.StatusNotFound, publicLiveW.Code)

	previewProductReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/products/%d", unpublishedDraft.ID), nil)
	previewProductReq.AddCookie(previewCookie)
	previewProductW := httptest.NewRecorder()
	r.ServeHTTP(previewProductW, previewProductReq)
	require.Equal(t, http.StatusOK, previewProductW.Code)
	previewProductBody := decodeJSON[apicontract.Product](t, previewProductW)
	assert.Equal(t, "Draft Product", previewProductBody.Name)
	assert.Equal(t, "private, no-store", previewProductW.Header().Get("Cache-Control"))
	assert.Equal(t, "noindex", previewProductW.Header().Get("X-Robots-Tag"))
	assert.Contains(t, strings.ToLower(previewProductW.Header().Get("Vary")), "cookie")

	previewPublishedReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/products/%d", publishedProduct.ID), nil)
	previewPublishedReq.AddCookie(previewCookie)
	previewPublishedW := httptest.NewRecorder()
	r.ServeHTTP(previewPublishedW, previewPublishedReq)
	require.Equal(t, http.StatusOK, previewPublishedW.Code)
	previewPublishedBody := decodeJSON[apicontract.Product](t, previewPublishedW)
	require.NotEmpty(t, previewPublishedBody.Images)
	assert.Contains(t, previewPublishedBody.Images[0], "products/draft.jpg")

	storefrontPreviewReq := httptest.NewRequest(http.MethodGet, "/api/v1/storefront", nil)
	storefrontPreviewReq.AddCookie(previewCookie)
	storefrontPreviewW := httptest.NewRecorder()
	r.ServeHTTP(storefrontPreviewW, storefrontPreviewReq)
	require.Equal(t, http.StatusOK, storefrontPreviewW.Code)
	storefrontPreviewBody := decodeJSON[StorefrontSettingsResponse](t, storefrontPreviewW)
	assert.Equal(t, "Draft Storefront", storefrontPreviewBody.Settings.SiteTitle)
	foundDraftHero := false
	for _, section := range storefrontPreviewBody.Settings.HomepageSections {
		if section.Type == "hero" && section.Hero != nil {
			assert.Contains(t, section.Hero.BackgroundImageUrl, "hero/draft.jpg")
			foundDraftHero = true
			break
		}
	}
	assert.True(t, foundDraftHero)
	assert.Equal(t, "private, no-store", storefrontPreviewW.Header().Get("Cache-Control"))
	assert.Equal(t, "noindex", storefrontPreviewW.Header().Get("X-Robots-Tag"))

	stopReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/preview/stop", nil)
	stopReq.Header.Set("Authorization", "Bearer "+adminToken)
	stopReq.AddCookie(previewCookie)
	stopW := httptest.NewRecorder()
	r.ServeHTTP(stopW, stopReq)
	require.Equal(t, http.StatusOK, stopW.Code)
	stopBody := decodeJSON[DraftPreviewSessionResponse](t, stopW)
	assert.False(t, stopBody.Active)

	postStopReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/products/%d", unpublishedDraft.ID), nil)
	postStopW := httptest.NewRecorder()
	r.ServeHTTP(postStopW, postStopReq)
	require.Equal(t, http.StatusNotFound, postStopW.Code)
}

func TestPublicEndpointsDoNotExposeDraftMetadataWithoutPreview(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.Product{},
		&models.StorefrontSettings{},
	)
	product := seedProduct(t, db, "sku-public-meta", "Public Product", 19.99, 8)

	productResp := performJSONRequest(t, r, http.MethodGet, fmt.Sprintf("/api/v1/products/%d", product.ID), nil, "")
	require.Equal(t, http.StatusOK, productResp.Code)
	var productBody map[string]any
	require.NoError(t, json.Unmarshal(productResp.Body.Bytes(), &productBody))
	_, hasProductDraftFlag := productBody["has_draft_changes"]
	_, hasProductDraftUpdated := productBody["draft_updated_at"]
	assert.False(t, hasProductDraftFlag)
	assert.False(t, hasProductDraftUpdated)

	storefrontReq := httptest.NewRequest(http.MethodGet, "/api/v1/storefront", nil)
	storefrontW := httptest.NewRecorder()
	r.ServeHTTP(storefrontW, storefrontReq)
	require.Equal(t, http.StatusOK, storefrontW.Code)
	var storefrontBody map[string]any
	require.NoError(t, json.Unmarshal(storefrontW.Body.Bytes(), &storefrontBody))
	_, hasStorefrontDraftFlag := storefrontBody["has_draft_changes"]
	_, hasStorefrontDraftUpdated := storefrontBody["draft_updated_at"]
	assert.False(t, hasStorefrontDraftFlag)
	assert.False(t, hasStorefrontDraftUpdated)
}

func TestCreateOrderDuplicateItemsAggregateStockValidation(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.Product{},
		&models.ProductVariant{},
		&models.Order{},
		&models.OrderItem{},
	)

	user := seedUser(t, db, "sub-order-aggregate", "order-aggregate", "order-aggregate@example.com", "customer")
	product := seedProduct(t, db, "sku-order-aggregate", "Aggregate Product", 25, 5)
	variantID := requireDefaultVariantID(t, product)
	token := issueBearerTokenWithRole(t, generatedTestJWTSecret, user.Subject, user.Role)

	resp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/orders", map[string]any{
		"items": []map[string]any{
			{"product_variant_id": variantID, "quantity": 3},
			{"product_variant_id": variantID, "quantity": 3},
		},
	}, token)
	require.Equal(t, http.StatusBadRequest, resp.Code)
	body := decodeJSON[map[string]any](t, resp)
	assert.Equal(t, "Insufficient stock", body["error"])
	assert.EqualValues(t, 6, body["requested"])
	assert.EqualValues(t, 5, body["available"])
}

func TestAdminUpdateProductAllowsZeroValues(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{}, &models.Product{})
	admin := seedUser(t, db, "sub-admin-update-zero", "admin-update-zero", "admin-update-zero@example.com", "admin")
	product := seedProduct(t, db, "sku-update-zero", "Update Zero Product", 15, 4)
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)

	updateResp := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/admin/products/%d", product.ID), singleVariantProductUpsertPayload(
		product.SKU,
		product.Name,
		product.Description,
		product.Price.Float64(),
		0,
	), adminToken)
	require.Equal(t, http.StatusOK, updateResp.Code)

	updated := decodeJSON[apicontract.Product](t, updateResp)
	assert.Equal(t, 0, updated.Stock)
	assert.Equal(t, true, *updated.IsPublished)
	assert.Equal(t, true, *updated.HasDraftChanges)

	var reloaded models.Product
	require.NoError(t, db.First(&reloaded, product.ID).Error)
	assert.Equal(t, 4, reloaded.Stock)

	publishResp := performJSONRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/admin/products/%d/publish", product.ID), nil, adminToken)
	require.Equal(t, http.StatusOK, publishResp.Code)

	require.NoError(t, db.First(&reloaded, product.ID).Error)
	assert.Equal(t, 0, reloaded.Stock)
}

func TestCartModelEnforcesSingleCartPerCheckoutSession(t *testing.T) {
	db := newTestDB(t, &models.User{}, &models.CheckoutSession{}, &models.Cart{})
	user := seedUser(t, db, "sub-cart-unique", "cart-unique", "cart-unique@example.com", "customer")
	session := seedCheckoutSession(t, db, &user.ID)

	require.NoError(t, db.Create(&models.Cart{CheckoutSessionID: session.ID}).Error)
	err := db.Create(&models.Cart{CheckoutSessionID: session.ID}).Error
	require.Error(t, err)
}

func TestGeneratedProductFiltersByBrandAttributeAndVariantStock(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.Brand{},
		&models.ProductAttribute{},
		&models.ProductAttributeValue{},
		&models.Product{},
		&models.ProductVariant{},
	)

	acme := models.Brand{Name: "Acme", Slug: "acme", IsActive: true}
	require.NoError(t, db.Create(&acme).Error)
	zen := models.Brand{Name: "Zen", Slug: "zen", IsActive: true}
	require.NoError(t, db.Create(&zen).Error)
	color := models.ProductAttribute{Key: "Color", Slug: "color", Type: "enum", Filterable: true}
	require.NoError(t, db.Create(&color).Error)
	redValue := "Red"
	blueValue := "Blue"

	redProduct := seedProduct(t, db, "sku-filter-red", "Filter Red", 19.99, 8)
	blueProduct := seedProduct(t, db, "sku-filter-blue", "Filter Blue", 18.99, 0)
	otherBrand := seedProduct(t, db, "sku-filter-zen", "Filter Zen", 21.99, 6)

	require.NoError(t, db.Model(&models.Product{}).Where("id = ?", redProduct.ID).Update("brand_id", acme.ID).Error)
	require.NoError(t, db.Model(&models.Product{}).Where("id = ?", blueProduct.ID).Update("brand_id", acme.ID).Error)
	require.NoError(t, db.Model(&models.Product{}).Where("id = ?", otherBrand.ID).Update("brand_id", zen.ID).Error)

	require.NoError(t, db.Create(&models.ProductAttributeValue{
		ProductID:          redProduct.ID,
		ProductAttributeID: color.ID,
		EnumValue:          &redValue,
		Position:           1,
	}).Error)
	require.NoError(t, db.Create(&models.ProductAttributeValue{
		ProductID:          blueProduct.ID,
		ProductAttributeID: color.ID,
		EnumValue:          &blueValue,
		Position:           1,
	}).Error)
	require.NoError(t, db.Create(&models.ProductAttributeValue{
		ProductID:          otherBrand.ID,
		ProductAttributeID: color.ID,
		EnumValue:          &redValue,
		Position:           1,
	}).Error)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/products?brand_slug=acme&has_variant_stock=true&attribute[color]=red",
		nil,
	)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	body := decodeJSON[apicontract.ProductPage](t, w)
	require.Len(t, body.Data, 1)
	assert.Equal(t, redProduct.SKU, body.Data[0].Sku)
	assert.Equal(t, "acme", body.Data[0].Brand.Slug)
}

func TestAdminBrandCRUD(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.Brand{},
		&models.Product{},
		&models.ProductDraft{},
	)
	admin := seedUser(t, db, "sub-admin-brand", "admin-brand", "admin-brand@example.com", "admin")
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)

	createResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/admin/brands", map[string]any{
		"name":        "Acme Labs",
		"description": "Primary brand",
		"is_active":   true,
	}, adminToken)
	require.Equal(t, http.StatusCreated, createResp.Code)
	created := decodeJSON[apicontract.Brand](t, createResp)
	assert.Equal(t, "acme-labs", created.Slug)

	updateResp := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/admin/brands/%d", created.Id), map[string]any{
		"name":      "Acme Refresh",
		"slug":      "acme-refresh",
		"is_active": false,
	}, adminToken)
	require.Equal(t, http.StatusOK, updateResp.Code)
	updated := decodeJSON[apicontract.Brand](t, updateResp)
	assert.Equal(t, "Acme Refresh", updated.Name)
	assert.Equal(t, "acme-refresh", updated.Slug)
	assert.False(t, updated.IsActive)

	listResp := performJSONRequest(t, r, http.MethodGet, "/api/v1/admin/brands", nil, adminToken)
	require.Equal(t, http.StatusOK, listResp.Code)
	listing := decodeJSON[apicontract.BrandListResponse](t, listResp)
	require.Len(t, listing.Data, 1)

	deleteResp := performJSONRequest(
		t,
		r,
		http.MethodDelete,
		fmt.Sprintf("/api/v1/admin/brands/%d", created.Id),
		nil,
		adminToken,
	)
	require.Equal(t, http.StatusOK, deleteResp.Code)

	var count int64
	require.NoError(t, db.Model(&models.Brand{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestAdminBrandListSearch(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.Brand{},
	)
	admin := seedUser(t, db, "sub-admin-brand-search", "admin-brand-search", "admin-brand-search@example.com", "admin")
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)

	description := func(value string) *string {
		return &value
	}

	brands := []models.Brand{
		{
			Name:        "Acme Labs",
			Slug:        "acme-labs",
			Description: description("Precision tools and accessories"),
			IsActive:    true,
		},
		{
			Name:        "Zen House",
			Slug:        "zen-refresh",
			Description: description("Calm essentials"),
			IsActive:    false,
		},
		{
			Name:        "Nova Goods",
			Slug:        "nova-goods",
			Description: description("Bright everyday items"),
			IsActive:    true,
		},
	}
	for i := range brands {
		require.NoError(t, db.Select("*").Create(&brands[i]).Error)
	}

	nameResp := performJSONRequest(t, r, http.MethodGet, "/api/v1/admin/brands?q=acme", nil, adminToken)
	require.Equal(t, http.StatusOK, nameResp.Code)
	nameResults := decodeJSON[apicontract.BrandListResponse](t, nameResp)
	require.Len(t, nameResults.Data, 1)
	assert.Equal(t, "Acme Labs", nameResults.Data[0].Name)

	slugResp := performJSONRequest(t, r, http.MethodGet, "/api/v1/admin/brands?q=refresh", nil, adminToken)
	require.Equal(t, http.StatusOK, slugResp.Code)
	slugResults := decodeJSON[apicontract.BrandListResponse](t, slugResp)
	require.Len(t, slugResults.Data, 1)
	assert.Equal(t, "Zen House", slugResults.Data[0].Name)

	descriptionResp := performJSONRequest(t, r, http.MethodGet, "/api/v1/admin/brands?q=accessories", nil, adminToken)
	require.Equal(t, http.StatusOK, descriptionResp.Code)
	descriptionResults := decodeJSON[apicontract.BrandListResponse](t, descriptionResp)
	require.Len(t, descriptionResults.Data, 1)
	assert.Equal(t, "Acme Labs", descriptionResults.Data[0].Name)
}

func TestAdminProductAttributeCRUD(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.ProductAttribute{},
		&models.ProductAttributeValue{},
		&models.ProductAttributeValueDraft{},
	)
	admin := seedUser(t, db, "sub-admin-attr", "admin-attr", "admin-attr@example.com", "admin")
	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)

	createResp := performJSONRequest(
		t,
		r,
		http.MethodPost,
		"/api/v1/admin/product-attributes",
		map[string]any{
			"key":        "Material",
			"type":       "text",
			"filterable": true,
			"sortable":   false,
		},
		adminToken,
	)
	require.Equal(t, http.StatusCreated, createResp.Code)
	created := decodeJSON[apicontract.ProductAttributeDefinition](t, createResp)
	assert.Equal(t, "material", created.Slug)
	assert.True(t, created.Filterable)

	updateResp := performJSONRequest(
		t,
		r,
		http.MethodPatch,
		fmt.Sprintf("/api/v1/admin/product-attributes/%d", created.Id),
		map[string]any{
			"key":        "Material Type",
			"slug":       "material-type",
			"type":       "enum",
			"filterable": true,
			"sortable":   true,
		},
		adminToken,
	)
	require.Equal(t, http.StatusOK, updateResp.Code)
	updated := decodeJSON[apicontract.ProductAttributeDefinition](t, updateResp)
	assert.Equal(t, "material-type", updated.Slug)
	assert.Equal(t, apicontract.ProductAttributeDefinitionTypeEnum, updated.Type)
	assert.True(t, updated.Sortable)

	listResp := performJSONRequest(t, r, http.MethodGet, "/api/v1/admin/product-attributes", nil, adminToken)
	require.Equal(t, http.StatusOK, listResp.Code)
	listing := decodeJSON[apicontract.ProductAttributeDefinitionListResponse](t, listResp)
	require.Len(t, listing.Data, 1)

	deleteResp := performJSONRequest(
		t,
		r,
		http.MethodDelete,
		fmt.Sprintf("/api/v1/admin/product-attributes/%d", created.Id),
		nil,
		adminToken,
	)
	require.Equal(t, http.StatusOK, deleteResp.Code)

	var count int64
	require.NoError(t, db.Model(&models.ProductAttribute{}).Count(&count).Error)
	assert.Zero(t, count)
}
