package search

import (
	"context"
	"ecommerce/models"
	"fmt"
	"gorm.io/gorm"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type sqlSelection struct {
	plan             queryPlan
	relaxed          bool
	filters          normalizedFilters
	hidden, included []uint
}

func mapKeys[T comparable](m map[T]struct{}) []T {
	v := make([]T, 0, len(m))
	for k := range m {
		v = append(v, k)
	}
	return v
}
func (b *databaseBackend) selection(ctx context.Context, s sqlSelection, excluded string) *gorm.DB {
	q := b.db.WithContext(ctx).Table("search_documents d").Joins("JOIN search_product_ranks r ON r.product_id = d.entity_id").Where("d.entity_type = ? AND d.active = ?", ProductEntityType, true)
	if len(s.hidden) > 0 {
		q = q.Where("d.entity_id NOT IN ?", s.hidden)
	}
	clauses, args := []string{}, []any{}
	for _, g := range s.plan.groups {
		alternatives, values := []string{}, []any{}
		for _, a := range g.alternatives {
			// A unique posting is the term-first candidate gate; phrase verification
			// retains adjacency and boundaries for multiword synonym alternatives.
			words := strings.Fields(a)
			if len(words) == 0 {
				continue
			}
			predicate := "d.entity_id IN (SELECT t.product_id FROM search_product_terms t WHERE t.term = ?)"
			values = append(values, words[0])
			if len(words) > 1 {
				predicate += " AND r.searchable LIKE ?"
				values = append(values, "% "+a+" %")
			}
			alternatives = append(alternatives, "("+predicate+")")
		}
		if len(alternatives) > 0 {
			clauses = append(clauses, "("+strings.Join(alternatives, " OR ")+")")
			args = append(args, values...)
		}
	}
	if len(clauses) > 0 {
		op := " AND "
		if s.relaxed {
			op = " OR "
		}
		predicate := "(" + strings.Join(clauses, op) + ")"
		if len(s.included) > 0 {
			predicate = "(" + predicate + " OR d.entity_id IN ?)"
			args = append(args, s.included)
		}
		q = q.Where(predicate, args...)
	}
	f := s.filters
	if excluded != "brand" && len(f.brands) > 0 {
		q = q.Where("d.brand_slug IN ?", mapKeys(f.brands))
	}
	if excluded != "category" && len(f.categories) > 0 {
		q = q.Where("EXISTS (SELECT 1 FROM search_product_categories c WHERE c.product_id = d.entity_id AND c.slug IN ?)", mapKeys(f.categories))
	}
	if excluded != "stock" && len(f.stock) > 0 {
		q = q.Where("d.available IN ?", mapKeys(f.stock))
	}
	if excluded != "price" {
		if len(f.priceRanges) > 0 {
			parts, values := []string{}, []any{}
			for _, p := range f.priceRanges {
				part := "1=1"
				if p.Min != nil {
					part += " AND d.max_price >= ?"
					values = append(values, models.MoneyFromFloat(*p.Min))
				}
				if p.Max != nil {
					part += " AND d.min_price <= ?"
					values = append(values, models.MoneyFromFloat(*p.Max))
				}
				parts = append(parts, "("+part+")")
			}
			q = q.Where("("+strings.Join(parts, " OR ")+")", values...)
		} else {
			if f.minimum != nil {
				q = q.Where("d.max_price >= ?", *f.minimum)
			}
			if f.maximum != nil {
				q = q.Where("d.min_price <= ?", *f.maximum)
			}
		}
	}
	for slug, values := range f.attributes {
		if excluded != "attribute:"+slug {
			q = q.Where("EXISTS (SELECT 1 FROM search_product_attributes a WHERE a.product_id = d.entity_id AND a.slug = ? AND a.value IN ?)", slug, mapKeys(values))
		}
	}
	return q
}
func (b *databaseBackend) sqlCount(ctx context.Context, s sqlSelection) (int64, error) {
	var n int64
	err := b.selection(ctx, s, "").Count(&n).Error
	return n, err
}

// Planning needs only zero/nonzero before unfiltered shared facets compute the
// authoritative post-merchandising total. Selected disjunctive contexts retain
// their exact count here, because their final facets do not supply a total.
func (b *databaseBackend) sqlPlanningMatches(ctx context.Context, s sqlSelection) (int64, error) {
	if !sharedFacetContext(s) {
		return b.sqlCount(ctx, s)
	}
	var ids []uint
	if err := b.selection(ctx, s, "").Limit(1).Pluck("d.entity_id", &ids).Error; err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}

// Vocabulary is read as grouped lightweight postings, never product JSON. The
// length restriction is exact for edit distance, not an approximate top-N cap.
func (b *databaseBackend) sqlVocabulary(ctx context.Context, query string, profile models.SearchTypoToleranceProfile) ([]vocabularyEntry, error) {
	lengths := map[int]struct{}{}
	distances := map[string]int{}
	predicates, arguments := []string{}, []any{}
	for _, token := range strings.Fields(query) {
		if containsDigit(token) {
			continue
		}
		n := utf8.RuneCountInString(token)
		distance := 0
		if n >= profile.MinimumTokenLength && n >= profile.OneEditMinimumLength {
			distance = 1
		}
		if n >= profile.TwoEditMinimumLength {
			distance = 2
		}
		distances[token] = distance
		for i := max(1, n-distance); i <= n+distance; i++ {
			lengths[i] = struct{}{}
		}
		if distance == 0 {
			predicates = append(predicates, "c.term = ?")
			arguments = append(arguments, token)
			continue
		}
		if n <= distance {
			predicates = append(predicates, "1=1")
			continue
		}
		// An edit/transposition changes at most distance original characters.
		// Since n>distance, every valid candidate shares an original rune.
		shared := []string{}
		seen := map[rune]bool{}
		for _, r := range token {
			if !seen[r] {
				seen[r] = true
				shared = append(shared, "c.term LIKE ?")
				arguments = append(arguments, "%"+string(r)+"%")
			}
		}
		predicates = append(predicates, "("+strings.Join(shared, " OR ")+")")
	}
	if len(predicates) == 0 {
		return []vocabularyEntry{}, nil
	}
	// Read distinct scalar keys first and apply exact edit distance before
	// joining document postings for tie-breaking frequencies. This avoids
	// counting common words which cannot possibly be a correction.
	parameters := append([]any{mapKeys(lengths)}, arguments...)
	statement := "WITH dictionary AS MATERIALIZED (SELECT DISTINCT term FROM search_product_terms WHERE length IN ?) SELECT c.term FROM dictionary c WHERE (" + strings.Join(predicates, " OR ") + ") ORDER BY c.term ASC"
	var dictionary []string
	if err := b.db.WithContext(ctx).Raw(statement, parameters...).Scan(&dictionary).Error; err != nil {
		return nil, err
	}
	terms := make([]string, 0, len(dictionary))
	for _, term := range dictionary {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for token, distance := range distances {
			if difference(utf8.RuneCountInString(term), utf8.RuneCountInString(token)) <= distance && damerauLevenshtein(token, term) <= distance {
				terms = append(terms, term)
				break
			}
		}
	}
	out := make([]vocabularyEntry, 0, len(terms))
	// Batches bound SQL parameter counts without truncating the vocabulary.
	for start := 0; start < len(terms); start += 500 {
		var rows []struct {
			Term      string
			Frequency int
		}
		if err := b.db.WithContext(ctx).Raw("SELECT t.term, COUNT(*) AS frequency FROM search_product_terms t JOIN search_documents d ON d.entity_id = t.product_id AND d.entity_type = 'product' AND d.active = TRUE WHERE t.term IN ? GROUP BY t.term ORDER BY t.term ASC", terms[start:min(start+500, len(terms))]).Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			out = append(out, vocabularyEntry{row.Term, row.Frequency})
		}
	}
	return out, nil
}
func sqlCoverage(field string, plan queryPlan) (string, []any) {
	if len(plan.groups) == 0 {
		return "0.0", nil
	}
	parts, args := []string{}, []any{}
	for _, g := range plan.groups {
		part := "CASE WHEN " + field + " LIKE ? THEN 1.0"
		args = append(args, "% "+g.original+" %")
		for _, a := range g.alternatives {
			if a == g.original {
				continue
			}
			part += " WHEN " + field + " LIKE ? THEN 0.5"
			args = append(args, "% "+a+" %")
		}
		parts = append(parts, part+" ELSE 0.0 END")
	}
	return "((" + strings.Join(parts, " + ") + ") / " + strconv.Itoa(len(plan.groups)) + ".0)", args
}
func (b *databaseBackend) sqlScore(plan queryPlan, p RankingProfile) (string, []any) {
	parts, args := []string{}, []any{}
	for _, c := range []struct {
		field  string
		weight float64
	}{{"r.searchable", p.Weights.TokenCoverage}, {"r.name", p.Weights.Name}, {"r.brand", p.Weights.Brand}, {"r.attributes", p.Weights.Attributes}} {
		if c.weight == 0 {
			continue
		}
		expr, a := sqlCoverage(c.field, plan)
		parts = append(parts, "("+expr+" * ?)")
		args = append(args, a...)
		args = append(args, c.weight)
	}
	phrase := NormalizeQuery(strings.Join(originalQueryTerms(plan), " "))
	if p.Weights.ExactPhrase > 0 && len(strings.Fields(phrase)) > 1 {
		parts = append(parts, "(CASE WHEN r.name LIKE ? THEN 1.0 WHEN r.description LIKE ? THEN 0.5 ELSE 0.0 END * ?)")
		args = append(args, "% "+phrase+" %", "% "+phrase+" %", p.Weights.ExactPhrase)
	}
	factor := math.Exp2(-float64(b.now().UTC().Truncate(24*time.Hour).Unix()) / (30 * 86400))
	if p.Weights.Recency > 0 {
		parts = append(parts, "(CASE WHEN r.recency_base * ? > 1.0 THEN 1.0 ELSE r.recency_base * ? END * ?)")
		args = append(args, factor, factor, p.Weights.Recency)
	}
	if p.Weights.Availability > 0 {
		parts = append(parts, "(CASE WHEN d.available THEN 1.0 ELSE 0.0 END * ?)")
		args = append(args, p.Weights.Availability)
	}
	if p.Weights.Sales > 0 {
		parts = append(parts, "(CASE WHEN ss.units30_days > 0 THEN 1.0 * ss.units30_days / (ss.units30_days + 20.0) ELSE 0.0 END * ?)")
		args = append(args, p.Weights.Sales)
	}
	if p.Weights.Margin > 0 {
		parts = append(parts, "(d.margin_rate * ?)")
		args = append(args, p.Weights.Margin)
	}
	if p.Weights.Conversion > 0 {
		parts = append(parts, "(CASE WHEN cs.impressions30_days > 0 AND cs.conversions30_days > 0 THEN 1.0 * cs.conversions30_days / (cs.impressions30_days + 20.0) ELSE 0.0 END * ?)")
		args = append(args, p.Weights.Conversion)
	}
	if len(parts) == 0 {
		return "0.0", nil
	}
	return "(" + strings.Join(parts, " + ") + ")", args
}

func (b *databaseBackend) searchProductsSQLQuery(ctx context.Context, f Filters) (Result, error) {
	normalized, err := normalizeFilters(f)
	if err != nil {
		return Result{}, err
	}
	synonyms, err := b.loadSynonymRules(ctx)
	if err != nil {
		return Result{}, err
	}
	typo, err := b.loadTypoToleranceProfile(ctx)
	if err != nil {
		return Result{}, err
	}
	profile, err := b.loadRankingProfile(ctx, f.RankingProfile)
	if err != nil {
		return Result{}, err
	}
	// Cached exposures are not validated strongly enough for public ranking.
	// Profiles remain intact for administrative explanations and previews.
	if !f.Explain {
		profile.Weights.Conversion = 0
	}
	query := NormalizeQuery(f.Query)
	plan := buildQueryPlan(query, synonyms)
	s := sqlSelection{plan: plan, filters: normalized}
	n, err := b.sqlPlanningMatches(ctx, s)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrIndexUnavailable, err)
	}
	rewrites, hint := append([]AppliedRewrite(nil), plan.rewrites...), ""
	if query != "" && n == 0 {
		vocab, err := b.sqlVocabulary(ctx, query, typo)
		if err != nil {
			return Result{}, fmt.Errorf("%w: %w", ErrIndexUnavailable, err)
		}
		corrected, changes := correctQuery(query, vocab, typo)
		if len(changes) > 0 {
			candidate := s
			candidate.plan = buildQueryPlan(corrected, synonyms)
			count, err := b.sqlPlanningMatches(ctx, candidate)
			if err != nil {
				return Result{}, err
			}
			if count > 0 {
				hint = corrected
				if !typo.StrictMode {
					s = candidate
					n = count
					rewrites = append(changes, candidate.plan.rewrites...)
				}
			}
		}
		if !typo.StrictMode && n == 0 && len(plan.groups) > 1 {
			candidate := s
			candidate.relaxed = true
			count, err := b.sqlPlanningMatches(ctx, candidate)
			if err != nil {
				return Result{}, err
			}
			if count > 0 {
				s = candidate
				n = count
			}
		}
	}
	targets, state, err := b.sqlMerchandising(ctx, f, normalized, query)
	if err != nil {
		return Result{}, err
	}
	state.finalizeHideDecisions(targets, s.plan, s.relaxed, normalized)
	for id := range state.hidden {
		s.hidden = append(s.hidden, id)
	}
	for _, t := range targets {
		if t.forcedInclude {
			s.included = append(s.included, t.document.EntityID)
		}
	}
	var facets []Facet
	if sharedFacetContext(s) {
		facets, n, err = b.sqlFacetsShared(ctx, s)
	} else {
		if len(s.hidden) > 0 || len(s.included) > 0 {
			n, err = b.sqlCount(ctx, s)
			if err != nil {
				return Result{}, err
			}
		}
		facets, err = b.sqlFacets(ctx, s)
	}
	if err != nil {
		return Result{}, err
	}
	eligible := filterIndexedProducts(targets, s.plan, s.relaxed, normalized, "")
	pins, bury, boost := state.sqlOrdering(eligible, n, f.SortField == "" || f.SortField == "relevance")
	page, limit := max(1, f.Page), f.Limit
	if limit < 1 {
		limit = 20
	}
	limit = min(100, limit)
	start, end := sqlPageWindow(n, page, limit)
	pinIDs := []uint{}
	before, inPage := int64(0), int64(0)
	for pos, id := range pins {
		pinIDs = append(pinIDs, id)
		if int64(pos) <= start {
			before++
		} else if int64(pos) <= end {
			inPage++
		}
	}
	q := b.selection(ctx, s, "")
	if len(pinIDs) > 0 {
		q = q.Where("d.entity_id NOT IN ?", pinIDs)
	}
	field := f.SortField
	if field == "" {
		field = "created_at"
		if query != "" {
			field = "relevance"
		}
	}
	direction := "DESC"
	if strings.EqualFold(f.SortOrder, "asc") {
		direction = "ASC"
	}
	score, args := b.sqlScore(s.plan, profile)
	if len(boost) > 0 {
		field = "relevance"
		parts := []string{"CASE d.entity_id"}
		for id, m := range boost {
			parts = append(parts, "WHEN "+strconv.FormatUint(uint64(id), 10)+" THEN "+strconv.FormatFloat(m, 'g', -1, 64))
		}
		score = "(" + score + " * " + strings.Join(parts, " ") + " ELSE 1.0 END)"
	}
	if len(bury) > 0 {
		ids := []string{}
		for id := range bury {
			ids = append(ids, strconv.FormatUint(uint64(id), 10))
		}
		q = q.Order("CASE WHEN d.entity_id IN (" + strings.Join(ids, ",") + ") THEN 1 ELSE 0 END ASC")
	}
	if field == "relevance" {
		if profile.Weights.Sales > 0 {
			q = q.Joins("LEFT JOIN search_sales_signals ss ON ss.product_id = d.entity_id")
		}
		if profile.Weights.Conversion > 0 {
			q = q.Joins("LEFT JOIN search_conversion_signals cs ON cs.product_id = d.entity_id")
		}
		q = q.Select("d.entity_id, "+score+" AS score", args...).Order("score " + direction + ", d.entity_id ASC")
	} else {
		column := map[string]string{"name": "d.normalized_name", "price": "d.min_price"}[field]
		if column == "" {
			column = "d.source_created_at"
		}
		q = q.Select("d.entity_id").Order(column + " " + direction + ", d.entity_id ASC")
	}
	var ids []struct {
		EntityID uint
		Score    float64
	}
	if end-start-inPage > 0 {
		if err := q.Offset(int(start - before)).Limit(int(end - start - inPage)).Scan(&ids).Error; err != nil {
			return Result{}, fmt.Errorf("%w: %w", ErrIndexUnavailable, err)
		}
	}
	ordered := []uint{}
	cursor := 0
	for pos := start + 1; pos <= end; pos++ {
		if id, ok := pins[int(pos)]; ok {
			ordered = append(ordered, id)
		} else {
			if cursor >= len(ids) {
				return Result{}, ErrIndexUnavailable
			}
			ordered = append(ordered, ids[cursor].EntityID)
			cursor++
		}
	}
	var rows []models.SearchDocument
	if len(ordered) > 0 {
		if err := b.db.WithContext(ctx).Where("entity_type = ? AND entity_id IN ?", ProductEntityType, ordered).Find(&rows).Error; err != nil {
			return Result{}, err
		}
	}
	docs, err := decodeIndexedProducts(ctx, rows)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrIndexUnavailable, err)
	}
	if f.Explain {
		if err := b.rankProductsUncached(ctx, docs, s.plan, NormalizeQuery(strings.Join(originalQueryTerms(s.plan), " ")), profile, f.SortOrder, true); err != nil {
			return Result{}, err
		}
	}
	byID := map[uint]indexedProduct{}
	for _, d := range docs {
		byID[d.document.EntityID] = d
	}
	result := Result{Products: []models.Product{}, Facets: facets, Total: n, TotalPages: int((n + int64(limit) - 1) / int64(limit)), NormalizedQuery: query, AppliedRewrites: rewrites, DidYouMean: hint, Relaxed: s.relaxed, RankingProfile: profile.Name, RankingProfileVersion: profile.Version}
	if f.Explain {
		result.RuleDecisions = state.decisions
	}
	for _, id := range ordered {
		d, ok := byID[id]
		if !ok {
			return Result{}, ErrIndexUnavailable
		}
		result.Products = append(result.Products, *d.product)
		if f.Explain {
			if m, ok := boost[id]; ok {
				d.ranking.AdjustedScore = d.ranking.Score * m
			}
			result.Explanations = append(result.Explanations, d.ranking)
		}
		if result.IndexedAt == nil || d.document.IndexedAt.After(*result.IndexedAt) {
			v := d.document.IndexedAt
			result.IndexedAt = &v
		}
	}
	return result, nil
}

// sqlPageWindow rejects arithmetic overflow without imposing an approximate
// candidate cap. Pages beyond the exact total have an empty window.
func sqlPageWindow(total int64, page, limit int) (int64, int64) {
	start := total
	if int64(page-1) <= total/int64(limit) {
		start = int64(page-1) * int64(limit)
	}
	return start, start + min(int64(limit), total-start)
}
