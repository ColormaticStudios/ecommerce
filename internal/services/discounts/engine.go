package discounts

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
)

const (
	ActionModePercent    = "percent"
	ActionModeFixed      = "fixed"
	ActionModeFixedPrice = "fixed_price"
	ActionModeFreeItem   = "free_item"

	StackPolicyNone     = "none"
	StackPolicyAdditive = "additive"
)

type CartLine struct {
	ProductID        uint
	ProductVariantID uint
	BrandID          *uint
	CategoryIDs      []uint
	SKU              string
	Quantity         int
	UnitPrice        models.Money
}

type EvaluatedLine struct {
	CartLine
	BasePrice        models.Money
	DiscountAmount   models.Money
	FinalPrice       models.Money
	AppliedCampaigns []AppliedCampaign
}

type EvaluationResult struct {
	Lines         []EvaluatedLine
	Subtotal      models.Money
	DiscountTotal models.Money
	FinalSubtotal models.Money
}

type EvaluationOptions struct {
	CouponCode      string
	Channel         string
	CustomerSegment string
}

type RuleCondition struct {
	ProductIDs        []uint       `json:"product_ids,omitempty"`
	ProductVariantIDs []uint       `json:"product_variant_ids,omitempty"`
	CategoryIDs       []uint       `json:"category_ids,omitempty"`
	BrandIDs          []uint       `json:"brand_ids,omitempty"`
	MinQuantity       int          `json:"min_quantity,omitempty"`
	MinSubtotal       models.Money `json:"min_subtotal,omitempty"`
}

type RuleAction struct {
	Mode              string       `json:"mode"`
	Value             models.Money `json:"value,omitempty"`
	TargetType        string       `json:"target_type,omitempty"`
	TargetIDs         []uint       `json:"target_ids,omitempty"`
	ProductIDs        []uint       `json:"product_ids,omitempty"`
	ProductVariantIDs []uint       `json:"product_variant_ids,omitempty"`
	CategoryIDs       []uint       `json:"category_ids,omitempty"`
	BrandIDs          []uint       `json:"brand_ids,omitempty"`
	SKU               string       `json:"sku,omitempty"`
}

type CreatePromotionInput struct {
	Name                string
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
	Rules               []PromotionRuleInput
	Levels              []PromotionLevelInput
	ActorID             *uint
}

type PromotionRuleInput struct {
	Condition               RuleCondition
	Action                  RuleAction
	StackPolicy             string
	MaxApplicationsPerOrder *int
}

type PromotionLevelInput struct {
	Name                    string
	Priority                int
	Action                  RuleAction
	StackPolicy             string
	MaxApplicationsPerOrder *int
	Targets                 []PromotionTargetInput
}

type PromotionTargetInput struct {
	TargetType string
	TargetID   uint
}

func EvaluateCart(db *gorm.DB, lines []CartLine, now time.Time) (EvaluationResult, error) {
	return EvaluateCartWithOptions(db, lines, now, EvaluationOptions{Channel: models.DiscountChannelWeb})
}

func EvaluateCartWithOptions(db *gorm.DB, lines []CartLine, now time.Time, options EvaluationOptions) (EvaluationResult, error) {
	start := time.Now()
	candidateCount := 0
	var evalErr error
	result := EvaluationResult{
		Lines: make([]EvaluatedLine, 0, len(lines)),
	}
	defer func() {
		recordEvaluationMetric(start, len(lines), candidateCount, result, evalErr)
	}()
	for _, line := range lines {
		if line.Quantity < 1 {
			continue
		}
		base := line.UnitPrice
		result.Subtotal += base.Mul(line.Quantity)
		result.Lines = append(result.Lines, EvaluatedLine{
			CartLine:   line,
			BasePrice:  base,
			FinalPrice: base,
		})
	}
	if len(result.Lines) == 0 {
		return result, nil
	}
	if !db.Migrator().HasTable(&models.DiscountCampaign{}) || !db.Migrator().HasTable(&models.DiscountTarget{}) {
		result.FinalSubtotal = result.Subtotal
		return result, nil
	}

	campaigns, err := loadActiveCampaigns(db, now, options)
	if err != nil {
		evalErr = err
		return EvaluationResult{}, err
	}
	candidateCount = len(campaigns)
	exclusiveApplied := false
	for _, campaign := range campaigns {
		if exclusiveApplied {
			break
		}
		applied := false
		switch campaign.Type {
		case models.DiscountCampaignTypeProductDiscount:
			applied = applyProductDiscountCampaign(&result, campaign)
		case models.DiscountCampaignTypePromotion:
			applied, err = applyPromotionCampaign(&result, campaign)
			if err != nil {
				evalErr = err
				return EvaluationResult{}, err
			}
		}
		if applied && campaign.IsExclusive {
			exclusiveApplied = true
		}
	}

	for i := range result.Lines {
		line := &result.Lines[i]
		if line.DiscountAmount > line.BasePrice {
			line.DiscountAmount = line.BasePrice
		}
		line.FinalPrice = line.BasePrice - line.DiscountAmount
		if line.FinalPrice < 0 {
			line.FinalPrice = 0
		}
		result.DiscountTotal += line.DiscountAmount.Mul(line.Quantity)
		result.FinalSubtotal += line.FinalPrice.Mul(line.Quantity)
	}
	return result, nil
}

func CreatePromotion(db *gorm.DB, input CreatePromotionInput) (models.DiscountCampaign, error) {
	if err := validatePromotion(input); err != nil {
		return models.DiscountCampaign{}, err
	}
	status := input.Status
	if status == "" {
		status = models.DiscountCampaignStatusActive
	}
	campaign := models.DiscountCampaign{
		Name:        strings.TrimSpace(input.Name),
		Type:        models.DiscountCampaignTypePromotion,
		Status:      status,
		StartsAt:    input.StartsAt.UTC(),
		EndsAt:      utcTimePtr(input.EndsAt),
		Timezone:    "UTC",
		Priority:    input.Priority,
		IsExclusive: input.IsExclusive,
		// Keep legacy P0 columns populated with harmless defaults; promotion
		// actions live in discount_rules/discount_levels.
		DiscountMode:        models.DiscountModeFixed,
		DiscountValue:       0,
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
		for _, ruleInput := range input.Rules {
			conditionJSON, err := encodeJSON(ruleInput.Condition)
			if err != nil {
				return err
			}
			actionJSON, err := encodeJSON(ruleInput.Action)
			if err != nil {
				return err
			}
			rule := models.DiscountRule{
				CampaignID:              campaign.ID,
				ConditionJSON:           conditionJSON,
				ActionJSON:              actionJSON,
				StackPolicy:             normalizeStackPolicy(ruleInput.StackPolicy),
				MaxApplicationsPerOrder: ruleInput.MaxApplicationsPerOrder,
			}
			if err := tx.Create(&rule).Error; err != nil {
				return err
			}
		}
		for _, levelInput := range input.Levels {
			actionJSON, err := encodeJSON(levelInput.Action)
			if err != nil {
				return err
			}
			level := models.DiscountLevel{
				CampaignID:              campaign.ID,
				Name:                    strings.TrimSpace(levelInput.Name),
				Priority:                levelInput.Priority,
				ActionJSON:              actionJSON,
				StackPolicy:             normalizeStackPolicy(levelInput.StackPolicy),
				MaxApplicationsPerOrder: levelInput.MaxApplicationsPerOrder,
			}
			if err := tx.Create(&level).Error; err != nil {
				return err
			}
			targets := make([]models.DiscountTarget, 0, len(levelInput.Targets))
			for _, target := range levelInput.Targets {
				levelID := level.ID
				targets = append(targets, models.DiscountTarget{
					CampaignID: campaign.ID,
					LevelID:    &levelID,
					TargetType: target.TargetType,
					TargetID:   target.TargetID,
				})
			}
			if len(targets) > 0 {
				if err := tx.Create(&targets).Error; err != nil {
					return err
				}
			}
		}
		return createCampaignAudit(tx, campaign.ID, AuditEventCampaignCreated, LifecycleSourceAdmin, actorIDString(input.ActorID), "created promotion campaign", nil, campaign, time.Now().UTC())
	})
	if err != nil {
		return models.DiscountCampaign{}, err
	}
	return LoadCampaign(db, campaign.ID)
}

func loadActiveCampaigns(db *gorm.DB, now time.Time, options EvaluationOptions) ([]models.DiscountCampaign, error) {
	var campaigns []models.DiscountCampaign
	err := db.Preload("Targets").
		Preload("Rules").
		Preload("Levels").
		Where("status = ?", models.DiscountCampaignStatusActive).
		Where("is_archived = ?", false).
		Where("starts_at <= ?", now.UTC()).
		Where("(ends_at IS NULL OR ends_at > ?)", now.UTC()).
		Order("priority DESC").
		Order("id ASC").
		Find(&campaigns).Error
	if err != nil {
		return nil, err
	}
	filtered := campaigns[:0]
	for _, campaign := range campaigns {
		if !campaignMatchesOptions(campaign, options) {
			continue
		}
		filtered = append(filtered, campaign)
	}
	campaigns = filtered
	sort.SliceStable(campaigns, func(i, j int) bool {
		if campaigns[i].Priority == campaigns[j].Priority {
			return campaigns[i].ID < campaigns[j].ID
		}
		return campaigns[i].Priority > campaigns[j].Priority
	})
	return campaigns, nil
}

func campaignMatchesOptions(campaign models.DiscountCampaign, options EvaluationOptions) bool {
	coupon := strings.ToUpper(strings.TrimSpace(options.CouponCode))
	if campaign.CouponCode != nil && strings.ToUpper(strings.TrimSpace(*campaign.CouponCode)) != coupon {
		return false
	}
	channel := strings.TrimSpace(options.Channel)
	if channel == "" {
		channel = models.DiscountChannelWeb
	}
	channels := decodeStringSlice(campaign.ChannelsJSON)
	if len(channels) > 0 && !containsString(channels, channel) {
		return false
	}
	segment := strings.TrimSpace(options.CustomerSegment)
	if strings.TrimSpace(campaign.CustomerSegment) != "" && campaign.CustomerSegment != segment {
		return false
	}
	return true
}

func applyProductDiscountCampaign(result *EvaluationResult, campaign models.DiscountCampaign) bool {
	targets := targetSet(campaign.Targets, models.DiscountTargetTypeProduct, nil)
	applied := false
	for i := range result.Lines {
		line := &result.Lines[i]
		if _, ok := targets[line.ProductID]; !ok || len(line.AppliedCampaigns) > 0 {
			continue
		}
		amount := calculateDiscount(line.BasePrice, campaign)
		if amount <= 0 {
			continue
		}
		applyLineAdjustment(line, campaign, nil, amount, false)
		applied = true
	}
	return applied
}

func applyPromotionCampaign(result *EvaluationResult, campaign models.DiscountCampaign) (bool, error) {
	applied := false

	for _, rule := range campaign.Rules {
		condition, action, err := decodeRule(rule)
		if err != nil {
			return false, err
		}
		if !conditionMatches(*result, condition) {
			continue
		}
		if applyAction(result, campaign, nil, action, normalizeStackPolicy(rule.StackPolicy), rule.MaxApplicationsPerOrder) {
			applied = true
		}
	}

	levels := append([]models.DiscountLevel(nil), campaign.Levels...)
	sort.SliceStable(levels, func(i, j int) bool {
		if levels[i].Priority == levels[j].Priority {
			return levels[i].ID < levels[j].ID
		}
		return levels[i].Priority > levels[j].Priority
	})
	for _, level := range levels {
		action, err := decodeAction(level.ActionJSON)
		if err != nil {
			return false, err
		}
		if action.TargetType == "" {
			action.TargetType = levelTargetType(campaign.Targets, level.ID)
			action.TargetIDs = levelTargetIDs(campaign.Targets, level.ID, action.TargetType)
		}
		levelID := level.ID
		if applyAction(result, campaign, &levelID, action, normalizeStackPolicy(level.StackPolicy), level.MaxApplicationsPerOrder) {
			applied = true
		}
	}
	return applied, nil
}

func conditionMatches(result EvaluationResult, condition RuleCondition) bool {
	var quantity int
	var subtotal models.Money
	for _, line := range result.Lines {
		if !lineMatchesCondition(line.CartLine, condition) {
			continue
		}
		quantity += line.Quantity
		subtotal += line.BasePrice.Mul(line.Quantity)
	}
	if condition.MinQuantity > 0 && quantity < condition.MinQuantity {
		return false
	}
	if condition.MinSubtotal > 0 && subtotal < condition.MinSubtotal {
		return false
	}
	return quantity > 0 || subtotal > 0 || conditionIsGlobal(condition)
}

func lineMatchesCondition(line CartLine, condition RuleCondition) bool {
	if len(condition.ProductIDs) > 0 && !containsUint(condition.ProductIDs, line.ProductID) {
		return false
	}
	if len(condition.ProductVariantIDs) > 0 && !containsUint(condition.ProductVariantIDs, line.ProductVariantID) {
		return false
	}
	if len(condition.BrandIDs) > 0 && (line.BrandID == nil || !containsUint(condition.BrandIDs, *line.BrandID)) {
		return false
	}
	if len(condition.CategoryIDs) > 0 && !intersects(condition.CategoryIDs, line.CategoryIDs) {
		return false
	}
	return true
}

func conditionIsGlobal(condition RuleCondition) bool {
	return len(condition.ProductIDs) == 0 &&
		len(condition.ProductVariantIDs) == 0 &&
		len(condition.CategoryIDs) == 0 &&
		len(condition.BrandIDs) == 0
}

func applyAction(result *EvaluationResult, campaign models.DiscountCampaign, levelID *uint, action RuleAction, stackPolicy string, maxApplications *int) bool {
	if maxApplications != nil && *maxApplications < 1 {
		return false
	}
	applied := false
	applications := 0
	for i := range result.Lines {
		if maxApplications != nil && applications >= *maxApplications {
			break
		}
		line := &result.Lines[i]
		if !lineMatchesActionTarget(line.CartLine, action) {
			continue
		}
		if stackPolicy != StackPolicyAdditive && len(line.AppliedCampaigns) > 0 {
			continue
		}
		amount := actionDiscount(line.BasePrice, action)
		if amount <= 0 {
			continue
		}
		applyLineAdjustment(line, campaign, levelID, amount, stackPolicy == StackPolicyAdditive)
		applied = true
		applications++
	}
	return applied
}

func lineMatchesActionTarget(line CartLine, action RuleAction) bool {
	targetType := action.TargetType
	targetIDs := action.TargetIDs
	if len(action.ProductIDs) > 0 {
		targetType = models.DiscountTargetTypeProduct
		targetIDs = action.ProductIDs
	}
	if len(action.ProductVariantIDs) > 0 {
		targetType = models.DiscountTargetTypeVariant
		targetIDs = action.ProductVariantIDs
	}
	if len(action.CategoryIDs) > 0 {
		targetType = models.DiscountTargetTypeCategory
		targetIDs = action.CategoryIDs
	}
	if len(action.BrandIDs) > 0 {
		targetType = models.DiscountTargetTypeBrand
		targetIDs = action.BrandIDs
	}
	if strings.TrimSpace(action.SKU) != "" && action.SKU != line.SKU {
		return false
	}
	switch targetType {
	case "", "cart":
		return true
	case models.DiscountTargetTypeProduct:
		return containsUint(targetIDs, line.ProductID)
	case models.DiscountTargetTypeVariant:
		return containsUint(targetIDs, line.ProductVariantID)
	case models.DiscountTargetTypeCategory:
		return intersects(targetIDs, line.CategoryIDs)
	case models.DiscountTargetTypeBrand:
		return line.BrandID != nil && containsUint(targetIDs, *line.BrandID)
	default:
		return false
	}
}

func actionDiscount(base models.Money, action RuleAction) models.Money {
	switch action.Mode {
	case ActionModePercent:
		amount := base * action.Value / 10000
		if amount > base {
			return base
		}
		return amount
	case ActionModeFixed:
		if action.Value > base {
			return base
		}
		return action.Value
	case ActionModeFixedPrice:
		if action.Value >= base {
			return 0
		}
		return base - action.Value
	case ActionModeFreeItem:
		return base
	default:
		return 0
	}
}

func applyLineAdjustment(line *EvaluatedLine, campaign models.DiscountCampaign, levelID *uint, amount models.Money, additive bool) {
	if additive {
		line.DiscountAmount += amount
	} else {
		line.DiscountAmount = amount
	}
	if line.DiscountAmount > line.BasePrice {
		line.DiscountAmount = line.BasePrice
	}
	line.AppliedCampaigns = append(line.AppliedCampaigns, AppliedCampaign{
		ID:             campaign.ID,
		Name:           campaign.Name,
		DiscountAmount: amount,
		LevelID:        levelID,
	})
}

func validatePromotion(input CreatePromotionInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidCampaign)
	}
	if input.StartsAt.IsZero() {
		return fmt.Errorf("%w: starts_at is required", ErrInvalidCampaign)
	}
	if input.EndsAt != nil && !input.EndsAt.After(input.StartsAt) {
		return fmt.Errorf("%w: ends_at must be after starts_at", ErrInvalidCampaign)
	}
	if len(input.Rules) == 0 && len(input.Levels) == 0 {
		return fmt.Errorf("%w: promotion requires at least one rule or level", ErrInvalidCampaign)
	}
	for _, rule := range input.Rules {
		if err := validateMaxApplicationsPerOrder(rule.MaxApplicationsPerOrder); err != nil {
			return err
		}
		if err := validateAction(rule.Action); err != nil {
			return err
		}
	}
	for _, level := range input.Levels {
		if strings.TrimSpace(level.Name) == "" {
			return fmt.Errorf("%w: level name is required", ErrInvalidCampaign)
		}
		if err := validateMaxApplicationsPerOrder(level.MaxApplicationsPerOrder); err != nil {
			return err
		}
		if err := validateAction(level.Action); err != nil {
			return err
		}
		for _, target := range level.Targets {
			if !validTargetType(target.TargetType) || target.TargetID == 0 {
				return fmt.Errorf("%w: invalid level target", ErrInvalidCampaign)
			}
		}
	}
	if input.Status != "" && input.Status != models.DiscountCampaignStatusActive && input.Status != models.DiscountCampaignStatusScheduled && input.Status != models.DiscountCampaignStatusDisabled {
		return fmt.Errorf("%w: unsupported status", ErrInvalidCampaign)
	}
	if err := validateAdvancedControls(input.CouponCode, input.Channels, input.GlobalUsageCap, input.PerCustomerUsageCap); err != nil {
		return err
	}
	return nil
}

func validateMaxApplicationsPerOrder(value *int) error {
	if value != nil && *value < 1 {
		return fmt.Errorf("%w: max_applications_per_order must be positive", ErrInvalidCampaign)
	}
	return nil
}

func validateAction(action RuleAction) error {
	switch action.Mode {
	case ActionModePercent:
		if action.Value <= 0 || action.Value > 10000 {
			return fmt.Errorf("%w: percent action must be greater than 0 and no more than 100", ErrInvalidCampaign)
		}
	case ActionModeFixed, ActionModeFixedPrice:
		if action.Value <= 0 {
			return fmt.Errorf("%w: action value must be positive", ErrInvalidCampaign)
		}
	case ActionModeFreeItem:
	default:
		return fmt.Errorf("%w: unsupported action mode", ErrInvalidCampaign)
	}
	return nil
}

func validTargetType(value string) bool {
	switch value {
	case models.DiscountTargetTypeProduct, models.DiscountTargetTypeVariant, models.DiscountTargetTypeCategory, models.DiscountTargetTypeBrand:
		return true
	default:
		return false
	}
}

func normalizeStackPolicy(value string) string {
	if value == StackPolicyAdditive {
		return StackPolicyAdditive
	}
	return StackPolicyNone
}

func encodeJSON(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func decodeRule(rule models.DiscountRule) (RuleCondition, RuleAction, error) {
	var condition RuleCondition
	if strings.TrimSpace(rule.ConditionJSON) != "" {
		if err := json.Unmarshal([]byte(rule.ConditionJSON), &condition); err != nil {
			return RuleCondition{}, RuleAction{}, err
		}
	}
	action, err := decodeAction(rule.ActionJSON)
	return condition, action, err
}

func decodeAction(raw string) (RuleAction, error) {
	var action RuleAction
	if strings.TrimSpace(raw) == "" {
		return action, nil
	}
	err := json.Unmarshal([]byte(raw), &action)
	return action, err
}

func targetSet(targets []models.DiscountTarget, targetType string, levelID *uint) map[uint]struct{} {
	result := map[uint]struct{}{}
	for _, target := range targets {
		if target.TargetType != targetType {
			continue
		}
		if levelID != nil && (target.LevelID == nil || *target.LevelID != *levelID) {
			continue
		}
		result[target.TargetID] = struct{}{}
	}
	return result
}

func levelTargetType(targets []models.DiscountTarget, levelID uint) string {
	for _, target := range targets {
		if target.LevelID != nil && *target.LevelID == levelID {
			return target.TargetType
		}
	}
	return ""
}

func levelTargetIDs(targets []models.DiscountTarget, levelID uint, targetType string) []uint {
	ids := make([]uint, 0)
	for _, target := range targets {
		if target.LevelID != nil && *target.LevelID == levelID && target.TargetType == targetType {
			ids = append(ids, target.TargetID)
		}
	}
	return ids
}

func containsUint(values []uint, target uint) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func intersects(left []uint, right []uint) bool {
	for _, value := range left {
		if containsUint(right, value) {
			return true
		}
	}
	return false
}

func decodeStringSlice(raw string) []string {
	var values []string
	if strings.TrimSpace(raw) == "" {
		return values
	}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	normalized := values[:0]
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return normalized
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
