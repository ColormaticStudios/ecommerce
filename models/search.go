package models

import "time"

// SearchDocument is the database-backed product search projection. PayloadJSON
// is the retrieval source; the remaining columns are intentionally denormalized
// lookup and ordering fields shared by SQLite and PostgreSQL.
type SearchDocument struct {
	ID              uint      `gorm:"primaryKey"`
	EntityType      string    `gorm:"not null;size:32;uniqueIndex:idx_search_documents_entity,priority:1;index"`
	EntityID        uint      `gorm:"not null;uniqueIndex:idx_search_documents_entity,priority:2;index"`
	PayloadJSON     string    `gorm:"type:text;not null"`
	SearchableText  string    `gorm:"type:text;not null"`
	NormalizedName  string    `gorm:"not null;size:255;index"`
	BrandSlug       string    `gorm:"not null;size:255;default:'';index"`
	CategoryTokens  string    `gorm:"type:text;not null;default:''"`
	AttributeTokens string    `gorm:"type:text;not null;default:''"`
	MinPrice        Money     `gorm:"type:numeric(12,2);not null;index"`
	MaxPrice        Money     `gorm:"type:numeric(12,2);not null;index"`
	Available       bool      `gorm:"not null;default:false;index"`
	Active          bool      `gorm:"not null;default:true;index"`
	Version         int64     `gorm:"not null"`
	SourceCreatedAt time.Time `gorm:"not null;index"`
	SourceUpdatedAt time.Time `gorm:"not null;index"`
	IndexedAt       time.Time `gorm:"not null;index"`
	CreatedAt       time.Time `gorm:"not null"`
	UpdatedAt       time.Time `gorm:"not null"`
}

type SearchIndexState struct {
	Name                   string     `gorm:"primaryKey;size:64"`
	LastIndexedAt          *time.Time `gorm:"index"`
	LastFullReindexStarted *time.Time
	LastFullReindexAt      *time.Time `gorm:"index"`
	DocumentCount          int64      `gorm:"not null;default:0"`
	CreatedAt              time.Time  `gorm:"not null"`
	UpdatedAt              time.Time  `gorm:"not null"`
}

type SearchSynonymSet struct {
	BaseModel
	Name      string `gorm:"not null;size:120;uniqueIndex"`
	Direction string `gorm:"not null;size:8"`
	TermsJSON string `gorm:"type:text;not null"`
	IsActive  bool   `gorm:"not null;default:true;index"`
	UpdatedBy *uint  `gorm:"index"`
}

type SearchRankingProfile struct {
	BaseModel
	Name        string `gorm:"not null;size:120;uniqueIndex"`
	WeightsJSON string `gorm:"type:text;not null"`
	IsDefault   bool   `gorm:"not null;default:false;index"`
	UpdatedBy   *uint  `gorm:"index"`
}

type SearchMerchandisingRule struct {
	BaseModel
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

type SearchQueryEvent struct {
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
	ID               uint      `gorm:"primaryKey"`
	QueryEventID     uint      `gorm:"not null;index"`
	ProductVariantID uint      `gorm:"not null;index"`
	Position         int       `gorm:"not null"`
	ClickedAt        time.Time `gorm:"not null;index"`
}
