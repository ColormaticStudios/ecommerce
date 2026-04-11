import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub, pendingPromise } from "$lib/storybook/api";
import { makeOrder, makeProduct, makeUser } from "$lib/storybook/factories";
import { makeAdminLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import AdminOrdersPage from "./+page.svelte";

type AdminOrdersData = ComponentProps<typeof AdminOrdersPage>["data"];

const customer = makeUser({ id: 22, username: "buyer", name: "Checkout Buyer" });

const meta = {
	title: "Routes/Admin/Orders",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function createData(overrides: Partial<AdminOrdersData> = {}): AdminOrdersData {
	return {
		...makeAdminLayoutData(),
		orders: [],
		orderPage: 1,
		orderTotalPages: 1,
		orderLimit: 10,
		orderTotal: 0,
		errorMessage: "",
		...overrides,
	};
}

export const Loading: Story = {
	render: () =>
		renderRouteStory({
			component: AdminOrdersPage,
			componentProps: { data: createData() },
			api: createApiStub({
				listAdminOrders: async () => pendingPromise(),
				listUsers: async () => ({
					data: [],
					pagination: { page: 1, limit: 100, total_pages: 1, total: 0 },
				}),
			}),
		}),
};

export const Empty: Story = {
	render: () =>
		renderRouteStory({
			component: AdminOrdersPage,
			componentProps: { data: createData() },
			api: createApiStub({
				listUsers: async () => ({
					data: [],
					pagination: { page: 1, limit: 100, total_pages: 1, total: 0 },
				}),
			}),
		}),
};

export const Populated: Story = {
	render: () =>
		renderRouteStory({
			component: AdminOrdersPage,
			componentProps: {
				data: createData({
					orders: [
						makeOrder({
							id: 501,
							user_id: customer.id,
							status: "PENDING",
						}),
						makeOrder({
							id: 502,
							user_id: null,
							guest_email: "guest@example.com",
							status: "FAILED",
							items: [
								{
									...makeOrder().items[0],
									product: makeProduct({ id: 104, name: "Transit Pack" }),
								},
							],
						}),
					],
					orderTotal: 2,
				}),
			},
			api: createApiStub({
				listUsers: async () => ({
					data: [customer],
					pagination: { page: 1, limit: 100, total_pages: 1, total: 1 },
				}),
			}),
		}),
};

export const LoadError: Story = {
	render: () =>
		renderRouteStory({
			component: AdminOrdersPage,
			componentProps: {
				data: createData({
					errorMessage: "Unable to load orders.",
				}),
			},
			api: createApiStub({
				listUsers: async () => ({
					data: [],
					pagination: { page: 1, limit: 100, total_pages: 1, total: 0 },
				}),
			}),
		}),
};
