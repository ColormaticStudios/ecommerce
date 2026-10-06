package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"ecommerce/models"
)

const maxAttributeFacets = 10

type indexedProduct struct {
	ranking        RankingExplanation
	document       models.SearchDocument
	product        models.Product
	categories     map[string]string
	attributes     map[string]map[string]string
	attributeNames map[string]string
}

type normalizedFilters struct {
	brands      map[string]struct{}
	categories  map[string]struct{}
	stock       map[bool]struct{}
	attributes  map[string]map[string]struct{}
	priceRanges []PriceRange
	minimum     *models.Money
	maximum     *models.Money
}

type synonymRule struct {
	id           uint
	source       string
	alternatives []string
}

type queryGroup struct {
	original     string
	alternatives []string
}

type queryPlan struct {
	groups   []queryGroup
	rewrites []AppliedRewrite
}

type vocabularyEntry struct {
	term      string
	frequency int
}

func (b *databaseBackend) searchProducts(ctx context.Context, filters Filters) (Result, error) {
	page, limit := filters.Page, filters.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	normalizedFilters, err := normalizeFilters(filters)
	if err != nil {
		return Result{}, err
	}
	rules, err := b.loadSynonymRules(ctx)
	if err != nil {
		return Result{}, err
	}
	profile, err := b.loadTypoToleranceProfile(ctx)
	if err != nil {
		return Result{}, err
	}

	normalizedQuery := NormalizeQuery(filters.Query)
	plan := buildQueryPlan(normalizedQuery, rules)
	documents, err := b.loadIndexedProducts(ctx, &plan)
	if err != nil {
		return Result{}, err
	}
	matched := filterIndexedProducts(documents, plan, false, normalizedFilters, "")
	resultPlan := plan
	rewrites := append([]AppliedRewrite(nil), plan.rewrites...)
	didYouMean := ""
	relaxed := false

	if normalizedQuery != "" && len(matched) == 0 {
		// Correction and relaxed recovery need the complete vocabulary. Successful
		// searches retain only SQL prefiltered candidates for facet aggregation.
		documents, err = b.loadIndexedProducts(ctx, nil)
		if err != nil {
			return Result{}, err
		}
		correctedQuery, typoRewrites := correctQuery(normalizedQuery, buildVocabulary(documents), profile)
		if len(typoRewrites) > 0 {
			correctedPlan := buildQueryPlan(correctedQuery, rules)
			correctedMatches := filterIndexedProducts(documents, correctedPlan, false, normalizedFilters, "")
			if len(correctedMatches) > 0 {
				didYouMean = correctedQuery
				if !profile.StrictMode {
					matched = correctedMatches
					resultPlan = correctedPlan
					rewrites = append(append([]AppliedRewrite(nil), typoRewrites...), correctedPlan.rewrites...)
				}
			}
		}
		if !profile.StrictMode && len(matched) == 0 && len(plan.groups) > 1 {
			relaxedMatches := filterIndexedProducts(documents, plan, true, normalizedFilters, "")
			if len(relaxedMatches) > 0 {
				matched = relaxedMatches
				resultPlan = plan
				relaxed = true
			}
		}
	}

	facets := buildFacets(documents, resultPlan, relaxed, normalizedFilters)
	rankingProfile, err := b.loadRankingProfile(ctx, filters.RankingProfile)
	if err != nil {
		return Result{}, err
	}
	field := filters.SortField
	if field == "" {
		if normalizedQuery != "" {
			field = "relevance"
		} else {
			field = "created_at"
		}
	}
	if field == "relevance" || filters.Explain {
		if err := b.rankProducts(ctx, matched, resultPlan, NormalizeQuery(strings.Join(originalQueryTerms(resultPlan), " ")), rankingProfile, filters.SortOrder); err != nil {
			return Result{}, err
		}
	}
	if field != "relevance" {
		sortIndexedProducts(matched, field, filters.SortOrder)
	}
	total := int64(len(matched))
	totalPages := len(matched) / limit
	if len(matched)%limit != 0 {
		totalPages++
	}
	start := (page - 1) * limit
	if start > len(matched) {
		start = len(matched)
	}
	end := start + limit
	if end > len(matched) {
		end = len(matched)
	}

	products := make([]models.Product, 0, end-start)
	explanations := make([]RankingExplanation, 0, end-start)
	var indexedAt *time.Time
	for _, document := range matched[start:end] {
		products = append(products, document.product)
		if filters.Explain {
			explanations = append(explanations, document.ranking)
		}
		if indexedAt == nil || document.document.IndexedAt.After(*indexedAt) {
			value := document.document.IndexedAt
			indexedAt = &value
		}
	}
	return Result{
		RankingProfile: rankingProfile.Name, RankingProfileVersion: rankingProfile.Version, Explanations: explanations,
		Products: products, Facets: facets, Total: total, TotalPages: totalPages,
		NormalizedQuery: normalizedQuery, AppliedRewrites: rewrites, DidYouMean: didYouMean,
		Relaxed: relaxed, IndexedAt: indexedAt,
	}, nil
}

func (b *databaseBackend) loadIndexedProducts(ctx context.Context, plan *queryPlan) ([]indexedProduct, error) {
	var rows []models.SearchDocument
	query := b.db.WithContext(ctx).Where("entity_type = ? AND active = ?", ProductEntityType, true)
	if plan != nil {
		for _, group := range plan.groups {
			clauses := make([]string, 0, len(group.alternatives))
			arguments := make([]any, 0, len(group.alternatives))
			for _, alternative := range group.alternatives {
				clauses = append(clauses, "searchable_text LIKE ?")
				arguments = append(arguments, "%"+escapeLike(alternative)+"%")
			}
			query = query.Where("("+strings.Join(clauses, " OR ")+")", arguments...)
		}
	}
	if err := query.Order("entity_id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]indexedProduct, 0, len(rows))
	for index, row := range rows {
		if index%256 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		var product models.Product
		if err := json.Unmarshal([]byte(row.PayloadJSON), &product); err != nil {
			return nil, fmt.Errorf("decode search document %d: %w", row.ID, err)
		}
		item := indexedProduct{
			document: row, product: product, categories: map[string]string{},
			attributes: map[string]map[string]string{}, attributeNames: map[string]string{},
		}
		categorySet := parseTokenSet(row.CategoryTokens)
		for _, category := range product.Categories {
			slug := NormalizeQuery(category.Slug)
			if _, ok := categorySet[slug]; ok {
				item.categories[slug] = category.Name
			}
		}
		for slug := range categorySet {
			if _, ok := item.categories[slug]; !ok {
				item.categories[slug] = slug
			}
		}
		for _, value := range product.AttributeValues {
			if value.ProductAttribute == nil || !value.ProductAttribute.Filterable {
				continue
			}
			slug := NormalizeQuery(value.ProductAttribute.Slug)
			raw := attributeValue(value)
			normalizedValue := NormalizeQuery(raw)
			if slug == "" || normalizedValue == "" {
				continue
			}
			if item.attributes[slug] == nil {
				item.attributes[slug] = map[string]string{}
			}
			item.attributes[slug][normalizedValue] = raw
			item.attributeNames[slug] = value.ProductAttribute.Key
		}
		result = append(result, item)
	}
	return result, nil
}

func (b *databaseBackend) loadSynonymRules(ctx context.Context) ([]synonymRule, error) {
	var sets []models.SearchSynonymSet
	if err := b.db.WithContext(ctx).Where("is_active = ?", true).Order("id asc").Find(&sets).Error; err != nil {
		return nil, err
	}
	rules := make([]synonymRule, 0, len(sets))
	for _, set := range sets {
		var storedTerms []string
		if err := json.Unmarshal([]byte(set.TermsJSON), &storedTerms); err != nil {
			return nil, fmt.Errorf("decode synonym set %d: %w", set.ID, err)
		}
		terms := make([]string, 0, len(storedTerms))
		seen := map[string]struct{}{}
		for _, storedTerm := range storedTerms {
			term := NormalizeQuery(storedTerm)
			if term == "" {
				continue
			}
			if _, exists := seen[term]; exists {
				continue
			}
			seen[term] = struct{}{}
			terms = append(terms, term)
		}
		if len(terms) < 2 {
			return nil, fmt.Errorf("synonym set %d must contain at least two unique terms", set.ID)
		}
		switch set.Direction {
		case "uni":
			rules = append(rules, synonymRule{id: set.ID, source: terms[0], alternatives: append([]string(nil), terms...)})
		case "bi":
			for _, source := range terms {
				rules = append(rules, synonymRule{id: set.ID, source: source, alternatives: append([]string(nil), terms...)})
			}
		default:
			return nil, fmt.Errorf("synonym set %d has unsupported direction %q", set.ID, set.Direction)
		}
	}
	return rules, nil
}

func (b *databaseBackend) loadTypoToleranceProfile(ctx context.Context) (models.SearchTypoToleranceProfile, error) {
	var profiles []models.SearchTypoToleranceProfile
	err := b.db.WithContext(ctx).Where("is_active = ?", true).Order("id asc").Limit(2).Find(&profiles).Error
	if err != nil {
		return models.SearchTypoToleranceProfile{}, err
	}
	if len(profiles) == 0 {
		return models.SearchTypoToleranceProfile{}, ErrActiveTypoProfileRequired
	}
	if len(profiles) > 1 {
		return models.SearchTypoToleranceProfile{}, errors.New("multiple active search typo tolerance profiles")
	}
	profile := profiles[0]
	if profile.MinimumTokenLength < 1 || profile.OneEditMinimumLength < profile.MinimumTokenLength || profile.TwoEditMinimumLength < profile.OneEditMinimumLength {
		return models.SearchTypoToleranceProfile{}, fmt.Errorf("invalid active search typo tolerance profile %d", profile.ID)
	}
	return profile, nil
}

func normalizeFilters(filters Filters) (normalizedFilters, error) {
	result := normalizedFilters{
		brands: map[string]struct{}{}, categories: map[string]struct{}{}, stock: map[bool]struct{}{},
		attributes: map[string]map[string]struct{}{},
	}
	brands := append([]string(nil), filters.BrandSlugs...)
	brands = append(brands, filters.BrandSlug)
	for _, value := range brands {
		if normalized := NormalizeQuery(value); normalized != "" {
			result.brands[normalized] = struct{}{}
		}
	}
	for _, value := range filters.CategorySlugs {
		if normalized := NormalizeQuery(value); normalized != "" {
			result.categories[normalized] = struct{}{}
		}
	}
	for _, value := range filters.StockAvailability {
		result.stock[value] = struct{}{}
	}
	if filters.HasVariantStock != nil {
		result.stock[*filters.HasVariantStock] = struct{}{}
	}
	for key, value := range filters.Attributes {
		addAttributeFilter(result.attributes, key, value)
	}
	for key, values := range filters.AttributeValues {
		for _, value := range values {
			addAttributeFilter(result.attributes, key, value)
		}
	}
	if len(filters.PriceRanges) > 0 {
		for _, priceRange := range filters.PriceRanges {
			if priceRange.Min == nil && priceRange.Max == nil {
				return normalizedFilters{}, errors.New("search price range requires at least one bound")
			}
			if priceRange.Min != nil && priceRange.Max != nil && *priceRange.Min > *priceRange.Max {
				return normalizedFilters{}, errors.New("search price range minimum cannot exceed maximum")
			}
			if (priceRange.Min != nil && (*priceRange.Min < 0 || math.IsNaN(*priceRange.Min) || math.IsInf(*priceRange.Min, 0))) ||
				(priceRange.Max != nil && (*priceRange.Max < 0 || math.IsNaN(*priceRange.Max) || math.IsInf(*priceRange.Max, 0))) {
				return normalizedFilters{}, errors.New("search price range bounds must be finite and non-negative")
			}
			result.priceRanges = append(result.priceRanges, clonePriceRange(priceRange))
		}
	} else {
		if filters.MinPrice != nil {
			value := models.MoneyFromFloat(*filters.MinPrice)
			result.minimum = &value
		}
		if filters.MaxPrice != nil {
			value := models.MoneyFromFloat(*filters.MaxPrice)
			result.maximum = &value
		}
	}
	return result, nil
}

func addAttributeFilter(attributes map[string]map[string]struct{}, key, value string) {
	slug, normalizedValue := NormalizeQuery(key), NormalizeQuery(value)
	if slug == "" || normalizedValue == "" {
		return
	}
	if attributes[slug] == nil {
		attributes[slug] = map[string]struct{}{}
	}
	attributes[slug][normalizedValue] = struct{}{}
}

func clonePriceRange(value PriceRange) PriceRange {
	result := PriceRange{}
	if value.Min != nil {
		minimum := *value.Min
		result.Min = &minimum
	}
	if value.Max != nil {
		maximum := *value.Max
		result.Max = &maximum
	}
	return result
}

func buildQueryPlan(query string, rules []synonymRule) queryPlan {
	tokens := strings.Fields(query)
	plan := queryPlan{groups: make([]queryGroup, 0, len(tokens)), rewrites: []AppliedRewrite{}}
	for index := 0; index < len(tokens); {
		var source string
		var expansions []string
		selectedLength := 0
		for ruleIndex := range rules {
			rule := rules[ruleIndex]
			ruleTokens := strings.Fields(rule.source)
			if len(ruleTokens) < selectedLength || index+len(ruleTokens) > len(tokens) {
				continue
			}
			if strings.Join(tokens[index:index+len(ruleTokens)], " ") == rule.source {
				if len(ruleTokens) > selectedLength {
					expansions = nil
				}
				source = rule.source
				expansions = append(expansions, rule.alternatives...)
				selectedLength = len(ruleTokens)
			}
		}
		if selectedLength == 0 {
			plan.groups = append(plan.groups, queryGroup{original: tokens[index], alternatives: []string{tokens[index]}})
			index++
			continue
		}
		alternatives := uniqueStrings(append([]string{source}, expansions...))
		plan.groups = append(plan.groups, queryGroup{original: source, alternatives: alternatives})
		for _, alternative := range alternatives {
			if alternative != source {
				plan.rewrites = append(plan.rewrites, AppliedRewrite{Kind: "synonym", Original: source, Replacement: alternative})
			}
		}
		index += selectedLength
	}
	return plan
}

func uniqueStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func filterIndexedProducts(documents []indexedProduct, plan queryPlan, relaxed bool, filters normalizedFilters, excludedFacet string) []indexedProduct {
	result := make([]indexedProduct, 0, len(documents))
	for _, document := range documents {
		if !matchesQuery(document.document.SearchableText, plan, relaxed) || !matchesFilters(document, filters, excludedFacet) {
			continue
		}
		result = append(result, document)
	}
	return result
}

func matchesQuery(searchableText string, plan queryPlan, relaxed bool) bool {
	if len(plan.groups) == 0 {
		return true
	}
	matchedGroups := 0
	for _, group := range plan.groups {
		matched := false
		for _, alternative := range group.alternatives {
			if containsPhrase(searchableText, alternative) {
				matched = true
				break
			}
		}
		if matched {
			matchedGroups++
		} else if !relaxed {
			return false
		}
	}
	if relaxed {
		return matchedGroups > 0
	}
	return true
}

func containsPhrase(text, phrase string) bool {
	return strings.Contains(" "+text+" ", " "+phrase+" ")
}

func matchesFilters(document indexedProduct, filters normalizedFilters, excludedFacet string) bool {
	if excludedFacet != "brand" && len(filters.brands) > 0 {
		if _, ok := filters.brands[document.document.BrandSlug]; !ok {
			return false
		}
	}
	if excludedFacet != "category" && len(filters.categories) > 0 && !containsAnyKey(document.categories, filters.categories) {
		return false
	}
	if excludedFacet != "stock" && len(filters.stock) > 0 {
		if _, ok := filters.stock[document.document.Available]; !ok {
			return false
		}
	}
	if excludedFacet != "price" && !matchesPriceFilter(document.document, filters) {
		return false
	}
	for slug, selected := range filters.attributes {
		if excludedFacet == "attribute:"+slug {
			continue
		}
		values := document.attributes[slug]
		if len(values) == 0 || !containsAnyKey(values, selected) {
			return false
		}
	}
	return true
}

func containsAnyKey[T any](values map[string]T, selected map[string]struct{}) bool {
	for value := range selected {
		if _, ok := values[value]; ok {
			return true
		}
	}
	return false
}

func matchesPriceFilter(document models.SearchDocument, filters normalizedFilters) bool {
	if len(filters.priceRanges) > 0 {
		for _, priceRange := range filters.priceRanges {
			if overlapsPriceRange(document, priceRange) {
				return true
			}
		}
		return false
	}
	if filters.minimum != nil && document.MaxPrice < *filters.minimum {
		return false
	}
	return filters.maximum == nil || document.MinPrice <= *filters.maximum
}

func overlapsPriceRange(document models.SearchDocument, priceRange PriceRange) bool {
	if priceRange.Min != nil && document.MaxPrice < models.MoneyFromFloat(*priceRange.Min) {
		return false
	}
	return priceRange.Max == nil || document.MinPrice <= models.MoneyFromFloat(*priceRange.Max)
}

func buildVocabulary(documents []indexedProduct) []vocabularyEntry {
	frequencies := map[string]int{}
	for _, document := range documents {
		seen := map[string]struct{}{}
		for _, term := range strings.Fields(document.document.SearchableText) {
			if _, ok := seen[term]; ok {
				continue
			}
			seen[term] = struct{}{}
			frequencies[term]++
		}
	}
	result := make([]vocabularyEntry, 0, len(frequencies))
	for term, frequency := range frequencies {
		result = append(result, vocabularyEntry{term: term, frequency: frequency})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].term < result[j].term })
	return result
}

func correctQuery(query string, vocabulary []vocabularyEntry, profile models.SearchTypoToleranceProfile) (string, []AppliedRewrite) {
	tokens := strings.Fields(query)
	rewrites := make([]AppliedRewrite, 0)
	known := make(map[string]struct{}, len(vocabulary))
	for _, entry := range vocabulary {
		known[entry.term] = struct{}{}
	}
	for index, token := range tokens {
		if _, ok := known[token]; ok || containsDigit(token) {
			continue
		}
		length := utf8.RuneCountInString(token)
		maximumDistance := 0
		if length >= profile.MinimumTokenLength && length >= profile.OneEditMinimumLength {
			maximumDistance = 1
		}
		if length >= profile.TwoEditMinimumLength {
			maximumDistance = 2
		}
		if maximumDistance == 0 {
			continue
		}
		candidate, found := closestTerm(token, vocabulary, maximumDistance)
		if !found {
			continue
		}
		tokens[index] = candidate
		rewrites = append(rewrites, AppliedRewrite{Kind: "typo", Original: token, Replacement: candidate})
	}
	return strings.Join(tokens, " "), rewrites
}

func containsDigit(value string) bool {
	for _, character := range value {
		if unicode.IsDigit(character) {
			return true
		}
	}
	return false
}

func closestTerm(term string, vocabulary []vocabularyEntry, maximumDistance int) (string, bool) {
	bestTerm, bestDistance, bestFrequency := "", maximumDistance+1, -1
	termLength := utf8.RuneCountInString(term)
	for _, candidate := range vocabulary {
		if difference(termLength, utf8.RuneCountInString(candidate.term)) > maximumDistance {
			continue
		}
		distance := damerauLevenshtein(term, candidate.term)
		if distance > maximumDistance {
			continue
		}
		if distance < bestDistance || (distance == bestDistance && candidate.frequency > bestFrequency) ||
			(distance == bestDistance && candidate.frequency == bestFrequency && candidate.term < bestTerm) {
			bestTerm, bestDistance, bestFrequency = candidate.term, distance, candidate.frequency
		}
	}
	return bestTerm, bestTerm != ""
}

func difference(left, right int) int {
	if left < right {
		return right - left
	}
	return left - right
}

func damerauLevenshtein(left, right string) int {
	a, b := []rune(left), []rune(right)
	distance := make([][]int, len(a)+1)
	for i := range distance {
		distance[i] = make([]int, len(b)+1)
		distance[i][0] = i
	}
	for j := range distance[0] {
		distance[0][j] = j
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			distance[i][j] = minInt(distance[i-1][j]+1, distance[i][j-1]+1, distance[i-1][j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				distance[i][j] = minInt(distance[i][j], distance[i-2][j-2]+1)
			}
		}
	}
	return distance[len(a)][len(b)]
}

func minInt(values ...int) int {
	result := values[0]
	for _, value := range values[1:] {
		if value < result {
			result = value
		}
	}
	return result
}

func buildFacets(documents []indexedProduct, plan queryPlan, relaxed bool, filters normalizedFilters) []Facet {
	queryMatches := make([]indexedProduct, 0, len(documents))
	for _, document := range documents {
		if matchesQuery(document.document.SearchableText, plan, relaxed) {
			queryMatches = append(queryMatches, document)
		}
	}
	result := []Facet{
		buildTermsFacet("category", "Category", "terms", queryMatches, filters, func(document indexedProduct) map[string]string { return document.categories }),
		buildBrandFacet(queryMatches, filters),
		buildPriceFacet(queryMatches, filters),
		buildStockFacet(queryMatches, filters),
	}
	result = append(result, buildAttributeFacets(queryMatches, filters)...)
	return result
}

func buildTermsFacet(name, label, facetType string, documents []indexedProduct, filters normalizedFilters, values func(indexedProduct) map[string]string) Facet {
	contextDocuments := filterFacetContext(documents, filters, name)
	counts := map[string]int64{}
	labels := map[string]string{}
	for _, document := range contextDocuments {
		for value, valueLabel := range values(document) {
			counts[value]++
			if labels[value] == "" || valueLabel < labels[value] {
				labels[value] = valueLabel
			}
		}
	}
	selected := filters.categories
	if name == "brand" {
		selected = filters.brands
	}
	return Facet{Name: name, Label: label, Type: facetType, Values: facetValues(counts, labels, selected)}
}

func buildBrandFacet(documents []indexedProduct, filters normalizedFilters) Facet {
	return buildTermsFacet("brand", "Brand", "terms", documents, filters, func(document indexedProduct) map[string]string {
		if document.document.BrandSlug == "" {
			return nil
		}
		label := document.document.BrandSlug
		if document.product.Brand != nil && NormalizeQuery(document.product.Brand.Slug) == document.document.BrandSlug {
			label = document.product.Brand.Name
		}
		return map[string]string{document.document.BrandSlug: label}
	})
}

func facetValues(counts map[string]int64, labels map[string]string, selected map[string]struct{}) []FacetValue {
	for value := range selected {
		if _, ok := counts[value]; !ok {
			counts[value] = 0
		}
		if labels[value] == "" {
			labels[value] = value
		}
	}
	values := make([]FacetValue, 0, len(counts))
	for value, count := range counts {
		_, isSelected := selected[value]
		values = append(values, FacetValue{Value: value, Label: labels[value], Count: count, Selected: isSelected, Disabled: count == 0})
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].Count != values[j].Count {
			return values[i].Count > values[j].Count
		}
		return values[i].Value < values[j].Value
	})
	return values
}

func filterFacetContext(documents []indexedProduct, filters normalizedFilters, excludedFacet string) []indexedProduct {
	result := make([]indexedProduct, 0, len(documents))
	for _, document := range documents {
		if matchesFilters(document, filters, excludedFacet) {
			result = append(result, document)
		}
	}
	return result
}

func buildStockFacet(documents []indexedProduct, filters normalizedFilters) Facet {
	contextDocuments := filterFacetContext(documents, filters, "stock")
	counts := map[bool]int64{}
	for _, document := range contextDocuments {
		counts[document.document.Available]++
	}
	values := make([]FacetValue, 0, 2)
	for _, available := range []bool{true, false} {
		_, selected := filters.stock[available]
		count := counts[available]
		values = append(values, FacetValue{
			Value: strconv.FormatBool(available), Label: map[bool]string{true: "In stock", false: "Out of stock"}[available],
			Count: count, Selected: selected, Disabled: count == 0,
		})
	}
	return Facet{Name: "stock", Label: "Availability", Type: "boolean", Values: values}
}

func buildPriceFacet(documents []indexedProduct, filters normalizedFilters) Facet {
	contextDocuments := filterFacetContext(documents, filters, "price")
	buckets := dynamicPriceRanges(contextDocuments)
	for _, selected := range selectedPriceRanges(filters) {
		if !containsPriceRange(buckets, selected) {
			buckets = append(buckets, selected)
		}
	}
	sort.Slice(buckets, func(i, j int) bool { return comparePriceRanges(buckets[i], buckets[j]) < 0 })
	values := make([]FacetValue, 0, len(buckets))
	for _, bucket := range buckets {
		count := int64(0)
		for _, document := range contextDocuments {
			if overlapsPriceRange(document.document, bucket) {
				count++
			}
		}
		selected := containsPriceRange(selectedPriceRanges(filters), bucket)
		minimum, maximum := cloneFloatPointer(bucket.Min), cloneFloatPointer(bucket.Max)
		values = append(values, FacetValue{
			Value: priceRangeValue(bucket), Label: priceRangeLabel(bucket), Count: count,
			Selected: selected, Disabled: count == 0, MinPrice: minimum, MaxPrice: maximum,
		})
	}
	return Facet{Name: "price", Label: "Price", Type: "range", Values: values}
}

func selectedPriceRanges(filters normalizedFilters) []PriceRange {
	if len(filters.priceRanges) > 0 {
		result := make([]PriceRange, 0, len(filters.priceRanges))
		for _, value := range filters.priceRanges {
			result = append(result, clonePriceRange(value))
		}
		return result
	}
	if filters.minimum == nil && filters.maximum == nil {
		return nil
	}
	result := PriceRange{}
	if filters.minimum != nil {
		value := filters.minimum.Float64()
		result.Min = &value
	}
	if filters.maximum != nil {
		value := filters.maximum.Float64()
		result.Max = &value
	}
	return []PriceRange{result}
}

func dynamicPriceRanges(documents []indexedProduct) []PriceRange {
	if len(documents) == 0 {
		return nil
	}
	minimum, maximum := documents[0].document.MinPrice, documents[0].document.MaxPrice
	for _, document := range documents[1:] {
		if document.document.MinPrice < minimum {
			minimum = document.document.MinPrice
		}
		if document.document.MaxPrice > maximum {
			maximum = document.document.MaxPrice
		}
	}
	if minimum == maximum {
		value := minimum.Float64()
		return []PriceRange{{Min: &value, Max: cloneFloatPointer(&value)}}
	}
	span := int64(maximum-minimum) + 1
	width := niceBucketWidth(int64(math.Ceil(float64(span) / 5)))
	lower := (int64(minimum) / width) * width
	for lower+5*width-1 < int64(maximum) {
		width = niceBucketWidth(width + 1)
		lower = (int64(minimum) / width) * width
	}
	ranges := make([]PriceRange, 0, 5)
	for start := lower; start <= int64(maximum) && len(ranges) < 5; start += width {
		end := start + width - 1
		minimumValue, maximumValue := models.Money(start).Float64(), models.Money(end).Float64()
		ranges = append(ranges, PriceRange{Min: &minimumValue, Max: &maximumValue})
	}
	return ranges
}

func niceBucketWidth(value int64) int64 {
	if value <= 1 {
		return 1
	}
	power := int64(math.Pow10(int(math.Floor(math.Log10(float64(value))))))
	fraction := float64(value) / float64(power)
	switch {
	case fraction <= 1:
		return power
	case fraction <= 2:
		return 2 * power
	case fraction <= 5:
		return 5 * power
	default:
		return 10 * power
	}
}

func containsPriceRange(values []PriceRange, target PriceRange) bool {
	for _, value := range values {
		if comparePriceRanges(value, target) == 0 {
			return true
		}
	}
	return false
}

func comparePriceRanges(left, right PriceRange) int {
	leftMin, rightMin := moneyBound(left.Min, math.MinInt64), moneyBound(right.Min, math.MinInt64)
	if leftMin < rightMin {
		return -1
	}
	if leftMin > rightMin {
		return 1
	}
	leftMax, rightMax := moneyBound(left.Max, math.MaxInt64), moneyBound(right.Max, math.MaxInt64)
	if leftMax < rightMax {
		return -1
	}
	if leftMax > rightMax {
		return 1
	}
	return 0
}

func moneyBound(value *float64, fallback int64) int64 {
	if value == nil {
		return fallback
	}
	return int64(models.MoneyFromFloat(*value))
}

func priceRangeValue(value PriceRange) string {
	return formatPriceBound(value.Min) + ":" + formatPriceBound(value.Max)
}

func priceRangeLabel(value PriceRange) string {
	switch {
	case value.Min == nil:
		return "Up to " + formatPriceBound(value.Max)
	case value.Max == nil:
		return formatPriceBound(value.Min) + " and up"
	default:
		return formatPriceBound(value.Min) + "–" + formatPriceBound(value.Max)
	}
}

func formatPriceBound(value *float64) string {
	if value == nil {
		return ""
	}
	return models.MoneyFromFloat(*value).String()
}

func cloneFloatPointer(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

type attributeFacetRank struct {
	slug     string
	coverage int
}

func buildAttributeFacets(documents []indexedProduct, filters normalizedFilters) []Facet {
	candidates := map[string]struct{}{}
	for _, document := range documents {
		for slug := range document.attributes {
			candidates[slug] = struct{}{}
		}
	}
	for slug := range filters.attributes {
		candidates[slug] = struct{}{}
	}
	ranks := make([]attributeFacetRank, 0, len(candidates))
	for slug := range candidates {
		coverage := 0
		for _, document := range filterFacetContext(documents, filters, "attribute:"+slug) {
			if len(document.attributes[slug]) > 0 {
				coverage++
			}
		}
		ranks = append(ranks, attributeFacetRank{slug: slug, coverage: coverage})
	}
	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].coverage != ranks[j].coverage {
			return ranks[i].coverage > ranks[j].coverage
		}
		return ranks[i].slug < ranks[j].slug
	})
	included := map[string]struct{}{}
	ordered := make([]attributeFacetRank, 0, maxAttributeFacets+len(filters.attributes))
	for _, rank := range ranks {
		if len(ordered) >= maxAttributeFacets {
			break
		}
		ordered = append(ordered, rank)
		included[rank.slug] = struct{}{}
	}
	selectedExtras := make([]string, 0)
	for slug := range filters.attributes {
		if _, ok := included[slug]; !ok {
			selectedExtras = append(selectedExtras, slug)
		}
	}
	sort.Strings(selectedExtras)
	for _, slug := range selectedExtras {
		ordered = append(ordered, attributeFacetRank{slug: slug})
	}

	result := make([]Facet, 0, len(ordered))
	for _, rank := range ordered {
		contextDocuments := filterFacetContext(documents, filters, "attribute:"+rank.slug)
		counts := map[string]int64{}
		labels := map[string]string{}
		facetLabel := rank.slug
		for _, document := range contextDocuments {
			if name := document.attributeNames[rank.slug]; name != "" && (facetLabel == rank.slug || name < facetLabel) {
				facetLabel = name
			}
			for value, label := range document.attributes[rank.slug] {
				counts[value]++
				if labels[value] == "" || label < labels[value] {
					labels[value] = label
				}
			}
		}
		result = append(result, Facet{
			Name: "attribute:" + rank.slug, Label: facetLabel, Type: "attribute",
			Values: facetValues(counts, labels, filters.attributes[rank.slug]),
		})
	}
	return result
}

func sortIndexedProducts(documents []indexedProduct, field, order string) {
	descending := !strings.EqualFold(order, "asc")
	sort.SliceStable(documents, func(i, j int) bool {
		comparison := compareIndexedProducts(documents[i], documents[j], field)
		if comparison == 0 {
			return documents[i].document.EntityID < documents[j].document.EntityID
		}
		if descending {
			return comparison > 0
		}
		return comparison < 0
	})
}

func compareIndexedProducts(left, right indexedProduct, field string) int {
	switch field {
	case "price":
		return compareOrdered(left.document.MinPrice, right.document.MinPrice)
	case "name":
		return strings.Compare(left.document.NormalizedName, right.document.NormalizedName)
	default:
		if left.document.SourceCreatedAt.Before(right.document.SourceCreatedAt) {
			return -1
		}
		if left.document.SourceCreatedAt.After(right.document.SourceCreatedAt) {
			return 1
		}
		return 0
	}
}

func compareOrdered[T ~int64](left, right T) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func parseTokenSet(value string) map[string]struct{} {
	result := map[string]struct{}{}
	for _, token := range strings.Split(value, "|") {
		if token != "" {
			result[token] = struct{}{}
		}
	}
	return result
}

func originalQueryTerms(plan queryPlan) []string {
	terms := make([]string, 0, len(plan.groups))
	for _, group := range plan.groups {
		terms = append(terms, group.original)
	}
	return terms
}
