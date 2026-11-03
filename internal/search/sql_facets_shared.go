package search

import (
	"context"
	"sort"

	"ecommerce/models"
)

func sharedFacetContext(s sqlSelection) bool {
	f := s.filters
	return len(f.brands)+len(f.categories)+len(f.stock)+len(f.attributes)+len(f.priceRanges) == 0 && f.minimum == nil && f.maximum == nil
}

func (b *databaseBackend) sqlFacets(ctx context.Context, s sqlSelection) ([]Facet, error) {
	if !sharedFacetContext(s) {
		return b.sqlFacetsIndividually(ctx, s)
	}
	facets, _, err := b.sqlFacetsShared(ctx, s)
	return facets, err
}

func (b *databaseBackend) sqlFacetsShared(ctx context.Context, s sqlSelection) ([]Facet, int64, error) {
	// Without selected filters every disjunctive context is identical. Compute
	// eligibility once in the database and share it across exact aggregates.
	// No document payload or candidate set is materialized in application memory.
	// Category and attribute value primary keys guarantee one row per product
	// in each value group. Only attribute coverage must deduplicate a product
	// which has several values for the same attribute.
	var rows []struct {
		Kind, Value, Label, Name string
		Count                    int64
		Minimum, Maximum         *models.Money
	}
	eligible := b.selection(ctx, s, "").Select("d.entity_id, d.brand_slug, d.min_price, d.max_price, d.available, r.brand_label")
	statement := `WITH eligible AS MATERIALIZED (?)
SELECT 'category' AS kind, c.slug AS value, MIN(c.label) AS label, '' AS name, COUNT(*) AS count, CAST(NULL AS BIGINT) AS minimum, CAST(NULL AS BIGINT) AS maximum FROM eligible e JOIN search_product_categories c ON c.product_id = e.entity_id GROUP BY c.slug
UNION ALL SELECT 'brand', e.brand_slug, MIN(CASE WHEN e.brand_label = '' THEN e.brand_slug ELSE e.brand_label END), '', COUNT(*), CAST(NULL AS BIGINT), CAST(NULL AS BIGINT) FROM eligible e WHERE e.brand_slug <> '' GROUP BY e.brand_slug
UNION ALL SELECT 'stock', CASE WHEN e.available THEN 'true' ELSE 'false' END, '', '', COUNT(*), CAST(NULL AS BIGINT), CAST(NULL AS BIGINT) FROM eligible e GROUP BY e.available
UNION ALL SELECT 'price', '', '', '', 0, MIN(e.min_price), MAX(e.max_price) FROM eligible e
UNION ALL SELECT 'total', '', '', '', COUNT(*), CAST(NULL AS BIGINT), CAST(NULL AS BIGINT) FROM eligible e
UNION ALL SELECT 'coverage', a.slug, '', '', COUNT(*), CAST(NULL AS BIGINT), CAST(NULL AS BIGINT) FROM (SELECT a.product_id, a.slug FROM eligible e JOIN search_product_attributes a ON a.product_id = e.entity_id GROUP BY a.product_id, a.slug) a GROUP BY a.slug
UNION ALL SELECT 'attribute:' || a.slug, a.value, MIN(a.label), MIN(a.name), COUNT(*), CAST(NULL AS BIGINT), CAST(NULL AS BIGINT) FROM eligible e JOIN search_product_attributes a ON a.product_id = e.entity_id GROUP BY a.slug, a.value`
	if err := b.db.WithContext(ctx).Raw(statement, eligible).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	groups := map[string][]sqlFacetRow{}
	counts := map[string]int64{}
	var total int64
	var minimum, maximum *models.Money
	ranks := []attributeFacetRank{}
	for _, row := range rows {
		switch row.Kind {
		case "total":
			total = row.Count
		case "price":
			minimum, maximum = row.Minimum, row.Maximum
		case "stock":
			counts[row.Value] = row.Count
		case "coverage":
			ranks = append(ranks, attributeFacetRank{slug: row.Value, coverage: int(row.Count)})
		default:
			groups[row.Kind] = append(groups[row.Kind], sqlFacetRow{Value: row.Value, Label: row.Label, Name: row.Name, Count: row.Count})
		}
	}
	buckets := []PriceRange{}
	if minimum != nil {
		doc := indexedProduct{document: &models.SearchDocument{MinPrice: *minimum, MaxPrice: *maximum}}
		buckets = dynamicPriceRanges([]*indexedProduct{&doc})
	}
	price, err := b.sqlPriceFacet(ctx, s, buckets)
	if err != nil {
		return nil, 0, err
	}
	stock := Facet{Name: "stock", Label: "Availability", Type: "boolean", Values: []FacetValue{{Value: "true", Label: "In stock", Count: counts["true"], Disabled: counts["true"] == 0}, {Value: "false", Label: "Out of stock", Count: counts["false"], Disabled: counts["false"] == 0}}}
	result := []Facet{termFacet("category", "Category", "terms", groups["category"], nil), termFacet("brand", "Brand", "terms", groups["brand"], nil), price, stock}
	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].coverage != ranks[j].coverage {
			return ranks[i].coverage > ranks[j].coverage
		}
		return ranks[i].slug < ranks[j].slug
	})
	for _, rank := range ranks[:min(maxAttributeFacets, len(ranks))] {
		name, label := "attribute:"+rank.slug, rank.slug
		for _, row := range groups[name] {
			if row.Name != "" && (label == rank.slug || row.Name < label) {
				label = row.Name
			}
		}
		result = append(result, termFacet(name, label, "attribute", groups[name], nil))
	}
	return result, total, nil
}
