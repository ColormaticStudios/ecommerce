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
	db := newTestDB(t, &models.Product{}, &models.MediaObject{}, &models.MediaReference{})
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
	server, err := NewGeneratedAPIServer(db, nil, GeneratedAPIServerConfig{
		JWTSecret: generatedTestJWTSecret,
	})
	require.NoError(t, err)
	apicontract.RegisterHandlers(r, server)
	return r
}

func setupGeneratedCartRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	db := newTestDB(t, &models.User{}, &models.Product{}, &models.ProductVariant{}, &models.Cart{}, &models.CartItem{})

	require.NoError(t, db.Create(&models.User{
		Subject:  "sub-cart-1",
		Username: "cart-user",
		Email:    "cart@example.com",
		Role:     "customer",
		Currency: "USD",
	}).Error)
	product := models.Product{
		SKU:         "sku-cart-1",
		Name:        "Cart Product",
		Description: "Cart Product Description",
		Price:       models.MoneyFromFloat(9.99),
		Stock:       20,
	}
	require.NoError(t, db.Create(&product).Error)
	variant := models.ProductVariant{
		ProductID:   product.ID,
		SKU:         "sku-cart-1-default",
		Title:       product.Name,
		Price:       product.Price,
		Stock:       product.Stock,
		Position:    1,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&variant).Error)
	require.NoError(t, db.Model(&product).Update("default_variant_id", variant.ID).Error)

	r := gin.New()
	server, err := NewGeneratedAPIServer(db, nil, GeneratedAPIServerConfig{
		JWTSecret: generatedTestJWTSecret,
	})
	require.NoError(t, err)
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

func TestGeneratedProfileReturnsUnauthorizedWhenTokenSubjectMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t, &models.User{})
	r := gin.New()
	server, err := NewGeneratedAPIServer(db, nil, GeneratedAPIServerConfig{
		JWTSecret: generatedTestJWTSecret,
	})
	require.NoError(t, err)
	apicontract.RegisterHandlers(r, server)

	token := issueBearerToken(t, "missing-subject")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
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

func TestGeneratedCMSPageCreatePublishResolveRollback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t)
	admin := seedUser(t, db, "sub-cms-admin", "cms-admin", "cms-admin@example.com", "admin")
	token := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)

	r := gin.New()
	server, err := NewGeneratedAPIServer(db, nil, GeneratedAPIServerConfig{JWTSecret: generatedTestJWTSecret})
	require.NoError(t, err)
	apicontract.RegisterHandlers(r, server)

	createW := adminLifecycleRequest(t, r, http.MethodPost, "/api/v1/admin/cms/pages", `{
		"path": "/shipping",
		"title": "Shipping",
		"payload": {"headline": "Draft shipping"}
	}`, token, "")
	require.Equal(t, http.StatusCreated, createW.Code)
	var created apicontract.CmsPageResponse
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))
	require.True(t, created.HasUnpublishedDraft)

	unpublishedW := httptest.NewRecorder()
	r.ServeHTTP(unpublishedW, httptest.NewRequest(http.MethodGet, "/api/v1/content/shipping", nil))
	require.Equal(t, http.StatusNotFound, unpublishedW.Code)

	publishW := adminLifecycleRequest(t, r, http.MethodPost, "/api/v1/admin/cms/pages/1/publish", `{"notes":"launch"}`, token, "")
	require.Equal(t, http.StatusOK, publishW.Code)
	var published apicontract.CmsPageResponse
	require.NoError(t, json.Unmarshal(publishW.Body.Bytes(), &published))
	require.NotNil(t, published.PublishedVersion)
	require.False(t, published.HasUnpublishedDraft)

	resolveW := httptest.NewRecorder()
	r.ServeHTTP(resolveW, httptest.NewRequest(http.MethodGet, "/api/v1/content/shipping", nil))
	require.Equal(t, http.StatusOK, resolveW.Code)
	var resolved apicontract.CmsPageResponse
	require.NoError(t, json.Unmarshal(resolveW.Body.Bytes(), &resolved))
	var resolvedRaw map[string]any
	require.NoError(t, json.Unmarshal(resolveW.Body.Bytes(), &resolvedRaw))
	publishedVersion := resolvedRaw["published_version"].(map[string]any)
	publishedPayload := publishedVersion["payload"].(map[string]any)
	assert.Equal(t, "Draft shipping", publishedPayload["headline"])

	updateW := adminLifecycleRequest(t, r, http.MethodPatch, "/api/v1/admin/cms/pages/1", `{
		"path": "/shipping",
		"title": "Shipping",
		"payload": {"headline": "Updated shipping"}
	}`, token, "")
	require.Equal(t, http.StatusOK, updateW.Code)

	rollbackW := adminLifecycleRequest(t, r, http.MethodPost, "/api/v1/admin/cms/pages/1/rollback", `{"version_id":1}`, token, "")
	require.Equal(t, http.StatusOK, rollbackW.Code)
	var rolledBack apicontract.CmsPageResponse
	require.NoError(t, json.Unmarshal(rollbackW.Body.Bytes(), &rolledBack))
	require.NotNil(t, rolledBack.PublishedVersion)
	assert.Equal(t, 1, rolledBack.PublishedVersion.Id)
}

func TestGeneratedCMSResolveUsesDraftPreviewForCurrentVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t)
	admin := seedUser(t, db, "sub-cms-preview", "cms-preview", "cms-preview@example.com", "admin")
	token := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)

	r := gin.New()
	server, err := NewGeneratedAPIServer(db, nil, GeneratedAPIServerConfig{JWTSecret: generatedTestJWTSecret})
	require.NoError(t, err)
	apicontract.RegisterHandlers(r, server)

	createW := adminLifecycleRequest(t, r, http.MethodPost, "/api/v1/admin/cms/pages", `{
		"path": "/returns",
		"title": "Returns",
		"payload": {"blocks": [{"type": "rich_text", "body": "Draft return policy"}]}
	}`, token, "")
	require.Equal(t, http.StatusCreated, createW.Code)

	publicW := httptest.NewRecorder()
	r.ServeHTTP(publicW, httptest.NewRequest(http.MethodGet, "/api/v1/content/returns", nil))
	require.Equal(t, http.StatusNotFound, publicW.Code)

	previewToken, _, err := buildDraftPreviewToken(admin.Subject, admin.Role, generatedTestJWTSecret, time.Minute)
	require.NoError(t, err)
	previewReq := httptest.NewRequest(http.MethodGet, "/api/v1/content/returns", nil)
	previewReq.AddCookie(&http.Cookie{Name: draftPreviewCookieName, Value: previewToken})
	previewW := httptest.NewRecorder()
	r.ServeHTTP(previewW, previewReq)
	require.Equal(t, http.StatusOK, previewW.Code)
	assert.Equal(t, "private, no-store", previewW.Header().Get("Cache-Control"))

	var previewBody apicontract.CmsPageResponse
	require.NoError(t, json.Unmarshal(previewW.Body.Bytes(), &previewBody))
	require.NotNil(t, previewBody.CurrentVersion)
	var previewRaw map[string]any
	require.NoError(t, json.Unmarshal(previewW.Body.Bytes(), &previewRaw))
	currentVersion := previewRaw["current_version"].(map[string]any)
	currentPayload := currentVersion["payload"].(map[string]any)
	currentBlocks := currentPayload["blocks"].([]any)
	assert.Equal(t, "Draft return policy", currentBlocks[0].(map[string]any)["body"])
}

func TestGeneratedCMSNavigationAndGlobalRegionPublishResolve(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newTestDB(t)
	admin := seedUser(t, db, "sub-cms-p2", "cms-p2", "cms-p2@example.com", "admin")
	token := issueBearerTokenWithRole(t, generatedTestJWTSecret, admin.Subject, admin.Role)

	r := gin.New()
	server, err := NewGeneratedAPIServer(db, nil, GeneratedAPIServerConfig{JWTSecret: generatedTestJWTSecret})
	require.NoError(t, err)
	apicontract.RegisterHandlers(r, server)

	navW := adminLifecycleRequest(t, r, http.MethodPost, "/api/v1/admin/cms/navigation", `{
		"key": "main",
		"title": "Main",
		"location": "header",
		"items": [
			{"id": 1, "label": "Shop", "item_type": "dropdown", "target_ref": "", "url": "", "sort_order": 1, "is_enabled": true},
			{"id": 2, "parent_id": 1, "label": "Search", "item_type": "internal", "target_ref": "/search", "url": "/search", "sort_order": 2, "is_enabled": true}
		]
	}`, token, "")
	require.Equal(t, http.StatusCreated, navW.Code)
	navPublishW := adminLifecycleRequest(t, r, http.MethodPost, "/api/v1/admin/cms/navigation/1/publish", `{"notes":"publish nav"}`, token, "")
	require.Equal(t, http.StatusOK, navPublishW.Code)
	navPublicW := httptest.NewRecorder()
	r.ServeHTTP(navPublicW, httptest.NewRequest(http.MethodGet, "/api/v1/content/navigation/header", nil))
	require.Equal(t, http.StatusOK, navPublicW.Code)
	var navBody apicontract.CmsNavigationResponse
	require.NoError(t, json.Unmarshal(navPublicW.Body.Bytes(), &navBody))
	require.Len(t, navBody.Items, 2)
	assert.Equal(t, "Shop", navBody.Items[0].Label)
	assert.Equal(t, apicontract.CmsNavigationItemItemTypeDropdown, navBody.Items[0].ItemType)
	assert.Equal(t, "Search", navBody.Items[1].Label)
	require.NotNil(t, navBody.Items[1].ParentId)
	assert.Equal(t, navBody.Items[0].Id, *navBody.Items[1].ParentId)

	globalW := adminLifecycleRequest(t, r, http.MethodPost, "/api/v1/admin/cms/global", `{
		"key": "announcement",
		"title": "Announcement",
		"region": "announcement_bar",
		"payload": {"blocks": [{"type": "promo_banner", "title": "Free shipping"}]}
	}`, token, "")
	require.Equal(t, http.StatusCreated, globalW.Code)
	globalPublishW := adminLifecycleRequest(t, r, http.MethodPost, "/api/v1/admin/cms/global/1/publish", `{"notes":"publish global"}`, token, "")
	require.Equal(t, http.StatusOK, globalPublishW.Code)
	globalPublicW := httptest.NewRecorder()
	r.ServeHTTP(globalPublicW, httptest.NewRequest(http.MethodGet, "/api/v1/content/global/announcement_bar", nil))
	require.Equal(t, http.StatusOK, globalPublicW.Code)
	var globalRaw map[string]any
	require.NoError(t, json.Unmarshal(globalPublicW.Body.Bytes(), &globalRaw))
	publishedVersion := globalRaw["published_version"].(map[string]any)
	payload := publishedVersion["payload"].(map[string]any)
	blocks := payload["blocks"].([]any)
	assert.Equal(t, "Free shipping", blocks[0].(map[string]any)["title"])
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

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.NotZero(t, body["id"])
	assert.Empty(t, body["items"])
}

func TestGeneratedAddCartItemWithBearerToken(t *testing.T) {
	r, _ := setupGeneratedCartRouter(t)
	token := issueBearerToken(t, "sub-cart-1")

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/me/cart",
		strings.NewReader(`{"product_variant_id":1,"quantity":2}`),
	)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	items, ok := body["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	first, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1), first["product_variant_id"])
	assert.Equal(t, float64(2), first["quantity"])
}

func TestGeneratedAddCartItemSessionRequiresCSRF(t *testing.T) {
	r, _ := setupGeneratedCartRouter(t)
	token := issueBearerToken(t, "sub-cart-1")

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/me/cart",
		strings.NewReader(`{"product_variant_id":1,"quantity":1}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session_token", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
