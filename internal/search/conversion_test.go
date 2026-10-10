package search

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"ecommerce/internal/jobs"
	"ecommerce/internal/reliability"
	"ecommerce/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func cost(value float64) *models.Money { m := models.MoneyFromFloat(value); return &m }
func TestProductMarginUsesLowestPublishedVariant(t *testing.T) {
	for _, test := range []struct {
		name     string
		variants []models.ProductVariant
		want     float64
	}{
		{"none", nil, 0}, {"unpublished", []models.ProductVariant{{Price: 10000, UnitCost: cost(20)}}, 0},
		{"minimum", []models.ProductVariant{{IsPublished: true, Price: 10000, UnitCost: cost(20)}, {IsPublished: true, Price: 20000, UnitCost: cost(150)}}, .25},
		{"unknown cost wins", []models.ProductVariant{{IsPublished: true, Price: 10000, UnitCost: cost(20)}, {IsPublished: true, Price: 10000}}, 0},
		{"zero price", []models.ProductVariant{{IsPublished: true, UnitCost: cost(0)}}, 0},
		{"loss clamped", []models.ProductVariant{{IsPublished: true, Price: 10000, UnitCost: cost(150)}}, 0},
		{"zero known cost", []models.ProductVariant{{IsPublished: true, Price: 10000, UnitCost: cost(0)}}, 1},
		{"unpublished ignored", []models.ProductVariant{{IsPublished: true, Price: 10000, UnitCost: cost(20)}, {Price: 10000}}, .8},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.InDelta(t, test.want, productMarginRate(models.Product{Variants: test.variants}), 1e-12)
		})
	}
	require.Zero(t, DefaultRankingWeights().Margin)
	require.Zero(t, DefaultRankingWeights().Conversion)
	for _, weights := range []RankingWeights{{Margin: math.NaN()}, {Margin: 101}, {Conversion: -1}, {Conversion: math.Inf(1)}} {
		require.Error(t, validateRankingWeights(weights))
	}
	require.NoError(t, validateRankingWeights(RankingWeights{Margin: 100, Conversion: 100}))
}
func TestMarginProjectionCostUpdateAndPrivacy(t *testing.T) {
	db := searchTestDB(t)
	p := models.Product{Name: "Shoe", SKU: "MARGIN", IsPublished: true}
	require.NoError(t, db.Create(&p).Error)
	v := models.ProductVariant{ProductID: p.ID, SKU: "MARGIN-V", Title: "Published", IsPublished: true, Price: 10000, UnitCost: cost(20)}
	require.NoError(t, db.Create(&v).Error)
	s := NewService(db, nil, nil)
	require.NoError(t, s.Reindex(context.Background()))
	var doc models.SearchDocument
	require.NoError(t, db.First(&doc, "entity_id = ?", p.ID).Error)
	require.InDelta(t, .8, doc.MarginRate, 1e-12)
	require.NotContains(t, doc.PayloadJSON, "unit_cost")
	require.NotContains(t, doc.PayloadJSON, "margin")
	result, err := s.Search(context.Background(), Filters{Query: "shoe", Explain: true})
	require.NoError(t, err)
	require.InDelta(t, .8, result.Explanations[0].Components[8].Value, 1e-12)
	raw, err := json.Marshal(result.Products)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "unit_cost")
	require.NotContains(t, string(raw), "MarginRate")
	require.NoError(t, db.Model(&v).Update("unit_cost", models.MoneyFromFloat(90)).Error)
	require.NoError(t, db.Model(&p).Update("updated_at", time.Now().Add(time.Second)).Error)
	require.NoError(t, s.SyncProduct(context.Background(), p.ID))
	result, err = s.Search(context.Background(), Filters{Query: "shoe", Explain: true})
	require.NoError(t, err)
	require.InDelta(t, .1, result.Explanations[0].Components[8].Value, 1e-12)
}
func conversionTestService(t *testing.T) (*Service, *gorm.DB, time.Time) {
	t.Helper()
	db := searchTestDB(t)
	require.NoError(t, db.Exec(`CREATE TABLE orders (id INTEGER PRIMARY KEY,status TEXT,created_at DATETIME,deleted_at DATETIME)`).Error)
	now := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	s := NewService(db, nil, jobs.NewRuntime(db, jobs.Config{}))
	s.now = func() time.Time { return now }
	return s, db, now
}
func addExposure(t *testing.T, db *gorm.DB, name, owner string, at time.Time, ids []uint) models.SearchQueryEvent {
	t.Helper()
	raw, err := json.Marshal(ids)
	require.NoError(t, err)
	q := models.SearchQueryEvent{ImpressionID: name, SessionHash: owner, Query: "shoe", NormalizedQuery: "shoe", ProductsJSON: string(raw), FiltersJSON: "{}", CreatedAt: at}
	require.NoError(t, db.Create(&q).Error)
	return q
}
func addConversion(t *testing.T, db *gorm.DB, q models.SearchQueryEvent, id uint, order uint, status string, paid, clicked time.Time) {
	t.Helper()
	click := models.SearchClickEvent{EventID: fmt.Sprintf("click-%s-%d-%d", q.ImpressionID, id, order), ImpressionID: q.ImpressionID, SessionHash: q.SessionHash, QueryEventID: q.ID, ProductID: id, ClickedAt: clicked}
	require.NoError(t, db.Create(&click).Error)
	require.NoError(t, db.Exec(`INSERT INTO orders VALUES (?,?,?,NULL)`, order, status, q.CreatedAt).Error)
	require.NoError(t, db.Create(&models.SearchOrderAttribution{OrderID: order, ProductID: id, ImpressionID: q.ImpressionID, ClickID: click.EventID, PaidAt: &paid, CreatedAt: clicked}).Error)
}
func TestConversionMatureCohortDistinctExposureAndPaidAttribution(t *testing.T) {
	s, db, now := conversionTestService(t)
	valid := addExposure(t, db, "valid", "owner", now.Add(-10*24*time.Hour), []uint{1, 1, 2})
	addConversion(t, db, valid, 1, 1, models.StatusPaid, valid.CreatedAt.Add(time.Hour), valid.CreatedAt.Add(time.Minute))
	addConversion(t, db, valid, 1, 2, models.StatusShipped, valid.CreatedAt.Add(2*time.Hour), valid.CreatedAt.Add(time.Minute))
	since := addExposure(t, db, "since", "owner", now.Add(-37*24*time.Hour), []uint{1})
	addConversion(t, db, since, 1, 3, models.StatusDelivered, since.CreatedAt.Add(AttributionWindow), since.CreatedAt.Add(time.Hour))
	addExposure(t, db, "young", "owner", now.Add(-7*24*time.Hour), []uint{1})
	addExposure(t, db, "old", "owner", now.Add(-37*24*time.Hour-time.Second), []uint{1})
	revoked := addExposure(t, db, "revoked", "revoked", now.Add(-10*24*time.Hour), []uint{1})
	addConversion(t, db, revoked, 1, 4, models.StatusPaid, revoked.CreatedAt.Add(time.Hour), revoked.CreatedAt)
	require.NoError(t, db.Create(&models.SearchRevokedSession{SessionHash: "revoked", CreatedAt: now}).Error)
	for i, status := range []string{models.StatusRefunded, models.StatusCancelled, models.StatusPending} {
		q := addExposure(t, db, fmt.Sprintf("unpaid%d", i), "owner", now.Add(-10*24*time.Hour), []uint{2})
		addConversion(t, db, q, 2, uint(5+i), status, q.CreatedAt.Add(time.Hour), q.CreatedAt)
	}
	late := addExposure(t, db, "late", "owner", now.Add(-12*24*time.Hour), []uint{2})
	addConversion(t, db, late, 2, 8, models.StatusPaid, late.CreatedAt.Add(AttributionWindow+time.Second), late.CreatedAt)
	require.NoError(t, s.RefreshConversionSignals(context.Background(), now))
	var one, two models.SearchConversionSignal
	require.NoError(t, db.First(&one, "product_id = 1").Error)
	require.NoError(t, db.First(&two, "product_id = 2").Error)
	require.Equal(t, int64(2), one.Impressions30Days)
	require.Equal(t, int64(2), one.Conversions30Days)
	require.InDelta(t, 2.0/22, conversionRate(one), 1e-12)
	require.Equal(t, int64(5), two.Impressions30Days)
	require.Zero(t, two.Conversions30Days)
	require.NoError(t, s.RefreshConversionSignals(context.Background(), now))
	var again models.SearchConversionSignal
	require.NoError(t, db.First(&again, "product_id = 1").Error)
	require.Equal(t, one.Impressions30Days, again.Impressions30Days)
	s.now = func() time.Time { return now.Add(40 * 24 * time.Hour) }
	require.NoError(t, s.RefreshConversionSignals(context.Background(), s.now()))
	require.NoError(t, db.First(&again, "product_id = 1").Error)
	require.Zero(t, again.Impressions30Days)
	require.Zero(t, again.Conversions30Days)
	require.NoError(t, s.RefreshConversionSignals(context.Background(), now))
	require.NoError(t, db.First(&again, "product_id = 1").Error)
	require.Zero(t, again.Impressions30Days)
}
func TestConversionRefreshValidationAtomicityAndRevocation(t *testing.T) {
	s, db, now := conversionTestService(t)
	token := strings.Repeat("a", 64)
	owner, err := sessionHash(token)
	require.NoError(t, err)
	q := addExposure(t, db, "revoke", owner, now.Add(-10*24*time.Hour), []uint{1})
	addConversion(t, db, q, 1, 1, models.StatusPaid, q.CreatedAt.Add(time.Hour), q.CreatedAt)
	require.NoError(t, s.RefreshConversionSignals(context.Background(), now))
	require.NoError(t, s.RevokeConsent(context.Background(), token))
	var signal models.SearchConversionSignal
	require.NoError(t, db.First(&signal, "product_id = 1").Error)
	require.Zero(t, signal.Impressions30Days)
	require.Zero(t, signal.Conversions30Days)
	require.Error(t, s.RefreshConversionSignals(context.Background(), now.Add(time.Hour)))
	require.Error(t, s.RefreshConversionSignals(context.Background(), now.Add(24*time.Hour)))
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, s.RefreshConversionSignals(canceled, now), context.Canceled)
	for _, raw := range []string{`{}`, `{"version":2}`, `{"version":1,"as_of":"2026-10-09T00:00:00Z"}`} {
		class, ok := reliability.ErrorClassOf(s.handleConversionRefresh(context.Background(), []byte(raw)))
		require.True(t, ok)
		require.Equal(t, reliability.ClassTerminal, class)
	}
	first, err := s.EnqueueConversionRefresh(context.Background())
	require.NoError(t, err)
	second, err := s.EnqueueConversionRefresh(context.Background())
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	// A failed replacement retains the previous snapshot and generation.
	addExposure(t, db, "rollback", "other", now.Add(-10*24*time.Hour), []uint{1})
	var before models.SearchIndexState
	require.NoError(t, db.First(&before, "name = ?", ProductIndexName).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER fail_conversion BEFORE UPDATE ON search_conversion_signals BEGIN SELECT RAISE(ABORT,'test failure'); END`).Error)
	require.Error(t, s.RefreshConversionSignals(context.Background(), now))
	var after models.SearchIndexState
	require.NoError(t, db.First(&after, "name = ?", ProductIndexName).Error)
	require.Equal(t, before.Generation, after.Generation)
	require.NoError(t, db.First(&signal, "product_id = 1").Error)
	require.Zero(t, signal.Impressions30Days)
}

func TestEmptyConversionSnapshotCannotBeOverwrittenByOlderJob(t *testing.T) {
	s, db, now := conversionTestService(t)
	require.NoError(t, s.RefreshConversionSignals(context.Background(), now))
	addExposure(t, db, "late-backfill", "owner", now.Add(-10*24*time.Hour), []uint{1})
	require.NoError(t, s.RefreshConversionSignals(context.Background(), now.Add(-24*time.Hour)))
	var count int64
	require.NoError(t, db.Model(&models.SearchConversionSignal{}).Count(&count).Error)
	require.Zero(t, count)
	var state models.SearchIndexState
	require.NoError(t, db.First(&state, "name = ?", ProductIndexName).Error)
	require.True(t, state.LastConversionRefreshAt.Equal(now))
	require.Nil(t, state.LastIndexedAt)
	require.Nil(t, state.LastFullReindexAt)
}
func TestConversionSnapshotInvalidatesRankingCache(t *testing.T) {
	s, db, now := conversionTestService(t)
	products := make([]models.Product, 500)
	ids := make([]uint, 500)
	for i := range products {
		products[i] = models.Product{Name: "Shoe", SKU: fmt.Sprintf("CONVERSION-%d", i), IsPublished: true}
	}
	require.NoError(t, db.CreateInBatches(&products, 100).Error)
	for i, p := range products {
		ids[i] = p.ID
	}
	require.NoError(t, s.Reindex(context.Background()))
	_, err := s.CreateRankingProfile(context.Background(), RankingProfileInput{Name: "conversion", IsDefault: true, Weights: RankingWeights{Conversion: 10}}, nil)
	require.NoError(t, err)
	result, err := s.Search(context.Background(), Filters{Query: "shoe", Explain: true})
	require.NoError(t, err)
	require.Equal(t, products[0].ID, result.Products[0].ID)
	q := addExposure(t, db, "all-products", "owner", now.Add(-10*24*time.Hour), ids)
	target := products[len(products)-1].ID
	addConversion(t, db, q, target, 1, models.StatusPaid, q.CreatedAt.Add(time.Hour), q.CreatedAt)
	require.NoError(t, s.RefreshConversionSignals(context.Background(), now))
	result, err = s.Search(context.Background(), Filters{Query: "shoe", Explain: true})
	require.NoError(t, err)
	require.Equal(t, target, result.Products[0].ID)
	require.InDelta(t, 1.0/21, result.Explanations[0].Components[9].Value, 1e-12)
	require.NoError(t, db.Exec("UPDATE orders SET status = ?", models.StatusRefunded).Error)
	require.NoError(t, s.RefreshConversionSignals(context.Background(), now))
	result, err = s.Search(context.Background(), Filters{Query: "shoe"})
	require.NoError(t, err)
	require.Equal(t, products[0].ID, result.Products[0].ID)
}
func TestMalformedConversionSourceIsTerminalAndDoesNotReplaceSnapshot(t *testing.T) {
	s, db, now := conversionTestService(t)
	q := addExposure(t, db, "bad", "owner", now.Add(-10*24*time.Hour), []uint{1})
	require.NoError(t, s.RefreshConversionSignals(context.Background(), now))
	require.NoError(t, db.Model(&q).Update("products_json", "bad json").Error)
	raw, err := json.Marshal(ConversionRefreshPayload{Version: 1, AsOf: now})
	require.NoError(t, err)
	class, ok := reliability.ErrorClassOf(s.handleConversionRefresh(context.Background(), raw))
	require.True(t, ok)
	require.Equal(t, reliability.ClassTerminal, class)
	var signal models.SearchConversionSignal
	require.NoError(t, db.First(&signal, "product_id = 1").Error)
	require.Equal(t, int64(1), signal.Impressions30Days)
}

func TestConsentWithdrawalClearsDerivedInfluenceWhenProjectionSourceIsCorrupt(t *testing.T) {
	s, db, now := conversionTestService(t)
	token := strings.Repeat("b", 64)
	owner, err := sessionHash(token)
	require.NoError(t, err)
	q := addExposure(t, db, "withdraw", owner, now.Add(-10*24*time.Hour), []uint{1})
	other := addExposure(t, db, "corrupt-other", "other", now.Add(-10*24*time.Hour), []uint{2})
	addConversion(t, db, q, 1, 1, models.StatusPaid, q.CreatedAt.Add(time.Hour), q.CreatedAt)
	require.NoError(t, s.RefreshConversionSignals(context.Background(), now))
	require.NoError(t, db.Model(&other).Update("products_json", "bad JSON").Error)
	require.NoError(t, s.RevokeConsent(context.Background(), token))
	var count int64
	require.NoError(t, db.Model(&models.SearchQueryEvent{}).Where("session_hash = ?", owner).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Model(&models.SearchRevokedSession{}).Where("session_hash = ?", owner).Count(&count).Error)
	require.Equal(t, int64(1), count)
	var signals []models.SearchConversionSignal
	require.NoError(t, db.Find(&signals).Error)
	for _, signal := range signals {
		require.Zero(t, signal.Impressions30Days)
		require.Zero(t, signal.Conversions30Days)
	}
	var state models.SearchIndexState
	require.NoError(t, db.First(&state, "name = ?", ProductIndexName).Error)
	require.True(t, state.LastConversionRefreshAt.Equal(now))
}
