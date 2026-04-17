import type { Meta, StoryObj } from "@storybook/sveltekit";
import { expect, within } from "storybook/test";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import {
	makeCart,
	makeCheckoutCatalog,
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
