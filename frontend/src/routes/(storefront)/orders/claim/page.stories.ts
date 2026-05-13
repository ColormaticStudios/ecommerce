import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { makeOrder, makeUser } from "$lib/storybook/factories";
import { makeRouteLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import ClaimOrderPage from "./+page.svelte";

type ClaimOrderPageData = ComponentProps<typeof ClaimOrderPage>["data"];

const meta = {
	title: "Routes/Claim Guest Order",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function createData(overrides: Partial<ClaimOrderPageData> = {}): ClaimOrderPageData {
	return {
		...makeRouteLayoutData({ isAuthenticated: true }),
		isAuthenticated: true,
		initialEmail: "guest@example.com",
		initialToken: "claim-token-123",
		...overrides,
	};
}

export const SignedOut: Story = {
	render: () =>
		renderRouteStory({
			component: ClaimOrderPage,
			componentProps: {
				data: createData({
					isAuthenticated: false,
				}),
			},
		}),
};

export const Ready: Story = {
	render: () =>
		renderRouteStory({
			component: ClaimOrderPage,
			componentProps: {
				data: createData(),
			},
			api: {
				claimGuestOrder: async () => ({
					message: "Order claimed.",
					order: makeOrder({
						id: 702,
						guest_email: "guest@example.com",
						confirmation_token: null,
					}),
				}),
			},
			user: makeUser(),
		}),
};
