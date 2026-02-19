package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ecommerce/internal/apicontract"
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
