package search

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"ecommerce/models"
	"github.com/stretchr/testify/require"
)

func TestDocumentCacheInvalidatesAcrossBackendInstances(t *testing.T) {
	db := searchTestDB(t)
	p := models.Product{Name: "Initial", SKU: "CACHE", IsPublished: true}
	require.NoError(t, db.Create(&p).Error)
	writer, reader := NewService(db, nil, nil), NewService(db, nil, nil)
	require.NoError(t, writer.Reindex(context.Background()))
	result, err := reader.Search(context.Background(), Filters{})
	require.NoError(t, err)
	require.Equal(t, "Initial", result.Products[0].Name)
	// Product responses must not share mutable slices or pointers with cache rows.
	result.Products[0].Name = "Caller mutation"
	result.Products[0].Categories = append(result.Products[0].Categories, models.Category{Name: "Caller category"})
	result, err = reader.Search(context.Background(), Filters{})
	require.NoError(t, err)
	require.Equal(t, "Initial", result.Products[0].Name)
	require.Empty(t, result.Products[0].Categories)
	frozen := time.Now().UTC()
	writer.backend.(*databaseBackend).now = func() time.Time { return frozen }
	require.NoError(t, db.Model(&p).Updates(map[string]any{"name": "Changed", "updated_at": frozen}).Error)
	require.NoError(t, writer.SyncProduct(context.Background(), p.ID))
	result, err = reader.Search(context.Background(), Filters{})
	require.NoError(t, err)
	require.Equal(t, "Changed", result.Products[0].Name)
	require.NoError(t, db.Model(&p).Updates(map[string]any{"is_published": false, "updated_at": frozen.Add(time.Second)}).Error)
	require.NoError(t, writer.SyncProduct(context.Background(), p.ID))
	result, err = reader.Search(context.Background(), Filters{})
	require.NoError(t, err)
	require.Zero(t, result.Total)
	require.NoError(t, db.Model(&p).Updates(map[string]any{"is_published": true, "updated_at": frozen.Add(2 * time.Second)}).Error)
	require.NoError(t, writer.Reindex(context.Background()))
	result, err = reader.Search(context.Background(), Filters{})
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)
}
func TestDocumentCacheKeepsLiveRulesAndDeletes(t *testing.T) {
	db := searchTestDB(t)
	p := models.Product{Name: "Shoe", SKU: "LIVE", IsPublished: true}
	require.NoError(t, db.Create(&p).Error)
	s := NewService(db, nil, nil)
	require.NoError(t, s.Reindex(context.Background()))
	_, err := s.Search(context.Background(), Filters{Query: "shoe"})
	require.NoError(t, err)
	rule, err := s.CreateMerchandisingRule(context.Background(), MerchandisingRuleInput{Name: "Live hide", RuleType: "hide", IsActive: true, Action: MerchandisingAction{Targets: []MerchandisingTarget{{ProductID: p.ID}}}}, nil)
	require.NoError(t, err)
	result, err := s.Search(context.Background(), Filters{Query: "shoe"})
	require.NoError(t, err)
	require.Zero(t, result.Total)
	require.NoError(t, s.DeleteMerchandisingRule(context.Background(), rule.ID, nil))
	result, err = s.Search(context.Background(), Filters{Query: "shoe"})
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)
	_, err = s.CreateSynonymSet(context.Background(), SynonymSetInput{Name: "Live synonym", Direction: "bi", Terms: []string{"trainer", "shoe"}, IsActive: true})
	require.NoError(t, err)
	result, err = s.Search(context.Background(), Filters{Query: "trainer"})
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)
}
func TestDocumentCacheIsBounded(t *testing.T) {
	db := searchTestDB(t)
	s := NewService(db, nil, nil)
	require.NoError(t, s.Reindex(context.Background()))
	b := s.backend.(*databaseBackend)
	for i := 0; i < 40; i++ {
		raw, _ := json.Marshal(i)
		plan := queryPlan{groups: []queryGroup{{alternatives: []string{string(raw)}}}}
		_, err := b.loadIndexedProducts(context.Background(), &plan)
		require.NoError(t, err)
	}
	require.LessOrEqual(t, len(b.cache.entries), cachedPlanLimit)
}
func TestCircuitIgnoresOldInflightCompletion(t *testing.T) {
	c := DefaultHardeningConfig()
	c.CircuitFailureThreshold = 1
	h := newHardeningState(c)
	now := time.Now()
	allowed, generation := h.probe(now)
	require.True(t, allowed)
	h.finish(now, ErrIndexUnavailable, generation)
	h.finish(now, nil, generation)
	allowed, _ = h.probe(now)
	require.False(t, allowed)
}

func TestRankingCacheUsesLiveWeightsSalesAndImmutableExplanations(t *testing.T) {
	db := searchTestDB(t)
	ctx := context.Background()
	products := make([]models.Product, 500)
	for i := range products {
		products[i] = models.Product{Name: "Shoe", SKU: fmt.Sprintf("SCORE-%d", i), IsPublished: true}
	}
	require.NoError(t, db.CreateInBatches(&products, 100).Error)
	s := NewService(db, nil, nil)
	require.NoError(t, s.Reindex(ctx))
	result, err := s.Search(ctx, Filters{Query: "shoe", Explain: true})
	require.NoError(t, err)
	firstScore := result.Explanations[0].Score
	result.Explanations[0].Components[0].Value = 999
	result, err = s.Search(ctx, Filters{Query: "shoe", Explain: true})
	require.NoError(t, err)
	require.NotEqual(t, 999.0, result.Explanations[0].Components[0].Value)
	profiles, err := s.ListRankingProfiles(ctx)
	require.NoError(t, err)
	weights := profiles[0].Weights
	weights.Name += 10
	_, err = s.UpdateRankingProfile(ctx, profiles[0].ID, RankingProfilePatch{Weights: &weights}, nil)
	require.NoError(t, err)
	result, err = s.Search(ctx, Filters{Query: "shoe", Explain: true})
	require.NoError(t, err)
	require.Greater(t, result.Explanations[0].Score, firstScore)
	require.NoError(t, db.Exec(`CREATE TABLE orders (id INTEGER PRIMARY KEY,status TEXT,created_at DATETIME,deleted_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE order_items (id INTEGER PRIMARY KEY,order_id INTEGER,product_variant_id INTEGER,quantity INTEGER,deleted_at DATETIME)`).Error)
	target := products[len(products)-1]
	variant := models.ProductVariant{ProductID: target.ID, SKU: "SCORED", IsPublished: true}
	require.NoError(t, db.Create(&variant).Error)
	midnight := time.Now().UTC().Truncate(24 * time.Hour)
	require.NoError(t, db.Exec(`INSERT INTO orders VALUES (1,?,?,NULL)`, models.StatusPaid, midnight.Add(-time.Hour)).Error)
	require.NoError(t, db.Exec(`INSERT INTO order_items VALUES (1,1,?,100,NULL)`, variant.ID).Error)
	var before models.SearchIndexState
	require.NoError(t, db.First(&before, "name = ?", ProductIndexName).Error)
	require.NoError(t, s.RefreshSalesSignals(ctx, midnight))
	var after models.SearchIndexState
	require.NoError(t, db.First(&after, "name = ?", ProductIndexName).Error)
	require.Greater(t, after.Generation, before.Generation)
	require.Equal(t, before.LastIndexedAt, after.LastIndexedAt)
	result, err = s.Search(ctx, Filters{Query: "shoe", Explain: true})
	require.NoError(t, err)
	require.Equal(t, target.ID, result.Products[0].ID)
}

func TestCanceledHalfOpenProbeDoesNotStrandCircuit(t *testing.T) {
	c := DefaultHardeningConfig()
	c.CircuitFailureThreshold = 1
	c.CircuitOpenMS = 10
	h := newHardeningState(c)
	now := time.Now()
	_, generation := h.probe(now)
	h.finish(now, ErrIndexUnavailable, generation)
	later := now.Add(11 * time.Millisecond)
	allowed, generation := h.probe(later)
	require.True(t, allowed)
	h.finish(later, context.Canceled, generation)
	allowed, _ = h.probe(later)
	require.True(t, allowed)
}
