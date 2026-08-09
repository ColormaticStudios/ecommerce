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

export interface CmsRejectedContentBlock {
	index: number;
	reason: string;
	value: unknown;
}

export interface CmsContentBlockDecodeResult {
	blocks: CmsContentBlock[];
	rejectedBlocks: CmsRejectedContentBlock[];
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isString(value: unknown): value is string {
	return typeof value === "string";
}

function isFiniteNumber(value: unknown): value is number {
	return typeof value === "number" && Number.isFinite(value);
}

function hasOptional(
	value: Record<string, unknown>,
	key: string,
	predicate: (candidate: unknown) => boolean
): boolean {
	return !(key in value) || value[key] === undefined || predicate(value[key]);
}

function isOneOf<T extends string>(value: unknown, options: readonly T[]): value is T {
	return typeof value === "string" && options.some((option) => option === value);
}

function isCmsLink(value: unknown): value is CmsLink {
	return isRecord(value) && isString(value.label) && isString(value.url);
}

function isCmsGalleryImage(
	value: unknown
): value is { media_id: string; alt?: string; caption?: string } {
	return (
		isRecord(value) &&
		isString(value.media_id) &&
		hasOptional(value, "alt", isString) &&
		hasOptional(value, "caption", isString)
	);
}

function isCmsFaqItem(value: unknown): value is { question: string; answer: string } {
	return isRecord(value) && isString(value.question) && isString(value.answer);
}

function isCmsFooterColumn(value: unknown): value is { title: string; links: CmsLink[] } {
	return (
		isRecord(value) &&
		isString(value.title) &&
		Array.isArray(value.links) &&
		value.links.every(isCmsLink)
	);
}

function isStringRecord(value: unknown): value is Record<string, string> {
	return isRecord(value) && Object.values(value).every(isString);
}

export function isCmsContentBlock(value: unknown): value is CmsContentBlock {
	if (!isRecord(value) || !isString(value.type)) {
		return false;
	}

	switch (value.type) {
		case "hero":
			return (
				isString(value.title) &&
				hasOptional(value, "subtitle", isString) &&
				hasOptional(value, "image_media_id", isString) &&
				hasOptional(value, "primary_cta", isCmsLink)
			);
		case "rich_text":
			return isString(value.body);
		case "image":
			return (
				isString(value.media_id) &&
				hasOptional(value, "alt", isString) &&
				hasOptional(value, "caption", isString)
			);
		case "gallery":
			return Array.isArray(value.images) && value.images.every(isCmsGalleryImage);
		case "video":
			return isString(value.url) && hasOptional(value, "title", isString);
		case "faq":
			return Array.isArray(value.items) && value.items.every(isCmsFaqItem);
		case "cta":
			return isString(value.label) && isString(value.url) && hasOptional(value, "body", isString);
		case "promo_banner":
			return (
				isString(value.title) &&
				hasOptional(value, "body", isString) &&
				hasOptional(value, "link", isCmsLink)
			);
		case "product_rail":
			return (
				isString(value.title) &&
				isOneOf(value.source, ["manual", "newest", "search", "category"]) &&
				isFiniteNumber(value.limit) &&
				hasOptional(value, "subtitle", isString) &&
				hasOptional(
					value,
					"product_ids",
					(candidate) => Array.isArray(candidate) && candidate.every(isFiniteNumber)
				) &&
				hasOptional(value, "query", isString) &&
				hasOptional(value, "category_slug", isString) &&
				hasOptional(value, "sort", (candidate) =>
					isOneOf(candidate, ["created_at", "price", "name"])
				) &&
				hasOptional(value, "order", (candidate) => isOneOf(candidate, ["asc", "desc"])) &&
				hasOptional(value, "image_aspect", (candidate) => isOneOf(candidate, ["square", "wide"]))
			);
		case "category_tiles":
			return (
				isString(value.title) &&
				Array.isArray(value.category_slugs) &&
				value.category_slugs.every(isString) &&
				hasOptional(value, "subtitle", isString) &&
				hasOptional(value, "category_media_ids", isStringRecord) &&
				hasOptional(value, "image_aspect", (candidate) => isOneOf(candidate, ["square", "wide"]))
			);
		case "promotion_highlight":
			return (
				isString(value.title) &&
				hasOptional(value, "body", isString) &&
				hasOptional(value, "badge", isString) &&
				hasOptional(value, "promotion_code", isString) &&
				hasOptional(value, "campaign_id", isFiniteNumber) &&
				hasOptional(value, "link", isCmsLink)
			);
		case "inventory_message":
			return (
				isFiniteNumber(value.product_id) &&
				hasOptional(value, "low_stock_threshold", isFiniteNumber) &&
				hasOptional(value, "in_stock_message", isString) &&
				hasOptional(value, "low_stock_message", isString) &&
				hasOptional(value, "out_of_stock_message", isString)
			);
		case "testimonial":
			return (
				isString(value.quote) &&
				isString(value.attribution) &&
				hasOptional(value, "rating", isFiniteNumber)
			);
		case "social_embed":
			return (
				isOneOf(value.provider, ["instagram", "tiktok", "youtube"]) &&
				isString(value.url) &&
				hasOptional(value, "title", isString)
			);
		case "footer":
			return (
				isString(value.brand_name) &&
				Array.isArray(value.columns) &&
				value.columns.every(isCmsFooterColumn) &&
				Array.isArray(value.social_links) &&
				value.social_links.every(isCmsLink) &&
				isString(value.copyright) &&
				isOneOf(value.layout, ["columns", "centered", "minimal"]) &&
				hasOptional(value, "tagline", isString)
			);
		case "custom_html":
			return isString(value.html);
		default:
			return false;
	}
}

export function decodeCmsContentBlocks(value: unknown): CmsContentBlockDecodeResult {
	if (!Array.isArray(value)) {
		return {
			blocks: [],
			rejectedBlocks: [{ index: -1, reason: "CMS blocks must be an array", value }],
		};
	}

	const blocks: CmsContentBlock[] = [];
	const rejectedBlocks: CmsRejectedContentBlock[] = [];
	value.forEach((candidate, index) => {
		if (isCmsContentBlock(candidate)) {
			blocks.push(candidate);
			return;
		}
		const type = isRecord(candidate) && isString(candidate.type) ? candidate.type : "unknown";
		const reason =
			type === "unknown"
				? "CMS block must be an object with a supported type"
				: `CMS block has an unsupported type or invalid structure: ${type}`;
		rejectedBlocks.push({ index, reason, value: candidate });
	});
	return { blocks, rejectedBlocks };
}

export interface CmsPageModel {
	id: number;
	path: string;
	title: string;
	templateKey: string;
	blocks: CmsContentBlock[];
	rejectedBlocks?: CmsRejectedContentBlock[];
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
	rejectedBlocks?: CmsRejectedContentBlock[];
	hasUnpublishedDraft: boolean;
}

export function parseCmsPage(response: CmsPageResponsePayload, useDraft = false): CmsPageModel {
	const version =
		useDraft && response.current_version ? response.current_version : response.published_version;
	const { blocks, rejectedBlocks } = decodeCmsContentBlocks(version?.payload.blocks ?? []);
	return {
		id: response.page.id,
		path: response.page.path,
		title: response.page.title,
		templateKey: response.page.template_key,
		blocks,
		rejectedBlocks,
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
	const { blocks, rejectedBlocks } = decodeCmsContentBlocks(version?.payload.blocks ?? []);
	return {
		id: response.region.id,
		key: response.region.key,
		title: response.region.title,
		region: response.region.region,
		blocks,
		rejectedBlocks,
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
