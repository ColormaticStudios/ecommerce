package discounts

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
)

var ErrInvalidCampaign = errors.New("invalid discount campaign")

type AppliedCampaign struct {
	ID             uint
	LevelID        *uint
	Name           string
	DiscountAmount models.Money
}

type Price struct {
	BasePrice        models.Money
	DiscountAmount   models.Money
	FinalPrice       models.Money
	AppliedCampaigns []AppliedCampaign
}

type ProductDiscountInput struct {
	Name                string
	ProductIDs          []uint
	DiscountMode        string
	DiscountValue       models.Money
	StartsAt            time.Time
	EndsAt              *time.Time
	Priority            int
	IsExclusive         bool
	Status              string
	Metadata            map[string]any
	CouponCode          *string
	Channels            []string
	CustomerSegment     string
	GlobalUsageCap      *int
	PerCustomerUsageCap *int
	ActorID             *uint
}

func ValidateProductDiscount(input ProductDiscountInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidCampaign)
	}
	if len(input.ProductIDs) == 0 {
		return fmt.Errorf("%w: at least one product target is required", ErrInvalidCampaign)
	}
	if input.StartsAt.IsZero() {
		return fmt.Errorf("%w: starts_at is required", ErrInvalidCampaign)
	}
	if input.EndsAt != nil && !input.EndsAt.After(input.StartsAt) {
		return fmt.Errorf("%w: ends_at must be after starts_at", ErrInvalidCampaign)
	}
	switch input.DiscountMode {
	case models.DiscountModeFixed:
		if input.DiscountValue <= 0 {
			return fmt.Errorf("%w: fixed discount must be positive", ErrInvalidCampaign)
		}
	case models.DiscountModePercent:
		if input.DiscountValue <= 0 || input.DiscountValue > 10000 {
			return fmt.Errorf("%w: percent discount must be greater than 0 and no more than 100", ErrInvalidCampaign)
		}
	default:
		return fmt.Errorf("%w: unsupported discount mode", ErrInvalidCampaign)
	}
	if input.Status != "" && input.Status != models.DiscountCampaignStatusActive && input.Status != models.DiscountCampaignStatusScheduled && input.Status != models.DiscountCampaignStatusDisabled {
		return fmt.Errorf("%w: unsupported status", ErrInvalidCampaign)
	}
	if err := validateAdvancedControls(input.CouponCode, input.Channels, input.GlobalUsageCap, input.PerCustomerUsageCap); err != nil {
		return err
	}
	return nil
}

func CreateProductDiscount(db *gorm.DB, input ProductDiscountInput) (models.DiscountCampaign, error) {
	if err := ValidateProductDiscount(input); err != nil {
		return models.DiscountCampaign{}, err
	}
	status := input.Status
	if status == "" {
		status = models.DiscountCampaignStatusActive
	}
	campaign := models.DiscountCampaign{
		Name:                strings.TrimSpace(input.Name),
		Type:                models.DiscountCampaignTypeProductDiscount,
		Status:              status,
		StartsAt:            input.StartsAt.UTC(),
		EndsAt:              utcTimePtr(input.EndsAt),
		Timezone:            "UTC",
		Priority:            input.Priority,
		IsExclusive:         input.IsExclusive,
		DiscountMode:        input.DiscountMode,
		DiscountValue:       input.DiscountValue,
		MetadataJSON:        mustEncodeJSON(input.Metadata, "{}"),
		CouponCode:          normalizeCouponPtr(input.CouponCode),
		ChannelsJSON:        mustEncodeJSON(input.Channels, "[]"),
		CustomerSegment:     strings.TrimSpace(input.CustomerSegment),
		GlobalUsageCap:      input.GlobalUsageCap,
		PerCustomerUsageCap: input.PerCustomerUsageCap,
		CreatedBy:           input.ActorID,
		UpdatedBy:           input.ActorID,
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&campaign).Error; err != nil {
			return err
		}
		targets := makeTargets(campaign.ID, input.ProductIDs)
		if err := tx.Create(&targets).Error; err != nil {
			return err
		}
		return createCampaignAudit(tx, campaign.ID, AuditEventCampaignCreated, LifecycleSourceAdmin, actorIDString(input.ActorID), "created product discount campaign", nil, campaign, time.Now().UTC())
	})
	if err != nil {
		return models.DiscountCampaign{}, err
	}
	return LoadCampaign(db, campaign.ID)
}

func UpdateProductDiscount(db *gorm.DB, id uint, input ProductDiscountInput) (models.DiscountCampaign, error) {
	if err := ValidateProductDiscount(input); err != nil {
		return models.DiscountCampaign{}, err
	}
	status := input.Status
	if status == "" {
		status = models.DiscountCampaignStatusActive
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		var before models.DiscountCampaign
		if err := tx.Preload("Targets").First(&before, "id = ? AND type = ?", id, models.DiscountCampaignTypeProductDiscount).Error; err != nil {
			return err
		}
		updates := map[string]any{
			"name":                   strings.TrimSpace(input.Name),
			"status":                 status,
			"starts_at":              input.StartsAt.UTC(),
			"ends_at":                utcTimePtr(input.EndsAt),
			"priority":               input.Priority,
			"is_exclusive":           input.IsExclusive,
			"discount_mode":          input.DiscountMode,
			"discount_value":         input.DiscountValue,
			"metadata_json":          mustEncodeJSON(input.Metadata, "{}"),
			"coupon_code":            normalizeCouponPtr(input.CouponCode),
			"channels_json":          mustEncodeJSON(input.Channels, "[]"),
			"customer_segment":       strings.TrimSpace(input.CustomerSegment),
			"global_usage_cap":       input.GlobalUsageCap,
			"per_customer_usage_cap": input.PerCustomerUsageCap,
			"updated_by":             input.ActorID,
		}
		result := tx.Model(&models.DiscountCampaign{}).
			Where("id = ? AND type = ?", id, models.DiscountCampaignTypeProductDiscount).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Where("campaign_id = ?", id).Delete(&models.DiscountTarget{}).Error; err != nil {
			return err
		}
		targets := makeTargets(id, input.ProductIDs)
		if err := tx.Create(&targets).Error; err != nil {
			return err
		}
		var after models.DiscountCampaign
		if err := tx.Preload("Targets").First(&after, id).Error; err != nil {
			return err
		}
		return createCampaignAudit(tx, id, AuditEventCampaignUpdated, LifecycleSourceAdmin, actorIDString(input.ActorID), "updated product discount campaign", before, after, time.Now().UTC())
	})
	if err != nil {
		return models.DiscountCampaign{}, err
	}
	return LoadCampaign(db, id)
}

func DisableProductDiscount(db *gorm.DB, id uint, actorID *uint) (models.DiscountCampaign, error) {
	err := db.Transaction(func(tx *gorm.DB) error {
		var before models.DiscountCampaign
		if err := tx.First(&before, "id = ? AND type = ?", id, models.DiscountCampaignTypeProductDiscount).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.DiscountCampaign{}).
			Where("id = ? AND type = ?", id, models.DiscountCampaignTypeProductDiscount).
			Updates(map[string]any{
				"status":     models.DiscountCampaignStatusDisabled,
				"updated_by": actorID,
			}).Error; err != nil {
			return err
		}
		var after models.DiscountCampaign
		if err := tx.First(&after, id).Error; err != nil {
			return err
		}
		return createCampaignAudit(tx, id, AuditEventCampaignDisabled, LifecycleSourceAdmin, actorIDString(actorID), "disabled product discount campaign", before, after, time.Now().UTC())
	})
	if err != nil {
		return models.DiscountCampaign{}, err
	}
	return LoadCampaign(db, id)
}

func LoadCampaign(db *gorm.DB, id uint) (models.DiscountCampaign, error) {
	var campaign models.DiscountCampaign
	err := db.Preload("Targets").Preload("Rules").Preload("Levels").First(&campaign, id).Error
	return campaign, err
}

func ListDiscountCampaigns(db *gorm.DB, status string) ([]models.DiscountCampaign, error) {
	query := db.Preload("Targets").
		Preload("Rules").
		Preload("Levels").
		Order("priority DESC").
		Order("id ASC")
	if status != "" {
		query = query.Where("status = ?", status)
		if status == models.DiscountCampaignStatusArchived {
			query = query.Or("is_archived = ?", true)
		}
	} else {
		query = query.Where("is_archived = ?", false)
	}
	var campaigns []models.DiscountCampaign
	if err := query.Find(&campaigns).Error; err != nil {
		return nil, err
	}
	return campaigns, nil
}

func ListProductDiscounts(db *gorm.DB, status string) ([]models.DiscountCampaign, error) {
	query := db.Preload("Targets").
		Where("type = ?", models.DiscountCampaignTypeProductDiscount).
		Order("priority DESC").
		Order("id ASC")
	if status != "" {
		query = query.Where("status = ?", status)
	} else {
		query = query.Where("is_archived = ?", false)
	}
	var campaigns []models.DiscountCampaign
	if err := query.Find(&campaigns).Error; err != nil {
		return nil, err
	}
	return campaigns, nil
}

func PriceForProduct(db *gorm.DB, productID uint, basePrice models.Money, now time.Time) (Price, error) {
	prices, err := PricesForProducts(db, map[uint]models.Money{productID: basePrice}, now)
	if err != nil {
		return Price{}, err
	}
	return prices[productID], nil
}

func PricesForProducts(db *gorm.DB, basePrices map[uint]models.Money, now time.Time) (map[uint]Price, error) {
	result := make(map[uint]Price, len(basePrices))
	productIDs := make([]uint, 0, len(basePrices))
	for id, base := range basePrices {
		productIDs = append(productIDs, id)
		result[id] = Price{BasePrice: base, FinalPrice: base}
	}
	if len(productIDs) == 0 {
		return result, nil
	}
	if !db.Migrator().HasTable(&models.DiscountCampaign{}) || !db.Migrator().HasTable(&models.DiscountTarget{}) {
		return result, nil
	}

	var campaigns []models.DiscountCampaign
	if err := db.Preload("Targets", "target_type = ? AND target_id IN ?", models.DiscountTargetTypeProduct, productIDs).
		Joins("JOIN discount_targets ON discount_targets.campaign_id = discount_campaigns.id").
		Where("discount_campaigns.type = ?", models.DiscountCampaignTypeProductDiscount).
		Where("discount_campaigns.status = ?", models.DiscountCampaignStatusActive).
		Where("discount_campaigns.is_archived = ?", false).
		Where("discount_targets.target_type = ? AND discount_targets.target_id IN ?", models.DiscountTargetTypeProduct, productIDs).
		Where("discount_campaigns.starts_at <= ?", now.UTC()).
		Where("(discount_campaigns.ends_at IS NULL OR discount_campaigns.ends_at > ?)", now.UTC()).
		Order("discount_campaigns.priority DESC").
		Order("discount_campaigns.id ASC").
		Find(&campaigns).Error; err != nil {
		return nil, err
	}
	sort.SliceStable(campaigns, func(i, j int) bool {
		if campaigns[i].Priority == campaigns[j].Priority {
			return campaigns[i].ID < campaigns[j].ID
		}
		return campaigns[i].Priority > campaigns[j].Priority
	})

	for _, campaign := range campaigns {
		for _, target := range campaign.Targets {
			price := result[target.TargetID]
			if len(price.AppliedCampaigns) > 0 {
				continue
			}
			amount := calculateDiscount(price.BasePrice, campaign)
			if amount <= 0 {
				continue
			}
			price.DiscountAmount = amount
			price.FinalPrice = price.BasePrice - amount
			if price.FinalPrice < 0 {
				price.FinalPrice = 0
			}
			price.AppliedCampaigns = []AppliedCampaign{{
				ID:             campaign.ID,
				Name:           campaign.Name,
				DiscountAmount: amount,
			}}
			result[target.TargetID] = price
		}
	}
	return result, nil
}

func calculateDiscount(base models.Money, campaign models.DiscountCampaign) models.Money {
	switch campaign.DiscountMode {
	case models.DiscountModeFixed:
		if campaign.DiscountValue > base {
			return base
		}
		return campaign.DiscountValue
	case models.DiscountModePercent:
		amount := base * campaign.DiscountValue / 10000
		if amount > base {
			return base
		}
		return amount
	default:
		return 0
	}
}

func makeTargets(campaignID uint, productIDs []uint) []models.DiscountTarget {
	seen := map[uint]struct{}{}
	targets := make([]models.DiscountTarget, 0, len(productIDs))
	for _, productID := range productIDs {
		if productID == 0 {
			continue
		}
		if _, exists := seen[productID]; exists {
			continue
		}
		seen[productID] = struct{}{}
		targets = append(targets, models.DiscountTarget{
			CampaignID: campaignID,
			TargetType: models.DiscountTargetTypeProduct,
			TargetID:   productID,
		})
	}
	return targets
}

func utcTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func actorIDString(actorID *uint) string {
	if actorID == nil {
		return ""
	}
	return fmt.Sprintf("user:%d", *actorID)
}
