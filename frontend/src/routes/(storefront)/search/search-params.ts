import type { SearchProductsQuery } from "$lib/api/openapi-client";

export type SearchSort = NonNullable<NonNullable<SearchProductsQuery>["sort"]>;

export function defaultSearchSort(query: string): SearchSort {
	return query.trim() ? "relevance" : "created_at";
}

export function isSearchSort(value: string | null): value is SearchSort {
	return value === "relevance" || value === "created_at" || value === "price" || value === "name";
}

export interface SearchUrlState {
	query: string;
	brandSlugs?: string[];
	categorySlugs?: string[];
	stockSelections?: boolean[];
	priceRanges?: string[];
	attributeSelections?: Record<string, string[]>;
	brandSlug: string;
	hasVariantStock: boolean;
	attributeFilters: Record<string, string>;
	page: number;
	limit: number;
	sort: SearchSort;
	sortExplicit: boolean;
	order: "asc" | "desc";
	rankingProfile: string;
}

export function buildSearchParams(next: SearchUrlState): URLSearchParams {
	const params = new URLSearchParams();
	if (next.query) params.set("q", next.query);
	for (const value of next.brandSlugs ?? (next.brandSlug ? [next.brandSlug] : []))
		params.append("brand_slug", value);
	for (const value of next.categorySlugs ?? []) params.append("category_slug", value);
	for (const value of next.stockSelections ?? (next.hasVariantStock ? [true] : []))
		params.append("has_variant_stock", String(value));
	for (const value of next.priceRanges ?? []) params.append("price_range", value);
	const attributes =
		next.attributeSelections ??
		Object.fromEntries(
			Object.entries(next.attributeFilters).map(([slug, value]) => [slug, [value]])
		);
	for (const [slug, values] of Object.entries(attributes))
		for (const value of values) if (value.trim()) params.append(`attribute[${slug}]`, value.trim());
	if (next.page > 1) params.set("page", String(next.page));
	if (next.limit !== 12) params.set("limit", String(next.limit));
	if (next.sortExplicit) params.set("sort", next.sort);
	if (next.order !== "desc") params.set("order", next.order);
	if (next.rankingProfile) params.set("ranking_profile", next.rankingProfile);
	return params;
}
