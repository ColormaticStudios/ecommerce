import type { PageServerLoad } from "./$types";
import { parseProduct, type ProductModel } from "$lib/models";
import { setPublicPageCacheHeaders } from "$lib/server/cache";
import { serverRequest } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";

type ProductPayload = components["schemas"]["Product"];

export const load: PageServerLoad = async (event) => {
	setPublicPageCacheHeaders(event);
	const { params } = event;
	const id = Number(params.id);
	if (!Number.isFinite(id) || id <= 0) {
		return {
			product: null as ProductModel | null,
			errorMessage: "Product not found.",
		};
	}

	try {
		return {
			product: parseProduct(await serverRequest<ProductPayload>(event, `/products/${id}`)),
			errorMessage: "",
		};
	} catch (err) {
		console.error(err);
		return {
			product: null as ProductModel | null,
			errorMessage: "Unable to load this product.",
		};
	}
};
