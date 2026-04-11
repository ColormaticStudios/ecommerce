import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub, pendingPromise } from "$lib/storybook/api";
import { makeUser } from "$lib/storybook/factories";
import { makeAdminLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import AdminUsersPage from "./+page.svelte";

type AdminUsersData = ComponentProps<typeof AdminUsersPage>["data"];

const meta = {
	title: "Routes/Admin/Users",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function createData(overrides: Partial<AdminUsersData> = {}): AdminUsersData {
	return {
		...makeAdminLayoutData(),
		users: [],
		userPage: 1,
		userTotalPages: 1,
		userLimit: 10,
		userTotal: 0,
		errorMessage: "",
		...overrides,
	};
}

export const Loading: Story = {
	render: () =>
		renderRouteStory({
			component: AdminUsersPage,
			componentProps: { data: createData() },
			api: createApiStub({
				listUsers: async () => pendingPromise(),
			}),
		}),
};

export const Empty: Story = {
	render: () =>
		renderRouteStory({
			component: AdminUsersPage,
			componentProps: { data: createData() },
		}),
};

export const Populated: Story = {
	render: () =>
		renderRouteStory({
			component: AdminUsersPage,
			componentProps: {
				data: createData({
					users: [
						makeUser({ id: 1, role: "admin", username: "owner" }),
						makeUser({
							id: 2,
							username: "buyer",
							deleted_at: new Date("2026-03-20T10:00:00.000Z"),
						}),
					],
					userTotal: 2,
				}),
			},
		}),
};

export const LoadError: Story = {
	render: () =>
		renderRouteStory({
			component: AdminUsersPage,
			componentProps: {
				data: createData({
					errorMessage: "Unable to load users.",
				}),
			},
		}),
};
