import type { LocalizationRuntime } from "$lib/localization/runtime";

export function merchandisingLabel(
	value: string,
	messages: Pick<LocalizationRuntime, "translate">
): string {
	switch (value) {
		case "pin":
			return messages.translate("admin.search.merchandising.labels.pin", "Pin");
		case "bury":
			return messages.translate("admin.search.merchandising.labels.bury", "Bury");
		case "hide":
			return messages.translate("admin.search.merchandising.labels.hide", "Hide");
		case "boost":
			return messages.translate("admin.search.merchandising.labels.boost", "Boost");
		case "include":
			return messages.translate("admin.search.merchandising.labels.include", "Include");
		case "exact":
			return messages.translate("admin.search.merchandising.labels.exact", "Exact");
		case "prefix":
			return messages.translate("admin.search.merchandising.labels.prefix", "Prefix");
		case "contains":
			return messages.translate("admin.search.merchandising.labels.contains", "Contains");
		case "storefront":
			return messages.translate("admin.search.merchandising.labels.storefront", "Storefront");
		case "applied":
			return messages.translate("admin.search.merchandising.labels.applied", "Applied");
		case "skipped":
			return messages.translate("admin.search.merchandising.labels.skipped", "Skipped");
		case "conflict":
			return messages.translate("admin.search.merchandising.labels.conflict", "Conflict");
		case "create":
			return messages.translate("admin.search.merchandising.labels.create", "Created");
		case "update":
			return messages.translate("admin.search.merchandising.labels.update", "Updated");
		case "delete":
			return messages.translate("admin.search.merchandising.labels.delete", "Deleted");
		case "Rule is disabled.":
			return messages.translate(
				"admin.search.merchandising.reasons.rule_is_disabled",
				"Rule is disabled."
			);
		case "Rule has not started.":
			return messages.translate(
				"admin.search.merchandising.reasons.rule_has_not_started",
				"Rule has not started."
			);
		case "Rule has expired.":
			return messages.translate(
				"admin.search.merchandising.reasons.rule_has_expired",
				"Rule has expired."
			);
		case "Channel does not match.":
			return messages.translate(
				"admin.search.merchandising.reasons.channel_does_not_match",
				"Channel does not match."
			);
		case "Category context does not match.":
			return messages.translate(
				"admin.search.merchandising.reasons.category_context_does_not_match",
				"Category context does not match."
			);
		case "Original query does not match.":
			return messages.translate(
				"admin.search.merchandising.reasons.original_query_does_not_match",
				"Original query does not match."
			);
		case "Product is not an eligible search result.":
			return messages.translate(
				"admin.search.merchandising.reasons.product_is_not_an_eligible_search_result",
				"Product is not an eligible search result."
			);
		case "A higher-precedence hide rule already excludes this product.":
			return messages.translate(
				"admin.search.merchandising.reasons.a_higher_precedence_hide_rule_already_excludes_this_product",
				"A higher-precedence hide rule already excludes this product."
			);
		case "Product is excluded from results and facets.":
			return messages.translate(
				"admin.search.merchandising.reasons.product_is_excluded_from_results_and_facets",
				"Product is excluded from results and facets."
			);
		case "Product is not an eligible indexed result or facet candidate.":
			return messages.translate(
				"admin.search.merchandising.reasons.product_is_not_an_eligible_indexed_result_or_facet_candidate",
				"Product is not an eligible indexed result or facet candidate."
			);
		case "Hide takes precedence over this action.":
			return messages.translate(
				"admin.search.merchandising.reasons.hide_takes_precedence_over_this_action",
				"Hide takes precedence over this action."
			);
		case "A higher-precedence include rule already includes this product.":
			return messages.translate(
				"admin.search.merchandising.reasons.a_higher_precedence_include_rule_already_includes_this_product",
				"A higher-precedence include rule already includes this product."
			);
		case "Product is included while preserving shopper filters.":
			return messages.translate(
				"admin.search.merchandising.reasons.product_is_included_while_preserving_shopper_filters",
				"Product is included while preserving shopper filters."
			);
		case "Explicit sorting overrides this ordering action.":
			return messages.translate(
				"admin.search.merchandising.reasons.explicit_sorting_overrides_this_ordering_action",
				"Explicit sorting overrides this ordering action."
			);
		case "A higher-precedence pin already positions this product.":
			return messages.translate(
				"admin.search.merchandising.reasons.a_higher_precedence_pin_already_positions_this_product",
				"A higher-precedence pin already positions this product."
			);
		case "Requested pin position exceeds the result count; ordinary ordering is retained.":
			return messages.translate(
				"admin.search.merchandising.reasons.requested_pin_position_exceeds_the_result_count_ordinary_ordering_is_retained",
				"Requested pin position exceeds the result count; ordinary ordering is retained."
			);
		case "A higher-precedence pin occupies this position.":
			return messages.translate(
				"admin.search.merchandising.reasons.a_higher_precedence_pin_occupies_this_position",
				"A higher-precedence pin occupies this position."
			);
		case "Product is pinned to its requested position.":
			return messages.translate(
				"admin.search.merchandising.reasons.product_is_pinned_to_its_requested_position",
				"Product is pinned to its requested position."
			);
		case "Pin takes precedence over this ordering action.":
			return messages.translate(
				"admin.search.merchandising.reasons.pin_takes_precedence_over_this_ordering_action",
				"Pin takes precedence over this ordering action."
			);
		case "A higher-precedence bury rule already demotes this product.":
			return messages.translate(
				"admin.search.merchandising.reasons.a_higher_precedence_bury_rule_already_demotes_this_product",
				"A higher-precedence bury rule already demotes this product."
			);
		case "Product is moved after unburied results.":
			return messages.translate(
				"admin.search.merchandising.reasons.product_is_moved_after_unburied_results",
				"Product is moved after unburied results."
			);
		case "Only the highest-precedence boost multiplier applies.":
			return messages.translate(
				"admin.search.merchandising.reasons.only_the_highest_precedence_boost_multiplier_applies",
				"Only the highest-precedence boost multiplier applies."
			);
		case "Winning boost multiplier adjusts the relevance score.":
			return messages.translate(
				"admin.search.merchandising.reasons.winning_boost_multiplier_adjusts_the_relevance_score",
				"Winning boost multiplier adjusts the relevance score."
			);
		default:
			return value;
	}
}
