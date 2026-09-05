import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import { expect, userEvent, within } from "storybook/test";
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
			oidc_display_name: "Colormatic SSO",
			allow_guest_checkout: true,
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

export const RedirectingToProvider: Story = {
	render: () =>
		renderRouteStory({
			component: SignupPage,
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

export const OIDCOnly: Story = {
	render: () =>
		renderRouteStory({
			component: SignupPage,
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
			component: SignupPage,
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
						oidc_display_name: "",
						allow_guest_checkout: true,
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
