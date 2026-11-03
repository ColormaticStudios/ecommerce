import type { PageServerLoad } from "./$types";
import { defaultSearchSort, isSearchSort } from "./search-params";
import { parseProduct, type ProductModel } from "$lib/models";
import { setPublicPageCacheHeaders } from "$lib/server/cache";
import { serverRequest } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";

type ProductSearchPayload = components["schemas"]["ProductSearchResponse"];

const pageSizeOptions = [8, 12, 24, 36] as const;

function normalizeOrder(value: string | null): "asc" | "desc" {
	if (value === "asc" || value === "desc") {
		return value;
	}
	return "desc";
}

function normalizeLimit(value: number): number {
	if (pageSizeOptions.includes(value as (typeof pageSizeOptions)[number])) {
		return value;
	}
	return 12;
}

export const load: PageServerLoad = async (event) => {
	// Cache only server-signed snapshots; browser exposure proofs are fetched after consent.
	setPublicPageCacheHeaders(event);
	const { url } = event;
	const searchQuery = url.searchParams.get("q") ?? "";
	const brandSlugs = url.searchParams.getAll("brand_slug").filter(Boolean);
	const categorySlugs = url.searchParams.getAll("category_slug").filter(Boolean);
	const stockSelections = url.searchParams
		.getAll("has_variant_stock")
		.filter((value) => value === "true" || value === "false")
		.map((value) => value === "true");
	const priceRanges = url.searchParams.getAll("price_range").filter(Boolean);
	const attributeSelections: Record<string, string[]> = {};
	const requestedPage = Number(url.searchParams.get("page") ?? 1);
	const currentPage = Number.isSafeInteger(requestedPage) ? Math.max(1, requestedPage) : 1;
	const pageSize = normalizeLimit(Number(url.searchParams.get("limit") ?? 12));
	const requestedSort = url.searchParams.get("sort");
	const sortExplicit = isSearchSort(requestedSort);
	const sortBy = sortExplicit ? requestedSort : defaultSearchSort(searchQuery);
	const sortOrder = normalizeOrder(url.searchParams.get("order"));
	for (const [key, value] of url.searchParams.entries()) {
		if (!key.startsWith("attribute[") || !key.endsWith("]")) {
			continue;
		}
		const slug = key.slice("attribute[".length, -1).trim();
		if (!slug || !value.trim()) {
			continue;
		}
		(attributeSelections[slug] ??= []).push(value.trim());
	}

	let results: ProductModel[] = [];
	let facets: ProductSearchPayload["facets"] = [];
	let resultSnapshot = "";
	let totalPages = 1;
	let totalResults = 0;
	let errorMessage = "";
	let metadata: ProductSearchPayload["metadata"] | null = null;
	let suggestions: components["schemas"]["SearchSuggestionsResponse"] = {
		suggestions: [],
		corrections: [],
	};

	const [search, discovery] = await Promise.allSettled([
		serverRequest<ProductSearchPayload>(event, "/search/products", {
			q: searchQuery.trim() || undefined,
			brand_slug: brandSlugs.length ? brandSlugs : undefined,
			category_slug: categorySlugs.length ? categorySlugs : undefined,
			has_variant_stock: stockSelections.length ? stockSelections : undefined,
			price_range: priceRanges.length ? priceRanges : undefined,
			attribute: Object.keys(attributeSelections).length ? attributeSelections : undefined,
			page: currentPage,
			limit: pageSize,
			sort: sortExplicit ? sortBy : undefined,
			order: sortOrder,
		}),
		serverRequest<components["schemas"]["SearchSuggestionsResponse"]>(
			event,
			"/search/suggestions",
			{
				q: searchQuery.trim() || undefined,
			}
		),
	]);
	if (search.status === "fulfilled") {
		const response = search.value;
		resultSnapshot = response.result_snapshot ?? "";
		metadata = response.metadata ?? null;
		facets = response.facets ?? [];
		results = response.items.map(parseProduct);
		totalPages = Math.max(1, response.pagination.total_pages);
		totalResults = response.pagination.total;
	} else {
		console.error(search.reason);
		errorMessage = "Unable to load search results.";
	}

	if (discovery.status === "fulfilled" && !metadata?.degraded) suggestions = discovery.value;
	return {
		brandSlugs,
		categorySlugs,
		stockSelections,
		priceRanges,
		attributeSelections,
		facets,
		resultSnapshot,
		metadata,
		suggestions,
		results,
		errorMessage,
		searchQuery,
		draftQuery: searchQuery,
		currentPage,
		pageSize,
		totalPages,
		totalResults,
		sortBy,
		sortExplicit,
		sortOrder,
	};
};
