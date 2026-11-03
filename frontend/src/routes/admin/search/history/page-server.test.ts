import { afterEach, expect, test, vi } from "vitest";
import { load } from "./+page.server";
afterEach(() => vi.restoreAllMocks());
test("history does not request private data without admin access", async () => {
	const fetch = vi.spyOn(globalThis, "fetch");
	const url = new URL("http://localhost/admin/search/history");
	await load({
		url,
		getClientAddress: () => "198.51.100.1",
		request: new Request(url),
		parent: async () => ({ isAdmin: false }),
	} as never);
	expect(fetch).not.toHaveBeenCalled();
});
test("history sanitizes filters and reports errors without hiding filters", async () => {
	const fetch = vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("offline"));
	const url = new URL("http://localhost/admin/search/history?kind=bad&entity_id=-1&page=NaN");
	expect(
		await load({
			url,
			getClientAddress: () => "198.51.100.1",
			request: new Request(url),
			parent: async () => ({ isAdmin: true }),
		} as never)
	).toMatchObject({
		query: { page: 1, limit: 20 },
		errorMessage: expect.stringContaining("Unable to load"),
	});
	expect(String(fetch.mock.calls[0][0])).toContain("configuration-history?page=1&limit=20");
});
test("history forwards canonical kind, entity and pagination", async () => {
	const fetch = vi.spyOn(globalThis, "fetch").mockResolvedValue(
		new Response(
			JSON.stringify({
				items: [],
				pagination: { page: 2, limit: 20, total: 21, total_pages: 2 },
			}),
			{ headers: { "Content-Type": "application/json" } }
		)
	);
	const url = new URL("http://localhost/admin/search/history?kind=synonym&entity_id=7&page=2");
	expect(
		await load({
			url,
			getClientAddress: () => "198.51.100.1",
			request: new Request(url),
			parent: async () => ({ isAdmin: true }),
		} as never)
	).toMatchObject({ query: { kind: "synonym", entity_id: 7, page: 2 }, errorMessage: "" });
	expect(String(fetch.mock.calls[0][0])).toContain("entity_id=7");
});
