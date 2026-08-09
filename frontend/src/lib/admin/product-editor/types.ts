import type { components } from "$lib/api/generated/openapi";
import type { ProductAttributeDefinitionModel, RelatedProductModel } from "$lib/models";

export type ProductUpsertInput = components["schemas"]["ProductUpsertInput"];
export type ProductAttributeDefinitionInput =
	components["schemas"]["ProductAttributeDefinitionInput"];
export type EditorLayout = "stacked" | "split";

export interface EditorOptionValue {
	key: string;
	value: string;
	position: number;
}

export interface EditorOption {
	key: string;
	name: string;
	display_type: string;
	position: number;
	values: EditorOptionValue[];
}

export interface EditorVariantSelection {
	key: string;
	option_name: string;
	option_value: string;
	position: number;
}

export interface EditorVariant {
	key: string;
	sku: string;
	title: string;
	price: string;
	compare_at_price: string;
	stock: string;
	is_published: boolean;
	selections: EditorVariantSelection[];
}

export interface EditorAttributeValue {
	key: string;
	product_attribute_id: string;
	type: ProductAttributeDefinitionModel["type"] | "";
	text_value: string;
	number_value: string;
	boolean_value: boolean;
	enum_value: string;
	position: number;
}

export interface ProductEditorValues {
	sku: string;
	name: string;
	subtitle: string;
	description: string;
	selectedBrandId: string;
	selectedCategoryIds: string[];
	seoTitle: string;
	seoDescription: string;
	seoCanonicalPath: string;
	seoOgImageMediaId: string;
	seoNoIndex: boolean;
	options: EditorOption[];
	variants: EditorVariant[];
	attributeValues: EditorAttributeValue[];
	defaultVariantSku: string;
	relatedSelected: RelatedProductModel[];
}

export type EditorKeyFactory = (prefix: string) => string;
