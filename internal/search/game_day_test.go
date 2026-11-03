package search

import (
	"context"
	"testing"
	"time"

	"ecommerce/internal/jobs"
	"ecommerce/models"
	"github.com/stretchr/testify/require"
)

// These isolated simulations are repeatable CI evidence, not a live incident
// exercise or proof that deployment alerts reached an on-call operator.
func TestSearchGameDayStaleIndexRecovery(t *testing.T) {
	db := searchTestDB(t)
	s := NewService(db, nil, jobs.NewRuntime(db, jobs.Config{}))
	require.NoError(t, s.Reindex(context.Background()))
	now := time.Now().UTC()
	old := now.Add(-10 * time.Minute)
	job := models.JobQueue{ID: "search-game-day-stale", JobType: JobTypeProductSync, Status: models.JobStatusPending, PayloadJSON: `{"version":1,"product_id":1,"revision":1}`, RunAt: old}
	job.CreatedAt = old
	require.NoError(t, db.Create(&job).Error)
	status, err := s.Freshness(context.Background())
	require.NoError(t, err)
	require.Equal(t, "stale", status.Status)
	require.Greater(t, status.Lag, StaleFreshnessTarget)
	require.NoError(t, db.Model(&job).Update("status", models.JobStatusSucceeded).Error)
	require.NoError(t, s.Reindex(context.Background()))
	status, err = s.Freshness(context.Background())
	require.NoError(t, err)
	require.Equal(t, "healthy", status.Status)
}
func TestSearchGameDayBadRuleRollback(t *testing.T) {
	db := searchTestDB(t)
	p := models.Product{Name: "Visible", SKU: "ROLLBACK", IsPublished: true}
	require.NoError(t, db.Create(&p).Error)
	s := NewService(db, nil, nil)
	require.NoError(t, s.Reindex(context.Background()))
	rule, err := s.CreateMerchandisingRule(context.Background(), MerchandisingRuleInput{Name: "Campaign", RuleType: "hide", IsActive: true, Action: MerchandisingAction{Targets: []MerchandisingTarget{{ProductID: p.ID}}}}, nil)
	require.NoError(t, err)
	result, err := s.Search(context.Background(), Filters{})
	require.NoError(t, err)
	require.Zero(t, result.Total)
	disabled := false
	_, err = s.UpdateMerchandisingRule(context.Background(), rule.ID, MerchandisingRulePatch{IsActive: &disabled}, nil)
	require.NoError(t, err)
	result, err = s.Search(context.Background(), Filters{})
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)
	history, err := s.ListMerchandisingAudit(context.Background(), rule.ID)
	require.NoError(t, err)
	require.Len(t, history, 2)
	require.True(t, history[0].Before.IsActive)
	require.False(t, history[0].After.IsActive)
}
func TestSearchGameDaySlowSearchRecovery(t *testing.T) {
	db := searchTestDB(t)
	slow := true
	s := NewService(db, faultBackend{search: func(ctx context.Context, f Filters) (Result, error) {
		if slow {
			<-ctx.Done()
			return Result{}, ctx.Err()
		}
		return Result{}, nil
	}}, nil)
	c := DefaultHardeningConfig()
	c.SearchTimeoutMS = 10
	c.CircuitFailureThreshold = 1
	c.CircuitOpenMS = 10
	require.NoError(t, s.ConfigureHardening(c))
	now := time.Now().UTC()
	s.now = func() time.Time { return now }
	result, err := s.Search(context.Background(), Filters{})
	require.NoError(t, err)
	require.Equal(t, "search_timeout", result.FallbackReason)
	status, err := s.Operations(context.Background())
	require.NoError(t, err)
	require.Equal(t, "open", status.CircuitState)
	slow = false
	now = now.Add(11 * time.Millisecond)
	result, err = s.Search(context.Background(), Filters{})
	require.NoError(t, err)
	require.False(t, result.Degraded)
	status, err = s.Operations(context.Background())
	require.NoError(t, err)
	require.Equal(t, "closed", status.CircuitState)
}
