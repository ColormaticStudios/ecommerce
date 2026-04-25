import type { PageServerLoad } from "./$types";
import { parseProduct, type ProductModel } from "$lib/models";
import { serverRequest } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";

type InventoryReservationList = components["schemas"]["InventoryReservationList"];
type InventoryAlertList = components["schemas"]["InventoryAlertList"];
type InventoryThresholdList = components["schemas"]["InventoryThresholdList"];
type ProductPagePayload = components["schemas"]["ProductPage"];
type ReservationStatus = components["schemas"]["InventoryReservation"]["status"];
type AlertStatus = components["schemas"]["InventoryAlert"]["status"];

const defaultReservations: InventoryReservationList = { items: [] };
const defaultAlerts: InventoryAlertList = { items: [] };
const defaultThresholds: InventoryThresholdList = { items: [] };
const defaultProducts: ProductModel[] = [];

export const load: PageServerLoad = async (event) => {
	const { isAdmin } = await event.parent();
	const status = event.url.searchParams.getAll("status") as ReservationStatus[];
	const alertStatus = event.url.searchParams.getAll("alert_status") as AlertStatus[];
	const limit = Number(event.url.searchParams.get("limit") ?? 100);
	const errorMessages: string[] = [];

	if (!isAdmin) {
		return {
			reservations: defaultReservations,
			alerts: defaultAlerts,
			thresholds: defaultThresholds,
			products: defaultProducts,
			productTotal: 0,
			status,
			alertStatus,
			limit,
			errorMessages,
		};
	}

	try {
		const [reservations, alerts, thresholds, productPage] = await Promise.all([
			serverRequest<InventoryReservationList>(event, "/admin/inventory/reservations", {
				status: status.length ? status : undefined,
				limit,
			}),
			serverRequest<InventoryAlertList>(event, "/admin/inventory/alerts", {
				status: alertStatus.length ? alertStatus : ["OPEN", "ACKED"],
				limit,
			}),
			serverRequest<InventoryThresholdList>(event, "/admin/inventory/thresholds"),
			serverRequest<ProductPagePayload>(event, "/admin/products", {
				page: 1,
				limit: 20,
			}),
		]);
		return {
			reservations,
			alerts,
			thresholds,
			products: productPage.data.map(parseProduct),
			productTotal: productPage.pagination.total,
			status,
			alertStatus,
			limit,
			errorMessages,
		};
	} catch (err) {
		console.error(err);
		errorMessages.push("Unable to load inventory operations data.");
		return {
			reservations: defaultReservations,
			alerts: defaultAlerts,
			thresholds: defaultThresholds,
			products: defaultProducts,
			productTotal: 0,
			status,
			alertStatus,
			limit,
			errorMessages,
		};
	}
};
