import type { PageServerLoad } from "./$types";
import { defaultSearchSort, isSearchSort } from "./search-params";
import {
	parseBrand,
	parseCategory,
	parseProduct,
	parseProductAttributeDefinition,
	type BrandModel,
	type ProductAttributeDefinitionModel,
	type ProductModel,
} from "$lib/models";
import { setPublicPageCacheHeaders } from "$lib/server/cache";
import { serverRequest } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";

type ProductSearchPayload = components["schemas"]["ProductSearchResponse"];
type BrandListPayload = components["schemas"]["BrandListResponse"];
type ProductAttributeDefinitionListPayload =
	components["schemas"]["ProductAttributeDefinitionListResponse"];

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
	setPublicPageCacheHeaders(event);
	const { url } = event;
	const searchQuery = url.searchParams.get("q") ?? "";
	const brandSlugs = url.searchParams.getAll("brand_slug").filter(Boolean);
	const brandSlug = brandSlugs[0] ?? "";
	const categorySlugs = url.searchParams.getAll("category_slug").filter(Boolean);
	const stockSelections = url.searchParams
		.getAll("has_variant_stock")
		.filter((value) => value === "true" || value === "false")
		.map((value) => value === "true");
	const priceRanges = url.searchParams.getAll("price_range").filter(Boolean);
	const attributeSelections: Record<string, string[]> = {};
	const hasVariantStock = url.searchParams.get("has_variant_stock") === "true";
	const currentPage = Math.max(1, Number(url.searchParams.get("page") ?? 1));
	const pageSize = normalizeLimit(Number(url.searchParams.get("limit") ?? 12));
	const requestedSort = url.searchParams.get("sort");
	const sortExplicit = isSearchSort(requestedSort);
	const sortBy = sortExplicit ? requestedSort : defaultSearchSort(searchQuery);
	const rankingProfile = url.searchParams.get("ranking_profile")?.trim() ?? "";
	const sortOrder = normalizeOrder(url.searchParams.get("order"));
	const attributeFilters: Record<string, string> = {};
	for (const [key, value] of url.searchParams.entries()) {
		if (!key.startsWith("attribute[") || !key.endsWith("]")) {
			continue;
		}
		const slug = key.slice("attribute[".length, -1).trim();
		if (!slug || !value.trim()) {
			continue;
		}
		attributeFilters[slug] = value.trim();
		(attributeSelections[slug] ??= []).push(value.trim());
	}

	let results: ProductModel[] = [];
	let brands: BrandModel[] = [];
	let categories: ReturnType<typeof parseCategory>[] = [];
	let facets: ProductSearchPayload["facets"] = [];
	let attributes: ProductAttributeDefinitionModel[] = [];
	let totalPages = 1;
	let totalResults = 0;
	let errorMessage = "";
	let metadata: ProductSearchPayload["metadata"] | null = null;
	let suggestions: components["schemas"]["SearchSuggestionsResponse"] = {
		suggestions: [],
		corrections: [],
		popular: [],
		trending: [],
	};

	try {
		const [response, brandsPayload, attributesPayload, categoriesPayload] = await Promise.all([
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
				ranking_profile: rankingProfile || undefined,
				order: sortOrder,
			}),
			serverRequest<BrandListPayload>(event, "/brands"),
			serverRequest<ProductAttributeDefinitionListPayload>(event, "/product-attributes"),
			serverRequest<components["schemas"]["CategoryListResponse"]>(event, "/categories"),
		]);
		metadata = response.metadata ?? null;
		facets = response.facets ?? [];
		categories = categoriesPayload.data.map(parseCategory);
		results = response.items.map(parseProduct);
		brands = brandsPayload.data.map(parseBrand);
		attributes = attributesPayload.data.map(parseProductAttributeDefinition);
		totalPages = Math.max(1, response.pagination.total_pages);
		totalResults = response.pagination.total;
	} catch (err) {
		console.error(err);
		errorMessage = "Unable to load search results.";
	}

	try {
		suggestions = await serverRequest<components["schemas"]["SearchSuggestionsResponse"]>(
			event,
			"/search/suggestions",
			{ q: searchQuery.trim() || undefined }
		);
	} catch {
		/* Discovery is optional during search outages. */
	}
	return {
		brandSlugs,
		categorySlugs,
		stockSelections,
		priceRanges,
		attributeSelections,
		facets,
		categories,
		metadata,
		suggestions,
		results,
		brands,
		attributes,
		errorMessage,
		searchQuery,
		draftQuery: searchQuery,
		brandSlug,
		hasVariantStock,
		attributeFilters,
		currentPage,
		pageSize,
		totalPages,
		totalResults,
		sortBy,
		sortExplicit,
		rankingProfile,
		sortOrder,
	};
};
