package catalog

import (
	"fmt"
	"testing"

	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Product{}, &models.ProductVariant{}))
	return db
}

func TestListProductsExcludesPublicProductsWithoutPublishedVariants(t *testing.T) {
	db := newRepositoryTestDB(t)
	repo := NewRepository(db)

	legacy := models.Product{
		SKU:         "LEGACY",
		Name:        "Legacy",
		Description: "legacy",
		Price:       models.MoneyFromFloat(10),
		Stock:       1,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&legacy).Error)

	publicWithVariant := models.Product{
		SKU:         "PUBLIC-VARIANT",
		Name:        "Public Variant",
		Description: "public variant",
		Price:       models.MoneyFromFloat(11),
		Stock:       1,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&publicWithVariant).Error)
	require.NoError(t, db.Create(&models.ProductVariant{
		ProductID:   publicWithVariant.ID,
		SKU:         "PUBLIC-VARIANT-1",
		Title:       "Public Variant",
		Price:       models.MoneyFromFloat(12),
		Stock:       2,
		Position:    1,
		IsPublished: true,
	}).Error)

	hiddenOnly := models.Product{
		SKU:         "HIDDEN-ONLY",
		Name:        "Hidden Only",
		Description: "hidden only",
		Price:       models.MoneyFromFloat(13),
		Stock:       1,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&hiddenOnly).Error)
	require.NoError(t, db.Select("*").Create(&models.ProductVariant{
		ProductID:   hiddenOnly.ID,
		SKU:         "HIDDEN-ONLY-1",
		Title:       "Hidden Variant",
		Price:       models.MoneyFromFloat(14),
		Stock:       0,
		Position:    1,
		IsPublished: false,
	}).Error)
	require.NoError(t, db.Model(&models.ProductVariant{}).
		Where("product_id = ?", hiddenOnly.ID).
		Update("is_published", false).Error)

	result, err := repo.ListProducts(ProductListFilters{
		Page:  1,
		Limit: 20,
	})
	require.NoError(t, err)

	skus := make([]string, 0, len(result.Products))
	for _, product := range result.Products {
		skus = append(skus, product.SKU)
	}

	assert.ElementsMatch(t, []string{"LEGACY", "PUBLIC-VARIANT"}, skus)
}

func TestGetPublicProductByIDRejectsProductsWithoutPublishedVariants(t *testing.T) {
	db := newRepositoryTestDB(t)
	repo := NewRepository(db)

	product := models.Product{
		SKU:         "HIDDEN-ONLY",
		Name:        "Hidden Only",
		Description: "hidden only",
		Price:       models.MoneyFromFloat(13),
		Stock:       1,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&product).Error)
	require.NoError(t, db.Select("*").Create(&models.ProductVariant{
		ProductID:   product.ID,
		SKU:         "HIDDEN-ONLY-1",
		Title:       "Hidden Variant",
		Price:       models.MoneyFromFloat(14),
		Stock:       0,
		Position:    1,
		IsPublished: false,
	}).Error)
	require.NoError(t, db.Model(&models.ProductVariant{}).
		Where("product_id = ?", product.ID).
		Update("is_published", false).Error)

	_, err := repo.GetPublicProductByID(fmt.Sprintf("%d", product.ID))
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
