import type { ProductModel } from "$lib/models";
import type {
	EditorAttributeValue,
	EditorKeyFactory,
	EditorOption,
	EditorOptionValue,
	EditorVariant,
	EditorVariantSelection,
	ProductEditorValues,
} from "./types";

export function asTrimmedString(value: unknown): string {
	return String(value ?? "").trim();
}

export function normalizedNumber(value: string): number | null | "invalid" {
	const trimmed = asTrimmedString(value);
	if (trimmed === "") return null;
	const parsed = Number(trimmed);
	return Number.isNaN(parsed) ? "invalid" : parsed;
}

export function createEditorKeyFactory(): EditorKeyFactory {
	let seed = 0;
	return (prefix) => `${prefix}-${++seed}`;
}

export function createOptionValue(
	nextKey: EditorKeyFactory,
	value = "",
	position = 1
): EditorOptionValue {
	return { key: nextKey("option-value"), value, position };
}

export function createOption(
	nextKey: EditorKeyFactory,
	position: number,
	name = "",
	displayType = "select",
	values: string[] = []
): EditorOption {
	return {
		key: nextKey("option"),
		name,
		display_type: displayType,
		position,
		values:
			values.length > 0
				? values.map((value, index) => createOptionValue(nextKey, value, index + 1))
				: [createOptionValue(nextKey)],
	};
}

export function createVariantSelection(
	nextKey: EditorKeyFactory,
	optionName = "",
	optionValue = "",
	position = 1
): EditorVariantSelection {
	return {
		key: nextKey("variant-selection"),
		option_name: optionName,
		option_value: optionValue,
		position,
	};
}

export function createVariant(
	nextKey: EditorKeyFactory,
	overrides: Partial<EditorVariant> = {}
): EditorVariant {
	return {
		key: nextKey("variant"),
		sku: "",
		title: "",
		price: "",
		compare_at_price: "",
		stock: "0",
		is_published: true,
		selections: [],
		...overrides,
	};
}

export function createAttributeValue(
	nextKey: EditorKeyFactory,
	position: number,
	overrides: Partial<EditorAttributeValue> = {}
): EditorAttributeValue {
	return {
		key: nextKey("attribute"),
		product_attribute_id: "",
		type: "",
		text_value: "",
		number_value: "",
		boolean_value: false,
		enum_value: "",
		position,
		...overrides,
	};
}

export function emptyEditorValues(nextKey: EditorKeyFactory): ProductEditorValues {
	return {
		sku: "",
		name: "",
		subtitle: "",
		description: "",
		selectedBrandId: "",
		selectedCategoryIds: [],
		seoTitle: "",
		seoDescription: "",
		seoCanonicalPath: "",
		seoOgImageMediaId: "",
		seoNoIndex: false,
		options: [],
		variants: [createVariant(nextKey)],
		attributeValues: [],
		defaultVariantSku: "",
		relatedSelected: [],
	};
}

export function editorValuesFromProduct(
	value: ProductModel,
	nextKey: EditorKeyFactory
): ProductEditorValues {
	const options = (value.options ?? []).map((option, optionIndex) => ({
		key: nextKey("option"),
		name: option.name,
		display_type: option.display_type,
		position: option.position || optionIndex + 1,
		values:
			option.values.length > 0
				? option.values.map((item, valueIndex) => ({
						key: nextKey("option-value"),
						value: item.value,
						position: item.position || valueIndex + 1,
					}))
				: [createOptionValue(nextKey)],
	}));
	const variants =
		(value.variants ?? []).length > 0
			? value.variants.map((variant) => ({
					key: nextKey("variant"),
					sku: variant.sku,
					title: variant.title,
					price: String(variant.price),
					compare_at_price:
						variant.compare_at_price == null ? "" : String(variant.compare_at_price),
					stock: String(variant.stock),
					is_published: variant.is_published,
					selections: (variant.selections ?? []).map((selection, selectionIndex) => ({
						key: nextKey("variant-selection"),
						option_name: selection.option_name,
						option_value: selection.option_value,
						position: selection.position || selectionIndex + 1,
					})),
				}))
			: [createVariant(nextKey)];

	return {
		sku: value.sku,
		name: value.name,
		subtitle: value.subtitle ?? "",
		description: value.description ?? "",
		selectedBrandId: value.brand ? String(value.brand.id) : "",
		selectedCategoryIds: (value.categories ?? []).map((category) => String(category.id)),
		seoTitle: value.seo.title ?? "",
		seoDescription: value.seo.description ?? "",
		seoCanonicalPath: value.seo.canonical_path ?? "",
		seoOgImageMediaId: value.seo.og_image_media_id ?? "",
		seoNoIndex: value.seo.noindex,
		options,
		variants,
		attributeValues: (value.attributes ?? []).map((attribute, index) =>
			createAttributeValue(nextKey, attribute.position || index + 1, {
				product_attribute_id: String(attribute.product_attribute_id),
				type: attribute.type,
				text_value: attribute.text_value ?? "",
				number_value: attribute.number_value == null ? "" : String(attribute.number_value),
				boolean_value: attribute.boolean_value ?? false,
				enum_value: attribute.enum_value ?? "",
			})
		),
		defaultVariantSku: value.default_variant_sku ?? value.variants?.[0]?.sku ?? "",
		relatedSelected: value.related_products ?? [],
	};
}

export function buildProductSnapshot(
	productId: number | null,
	values: ProductEditorValues
): string {
	return JSON.stringify({
		product_id: productId,
		fields: {
			sku: asTrimmedString(values.sku),
			name: asTrimmedString(values.name),
			subtitle: asTrimmedString(values.subtitle),
			description: asTrimmedString(values.description),
			brand_id: asTrimmedString(values.selectedBrandId),
			category_ids: values.selectedCategoryIds
				.map(Number)
				.filter((id) => Number.isInteger(id) && id > 0)
				.sort((a, b) => a - b),
			default_variant_sku: asTrimmedString(values.defaultVariantSku),
		},
		seo: {
			title: asTrimmedString(values.seoTitle),
			description: asTrimmedString(values.seoDescription),
			canonical_path: asTrimmedString(values.seoCanonicalPath),
			og_image_media_id: asTrimmedString(values.seoOgImageMediaId),
			noindex: values.seoNoIndex,
		},
		options: values.options.map((option, optionIndex) => ({
			name: asTrimmedString(option.name),
			display_type: asTrimmedString(option.display_type) || "select",
			position: optionIndex + 1,
			values: option.values.map((item, valueIndex) => ({
				value: asTrimmedString(item.value),
				position: valueIndex + 1,
			})),
		})),
		variants: values.variants.map((variant, variantIndex) => ({
			sku: asTrimmedString(variant.sku),
			title: asTrimmedString(variant.title),
			price: normalizedNumber(variant.price),
			compare_at_price: normalizedNumber(variant.compare_at_price),
			stock: normalizedNumber(variant.stock),
			is_published: variant.is_published,
			position: variantIndex + 1,
			selections: variant.selections.map((selection, selectionIndex) => ({
				option_name: asTrimmedString(selection.option_name),
				option_value: asTrimmedString(selection.option_value),
				position: selectionIndex + 1,
			})),
		})),
		related_product_ids: [...values.relatedSelected.map((item) => item.id)].sort((a, b) => a - b),
		attributes: values.attributeValues.map((attribute, index) => ({
			product_attribute_id: Number(attribute.product_attribute_id),
			type: attribute.type,
			position: index + 1,
			text_value: asTrimmedString(attribute.text_value),
			number_value: normalizedNumber(attribute.number_value),
			boolean_value: attribute.boolean_value,
			enum_value: asTrimmedString(attribute.enum_value),
		})),
	});
}
