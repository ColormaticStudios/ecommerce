import { afterEach, expect, test, vi } from "vitest";
import { load } from "./+page.server";
afterEach(() => vi.restoreAllMocks());

test("unauthorized route does not fetch merchandising data", async () => {
	const fetch = vi.spyOn(globalThis, "fetch");
	const result = await load({ parent: async () => ({ isAdmin: false }) } as never);
	expect(result).toMatchObject({ rules: [], audit: [], categories: [], errorMessages: [] });
	expect(fetch).not.toHaveBeenCalled();
});

test("rule loading errors remain visible while unrelated category loading succeeds", async () => {
	vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
		const url = new URL(String(input));
		return url.pathname.endsWith("merchandising-rules")
			? new Response(JSON.stringify({ detail: "Unavailable" }), { status: 503 })
			: new Response(JSON.stringify({ data: [] }));
	});
	const result = await load({
		parent: async () => ({ isAdmin: true }),
		getClientAddress: () => "198.51.100.1",
		request: new Request("http://localhost/admin/search/merchandising"),
	} as never);
	expect(result).toMatchObject({
		rules: [],
		categories: [],
		errorMessages: ["Unable to load merchandising rules."],
	});
});

test("deleted rule history loads even when no live rules remain", async () => {
	const deleted = {
		id: 3,
		rule_id: 7,
		operation: "delete",
		actor_id: 1,
		before: {
			name: "Retired jacket placement",
			action: { targets: [{ product_id: 101, product_name: "Field Jacket" }] },
		},
		after: null,
		created_at: "2026-10-05T12:00:00Z",
	};
	const urls: string[] = [];
	vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
		const url = new URL(String(input));
		urls.push(url.pathname);
		return new Response(
			JSON.stringify({ data: url.pathname.endsWith("merchandising-audit") ? [deleted] : [] })
		);
	});
	const result = await load({
		parent: async () => ({ isAdmin: true }),
		getClientAddress: () => "198.51.100.1",
		request: new Request("http://localhost/admin/search/merchandising"),
	} as never);
	expect(result).toMatchObject({ rules: [], audit: [deleted], errorMessages: [] });
	expect(urls).toContain("/api/v1/admin/search/merchandising-audit");
});

test("global history failures remain visible without hiding loaded rules", async () => {
	vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
		const url = new URL(String(input));
		return url.pathname.endsWith("merchandising-audit")
			? new Response(JSON.stringify({ detail: "Unavailable" }), { status: 503 })
			: new Response(JSON.stringify({ data: [] }));
	});
	const result = await load({
		parent: async () => ({ isAdmin: true }),
		getClientAddress: () => "198.51.100.1",
		request: new Request("http://localhost/admin/search/merchandising"),
	} as never);
	expect(result).toMatchObject({ audit: [], errorMessages: ["Unable to load rule history."] });
});

test("loader errors use the active locale bundle", async () => {
	vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
		const url = new URL(String(input));
		return url.pathname.endsWith("merchandising-rules")
			? new Response(JSON.stringify({ detail: "Unavailable" }), { status: 503 })
			: new Response(JSON.stringify({ data: [] }));
	});
	const locale = "de-DE";
	const result = await load({
		parent: async () => ({
			isAdmin: true,
			localization: {
				locale,
				locales: [],
				bundle: {
					resolution: {
						requested_locale: locale,
						resolved_locale: locale,
						source: "explicit",
						fallback_chain: [locale],
						used_fallback: false,
					},
					release: {
						id: 1,
						name: "German",
						snapshot_hash: "a".repeat(64),
						version: "b".repeat(64),
						published_at: "2026-10-12T00:00:00Z",
					},
					messages: {
						"admin.search.merchandising.load_rules_error": {
							value: "Regeln konnten nicht geladen werden.",
							requested_locale: locale,
							source_locale: locale,
							used_fallback: false,
							missing_translation: false,
						},
					},
				},
			},
		}),
		getClientAddress: () => "198.51.100.1",
		request: new Request("http://localhost/admin/search/merchandising"),
	} as never);
	expect(result).toMatchObject({ errorMessages: ["Regeln konnten nicht geladen werden."] });
});
