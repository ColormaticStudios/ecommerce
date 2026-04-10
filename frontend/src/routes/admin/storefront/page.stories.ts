import type { Meta, StoryObj } from "@storybook/sveltekit";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub, pendingPromise } from "$lib/storybook/api";
import { makeDraftPreviewSession, makeStorefrontResponse } from "$lib/storybook/factories";
import { renderRouteStory } from "$lib/storybook/render";
import AdminStorefrontPage from "./+page.svelte";

const draftStorefront = makeStorefrontResponse({
	has_draft_changes: true,
	draft_updated_at: new Date("2026-04-07T11:30:00.000Z"),
});

const meta = {
	title: "Routes/Admin/Storefront",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

export const Loading: Story = {
	render: () =>
		renderRouteStory({
			component: AdminStorefrontPage,
			api: createApiStub({
				getAdminStorefrontSettings: async () => pendingPromise(),
				getAdminPreviewSession: async () => pendingPromise(),
			}),
		}),
};

export const LoadedDraft: Story = {
	render: () =>
		renderRouteStory({
			component: AdminStorefrontPage,
			api: createApiStub({
				getAdminStorefrontSettings: async () => draftStorefront,
				getAdminPreviewSession: async () =>
					makeDraftPreviewSession({
						active: true,
						expires_at: new Date("2026-04-07T12:45:00.000Z"),
					}),
			}),
		}),
};

export const LoadError: Story = {
	render: () =>
		renderRouteStory({
			component: AdminStorefrontPage,
			api: createApiStub({
				getAdminStorefrontSettings: async () => {
					throw new Error("storefront load failed");
				},
				getAdminPreviewSession: async () => makeDraftPreviewSession(),
			}),
		}),
};
