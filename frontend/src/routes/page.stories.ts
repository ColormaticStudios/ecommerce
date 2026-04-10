import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { makeProduct, makeStorefrontSettings } from "$lib/storybook/factories";
import { makeRouteLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import HomePage from "./+page.svelte";

type HomePageData = ComponentProps<typeof HomePage>["data"];

const featuredProduct = makeProduct({
	id: 101,
	name: "Field Jacket",
	price: 129,
	stock: 12,
});
const lowStockProduct = makeProduct({
	id: 102,
	name: "Canvas Tote",
	price: 58,
	stock: 3,
	images: [],
	cover_image: undefined,
});
const homeStorefront = makeStorefrontSettings();

const meta = {
	title: "Routes/Home",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function createData(overrides: Partial<HomePageData> = {}): HomePageData {
	return {
		...makeRouteLayoutData(),
		storefront: homeStorefront,
		errorMessage: "",
		homepageSections: [
			{ ...homeStorefront.homepage_sections[0], products: [] },
			{
				...homeStorefront.homepage_sections[1],
				products: [featuredProduct, lowStockProduct],
			},
			{
				id: "promo-1",
				type: "promo_cards",
				enabled: true,
				promo_card_limit: 2,
				promo_cards: [
					{
						kicker: "Maker notes",
						title: "Hardwearing fabrics",
						description: "Built for repeat use instead of one polished product photo.",
						image_url:
							"https://images.unsplash.com/photo-1503342217505-b0a15ec3261c?auto=format&fit=crop&w=900&q=80",
						link: { label: "Read more", url: "/search?q=canvas" },
					},
				],
				products: [],
			},
			{
				id: "badges-1",
				type: "badges",
				enabled: true,
				badges: ["Small batch", "Fast dispatch", "Repairable"],
				products: [],
			},
		],
		...overrides,
	};
}

export const Default: Story = {
	render: () =>
		renderRouteStory({
			component: HomePage,
			componentProps: { data: createData() },
		}),
};

export const ProductSectionEmpty: Story = {
	render: () =>
		renderRouteStory({
			component: HomePage,
			componentProps: {
				data: createData({
					homepageSections: [
						{ ...homeStorefront.homepage_sections[0], products: [] },
						{
							...homeStorefront.homepage_sections[1],
							products: [],
						},
					],
				}),
			},
		}),
};

export const SectionLoadError: Story = {
	render: () =>
		renderRouteStory({
			component: HomePage,
			componentProps: {
				data: createData({
					errorMessage: "Unable to load one or more homepage product sections.",
				}),
			},
		}),
};
