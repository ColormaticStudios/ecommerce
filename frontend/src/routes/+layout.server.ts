import type { LayoutServerLoad } from "./$types";
import { serverIsAuthenticated, serverRequest, type ServerAPIError } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";
import {
	parseCmsGlobalRegion,
	parseCmsNavigation,
	type CmsGlobalRegionModel,
	type CmsNavigationModel,
	type CmsGlobalRegionResponsePayload,
	type CmsNavigationResponsePayload,
} from "$lib/cms";

type DraftPreviewSessionPayload = components["schemas"]["DraftPreviewSessionResponse"];

async function optionalServerRequest<T>(
	event: Parameters<LayoutServerLoad>[0],
	path: string
): Promise<T | null> {
	try {
		return await serverRequest<T>(event, path);
	} catch (err) {
		const error = err as ServerAPIError;
		if (error.status === 404 || error.status === 401 || error.status === 403) {
			return null;
		}
		throw err;
	}
}

export const load: LayoutServerLoad = async (event) => {
	let draftPreview: DraftPreviewSessionPayload = { active: false };
	let isAuthenticated = false;
	let cmsNavigation: CmsNavigationModel | null = null;
	const cmsGlobalRegions: Record<string, CmsGlobalRegionModel> = {};

	try {
		draftPreview = await serverRequest<DraftPreviewSessionPayload>(event, "/admin/preview");
	} catch (err) {
		const error = err as ServerAPIError;
		if (error.status !== 401 && error.status !== 403) {
			console.error("Failed to load draft preview state in layout", err);
		}
	}

	try {
		isAuthenticated = await serverIsAuthenticated(event);
	} catch (err) {
		console.error("Failed to resolve authentication state in layout", err);
	}

	try {
		const navigation = await optionalServerRequest<CmsNavigationResponsePayload>(
			event,
			"/content/navigation/header"
		);
		cmsNavigation = navigation ? parseCmsNavigation(navigation) : null;
	} catch (err) {
		console.error("Failed to load CMS navigation in layout", err);
	}

	for (const region of ["announcement_bar", "trust_strip", "sitewide_banner", "footer"]) {
		try {
			const response = await optionalServerRequest<CmsGlobalRegionResponsePayload>(
				event,
				`/content/global/${region}`
			);
			if (response) {
				cmsGlobalRegions[region] = parseCmsGlobalRegion(response, Boolean(draftPreview.active));
			}
		} catch (err) {
			console.error(`Failed to load CMS global region ${region}`, err);
		}
	}

	return {
		draftPreview,
		isAuthenticated,
		cmsNavigation,
		cmsGlobalRegions,
	};
};
