package search

import (
	"context"
	"errors"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const IncidentRetention = 365 * 24 * time.Hour

var ErrIncidentQueryInvalid = errors.New("invalid search incident query")

type IndexIncident struct {
	models.SearchIndexIncident
	Status          string
	DurationSeconds int64
}
type IncidentList struct {
	Items       []IndexIncident
	Page, Limit int
	Total       int64
	TotalPages  int
}

// ObserveFreshness owns incident persistence independently of the durable job
// workers. Successful observations do not update index freshness timestamps.
func (s *Service) ObserveFreshness(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("search database is required")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// This also provides a stable row to lock on a new installation. SQLite's
		// write serialization and PostgreSQL's row lock prevent split episodes.
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.SearchIndexState{Name: ProductIndexName}).Error; err != nil {
			return err
		}
		var state models.SearchIndexState
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&state, "name = ?", ProductIndexName).Error; err != nil {
			return err
		}
		now := s.now().UTC()
		var observation models.SearchFreshnessObservation
		err := tx.First(&observation, "index_name = ?", ProductIndexName).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if !observation.LastObservedAt.IsZero() && now.Before(observation.LastObservedAt) {
			return nil
		}
		observation.IndexName = ProductIndexName
		observation.LastObservedAt = now
		backend := databaseBackend{db: tx, now: func() time.Time { return now }}
		freshness, err := backend.Freshness(ctx)
		if err != nil {
			return err
		}
		reason := "index_lag"
		openedAt := now.Add(-(freshness.Lag - StaleFreshnessTarget))
		stale := freshness.Status == "stale"
		if state.LastFullReindexAt == nil {
			reason = "missing_baseline"
			if observation.MissingBaselineSince == nil {
				at := now
				observation.MissingBaselineSince = &at
			}
			openedAt = observation.MissingBaselineSince.Add(StaleFreshnessTarget)
			stale = !now.Before(openedAt)
		} else {
			observation.MissingBaselineSince = nil
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "index_name"}}, DoUpdates: clause.AssignmentColumns([]string{"missing_baseline_since", "last_observed_at"})}).Create(&observation).Error; err != nil {
			return err
		}
		var incident models.SearchIndexIncident
		err = tx.Where("index_name = ? AND recovered_at IS NULL", ProductIndexName).First(&incident).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		exists := err == nil
		lagSeconds := int64(freshness.Lag / time.Second)
		if lagSeconds < 0 {
			lagSeconds = 0
		}
		if exists {
			updates := map[string]any{"last_observed_at": now, "document_count": freshness.DocumentCount}
			if lagSeconds > incident.MaxLagSeconds {
				updates["max_lag_seconds"] = lagSeconds
			}
			if freshness.PendingJobs > incident.MaxPendingJobs {
				updates["max_pending_jobs"] = freshness.PendingJobs
			}
			// Missing-baseline grace never resolves an already open episode. A real
			// healthy/degraded observation is required before closing it.
			if freshness.Status != "stale" {
				updates["recovered_at"] = now
			}
			if err := tx.Model(&incident).Updates(updates).Error; err != nil {
				return err
			}
		} else if stale {
			incident = models.SearchIndexIncident{IndexName: ProductIndexName, Reason: reason, OpenedAt: openedAt, DetectedAt: now, LastObservedAt: now, MaxLagSeconds: lagSeconds, MaxPendingJobs: freshness.PendingJobs, DocumentCount: freshness.DocumentCount}
			if err := tx.Create(&incident).Error; err != nil {
				return err
			}
		}
		// Retain active incidents even if an outage lasts more than a year.
		return tx.Where("recovered_at IS NOT NULL AND recovered_at < ?", now.Add(-IncidentRetention)).Delete(&models.SearchIndexIncident{}).Error
	})
}

// ListIncidents is read-only; merely visiting operations never creates history.
func (s *Service) ListIncidents(ctx context.Context, page, limit int, status string) (IncidentList, error) {
	result := IncidentList{Items: []IndexIncident{}, Page: page, Limit: limit}
	if s == nil || s.db == nil {
		return result, errors.New("search database is required")
	}
	if page < 1 || limit < 1 || limit > 100 || (status != "" && status != "open" && status != "resolved") {
		return result, ErrIncidentQueryInvalid
	}
	query := s.db.WithContext(ctx).Model(&models.SearchIndexIncident{}).Where("index_name = ?", ProductIndexName)
	if status == "open" {
		query = query.Where("recovered_at IS NULL")
	} else if status == "resolved" {
		query = query.Where("recovered_at IS NOT NULL")
	}
	if err := query.Session(&gorm.Session{}).Count(&result.Total).Error; err != nil {
		return result, err
	}
	result.TotalPages = int((result.Total + int64(limit) - 1) / int64(limit))
	if int64(page-1) > int64(^uint(0)>>1)/int64(limit) {
		return result, ErrIncidentQueryInvalid
	}
	var rows []models.SearchIndexIncident
	if err := query.Order("opened_at DESC, id DESC").Limit(limit).Offset((page - 1) * limit).Find(&rows).Error; err != nil {
		return result, err
	}
	now := s.now().UTC()
	for _, row := range rows {
		item := IndexIncident{SearchIndexIncident: row, Status: "open"}
		until := now
		if row.RecoveredAt != nil {
			item.Status = "resolved"
			until = *row.RecoveredAt
		}
		item.DurationSeconds = int64(until.Sub(row.OpenedAt) / time.Second)
		if item.DurationSeconds < 0 {
			item.DurationSeconds = 0
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}
