import type { ProductEditorValues, ProductUpsertInput } from "./types";
import { asTrimmedString } from "./state";

function optionalString(value: string): string | undefined {
	const trimmed = asTrimmedString(value);
	return trimmed === "" ? undefined : trimmed;
}

export function buildProductPayload(
	values: ProductEditorValues,
	images: string[]
): ProductUpsertInput {
	const variants = values.variants.map((variant, variantIndex) => ({
		sku: asTrimmedString(variant.sku),
		title: asTrimmedString(variant.title),
		price: Number(variant.price),
		compare_at_price:
			asTrimmedString(variant.compare_at_price) === ""
				? undefined
				: Number(variant.compare_at_price),
		stock: Number(variant.stock),
		position: variantIndex + 1,
		is_published: variant.is_published,
		selections: variant.selections.map((selection, selectionIndex) => ({
			option_name: asTrimmedString(selection.option_name),
			option_value: asTrimmedString(selection.option_value),
			position: selectionIndex + 1,
		})),
	}));

	return {
		sku: asTrimmedString(values.sku),
		name: asTrimmedString(values.name),
		subtitle: optionalString(values.subtitle),
		description: asTrimmedString(values.description),
		images,
		related_product_ids: values.relatedSelected.map((item) => item.id),
		category_ids: values.selectedCategoryIds
			.map(Number)
			.filter((id) => Number.isInteger(id) && id > 0),
		brand_id: values.selectedBrandId ? Number(values.selectedBrandId) : undefined,
		default_variant_sku:
			optionalString(values.defaultVariantSku) ?? optionalString(variants[0]?.sku ?? ""),
		options: values.options.map((option, optionIndex) => ({
			name: asTrimmedString(option.name),
			position: optionIndex + 1,
			display_type: optionalString(option.display_type) ?? "select",
			values: option.values.map((item, valueIndex) => ({
				value: asTrimmedString(item.value),
				position: valueIndex + 1,
			})),
		})),
		variants,
		attributes: values.attributeValues
			.map((attribute, index) => {
				const productAttributeID = Number(attribute.product_attribute_id);
				if (!Number.isInteger(productAttributeID) || productAttributeID <= 0) return null;
				const payload: NonNullable<ProductUpsertInput["attributes"]>[number] = {
					product_attribute_id: productAttributeID,
					position: index + 1,
				};
				if (attribute.type === "text") payload.text_value = optionalString(attribute.text_value);
				if (attribute.type === "number") {
					payload.number_value =
						asTrimmedString(attribute.number_value) === ""
							? undefined
							: Number(attribute.number_value);
				}
				if (attribute.type === "boolean") payload.boolean_value = attribute.boolean_value;
				if (attribute.type === "enum") payload.enum_value = optionalString(attribute.enum_value);
				return payload;
			})
			.filter((attribute): attribute is NonNullable<typeof attribute> => attribute !== null),
		seo: {
			title: optionalString(values.seoTitle),
			description: optionalString(values.seoDescription),
			canonical_path: optionalString(values.seoCanonicalPath),
			og_image_media_id: optionalString(values.seoOgImageMediaId),
			noindex: values.seoNoIndex,
		},
	};
}
