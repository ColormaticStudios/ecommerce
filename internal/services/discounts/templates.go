package discounts

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrUsageCapExceeded = fmt.Errorf("%w: usage cap exceeded", ErrInvalidCampaign)

type TemplateInput struct {
	Name        string
	Description string
	Template    CreatePromotionInput
	IsActive    *bool
}

type InstantiateTemplateInput struct {
	Name            *string
	StartsAt        *time.Time
	EndsAt          *time.Time
	CouponCode      *string
	Channels        []string
	CustomerSegment *string
	GlobalUsageCap  *int
	PerCustomerCap  *int
	ActorID         *uint
}

func CreateTemplate(db *gorm.DB, input TemplateInput) (models.PromotionTemplate, error) {
	if strings.TrimSpace(input.Name) == "" {
		return models.PromotionTemplate{}, fmt.Errorf("%w: template name is required", ErrInvalidCampaign)
	}
	if err := validatePromotion(input.Template); err != nil {
		return models.PromotionTemplate{}, err
	}
	raw, err := json.Marshal(input.Template)
	if err != nil {
		return models.PromotionTemplate{}, err
	}
	active := true
	if input.IsActive != nil {
		active = *input.IsActive
	}
	template := models.PromotionTemplate{
		Name:         strings.TrimSpace(input.Name),
		Description:  strings.TrimSpace(input.Description),
		TemplateJSON: string(raw),
		IsActive:     active,
	}
	return template, db.Create(&template).Error
}

func InstantiateTemplate(db *gorm.DB, id uint, input InstantiateTemplateInput) (models.DiscountCampaign, error) {
	var template models.PromotionTemplate
	if err := db.First(&template, "id = ? AND is_active = ?", id, true).Error; err != nil {
		return models.DiscountCampaign{}, err
	}
	var promotion CreatePromotionInput
	if err := json.Unmarshal([]byte(template.TemplateJSON), &promotion); err != nil {
		return models.DiscountCampaign{}, err
	}
	if input.Name != nil {
		promotion.Name = *input.Name
	}
	if input.StartsAt != nil {
		promotion.StartsAt = *input.StartsAt
	}
	if input.EndsAt != nil {
		promotion.EndsAt = input.EndsAt
	}
	if input.CouponCode != nil {
		promotion.CouponCode = input.CouponCode
	}
	if input.Channels != nil {
		promotion.Channels = input.Channels
	}
	if input.CustomerSegment != nil {
		promotion.CustomerSegment = *input.CustomerSegment
	}
	if input.GlobalUsageCap != nil {
		promotion.GlobalUsageCap = input.GlobalUsageCap
	}
	if input.PerCustomerCap != nil {
		promotion.PerCustomerUsageCap = input.PerCustomerCap
	}
	promotion.ActorID = input.ActorID
	return CreatePromotion(db, promotion)
}

func ListTemplates(db *gorm.DB, activeOnly bool) ([]models.PromotionTemplate, error) {
	query := db.Order("name ASC").Order("id ASC")
	if activeOnly {
		query = query.Where("is_active = ?", true)
	}
	var templates []models.PromotionTemplate
	if err := query.Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

func VerifyUsageCaps(tx *gorm.DB, result EvaluationResult, customerID *uint) error {
	campaignAmounts := appliedAmounts(result)
	if len(campaignAmounts) == 0 {
		return nil
	}
	ids := make([]uint, 0, len(campaignAmounts))
	for id := range campaignAmounts {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var campaigns []models.DiscountCampaign
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", ids).Find(&campaigns).Error; err != nil {
		return err
	}
	for _, campaign := range campaigns {
		if campaign.GlobalUsageCap != nil {
			var count int64
			if err := tx.Model(&models.DiscountRedemption{}).Where("campaign_id = ?", campaign.ID).Count(&count).Error; err != nil {
				return err
			}
			if count >= int64(*campaign.GlobalUsageCap) {
				return ErrUsageCapExceeded
			}
		}
		if campaign.PerCustomerUsageCap != nil && customerID != nil {
			var count int64
			if err := tx.Model(&models.DiscountRedemption{}).
				Where("campaign_id = ? AND customer_id = ?", campaign.ID, *customerID).
				Count(&count).Error; err != nil {
				return err
			}
			if count >= int64(*campaign.PerCustomerUsageCap) {
				return ErrUsageCapExceeded
			}
		}
	}
	return nil
}

func RecordRedemptions(tx *gorm.DB, orderID uint, customerID *uint, result EvaluationResult, now time.Time) error {
	campaignAmounts := appliedAmounts(result)
	if len(campaignAmounts) == 0 {
		return nil
	}
	hash := evaluationHash(result)
	for campaignID, amount := range campaignAmounts {
		redemption := models.DiscountRedemption{
			CampaignID:             campaignID,
			OrderID:                orderID,
			CustomerID:             customerID,
			AppliedAmount:          amount,
			AppliedAt:              now.UTC(),
			EvaluationSnapshotHash: hash,
		}
		if err := tx.Create(&redemption).Error; err != nil {
			return err
		}
	}
	return nil
}

func appliedAmounts(result EvaluationResult) map[uint]models.Money {
	amounts := map[uint]models.Money{}
	for _, line := range result.Lines {
		for _, campaign := range line.AppliedCampaigns {
			amounts[campaign.ID] += campaign.DiscountAmount.Mul(line.Quantity)
		}
	}
	return amounts
}

func evaluationHash(result EvaluationResult) string {
	type hashLine struct {
		ProductVariantID uint         `json:"product_variant_id"`
		Quantity         int          `json:"quantity"`
		FinalPrice       models.Money `json:"final_price"`
	}
	lines := make([]hashLine, 0, len(result.Lines))
	for _, line := range result.Lines {
		lines = append(lines, hashLine{ProductVariantID: line.ProductVariantID, Quantity: line.Quantity, FinalPrice: line.FinalPrice})
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].ProductVariantID < lines[j].ProductVariantID })
	raw, _ := json.Marshal(lines)
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("%x", sum)
}

func validateAdvancedControls(coupon *string, channels []string, globalCap *int, customerCap *int) error {
	if coupon != nil && strings.TrimSpace(*coupon) == "" {
		return fmt.Errorf("%w: coupon_code cannot be blank", ErrInvalidCampaign)
	}
	for _, channel := range channels {
		switch strings.TrimSpace(channel) {
		case models.DiscountChannelWeb, models.DiscountChannelApp, models.DiscountChannelAdmin:
		default:
			return fmt.Errorf("%w: unsupported channel", ErrInvalidCampaign)
		}
	}
	if globalCap != nil && *globalCap < 1 {
		return fmt.Errorf("%w: global_usage_cap must be positive", ErrInvalidCampaign)
	}
	if customerCap != nil && *customerCap < 1 {
		return fmt.Errorf("%w: per_customer_usage_cap must be positive", ErrInvalidCampaign)
	}
	return nil
}

func normalizeCouponPtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.ToUpper(strings.TrimSpace(*value))
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func mustEncodeJSON(value any, fallback string) string {
	if value == nil {
		return fallback
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fallback
	}
	return string(raw)
}
