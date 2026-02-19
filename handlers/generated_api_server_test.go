package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

const generatedTestJWTSecret = "generated-test-secret"

func setupGeneratedRouter(t *testing.T) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	db := newTestDB(t, &models.Product{}, &models.StorefrontSettings{}, &models.MediaObject{}, &models.MediaReference{})
	require.NoError(t, db.Create(&models.Product{
		SKU:         "sku-1",
		Name:        "Generated Product 1",
		Description: "First",
		Price:       models.MoneyFromFloat(10.99),
		Stock:       4,
	}).Error)
	require.NoError(t, db.Create(&models.Product{
		SKU:         "sku-2",
		Name:        "Generated Product 2",
		Description: "Second",
		Price:       models.MoneyFromFloat(21.50),
		Stock:       8,
	}).Error)

	r := gin.New()
	server := NewGeneratedAPIServer(db, nil, GeneratedAPIServerConfig{
		JWTSecret: generatedTestJWTSecret,
	})
	apicontract.RegisterHandlers(r, server)
	return r
}

func setupGeneratedCartRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	db := newTestDB(t, &models.User{}, &models.Product{}, &models.Cart{}, &models.CartItem{})

	require.NoError(t, db.Create(&models.User{
		Subject:  "sub-cart-1",
		Username: "cart-user",
		Email:    "cart@example.com",
		Role:     "customer",
		Currency: "USD",
	}).Error)
	require.NoError(t, db.Create(&models.Product{
		SKU:         "sku-cart-1",
		Name:        "Cart Product",
		Description: "Cart Product Description",
		Price:       models.MoneyFromFloat(9.99),
		Stock:       20,
	}).Error)

	r := gin.New()
	server := NewGeneratedAPIServer(db, nil, GeneratedAPIServerConfig{
		JWTSecret: generatedTestJWTSecret,
	})
	apicontract.RegisterHandlers(r, server)
	return r, db
}

func issueBearerToken(t *testing.T, subject string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":   subject,
		"email": "cart@example.com",
		"role":  "customer",
		"exp":   time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(generatedTestJWTSecret))
	require.NoError(t, err)
	return signed
}

func TestGeneratedListProductsSuccess(t *testing.T) {
	r := setupGeneratedRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products?sort=name&order=asc&page=1&limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body apicontract.ProductPage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Len(t, body.Data, 2)
	assert.Equal(t, 1, body.Pagination.Page)
	assert.Equal(t, 10, body.Pagination.Limit)
}

func TestGeneratedProductPathValidation(t *testing.T) {
	r := setupGeneratedRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/not-an-int", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGeneratedQueryValidation(t *testing.T) {
	r := setupGeneratedRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products?page=oops", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGeneratedGetProductSuccess(t *testing.T) {
	r := setupGeneratedRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body apicontract.Product
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, 1, body.Id)
	assert.Equal(t, "Generated Product 1", body.Name)
}

func TestGeneratedStorefrontSuccess(t *testing.T) {
	r := setupGeneratedRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/storefront", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body apicontract.StorefrontSettingsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.NotEmpty(t, body.Settings.SiteTitle)
}

func TestGeneratedLogoutEndpoint(t *testing.T) {
	r := setupGeneratedRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGeneratedGetCartRequiresAuth(t *testing.T) {
	r, _ := setupGeneratedCartRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/cart", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGeneratedGetCartWithBearerToken(t *testing.T) {
	r, _ := setupGeneratedCartRouter(t)
	token := issueBearerToken(t, "sub-cart-1")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/cart", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body models.Cart
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.NotZero(t, body.ID)
	assert.Empty(t, body.Items)
}

func TestGeneratedAddCartItemWithBearerToken(t *testing.T) {
	r, _ := setupGeneratedCartRouter(t)
	token := issueBearerToken(t, "sub-cart-1")

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/me/cart",
		strings.NewReader(`{"product_id":1,"quantity":2}`),
	)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body models.Cart
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Items, 1)
	assert.Equal(t, uint(1), body.Items[0].ProductID)
	assert.Equal(t, 2, body.Items[0].Quantity)
}

func TestGeneratedAddCartItemSessionRequiresCSRF(t *testing.T) {
	r, _ := setupGeneratedCartRouter(t)
	token := issueBearerToken(t, "sub-cart-1")

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/me/cart",
		strings.NewReader(`{"product_id":1,"quantity":1}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
