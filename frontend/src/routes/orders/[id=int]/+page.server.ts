import type { PageServerLoad } from "./$types";
import {
	parseCheckoutOrderTracking,
	parseOrder,
	type OrderModel,
	type ShipmentModel,
} from "$lib/models";
import type { components } from "$lib/api/generated/openapi";
import { serverIsAuthenticated, serverRequest, type ServerAPIError } from "$lib/server/api";

type OrderPayload = components["schemas"]["Order"];
type CheckoutOrderTrackingPayload = components["schemas"]["CheckoutOrderTrackingResponse"];
type AuthenticationState = boolean | null;

function createEmptyResponse(
	overrides: {
		isAuthenticated?: AuthenticationState;
		order?: OrderModel | null;
		shipments?: ShipmentModel[];
		errorMessage?: string;
		trackingErrorMessage?: string;
	} = {}
) {
	return {
		isAuthenticated: null as AuthenticationState,
		order: null as OrderModel | null,
		shipments: [] as ShipmentModel[],
		errorMessage: "",
		trackingErrorMessage: "",
		...overrides,
	};
}

export const load: PageServerLoad = async (event) => {
	let isAuthenticated: AuthenticationState = null;

	try {
		isAuthenticated = await serverIsAuthenticated(event);
		const id = Number(event.params.id);
		if (!Number.isFinite(id) || id <= 0) {
			return createEmptyResponse({
				isAuthenticated,
				errorMessage: "Order not found.",
			});
		}

		if (!isAuthenticated) {
			return createEmptyResponse({
				isAuthenticated,
			});
		}

		const [orderResult, trackingResult] = await Promise.allSettled([
			serverRequest<OrderPayload>(event, `/me/orders/${id}`),
			serverRequest<CheckoutOrderTrackingPayload>(
				event,
				`/checkout/orders/${id}/shipping/tracking`
			),
		]);

		if (orderResult.status === "rejected") {
			const error = orderResult.reason as ServerAPIError;
			if (error.status === 401) {
				return createEmptyResponse({
					isAuthenticated: false,
				});
			}
			if (error.status === 404) {
				return createEmptyResponse({
					isAuthenticated,
					errorMessage: "Order not found.",
				});
			}
			console.error(orderResult.reason);
			return createEmptyResponse({
				isAuthenticated,
				errorMessage: "Unable to load this order.",
			});
		}

		const order = parseOrder(orderResult.value);
		let shipments: ShipmentModel[] = [];
		let trackingErrorMessage = "";

		if (trackingResult.status === "fulfilled") {
			shipments = parseCheckoutOrderTracking(trackingResult.value).shipments;
		} else {
			const error = trackingResult.reason as ServerAPIError;
			if (error.status === 401) {
				return createEmptyResponse({
					isAuthenticated: false,
				});
			}
			console.error(trackingResult.reason);
			trackingErrorMessage = "Unable to load shipment tracking.";
		}

		return createEmptyResponse({
			isAuthenticated,
			order,
			shipments,
			trackingErrorMessage,
		});
	} catch (err) {
		console.error(err);
		return createEmptyResponse({
			isAuthenticated,
			errorMessage: "Unable to load this order.",
		});
	}
};
