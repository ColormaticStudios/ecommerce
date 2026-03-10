import type { PageServerLoad } from "./$types";
import { serverRequest } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";

type CheckoutPluginCatalogPayload = components["schemas"]["CheckoutPluginCatalog"];

export const load: PageServerLoad = async (event) => {
	const { isAdmin } = await event.parent();

	let checkoutPlugins: CheckoutPluginCatalogPayload | null = null;
	let errorMessage = "";

	if (!isAdmin) {
		return { checkoutPlugins, errorMessage };
	}

	try {
		checkoutPlugins = await serverRequest<CheckoutPluginCatalogPayload>(
			event,
			"/admin/checkout/plugins"
		);
	} catch (err) {
		console.error(err);
		errorMessage = "Unable to load checkout providers.";
	}

	return { checkoutPlugins, errorMessage };
};
