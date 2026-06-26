import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { makeCategory, makeProduct } from "$lib/storybook/factories";
import { makeRouteLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import CmsPageRoute from "./+page.svelte";

type CmsPageData = ComponentProps<typeof CmsPageRoute>["data"];

const meta = {
	title: "Routes/CMS Page",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function createData(overrides: Partial<CmsPageData> = {}): CmsPageData {
	return {
		...makeRouteLayoutData(),
		draftPreviewActive: false,
		page: {
			localization: null,
			id: 1,
			path: "/shipping",
			title: "Shipping",
			templateKey: "default",
			hasUnpublishedDraft: false,
			seo: null,
			blocks: [
				{
					type: "hero",
					title: "Shipping",
					subtitle: "Clear delivery expectations for every order.",
					primary_cta: { label: "Shop products", url: "/search" },
				},
				{
					type: "rich_text",
					body: "Orders usually leave the studio within two business days. Delivery windows depend on the selected carrier and destination.",
				},
				{
					type: "faq",
					items: [
						{
							question: "Do you ship internationally?",
							answer:
								"International shipping can be enabled per market as carrier support comes online.",
						},
						{
							question: "Can I change my address?",
							answer:
								"Contact support before fulfillment starts so the shipping label can be updated.",
						},
					],
				},
				{
					type: "promo_banner",
					title: "Free domestic shipping over $100",
					body: "Cart totals are checked before checkout payment authorization.",
					link: { label: "Browse new arrivals", url: "/search?sort=created_at" },
				},
				{
					type: "product_rail",
					title: "New arrivals",
					subtitle: "Live catalog products rendered inside CMS content.",
					source: "newest",
					limit: 4,
					sort: "created_at",
					order: "desc",
					image_aspect: "square",
				},
				{
					type: "category_tiles",
					title: "Shop campaign categories",
					subtitle: "Active categories pulled into CMS content.",
					category_slugs: ["bags", "outerwear"],
					image_aspect: "wide",
				},
				{
					type: "promotion_highlight",
					title: "Launch week offer",
					body: "Use the active campaign code at checkout while the collection is featured.",
					badge: "Limited campaign",
					promotion_code: "LAUNCH20",
					link: { label: "Shop the edit", url: "/search" },
				},
				{
					type: "inventory_message",
					product_id: 501,
					low_stock_threshold: 5,
					in_stock_message: "Ready to ship",
					low_stock_message: "Almost sold out",
					out_of_stock_message: "Currently unavailable",
				},
				{
					type: "testimonial",
					quote: "The launch edit made it easy to find the right pieces.",
					attribution: "Early access customer",
					rating: 5,
				},
				{
					type: "social_embed",
					provider: "instagram",
					url: "https://www.instagram.com/p/example/",
					title: "Launch styling reel",
				},
			],
		},
		productRails: {
			"product_rail:4": [
				makeProduct({ id: 501, sku: "cms-rail-1", name: "Canvas Tote" }),
				makeProduct({ id: 502, sku: "cms-rail-2", name: "Everyday Jacket" }),
			],
		},
		categoryTiles: {
			"category_tiles:5": [
				makeCategory({ id: 301, name: "Bags", slug: "bags" }),
				makeCategory({ id: 302, name: "Outerwear", slug: "outerwear" }),
			],
		},
		inventoryProducts: {
			"inventory_message:7": makeProduct({
				id: 501,
				sku: "cms-inventory-1",
				name: "Canvas Tote",
				stock: 3,
			}),
		},
		...overrides,
	};
}

export const Default: Story = {
	render: () =>
		renderRouteStory({
			component: CmsPageRoute,
			componentProps: { data: createData() },
		}),
};

export const DraftPreview: Story = {
	render: () =>
		renderRouteStory({
			component: CmsPageRoute,
			componentProps: {
				data: createData({
					draftPreviewActive: true,
					page: {
						...createData().page,
						hasUnpublishedDraft: true,
					},
				}),
			},
		}),
};

export const EmptyPage: Story = {
	render: () =>
		renderRouteStory({
			component: CmsPageRoute,
			componentProps: {
				data: createData({
					page: {
						localization: null,
						id: 2,
						path: "/returns",
						title: "Returns",
						templateKey: "default",
						hasUnpublishedDraft: false,
						seo: null,
						blocks: [],
					},
				}),
			},
		}),
};
