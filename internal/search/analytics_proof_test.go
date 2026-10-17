package search

import (
	"context"
	"ecommerce/models"
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"strings"
	"testing"
	"time"
)

func TestAnalyticsProofTamperingExpiryAndSnapshotDedupe(t *testing.T) {
	s, _ := analyticsFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	s.now = func() time.Time { return now }
	token, _ := analyticsProof(t, s, Filters{Query: "boots", Page: 1, Limit: 10})
	for _, bad := range []string{"", token + "x", strings.Repeat("x", MaxResultTokenLength+1)} {
		_, err := s.RecordImpression(ctx, true, "", uuid.NewString(), bad)
		require.ErrorIs(t, err, ErrAnalyticsInvalid)
	}
	im, err := s.RecordImpression(ctx, true, "", uuid.NewString(), token)
	require.NoError(t, err)
	again, err := s.RecordImpression(ctx, true, im.SessionToken, uuid.NewString(), token)
	require.NoError(t, err)
	require.Equal(t, im.ID, again.ID)
	var count int64
	require.NoError(t, s.db.Model(&models.SearchQueryEvent{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
	_, err = s.RecordImpression(ctx, true, "", im.ID, token)
	require.ErrorIs(t, err, ErrAnalyticsInvalid)
	s.now = func() time.Time { return now.Add(ResultProofLifetime) }
	_, err = s.RecordImpression(ctx, true, im.SessionToken, uuid.NewString(), token)
	require.ErrorIs(t, err, ErrAnalyticsInvalid)
	s.now = func() time.Time { return now.Add(-time.Second) }
	_, err = s.RecordImpression(ctx, true, im.SessionToken, uuid.NewString(), token)
	require.ErrorIs(t, err, ErrAnalyticsInvalid)
}

func TestAnalyticsProofIsCompactAndImpressionNeverRerunsSearch(t *testing.T) {
	s, p := analyticsFixture(t)
	ctx := context.Background()
	f := Filters{Query: "boots", Page: 1, Limit: 10}
	r, err := s.Search(ctx, f)
	require.NoError(t, err)
	r.Products[0].Description = strings.Repeat("private-description", 10000)
	token, err := s.SignResultSnapshot(r, f, time.Millisecond)
	require.NoError(t, err)
	require.Less(t, len(token), 4096)
	require.NoError(t, s.db.Model(&models.Product{}).Where("id=?", p).Update("is_published", false).Error)
	require.NoError(t, s.SyncProduct(ctx, p))
	reads := []string{}
	require.NoError(t, s.db.Callback().Query().Before("gorm:query").Register("proof_no_search", func(db *gorm.DB) { reads = append(reads, db.Statement.Table) }))
	im, err := s.RecordImpression(ctx, true, "", uuid.NewString(), token)
	require.NoError(t, err)
	for _, table := range reads {
		require.NotContains(t, []string{"search_documents", "products", "search_product_revisions"}, table)
	}
	var row models.SearchQueryEvent
	require.NoError(t, s.db.First(&row, "impression_id=?", im.ID).Error)
	require.Equal(t, fmt.Sprintf("[%d]", p), row.ProductsJSON)
	require.Equal(t, 1, row.ProductCount)
	require.False(t, s.db.Migrator().HasColumn(&row, "result_json"))
	rules := []MerchandisingRule{}
	for _, preview := range []Filters{{Explain: true}, {SkipMerchandising: true}, {RuleOverrides: &rules}} {
		proof, err := s.SignResultSnapshot(r, preview, 0)
		require.NoError(t, err)
		require.Empty(t, proof)
	}
	r.Degraded = true
	proof, err := s.SignResultSnapshot(r, f, 0)
	require.NoError(t, err)
	require.Empty(t, proof)
}

func TestAnalyticsReportingPaginationPreservesGlobalMetricsWithConstantQueries(t *testing.T) {
	s, _ := analyticsFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	rows := make([]models.SearchQueryEvent, 120)
	for i := range rows {
		rows[i] = models.SearchQueryEvent{ImpressionID: uuid.NewString(), SnapshotID: uuid.NewString(), SessionHash: fmt.Sprintf("session-%d", i), NormalizedQuery: fmt.Sprintf("query-%03d", i), CreatedAt: now, LatencyMs: int64(i + 1), ProductsJSON: "[1]", FiltersJSON: "{}", FirstPosition: 1, ProductCount: 1, ResultCount: 1}
	}
	require.NoError(t, s.db.Create(&rows).Error)
	queries := 0
	require.NoError(t, s.db.Callback().Raw().Before("gorm:raw").Register("analytics_query_count", func(*gorm.DB) { queries++ }))
	// Raw(...).Scan executes Row callbacks, unlike Exec's Raw callbacks.
	require.NoError(t, s.db.Callback().Row().Before("gorm:row").Register("analytics_row_count", func(*gorm.DB) { queries++ }))
	first, err := s.AnalyticsPage(ctx, 7, 1, 10)
	require.NoError(t, err)
	firstQueries := queries
	last, err := s.AnalyticsPage(ctx, 7, 12, 10)
	require.NoError(t, err)
	require.Equal(t, firstQueries, queries-firstQueries)
	require.LessOrEqual(t, firstQueries, 7)
	require.Equal(t, int64(120), first.Searches)
	require.Equal(t, first.QueryMetrics, last.QueryMetrics)
	require.Equal(t, 120, first.TotalQueries)
	require.Len(t, last.Queries, 10)
	require.Equal(t, "query-110", last.Queries[0].Query)
	require.Equal(t, float64(114), first.P95LatencyMs)
	require.Equal(t, float64(119), first.P99LatencyMs)
	for _, input := range [][3]int{{0, 1, 10}, {7, 0, 10}, {7, 1, 101}} {
		_, err := s.AnalyticsPage(ctx, input[0], input[1], input[2])
		require.ErrorIs(t, err, ErrAnalyticsInvalid)
	}
}
