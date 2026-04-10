import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub } from "$lib/storybook/api";
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
				listAdminProductAttributes: async () => [],
			}),
		}),
};
