import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { makeAdminLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import Page from "./+page.svelte";
const meta = {
	title: "Routes/Admin/Search/Typo Profiles",
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
				minimum_token_length: 4,
				one_edit_minimum_length: 4,
				two_edit_minimum_length: 8,
				strict_mode: false,
				is_active: true,
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
