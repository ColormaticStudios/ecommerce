import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub } from "$lib/storybook/api";
import {
	makeAttributeDefinition,
	makeDraftPreviewSession,
	makeProduct,
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
