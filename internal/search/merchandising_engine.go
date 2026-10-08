package search

import (
	"context"
	"sort"
	"strings"
	"time"

	"ecommerce/models"
)

type merchandisingTargetDecision struct {
	rule   MerchandisingRule
	target MerchandisingTarget
	index  int
}
type merchandisingState struct {
	decisions []RuleDecision
	targets   []merchandisingTargetDecision
	hidden    map[uint]bool
}

func merchandisingPredicateReason(rule MerchandisingRule, query string, filters normalizedFilters, channel string, at time.Time) string {
	if !rule.IsActive {
		return "Rule is disabled."
	}
	if rule.StartsAt != nil && at.Before(*rule.StartsAt) {
		return "Rule has not started."
	}
	if rule.EndsAt != nil && !at.Before(*rule.EndsAt) {
		return "Rule has expired."
	}
	if channel == "" {
		channel = "storefront"
	}
	if rule.Predicate.Channel != "" && rule.Predicate.Channel != channel {
		return "Channel does not match."
	}
	if len(rule.Predicate.CategorySlugs) > 0 {
		matched := false
		for _, slug := range rule.Predicate.CategorySlugs {
			if _, ok := filters.categories[NormalizeQuery(slug)]; ok {
				matched = true
				break
			}
		}
		if !matched {
			return "Category context does not match."
		}
	}
	if q := rule.Predicate.Query; q != nil {
		matched := false
		switch q.Mode {
		case "exact":
			matched = query == q.Value
		case "prefix":
			matched = strings.HasPrefix(query, q.Value)
		case "contains":
			matched = strings.Contains(query, q.Value)
		}
		if !matched {
			return "Original query does not match."
		}
	}
	return ""
}
func merchandisingRuleBefore(a, b MerchandisingRule) bool {
	if a.Priority != b.Priority {
		return a.Priority > b.Priority
	}
	if a.ID == 0 && b.ID != 0 {
		return false
	}
	if a.ID != 0 && b.ID == 0 {
		return true
	}
	return a.ID < b.ID
}
func (b *databaseBackend) prepareMerchandising(ctx context.Context, documents []indexedProduct, filters Filters, normalized normalizedFilters, query string) ([]indexedProduct, *merchandisingState, error) {
	state := &merchandisingState{hidden: map[uint]bool{}}
	if filters.SkipMerchandising {
		return documents, state, nil
	}
	var rules []MerchandisingRule
	if filters.RuleOverrides != nil {
		rules = append([]MerchandisingRule(nil), (*filters.RuleOverrides)...)
	} else {
		var err error
		rules, err = NewService(b.db, nil, nil).ListMerchandisingRules(ctx)
		if err != nil {
			return nil, nil, err
		}
	}
	sort.SliceStable(rules, func(i, j int) bool { return merchandisingRuleBefore(rules[i], rules[j]) })
	at := b.now().UTC()
	if filters.MerchandisingAt != nil {
		at = filters.MerchandisingAt.UTC()
	}
	for _, rule := range rules {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if reason := merchandisingPredicateReason(rule, query, normalized, filters.Channel, at); reason != "" {
			state.decisions = append(state.decisions, RuleDecision{RuleID: rule.ID, RuleName: rule.Name, RuleType: rule.RuleType, Outcome: "skipped", Reason: reason})
			continue
		}
		for _, target := range rule.Action.Targets {
			index := len(state.decisions)
			state.decisions = append(state.decisions, RuleDecision{RuleID: rule.ID, RuleName: rule.Name, RuleType: rule.RuleType, ProductID: target.ProductID, ProductName: target.ProductName, Outcome: "skipped", Reason: "Product is not an eligible search result.", Position: target.Position, Multiplier: rule.Action.Multiplier})
			state.targets = append(state.targets, merchandisingTargetDecision{rule: rule, target: target, index: index})
		}
	}
	// Hide is independent of rule priority and wins over inclusion and ordering.
	for _, entry := range state.targets {
		if entry.rule.RuleType == "hide" {
			decision := &state.decisions[entry.index]
			if state.hidden[entry.target.ProductID] {
				decision.Outcome = "conflict"
				decision.Reason = "A higher-precedence hide rule already excludes this product."
			} else {
				state.hidden[entry.target.ProductID] = true
				decision.Outcome = "applied"
				decision.Reason = "Product is excluded from results and facets."
			}
		}
	}
	includeIDs := []uint{}
	seen := map[uint]bool{}
	for _, entry := range state.targets {
		if entry.rule.RuleType == "include" && !seen[entry.target.ProductID] {
			includeIDs = append(includeIDs, entry.target.ProductID)
			seen[entry.target.ProductID] = true
		}
	}
	existing := map[uint]int{}
	for i := range documents {
		existing[documents[i].document.EntityID] = i
	}
	if len(includeIDs) > 0 {
		for start := 0; start < len(includeIDs); start += 500 {
			var rows []models.SearchDocument
			if err := b.db.WithContext(ctx).Where("entity_type = ? AND active = ? AND entity_id IN ?", ProductEntityType, true, includeIDs[start:min(start+500, len(includeIDs))]).Order("entity_id ASC").Find(&rows).Error; err != nil {
				return nil, nil, err
			}
			included, err := decodeIndexedProducts(ctx, rows)
			if err != nil {
				return nil, nil, err
			}
			for _, document := range included {
				if !document.product.IsPublished {
					continue
				}
				if _, ok := existing[document.document.EntityID]; !ok {
					existing[document.document.EntityID] = len(documents)
					documents = append(documents, document)
				}
			}
		}
	}
	for i := range documents {
		documents[i].hidden = state.hidden[documents[i].document.EntityID]
		documents[i].forcedInclude = seen[documents[i].document.EntityID]
	}
	return documents, state, nil
}

// Hide masks also affect disjunctive facet contexts. Report impact only for
// indexed candidates that would qualify in results or one of those contexts.
func (state *merchandisingState) finalizeHideDecisions(documents []indexedProduct, plan queryPlan, relaxed bool, filters normalizedFilters) {
	visible := map[uint]bool{}
	excludedFacets := []string{"", "brand", "category", "stock", "price"}
	for slug := range filters.attributes {
		excludedFacets = append(excludedFacets, "attribute:"+slug)
	}
	for _, document := range documents {
		if !document.forcedInclude && !matchesQuery(document.document.SearchableText, plan, relaxed) {
			continue
		}
		for _, facet := range excludedFacets {
			if matchesFilters(document, filters, facet) {
				visible[document.document.EntityID] = true
				break
			}
		}
	}
	for i := range state.decisions {
		decision := &state.decisions[i]
		if decision.RuleType == "hide" && decision.Outcome == "applied" && !visible[decision.ProductID] {
			decision.Outcome = "skipped"
			decision.Reason = "Product is not an eligible indexed result or facet candidate."
		}
	}
}

func (state *merchandisingState) order(documents []indexedProduct, allowOrdering bool, order string) []indexedProduct {
	eligible := map[uint]int{}
	for i := range documents {
		eligible[documents[i].document.EntityID] = i
	}
	pinByProduct := map[uint]merchandisingTargetDecision{}
	pinByPosition := map[int]merchandisingTargetDecision{}
	bury := map[uint]bool{}
	boosted := map[uint]bool{}
	included := map[uint]bool{}
	for _, entry := range state.targets {
		kind, id := entry.rule.RuleType, entry.target.ProductID
		decision := &state.decisions[entry.index]
		if kind == "hide" {
			continue
		}
		if state.hidden[id] {
			decision.Outcome = "conflict"
			decision.Reason = "Hide takes precedence over this action."
			continue
		}
		index, ok := eligible[id]
		if !ok {
			continue
		}
		decision.ProductName = documents[index].product.Name
		if kind == "include" {
			if included[id] {
				decision.Outcome = "conflict"
				decision.Reason = "A higher-precedence include rule already includes this product."
			} else {
				included[id] = true
				decision.Outcome = "applied"
				decision.Reason = "Product is included while preserving shopper filters."
			}
			continue
		}
		if !allowOrdering {
			decision.Reason = "Explicit sorting overrides this ordering action."
			continue
		}
		if kind == "pin" {
			if _, ok := pinByProduct[id]; ok {
				decision.Outcome = "conflict"
				decision.Reason = "A higher-precedence pin already positions this product."
				continue
			}
			position := *entry.target.Position
			if position > len(documents) {
				decision.Reason = "Requested pin position exceeds the result count; ordinary ordering is retained."
				continue
			}
			if _, ok := pinByPosition[position]; ok {
				decision.Outcome = "conflict"
				decision.Reason = "A higher-precedence pin occupies this position."
				continue
			}
			pinByProduct[id] = entry
			pinByPosition[position] = entry
			decision.Outcome = "applied"
			decision.Reason = "Product is pinned to its requested position."
		}
	}
	for _, entry := range state.targets {
		kind, id := entry.rule.RuleType, entry.target.ProductID
		if kind != "bury" && kind != "boost" {
			continue
		}
		decision := &state.decisions[entry.index]
		index, ok := eligible[id]
		if !ok || state.hidden[id] || !allowOrdering {
			continue
		}
		if _, ok := pinByProduct[id]; ok {
			decision.Outcome = "conflict"
			decision.Reason = "Pin takes precedence over this ordering action."
			continue
		}
		if kind == "bury" {
			if bury[id] {
				decision.Outcome = "conflict"
				decision.Reason = "A higher-precedence bury rule already demotes this product."
			} else {
				bury[id] = true
				decision.Outcome = "applied"
				decision.Reason = "Product is moved after unburied results."
			}
		} else {
			if boosted[id] {
				decision.Outcome = "conflict"
				decision.Reason = "Only the highest-precedence boost multiplier applies."
			} else {
				boosted[id] = true
				documents[index].ranking.AdjustedScore = documents[index].ranking.Score * (*entry.rule.Action.Multiplier)
				decision.Outcome = "applied"
				decision.Reason = "Winning boost multiplier adjusts the relevance score."
			}
		}
	}
	if !allowOrdering {
		return documents
	}
	// Preserve the ordinary sort when no boost affects it, including default browse.
	if len(boosted) > 0 {
		ascending := strings.EqualFold(order, "asc")
		sort.SliceStable(documents, func(i, j int) bool {
			a, c := documents[i].ranking.AdjustedScore, documents[j].ranking.AdjustedScore
			if a == c {
				return documents[i].document.EntityID < documents[j].document.EntityID
			}
			if ascending {
				return a < c
			}
			return a > c
		})
	}
	regular := make([]indexedProduct, 0, len(documents))
	buried := make([]indexedProduct, 0, len(documents))
	pinned := map[uint]indexedProduct{}
	for _, document := range documents {
		id := document.document.EntityID
		if _, ok := pinByProduct[id]; ok {
			pinned[id] = document
		} else if bury[id] {
			buried = append(buried, document)
		} else {
			regular = append(regular, document)
		}
	}
	regular = append(regular, buried...)
	result := make([]indexedProduct, 0, len(documents))
	cursor := 0
	for position := 1; position <= len(documents); position++ {
		if pin, ok := pinByPosition[position]; ok {
			result = append(result, pinned[pin.target.ProductID])
			delete(pinned, pin.target.ProductID)
		} else if cursor < len(regular) {
			result = append(result, regular[cursor])
			cursor++
		}
	}
	return result
}
