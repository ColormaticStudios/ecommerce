package search

import (
	"context"
	"ecommerce/internal/jobs"
	"ecommerce/internal/reliability"
	"ecommerce/models"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func analyticsFixture(t *testing.T) (*Service, uint) {
	db := searchTestDB(t)
	p := models.Product{Name: "Green boots", SKU: uuid.NewString(), IsPublished: true, Price: models.MoneyFromFloat(10)}
	require.NoError(t, db.Create(&p).Error)
	s := NewService(db, nil, nil)
	require.NoError(t, s.Reindex(context.Background()))
	return s, p.ID
}
func TestAnalyticsConsentSnapshotOwnershipAndDedupe(t *testing.T) {
	s, p := analyticsFixture(t)
	ctx := context.Background()
	filters := Filters{Query: "boots", Page: 1, Limit: 10}
	_, err := s.CreateImpression(ctx, false, "", filters)
	require.ErrorIs(t, err, ErrAnalyticsInvalid)
	id := uuid.NewString()
	im, err := s.CreateImpressionIdempotent(ctx, true, "", id, filters)
	require.NoError(t, err)
	require.Equal(t, p, im.Result.Products[0].ID)
	again, err := s.CreateImpressionIdempotent(ctx, true, im.SessionToken, id, filters)
	require.NoError(t, err)
	require.Equal(t, im.Result, again.Result)
	var count int64
	require.NoError(t, s.db.Model(&models.SearchQueryEvent{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
	click := ClickInput{SessionToken: im.SessionToken, ImpressionID: im.ID, EventID: uuid.NewString(), ProductID: p, Position: 1}
	cid, err := s.RecordClick(ctx, click)
	require.NoError(t, err)
	require.Equal(t, click.EventID, cid)
	_, err = s.RecordClick(ctx, click)
	require.NoError(t, err)
	require.NoError(t, s.db.Model(&models.SearchClickEvent{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
	invalid := click
	invalid.ProductID++
	_, err = s.RecordClick(ctx, invalid)
	require.ErrorIs(t, err, ErrAnalyticsInvalid)
	invalid = click
	invalid.Position = 2
	_, err = s.RecordClick(ctx, invalid)
	require.ErrorIs(t, err, ErrAnalyticsInvalid)
	invalid = click
	invalid.SessionToken = hex.EncodeToString(make([]byte, 32))
	_, err = s.RecordClick(ctx, invalid)
	require.ErrorIs(t, err, ErrAnalyticsInvalid)
	filters.Query = "green"
	_, err = s.CreateImpressionIdempotent(ctx, true, im.SessionToken, id, filters)
	require.ErrorIs(t, err, ErrAnalyticsInvalid)
	require.NoError(t, s.RevokeConsent(ctx, im.SessionToken))
	require.NoError(t, s.RevokeConsent(ctx, im.SessionToken))
	_, err = s.RecordClick(ctx, click)
	require.ErrorIs(t, err, ErrAnalyticsInvalid)
	_, err = s.CreateImpression(ctx, true, im.SessionToken, filters)
	require.ErrorIs(t, err, ErrAnalyticsInvalid)
	require.NoError(t, s.db.Model(&models.SearchQueryEvent{}).Count(&count).Error)
	require.Zero(t, count)
}
func TestAnalyticsPopularityThresholdMetricsAndRetention(t *testing.T) {
	s, p := analyticsFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	s.now = func() time.Time { return now }
	for i := 0; i < 5; i++ {
		im, err := s.CreateImpression(ctx, true, "", Filters{Query: "boots", Page: 1, Limit: 10})
		require.NoError(t, err)
		if i == 0 {
			_, err = s.RecordClick(ctx, ClickInput{SessionToken: im.SessionToken, ImpressionID: im.ID, EventID: uuid.NewString(), ProductID: p, Position: 1})
			require.NoError(t, err)
		}
		popular, _, err := s.PopularQueries(ctx, "b", 8)
		require.NoError(t, err)
		if i < 4 {
			require.Empty(t, popular)
		} else {
			require.Equal(t, []string{"boots"}, popular)
		}
	}
	metrics, err := s.Analytics(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, int64(5), metrics.Searches)
	require.Equal(t, int64(5), metrics.UniqueSessions)
	require.Equal(t, int64(1), metrics.Clicks)
	require.Equal(t, .2, metrics.CTRAt5)
	require.Equal(t, []string{"boots"}, metrics.Trending)
	s.now = func() time.Time { return now.Add(91 * 24 * time.Hour) }
	require.NoError(t, s.PruneAnalytics(ctx))
	metrics, err = s.Analytics(ctx, 90)
	require.NoError(t, err)
	require.Zero(t, metrics.Searches)
}
func TestAnalyticsPreviewAndExpiredClickAreExcluded(t *testing.T) {
	s, p := analyticsFixture(t)
	ctx := context.Background()
	_, err := s.CreateImpression(ctx, true, "", Filters{Explain: true})
	require.ErrorIs(t, err, ErrAnalyticsInvalid)
	im, err := s.CreateImpression(ctx, true, "", Filters{Query: "boots", Page: 1, Limit: 10})
	require.NoError(t, err)
	now := s.now()
	s.now = func() time.Time { return now.Add(8 * 24 * time.Hour) }
	_, err = s.RecordClick(ctx, ClickInput{SessionToken: im.SessionToken, ImpressionID: im.ID, EventID: uuid.NewString(), ProductID: p, Position: 1})
	require.ErrorIs(t, err, ErrAnalyticsInvalid)
}

func TestAnalyticsSnapshotReplaysAfterCatalogChangesAndUsesAbsoluteRanks(t *testing.T) {
	s, p := analyticsFixture(t)
	ctx := context.Background()
	second := models.Product{Name: "Green boots two", SKU: uuid.NewString(), IsPublished: true, Price: models.MoneyFromFloat(20)}
	require.NoError(t, s.db.Create(&second).Error)
	require.NoError(t, s.Reindex(ctx))
	filters := Filters{Query: "boots", Page: 2, Limit: 1, SortField: "name", SortOrder: "asc"}
	im, err := s.CreateImpression(ctx, true, "", filters)
	require.NoError(t, err)
	require.Len(t, im.Result.Products, 1)
	target := im.Result.Products[0].ID
	click := ClickInput{SessionToken: im.SessionToken, ImpressionID: im.ID, EventID: uuid.NewString(), ProductID: target, Position: 1}
	_, err = s.RecordClick(ctx, click)
	require.ErrorIs(t, err, ErrAnalyticsInvalid)
	click.Position = 2
	_, err = s.RecordClick(ctx, click)
	require.NoError(t, err)
	require.NoError(t, s.db.Model(&models.Product{}).Where("id IN ?", []uint{p, second.ID}).Update("is_published", false).Error)
	require.NoError(t, s.SyncProduct(ctx, p))
	require.NoError(t, s.SyncProduct(ctx, second.ID))
	current, err := s.Search(ctx, filters)
	require.NoError(t, err)
	require.Zero(t, current.Total)
	replay, err := s.CreateImpressionIdempotent(ctx, true, im.SessionToken, im.ID, filters)
	require.NoError(t, err)
	require.Equal(t, im.Result, replay.Result)
	metrics, err := s.Analytics(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, 1.0, metrics.CTRAt5)
}
func TestAnalyticsRetentionJobIsDailyAndRejectsMalformedPayloads(t *testing.T) {
	s, _ := analyticsFixture(t)
	runtime := jobs.NewRuntime(s.db, jobs.Config{})
	s.jobs = runtime
	now := time.Date(2026, 10, 7, 8, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	require.NoError(t, s.RegisterJobHandlers())
	first, err := s.EnqueueAnalyticsRetention(context.Background())
	require.NoError(t, err)
	again, err := s.EnqueueAnalyticsRetention(context.Background())
	require.NoError(t, err)
	require.Equal(t, first.ID, again.ID)
	s.now = func() time.Time { return now.Add(24 * time.Hour) }
	next, err := s.EnqueueAnalyticsRetention(context.Background())
	require.NoError(t, err)
	require.NotEqual(t, first.ID, next.ID)
	for _, raw := range []json.RawMessage{json.RawMessage(`{}`), json.RawMessage(`{"version":2}`), json.RawMessage(`{`)} {
		err = s.handleAnalyticsRetention(context.Background(), raw)
		class, ok := reliability.ErrorClassOf(err)
		require.True(t, ok)
		require.Equal(t, reliability.ClassTerminal, class)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, s.PruneAnalytics(canceled), context.Canceled)
}
