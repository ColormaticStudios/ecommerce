package search

import (
	"context"
	"testing"
	"time"

	"ecommerce/internal/jobs"
	"ecommerce/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func searchTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Product{}, &models.ProductVariant{}, &models.Brand{}, &models.Category{}, &models.ProductCategory{},
		&models.ProductAttribute{}, &models.ProductAttributeValue{}, &models.SearchDocument{}, &models.SearchIndexState{},
		&models.JobQueue{},
	))
	return db
}

func TestReindexSearchAndSuggestions(t *testing.T) {
	db := searchTestDB(t)
	brand := models.Brand{Name: "Northstar", Slug: "northstar", IsActive: true}
	require.NoError(t, db.Create(&brand).Error)
	category := models.Category{Name: "Trail", Slug: "trail", IsActive: true, Path: "trail"}
	require.NoError(t, db.Create(&category).Error)
	product := models.Product{SKU: "BOOT-1", Name: "Alpine Boot", Description: "Waterproof hiking boot", Price: models.MoneyFromFloat(120), IsPublished: true, BrandID: &brand.ID}
	require.NoError(t, db.Create(&product).Error)
	require.NoError(t, db.Model(&product).Association("Categories").Append(&category))
	variant := models.ProductVariant{ProductID: product.ID, SKU: "BOOT-1-BLK", Title: "Black", Price: models.MoneyFromFloat(125), Stock: 4, Position: 1, IsPublished: true}
	require.NoError(t, db.Create(&variant).Error)

	service := NewService(db, nil, nil)
	require.NoError(t, service.Reindex(context.Background()))

	result, err := service.Search(context.Background(), Filters{Query: "waterproof black", BrandSlug: "northstar", CategorySlugs: []string{"trail"}, Page: 1, Limit: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)
	require.Len(t, result.Products, 1)
	require.Equal(t, "Alpine Boot", result.Products[0].Name)
	require.Len(t, result.Products[0].Variants, 1)
	require.Equal(t, "waterproof black", result.NormalizedQuery)

	suggestions, err := service.Suggest(context.Background(), "alp", 5)
	require.NoError(t, err)
	require.Equal(t, []string{"Alpine Boot"}, suggestions.Suggestions)
	require.Empty(t, suggestions.Corrections)
	require.Empty(t, suggestions.Popular)
	brandSuggestions, err := service.Suggest(context.Background(), "north", 5)
	require.NoError(t, err)
	require.Equal(t, []string{"Northstar"}, brandSuggestions.Suggestions)

	freshness, err := service.Freshness(context.Background())
	require.NoError(t, err)
	require.Equal(t, "healthy", freshness.Status)
	require.Equal(t, int64(1), freshness.DocumentCount)
	require.NotNil(t, freshness.LastFullReindexAt)
}

func TestNewerTombstoneRejectsStaleDocument(t *testing.T) {
	db := searchTestDB(t)
	backend := NewDatabaseBackend(db)
	now := time.Now().UTC()
	require.NoError(t, backend.Delete(context.Background(), ProductEntityType, 42, 20))
	require.NoError(t, backend.Upsert(context.Background(), models.SearchDocument{
		EntityType: ProductEntityType, EntityID: 42, PayloadJSON: `{}`, SearchableText: "old",
		NormalizedName: "old", CategoryTokens: "|", AttributeTokens: "|", Active: true,
		Version: 10, SourceCreatedAt: now, SourceUpdatedAt: now, IndexedAt: now,
	}))

	result, err := backend.Search(context.Background(), Filters{Page: 1, Limit: 10})
	require.NoError(t, err)
	require.Zero(t, result.Total)
	var stored models.SearchDocument
	require.NoError(t, db.Where("entity_type = ? AND entity_id = ?", ProductEntityType, 42).First(&stored).Error)
	require.False(t, stored.Active)
	require.Equal(t, int64(20), stored.Version)
}

func TestNormalizeQuery(t *testing.T) {
	require.Equal(t, "trail running shoe", NormalizeQuery("  Trail—Running_shoe! "))
}

func TestEnqueueProductSyncTxIsAtomicAndIdempotent(t *testing.T) {
	db := searchTestDB(t)
	runtime := jobs.NewRuntime(db, jobs.Config{})
	revision := time.Unix(100, 0).UTC()

	require.Error(t, db.Transaction(func(tx *gorm.DB) error {
		_, err := EnqueueProductSyncTx(context.Background(), runtime, tx, 7, revision)
		require.NoError(t, err)
		return gorm.ErrInvalidTransaction
	}))
	var count int64
	require.NoError(t, db.Model(&models.JobQueue{}).Count(&count).Error)
	require.Zero(t, count)

	first, err := EnqueueProductSyncTx(context.Background(), runtime, db, 7, revision)
	require.NoError(t, err)
	second, err := EnqueueProductSyncTx(context.Background(), runtime, db, 7, revision)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
}
