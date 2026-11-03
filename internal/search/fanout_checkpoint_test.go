package search

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"ecommerce/internal/jobs"
	"ecommerce/internal/reliability"
	"ecommerce/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFanoutCheckpointRollsBackBatchOnLeaseExpiry(t *testing.T) {
	db := searchTestDB(t)
	var clock atomic.Int64
	clock.Store(time.Now().UTC().UnixNano())
	runtime := jobs.NewRuntime(db, jobs.Config{Now: func() time.Time { return time.Unix(0, clock.Load()).UTC() }, Jitter: func(time.Duration) time.Duration { return 0 }})
	service := NewService(db, nil, runtime)
	require.NoError(t, service.RegisterJobHandlers())
	brand := models.Brand{Name: "Checkpoint", Slug: "checkpoint", IsActive: true}
	require.NoError(t, db.Create(&brand).Error)
	products := []models.Product{{SKU: "CHECKPOINT-A", Name: "A", BrandID: &brand.ID}, {SKU: "CHECKPOINT-B", Name: "B", BrandID: &brand.ID}}
	require.NoError(t, db.Create(&products).Error)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return EnqueueCatalogFanoutTx(context.Background(), runtime, tx, "brand", []uint{brand.ID})
	}))
	var job models.JobQueue
	require.NoError(t, db.First(&job).Error)
	creates := 0
	require.NoError(t, db.Callback().Create().After("gorm:create").Register("expire_fanout_batch", func(tx *gorm.DB) {
		if tx.Statement.Table == "search_product_revisions" {
			creates++
			if creates == 2 {
				clock.Add(int64(time.Hour))
			}
		}
	}))
	err := runtime.Execute(context.Background(), job.ID, "expiring-worker")
	require.ErrorIs(t, err, jobs.ErrLeaseLost)
	require.NoError(t, db.Callback().Create().Remove("expire_fanout_batch"))
	var debt models.SearchCatalogFanoutDebt
	require.NoError(t, db.First(&debt, "job_id = ?", job.ID).Error)
	require.Zero(t, debt.LastProductID)
	require.Nil(t, debt.CoveredAt)
	for _, model := range []any{&models.SearchProductRevision{}, &models.JobQueue{}} {
		var count int64
		query := db.Model(model)
		if _, ok := model.(*models.JobQueue); ok {
			query = query.Where("job_type = ?", JobTypeProductSync)
		}
		require.NoError(t, query.Count(&count).Error)
		require.Zero(t, count, "expired work and cursor must roll back together")
	}
	require.NoError(t, runtime.Execute(context.Background(), job.ID, "recovered-worker"))
	require.NoError(t, db.First(&debt, "job_id = ?", job.ID).Error)
	require.Equal(t, products[1].ID, debt.LastProductID)
	require.NotNil(t, debt.CoveredAt)
}

func TestFanoutRejectsMalformedPayloadAndMissingExecution(t *testing.T) {
	service := NewService(searchTestDB(t), nil, nil)
	for _, payload := range []string{`{`, `{"version":2}`, `{"version":1,"kind":"brand","ids":[1]}`, `{"version":1,"kind":"unknown","ids":[1],"changed_at":"2026-10-17T00:00:00Z"}`} {
		err := service.handleCatalogFanout(context.Background(), json.RawMessage(payload))
		class, ok := reliability.ErrorClassOf(err)
		require.True(t, ok)
		require.Equal(t, reliability.ClassTerminal, class)
	}
	valid := json.RawMessage(`{"version":1,"kind":"brand","ids":[1],"changed_at":"2026-10-17T00:00:00Z"}`)
	require.ErrorIs(t, service.handleCatalogFanout(context.Background(), valid), jobs.ErrMissingExecution)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, service.handleCatalogFanout(ctx, valid), context.Canceled)
}
