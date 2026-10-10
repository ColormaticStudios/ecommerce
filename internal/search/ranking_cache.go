package search

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"ecommerce/models"
	"golang.org/x/sync/singleflight"
)

type rankingCacheEntry struct {
	scores        map[uint]RankingExplanation
	created, used time.Time
}
type rankingCache struct {
	mu         sync.Mutex
	generation uint64
	entries    map[string]rankingCacheEntry
	flight     singleflight.Group
}

func rankingCacheKey(documents []indexedProduct, plan queryPlan, query string, profile RankingProfile, now time.Time) string {
	hash := sha256.New()
	weights, _ := json.Marshal(profile.Weights)
	fmt.Fprintf(hash, "%s:%s:%s:%s:", planCacheKey(&plan), query, weights, now.UTC().Truncate(24*time.Hour).Format(time.RFC3339))
	var buf [8]byte
	for _, doc := range documents {
		binary.LittleEndian.PutUint64(buf[:], uint64(doc.document.EntityID))
		hash.Write(buf[:])
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}
func (b *databaseBackend) rankProducts(ctx context.Context, documents []indexedProduct, plan queryPlan, query string, profile RankingProfile, order string, explain bool) error {
	if len(documents) < 500 {
		return b.rankProductsUncached(ctx, documents, plan, query, profile, order, explain)
	}
	var state models.SearchIndexState
	if err := b.db.WithContext(ctx).Select("generation").First(&state, "name = ?", ProductIndexName).Error; err != nil {
		return b.rankProductsUncached(ctx, documents, plan, query, profile, order, explain)
	}
	for _, document := range documents {
		if document.cacheGeneration != state.Generation {
			return b.rankProductsUncached(ctx, documents, plan, query, profile, order, explain)
		}
	}
	c := &b.rankCache
	key := fmt.Sprintf("%t:%s", explain, rankingCacheKey(documents, plan, query, profile, b.now()))
	c.mu.Lock()
	if state.Generation < c.generation {
		c.mu.Unlock()
		return b.rankProductsUncached(ctx, documents, plan, query, profile, order, explain)
	}
	if c.generation != state.Generation {
		c.generation = state.Generation
		c.entries = map[string]rankingCacheEntry{}
	}
	entry, hit := c.entries[key]
	if hit && time.Since(entry.created) < cachedDocumentTTL {
		entry.used = time.Now()
		c.entries[key] = entry
		c.mu.Unlock()
		applyCachedRanking(documents, entry.scores, order)
		return nil
	}
	c.mu.Unlock()
	pending := c.flight.DoChan(fmt.Sprintf("%d:%s", state.Generation, key), func() (any, error) {
		c.mu.Lock()
		entry, hit := c.entries[key]
		if c.generation == state.Generation && hit && time.Since(entry.created) < cachedDocumentTTL {
			c.mu.Unlock()
			return entry.scores, nil
		}
		c.mu.Unlock()
		copyDocs := append([]indexedProduct(nil), documents...)
		if err := b.rankProductsUncached(ctx, copyDocs, plan, query, profile, order, explain); err != nil {
			return nil, err
		}
		scores := make(map[uint]RankingExplanation, len(copyDocs))
		for _, doc := range copyDocs {
			scores[doc.document.EntityID] = doc.ranking
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.generation != state.Generation || len(scores) > cachedDocumentLimit {
			return scores, nil
		}
		if c.entries == nil {
			c.entries = map[string]rankingCacheEntry{}
		}
		for {
			count := len(scores)
			for _, entry := range c.entries {
				count += len(entry.scores)
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
		now := time.Now()
		c.entries[key] = rankingCacheEntry{scores: scores, created: now, used: now}
		return scores, nil
	})
	select {
	case <-ctx.Done():
		return ctx.Err()
	case result := <-pending:
		if result.Err != nil {
			if ctx.Err() == nil && (errors.Is(result.Err, context.Canceled) || errors.Is(result.Err, context.DeadlineExceeded)) {
				return b.rankProductsUncached(ctx, documents, plan, query, profile, order, explain)
			}
			return result.Err
		}
		applyCachedRanking(documents, result.Val.(map[uint]RankingExplanation), order)
		return nil
	}
}
func applyCachedRanking(documents []indexedProduct, scores map[uint]RankingExplanation, order string) {
	for i := range documents {
		documents[i].ranking = scores[documents[i].document.EntityID]
	}
	ascending := strings.EqualFold(order, "asc")
	sort.Slice(documents, func(i, j int) bool {
		a, b := documents[i].ranking.Score, documents[j].ranking.Score
		if a == b {
			return documents[i].document.EntityID < documents[j].document.EntityID
		}
		if ascending {
			return a < b
		}
		return a > b
	})
}
