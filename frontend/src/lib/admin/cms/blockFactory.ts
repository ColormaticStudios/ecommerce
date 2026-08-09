import type { CmsContentBlock } from "$lib/cms";
import type { CmsEditableBlock } from "./blocks";

export type CmsGlobalRegionType = "announcement_bar" | "sitewide_banner" | "trust_strip" | "footer";
export type CmsCampaignTemplateKey =
	| "seasonal_sale"
	| "collection_launch"
	| "bundle_spotlight"
	| "new_arrivals";
export type CmsEditorIdFactory = (prefix: string) => string;

export function createCmsBlock(
	type: CmsContentBlock["type"],
	id: CmsEditorIdFactory
): CmsEditableBlock {
	switch (type) {
		case "hero":
			return { editorId: id("hero"), type, title: "Hero title", subtitle: "" };
		case "image":
			return { editorId: id("image"), type, media_id: "", alt: "", caption: "" };
		case "cta":
			return { editorId: id("cta"), type, label: "Learn more", url: "/", body: "" };
		case "promo_banner":
			return {
				editorId: id("promo"),
				type,
				title: "Promotion",
				body: "",
				link: { label: "", url: "" },
			};
		case "product_rail":
			return createCmsProductRail("Featured products", "newest", id);
		case "category_tiles":
			return {
				editorId: id("category-tiles"),
				type,
				title: "Shop by category",
				subtitle: "",
				category_slugs: [],
				category_media_ids: {},
				image_aspect: "square",
			};
		case "promotion_highlight":
			return {
				editorId: id("promotion-highlight"),
				type,
				title: "Promotion",
				body: "",
				badge: "",
				promotion_code: "",
				link: { label: "", url: "" },
			};
		case "inventory_message":
			return {
				editorId: id("inventory-message"),
				type,
				product_id: 1,
				low_stock_threshold: 5,
				in_stock_message: "In stock",
				low_stock_message: "Almost gone",
				out_of_stock_message: "Out of stock",
			};
		case "testimonial":
			return { editorId: id("testimonial"), type, quote: "", attribution: "", rating: 5 };
		case "social_embed":
			return { editorId: id("social-embed"), type, provider: "instagram", url: "", title: "" };
		default:
			return { editorId: id("rich"), type: "rich_text", body: "" };
	}
}

export function createCmsProductRail(
	title: string,
	source: Extract<CmsContentBlock, { type: "product_rail" }>["source"],
	id: CmsEditorIdFactory,
	query = ""
): CmsEditableBlock {
	return {
		editorId: id("product-rail"),
		type: "product_rail",
		title,
		subtitle: "",
		source,
		product_ids: [],
		query,
		category_slug: "",
		sort: "created_at",
		order: "desc",
		limit: 8,
		image_aspect: "square",
	};
}

export function createDefaultPageBlocks(id: CmsEditorIdFactory): CmsEditableBlock[] {
	return [
		{
			editorId: id("hero"),
			type: "hero",
			title: "Shipping",
			subtitle: "Useful storefront page copy.",
		},
		{ editorId: id("rich"), type: "rich_text", body: "Add page content here." },
	];
}

export function createDefaultGlobalBlocks(
	region: CmsGlobalRegionType,
	id: CmsEditorIdFactory
): CmsEditableBlock[] {
	if (region === "footer")
		return [
			{
				editorId: id("footer"),
				type: "footer",
				brand_name: "Store",
				tagline: "Thoughtfully selected products for everyday use.",
				columns: [
					{
						title: "Shop",
						links: [
							{ label: "All products", url: "/search" },
							{ label: "New arrivals", url: "/search?sort=created_at" },
						],
					},
					{
						title: "Help",
						links: [
							{ label: "Shipping", url: "/shipping" },
							{ label: "Returns", url: "/returns" },
						],
					},
				],
				social_links: [],
				copyright: `© ${new Date().getFullYear()} Store`,
				layout: "columns",
			},
		];
	if (region === "trust_strip")
		return [
			{ editorId: id("trust"), type: "rich_text", body: "Free shipping over $100" },
			{ editorId: id("trust"), type: "rich_text", body: "30-day returns" },
		];
	return [
		{
			editorId: id("promo"),
			type: "promo_banner",
			title: "Free domestic shipping over $100",
			body: "Applied automatically.",
			link: { label: "Shop now", url: "/search" },
		},
	];
}

export function createCampaignTemplateBlocks(
	template: CmsCampaignTemplateKey,
	id: CmsEditorIdFactory
): CmsEditableBlock[] {
	const block = (type: CmsContentBlock["type"]) => createCmsBlock(type, id);
	switch (template) {
		case "collection_launch":
			return [
				{ editorId: id("hero"), type: "hero", title: "Collection launch", subtitle: "" },
				{
					editorId: id("category-tiles"),
					type: "category_tiles",
					title: "Explore the collection",
					subtitle: "",
					category_slugs: [],
					image_aspect: "wide",
				},
				createCmsProductRail("Featured products", "newest", id),
				block("testimonial"),
			];
		case "bundle_spotlight":
			return [
				{ editorId: id("hero"), type: "hero", title: "Bundle spotlight", subtitle: "" },
				block("promotion_highlight"),
				createCmsProductRail("Bundle picks", "manual", id),
				block("inventory_message"),
			];
		case "new_arrivals":
			return [
				{ editorId: id("hero"), type: "hero", title: "New arrivals", subtitle: "" },
				createCmsProductRail("Just added", "newest", id),
				block("category_tiles"),
				block("social_embed"),
			];
		default:
			return [
				{ editorId: id("hero"), type: "hero", title: "Seasonal sale", subtitle: "" },
				block("promotion_highlight"),
				createCmsProductRail("Sale picks", "search", id, "sale"),
				block("category_tiles"),
			];
	}
}
