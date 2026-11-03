package search

import (
	"context"
	"ecommerce/models"
	"sort"
	"strconv"
	"strings"
)

type sqlFacetRow struct {
	Value, Label, Name string
	Count              int64
}

func termFacet(name, label, kind string, rows []sqlFacetRow, selected map[string]struct{}) Facet {
	counts, labels := map[string]int64{}, map[string]string{}
	for _, r := range rows {
		counts[r.Value] = r.Count
		labels[r.Value] = r.Label
	}
	return Facet{Name: name, Label: label, Type: kind, Values: facetValues(counts, labels, selected)}
}
func (b *databaseBackend) sqlFacetsIndividually(ctx context.Context, s sqlSelection) ([]Facet, error) {
	result := []Facet{}
	var rows []sqlFacetRow
	if err := b.selection(ctx, s, "category").Joins("JOIN search_product_categories c ON c.product_id = d.entity_id").Select("c.slug AS value, MIN(c.label) AS label, COUNT(*) AS count").Group("c.slug").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result = append(result, termFacet("category", "Category", "terms", rows, s.filters.categories))
	rows = nil
	if err := b.selection(ctx, s, "brand").Where("d.brand_slug <> ''").Select("d.brand_slug AS value, MIN(CASE WHEN r.brand_label = '' THEN d.brand_slug ELSE r.brand_label END) AS label, COUNT(*) AS count").Group("d.brand_slug").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result = append(result, termFacet("brand", "Brand", "terms", rows, s.filters.brands))
	var bounds struct{ Minimum, Maximum *models.Money }
	if err := b.selection(ctx, s, "price").Select("MIN(d.min_price) AS minimum, MAX(d.max_price) AS maximum").Scan(&bounds).Error; err != nil {
		return nil, err
	}
	buckets := []PriceRange{}
	if bounds.Minimum != nil {
		doc := indexedProduct{document: &models.SearchDocument{MinPrice: *bounds.Minimum, MaxPrice: *bounds.Maximum}}
		buckets = dynamicPriceRanges([]*indexedProduct{&doc})
	}
	for _, p := range selectedPriceRanges(s.filters) {
		if !containsPriceRange(buckets, p) {
			buckets = append(buckets, p)
		}
	}
	sort.Slice(buckets, func(i, j int) bool { return comparePriceRanges(buckets[i], buckets[j]) < 0 })
	price, err := b.sqlPriceFacet(ctx, s, buckets)
	if err != nil {
		return nil, err
	}
	result = append(result, price)
	var stock []struct {
		Available bool
		Count     int64
	}
	if err := b.selection(ctx, s, "stock").Select("d.available, COUNT(*) AS count").Group("d.available").Scan(&stock).Error; err != nil {
		return nil, err
	}
	counts := map[bool]int64{}
	for _, r := range stock {
		counts[r.Available] = r.Count
	}
	facet := Facet{Name: "stock", Label: "Availability", Type: "boolean", Values: []FacetValue{}}
	for _, available := range []bool{true, false} {
		_, selected := s.filters.stock[available]
		label := "Out of stock"
		if available {
			label = "In stock"
		}
		facet.Values = append(facet.Values, FacetValue{Value: strconv.FormatBool(available), Label: label, Count: counts[available], Selected: selected, Disabled: counts[available] == 0})
	}
	result = append(result, facet)
	// Candidate attributes come from the query context before shopper filters.
	candidate := s
	candidate.filters = normalizedFilters{}
	var slugs []string
	if err := b.selection(ctx, candidate, "").Joins("JOIN search_product_attributes a ON a.product_id = d.entity_id").Distinct("a.slug").Pluck("a.slug", &slugs).Error; err != nil {
		return nil, err
	}
	candidates := map[string]struct{}{}
	for _, slug := range slugs {
		candidates[slug] = struct{}{}
	}
	for slug := range s.filters.attributes {
		candidates[slug] = struct{}{}
	}
	// All unselected attributes share one filter context and one grouped query.
	var coverage []struct {
		Slug  string
		Count int64
	}
	if err := b.selection(ctx, s, "").Joins("JOIN search_product_attributes a ON a.product_id = d.entity_id").Select("a.slug, COUNT(DISTINCT d.entity_id) AS count").Group("a.slug").Scan(&coverage).Error; err != nil {
		return nil, err
	}
	coverageBySlug := map[string]int64{}
	for _, r := range coverage {
		coverageBySlug[r.Slug] = r.Count
	}
	for slug := range s.filters.attributes {
		var count int64
		if err := b.selection(ctx, s, "attribute:"+slug).Where("EXISTS (SELECT 1 FROM search_product_attributes a WHERE a.product_id = d.entity_id AND a.slug = ?)", slug).Count(&count).Error; err != nil {
			return nil, err
		}
		coverageBySlug[slug] = count
	}
	ranks := []attributeFacetRank{}
	for slug := range candidates {
		ranks = append(ranks, attributeFacetRank{slug: slug, coverage: int(coverageBySlug[slug])})
	}
	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].coverage != ranks[j].coverage {
			return ranks[i].coverage > ranks[j].coverage
		}
		return ranks[i].slug < ranks[j].slug
	})
	ordered := ranks[:min(maxAttributeFacets, len(ranks))]
	included := map[string]bool{}
	for _, r := range ordered {
		included[r.slug] = true
	}
	extras := []string{}
	for slug := range s.filters.attributes {
		if !included[slug] {
			extras = append(extras, slug)
		}
	}
	sort.Strings(extras)
	for _, slug := range extras {
		ordered = append(ordered, attributeFacetRank{slug: slug})
	}
	for _, r := range ordered {
		rows = nil
		if err := b.selection(ctx, s, "attribute:"+r.slug).Joins("JOIN search_product_attributes a ON a.product_id = d.entity_id AND a.slug = ?", r.slug).Select("a.value, MIN(a.label) AS label, MIN(a.name) AS name, COUNT(*) AS count").Group("a.value").Scan(&rows).Error; err != nil {
			return nil, err
		}
		label := r.slug
		for _, row := range rows {
			if row.Name != "" && (label == r.slug || row.Name < label) {
				label = row.Name
			}
		}
		result = append(result, termFacet("attribute:"+r.slug, label, "attribute", rows, s.filters.attributes[r.slug]))
	}
	return result, nil
}

func (b *databaseBackend) sqlPriceFacet(ctx context.Context, s sqlSelection, buckets []PriceRange) (Facet, error) {
	price := Facet{Name: "price", Label: "Price", Type: "range", Values: []FacetValue{}}
	expressions, arguments := []string{}, []any{}
	for _, p := range buckets {
		predicate := "1=1"
		if p.Min != nil {
			predicate += " AND d.max_price >= ?"
			arguments = append(arguments, models.MoneyFromFloat(*p.Min))
		}
		if p.Max != nil {
			predicate += " AND d.min_price <= ?"
			arguments = append(arguments, models.MoneyFromFloat(*p.Max))
		}
		expressions = append(expressions, "COALESCE(SUM(CASE WHEN "+predicate+" THEN 1 ELSE 0 END),0)")
	}
	priceCounts := make([]int64, len(buckets))
	if len(buckets) > 0 {
		cursor, err := b.selection(ctx, s, "price").Select(strings.Join(expressions, ", "), arguments...).Rows()
		if err != nil {
			return Facet{}, err
		}
		destinations := make([]any, len(buckets))
		for i := range priceCounts {
			destinations[i] = &priceCounts[i]
		}
		if cursor.Next() {
			err = cursor.Scan(destinations...)
		}
		if err == nil {
			err = cursor.Err()
		}
		cursor.Close()
		if err != nil {
			return Facet{}, err
		}
	}
	for i, p := range buckets {
		count := priceCounts[i]
		price.Values = append(price.Values, FacetValue{Value: priceRangeValue(p), Label: priceRangeLabel(p), Count: count, Selected: containsPriceRange(selectedPriceRanges(s.filters), p), Disabled: count == 0, MinPrice: cloneFloatPointer(p.Min), MaxPrice: cloneFloatPointer(p.Max)})
	}
	return price, nil
}
