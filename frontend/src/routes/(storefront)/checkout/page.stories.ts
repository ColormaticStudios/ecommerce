import type { Meta, StoryObj } from "@storybook/sveltekit";
import { expect, userEvent, within } from "storybook/test";
import type { ComponentProps } from "svelte";
import { createApiStub } from "$lib/storybook/api";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import {
	makeCart,
	makeCheckoutCatalog,
	makeCheckoutQuote,
	makeOrder,
	makeSavedAddress,
	makeSavedPaymentMethod,
	makeUser,
} from "$lib/storybook/factories";
import { makeRouteLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import CheckoutPage from "./+page.svelte";

type CheckoutPageData = ComponentProps<typeof CheckoutPage>["data"];

const meta = {
	title: "Routes/Checkout",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function createData(overrides: Partial<CheckoutPageData> = {}): CheckoutPageData {
	return {
		...makeRouteLayoutData(),
		isAuthenticated: false,
		cart: null,
		plugins: null,
		savedPaymentMethods: [],
		savedAddresses: [],
		errorMessage: "",
		guestCheckoutDisabled: false,
		...overrides,
	};
}

export const SignInRequired: Story = {
	render: () =>
		renderRouteStory({
			component: CheckoutPage,
			componentProps: {
				data: createData({
					guestCheckoutDisabled: true,
				}),
			},
		}),
};

export const EmptyCart: Story = {
	render: () =>
		renderRouteStory({
			component: CheckoutPage,
			componentProps: { data: createData() },
		}),
};

export const GuestReady: Story = {
	render: () =>
		renderRouteStory({
			component: CheckoutPage,
			componentProps: {
				data: createData({
					cart: makeCart(),
					plugins: makeCheckoutCatalog(),
				}),
			},
		}),
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByRole("link", { name: "Claim past guest order" })).toBeVisible();
	},
};

export const CustomerWithSavedData: Story = {
	render: () =>
		renderRouteStory({
			component: CheckoutPage,
			componentProps: {
				data: createData({
					isAuthenticated: true,
					cart: makeCart(),
					plugins: makeCheckoutCatalog({
						payment: [makeCheckoutCatalog().payment[0]],
						shipping: [makeCheckoutCatalog().shipping[0]],
					}),
					savedPaymentMethods: [makeSavedPaymentMethod()],
					savedAddresses: [makeSavedAddress()],
				}),
			},
			user: makeUser(),
		}),
};

export const ShippingSelectionRequired: Story = {
	render: () =>
		renderRouteStory({
			component: CheckoutPage,
			componentProps: {
				data: createData({
					cart: makeCart(),
					plugins: makeCheckoutCatalog({
						payment: [makeCheckoutCatalog().payment[0]],
					}),
				}),
			},
		}),
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(canvas.getByRole("button", { name: "Load shipping" }));
		await expect(
			canvas.getByText("Choose a shipping method to see delivery options.")
		).toBeVisible();
	},
};

export const ShippingOptionsLoaded: Story = {
	render: () => {
		let currentService = "standard";

		return renderRouteStory({
			component: CheckoutPage,
			componentProps: {
				data: createData({
					isAuthenticated: true,
					cart: makeCart(),
					plugins: makeCheckoutCatalog({
						payment: [makeCheckoutCatalog().payment[0]],
						shipping: [makeCheckoutCatalog().shipping[0]],
					}),
				}),
			},
			api: createApiStub({
				quoteCheckout: async (request) => {
					currentService =
						request.shipping_data?.service_level === "express" ? "express" : "standard";

					return makeCheckoutQuote(
						currentService === "express"
							? {
									snapshot_id: 902,
									shipping: 24,
									tax: 11.28,
									total: 164.28,
								}
							: {}
					);
				},
				createOrder: async () =>
					makeOrder({
						id: 640,
						payment_method_display: "",
						shipping_address_pretty: "",
					}),
				quoteOrderShippingRates: async (_orderId, body) => ({
					order_id: 640,
					snapshot_id: body.snapshot_id,
					provider: "dummy-ground",
					rates: [
						{
							id: 801,
							provider: "dummy-ground",
							provider_rate_id: "rate_standard",
							service_code: "standard",
							service_name: "Standard",
							amount: 12,
							currency: "USD",
							selected: currentService === "standard",
							shipment_id: null,
							expires_at: null,
						},
						{
							id: 802,
							provider: "dummy-ground",
							provider_rate_id: "rate_express",
							service_code: "express",
							service_name: "Express",
							amount: 24,
							currency: "USD",
							selected: currentService === "express",
							shipment_id: null,
							expires_at: null,
						},
					],
				}),
			}),
			user: makeUser(),
		});
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(canvas.getByRole("button", { name: "Load shipping" }));
		await expect(canvas.getByRole("button", { name: /Standard/ })).toBeVisible();
		await expect(canvas.getByRole("button", { name: /Express/ })).toBeVisible();
		await userEvent.click(canvas.getByRole("button", { name: /Express/ }));
		await expect(canvas.getAllByText("$24.00")[0]).toBeVisible();
	},
};

export const LoadError: Story = {
	render: () =>
		renderRouteStory({
			component: CheckoutPage,
			componentProps: {
				data: createData({
					errorMessage: "Unable to load your checkout data.",
				}),
			},
		}),
};
