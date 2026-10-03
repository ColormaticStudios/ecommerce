import { afterEach, expect, test, vi } from "vitest";
import { load } from "./+page.server";

afterEach(() => vi.restoreAllMocks());

test("search route serializes storefront attribute URLs using the search array contract", async () => {
	const urls: URL[] = [];
	vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
		const url = new URL(String(input));
		urls.push(url);
		const body = url.pathname.endsWith("/search/products")
			? { items: [], pagination: { total: 0, total_pages: 0 } }
			: { data: [] };
		return new Response(JSON.stringify(body), { status: 200 });
	});
	const url = new URL(
		"http://localhost/search?attribute[color]=red&attribute[waterproof]=false&attribute[weight]=0"
	);
	const result = await load({ url, request: new Request(url), setHeaders: vi.fn() } as never);
	expect(result).toMatchObject({
		errorMessage: "",
		attributeFilters: { color: "red", waterproof: "false", weight: "0" },
	});
	const search = urls.find((url) => url.pathname.endsWith("/search/products"));
	expect(search?.searchParams.get("attribute[color][0]")).toBe("red");
	expect(search?.searchParams.get("attribute[waterproof][0]")).toBe("false");
	expect(search?.searchParams.get("attribute[weight][0]")).toBe("0");
	expect(search?.searchParams.has("attribute[color]")).toBe(false);
});
