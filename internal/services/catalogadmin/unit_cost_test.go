package catalogadmin

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/jobs"
	"ecommerce/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func unitCostTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Locale{}, &models.LocalizedEntityValue{}, &models.Product{}, &models.ProductVariant{}, &models.ProductDraft{}, &models.ProductVariantDraft{}, &models.ProductRelatedDraft{}, &models.ProductCategory{}, &models.ProductCategoryDraft{}, &models.ProductAttributeValueDraft{}, &models.ProductOptionDraft{}, &models.ProductOptionValueDraft{}, &models.ProductVariantOptionValueDraft{}, &models.ProductOption{}, &models.ProductOptionValue{}, &models.ProductVariantOptionValue{}, &models.ProductAttributeValue{}, &models.SEOMetadata{}, &models.MediaReference{}, &models.Brand{}, &models.Category{}, &models.JobQueue{}))
	require.NoError(t, db.Create(&models.Locale{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true}).Error)
	return NewService(db, nil, jobs.NewRuntime(db, jobs.Config{})), db
}
func unitCostInput(cost *float64) apicontract.ProductUpsertInput {
	return apicontract.ProductUpsertInput{Sku: "COST", Name: "Cost test product", Description: "A product", Variants: []apicontract.ProductVariantInput{{Sku: "COST", Title: "Default", Price: 30, Stock: 5, UnitCost: cost}}}
}
func TestVariantUnitCostValidatesAmountsBeforeWrites(t *testing.T) {
	for _, value := range []float64{-0.01, math.NaN(), math.Inf(1), math.Inf(-1), 10000000000} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			service, db := unitCostTestService(t)
			_, err := service.CreateProduct(context.Background(), unitCostInput(&value))
			require.ErrorContains(t, err, "unit cost")
			var count int64
			require.NoError(t, db.Model(&models.Product{}).Count(&count).Error)
			require.Zero(t, count)
		})
	}
	for _, value := range []float64{0, 12.345, 9999999999.99} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			service, _ := unitCostTestService(t)
			product, err := service.CreateProduct(context.Background(), unitCostInput(&value))
			require.NoError(t, err)
			require.Len(t, product.Variants, 1)
			require.Equal(t, models.MoneyFromFloat(value), *product.Variants[0].UnitCost)
		})
	}
}
func TestVariantUnitCostDraftPublishDiscardUnpublishAndClear(t *testing.T) {
	service, db := unitCostTestService(t)
	ctx := context.Background()
	original := 12.34
	product, err := service.CreateProduct(ctx, unitCostInput(&original))
	require.NoError(t, err)
	require.Equal(t, models.Money(1234), *product.Variants[0].UnitCost)
	product, err = service.PublishProduct(ctx, product.ID)
	require.NoError(t, err)
	require.Equal(t, models.Money(1234), *product.Variants[0].UnitCost)
	live, err := LoadLiveProductUpsertInput(db, nil, product.ID)
	require.NoError(t, err)
	require.Equal(t, original, *live.Variants[0].UnitCost)
	changed := 20.15
	draft, err := service.UpdateProduct(ctx, product.ID, unitCostInput(&changed))
	require.NoError(t, err)
	require.Equal(t, models.Money(2015), *draft.Variants[0].UnitCost)
	current, err := service.GetProduct(ctx, product.ID, false)
	require.NoError(t, err)
	require.Equal(t, models.Money(1234), *current.Variants[0].UnitCost, "draft edits must not change live costs")
	product, err = service.DiscardProductDraft(ctx, product.ID)
	require.NoError(t, err)
	require.Equal(t, models.Money(1234), *product.Variants[0].UnitCost)
	product, err = service.UnpublishProduct(ctx, product.ID)
	require.NoError(t, err)
	require.Equal(t, models.Money(1234), *product.Variants[0].UnitCost, "unpublish copies private costs into the draft")
	product, err = service.PublishProduct(ctx, product.ID)
	require.NoError(t, err)
	require.Equal(t, models.Money(1234), *product.Variants[0].UnitCost)
	zero := 0.0
	_, err = service.UpdateProduct(ctx, product.ID, unitCostInput(&zero))
	require.NoError(t, err)
	product, err = service.PublishProduct(ctx, product.ID)
	require.NoError(t, err)
	require.NotNil(t, product.Variants[0].UnitCost)
	require.Zero(t, *product.Variants[0].UnitCost)
	_, err = service.UpdateProduct(ctx, product.ID, unitCostInput(nil))
	require.NoError(t, err)
	product, err = service.PublishProduct(ctx, product.ID)
	require.NoError(t, err)
	require.Nil(t, product.Variants[0].UnitCost, "null replacement clears persisted costs")
	var stored models.ProductVariant
	require.NoError(t, db.Where("product_id = ?", product.ID).First(&stored).Error)
	require.Nil(t, stored.UnitCost)
}
func TestVariantUnitCostIsNeverInModelJSON(t *testing.T) {
	cost := models.Money(1234)
	for _, value := range []any{models.ProductVariant{Price: 3000, UnitCost: &cost}, models.ProductVariantDraft{Price: 3000, UnitCost: &cost}} {
		encoded, err := json.Marshal(value)
		require.NoError(t, err)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(encoded, &payload))
		require.NotContains(t, payload, "unit_cost")
		require.NotContains(t, payload, "UnitCost")
		require.EqualValues(t, 30, payload["price"])
	}
}
