package migrations

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

const searchReviewCatalogVersion = "2026101007_search_review_catalog"

// Frozen source messages for the search review; later extraction must not alter
// what this migration publishes when an older database is replayed.
var searchReviewCatalogKeys = []localizationP1BaselineKey{
	{Namespace: "errors", Key: "search_client_rate_limited", SourceText: "Search request budget exhausted; retry later.", OwnerDomain: "http"},
	{Namespace: "errors", Key: "search_reindex_already_queued", SourceText: "A search reindex is already queued or running.", OwnerDomain: "http"},
	{Namespace: "errors", Key: "search_analytics_admin_excluded", SourceText: "Administrators are excluded from shopper analytics.", OwnerDomain: "http"},
	{Namespace: "admin", Key: "navigation.search", SourceText: "Search", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.analytics.pagination", SourceText: "Page {page} of {pages} · {total} queries", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.analytics.title", SourceText: "search analytics", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.history.actor", SourceText: "Actor", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.history.after", SourceText: "After", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.history.all", SourceText: "All types", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.history.apply", SourceText: "Apply filters", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.history.before", SourceText: "Before", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.history.changes", SourceText: "configuration changes", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.history.configuration", SourceText: "Configuration", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.history.empty", SourceText: "No configuration changes match these filters.", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.history.entity_id", SourceText: "Configuration ID (optional)", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.history.filters", SourceText: "filter history", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.history.inspect", SourceText: "Inspect changes", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.history.kind", SourceText: "Configuration type", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.history.load_error", SourceText: "Unable to load search configuration history.", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.history.operation", SourceText: "Operation", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.history.pagination", SourceText: "Page {page} of {pages} · {total} changes", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.history.snapshots", SourceText: "Before and after", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.history.time", SourceText: "Changed at", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.history.title", SourceText: "search configuration history", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.history.unnamed", SourceText: "Unnamed configuration", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.merchandising.category_context", SourceText: "Category context (any selected)", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.merchandising.ends_at", SourceText: "Ends at (local time, optional)", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.merchandising.starts_at", SourceText: "Starts at (local time, optional)", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.merchandising.timezone", SourceText: "Times use your browser timezone: {timezone}. Saved as UTC.", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.merchandising.title", SourceText: "search merchandising", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.operations.title", SourceText: "search operations", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.preview.add_attribute", SourceText: "Add attribute", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.preview.attribute", SourceText: "Attribute", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.preview.attribute_values", SourceText: "Attribute values", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.preview.attributes", SourceText: "Attributes (comma-separated values)", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.preview.brands", SourceText: "Brands", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.preview.categories", SourceText: "Categories", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.preview.invalid_price", SourceText: "Enter a valid price range.", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.preview.price_max", SourceText: "Maximum price", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.preview.price_min", SourceText: "Minimum price", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.preview.remove_attribute", SourceText: "Remove attribute", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.preview.select_attribute", SourceText: "Select an attribute", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.preview.simulate_time", SourceText: "Simulate time (local time, optional)", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.preview.stock", SourceText: "Stock", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.ranking_profiles.title", SourceText: "search ranking profiles", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.selection.add", SourceText: "Search {label} to add", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.selection.remove", SourceText: "Remove {name}", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.synonyms.title", SourceText: "search synonyms", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.tabs.analytics", SourceText: "Analytics", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.tabs.history", SourceText: "History", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.tabs.label", SourceText: "{search} sections", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.tabs.merchandising", SourceText: "Merchandising", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.tabs.operations", SourceText: "Operations", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.tabs.ranking_profiles", SourceText: "Ranking profiles", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.tabs.synonyms", SourceText: "Synonyms", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.tabs.typo_profiles", SourceText: "Typo profiles", OwnerDomain: "admin"},
	{Namespace: "admin", Key: "search.typo_profiles.title", SourceText: "search typo profiles", OwnerDomain: "admin"},
	{Namespace: "storefront", Key: "product.no_image", SourceText: "No image", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "product.price_from", SourceText: "From {price}", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.ascending", SourceText: "Ascending", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.browse_all", SourceText: "Browse all products", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.browse_count.one", SourceText: "Browse {count} product", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.browse_count.other", SourceText: "Browse {count} products", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.clear_filters", SourceText: "Clear filters", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.descending", SourceText: "Descending", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.load_error", SourceText: "Unable to load search results.", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.loading", SourceText: "Loading results…", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.next", SourceText: "Next", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.no_matches", SourceText: "No matches found.", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.no_matches_help", SourceText: "Try a different keyword or clear filters.", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.page_count", SourceText: "Page {page} of {pages}", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.per_page", SourceText: "Per page", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.placeholder", SourceText: "Search products", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.previous", SourceText: "Previous", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.price_from", SourceText: "{min} and up", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.price_range", SourceText: "{min} – {max}", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.price_to", SourceText: "Up to {max}", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.results.one", SourceText: "{count} result for “{query}”", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.results.other", SourceText: "{count} results for “{query}”", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.sort_by", SourceText: "Sort by", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.sort_name", SourceText: "Name", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.sort_newest", SourceText: "Newest", OwnerDomain: "storefront"},
	{Namespace: "storefront", Key: "search.sort_relevance", SourceText: "Relevance", OwnerDomain: "storefront"},
}

func migrateSearchReviewCatalog(tx *gorm.DB) error {
	for _, key := range searchReviewCatalogKeys {
		now := time.Now().UTC()
		row := map[string]any{
			"namespace": key.Namespace, "key": key.Key, "source_text": key.SourceText,
			"description": "Search administration and storefront", "owner_domain": key.OwnerDomain,
			"is_deprecated": false, "created_at": now, "updated_at": now,
		}
		if err := tx.Session(&gorm.Session{NewDB: true}).Table("translation_keys").Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "namespace"}, {Name: "key"}}, DoNothing: true,
		}).Create(&row).Error; err != nil {
			return err
		}
	}
	return publishLocalizationDefaultSourceCatalog(tx)
}
