import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub } from "$lib/storybook/api";
import {
	makeAttributeDefinition,
	makeDraftPreviewSession,
	makeProduct,
	makeVariant,
} from "$lib/storybook/factories";
import { makeAdminLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import AdminProductPage from "./+page.svelte";

type AdminProductData = ComponentProps<typeof AdminProductPage>["data"];

const publishedProduct = makeProduct({
	id: 101,
	name: "Field Jacket",
	is_published: true,
	attributes: [
		{
			product_attribute_id: 1,
			key: "material",
			slug: "material",
			type: "text",
			text_value: "Waxed cotton",
			number_value: null,
			boolean_value: null,
			enum_value: null,
			position: 1,
		},
		{
			product_attribute_id: 2,
			key: "fit",
			slug: "fit",
			type: "enum",
			text_value: null,
			number_value: null,
			boolean_value: null,
			enum_value: "Regular",
			position: 2,
		},
	],
});

const unpublishedProduct = makeProduct({
	id: 102,
	name: "Field Jacket Draft",
	is_published: false,
});

const productWithDraftMatrix = makeProduct({
	id: 103,
	name: "Field Jacket — Expanded Range",
	has_draft_changes: true,
	default_variant_id: 31,
	default_variant_sku: "FIELD-JACKET-NAVY-S",
	price_range: { min: 129, max: 149 },
	options: [
		{
			id: 21,
			name: "Color",
			position: 1,
			display_type: "swatch",
			values: [
				{ id: 211, value: "Navy", position: 1 },
				{ id: 212, value: "Ochre", position: 2 },
			],
		},
		{
			id: 22,
			name: "Size",
			position: 2,
			display_type: "select",
			values: [
				{ id: 221, value: "S", position: 1 },
				{ id: 222, value: "M", position: 2 },
			],
		},
	],
	variants: [
		makeVariant({
			id: 31,
			sku: "FIELD-JACKET-NAVY-S",
			title: "Navy / S",
			price: 129,
			selections: [
				{ product_option_value_id: 211, option_name: "Color", option_value: "Navy", position: 1 },
				{ product_option_value_id: 221, option_name: "Size", option_value: "S", position: 2 },
			],
		}),
		makeVariant({
			id: 32,
			sku: "FIELD-JACKET-OCHRE-M",
			title: "Ochre / M",
			price: 149,
			stock: 0,
			is_published: false,
			selections: [
				{ product_option_value_id: 212, option_name: "Color", option_value: "Ochre", position: 1 },
				{ product_option_value_id: 222, option_name: "Size", option_value: "M", position: 2 },
			],
		}),
	],
	seo: {
		title: "Field Jacket | Colormatic",
		description: "A utility jacket available in seasonal colors.",
		canonical_path: "/products/field-jacket",
		og_image_media_id: null,
		noindex: false,
	},
});

const meta = {
	title: "Routes/Admin/Product Editor",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function createData(overrides: Partial<AdminProductData> = {}): AdminProductData {
	return {
		...makeAdminLayoutData(),
		initialProduct: null,
		...overrides,
	};
}

function createEditorApi(product = publishedProduct) {
	const definitions = [
		makeAttributeDefinition(),
		makeAttributeDefinition({
			id: 2,
			key: "fit",
			slug: "fit",
			type: "enum",
			enum_values: ["Regular", "Slim", "Relaxed"],
		}),
		makeAttributeDefinition({
			id: 3,
			key: "weight",
			slug: "weight",
			type: "number",
			filterable: false,
			sortable: true,
		}),
		makeAttributeDefinition({ id: 4, key: "waterproof", slug: "waterproof", type: "boolean" }),
	];

	return createApiStub({
		listAdminBrands: async () => [],
		listAdminProductAttributes: async () => definitions,
		createAdminProductAttribute: async (input) =>
			makeAttributeDefinition({
				id: 99,
				key: input.key,
				slug: input.slug ?? input.key.toLowerCase().replaceAll(" ", "-"),
				type: input.type,
				filterable: input.filterable ?? false,
				sortable: input.sortable ?? false,
				enum_values: input.enum_values ?? [],
			}),
		updateAdminProductAttribute: async (id, input) =>
			makeAttributeDefinition({
				id,
				key: input.key,
				slug: input.slug ?? input.key.toLowerCase().replaceAll(" ", "-"),
				type: input.type,
				filterable: input.filterable ?? false,
				sortable: input.sortable ?? false,
				enum_values: input.enum_values ?? [],
			}),
		deleteAdminProductAttribute: async () => ({ message: "deleted" }),
		getAdminPreviewSession: async () => makeDraftPreviewSession(),
		getAdminProduct: async () => product,
	});
}

export const PublishedProduct: Story = {
	render: () =>
		renderRouteStory({
			component: AdminProductPage,
			componentProps: {
				data: createData({
					initialProduct: publishedProduct,
				}),
			},
			api: createEditorApi(),
		}),
	parameters: {
		sveltekit_experimental: {
			state: {
				page: {
					params: { id: String(publishedProduct.id) },
				},
			},
		},
	},
};

export const DraftChangesWithVariantMatrix: Story = {
	render: () =>
		renderRouteStory({
			component: AdminProductPage,
			componentProps: {
				data: createData({ initialProduct: productWithDraftMatrix }),
			},
			api: createEditorApi(productWithDraftMatrix),
		}),
	parameters: {
		sveltekit_experimental: {
			state: {
				page: { params: { id: String(productWithDraftMatrix.id) } },
			},
		},
	},
};

export const UnpublishedProduct: Story = {
	render: () =>
		renderRouteStory({
			component: AdminProductPage,
			componentProps: {
				data: createData({
					initialProduct: unpublishedProduct,
				}),
			},
			api: createEditorApi(unpublishedProduct),
		}),
	parameters: {
		sveltekit_experimental: {
			state: {
				page: {
					params: { id: String(unpublishedProduct.id) },
				},
			},
		},
	},
};
