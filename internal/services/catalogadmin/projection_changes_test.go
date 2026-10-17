package catalogadmin

import (
	"context"
	"ecommerce/internal/apicontract"
	"ecommerce/internal/jobs"
	searchservice "ecommerce/internal/search"
	"ecommerce/models"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestCategoryAncestorChangesQueueDescendantProductsAtomically(t *testing.T) {
	for _, failure := range []string{"success", "nil_runtime", "queue_failure"} {
		t.Run(failure, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
			require.NoError(t, err)
			require.NoError(t, db.AutoMigrate(&models.Product{}, &models.Category{}, &models.ProductCategory{}, &models.Locale{}, &models.LocalizedEntityValue{}, &models.SearchCatalogFanoutDebt{}, &models.JobQueue{}, &models.JobAttempt{}, &models.JobDeadLetter{}, &models.SearchCatalogFanoutDebt{}, &models.SearchInventoryRevision{}, &models.SearchProductRevision{}, &models.SearchDocument{}))
			require.NoError(t, db.Create(&models.Locale{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true}).Error)
			service := NewService(db, nil, jobs.NewRuntime(db, jobs.Config{}))
			ctx := context.Background()
			createCategory := func(name string, parent *int) models.Category {
				value, err := service.CreateCategory(ctx, apicontract.CategoryInput{Name: name, ParentId: parent})
				require.NoError(t, err)
				return value
			}
			root := createCategory("Old root", nil)
			rootID := int(root.ID)
			child := createCategory("Child", &rootID)
			childID := int(child.ID)
			grandchild := createCategory("Grandchild", &childID)
			anchor := createCategory("Anchor", nil)
			products := make([]models.Product, 3)
			for i := range products {
				products[i] = models.Product{SKU: fmt.Sprintf("CATEGORY-%d", i), Name: "Categorized product", IsPublished: true}
				require.NoError(t, db.Create(&products[i]).Error)
			}
			require.NoError(t, db.Create(&[]models.ProductCategory{
				{ProductID: products[0].ID, CategoryID: child.ID},
				{ProductID: products[1].ID, CategoryID: root.ID},
				{ProductID: products[1].ID, CategoryID: grandchild.ID},
				{ProductID: products[2].ID, CategoryID: anchor.ID},
			}).Error)
			if failure == "nil_runtime" {
				service.jobs = nil
			} else if failure == "queue_failure" {
				require.NoError(t, db.Migrator().DropTable(&models.JobQueue{}))
			}
			anchorID, slug := int(anchor.ID), "new-root"
			_, err = service.UpdateCategory(ctx, root.ID, apicontract.CategoryInput{Name: "New root", Slug: &slug, ParentId: &anchorID})
			if failure != "success" {
				require.Error(t, err)
				for _, original := range []models.Category{root, child, grandchild} {
					var actual models.Category
					require.NoError(t, db.First(&actual, original.ID).Error)
					require.Equal(t, original.Path, actual.Path)
					require.Equal(t, original.Depth, actual.Depth)
					require.Equal(t, original.Name, actual.Name)
				}
				var translation models.LocalizedEntityValue
				require.NoError(t, db.Where("entity_type = ? AND entity_id = ? AND field = ?", "category", root.ID, "name").First(&translation).Error)
				require.Equal(t, "Old root", translation.Value)
				var revisions int64
				require.NoError(t, db.Model(&models.SearchProductRevision{}).Count(&revisions).Error)
				require.Zero(t, revisions)
				return
			}
			require.NoError(t, err)
			for i, original := range []models.Category{root, child, grandchild} {
				var actual models.Category
				require.NoError(t, db.First(&actual, original.ID).Error)
				require.Equal(t, i+1, actual.Depth)
				paths := []string{"/anchor/new-root", "/anchor/new-root/child", "/anchor/new-root/child/grandchild"}
				require.Equal(t, paths[i], actual.Path)
			}
			var fanout models.JobQueue
			require.NoError(t, db.Where("job_type = ?", searchservice.JobTypeCatalogFanout).First(&fanout).Error)
			var before int64
			require.NoError(t, db.Model(&models.SearchProductRevision{}).Count(&before).Error)
			require.Zero(t, before, "admin save persists one fanout, not per-product writes")
			projector := searchservice.NewService(db, nil, service.jobs)
			require.NoError(t, projector.RegisterJobHandlers())
			require.NoError(t, service.jobs.Execute(ctx, fanout.ID, "fanout-test"))
			var queued []models.JobQueue
			require.NoError(t, db.Where("job_type = ?", "search.product_sync").Find(&queued).Error)
			require.Len(t, queued, 2, "a product in multiple affected categories is queued once")
			queuedIDs := []uint{}
			for _, job := range queued {
				var payload struct {
					ProductID uint `json:"product_id"`
				}
				require.NoError(t, json.Unmarshal([]byte(job.PayloadJSON), &payload))
				queuedIDs = append(queuedIDs, payload.ProductID)
			}
			require.ElementsMatch(t, []uint{products[0].ID, products[1].ID}, queuedIDs)
			var revisions []models.SearchProductRevision
			require.NoError(t, db.Find(&revisions).Error)
			require.Len(t, revisions, 2)
			for _, revision := range revisions {
				require.NotNil(t, revision.PendingSince)
			}
		})
	}
}

func TestRelatedCatalogChangesQueueAffectedProductsAndRollback(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Product{}, &models.Brand{}, &models.Category{}, &models.ProductCategory{}, &models.ProductAttribute{}, &models.ProductAttributeValue{}, &models.Locale{}, &models.LocalizedEntityValue{}, &models.MediaReference{}, &models.SearchCatalogFanoutDebt{}, &models.JobQueue{}, &models.JobAttempt{}, &models.JobDeadLetter{}, &models.SearchCatalogFanoutDebt{}, &models.SearchInventoryRevision{}, &models.SearchProductRevision{}, &models.SearchDocument{}))
	require.NoError(t, db.Create(&models.Locale{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true}).Error)
	brand := models.Brand{Name: "Original brand", Slug: "original-brand", IsActive: true}
	require.NoError(t, db.Create(&brand).Error)
	category := models.Category{Name: "Original category", Slug: "original-category", Path: "original-category", IsActive: true}
	require.NoError(t, db.Create(&category).Error)
	attribute := models.ProductAttribute{Key: "Color", Slug: "color", Type: "text", Filterable: true}
	require.NoError(t, db.Create(&attribute).Error)
	product := models.Product{SKU: "RELATED", Name: "Affected", BrandID: &brand.ID, IsPublished: true}
	require.NoError(t, db.Create(&product).Error)
	unchanged := product.UpdatedAt
	require.NoError(t, db.Create(&models.ProductCategory{ProductID: product.ID, CategoryID: category.ID}).Error)
	text := "red"
	require.NoError(t, db.Create(&models.ProductAttributeValue{ProductID: product.ID, ProductAttributeID: attribute.ID, TextValue: &text}).Error)
	service := NewService(db, nil, jobs.NewRuntime(db, jobs.Config{}))
	ctx := context.Background()
	active := true
	_, err = service.UpdateBrand(ctx, brand.ID, apicontract.BrandInput{Name: "Updated brand", IsActive: &active})
	require.NoError(t, err)
	_, err = service.UpdateCategory(ctx, category.ID, apicontract.CategoryInput{Name: "Updated category", IsActive: &active})
	require.NoError(t, err)
	filterable := false
	slug := "color"
	_, err = service.UpdateAttribute(ctx, attribute.ID, apicontract.ProductAttributeDefinitionInput{Key: "Shade", Slug: &slug, Type: "text", Filterable: &filterable}, nil)
	require.NoError(t, err)
	var fanouts []models.JobQueue
	require.NoError(t, db.Where("job_type = ?", searchservice.JobTypeCatalogFanout).Find(&fanouts).Error)
	require.Len(t, fanouts, 3)
	projector := searchservice.NewService(db, nil, service.jobs)
	require.NoError(t, projector.RegisterJobHandlers())
	for _, fanout := range fanouts {
		require.NoError(t, service.jobs.Execute(ctx, fanout.ID, "fanout-test"))
	}

	var queued []models.JobQueue
	require.NoError(t, db.Where("job_type = ?", "search.product_sync").Find(&queued).Error)
	require.Len(t, queued, 1, "all related changes coalesce into one active product job")
	require.NoError(t, db.First(&product, product.ID).Error)
	require.WithinDuration(t, unchanged, product.UpdatedAt, time.Millisecond, "related changes need their own projection revision")
	var revision models.SearchProductRevision
	require.NoError(t, db.First(&revision, "product_id = ?", product.ID).Error)
	require.NotNil(t, revision.PendingSince)
	require.NoError(t, db.Migrator().DropTable(&models.JobQueue{}))
	_, err = service.UpdateBrand(ctx, brand.ID, apicontract.BrandInput{Name: "Should roll back", IsActive: &active})
	require.Error(t, err)
	require.NoError(t, db.First(&brand, brand.ID).Error)
	require.Equal(t, "Updated brand", brand.Name)
	var localization models.LocalizedEntityValue
	require.NoError(t, db.Where("entity_type = ? AND entity_id = ? AND field = ?", "brand", brand.ID, "name").First(&localization).Error)
	require.Equal(t, "Updated brand", localization.Value)
}

func TestAttributeHeadingSynchronizationPreservesOtherLocalesAndRollsBack(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProductAttribute{}, &models.ProductAttributeValue{}, &models.Product{}, &models.Locale{}, &models.LocalizedEntityValue{}, &models.SearchCatalogFanoutDebt{}, &models.JobQueue{}, &models.JobAttempt{}, &models.JobDeadLetter{}, &models.SearchCatalogFanoutDebt{}, &models.SearchInventoryRevision{}, &models.SearchProductRevision{}, &models.SearchDocument{}))
	english := models.Locale{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true}
	french := models.Locale{Code: "fr", Name: "French", IsEnabled: true}
	require.NoError(t, db.Create(&english).Error)
	require.NoError(t, db.Create(&french).Error)
	service := NewService(db, nil, jobs.NewRuntime(db, jobs.Config{}))
	actor := uint(42)
	input := apicontract.ProductAttributeDefinitionInput{Key: "Color", Type: "text"}
	attribute, err := service.CreateAttribute(context.Background(), input, &actor)
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.LocalizedEntityValue{EntityType: "product_attribute", EntityID: attribute.ID, LocaleID: french.ID, Field: "key", Value: "Couleur"}).Error)
	input.Key = "Shade"
	_, err = service.UpdateAttribute(context.Background(), attribute.ID, input, &actor)
	require.NoError(t, err)
	var values []models.LocalizedEntityValue
	require.NoError(t, db.Where("entity_type=? AND entity_id=?", "product_attribute", attribute.ID).Order("locale_id").Find(&values).Error)
	require.Len(t, values, 2)
	require.Equal(t, "Shade", values[0].Value)
	require.Equal(t, &actor, values[0].UpdatedBy)
	require.Equal(t, "Couleur", values[1].Value)
	require.NoError(t, db.Migrator().DropTable(&models.LocalizedEntityValue{}))
	input.Key = "Should roll back"
	_, err = service.UpdateAttribute(context.Background(), attribute.ID, input, &actor)
	require.Error(t, err)
	require.NoError(t, db.First(&attribute, attribute.ID).Error)
	require.Equal(t, "Shade", attribute.Key)
}
