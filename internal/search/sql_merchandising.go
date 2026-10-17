package search

import (
	"context"
	"ecommerce/models"
)

func (b *databaseBackend) sqlMerchandising(ctx context.Context, f Filters, n normalizedFilters, query string) ([]indexedProduct, *merchandisingState, error) {
	rules := []MerchandisingRule{}
	if !f.SkipMerchandising {
		if f.RuleOverrides != nil {
			rules = append(rules, (*f.RuleOverrides)...)
		} else {
			var err error
			rules, err = b.loadMerchandisingRules(ctx)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	ids := map[uint]struct{}{}
	for _, r := range rules {
		for _, t := range r.Action.Targets {
			ids[t.ProductID] = struct{}{}
		}
	}
	keys := mapKeys(ids)
	rows := []models.SearchDocument{}
	for start := 0; start < len(keys); start += 500 {
		var batch []models.SearchDocument
		if err := b.db.WithContext(ctx).Where("entity_type = ? AND active = ? AND entity_id IN ?", ProductEntityType, true, keys[start:min(start+500, len(keys))]).Find(&batch).Error; err != nil {
			return nil, nil, err
		}
		rows = append(rows, batch...)
	}
	docs, err := decodeIndexedProducts(ctx, rows)
	if err != nil {
		return nil, nil, err
	}
	f.RuleOverrides = &rules
	return b.prepareMerchandising(ctx, docs, f, n, query)
}

// Resolve only configured targets, using the SQL eligible total for pin bounds.
// This is the same precedence as order, without materializing ordinary results.
func (state *merchandisingState) sqlOrdering(documents []indexedProduct, total int64, allow bool) (map[int]uint, map[uint]bool, map[uint]float64) {
	eligible := map[uint]indexedProduct{}
	for _, d := range documents {
		eligible[d.document.EntityID] = d
	}
	pins := map[int]uint{}
	pinned := map[uint]bool{}
	bury := map[uint]bool{}
	boost := map[uint]float64{}
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
		d, ok := eligible[id]
		if !ok {
			continue
		}
		decision.ProductName = d.product.Name
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
		if !allow {
			decision.Reason = "Explicit sorting overrides this ordering action."
			continue
		}
		if kind == "pin" {
			if pinned[id] {
				decision.Outcome = "conflict"
				decision.Reason = "A higher-precedence pin already positions this product."
				continue
			}
			pos := *entry.target.Position
			if int64(pos) > total {
				decision.Reason = "Requested pin position exceeds the result count; ordinary ordering is retained."
				continue
			}
			if _, ok := pins[pos]; ok {
				decision.Outcome = "conflict"
				decision.Reason = "A higher-precedence pin occupies this position."
				continue
			}
			pinned[id] = true
			pins[pos] = id
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
		if _, ok := eligible[id]; !ok || state.hidden[id] || !allow {
			continue
		}
		if pinned[id] {
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
			if _, ok := boost[id]; ok {
				decision.Outcome = "conflict"
				decision.Reason = "Only the highest-precedence boost multiplier applies."
			} else {
				boost[id] = *entry.rule.Action.Multiplier
				decision.Outcome = "applied"
				decision.Reason = "Winning boost multiplier adjusts the relevance score."
			}
		}
	}
	return pins, bury, boost
}
