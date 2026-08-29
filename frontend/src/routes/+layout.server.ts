import type { LayoutServerLoad } from "./$types";
import { serverRequest, type ServerAPIError } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";
import { parseAcceptedLanguages, selectInitialLocale } from "$lib/localization/server";
import { localizationDomainForPath } from "$lib/localization/domain";
import {
	parseCmsGlobalRegion,
	parseCmsNavigation,
	type CmsGlobalRegionModel,
	type CmsNavigationModel,
	type CmsGlobalRegionResponsePayload,
	type CmsNavigationResponsePayload,
} from "$lib/cms";

type DraftPreviewSessionPayload = components["schemas"]["DraftPreviewSessionResponse"];
type UserPayload = components["schemas"]["User"];
type LocalizationLocaleList = components["schemas"]["LocalizationLocaleList"];
type LocalizationBundle = components["schemas"]["LocalizationBundle"];

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
	let accountLocale = "";
	let localizationLocales: LocalizationLocaleList = { default_locale: "en-US", locales: [] };
	let localizationBundle: LocalizationBundle | null = null;
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
		const profile = await serverRequest<UserPayload>(event, "/me/");
		isAuthenticated = true;
		accountLocale = profile.locale;
	} catch (err) {
		const error = err as ServerAPIError;
		if (error.status !== 401) {
			console.error("Failed to resolve authentication state in layout", err);
		}
	}

	try {
		localizationLocales = await serverRequest<LocalizationLocaleList>(
			event,
			"/localization/locales"
		);
		const cookieLocale = event.cookies.get("locale") ?? "";
		const market =
			event.url.searchParams.get("market") ??
			event.cookies.get("market") ??
			event.request.headers.get("x-market") ??
			"";
		const locale = selectInitialLocale({
			locales: localizationLocales.locales,
			queryLocale: event.url.searchParams.get("locale") ?? "",
			cookieLocale,
			headerLocale: event.request.headers.get("x-locale") ?? "",
			accountLocale,
			acceptedLanguages: parseAcceptedLanguages(event.request.headers.get("accept-language") ?? ""),
			market,
		});
		localizationBundle = await serverRequest<LocalizationBundle>(
			event,
			`/localization/bundles/${encodeURIComponent(locale)}`,
			{ domain: localizationDomainForPath(event.url.pathname) }
		);
	} catch (err) {
		console.error("Failed to load localization bundle in layout", err);
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
		localization: {
			locale: localizationBundle?.resolution.resolved_locale ?? localizationLocales.default_locale,
			locales: localizationLocales.locales,
			bundle: localizationBundle,
		},
	};
};
