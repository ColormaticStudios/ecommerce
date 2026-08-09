import { describe, expect, it } from "vitest";
import type { CmsPageResponsePayload } from "./cms";
import { decodeCmsContentBlocks, parseCmsPage } from "./cms";

const validBlocks = [
	{
		type: "hero",
		title: "New arrivals",
		subtitle: "For summer",
		primary_cta: { label: "Shop", url: "/products" },
	},
	{ type: "rich_text", body: "Welcome" },
	{ type: "image", media_id: "media-1", alt: "A product" },
	{ type: "gallery", images: [{ media_id: "media-2", caption: "Front" }] },
	{ type: "video", url: "https://example.test/video", title: "Demo" },
	{ type: "faq", items: [{ question: "When?", answer: "Now." }] },
	{ type: "cta", label: "Buy", url: "/checkout", body: "Limited stock" },
	{ type: "promo_banner", title: "Sale", link: { label: "View", url: "/sale" } },
	{
		type: "product_rail",
		title: "Popular",
		source: "manual",
		product_ids: [1, 2],
		sort: "price",
		order: "asc",
		limit: 8,
		image_aspect: "square",
	},
	{
		type: "category_tiles",
		title: "Categories",
		category_slugs: ["shoes"],
		category_media_ids: { shoes: "media-3" },
	},
	{
		type: "promotion_highlight",
		title: "Member offer",
		campaign_id: 12,
		link: { label: "Join", url: "/join" },
	},
	{ type: "inventory_message", product_id: 42, low_stock_threshold: 3 },
	{ type: "testimonial", quote: "Excellent", attribution: "A customer", rating: 5 },
	{ type: "social_embed", provider: "youtube", url: "https://youtube.example/video" },
	{
		type: "footer",
		brand_name: "Store",
		columns: [{ title: "Help", links: [{ label: "Contact", url: "/contact" }] }],
		social_links: [{ label: "Social", url: "https://example.test" }],
		copyright: "2026 Store",
		layout: "columns",
	},
	{ type: "custom_html", html: "<p>Trusted server content</p>" },
];

describe("decodeCmsContentBlocks", () => {
	it("accepts every supported structurally valid block type", () => {
		const result = decodeCmsContentBlocks(validBlocks);

		expect(result.blocks).toEqual(validBlocks);
		expect(result.rejectedBlocks).toEqual([]);
	});

	it("keeps valid blocks while reporting unsupported and malformed entries", () => {
		const malformedGallery = {
			type: "gallery",
			images: [{ media_id: 123, alt: "Wrong ID type" }],
		};
		const unsupported = { type: "weather", location: "outside" };
		const result = decodeCmsContentBlocks([validBlocks[0], malformedGallery, unsupported, null]);

		expect(result.blocks).toEqual([validBlocks[0]]);
		expect(result.rejectedBlocks.map(({ index }) => index)).toEqual([1, 2, 3]);
		expect(result.rejectedBlocks[0]?.reason).toContain("gallery");
		expect(result.rejectedBlocks[1]?.reason).toContain("weather");
		expect(result.rejectedBlocks[2]?.value).toBeNull();
	});

	it("rejects a non-array block container", () => {
		const result = decodeCmsContentBlocks({ type: "rich_text", body: "not an array" });

		expect(result.blocks).toEqual([]);
		expect(result.rejectedBlocks).toEqual([
			{
				index: -1,
				reason: "CMS blocks must be an array",
				value: { type: "rich_text", body: "not an array" },
			},
		]);
	});
});

describe("parseCmsPage", () => {
	it("returns only decoded blocks and exposes rejected block details", () => {
		const response = {
			page: {
				id: 7,
				entry_id: 9,
				path: "/",
				slug: "home",
				title: "Home",
				template_key: "default",
				visibility: "public",
				is_homepage: true,
				created_at: "2026-01-01T00:00:00Z",
				updated_at: "2026-01-01T00:00:00Z",
			},
			entry: {
				id: 9,
				entry_type: "page",
				key: "home",
				status: "PUBLISHED",
				created_at: "2026-01-01T00:00:00Z",
				updated_at: "2026-01-01T00:00:00Z",
			},
			published_version: {
				id: 10,
				entry_id: 9,
				version_number: 1,
				schema_version: 1,
				payload: { blocks: [validBlocks[1], { type: "image", media_id: false }] },
				created_at: "2026-01-01T00:00:00Z",
			},
			has_unpublished_draft: false,
		} as CmsPageResponsePayload;

		const page = parseCmsPage(response);

		expect(page.blocks).toEqual([validBlocks[1]]);
		expect(page.rejectedBlocks).toHaveLength(1);
		expect(page.rejectedBlocks?.[0]?.index).toBe(1);
	});
});
