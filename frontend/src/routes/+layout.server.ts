import type { LayoutServerLoad } from "./$types";
import { createDefaultStorefrontSettings, parseStorefrontSettingsResponse } from "$lib/storefront";
import { serverRequest, type ServerAPIError } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";

type StorefrontSettingsPayload = components["schemas"]["StorefrontSettingsResponse"];
type DraftPreviewSessionPayload = components["schemas"]["DraftPreviewSessionResponse"];

export const load: LayoutServerLoad = async (event) => {
	let storefront = createDefaultStorefrontSettings();
	let draftPreview: DraftPreviewSessionPayload = { active: false };

	try {
		const storefrontPayload = await serverRequest<StorefrontSettingsPayload>(event, "/storefront");
		const parsed = parseStorefrontSettingsResponse(storefrontPayload);
		storefront = parsed.settings;
	} catch (err) {
		console.error("Failed to load storefront settings in layout", err);
	}

	try {
		draftPreview = await serverRequest<DraftPreviewSessionPayload>(event, "/admin/preview");
	} catch (err) {
		const error = err as ServerAPIError;
		if (error.status !== 401 && error.status !== 403) {
			console.error("Failed to load draft preview state in layout", err);
		}
	}

	return {
		storefront,
		draftPreview,
	};
};
