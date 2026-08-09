import type { ProductAttributeDefinitionModel } from "$lib/models";
import type { EditorAttributeValue, ProductUpsertInput } from "./types";
import { asTrimmedString } from "./state";

export function validateProductAttributes(
	attributes: EditorAttributeValue[],
	definitions: ProductAttributeDefinitionModel[]
): string | null {
	const selected = new Set<number>();
	for (const attribute of attributes) {
		const id = Number(attribute.product_attribute_id);
		if (!Number.isInteger(id) || id <= 0)
			return "Select an attribute for each assigned attribute row.";
		if (selected.has(id)) return "Each product attribute can only be assigned once.";
		selected.add(id);
		if (attribute.type === "number") {
			if (asTrimmedString(attribute.number_value) === "") return "Number attributes need a value.";
			if (!Number.isFinite(Number(attribute.number_value)))
				return "Number attributes need a valid numeric value.";
		}
		if (attribute.type === "text" && asTrimmedString(attribute.text_value) === "")
			return "Text attributes need a value.";
		if (attribute.type === "enum" && asTrimmedString(attribute.enum_value) === "")
			return "Enum attributes need a value.";
		const definition = definitions.find((item) => item.id === id);
		if (
			attribute.type === "enum" &&
			definition &&
			!definition.enum_values.includes(asTrimmedString(attribute.enum_value))
		) {
			return "Enum attributes need one of the allowed values.";
		}
	}
	return null;
}

export function validateProductPayload(
	payload: ProductUpsertInput,
	attributes: EditorAttributeValue[],
	definitions: ProductAttributeDefinitionModel[]
): string | null {
	if (!payload.sku || !payload.name) return "Please provide SKU and product name.";
	if (payload.variants.length === 0) return "Add at least one variant before saving.";
	if (
		payload.variants.some(
			(variant) => !Number.isFinite(variant.price) || !Number.isFinite(variant.stock)
		)
	) {
		return "Each variant needs a valid price and stock value.";
	}
	return validateProductAttributes(attributes, definitions);
}
