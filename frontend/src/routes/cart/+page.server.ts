import type { PageServerLoad } from "./$types";
import { parseCart, type CartModel } from "$lib/models";
import type { components } from "$lib/api/generated/openapi";
import { serverIsAuthenticated, serverRequest, type ServerAPIError } from "$lib/server/api";

type CartPayload = components["schemas"]["Cart"];

function isGuestCheckoutDisabled(body: unknown): boolean {
	return typeof body === "object" && body !== null && "code" in body
		? (body as { code?: unknown }).code === "guest_checkout_disabled"
		: false;
}

export const load: PageServerLoad = async (event) => {
	const isAuthenticated = await serverIsAuthenticated(event);

	try {
		const cart = parseCart(await serverRequest<CartPayload>(event, "/checkout/cart"));
		return {
			isAuthenticated,
			cart,
			errorMessage: "",
			guestCheckoutDisabled: false,
		};
	} catch (err) {
		const error = err as ServerAPIError;
		if (!isAuthenticated && error.status === 403 && isGuestCheckoutDisabled(error.body)) {
			return {
				isAuthenticated,
				cart: null as CartModel | null,
				errorMessage: "",
				guestCheckoutDisabled: true,
			};
		}
		if (error.status === 401) {
			return {
				isAuthenticated: false,
				cart: null as CartModel | null,
				errorMessage: "",
				guestCheckoutDisabled: false,
			};
		}
		console.error(err);
		return {
			isAuthenticated,
			cart: null as CartModel | null,
			errorMessage: "Unable to load your cart.",
			guestCheckoutDisabled: false,
		};
	}
};
