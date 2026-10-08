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
		request: new Request("http://localhost/admin/search/merchandising"),
	} as never);
	expect(result).toMatchObject({ audit: [], errorMessages: ["Unable to load rule history."] });
});
