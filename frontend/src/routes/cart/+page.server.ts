import type { PageServerLoad } from "./$types";
import { parseCart, type CartModel } from "$lib/models";
import type { components } from "$lib/api/generated/openapi";
import { serverIsAuthenticated, serverRequest, type ServerAPIError } from "$lib/server/api";

type CartPayload = components["schemas"]["Cart"];

export const load: PageServerLoad = async (event) => {
	const isAuthenticated = await serverIsAuthenticated(event);
	if (!isAuthenticated) {
		return {
			isAuthenticated,
			cart: null as CartModel | null,
			errorMessage: "",
		};
	}

	try {
		const cart = parseCart(await serverRequest<CartPayload>(event, "/me/cart"));
		return {
			isAuthenticated,
			cart,
			errorMessage: "",
		};
	} catch (err) {
		console.error(err);
		const error = err as ServerAPIError;
		if (error.status === 401) {
			return {
				isAuthenticated: false,
				cart: null as CartModel | null,
				errorMessage: "",
			};
		}
		return {
			isAuthenticated,
			cart: null as CartModel | null,
			errorMessage: "Unable to load your cart.",
		};
	}
};
