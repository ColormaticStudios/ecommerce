package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"ecommerce/internal/jobs"
	"ecommerce/internal/reliability"
	"ecommerce/models"
	"github.com/mattn/go-sqlite3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const ConversionRefreshVersion = 1

type ConversionRefreshPayload struct {
	Version int       `json:"version"`
	AsOf    time.Time `json:"as_of"`
}

func productMarginRate(product models.Product) float64 {
	minimum := 1.0
	found := false
	for _, variant := range product.Variants {
		if !variant.IsPublished || variant.DeletedAt.Valid {
			continue
		}
		found = true
		rate := 0.0
		if variant.UnitCost != nil && variant.Price > 0 {
			rate = math.Max(0, math.Min(1, (float64(variant.Price)-float64(*variant.UnitCost))/float64(variant.Price)))
		}
		minimum = math.Min(minimum, rate)
	}
	if !found {
		return 0
	}
	return minimum
}
func conversionRate(row models.SearchConversionSignal) float64 {
	if row.Impressions30Days < 1 || row.Conversions30Days < 1 {
		return 0
	}
	return float64(row.Conversions30Days) / (float64(row.Impressions30Days) + 20)
}
func (s *Service) EnqueueConversionRefresh(ctx context.Context) (models.JobQueue, error) {
	if s == nil || s.jobs == nil {
		return models.JobQueue{}, errors.New("search job runtime is required")
	}
	asOf := s.now().UTC().Truncate(24 * time.Hour)
	return s.jobs.Enqueue(ctx, jobs.EnqueueInput{JobType: JobTypeConversionRefresh, Payload: ConversionRefreshPayload{Version: ConversionRefreshVersion, AsOf: asOf}, IdempotencyKey: "conversion:" + asOf.Format("2006-01-02")})
}
func (s *Service) handleConversionRefresh(ctx context.Context, raw json.RawMessage) error {
	var payload ConversionRefreshPayload
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Version != ConversionRefreshVersion || payload.AsOf.IsZero() || !payload.AsOf.Equal(payload.AsOf.UTC().Truncate(24*time.Hour)) || payload.AsOf.After(s.now()) {
		return reliability.Classify(errors.New("invalid conversion refresh payload"), reliability.ClassTerminal)
	}
	if err := s.RefreshConversionSignals(ctx, payload.AsOf); err != nil {
		class := reliability.ClassTerminal
		var sqliteError sqlite3.Error
		var sqlState interface{ SQLState() string }
		if operationalSearchError(err) {
			class = reliability.ClassRetryable
		}
		if errors.As(err, &sqliteError) && (sqliteError.Code == sqlite3.ErrBusy || sqliteError.Code == sqlite3.ErrLocked) {
			class = reliability.ClassRetryable
		}
		if errors.As(err, &sqlState) {
			switch sqlState.SQLState() {
			case "40001", "40P01", "08000", "08003", "08006", "57P01", "53300":
				class = reliability.ClassRetryable
			}
		}
		return reliability.Classify(err, class)
	}
	return nil
}
func (s *Service) RefreshConversionSignals(ctx context.Context, asOf time.Time) error {
	if s == nil || s.db == nil {
		return errors.New("search database is required")
	}
	if asOf.IsZero() || !asOf.Equal(asOf.UTC().Truncate(24*time.Hour)) || asOf.After(s.now()) {
		return errors.New("conversion snapshot must be a nonfuture daily UTC boundary")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return s.refreshConversionSignalsTx(ctx, tx, asOf) })
}

// RecomputeConversionSignalsTx is called after consent-owned rows are removed,
// in the same revocation transaction. It shares the snapshot lock so a racing
// daily refresh cannot restore revoked contributions after revocation commits.
func (s *Service) RecomputeConversionSignalsTx(ctx context.Context, tx *gorm.DB) error {
	if err := lockConversionProjection(ctx, tx); err != nil {
		return err
	}
	var state models.SearchIndexState
	if err := tx.WithContext(ctx).First(&state, "name = ?", ProductIndexName).Error; err != nil {
		return err
	}
	if state.LastConversionRefreshAt == nil {
		return nil
	}
	const savepoint = "conversion_consent_projection"
	if err := tx.SavePoint(savepoint).Error; err != nil {
		return err
	}
	if err := s.refreshConversionSignalsTx(ctx, tx, *state.LastConversionRefreshAt); err != nil {
		if rollbackErr := tx.RollbackTo(savepoint).Error; rollbackErr != nil {
			return rollbackErr
		}
		// Consent withdrawal must not depend on another session's valid analytics.
		// Remove all derived influence until a valid refresh can recompute it.
		if resetErr := tx.WithContext(ctx).Model(&models.SearchConversionSignal{}).Where("1 = 1").UpdateColumns(map[string]any{"impressions30_days": 0, "conversions30_days": 0, "updated_at": s.now()}).Error; resetErr != nil {
			return resetErr
		}
		if resetErr := tx.WithContext(ctx).Model(&models.SearchIndexState{}).Where("name = ?", ProductIndexName).UpdateColumn("generation", gorm.Expr("generation + 1")).Error; resetErr != nil {
			return resetErr
		}
		slog.WarnContext(ctx, "Conversion ranking snapshot cleared after consent revocation", "event", "search_conversion_revocation_reset", "error_type", fmt.Sprintf("%T", err))
	}
	return tx.Exec("RELEASE SAVEPOINT " + savepoint).Error

}
func lockConversionProjection(ctx context.Context, tx *gorm.DB) error {
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&models.SearchIndexState{Name: ProductIndexName}).Error; err != nil {
		return err
	}
	var state models.SearchIndexState
	return tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&state, "name = ?", ProductIndexName).Error
}

type conversionExposure struct {
	impression string
	product    uint
}

func (s *Service) refreshConversionSignalsTx(ctx context.Context, tx *gorm.DB, asOf time.Time) error {
	if err := lockConversionProjection(ctx, tx); err != nil {
		return err
	}
	var state models.SearchIndexState
	if err := tx.WithContext(ctx).First(&state, "name = ?", ProductIndexName).Error; err != nil {
		return err
	}
	if state.LastConversionRefreshAt != nil && state.LastConversionRefreshAt.After(asOf) {
		return nil
	}
	since, until := asOf.Add(-37*24*time.Hour), asOf.Add(-7*24*time.Hour)
	exposures := map[conversionExposure]time.Time{}
	totals := map[uint]models.SearchConversionSignal{}
	var cursor uint
	for {
		var rows []models.SearchQueryEvent
		if err := tx.WithContext(ctx).Where("id > ? AND created_at >= ? AND created_at < ? AND normalized_query <> '' AND session_hash <> '' AND impression_id <> '' AND NOT EXISTS (SELECT 1 FROM search_revoked_sessions rs WHERE rs.session_hash = search_query_events.session_hash)", cursor, since, until).Order("id ASC").Limit(500).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if err := ctx.Err(); err != nil {
				return err
			}
			cursor = row.ID
			var ids []uint
			if err := json.Unmarshal([]byte(row.ProductsJSON), &ids); err != nil {
				return fmt.Errorf("invalid conversion impression product snapshot %d: %w", row.ID, err)
			}
			for _, id := range ids {
				if id == 0 {
					continue
				}
				key := conversionExposure{row.ImpressionID, id}
				if _, seen := exposures[key]; seen {
					continue
				}
				exposures[key] = row.CreatedAt
				total := totals[id]
				total.ProductID = id
				total.Impressions30Days++
				totals[id] = total
			}
		}
	}
	converted := map[conversionExposure]bool{}
	cursor = 0
	for {
		var rows []struct {
			ID           uint
			ProductID    uint
			ImpressionID string
			PaidAt       time.Time
			ClickedAt    time.Time
		}
		if err := tx.WithContext(ctx).Table("search_order_attributions a").Select("a.id,a.product_id,a.impression_id,a.paid_at,e.clicked_at").Joins("JOIN orders o ON o.id = a.order_id AND o.deleted_at IS NULL AND o.status IN ?", []string{models.StatusPaid, models.StatusShipped, models.StatusDelivered}).Joins("JOIN search_click_events e ON e.event_id = a.click_id AND e.impression_id = a.impression_id AND e.product_id = a.product_id").Joins("JOIN search_query_events q ON q.impression_id = a.impression_id AND q.session_hash = e.session_hash AND e.query_event_id = q.id").Where("a.id > ? AND a.paid_at >= ? AND a.paid_at < ?", cursor, since, asOf).Order("a.id ASC").Limit(500).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if err := ctx.Err(); err != nil {
				return err
			}
			cursor = row.ID
			key := conversionExposure{row.ImpressionID, row.ProductID}
			impressed, eligible := exposures[key]
			if !eligible || converted[key] || row.ClickedAt.Before(impressed) || row.ClickedAt.After(impressed.Add(AttributionWindow)) || row.PaidAt.Before(row.ClickedAt) || row.PaidAt.After(impressed.Add(AttributionWindow)) || row.PaidAt.After(row.ClickedAt.Add(AttributionWindow)) {
				continue
			}
			converted[key] = true
			total := totals[row.ProductID]
			total.Conversions30Days++
			totals[row.ProductID] = total
		}
	}
	var prior []models.SearchConversionSignal
	if err := tx.WithContext(ctx).Find(&prior).Error; err != nil {
		return err
	}
	for _, row := range prior {
		if _, exists := totals[row.ProductID]; !exists {
			totals[row.ProductID] = models.SearchConversionSignal{ProductID: row.ProductID}
		}
	}
	now := s.now().UTC()
	rows := make([]models.SearchConversionSignal, 0, len(totals))
	for _, row := range totals {
		row.AsOf = asOf
		row.UpdatedAt = now
		rows = append(rows, row)
	}
	for start := 0; start < len(rows); start += 250 {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "product_id"}}, DoUpdates: clause.AssignmentColumns([]string{"impressions30_days", "conversions30_days", "as_of", "updated_at"}), Where: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "search_conversion_signals.as_of <= excluded.as_of"}}}}).Create(rows[start:min(start+250, len(rows))]).Error; err != nil {
			return err
		}
	}
	return tx.WithContext(ctx).Model(&models.SearchIndexState{}).Where("name = ?", ProductIndexName).UpdateColumns(map[string]any{"generation": gorm.Expr("generation + 1"), "last_conversion_refresh_at": asOf}).Error
}
