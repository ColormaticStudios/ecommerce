package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"ecommerce/models"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

const cachedDocumentLimit = 20000
const cachedPlanLimit = 16
const cachedDocumentTTL = 10 * time.Second

type documentCacheEntry struct {
	documents     []indexedProduct
	used, created time.Time
}
type documentCache struct {
	mu         sync.Mutex
	generation uint64
	entries    map[string]documentCacheEntry
	flight     singleflight.Group
}

func planCacheKey(plan *queryPlan) string {
	if plan == nil {
		return "*"
	}
	var key strings.Builder
	for _, group := range plan.groups {
		key.WriteString("(")
		for _, value := range group.alternatives {
			fmt.Fprintf(&key, "%d:%s", len(value), value)
		}
		key.WriteString(")")
	}
	return key.String()
}

// Configurations are not cached: every request still loads its live synonyms,
// typo/ranking profiles and merchandising rules. Only immutable decoded index
// candidates are shared, with a durable projection generation checked first.
func (b *databaseBackend) loadIndexedProducts(ctx context.Context, plan *queryPlan) ([]indexedProduct, error) {
	var state models.SearchIndexState
	if err := b.db.WithContext(ctx).Select("generation").First(&state, "name = ?", ProductIndexName).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return b.loadIndexedProductsUncached(ctx, plan)
		}
		return nil, fmt.Errorf("%w: %w", ErrIndexUnavailable, err)
	}
	c := &b.cache
	key := planCacheKey(plan)
	now := time.Now()
	c.mu.Lock()
	if state.Generation < c.generation {
		c.mu.Unlock()
		return b.loadIndexedProductsUncached(ctx, plan)
	}
	if c.generation != state.Generation {
		c.generation = state.Generation
		c.entries = map[string]documentCacheEntry{}
	}
	entry, hit := c.entries[key]
	if hit && now.Sub(entry.created) < cachedDocumentTTL {
		entry.used = now
		c.entries[key] = entry
		c.mu.Unlock()
		return append([]indexedProduct(nil), entry.documents...), nil
	}
	c.mu.Unlock()
	pending := c.flight.DoChan(fmt.Sprintf("%d:%s", state.Generation, key), func() (any, error) {
		// A concurrent fill might already have finished between lookup and joining.
		c.mu.Lock()
		entry, hit := c.entries[key]
		if c.generation == state.Generation && hit && time.Since(entry.created) < cachedDocumentTTL {
			c.mu.Unlock()
			return entry.documents, nil
		}
		c.mu.Unlock()
		documents, err := b.loadIndexedProductsUncached(ctx, plan)
		if err != nil {
			return nil, err
		}
		for i := range documents {
			documents[i].cacheGeneration = state.Generation
		}
		if len(documents) > cachedDocumentLimit {
			return documents, nil
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.generation != state.Generation {
			return documents, nil
		}
		if c.entries == nil {
			c.entries = map[string]documentCacheEntry{}
		}
		for {
			count := len(documents)
			for _, entry := range c.entries {
				count += len(entry.documents)
			}
			if count <= cachedDocumentLimit && len(c.entries) < cachedPlanLimit {
				break
			}
			oldestKey := ""
			var oldest time.Time
			for key, entry := range c.entries {
				if oldest.IsZero() || entry.used.Before(oldest) {
					oldestKey, oldest = key, entry.used
				}
			}
			delete(c.entries, oldestKey)
		}
		at := time.Now()
		c.entries[key] = documentCacheEntry{documents: documents, created: at, used: at}
		return documents, nil
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-pending:
		if result.Err != nil {
			if ctx.Err() == nil && (errors.Is(result.Err, context.Canceled) || errors.Is(result.Err, context.DeadlineExceeded)) {
				return b.loadIndexedProductsUncached(ctx, plan)
			}
			return nil, result.Err
		}
		return append([]indexedProduct(nil), result.Val.([]indexedProduct)...), nil
	}
}
func cloneIndexedProduct(item indexedProduct) (models.Product, error) {
	var product models.Product
	err := json.Unmarshal([]byte(item.document.PayloadJSON), &product)
	return product, err
}
