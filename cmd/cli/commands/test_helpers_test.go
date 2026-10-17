package commands

import (
	"ecommerce/internal/jobs"
	"ecommerce/internal/searchprojection"
	catalogmodels "ecommerce/models"
	"fmt"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T, models ...any) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if len(models) > 0 {
		if err := db.AutoMigrate(append(models, &searchprojection.Rank{}, &searchprojection.Category{}, &searchprojection.Attribute{}, &searchprojection.Term{}, &catalogmodels.SearchConfigurationAudit{}, &catalogmodels.SearchCatalogFanoutDebt{}, &catalogmodels.SearchInventoryRevision{}, &catalogmodels.SearchProductRevision{}, &catalogmodels.SearchDocument{})...); err != nil {
			t.Fatalf("migrate sqlite db: %v", err)
		}
	}

	sqlDB, err := db.DB()
	if err == nil {
		t.Cleanup(func() {
			_ = sqlDB.Close()
		})
	}

	return jobs.WithRuntime(db, jobs.NewRuntime(db, jobs.Config{}))
}
