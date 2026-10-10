package search

import (
	"context"
	"sync"
	"testing"
	"time"

	"ecommerce/models"

	"github.com/stretchr/testify/require"
)

func incidentFixture(t *testing.T) (*Service, *time.Time) {
	db := searchTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SearchIndexIncident{}, &models.SearchFreshnessObservation{}))
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX idx_search_index_incidents_open ON search_index_incidents(index_name) WHERE recovered_at IS NULL").Error)
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	s := NewService(db, nil, nil)
	s.now = func() time.Time { return now }
	return s, &now
}
func setIncidentBaseline(t *testing.T, s *Service, now time.Time) {
	t.Helper()
	require.NoError(t, s.db.Save(&models.SearchIndexState{Name: ProductIndexName, LastFullReindexAt: &now, LastIndexedAt: &now, DocumentCount: 10}).Error)
}
func staleIncidentJob(t *testing.T, s *Service, now time.Time) {
	t.Helper()
	require.NoError(t, s.db.Create(&models.JobQueue{JobType: JobTypeProductSync, Status: models.JobStatusPending, PayloadJSON: "{}", ID: now.String(), CreatedAt: now.Add(-6 * time.Minute)}).Error)
}
func TestFreshnessIncidentMissingBaselineGraceSurvivesRestart(t *testing.T) {
	s, now := incidentFixture(t)
	ctx := context.Background()
	require.NoError(t, s.ObserveFreshness(ctx))
	list, err := s.ListIncidents(ctx, 1, 20, "")
	require.NoError(t, err)
	require.Empty(t, list.Items)
	*now = now.Add(4 * time.Minute)
	other := NewService(s.db, nil, nil)
	other.now = s.now
	require.NoError(t, other.ObserveFreshness(ctx))
	list, err = other.ListIncidents(ctx, 1, 20, "")
	require.NoError(t, err)
	require.Empty(t, list.Items)
	*now = now.Add(time.Minute)
	require.NoError(t, other.ObserveFreshness(ctx))
	list, err = other.ListIncidents(ctx, 1, 20, "open")
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	require.Equal(t, "missing_baseline", list.Items[0].Reason)
	require.Equal(t, int64(0), list.Items[0].DurationSeconds)
	setIncidentBaseline(t, s, *now)
	*now = now.Add(time.Minute)
	require.NoError(t, s.ObserveFreshness(ctx))
	list, err = s.ListIncidents(ctx, 1, 20, "resolved")
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	require.Equal(t, int64(60), list.Items[0].DurationSeconds)
}
func TestFreshnessIncidentContinuousEpisodeRecoveryAndRepeat(t *testing.T) {
	s, now := incidentFixture(t)
	ctx := context.Background()
	setIncidentBaseline(t, s, *now)
	staleIncidentJob(t, s, *now)
	require.NoError(t, s.ObserveFreshness(ctx))
	*now = now.Add(time.Minute)
	require.NoError(t, s.ObserveFreshness(ctx))
	list, err := s.ListIncidents(ctx, 1, 1, "open")
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	require.Equal(t, "index_lag", list.Items[0].Reason)
	require.Equal(t, int64(420), list.Items[0].MaxLagSeconds)
	require.Equal(t, int64(120), list.Items[0].DurationSeconds)
	id := list.Items[0].ID
	// Recover to degraded, which ends staleness even before fully healthy.
	require.NoError(t, s.db.Model(&models.JobQueue{}).Where("job_type = ?", JobTypeProductSync).Update("created_at", now.Add(-2*time.Minute)).Error)
	require.NoError(t, s.ObserveFreshness(ctx))
	list, err = s.ListIncidents(ctx, 1, 20, "resolved")
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	require.Equal(t, id, list.Items[0].ID)
	require.Equal(t, int64(120), list.Items[0].DurationSeconds)
	*now = now.Add(4 * time.Minute)
	require.NoError(t, s.ObserveFreshness(ctx))
	list, err = s.ListIncidents(ctx, 1, 20, "")
	require.NoError(t, err)
	require.Len(t, list.Items, 2)
	require.Equal(t, int64(2), list.Total)
	require.Equal(t, 1, list.TotalPages)
	require.NotEqual(t, id, list.Items[0].ID)
}
func TestFreshnessIncidentConcurrentObserversAndRollback(t *testing.T) {
	s, now := incidentFixture(t)
	ctx := context.Background()
	setIncidentBaseline(t, s, *now)
	staleIncidentJob(t, s, *now)
	sqlDB, err := s.db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	var wg sync.WaitGroup
	errors := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errors <- s.ObserveFreshness(ctx) }()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	list, err := s.ListIncidents(ctx, 1, 20, "")
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	first := list.Items[0]
	require.NoError(t, s.db.Exec("CREATE TRIGGER fail_incident_observation BEFORE UPDATE ON search_index_incidents BEGIN SELECT RAISE(ABORT, 'incident test failure'); END").Error)
	*now = now.Add(time.Minute)
	require.Error(t, s.ObserveFreshness(ctx))
	var observed models.SearchFreshnessObservation
	require.NoError(t, s.db.First(&observed, "index_name = ?", ProductIndexName).Error)
	require.True(t, first.LastObservedAt.Equal(observed.LastObservedAt))
	require.NoError(t, s.db.Exec("DROP TRIGGER fail_incident_observation").Error)
	require.NoError(t, s.ObserveFreshness(ctx))
}
func TestFreshnessIncidentReadOnlyInvalidQueriesCancellationAndRetention(t *testing.T) {
	s, now := incidentFixture(t)
	ctx := context.Background()
	list, err := s.ListIncidents(ctx, 1, 20, "")
	require.NoError(t, err)
	require.Empty(t, list.Items)
	var count int64
	require.NoError(t, s.db.Model(&models.SearchFreshnessObservation{}).Count(&count).Error)
	require.Zero(t, count)
	for _, q := range []struct {
		page, limit int
		status      string
	}{{0, 20, ""}, {1, 101, ""}, {1, 20, "bad"}} {
		_, err = s.ListIncidents(ctx, q.page, q.limit, q.status)
		require.ErrorIs(t, err, ErrIncidentQueryInvalid)
	}
	old := now.Add(-IncidentRetention - time.Hour)
	rows := []models.SearchIndexIncident{{IndexName: ProductIndexName, Reason: "index_lag", OpenedAt: old, DetectedAt: old, LastObservedAt: old, RecoveredAt: &old}, {IndexName: "other", Reason: "index_lag", OpenedAt: old, DetectedAt: old, LastObservedAt: old}}
	require.NoError(t, s.db.Create(&rows).Error)
	setIncidentBaseline(t, s, *now)
	require.NoError(t, s.ObserveFreshness(ctx))
	require.NoError(t, s.db.Model(&models.SearchIndexIncident{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	require.ErrorIs(t, s.ObserveFreshness(canceled), context.Canceled)
	// Database failures cannot manufacture an observation or incident.
	require.NoError(t, s.db.Migrator().DropTable(&models.JobQueue{}))
	require.Error(t, s.ObserveFreshness(ctx))
	var observation models.SearchFreshnessObservation
	require.NoError(t, s.db.First(&observation, "index_name = ?", ProductIndexName).Error)
	require.True(t, observation.LastObservedAt.Equal(*now))
}
