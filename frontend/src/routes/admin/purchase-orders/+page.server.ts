import type { PageServerLoad } from "./$types";
import { parseProduct, type ProductModel } from "$lib/models";
import { serverRequest } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";

type PurchaseOrderList = components["schemas"]["PurchaseOrderList"];
type ProductPagePayload = components["schemas"]["ProductPage"];

export const load: PageServerLoad = async (event) => {
	const { isAdmin } = await event.parent();
	const errorMessages: string[] = [];
	let purchaseOrders: PurchaseOrderList = { items: [] };
	let products: ProductModel[] = [];

	if (!isAdmin) {
		return { purchaseOrders, products, errorMessages };
	}

	try {
		const [poPayload, productPayload] = await Promise.all([
			serverRequest<PurchaseOrderList>(event, "/admin/purchase-orders", { limit: 100 }),
			serverRequest<ProductPagePayload>(event, "/admin/products", { page: 1, limit: 20 }),
		]);
		purchaseOrders = poPayload;
		products = productPayload.data.map(parseProduct);
	} catch (err) {
		console.error(err);
		errorMessages.push("Unable to load purchase orders.");
	}

	return { purchaseOrders, products, errorMessages };
};
