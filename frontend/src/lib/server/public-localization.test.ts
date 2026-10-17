import { afterEach, describe, expect, it, vi } from "vitest";
import type { RequestEvent } from "@sveltejs/kit";
import {
	canonicalPublicContext,
	preparePublicLocalization,
	isPublicStorefront,
} from "./public-localization";
import { setPublicPageCacheHeaders } from "./cache";

const registry = {
	default_locale: "en-US",
	locales: [
		{ code: "en-US", name: "English", is_enabled: true, is_default: true, default_for_markets: [] },
		{
			code: "fr",
			name: "French",
			is_enabled: true,
			is_default: false,
			default_for_markets: ["CA"],
		},
		{ code: "de", name: "German", is_enabled: false, is_default: false, default_for_markets: [] },
	],
};
function event(url: string, headers: HeadersInit = {}, cookieValues: Record<string, string> = {}) {
	const values = new Map(Object.entries(cookieValues));
	return {
		url: new URL(url),
		request: new Request(url, { headers }),
		getClientAddress: () => "198.51.100.9",
		route: { id: "/(storefront)/search" },
		locals: {},
		cookies: {
			get: (name: string) => values.get(name),
			getAll: () => [...values].map(([name, value]) => ({ name, value })),
			set: (name: string, value: string) => values.set(name, value),
		},
		setHeaders: vi.fn(),
	} as unknown as RequestEvent;
}
afterEach(() => vi.restoreAllMocks());
describe("canonical public localization", () => {
	it("privately negotiates missing locale and market without losing search filters", () => {
		const e = event(
			"https://shop.test/search?q=boots",
			{ "accept-language": "fr-CA, en;q=0.5" },
			{ market: "ca" }
		);
		expect(canonicalPublicContext(e, registry)).toEqual({
			locale: "fr",
			market: "CA",
			redirect: "/search?q=boots&locale=fr&market=CA",
		});
	});
	it("normalizes casing, duplicates, disabled locales and invalid market values", () => {
		expect(
			canonicalPublicContext(
				event("https://shop.test/search?locale=FR&locale=en-US&market=ca&market=US"),
				registry
			).redirect
		).toBe("/search?locale=fr&market=CA");
		expect(
			canonicalPublicContext(event("https://shop.test/search?locale=de&market=invalid"), registry)
		).toEqual({ locale: "en-US", market: "", redirect: "/search?locale=en-US&market=" });
		expect(() =>
			canonicalPublicContext(event("https://shop.test/search"), { ...registry, locales: [] })
		).toThrow();
	});
	it("explicit canonical values isolate cookies and language headers including the blank market", () => {
		const e = event(
			"https://shop.test/search?locale=fr&market=",
			{ "accept-language": "en-US", "x-market": "US", "x-locale": "en-US" },
			{ locale: "en-US", market: "CA" }
		);
		expect(canonicalPublicContext(e, registry)).toEqual({
			locale: "fr",
			market: "",
			redirect: null,
		});
	});
	it("keeps authenticated, preview, cohort and uninitialized responses private without raw Vary", () => {
		for (const [headers, cacheable] of [
			[{}, true],
			[{ cookie: "session_token=abc" }, true],
			[{ cookie: "draft_preview_token=abc" }, true],
			[{ authorization: "Bearer x" }, true],
			[{}, false],
		] as const) {
			const e = event("https://shop.test/search?locale=en-US&market=", headers);
			e.locals.publicLocalization = {
				locale: "en-US",
				market: "",
				cacheable,
				locales: registry,
				bundle: null,
			};
			setPublicPageCacheHeaders(e);
			const expected = cacheable && Object.keys(headers).length === 0 ? "public," : "private,";
			expect(vi.mocked(e.setHeaders).mock.calls[0][0]["Cache-Control"]).toMatch(
				new RegExp(`^${expected}`)
			);
			expect(vi.mocked(e.setHeaders).mock.calls[0][0]).not.toHaveProperty("Vary");
		}
		const uninitialized = event("https://shop.test/search");
		setPublicPageCacheHeaders(uninitialized);
		expect(vi.mocked(uninitialized.setHeaders).mock.calls[0][0]["Cache-Control"]).toBe(
			"private, no-store"
		);
	});
	it("reuses registry/bundle and forwards a newly issued private cohort to later SSR requests", async () => {
		const e = event("https://shop.test/search?locale=fr&market=");
		const calls: { url: URL; headers: Headers }[] = [];
		vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
			const url = new URL(String(input));
			calls.push({ url, headers: new Headers(init?.headers) });
			return url.pathname.endsWith("/locales")
				? new Response(JSON.stringify(registry))
				: new Response(JSON.stringify({ resolution: { resolved_locale: "en-US" } }), {
						headers: {
							"Cache-Control": "private, no-store",
							"Set-Cookie": "localization_rollout=stable; Path=/; HttpOnly",
						},
					});
		});
		expect(await preparePublicLocalization(e)).toBeNull();
		expect(e.cookies.get("localization_rollout")).toBe("stable");
		expect(e.locals.publicLocalization?.cacheable).toBe(false);
		expect(e.locals.publicLocalization?.locales).toEqual(registry);
		const { serverRequest } = await import("./api");
		await serverRequest(e, "/search/products");
		expect(calls[2].headers.get("cookie")).toContain("localization_rollout=stable");
		expect(calls[2].url.searchParams.get("locale")).toBe("fr");
		expect(calls[2].url.searchParams.has("market")).toBe(true);
	});
	it("does not canonicalize account, admin, non-page and mutation routes", () => {
		expect(isPublicStorefront(event("https://shop.test/search"))).toBe(true);
		for (const path of ["/checkout", "/profile", "/cart"])
			expect(isPublicStorefront(event(`https://shop.test${path}`))).toBe(false);
		const admin = event("https://shop.test/admin");
		admin.route.id = "/admin";
		expect(isPublicStorefront(admin)).toBe(false);
	});
});
