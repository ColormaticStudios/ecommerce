package dbcontext

import (
	"context"

	"gorm.io/gorm"
)

type contextKey struct{}

func WithDB(ctx context.Context, db *gorm.DB) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if db == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, db)
}

func GetDB(ctx context.Context) *gorm.DB {
	if ctx == nil {
		return nil
	}
	db, _ := ctx.Value(contextKey{}).(*gorm.DB)
	return db
}

func OrBackground(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}
