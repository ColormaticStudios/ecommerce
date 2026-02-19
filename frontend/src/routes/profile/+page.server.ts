import type { PageServerLoad } from "./$types";
import {
	parseProfile,
	parseSavedAddress,
	parseSavedPaymentMethod,
	type SavedAddressModel,
	type SavedPaymentMethodModel,
	type UserModel,
} from "$lib/models";
import type { components } from "$lib/api/generated/openapi";
import { serverIsAuthenticated, serverRequest, type ServerAPIError } from "$lib/server/api";

type ProfilePayload = components["schemas"]["User"];
type SavedAddressPayload = components["schemas"]["SavedAddress"];
type SavedPaymentMethodPayload = components["schemas"]["SavedPaymentMethod"];

export const load: PageServerLoad = async (event) => {
	const isAuthenticated = await serverIsAuthenticated(event);
	if (!isAuthenticated) {
		return {
			isAuthenticated,
			profile: null as UserModel | null,
			savedPaymentMethods: [] as SavedPaymentMethodModel[],
			savedAddresses: [] as SavedAddressModel[],
			errorMessage: "",
		};
	}

	try {
		const [profilePayload, paymentMethodsPayload, addressesPayload] = await Promise.all([
			serverRequest<ProfilePayload>(event, "/me/"),
			serverRequest<SavedPaymentMethodPayload[]>(event, "/me/payment-methods"),
			serverRequest<SavedAddressPayload[]>(event, "/me/addresses"),
		]);

		return {
			isAuthenticated,
			profile: parseProfile(profilePayload),
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
				profile: null as UserModel | null,
				savedPaymentMethods: [] as SavedPaymentMethodModel[],
				savedAddresses: [] as SavedAddressModel[],
				errorMessage: "",
			};
		}

		return {
			isAuthenticated,
			profile: null as UserModel | null,
			savedPaymentMethods: [] as SavedPaymentMethodModel[],
			savedAddresses: [] as SavedAddressModel[],
			errorMessage: "Unable to load your profile. Please try again.",
		};
	}
};
