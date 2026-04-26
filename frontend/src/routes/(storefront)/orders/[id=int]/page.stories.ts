import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { makeOrder, makeShipment, makeTrackingEvent, makeUser } from "$lib/storybook/factories";
import { makeRouteLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import OrderDetailPage from "./+page.svelte";

type OrderDetailPageData = ComponentProps<typeof OrderDetailPage>["data"];

const meta = {
	title: "Routes/Order Detail",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function createData(overrides: Partial<OrderDetailPageData> = {}): OrderDetailPageData {
	return {
		...makeRouteLayoutData({ isAuthenticated: true }),
		isAuthenticated: true,
		order: makeOrder(),
		shipments: [],
		errorMessage: "",
		trackingErrorMessage: "",
		...overrides,
	};
}

export const SignedOut: Story = {
	render: () =>
		renderRouteStory({
			component: OrderDetailPage,
			componentProps: {
				data: createData({
					isAuthenticated: false,
					order: null,
				}),
			},
		}),
};

export const AwaitingShipment: Story = {
	render: () =>
		renderRouteStory({
			component: OrderDetailPage,
			componentProps: {
				data: createData({
					order: makeOrder({
						id: 611,
						status: "PAID",
						can_cancel: true,
					}),
				}),
			},
			user: makeUser(),
		}),
};

export const InTransit: Story = {
	render: () =>
		renderRouteStory({
			component: OrderDetailPage,
			componentProps: {
				data: createData({
					order: makeOrder({
						id: 612,
						status: "SHIPPED",
						can_cancel: false,
					}),
					shipments: [
						makeShipment({
							id: 861,
							order_id: 612,
							status: "IN_TRANSIT",
							tracking_events: [
								makeTrackingEvent({
									id: 911,
									status: "IN_TRANSIT",
									location: "Oakland, CA",
									description: "Package departed regional facility.",
									occurred_at: new Date("2026-04-07T08:15:00.000Z"),
								}),
								makeTrackingEvent({
									id: 912,
									status: "LABEL_PURCHASED",
									location: "San Francisco, CA",
									description: "Shipping label created.",
									occurred_at: new Date("2026-04-06T17:00:00.000Z"),
								}),
							],
						}),
					],
				}),
			},
			user: makeUser(),
		}),
};

export const Delivered: Story = {
	render: () =>
		renderRouteStory({
			component: OrderDetailPage,
			componentProps: {
				data: createData({
					order: makeOrder({
						id: 613,
						status: "DELIVERED",
						can_cancel: false,
					}),
					shipments: [
						makeShipment({
							id: 862,
							order_id: 613,
							status: "DELIVERED",
							delivered_at: new Date("2026-04-08T18:10:00.000Z"),
							tracking_events: [
								makeTrackingEvent({
									id: 913,
									status: "DELIVERED",
									location: "San Francisco, CA",
									description: "Package delivered at front desk.",
									occurred_at: new Date("2026-04-08T18:10:00.000Z"),
								}),
								makeTrackingEvent({
									id: 914,
									status: "IN_TRANSIT",
									location: "San Francisco, CA",
									description: "Out for delivery.",
									occurred_at: new Date("2026-04-08T09:20:00.000Z"),
								}),
							],
						}),
					],
				}),
			},
			user: makeUser(),
		}),
};

export const TrackingUnavailable: Story = {
	render: () =>
		renderRouteStory({
			component: OrderDetailPage,
			componentProps: {
				data: createData({
					order: makeOrder({
						id: 614,
						status: "SHIPPED",
						can_cancel: false,
					}),
					trackingErrorMessage: "Unable to load shipment tracking.",
				}),
			},
			user: makeUser(),
		}),
};

export const NotFound: Story = {
	render: () =>
		renderRouteStory({
			component: OrderDetailPage,
			componentProps: {
				data: createData({
					order: null,
					errorMessage: "Order not found.",
				}),
			},
			user: makeUser(),
		}),
};
