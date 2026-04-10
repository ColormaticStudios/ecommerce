import type { Meta, StoryObj } from "@storybook/sveltekit";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub, pendingPromise } from "$lib/storybook/api";
import { makeBrand } from "$lib/storybook/factories";
import { renderRouteStory } from "$lib/storybook/render";
import AdminBrandsPage from "./+page.svelte";

const catalog = [
	makeBrand(),
	makeBrand({ id: 2, name: "Northline", slug: "northline", is_active: false }),
];

const meta = {
	title: "Routes/Admin/Brands",
	component: RouteStoryHarness,
	parameters: {
		backgrounds: { disable: true },
	},
} satisfies Meta;

export default meta;
type Story = StoryObj;

export const Loading: Story = {
	render: () =>
		renderRouteStory({
			component: AdminBrandsPage,
			api: createApiStub({
				listAdminBrands: async () => pendingPromise(),
			}),
		}),
};

export const Empty: Story = {
	render: () =>
		renderRouteStory({
			component: AdminBrandsPage,
			api: createApiStub({
				listAdminBrands: async () => [],
			}),
		}),
};

export const Catalog: Story = {
	render: () =>
		renderRouteStory({
			component: AdminBrandsPage,
			api: createApiStub({
				listAdminBrands: async () => catalog,
			}),
		}),
};
