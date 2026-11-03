import { describe, expect, it } from "vitest";
import { extractMediaId, moveItem } from "./media";
import { buildProductPayload } from "./payload";
import { mapMediaUploadProblem, mapProductEditorProblem } from "./problems";
import {
	buildProductSnapshot,
	createEditorKeyFactory,
	createOption,
	createVariant,
	emptyEditorValues,
	editorValuesFromProduct,
} from "./state";
import { makeProduct, makeVariant } from "$lib/storybook/factories";
import type { ProductEditorValues } from "./types";
import { validateProductPayload } from "./validation";
import { generateVariants } from "./variants";

function validValues(): ProductEditorValues {
	const nextKey = createEditorKeyFactory();
	const values = emptyEditorValues(nextKey);
	return {
		...values,
		sku: " JACKET ",
		name: " Field Jacket ",
		subtitle: " ",
		description: " Durable shell ",
		selectedBrandId: "4",
		selectedCategoryIds: ["3", "bad", "2"],
		seoCanonicalPath: " /products/field-jacket ",
		seoNoIndex: true,
		variants: [
			createVariant(nextKey, {
				sku: " JACKET-BLUE ",
				title: " Blue ",
				price: "129.50",
				stock: "8",
				compare_at_price: "149",
			}),
		],
	};
}

describe("buildProductPayload", () => {
	it("normalizes editor strings, IDs, numeric values, and defaults deterministically", () => {
		const payload = buildProductPayload(validValues(), ["/media/image/original"]);

		expect(payload).toMatchObject({
			sku: "JACKET",
			name: "Field Jacket",
			subtitle: undefined,
			description: "Durable shell",
			images: ["/media/image/original"],
			brand_id: 4,
			category_ids: [3, 2],
			default_variant_sku: "JACKET-BLUE",
			seo: { canonical_path: "/products/field-jacket", noindex: true },
		});
		expect(payload.variants[0]).toMatchObject({ price: 129.5, stock: 8, compare_at_price: 149 });
	});
});

describe("product editor validation", () => {
	it("accepts a complete payload", () => {
		const values = validValues();
		const payload = buildProductPayload(values, []);
		expect(validateProductPayload(payload, values.attributeValues, [])).toBeNull();
	});

	it("returns stable errors in a deterministic order", () => {
		const values = validValues();
		values.sku = "";
		values.variants[0].price = "not-a-number";
		let payload = buildProductPayload(values, []);
		expect(validateProductPayload(payload, values.attributeValues, [])).toBe(
			"Please provide SKU and product name."
		);

		values.sku = "JACKET";
		payload = buildProductPayload(values, []);
		expect(validateProductPayload(payload, values.attributeValues, [])).toBe(
			"Each variant needs a valid price and stock value."
		);
	});

	it("rejects duplicate and invalid typed attributes", () => {
		const values = validValues();
		values.attributeValues = [
			{
				key: "a",
				product_attribute_id: "7",
				type: "number",
				text_value: "",
				number_value: "12",
				boolean_value: false,
				enum_value: "",
				position: 1,
			},
			{
				key: "b",
				product_attribute_id: "7",
				type: "text",
				text_value: "cotton",
				number_value: "",
				boolean_value: false,
				enum_value: "",
				position: 2,
			},
		];
		const payload = buildProductPayload(values, []);
		expect(validateProductPayload(payload, values.attributeValues, [])).toBe(
			"Each product attribute can only be assigned once."
		);
	});
});

describe("editor state and helpers", () => {
	it("creates stable snapshots independent of category and related-product ordering", () => {
		const values = validValues();
		values.relatedSelected = [
			{ id: 9, sku: "B", name: "B", description: null, cover_image: null, stock: 0 },
			{ id: 5, sku: "A", name: "A", description: null, cover_image: null, stock: 0 },
		];
		const first = buildProductSnapshot(10, values);
		values.selectedCategoryIds.reverse();
		values.relatedSelected.reverse();
		expect(buildProductSnapshot(10, values)).toBe(first);
	});

	it("generates a deterministic variant matrix while preserving matching edits", () => {
		const nextKey = createEditorKeyFactory();
		const color = createOption(nextKey, 1, "Color", "swatch", ["Blue", "Red"]);
		const existing = createVariant(nextKey, {
			sku: "CUSTOM-BLUE",
			title: "Blue",
			price: "99",
			stock: "2",
			selections: [
				{ key: nextKey("selection"), option_name: "Color", option_value: "Blue", position: 1 },
			],
		});
		const generated = generateVariants(
			[color],
			[existing],
			"JACKET",
			"Jacket",
			"CUSTOM-BLUE",
			nextKey
		);
		expect(generated.map((variant) => variant.sku)).toEqual(["CUSTOM-BLUE", "JACKET-RED"]);
		expect(generated[0].price).toBe("99");
	});

	it("extracts media IDs and reorders without mutating the source", () => {
		const source = ["a", "b", "c"];
		expect(extractMediaId("https://shop.test/media/abc/original.jpg")).toBe("abc");
		expect(moveItem(source, 1, -1)).toEqual(["b", "a", "c"]);
		expect(source).toEqual(["a", "b", "c"]);
	});
});

describe("product editor problem mapping", () => {
	it("preserves legacy action and conflict messages with safe fallbacks", () => {
		expect(mapProductEditorProblem({ body: { error: "Draft is stale" } }, "fallback")).toBe(
			"Draft is stale"
		);
		expect(mapProductEditorProblem(new Error("private network detail"), "Unable to save.")).toBe(
			"Unable to save."
		);
		expect(
			mapMediaUploadProblem({ status: 409, body: { error: "Media is still processing" } })
		).toBe("Media is still processing");
		expect(mapMediaUploadProblem({ status: 500, body: { error: "internal" } })).toBe(
			"Unable to upload media."
		);
	});
});

describe("private variant unit costs", () => {
	it.each([
		["", null],
		["0", 0],
		["12.34", 12.34],
		["9999999999.99", 9999999999.99],
	] as const)("serializes %s with explicit nullable semantics", (input, expected) => {
		const values = validValues();
		values.variants[0].unit_cost = input;
		const payload = buildProductPayload(values, []);
		expect(payload.variants[0].unit_cost).toBe(expected);
		expect(validateProductPayload(payload, [], [])).toBeNull();
	});
	it.each(["-0.01", "NaN", "Infinity", "-Infinity", "10000000000"])(
		"rejects invalid unit cost %s before saving",
		(input) => {
			const values = validValues();
			values.variants[0].unit_cost = input;
			expect(validateProductPayload(buildProductPayload(values, []), [], [])).toMatch(/unit cost/);
		}
	);
	it("loads known zero and unknown costs without conflating them", () => {
		const product = makeProduct({
			variants: [
				makeVariant({ unit_cost: 0 }),
				makeVariant({ id: 12, sku: "other", unit_cost: null }),
			],
		});
		const values = editorValuesFromProduct(product, createEditorKeyFactory());
		expect(values.variants.map((variant) => variant.unit_cost)).toEqual(["0", ""]);
	});
	it("includes cost changes in the unsaved snapshot", () => {
		const values = validValues();
		const before = buildProductSnapshot(1, values);
		values.variants[0].unit_cost = "0";
		expect(buildProductSnapshot(1, values)).not.toBe(before);
	});
	it("preserves matching variant costs and leaves new option variants unknown", () => {
		const nextKey = createEditorKeyFactory();
		const color = createOption(nextKey, 1, "Color", "select", ["Blue", "Red"]);
		const existing = createVariant(nextKey, {
			sku: "CUSTOM-BLUE",
			price: "30",
			unit_cost: "12.34",
			selections: [
				{ key: nextKey("selection"), option_name: "Color", option_value: "Blue", position: 1 },
			],
		});
		const variants = generateVariants(
			[color],
			[existing],
			"JACKET",
			"Jacket",
			existing.sku,
			nextKey
		);
		expect(variants.map((variant) => variant.unit_cost)).toEqual(["12.34", ""]);
		expect(
			generateVariants([], [existing], "JACKET", "Jacket", existing.sku, nextKey)[0].unit_cost
		).toBe("12.34");
	});
});
