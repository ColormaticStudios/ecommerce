import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { makeAdminLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import Page from "./+page.svelte";
const meta = {
	title: "Routes/Admin/Search/Ranking Profiles",
	component: RouteStoryHarness,
} satisfies Meta;
export default meta;
type Story = StoryObj;
type Data = ComponentProps<typeof Page>["data"];
const now = "2026-10-07T12:00:00Z";
function data(overrides: Partial<Data> = {}): Data {
	return {
		...makeAdminLayoutData(),
		items: [
			{
				id: 1,
				name: "Default",
				weights: {
					token_coverage: 100,
					exact_phrase: 50,
					name: 30,
					brand: 10,
					attributes: 5,
					recency: 2,
					availability: 2,
					sales: 1,
					margin: 0,
					conversion: 0,
				},
				is_default: true,
				version: 1,
				updated_by: null,
				created_at: now,
				updated_at: now,
			},
		],
		errorMessage: "",
		...overrides,
	};
}
export const Empty: Story = {
	render: () =>
		renderRouteStory({ component: Page, componentProps: { data: data({ items: [] }) } }),
};
export const LoadFailure: Story = {
	render: () =>
		renderRouteStory({
			component: Page,
			componentProps: { data: data({ errorMessage: "Unable to load search settings." }) },
		}),
};
export const Configured: Story = {
	render: () => renderRouteStory({ component: Page, componentProps: { data: data() } }),
};

export const BusinessSignalsEnabled: Story = {
	render: () => {
		const configured = data();
		configured.items[0].weights.margin = 5;
		configured.items[0].weights.conversion = 10;
		return renderRouteStory({ component: Page, componentProps: { data: configured } });
	},
};
