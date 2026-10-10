package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/httpapi"
	"ecommerce/internal/requestctx"
	localizationservice "ecommerce/internal/services/localization"
	"ecommerce/models"

	"github.com/stretchr/testify/require"
)

func TestCatalogVariantUnitCostIsPrivateEvenForAdminStorefrontRequests(t *testing.T) {
	db := catalogTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Product{}, &models.ProductVariant{}, &models.ProductOption{}, &models.ProductOptionValue{}, &models.ProductVariantOptionValue{}, &models.ProductAttributeValue{}, &models.ProductDraft{}, &models.ProductVariantDraft{}, &models.ProductRelatedDraft{}, &models.ProductCategoryDraft{}, &models.SEOMetadata{}))
	cost := models.Money(1234)
	product := models.Product{SKU: "PRIVATE-COST", Name: "Private cost product", Description: "Public copy", Price: 3000, Stock: 5, IsPublished: true, Variants: []models.ProductVariant{{SKU: "PRIVATE-COST", Title: "Default", Price: 3000, Stock: 5, Position: 1, IsPublished: true, UnitCost: &cost}}}
	require.NoError(t, db.Create(&product).Error)
	require.NoError(t, localizationservice.SyncDefaultEntityLocalization(db, localizationservice.EntityTypeProduct, product.ID, map[string]string{"name": product.Name, "description": product.Description}, nil))
	require.NoError(t, localizationservice.SyncDefaultEntityLocalization(db, localizationservice.EntityTypeProductVariant, product.Variants[0].ID, map[string]string{"title": "Default"}, nil))
	endpoints, err := httpapi.NewCatalogEndpoints(db, nil)
	require.NoError(t, err)
	adminCtx := requestctx.WithPrincipal(context.Background(), requestctx.Principal{Subject: "admin", AccountID: 1, Roles: []string{"admin"}})
	admin, err := endpoints.GetAdminProduct(adminCtx, apicontract.GetAdminProductRequestObject{Id: int(product.ID)})
	require.NoError(t, err)
	adminPayload := admin.(apicontract.GetAdminProduct200JSONResponse)
	require.Len(t, adminPayload.Variants, 1)
	require.Equal(t, 12.34, *adminPayload.Variants[0].UnitCost)
	for _, ctx := range []context.Context{context.Background(), adminCtx} {
		response, err := endpoints.GetProduct(ctx, apicontract.GetProductRequestObject{Id: int(product.ID)})
		require.NoError(t, err)
		recorder := httptest.NewRecorder()
		require.NoError(t, response.VisitGetProductResponse(recorder))
		var payload map[string]any
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
		variants := payload["variants"].([]any)
		require.Len(t, variants, 1)
		require.NotContains(t, variants[0].(map[string]any), "unit_cost")
		listing, err := endpoints.ListProducts(ctx, apicontract.ListProductsRequestObject{})
		require.NoError(t, err)
		recorder = httptest.NewRecorder()
		require.NoError(t, listing.VisitListProductsResponse(recorder))
		require.NotContains(t, recorder.Body.String(), "unit_cost")
	}
}
