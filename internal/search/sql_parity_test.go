package search

import (
	"context"
	"ecommerce/models"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSQLRetrievalMatchesGoFacetAndRankingOracle(t *testing.T) {
	db := searchTestDB(t)
	for i, name := range []string{"Red Trail Shoe", "Blue Trail Shoe", "Trail Red Shoe", "Red Hiking Shoe", "Été Trail Shoe", "Trail Shoe"} {
		indexedFixture(t, db, uint(i+1), name, []string{"north", "south"}[i%2], []string{"trail", "summer"}[i%2], []string{"red", "blue", "réd"}[i%3], float64(10+i*20), i%3 != 0)
	}
	b := NewDatabaseBackend(db).(*databaseBackend)
	ctx := context.Background()
	all, err := b.loadIndexedProductsUncached(ctx, nil)
	require.NoError(t, err)
	for _, f := range []Filters{{Query: "trail", Limit: 100, Explain: true}, {Query: "red trail", Limit: 100, Explain: true}, {Query: "shoe", BrandSlugs: []string{"north"}, AttributeValues: map[string][]string{"color": {"red", "missing"}}, Limit: 100, Explain: true}, {Query: "shoe", CategorySlugs: []string{"summer"}, StockAvailability: []bool{true}, Limit: 100, SortField: "price", SortOrder: "asc"}, {Query: "shoe", Attributes: map[string]string{"missing": "none"}, Limit: 100}, {Query: "shoe", Limit: 100, SortField: "name", SortOrder: "asc"}} {
		n, err := normalizeFilters(f)
		require.NoError(t, err)
		plan := buildQueryPlan(NormalizeQuery(f.Query), nil)
		expected := filterIndexedProducts(append([]indexedProduct(nil), all...), plan, false, n, "")
		facets := buildFacets(all, plan, false, n)
		profile, err := b.loadRankingProfile(ctx, "")
		require.NoError(t, err)
		if f.SortField == "" {
			require.NoError(t, b.rankProductsUncached(ctx, expected, plan, NormalizeQuery(f.Query), profile, f.SortOrder, true))
		} else {
			sortIndexedProducts(expected, f.SortField, f.SortOrder)
		}
		result, err := b.Search(ctx, f)
		require.NoError(t, err)
		require.Equal(t, int64(len(expected)), result.Total)
		require.Equal(t, facets, result.Facets)
		ids := []uint{}
		for _, d := range expected {
			ids = append(ids, d.document.EntityID)
		}
		actual := []uint{}
		for _, p := range result.Products {
			actual = append(actual, p.ID)
		}
		require.Equal(t, ids, actual)
		if f.Explain {
			for i, e := range result.Explanations {
				require.InDelta(t, expected[i].ranking.Score, e.Score, 1e-10)
			}
		}
	}
}

func TestCanonicalFallbackRefusesOversizedMetadataWithoutApproximation(t *testing.T) {
	db := searchTestDB(t)
	rows := make([]models.Brand, canonicalMetadataLimit+1)
	for i := range rows {
		rows[i] = models.Brand{Name: fmt.Sprintf("Brand%d", i), Slug: fmt.Sprintf("brand%d", i), IsActive: true}
	}
	require.NoError(t, db.CreateInBatches(rows, 250).Error)
	s := NewService(db, faultBackend{search: func(context.Context, Filters) (Result, error) { return Result{}, ErrIndexUnavailable }}, nil)
	_, err := s.Search(context.Background(), Filters{BrandSlug: "brand0"})
	require.ErrorIs(t, err, ErrIndexUnavailable)
}

func TestSQLVocabularyPruningPreservesExactCorrections(t *testing.T) {
	db := searchTestDB(t)
	for i, name := range []string{"trail", "1rali", "trails", "b", "é", "été", "red", "12345"} {
		indexedFixture(t, db, uint(i+1), name, "", "", "", 10, true)
	}
	b := NewDatabaseBackend(db).(*databaseBackend)
	all, err := b.loadIndexedProductsUncached(context.Background(), nil)
	require.NoError(t, err)
	vocabulary := buildVocabulary(all)
	for _, profile := range []models.SearchTypoToleranceProfile{{MinimumTokenLength: 4, OneEditMinimumLength: 4, TwoEditMinimumLength: 8}, {MinimumTokenLength: 1, OneEditMinimumLength: 1, TwoEditMinimumLength: 1}} {
		for _, query := range []string{"trali", "trali shoe", "a", "étè", "réd", "12345", "12346"} {
			pruned, err := b.sqlVocabulary(context.Background(), query, profile)
			require.NoError(t, err)
			want, rewrites := correctQuery(query, vocabulary, profile)
			got, actual := correctQuery(query, pruned, profile)
			require.Equal(t, want, got)
			require.Equal(t, rewrites, actual)
		}
	}
}

func TestSQLSharedFacetsMatchIndependentContexts(t *testing.T) {
	db := searchTestDB(t)
	for i := 1; i <= 30; i++ {
		indexedFixture(t, db, uint(i), "Trail Shoe", fmt.Sprintf("brand%d", i%3), fmt.Sprintf("category%d", i%5), fmt.Sprintf("color%d", i%4), float64(i*10), i%2 == 0)
	}
	b := NewDatabaseBackend(db).(*databaseBackend)
	for _, selection := range []sqlSelection{{}, {plan: buildQueryPlan("trail", nil), hidden: []uint{1, 2}}, {plan: buildQueryPlan("missing", nil)}, {plan: buildQueryPlan("missing", nil), included: []uint{1, 2}}, {plan: buildQueryPlan("missing trail", nil), relaxed: true}} {
		want, err := b.sqlFacetsIndividually(context.Background(), selection)
		require.NoError(t, err)
		got, err := b.sqlFacets(context.Background(), selection)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
}

func TestSQLRetrievalNormalizesUnicodeBrandFilterKeys(t *testing.T) {
	db := searchTestDB(t)
	indexedFixture(t, db, 1, "Trail Shoe", "Été-North!", "trail", "blue", 20, true)
	result, err := NewDatabaseBackend(db).Search(context.Background(), Filters{Query: "trail", BrandSlugs: []string{"été-north!"}})
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)
	brand := facetByName(t, result.Facets, "brand")
	require.Equal(t, "été north", brand.Values[0].Value)
	require.True(t, brand.Values[0].Selected)
	require.Equal(t, "Été-North!", brand.Values[0].Label)
}

func TestSQLRetrievalExtremePageCannotOverflow(t *testing.T) {
	db := searchTestDB(t)
	indexedFixture(t, db, 1, "Trail", "", "", "", 10, true)
	page := int(^uint(0) >> 1)
	result, err := NewDatabaseBackend(db).Search(context.Background(), Filters{Query: "trail", Page: page, Limit: 100})
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)
	require.Empty(t, result.Products)
	p := models.Product{Name: "Canonical", SKU: "canonical", IsPublished: true}
	require.NoError(t, db.Create(&p).Error)
	s := NewService(db, faultBackend{search: func(context.Context, Filters) (Result, error) { return Result{}, ErrIndexUnavailable }}, nil)
	result, err = s.Search(context.Background(), Filters{Page: page, Limit: 100})
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)
	require.Empty(t, result.Products)
}

func TestNormalizedProjectionFailureRollsBackDocument(t *testing.T) {
	db := searchTestDB(t)
	b := NewDatabaseBackend(db)
	p := models.Product{BaseModel: models.BaseModel{ID: 99}, Name: "Test", SKU: "test", IsPublished: true}
	doc, _, err := projectProduct(p, time.Now().UTC())
	require.NoError(t, err)
	require.NoError(t, db.Exec("DROP TABLE search_product_terms").Error)
	require.Error(t, b.Upsert(context.Background(), doc))
	var count int64
	require.NoError(t, db.Model(&models.SearchDocument{}).Where("entity_id = ?", 99).Count(&count).Error)
	require.Zero(t, count)
}

func TestCanonicalFallbackUnicodeFiltersAndPublishedVariantBounds(t *testing.T) {
	db := searchTestDB(t)
	ctx := context.Background()
	brand := models.Brand{Name: "Été", Slug: "été", IsActive: true}
	category := models.Category{Name: "Summer", Slug: "ÉTÉ---TRAIL", IsActive: true}
	attribute := models.ProductAttribute{Key: "Color", Slug: "CÖLOR", Filterable: true}
	require.NoError(t, db.Create(&brand).Error)
	require.NoError(t, db.Create(&category).Error)
	require.NoError(t, db.Create(&attribute).Error)
	p := models.Product{Name: "Target", SKU: "target", Price: models.MoneyFromFloat(999), Stock: 0, IsPublished: true, BrandID: &brand.ID}
	require.NoError(t, db.Create(&p).Error)
	require.NoError(t, db.Model(&p).Association("Categories").Append(&category))
	value := "RÉD!!!"
	require.NoError(t, db.Create(&models.ProductAttributeValue{ProductID: p.ID, ProductAttributeID: attribute.ID, TextValue: &value}).Error)
	require.NoError(t, db.Create(&models.ProductVariant{ProductID: p.ID, SKU: "live", Price: models.MoneyFromFloat(25), Stock: 4, IsPublished: true}).Error)
	require.NoError(t, db.Table("product_variants").Create(map[string]any{"product_id": p.ID, "sku": "draft", "title": "Draft", "price": 1, "stock": 99, "is_published": false}).Error)
	s := NewService(db, faultBackend{search: func(context.Context, Filters) (Result, error) { return Result{}, ErrIndexUnavailable }}, nil)
	min, max := 20.0, 30.0
	f := Filters{BrandSlug: "ÉTÉ", CategorySlugs: []string{"été trail"}, Attributes: map[string]string{"cölor": "réd"}, StockAvailability: []bool{true}, MinPrice: &min, MaxPrice: &max, SortField: "price", Limit: 1}
	result, err := s.Search(ctx, f)
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)
	require.Len(t, result.Products, 1)
	require.Equal(t, p.ID, result.Products[0].ID)
	require.Len(t, result.Products[0].Variants, 1)
	f.MinPrice = nil
	f.MaxPrice = nil
	f.PriceRanges = []PriceRange{{Max: func() *float64 { v := 2.0; return &v }()}}
	result, err = s.Search(ctx, f)
	require.NoError(t, err)
	require.Zero(t, result.Total)
}

func TestSQLFacetCountsPreserveMultipleValuesAndNormalizedDuplicates(t *testing.T) {
	db := searchTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for id := uint(1); id <= 3; id++ {
		product := models.Product{BaseModel: models.BaseModel{ID: id, CreatedAt: now, UpdatedAt: now}, SKU: fmt.Sprint(id), Name: "Trail Shoe", Price: 1000, IsPublished: true,
			Categories: []models.Category{{Name: "Trail", Slug: "trail", IsActive: true}, {Name: "Trail duplicate", Slug: "TRAIL!", IsActive: true}, {Name: "Outdoor", Slug: "outdoor", IsActive: true}}}
		for _, value := range []string{"red", "RED!", "blue"} {
			product.AttributeValues = append(product.AttributeValues, models.ProductAttributeValue{TextValue: &value, ProductAttribute: &models.ProductAttribute{Slug: "color", Key: "Color", Filterable: true}})
		}
		doc, include, err := projectProduct(product, now)
		require.NoError(t, err)
		require.True(t, include)
		require.NoError(t, NewDatabaseBackend(db).Upsert(ctx, doc))
	}
	b := NewDatabaseBackend(db).(*databaseBackend)
	all, err := b.loadIndexedProductsUncached(ctx, nil)
	require.NoError(t, err)
	for _, filters := range []Filters{{Query: "trail", Limit: 20}, {Query: "trail", CategorySlugs: []string{"trail"}, AttributeValues: map[string][]string{"color": {"red"}}, Limit: 20}} {
		normalized, err := normalizeFilters(filters)
		require.NoError(t, err)
		result, err := b.Search(ctx, filters)
		require.NoError(t, err)
		require.Equal(t, int64(3), result.Total)
		require.Equal(t, buildFacets(all, buildQueryPlan("trail", nil), false, normalized), result.Facets)
		require.Equal(t, int64(3), facetValueByName(t, facetByName(t, result.Facets, "category"), "trail").Count)
		require.Equal(t, int64(3), facetValueByName(t, facetByName(t, result.Facets, "attribute:color"), "red").Count)
		require.Equal(t, int64(3), facetValueByName(t, facetByName(t, result.Facets, "attribute:color"), "blue").Count)
	}
}
