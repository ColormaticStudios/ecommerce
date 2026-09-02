package discounts

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	"ecommerce/models"

	"github.com/stretchr/testify/require"
)

const (
	discountPerformanceCampaignCount = 500
	discountPerformanceCartLineCount = 25
	discountEvaluationP95Budget      = 50 * time.Millisecond
)

func TestDiscountEvaluationPerformanceBudget(t *testing.T) {
	db := newDiscountTestDB(t)
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	campaigns := make([]models.DiscountCampaign, discountPerformanceCampaignCount)
	for index := range campaigns {
		campaigns[index] = models.DiscountCampaign{
			Name: fmt.Sprintf("Performance campaign %03d", index), Type: models.DiscountCampaignTypePromotion,
			Status: models.DiscountCampaignStatusActive, StartsAt: now.Add(-time.Hour), Priority: index,
			DiscountMode: models.DiscountModePercent, DiscountValue: models.MoneyFromFloat(5),
		}
	}
	require.NoError(t, db.CreateInBatches(&campaigns, 100).Error)
	rules := make([]models.DiscountRule, 0, len(campaigns))
	for index, campaign := range campaigns {
		condition, err := json.Marshal(RuleCondition{ProductIDs: []uint{uint(index%100 + 1)}, MinQuantity: 1})
		require.NoError(t, err)
		action, err := json.Marshal(RuleAction{Mode: ActionModePercent, Value: models.MoneyFromFloat(5), TargetType: models.DiscountTargetTypeProduct, TargetIDs: []uint{uint(index%100 + 1)}})
		require.NoError(t, err)
		rules = append(rules, models.DiscountRule{CampaignID: campaign.ID, ConditionJSON: string(condition), ActionJSON: string(action), StackPolicy: StackPolicyNone})
	}
	require.NoError(t, db.CreateInBatches(&rules, 100).Error)
	lines := make([]CartLine, discountPerformanceCartLineCount)
	for index := range lines {
		lines[index] = CartLine{ProductID: uint(index + 1), ProductVariantID: uint(index + 1001), Quantity: 1, UnitPrice: models.MoneyFromFloat(100)}
	}
	_, err := EvaluateCart(db, lines, now)
	require.NoError(t, err)

	durations := make([]time.Duration, 20)
	for index := range durations {
		started := time.Now()
		_, err := EvaluateCart(db, lines, now)
		require.NoError(t, err)
		durations[index] = time.Since(started)
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := durations[18]
	if p95 >= discountEvaluationP95Budget {
		t.Fatalf("discount evaluation p95 %s exceeds %s budget with %d campaigns and %d cart lines", p95, discountEvaluationP95Budget, discountPerformanceCampaignCount, discountPerformanceCartLineCount)
	}
}
