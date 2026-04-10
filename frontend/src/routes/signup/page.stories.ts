import type { Meta, StoryObj } from "@storybook/sveltekit";
import { expect, userEvent, within } from "storybook/test";
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

export const PasswordMismatch: Story = {
	render: Default.render,
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.type(canvas.getByPlaceholderText("Username"), "new-user");
		await userEvent.type(canvas.getByPlaceholderText("Email"), "new@example.com");
		await userEvent.type(canvas.getByPlaceholderText("Password"), "first-password");
		await userEvent.type(canvas.getByPlaceholderText("Confirm Password"), "second-password");
		await userEvent.click(canvas.getByRole("button", { name: "Create Account" }));
		await expect(canvas.getByText("Passwords do not match.")).toBeVisible();
	},
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
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.type(canvas.getByPlaceholderText("Username"), "existing-user");
		await userEvent.type(canvas.getByPlaceholderText("Email"), "existing@example.com");
		await userEvent.type(canvas.getByPlaceholderText("Password"), "password-123");
		await userEvent.type(canvas.getByPlaceholderText("Confirm Password"), "password-123");
		await userEvent.click(canvas.getByRole("button", { name: "Create Account" }));
		await expect(canvas.getByText("That email address is already registered.")).toBeVisible();
	},
};
