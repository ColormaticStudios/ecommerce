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
	if (next.brandSlug) params.set("brand_slug", next.brandSlug);
	if (next.hasVariantStock) params.set("has_variant_stock", "true");
	for (const [slug, value] of Object.entries(next.attributeFilters)) {
		if (value.trim()) params.set(`attribute[${slug}]`, value.trim());
	}
	if (next.page > 1) params.set("page", String(next.page));
	if (next.limit !== 12) params.set("limit", String(next.limit));
	if (next.sortExplicit) params.set("sort", next.sort);
	if (next.order !== "desc") params.set("order", next.order);
	if (next.rankingProfile) params.set("ranking_profile", next.rankingProfile);
	return params;
}
