import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub, pendingPromise } from "$lib/storybook/api";
import {
	makeAttributeDefinition,
	makeDraftPreviewSession,
	makeProduct,
} from "$lib/storybook/factories";
import { makeAdminLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import AdminProductsPage from "./+page.svelte";

type AdminProductsData = ComponentProps<typeof AdminProductsPage>["data"];

const meta = {
	title: "Routes/Admin/Products",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function createData(overrides: Partial<AdminProductsData> = {}): AdminProductsData {
	return {
		...makeAdminLayoutData(),
		products: [],
		productPage: 1,
		productTotalPages: 1,
		productLimit: 10,
		productTotal: 0,
		errorMessage: "",
		...overrides,
	};
}

function createProductsApi() {
	return createApiStub({
		listAdminBrands: async () => [],
		listAdminProductAttributes: async () => [makeAttributeDefinition()],
		getAdminPreviewSession: async () => makeDraftPreviewSession(),
	});
}

export const Loading: Story = {
	render: () =>
		renderRouteStory({
			component: AdminProductsPage,
			componentProps: { data: createData() },
			api: createApiStub({
				listAdminProducts: async () => pendingPromise(),
				listAdminBrands: async () => [],
				listAdminProductAttributes: async () => [makeAttributeDefinition()],
				getAdminPreviewSession: async () => makeDraftPreviewSession(),
			}),
		}),
};

export const Empty: Story = {
	render: () =>
		renderRouteStory({
			component: AdminProductsPage,
			componentProps: { data: createData() },
			api: createProductsApi(),
		}),
};

export const Populated: Story = {
	render: () =>
		renderRouteStory({
			component: AdminProductsPage,
			componentProps: {
				data: createData({
					products: [
						makeProduct({
							id: 101,
							name: "Field Jacket",
							has_draft_changes: true,
							stock: 12,
						}),
						makeProduct({
							id: 102,
							name: "Transit Pack",
							is_published: false,
							stock: 0,
						}),
					],
					productTotal: 2,
				}),
			},
			api: createProductsApi(),
		}),
};

export const LoadError: Story = {
	render: () =>
		renderRouteStory({
			component: AdminProductsPage,
			componentProps: {
				data: createData({
					errorMessage: "Unable to load products.",
				}),
			},
			api: createProductsApi(),
		}),
};
