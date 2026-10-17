import { expect, test } from "vitest";
import type { components } from "$lib/api/generated/openapi";
import { priceFacetBounds, visibleSearchFacets } from "./search-facets";

type FacetValue = components["schemas"]["SearchFacetValue"];
const value = (overrides: Partial<FacetValue> = {}): FacetValue => ({
	value: "0:100",
	count: 0,
	selected: false,
	disabled: true,
	...overrides,
});

test("price bounds prefer the numeric contract over encoded values and include zero", () => {
	expect(priceFacetBounds(value({ min_price: 0, max_price: 12.5, value: "10:20" }))).toEqual({
		min: 0,
		max: 12.5,
	});
	expect(priceFacetBounds(value({ min_price: 5, max_price: 5 }))).toEqual({ min: 5, max: 5 });
});

test.each([
	[":25", { min: undefined, max: 25 }],
	["0:", { min: 0, max: undefined }],
	["unrecognized", { min: undefined, max: undefined }],
	["Infinity:invalid", { min: undefined, max: undefined }],
])("legacy or selected range %s parses only finite nonempty bounds", (encoded, expected) => {
	expect(priceFacetBounds(value({ value: encoded as string }))).toEqual(expected);
});

test("empty groups disappear and unknown selected values remain removable", () => {
	const result = visibleSearchFacets(
		[
			{ name: "brand", type: "terms", values: [] },
			{ name: "category", type: "terms", values: [] },
		],
		[
			{ name: "brand", type: "terms", values: [value({ value: "unknown", selected: true })] },
			{ name: "stock", type: "boolean", values: [value({ value: "true" })] },
		]
	);
	expect(result.map((facet) => facet.name)).toEqual(["brand"]);
	expect(result[0].values[0]).toMatchObject({ value: "unknown", selected: true });
});

test("canonical labels survive fallback merging without mutating loader data", () => {
	const canonical = {
		name: "brand",
		type: "terms" as const,
		values: [value({ value: "known", label: "Canonical", selected: true })],
	};
	const fallback = {
		name: "brand",
		type: "terms" as const,
		values: [
			value({ value: "known", selected: true }),
			value({ value: "unknown", selected: true }),
		],
	};
	const [result] = visibleSearchFacets([canonical], [fallback]);
	expect(result.values.map((item) => item.value)).toEqual(["known", "unknown"]);
	expect(result.values[0].label).toBe("Canonical");
	expect(canonical.values).toHaveLength(1);
});
