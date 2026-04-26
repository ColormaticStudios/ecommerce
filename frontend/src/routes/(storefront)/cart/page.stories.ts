import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { makeCart, makeCartItem, makeProduct, makeUser } from "$lib/storybook/factories";
import { makeRouteLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import CartPage from "./+page.svelte";

type CartPageData = ComponentProps<typeof CartPage>["data"];

const cart = makeCart({
	items: [
		makeCartItem({
			id: 301,
			quantity: 2,
			product: makeProduct({ id: 101, name: "Field Jacket", price: 129 }),
		}),
		makeCartItem({
			id: 302,
			product_variant_id: 12,
			product_variant: {
				...makeProduct({ id: 102, name: "Travel Tote", price: 58 }).variants[0],
				id: 12,
				sku: "travel-tote-default",
				title: "Standard",
				price: 58,
			},
			product: makeProduct({ id: 102, name: "Travel Tote", price: 58, images: [] }),
			quantity: 1,
		}),
	],
});

const meta = {
	title: "Routes/Cart",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function createData(overrides: Partial<CartPageData> = {}): CartPageData {
	return {
		...makeRouteLayoutData(),
		isAuthenticated: false,
		cart: null,
		errorMessage: "",
		guestCheckoutDisabled: false,
		...overrides,
	};
}

export const SignInRequired: Story = {
	render: () =>
		renderRouteStory({
			component: CartPage,
			componentProps: {
				data: createData({
					guestCheckoutDisabled: true,
				}),
			},
		}),
};

export const EmptyGuest: Story = {
	render: () =>
		renderRouteStory({
			component: CartPage,
			componentProps: { data: createData() },
		}),
};

export const EmptyCustomer: Story = {
	render: () =>
		renderRouteStory({
			component: CartPage,
			componentProps: {
				data: createData({
					isAuthenticated: true,
				}),
			},
			user: makeUser(),
		}),
};

export const WithItems: Story = {
	render: () =>
		renderRouteStory({
			component: CartPage,
			componentProps: {
				data: createData({
					cart,
				}),
			},
		}),
};

export const LoadError: Story = {
	render: () =>
		renderRouteStory({
			component: CartPage,
			componentProps: {
				data: createData({
					errorMessage: "Unable to load your cart.",
				}),
			},
		}),
};
