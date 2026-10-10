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

test.each([
	["?q=jacket", "relevance", "desc", false, ""],
	["", "created_at", "desc", false, ""],
	["?q=%20", "created_at", "desc", false, ""],
	["?q=jacket&sort=created_at", "created_at", "desc", true, ""],
	["?sort=relevance&ranking_profile=new_arrivals", "relevance", "desc", true, "new_arrivals"],
	["?q=jacket&sort=price&order=asc", "price", "asc", true, ""],
	["?q=jacket&sort=name&order=asc", "name", "asc", true, ""],
	["?q=jacket&sort=invalid", "relevance", "desc", false, ""],
])(
	"search route selects ordering for %s",
	async (query, sortBy, sortOrder, sortExplicit, rankingProfile) => {
		const urls: URL[] = [];
		vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
			const url = new URL(String(input));
			urls.push(url);
			const body = url.pathname.endsWith("/search/products")
				? { items: [], pagination: { total: 0, total_pages: 0 } }
				: { data: [] };
			return new Response(JSON.stringify(body), { status: 200 });
		});
		const url = new URL(`http://localhost/search${query}`);
		const result = await load({ url, request: new Request(url), setHeaders: vi.fn() } as never);
		expect(result).toMatchObject({
			errorMessage: "",
			sortBy,
			sortOrder,
			sortExplicit,
			rankingProfile,
		});
		const search = urls.find((url) => url.pathname.endsWith("/search/products"));
		expect(search?.searchParams.get("sort")).toBe(sortExplicit ? sortBy : null);
		expect(search?.searchParams.get("order")).toBe(sortOrder);
		expect(search?.searchParams.get("ranking_profile")).toBe(rankingProfile || null);
	}
);
test("search loader retains all selected values for disjunctive facets", async () => {
	const urls: URL[] = [];
	vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
		const url = new URL(String(input));
		urls.push(url);
		return new Response(
			JSON.stringify(
				url.pathname.endsWith("/search/products")
					? { items: [], facets: [], pagination: { total: 0, total_pages: 1 } }
					: { data: [] }
			)
		);
	});
	const url = new URL(
		"http://localhost/search?brand_slug=one&brand_slug=two&category_slug=jackets&category_slug=coats&has_variant_stock=true&has_variant_stock=false&price_range=0:100&price_range=100:200&attribute[color]=red&attribute[color]=blue"
	);
	const result = await load({ url, request: new Request(url), setHeaders: vi.fn() } as never);
	expect(result).toMatchObject({
		brandSlugs: ["one", "two"],
		categorySlugs: ["jackets", "coats"],
		stockSelections: [true, false],
		priceRanges: ["0:100", "100:200"],
		attributeSelections: { color: ["red", "blue"] },
	});
	const search = urls.find((url) => url.pathname.endsWith("/search/products"));
	expect(search?.searchParams.getAll("brand_slug")).toEqual(["one", "two"]);
	expect(search?.searchParams.get("attribute[color][0]")).toBe("red");
	expect(search?.searchParams.get("attribute[color][1]")).toBe("blue");
});
