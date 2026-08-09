package commands

import (
	"context"

	"ecommerce/internal/httpapi"
)

func withCatalogEndpoints[T any](ctx context.Context, run func(context.Context, *httpapi.CatalogEndpoints) (T, error)) (T, error) {
	if err := requireLocalMode("catalog commands without remote API support"); err != nil {
		var zero T
		return zero, err
	}
	db := getDB()
	defer closeDB(db)
	endpoints, err := httpapi.NewCatalogEndpoints(db, nil)
	if err != nil {
		var zero T
		return zero, err
	}
	return run(ctx, endpoints)
}
