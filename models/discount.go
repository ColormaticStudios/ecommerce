package models

import "time"

const (
	DiscountCampaignTypeProductDiscount = "product_discount"
	DiscountCampaignTypePromotion       = "promotion"

	DiscountCampaignStatusActive    = "active"
	DiscountCampaignStatusScheduled = "scheduled"
	DiscountCampaignStatusDisabled  = "disabled"
	DiscountCampaignStatusArchived  = "archived"

	DiscountModePercent = "percent"
	DiscountModeFixed   = "fixed"

	DiscountTargetTypeProduct  = "product"
	DiscountTargetTypeVariant  = "variant"
	DiscountTargetTypeCategory = "category"
	DiscountTargetTypeBrand    = "brand"

	DiscountScheduleTypeOneTime   = "one_time"
	DiscountScheduleTypeRecurring = "recurring"

	DiscountRecurrenceDaily   = "daily"
	DiscountRecurrenceWeekly  = "weekly"
	DiscountRecurrenceMonthly = "monthly"

	DiscountChannelWeb   = "web"
	DiscountChannelApp   = "app"
	DiscountChannelAdmin = "admin"
)

type DiscountCampaign struct {
	BaseModel
	Name                string           `json:"name" gorm:"not null"`
	Type                string           `json:"type" gorm:"not null;index"`
	Status              string           `json:"status" gorm:"not null;index"`
	StartsAt            time.Time        `json:"starts_at" gorm:"not null;index"`
	EndsAt              *time.Time       `json:"ends_at" gorm:"index"`
	Timezone            string           `json:"timezone" gorm:"not null;default:'UTC'"`
	IsArchived          bool             `json:"is_archived" gorm:"not null;default:false;index"`
	Priority            int              `json:"priority" gorm:"not null;default:0;index"`
	IsExclusive         bool             `json:"is_exclusive" gorm:"not null;default:false"`
	DiscountMode        string           `json:"discount_mode" gorm:"not null"`
	DiscountValue       Money            `json:"discount_value" gorm:"type:numeric(12,2);not null"`
	MetadataJSON        string           `json:"metadata_json" gorm:"type:text;not null;default:'{}'"`
	CouponCode          *string          `json:"coupon_code,omitempty" gorm:"uniqueIndex"`
	ChannelsJSON        string           `json:"channels_json" gorm:"type:text;not null;default:'[]'"`
	CustomerSegment     string           `json:"customer_segment" gorm:"not null;default:''"`
	GlobalUsageCap      *int             `json:"global_usage_cap"`
	PerCustomerUsageCap *int             `json:"per_customer_usage_cap"`
	CreatedBy           *uint            `json:"created_by" gorm:"index"`
	UpdatedBy           *uint            `json:"updated_by" gorm:"index"`
	Targets             []DiscountTarget `json:"targets,omitempty" gorm:"foreignKey:CampaignID"`
	Rules               []DiscountRule   `json:"rules,omitempty" gorm:"foreignKey:CampaignID"`
	Levels              []DiscountLevel  `json:"levels,omitempty" gorm:"foreignKey:CampaignID"`
}

type DiscountRedemption struct {
	BaseModel
	CampaignID             uint              `json:"campaign_id" gorm:"not null;index:idx_discount_redemptions_campaign_customer;uniqueIndex:idx_discount_redemptions_campaign_order"`
	Campaign               *DiscountCampaign `json:"-" gorm:"foreignKey:CampaignID"`
	LevelID                *uint             `json:"level_id" gorm:"index"`
	OrderID                uint              `json:"order_id" gorm:"not null;index;uniqueIndex:idx_discount_redemptions_campaign_order"`
	CustomerID             *uint             `json:"customer_id" gorm:"index:idx_discount_redemptions_campaign_customer"`
	AppliedAmount          Money             `json:"applied_amount" gorm:"type:numeric(12,2);not null"`
	AppliedAt              time.Time         `json:"applied_at" gorm:"not null;index"`
	EvaluationSnapshotHash string            `json:"evaluation_snapshot_hash" gorm:"not null;default:''"`
}

type PromotionTemplate struct {
	BaseModel
	Name         string `json:"name" gorm:"not null"`
	Description  string `json:"description" gorm:"not null;default:''"`
	TemplateJSON string `json:"template_json" gorm:"type:text;not null"`
	IsActive     bool   `json:"is_active" gorm:"not null;default:true;index"`
}

type DiscountTarget struct {
	BaseModel
	CampaignID uint              `json:"campaign_id" gorm:"not null;index:idx_discount_target,unique"`
	Campaign   *DiscountCampaign `json:"-" gorm:"foreignKey:CampaignID"`
	LevelID    *uint             `json:"level_id" gorm:"index;index:idx_discount_target,unique"`
	TargetType string            `json:"target_type" gorm:"not null;index:idx_discount_target,unique"`
	TargetID   uint              `json:"target_id" gorm:"not null;index:idx_discount_target,unique"`
}

type DiscountRule struct {
	BaseModel
	CampaignID              uint              `json:"campaign_id" gorm:"not null;index"`
	Campaign                *DiscountCampaign `json:"-" gorm:"foreignKey:CampaignID"`
	ConditionJSON           string            `json:"condition_json" gorm:"type:text;not null;default:'{}'"`
	ActionJSON              string            `json:"action_json" gorm:"type:text;not null;default:'{}'"`
	StackPolicy             string            `json:"stack_policy" gorm:"not null;default:'none'"`
	MaxApplicationsPerOrder *int              `json:"max_applications_per_order"`
}

type DiscountLevel struct {
	BaseModel
	CampaignID              uint              `json:"campaign_id" gorm:"not null;index"`
	Campaign                *DiscountCampaign `json:"-" gorm:"foreignKey:CampaignID"`
	Name                    string            `json:"name" gorm:"not null"`
	Priority                int               `json:"priority" gorm:"not null;default:0"`
	ActionJSON              string            `json:"action_json" gorm:"type:text;not null;default:'{}'"`
	StackPolicy             string            `json:"stack_policy" gorm:"not null;default:'none'"`
	MaxApplicationsPerOrder *int              `json:"max_applications_per_order"`
}

type DiscountSchedule struct {
	BaseModel
	CampaignID   uint              `json:"campaign_id" gorm:"not null;uniqueIndex"`
	Campaign     *DiscountCampaign `json:"-" gorm:"foreignKey:CampaignID"`
	ScheduleType string            `json:"schedule_type" gorm:"not null;index"`
	RRule        string            `json:"rrule" gorm:"not null;default:''"`
	WindowStart  time.Time         `json:"window_start" gorm:"not null;index"`
	WindowEnd    time.Time         `json:"window_end" gorm:"not null;index"`
	UntilAt      *time.Time        `json:"until_at" gorm:"index"`
	Timezone     string            `json:"timezone" gorm:"not null;default:'UTC'"`
	LastRunAt    *time.Time        `json:"last_run_at"`
	NextRunAt    *time.Time        `json:"next_run_at" gorm:"index"`
}

type DiscountStateHistory struct {
	BaseModel
	CampaignID uint              `json:"campaign_id" gorm:"not null;index"`
	Campaign   *DiscountCampaign `json:"-" gorm:"foreignKey:CampaignID"`
	FromStatus string            `json:"from_status" gorm:"not null;default:''"`
	ToStatus   string            `json:"to_status" gorm:"not null"`
	Reason     string            `json:"reason" gorm:"not null"`
	Source     string            `json:"source" gorm:"not null"`
	Actor      string            `json:"actor" gorm:"not null;default:''"`
	ChangedAt  time.Time         `json:"changed_at" gorm:"not null;index"`
}

type DiscountCampaignAudit struct {
	BaseModel
	CampaignID uint              `json:"campaign_id" gorm:"not null;index"`
	Campaign   *DiscountCampaign `json:"-" gorm:"foreignKey:CampaignID"`
	EventType  string            `json:"event_type" gorm:"not null;index"`
	Source     string            `json:"source" gorm:"not null"`
	Actor      string            `json:"actor" gorm:"not null;default:''"`
	Summary    string            `json:"summary" gorm:"not null"`
	BeforeJSON string            `json:"before_json" gorm:"type:text;not null;default:'{}'"`
	AfterJSON  string            `json:"after_json" gorm:"type:text;not null;default:'{}'"`
	ChangedAt  time.Time         `json:"changed_at" gorm:"not null;index"`
}
