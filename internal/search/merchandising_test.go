package search

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"ecommerce/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func merchandisingTestService(t *testing.T) (*gorm.DB, *Service, time.Time) {
	t.Helper()
	db := searchTestDB(t)
	service := NewService(db, nil, nil)
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	service.backend.(*databaseBackend).now = service.now
	fixtures := []struct {
		id                           uint
		name, brand, category, color string
		price                        float64
		available                    bool
	}{
		{1, "Red Trail Shoe", "north", "trail", "red", 20, true},
		{2, "Blue Trail Shoe", "south", "trail", "blue", 40, false},
		{3, "Cotton Shirt", "north", "shirts", "white", 15, true},
		{4, "Canvas Bag", "north", "bags", "red", 10, true},
	}
	for _, fixture := range fixtures {
		require.NoError(t, db.Create(&models.Product{BaseModel: models.BaseModel{ID: fixture.id}, SKU: fmt.Sprintf("merch-%d", fixture.id), Name: fixture.name, IsPublished: true}).Error)
		indexedFixture(t, db, fixture.id, fixture.name, fixture.brand, fixture.category, fixture.color, fixture.price, fixture.available)
	}
	profiles, err := service.ListRankingProfiles(context.Background())
	require.NoError(t, err)
	weights := RankingWeights{TokenCoverage: 1}
	_, err = service.UpdateRankingProfile(context.Background(), profiles[0].ID, RankingProfilePatch{Weights: &weights}, nil)
	require.NoError(t, err)
	return db, service, now
}
func merchandisingInput(name, kind string, ids ...uint) MerchandisingRuleInput {
	input := MerchandisingRuleInput{Name: name, RuleType: kind, IsActive: true}
	for i, id := range ids {
		target := MerchandisingTarget{ProductID: id}
		if kind == "pin" {
			position := i + 1
			target.Position = &position
		}
		input.Action.Targets = append(input.Action.Targets, target)
	}
	if kind == "boost" {
		value := 2.0
		input.Action.Multiplier = &value
	}
	return input
}
func createMerchandising(t *testing.T, service *Service, input MerchandisingRuleInput) MerchandisingRule {
	t.Helper()
	value, err := service.CreateMerchandisingRule(context.Background(), input, nil)
	require.NoError(t, err)
	return value
}
func merchandisingIDs(result Result) []uint {
	ids := make([]uint, 0, len(result.Products))
	for _, product := range result.Products {
		ids = append(ids, product.ID)
	}
	return ids
}
func searchMerchandising(t *testing.T, service *Service, filters Filters) Result {
	t.Helper()
	if filters.Limit == 0 {
		filters.Limit = 100
	}
	filters.Explain = true
	result, err := service.Search(context.Background(), filters)
	require.NoError(t, err)
	return result
}
func merchandisingDecision(t *testing.T, result Result, ruleID, productID uint) RuleDecision {
	t.Helper()
	for _, decision := range result.RuleDecisions {
		if decision.RuleID == ruleID && decision.ProductID == productID {
			return decision
		}
	}
	t.Fatalf("missing decision for rule %d product %d: %+v", ruleID, productID, result.RuleDecisions)
	return RuleDecision{}
}
func merchandisingCounts(t *testing.T, db *gorm.DB) (int64, int64) {
	t.Helper()
	var rules, audits int64
	require.NoError(t, db.Model(&models.SearchMerchandisingRule{}).Count(&rules).Error)
	require.NoError(t, db.Model(&models.SearchMerchandisingAudit{}).Count(&audits).Error)
	return rules, audits
}

func TestMerchandisingCRUDRetainsTypedAuditSnapshotsAndExplicitDisabledValue(t *testing.T) {
	db, service, now := merchandisingTestService(t)
	ctx := context.Background()
	actor := uint(99)
	input := merchandisingInput(" Disabled campaign ", "pin", 2)
	input.IsActive = false
	start := now.Add(-time.Hour)
	end := now.Add(time.Hour)
	input.StartsAt = &start
	input.EndsAt = &end
	created, err := service.CreateMerchandisingRule(ctx, input, &actor)
	require.NoError(t, err)
	require.Equal(t, "Disabled campaign", created.Name)
	require.False(t, created.IsActive)
	require.Equal(t, 1, created.Version)
	require.Equal(t, &actor, created.UpdatedBy)
	require.Equal(t, "Blue Trail Shoe", created.Action.Targets[0].ProductName)
	stored, err := service.GetMerchandisingRule(ctx, created.ID)
	require.NoError(t, err)
	require.False(t, stored.IsActive)
	audit, err := service.ListMerchandisingAudit(ctx, created.ID)
	require.NoError(t, err)
	require.Len(t, audit, 1)
	require.Equal(t, "create", audit[0].Operation)
	require.Nil(t, audit[0].Before)
	expectedJSON, err := json.Marshal(created)
	require.NoError(t, err)
	actualJSON, err := json.Marshal(audit[0].After)
	require.NoError(t, err)
	require.JSONEq(t, string(expectedJSON), string(actualJSON))
	require.Equal(t, &actor, audit[0].ActorID)
	enabled := true
	newName := "Enabled campaign"
	updated, err := service.UpdateMerchandisingRule(ctx, created.ID, MerchandisingRulePatch{Name: &newName, IsActive: &enabled, ClearStartsAt: true, ClearEndsAt: true}, &actor)
	require.NoError(t, err)
	require.Equal(t, 2, updated.Version)
	require.True(t, updated.IsActive)
	require.Nil(t, updated.StartsAt)
	require.Nil(t, updated.EndsAt)
	require.NoError(t, db.Model(&models.Product{}).Where("id = ?", 2).Update("name", "Renamed Shoe").Error)
	current, err := service.GetMerchandisingRule(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "Renamed Shoe", current.Action.Targets[0].ProductName)
	require.NoError(t, service.DeleteMerchandisingRule(ctx, created.ID, &actor))
	_, err = service.GetMerchandisingRule(ctx, created.ID)
	require.ErrorIs(t, err, ErrConfigurationNotFound)
	audit, err = service.ListMerchandisingAudit(ctx, created.ID)
	require.NoError(t, err)
	require.Len(t, audit, 3)
	require.Equal(t, "delete", audit[0].Operation)
	require.Equal(t, 2, audit[0].Before.Version)
	require.Equal(t, "Renamed Shoe", audit[0].Before.Action.Targets[0].ProductName)
	require.Nil(t, audit[0].After)
	require.Equal(t, "update", audit[1].Operation)
	require.Equal(t, 1, audit[1].Before.Version)
	require.Equal(t, 2, audit[1].After.Version)
	require.False(t, audit[1].Before.IsActive)
	require.True(t, audit[1].After.IsActive)
	require.Equal(t, "Blue Trail Shoe", audit[2].After.Action.Targets[0].ProductName, "later product renames must not rewrite history")
	for _, entry := range audit {
		require.Equal(t, &actor, entry.ActorID)
		require.Equal(t, now, entry.CreatedAt)
	}
	var ruleCount int64
	require.NoError(t, db.Unscoped().Model(&models.SearchMerchandisingRule{}).Count(&ruleCount).Error)
	require.Zero(t, ruleCount, "rule delete is physical")
	_, err = service.ListMerchandisingAudit(ctx, 9999)
	require.ErrorIs(t, err, ErrConfigurationNotFound)
}

func TestMerchandisingMutationsRollBackWhenAuditInsertFails(t *testing.T) {
	db, service, _ := merchandisingTestService(t)
	ctx := context.Background()
	created := createMerchandising(t, service, merchandisingInput("retained", "hide", 1))
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_merch_audit BEFORE INSERT ON search_merchandising_audits BEGIN SELECT RAISE(ABORT, 'audit unavailable'); END`).Error)
	_, err := service.CreateMerchandisingRule(ctx, merchandisingInput("must roll back", "pin", 2), nil)
	require.Error(t, err)
	name := "must not persist"
	_, err = service.UpdateMerchandisingRule(ctx, created.ID, MerchandisingRulePatch{Name: &name}, nil)
	require.Error(t, err)
	err = service.DeleteMerchandisingRule(ctx, created.ID, nil)
	require.Error(t, err)
	rule, err := service.GetMerchandisingRule(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.Name, rule.Name)
	require.Equal(t, 1, rule.Version)
	rules, audits := merchandisingCounts(t, db)
	require.EqualValues(t, 1, rules)
	require.EqualValues(t, 1, audits)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = service.CreateMerchandisingRule(canceled, merchandisingInput("canceled", "hide", 1), nil)
	require.ErrorIs(t, err, context.Canceled)
}

func TestMerchandisingValidationRejectsInvalidActionsAndStateWithoutWrites(t *testing.T) {
	db, service, now := merchandisingTestService(t)
	for _, test := range []struct {
		name   string
		mutate func(*MerchandisingRuleInput)
	}{
		{"empty name", func(v *MerchandisingRuleInput) { v.Name = "  " }},
		{"long name", func(v *MerchandisingRuleInput) { v.Name = strings.Repeat("x", 121) }},
		{"too many contexts", func(v *MerchandisingRuleInput) {
			for i := 0; i < 21; i++ {
				v.Predicate.CategorySlugs = append(v.Predicate.CategorySlugs, fmt.Sprintf("c%d", i))
			}
		}},
		{"unknown action", func(v *MerchandisingRuleInput) { v.RuleType = "discount" }},
		{"no targets", func(v *MerchandisingRuleInput) { v.Action.Targets = nil }},
		{"zero target", func(v *MerchandisingRuleInput) { v.Action.Targets[0].ProductID = 0 }},
		{"missing product", func(v *MerchandisingRuleInput) { v.Action.Targets[0].ProductID = 9999 }},
		{"duplicate targets", func(v *MerchandisingRuleInput) { v.Action.Targets = append(v.Action.Targets, v.Action.Targets[0]) }},
		{"negative priority", func(v *MerchandisingRuleInput) { v.Priority = -1 }},
		{"large priority", func(v *MerchandisingRuleInput) { v.Priority = 10001 }},
		{"bad window", func(v *MerchandisingRuleInput) { v.StartsAt = &now; v.EndsAt = &now }},
		{"unsupported channel", func(v *MerchandisingRuleInput) { v.Predicate.Channel = "cms" }},
		{"empty query", func(v *MerchandisingRuleInput) { v.Predicate.Query = &MerchandisingQuery{Mode: "exact", Value: "!!!"} }},
		{"unknown query mode", func(v *MerchandisingRuleInput) { v.Predicate.Query = &MerchandisingQuery{Mode: "regex", Value: "shoe"} }},
		{"pin no position", func(v *MerchandisingRuleInput) { v.RuleType = "pin" }},
		{"position on hide", func(v *MerchandisingRuleInput) { position := 1; v.Action.Targets[0].Position = &position }},
		{"multiplier on hide", func(v *MerchandisingRuleInput) { multiplier := 2.0; v.Action.Multiplier = &multiplier }},
		{"boost missing multiplier", func(v *MerchandisingRuleInput) { v.RuleType = "boost" }},
		{"boost one", func(v *MerchandisingRuleInput) {
			v.RuleType = "boost"
			multiplier := 1.0
			v.Action.Multiplier = &multiplier
		}},
		{"boost infinite", func(v *MerchandisingRuleInput) {
			v.RuleType = "boost"
			multiplier := math.Inf(1)
			v.Action.Multiplier = &multiplier
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := merchandisingInput("invalid", "hide", 1)
			test.mutate(&input)
			_, err := service.CreateMerchandisingRule(context.Background(), input, nil)
			require.ErrorIs(t, err, ErrConfigurationInvalid)
		})
	}
	for _, multiplier := range []float64{0, 1, 100.01, math.NaN()} {
		input := merchandisingInput("invalid boost", "boost", 1)
		input.Action.Multiplier = &multiplier
		_, err := service.CreateMerchandisingRule(context.Background(), input, nil)
		require.ErrorIs(t, err, ErrConfigurationInvalid)
	}

	for _, position := range []int{0, 10001} {
		input := merchandisingInput("bad pin", "pin", 1)
		input.Action.Targets[0].Position = &position
		_, err := service.CreateMerchandisingRule(context.Background(), input, nil)
		require.ErrorIs(t, err, ErrConfigurationInvalid)
	}
	duplicate := merchandisingInput("bad positions", "pin", 1, 2)
	duplicate.Action.Targets[1].Position = duplicate.Action.Targets[0].Position
	_, err := service.CreateMerchandisingRule(context.Background(), duplicate, nil)
	require.ErrorIs(t, err, ErrConfigurationInvalid)
	tooMany := merchandisingInput("too many", "hide", 1)
	tooMany.Action.Targets = make([]MerchandisingTarget, 101)
	_, err = service.CreateMerchandisingRule(context.Background(), tooMany, nil)
	require.ErrorIs(t, err, ErrConfigurationInvalid)
	rules, audits := merchandisingCounts(t, db)
	require.Zero(t, rules)
	require.Zero(t, audits)

	boundary := merchandisingInput("boundary valid", "pin", 1)
	boundary.Priority = 10000
	position := 10000
	boundary.Action.Targets[0].Position = &position
	boundaryRule := createMerchandising(t, service, boundary)
	require.Equal(t, 10000, boundaryRule.Priority)
	require.Equal(t, 10000, *boundaryRule.Action.Targets[0].Position)
	created := createMerchandising(t, service, merchandisingInput("valid", "hide", 1))
	for _, patch := range []MerchandisingRulePatch{{}, {ClearStartsAt: true, StartsAt: &now}, {ClearEndsAt: true, EndsAt: &now}} {
		_, err := service.UpdateMerchandisingRule(context.Background(), created.ID, patch, nil)
		require.ErrorIs(t, err, ErrConfigurationInvalid)
	}
	_, err = service.UpdateMerchandisingRule(context.Background(), 999, MerchandisingRulePatch{IsActive: new(bool)}, nil)
	require.ErrorIs(t, err, ErrConfigurationNotFound)
	require.ErrorIs(t, service.DeleteMerchandisingRule(context.Background(), 999, nil), ErrConfigurationNotFound)
}

func TestMerchandisingFiveActionsPrecedenceFacetsAndStablePagination(t *testing.T) {
	_, service, _ := merchandisingTestService(t)
	include := createMerchandising(t, service, merchandisingInput("include shirt", "include", 3))
	pin := createMerchandising(t, service, merchandisingInput("pin blue", "pin", 2))
	buryInput := merchandisingInput("bury red and blue", "bury", 1, 2)
	buryInput.Priority = 100
	bury := createMerchandising(t, service, buryInput)
	boostInput := merchandisingInput("boost blue", "boost", 2)
	boostInput.Priority = 200
	boost := createMerchandising(t, service, boostInput)
	result := searchMerchandising(t, service, Filters{Query: "shoe"})
	require.Equal(t, []uint{2, 3, 1}, merchandisingIDs(result))
	require.EqualValues(t, 3, result.Total)
	require.Equal(t, "applied", merchandisingDecision(t, result, include.ID, 3).Outcome)
	require.Equal(t, "applied", merchandisingDecision(t, result, pin.ID, 2).Outcome)
	require.Equal(t, "conflict", merchandisingDecision(t, result, bury.ID, 2).Outcome, "pin beats higher-priority bury")
	require.Equal(t, "conflict", merchandisingDecision(t, result, boost.ID, 2).Outcome, "pin beats higher-priority boost")
	hide := createMerchandising(t, service, merchandisingInput("hide blue", "hide", 2))
	result = searchMerchandising(t, service, Filters{Query: "shoe"})
	require.Equal(t, []uint{3, 1}, merchandisingIDs(result))
	require.EqualValues(t, 2, result.Total)
	require.Equal(t, "applied", merchandisingDecision(t, result, hide.ID, 2).Outcome)
	require.Equal(t, "conflict", merchandisingDecision(t, result, pin.ID, 2).Outcome)
	brand := facetByName(t, result.Facets, "brand")
	for _, value := range brand.Values {
		if value.Value == "south" {
			require.Zero(t, value.Count, "hidden product must not contribute to facets")
		}
	}
	require.EqualValues(t, 1, facetValueByName(t, facetByName(t, result.Facets, "category"), "shirts").Count)
	for repeat := 0; repeat < 3; repeat++ {
		ids := []uint{}
		for page := 1; page <= 2; page++ {
			result := searchMerchandising(t, service, Filters{Query: "shoe", Page: page, Limit: 1})
			require.Equal(t, 2, result.TotalPages)
			ids = append(ids, merchandisingIDs(result)...)
		}
		require.Equal(t, []uint{3, 1}, ids)
	}
}

func TestMerchandisingPinCollisionsPriorityTieBreakAndOutOfRangePins(t *testing.T) {
	_, service, _ := merchandisingTestService(t)
	first := createMerchandising(t, service, merchandisingInput("first ID wins", "pin", 2))
	losing := createMerchandising(t, service, merchandisingInput("same position loses", "pin", 1))
	result := searchMerchandising(t, service, Filters{Query: "shoe"})
	require.Equal(t, []uint{2, 1}, merchandisingIDs(result))
	require.Equal(t, "conflict", merchandisingDecision(t, result, losing.ID, 1).Outcome)
	priority := 1
	_, err := service.UpdateMerchandisingRule(context.Background(), losing.ID, MerchandisingRulePatch{Priority: &priority}, nil)
	require.NoError(t, err)
	result = searchMerchandising(t, service, Filters{Query: "shoe"})
	require.Equal(t, []uint{1, 2}, merchandisingIDs(result))
	require.Equal(t, "conflict", merchandisingDecision(t, result, first.ID, 2).Outcome)
	require.NoError(t, service.DeleteMerchandisingRule(context.Background(), losing.ID, nil))
	action := MerchandisingAction{Targets: []MerchandisingTarget{{ProductID: 2, Position: new(int)}}}
	*action.Targets[0].Position = 10000
	_, err = service.UpdateMerchandisingRule(context.Background(), first.ID, MerchandisingRulePatch{Action: &action}, nil)
	require.NoError(t, err)
	result = searchMerchandising(t, service, Filters{Query: "shoe"})
	require.Equal(t, []uint{1, 2}, merchandisingIDs(result))
	require.EqualValues(t, 2, result.Total, "far-away pins must not create gaps or lose products")
	// Persisted rules win equal-priority conflicts against unsaved preview rules.
	action.Targets[0].Position = new(int)
	*action.Targets[0].Position = 1
	_, err = service.UpdateMerchandisingRule(context.Background(), first.ID, MerchandisingRulePatch{Action: &action}, nil)
	require.NoError(t, err)
	saved, err := service.GetMerchandisingRule(context.Background(), first.ID)
	require.NoError(t, err)
	overrides := []MerchandisingRule{{MerchandisingRuleInput: merchandisingInput("unsaved", "pin", 1)}, saved}
	preview, err := service.PreviewMerchandising(context.Background(), MerchandisingPreviewInput{Filters: Filters{Query: "shoe", Limit: 10}, Rules: &overrides})
	require.NoError(t, err)
	require.Equal(t, []uint{2, 1}, merchandisingIDs(preview.Proposed))
}

func TestMerchandisingBoostDoesNotStackAndExplicitSortDisablesOrderingActions(t *testing.T) {
	_, service, _ := merchandisingTestService(t)
	winnerInput := merchandisingInput("winning boost", "boost", 2)
	winnerInput.Priority = 10
	winner := createMerchandising(t, service, winnerInput)
	losingInput := merchandisingInput("larger losing boost", "boost", 2)
	value := 100.0
	losingInput.Action.Multiplier = &value
	losing := createMerchandising(t, service, losingInput)
	result := searchMerchandising(t, service, Filters{Query: "shoe"})
	require.Equal(t, []uint{2, 1}, merchandisingIDs(result))
	require.InDelta(t, result.Explanations[0].Score*2, result.Explanations[0].AdjustedScore, 1e-12)
	require.Equal(t, "conflict", merchandisingDecision(t, result, losing.ID, 2).Outcome)
	pin := createMerchandising(t, service, merchandisingInput("manual pin", "pin", 2))
	bury := createMerchandising(t, service, merchandisingInput("manual bury", "bury", 1))
	include := createMerchandising(t, service, merchandisingInput("include shirt", "include", 3))
	hide := createMerchandising(t, service, merchandisingInput("hide red", "hide", 1))
	result = searchMerchandising(t, service, Filters{Query: "shoe", SortField: "price", SortOrder: "asc"})
	require.Equal(t, []uint{3, 2}, merchandisingIDs(result))
	require.Equal(t, "applied", merchandisingDecision(t, result, include.ID, 3).Outcome)
	require.Equal(t, "applied", merchandisingDecision(t, result, hide.ID, 1).Outcome)
	for _, rule := range []MerchandisingRule{winner, pin} {
		decision := merchandisingDecision(t, result, rule.ID, 2)
		require.Equal(t, "skipped", decision.Outcome)
		require.Contains(t, decision.Reason, "Explicit sorting")
	}
	require.Equal(t, "conflict", merchandisingDecision(t, result, bury.ID, 1).Outcome, "hide still wins when explicit sorting is used")
	for _, explanation := range result.Explanations {
		require.Equal(t, explanation.Score, explanation.AdjustedScore)
	}
}

func TestMerchandisingIncludePreservesAllFiltersAndIndexEligibility(t *testing.T) {
	db, service, _ := merchandisingTestService(t)
	for _, id := range []uint{5, 6} {
		require.NoError(t, db.Create(&models.Product{BaseModel: models.BaseModel{ID: id}, SKU: fmt.Sprintf("excluded-%d", id), Name: "Excluded", IsPublished: true}).Error)
	}
	require.NoError(t, db.Model(&models.Product{}).Where("id = ?", 5).Update("is_published", false).Error)
	// A defensive check also rejects malformed active indexed payloads for drafts.
	product := models.Product{BaseModel: models.BaseModel{ID: 5}, Name: "Draft", IsPublished: false}
	payload, err := json.Marshal(product)
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.SearchDocument{EntityType: ProductEntityType, EntityID: 5, PayloadJSON: string(payload), SearchableText: "draft", NormalizedName: "draft", Active: true}).Error)
	createMerchandising(t, service, merchandisingInput("include catalog targets", "include", 2, 3, 4, 5, 6))
	available := true
	max := 25.0
	for _, test := range []struct {
		name    string
		filters Filters
		want    []uint
	}{
		{"brand", Filters{Query: "shoe", BrandSlugs: []string{"north"}}, []uint{1, 3, 4}},
		{"category", Filters{Query: "shoe", CategorySlugs: []string{"trail"}}, []uint{1, 2}},
		{"attribute", Filters{Query: "shoe", AttributeValues: map[string][]string{"color": {"red"}}}, []uint{1, 4}},
		{"price", Filters{Query: "shoe", MaxPrice: &max}, []uint{1, 3, 4}},
		{"stock", Filters{Query: "shoe", HasVariantStock: &available}, []uint{1, 3, 4}},
		{"all", Filters{Query: "shoe", BrandSlugs: []string{"north"}, CategorySlugs: []string{"bags"}, AttributeValues: map[string][]string{"color": {"red"}}, HasVariantStock: &available, MaxPrice: &max}, []uint{4}},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := searchMerchandising(t, service, test.filters)
			require.Equal(t, test.want, merchandisingIDs(result))
			for _, id := range merchandisingIDs(result) {
				require.NotContains(t, []uint{5, 6}, id)
			}
		})
	}
	// Include cannot resurrect a tombstoned document even if the catalog row exists.
	require.NoError(t, db.Model(&models.SearchDocument{}).Where("entity_id = ?", 4).Update("active", false).Error)
	result := searchMerchandising(t, service, Filters{Query: "shoe"})
	require.Equal(t, []uint{1, 2, 3}, merchandisingIDs(result))
}

func TestMerchandisingSchedulesPredicatesAndOriginalQueryAfterTypoCorrection(t *testing.T) {
	_, service, now := merchandisingTestService(t)
	start := now
	end := now.Add(time.Hour)
	input := merchandisingInput("timed contextual", "hide", 1)
	input.StartsAt = &start
	input.EndsAt = &end
	input.Predicate = MerchandisingPredicate{Query: &MerchandisingQuery{Mode: "prefix", Value: "TRAIL"}, CategorySlugs: []string{"city", "trail"}, Channel: "storefront"}
	rule := createMerchandising(t, service, input)
	for _, test := range []struct {
		name, query, category, channel string
		at                             time.Time
		hidden                         bool
	}{
		{"before", "trail shoe", "trail", "", now.Add(-time.Nanosecond), false},
		{"start inclusive", "trail shoe", "trail", "", now, true},
		{"end exclusive", "trail shoe", "trail", "", end, false},
		{"query mismatch", "shoe", "trail", "", now, false},
		{"category mismatch", "trail shoe", "shirts", "", now, false},
		{"missing category context", "trail shoe", "", "", now, false},
		{"channel mismatch", "trail shoe", "trail", "cms", now, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			filters := Filters{Query: test.query, Channel: test.channel, MerchandisingAt: &test.at}
			if test.category != "" {
				filters.CategorySlugs = []string{test.category}
			}
			result := searchMerchandising(t, service, filters)
			var decision RuleDecision
			if test.hidden {
				decision = merchandisingDecision(t, result, rule.ID, 1)
				require.Equal(t, "applied", decision.Outcome)
			} else {
				decision = merchandisingDecision(t, result, rule.ID, 0)
				require.Equal(t, "skipped", decision.Outcome)
			}
		})
	}
	// Corrected text retrieves the shoe, but campaign matching uses the original typo.
	exact := merchandisingInput("exact shoe", "hide", 1)
	exact.Predicate.Query = &MerchandisingQuery{Mode: "exact", Value: "trail"}
	exactRule := createMerchandising(t, service, exact)
	result := searchMerchandising(t, service, Filters{Query: "traii"})
	require.Equal(t, "trail", result.DidYouMean)
	require.Contains(t, merchandisingIDs(result), uint(1))
	require.Equal(t, "skipped", merchandisingDecision(t, result, exactRule.ID, 0).Outcome)
	typo := merchandisingInput("original typo campaign", "hide", 1)
	typo.Predicate.Query = &MerchandisingQuery{Mode: "exact", Value: "traii"}
	typoRule := createMerchandising(t, service, typo)
	result = searchMerchandising(t, service, Filters{Query: "traii"})
	require.NotContains(t, merchandisingIDs(result), uint(1))
	require.Equal(t, "applied", merchandisingDecision(t, result, typoRule.ID, 1).Outcome)
}

func TestMerchandisingPreviewUsesProductionPipelineWithoutWrites(t *testing.T) {
	db, service, now := merchandisingTestService(t)
	ctx := context.Background()
	createMerchandising(t, service, merchandisingInput("persisted hide", "hide", 1))
	rulesBefore, auditsBefore := merchandisingCounts(t, db)
	persistedBefore, err := service.ListMerchandisingRules(ctx)
	require.NoError(t, err)
	preview, err := service.PreviewMerchandising(ctx, MerchandisingPreviewInput{Filters: Filters{Query: "shoe", Limit: 10}})
	require.NoError(t, err)
	require.Equal(t, []uint{1, 2}, merchandisingIDs(preview.Baseline))
	require.Equal(t, []uint{2}, merchandisingIDs(preview.Proposed))
	require.NotEmpty(t, preview.Proposed.Explanations)
	empty := []MerchandisingRule{}
	preview, err = service.PreviewMerchandising(ctx, MerchandisingPreviewInput{Filters: Filters{Query: "shoe", Limit: 10}, Rules: &empty})
	require.NoError(t, err)
	require.Equal(t, merchandisingIDs(preview.Baseline), merchandisingIDs(preview.Proposed))
	future := now.Add(time.Hour)
	draft := merchandisingInput("unsaved include", "include", 3)
	draft.StartsAt = &future
	overrides := []MerchandisingRule{{MerchandisingRuleInput: draft}}
	preview, err = service.PreviewMerchandising(ctx, MerchandisingPreviewInput{Filters: Filters{Query: "shoe", Limit: 10}, Rules: &overrides, At: &future})
	require.NoError(t, err)
	require.Equal(t, []uint{1, 2, 3}, merchandisingIDs(preview.Proposed))
	require.Equal(t, []uint{1, 2}, merchandisingIDs(preview.Baseline))
	require.Equal(t, uint(0), preview.Proposed.RuleDecisions[0].RuleID)
	require.Equal(t, preview.Baseline.Explanations[0].Score, preview.Proposed.Explanations[0].Score, "simulated campaign time must not change ranking signals")
	invalid := []MerchandisingRule{{MerchandisingRuleInput: merchandisingInput("missing target", "hide", 9999)}}
	_, err = service.PreviewMerchandising(ctx, MerchandisingPreviewInput{Filters: Filters{Query: "shoe"}, Rules: &invalid})
	require.ErrorIs(t, err, ErrConfigurationInvalid)
	duplicate := []MerchandisingRule{{ID: 123, MerchandisingRuleInput: merchandisingInput("one", "hide", 1)}, {ID: 123, MerchandisingRuleInput: merchandisingInput("two", "hide", 2)}}
	_, err = service.PreviewMerchandising(ctx, MerchandisingPreviewInput{Filters: Filters{Query: "shoe"}, Rules: &duplicate})
	require.ErrorIs(t, err, ErrConfigurationInvalid)
	rulesAfter, auditsAfter := merchandisingCounts(t, db)
	require.Equal(t, rulesBefore, rulesAfter)
	require.Equal(t, auditsBefore, auditsAfter)
	persistedAfter, err := service.ListMerchandisingRules(ctx)
	require.NoError(t, err)
	require.Equal(t, persistedBefore, persistedAfter, "preview must not change versions or stored rule configuration")
	require.Equal(t, []uint{2}, merchandisingIDs(searchMerchandising(t, service, Filters{Query: "shoe"})), "preview must not publish its replacement rules")
}

func TestMerchandisingMalformedReservedConfigurationFailsClosed(t *testing.T) {
	db, service, _ := merchandisingTestService(t)
	require.NoError(t, db.Create(&models.SearchMerchandisingRule{Name: "reserved legacy", RuleType: "hide", PredicateJSON: `{`, ActionJSON: `{}`, IsActive: true}).Error)
	_, err := service.Search(context.Background(), Filters{Query: "shoe"})
	require.Error(t, err)
	_, err = service.ListMerchandisingRules(context.Background())
	require.Error(t, err)
	// Baseline preview deliberately bypasses rules, but proposed live retrieval fails.
	_, err = service.PreviewMerchandising(context.Background(), MerchandisingPreviewInput{Filters: Filters{Query: "shoe"}})
	require.Error(t, err)
}

func TestMerchandisingDisabledContainsPredicatesAndDefaultBrowseOrdering(t *testing.T) {
	_, service, _ := merchandisingTestService(t)
	disabled := merchandisingInput("disabled", "hide", 1)
	disabled.IsActive = false
	rule := createMerchandising(t, service, disabled)
	result := searchMerchandising(t, service, Filters{Query: "shoe"})
	require.Equal(t, []uint{1, 2}, merchandisingIDs(result))
	require.Equal(t, "skipped", merchandisingDecision(t, result, rule.ID, 0).Outcome)
	contains := merchandisingInput("contains normalized text", "hide", 2)
	contains.Predicate.Query = &MerchandisingQuery{Mode: "contains", Value: "SHOE"}
	containsRule := createMerchandising(t, service, contains)
	result = searchMerchandising(t, service, Filters{Query: "  TRAIL—Shoe! "})
	require.Equal(t, []uint{1}, merchandisingIDs(result))
	require.Equal(t, "applied", merchandisingDecision(t, result, containsRule.ID, 2).Outcome)
	require.NoError(t, service.DeleteMerchandisingRule(context.Background(), containsRule.ID, nil))
	createMerchandising(t, service, merchandisingInput("pin browse", "pin", 2))
	createMerchandising(t, service, merchandisingInput("bury browse", "bury", 1))
	result = searchMerchandising(t, service, Filters{})
	require.Equal(t, []uint{2, 3, 4, 1}, merchandisingIDs(result))
	result = searchMerchandising(t, service, Filters{SortField: "created_at"})
	require.Equal(t, []uint{1, 2, 3, 4}, merchandisingIDs(result), "explicit newest order suppresses pin/bury")
}

func TestMerchandisingSynonymPredicatesUseOriginalNormalizedQuery(t *testing.T) {
	_, service, _ := merchandisingTestService(t)
	_, err := service.CreateSynonymSet(context.Background(), SynonymSetInput{Name: "footwear", Direction: SynonymDirectionUnidirectional, Terms: []string{"sneaker", "shoe"}, IsActive: true})
	require.NoError(t, err)
	expanded := merchandisingInput("expanded text must not match", "hide", 1)
	expanded.Predicate.Query = &MerchandisingQuery{Mode: "exact", Value: "shoe"}
	expandedRule := createMerchandising(t, service, expanded)
	result := searchMerchandising(t, service, Filters{Query: "SNEAKER!"})
	require.Equal(t, []uint{1, 2}, merchandisingIDs(result))
	require.NotEmpty(t, result.AppliedRewrites)
	require.Equal(t, "skipped", merchandisingDecision(t, result, expandedRule.ID, 0).Outcome)
	original := merchandisingInput("shopper text matches", "hide", 1)
	original.Predicate.Query = &MerchandisingQuery{Mode: "exact", Value: "sneaker"}
	originalRule := createMerchandising(t, service, original)
	result = searchMerchandising(t, service, Filters{Query: "SNEAKER!"})
	require.Equal(t, []uint{2}, merchandisingIDs(result))
	require.Equal(t, "applied", merchandisingDecision(t, result, originalRule.ID, 1).Outcome)
}

func TestMerchandisingOutOfRangePinKeepsOrdinaryOrderAroundValidSlots(t *testing.T) {
	_, service, _ := merchandisingTestService(t)
	createMerchandising(t, service, merchandisingInput("include bag", "include", 4))
	valid := merchandisingInput("blue at third", "pin", 2)
	third := 3
	valid.Action.Targets[0].Position = &third
	createMerchandising(t, service, valid)
	beyond := merchandisingInput("bag beyond results", "pin", 4)
	fifth := 5
	beyond.Action.Targets[0].Position = &fifth
	rule := createMerchandising(t, service, beyond)
	result := searchMerchandising(t, service, Filters{Query: "shoe"})
	require.Equal(t, []uint{1, 4, 2}, merchandisingIDs(result), "out-of-range pin keeps ordinary position while valid pin occupies its exact slot")
	decision := merchandisingDecision(t, result, rule.ID, 4)
	require.Equal(t, "skipped", decision.Outcome)
	require.Contains(t, strings.ToLower(decision.Reason), "result")
}

func TestMerchandisingCategorySlugRoundTripKeepsCanonicalHyphens(t *testing.T) {
	_, service, _ := merchandisingTestService(t)
	input := merchandisingInput("travel context", "hide", 1)
	input.Predicate.CategorySlugs = []string{" Travel-Bags "}
	created := createMerchandising(t, service, input)
	require.Equal(t, []string{"travel-bags"}, created.Predicate.CategorySlugs)
	fetched, err := service.GetMerchandisingRule(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"travel-bags"}, fetched.Predicate.CategorySlugs)
	// Category context is the request selection, so an empty result still has an
	// applied hide decision after canonical slug normalization.
	result := searchMerchandising(t, service, Filters{Query: "shoe", CategorySlugs: []string{"travel-bags"}})
	require.Equal(t, "applied", merchandisingDecision(t, result, created.ID, 1).Outcome)
}

func TestMerchandisingGlobalAuditListsRetainedDeletedRulesAndEmptyHistory(t *testing.T) {
	_, service, _ := merchandisingTestService(t)
	ctx := context.Background()
	empty, err := service.ListMerchandisingAudit(ctx, 0)
	require.NoError(t, err)
	require.NotNil(t, empty)
	require.Empty(t, empty)
	one := createMerchandising(t, service, merchandisingInput("will be deleted", "hide", 1))
	two := createMerchandising(t, service, merchandisingInput("remains", "hide", 2))
	require.NoError(t, service.DeleteMerchandisingRule(ctx, one.ID, nil))
	global, err := service.ListMerchandisingAudit(ctx, 0)
	require.NoError(t, err)
	require.Len(t, global, 3)
	require.Equal(t, one.ID, global[0].RuleID)
	require.Equal(t, "delete", global[0].Operation)
	require.Equal(t, "will be deleted", global[0].Before.Name)
	require.Equal(t, two.ID, global[1].RuleID)
	require.Equal(t, one.ID, global[2].RuleID)
	require.Greater(t, global[0].ID, global[1].ID)
	require.Greater(t, global[1].ID, global[2].ID)
	retained, err := service.ListMerchandisingAudit(ctx, one.ID)
	require.NoError(t, err)
	require.Len(t, retained, 2)
}

func TestMerchandisingHideDecisionsSkipIneligibleTargetsAndReportIncludedCandidates(t *testing.T) {
	db, service, _ := merchandisingTestService(t)
	for _, id := range []uint{5, 6} {
		require.NoError(t, db.Create(&models.Product{BaseModel: models.BaseModel{ID: id}, SKU: fmt.Sprintf("hide-excluded-%d", id), Name: "Excluded", IsPublished: true}).Error)
	}
	require.NoError(t, db.Model(&models.Product{}).Where("id = ?", 5).Update("is_published", false).Error)
	draft := models.Product{BaseModel: models.BaseModel{ID: 5}, Name: "Draft", IsPublished: false}
	payload, err := json.Marshal(draft)
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.SearchDocument{EntityType: ProductEntityType, EntityID: 5, PayloadJSON: string(payload), SearchableText: "draft", NormalizedName: "draft", Active: true}).Error)
	hide := createMerchandising(t, service, merchandisingInput("ineligible hides", "hide", 3, 5, 6))
	result := searchMerchandising(t, service, Filters{Query: "shoe"})
	require.Equal(t, []uint{1, 2}, merchandisingIDs(result))
	for _, id := range []uint{3, 5, 6} {
		require.Equal(t, "skipped", merchandisingDecision(t, result, hide.ID, id).Outcome)
	}
	// A text miss made eligible by include must still be excluded by hide, while
	// diagnostics record the actual include/hide conflict rather than a phantom.
	include := createMerchandising(t, service, merchandisingInput("include shirt", "include", 3))
	result = searchMerchandising(t, service, Filters{Query: "impossible"})
	require.Zero(t, result.Total)
	require.Empty(t, result.Products)
	require.Equal(t, "applied", merchandisingDecision(t, result, hide.ID, 3).Outcome)
	require.Equal(t, "conflict", merchandisingDecision(t, result, include.ID, 3).Outcome)
	for _, id := range []uint{5, 6} {
		require.Equal(t, "skipped", merchandisingDecision(t, result, hide.ID, id).Outcome)
	}
}
