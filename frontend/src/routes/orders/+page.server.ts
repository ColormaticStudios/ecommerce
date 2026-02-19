import type { PageServerLoad } from "./$types";
import { parseOrder, type OrderModel } from "$lib/models";
import type { components } from "$lib/api/generated/openapi";
import { serverIsAuthenticated, serverRequest, type ServerAPIError } from "$lib/server/api";

type OrderPayload = components["schemas"]["Order"];
type OrderPagePayload = components["schemas"]["OrderPage"];
type OrderStatus = components["schemas"]["Order"]["status"];

function normalizeStatus(value: string | null): "" | OrderStatus {
	if (value === "PAID" || value === "PENDING" || value === "FAILED") {
		return value;
	}
	return "";
}

function normalizePage(value: string | null): number {
	const parsed = Number(value ?? 1);
	if (!Number.isFinite(parsed) || parsed < 1) {
		return 1;
	}
	return Math.floor(parsed);
}

function normalizeLimit(value: string | null): string {
	if (value === "20" || value === "50") {
		return value;
	}
	return "10";
}

export const load: PageServerLoad = async (event) => {
	const page = normalizePage(event.url.searchParams.get("page"));
	const limit = normalizeLimit(event.url.searchParams.get("limit"));
	const statusFilter = normalizeStatus(event.url.searchParams.get("status"));
	const startDate = event.url.searchParams.get("start_date") ?? "";
	const endDate = event.url.searchParams.get("end_date") ?? "";

	const isAuthenticated = await serverIsAuthenticated(event);
	if (!isAuthenticated) {
		return {
			isAuthenticated,
			orders: [] as OrderModel[],
			totalPages: 1,
			totalOrders: 0,
			page,
			limit,
			statusFilter,
			startDate,
			endDate,
			errorMessage: "",
		};
	}

	try {
		const response = await serverRequest<OrderPagePayload>(event, "/me/orders", {
			page,
			limit: Number(limit),
			status: statusFilter || undefined,
			start_date: startDate || undefined,
			end_date: endDate || undefined,
		});
		const totalPages = Math.max(1, response.pagination.total_pages);
		const totalOrders = response.pagination.total;
		const parsedOrders = response.data.map(parseOrder);
		const missingItems = parsedOrders.filter((order) => order.items.length === 0);

		let orders = parsedOrders;
		if (missingItems.length > 0) {
			const detailResults = await Promise.allSettled(
				missingItems.map((order) => serverRequest<OrderPayload>(event, `/me/orders/${order.id}`))
			);
			const detailsById = new Map<number, OrderModel>();
			for (const result of detailResults) {
				if (result.status === "fulfilled") {
					const detailed = parseOrder(result.value);
					detailsById.set(detailed.id, detailed);
				}
			}
			orders = parsedOrders.map((order) => detailsById.get(order.id) ?? order);
		}

		return {
			isAuthenticated,
			orders,
			totalPages,
			totalOrders,
			page,
			limit,
			statusFilter,
			startDate,
			endDate,
			errorMessage: "",
		};
	} catch (err) {
		console.error(err);
		const error = err as ServerAPIError;
		if (error.status === 401) {
			return {
				isAuthenticated: false,
				orders: [] as OrderModel[],
				totalPages: 1,
				totalOrders: 0,
				page,
				limit,
				statusFilter,
				startDate,
				endDate,
				errorMessage: "",
			};
		}
		return {
			isAuthenticated,
			orders: [] as OrderModel[],
			totalPages: 1,
			totalOrders: 0,
			page,
			limit,
			statusFilter,
			startDate,
			endDate,
			errorMessage: "Unable to load orders.",
		};
	}
};
