package catalogadmin

import (
	"context"
	"fmt"
	"testing"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/jobs"
	searchservice "ecommerce/internal/search"
	"ecommerce/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBrandMutationsSynchronizeDefaultLocalization(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Locale{}, &models.LocalizedEntityValue{}, &models.Brand{}, &models.MediaReference{}))
	require.NoError(t, db.Select("*").Create(&models.Locale{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true}).Error)

	service := NewService(db, nil)
	description := "Original description"
	slug := "original"
	active := true
	brand, err := service.CreateBrand(context.Background(), apicontract.BrandInput{Name: "Original", Slug: &slug, Description: &description, IsActive: &active})
	require.NoError(t, err)
	description = "Updated description"
	slug = "updated"
	_, err = service.UpdateBrand(context.Background(), brand.ID, apicontract.BrandInput{Name: "Updated", Slug: &slug, Description: &description, IsActive: &active})
	require.NoError(t, err)

	var rows []models.LocalizedEntityValue
	require.NoError(t, db.Where("entity_type = ? AND entity_id = ?", "brand", brand.ID).Order("field").Find(&rows).Error)
	require.Len(t, rows, 2)
	require.Equal(t, "Updated description", rows[0].Value)
	require.Equal(t, "Updated", rows[1].Value)
}

func TestCreateBrandPersistsExplicitlyInactiveState(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Locale{}, &models.LocalizedEntityValue{}, &models.Brand{}, &models.MediaReference{}))
	require.NoError(t, db.Select("*").Create(&models.Locale{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true}).Error)

	inactive := false
	slug := "inactive-brand"
	brand, err := NewService(db, nil).CreateBrand(context.Background(), apicontract.BrandInput{Name: "Inactive Brand", Slug: &slug, IsActive: &inactive})
	require.NoError(t, err)
	require.False(t, brand.IsActive, "explicitly inactive brand must persist as inactive")

	var reloaded models.Brand
	require.NoError(t, db.First(&reloaded, brand.ID).Error)
	require.False(t, reloaded.IsActive, "explicitly inactive brand must be persisted as inactive in the database")
}

func TestCreateCategoryPersistsExplicitlyInactiveState(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Locale{}, &models.LocalizedEntityValue{}, &models.Category{}))
	require.NoError(t, db.Select("*").Create(&models.Locale{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true}).Error)

	inactive := false
	slug := "inactive-category"
	category, err := NewService(db, nil).CreateCategory(context.Background(), apicontract.CategoryInput{Name: "Inactive Category", Slug: &slug, IsActive: &inactive})
	require.NoError(t, err)
	require.False(t, category.IsActive, "explicitly inactive category must persist as inactive")

	var reloaded models.Category
	require.NoError(t, db.First(&reloaded, category.ID).Error)
	require.False(t, reloaded.IsActive, "explicitly inactive category must be persisted as inactive in the database")
}

func TestCreateProductPersistsExplicitlyUnpublishedVariantDraft(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Locale{},
		&models.LocalizedEntityValue{},
		&models.Product{},
		&models.ProductVariant{},
		&models.ProductDraft{},
		&models.ProductVariantDraft{},
		&models.ProductRelatedDraft{},
		&models.ProductCategory{},
		&models.ProductCategoryDraft{},
		&models.ProductAttributeValueDraft{},
		&models.ProductOptionDraft{}, &models.ProductOption{}, &models.ProductOptionValue{}, &models.ProductVariantOptionValue{}, &models.ProductAttribute{}, &models.ProductAttributeValue{}, &models.SEOMetadata{},
		&models.ProductOptionValueDraft{},
		&models.ProductVariantOptionValueDraft{},
		&models.MediaReference{},
	))
	require.NoError(t, db.Select("*").Create(&models.Locale{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true}).Error)

	unpublished := false
	input := apicontract.ProductUpsertInput{
		Sku:  "CLPILLO-002",
		Name: "Colormatic Logo Pillow (Draft Variant)",
		Variants: []apicontract.ProductVariantInput{
			{Sku: "CLPILLO-002", Title: "Default", Price: 15, Stock: 10, IsPublished: &unpublished},
		},
	}
	product, err := NewService(db, nil).CreateProduct(context.Background(), input)
	require.NoError(t, err)

	require.False(t, product.IsPublished, "new products must remain drafts")
	var storedProduct models.Product
	require.NoError(t, db.First(&storedProduct, product.ID).Error)
	require.False(t, storedProduct.IsPublished)

	var draft models.ProductDraft
	require.NoError(t, db.Where("product_id = ?", product.ID).First(&draft).Error)
	var variantDraft models.ProductVariantDraft
	require.NoError(t, db.Where("product_draft_id = ?", draft.ID).First(&variantDraft).Error)
	require.False(t, variantDraft.IsPublished, "explicitly unpublished variant draft must persist as unpublished")
}

func TestPublishProductPersistsExplicitlyUnpublishedNewVariant(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Locale{},
		&models.LocalizedEntityValue{},
		&models.Product{},
		&models.ProductVariant{},
		&models.ProductDraft{},
		&models.ProductVariantDraft{},
		&models.ProductRelatedDraft{},
		&models.ProductCategory{},
		&models.ProductCategoryDraft{},
		&models.ProductAttributeValueDraft{},
		&models.ProductOptionDraft{}, &models.ProductOption{}, &models.ProductOptionValue{}, &models.ProductVariantOptionValue{}, &models.ProductAttribute{}, &models.ProductAttributeValue{}, &models.SEOMetadata{},
		&models.ProductOptionValueDraft{},
		&models.ProductVariantOptionValueDraft{},
		&models.MediaReference{},
	))
	require.NoError(t, db.Select("*").Create(&models.Locale{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true}).Error)
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX idx_product_variants_sku_unique ON product_variants (sku)").Error)

	product := models.Product{SKU: "CLPILLO-003", Name: "Colormatic Logo Pillow", Price: models.MoneyFromFloat(15), Stock: 100, IsPublished: true}
	require.NoError(t, db.Create(&product).Error)
	variant := models.ProductVariant{ProductID: product.ID, SKU: product.SKU, Title: product.Name, Price: product.Price, Stock: product.Stock, Position: 1, IsPublished: true}
	require.NoError(t, db.Create(&variant).Error)
	require.NoError(t, db.Model(&product).Update("default_variant_id", variant.ID).Error)

	draft := models.ProductDraft{ProductID: product.ID, SKU: product.SKU, DefaultVariantSKU: product.SKU, Name: product.Name, Price: product.Price, Stock: product.Stock, ImagesJSON: "[]"}
	require.NoError(t, db.Create(&draft).Error)
	require.NoError(t, db.Create(&models.ProductVariantDraft{ProductDraftID: draft.ID, SKU: variant.SKU, Title: variant.Title, Price: variant.Price, Stock: variant.Stock, Position: 1, IsPublished: true}).Error)
	// New, second variant introduced only in the draft, explicitly unpublished.
	newDraftVariant := models.ProductVariantDraft{ProductDraftID: draft.ID, SKU: "CLPILLO-003-NEW", Title: "New unpublished variant", Price: models.MoneyFromFloat(25), Stock: 5, Position: 2, IsPublished: false}
	require.NoError(t, db.Create(&newDraftVariant).Error)
	require.NoError(t, db.Model(&newDraftVariant).Update("is_published", false).Error)

	published, err := NewService(db, nil).PublishProduct(context.Background(), product.ID)
	require.NoError(t, err)
	require.Len(t, published.Variants, 2)

	var newVariant models.ProductVariant
	require.NoError(t, db.Where("product_id = ? AND sku = ?", product.ID, "CLPILLO-003-NEW").First(&newVariant).Error)
	require.False(t, newVariant.IsPublished, "explicitly unpublished new live variant must persist as unpublished")
}

func TestPublishProductUpdatesExistingVariantWithSameSKU(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Locale{},
		&models.LocalizedEntityValue{},
		&models.Product{},
		&models.ProductVariant{},
		&models.ProductDraft{},
		&models.ProductVariantDraft{},
		&models.ProductRelatedDraft{},
		&models.ProductCategory{},
		&models.ProductCategoryDraft{},
		&models.ProductAttributeValueDraft{},
		&models.ProductOptionDraft{}, &models.ProductOption{}, &models.ProductOptionValue{}, &models.ProductVariantOptionValue{}, &models.ProductAttribute{}, &models.ProductAttributeValue{}, &models.SEOMetadata{},
		&models.ProductOptionValueDraft{},
		&models.ProductVariantOptionValueDraft{},
		&models.MediaReference{},
	))
	require.NoError(t, db.Select("*").Create(&models.Locale{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true}).Error)
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
	var productLocalization models.LocalizedEntityValue
	require.NoError(t, db.Where("entity_type = ? AND entity_id = ? AND field = ?", "product", product.ID, "name").First(&productLocalization).Error)
	require.Equal(t, product.Name, productLocalization.Value)
	var variantLocalization models.LocalizedEntityValue
	require.NoError(t, db.Where("entity_type = ? AND entity_id = ? AND field = ?", "product_variant", variant.ID, "title").First(&variantLocalization).Error)
	require.Equal(t, "Updated Pillow", variantLocalization.Value)
}

func TestProductDraftLifecycleCanRepeatAfterPublish(t *testing.T) {
	for _, legacyDeletedDraft := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy_deleted_draft_%t", legacyDeletedDraft), func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
			require.NoError(t, err)
			require.NoError(t, db.AutoMigrate(&models.Locale{}, &models.LocalizedEntityValue{}, &models.Product{}, &models.ProductVariant{}, &models.ProductDraft{}, &models.ProductVariantDraft{}, &models.ProductRelatedDraft{}, &models.ProductCategory{}, &models.ProductCategoryDraft{}, &models.ProductAttributeValueDraft{}, &models.ProductOptionDraft{}, &models.ProductOption{}, &models.ProductOptionValue{}, &models.ProductVariantOptionValue{}, &models.ProductAttribute{}, &models.ProductAttributeValue{}, &models.SEOMetadata{},
				&models.ProductOptionValueDraft{},
				&models.ProductVariantOptionValueDraft{}, &models.MediaReference{}, &models.Brand{}, &models.Category{}, &models.ProductAttribute{}, &models.ProductAttributeValue{}, &models.SearchDocument{}, &models.SearchIndexState{}, &models.JobQueue{}))
			require.NoError(t, db.Create(&models.Locale{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true}).Error)
			runtime := jobs.NewRuntime(db, jobs.Config{})
			service := NewService(db, nil, runtime)
			index := searchservice.NewService(db, nil, runtime)
			ctx := context.Background()
			input := apicontract.ProductUpsertInput{Sku: "LIFECYCLE", Name: "Lifecycle product", Description: "Published description", Variants: []apicontract.ProductVariantInput{{Sku: "LIFECYCLE", Title: "Default", Price: 15, Stock: 10}}}
			product, err := service.CreateProduct(ctx, input)
			require.NoError(t, err)
			require.False(t, product.IsPublished)
			product, err = service.PublishProduct(ctx, product.ID)
			require.NoError(t, err)
			require.True(t, product.IsPublished)
			require.NoError(t, index.SyncProduct(ctx, product.ID))
			if legacyDeletedDraft {
				// Existing databases may retain soft-deleted drafts from older releases.
				old := models.ProductDraft{ProductID: product.ID, SKU: input.Sku, Name: input.Name, ImagesJSON: "[]"}
				require.NoError(t, db.Create(&old).Error)
				require.NoError(t, db.Delete(&old).Error)
			}
			product, err = service.UnpublishProduct(ctx, product.ID)
			require.NoError(t, err)
			require.False(t, product.IsPublished)
			require.NotNil(t, product.DraftUpdatedAt)
			require.NoError(t, index.SyncProduct(ctx, product.ID))
			var count int64
			require.NoError(t, db.Model(&models.SearchDocument{}).Where("entity_id = ? AND active = ?", product.ID, true).Count(&count).Error)
			require.Zero(t, count)
			product, err = service.PublishProduct(ctx, product.ID)
			require.NoError(t, err)
			require.True(t, product.IsPublished)
			input.Name = "Updated product"
			_, err = service.UpdateProduct(ctx, product.ID, input)
			require.NoError(t, err)
			// Replacing an existing draft must also release its unique key.
			_, err = service.UpdateProduct(ctx, product.ID, input)
			require.NoError(t, err)
			product, err = service.PublishProduct(ctx, product.ID)
			require.NoError(t, err)
			require.Equal(t, input.Name, product.Name)
			var localized models.LocalizedEntityValue
			require.NoError(t, db.Where("entity_type = ? AND entity_id = ? AND field = ?", "product", product.ID, "name").First(&localized).Error)
			require.Equal(t, input.Name, localized.Value)
			require.NoError(t, db.Model(&models.JobQueue{}).Where("job_type = ?", searchservice.JobTypeProductSync).Count(&count).Error)
			require.EqualValues(t, 4, count)
		})
	}
}
