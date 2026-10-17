package models

import "time"

// SearchDocument is the database-backed product search projection. PayloadJSON
// is the retrieval source; the remaining columns are intentionally denormalized
// lookup and ordering fields shared by SQLite and PostgreSQL.
type SearchDocument struct {
	MarginRate      float64   `gorm:"not null;default:0" json:"-"`
	ID              uint      `gorm:"primaryKey"`
	EntityType      string    `gorm:"not null;size:32;uniqueIndex:idx_search_documents_entity,priority:1;index;index:idx_search_documents_eligibility,priority:1"`
	EntityID        uint      `gorm:"not null;uniqueIndex:idx_search_documents_entity,priority:2;index;index:idx_search_documents_eligibility,priority:3"`
	PayloadJSON     string    `gorm:"type:text;not null"`
	SearchableText  string    `gorm:"type:text;not null"`
	NormalizedName  string    `gorm:"not null;size:255;index"`
	BrandSlug       string    `gorm:"not null;size:255;default:'';index"`
	CategoryTokens  string    `gorm:"type:text;not null;default:''"`
	AttributeTokens string    `gorm:"type:text;not null;default:''"`
	MinPrice        Money     `gorm:"type:numeric(12,2);not null;index"`
	MaxPrice        Money     `gorm:"type:numeric(12,2);not null;index"`
	Available       bool      `gorm:"not null;default:false;index"`
	Active          bool      `gorm:"not null;default:true;index;index:idx_search_documents_eligibility,priority:2"`
	Version         int64     `gorm:"not null"`
	SourceCreatedAt time.Time `gorm:"not null;index"`
	SourceUpdatedAt time.Time `gorm:"not null;index"`
	IndexedAt       time.Time `gorm:"not null;index"`
	CreatedAt       time.Time `gorm:"not null"`
	UpdatedAt       time.Time `gorm:"not null"`
}

type SearchIndexState struct {
	LastConversionRefreshAt *time.Time `json:"-"`
	Generation              uint64     `gorm:"not null;default:0"`
	Name                    string     `gorm:"primaryKey;size:64"`
	LastIndexedAt           *time.Time `gorm:"index"`
	LastFullReindexStarted  *time.Time
	LastFullReindexAt       *time.Time `gorm:"index"`
	LastFullReindexSourceAt *time.Time
	DocumentCount           int64     `gorm:"not null;default:0"`
	CreatedAt               time.Time `gorm:"not null"`
	UpdatedAt               time.Time `gorm:"not null"`
}

type SearchSynonymSet struct {
	BaseModel
	Name      string `gorm:"not null;size:120;uniqueIndex"`
	Direction string `gorm:"not null;size:8"`
	TermsJSON string `gorm:"type:text;not null"`
	IsActive  bool   `gorm:"not null;default:true;index"`
	UpdatedBy *uint  `gorm:"index"`
}

// SearchTypoToleranceProfile is a named query-correction policy. Exactly one
// non-deleted profile is active globally; internal/search owns that invariant.
type SearchTypoToleranceProfile struct {
	BaseModel
	Name                 string `gorm:"not null;size:120;uniqueIndex"`
	MinimumTokenLength   int    `gorm:"not null"`
	OneEditMinimumLength int    `gorm:"not null"`
	TwoEditMinimumLength int    `gorm:"not null"`
	StrictMode           bool   `gorm:"not null;default:false"`
	IsActive             bool   `gorm:"not null;default:false;index"`
	UpdatedBy            *uint  `gorm:"index"`
}

type SearchRankingProfile struct {
	BaseModel
	Name        string `gorm:"not null;size:120;uniqueIndex"`
	WeightsJSON string `gorm:"type:text;not null"`
	Version     int    `gorm:"not null;default:1"`
	IsDefault   bool   `gorm:"not null;default:false;index"`
	UpdatedBy   *uint  `gorm:"index"`
}

type SearchMerchandisingRule struct {
	BaseModel
	Version       int        `gorm:"not null;default:1"`
	Name          string     `gorm:"not null;size:120"`
	RuleType      string     `gorm:"not null;size:32;index"`
	PredicateJSON string     `gorm:"type:text;not null"`
	ActionJSON    string     `gorm:"type:text;not null"`
	Priority      int        `gorm:"not null;default:0;index"`
	StartsAt      *time.Time `gorm:"index"`
	EndsAt        *time.Time `gorm:"index"`
	IsActive      bool       `gorm:"not null;default:true;index"`
	UpdatedBy     *uint      `gorm:"index"`
}

// SearchMerchandisingAudit retains immutable configuration snapshots after a
// rule is physically deleted. IDs are historical references without foreign keys.
type SearchMerchandisingAudit struct {
	ID         uint      `gorm:"primaryKey"`
	RuleID     uint      `gorm:"not null;index"`
	Operation  string    `gorm:"not null;size:16"`
	ActorID    *uint     `gorm:"index"`
	BeforeJSON *string   `gorm:"type:text"`
	AfterJSON  *string   `gorm:"type:text"`
	CreatedAt  time.Time `gorm:"not null;index"`
}

type SearchQueryEvent struct {
	SnapshotID      string    `gorm:"size:64;uniqueIndex:idx_search_session_snapshot,priority:2"`
	FirstPosition   int       `gorm:"not null;default:1"`
	ProductCount    int       `gorm:"not null;default:0"`
	FacetMask       int       `gorm:"not null;default:0"`
	ImpressionID    string    `gorm:"size:64;uniqueIndex"`
	SessionHash     string    `gorm:"size:64;index;uniqueIndex:idx_search_session_snapshot,priority:1"`
	ProductsJSON    string    `gorm:"type:text"`
	ID              uint      `gorm:"primaryKey"`
	Query           string    `gorm:"type:text;not null"`
	NormalizedQuery string    `gorm:"type:text;not null;index"`
	FiltersJSON     string    `gorm:"type:text;not null"`
	ResultCount     int       `gorm:"not null"`
	LatencyMs       int64     `gorm:"not null"`
	SessionID       string    `gorm:"not null;size:128;default:'';index"`
	UserID          *uint     `gorm:"index"`
	CreatedAt       time.Time `gorm:"not null;index"`
}

type SearchClickEvent struct {
	EventID          string    `gorm:"size:64;uniqueIndex"`
	ImpressionID     string    `gorm:"size:64;index"`
	SessionHash      string    `gorm:"size:64;index"`
	ProductID        uint      `gorm:"index"`
	ID               uint      `gorm:"primaryKey"`
	QueryEventID     uint      `gorm:"not null;index"`
	ProductVariantID uint      `gorm:"not null;index"`
	Position         int       `gorm:"not null"`
	ClickedAt        time.Time `gorm:"not null;index"`
}

// SearchSalesSignal is the daily projection of qualifying sold units.
type SearchSalesSignal struct {
	ProductID   uint      `gorm:"primaryKey;autoIncrement:false"`
	Units30Days int64     `gorm:"not null;default:0"`
	AsOf        time.Time `gorm:"not null;index"`
	UpdatedAt   time.Time `gorm:"not null"`
}

type SearchCartAttribution struct {
	CartItemID   uint      `gorm:"primaryKey;autoIncrement:false"`
	ClickID      string    `gorm:"not null;size:64;index"`
	ImpressionID string    `gorm:"not null;size:64;index"`
	ProductID    uint      `gorm:"not null"`
	CreatedAt    time.Time `gorm:"not null;index"`
}
type SearchOrderAttribution struct {
	ID           uint       `gorm:"primaryKey"`
	OrderID      uint       `gorm:"not null;uniqueIndex:idx_search_order_product,priority:1;index"`
	ProductID    uint       `gorm:"not null;uniqueIndex:idx_search_order_product,priority:2"`
	ImpressionID string     `gorm:"not null;size:64;index"`
	ClickID      string     `gorm:"not null;size:64"`
	CreatedAt    time.Time  `gorm:"not null;index"`
	PaidAt       *time.Time `gorm:"index"`
}

type SearchRevokedSession struct {
	SessionHash string    `gorm:"primaryKey;size:64"`
	CreatedAt   time.Time `gorm:"not null;index"`
}

// SearchConversionSignal is a consent-aware mature exposure cohort snapshot.
type SearchConversionSignal struct {
	ProductID         uint      `gorm:"primaryKey;autoIncrement:false"`
	Impressions30Days int64     `gorm:"not null;default:0"`
	Conversions30Days int64     `gorm:"not null;default:0"`
	AsOf              time.Time `gorm:"not null;index"`
	UpdatedAt         time.Time `gorm:"not null"`
}

// SearchProductRevision records projection debt independently of job status.
// PendingSince survives failed delivery until a successful covered projection.
type SearchProductRevision struct {
	ProductID       uint       `gorm:"primaryKey;autoIncrement:false"`
	Revision        int64      `gorm:"not null"`
	IndexedRevision int64      `gorm:"not null;default:0"`
	ChangedAt       time.Time  `gorm:"not null"`
	PendingSince    *time.Time `gorm:"index"`
}

// SearchInventoryRevision records stock projection debt at the variant boundary,
// avoiding a shared product revision lock in unrelated variant checkouts.
type SearchInventoryRevision struct {
	VariantID    uint       `gorm:"primaryKey;autoIncrement:false"`
	ProductID    uint       `gorm:"not null;index"`
	Revision     int64      `gorm:"not null"`
	ChangedAt    time.Time  `gorm:"not null"`
	PendingSince *time.Time `gorm:"index"`
}

// SearchCatalogFanoutDebt is committed alongside the shared source mutation.
// Rebuild coverage names exact committed job identities, never wall-clock time.
type SearchCatalogFanoutDebt struct {
	JobID         string     `gorm:"primaryKey;size:36"`
	PendingSince  time.Time  `gorm:"not null;index"`
	CoveredAt     *time.Time `gorm:"index"`
	LastProductID uint       `gorm:"not null;default:0"`
}

// SearchConfigurationAudit is an immutable snapshot of a configuration change,
// including changes to active/default ownership and deleted configurations.
type SearchConfigurationAudit struct {
	ID         uint   `gorm:"primaryKey"`
	Kind       string `gorm:"size:32;not null;index:idx_search_configuration_history,priority:1"`
	EntityID   uint   `gorm:"not null;index:idx_search_configuration_history,priority:2"`
	Operation  string `gorm:"size:16;not null"`
	ActorID    *uint
	BeforeJSON *string   `gorm:"type:text"`
	AfterJSON  *string   `gorm:"type:text"`
	CreatedAt  time.Time `gorm:"not null;index"`
}
