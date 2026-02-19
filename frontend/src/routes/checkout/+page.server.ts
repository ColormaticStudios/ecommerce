import type { PageServerLoad } from "./$types";
import {
	parseCart,
	parseSavedAddress,
	parseSavedPaymentMethod,
	type CartModel,
	type SavedAddressModel,
	type SavedPaymentMethodModel,
} from "$lib/models";
import type { components } from "$lib/api/generated/openapi";
import { serverIsAuthenticated, serverRequest, type ServerAPIError } from "$lib/server/api";

type CartPayload = components["schemas"]["Cart"];
type SavedAddressPayload = components["schemas"]["SavedAddress"];
type SavedPaymentMethodPayload = components["schemas"]["SavedPaymentMethod"];

export const load: PageServerLoad = async (event) => {
	const isAuthenticated = await serverIsAuthenticated(event);
	if (!isAuthenticated) {
		return {
			isAuthenticated,
			cart: null as CartModel | null,
			savedPaymentMethods: [] as SavedPaymentMethodModel[],
			savedAddresses: [] as SavedAddressModel[],
			errorMessage: "",
		};
	}

	try {
		const [cartPayload, paymentMethodsPayload, addressesPayload] = await Promise.all([
			serverRequest<CartPayload>(event, "/me/cart"),
			serverRequest<SavedPaymentMethodPayload[]>(event, "/me/payment-methods"),
			serverRequest<SavedAddressPayload[]>(event, "/me/addresses"),
		]);

		return {
			isAuthenticated,
			cart: parseCart(cartPayload),
			savedPaymentMethods: paymentMethodsPayload.map(parseSavedPaymentMethod),
			savedAddresses: addressesPayload.map(parseSavedAddress),
			errorMessage: "",
		};
	} catch (err) {
		console.error(err);
		const error = err as ServerAPIError;
		if (error.status === 401) {
			return {
				isAuthenticated: false,
				cart: null as CartModel | null,
				savedPaymentMethods: [] as SavedPaymentMethodModel[],
				savedAddresses: [] as SavedAddressModel[],
				errorMessage: "",
			};
		}
		return {
			isAuthenticated,
			cart: null as CartModel | null,
			savedPaymentMethods: [] as SavedPaymentMethodModel[],
			savedAddresses: [] as SavedAddressModel[],
			errorMessage: "Unable to load your checkout data.",
		};
	}
};
