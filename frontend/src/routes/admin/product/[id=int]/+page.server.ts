import type { PageServerLoad } from "./$types";
import { parseProduct, parseProfile, type ProductModel } from "$lib/models";
import type { components } from "$lib/api/generated/openapi";
import { serverRequest, type ServerAPIError } from "$lib/server/api";

type ProfilePayload = components["schemas"]["User"];
type ProductPayload = components["schemas"]["Product"];

export const load: PageServerLoad = async (event) => {
	const id = Number(event.params.id);
	const hasProductId = Number.isFinite(id) && id > 0;
	let isAuthenticated = false;
	let isAdmin = false;
	let accessError = "";
	let initialProduct: ProductModel | null = null;

	try {
		const profilePayload = await serverRequest<ProfilePayload>(event, "/me/");
		isAuthenticated = true;
		isAdmin = parseProfile(profilePayload).role === "admin";
	} catch (err) {
		const error = err as ServerAPIError;
		if (error.status !== 401) {
			console.error(err);
			accessError = "Unable to check admin access.";
		}
		return { isAuthenticated, isAdmin, accessError, initialProduct };
	}

	if (!isAdmin || !hasProductId) {
		return { isAuthenticated, isAdmin, accessError, initialProduct };
	}

	try {
		const payload = await serverRequest<ProductPayload>(event, `/admin/products/${id}`);
		initialProduct = parseProduct(payload);
	} catch (err) {
		console.error(err);
	}

	return { isAuthenticated, isAdmin, accessError, initialProduct };
};
