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
	"sync"
	"time"
	"unicode"

	"ecommerce/internal/jobs"
	"ecommerce/internal/reliability"
	"ecommerce/internal/searchprojection"
	"ecommerce/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	JobTypeConversionRefresh = "search.conversion_refresh"
	ProductEntityType        = "product"
	ProductIndexName         = "products"
	JobTypeProductSync       = "search.product_sync"
	JobTypeFullReindex       = "search.full_reindex"
	ProductSyncVersion       = 1
	FullReindexVersion       = 1
	HealthyFreshnessTarget   = 60 * time.Second
	StaleFreshnessTarget     = 5 * time.Minute
)

type ProductSyncPayload struct {
	Version   int   `json:"version"`
	ProductID uint  `json:"product_id"`
	VariantID uint  `json:"variant_id,omitempty"`
	Revision  int64 `json:"revision,omitempty"`
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
	Degraded              bool
	FallbackReason        string
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
	// ReplaceAll applies a captured full snapshot, including explicit tombstones
	// for captured absent sources. Other rows must not be deactivated or
	// acknowledged using wall-clock time: they may cover newer source changes.
	ReplaceAll(context.Context, []models.SearchDocument, time.Time) error
	Upsert(context.Context, models.SearchDocument) error
	Delete(context.Context, string, uint, int64) error
	Search(context.Context, Filters) (Result, error)
	Suggest(context.Context, string, int) (SuggestionResult, error)
	Freshness(context.Context) (Freshness, error)
}

type Service struct {
	db           *gorm.DB
	backend      SearchBackend
	jobs         *jobs.Runtime
	now          func() time.Time
	hardening    *hardeningState
	reindexMu    sync.Mutex
	analyticsKey []byte
}

func NewService(db *gorm.DB, backend SearchBackend, runtime *jobs.Runtime) *Service {
	if backend == nil && db != nil {
		backend = NewDatabaseBackend(db)
	}
	return &Service{db: db, backend: backend, jobs: runtime, now: func() time.Time { return time.Now().UTC() }, hardening: newHardeningState(DefaultHardeningConfig())}
}

func NewDatabaseBackend(db *gorm.DB) SearchBackend {
	return &databaseBackend{db: db, now: func() time.Time { return time.Now().UTC() }, configuration: newConfigurationCache()}
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
	return s.searchWithProtection(ctx, filters)
}

func (s *Service) Suggest(ctx context.Context, query string, limit int) (SuggestionResult, error) {
	if s == nil || s.backend == nil {
		return SuggestionResult{}, errors.New("search backend is required")
	}
	return s.suggestWithProtection(ctx, query, limit)
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
	if err := s.jobs.Register(JobTypeProductSync, jobs.Registration{Handle: s.handleProductSync, ReleaseKeyOnCompletion: true, OnSuccess: s.finishProductSync}); err != nil {
		return err
	}
	if err := s.jobs.Register(JobTypeCatalogFanout, jobs.Registration{Handle: s.handleCatalogFanout, OnSuccess: s.finishCatalogFanout}); err != nil {
		return err
	}
	if err := s.jobs.Register(JobTypeFullReindex, jobs.Registration{Handle: s.handleFullReindex}); err != nil {
		return err
	}
	if err := s.jobs.Register(JobTypeSalesRefresh, jobs.Registration{Handle: s.handleSalesRefresh}); err != nil {
		return err
	}
	if err := s.RegisterAnalyticsJobHandler(); err != nil {
		return err
	}
	return s.jobs.Register(JobTypeConversionRefresh, jobs.Registration{Handle: s.handleConversionRefresh})
}

var ErrReindexQueued = errors.New("a full search reindex is already queued or running")

func (s *Service) EnqueueFullReindex(ctx context.Context) (models.JobQueue, error) {
	return s.enqueueFullReindex(ctx, false)
}

// EnqueueOrReuseFullReindex lets synchronous administrators join existing work
// without bypassing queue admission or executing an already-owned lease.
func (s *Service) EnqueueOrReuseFullReindex(ctx context.Context) (models.JobQueue, error) {
	return s.enqueueFullReindex(ctx, true)
}
func (s *Service) enqueueFullReindex(ctx context.Context, reuse bool) (models.JobQueue, error) {
	if s == nil || s.jobs == nil || s.db == nil {
		return models.JobQueue{}, errors.New("search job runtime is required")
	}
	var job models.JobQueue
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize admission across replicas through the index-state row.
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.SearchIndexState{Name: ProductIndexName}).Error; err != nil {
			return err
		}
		var state models.SearchIndexState
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&state, "name = ?", ProductIndexName).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&models.JobQueue{}).Where("job_type = ? AND status IN ?", JobTypeFullReindex, []string{models.JobStatusPending, models.JobStatusRunning, models.JobStatusRetryScheduled}).Count(&count).Error; err != nil {
			return err
		}
		if reuse && count > 0 {
			return tx.Where("job_type = ? AND status IN ?", JobTypeFullReindex, []string{models.JobStatusPending, models.JobStatusRunning, models.JobStatusRetryScheduled}).Order("created_at ASC, id ASC").First(&job).Error
		}
		if count >= int64(s.hardening.config.ReindexQueueLimit) {
			return ErrReindexQueued
		}
		requestID := uuid.NewString()
		var err error
		job, err = s.jobs.EnqueueTx(ctx, tx, jobs.EnqueueInput{JobType: JobTypeFullReindex, Payload: FullReindexPayload{Version: FullReindexVersion, RequestID: requestID}, IdempotencyKey: "full:" + requestID})
		return err
	})
	return job, err
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
		JobType: JobTypeFullReindex, Payload: FullReindexPayload{Version: FullReindexVersion, RequestID: "initial"}, IdempotencyKey: "full:initial",
	})
	return job, err == nil, err
}

func EnqueueProductSyncTx(ctx context.Context, runtime *jobs.Runtime, tx *gorm.DB, productID uint, revision time.Time) (models.JobQueue, error) {
	return RecordProductChangeTx(ctx, runtime, tx, productID, revision)
}

func (s *Service) Reindex(ctx context.Context) error {
	if s == nil || s.db == nil || s.backend == nil {
		return errors.New("search database and backend are required")
	}
	if !s.reindexMu.TryLock() {
		return ErrCapacity
	}
	defer s.reindexMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	started := s.now()
	var fanoutIDs []string
	if err := s.db.WithContext(ctx).Model(&models.SearchCatalogFanoutDebt{}).Where("covered_at IS NULL").Pluck("job_id", &fanoutIDs).Error; err != nil {
		return err
	}
	if len(fanoutIDs) > 0 {
		// Shared metadata has no per-product revision until fanout expands.
		// Allocate rebuild-owned boundaries before source reads, so a pre-edit
		// incremental write cannot later overwrite this rebuild at equal version.
		if err := allocateRebuildFanoutRevisions(ctx, s.db, started); err != nil {
			return err
		}
	}
	inventoryDebt, err := captureInventoryDebt(ctx, s.db, nil)
	if err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.Assignments(map[string]any{"last_full_reindex_started": started, "updated_at": started}),
	}).Create(&models.SearchIndexState{Name: ProductIndexName, LastFullReindexStarted: &started}).Error; err != nil {
		return err
	}
	if err := repairProjectionRevisionFloors(s.db.WithContext(ctx), nil); err != nil {
		return err
	}
	// Capture source debt and existing projections in one statement. Explicit
	// tombstones below cover only these boundaries, never later source writes.
	var revisions []models.SearchProductRevision
	if err := s.db.WithContext(ctx).Raw(`SELECT product_id, revision FROM search_product_revisions
		UNION ALL SELECT d.entity_id AS product_id, d.version AS revision FROM search_documents d
		WHERE d.entity_type = ? AND NOT EXISTS (SELECT 1 FROM search_product_revisions r WHERE r.product_id = d.entity_id)`, ProductEntityType).Scan(&revisions).Error; err != nil {
		return err
	}
	revisionByID := map[uint]int64{}
	covered := map[uint]bool{}
	for _, revision := range revisions {
		revisionByID[revision.ProductID] = revision.Revision
	}

	var products []models.Product
	documents := make([]models.SearchDocument, 0)
	err = s.db.WithContext(ctx).
		Preload("Brand").Preload("Categories").Preload("Variants").Preload("AttributeValues.ProductAttribute").
		Where("products.is_published = ?", true).
		Where(`NOT EXISTS (SELECT 1 FROM product_variants pv_all WHERE pv_all.product_id = products.id) OR EXISTS (SELECT 1 FROM product_variants pv_public WHERE pv_public.product_id = products.id AND pv_public.is_published = TRUE)`).
		Order("products.id asc").FindInBatches(&products, 500, func(tx *gorm.DB, batch int) error {
		for _, product := range products {
			if err := ctx.Err(); err != nil {
				return err
			}
			document, include, err := projectProduct(product, started)
			if err != nil {
				return err
			}
			if include {
				document.Version = revisionByID[product.ID]
				documents = append(documents, document)
				covered[product.ID] = true
			}
		}
		return nil
	}).Error
	if err != nil {
		return err
	}
	// Preserve captured revision boundaries for absent sources too. A future
	// document version (for example after correcting clock skew) must not make
	// an orphan permanently immune to a successful full rebuild.
	for _, revision := range revisions {
		if covered[revision.ProductID] {
			continue
		}
		documents = append(documents, models.SearchDocument{EntityType: ProductEntityType, EntityID: revision.ProductID, PayloadJSON: "{}", CategoryTokens: "|", AttributeTokens: "|", Version: revision.Revision, Active: false, IndexedAt: started, SourceCreatedAt: started, SourceUpdatedAt: started})
	}
	if err := s.backend.ReplaceAll(ctx, documents, started); err != nil {
		return err
	}
	if err := acknowledgeInventoryDebt(ctx, s.db, inventoryDebt); err != nil {
		return err
	}
	return coverFanoutDebt(ctx, s.db, fanoutIDs, s.now())
}

func (s *Service) SyncProduct(ctx context.Context, productID uint) error {
	inventoryDebt, err := captureInventoryDebt(ctx, s.db, &productID)
	if err != nil {
		return err
	}
	revision, err := projectionRevision(ctx, s.db, productID)
	if err != nil {
		return err
	}
	var product models.Product
	err = s.db.WithContext(ctx).Unscoped().Preload("Brand").Preload("Categories").Preload("Variants").
		Preload("AttributeValues.ProductAttribute").First(&product, productID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && (product.DeletedAt.Valid || !product.IsPublished)) {
		if err := s.backend.Delete(ctx, ProductEntityType, productID, revision); err != nil {
			return err
		}
		return acknowledgeInventoryDebt(ctx, s.db, inventoryDebt)
	}
	if err != nil {
		return err
	}
	document, include, err := projectProduct(product, s.now())
	if err != nil {
		return err
	}
	if !include {
		if err := s.backend.Delete(ctx, ProductEntityType, productID, revision); err != nil {
			return err
		}
		return acknowledgeInventoryDebt(ctx, s.db, inventoryDebt)
	}
	document.Version = revision
	if err := s.backend.Upsert(ctx, document); err != nil {
		return err
	}
	return acknowledgeInventoryDebt(ctx, s.db, inventoryDebt)
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
	available := len(product.Variants) == 0 && product.Stock > 0
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
	if len(variants) > 0 {
		product.Stock = 0
		for _, variant := range variants {
			product.Stock += variant.Stock
		}
	}
	if product.Related == nil {
		product.Related = []models.Product{}
	}
	if product.Categories == nil {
		product.Categories = []models.Category{}
	}
	brandSlug := ""
	if product.Brand != nil && product.Brand.IsActive {
		brandSlug = NormalizeQuery(product.Brand.Slug)
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
		MinPrice: minPrice, MaxPrice: maxPrice, Available: available, MarginRate: productMarginRate(product),
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
	db            *gorm.DB
	now           func() time.Time
	configuration *configurationCache
}

func (b *databaseBackend) ReplaceAll(ctx context.Context, documents []models.SearchDocument, completed time.Time) error {
	return b.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, document := range documents {
			if _, err := upsertDocument(tx, document); err != nil {
				return err
			}
		}
		// Count reconciliation is reserved for rebuilds. Fence incremental
		// deltas after every document lock has been taken, so a concurrent
		// uncommitted document contributes its delta after this baseline.
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.SearchIndexState{Name: ProductIndexName}).Error; err != nil {
			return err
		}
		var state models.SearchIndexState
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&state, "name = ?", ProductIndexName).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&models.SearchDocument{}).Where("entity_type = ? AND active = ?", ProductEntityType, true).Count(&count).Error; err != nil {
			return err
		}

		for _, document := range documents {
			if err := acknowledgeProjection(tx, document.EntityID, document.Version); err != nil {
				return err
			}
		}
		finished := b.now()
		return upsertIndexState(tx, finished, &finished, &completed, count)
	})
}

func (b *databaseBackend) Upsert(ctx context.Context, document models.SearchDocument) error {
	now := b.now()
	document.IndexedAt = now
	return b.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		delta, err := upsertDocument(tx, document)
		if err != nil {
			return err
		}

		if err := acknowledgeProjection(tx, document.EntityID, document.Version); err != nil {
			return err
		}
		return incrementIndexState(tx, now, delta)
	})
}

func (b *databaseBackend) Delete(ctx context.Context, entityType string, entityID uint, version int64) error {
	now := b.now()
	return b.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		document := models.SearchDocument{EntityType: entityType, EntityID: entityID, PayloadJSON: "{}", SearchableText: "", NormalizedName: "", CategoryTokens: "|", AttributeTokens: "|", Version: version, SourceCreatedAt: now, SourceUpdatedAt: now, IndexedAt: now, Active: false}
		delta, err := upsertDocument(tx, document)
		if err != nil {
			return err
		}

		if err := acknowledgeProjection(tx, document.EntityID, document.Version); err != nil {
			return err
		}
		return incrementIndexState(tx, now, delta)
	})
}

func upsertDocument(tx *gorm.DB, document models.SearchDocument) (int64, error) {
	document.BrandSlug = NormalizeQuery(document.BrandSlug)
	now := document.IndexedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	values := map[string]any{
		"entity_type": document.EntityType, "entity_id": document.EntityID, "payload_json": document.PayloadJSON,
		"searchable_text": document.SearchableText, "normalized_name": document.NormalizedName, "brand_slug": document.BrandSlug,
		"category_tokens": document.CategoryTokens, "attribute_tokens": document.AttributeTokens,
		"min_price": document.MinPrice, "max_price": document.MaxPrice, "available": document.Available, "margin_rate": document.MarginRate, "active": document.Active,
		"version": document.Version, "source_created_at": document.SourceCreatedAt, "source_updated_at": document.SourceUpdatedAt,
		"indexed_at": document.IndexedAt, "created_at": now, "updated_at": now,
	}
	// Create an inactive placeholder only for new identities, then lock the
	// durable document row. This fences both concurrent inserts and activity
	// transitions without an index-wide count or an index-wide projection lock.
	placeholder := make(map[string]any, len(values))
	for key, value := range values {
		placeholder[key] = value
	}
	placeholder["active"] = false
	if err := tx.Table("search_documents").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "entity_type"}, {Name: "entity_id"}}, DoNothing: true}).Create(placeholder).Error; err != nil {
		return 0, err
	}
	var previous models.SearchDocument
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("active").Where("entity_type = ? AND entity_id = ?", document.EntityType, document.EntityID).First(&previous).Error; err != nil {
		return 0, err
	}
	result := tx.Table("search_documents").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "entity_type"}, {Name: "entity_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"payload_json", "searchable_text", "normalized_name", "brand_slug", "category_tokens", "attribute_tokens", "min_price", "max_price", "available", "margin_rate", "active", "version", "source_created_at", "source_updated_at", "indexed_at", "updated_at"}),
		Where:     clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "search_documents.version <= excluded.version"}}},
	}).Create(values)
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 || document.EntityType != ProductEntityType {
		return 0, nil
	}
	if err := searchprojection.Replace(tx, searchprojection.Input{ProductID: document.EntityID, Payload: document.PayloadJSON, Searchable: document.SearchableText, Categories: document.CategoryTokens, Created: document.SourceCreatedAt, Active: document.Active}); err != nil {
		return 0, err
	}
	delta := int64(0)
	if document.Active && !previous.Active {
		delta = 1
	} else if !document.Active && previous.Active {
		delta = -1
	}
	return delta, nil
}

func incrementIndexState(tx *gorm.DB, indexedAt time.Time, delta int64) error {
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoUpdates: clause.Assignments(map[string]any{
		"last_indexed_at": indexedAt, "document_count": gorm.Expr("search_index_states.document_count + ?", delta), "generation": gorm.Expr("search_index_states.generation + 1"), "updated_at": indexedAt,
	})}).Create(&models.SearchIndexState{Name: ProductIndexName, DocumentCount: delta, LastIndexedAt: &indexedAt, Generation: 1}).Error
}

func upsertIndexState(tx *gorm.DB, indexedAt time.Time, fullAt, sourceAt *time.Time, count int64) error {
	values := map[string]any{"last_indexed_at": indexedAt, "document_count": count, "updated_at": indexedAt, "generation": gorm.Expr("search_index_states.generation + 1")}
	if fullAt != nil {
		values["last_full_reindex_at"] = *fullAt
		values["last_full_reindex_source_at"] = sourceAt
	}
	state := models.SearchIndexState{Name: ProductIndexName, LastIndexedAt: &indexedAt, DocumentCount: count, Generation: 1}
	if fullAt != nil {
		state.LastFullReindexAt = fullAt
		state.LastFullReindexSourceAt = sourceAt
	}
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoUpdates: clause.Assignments(values)}).Create(&state).Error
}

func (b *databaseBackend) Search(ctx context.Context, filters Filters) (Result, error) {
	return b.searchProductsSQL(ctx, filters)
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
	result := SuggestionResult{Suggestions: []string{}, Corrections: []string{}}
	if normalized == "" {
		return result, nil
	}
	var names, brands []string
	prefix := normalized + "%"
	base := func() *gorm.DB {
		return b.db.WithContext(ctx).Table("search_product_ranks r").Joins("JOIN search_documents d ON d.entity_id = r.product_id AND d.entity_type = 'product' AND d.active = TRUE")
	}
	if err := base().Where("d.normalized_name LIKE ?", prefix).Distinct("r.name_label").Order("r.name_label ASC").Limit(limit).Pluck("r.name_label", &names).Error; err != nil {
		return result, fmt.Errorf("%w: %w", ErrIndexUnavailable, err)
	}
	if err := base().Where("TRIM(r.brand) LIKE ?", prefix).Distinct("r.brand_label").Order("r.brand_label ASC").Limit(limit).Pluck("r.brand_label", &brands).Error; err != nil {
		return result, fmt.Errorf("%w: %w", ErrIndexUnavailable, err)
	}
	seen := map[string]struct{}{}
	values := make([]string, 0, limit)
	for _, candidates := range [][]string{names, brands} {
		for _, candidate := range candidates {
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
		vocabulary, err := b.sqlVocabulary(ctx, normalized, profile)
		if err != nil {
			return result, err
		}
		corrected, rewrites := correctQuery(normalized, vocabulary, profile)
		if len(rewrites) > 0 {
			plan := buildQueryPlan(corrected, nil)
			count, err := b.sqlCount(ctx, sqlSelection{plan: plan})
			if err != nil {
				return result, fmt.Errorf("%w: %w", ErrIndexUnavailable, err)
			}
			if count > 0 {
				result.Corrections = []string{corrected}
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
		Where("job_type IN ? AND status IN ?", []string{JobTypeProductSync, JobTypeFullReindex, JobTypeCatalogFanout}, []string{models.JobStatusPending, models.JobStatusRunning, models.JobStatusRetryScheduled}).Count(&pending).Error; err != nil {
		return Freshness{}, err
	}
	result := Freshness{Status: "healthy", PendingJobs: pending, DocumentCount: state.DocumentCount, LastIndexedAt: state.LastIndexedAt, LastFullReindexAt: state.LastFullReindexAt}
	var debt models.SearchProductRevision
	debtErr := b.db.WithContext(ctx).Where("pending_since IS NOT NULL").Order("pending_since ASC").First(&debt).Error
	if debtErr != nil && !errors.Is(debtErr, gorm.ErrRecordNotFound) {
		return result, debtErr
	}
	if debtErr == nil {
		result.Lag = b.now().Sub(*debt.PendingSince)
	}
	var stockDebt models.SearchInventoryRevision
	stockErr := b.db.WithContext(ctx).Where("pending_since IS NOT NULL").Order("pending_since ASC").First(&stockDebt).Error
	if stockErr != nil && !errors.Is(stockErr, gorm.ErrRecordNotFound) {
		return result, stockErr
	}
	if stockErr == nil && b.now().Sub(*stockDebt.PendingSince) > result.Lag {
		result.Lag = b.now().Sub(*stockDebt.PendingSince)
	}
	var oldest models.JobQueue
	fullErr := b.db.WithContext(ctx).Where("job_type IN ? AND status IN ?", []string{JobTypeProductSync, JobTypeFullReindex, JobTypeCatalogFanout}, []string{models.JobStatusPending, models.JobStatusRunning, models.JobStatusRetryScheduled}).Order("created_at ASC").First(&oldest).Error
	if fullErr != nil && !errors.Is(fullErr, gorm.ErrRecordNotFound) {
		return result, fullErr
	}
	if fullErr == nil && b.now().Sub(oldest.CreatedAt) > result.Lag {
		result.Lag = b.now().Sub(oldest.CreatedAt)
	}
	// Only committed fanout identities captured by a successful rebuild or
	// expanded by their handler are covered; wall-clock timing is insufficient.
	var fanoutDebt models.SearchCatalogFanoutDebt
	fanoutErr := b.db.WithContext(ctx).Where("covered_at IS NULL").Order("pending_since ASC").First(&fanoutDebt).Error
	if fanoutErr != nil && !errors.Is(fanoutErr, gorm.ErrRecordNotFound) {
		return result, fanoutErr
	}
	if fanoutErr == nil && b.now().Sub(fanoutDebt.PendingSince) > result.Lag {
		result.Lag = b.now().Sub(fanoutDebt.PendingSince)
	}
	if result.Lag < 0 {
		result.Lag = 0
	}

	if state.LastFullReindexAt == nil || result.Lag > StaleFreshnessTarget {
		result.Status = "stale"
	} else if result.Lag > HealthyFreshnessTarget {
		result.Status = "degraded"
	}
	return result, nil
}
