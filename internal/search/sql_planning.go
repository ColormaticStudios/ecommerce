package search

import (
	"context"
	"database/sql"
	"fmt"

	"gorm.io/gorm"
)

func (b *databaseBackend) searchProductsSQL(ctx context.Context, filters Filters) (Result, error) {
	if b.db.Dialector.Name() != "postgres" {
		return b.searchProductsSQLQuery(ctx, filters)
	}
	// Word frequencies vary from one posting to the whole catalog. A generic
	// prepared plan can choose thousands of indexed lookups for a broad term.
	// Keep parameter-sensitive planning local to this read-only transaction,
	// rather than changing pooled connections or interpolating shopper input.
	var result Result
	var queryErr error
	err := b.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL plan_cache_mode = force_custom_plan").Error; err != nil {
			return err
		}
		// This transaction owns its storage handle. The shared backend and its
		// database pointer stay immutable across concurrent requests.
		scoped := databaseBackend{db: tx, now: b.now, configuration: b.configuration}
		result, queryErr = scoped.searchProductsSQLQuery(ctx, filters)
		return queryErr
	}, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		if queryErr != nil {
			return Result{}, err
		}
		// Beginning, configuring, or committing the transaction failed. Never
		// return a computed result whose transaction did not finish successfully.
		return Result{}, fmt.Errorf("%w: %w", ErrIndexUnavailable, err)
	}
	return result, nil
}
