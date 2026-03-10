import type { PageServerLoad } from "./$types";
import { parseProduct, type ProductModel } from "$lib/models";
import type { components } from "$lib/api/generated/openapi";
import { serverRequest } from "$lib/server/api";

type ProductPayload = components["schemas"]["Product"];

export const load: PageServerLoad = async (event) => {
	const { isAdmin } = await event.parent();
	const id = Number(event.params.id);
	const hasProductId = Number.isFinite(id) && id > 0;
	let initialProduct: ProductModel | null = null;

	if (!isAdmin || !hasProductId) {
		return { initialProduct };
	}

	try {
		const payload = await serverRequest<ProductPayload>(event, `/admin/products/${id}`);
		initialProduct = parseProduct(payload);
	} catch (err) {
		console.error(err);
	}

	return { initialProduct };
};
