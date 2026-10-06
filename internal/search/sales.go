package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ecommerce/internal/jobs"
	"ecommerce/internal/reliability"
	"ecommerce/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const JobTypeSalesRefresh = "search.sales_refresh"

type SalesRefreshPayload struct {
	Version int       `json:"version"`
	AsOf    time.Time `json:"as_of"`
}

// Daily UTC windows produce identical payloads/keys on every application replica.
func (s *Service) EnqueueSalesRefresh(ctx context.Context) (models.JobQueue, error) {
	if s == nil || s.jobs == nil {
		return models.JobQueue{}, errors.New("search job runtime is required")
	}
	asOf := s.now().UTC().Truncate(24 * time.Hour)
	return s.jobs.Enqueue(ctx, jobs.EnqueueInput{JobType: JobTypeSalesRefresh, Payload: SalesRefreshPayload{Version: 1, AsOf: asOf}, IdempotencyKey: "sales:" + asOf.Format("2006-01-02")})
}

func (s *Service) handleSalesRefresh(ctx context.Context, raw json.RawMessage) error {
	var payload SalesRefreshPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return reliability.Classify(err, reliability.ClassTerminal)
	}
	if payload.Version != 1 || payload.AsOf.IsZero() || !payload.AsOf.Equal(payload.AsOf.UTC().Truncate(24*time.Hour)) || payload.AsOf.After(s.now()) {
		return reliability.Classify(errors.New("invalid search sales refresh payload"), reliability.ClassTerminal)
	}
	if err := s.RefreshSalesSignals(ctx, payload.AsOf); err != nil {
		class := reliability.ClassTerminal
		message := strings.ToLower(err.Error())
		for _, transient := range []string{"database is locked", "database table is locked", "deadlock detected", "serialization failure", "connection reset", "connection refused", "broken pipe"} {
			if strings.Contains(message, transient) {
				class = reliability.ClassRetryable
				break
			}
		}
		return reliability.Classify(err, class)
	}
	return nil
}

// RefreshSalesSignals projects a complete snapshot in one transaction. Ranking
// never scans orders; missing sales signals intentionally score zero.
func (s *Service) RefreshSalesSignals(ctx context.Context, asOf time.Time) error {
	if s == nil || s.db == nil {
		return errors.New("search database is required")
	}
	if asOf.IsZero() || !asOf.Equal(asOf.UTC().Truncate(24*time.Hour)) {
		return errors.New("sales snapshot must be a daily UTC boundary")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sales []struct {
			ProductID uint
			Units     int64
		}
		err := tx.Table("order_items AS items").Select("variants.product_id AS product_id, SUM(items.quantity) AS units").
			Joins("JOIN orders ON orders.id = items.order_id").Joins("JOIN product_variants AS variants ON variants.id = items.product_variant_id").
			Where("orders.deleted_at IS NULL AND items.deleted_at IS NULL AND items.quantity > 0 AND orders.status IN ? AND orders.created_at >= ? AND orders.created_at < ?", []string{models.StatusPaid, models.StatusShipped, models.StatusDelivered}, asOf.AddDate(0, 0, -30), asOf).
			Group("variants.product_id").Scan(&sales).Error
		if err != nil {
			return err
		}
		totals := map[uint]int64{}
		for _, row := range sales {
			totals[row.ProductID] = row.Units
		}
		// Include previous signals so canceled/refunded/expired sales become zero.
		var ids []uint
		if err := tx.Raw("SELECT id FROM products WHERE deleted_at IS NULL UNION SELECT product_id FROM search_sales_signals").Scan(&ids).Error; err != nil {
			return err
		}
		now := s.now().UTC()
		for start := 0; start < len(ids); start += 250 {
			if err := ctx.Err(); err != nil {
				return err
			}
			end := min(start+250, len(ids))
			rows := make([]models.SearchSalesSignal, 0, end-start)
			for _, id := range ids[start:end] {
				rows = append(rows, models.SearchSalesSignal{ProductID: id, Units30Days: totals[id], AsOf: asOf, UpdatedAt: now})
			}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "product_id"}}, DoUpdates: clause.AssignmentColumns([]string{"units30_days", "as_of", "updated_at"}), Where: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "search_sales_signals.as_of <= excluded.as_of"}}}}).Create(&rows).Error; err != nil {
				return fmt.Errorf("project search sales signals: %w", err)
			}
		}
		return nil
	})
}
