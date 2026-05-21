package discounts

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"ecommerce/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newDiscountTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.DiscountCampaign{},
		&models.DiscountRule{},
		&models.DiscountLevel{},
		&models.DiscountTarget{},
		&models.DiscountSchedule{},
		&models.DiscountStateHistory{},
		&models.DiscountCampaignAudit{},
		&models.DiscountRedemption{},
		&models.PromotionTemplate{},
	))
	return db
}

func TestPricesForProductsAppliesActiveProductDiscount(t *testing.T) {
	db := newDiscountTestDB(t)
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)

	_, err := CreateProductDiscount(db, ProductDiscountInput{
		Name:          "Launch sale",
		ProductIDs:    []uint{10},
		DiscountMode:  models.DiscountModePercent,
		DiscountValue: models.MoneyFromFloat(25),
		StartsAt:      now.Add(-time.Hour),
		EndsAt:        timePtr(now.Add(time.Hour)),
	})
	require.NoError(t, err)

	prices, err := PricesForProducts(db, map[uint]models.Money{10: models.MoneyFromFloat(20)}, now)
	require.NoError(t, err)

	require.Equal(t, 20.0, prices[10].BasePrice.Float64())
	require.Equal(t, 5.0, prices[10].DiscountAmount.Float64())
	require.Equal(t, 15.0, prices[10].FinalPrice.Float64())
	require.Len(t, prices[10].AppliedCampaigns, 1)
}

func TestPricesForProductsIgnoresExpiredFutureAndDisabledDiscounts(t *testing.T) {
	db := newDiscountTestDB(t)
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)

	for _, input := range []ProductDiscountInput{
		{
			Name:          "Expired",
			ProductIDs:    []uint{10},
			DiscountMode:  models.DiscountModeFixed,
			DiscountValue: models.MoneyFromFloat(3),
			StartsAt:      now.Add(-2 * time.Hour),
			EndsAt:        timePtr(now.Add(-time.Hour)),
		},
		{
			Name:          "Future",
			ProductIDs:    []uint{10},
			DiscountMode:  models.DiscountModeFixed,
			DiscountValue: models.MoneyFromFloat(4),
			StartsAt:      now.Add(time.Hour),
		},
		{
			Name:          "Disabled",
			ProductIDs:    []uint{10},
			DiscountMode:  models.DiscountModeFixed,
			DiscountValue: models.MoneyFromFloat(5),
			StartsAt:      now.Add(-time.Hour),
			Status:        models.DiscountCampaignStatusDisabled,
		},
	} {
		_, err := CreateProductDiscount(db, input)
		require.NoError(t, err)
	}

	prices, err := PricesForProducts(db, map[uint]models.Money{10: models.MoneyFromFloat(20)}, now)
	require.NoError(t, err)
	require.Equal(t, 0.0, prices[10].DiscountAmount.Float64())
	require.Equal(t, 20.0, prices[10].FinalPrice.Float64())
	require.Empty(t, prices[10].AppliedCampaigns)
}

func TestPricesForProductsUsesHighestPriorityDiscount(t *testing.T) {
	db := newDiscountTestDB(t)
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)

	_, err := CreateProductDiscount(db, ProductDiscountInput{
		Name:          "Lower priority",
		ProductIDs:    []uint{10},
		DiscountMode:  models.DiscountModeFixed,
		DiscountValue: models.MoneyFromFloat(2),
		StartsAt:      now.Add(-time.Hour),
		Priority:      1,
	})
	require.NoError(t, err)
	_, err = CreateProductDiscount(db, ProductDiscountInput{
		Name:          "Higher priority",
		ProductIDs:    []uint{10},
		DiscountMode:  models.DiscountModeFixed,
		DiscountValue: models.MoneyFromFloat(7),
		StartsAt:      now.Add(-time.Hour),
		Priority:      10,
	})
	require.NoError(t, err)

	prices, err := PricesForProducts(db, map[uint]models.Money{10: models.MoneyFromFloat(20)}, now)
	require.NoError(t, err)
	require.Equal(t, 7.0, prices[10].DiscountAmount.Float64())
	require.Equal(t, "Higher priority", prices[10].AppliedCampaigns[0].Name)
}

func TestValidateProductDiscountRejectsInvalidPayloads(t *testing.T) {
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)

	invalid := []ProductDiscountInput{
		{ProductIDs: []uint{1}, DiscountMode: models.DiscountModeFixed, DiscountValue: models.MoneyFromFloat(1), StartsAt: now},
		{Name: "No targets", DiscountMode: models.DiscountModeFixed, DiscountValue: models.MoneyFromFloat(1), StartsAt: now},
		{Name: "Negative", ProductIDs: []uint{1}, DiscountMode: models.DiscountModeFixed, DiscountValue: models.MoneyFromFloat(-1), StartsAt: now},
		{Name: "Too high", ProductIDs: []uint{1}, DiscountMode: models.DiscountModePercent, DiscountValue: models.MoneyFromFloat(101), StartsAt: now},
		{Name: "Bad window", ProductIDs: []uint{1}, DiscountMode: models.DiscountModeFixed, DiscountValue: models.MoneyFromFloat(1), StartsAt: now, EndsAt: timePtr(now)},
	}
	for _, input := range invalid {
		require.ErrorIs(t, ValidateProductDiscount(input), ErrInvalidCampaign)
	}
}

func TestEvaluateCartAppliesCrossProductPromotionOnlyWhenConditionMatches(t *testing.T) {
	db := newDiscountTestDB(t)
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)

	_, err := CreatePromotion(db, CreatePromotionInput{
		Name:     "Buy A get B",
		StartsAt: now.Add(-time.Hour),
		Rules: []PromotionRuleInput{
			{
				Condition: RuleCondition{ProductIDs: []uint{1}, MinQuantity: 1},
				Action: RuleAction{
					Mode:       ActionModePercent,
					Value:      models.MoneyFromFloat(50),
					TargetType: models.DiscountTargetTypeProduct,
					TargetIDs:  []uint{2},
				},
			},
		},
	})
	require.NoError(t, err)

	withoutTrigger, err := EvaluateCart(db, []CartLine{
		{ProductID: 2, ProductVariantID: 20, Quantity: 1, UnitPrice: models.MoneyFromFloat(10)},
	}, now)
	require.NoError(t, err)
	require.Equal(t, 0.0, withoutTrigger.DiscountTotal.Float64())

	withTrigger, err := EvaluateCart(db, []CartLine{
		{ProductID: 1, ProductVariantID: 10, Quantity: 1, UnitPrice: models.MoneyFromFloat(20)},
		{ProductID: 2, ProductVariantID: 20, Quantity: 1, UnitPrice: models.MoneyFromFloat(10)},
	}, now)
	require.NoError(t, err)
	require.Equal(t, 5.0, withTrigger.DiscountTotal.Float64())
	require.Equal(t, 5.0, withTrigger.Lines[1].FinalPrice.Float64())
}

func TestEvaluateCartAppliesCategoryPromotionToMatchingLines(t *testing.T) {
	db := newDiscountTestDB(t)
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)

	_, err := CreatePromotion(db, CreatePromotionInput{
		Name:     "Category sale",
		StartsAt: now.Add(-time.Hour),
		Rules: []PromotionRuleInput{
			{
				Condition: RuleCondition{MinQuantity: 1},
				Action: RuleAction{
					Mode:       ActionModeFixed,
					Value:      models.MoneyFromFloat(3),
					TargetType: models.DiscountTargetTypeCategory,
					TargetIDs:  []uint{9},
				},
			},
		},
	})
	require.NoError(t, err)

	result, err := EvaluateCart(db, []CartLine{
		{ProductID: 1, ProductVariantID: 10, CategoryIDs: []uint{9}, Quantity: 1, UnitPrice: models.MoneyFromFloat(20)},
		{ProductID: 2, ProductVariantID: 20, CategoryIDs: []uint{8}, Quantity: 1, UnitPrice: models.MoneyFromFloat(20)},
	}, now)
	require.NoError(t, err)
	require.Equal(t, 3.0, result.DiscountTotal.Float64())
	require.Equal(t, 17.0, result.Lines[0].FinalPrice.Float64())
	require.Equal(t, 20.0, result.Lines[1].FinalPrice.Float64())
}

func TestEvaluateCartAppliesPromotionLevelsByTarget(t *testing.T) {
	db := newDiscountTestDB(t)
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)

	_, err := CreatePromotion(db, CreatePromotionInput{
		Name:     "Tiered categories",
		StartsAt: now.Add(-time.Hour),
		Levels: []PromotionLevelInput{
			{
				Name:   "Ten off",
				Action: RuleAction{Mode: ActionModePercent, Value: models.MoneyFromFloat(10)},
				Targets: []PromotionTargetInput{
					{TargetType: models.DiscountTargetTypeCategory, TargetID: 1},
				},
			},
			{
				Name:   "Twenty off",
				Action: RuleAction{Mode: ActionModePercent, Value: models.MoneyFromFloat(20)},
				Targets: []PromotionTargetInput{
					{TargetType: models.DiscountTargetTypeCategory, TargetID: 2},
				},
			},
		},
	})
	require.NoError(t, err)

	result, err := EvaluateCart(db, []CartLine{
		{ProductID: 1, ProductVariantID: 10, CategoryIDs: []uint{1}, Quantity: 1, UnitPrice: models.MoneyFromFloat(100)},
		{ProductID: 2, ProductVariantID: 20, CategoryIDs: []uint{2}, Quantity: 1, UnitPrice: models.MoneyFromFloat(100)},
	}, now)
	require.NoError(t, err)
	require.Equal(t, 30.0, result.DiscountTotal.Float64())
	require.Equal(t, 90.0, result.Lines[0].FinalPrice.Float64())
	require.Equal(t, 80.0, result.Lines[1].FinalPrice.Float64())
	require.NotNil(t, result.Lines[0].AppliedCampaigns[0].LevelID)
}

func TestEvaluateCartHonorsRuleMaxApplicationsPerOrder(t *testing.T) {
	db := newDiscountTestDB(t)
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	maxApplications := 1

	_, err := CreatePromotion(db, CreatePromotionInput{
		Name:     "One eligible item",
		StartsAt: now.Add(-time.Hour),
		Rules: []PromotionRuleInput{
			{
				Condition: RuleCondition{MinQuantity: 1},
				Action: RuleAction{
					Mode:       ActionModeFreeItem,
					TargetType: models.DiscountTargetTypeProduct,
					TargetIDs:  []uint{1, 2},
				},
				MaxApplicationsPerOrder: &maxApplications,
			},
		},
	})
	require.NoError(t, err)

	result, err := EvaluateCart(db, []CartLine{
		{ProductID: 1, ProductVariantID: 10, Quantity: 1, UnitPrice: models.MoneyFromFloat(20)},
		{ProductID: 2, ProductVariantID: 20, Quantity: 1, UnitPrice: models.MoneyFromFloat(20)},
		{ProductID: 3, ProductVariantID: 30, Quantity: 1, UnitPrice: models.MoneyFromFloat(20)},
	}, now)
	require.NoError(t, err)

	require.Equal(t, 20.0, result.DiscountTotal.Float64())
	require.Equal(t, 0.0, result.Lines[0].FinalPrice.Float64())
	require.Equal(t, 20.0, result.Lines[1].FinalPrice.Float64())
	require.Equal(t, 20.0, result.Lines[2].FinalPrice.Float64())
	require.Len(t, result.Lines[0].AppliedCampaigns, 1)
	require.Empty(t, result.Lines[1].AppliedCampaigns)
}

func TestEvaluateCartHonorsLevelMaxApplicationsPerOrder(t *testing.T) {
	db := newDiscountTestDB(t)
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	maxApplications := 1

	_, err := CreatePromotion(db, CreatePromotionInput{
		Name:     "One level target",
		StartsAt: now.Add(-time.Hour),
		Levels: []PromotionLevelInput{
			{
				Name:                    "Category cap",
				Action:                  RuleAction{Mode: ActionModePercent, Value: models.MoneyFromFloat(25)},
				MaxApplicationsPerOrder: &maxApplications,
				Targets: []PromotionTargetInput{
					{TargetType: models.DiscountTargetTypeCategory, TargetID: 9},
				},
			},
		},
	})
	require.NoError(t, err)

	result, err := EvaluateCart(db, []CartLine{
		{ProductID: 1, ProductVariantID: 10, CategoryIDs: []uint{9}, Quantity: 1, UnitPrice: models.MoneyFromFloat(40)},
		{ProductID: 2, ProductVariantID: 20, CategoryIDs: []uint{9}, Quantity: 1, UnitPrice: models.MoneyFromFloat(40)},
	}, now)
	require.NoError(t, err)

	require.Equal(t, 10.0, result.DiscountTotal.Float64())
	require.Equal(t, 30.0, result.Lines[0].FinalPrice.Float64())
	require.Equal(t, 40.0, result.Lines[1].FinalPrice.Float64())
	require.NotNil(t, result.Lines[0].AppliedCampaigns[0].LevelID)
	require.Empty(t, result.Lines[1].AppliedCampaigns)
}

func TestEvaluateCartExclusiveCampaignBlocksLowerPriorityCampaigns(t *testing.T) {
	db := newDiscountTestDB(t)
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)

	_, err := CreatePromotion(db, CreatePromotionInput{
		Name:        "Exclusive",
		StartsAt:    now.Add(-time.Hour),
		Priority:    10,
		IsExclusive: true,
		Rules: []PromotionRuleInput{
			{
				Condition: RuleCondition{MinQuantity: 1},
				Action:    RuleAction{Mode: ActionModeFixed, Value: models.MoneyFromFloat(5), TargetType: "cart"},
			},
		},
	})
	require.NoError(t, err)
	_, err = CreatePromotion(db, CreatePromotionInput{
		Name:     "Lower",
		StartsAt: now.Add(-time.Hour),
		Priority: 1,
		Rules: []PromotionRuleInput{
			{
				Condition: RuleCondition{MinQuantity: 1},
				Action:    RuleAction{Mode: ActionModeFixed, Value: models.MoneyFromFloat(7), TargetType: "cart"},
			},
		},
	})
	require.NoError(t, err)

	result, err := EvaluateCart(db, []CartLine{
		{ProductID: 1, ProductVariantID: 10, Quantity: 1, UnitPrice: models.MoneyFromFloat(20)},
	}, now)
	require.NoError(t, err)
	require.Equal(t, 5.0, result.DiscountTotal.Float64())
	require.Equal(t, "Exclusive", result.Lines[0].AppliedCampaigns[0].Name)
}

func TestScheduleOneTimeArchivesExpiredCampaignAndRemovesEligibility(t *testing.T) {
	db := newDiscountTestDB(t)
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	campaign, err := CreateProductDiscount(db, ProductDiscountInput{
		Name:          "Flash",
		ProductIDs:    []uint{10},
		DiscountMode:  models.DiscountModeFixed,
		DiscountValue: models.MoneyFromFloat(5),
		StartsAt:      now,
	})
	require.NoError(t, err)

	_, err = UpsertSchedule(db, campaign.ID, ScheduleInput{
		ScheduleType: models.DiscountScheduleTypeOneTime,
		WindowStart:  now.Add(-2 * time.Hour),
		WindowEnd:    now.Add(-time.Hour),
	}, now)
	require.NoError(t, err)

	prices, err := PricesForProducts(db, map[uint]models.Money{10: models.MoneyFromFloat(20)}, now)
	require.NoError(t, err)
	require.Equal(t, 20.0, prices[10].FinalPrice.Float64())

	history, err := ListHistory(db, &campaign.ID)
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, models.DiscountCampaignStatusArchived, history[0].ToStatus)
}

func TestRunLifecycleRecurringCampaignCyclesAcrossThreeRuns(t *testing.T) {
	db := newDiscountTestDB(t)
	base := time.Date(2026, 5, 19, 9, 0, 0, 0, time.UTC)
	campaign, err := CreateProductDiscount(db, ProductDiscountInput{
		Name:          "Daily hour",
		ProductIDs:    []uint{10},
		DiscountMode:  models.DiscountModeFixed,
		DiscountValue: models.MoneyFromFloat(5),
		StartsAt:      base,
		Status:        models.DiscountCampaignStatusScheduled,
	})
	require.NoError(t, err)
	_, err = UpsertSchedule(db, campaign.ID, ScheduleInput{
		ScheduleType: models.DiscountScheduleTypeRecurring,
		Recurrence:   models.DiscountRecurrenceDaily,
		WindowStart:  base,
		WindowEnd:    base.Add(time.Hour),
		UntilAt:      timePtr(base.AddDate(0, 0, 3)),
	}, base.Add(-time.Hour))
	require.NoError(t, err)

	run1, err := RunLifecycle(db, base.Add(30*time.Minute))
	require.NoError(t, err)
	require.Equal(t, 1, run1.Activated)
	assertCampaignStatus(t, db, campaign.ID, models.DiscountCampaignStatusActive)

	run2, err := RunLifecycle(db, base.Add(2*time.Hour))
	require.NoError(t, err)
	require.Equal(t, 1, run2.Deactivated)
	assertCampaignStatus(t, db, campaign.ID, models.DiscountCampaignStatusScheduled)

	run3, err := RunLifecycle(db, base.AddDate(0, 0, 1).Add(30*time.Minute))
	require.NoError(t, err)
	require.Equal(t, 1, run3.Activated)
	assertCampaignStatus(t, db, campaign.ID, models.DiscountCampaignStatusActive)

	run4, err := RunLifecycle(db, base.AddDate(0, 0, 4))
	require.NoError(t, err)
	require.Equal(t, 1, run4.Archived)
	assertCampaignStatus(t, db, campaign.ID, models.DiscountCampaignStatusArchived)
}

func TestRunLifecycleIsIdempotentForSameState(t *testing.T) {
	db := newDiscountTestDB(t)
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	campaign, err := CreateProductDiscount(db, ProductDiscountInput{
		Name:          "Idempotent",
		ProductIDs:    []uint{10},
		DiscountMode:  models.DiscountModeFixed,
		DiscountValue: models.MoneyFromFloat(5),
		StartsAt:      now,
		Status:        models.DiscountCampaignStatusScheduled,
	})
	require.NoError(t, err)
	_, err = UpsertSchedule(db, campaign.ID, ScheduleInput{
		ScheduleType: models.DiscountScheduleTypeOneTime,
		WindowStart:  now.Add(time.Hour),
		WindowEnd:    now.Add(2 * time.Hour),
	}, now)
	require.NoError(t, err)

	_, err = RunLifecycle(db, now)
	require.NoError(t, err)
	_, err = RunLifecycle(db, now)
	require.NoError(t, err)

	history, err := ListHistory(db, &campaign.ID)
	require.NoError(t, err)
	require.Len(t, history, 1)
}

func TestPromotionTemplateInstantiateCreatesCampaignWithOverrides(t *testing.T) {
	db := newDiscountTestDB(t)
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	template, err := CreateTemplate(db, TemplateInput{
		Name: "Category offer",
		Template: CreatePromotionInput{
			Name:     "Template base",
			StartsAt: now,
			Rules: []PromotionRuleInput{{
				Condition: RuleCondition{CategoryIDs: []uint{9}, MinQuantity: 1},
				Action:    RuleAction{Mode: ActionModePercent, Value: models.MoneyFromFloat(10), TargetType: models.DiscountTargetTypeCategory, TargetIDs: []uint{9}},
			}},
		},
	})
	require.NoError(t, err)
	code := "spring10"
	channels := []string{models.DiscountChannelWeb}
	campaign, err := InstantiateTemplate(db, template.ID, InstantiateTemplateInput{
		Name:       stringPtr("Spring"),
		CouponCode: &code,
		Channels:   channels,
	})
	require.NoError(t, err)
	require.Equal(t, "Spring", campaign.Name)
	require.NotNil(t, campaign.CouponCode)
	require.Equal(t, "SPRING10", *campaign.CouponCode)
	require.Len(t, campaign.Rules, 1)
}

func TestValidatePromotionRejectsInvalidMaxApplicationsPerOrder(t *testing.T) {
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	zero := 0
	negative := -1

	invalid := []CreatePromotionInput{
		{
			Name:     "Bad rule cap",
			StartsAt: now,
			Rules: []PromotionRuleInput{{
				Condition:               RuleCondition{MinQuantity: 1},
				Action:                  RuleAction{Mode: ActionModeFixed, Value: models.MoneyFromFloat(5), TargetType: "cart"},
				MaxApplicationsPerOrder: &zero,
			}},
		},
		{
			Name:     "Bad level cap",
			StartsAt: now,
			Levels: []PromotionLevelInput{{
				Name:                    "Level",
				Action:                  RuleAction{Mode: ActionModeFixed, Value: models.MoneyFromFloat(5)},
				MaxApplicationsPerOrder: &negative,
				Targets:                 []PromotionTargetInput{{TargetType: models.DiscountTargetTypeProduct, TargetID: 1}},
			}},
		},
	}
	for _, input := range invalid {
		require.ErrorIs(t, validatePromotion(input), ErrInvalidCampaign)
	}
}

func TestEvaluateCartFiltersCouponChannelAndSegmentCampaigns(t *testing.T) {
	db := newDiscountTestDB(t)
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	code := "VIP20"
	_, err := CreatePromotion(db, CreatePromotionInput{
		Name:            "VIP web coupon",
		StartsAt:        now.Add(-time.Hour),
		CouponCode:      &code,
		Channels:        []string{models.DiscountChannelWeb},
		CustomerSegment: "vip",
		Rules: []PromotionRuleInput{{
			Condition: RuleCondition{ProductIDs: []uint{1}, MinQuantity: 1},
			Action:    RuleAction{Mode: ActionModeFixed, Value: models.MoneyFromFloat(5), TargetType: models.DiscountTargetTypeProduct, TargetIDs: []uint{1}},
		}},
	})
	require.NoError(t, err)
	lines := []CartLine{{ProductID: 1, ProductVariantID: 10, Quantity: 1, UnitPrice: models.MoneyFromFloat(20)}}

	withoutCode, err := EvaluateCart(db, lines, now)
	require.NoError(t, err)
	require.Zero(t, withoutCode.DiscountTotal)

	withCode, err := EvaluateCartWithOptions(db, lines, now, EvaluationOptions{CouponCode: "vip20", Channel: models.DiscountChannelWeb, CustomerSegment: "vip"})
	require.NoError(t, err)
	require.Equal(t, 5.0, withCode.DiscountTotal.Float64())
}

func TestUsageCapsBlockSubsequentRedemptions(t *testing.T) {
	db := newDiscountTestDB(t)
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	capValue := 1
	campaign, err := CreatePromotion(db, CreatePromotionInput{
		Name:           "Once only",
		StartsAt:       now.Add(-time.Hour),
		GlobalUsageCap: &capValue,
		Rules: []PromotionRuleInput{{
			Condition: RuleCondition{ProductIDs: []uint{1}, MinQuantity: 1},
			Action:    RuleAction{Mode: ActionModeFixed, Value: models.MoneyFromFloat(5), TargetType: models.DiscountTargetTypeProduct, TargetIDs: []uint{1}},
		}},
	})
	require.NoError(t, err)
	result, err := EvaluateCart(db, []CartLine{{ProductID: 1, ProductVariantID: 10, Quantity: 1, UnitPrice: models.MoneyFromFloat(20)}}, now)
	require.NoError(t, err)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, VerifyUsageCaps(tx, result, nil))
		return RecordRedemptions(tx, 100, nil, result, now)
	}))
	require.ErrorIs(t, db.Transaction(func(tx *gorm.DB) error {
		return VerifyUsageCaps(tx, result, nil)
	}), ErrUsageCapExceeded)
	require.Equal(t, uint(campaign.ID), result.Lines[0].AppliedCampaigns[0].ID)
}

func TestCampaignAuditRecordsCreateUpdateDisableAndScheduleChanges(t *testing.T) {
	db := newDiscountTestDB(t)
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	actorID := uint(42)
	campaign, err := CreateProductDiscount(db, ProductDiscountInput{
		Name:          "Audited",
		ProductIDs:    []uint{10},
		DiscountMode:  models.DiscountModeFixed,
		DiscountValue: models.MoneyFromFloat(5),
		StartsAt:      now,
		ActorID:       &actorID,
	})
	require.NoError(t, err)
	_, err = UpdateProductDiscount(db, campaign.ID, ProductDiscountInput{
		Name:          "Audited updated",
		ProductIDs:    []uint{10},
		DiscountMode:  models.DiscountModeFixed,
		DiscountValue: models.MoneyFromFloat(6),
		StartsAt:      now,
		ActorID:       &actorID,
	})
	require.NoError(t, err)
	_, err = UpsertSchedule(db, campaign.ID, ScheduleInput{
		ScheduleType: models.DiscountScheduleTypeOneTime,
		WindowStart:  now.Add(time.Hour),
		WindowEnd:    now.Add(2 * time.Hour),
		Actor:        "user:42",
	}, now)
	require.NoError(t, err)
	_, err = DisableProductDiscount(db, campaign.ID, &actorID)
	require.NoError(t, err)

	audits, err := ListCampaignAudits(db, &campaign.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(audits), 4)
	events := map[string]bool{}
	for _, audit := range audits {
		events[audit.EventType] = true
		require.Equal(t, campaign.ID, audit.CampaignID)
		require.NotEmpty(t, audit.AfterJSON)
	}
	require.True(t, events[AuditEventCampaignCreated])
	require.True(t, events[AuditEventCampaignUpdated])
	require.True(t, events[AuditEventScheduleUpdated])
	require.True(t, events[AuditEventCampaignDisabled])
}

func TestEvaluationMetricsTrackLatencyMatchesAndFailures(t *testing.T) {
	db := newDiscountTestDB(t)
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	_, err := CreatePromotion(db, CreatePromotionInput{
		Name:     "Metrics",
		StartsAt: now.Add(-time.Hour),
		Rules: []PromotionRuleInput{{
			Condition: RuleCondition{ProductIDs: []uint{1}, MinQuantity: 1},
			Action:    RuleAction{Mode: ActionModeFixed, Value: models.MoneyFromFloat(5), TargetType: models.DiscountTargetTypeProduct, TargetIDs: []uint{1}},
		}},
	})
	require.NoError(t, err)

	before := EvaluationMetricsSnapshot()
	result, err := EvaluateCart(db, []CartLine{{ProductID: 1, ProductVariantID: 10, Quantity: 1, UnitPrice: models.MoneyFromFloat(20)}}, now)
	require.NoError(t, err)
	require.Equal(t, 5.0, result.DiscountTotal.Float64())
	after := EvaluationMetricsSnapshot()

	require.Equal(t, before.TotalEvaluations+1, after.TotalEvaluations)
	require.Equal(t, uint64(1), after.LastLineCount)
	require.Equal(t, uint64(1), after.LastCandidateCampaigns)
	require.Equal(t, uint64(1), after.LastMatchedCampaigns)
	require.Empty(t, after.LastError)
}

func TestRunReconciliationDetectsScheduleDrift(t *testing.T) {
	db := newDiscountTestDB(t)
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	campaign, err := CreateProductDiscount(db, ProductDiscountInput{
		Name:          "Drift",
		ProductIDs:    []uint{10},
		DiscountMode:  models.DiscountModeFixed,
		DiscountValue: models.MoneyFromFloat(5),
		StartsAt:      now.Add(-time.Hour),
		Status:        models.DiscountCampaignStatusScheduled,
	})
	require.NoError(t, err)
	_, err = UpsertSchedule(db, campaign.ID, ScheduleInput{
		ScheduleType: models.DiscountScheduleTypeOneTime,
		WindowStart:  now.Add(-time.Hour),
		WindowEnd:    now.Add(time.Hour),
	}, now)
	require.NoError(t, err)
	require.NoError(t, db.Model(&models.DiscountCampaign{}).Where("id = ?", campaign.ID).Update("status", models.DiscountCampaignStatusScheduled).Error)

	report, err := RunReconciliation(db, now)
	require.NoError(t, err)
	require.Len(t, report.Issues, 1)
	require.Equal(t, models.DiscountCampaignStatusActive, report.Issues[0].ExpectedStatus)
	require.Equal(t, models.DiscountCampaignStatusScheduled, report.Issues[0].ActualStatus)
}

func assertCampaignStatus(t *testing.T, db *gorm.DB, id uint, status string) {
	t.Helper()
	var campaign models.DiscountCampaign
	require.NoError(t, db.First(&campaign, id).Error)
	require.Equal(t, status, campaign.Status)
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func stringPtr(value string) *string {
	return &value
}
