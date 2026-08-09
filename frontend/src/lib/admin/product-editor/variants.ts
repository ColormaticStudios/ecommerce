import type { EditorKeyFactory, EditorOption, EditorVariant } from "./types";
import { asTrimmedString, createVariant, createVariantSelection } from "./state";

export function variantSelectionKey(
	selections: Array<{ option_name: string; option_value: string }>
): string {
	return selections
		.map(
			(selection) =>
				`${selection.option_name.toLowerCase()}=${selection.option_value.toLowerCase()}`
		)
		.sort()
		.join("|");
}

export function buildVariantSku(baseSku: string, selections: string[]): string {
	const suffix = selections
		.map((value) =>
			value
				.toUpperCase()
				.replace(/[^A-Z0-9]+/g, "-")
				.replace(/^-+|-+$/g, "")
		)
		.filter(Boolean)
		.join("-");
	return suffix ? `${baseSku}-${suffix}` : baseSku;
}

export function generateVariants(
	options: EditorOption[],
	existingVariants: EditorVariant[],
	baseSku: string,
	name: string,
	defaultVariantSku: string,
	nextKey: EditorKeyFactory
): EditorVariant[] {
	const seed =
		existingVariants.find((variant) => variant.sku === defaultVariantSku) ??
		existingVariants[0] ??
		createVariant(nextKey);
	const matrix = options
		.map((option) => ({
			name: asTrimmedString(option.name),
			values: option.values.map((item) => asTrimmedString(item.value)).filter(Boolean),
		}))
		.filter((option) => option.name !== "" && option.values.length > 0);
	if (matrix.length === 0) {
		return [
			createVariant(nextKey, {
				sku: asTrimmedString(baseSku),
				title: asTrimmedString(name) || "Default Variant",
				price: seed.price,
				compare_at_price: seed.compare_at_price,
				stock: seed.stock,
			}),
		];
	}
	const existingByKey = new Map(
		existingVariants.map((variant) => [variantSelectionKey(variant.selections), variant])
	);
	let combinations: Array<Array<{ option_name: string; option_value: string }>> = [[]];
	for (const option of matrix) {
		combinations = combinations.flatMap((set) =>
			option.values.map((value) => [...set, { option_name: option.name, option_value: value }])
		);
	}
	return combinations.map((set) => {
		const existing = existingByKey.get(variantSelectionKey(set));
		return createVariant(nextKey, {
			key: existing?.key ?? nextKey("variant"),
			sku:
				existing?.sku ??
				buildVariantSku(
					asTrimmedString(baseSku),
					set.map((item) => item.option_value)
				),
			title: existing?.title ?? set.map((item) => item.option_value).join(" / "),
			price: existing?.price ?? seed.price,
			compare_at_price: existing?.compare_at_price ?? seed.compare_at_price,
			stock: existing?.stock ?? seed.stock,
			is_published: existing?.is_published ?? true,
			selections: set.map((selection, index) =>
				createVariantSelection(nextKey, selection.option_name, selection.option_value, index + 1)
			),
		});
	});
}
