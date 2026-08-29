package models

import "time"

type TranslationState string

const (
	TranslationStateDraft     TranslationState = "draft"
	TranslationStateReview    TranslationState = "review"
	TranslationStatePublished TranslationState = "published"
)

type TranslationReleaseStatus string

const (
	TranslationReleaseStatusDraft      TranslationReleaseStatus = "draft"
	TranslationReleaseStatusActive     TranslationReleaseStatus = "active"
	TranslationReleaseStatusSuperseded TranslationReleaseStatus = "superseded"
)

// Locale is the platform-wide locale registry. CMS and every other consumer
// resolve locale behavior through the localization service rather than owning
// a separate registry.
type Locale struct {
	BaseModel
	Code             string  `json:"code" gorm:"size:35;not null;uniqueIndex"`
	Name             string  `json:"name" gorm:"size:128;not null"`
	IsEnabled        bool    `json:"is_enabled" gorm:"not null;default:true;index"`
	IsDefault        bool    `json:"is_default" gorm:"not null;default:false;index"`
	FallbackLocaleID *uint   `json:"fallback_locale_id,omitempty" gorm:"index"`
	FallbackLocale   *Locale `json:"-" gorm:"foreignKey:FallbackLocaleID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

// LocaleMarketDefault maps one market to its configured default locale. A
// market can have only one default while one locale can serve many markets.
type LocaleMarketDefault struct {
	BaseModel
	Market   string `json:"market" gorm:"size:3;not null;uniqueIndex"`
	LocaleID uint   `json:"locale_id" gorm:"not null;index"`
	Locale   Locale `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

type TranslationKey struct {
	BaseModel
	Namespace    string `json:"namespace" gorm:"size:64;not null;uniqueIndex:idx_translation_key_name,priority:1"`
	Key          string `json:"key" gorm:"size:191;not null;uniqueIndex:idx_translation_key_name,priority:2"`
	SourceText   string `json:"source_text" gorm:"type:text;not null"`
	Description  string `json:"description" gorm:"type:text;not null;default:''"`
	OwnerDomain  string `json:"owner_domain" gorm:"size:64;not null"`
	IsDeprecated bool   `json:"is_deprecated" gorm:"not null;default:false;index"`
}

type TranslationKeyUsage struct {
	BaseModel
	TranslationKeyID uint           `json:"translation_key_id" gorm:"not null;index"`
	Route            string         `json:"route" gorm:"size:512;not null;default:''"`
	Component        string         `json:"component" gorm:"size:255;not null;default:''"`
	Description      string         `json:"description" gorm:"type:text;not null;default:''"`
	Position         int            `json:"position" gorm:"not null;default:0;index"`
	TranslationKey   TranslationKey `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

type TranslationValue struct {
	BaseModel
	TranslationKeyID uint             `json:"translation_key_id" gorm:"not null;index;uniqueIndex:idx_translation_value_version,priority:1"`
	LocaleID         uint             `json:"locale_id" gorm:"not null;index;uniqueIndex:idx_translation_value_version,priority:2"`
	Value            string           `json:"value" gorm:"type:text;not null"`
	State            TranslationState `json:"state" gorm:"size:16;not null;index"`
	Version          uint             `json:"version" gorm:"not null;uniqueIndex:idx_translation_value_version,priority:3"`
	UpdatedBy        *uint            `json:"updated_by,omitempty" gorm:"index"`
	ReviewedBy       *uint            `json:"reviewed_by,omitempty" gorm:"index"`
	AssigneeID       *uint            `json:"assignee_id,omitempty" gorm:"index"`
	SourceHash       string           `json:"source_hash" gorm:"size:64;not null;default:'';index"`
	ChangeSummary    string           `json:"change_summary" gorm:"type:text;not null;default:''"`
	TranslationKey   TranslationKey   `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Locale           Locale           `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

// LocalizedEntityValue stores locale-specific merchant-authored fields for
// catalog, storefront settings, and SEO entities. CMS content keeps its own
// versioned variant lifecycle while consuming the same locale registry.
type LocalizedEntityValue struct {
	BaseModel
	EntityType string `json:"entity_type" gorm:"size:32;not null;uniqueIndex:idx_localized_entity_field,priority:1"`
	EntityID   uint   `json:"entity_id" gorm:"not null;uniqueIndex:idx_localized_entity_field,priority:2;index"`
	LocaleID   uint   `json:"locale_id" gorm:"not null;uniqueIndex:idx_localized_entity_field,priority:3;index"`
	Field      string `json:"field" gorm:"size:64;not null;uniqueIndex:idx_localized_entity_field,priority:4"`
	Value      string `json:"value" gorm:"type:text;not null"`
	UpdatedBy  *uint  `json:"updated_by,omitempty" gorm:"index"`
	Locale     Locale `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

type TranslationComment struct {
	BaseModel
	TranslationValueID uint       `json:"translation_value_id" gorm:"not null;index"`
	AuthorID           *uint      `json:"author_id,omitempty" gorm:"index"`
	AuthorSubject      string     `json:"-" gorm:"size:255;not null"`
	AuthorName         string     `json:"author_name" gorm:"-"`
	Comment            string     `json:"comment" gorm:"type:text;not null"`
	ResolvedAt         *time.Time `json:"resolved_at,omitempty" gorm:"index"`
}

type TranslationAuditEvent struct {
	ID                 uint      `json:"id" gorm:"primaryKey"`
	TranslationKeyID   *uint     `json:"translation_key_id,omitempty" gorm:"index"`
	TranslationValueID *uint     `json:"translation_value_id,omitempty" gorm:"index"`
	ReleaseID          *uint     `json:"release_id,omitempty" gorm:"index"`
	ActorID            *uint     `json:"actor_id,omitempty" gorm:"index"`
	ActorSubject       string    `json:"actor_subject" gorm:"size:255;not null"`
	Action             string    `json:"action" gorm:"size:64;not null;index"`
	Locale             string    `json:"locale" gorm:"size:35;not null;default:'';index"`
	Namespace          string    `json:"namespace" gorm:"size:64;not null;default:'';index"`
	ChangeSummary      string    `json:"change_summary" gorm:"type:text;not null;default:''"`
	CreatedAt          time.Time `json:"created_at" gorm:"not null;index"`
}

type LocalizationGlossaryTerm struct {
	BaseModel
	LocaleID       uint   `json:"locale_id" gorm:"not null;uniqueIndex:idx_localization_glossary_term,priority:1;index"`
	SourceTerm     string `json:"source_term" gorm:"size:255;not null;uniqueIndex:idx_localization_glossary_term,priority:2"`
	TranslatedTerm string `json:"translated_term" gorm:"size:255;not null"`
	Description    string `json:"description" gorm:"type:text;not null;default:''"`
	IsLocked       bool   `json:"is_locked" gorm:"not null;default:false;index"`
	Locale         Locale `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

type LocalizationRole string

const (
	LocalizationRoleTranslator LocalizationRole = "translator"
	LocalizationRoleEditor     LocalizationRole = "editor"
	LocalizationRolePublisher  LocalizationRole = "publisher"
)

type LocalizationRoleAssignment struct {
	BaseModel
	Subject string           `json:"subject" gorm:"size:255;not null;uniqueIndex"`
	Role    LocalizationRole `json:"role" gorm:"size:32;not null;index"`
}

type TranslationRelease struct {
	BaseModel
	Name         string                   `json:"name" gorm:"size:128;not null"`
	Status       TranslationReleaseStatus `json:"status" gorm:"size:16;not null;index"`
	PublishedAt  *time.Time               `json:"published_at,omitempty" gorm:"index"`
	PublishedBy  *uint                    `json:"published_by,omitempty" gorm:"index"`
	Notes        string                   `json:"notes" gorm:"type:text;not null;default:''"`
	SnapshotHash string                   `json:"snapshot_hash" gorm:"size:64;not null;index"`
}

type TranslationReleaseEntry struct {
	BaseModel
	ReleaseID          uint               `json:"release_id" gorm:"not null;index;uniqueIndex:idx_translation_release_value,priority:1"`
	TranslationValueID uint               `json:"translation_value_id" gorm:"not null;index;uniqueIndex:idx_translation_release_value,priority:2"`
	Release            TranslationRelease `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	TranslationValue   TranslationValue   `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

// LocalizationRollout controls runtime exposure of an enabled locale within
// one product domain. The global locale registry remains the source of truth
// for whether a locale can be used at all.
type LocalizationRollout struct {
	BaseModel
	LocaleID   uint   `json:"locale_id" gorm:"not null;index;uniqueIndex:idx_localization_rollout,priority:1"`
	Domain     string `json:"domain" gorm:"size:32;not null;index;uniqueIndex:idx_localization_rollout,priority:2"`
	IsEnabled  bool   `json:"is_enabled" gorm:"not null;default:true;index"`
	Percentage int    `json:"percentage" gorm:"not null;default:100"`
	UpdatedBy  *uint  `json:"updated_by,omitempty" gorm:"index"`
	Locale     Locale `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// LocalizationMetric stores low-cardinality operational counters. Count and
// value aggregates are updated atomically so runtime reads do not create an
// unbounded event table.
type LocalizationMetric struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	MetricType   string    `json:"metric_type" gorm:"size:32;not null;uniqueIndex:idx_localization_metric,priority:1"`
	Locale       string    `json:"locale" gorm:"size:35;not null;default:'';uniqueIndex:idx_localization_metric,priority:2;index"`
	Domain       string    `json:"domain" gorm:"size:32;not null;default:'';uniqueIndex:idx_localization_metric,priority:3;index"`
	Key          string    `json:"key" gorm:"size:255;not null;default:'';uniqueIndex:idx_localization_metric,priority:4"`
	Count        int64     `json:"count" gorm:"not null;default:0"`
	TotalValue   int64     `json:"total_value" gorm:"not null;default:0"`
	MaximumValue int64     `json:"maximum_value" gorm:"not null;default:0"`
	LastSeenAt   time.Time `json:"last_seen_at" gorm:"not null;index"`
	CreatedAt    time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"not null"`
}
