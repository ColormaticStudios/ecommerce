import type { components } from "$lib/api/generated/openapi";
import { API_BASE_URL } from "$lib/config";

export type CmsPageResponsePayload = components["schemas"]["CmsPageResponse"];
export type CmsNavigationResponsePayload = components["schemas"]["CmsNavigationResponse"];
export type CmsGlobalRegionResponsePayload = components["schemas"]["CmsGlobalRegionResponse"];

export type CmsContentBlock =
	| {
			type: "hero";
			title: string;
			subtitle?: string;
			image_media_id?: string;
			primary_cta?: CmsLink;
	  }
	| { type: "rich_text"; body: string }
	| { type: "image"; media_id: string; alt?: string; caption?: string }
	| { type: "gallery"; images: Array<{ media_id: string; alt?: string; caption?: string }> }
	| { type: "video"; url: string; title?: string }
	| { type: "faq"; items: Array<{ question: string; answer: string }> }
	| { type: "cta"; label: string; url: string; body?: string }
	| { type: "promo_banner"; title: string; body?: string; link?: CmsLink }
	| {
			type: "product_rail";
			title: string;
			subtitle?: string;
			source: "manual" | "newest" | "search" | "category";
			product_ids?: number[];
			query?: string;
			category_slug?: string;
			sort?: "created_at" | "price" | "name";
			order?: "asc" | "desc";
			limit: number;
			image_aspect?: "square" | "wide";
	  }
	| {
			type: "category_tiles";
			title: string;
			subtitle?: string;
			category_slugs: string[];
			category_media_ids?: Record<string, string>;
			image_aspect?: "square" | "wide";
	  }
	| {
			type: "promotion_highlight";
			title: string;
			body?: string;
			badge?: string;
			promotion_code?: string;
			campaign_id?: number;
			link?: CmsLink;
	  }
	| {
			type: "inventory_message";
			product_id: number;
			low_stock_threshold?: number;
			in_stock_message?: string;
			low_stock_message?: string;
			out_of_stock_message?: string;
	  }
	| { type: "testimonial"; quote: string; attribution: string; rating?: number }
	| {
			type: "social_embed";
			provider: "instagram" | "tiktok" | "youtube";
			url: string;
			title?: string;
	  }
	| {
			type: "footer";
			brand_name: string;
			tagline?: string;
			columns: Array<{ title: string; links: CmsLink[] }>;
			social_links: CmsLink[];
			copyright: string;
			layout: "columns" | "centered" | "minimal";
	  }
	| { type: "custom_html"; html: string };

interface CmsLink {
	label: string;
	url: string;
}

export interface CmsPageModel {
	id: number;
	path: string;
	title: string;
	templateKey: string;
	blocks: CmsContentBlock[];
	hasUnpublishedDraft: boolean;
	seo: components["schemas"]["CmsSEOMetadata"] | null;
	localization: components["schemas"]["CmsResolvedLocalization"] | null;
}

export interface CmsNavigationItemModel {
	id: number;
	parentId: number | null;
	label: string;
	itemType: "internal" | "external" | "category" | "product" | "page" | "dropdown";
	targetRef: string;
	url: string;
	sortOrder: number;
	isEnabled: boolean;
	children: CmsNavigationItemModel[];
}

export interface CmsNavigationModel {
	id: number;
	key: string;
	title: string;
	location: string;
	items: CmsNavigationItemModel[];
	hasUnpublishedDraft: boolean;
}

export interface CmsGlobalRegionModel {
	id: number;
	key: string;
	title: string;
	region: string;
	blocks: CmsContentBlock[];
	hasUnpublishedDraft: boolean;
}

export function parseCmsPage(response: CmsPageResponsePayload, useDraft = false): CmsPageModel {
	const version =
		useDraft && response.current_version ? response.current_version : response.published_version;
	const blocks = Array.isArray(version?.payload.blocks)
		? (version.payload.blocks as unknown as CmsContentBlock[])
		: [];
	return {
		id: response.page.id,
		path: response.page.path,
		title: response.page.title,
		templateKey: response.page.template_key,
		blocks,
		hasUnpublishedDraft: response.has_unpublished_draft,
		seo: response.seo ?? null,
		localization: response.localization ?? null,
	};
}

export function parseCmsNavigation(response: CmsNavigationResponsePayload): CmsNavigationModel {
	const flatItems = response.items
		.filter((item) => item.is_enabled)
		.map((item) => ({
			id: item.id,
			parentId: item.parent_id ?? null,
			label: item.label,
			itemType: item.item_type,
			targetRef: item.target_ref,
			url: item.url || item.target_ref,
			sortOrder: item.sort_order,
			isEnabled: item.is_enabled,
			children: [],
		}));
	return {
		id: response.menu.id,
		key: response.menu.key,
		title: response.menu.title,
		location: response.menu.location,
		items: nestNavigationItems(flatItems),
		hasUnpublishedDraft: response.has_unpublished_draft,
	};
}

export function parseCmsGlobalRegion(
	response: CmsGlobalRegionResponsePayload,
	useDraft = false
): CmsGlobalRegionModel {
	const version =
		useDraft && response.current_version ? response.current_version : response.published_version;
	const blocks = Array.isArray(version?.payload.blocks)
		? (version.payload.blocks as unknown as CmsContentBlock[])
		: [];
	return {
		id: response.region.id,
		key: response.region.key,
		title: response.region.title,
		region: response.region.region,
		blocks,
		hasUnpublishedDraft: response.has_unpublished_draft,
	};
}

function nestNavigationItems(items: CmsNavigationItemModel[]): CmsNavigationItemModel[] {
	const byID = new Map<number, CmsNavigationItemModel>();
	for (const item of items) {
		byID.set(item.id, item);
	}
	const roots: CmsNavigationItemModel[] = [];
	for (const item of items) {
		if (item.parentId && byID.has(item.parentId)) {
			byID.get(item.parentId)?.children.push(item);
		} else {
			roots.push(item);
		}
	}
	const sortItems = (entries: CmsNavigationItemModel[]) => {
		entries.sort((a, b) => a.sortOrder - b.sortOrder || a.id - b.id);
		for (const entry of entries) {
			sortItems(entry.children);
		}
	};
	sortItems(roots);
	return roots;
}

export function cmsHref(url: string): string {
	const value = (url || "").trim();
	if (!value) {
		return "/";
	}
	if (/^[a-z][a-z\d+.-]*:/i.test(value)) {
		return value;
	}
	return value.startsWith("/") ? value : `/${value}`;
}

export function isExternalHref(url: string): boolean {
	return /^https?:\/\//i.test(url);
}

export function cmsMediaURL(mediaID: string | null | undefined): string {
	const id = mediaID?.trim();
	return id ? `${API_BASE_URL}/media/${encodeURIComponent(id)}/original.webp` : "";
}
