import { expect, test } from "vitest";
import { buildSearchParams, defaultSearchSort, type SearchUrlState } from "./search-params";

const state: SearchUrlState = {
	query: "jacket",
	brandSlug: "colormatic",
	hasVariantStock: true,
	attributeFilters: { color: "red" },
	page: 3,
	limit: 24,
	sort: "relevance",
	sortExplicit: false,
	order: "desc",
	rankingProfile: "new_arrivals",
};

test("default ordering follows whether the query has keywords", () => {
	expect(defaultSearchSort("jacket")).toBe("relevance");
	expect(defaultSearchSort("  ")).toBe("created_at");
});

test.each(["relevance", "created_at", "price", "name"] as const)(
	"explicit %s ordering survives query changes and preserves the ranking profile",
	(sort) => {
		for (const query of ["jacket", ""]) {
			const params = buildSearchParams({ ...state, query, sort, sortExplicit: true, page: 1 });
			expect(params.get("sort")).toBe(sort);
			expect(params.get("ranking_profile")).toBe("new_arrivals");
			expect(params.has("page")).toBe(false);
			expect(params.get("brand_slug")).toBe("colormatic");
			expect(params.get("attribute[color]")).toBe("red");
		}
	}
);

test("implicit ordering remains implicit when navigating or changing a query", () => {
	const params = buildSearchParams(state);
	expect(params.has("sort")).toBe(false);
	expect(params.get("page")).toBe("3");
	expect(params.get("limit")).toBe("24");
	expect(params.get("ranking_profile")).toBe("new_arrivals");
});
test("multi-value facets survive pagination and explicit ordering", () => {
	const params = buildSearchParams({
		...state,
		brandSlugs: ["one", "two"],
		categorySlugs: ["jackets", "coats"],
		stockSelections: [true, false],
		priceRanges: ["0:100", "100:200"],
		attributeSelections: { color: ["red", "blue"] },
	});
	expect(params.getAll("brand_slug")).toEqual(["one", "two"]);
	expect(params.getAll("category_slug")).toEqual(["jackets", "coats"]);
	expect(params.getAll("has_variant_stock")).toEqual(["true", "false"]);
	expect(params.getAll("price_range")).toEqual(["0:100", "100:200"]);
	expect(params.getAll("attribute[color]")).toEqual(["red", "blue"]);
	expect(params.get("page")).toBe("3");
});
