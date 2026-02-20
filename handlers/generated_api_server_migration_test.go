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
	server := NewGeneratedAPIServer(db, nil, cfg)
	apicontract.RegisterHandlers(r, server)
	return r, db
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
	return product
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
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{}, &models.Product{}, &models.Cart{}, &models.CartItem{})
	user := seedUser(t, db, "sub-csrf", "csrf-user", "csrf@example.com", "customer")
	product := seedProduct(t, db, "sku-csrf", "CSRF Product", 12.99, 10)
	token := issueBearerTokenWithRole(t, generatedTestJWTSecret, user.Subject, user.Role)

	bearerReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart", strings.NewReader(`{"product_id":`+strconv.Itoa(int(product.ID))+`,"quantity":1}`))
	bearerReq.Header.Set("Authorization", "Bearer "+token)
	bearerReq.Header.Set("Content-Type", "application/json")
	bearerW := httptest.NewRecorder()
	r.ServeHTTP(bearerW, bearerReq)
	assert.Equal(t, http.StatusOK, bearerW.Code)

	sessionNoCsrfReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart", strings.NewReader(`{"product_id":`+strconv.Itoa(int(product.ID))+`,"quantity":1}`))
	sessionNoCsrfReq.Header.Set("Content-Type", "application/json")
	sessionNoCsrfReq.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	sessionNoCsrfW := httptest.NewRecorder()
	r.ServeHTTP(sessionNoCsrfW, sessionNoCsrfReq)
	assert.Equal(t, http.StatusForbidden, sessionNoCsrfW.Code)

	sessionCsrfReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart", strings.NewReader(`{"product_id":`+strconv.Itoa(int(product.ID))+`,"quantity":2}`))
	sessionCsrfReq.Header.Set("Content-Type", "application/json")
	sessionCsrfReq.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	sessionCsrfReq.AddCookie(&http.Cookie{Name: "csrf_token", Value: "csrf-123"})
	sessionCsrfReq.Header.Set("X-CSRF-Token", "csrf-123")
	sessionCsrfW := httptest.NewRecorder()
	r.ServeHTTP(sessionCsrfW, sessionCsrfReq)
	assert.Equal(t, http.StatusOK, sessionCsrfW.Code)

	invalidDataReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart", strings.NewReader(`{"product_id":`+strconv.Itoa(int(product.ID))+`,"quantity":0}`))
	invalidDataReq.Header.Set("Content-Type", "application/json")
	invalidDataReq.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	invalidDataReq.AddCookie(&http.Cookie{Name: "csrf_token", Value: "csrf-123"})
	invalidDataReq.Header.Set("X-CSRF-Token", "csrf-123")
	invalidDataW := httptest.NewRecorder()
	r.ServeHTTP(invalidDataW, invalidDataReq)
	assert.Equal(t, http.StatusBadRequest, invalidDataW.Code)
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
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{}, &models.Product{}, &models.Cart{}, &models.CartItem{}, &models.Order{}, &models.OrderItem{})
	product := seedProduct(t, db, "sku-smoke-1", "Smoke Product", 15.50, 10)

	registerReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"username":"smoke-user","email":"smoke@example.com","password":"supersecret"}`))
	registerReq.Header.Set("Content-Type", "application/json")
	registerW := httptest.NewRecorder()
	r.ServeHTTP(registerW, registerReq)
	assert.Equal(t, http.StatusCreated, registerW.Code)

	var authResp AuthResponse
	require.NoError(t, json.Unmarshal(registerW.Body.Bytes(), &authResp))
	require.NotEmpty(t, authResp.User.Subject)

	token := issueBearerTokenWithRole(t, generatedTestJWTSecret, authResp.User.Subject, "customer")

	addCartReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart", strings.NewReader(`{"product_id":`+strconv.Itoa(int(product.ID))+`,"quantity":2}`))
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

	createValidOrderReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/orders", strings.NewReader(`{"items":[{"product_id":`+strconv.Itoa(int(product.ID))+`,"quantity":2}]}`))
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
	server := NewGeneratedAPIServer(db, mediaService, GeneratedAPIServerConfig{JWTSecret: generatedTestJWTSecret})
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
	require.NoError(t, db.Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeProduct, product.ID, media.RoleProductImage).Order("position asc").Find(&refs).Error)
	require.Len(t, refs, 2)
	assert.Equal(t, prodB.ID, refs[0].MediaID)
	assert.Equal(t, prodA.ID, refs[1].MediaID)

	detachResp := performJSONRequest(t, r, http.MethodDelete, fmt.Sprintf("/api/v1/admin/products/%d/media/%s", product.ID, prodA.ID), nil, adminToken)
	require.Equal(t, http.StatusOK, detachResp.Code)

	var count int64
	require.NoError(t, db.Model(&models.MediaReference{}).Where("owner_type = ? AND owner_id = ? AND role = ?", media.OwnerTypeProduct, product.ID, media.RoleProductImage).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestCartUpdateDeleteOwnershipIsolation(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{}, &models.Product{}, &models.Cart{}, &models.CartItem{})
	owner := seedUser(t, db, "sub-owner", "owner", "owner@example.com", "customer")
	other := seedUser(t, db, "sub-other", "other", "other@example.com", "customer")
	product := seedProduct(t, db, "sku-cart-gap", "Cart Gap Product", 10, 5)

	ownerToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, owner.Subject, owner.Role)
	otherToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, other.Subject, other.Role)

	addResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/cart", map[string]any{"product_id": product.ID, "quantity": 1}, ownerToken)
	require.Equal(t, http.StatusOK, addResp.Code)
	cart := decodeJSON[models.Cart](t, addResp)
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
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{}, &models.Product{}, &models.Order{}, &models.OrderItem{})
	user := seedUser(t, db, "sub-order-gap", "order-gap", "order-gap@example.com", "customer")
	product := seedProduct(t, db, "sku-order-gap", "Order Gap Product", 30, 8)
	token := issueBearerTokenWithRole(t, generatedTestJWTSecret, user.Subject, user.Role)

	createOrderResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/orders", map[string]any{
		"items": []map[string]any{{"product_id": product.ID, "quantity": 1}},
	}, token)
	require.Equal(t, http.StatusCreated, createOrderResp.Code)
	createdOrder := decodeJSON[models.Order](t, createOrderResp)

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

	getOrderResp := performJSONRequest(t, r, http.MethodGet, fmt.Sprintf("/api/v1/me/orders/%d", createdOrder.ID), nil, token)
	require.Equal(t, http.StatusOK, getOrderResp.Code)
}

func TestAdminUpdateOrderStatusDeductsStockOnceAndRollbackOnFailure(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(t, GeneratedAPIServerConfig{}, &models.User{}, &models.Product{}, &models.Order{}, &models.OrderItem{})
	admin := seedUser(t, db, "sub-admin-order", "admin-order", "admin-order@example.com", "admin")
	customer := seedUser(t, db, "sub-customer-order", "customer-order", "customer-order@example.com", "customer")
	product := seedProduct(t, db, "sku-stock-gap", "Stock Gap Product", 12.5, 5)

	order := models.Order{UserID: customer.ID, Status: models.StatusPending, Total: models.MoneyFromFloat(25)}
	require.NoError(t, db.Create(&order).Error)
	require.NoError(t, db.Create(&models.OrderItem{OrderID: order.ID, ProductID: product.ID, Quantity: 2, Price: models.MoneyFromFloat(12.5)}).Error)

	adminToken := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)

	firstPay := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/admin/orders/%d/status", order.ID), map[string]any{"status": models.StatusPaid}, adminToken)
	require.Equal(t, http.StatusOK, firstPay.Code)

	var reloaded models.Product
	require.NoError(t, db.First(&reloaded, product.ID).Error)
	assert.Equal(t, 3, reloaded.Stock)

	secondPay := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/admin/orders/%d/status", order.ID), map[string]any{"status": models.StatusPaid}, adminToken)
	require.Equal(t, http.StatusOK, secondPay.Code)
	require.NoError(t, db.First(&reloaded, product.ID).Error)
	assert.Equal(t, 3, reloaded.Stock)

	reverseToFailed := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/admin/orders/%d/status", order.ID), map[string]any{"status": models.StatusFailed}, adminToken)
	require.Equal(t, http.StatusOK, reverseToFailed.Code)
	require.NoError(t, db.First(&reloaded, product.ID).Error)
	assert.Equal(t, 5, reloaded.Stock)

	reapplyPaid := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/admin/orders/%d/status", order.ID), map[string]any{"status": models.StatusPaid}, adminToken)
	require.Equal(t, http.StatusOK, reapplyPaid.Code)
	require.NoError(t, db.First(&reloaded, product.ID).Error)
	assert.Equal(t, 3, reloaded.Stock)

	lowStockProduct := seedProduct(t, db, "sku-low-stock", "Low Stock Product", 9.99, 1)
	failingOrder := models.Order{UserID: customer.ID, Status: models.StatusPending, Total: models.MoneyFromFloat(29.97)}
	require.NoError(t, db.Create(&failingOrder).Error)
	require.NoError(t, db.Create(&models.OrderItem{OrderID: failingOrder.ID, ProductID: lowStockProduct.ID, Quantity: 2, Price: models.MoneyFromFloat(9.99)}).Error)

	failPay := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/admin/orders/%d/status", failingOrder.ID), map[string]any{"status": models.StatusPaid}, adminToken)
	require.Equal(t, http.StatusBadRequest, failPay.Code)
	assert.Contains(t, strings.ToLower(failPay.Body.String()), "insufficient stock")

	var orderAfter models.Order
	require.NoError(t, db.First(&orderAfter, failingOrder.ID).Error)
	assert.Equal(t, models.StatusPending, orderAfter.Status)
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

func TestProcessPaymentRemovesOnlyOrderedCartQuantities(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.Product{},
		&models.Cart{},
		&models.CartItem{},
		&models.Order{},
		&models.OrderItem{},
	)

	user := seedUser(t, db, "sub-partial-checkout", "partial-checkout", "partial-checkout@example.com", "customer")
	productA := seedProduct(t, db, "sku-partial-a", "Partial A", 10, 20)
	productB := seedProduct(t, db, "sku-partial-b", "Partial B", 12, 20)
	token := issueBearerTokenWithRole(t, generatedTestJWTSecret, user.Subject, user.Role)

	cart := models.Cart{UserID: user.ID}
	require.NoError(t, db.Create(&cart).Error)
	require.NoError(t, db.Create(&models.CartItem{CartID: cart.ID, ProductID: productA.ID, Quantity: 3}).Error)
	require.NoError(t, db.Create(&models.CartItem{CartID: cart.ID, ProductID: productB.ID, Quantity: 4}).Error)

	createOrderResp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/orders", map[string]any{
		"items": []map[string]any{{"product_id": productA.ID, "quantity": 2}},
	}, token)
	require.Equal(t, http.StatusCreated, createOrderResp.Code)
	order := decodeJSON[models.Order](t, createOrderResp)

	payResp := performJSONRequest(t, r, http.MethodPost, fmt.Sprintf("/api/v1/me/orders/%d/pay", order.ID), map[string]any{
		"payment_method": map[string]any{
			"cardholder_name": "Partial Checkout",
			"card_number":     "4111111111111111",
			"exp_month":       12,
			"exp_year":        2035,
		},
		"address": map[string]any{
			"full_name":   "Partial Checkout",
			"line1":       "100 Main St",
			"city":        "Austin",
			"postal_code": "78701",
			"country":     "US",
		},
	}, token)
	require.Equal(t, http.StatusOK, payResp.Code)

	var remaining []models.CartItem
	require.NoError(t, db.Where("cart_id = ?", cart.ID).Order("product_id asc").Find(&remaining).Error)
	require.Len(t, remaining, 2)
	assert.Equal(t, productA.ID, remaining[0].ProductID)
	assert.Equal(t, 1, remaining[0].Quantity)
	assert.Equal(t, productB.ID, remaining[1].ProductID)
	assert.Equal(t, 4, remaining[1].Quantity)
}

func TestCreateOrderDuplicateItemsAggregateStockValidation(t *testing.T) {
	r, db := setupGeneratedRouterWithConfig(
		t,
		GeneratedAPIServerConfig{},
		&models.User{},
		&models.Product{},
		&models.Order{},
		&models.OrderItem{},
	)

	user := seedUser(t, db, "sub-order-aggregate", "order-aggregate", "order-aggregate@example.com", "customer")
	product := seedProduct(t, db, "sku-order-aggregate", "Aggregate Product", 25, 5)
	token := issueBearerTokenWithRole(t, generatedTestJWTSecret, user.Subject, user.Role)

	resp := performJSONRequest(t, r, http.MethodPost, "/api/v1/me/orders", map[string]any{
		"items": []map[string]any{
			{"product_id": product.ID, "quantity": 3},
			{"product_id": product.ID, "quantity": 3},
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

	updateResp := performJSONRequest(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/admin/products/%d", product.ID), map[string]any{
		"stock": 0,
	}, adminToken)
	require.Equal(t, http.StatusOK, updateResp.Code)

	updated := decodeJSON[models.Product](t, updateResp)
	assert.Equal(t, 0, updated.Stock)

	var reloaded models.Product
	require.NoError(t, db.First(&reloaded, product.ID).Error)
	assert.Equal(t, 0, reloaded.Stock)
}

func TestCartModelEnforcesSingleCartPerUser(t *testing.T) {
	db := newTestDB(t, &models.User{}, &models.Cart{})
	user := seedUser(t, db, "sub-cart-unique", "cart-unique", "cart-unique@example.com", "customer")

	require.NoError(t, db.Create(&models.Cart{UserID: user.ID}).Error)
	err := db.Create(&models.Cart{UserID: user.ID}).Error
	require.Error(t, err)
}
