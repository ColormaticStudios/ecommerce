package search

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"ecommerce/internal/jobs"
	"ecommerce/internal/reliability"
	"ecommerce/models"
	"github.com/stretchr/testify/require"
)

func TestSalesRefreshWindowStatusesResetsAndStaleJobs(t *testing.T) {
	ctx := context.Background()
	db := searchTestDB(t)
	// Only the historical order columns consumed by this projection are needed.
	require.NoError(t, db.Exec(`CREATE TABLE orders (id INTEGER PRIMARY KEY, status TEXT, created_at DATETIME, deleted_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE order_items (id INTEGER PRIMARY KEY, order_id INTEGER, product_variant_id INTEGER, quantity INTEGER, deleted_at DATETIME)`).Error)
	now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	for _, id := range []uint{1, 2} {
		require.NoError(t, db.Create(&models.Product{BaseModel: models.BaseModel{ID: id}, SKU: fmt.Sprintf("product-%d", id), Name: "Product"}).Error)
		require.NoError(t, db.Create(&models.ProductVariant{BaseModel: models.BaseModel{ID: id}, ProductID: id, SKU: fmt.Sprintf("variant-%d", id)}).Error)
	}
	// Historical deleted variants still contribute to their parent product.
	require.NoError(t, db.Model(&models.ProductVariant{}).Where("id = ?", 1).Update("deleted_at", now).Error)
	cases := []struct {
		status                    string
		age                       int
		quantity                  int
		deletedOrder, deletedItem bool
	}{
		{models.StatusPaid, 30, 2, false, false}, {models.StatusShipped, 1, 3, false, false}, {models.StatusDelivered, 29, 4, false, false},
		{models.StatusPaid, 31, 50, false, false}, {models.StatusPaid, 0, 50, false, false},
		{models.StatusPending, 1, 50, false, false}, {models.StatusRefunded, 1, 50, false, false}, {models.StatusCancelled, 1, 50, false, false}, {models.StatusFailed, 1, 50, false, false},
		{models.StatusPaid, 1, -1, false, false}, {models.StatusPaid, 1, 50, true, false}, {models.StatusPaid, 1, 50, false, true},
	}
	for i, c := range cases {
		var orderDeleted, itemDeleted any
		if c.deletedOrder {
			orderDeleted = now
		}
		if c.deletedItem {
			itemDeleted = now
		}
		require.NoError(t, db.Exec(`INSERT INTO orders VALUES (?, ?, ?, ?)`, i+1, c.status, now.AddDate(0, 0, -c.age), orderDeleted).Error)
		require.NoError(t, db.Exec(`INSERT INTO order_items VALUES (?, ?, 1, ?, ?)`, i+1, i+1, c.quantity, itemDeleted).Error)
	}
	service := NewService(db, nil, nil)
	service.now = func() time.Time { return now }
	require.NoError(t, service.RefreshSalesSignals(ctx, now))
	var signal models.SearchSalesSignal
	require.NoError(t, db.First(&signal, "product_id = ?", 1).Error)
	require.Equal(t, int64(9), signal.Units30Days)
	signal = models.SearchSalesSignal{}
	require.NoError(t, db.First(&signal, "product_id = ?", 2).Error)
	require.Zero(t, signal.Units30Days)
	// Refunds and windows aging out reset old values, including deleted products.
	require.NoError(t, db.Exec(`UPDATE orders SET status = ? WHERE id IN (1, 2, 3, 5)`, models.StatusRefunded).Error)
	require.NoError(t, db.Model(&models.Product{}).Where("id = ?", 1).Update("deleted_at", now).Error)
	next := now.AddDate(0, 0, 1)
	require.NoError(t, service.RefreshSalesSignals(ctx, next))
	signal = models.SearchSalesSignal{}
	require.NoError(t, db.First(&signal, "product_id = ?", 1).Error)
	require.Zero(t, signal.Units30Days)
	require.True(t, next.Equal(signal.AsOf))
	require.NoError(t, db.Exec(`UPDATE orders SET status = ? WHERE id = 1`, models.StatusPaid).Error)
	require.NoError(t, service.RefreshSalesSignals(ctx, now))
	signal = models.SearchSalesSignal{}
	require.NoError(t, db.First(&signal, "product_id = ?", 1).Error)
	require.Zero(t, signal.Units30Days)
	require.True(t, next.Equal(signal.AsOf))
	require.Error(t, service.RefreshSalesSignals(ctx, now.Add(time.Hour)))
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	require.ErrorIs(t, service.RefreshSalesSignals(canceled, next), context.Canceled)
}

func TestSalesRefreshIsAtomicAndDailyEnqueueIsDurable(t *testing.T) {
	ctx := context.Background()
	db := searchTestDB(t)
	now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.Exec(`CREATE TABLE orders (id INTEGER PRIMARY KEY, status TEXT, created_at DATETIME, deleted_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE order_items (id INTEGER PRIMARY KEY, order_id INTEGER, product_variant_id INTEGER, quantity INTEGER, deleted_at DATETIME)`).Error)
	for id := uint(1); id <= 251; id++ {
		require.NoError(t, db.Create(&models.SearchSalesSignal{ProductID: id, Units30Days: 7, AsOf: now.AddDate(0, 0, -1), UpdatedAt: now}).Error)
	}
	require.NoError(t, db.Exec(`CREATE TRIGGER fail_sales_refresh BEFORE UPDATE ON search_sales_signals WHEN NEW.product_id = 251 BEGIN SELECT RAISE(ABORT, 'test failure'); END`).Error)
	runtime := jobs.NewRuntime(db, jobs.Config{})
	service := NewService(db, nil, runtime)
	service.now = func() time.Time { return now.Add(time.Hour) }
	require.Error(t, service.RefreshSalesSignals(ctx, now))
	var row models.SearchSalesSignal
	require.NoError(t, db.First(&row, "product_id = ?", 1).Error)
	require.Equal(t, int64(7), row.Units30Days)
	require.True(t, now.AddDate(0, 0, -1).Equal(row.AsOf))
	first, err := service.EnqueueSalesRefresh(ctx)
	require.NoError(t, err)
	second, err := service.EnqueueSalesRefresh(ctx)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	service.now = func() time.Time { return now.AddDate(0, 0, 1) }
	third, err := service.EnqueueSalesRefresh(ctx)
	require.NoError(t, err)
	require.NotEqual(t, first.ID, third.ID)
	for _, payload := range []SalesRefreshPayload{{Version: 2, AsOf: now}, {Version: 1, AsOf: now.Add(time.Hour)}, {Version: 1, AsOf: now.AddDate(0, 0, 3)}} {
		raw, err := json.Marshal(payload)
		require.NoError(t, err)
		err = service.handleSalesRefresh(ctx, raw)
		require.Error(t, err)
		class, ok := reliability.ErrorClassOf(err)
		require.True(t, ok)
		require.Equal(t, reliability.ClassTerminal, class)
	}
	require.NoError(t, db.Exec(`DROP TRIGGER fail_sales_refresh`).Error)
	raw, _ := json.Marshal(SalesRefreshPayload{Version: 1, AsOf: now})
	require.NoError(t, service.handleSalesRefresh(ctx, raw))
	require.NoError(t, db.First(&row, "product_id = ?", 1).Error)
	require.Zero(t, row.Units30Days)
	require.NoError(t, service.RegisterJobHandlers())
}
