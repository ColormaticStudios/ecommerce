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

function isGuestCheckoutDisabled(body: unknown): boolean {
	return typeof body === "object" && body !== null && "code" in body
		? (body as { code?: unknown }).code === "guest_checkout_disabled"
		: false;
}

export const load: PageServerLoad = async (event) => {
	const isAuthenticated = await serverIsAuthenticated(event);

	try {
		const [cartPayload, pluginsPayload] = await Promise.all([
			serverRequest<CartPayload>(event, "/checkout/cart"),
			serverRequest<CheckoutPluginCatalogPayload>(event, "/checkout/plugins"),
		]);

		let savedPaymentMethods: SavedPaymentMethodModel[] = [];
		let savedAddresses: SavedAddressModel[] = [];
		if (isAuthenticated) {
			const [savedPaymentMethodsPayload, savedAddressesPayload] = await Promise.all([
				serverRequest<SavedPaymentMethodPayload[]>(event, "/me/payment-methods"),
				serverRequest<SavedAddressPayload[]>(event, "/me/addresses"),
			]);
			savedPaymentMethods = savedPaymentMethodsPayload.map(parseSavedPaymentMethod);
			savedAddresses = savedAddressesPayload.map(parseSavedAddress);
		}

		return {
			isAuthenticated,
			cart: parseCart(cartPayload),
			plugins: pluginsPayload,
			savedPaymentMethods,
			savedAddresses,
			errorMessage: "",
			guestCheckoutDisabled: false,
		};
	} catch (err) {
		const error = err as ServerAPIError;
		if (!isAuthenticated && error.status === 403 && isGuestCheckoutDisabled(error.body)) {
			return {
				isAuthenticated,
				cart: null as CartModel | null,
				plugins: null as CheckoutPluginCatalogPayload | null,
				savedPaymentMethods: [] as SavedPaymentMethodModel[],
				savedAddresses: [] as SavedAddressModel[],
				errorMessage: "",
				guestCheckoutDisabled: true,
			};
		}
		if (error.status === 401) {
			return {
				isAuthenticated: false,
				cart: null as CartModel | null,
				plugins: null as CheckoutPluginCatalogPayload | null,
				savedPaymentMethods: [] as SavedPaymentMethodModel[],
				savedAddresses: [] as SavedAddressModel[],
				errorMessage: "",
				guestCheckoutDisabled: false,
			};
		}
		console.error(err);
		return {
			isAuthenticated,
			cart: null as CartModel | null,
			plugins: null as CheckoutPluginCatalogPayload | null,
			savedPaymentMethods: [] as SavedPaymentMethodModel[],
			savedAddresses: [] as SavedAddressModel[],
			errorMessage: "Unable to load your checkout data.",
			guestCheckoutDisabled: false,
		};
	}
};
