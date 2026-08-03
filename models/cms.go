package models

import (
	"time"
)

type CMSEntryStatus string

const (
	CMSEntryStatusDraft     CMSEntryStatus = "DRAFT"
	CMSEntryStatusScheduled CMSEntryStatus = "SCHEDULED"
	CMSEntryStatusPublished CMSEntryStatus = "PUBLISHED"
	CMSEntryStatusArchived  CMSEntryStatus = "ARCHIVED"
)

type CMSEntryType string

const (
	CMSEntryTypePage       CMSEntryType = "page"
	CMSEntryTypeLayout     CMSEntryType = "layout"
	CMSEntryTypeGlobal     CMSEntryType = "global"
	CMSEntryTypeNavigation CMSEntryType = "navigation"
	CMSEntryTypeTemplate   CMSEntryType = "template"
)

type CMSEntry struct {
	BaseModel
	EntryType          CMSEntryType      `json:"entry_type" gorm:"size:32;not null;index:idx_cms_entries_type_key,priority:1"`
	Key                string            `json:"key" gorm:"size:255;not null;index:idx_cms_entries_type_key,priority:2"`
	Status             CMSEntryStatus    `json:"status" gorm:"size:32;not null;default:DRAFT;index"`
	CurrentVersionID   *uint             `json:"current_version_id,omitempty" gorm:"index"`
	PublishedVersionID *uint             `json:"published_version_id,omitempty" gorm:"index"`
	CurrentVersion     *CMSEntryVersion  `gorm:"-"`
	PublishedVersion   *CMSEntryVersion  `gorm:"-"`
	Versions           []CMSEntryVersion `gorm:"-"`
}

type CMSEntryVersion struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	EntryID       uint      `json:"entry_id" gorm:"not null;index:idx_cms_versions_entry_number,priority:1"`
	VersionNumber uint      `json:"version_number" gorm:"not null;index:idx_cms_versions_entry_number,priority:2"`
	SchemaVersion uint      `json:"schema_version" gorm:"not null;default:1"`
	PayloadJSON   string    `json:"payload_json" gorm:"type:jsonb;not null"`
	CreatedBy     *uint     `json:"created_by,omitempty" gorm:"index"`
	ChangeSummary string    `json:"change_summary" gorm:"type:text;not null;default:''"`
	CreatedAt     time.Time `json:"created_at"`
	Entry         CMSEntry  `gorm:"-"`
}

type CMSPublication struct {
	ID                        uint            `json:"id" gorm:"primaryKey"`
	EntryID                   uint            `json:"entry_id" gorm:"not null;index"`
	VersionID                 uint            `json:"version_id" gorm:"not null;index"`
	PublishedBy               *uint           `json:"published_by,omitempty" gorm:"index"`
	PublishedAt               time.Time       `json:"published_at" gorm:"not null;index"`
	RollbackFromPublicationID *uint           `json:"rollback_from_publication_id,omitempty" gorm:"index"`
	Notes                     string          `json:"notes" gorm:"type:text;not null;default:''"`
	Entry                     CMSEntry        `gorm:"-"`
	Version                   CMSEntryVersion `gorm:"-"`
}

type CMSPageVisibility string

const (
	CMSPageVisibilityPublic CMSPageVisibility = "public"
	CMSPageVisibilityHidden CMSPageVisibility = "hidden"
)

type CMSPage struct {
	BaseModel
	EntryID       uint              `json:"entry_id" gorm:"not null;uniqueIndex"`
	Path          string            `json:"path" gorm:"size:255;not null;index"`
	Slug          string            `json:"slug" gorm:"size:255;not null;index"`
	Title         string            `json:"title" gorm:"size:255;not null"`
	TemplateKey   string            `json:"template_key" gorm:"size:128;not null;default:default"`
	Visibility    CMSPageVisibility `json:"visibility" gorm:"size:32;not null;default:public;index"`
	SEOMetadataID *uint             `json:"seo_metadata_id,omitempty" gorm:"index"`
	IsHomepage    bool              `json:"is_homepage" gorm:"not null;default:false;index"`
	Entry         CMSEntry          `gorm:"-"`
}

type CMSNavigationMenu struct {
	BaseModel
	EntryID  uint     `json:"entry_id" gorm:"not null;uniqueIndex"`
	Key      string   `json:"key" gorm:"size:128;not null;uniqueIndex"`
	Title    string   `json:"title" gorm:"size:255;not null"`
	Location string   `json:"location" gorm:"size:64;not null;index"`
	Entry    CMSEntry `gorm:"-"`
}

type CMSNavigationItemType string

const (
	CMSNavigationItemTypeInternal CMSNavigationItemType = "internal"
	CMSNavigationItemTypeExternal CMSNavigationItemType = "external"
	CMSNavigationItemTypeCategory CMSNavigationItemType = "category"
	CMSNavigationItemTypeProduct  CMSNavigationItemType = "product"
	CMSNavigationItemTypePage     CMSNavigationItemType = "page"
	CMSNavigationItemTypeDropdown CMSNavigationItemType = "dropdown"
)

type CMSNavigationItem struct {
	BaseModel
	MenuID    uint                  `json:"menu_id" gorm:"not null;index"`
	ParentID  *uint                 `json:"parent_id,omitempty" gorm:"index"`
	Label     string                `json:"label" gorm:"size:255;not null"`
	ItemType  CMSNavigationItemType `json:"item_type" gorm:"size:32;not null"`
	TargetRef string                `json:"target_ref" gorm:"size:255;not null;default:''"`
	URL       string                `json:"url" gorm:"size:1024;not null;default:''"`
	SortOrder int                   `json:"sort_order" gorm:"not null;default:0;index"`
	IsEnabled bool                  `json:"is_enabled" gorm:"not null;default:true;index"`
	Menu      CMSNavigationMenu     `gorm:"-"`
}

type CMSGlobalRegion struct {
	BaseModel
	EntryID uint     `json:"entry_id" gorm:"not null;uniqueIndex"`
	Key     string   `json:"key" gorm:"size:128;not null;uniqueIndex"`
	Title   string   `json:"title" gorm:"size:255;not null"`
	Region  string   `json:"region" gorm:"size:64;not null;index"`
	Entry   CMSEntry `gorm:"-"`
}

type CMSScheduleStatus string

const (
	CMSScheduleStatusPending   CMSScheduleStatus = "pending"
	CMSScheduleStatusActive    CMSScheduleStatus = "active"
	CMSScheduleStatusCompleted CMSScheduleStatus = "completed"
	CMSScheduleStatusCancelled CMSScheduleStatus = "cancelled"
)

type CMSSchedule struct {
	BaseModel
	EntryID          uint              `json:"entry_id" gorm:"not null;uniqueIndex"`
	VersionID        uint              `json:"version_id" gorm:"not null;index"`
	PublishAt        time.Time         `json:"publish_at" gorm:"not null;index"`
	UnpublishAt      *time.Time        `json:"unpublish_at,omitempty" gorm:"index"`
	Timezone         string            `json:"timezone" gorm:"size:64;not null;default:UTC"`
	Status           CMSScheduleStatus `json:"status" gorm:"size:32;not null;index"`
	LastTransitionAt *time.Time        `json:"last_transition_at,omitempty"`
}

type CMSTargetingRule struct {
	BaseModel
	EntryID   uint   `json:"entry_id" gorm:"not null;index"`
	VersionID uint   `json:"version_id" gorm:"not null;index"`
	RuleJSON  string `json:"rule_json" gorm:"type:jsonb;not null"`
	Priority  int    `json:"priority" gorm:"not null;default:0;index"`
	IsEnabled bool   `json:"is_enabled" gorm:"not null;default:true;index"`
}

type CMSExperimentStatus string

const (
	CMSExperimentStatusDraft     CMSExperimentStatus = "draft"
	CMSExperimentStatusActive    CMSExperimentStatus = "active"
	CMSExperimentStatusPaused    CMSExperimentStatus = "paused"
	CMSExperimentStatusCompleted CMSExperimentStatus = "completed"
)

type CMSExperiment struct {
	BaseModel
	EntryID   uint                   `json:"entry_id" gorm:"not null;index"`
	Name      string                 `json:"name" gorm:"size:255;not null"`
	Status    CMSExperimentStatus    `json:"status" gorm:"size:32;not null;index"`
	StickyKey string                 `json:"sticky_key" gorm:"size:32;not null"`
	StartsAt  time.Time              `json:"starts_at" gorm:"not null;index"`
	EndsAt    *time.Time             `json:"ends_at,omitempty" gorm:"index"`
	Variants  []CMSExperimentVariant `json:"variants" gorm:"-"`
}

type CMSExperimentVariant struct {
	ID           uint   `json:"id" gorm:"primaryKey"`
	ExperimentID uint   `json:"experiment_id" gorm:"not null;index"`
	Name         string `json:"name" gorm:"size:128;not null"`
	VersionID    uint   `json:"version_id" gorm:"not null;index"`
	Allocation   int    `json:"allocation" gorm:"not null"`
}

type CMSExposureEvent struct {
	ID                  uint      `json:"id" gorm:"primaryKey"`
	EntryID             uint      `json:"entry_id" gorm:"not null;index"`
	ContentVersionID    uint      `json:"content_version_id" gorm:"not null;index"`
	ExperimentID        *uint     `json:"experiment_id,omitempty" gorm:"index"`
	ExperimentVariantID *uint     `json:"experiment_variant_id,omitempty" gorm:"index"`
	CorrelationID       string    `json:"correlation_id" gorm:"size:128;not null;index"`
	AssignmentHash      string    `json:"assignment_hash" gorm:"size:64;not null;default:''"`
	EventType           string    `json:"event_type" gorm:"size:32;not null;index"`
	CreatedAt           time.Time `json:"created_at" gorm:"not null;index"`
}

type CMSRedirectRule struct {
	BaseModel
	SourcePattern string `json:"source_pattern" gorm:"size:1024;not null;index"`
	MatchType     string `json:"match_type" gorm:"size:16;not null;index"`
	TargetURL     string `json:"target_url" gorm:"size:2048;not null"`
	RedirectType  int    `json:"redirect_type" gorm:"not null"`
	Priority      int    `json:"priority" gorm:"not null;default:0;index"`
	IsEnabled     bool   `json:"is_enabled" gorm:"not null;default:true;index"`
}

type CMSLocale struct {
	BaseModel
	Code           string `json:"code" gorm:"size:35;not null;uniqueIndex"`
	Name           string `json:"name" gorm:"size:128;not null"`
	Enabled        bool   `json:"enabled" gorm:"not null;default:true;index"`
	IsDefault      bool   `json:"is_default" gorm:"not null;default:false;index"`
	FallbackLocale string `json:"fallback_locale" gorm:"size:35;not null;default:''"`
}

type CMSVariantStatus string

const (
	CMSVariantStatusDraft            CMSVariantStatus = "draft"
	CMSVariantStatusInReview         CMSVariantStatus = "in_review"
	CMSVariantStatusChangesRequested CMSVariantStatus = "changes_requested"
	CMSVariantStatusApproved         CMSVariantStatus = "approved"
	CMSVariantStatusPublished        CMSVariantStatus = "published"
)

type CMSPageVariant struct {
	BaseModel
	PageID               uint             `json:"page_id" gorm:"not null;index:idx_cms_page_variant_scope,priority:1"`
	EntryID              uint             `json:"entry_id" gorm:"not null;index"`
	Locale               string           `json:"locale" gorm:"size:35;not null;index:idx_cms_page_variant_scope,priority:2"`
	Market               string           `json:"market" gorm:"size:16;not null;default:'';index:idx_cms_page_variant_scope,priority:3"`
	Path                 string           `json:"path" gorm:"size:255;not null;index"`
	Slug                 string           `json:"slug" gorm:"size:255;not null"`
	Title                string           `json:"title" gorm:"size:255;not null"`
	DraftPayloadJSON     string           `json:"-" gorm:"type:jsonb;not null"`
	PublishedPayloadJSON string           `json:"-" gorm:"type:jsonb;not null;default:'{}'"`
	Status               CMSVariantStatus `json:"status" gorm:"size:32;not null;default:draft;index"`
	Revision             uint             `json:"revision" gorm:"not null;default:1"`
	SubmittedBy          string           `json:"submitted_by" gorm:"size:255;not null;default:''"`
	ApprovedBy           string           `json:"approved_by" gorm:"size:255;not null;default:''"`
	PublishedAt          *time.Time       `json:"published_at,omitempty" gorm:"index"`
}

type CMSAuditEvent struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	EntryID   uint      `json:"entry_id" gorm:"not null;index"`
	VersionID *uint     `json:"version_id,omitempty" gorm:"index"`
	VariantID *uint     `json:"variant_id,omitempty" gorm:"index"`
	Action    string    `json:"action" gorm:"size:64;not null;index"`
	Actor     string    `json:"actor" gorm:"size:255;not null"`
	Detail    string    `json:"detail" gorm:"type:text;not null;default:''"`
	CreatedAt time.Time `json:"created_at" gorm:"not null;index"`
}

type CMSChangeComment struct {
	ID         uint       `json:"id" gorm:"primaryKey"`
	EntryID    uint       `json:"entry_id" gorm:"not null;index"`
	VariantID  *uint      `json:"variant_id,omitempty" gorm:"index"`
	Actor      string     `json:"actor" gorm:"size:255;not null"`
	Body       string     `json:"body" gorm:"type:text;not null"`
	ResolvedBy string     `json:"resolved_by" gorm:"size:255;not null;default:''"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty" gorm:"index"`
	CreatedAt  time.Time  `json:"created_at" gorm:"not null;index"`
}

type CMSWorkflowStatus string

const (
	CMSWorkflowStatusDraft            CMSWorkflowStatus = "draft"
	CMSWorkflowStatusInReview         CMSWorkflowStatus = "in_review"
	CMSWorkflowStatusChangesRequested CMSWorkflowStatus = "changes_requested"
	CMSWorkflowStatusApproved         CMSWorkflowStatus = "approved"
)

type CMSEntryWorkflow struct {
	BaseModel
	EntryID     uint              `json:"entry_id" gorm:"not null;uniqueIndex"`
	VersionID   uint              `json:"version_id" gorm:"not null;index"`
	Status      CMSWorkflowStatus `json:"status" gorm:"size:32;not null;default:draft;index"`
	SubmittedBy string            `json:"submitted_by" gorm:"size:255;not null;default:''"`
	ApprovedBy  string            `json:"approved_by" gorm:"size:255;not null;default:''"`
}

type CMSContentVariant struct {
	BaseModel
	EntryID              uint             `json:"entry_id" gorm:"not null;index:idx_cms_content_variant_scope,priority:1"`
	Locale               string           `json:"locale" gorm:"size:35;not null;index:idx_cms_content_variant_scope,priority:2"`
	Market               string           `json:"market" gorm:"size:16;not null;default:'';index:idx_cms_content_variant_scope,priority:3"`
	DraftPayloadJSON     string           `json:"-" gorm:"type:jsonb;not null"`
	PublishedPayloadJSON string           `json:"-" gorm:"type:jsonb;not null;default:'{}'"`
	Status               CMSVariantStatus `json:"status" gorm:"size:32;not null;default:draft;index"`
	Revision             uint             `json:"revision" gorm:"not null;default:1"`
	SubmittedBy          string           `json:"submitted_by" gorm:"size:255;not null;default:''"`
	ApprovedBy           string           `json:"approved_by" gorm:"size:255;not null;default:''"`
	PublishedAt          *time.Time       `json:"published_at,omitempty" gorm:"index"`
}

type CMSSettings struct {
	ID                     uint      `json:"id" gorm:"primaryKey"`
	ApprovalRequired       bool      `json:"approval_required" gorm:"not null;default:true"`
	InvalidationWebhookURL string    `json:"invalidation_webhook_url" gorm:"size:2048;not null;default:''"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type CMSRoleAssignment struct {
	BaseModel
	Subject string `json:"subject" gorm:"size:255;not null;uniqueIndex"`
	Role    string `json:"role" gorm:"size:32;not null;index"`
}

type CMSInvalidationEvent struct {
	ID        uint       `json:"id" gorm:"primaryKey"`
	EntryID   uint       `json:"entry_id" gorm:"not null;index"`
	VariantID *uint      `json:"variant_id,omitempty" gorm:"index"`
	Reason    string     `json:"reason" gorm:"size:64;not null"`
	Status    string     `json:"status" gorm:"size:32;not null;default:pending;index"`
	Attempts  int        `json:"attempts" gorm:"not null;default:0"`
	LastError string     `json:"last_error" gorm:"type:text;not null;default:''"`
	CreatedAt time.Time  `json:"created_at" gorm:"not null;index"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
}
