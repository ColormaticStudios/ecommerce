package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"ecommerce/internal/jobs"
	"ecommerce/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RecordProductChangeTx persists convergence debt and its delivery intent in
// the transaction which changes catalog truth. A completed or failed job does
// not itself discharge this debt.
func RecordProductChangeTx(ctx context.Context, runtime *jobs.Runtime, tx *gorm.DB, productID uint, changed time.Time) (models.JobQueue, error) {
	if runtime == nil {
		return models.JobQueue{}, errors.New("search job runtime is required")
	}
	if tx == nil || productID == 0 {
		return models.JobQueue{}, errors.New("search transaction and product ID are required")
	}
	if changed.IsZero() {
		changed = time.Now().UTC()
	}
	if err := recordProductRevisionTx(ctx, tx, productID, changed); err != nil {
		return models.JobQueue{}, err
	}
	return enqueuePendingProduct(ctx, runtime, tx, productID)
}

func recordProductRevisionTx(ctx context.Context, tx *gorm.DB, productID uint, changed time.Time) error {
	revision := changed.UnixNano()
	// Source clocks are not convergence clocks. In particular, an existing
	// projection may have been written by the legacy timestamp-based indexer.
	// Allocate above every durable boundary even when the source clock goes back.
	floor := "CASE WHEN search_product_revisions.revision > search_product_revisions.indexed_revision THEN search_product_revisions.revision ELSE search_product_revisions.indexed_revision END"
	documentFloor := "COALESCE((SELECT version FROM search_documents WHERE entity_type = 'product' AND entity_id = excluded.product_id), 0)"
	floor = "CASE WHEN (" + floor + ") > " + documentFloor + " THEN (" + floor + ") ELSE " + documentFloor + " END"
	initialRevision := gorm.Expr("CASE WHEN ? > COALESCE((SELECT version FROM search_documents WHERE entity_type = ? AND entity_id = ?), 0) THEN ? ELSE COALESCE((SELECT version FROM search_documents WHERE entity_type = ? AND entity_id = ?), 0) + 1 END", revision, ProductEntityType, productID, revision, ProductEntityType, productID)
	values := map[string]any{"product_id": productID, "revision": initialRevision, "indexed_revision": int64(0), "changed_at": changed, "pending_since": changed}
	if err := tx.WithContext(ctx).Table("search_product_revisions").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "product_id"}}, DoUpdates: clause.Assignments(map[string]any{
		"revision":   gorm.Expr("CASE WHEN (" + floor + ") >= excluded.revision THEN (" + floor + ") + 1 ELSE excluded.revision END"),
		"changed_at": changed, "pending_since": gorm.Expr("COALESCE(search_product_revisions.pending_since, excluded.pending_since)"),
	})}).Create(values).Error; err != nil {
		return err
	}
	return nil
}

func enqueuePendingProduct(ctx context.Context, runtime *jobs.Runtime, tx *gorm.DB, productID uint) (models.JobQueue, error) {
	return runtime.EnqueueCoalescedTx(ctx, tx, jobs.EnqueueInput{JobType: JobTypeProductSync, Payload: ProductSyncPayload{Version: ProductSyncVersion, ProductID: productID}, IdempotencyKey: fmt.Sprintf("product:%d:pending", productID)})
}

func (s *Service) finishProductSync(ctx context.Context, tx *gorm.DB, job models.JobQueue) error {
	var payload ProductSyncPayload
	if err := json.Unmarshal([]byte(job.PayloadJSON), &payload); err != nil {
		return err
	}
	var pending int64
	if err := tx.WithContext(ctx).Model(&models.SearchProductRevision{}).Where("product_id = ? AND pending_since IS NOT NULL", payload.ProductID).Count(&pending).Error; err != nil {
		return err
	}
	if pending == 0 {
		query := tx.WithContext(ctx).Model(&models.SearchInventoryRevision{}).Where("product_id = ? AND pending_since IS NOT NULL", payload.ProductID)
		if payload.VariantID != 0 {
			query = query.Where("variant_id = ?", payload.VariantID)
		}
		if err := query.Count(&pending).Error; err != nil {
			return err
		}
		if pending == 0 {
			return nil
		}
	}
	var err error
	if payload.VariantID != 0 {
		_, err = enqueuePendingInventory(ctx, s.jobs, tx, payload.ProductID, payload.VariantID)
	} else {
		_, err = enqueuePendingProduct(ctx, s.jobs, tx, payload.ProductID)
	}
	return err
}

func projectionRevision(ctx context.Context, db *gorm.DB, id uint) (int64, error) {
	if err := repairProjectionRevisionFloors(db.WithContext(ctx), &id); err != nil {
		return 0, err
	}
	var state models.SearchProductRevision
	err := db.WithContext(ctx).First(&state, "product_id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		var document models.SearchDocument
		err = db.WithContext(ctx).Select("version").Where("entity_type = ? AND entity_id = ?", ProductEntityType, id).First(&document).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return document.Version, err
	}
	return state.Revision, err
}

// Repair timestamp-era acknowledgments before taking a source snapshot. A
// newer mutation still uses an atomic upsert, so this repair cannot lower its
// boundary or clear debt recorded by another writer.
func repairProjectionRevisionFloors(db *gorm.DB, id *uint) error {
	floor := "CASE WHEN indexed_revision > COALESCE((SELECT version FROM search_documents WHERE entity_type = 'product' AND entity_id = search_product_revisions.product_id), 0) THEN indexed_revision ELSE COALESCE((SELECT version FROM search_documents WHERE entity_type = 'product' AND entity_id = search_product_revisions.product_id), 0) END"
	query := db.Model(&models.SearchProductRevision{}).Where("revision < (" + floor + ") OR (pending_since IS NOT NULL AND indexed_revision >= revision)")
	if id != nil {
		query = query.Where("product_id = ?", *id)
	}
	return query.Updates(map[string]any{"revision": gorm.Expr("(" + floor + ") + 1"), "pending_since": gorm.Expr("COALESCE(pending_since, changed_at)")}).Error
}

func acknowledgeProjection(tx *gorm.DB, id uint, revision int64) error {
	return tx.Model(&models.SearchProductRevision{}).Where("product_id = ? AND indexed_revision < ? AND revision >= ?", id, revision, revision).
		Updates(map[string]any{"indexed_revision": revision, "pending_since": gorm.Expr("CASE WHEN revision <= ? THEN NULL ELSE pending_since END", revision)}).Error
}

// Inventory's transaction-level APIs inherit the explicitly composed producer
// from their database handle; registered API workers own execution.
func RecordInventoryChangeTx(ctx context.Context, tx *gorm.DB, variantID uint) error {
	runtime, err := jobs.RuntimeFromDB(tx)
	if err != nil {
		return err
	}
	var variant models.ProductVariant
	if err := tx.WithContext(ctx).Select("id, product_id").First(&variant, variantID).Error; err != nil {
		return err
	}
	changed := time.Now().UTC()
	values := map[string]any{"variant_id": variantID, "product_id": variant.ProductID, "revision": changed.UnixNano(), "changed_at": changed, "pending_since": changed}
	if err := tx.WithContext(ctx).Table("search_inventory_revisions").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "variant_id"}}, DoUpdates: clause.Assignments(map[string]any{
		"revision": gorm.Expr("CASE WHEN search_inventory_revisions.revision >= excluded.revision THEN search_inventory_revisions.revision + 1 ELSE excluded.revision END"), "changed_at": changed, "pending_since": gorm.Expr("COALESCE(search_inventory_revisions.pending_since, excluded.pending_since)"),
	})}).Create(values).Error; err != nil {
		return err
	}
	_, err = enqueuePendingInventory(ctx, runtime, tx, variant.ProductID, variantID)
	return err
}

func enqueuePendingInventory(ctx context.Context, runtime *jobs.Runtime, tx *gorm.DB, productID, variantID uint) (models.JobQueue, error) {
	return runtime.EnqueueCoalescedTx(ctx, tx, jobs.EnqueueInput{JobType: JobTypeProductSync, Payload: ProductSyncPayload{Version: ProductSyncVersion, ProductID: productID, VariantID: variantID}, IdempotencyKey: fmt.Sprintf("variant:%d:pending", variantID)})
}

func captureInventoryDebt(ctx context.Context, db *gorm.DB, productID *uint) ([]models.SearchInventoryRevision, error) {
	var captured []models.SearchInventoryRevision
	query := db.WithContext(ctx).Where("pending_since IS NOT NULL")
	if productID != nil {
		query = query.Where("product_id = ?", *productID)
	}
	if err := query.Order("product_id ASC, variant_id ASC").Find(&captured).Error; err != nil {
		return nil, err
	}
	if len(captured) == 0 {
		return captured, nil
	}
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var previous uint
		for _, debt := range captured {
			if debt.ProductID != previous {
				if err := recordProductRevisionTx(ctx, tx, debt.ProductID, debt.ChangedAt); err != nil {
					return err
				}
				previous = debt.ProductID
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return captured, nil
}

func acknowledgeInventoryDebt(ctx context.Context, db *gorm.DB, captured []models.SearchInventoryRevision) error {
	if len(captured) == 0 {
		return nil
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, debt := range captured {
			if err := tx.Model(&models.SearchInventoryRevision{}).Where("variant_id = ? AND revision = ?", debt.VariantID, debt.Revision).Update("pending_since", nil).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
