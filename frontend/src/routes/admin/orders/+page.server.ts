import type { PageServerLoad } from "./$types";
import { parseOrder, type OrderModel } from "$lib/models";
import { serverRequest } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";

type OrderPagePayload = components["schemas"]["OrderPage"];

const defaultAdminPageLimit = 10;

export const load: PageServerLoad = async (event) => {
	const { isAdmin } = await event.parent();

	let orders: OrderModel[] = [];
	const orderPage = 1;
	let orderTotalPages = 1;
	const orderLimit = defaultAdminPageLimit;
	let orderTotal = 0;
	let errorMessage = "";

	if (!isAdmin) {
		return {
			orders,
			orderPage,
			orderTotalPages,
			orderLimit,
			orderTotal,
			errorMessage,
		};
	}

	try {
		const payload = await serverRequest<OrderPagePayload>(event, "/admin/orders", {
			page: orderPage,
			limit: orderLimit,
		});
		orders = payload.data.map(parseOrder);
		orderTotalPages = Math.max(1, payload.pagination.total_pages);
		orderTotal = payload.pagination.total;
	} catch (err) {
		console.error(err);
		errorMessage = "Unable to load orders.";
	}

	return {
		orders,
		orderPage,
		orderTotalPages,
		orderLimit,
		orderTotal,
		errorMessage,
	};
};
