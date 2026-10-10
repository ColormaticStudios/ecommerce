import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { makeAdminLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import Page from "./+page.svelte";
const meta = {
	title: "Routes/Admin/Search/Analytics",
	component: RouteStoryHarness,
} satisfies Meta;
export default meta;
type Story = StoryObj;
type Data = ComponentProps<typeof Page>["data"];

function data(overrides: Partial<Data> = {}): Data {
	return { ...makeAdminLayoutData(), analytics: null, errorMessage: "", ...overrides };
}
export const Empty: Story = {
	render: () =>
		renderRouteStory({ component: Page, componentProps: { data: data({ analytics: null }) } }),
};
export const LoadFailure: Story = {
	render: () =>
		renderRouteStory({
			component: Page,
			componentProps: { data: data({ errorMessage: "Unable to load search settings." }) },
		}),
};
const metrics = {
	searches: 120,
	unique_sessions: 80,
	zero_results: 8,
	clicks: 65,
	add_to_carts: 18,
	paid_orders: 7,
	ctr: 0.54,
	conversion_rate: 0.058,
	zero_result_rate: 0.067,
	ctr_at_5: 0.45,
	ctr_at_10: 0.52,
	conversion_at_5: 0.05,
	conversion_at_10: 0.058,
	average_latency_ms: 42,
	p95_latency_ms: 88,
	p99_latency_ms: 120,
};
export const Activity: Story = {
	render: () =>
		renderRouteStory({
			component: Page,
			componentProps: {
				data: data({
					analytics: {
						...metrics,
						days: 7,
						queries: [{ ...metrics, query: "jacket" }],
						popular: ["jacket"],
						trending: ["storm shell"],
						facet_usage: { brand: 20, stock: 32 },
					},
				}),
			},
		}),
};
