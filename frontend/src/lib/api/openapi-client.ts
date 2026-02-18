import type { paths } from "$lib/api/generated/openapi";

const DEFAULT_BASE_URL = "http://localhost:3000";

export type ListProductsQuery = paths["/api/v1/products"]["get"]["parameters"]["query"];
export type ListProductsSuccess =
	paths["/api/v1/products"]["get"]["responses"]["200"]["content"]["application/json"];
export type ListProductsFailure =
	paths["/api/v1/products"]["get"]["responses"]["500"]["content"]["application/json"];
export type GetProductSuccess =
	paths["/api/v1/products/{id}"]["get"]["responses"]["200"]["content"]["application/json"];
export type GetProductFailure =
	paths["/api/v1/products/{id}"]["get"]["responses"]["404"]["content"]["application/json"];

export async function fetchProducts(baseUrl = DEFAULT_BASE_URL, query?: ListProductsQuery) {
	const url = new URL("/api/v1/products", baseUrl);
	if (query) {
		Object.entries(query).forEach(([key, value]) => {
			if (value === undefined || value === null || value === "") {
				return;
			}
			url.searchParams.append(key, String(value));
		});
	}

	const response = await fetch(url.toString(), {
		method: "GET",
		credentials: "include",
	});
	const body = (await response.json()) as ListProductsSuccess | ListProductsFailure;
	if (!response.ok) {
		return {
			data: null,
			error: body as ListProductsFailure,
			response,
		};
	}

	return {
		data: body as ListProductsSuccess,
		error: null,
		response,
	};
}

export async function fetchProduct(baseUrl = DEFAULT_BASE_URL, id: number) {
	const url = new URL(`/api/v1/products/${id}`, baseUrl);
	const response = await fetch(url.toString(), {
		method: "GET",
		credentials: "include",
	});
	const body = (await response.json()) as GetProductSuccess | GetProductFailure;
	if (!response.ok) {
		return {
			data: null,
			error: body as GetProductFailure,
			response,
		};
	}

	return {
		data: body as GetProductSuccess,
		error: null,
		response,
	};
}
