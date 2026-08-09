import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import type { CmsContentBlock } from "$lib/cms";
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

const allBlockFamilies = [
	{
		type: "hero",
		title: "The complete CMS collection",
		subtitle: "Every validated public block rendered through the typed dispatcher.",
		primary_cta: { label: "Explore", url: "/search" },
	},
	{ type: "rich_text", body: "Structured editorial copy keeps the collection grounded." },
	{ type: "image", media_id: "story-image", alt: "CMS collection", caption: "Image block" },
	{
		type: "gallery",
		images: [
			{ media_id: "story-gallery-one", alt: "Gallery item one", caption: "First view" },
			{ media_id: "story-gallery-two", alt: "Gallery item two", caption: "Second view" },
		],
	},
	{ type: "video", url: "https://www.youtube.com/embed/dQw4w9WgXcQ", title: "Collection film" },
	{
		type: "faq",
		items: [{ question: "Is every block public?", answer: "Yes, after validation." }],
	},
	{ type: "cta", label: "Start shopping", url: "/search", body: "Continue into the catalog." },
	{
		type: "promo_banner",
		title: "Public promotion",
		body: "Promotion banner content.",
		link: { label: "View offer", url: "/search" },
	},
	{
		type: "product_rail",
		title: "Featured products",
		source: "manual",
		product_ids: [601],
		limit: 4,
		image_aspect: "square",
	},
	{
		type: "category_tiles",
		title: "Featured categories",
		category_slugs: ["accessories"],
		image_aspect: "wide",
	},
	{
		type: "promotion_highlight",
		title: "Member pricing",
		badge: "Member offer",
		promotion_code: "MEMBER15",
		link: { label: "Learn more", url: "/search" },
	},
	{
		type: "inventory_message",
		product_id: 601,
		low_stock_threshold: 5,
		low_stock_message: "Only a few remain",
	},
	{
		type: "testimonial",
		quote: "Every section feels intentional.",
		attribution: "CMS customer",
		rating: 5,
	},
	{
		type: "social_embed",
		provider: "youtube",
		url: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		title: "Behind the collection",
	},
	{
		type: "footer",
		brand_name: "Colormatic Supply",
		tagline: "A footer block rendered as validated page content.",
		columns: [{ title: "Explore", links: [{ label: "Catalog", url: "/search" }] }],
		social_links: [{ label: "YouTube", url: "https://www.youtube.com" }],
		copyright: "© 2026 Colormatic Supply",
		layout: "columns",
	},
	{
		type: "custom_html",
		html: "<p><strong>Custom HTML</strong> remains isolated in its dedicated renderer.</p>",
	},
] satisfies CmsContentBlock[];

export const Default: Story = {
	render: () =>
		renderRouteStory({
			component: CmsPageRoute,
			componentProps: { data: createData() },
		}),
};

export const AllBlockFamilies: Story = {
	render: () =>
		renderRouteStory({
			component: CmsPageRoute,
			componentProps: {
				data: createData({
					page: {
						...createData().page,
						id: 3,
						path: "/cms-blocks",
						title: "CMS block catalog",
						blocks: allBlockFamilies,
					},
					productRails: {
						"product_rail:8": [
							makeProduct({ id: 601, sku: "cms-all-blocks", name: "Studio Carryall" }),
						],
					},
					categoryTiles: {
						"category_tiles:9": [
							makeCategory({ id: 401, name: "Accessories", slug: "accessories" }),
						],
					},
					inventoryProducts: {
						"inventory_message:11": makeProduct({
							id: 601,
							sku: "cms-all-blocks",
							name: "Studio Carryall",
							stock: 3,
						}),
					},
				}),
			},
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
