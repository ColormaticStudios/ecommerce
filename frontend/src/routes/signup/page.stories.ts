import type { Meta, StoryObj } from "@storybook/sveltekit";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub } from "$lib/storybook/api";
import { makeAuthResponse, makeUser } from "$lib/storybook/factories";
import { renderRouteStory } from "$lib/storybook/render";
import SignupPage from "./+page.svelte";

const meta = {
	title: "Routes/Signup",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

export const Default: Story = {
	render: () =>
		renderRouteStory({
			component: SignupPage,
			api: createApiStub({
				register: async () => makeAuthResponse(),
				getProfile: async () => makeUser(),
			}),
		}),
};

export const OpenIDConnectOption: Story = {
	render: Default.render,
};

export const PasswordMismatch: Story = {
	render: Default.render,
};

export const RegistrationRejected: Story = {
	render: () =>
		renderRouteStory({
			component: SignupPage,
			api: createApiStub({
				register: async () => {
					throw { body: { error: "That email address is already registered." } };
				},
			}),
		}),
};
