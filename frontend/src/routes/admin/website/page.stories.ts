import type { Meta, StoryObj } from "@storybook/sveltekit";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub, pendingPromise } from "$lib/storybook/api";
import { makeWebsiteSettingsResponse } from "$lib/storybook/factories";
import { renderRouteStory } from "$lib/storybook/render";
import AdminWebsitePage from "./+page.svelte";

const meta = {
	title: "Routes/Admin/Website",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

export const Loading: Story = {
	render: () =>
		renderRouteStory({
			component: AdminWebsitePage,
			api: createApiStub({
				getAdminWebsiteSettings: async () => pendingPromise(),
			}),
		}),
};

export const Configured: Story = {
	render: () =>
		renderRouteStory({
			component: AdminWebsitePage,
			api: createApiStub({
				getAdminWebsiteSettings: async () => makeWebsiteSettingsResponse(),
			}),
		}),
};

export const LoadError: Story = {
	render: () =>
		renderRouteStory({
			component: AdminWebsitePage,
			api: createApiStub({
				getAdminWebsiteSettings: async () => {
					throw new Error("website settings load failed");
				},
			}),
		}),
};
