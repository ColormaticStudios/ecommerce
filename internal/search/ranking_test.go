package search

import (
	"context"
	"math"
	"testing"
	"time"

	"ecommerce/models"
	"github.com/stretchr/testify/require"
)

func TestRankingWeightsRejectInvalidNumbers(t *testing.T) {
	for _, value := range []float64{-1, 101, math.NaN(), math.Inf(1), math.Inf(-1)} {
		weights := DefaultRankingWeights()
		weights.Sales = value
		require.ErrorIs(t, validateRankingWeights(weights), ErrConfigurationInvalid)
	}
	require.ErrorIs(t, validateRankingWeights(RankingWeights{}), ErrConfigurationInvalid)
	require.NoError(t, validateRankingWeights(RankingWeights{Sales: 100}))
}

func TestRankingConfigurationChangesLiveAndPaginationIsStable(t *testing.T) {
	ctx := context.Background()
	db := searchTestDB(t)
	now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	backend := NewDatabaseBackend(db).(*databaseBackend)
	backend.now = func() time.Time { return now }
	service := NewService(db, backend, nil)
	for _, id := range []uint{3, 1, 2} {
		product := models.Product{BaseModel: models.BaseModel{ID: id, CreatedAt: now.Add(-time.Duration(id) * 24 * time.Hour), UpdatedAt: now}, SKU: "shoe", Name: "Trail Shoe", Description: "Walking footwear", Price: models.MoneyFromFloat(float64(id) * 10), IsPublished: true, Variants: []models.ProductVariant{{SKU: "shoe", IsPublished: true, Stock: 5}}}
		doc, include, err := projectProduct(product, now)
		require.NoError(t, err)
		require.True(t, include)
		require.NoError(t, backend.Upsert(ctx, doc))
	}
	// Remove every varying business component, making the ID tie-break observable.
	weights := RankingWeights{Name: 1}
	profile, err := service.CreateRankingProfile(ctx, RankingProfileInput{Name: "ties", Weights: weights}, nil)
	require.NoError(t, err)
	for repeat := 0; repeat < 3; repeat++ {
		for page := 1; page <= 3; page++ {
			result, err := service.Search(ctx, Filters{Query: "shoe", RankingProfile: "ties", Page: page, Limit: 1, Explain: true})
			require.NoError(t, err)
			require.Equal(t, uint(page), result.Products[0].ID)
			require.Equal(t, "ties", result.RankingProfile)
			require.Equal(t, 1, result.RankingProfileVersion)
			require.Len(t, result.Explanations, 1)
			require.Len(t, result.Explanations[0].Components, 10)
			sum := 0.0
			for _, c := range result.Explanations[0].Components {
				require.InDelta(t, c.Value*c.Weight, c.Contribution, 1e-12)
				sum += c.Contribution
			}
			require.InDelta(t, sum, result.Explanations[0].Score, 1e-12)
		}
	}
	// Changing the named configuration takes effect without reindexing.
	require.NoError(t, db.Create(&models.SearchSalesSignal{ProductID: 3, Units30Days: 100, AsOf: now, UpdatedAt: now}).Error)
	weights = RankingWeights{Sales: 1}
	actor := uint(12)
	updated, err := service.UpdateRankingProfile(ctx, profile.ID, RankingProfilePatch{Weights: &weights}, &actor)
	require.NoError(t, err)
	require.Equal(t, 2, updated.Version)
	require.Equal(t, &actor, updated.UpdatedBy)
	result, err := service.Search(ctx, Filters{Query: "shoe", RankingProfile: "ties", Limit: 3})
	require.NoError(t, err)
	require.Equal(t, uint(3), result.Products[0].ID)
	require.Equal(t, 2, result.RankingProfileVersion)
	require.Empty(t, result.Explanations)
	// Explicit ordering is independent of score even when explanations are requested.
	result, err = service.Search(ctx, Filters{Query: "shoe", RankingProfile: "ties", SortField: "price", SortOrder: "asc", Explain: true, Limit: 3})
	require.NoError(t, err)
	require.Equal(t, uint(1), result.Products[0].ID)
	require.Equal(t, uint(3), result.Products[2].ID)
	_, err = service.Search(ctx, Filters{Query: "shoe", RankingProfile: "missing"})
	require.ErrorIs(t, err, ErrConfigurationNotFound)
	// An empty query still defaults to newest, regardless of ranking weights.
	result, err = service.Search(ctx, Filters{RankingProfile: "ties", Limit: 3})
	require.NoError(t, err)
	require.Equal(t, uint(1), result.Products[0].ID)
}

func TestRankingOriginalTermsOutscoreSynonymsAndBusinessComponentsAreBounded(t *testing.T) {
	now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	plan := queryPlan{groups: []queryGroup{{original: "shoe", alternatives: []string{"sneaker"}}}}
	profile := RankingProfile{Weights: DefaultRankingWeights()}
	document := indexedProduct{document: &models.SearchDocument{EntityID: 1, SearchableText: "shoe", Available: true, SourceCreatedAt: now.Add(-30 * 24 * time.Hour)}, product: &models.Product{Name: "Shoe"}}
	original := explainRanking(document, plan, "shoe", profile, 20, now)
	require.Equal(t, 1.0, original.Components[0].Value)
	require.Equal(t, .5, original.Components[5].Value)
	require.Equal(t, .5, original.Components[7].Value)
	document.document.SearchableText = "sneaker"
	document.product.Name = "Sneaker"
	synonym := explainRanking(document, plan, "shoe", profile, 20, now)
	require.Equal(t, .5, synonym.Components[0].Value)
	require.Greater(t, original.Score, synonym.Score)
	require.Zero(t, rankingCoverage("shoebox", plan))
	document.document.SourceCreatedAt = now.Add(24 * time.Hour)
	future := explainRanking(document, plan, "shoe", profile, -1, now)
	require.Equal(t, 1.0, future.Components[5].Value)
	require.Zero(t, future.Components[7].Value)
}

func TestRankingPhraseBonusRequiresMultipleWords(t *testing.T) {
	now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	profile := RankingProfile{Weights: RankingWeights{ExactPhrase: 6}}
	for _, test := range []struct {
		name, query, productName, description string
		want                                  float64
	}{
		{"single word in name", "red", "Red Cup", "", 0},
		{"single word in description", "red", "Cup", "Red ceramic", 0},
		{"empty query", "", "Red Cup", "Red ceramic", 0},
		{"phrase in name", "red cup", "Travel Red Cup", "Red cup for travel", 1},
		{"phrase in description", "red cup", "Travel Drinkware", "A red cup for travel", .5},
		{"separated words", "red cup", "Red Travel Cup", "", 0},
		{"reversed words", "red cup", "Cup Red", "", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			document := indexedProduct{document: &models.SearchDocument{SourceCreatedAt: now}, product: &models.Product{Name: test.productName, Description: test.description}}
			explanation := explainRanking(document, queryPlan{}, test.query, profile, 0, now)
			require.Equal(t, test.want, explanation.Components[1].Value)
			require.Equal(t, test.want*6, explanation.Components[1].Contribution)
			require.Equal(t, test.want*6, explanation.Score)
		})
	}
}
