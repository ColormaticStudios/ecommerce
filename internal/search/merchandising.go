package search

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"ecommerce/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MerchandisingQuery struct {
	Mode  string `json:"mode"`
	Value string `json:"value"`
}
type MerchandisingPredicate struct {
	Query         *MerchandisingQuery `json:"query,omitempty"`
	CategorySlugs []string            `json:"category_slugs,omitempty"`
	Channel       string              `json:"channel,omitempty"`
}
type MerchandisingTarget struct {
	ProductID   uint   `json:"product_id"`
	Position    *int   `json:"position,omitempty"`
	ProductName string `json:"product_name,omitempty"`
}
type MerchandisingAction struct {
	Targets    []MerchandisingTarget `json:"targets"`
	Multiplier *float64              `json:"multiplier,omitempty"`
}
type MerchandisingRuleInput struct {
	Name      string                 `json:"name"`
	RuleType  string                 `json:"rule_type"`
	Predicate MerchandisingPredicate `json:"predicate"`
	Action    MerchandisingAction    `json:"action"`
	Priority  int                    `json:"priority"`
	StartsAt  *time.Time             `json:"starts_at"`
	EndsAt    *time.Time             `json:"ends_at"`
	IsActive  bool                   `json:"is_active"`
}
type MerchandisingRule struct {
	MerchandisingRuleInput
	ID        uint      `json:"id"`
	Version   int       `json:"version"`
	UpdatedBy *uint     `json:"updated_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type MerchandisingRulePatch struct {
	Name, RuleType             *string
	Predicate                  *MerchandisingPredicate
	Action                     *MerchandisingAction
	Priority                   *int
	StartsAt, EndsAt           *time.Time
	ClearStartsAt, ClearEndsAt bool
	IsActive                   *bool
}
type MerchandisingAudit struct {
	ID, RuleID    uint
	Operation     string
	ActorID       *uint
	Before, After *MerchandisingRule
	CreatedAt     time.Time
}
type RuleDecision struct {
	RuleID                       uint
	RuleName, RuleType           string
	ProductID                    uint
	ProductName, Outcome, Reason string
	Position                     *int
	Multiplier                   *float64
}
type MerchandisingPreviewInput struct {
	Filters Filters
	Rules   *[]MerchandisingRule
	At      *time.Time
}
type MerchandisingPreview struct{ Baseline, Proposed Result }

func normalizeMerchandisingRule(input MerchandisingRuleInput) (MerchandisingRuleInput, error) {
	invalid := func(message string) (MerchandisingRuleInput, error) {
		return MerchandisingRuleInput{}, fmt.Errorf("%w: %s", ErrConfigurationInvalid, message)
	}
	name, err := normalizeConfigurationName(input.Name)
	if err != nil {
		return input, err
	}
	input.Name = name
	switch input.RuleType {
	case "pin", "bury", "hide", "boost", "include":
	default:
		return invalid("unsupported merchandising rule type")
	}
	if input.Priority < 0 || input.Priority > 10000 {
		return invalid("priority must be between 0 and 10000")
	}
	if input.StartsAt != nil && input.EndsAt != nil && !input.StartsAt.Before(*input.EndsAt) {
		return invalid("start must be before end")
	}
	if input.StartsAt != nil {
		v := input.StartsAt.UTC()
		input.StartsAt = &v
	}
	if input.EndsAt != nil {
		v := input.EndsAt.UTC()
		input.EndsAt = &v
	}
	input.Predicate.Channel = strings.TrimSpace(strings.ToLower(input.Predicate.Channel))
	if input.Predicate.Channel != "" && input.Predicate.Channel != "storefront" {
		return invalid("only the storefront channel is supported")
	}
	if input.Predicate.Query != nil {
		q := *input.Predicate.Query
		q.Value = NormalizeQuery(q.Value)
		if q.Value == "" || len([]rune(q.Value)) > 200 || len(strings.Fields(q.Value)) > 20 {
			return invalid("query predicate must contain text and be at most 200 characters and 20 words")
		}
		switch q.Mode {
		case "exact", "prefix", "contains":
		default:
			return invalid("unsupported query predicate mode")
		}
		input.Predicate.Query = &q
	}
	if len(input.Predicate.CategorySlugs) > 20 {
		return invalid("at most 20 category contexts are allowed")
	}
	slugs := make([]string, 0, len(input.Predicate.CategorySlugs))
	seenSlugs := map[string]bool{}
	for _, slug := range input.Predicate.CategorySlugs {
		slug = strings.ToLower(strings.TrimSpace(slug))
		if NormalizeQuery(slug) == "" || len([]rune(slug)) > 120 {
			return invalid("invalid category context")
		}
		if !seenSlugs[slug] {
			seenSlugs[slug] = true
			slugs = append(slugs, slug)
		}
	}
	input.Predicate.CategorySlugs = slugs
	if len(input.Action.Targets) < 1 || len(input.Action.Targets) > 100 {
		return invalid("rules require between 1 and 100 product targets")
	}
	targets := make([]MerchandisingTarget, 0, len(input.Action.Targets))
	ids := map[uint]bool{}
	positions := map[int]bool{}
	for _, target := range input.Action.Targets {
		if target.ProductID == 0 || ids[target.ProductID] {
			return invalid("product targets must be positive and unique")
		}
		ids[target.ProductID] = true
		target.ProductName = ""
		if input.RuleType == "pin" {
			if target.Position == nil || *target.Position < 1 || *target.Position > 10000 || positions[*target.Position] {
				return invalid("pin positions must be distinct values between 1 and 10000")
			}
			position := *target.Position
			target.Position = &position
			positions[position] = true
		} else if target.Position != nil {
			return invalid("positions are only supported for pin rules")
		}
		targets = append(targets, target)
	}
	input.Action.Targets = targets
	if input.RuleType == "boost" {
		if input.Action.Multiplier == nil || math.IsNaN(*input.Action.Multiplier) || math.IsInf(*input.Action.Multiplier, 0) || *input.Action.Multiplier <= 1 || *input.Action.Multiplier > 100 {
			return invalid("boost multiplier must be finite, greater than 1, and at most 100")
		}
		v := *input.Action.Multiplier
		input.Action.Multiplier = &v
	} else if input.Action.Multiplier != nil {
		return invalid("multipliers are only supported for boost rules")
	}
	return input, nil
}
func merchandisingRuleFromModel(row models.SearchMerchandisingRule) (MerchandisingRule, error) {
	input := MerchandisingRuleInput{Name: row.Name, RuleType: row.RuleType, Priority: row.Priority, StartsAt: row.StartsAt, EndsAt: row.EndsAt, IsActive: row.IsActive}
	if err := json.Unmarshal([]byte(row.PredicateJSON), &input.Predicate); err != nil {
		return MerchandisingRule{}, err
	}
	if err := json.Unmarshal([]byte(row.ActionJSON), &input.Action); err != nil {
		return MerchandisingRule{}, err
	}
	input, err := normalizeMerchandisingRule(input)
	if err != nil {
		return MerchandisingRule{}, err
	}
	return MerchandisingRule{MerchandisingRuleInput: input, ID: row.ID, Version: row.Version, UpdatedBy: row.UpdatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}
func hydrateMerchandisingTargets(db *gorm.DB, rules []MerchandisingRule, requireExisting bool) error {
	ids := []uint{}
	seen := map[uint]bool{}
	for _, rule := range rules {
		for _, target := range rule.Action.Targets {
			if !seen[target.ProductID] {
				ids = append(ids, target.ProductID)
				seen[target.ProductID] = true
			}
		}
	}
	names := map[uint]string{}
	for start := 0; start < len(ids); start += 500 {
		var rows []struct {
			ID   uint
			Name string
		}
		if err := db.Model(&models.Product{}).Select("id, name").Where("id IN ?", ids[start:min(start+500, len(ids))]).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			names[row.ID] = row.Name
		}
	}
	for i := range rules {
		for j := range rules[i].Action.Targets {
			target := &rules[i].Action.Targets[j]
			name, ok := names[target.ProductID]
			if !ok {
				if requireExisting {
					return fmt.Errorf("%w: a target product does not exist", ErrConfigurationInvalid)
				}
				name = "Removed product"
			}
			target.ProductName = name
		}
	}
	return nil
}
func (s *Service) ListMerchandisingRules(ctx context.Context) ([]MerchandisingRule, error) {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return nil, err
	}
	var rows []models.SearchMerchandisingRule
	if err := db.Order("priority DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]MerchandisingRule, 0, len(rows))
	for _, row := range rows {
		rule, err := merchandisingRuleFromModel(row)
		if err != nil {
			return nil, fmt.Errorf("invalid persisted merchandising rule %d: %v", row.ID, err)
		}
		result = append(result, rule)
	}
	if err := hydrateMerchandisingTargets(db, result, false); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *Service) GetMerchandisingRule(ctx context.Context, id uint) (MerchandisingRule, error) {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return MerchandisingRule{}, err
	}
	var row models.SearchMerchandisingRule
	if err := db.First(&row, id).Error; err != nil {
		return MerchandisingRule{}, configurationLookupError(err)
	}
	rule, err := merchandisingRuleFromModel(row)
	if err != nil {
		return MerchandisingRule{}, err
	}
	rules := []MerchandisingRule{rule}
	if err := hydrateMerchandisingTargets(db, rules, false); err != nil {
		return MerchandisingRule{}, err
	}
	return rules[0], nil
}
func (s *Service) recordMerchandisingAudit(tx *gorm.DB, id uint, operation string, actor *uint, before, after *MerchandisingRule) error {
	encode := func(rule *MerchandisingRule) (*string, error) {
		if rule == nil {
			return nil, nil
		}
		raw, err := json.Marshal(rule)
		value := string(raw)
		return &value, err
	}
	beforeJSON, err := encode(before)
	if err != nil {
		return err
	}
	afterJSON, err := encode(after)
	if err != nil {
		return err
	}
	return tx.Create(&models.SearchMerchandisingAudit{RuleID: id, Operation: operation, ActorID: actor, BeforeJSON: beforeJSON, AfterJSON: afterJSON, CreatedAt: s.now().UTC()}).Error
}
func (s *Service) CreateMerchandisingRule(ctx context.Context, input MerchandisingRuleInput, actor *uint) (MerchandisingRule, error) {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return MerchandisingRule{}, err
	}
	input, err = normalizeMerchandisingRule(input)
	if err != nil {
		return MerchandisingRule{}, err
	}
	var result MerchandisingRule
	err = db.Transaction(func(tx *gorm.DB) error {
		targets := []MerchandisingRule{{MerchandisingRuleInput: input}}
		if err := hydrateMerchandisingTargets(tx, targets, true); err != nil {
			return err
		}
		predicate, _ := json.Marshal(input.Predicate)
		action, _ := json.Marshal(input.Action)
		wantActive := input.IsActive
		row := models.SearchMerchandisingRule{Name: input.Name, RuleType: input.RuleType, PredicateJSON: string(predicate), ActionJSON: string(action), Priority: input.Priority, StartsAt: input.StartsAt, EndsAt: input.EndsAt, IsActive: wantActive, UpdatedBy: actor, Version: 1}
		row.CreatedAt = s.now().UTC()
		row.UpdatedAt = row.CreatedAt
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if !wantActive {
			if err := tx.Model(&row).Updates(map[string]any{"is_active": false, "updated_at": row.UpdatedAt}).Error; err != nil {
				return err
			}
			row.IsActive = false
		}
		result = targets[0]
		result.ID = row.ID
		result.Version = 1
		result.UpdatedBy = actor
		result.CreatedAt = row.CreatedAt
		result.UpdatedAt = row.UpdatedAt
		return s.recordMerchandisingAudit(tx, row.ID, "create", actor, nil, &result)
	})
	if err != nil {
		return MerchandisingRule{}, configurationWriteError(err)
	}
	return result, nil
}
func mergeMerchandisingPatch(current MerchandisingRuleInput, patch MerchandisingRulePatch) (MerchandisingRuleInput, error) {
	if patch.Name == nil && patch.RuleType == nil && patch.Predicate == nil && patch.Action == nil && patch.Priority == nil && patch.StartsAt == nil && patch.EndsAt == nil && !patch.ClearStartsAt && !patch.ClearEndsAt && patch.IsActive == nil {
		return current, fmt.Errorf("%w: patch cannot be empty", ErrConfigurationInvalid)
	}
	if patch.ClearStartsAt && patch.StartsAt != nil || patch.ClearEndsAt && patch.EndsAt != nil {
		return current, fmt.Errorf("%w: a schedule boundary cannot be set and cleared together", ErrConfigurationInvalid)
	}
	if patch.Name != nil {
		current.Name = *patch.Name
	}
	if patch.RuleType != nil {
		current.RuleType = *patch.RuleType
	}
	if patch.Predicate != nil {
		current.Predicate = *patch.Predicate
	}
	if patch.Action != nil {
		current.Action = *patch.Action
	}
	if patch.Priority != nil {
		current.Priority = *patch.Priority
	}
	if patch.StartsAt != nil {
		current.StartsAt = patch.StartsAt
	}
	if patch.EndsAt != nil {
		current.EndsAt = patch.EndsAt
	}
	if patch.ClearStartsAt {
		current.StartsAt = nil
	}
	if patch.ClearEndsAt {
		current.EndsAt = nil
	}
	if patch.IsActive != nil {
		current.IsActive = *patch.IsActive
	}
	return normalizeMerchandisingRule(current)
}
func (s *Service) UpdateMerchandisingRule(ctx context.Context, id uint, patch MerchandisingRulePatch, actor *uint) (MerchandisingRule, error) {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return MerchandisingRule{}, err
	}
	var result MerchandisingRule
	err = db.Transaction(func(tx *gorm.DB) error {
		var row models.SearchMerchandisingRule
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, id).Error; err != nil {
			return configurationLookupError(err)
		}
		before, err := merchandisingRuleFromModel(row)
		if err != nil {
			return err
		}
		input, err := mergeMerchandisingPatch(before.MerchandisingRuleInput, patch)
		if err != nil {
			return err
		}
		rules := []MerchandisingRule{before, {MerchandisingRuleInput: input}}
		if err := hydrateMerchandisingTargets(tx, rules[1:], true); err != nil {
			return err
		}
		if err := hydrateMerchandisingTargets(tx, rules[:1], false); err != nil {
			return err
		}
		before = rules[0]
		predicate, _ := json.Marshal(input.Predicate)
		action, _ := json.Marshal(input.Action)
		now := s.now().UTC()
		if err := tx.Model(&row).Updates(map[string]any{"name": input.Name, "rule_type": input.RuleType, "predicate_json": string(predicate), "action_json": string(action), "priority": input.Priority, "starts_at": input.StartsAt, "ends_at": input.EndsAt, "is_active": input.IsActive, "updated_by": actor, "updated_at": now, "version": row.Version + 1}).Error; err != nil {
			return err
		}
		result = rules[1]
		result.ID = id
		result.Version = before.Version + 1
		result.UpdatedBy = actor
		result.CreatedAt = before.CreatedAt
		result.UpdatedAt = now
		return s.recordMerchandisingAudit(tx, id, "update", actor, &before, &result)
	})
	if err != nil {
		return MerchandisingRule{}, configurationWriteError(err)
	}
	return result, nil
}
func (s *Service) DeleteMerchandisingRule(ctx context.Context, id uint, actor *uint) error {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var row models.SearchMerchandisingRule
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, id).Error; err != nil {
			return configurationLookupError(err)
		}
		before, err := merchandisingRuleFromModel(row)
		if err != nil {
			return err
		}
		rules := []MerchandisingRule{before}
		if err := hydrateMerchandisingTargets(tx, rules, false); err != nil {
			return err
		}
		if err := tx.Unscoped().Delete(&row).Error; err != nil {
			return err
		}
		return s.recordMerchandisingAudit(tx, id, "delete", actor, &rules[0], nil)
	})
}
func (s *Service) ListMerchandisingAudit(ctx context.Context, id uint) ([]MerchandisingAudit, error) {
	db, err := s.configurationDB(ctx)
	if err != nil {
		return nil, err
	}
	var rows []models.SearchMerchandisingAudit
	query := db.Order("id DESC")
	if id != 0 {
		query = query.Where("rule_id = ?", id)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 && id != 0 {
		return nil, ErrConfigurationNotFound
	}
	result := make([]MerchandisingAudit, 0, len(rows))
	for _, row := range rows {
		entry := MerchandisingAudit{ID: row.ID, RuleID: row.RuleID, Operation: row.Operation, ActorID: row.ActorID, CreatedAt: row.CreatedAt}
		if row.BeforeJSON != nil {
			entry.Before = &MerchandisingRule{}
			if err := json.Unmarshal([]byte(*row.BeforeJSON), entry.Before); err != nil {
				return nil, err
			}
		}
		if row.AfterJSON != nil {
			entry.After = &MerchandisingRule{}
			if err := json.Unmarshal([]byte(*row.AfterJSON), entry.After); err != nil {
				return nil, err
			}
		}
		result = append(result, entry)
	}
	return result, nil
}
func (s *Service) PreviewMerchandising(ctx context.Context, input MerchandisingPreviewInput) (MerchandisingPreview, error) {
	if input.Rules != nil {
		if len(*input.Rules) > 500 {
			return MerchandisingPreview{}, fmt.Errorf("%w: preview supports at most 500 rules", ErrConfigurationInvalid)
		}
		overrides := make([]MerchandisingRule, 0, len(*input.Rules))
		seen := map[uint]bool{}
		for _, rule := range *input.Rules {
			if rule.ID != 0 && seen[rule.ID] {
				return MerchandisingPreview{}, fmt.Errorf("%w: duplicate preview rule ID", ErrConfigurationInvalid)
			}
			seen[rule.ID] = true
			normalized, err := normalizeMerchandisingRule(rule.MerchandisingRuleInput)
			if err != nil {
				return MerchandisingPreview{}, err
			}
			rule.MerchandisingRuleInput = normalized
			overrides = append(overrides, rule)
		}
		db, err := s.configurationDB(ctx)
		if err != nil {
			return MerchandisingPreview{}, err
		}
		if err := hydrateMerchandisingTargets(db, overrides, true); err != nil {
			return MerchandisingPreview{}, err
		}
		input.Rules = &overrides
	}
	baselineFilters := input.Filters
	baselineFilters.Explain = true
	baselineFilters.SkipMerchandising = true
	baseline, err := s.Search(ctx, baselineFilters)
	if err != nil {
		return MerchandisingPreview{}, err
	}
	proposedFilters := input.Filters
	proposedFilters.Explain = true
	proposedFilters.SkipMerchandising = false
	proposedFilters.RuleOverrides = input.Rules
	proposedFilters.MerchandisingAt = input.At
	proposed, err := s.Search(ctx, proposedFilters)
	if err != nil {
		return MerchandisingPreview{}, err
	}
	return MerchandisingPreview{Baseline: baseline, Proposed: proposed}, nil
}
