package search

import (
	"context"
	"database/sql/driver"
	"errors"
	"net"
	"sync"
	"time"

	"ecommerce/internal/reliability"
	"ecommerce/models"
)

var (
	ErrCapacity         = errors.New("search capacity exhausted")
	ErrCircuitOpen      = errors.New("indexed search circuit is open")
	ErrIndexUnavailable = errors.New("indexed search unavailable")
)

type HardeningConfig struct {
	MaxConcurrent           int `json:"max_concurrent"`
	SearchTimeoutMS         int `json:"search_timeout_ms"`
	CircuitFailureThreshold int `json:"circuit_failure_threshold"`
	CircuitOpenMS           int `json:"circuit_open_ms"`
	ReindexQueueLimit       int `json:"reindex_queue_limit"`
}

func DefaultHardeningConfig() HardeningConfig { return HardeningConfig{50, 2000, 5, 30000, 1} }
func (c HardeningConfig) Validate() error {
	if c.MaxConcurrent < 1 || c.MaxConcurrent > 512 || c.SearchTimeoutMS < 10 || c.SearchTimeoutMS > 60000 || c.CircuitFailureThreshold < 1 || c.CircuitFailureThreshold > 100 || c.CircuitOpenMS < 10 || c.CircuitOpenMS > 600000 || c.ReindexQueueLimit < 1 || c.ReindexQueueLimit > 20 {
		return errors.New("invalid search hardening configuration")
	}
	return nil
}

type OperationsStatus struct {
	HardeningConfig
	ActiveSearches      int        `json:"active_searches"`
	CircuitState        string     `json:"circuit_state"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	CircuitOpenUntil    *time.Time `json:"circuit_open_until"`
	PendingReindexes    int64      `json:"pending_reindexes"`
}
type hardeningState struct {
	config     HardeningConfig
	slots      chan struct{}
	mu         sync.Mutex
	failures   int
	openUntil  time.Time
	probing    bool
	generation uint64
}

func newHardeningState(c HardeningConfig) *hardeningState {
	return &hardeningState{config: c, slots: make(chan struct{}, c.MaxConcurrent)}
}

// ConfigureHardening is startup-only, before serving requests or running jobs.
func (s *Service) ConfigureHardening(c HardeningConfig) error {
	if err := c.Validate(); err != nil {
		return err
	}
	s.hardening = newHardeningState(c)
	return nil
}
func (s *Service) Operations(ctx context.Context) (OperationsStatus, error) {
	h := s.hardening
	h.mu.Lock()
	status := OperationsStatus{HardeningConfig: h.config, ActiveSearches: len(h.slots), CircuitState: "closed", ConsecutiveFailures: h.failures}
	if !h.openUntil.IsZero() {
		until := h.openUntil
		status.CircuitOpenUntil = &until
		status.CircuitState = "open"
		if !s.now().Before(until) {
			status.CircuitState = "half_open"
		}
	}
	h.mu.Unlock()
	err := s.db.WithContext(ctx).Model(&models.JobQueue{}).Where("job_type = ? AND status IN ?", JobTypeFullReindex, []string{models.JobStatusPending, models.JobStatusRunning, models.JobStatusRetryScheduled}).Count(&status.PendingReindexes).Error
	return status, err
}
func (h *hardeningState) admit(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case h.slots <- struct{}{}:
		return nil
	default:
		return ErrCapacity
	}
}
func (h *hardeningState) probe(now time.Time) (bool, uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.openUntil.IsZero() {
		return true, h.generation
	}
	if now.Before(h.openUntil) || h.probing {
		return false, h.generation
	}
	h.generation++
	h.probing = true
	return true, h.generation
}
func (h *hardeningState) finish(now time.Time, err error, generation uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if generation != h.generation {
		return
	}
	h.probing = false
	if err == nil {
		h.failures = 0
		h.openUntil = time.Time{}
		return
	}
	if !operationalSearchError(err) {
		return
	}
	h.failures++
	if h.failures >= h.config.CircuitFailureThreshold {
		h.generation++
		h.openUntil = now.Add(time.Duration(h.config.CircuitOpenMS) * time.Millisecond)
	}
}
func operationalSearchError(err error) bool {
	if errors.Is(err, ErrIndexUnavailable) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, driver.ErrBadConn) {
		return true
	}
	var network net.Error
	if errors.As(err, &network) {
		return true
	}
	class, ok := reliability.ErrorClassOf(err)
	return ok && class == reliability.ClassRetryable
}
func (s *Service) searchWithProtection(ctx context.Context, f Filters) (Result, error) {
	h := s.hardening
	if err := h.admit(ctx); err != nil {
		return Result{}, err
	}
	defer func() { <-h.slots }()
	reason := "circuit_open"
	allowed, generation := h.probe(s.now())
	if allowed {
		bounded, cancel := context.WithTimeout(ctx, time.Duration(h.config.SearchTimeoutMS)*time.Millisecond)
		result, err := s.backend.Search(bounded, f)
		cancel()
		if ctx.Err() != nil {
			h.finish(s.now(), ctx.Err(), generation)
			return Result{}, ctx.Err()
		}
		h.finish(s.now(), err, generation)
		if err == nil {
			return result, nil
		}
		if !operationalSearchError(err) {
			return Result{}, err
		}
		reason = "indexed_search_unavailable"
		if errors.Is(err, context.DeadlineExceeded) {
			reason = "search_timeout"
		}
	}
	// Previews and administrator explanations must never pretend a browse result is ranked.
	if f.Explain || f.RuleOverrides != nil || f.SkipMerchandising {
		return Result{}, ErrIndexUnavailable
	}
	fallback, cancel := context.WithTimeout(ctx, time.Duration(h.config.SearchTimeoutMS)*time.Millisecond)
	defer cancel()
	result, err := s.catalogFallback(fallback, f)
	if err != nil {
		return Result{}, err
	}
	result.Degraded = true
	result.FallbackReason = reason
	return result, nil
}
func (s *Service) suggestWithProtection(ctx context.Context, q string, limit int) (SuggestionResult, error) {
	h := s.hardening
	if err := h.admit(ctx); err != nil {
		return SuggestionResult{}, err
	}
	defer func() { <-h.slots }()
	allowed, generation := h.probe(s.now())
	if !allowed {
		return SuggestionResult{}, ErrCircuitOpen
	}
	bounded, cancel := context.WithTimeout(ctx, time.Duration(h.config.SearchTimeoutMS)*time.Millisecond)
	defer cancel()
	result, err := s.backend.Suggest(bounded, q, limit)
	if ctx.Err() != nil {
		h.finish(s.now(), ctx.Err(), generation)
		return SuggestionResult{}, ctx.Err()
	}
	h.finish(s.now(), err, generation)
	if err == nil {
		result.Popular, result.Trending, err = s.PopularQueries(bounded, q, limit)
	}
	return result, err
}

// Basic browsing deliberately ignores keyword retrieval. Hide predicates still
// use the original query/context, so an outage cannot bypass merchandising.
func (s *Service) catalogFallback(ctx context.Context, f Filters) (Result, error) {
	if s.db == nil {
		return Result{}, ErrIndexUnavailable
	}
	if f.RankingProfile != "" {
		if _, err := (&databaseBackend{db: s.db}).loadRankingProfile(ctx, f.RankingProfile); err != nil {
			return Result{}, err
		}
	}
	normalized, err := normalizeFilters(f)
	if err != nil {
		return Result{}, err
	}
	rules, err := s.ListMerchandisingRules(ctx)
	if err != nil {
		return Result{}, err
	} // fail closed
	hidden := map[uint]bool{}
	for _, rule := range rules {
		if rule.RuleType == "hide" && merchandisingPredicateReason(rule, NormalizeQuery(f.Query), normalized, f.Channel, s.now()) == "" {
			for _, target := range rule.Action.Targets {
				hidden[target.ProductID] = true
			}
		}
	}
	documents := []indexedProduct{}
	var cursor uint
	for {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		var batch []models.Product
		err := s.db.WithContext(ctx).Preload("Brand").Preload("Categories").Preload("Variants").Preload("AttributeValues.ProductAttribute").Where("products.is_published = ? AND products.id > ?", true, cursor).Order("products.id ASC").Limit(250).Find(&batch).Error
		if err != nil {
			return Result{}, err
		}
		if len(batch) == 0 {
			break
		}
		rows := make([]models.SearchDocument, 0, len(batch))
		for _, product := range batch {
			cursor = product.ID
			if hidden[product.ID] {
				continue
			}
			row, include, err := projectProduct(product, s.now())
			if err != nil {
				return Result{}, err
			}
			if include {
				rows = append(rows, row)
			}
		}
		decoded, err := decodeIndexedProducts(ctx, rows)
		if err != nil {
			return Result{}, err
		}
		for _, item := range decoded {
			if matchesFilters(item, normalized, "") {
				documents = append(documents, item)
			}
		}
	}
	field := f.SortField
	if field == "" || field == "relevance" {
		field = "created_at"
	}
	sortIndexedProducts(documents, field, f.SortOrder)
	page, limit := f.Page, f.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	total := len(documents)
	pages := (total + limit - 1) / limit
	start := total
	if page <= total/limit+1 {
		start = min((page-1)*limit, total)
	}
	end := min(start+limit, total)
	products := make([]models.Product, 0, end-start)
	for _, item := range documents[start:end] {
		products = append(products, *item.product)
	}
	return Result{Products: products, Facets: []Facet{}, Total: int64(total), TotalPages: pages, NormalizedQuery: NormalizeQuery(f.Query), AppliedRewrites: []AppliedRewrite{}}, nil
}
