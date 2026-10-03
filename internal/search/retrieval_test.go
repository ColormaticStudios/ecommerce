package search

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"ecommerce/models"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func indexedFixture(t *testing.T, db *gorm.DB, id uint, name, brand, category, color string, price float64, available bool) {
	t.Helper()
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	product := models.Product{
		BaseModel: models.BaseModel{ID: id, CreatedAt: now, UpdatedAt: now},
		SKU:       name, Name: name, Price: models.MoneyFromFloat(price), IsPublished: true,
	}
	if brand != "" {
		product.Brand = &models.Brand{Name: brand, Slug: brand, IsActive: true}
	}
	if category != "" {
		product.Categories = []models.Category{{Name: category, Slug: category, IsActive: true}}
	}
	if color != "" {
		value := color
		product.AttributeValues = []models.ProductAttributeValue{{
			TextValue:        &value,
			ProductAttribute: &models.ProductAttribute{Key: "Color", Slug: "color", Filterable: true},
		}}
	}
	if available {
		product.Stock = 1
	}
	document, include, err := projectProduct(product, now)
	require.NoError(t, err)
	require.True(t, include)
	require.NoError(t, db.Create(&document).Error)
}

func facetByName(t *testing.T, facets []Facet, name string) Facet {
	t.Helper()
	for _, facet := range facets {
		if facet.Name == name {
			return facet
		}
	}
	t.Fatalf("facet %q was not returned", name)
	return Facet{}
}

func facetValueByName(t *testing.T, facet Facet, value string) FacetValue {
	t.Helper()
	for _, candidate := range facet.Values {
		if candidate.Value == value {
			return candidate
		}
	}
	t.Fatalf("facet %q has no value %q", facet.Name, value)
	return FacetValue{}
}

func TestSearchFacetsUseDisjunctiveContextsAndRetainSelections(t *testing.T) {
	db := searchTestDB(t)
	indexedFixture(t, db, 1, "Red Trail Shoe", "north", "trail", "red", 20, true)
	indexedFixture(t, db, 2, "Blue Trail Shoe", "south", "trail", "blue", 40, false)
	indexedFixture(t, db, 3, "Blue City Shoe", "north", "city", "blue", 80, true)
	service := NewService(db, nil, nil)

	result, err := service.Search(context.Background(), Filters{
		BrandSlugs: []string{"north", "south"}, CategorySlugs: []string{"trail", "city"},
		StockAvailability: []bool{true, false}, AttributeValues: map[string][]string{"color": {"red", "blue"}},
		Page: 1, Limit: 2,
	})
	require.NoError(t, err)
	require.EqualValues(t, 3, result.Total)
	require.Equal(t, 2, result.TotalPages)
	require.Len(t, result.Products, 2)
	require.EqualValues(t, 2, facetValueByName(t, facetByName(t, result.Facets, "category"), "trail").Count)
	require.EqualValues(t, 1, facetValueByName(t, facetByName(t, result.Facets, "category"), "city").Count)
	require.EqualValues(t, 2, facetValueByName(t, facetByName(t, result.Facets, "brand"), "north").Count)
	require.EqualValues(t, 1, facetValueByName(t, facetByName(t, result.Facets, "brand"), "south").Count)
	require.EqualValues(t, 1, facetValueByName(t, facetByName(t, result.Facets, "attribute:color"), "red").Count)
	require.EqualValues(t, 2, facetValueByName(t, facetByName(t, result.Facets, "attribute:color"), "blue").Count)
	require.True(t, facetValueByName(t, facetByName(t, result.Facets, "attribute:color"), "blue").Selected)
	require.EqualValues(t, 2, facetValueByName(t, facetByName(t, result.Facets, "stock"), "true").Count)

	selected, err := service.Search(context.Background(), Filters{
		BrandSlugs: []string{"north"}, CategorySlugs: []string{"trail"},
		AttributeValues: map[string][]string{"color": {"red"}}, StockAvailability: []bool{true},
		Page: 1, Limit: 10,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, selected.Total)
	require.EqualValues(t, 1, facetValueByName(t, facetByName(t, selected.Facets, "category"), "trail").Count)
	require.EqualValues(t, 1, facetValueByName(t, facetByName(t, selected.Facets, "attribute:color"), "red").Count)
	require.EqualValues(t, 0, facetValueByName(t, facetByName(t, selected.Facets, "stock"), "false").Count)
	conflicting, err := service.Search(context.Background(), Filters{
		BrandSlugs: []string{"north"}, CategorySlugs: []string{"trail"},
		AttributeValues: map[string][]string{"color": {"blue"}}, StockAvailability: []bool{true},
		Page: 1, Limit: 10,
	})
	require.NoError(t, err)
	require.Zero(t, conflicting.Total)
	require.EqualValues(t, 1, facetValueByName(t, facetByName(t, conflicting.Facets, "category"), "city").Count,
		"category counts must ignore the active category selection")

	empty, err := service.Search(context.Background(), Filters{CategorySlugs: []string{"unknown"}, Page: 1, Limit: 10})
	require.NoError(t, err)
	require.Zero(t, empty.Total)
	unknown := facetValueByName(t, facetByName(t, empty.Facets, "category"), "unknown")
	require.True(t, unknown.Selected)
	require.True(t, unknown.Disabled)
	require.Zero(t, unknown.Count)
	low, high := 20.0, 80.0
	priceFiltered, err := service.Search(context.Background(), Filters{
		PriceRanges: []PriceRange{{Min: &low, Max: &low}, {Min: &high, Max: &high}},
		Page:        1, Limit: 10,
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, priceFiltered.Total)
	require.True(t, facetValueByName(t, facetByName(t, priceFiltered.Facets, "price"), "20.00:20.00").Selected)
	require.True(t, facetValueByName(t, facetByName(t, priceFiltered.Facets, "price"), "80.00:80.00").Selected)
	require.EqualValues(t, 1, facetValueByName(t, facetByName(t, priceFiltered.Facets, "category"), "trail").Count)
	require.EqualValues(t, 1, facetValueByName(t, facetByName(t, priceFiltered.Facets, "category"), "city").Count)
}

func TestSearchSynonymsTyposAndRelaxedFallback(t *testing.T) {
	db := searchTestDB(t)
	indexedFixture(t, db, 1, "Red Running Shoes", "north", "shoes", "red", 25, true)
	indexedFixture(t, db, 2, "Cotton T Shirt", "south", "shirts", "white", 30, true)
	service := NewService(db, nil, nil)
	ctx := context.Background()

	uni, err := service.CreateSynonymSet(ctx, SynonymSetInput{
		Name: "footwear", Direction: SynonymDirectionUnidirectional,
		Terms: []string{"sneaker", "running shoes"}, IsActive: true,
	})
	require.NoError(t, err)
	bi, err := service.CreateSynonymSet(ctx, SynonymSetInput{
		Name: "shirts", Direction: SynonymDirectionBidirectional,
		Terms: []string{"tee", "t shirt"}, IsActive: true,
	})
	require.NoError(t, err)
	for _, query := range []string{"sneaker", "tee"} {
		result, err := service.Search(ctx, Filters{Query: query, Page: 1, Limit: 10})
		require.NoError(t, err)
		require.EqualValues(t, 1, result.Total)
		require.NotEmpty(t, result.AppliedRewrites)
	}
	indexedFixture(t, db, 3, "Leather Boots", "south", "shoes", "brown", 45, true)
	secondExpansion, err := service.CreateSynonymSet(ctx, SynonymSetInput{
		Name: "more footwear", Direction: SynonymDirectionUnidirectional,
		Terms: []string{"sneaker", "boots"}, IsActive: true,
	})
	require.NoError(t, err)
	expanded, err := service.Search(ctx, Filters{Query: "sneaker", Page: 1, Limit: 10})
	require.NoError(t, err)
	require.EqualValues(t, 2, expanded.Total, "sets sharing a source must combine their alternatives")
	require.NoError(t, service.DeleteSynonymSet(ctx, secondExpansion.ID))
	reverse, err := service.Search(ctx, Filters{Query: "running shoes", Page: 1, Limit: 10})
	require.NoError(t, err)
	require.Empty(t, reverse.AppliedRewrites, "a unidirectional replacement must not expand back to its source")
	require.NoError(t, service.DeleteSynonymSet(ctx, uni.ID))
	require.NoError(t, service.DeleteSynonymSet(ctx, bi.ID))

	corrected, err := service.Search(ctx, Filters{Query: "runnng", Page: 1, Limit: 10})
	require.NoError(t, err)
	require.EqualValues(t, 1, corrected.Total)
	require.Equal(t, "running", corrected.DidYouMean)
	require.Equal(t, []AppliedRewrite{{Kind: "typo", Original: "runnng", Replacement: "running"}}, corrected.AppliedRewrites)
	require.False(t, corrected.Relaxed)
	filteredCorrection, err := service.Search(ctx, Filters{
		Query: "runnng", CategorySlugs: []string{"unknown"}, Page: 1, Limit: 10,
	})
	require.NoError(t, err)
	require.Zero(t, filteredCorrection.Total)
	require.Empty(t, filteredCorrection.DidYouMean, "a correction must recover a result within the selected filters")
	hints, err := service.Suggest(ctx, "runnng", 5)
	require.NoError(t, err)
	require.Equal(t, []string{"running"}, hints.Corrections)
	require.Empty(t, hints.Popular)

	relaxed, err := service.Search(ctx, Filters{Query: "red impossible", Page: 1, Limit: 10})
	require.NoError(t, err)
	require.EqualValues(t, 1, relaxed.Total)
	require.True(t, relaxed.Relaxed)

	_, err = service.CreateTypoToleranceProfile(ctx, TypoToleranceProfileInput{
		Name: "strict", MinimumTokenLength: 4, OneEditMinimumLength: 4,
		TwoEditMinimumLength: 8, StrictMode: true, IsActive: true,
	})
	require.NoError(t, err)
	strict, err := service.Search(ctx, Filters{Query: "runnng", Page: 1, Limit: 10})
	require.NoError(t, err)
	require.Zero(t, strict.Total)
	require.Equal(t, "running", strict.DidYouMean)
	require.Empty(t, strict.AppliedRewrites)
	require.False(t, strict.Relaxed)
}

func TestTypoCorrectionCorpusKeepsKnownShortAndNumericTerms(t *testing.T) {
	profile := models.SearchTypoToleranceProfile{
		MinimumTokenLength: 4, OneEditMinimumLength: 4, TwoEditMinimumLength: 8,
	}
	vocabulary := []vocabularyEntry{{term: "running", frequency: 4}, {term: "runner", frequency: 2}, {term: "red", frequency: 5}}
	for _, test := range []struct {
		query string
		want  string
	}{
		{query: "running", want: "running"},
		{query: "run", want: "run"},
		{query: "run1ng", want: "run1ng"},
		{query: "runnng", want: "running"},
		{query: "runnign", want: "running"},
	} {
		corrected, _ := correctQuery(test.query, vocabulary, profile)
		require.Equal(t, test.want, corrected, test.query)
	}
}

func TestDynamicPriceBucketsCoverHighestIndexedPrice(t *testing.T) {
	db := searchTestDB(t)
	indexedFixture(t, db, 1, "Low", "", "", "", 49.99, true)
	indexedFixture(t, db, 2, "High", "", "", "", 250.01, true)
	result, err := NewService(db, nil, nil).Search(context.Background(), Filters{Page: 1, Limit: 10})
	require.NoError(t, err)
	price := facetByName(t, result.Facets, "price")
	require.NotEmpty(t, price.Values)
	require.LessOrEqual(t, len(price.Values), 5)
	last := price.Values[len(price.Values)-1]
	require.NotNil(t, last.MaxPrice)
	require.GreaterOrEqual(t, *last.MaxPrice, 250.01)
	counted := false
	for _, value := range price.Values {
		if value.Count > 0 && value.MaxPrice != nil && *value.MaxPrice >= 250.01 {
			counted = true
		}
	}
	require.True(t, counted)
}

func TestTopAttributeFacetsExcludeNonfilterableAndKeepSelection(t *testing.T) {
	db := searchTestDB(t)
	now := time.Now().UTC()
	product := models.Product{
		BaseModel: models.BaseModel{ID: 1, CreatedAt: now, UpdatedAt: now},
		SKU:       "many-attributes", Name: "Many Attributes", Price: models.MoneyFromFloat(10), IsPublished: true,
	}
	for index := 0; index < 12; index++ {
		slug := fmt.Sprintf("a%02d", index)
		value := "yes"
		product.AttributeValues = append(product.AttributeValues, models.ProductAttributeValue{
			TextValue:        &value,
			ProductAttribute: &models.ProductAttribute{Key: slug, Slug: slug, Filterable: true},
		})
	}
	secret := "internal"
	product.AttributeValues = append(product.AttributeValues, models.ProductAttributeValue{
		TextValue:        &secret,
		ProductAttribute: &models.ProductAttribute{Key: "Secret", Slug: "secret", Filterable: false},
	})
	document, include, err := projectProduct(product, now)
	require.NoError(t, err)
	require.True(t, include)
	require.NoError(t, db.Create(&document).Error)

	service := NewService(db, nil, nil)
	result, err := service.Search(context.Background(), Filters{Page: 1, Limit: 10})
	require.NoError(t, err)
	attributeNames := make(map[string]struct{})
	for _, facet := range result.Facets {
		if facet.Type == "attribute" {
			attributeNames[facet.Name] = struct{}{}
		}
	}
	require.Len(t, attributeNames, 10)
	require.NotContains(t, attributeNames, "attribute:secret")
	require.NotContains(t, attributeNames, "attribute:a11")

	selected, err := service.Search(context.Background(), Filters{
		AttributeValues: map[string][]string{"a11": {"yes"}}, Page: 1, Limit: 10,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, selected.Total)
	require.True(t, facetValueByName(t, facetByName(t, selected.Facets, "attribute:a11"), "yes").Selected)
}

func TestSearchFailsWhenActiveTypoProfileIsMissing(t *testing.T) {
	db := searchTestDB(t)
	require.NoError(t, db.Model(&models.SearchTypoToleranceProfile{}).Where("name = ?", "default").Update("is_active", false).Error)
	_, err := NewService(db, nil, nil).Search(context.Background(), Filters{Page: 1, Limit: 10})
	require.True(t, errors.Is(err, ErrActiveTypoProfileRequired))
}
