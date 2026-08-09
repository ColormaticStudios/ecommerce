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
	require.NoError(t, db.AutoMigrate(&models.OrderCheckoutSnapshot{}, &models.OrderTaxLine{}, &models.TaxExport{}))
	return db
}

func TestPersistTaxFinalizationIsIdempotent(t *testing.T) {
	db := newTaxTestDB(t)
	snapshot := models.OrderCheckoutSnapshot{TaxProviderID: "dummy-us-tax", Currency: "USD"}
	require.NoError(t, db.Create(&snapshot).Error)
	order := models.Order{}
	order.ID = 12
	input := FinalizeInput{Order: order, Snapshot: snapshot}
	result := TaxFinalized{Provider: "dummy-us-tax", ProviderReference: "tax-ref", Currency: "USD", TotalTax: 125, Lines: []TaxLine{{LineType: models.TaxLineTypeItem, Quantity: 1, TaxableAmount: 2000, TaxAmount: 125, TaxRateBasisPoints: 625}}}

	first, err := PersistTaxFinalization(db, input, result, time.Now().UTC())
	require.NoError(t, err)
	second, err := PersistTaxFinalization(db, input, result, time.Now().UTC().Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, first.TotalTax, second.TotalTax)
	var count int64
	require.NoError(t, db.Model(&models.OrderTaxLine{}).Where("order_id = ? AND snapshot_id = ?", input.Order.ID, snapshot.ID).Count(&count).Error)
	require.EqualValues(t, 1, count)
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
