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
	return createApiStub({
		listAdminBrands: async () => [],
		listAdminProductAttributes: async () => [makeAttributeDefinition()],
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
