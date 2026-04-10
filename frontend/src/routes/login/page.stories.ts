import type { Meta, StoryObj } from "@storybook/sveltekit";
import { expect, userEvent, within } from "storybook/test";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub } from "$lib/storybook/api";
import { makeAuthResponse, makeUser } from "$lib/storybook/factories";
import { renderRouteStory } from "$lib/storybook/render";
import LoginPage from "./+page.svelte";

const meta = {
	title: "Routes/Login",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

export const Default: Story = {
	render: () =>
		renderRouteStory({
			component: LoginPage,
			api: createApiStub({
				login: async () => makeAuthResponse(),
				getProfile: async () => makeUser(),
			}),
		}),
};

export const ReauthenticationRequired: Story = {
	render: Default.render,
	parameters: {
		sveltekit_experimental: {
			stores: {
				page: {
					url: new URL("https://storybook.local/login?reason=reauth"),
				},
			},
		},
	},
};

export const InvalidCredentials: Story = {
	render: () =>
		renderRouteStory({
			component: LoginPage,
			api: createApiStub({
				login: async () => {
					throw { body: { error: "Invalid email or password." } };
				},
			}),
		}),
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.type(canvas.getByPlaceholderText("Email"), "buyer@example.com");
		await userEvent.type(canvas.getByPlaceholderText("Password"), "wrong-password");
		await userEvent.click(canvas.getByRole("button", { name: "Log In" }));
		await expect(canvas.getByText("Invalid email or password.")).toBeVisible();
	},
};
