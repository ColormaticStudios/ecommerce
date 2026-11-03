package search

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"ecommerce/internal/jobs"
	"ecommerce/internal/reliability"
	"ecommerce/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const JobTypeCatalogFanout = "search.catalog_fanout"

type CatalogFanoutPayload struct {
	Version   int       `json:"version"`
	Kind      string    `json:"kind"`
	IDs       []uint    `json:"ids"`
	ChangedAt time.Time `json:"changed_at"`
}

// EnqueueCatalogFanoutTx persists one intent at the source mutation boundary.
// Product convergence debt is allocated in bounded worker transactions, not in
// the administrative save request. The job itself represents unexpanded debt.
func EnqueueCatalogFanoutTx(ctx context.Context, runtime *jobs.Runtime, tx *gorm.DB, kind string, ids []uint) error {
	if runtime == nil {
		return errors.New("search job runtime is required")
	}
	if kind != "brand" && kind != "category" && kind != "attribute" || len(ids) == 0 {
		return errors.New("invalid catalog fanout")
	}
	changed := time.Now().UTC()
	job, err := runtime.EnqueueTx(ctx, tx, jobs.EnqueueInput{JobType: JobTypeCatalogFanout, Payload: CatalogFanoutPayload{Version: 1, Kind: kind, IDs: ids, ChangedAt: changed}, IdempotencyKey: "fanout:" + uuid.NewString()})
	if err != nil {
		return err
	}
	return tx.WithContext(ctx).Create(&models.SearchCatalogFanoutDebt{JobID: job.ID, PendingSince: changed}).Error
}

func (s *Service) finishCatalogFanout(ctx context.Context, tx *gorm.DB, job models.JobQueue) error {
	return tx.WithContext(ctx).Model(&models.SearchCatalogFanoutDebt{}).Where("job_id = ? AND covered_at IS NULL", job.ID).Update("covered_at", s.now()).Error
}

func coverFanoutDebt(ctx context.Context, db *gorm.DB, ids []string, covered time.Time) error {
	for start := 0; start < len(ids); start += 250 {
		end := min(start+250, len(ids))
		if err := db.WithContext(ctx).Model(&models.SearchCatalogFanoutDebt{}).Where("job_id IN ? AND covered_at IS NULL", ids[start:end]).Update("covered_at", covered).Error; err != nil {
			return err
		}
	}
	return nil
}

func allocateRebuildFanoutRevisions(ctx context.Context, db *gorm.DB, changed time.Time) error {
	var cursor uint
	for {
		var ids []uint
		if err := db.WithContext(ctx).Unscoped().Model(&models.Product{}).Where("id > ?", cursor).Order("id ASC").Limit(250).Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			for _, id := range ids {
				if err := recordProductRevisionTx(ctx, tx, id, changed); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
		cursor = ids[len(ids)-1]
	}
}

func (s *Service) handleCatalogFanout(ctx context.Context, raw json.RawMessage) error {
	var payload CatalogFanoutPayload
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Version != 1 || len(payload.IDs) == 0 || payload.ChangedAt.IsZero() || (payload.Kind != "brand" && payload.Kind != "category" && payload.Kind != "attribute") {
		return reliability.Classify(errors.New("invalid catalog fanout payload"), reliability.ClassTerminal)
	}
	for {
		done := false
		if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			job, err := jobs.LockExecutionTx(ctx, tx)
			if err != nil {
				return err
			}
			if job.JobType != JobTypeCatalogFanout {
				return jobs.ErrInvalidJob
			}
			var debt models.SearchCatalogFanoutDebt
			if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&debt, "job_id = ?", job.ID).Error; err != nil {
				return err
			}
			var ids []uint
			query := tx.WithContext(ctx).Model(&models.Product{}).Where("products.id > ?", debt.LastProductID)
			switch payload.Kind {
			case "brand":
				query = query.Where("brand_id IN ?", payload.IDs)
			case "category":
				query = query.Where("id IN (SELECT product_id FROM product_categories WHERE category_id IN ?)", payload.IDs)
			case "attribute":
				query = query.Where("id IN (SELECT product_id FROM product_attribute_values WHERE product_attribute_id IN ?)", payload.IDs)
			}
			if err := query.Order("products.id ASC").Limit(250).Pluck("products.id", &ids).Error; err != nil {
				return err
			}
			if len(ids) == 0 {
				done = true
				return nil
			}
			for _, id := range ids {
				if _, err := RecordProductChangeTx(ctx, s.jobs, tx, id, payload.ChangedAt); err != nil {
					return err
				}
			}
			if _, err := jobs.LockExecutionTx(ctx, tx); err != nil {
				return err
			}
			return tx.WithContext(ctx).Model(&debt).Update("last_product_id", ids[len(ids)-1]).Error
		}); err != nil {
			return reliability.Classify(err, reliability.ClassRetryable)
		}
		if done {
			return nil
		}
	}
}
