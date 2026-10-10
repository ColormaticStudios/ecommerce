package search

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"ecommerce/internal/jobs"
	"ecommerce/models"
	"github.com/stretchr/testify/require"
)

type faultBackend struct {
	SearchBackend
	search func(context.Context, Filters) (Result, error)
}

func (b faultBackend) Search(ctx context.Context, f Filters) (Result, error) { return b.search(ctx, f) }
func TestSearchOutageFallbackPreservesHideAndFilters(t *testing.T) {
	db := searchTestDB(t)
	p := models.Product{Name: "Visible", SKU: "VISIBLE", IsPublished: true, Price: models.MoneyFromFloat(15), Stock: 3}
	hidden := models.Product{Name: "Hidden", SKU: "HIDDEN", IsPublished: true, Price: models.MoneyFromFloat(20), Stock: 3}
	draft := models.Product{Name: "Draft", SKU: "DRAFT"}
	require.NoError(t, db.Create(&p).Error)
	require.NoError(t, db.Create(&hidden).Error)
	require.NoError(t, db.Create(&draft).Error)
	s := NewService(db, faultBackend{search: func(context.Context, Filters) (Result, error) { return Result{}, ErrIndexUnavailable }}, nil)
	_, err := s.CreateMerchandisingRule(context.Background(), MerchandisingRuleInput{Name: "Hide", RuleType: "hide", IsActive: true, Predicate: MerchandisingPredicate{Query: &MerchandisingQuery{Mode: "exact", Value: "outage"}}, Action: MerchandisingAction{Targets: []MerchandisingTarget{{ProductID: hidden.ID}}}}, nil)
	require.NoError(t, err)
	min, max := 10.0, 25.0
	stock := true
	result, err := s.Search(context.Background(), Filters{Query: "outage", MinPrice: &min, MaxPrice: &max, HasVariantStock: &stock})
	require.NoError(t, err)
	require.True(t, result.Degraded)
	require.Equal(t, "indexed_search_unavailable", result.FallbackReason)
	require.Equal(t, int64(1), result.Total)
	require.Equal(t, p.ID, result.Products[0].ID)
	require.Empty(t, result.Facets)
	require.Empty(t, result.Explanations)
	require.NoError(t, db.Model(&models.SearchMerchandisingRule{}).Where("name = ?", "Hide").Update("action_json", "bad").Error)
	_, err = s.Search(context.Background(), Filters{Query: "outage"})
	require.Error(t, err)
}
func TestSearchCircuitRecoveryAndTerminalErrors(t *testing.T) {
	db := searchTestDB(t)
	now := time.Now().UTC()
	calls := 0
	fail := true
	s := NewService(db, faultBackend{search: func(context.Context, Filters) (Result, error) {
		calls++
		if fail {
			return Result{}, ErrIndexUnavailable
		}
		return Result{Products: []models.Product{}}, nil
	}}, nil)
	s.now = func() time.Time { return now }
	c := DefaultHardeningConfig()
	c.CircuitFailureThreshold = 1
	c.CircuitOpenMS = 10
	require.NoError(t, s.ConfigureHardening(c))
	result, err := s.Search(context.Background(), Filters{})
	require.NoError(t, err)
	require.True(t, result.Degraded)
	result, err = s.Search(context.Background(), Filters{})
	require.NoError(t, err)
	require.Equal(t, "circuit_open", result.FallbackReason)
	require.Equal(t, 1, calls)
	now = now.Add(11 * time.Millisecond)
	fail = false
	result, err = s.Search(context.Background(), Filters{})
	require.NoError(t, err)
	require.False(t, result.Degraded)
	require.Equal(t, 2, calls)
	status, err := s.Operations(context.Background())
	require.NoError(t, err)
	require.Equal(t, "closed", status.CircuitState)
	terminal := errors.New("invalid ranking configuration")
	s.backend = faultBackend{search: func(context.Context, Filters) (Result, error) { return Result{}, terminal }}
	_, err = s.Search(context.Background(), Filters{})
	require.ErrorIs(t, err, terminal)
}
func TestSearchAdmissionCancellationAndTimeout(t *testing.T) {
	db := searchTestDB(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	s := NewService(db, faultBackend{search: func(ctx context.Context, f Filters) (Result, error) {
		once.Do(func() { close(entered) })
		select {
		case <-release:
			return Result{}, nil
		case <-ctx.Done():
			return Result{}, ctx.Err()
		}
	}}, nil)
	c := DefaultHardeningConfig()
	c.MaxConcurrent = 1
	require.NoError(t, s.ConfigureHardening(c))
	done := make(chan error)
	go func() { _, err := s.Search(context.Background(), Filters{}); done <- err }()
	<-entered
	_, err := s.Search(context.Background(), Filters{})
	require.ErrorIs(t, err, ErrCapacity)
	close(release)
	require.NoError(t, <-done)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = s.Search(canceled, Filters{})
	require.ErrorIs(t, err, context.Canceled)
	c.SearchTimeoutMS = 10
	require.NoError(t, s.ConfigureHardening(c))
	s.backend = faultBackend{search: func(ctx context.Context, f Filters) (Result, error) { <-ctx.Done(); return Result{}, ctx.Err() }}
	result, err := s.Search(context.Background(), Filters{})
	require.NoError(t, err)
	require.Equal(t, "search_timeout", result.FallbackReason)
	_, err = s.Search(context.Background(), Filters{Explain: true})
	require.ErrorIs(t, err, ErrIndexUnavailable)
}
func TestReindexQueueBackpressure(t *testing.T) {
	db := searchTestDB(t)
	runtime := jobs.NewRuntime(db, jobs.Config{})
	s := NewService(db, nil, runtime)
	_, err := s.EnqueueFullReindex(context.Background())
	require.NoError(t, err)
	_, err = s.EnqueueFullReindex(context.Background())
	require.ErrorIs(t, err, ErrCapacity)
	require.NoError(t, db.Model(&models.JobQueue{}).Where("job_type = ?", JobTypeFullReindex).Update("status", models.JobStatusSucceeded).Error)
	_, err = s.EnqueueFullReindex(context.Background())
	require.NoError(t, err)
}
func TestHardeningConfiguration(t *testing.T) {
	require.NoError(t, DefaultHardeningConfig().Validate())
	bad := DefaultHardeningConfig()
	bad.MaxConcurrent = 0
	require.Error(t, bad.Validate())
}
