import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import { expect, userEvent, within } from "storybook/test";
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
			oidc_display_name: "Colormatic SSO",
			allow_guest_checkout: true,
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

export const LongProviderName: Story = {
	render: () =>
		renderRouteStory({
			component: LoginPage,
			componentProps: {
				data: createData({
					authConfig: {
						local_sign_in_enabled: true,
						oidc_enabled: true,
						oidc_display_name: "Northwest Regional Cooperative Single Sign-On",
						allow_guest_checkout: true,
					},
				}),
			},
			api: createApiStub(),
		}),
};

export const RedirectingToProvider: Story = {
	render: () =>
		renderRouteStory({
			component: LoginPage,
			componentProps: { data: createData() },
			api: createApiStub({ buildOIDCLoginURL: () => "#redirecting" }),
		}),
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(canvas.getByRole("button", { name: "Continue to Colormatic SSO" }));
		await expect(
			canvas.getByRole("button", { name: "Redirecting to Colormatic SSO..." })
		).toBeDisabled();
	},
};

export const OIDCCancelled: Story = {
	render: Default.render,
	parameters: {
		sveltekit_experimental: {
			state: { page: { url: new URL("https://storybook.local/login?reason=oidc_cancelled") } },
		},
	},
};

export const OIDCFailed: Story = {
	render: Default.render,
	parameters: {
		sveltekit_experimental: {
			state: { page: { url: new URL("https://storybook.local/login?reason=oidc_failed") } },
		},
	},
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
						oidc_display_name: "Colormatic SSO",
						allow_guest_checkout: true,
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
						oidc_display_name: "",
						allow_guest_checkout: true,
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
						oidc_display_name: "",
						allow_guest_checkout: true,
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
