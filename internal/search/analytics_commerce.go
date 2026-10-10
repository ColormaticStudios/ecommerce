package search

import (
	"context"
	"fmt"
	"log/slog"

	"gorm.io/gorm"
)

// BestEffortAnalyticsTx isolates an optional analytics write from its owning
// commerce transaction. PostgreSQL requires rollback to a savepoint after a
// failed statement before any subsequent commerce write can succeed.
func BestEffortAnalyticsTx(ctx context.Context, tx *gorm.DB, write func() error) {
	if tx == nil || write == nil {
		return
	}
	const savepoint = "search_analytics_commerce"
	if err := tx.SavePoint(savepoint).Error; err != nil {
		slog.WarnContext(ctx, "Search analytics write skipped", "event", "search_analytics_commerce_skipped", "error_type", fmt.Sprintf("%T", err))
		return
	}
	if err := write(); err != nil {
		if rollbackErr := tx.RollbackTo(savepoint).Error; rollbackErr != nil {
			slog.ErrorContext(ctx, "Search analytics savepoint rollback failed", "event", "search_analytics_rollback_failed", "error_type", fmt.Sprintf("%T", rollbackErr))
		} else {
			slog.WarnContext(ctx, "Search analytics write skipped", "event", "search_analytics_commerce_skipped", "error_type", fmt.Sprintf("%T", err))
		}
	}
	if err := tx.Exec("RELEASE SAVEPOINT " + savepoint).Error; err != nil {
		slog.ErrorContext(ctx, "Search analytics savepoint release failed", "event", "search_analytics_release_failed", "error_type", fmt.Sprintf("%T", err))
	}
}
