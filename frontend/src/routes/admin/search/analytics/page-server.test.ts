import { afterEach, expect, test, vi } from "vitest";
import { load } from "./+page.server";
afterEach(() => vi.restoreAllMocks());
test("search administration does not request private data without admin access", async () => {
	const fetch = vi.spyOn(globalThis, "fetch");
	const url = new URL("http://localhost/admin/search");
	await load({ url, request: new Request(url), parent: async () => ({ isAdmin: false }) } as never);
	expect(fetch).not.toHaveBeenCalled();
});
test("search administration exposes load failures without losing the editor", async () => {
	vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("offline"));
	const url = new URL("http://localhost/admin/search");
	expect(
		await load({ url, request: new Request(url), parent: async () => ({ isAdmin: true }) } as never)
	).toMatchObject({ errorMessage: expect.stringContaining("Unable to load") });
});
