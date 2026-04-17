import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub } from "$lib/storybook/api";
import { makeAuthResponse, makeUser } from "$lib/storybook/factories";
import { makeRouteLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import LoginPage from "./+page.svelte";

type LoginPageData = ComponentProps<typeof LoginPage>["data"];

const meta = {
	title: "Routes/Login",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function createData(overrides: Partial<LoginPageData> = {}): LoginPageData {
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
			component: LoginPage,
			componentProps: {
				data: createData(),
			},
			api: createApiStub({
				login: async () => makeAuthResponse(),
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
			component: LoginPage,
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
			component: LoginPage,
			componentProps: {
				data: createData({
					authConfig: {
						local_sign_in_enabled: true,
						oidc_enabled: false,
					},
				}),
			},
			api: createApiStub({
				login: async () => makeAuthResponse(),
				getProfile: async () => makeUser(),
			}),
		}),
};

export const AuthUnavailable: Story = {
	render: () =>
		renderRouteStory({
			component: LoginPage,
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

export const ReauthenticationRequired: Story = {
	render: Default.render,
	parameters: {
		sveltekit_experimental: {
			state: {
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
			componentProps: {
				data: createData(),
			},
			api: createApiStub({
				login: async () => {
					throw { body: { error: "Invalid email or password." } };
				},
			}),
		}),
};
