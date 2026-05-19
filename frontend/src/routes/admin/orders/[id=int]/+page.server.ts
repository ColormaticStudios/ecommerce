import type { PageServerLoad } from "./$types";
import { parseOrder, type OrderModel } from "$lib/models";
import { serverRequest, type ServerAPIError } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";

type OrderPayload = components["schemas"]["Order"];
type OrderPaymentLedger = components["schemas"]["OrderPaymentLedger"];

function emptyResponse(
	overrides: {
		order?: OrderModel | null;
		payments?: OrderPaymentLedger | null;
		errorMessage?: string;
		paymentErrorMessage?: string;
	} = {}
) {
	return {
		order: null as OrderModel | null,
		payments: null as OrderPaymentLedger | null,
		errorMessage: "",
		paymentErrorMessage: "",
		...overrides,
	};
}

export const load: PageServerLoad = async (event) => {
	const { isAdmin } = await event.parent();

	if (!isAdmin) {
		return emptyResponse();
	}

	const id = Number(event.params.id);
	if (!Number.isFinite(id) || id <= 0) {
		return emptyResponse({ errorMessage: "Order not found." });
	}

	try {
		const [orderResult, paymentsResult] = await Promise.allSettled([
			serverRequest<OrderPayload>(event, `/admin/orders/${id}`),
			serverRequest<OrderPaymentLedger>(event, `/admin/orders/${id}/payments`),
		]);

		if (orderResult.status === "rejected") {
			const error = orderResult.reason as ServerAPIError;
			if (error.status === 404) {
				return emptyResponse({ errorMessage: "Order not found." });
			}
			console.error(orderResult.reason);
			return emptyResponse({ errorMessage: "Unable to load this order." });
		}

		let payments: OrderPaymentLedger | null = null;
		let paymentErrorMessage = "";
		if (paymentsResult.status === "fulfilled") {
			payments = paymentsResult.value;
		} else {
			console.error(paymentsResult.reason);
			paymentErrorMessage = "Unable to load payment activity.";
		}

		return emptyResponse({
			order: parseOrder(orderResult.value),
			payments,
			paymentErrorMessage,
		});
	} catch (error) {
		console.error(error);
		return emptyResponse({ errorMessage: "Unable to load this order." });
	}
};
