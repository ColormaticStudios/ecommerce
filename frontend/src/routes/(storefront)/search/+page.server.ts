import type { PageServerLoad } from "./$types";
import {
	parseBrand,
	parseProduct,
	parseProductAttributeDefinition,
	type BrandModel,
	type ProductAttributeDefinitionModel,
	type ProductModel,
} from "$lib/models";
import { setPublicPageCacheHeaders } from "$lib/server/cache";
import { serverRequest } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";

type ProductPagePayload = components["schemas"]["ProductPage"];
type BrandListPayload = components["schemas"]["BrandListResponse"];
type ProductAttributeDefinitionListPayload =
	components["schemas"]["ProductAttributeDefinitionListResponse"];

const pageSizeOptions = [8, 12, 24, 36] as const;

function normalizeSort(value: string | null): "created_at" | "price" | "name" {
	if (value === "price" || value === "name" || value === "created_at") {
		return value;
	}
	return "created_at";
}

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
	const brandSlug = url.searchParams.get("brand_slug") ?? "";
	const hasVariantStock = url.searchParams.get("has_variant_stock") === "true";
	const currentPage = Math.max(1, Number(url.searchParams.get("page") ?? 1));
	const pageSize = normalizeLimit(Number(url.searchParams.get("limit") ?? 12));
	const sortBy = normalizeSort(url.searchParams.get("sort"));
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
	}

	let results: ProductModel[] = [];
	let brands: BrandModel[] = [];
	let attributes: ProductAttributeDefinitionModel[] = [];
	let totalPages = 1;
	let totalResults = 0;
	let errorMessage = "";

	try {
		const [response, brandsPayload, attributesPayload] = await Promise.all([
			serverRequest<ProductPagePayload>(event, "/products", {
				q: searchQuery.trim() || undefined,
				brand_slug: brandSlug || undefined,
				has_variant_stock: hasVariantStock ? true : undefined,
				attribute: Object.keys(attributeFilters).length > 0 ? attributeFilters : undefined,
				page: currentPage,
				limit: pageSize,
				sort: sortBy,
				order: sortOrder,
			}),
			serverRequest<BrandListPayload>(event, "/brands"),
			serverRequest<ProductAttributeDefinitionListPayload>(event, "/product-attributes"),
		]);
		results = response.data.map(parseProduct);
		brands = brandsPayload.data.map(parseBrand);
		attributes = attributesPayload.data.map(parseProductAttributeDefinition);
		totalPages = Math.max(1, response.pagination.total_pages);
		totalResults = response.pagination.total;
	} catch (err) {
		console.error(err);
		errorMessage = "Unable to load search results.";
	}

	return {
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
		sortOrder,
	};
};
