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
type CheckoutPluginCatalogPayload = components["schemas"]["CheckoutPluginCatalog"];
type SavedAddressPayload = components["schemas"]["SavedAddress"];
type SavedPaymentMethodPayload = components["schemas"]["SavedPaymentMethod"];

export const load: PageServerLoad = async (event) => {
	const isAuthenticated = await serverIsAuthenticated(event);
	if (!isAuthenticated) {
		return {
			isAuthenticated,
			cart: null as CartModel | null,
			plugins: null as CheckoutPluginCatalogPayload | null,
			savedPaymentMethods: [] as SavedPaymentMethodModel[],
			savedAddresses: [] as SavedAddressModel[],
			errorMessage: "",
		};
	}

	try {
		const [cartPayload, pluginsPayload, savedPaymentMethodsPayload, savedAddressesPayload] =
			await Promise.all([
				serverRequest<CartPayload>(event, "/me/cart"),
				serverRequest<CheckoutPluginCatalogPayload>(event, "/me/checkout/plugins"),
				serverRequest<SavedPaymentMethodPayload[]>(event, "/me/payment-methods"),
				serverRequest<SavedAddressPayload[]>(event, "/me/addresses"),
			]);

		return {
			isAuthenticated,
			cart: parseCart(cartPayload),
			plugins: pluginsPayload,
			savedPaymentMethods: savedPaymentMethodsPayload.map(parseSavedPaymentMethod),
			savedAddresses: savedAddressesPayload.map(parseSavedAddress),
			errorMessage: "",
		};
	} catch (err) {
		console.error(err);
		const error = err as ServerAPIError;
		if (error.status === 401) {
			return {
				isAuthenticated: false,
				cart: null as CartModel | null,
				plugins: null as CheckoutPluginCatalogPayload | null,
				savedPaymentMethods: [] as SavedPaymentMethodModel[],
				savedAddresses: [] as SavedAddressModel[],
				errorMessage: "",
			};
		}
		return {
			isAuthenticated,
			cart: null as CartModel | null,
			plugins: null as CheckoutPluginCatalogPayload | null,
			savedPaymentMethods: [] as SavedPaymentMethodModel[],
			savedAddresses: [] as SavedAddressModel[],
			errorMessage: "Unable to load your checkout data.",
		};
	}
};
