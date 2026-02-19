import type { PageServerLoad } from "./$types";
import { API } from "$lib/api";
import type { ProductModel } from "$lib/models";
import { setPublicPageCacheHeaders } from "$lib/server/cache";

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

	const api = new API();
	try {
		return {
			product: await api.getProduct(id),
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
