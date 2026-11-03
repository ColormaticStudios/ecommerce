package catalogadmin

import (
	"context"
	"ecommerce/internal/apicontract"
	"ecommerce/models"
	"fmt"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
)

func TestProductInputsBatchPreservesCompleteDraftsAndLiveMetadata(t *testing.T) {
	service, db := unitCostTestService(t)
	ctx := context.Background()
	ids := []uint{}
	for i := 0; i < 8; i++ {
		cost := 12.34
		input := unitCostInput(&cost)
		input.Sku = fmt.Sprintf("batch%d", i)
		input.Variants[0].Sku = fmt.Sprintf("batch%d-v", i)
		input.Options = []apicontract.ProductOptionInput{{Name: "Size", Values: []apicontract.ProductOptionValueInput{{Value: "M"}}}}
		input.Variants[0].Selections = []apicontract.ProductVariantSelectionInput{{OptionName: "Size", OptionValue: "M"}}
		input.Seo.Title = func() *string { v := "SEO title"; return &v }()
		p, err := service.CreateProduct(ctx, input)
		require.NoError(t, err)
		ids = append(ids, p.ID)
	}
	live := models.Product{Name: "Live", SKU: "live", Price: models.MoneyFromFloat(12)}
	require.NoError(t, db.Create(&live).Error)
	require.NoError(t, db.Create(&models.ProductVariant{ProductID: live.ID, SKU: "live-v", Title: "Live variant", Price: models.MoneyFromFloat(12), IsPublished: true}).Error)
	ids = append(ids, live.ID)
	expected := map[uint]apicontract.ProductUpsertInput{}
	for _, id := range ids {
		input, err := service.ProductInput(ctx, id)
		require.NoError(t, err)
		expected[id] = input
	}
	queries := 0
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("batch_count", func(*gorm.DB) { queries++ }))
	single, err := service.ProductInputs(ctx, ids[:1])
	require.NoError(t, err)
	require.Equal(t, expected[ids[0]], single[ids[0]])
	singleQueries := queries
	queries = 0
	batch, err := service.ProductInputs(ctx, ids[:8])
	require.NoError(t, err)
	require.Equal(t, singleQueries, queries, "draft query count must not grow with page size")
	for _, id := range ids[:8] {
		require.Equal(t, expected[id], batch[id])
	}
	all, err := service.ProductInputs(ctx, ids)
	require.NoError(t, err)
	require.Equal(t, expected, all)
	_, err = service.ProductInputs(ctx, []uint{999999})
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.NoError(t, db.Model(&models.ProductDraft{}).Where("product_id = ?", ids[0]).Update("images_json", "broken").Error)
	_, err = service.ProductInputs(ctx, ids)
	require.ErrorContains(t, err, "decode product draft images")
}
