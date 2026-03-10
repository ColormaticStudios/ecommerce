import type { PageServerLoad } from "./$types";
import { parseProduct, type ProductModel } from "$lib/models";
import { serverRequest } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";

type ProductPagePayload = components["schemas"]["ProductPage"];

const defaultAdminPageLimit = 10;

export const load: PageServerLoad = async (event) => {
	const { isAdmin } = await event.parent();

	let products: ProductModel[] = [];
	const productPage = 1;
	let productTotalPages = 1;
	const productLimit = defaultAdminPageLimit;
	let productTotal = 0;
	let errorMessage = "";

	if (!isAdmin) {
		return {
			products,
			productPage,
			productTotalPages,
			productLimit,
			productTotal,
			errorMessage,
		};
	}

	try {
		const payload = await serverRequest<ProductPagePayload>(event, "/admin/products", {
			page: productPage,
			limit: productLimit,
		});
		products = payload.data.map(parseProduct);
		productTotalPages = Math.max(1, payload.pagination.total_pages);
		productTotal = payload.pagination.total;
	} catch (err) {
		console.error(err);
		errorMessage = "Unable to load products.";
	}

	return {
		products,
		productPage,
		productTotalPages,
		productLimit,
		productTotal,
		errorMessage,
	};
};
