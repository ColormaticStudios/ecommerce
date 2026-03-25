package tax

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTaxTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.OrderTaxLine{}, &models.TaxExport{}))
	return db
}

func TestExportOrderTaxesUsesAllExporterForMixedProviders(t *testing.T) {
	db := newTaxTestDB(t)
	now := time.Now().UTC()

	lines := []models.OrderTaxLine{
		{
			OrderID:            1,
			SnapshotID:         11,
			LineType:           models.TaxLineTypeItem,
			TaxProviderID:      "dummy-us-tax",
			Jurisdiction:       "TX",
			TaxName:            "Sales Tax",
			TaxableAmount:      models.MoneyFromFloat(10),
			TaxAmount:          models.MoneyFromFloat(0.63),
			TaxRateBasisPoints: 625,
			FinalizedAt:        now,
		},
		{
			OrderID:            2,
			SnapshotID:         12,
			LineType:           models.TaxLineTypeItem,
			TaxProviderID:      "dummy-vat",
			Jurisdiction:       "DE",
			TaxName:            "VAT",
			TaxableAmount:      models.MoneyFromFloat(20),
			TaxAmount:          models.MoneyFromFloat(4),
			TaxRateBasisPoints: 2000,
			FinalizedAt:        now.Add(time.Minute),
		},
	}
	require.NoError(t, db.Create(&lines).Error)

	record, body, err := ExportOrderTaxes(context.Background(), db, NewDefaultProviderRegistry(), ExportInput{
		Format: "csv",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = body.Close()
	})

	assert.Equal(t, "all", record.Provider)
	assert.Equal(t, 2, record.RowCount)

	contents, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Contains(t, string(contents), "1")
	assert.Contains(t, string(contents), "2")

	var filters map[string]any
	require.NoError(t, json.Unmarshal([]byte(record.FiltersJSON), &filters))
	assert.Equal(t, "all", filters["provider"])
}
