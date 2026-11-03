package httpapi

import (
	"context"
	"fmt"
	"testing"

	"ecommerce/internal/media"
	"ecommerce/internal/requestctx"
	localizationservice "ecommerce/internal/services/localization"
	"ecommerce/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPublicProductPageBatchesLocalizationAndPublishedMedia(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Product{}, &models.ProductVariant{}, &models.Brand{}, &models.Category{}, &models.Locale{}, &models.LocaleMarketDefault{}, &models.LocalizedEntityValue{}, &models.MediaObject{}, &models.MediaReference{}))
	english := models.Locale{Code: "en-US", Name: "English", IsDefault: true, IsEnabled: true}
	require.NoError(t, db.Create(&english).Error)
	french := models.Locale{Code: "fr", Name: "French", IsEnabled: true, FallbackLocaleID: &english.ID}
	require.NoError(t, db.Create(&french).Error)
	brand := models.Brand{Name: "Canonical brand", Slug: "brand"}
	category := models.Category{Name: "Canonical category", Slug: "category"}
	require.NoError(t, db.Create(&brand).Error)
	require.NoError(t, db.Create(&category).Error)
	copy := []models.LocalizedEntityValue{{EntityType: "brand", EntityID: brand.ID, LocaleID: french.ID, Field: "name", Value: "Marque"}, {EntityType: "category", EntityID: category.ID, LocaleID: french.ID, Field: "name", Value: "Catégorie"}}
	products := []models.Product{}
	cost := models.Money(123)
	for i := 1; i <= 20; i++ {
		product := models.Product{SKU: fmt.Sprint(i), Name: "Canonical product", BrandID: &brand.ID, Price: 1000, IsPublished: true}
		require.NoError(t, db.Create(&product).Error)
		published := models.ProductVariant{ProductID: product.ID, SKU: fmt.Sprintf("%d-live", i), Title: "Canonical title", IsPublished: true, UnitCost: &cost, Price: 1000}
		draft := models.ProductVariant{ProductID: product.ID, SKU: fmt.Sprintf("%d-draft", i), Title: "Private draft", IsPublished: true, UnitCost: &cost, Price: 900}
		require.NoError(t, db.Create(&published).Error)
		require.NoError(t, db.Create(&draft).Error)
		require.NoError(t, db.Model(&draft).Update("is_published", false).Error)
		draft.IsPublished = false
		product.Brand, product.Categories, product.Variants = &brand, []models.Category{category}, []models.ProductVariant{published, draft}
		product.Images = []string{"legacy.jpg"}
		products = append(products, product)
		copy = append(copy, models.LocalizedEntityValue{EntityType: "product", EntityID: product.ID, LocaleID: french.ID, Field: "name", Value: fmt.Sprintf("Produit %d", i)}, models.LocalizedEntityValue{EntityType: "product", EntityID: product.ID, LocaleID: english.ID, Field: "description", Value: "English description"}, models.LocalizedEntityValue{EntityType: "product_variant", EntityID: published.ID, LocaleID: french.ID, Field: "title", Value: "Taille"})
	}
	products[0].Related = []models.Product{products[19]}
	require.NoError(t, db.Create(&copy).Error)
	require.NoError(t, db.Create(&[]models.MediaObject{{ID: "live", OriginalPath: "live.jpg", Status: media.StatusReady}, {ID: "draft", OriginalPath: "draft.jpg", Status: media.StatusReady}, {ID: "processing", OriginalPath: "processing.jpg", Status: media.StatusProcessing}, {ID: "logo", OriginalPath: "logo.jpg", Status: media.StatusReady}}).Error)
	refs := []models.MediaReference{{MediaID: "live", OwnerType: media.OwnerTypeProduct, OwnerID: products[0].ID, Role: media.RoleProductImage}, {MediaID: "draft", OwnerType: media.OwnerTypeProduct, OwnerID: products[0].ID, Role: media.RoleProductDraftImage}, {MediaID: "processing", OwnerType: media.OwnerTypeProduct, OwnerID: products[0].ID, Role: media.RoleProductImage}, {MediaID: "logo", OwnerType: media.OwnerTypeBrand, OwnerID: brand.ID, Role: media.RoleBrandLogo}}
	require.NoError(t, db.Create(&refs).Error)
	e := &CatalogEndpoints{db: db, localization: localizationservice.NewService(db), media: media.NewService(db, t.TempDir(), "/media", nil, nil)}
	ctx := requestctx.WithLocaleResolution(context.Background(), requestctx.LocaleResolution{ResolvedLocale: "fr"})
	queries := 0
	count := func(*gorm.DB) { queries++ }
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("public_page_query_count", count))
	require.NoError(t, db.Callback().Row().Before("gorm:row").Register("public_page_row_count", count))
	first, err := e.productsToContract(ctx, products[:1], false)
	require.NoError(t, err)
	firstQueries := queries
	all, err := e.productsToContract(ctx, products, false)
	require.NoError(t, err)
	require.Equal(t, firstQueries, queries-firstQueries, "query count is independent of page and variant count")
	require.LessOrEqual(t, firstQueries, 15)
	require.Equal(t, first[0], all[0])
	require.Equal(t, "Produit 20", all[0].RelatedProducts[0].Name)
	for i, product := range all {
		require.Equal(t, fmt.Sprintf("Produit %d", i+1), product.Name)
		require.Equal(t, "English description", product.Description)
		require.True(t, product.Localization.UsedFallback)
		require.Equal(t, "en-US", product.Localization.SourceLocales["description"])
		require.Equal(t, "Marque", product.Brand.Name)
		require.Equal(t, "Catégorie", product.Categories[0].Name)
		require.Len(t, product.Variants, 1)
		require.Equal(t, "Taille", product.Variants[0].Title)
		require.Nil(t, product.Variants[0].UnitCost)
	}
	require.Equal(t, []string{"/media/live.jpg"}, all[0].Images)
	require.Equal(t, "/media/logo.jpg", *all[0].Brand.LogoUrl)
	require.Equal(t, []string{"legacy.jpg"}, all[1].Images)
	queryCheckpoint := queries
	empty, err := e.productsToContract(ctx, nil, false)
	require.NoError(t, err)
	require.Empty(t, empty)
	require.Equal(t, queryCheckpoint, queries)
	// Canonical copy cannot become an unreviewed fallback when translations
	// disappear, and stale references retain existence validation.
	require.NoError(t, db.Where("entity_type = ? AND entity_id = ? AND field = ?", "product", products[0].ID, "name").Delete(&models.LocalizedEntityValue{}).Error)
	missingCopy, err := e.productsToContract(ctx, products[:1], false)
	require.NoError(t, err)
	require.Empty(t, missingCopy[0].Name)
	require.NoError(t, db.Delete(&brand).Error)
	_, err = e.productsToContract(ctx, products[:1], false)
	require.ErrorIs(t, err, localizationservice.ErrLocalizedEntityNotFound)
}
