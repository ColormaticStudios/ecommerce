// Package search owns the catalog search projection, retrieval boundary, and
// durable index lifecycle handlers.
package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"ecommerce/internal/jobs"
	"ecommerce/internal/reliability"
	"ecommerce/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ProductEntityType      = "product"
	ProductIndexName       = "products"
	JobTypeProductSync     = "search.product_sync"
	JobTypeFullReindex     = "search.full_reindex"
	ProductSyncVersion     = 1
	FullReindexVersion     = 1
	HealthyFreshnessTarget = 60 * time.Second
	StaleFreshnessTarget   = 5 * time.Minute
)

type ProductSyncPayload struct {
	Version   int   `json:"version"`
	ProductID uint  `json:"product_id"`
	Revision  int64 `json:"revision"`
}

type FullReindexPayload struct {
	Version   int    `json:"version"`
	RequestID string `json:"request_id"`
}

type Filters struct {
	Query             string
	MinPrice          *float64
	MaxPrice          *float64
	BrandSlug         string
	BrandSlugs        []string
	CategorySlugs     []string
	HasVariantStock   *bool
	StockAvailability []bool
	Attributes        map[string]string
	AttributeValues   map[string][]string
	PriceRanges       []PriceRange
	Channel           string
	RuleOverrides     *[]MerchandisingRule
	SkipMerchandising bool
	MerchandisingAt   *time.Time
	RankingProfile    string
	Explain           bool
	SortField         string
	SortOrder         string
	Page              int
	Limit             int
}

type PriceRange struct {
	Min *float64
	Max *float64
}

type FacetValue struct {
	Value    string
	Label    string
	Count    int64
	Selected bool
	Disabled bool
	MinPrice *float64
	MaxPrice *float64
}

type Facet struct {
	Name   string
	Label  string
	Type   string
	Values []FacetValue
}

type AppliedRewrite struct {
	Kind        string
	Original    string
	Replacement string
}

type Result struct {
	RuleDecisions         []RuleDecision
	RankingProfile        string
	RankingProfileVersion int
	Explanations          []RankingExplanation
	Products              []models.Product
	Facets                []Facet
	Total                 int64
	TotalPages            int
	NormalizedQuery       string
	AppliedRewrites       []AppliedRewrite
	DidYouMean            string
	Relaxed               bool
	IndexedAt             *time.Time
}

type SuggestionResult struct {
	Suggestions []string
	Corrections []string
	Popular     []string
}

type Freshness struct {
	Status            string
	Lag               time.Duration
	PendingJobs       int64
	DocumentCount     int64
	LastIndexedAt     *time.Time
	LastFullReindexAt *time.Time
}

// SearchBackend separates application orchestration from the database-backed
// index. A later engine adapter can implement this contract without moving
// catalog lifecycle logic into HTTP handlers.
type SearchBackend interface {
	ReplaceAll(context.Context, []models.SearchDocument, time.Time) error
	Upsert(context.Context, models.SearchDocument) error
	Delete(context.Context, string, uint, int64) error
	Search(context.Context, Filters) (Result, error)
	Suggest(context.Context, string, int) (SuggestionResult, error)
	Freshness(context.Context) (Freshness, error)
}

type Service struct {
	db      *gorm.DB
	backend SearchBackend
	jobs    *jobs.Runtime
	now     func() time.Time
}

func NewService(db *gorm.DB, backend SearchBackend, runtime *jobs.Runtime) *Service {
	if backend == nil && db != nil {
		backend = NewDatabaseBackend(db)
	}
	return &Service{db: db, backend: backend, jobs: runtime, now: func() time.Time { return time.Now().UTC() }}
}

func NewDatabaseBackend(db *gorm.DB) SearchBackend {
	return &databaseBackend{db: db, now: func() time.Time { return time.Now().UTC() }}
}

func NormalizeQuery(value string) string {
	var builder strings.Builder
	space := true
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			builder.WriteRune(r)
			space = false
			continue
		}
		if !space {
			builder.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(builder.String())
}

func (s *Service) Search(ctx context.Context, filters Filters) (Result, error) {
	if s == nil || s.backend == nil {
		return Result{}, errors.New("search backend is required")
	}
	return s.backend.Search(ctx, filters)
}

func (s *Service) Suggest(ctx context.Context, query string, limit int) (SuggestionResult, error) {
	if s == nil || s.backend == nil {
		return SuggestionResult{}, errors.New("search backend is required")
	}
	return s.backend.Suggest(ctx, query, limit)
}

func (s *Service) Freshness(ctx context.Context) (Freshness, error) {
	if s == nil || s.backend == nil {
		return Freshness{}, errors.New("search backend is required")
	}
	return s.backend.Freshness(ctx)
}

func (s *Service) RegisterJobHandlers() error {
	if s == nil || s.jobs == nil {
		return errors.New("search job runtime is required")
	}
	if err := s.jobs.Register(JobTypeProductSync, jobs.Registration{Handle: s.handleProductSync}); err != nil {
		return err
	}
	if err := s.jobs.Register(JobTypeFullReindex, jobs.Registration{Handle: s.handleFullReindex}); err != nil {
		return err
	}
	return s.jobs.Register(JobTypeSalesRefresh, jobs.Registration{Handle: s.handleSalesRefresh})
}

func (s *Service) EnqueueFullReindex(ctx context.Context) (models.JobQueue, error) {
	if s == nil || s.jobs == nil {
		return models.JobQueue{}, errors.New("search job runtime is required")
	}
	requestID := uuid.NewString()
	return s.jobs.Enqueue(ctx, jobs.EnqueueInput{
		JobType: JobTypeFullReindex, Payload: FullReindexPayload{Version: FullReindexVersion, RequestID: requestID},
		IdempotencyKey: "full:" + requestID,
	})
}

// EnsureInitialReindex durably schedules the first catalog backfill. The fixed
// key makes concurrent application starts collapse to one operation.
func (s *Service) EnsureInitialReindex(ctx context.Context) (models.JobQueue, bool, error) {
	if s == nil || s.db == nil || s.jobs == nil {
		return models.JobQueue{}, false, errors.New("search database and job runtime are required")
	}
	var state models.SearchIndexState
	err := s.db.WithContext(ctx).First(&state, "name = ?", ProductIndexName).Error
	if err == nil && state.LastFullReindexAt != nil {
		return models.JobQueue{}, false, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.JobQueue{}, false, err
	}
	job, err := s.jobs.Enqueue(ctx, jobs.EnqueueInput{
		JobType: JobTypeFullReindex, Payload: FullReindexPayload{Version: FullReindexVersion, RequestID: "initial"}, IdempotencyKey: "initial",
	})
	return job, err == nil, err
}

func EnqueueProductSyncTx(ctx context.Context, runtime *jobs.Runtime, tx *gorm.DB, productID uint, revision time.Time) (models.JobQueue, error) {
	if runtime == nil {
		return models.JobQueue{}, nil
	}
	if productID == 0 {
		return models.JobQueue{}, errors.New("search product ID is required")
	}
	if revision.IsZero() {
		revision = time.Now().UTC()
	}
	payload := ProductSyncPayload{Version: ProductSyncVersion, ProductID: productID, Revision: revision.UnixNano()}
	return runtime.EnqueueTx(ctx, tx, jobs.EnqueueInput{
		JobType: JobTypeProductSync, Payload: payload,
		IdempotencyKey: fmt.Sprintf("product:%d:%d", productID, payload.Revision),
	})
}

func (s *Service) Reindex(ctx context.Context) error {
	if s == nil || s.db == nil || s.backend == nil {
		return errors.New("search database and backend are required")
	}
	started := s.now()
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.Assignments(map[string]any{"last_full_reindex_started": started, "updated_at": started}),
	}).Create(&models.SearchIndexState{Name: ProductIndexName, LastFullReindexStarted: &started}).Error; err != nil {
		return err
	}
	var products []models.Product
	err := s.db.WithContext(ctx).
		Preload("Brand").Preload("Categories").Preload("Variants").
		Preload("AttributeValues.ProductAttribute").
		Where("products.is_published = ?", true).
		Where(`NOT EXISTS (SELECT 1 FROM product_variants pv_all WHERE pv_all.product_id = products.id) OR EXISTS (SELECT 1 FROM product_variants pv_public WHERE pv_public.product_id = products.id AND pv_public.is_published = TRUE)`).
		Order("products.id asc").Find(&products).Error
	if err != nil {
		return err
	}
	documents := make([]models.SearchDocument, 0, len(products))
	for _, product := range products {
		document, include, err := projectProduct(product, started)
		if err != nil {
			return err
		}
		if include {
			documents = append(documents, document)
		}
	}
	return s.backend.ReplaceAll(ctx, documents, started)
}

func (s *Service) SyncProduct(ctx context.Context, productID uint) error {
	var product models.Product
	err := s.db.WithContext(ctx).Unscoped().Preload("Brand").Preload("Categories").Preload("Variants").
		Preload("AttributeValues.ProductAttribute").First(&product, productID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && (product.DeletedAt.Valid || !product.IsPublished)) {
		version := s.now().UnixNano()
		if err == nil && !product.UpdatedAt.IsZero() {
			version = product.UpdatedAt.UnixNano()
		}
		return s.backend.Delete(ctx, ProductEntityType, productID, version)
	}
	if err != nil {
		return err
	}
	document, include, err := projectProduct(product, s.now())
	if err != nil {
		return err
	}
	if !include {
		return s.backend.Delete(ctx, ProductEntityType, productID, product.UpdatedAt.UnixNano())
	}
	return s.backend.Upsert(ctx, document)
}

func (s *Service) handleProductSync(ctx context.Context, raw json.RawMessage) error {
	var payload ProductSyncPayload
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Version != ProductSyncVersion || payload.ProductID == 0 {
		if err == nil {
			err = errors.New("unsupported or invalid product sync payload")
		}
		return reliability.Classify(err, reliability.ClassTerminal)
	}
	if err := s.SyncProduct(ctx, payload.ProductID); err != nil {
		return reliability.Classify(err, reliability.ClassRetryable)
	}
	return nil
}

func (s *Service) handleFullReindex(ctx context.Context, raw json.RawMessage) error {
	var payload FullReindexPayload
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Version != FullReindexVersion || strings.TrimSpace(payload.RequestID) == "" {
		if err == nil {
			err = errors.New("unsupported full reindex payload")
		}
		return reliability.Classify(err, reliability.ClassTerminal)
	}
	if err := s.Reindex(ctx); err != nil {
		return reliability.Classify(err, reliability.ClassRetryable)
	}
	return nil
}

func projectProduct(product models.Product, indexedAt time.Time) (models.SearchDocument, bool, error) {
	if !product.IsPublished || product.DeletedAt.Valid {
		return models.SearchDocument{}, false, nil
	}
	variants := make([]models.ProductVariant, 0, len(product.Variants))
	minPrice, maxPrice := product.Price, product.Price
	available := product.Stock > 0
	terms := []string{product.Name, product.SKU, product.Description}
	for _, variant := range product.Variants {
		if !variant.IsPublished {
			continue
		}
		variants = append(variants, variant)
		terms = append(terms, variant.Title, variant.SKU)
		if len(variants) == 1 || variant.Price < minPrice {
			minPrice = variant.Price
		}
		if len(variants) == 1 || variant.Price > maxPrice {
			maxPrice = variant.Price
		}
		if variant.Stock > 0 {
			available = true
		}
	}
	if len(product.Variants) > 0 && len(variants) == 0 {
		return models.SearchDocument{}, false, nil
	}
	product.Variants = variants
	if product.Related == nil {
		product.Related = []models.Product{}
	}
	if product.Categories == nil {
		product.Categories = []models.Category{}
	}
	brandSlug := ""
	if product.Brand != nil && product.Brand.IsActive {
		brandSlug = strings.ToLower(product.Brand.Slug)
		terms = append(terms, product.Brand.Name, product.Brand.Slug)
	}
	categoryTokens := "|"
	for _, category := range product.Categories {
		if !category.IsActive {
			continue
		}
		slug := NormalizeQuery(category.Slug)
		categoryTokens += slug + "|"
		terms = append(terms, category.Name, category.Slug)
	}
	attributeTokens := "|"
	for _, value := range product.AttributeValues {
		if value.ProductAttribute == nil {
			continue
		}
		slug := NormalizeQuery(value.ProductAttribute.Slug)
		raw := attributeValue(value)
		if slug == "" || raw == "" {
			continue
		}
		if value.ProductAttribute.Filterable {
			attributeTokens += slug + "=" + NormalizeQuery(raw) + "|"
		}
		terms = append(terms, value.ProductAttribute.Key, raw)
	}
	payload, err := json.Marshal(product)
	if err != nil {
		return models.SearchDocument{}, false, err
	}
	return models.SearchDocument{
		EntityType: ProductEntityType, EntityID: product.ID, PayloadJSON: string(payload),
		SearchableText: NormalizeQuery(strings.Join(terms, " ")), NormalizedName: NormalizeQuery(product.Name),
		BrandSlug: brandSlug, CategoryTokens: categoryTokens, AttributeTokens: attributeTokens,
		MinPrice: minPrice, MaxPrice: maxPrice, Available: available,
		Active:  true,
		Version: product.UpdatedAt.UnixNano(), SourceCreatedAt: product.CreatedAt, SourceUpdatedAt: product.UpdatedAt,
		IndexedAt: indexedAt,
	}, true, nil
}

func attributeValue(value models.ProductAttributeValue) string {
	switch {
	case value.TextValue != nil:
		return *value.TextValue
	case value.NumberValue != nil:
		return strconv.FormatFloat(*value.NumberValue, 'f', -1, 64)
	case value.BooleanValue != nil:
		return strconv.FormatBool(*value.BooleanValue)
	case value.EnumValue != nil:
		return *value.EnumValue
	default:
		return ""
	}
}

type databaseBackend struct {
	db  *gorm.DB
	now func() time.Time
}

func (b *databaseBackend) ReplaceAll(ctx context.Context, documents []models.SearchDocument, completed time.Time) error {
	return b.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.SearchDocument{}).Where("entity_type = ? AND version <= ?", ProductEntityType, completed.UnixNano()).Update("active", false).Error; err != nil {
			return err
		}
		for _, document := range documents {
			if err := upsertDocument(tx, document); err != nil {
				return err
			}
		}
		var count int64
		if err := tx.Model(&models.SearchDocument{}).Where("entity_type = ? AND active = ?", ProductEntityType, true).Count(&count).Error; err != nil {
			return err
		}
		return upsertIndexState(tx, completed, &completed, count)
	})
}

func (b *databaseBackend) Upsert(ctx context.Context, document models.SearchDocument) error {
	now := b.now()
	document.IndexedAt = now
	return b.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := upsertDocument(tx, document); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&models.SearchDocument{}).Where("entity_type = ? AND active = ?", ProductEntityType, true).Count(&count).Error; err != nil {
			return err
		}
		return upsertIndexState(tx, now, nil, count)
	})
}

func (b *databaseBackend) Delete(ctx context.Context, entityType string, entityID uint, version int64) error {
	now := b.now()
	return b.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		document := models.SearchDocument{EntityType: entityType, EntityID: entityID, PayloadJSON: "{}", SearchableText: "", NormalizedName: "", CategoryTokens: "|", AttributeTokens: "|", Version: version, SourceCreatedAt: now, SourceUpdatedAt: now, IndexedAt: now, Active: false}
		if err := upsertDocument(tx, document); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&models.SearchDocument{}).Where("entity_type = ? AND active = ?", ProductEntityType, true).Count(&count).Error; err != nil {
			return err
		}
		return upsertIndexState(tx, now, nil, count)
	})
}

func upsertDocument(tx *gorm.DB, document models.SearchDocument) error {
	now := document.IndexedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	values := map[string]any{
		"entity_type": document.EntityType, "entity_id": document.EntityID, "payload_json": document.PayloadJSON,
		"searchable_text": document.SearchableText, "normalized_name": document.NormalizedName, "brand_slug": document.BrandSlug,
		"category_tokens": document.CategoryTokens, "attribute_tokens": document.AttributeTokens,
		"min_price": document.MinPrice, "max_price": document.MaxPrice, "available": document.Available, "active": document.Active,
		"version": document.Version, "source_created_at": document.SourceCreatedAt, "source_updated_at": document.SourceUpdatedAt,
		"indexed_at": document.IndexedAt, "created_at": now, "updated_at": now,
	}
	return tx.Table("search_documents").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "entity_type"}, {Name: "entity_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"payload_json", "searchable_text", "normalized_name", "brand_slug", "category_tokens", "attribute_tokens", "min_price", "max_price", "available", "active", "version", "source_created_at", "source_updated_at", "indexed_at", "updated_at"}),
		Where:     clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "search_documents.version <= excluded.version"}}},
	}).Create(values).Error
}

func upsertIndexState(tx *gorm.DB, indexedAt time.Time, fullAt *time.Time, count int64) error {
	values := map[string]any{"last_indexed_at": indexedAt, "document_count": count, "updated_at": indexedAt}
	if fullAt != nil {
		values["last_full_reindex_at"] = *fullAt
	}
	state := models.SearchIndexState{Name: ProductIndexName, LastIndexedAt: &indexedAt, DocumentCount: count}
	if fullAt != nil {
		state.LastFullReindexAt = fullAt
	}
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoUpdates: clause.Assignments(values)}).Create(&state).Error
}

func (b *databaseBackend) Search(ctx context.Context, filters Filters) (Result, error) {
	return b.searchProducts(ctx, filters)
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}

func (b *databaseBackend) Suggest(ctx context.Context, query string, limit int) (SuggestionResult, error) {
	if limit < 1 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}
	normalized := NormalizeQuery(query)
	result := SuggestionResult{Suggestions: []string{}, Corrections: []string{}, Popular: []string{}}
	if normalized == "" {
		return result, nil
	}
	var documents []models.SearchDocument
	prefix := escapeLike(normalized) + "%"
	if err := b.db.WithContext(ctx).Where("entity_type = ? AND active = ? AND (normalized_name LIKE ? OR brand_slug LIKE ?)", ProductEntityType, true, prefix, prefix).Order("normalized_name asc, entity_id asc").Limit(limit * 2).Find(&documents).Error; err != nil {
		return result, err
	}
	seen := map[string]struct{}{}
	values := make([]string, 0, limit)
	for _, document := range documents {
		var product models.Product
		if err := json.Unmarshal([]byte(document.PayloadJSON), &product); err != nil {
			return result, err
		}
		candidates := []string{product.Name}
		if product.Brand != nil {
			candidates = append(candidates, product.Brand.Name)
		}
		for _, candidate := range candidates {
			if !strings.HasPrefix(NormalizeQuery(candidate), normalized) {
				continue
			}
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			values = append(values, candidate)
		}
	}
	sort.Strings(values)
	if len(values) > limit {
		values = values[:limit]
	}
	result.Suggestions = values
	if len(values) == 0 {
		profile, err := b.loadTypoToleranceProfile(ctx)
		if err != nil {
			return result, err
		}
		documents, err := b.loadIndexedProducts(ctx, nil)
		if err != nil {
			return result, err
		}
		corrected, rewrites := correctQuery(normalized, buildVocabulary(documents), profile)
		if len(rewrites) > 0 {
			plan := buildQueryPlan(corrected, nil)
			for _, document := range documents {
				if matchesQuery(document.document.SearchableText, plan, false) {
					result.Corrections = []string{corrected}
					break
				}
			}
		}
	}
	return result, nil
}

func (b *databaseBackend) Freshness(ctx context.Context) (Freshness, error) {
	var state models.SearchIndexState
	err := b.db.WithContext(ctx).First(&state, "name = ?", ProductIndexName).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return Freshness{}, err
	}
	var pending int64
	if err := b.db.WithContext(ctx).Model(&models.JobQueue{}).
		Where("job_type IN ? AND status IN ?", []string{JobTypeProductSync, JobTypeFullReindex}, []string{models.JobStatusPending, models.JobStatusRunning, models.JobStatusRetryScheduled}).Count(&pending).Error; err != nil {
		return Freshness{}, err
	}
	result := Freshness{Status: "healthy", PendingJobs: pending, DocumentCount: state.DocumentCount, LastIndexedAt: state.LastIndexedAt, LastFullReindexAt: state.LastFullReindexAt}
	if pending > 0 {
		var oldest models.JobQueue
		if err := b.db.WithContext(ctx).Where("job_type IN ? AND status IN ?", []string{JobTypeProductSync, JobTypeFullReindex}, []string{models.JobStatusPending, models.JobStatusRunning, models.JobStatusRetryScheduled}).Order("created_at asc").First(&oldest).Error; err != nil {
			return result, err
		}
		result.Lag = b.now().Sub(oldest.CreatedAt)
	}
	if state.LastFullReindexAt == nil || result.Lag > StaleFreshnessTarget {
		result.Status = "stale"
	} else if result.Lag > HealthyFreshnessTarget {
		result.Status = "degraded"
	}
	return result, nil
}
