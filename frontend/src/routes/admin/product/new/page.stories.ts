import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub } from "$lib/storybook/api";
import { makeAttributeDefinition } from "$lib/storybook/factories";
import { makeAdminLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import AdminNewProductPage from "./+page.svelte";

type AdminNewProductData = ComponentProps<typeof AdminNewProductPage>["data"];

const meta = {
	title: "Routes/Admin/Product Editor",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function createData(overrides: Partial<AdminNewProductData> = {}): AdminNewProductData {
	return {
		...makeAdminLayoutData(),
		initialProduct: null,
		...overrides,
	};
}

export const NewProduct: Story = {
	render: () =>
		renderRouteStory({
			component: AdminNewProductPage,
			componentProps: { data: createData() },
			api: createApiStub({
				listAdminBrands: async () => [],
				listAdminProductAttributes: async () => [
					makeAttributeDefinition(),
					makeAttributeDefinition({
						id: 2,
						key: "waterproof",
						slug: "waterproof",
						type: "boolean",
					}),
				],
				createAdminProductAttribute: async (input) =>
					makeAttributeDefinition({
						id: 99,
						key: input.key,
						slug: input.slug ?? input.key.toLowerCase().replaceAll(" ", "-"),
						type: input.type,
						filterable: input.filterable ?? false,
						sortable: input.sortable ?? false,
					}),
				updateAdminProductAttribute: async (id, input) =>
					makeAttributeDefinition({
						id,
						key: input.key,
						slug: input.slug ?? input.key.toLowerCase().replaceAll(" ", "-"),
						type: input.type,
						filterable: input.filterable ?? false,
						sortable: input.sortable ?? false,
					}),
				deleteAdminProductAttribute: async () => ({ message: "deleted" }),
			}),
		}),
};
