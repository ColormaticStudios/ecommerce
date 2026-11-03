package search

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"gorm.io/gorm"
)

const configurationCacheTTL = 10 * time.Second
const configurationCacheMaxRows = 4096
const configurationCacheMaxBytes = 2 << 20

type configurationKind uint8

const (
	synonymConfiguration configurationKind = iota
	typoConfiguration
	rankingConfiguration
	merchandisingConfiguration
	configurationKindCount
)

type configurationEntry struct {
	data       []byte
	expires    time.Time
	generation uint64
	loading    chan struct{}
}

// Fixed buckets avoid a shopper-controlled cache key space. Only configuration
// is retained; catalog targets and eligibility are always resolved live.
type configurationCache struct {
	mu      sync.Mutex
	entries [configurationKindCount]configurationEntry
	now     func() time.Time
}

func newConfigurationCache() *configurationCache {
	return &configurationCache{now: time.Now}
}

func (c *configurationCache) invalidate(kind configurationKind) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := &c.entries[kind]
	entry.generation++
	entry.data = nil
}

func (s *Service) invalidateConfiguration(kind configurationKind) {
	if backend, ok := s.backend.(*databaseBackend); ok {
		backend.configuration.invalidate(kind)
	}
}

// Every caller receives a decoded copy, including nested pointers and slices.
// Failed reads/validation and oversized snapshots are never retained. A load
// racing a committed admin change cannot restore the invalidated generation.
func cachedConfiguration[T any](ctx context.Context, c *configurationCache, kind configurationKind, load func() ([]T, error)) ([]T, error) {
	if c == nil {
		return load()
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		c.mu.Lock()
		entry := &c.entries[kind]
		if entry.data != nil && c.now().Before(entry.expires) {
			data := entry.data
			c.mu.Unlock()
			var rows []T
			err := json.Unmarshal(data, &rows)
			return rows, err
		}
		if waiting := entry.loading; waiting != nil {
			c.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-waiting:
				continue
			}
		}
		generation := entry.generation
		started := c.now()
		entry.loading = make(chan struct{})
		c.mu.Unlock()

		rows, err := load()
		var data []byte
		if err == nil && len(rows) <= configurationCacheMaxRows {
			data, err = json.Marshal(rows)
			if len(data) > configurationCacheMaxBytes {
				data = nil
			}
		}
		c.mu.Lock()
		entry = &c.entries[kind]
		if err == nil && data != nil && generation == entry.generation {
			entry.data = data
			entry.expires = started.Add(configurationCacheTTL)
		}
		close(entry.loading)
		entry.loading = nil
		c.mu.Unlock()
		return rows, err
	}
}

// Invalidate only after commit, before any optional follow-up read can fail.
func (s *Service) configurationTransaction(db *gorm.DB, kind configurationKind, write func(*gorm.DB) error) error {
	err := db.Transaction(write)
	if err == nil {
		s.invalidateConfiguration(kind)
	}
	return err
}
