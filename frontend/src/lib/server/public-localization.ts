import type { RequestEvent } from "@sveltejs/kit";
import type { components } from "$lib/api/generated/openapi";
import { parseAcceptedLanguages, selectInitialLocale } from "$lib/localization/server";
import { localizationDomainForPath } from "$lib/localization/domain";
import { serverRequest } from "./api";
import { hasAuthenticatedContext } from "./cache";

type LocaleList = components["schemas"]["LocalizationLocaleList"];
type Bundle = components["schemas"]["LocalizationBundle"];

export function isPublicStorefront(
	event: Pick<RequestEvent, "url" | "route" | "request">
): boolean {
	return (
		(event.request.method === "GET" || event.request.method === "HEAD") &&
		Boolean(event.route.id?.startsWith("/(storefront)")) &&
		localizationDomainForPath(event.url.pathname) === "storefront"
	);
}

export function normalizeMarket(value: string): string {
	const market = value.trim().toUpperCase();
	return /^[A-Z]{2,3}$/.test(market) ? market : "";
}

export function canonicalPublicContext(
	event: Pick<RequestEvent, "url" | "request" | "cookies">,
	registry: LocaleList,
	accountLocale = ""
): { locale: string; market: string; redirect: string | null } {
	if (!registry.locales.some((locale) => locale.is_enabled && locale.is_default))
		throw new Error("Enabled default locale is unavailable");
	const query = event.url.searchParams;
	const market = normalizeMarket(
		query.has("market")
			? (query.get("market") ?? "")
			: (event.cookies.get("market") ?? event.request.headers.get("x-market") ?? "")
	);
	const locale = selectInitialLocale({
		locales: registry.locales,
		queryLocale: query.get("locale") ?? "",
		cookieLocale: event.cookies.get("locale"),
		headerLocale: event.request.headers.get("x-locale") ?? "",
		acceptedLanguages: parseAcceptedLanguages(event.request.headers.get("accept-language") ?? ""),
		accountLocale,
		market,
	});
	const canonical = new URL(event.url);
	canonical.searchParams.set("locale", locale);
	canonical.searchParams.set("market", market);
	const exact =
		query.getAll("locale").length === 1 &&
		query.get("locale") === locale &&
		query.getAll("market").length === 1 &&
		query.get("market") === market;
	return {
		locale,
		market,
		redirect: exact ? null : canonical.pathname + canonical.search + canonical.hash,
	};
}

export async function preparePublicLocalization(event: RequestEvent): Promise<string | null> {
	const registry = await serverRequest<LocaleList>(event, "/localization/locales");
	let accountLocale = "";
	if (hasAuthenticatedContext(event) && !event.url.searchParams.has("locale")) {
		try {
			accountLocale = (await serverRequest<components["schemas"]["User"]>(event, "/me/")).locale;
		} catch {
			/* Expired sessions still receive a private response. */
		}
	}
	const context = canonicalPublicContext(event, registry, accountLocale);
	if (context.redirect) return context.redirect;
	event.locals.publicLocalization = {
		...context,
		cacheable: !hasAuthenticatedContext(event),
		locales: registry,
		bundle: null,
	};
	event.locals.publicLocalization.bundle = await serverRequest<Bundle>(
		event,
		`/localization/bundles/${encodeURIComponent(context.locale)}`,
		{ domain: "storefront" }
	);
	return null;
}
