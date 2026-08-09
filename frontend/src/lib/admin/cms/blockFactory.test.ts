import { describe, expect, it } from "vitest";
import {
	createCampaignTemplateBlocks,
	createCmsBlock,
	createDefaultGlobalBlocks,
} from "./blockFactory";

function ids() {
	let sequence = 0;
	return (prefix: string) => `${prefix}-${++sequence}`;
}

describe("CMS block factory", () => {
	it("creates every visual library block with a unique editor identity", () => {
		const id = ids();
		const blocks = [
			"hero",
			"rich_text",
			"image",
			"cta",
			"promo_banner",
			"product_rail",
			"category_tiles",
			"promotion_highlight",
			"inventory_message",
			"testimonial",
			"social_embed",
		].map((type) => createCmsBlock(type as Parameters<typeof createCmsBlock>[0], id));
		expect(new Set(blocks.map((block) => block.editorId)).size).toBe(blocks.length);
		expect(blocks.map((block) => block.type)).toContain("product_rail");
	});

	it("builds footer defaults and campaign presets through the same factory boundary", () => {
		const footer = createDefaultGlobalBlocks("footer", ids());
		const campaign = createCampaignTemplateBlocks("collection_launch", ids());
		expect(footer[0].type).toBe("footer");
		expect(campaign.map((block) => block.type)).toEqual([
			"hero",
			"category_tiles",
			"product_rail",
			"testimonial",
		]);
	});
});
