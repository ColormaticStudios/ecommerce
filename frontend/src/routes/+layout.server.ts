import type { LayoutServerLoad } from "./$types";
import { API_BASE_URL } from "$lib/config";
import { fetchStorefrontSettings } from "$lib/api/openapi-client";
import { createDefaultStorefrontSettings, parseStorefrontSettingsResponse } from "$lib/storefront";

export const load: LayoutServerLoad = async () => {
	try {
		const { data, error } = await fetchStorefrontSettings(API_BASE_URL);
		if (!data || error) {
			return {
				storefront: createDefaultStorefrontSettings(),
			};
		}

		const parsed = parseStorefrontSettingsResponse(data);
		return {
			storefront: parsed.settings,
		};
	} catch (err) {
		console.error("Failed to load storefront settings in layout", err);
		return {
			storefront: createDefaultStorefrontSettings(),
		};
	}
};
