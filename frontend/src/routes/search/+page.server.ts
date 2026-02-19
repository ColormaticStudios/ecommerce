import type { PageServerLoad } from "./$types";
import { API } from "$lib/api";
import type { ProductModel } from "$lib/models";
import { setPublicPageCacheHeaders } from "$lib/server/cache";

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
	const currentPage = Math.max(1, Number(url.searchParams.get("page") ?? 1));
	const pageSize = normalizeLimit(Number(url.searchParams.get("limit") ?? 12));
	const sortBy = normalizeSort(url.searchParams.get("sort"));
	const sortOrder = normalizeOrder(url.searchParams.get("order"));

	const api = new API();
	let results: ProductModel[] = [];
	let totalPages = 1;
	let totalResults = 0;
	let errorMessage = "";

	try {
		const response = await api.listProducts({
			q: searchQuery.trim() || undefined,
			page: currentPage,
			limit: pageSize,
			sort: sortBy,
			order: sortOrder,
		});
		results = response.data;
		totalPages = Math.max(1, response.pagination.total_pages);
		totalResults = response.pagination.total;
	} catch (err) {
		console.error(err);
		errorMessage = "Unable to load search results.";
	}

	return {
		results,
		errorMessage,
		searchQuery,
		draftQuery: searchQuery,
		currentPage,
		pageSize,
		totalPages,
		totalResults,
		sortBy,
		sortOrder,
	};
};
