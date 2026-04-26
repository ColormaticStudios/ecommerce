import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub } from "$lib/storybook/api";
import { makeAuthResponse, makeUser } from "$lib/storybook/factories";
import { makeRouteLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import SignupPage from "./+page.svelte";

type SignupPageData = ComponentProps<typeof SignupPage>["data"];

const meta = {
	title: "Routes/Signup",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function createData(overrides: Partial<SignupPageData> = {}): SignupPageData {
	return {
		...makeRouteLayoutData(),
		authConfig: {
			local_sign_in_enabled: true,
			oidc_enabled: true,
		},
		...overrides,
	};
}

export const Default: Story = {
	render: () =>
		renderRouteStory({
			component: SignupPage,
			componentProps: {
				data: createData(),
			},
			api: createApiStub({
				register: async () => makeAuthResponse(),
				getProfile: async () => makeUser(),
			}),
		}),
};

export const OpenIDConnectOption: Story = {
	render: Default.render,
};

export const OIDCOnly: Story = {
	render: () =>
		renderRouteStory({
			component: SignupPage,
			componentProps: {
				data: createData({
					authConfig: {
						local_sign_in_enabled: false,
						oidc_enabled: true,
					},
				}),
			},
			api: createApiStub(),
		}),
};

export const LocalOnly: Story = {
	render: () =>
		renderRouteStory({
			component: SignupPage,
			componentProps: {
				data: createData({
					authConfig: {
						local_sign_in_enabled: true,
						oidc_enabled: false,
					},
				}),
			},
			api: createApiStub({
				register: async () => makeAuthResponse(),
				getProfile: async () => makeUser(),
			}),
		}),
};

export const AuthUnavailable: Story = {
	render: () =>
		renderRouteStory({
			component: SignupPage,
			componentProps: {
				data: createData({
					authConfig: {
						local_sign_in_enabled: false,
						oidc_enabled: false,
					},
				}),
			},
			api: createApiStub(),
		}),
};

export const PasswordMismatch: Story = {
	render: Default.render,
};

export const RegistrationRejected: Story = {
	render: () =>
		renderRouteStory({
			component: SignupPage,
			componentProps: {
				data: createData(),
			},
			api: createApiStub({
				register: async () => {
					throw { body: { error: "That email address is already registered." } };
				},
			}),
		}),
};
