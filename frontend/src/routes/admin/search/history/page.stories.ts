import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { makeAdminLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import Page from "./+page.svelte";
const meta = { title: "Routes/Admin/Search/History", component: RouteStoryHarness } satisfies Meta;
export default meta;
type Story = StoryObj;
type Data = ComponentProps<typeof Page>["data"];
const data = (overrides: Partial<Data> = {}): Data => ({
	...makeAdminLayoutData(),
	query: { page: 1, limit: 20 },
	history: null,
	errorMessage: "",
	...overrides,
});
export const Empty: Story = {
	render: () => renderRouteStory({ component: Page, componentProps: { data: data() } }),
};
export const LoadFailure: Story = {
	render: () =>
		renderRouteStory({
			component: Page,
			componentProps: {
				data: data({ errorMessage: "Unable to load search configuration history." }),
			},
		}),
};
export const NamedChangesAndPagination: Story = {
	render: () =>
		renderRouteStory({
			component: Page,
			componentProps: {
				data: data({
					history: {
						items: [
							{
								id: 1,
								kind: "synonym",
								entity_id: 9,
								operation: "update",
								actor_id: 1,
								before: { name: "Autumn outerwear", terms: ["coat", "jacket"] },
								after: { name: "Autumn outerwear", terms: ["coat", "jacket", "shell"] },
								created_at: "2026-10-10T12:00:00Z",
							},
							{
								id: 2,
								kind: "ranking_profile",
								entity_id: 2,
								operation: "delete",
								actor_id: 1,
								before: { name: "Seasonal new arrivals", is_active: true },
								after: null,
								created_at: "2026-10-10T11:00:00Z",
							},
						],
						pagination: { page: 1, limit: 20, total: 45, total_pages: 3 },
					},
				}),
			},
		}),
};
