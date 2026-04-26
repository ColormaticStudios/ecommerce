import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { makeOrder, makeProduct, makeUser } from "$lib/storybook/factories";
import { makeRouteLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import OrdersPage from "./+page.svelte";

type OrdersPageData = ComponentProps<typeof OrdersPage>["data"];

const meta = {
	title: "Routes/Orders",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function createData(overrides: Partial<OrdersPageData> = {}): OrdersPageData {
	return {
		...makeRouteLayoutData({ isAuthenticated: true }),
		isAuthenticated: true,
		orders: [],
		totalPages: 1,
		totalOrders: 0,
		page: 1,
		limit: "10",
		statusFilter: "",
		startDate: "",
		endDate: "",
		errorMessage: "",
		...overrides,
	};
}

export const SignedOut: Story = {
	render: () =>
		renderRouteStory({
			component: OrdersPage,
			componentProps: {
				data: createData({
					isAuthenticated: false,
				}),
			},
		}),
};

export const Empty: Story = {
	render: () =>
		renderRouteStory({
			component: OrdersPage,
			componentProps: { data: createData() },
			user: makeUser(),
		}),
};

export const Populated: Story = {
	render: () =>
		renderRouteStory({
			component: OrdersPage,
			componentProps: {
				data: createData({
					orders: [
						makeOrder({
							id: 501,
							status: "PENDING",
						}),
						makeOrder({
							id: 502,
							status: "DELIVERED",
							can_cancel: false,
							items: [
								{
									...makeOrder().items[0],
									id: 402,
									product: makeProduct({ id: 105, name: "Travel Tote", images: [] }),
								},
							],
						}),
					],
					totalOrders: 2,
				}),
			},
			user: makeUser(),
		}),
};

export const LoadError: Story = {
	render: () =>
		renderRouteStory({
			component: OrdersPage,
			componentProps: {
				data: createData({
					errorMessage: "Unable to load orders.",
				}),
			},
			user: makeUser(),
		}),
};
