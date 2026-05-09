import type { Meta, StoryObj } from "@storybook/sveltekit";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub, pendingPromise } from "$lib/storybook/api";
import { makeCategory } from "$lib/storybook/factories";
import { renderRouteStory } from "$lib/storybook/render";
import AdminCategoriesPage from "./+page.svelte";

const categories = [
	makeCategory(),
	makeCategory({
		id: 2,
		name: "Outerwear",
		slug: "outerwear",
		parent_id: 1,
		path: "/apparel/outerwear",
		depth: 1,
		sort_order: 10,
	}),
	makeCategory({
		id: 3,
		name: "Archive",
		slug: "archive",
		is_active: false,
		sort_order: 20,
	}),
];

const meta = {
	title: "Routes/Admin/Categories",
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
			component: AdminCategoriesPage,
			api: createApiStub({
				listAdminCategories: async () => pendingPromise(),
			}),
		}),
};

export const Empty: Story = {
	render: () =>
		renderRouteStory({
			component: AdminCategoriesPage,
			api: createApiStub({
				listAdminCategories: async () => [],
			}),
		}),
};

export const Populated: Story = {
	render: () =>
		renderRouteStory({
			component: AdminCategoriesPage,
			api: createApiStub({
				listAdminCategories: async () => categories,
			}),
		}),
};

export const LoadError: Story = {
	render: () =>
		renderRouteStory({
			component: AdminCategoriesPage,
			api: createApiStub({
				listAdminCategories: async () => {
					throw new Error("categories load failed");
				},
			}),
		}),
};
