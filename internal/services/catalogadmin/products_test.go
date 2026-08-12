package catalogadmin

import (
	"context"
	"fmt"
	"testing"

	"ecommerce/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPublishProductUpdatesExistingVariantWithSameSKU(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Product{},
		&models.ProductVariant{},
		&models.ProductDraft{},
		&models.ProductVariantDraft{},
		&models.ProductRelatedDraft{},
		&models.ProductCategory{},
		&models.ProductCategoryDraft{},
		&models.ProductAttributeValueDraft{},
		&models.ProductOptionDraft{},
		&models.MediaReference{},
	))
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX idx_product_variants_sku_unique ON product_variants (sku)").Error)

	product := models.Product{SKU: "CLPILLO-001", Name: "Colormatic Logo Pillow", Price: models.MoneyFromFloat(15), Stock: 100, IsPublished: true}
	require.NoError(t, db.Create(&product).Error)
	variant := models.ProductVariant{ProductID: product.ID, SKU: product.SKU, Title: product.Name, Price: product.Price, Stock: product.Stock, Position: 1, IsPublished: true}
	require.NoError(t, db.Create(&variant).Error)
	require.NoError(t, db.Model(&product).Update("default_variant_id", variant.ID).Error)

	draft := models.ProductDraft{ProductID: product.ID, SKU: product.SKU, DefaultVariantSKU: product.SKU, Name: product.Name, Price: product.Price, Stock: product.Stock, ImagesJSON: "[]"}
	require.NoError(t, db.Create(&draft).Error)
	draftVariant := models.ProductVariantDraft{ProductDraftID: draft.ID, SKU: variant.SKU, Title: "Updated Pillow", Price: models.MoneyFromFloat(20), Stock: 75, Position: 1, IsPublished: true}
	require.NoError(t, db.Create(&draftVariant).Error)

	published, err := NewService(db, nil).PublishProduct(context.Background(), product.ID)
	require.NoError(t, err)
	require.Len(t, published.Variants, 1)
	require.Equal(t, variant.ID, published.Variants[0].ID)
	require.Equal(t, "Updated Pillow", published.Variants[0].Title)
	require.Equal(t, 20.0, published.Variants[0].Price.Float64())
	require.NotNil(t, published.DefaultVariantID)
	require.Equal(t, variant.ID, *published.DefaultVariantID)
}
