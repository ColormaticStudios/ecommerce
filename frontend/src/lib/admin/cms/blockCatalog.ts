import type { CmsContentBlock } from "$lib/cms";

export interface CmsBlockLibraryItem {
	type: CmsContentBlock["type"];
	label: string;
	icon: string;
	description: string;
}

export const cmsBlockLibrary: CmsBlockLibraryItem[] = [
	{ type: "hero", label: "Hero", icon: "bi-stars", description: "Headline, media, and CTA" },
	{ type: "rich_text", label: "Text", icon: "bi-text-paragraph", description: "Editorial copy" },
	{ type: "image", label: "Image", icon: "bi-image", description: "Single media block" },
	{ type: "cta", label: "CTA", icon: "bi-cursor", description: "Standalone action" },
	{ type: "promo_banner", label: "Banner", icon: "bi-megaphone", description: "Site message" },
	{ type: "product_rail", label: "Products", icon: "bi-grid", description: "Live catalog rail" },
	{
		type: "category_tiles",
		label: "Categories",
		icon: "bi-collection",
		description: "Active category tiles",
	},
	{
		type: "promotion_highlight",
		label: "Promotion",
		icon: "bi-percent",
		description: "Campaign callout",
	},
	{
		type: "inventory_message",
		label: "Inventory",
		icon: "bi-box-seam",
		description: "Stock-aware message",
	},
	{ type: "testimonial", label: "Review", icon: "bi-chat-quote", description: "Customer proof" },
	{ type: "social_embed", label: "Social", icon: "bi-play-btn", description: "Allowlisted post" },
];

export function cmsBlockLabel(type: string): string {
	return type.replaceAll("_", " ");
}

export function cmsLibraryPreviewClass(type: CmsContentBlock["type"]): string {
	switch (type) {
		case "hero":
			return "h-16 bg-linear-to-br from-stone-900 to-stone-600";
		case "image":
			return "h-16 bg-sky-100 dark:bg-sky-950";
		case "rich_text":
			return "h-12 bg-white dark:bg-stone-900";
		case "cta":
			return "h-10 bg-stone-900 dark:bg-stone-100";
		case "promo_banner":
		case "promotion_highlight":
			return "h-10 bg-emerald-600 dark:bg-emerald-400";
		case "product_rail":
			return "grid h-14 grid-cols-3 gap-1";
		case "category_tiles":
			return "grid h-14 grid-cols-2 gap-1";
		case "testimonial":
			return "h-12 border-l-4 border-amber-400 bg-stone-100 dark:bg-stone-800";
		default:
			return "h-12 bg-stone-100 dark:bg-stone-800";
	}
}
