import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { makeProduct, makeUser } from "$lib/storybook/factories";
import { makeRouteLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import ProductPage from "./+page.svelte";

type ProductPageData = ComponentProps<typeof ProductPage>["data"];

const baseProduct = makeProduct({
	id: 101,
	related_products: [
		{
			id: 102,
			sku: "story-related",
			name: "Travel Tote",
			description: "A lighter companion piece.",
			price: 78,
			cover_image:
				"https://images.unsplash.com/photo-1542291026-7eec264c27ff?auto=format&fit=crop&w=900&q=80",
			stock: 9,
		},
	],
});

const meta = {
	title: "Routes/Product Detail",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function createData(overrides: Partial<ProductPageData> = {}): ProductPageData {
	return {
		...makeRouteLayoutData(),
		product: baseProduct,
		errorMessage: "",
		...overrides,
	};
}

export const InStock: Story = {
	render: () =>
		renderRouteStory({
			component: ProductPage,
			componentProps: { data: createData() },
		}),
};

export const LowStockAdmin: Story = {
	render: () =>
		renderRouteStory({
			component: ProductPage,
			componentProps: {
				data: createData({
					product: makeProduct({
						id: 103,
						name: "Storm Shell",
						stock: 4,
					}),
				}),
			},
			user: makeUser({ role: "admin" }),
			api: {},
		}),
};

export const OutOfStock: Story = {
	render: () =>
		renderRouteStory({
			component: ProductPage,
			componentProps: {
				data: createData({
					product: makeProduct({
						id: 104,
						name: "Transit Pack",
						stock: 0,
					}),
				}),
			},
		}),
};

export const NotFound: Story = {
	render: () =>
		renderRouteStory({
			component: ProductPage,
			componentProps: {
				data: createData({
					product: null,
					errorMessage: "Product not found.",
				}),
			},
		}),
};
