import type { components } from "$lib/api/generated/openapi";
import storefrontDefaultsJson from "$defaults/storefront.json";
import storefrontLimitsJson from "$defaults/storefront-limits.json";

type StorefrontSettingsPayload = components["schemas"]["StorefrontSettings"];
type StorefrontSettingsResponsePayload = components["schemas"]["StorefrontSettingsResponse"];
type StorefrontHeroPayload = components["schemas"]["StorefrontHero"];
type StorefrontProductSectionPayload = components["schemas"]["StorefrontProductSection"];
type StorefrontHomepageSectionPayload = components["schemas"]["StorefrontHomepageSection"];

export interface StorefrontLimits {
	max_homepage_sections: number;
	max_manual_product_ids: number;
	max_section_promo_cards: number;
	max_section_badges: number;
	max_footer_columns: number;
	max_footer_links_per_column: number;
	max_social_links: number;
	default_product_section_limit: number;
	max_product_section_limit: number;
}

type StorefrontSectionType = StorefrontHomepageSectionPayload["type"];
type StorefrontProductSource = StorefrontProductSectionPayload["source"];
type StorefrontProductSort = StorefrontProductSectionPayload["sort"];
type StorefrontProductOrder = StorefrontProductSectionPayload["order"];
type StorefrontProductImageAspect = StorefrontProductSectionPayload["image_aspect"];

export interface StorefrontLinkModel {
	label: string;
	url: string;
}

export interface StorefrontHeroModel {
	eyebrow: string;
	title: string;
	subtitle: string;
	background_image_url: string;
	background_image_media_id: string;
	primary_cta: StorefrontLinkModel;
	secondary_cta: StorefrontLinkModel;
}

export interface StorefrontProductSectionModel {
	title: string;
	subtitle: string;
	source: StorefrontProductSource;
	query: string;
	product_ids: number[];
	sort: StorefrontProductSort;
	order: StorefrontProductOrder;
	limit: number;
	show_stock: boolean;
	show_description: boolean;
	image_aspect: StorefrontProductImageAspect;
}

export interface StorefrontPromoCardModel {
	kicker: string;
	title: string;
	description: string;
	image_url: string;
	link: StorefrontLinkModel;
}

export interface StorefrontHomepageSectionModel {
	id: string;
	type: StorefrontSectionType;
	enabled: boolean;
	hero?: StorefrontHeroModel;
	product_section?: StorefrontProductSectionModel;
	promo_cards?: StorefrontPromoCardModel[];
	promo_card_limit?: number;
	badges?: string[];
}

export interface StorefrontFooterColumnModel {
	title: string;
	links: StorefrontLinkModel[];
}

export interface StorefrontFooterModel {
	brand_name: string;
	tagline: string;
	copyright: string;
	columns: StorefrontFooterColumnModel[];
	social_links: StorefrontLinkModel[];
	bottom_notice: string;
}

export interface StorefrontSettingsModel {
	site_title: string;
	homepage_sections: StorefrontHomepageSectionModel[];
	footer: StorefrontFooterModel;
}

export interface StorefrontSettingsResponseModel {
	settings: StorefrontSettingsModel;
	updated_at: Date | null;
	has_draft_changes: boolean;
	draft_updated_at: Date | null;
	published_updated_at: Date | null;
}

type JsonRecord = Record<string, unknown>;

function isObject(value: unknown): value is JsonRecord {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function validateObjectKeys(
	obj: JsonRecord,
	required: string[],
	allowed: string[],
	path: string,
	errors: string[]
): void {
	for (const key of required) {
		if (!(key in obj)) {
			errors.push(`Missing ${path}.${key}`);
		}
	}
	for (const key of Object.keys(obj)) {
		if (!allowed.includes(key)) {
			errors.push(`Unknown key ${path}.${key}`);
		}
	}
}

function expectString(value: unknown, path: string, errors: string[]): void {
	if (typeof value !== "string") {
		errors.push(`Expected string at ${path}`);
	}
}

function expectBoolean(value: unknown, path: string, errors: string[]): void {
	if (typeof value !== "boolean") {
		errors.push(`Expected boolean at ${path}`);
	}
}

function expectNumber(value: unknown, path: string, errors: string[]): void {
	if (typeof value !== "number" || Number.isNaN(value)) {
		errors.push(`Expected number at ${path}`);
	}
}

function expectStringArray(value: unknown, path: string, errors: string[]): void {
	if (!Array.isArray(value)) {
		errors.push(`Expected array at ${path}`);
		return;
	}
	value.forEach((entry, index) => expectString(entry, `${path}[${index}]`, errors));
}

function validateLink(value: unknown, path: string, errors: string[]): void {
	if (!isObject(value)) {
		errors.push(`Expected object at ${path}`);
		return;
	}
	validateObjectKeys(value, ["label", "url"], ["label", "url"], path, errors);
	expectString(value.label, `${path}.label`, errors);
	expectString(value.url, `${path}.url`, errors);
}

function validateHero(value: unknown, path: string, errors: string[]): void {
	if (!isObject(value)) {
		errors.push(`Expected object at ${path}`);
		return;
	}
	validateObjectKeys(
		value,
		[
			"eyebrow",
			"title",
			"subtitle",
			"background_image_url",
			"background_image_media_id",
			"primary_cta",
			"secondary_cta",
		],
		[
			"eyebrow",
			"title",
			"subtitle",
			"background_image_url",
			"background_image_media_id",
			"primary_cta",
			"secondary_cta",
		],
		path,
		errors
	);
	expectString(value.eyebrow, `${path}.eyebrow`, errors);
	expectString(value.title, `${path}.title`, errors);
	expectString(value.subtitle, `${path}.subtitle`, errors);
	expectString(value.background_image_url, `${path}.background_image_url`, errors);
	expectString(value.background_image_media_id, `${path}.background_image_media_id`, errors);
	validateLink(value.primary_cta, `${path}.primary_cta`, errors);
	validateLink(value.secondary_cta, `${path}.secondary_cta`, errors);
}

function validateProductSection(value: unknown, path: string, errors: string[]): void {
	if (!isObject(value)) {
		errors.push(`Expected object at ${path}`);
		return;
	}
	validateObjectKeys(
		value,
		[
			"title",
			"subtitle",
			"source",
			"query",
			"product_ids",
			"sort",
			"order",
			"limit",
			"show_stock",
			"show_description",
			"image_aspect",
		],
		[
			"title",
			"subtitle",
			"source",
			"query",
			"product_ids",
			"sort",
			"order",
			"limit",
			"show_stock",
			"show_description",
			"image_aspect",
		],
		path,
		errors
	);
	expectString(value.title, `${path}.title`, errors);
	expectString(value.subtitle, `${path}.subtitle`, errors);
	expectString(value.source, `${path}.source`, errors);
	expectString(value.query, `${path}.query`, errors);
	expectNumber(value.limit, `${path}.limit`, errors);
	expectBoolean(value.show_stock, `${path}.show_stock`, errors);
	expectBoolean(value.show_description, `${path}.show_description`, errors);
	expectString(value.image_aspect, `${path}.image_aspect`, errors);
	if (!Array.isArray(value.product_ids)) {
		errors.push(`Expected array at ${path}.product_ids`);
	} else {
		value.product_ids.forEach((entry, index) =>
			expectNumber(entry, `${path}.product_ids[${index}]`, errors)
		);
	}
}

function validatePromoCard(value: unknown, path: string, errors: string[]): void {
	if (!isObject(value)) {
		errors.push(`Expected object at ${path}`);
		return;
	}
	validateObjectKeys(
		value,
		["kicker", "title", "description", "image_url", "link"],
		["kicker", "title", "description", "image_url", "link"],
		path,
		errors
	);
	expectString(value.kicker, `${path}.kicker`, errors);
	expectString(value.title, `${path}.title`, errors);
	expectString(value.description, `${path}.description`, errors);
	expectString(value.image_url, `${path}.image_url`, errors);
	validateLink(value.link, `${path}.link`, errors);
}

function validateHomepageSection(value: unknown, path: string, errors: string[]): void {
	if (!isObject(value)) {
		errors.push(`Expected object at ${path}`);
		return;
	}
	validateObjectKeys(
		value,
		["id", "type", "enabled"],
		[
			"id",
			"type",
			"enabled",
			"hero",
			"product_section",
			"promo_cards",
			"promo_card_limit",
			"badges",
		],
		path,
		errors
	);
	expectString(value.id, `${path}.id`, errors);
	expectString(value.type, `${path}.type`, errors);
	expectBoolean(value.enabled, `${path}.enabled`, errors);

	if (value.type === "hero") {
		if (value.hero === undefined) {
			errors.push(`Missing ${path}.hero`);
		} else {
			validateHero(value.hero, `${path}.hero`, errors);
		}
	}
	if (value.type === "products") {
		if (value.product_section === undefined) {
			errors.push(`Missing ${path}.product_section`);
		} else {
			validateProductSection(value.product_section, `${path}.product_section`, errors);
		}
	}
	if (value.type === "promo_cards") {
		if (value.promo_cards === undefined) {
			errors.push(`Missing ${path}.promo_cards`);
		} else if (!Array.isArray(value.promo_cards)) {
			errors.push(`Expected array at ${path}.promo_cards`);
		} else {
			value.promo_cards.forEach((card, index) =>
				validatePromoCard(card, `${path}.promo_cards[${index}]`, errors)
			);
		}
		if (value.promo_card_limit === undefined) {
			errors.push(`Missing ${path}.promo_card_limit`);
		} else {
			expectNumber(value.promo_card_limit, `${path}.promo_card_limit`, errors);
		}
	}
	if (value.type === "badges") {
		if (value.badges === undefined) {
			errors.push(`Missing ${path}.badges`);
		} else {
			expectStringArray(value.badges, `${path}.badges`, errors);
		}
	}
}

function validateFooterColumn(value: unknown, path: string, errors: string[]): void {
	if (!isObject(value)) {
		errors.push(`Expected object at ${path}`);
		return;
	}
	validateObjectKeys(value, ["title", "links"], ["title", "links"], path, errors);
	expectString(value.title, `${path}.title`, errors);
	if (!Array.isArray(value.links)) {
		errors.push(`Expected array at ${path}.links`);
		return;
	}
	value.links.forEach((link, index) => validateLink(link, `${path}.links[${index}]`, errors));
}

function validateStorefrontDefaults(raw: unknown): asserts raw is StorefrontSettingsPayload {
	const errors: string[] = [];
	if (!isObject(raw)) {
		throw new Error("defaults/storefront.json must be an object");
	}
	validateObjectKeys(
		raw,
		["site_title", "homepage_sections", "footer"],
		["site_title", "homepage_sections", "footer"],
		"settings",
		errors
	);
	expectString(raw.site_title, "settings.site_title", errors);

	if (!Array.isArray(raw.homepage_sections)) {
		errors.push("Expected array at settings.homepage_sections");
	} else {
		raw.homepage_sections.forEach((section, index) =>
			validateHomepageSection(section, `settings.homepage_sections[${index}]`, errors)
		);
	}

	if (!isObject(raw.footer)) {
		errors.push("Expected object at settings.footer");
	} else {
		validateObjectKeys(
			raw.footer,
			["brand_name", "tagline", "copyright", "columns", "social_links", "bottom_notice"],
			["brand_name", "tagline", "copyright", "columns", "social_links", "bottom_notice"],
			"settings.footer",
			errors
		);
		expectString(raw.footer.brand_name, "settings.footer.brand_name", errors);
		expectString(raw.footer.tagline, "settings.footer.tagline", errors);
		expectString(raw.footer.copyright, "settings.footer.copyright", errors);
		expectString(raw.footer.bottom_notice, "settings.footer.bottom_notice", errors);
		if (!Array.isArray(raw.footer.columns)) {
			errors.push("Expected array at settings.footer.columns");
		} else {
			raw.footer.columns.forEach((column, index) =>
				validateFooterColumn(column, `settings.footer.columns[${index}]`, errors)
			);
		}
		if (!Array.isArray(raw.footer.social_links)) {
			errors.push("Expected array at settings.footer.social_links");
		} else {
			raw.footer.social_links.forEach((link, index) =>
				validateLink(link, `settings.footer.social_links[${index}]`, errors)
			);
		}
	}

	if (errors.length > 0) {
		throw new Error(
			`Invalid defaults/storefront.json:\n${errors.map((error) => `- ${error}`).join("\n")}`
		);
	}
}

function validateStorefrontLimits(raw: unknown): asserts raw is StorefrontLimits {
	const errors: string[] = [];
	if (!isObject(raw)) {
		throw new Error("defaults/storefront-limits.json must be an object");
	}

	const keys = [
		"max_homepage_sections",
		"max_manual_product_ids",
		"max_section_promo_cards",
		"max_section_badges",
		"max_footer_columns",
		"max_footer_links_per_column",
		"max_social_links",
		"default_product_section_limit",
		"max_product_section_limit",
	];
	validateObjectKeys(raw, keys, keys, "limits", errors);
	for (const key of keys) {
		expectNumber(raw[key], `limits.${key}`, errors);
		if (typeof raw[key] === "number" && raw[key] <= 0) {
			errors.push(`Expected positive number at limits.${key}`);
		}
	}
	if (
		typeof raw.default_product_section_limit === "number" &&
		typeof raw.max_product_section_limit === "number" &&
		raw.default_product_section_limit > raw.max_product_section_limit
	) {
		errors.push("limits.default_product_section_limit must be <= limits.max_product_section_limit");
	}
	if (errors.length > 0) {
		throw new Error(
			`Invalid defaults/storefront-limits.json:\n${errors.map((error) => `- ${error}`).join("\n")}`
		);
	}
}

function parseDate(value: string | Date | null | undefined): Date | null {
	if (!value) {
		return null;
	}
	if (value instanceof Date) {
		return value;
	}
	const parsed = new Date(value);
	return Number.isNaN(parsed.valueOf()) ? null : parsed;
}

function toStringValue(value: string | null | undefined): string {
	return (value ?? "").trim();
}

function parseLink(link?: components["schemas"]["StorefrontLink"]): StorefrontLinkModel {
	return {
		label: toStringValue(link?.label),
		url: toStringValue(link?.url),
	};
}

function parseHero(hero?: StorefrontHeroPayload | null): StorefrontHeroModel {
	return {
		eyebrow: toStringValue(hero?.eyebrow),
		title: toStringValue(hero?.title),
		subtitle: toStringValue(hero?.subtitle),
		background_image_url: toStringValue(hero?.background_image_url),
		background_image_media_id: toStringValue(hero?.background_image_media_id),
		primary_cta: parseLink(hero?.primary_cta),
		secondary_cta: parseLink(hero?.secondary_cta),
	};
}

function normalizeHomepageSectionType(value: string | null | undefined): StorefrontSectionType {
	if (value === "hero" || value === "products" || value === "promo_cards" || value === "badges") {
		return value;
	}
	return "products";
}

function normalizeProductSource(value: string | null | undefined): StorefrontProductSource {
	if (value === "manual" || value === "search" || value === "newest") {
		return value;
	}
	return "newest";
}

function normalizeSort(value: string | null | undefined): StorefrontProductSort {
	if (value === "created_at" || value === "price" || value === "name") {
		return value;
	}
	return "created_at";
}

function normalizeOrder(value: string | null | undefined): StorefrontProductOrder {
	if (value === "asc" || value === "desc") {
		return value;
	}
	return "desc";
}

function normalizeImageAspect(value: string | null | undefined): StorefrontProductImageAspect {
	if (value === "square" || value === "wide") {
		return value;
	}
	return "square";
}

function clamp(value: number, min: number, max: number): number {
	return Math.min(max, Math.max(min, value));
}

function parseProductSection(
	section?: components["schemas"]["StorefrontProductSection"] | null
): StorefrontProductSectionModel {
	return {
		title: toStringValue(section?.title) || "Products",
		subtitle: toStringValue(section?.subtitle),
		source: normalizeProductSource(section?.source),
		query: toStringValue(section?.query),
		product_ids: (section?.product_ids ?? [])
			.map((id) => Number(id))
			.filter((id) => Number.isInteger(id) && id > 0),
		sort: normalizeSort(section?.sort),
		order: normalizeOrder(section?.order),
		limit: clamp(
			Number(section?.limit ?? STOREFRONT_LIMITS.default_product_section_limit),
			1,
			STOREFRONT_LIMITS.max_product_section_limit
		),
		show_stock: section?.show_stock ?? true,
		show_description: section?.show_description ?? true,
		image_aspect: normalizeImageAspect(section?.image_aspect),
	};
}

function parsePromoCards(
	cards?: components["schemas"]["StorefrontPromoCard"][] | null
): StorefrontPromoCardModel[] {
	return (cards ?? []).map((card) => ({
		kicker: toStringValue(card.kicker),
		title: toStringValue(card.title),
		description: toStringValue(card.description),
		image_url: toStringValue(card.image_url),
		link: parseLink(card.link),
	}));
}

function parseBadges(badges?: string[] | null): string[] {
	return (badges ?? []).map((badge) => toStringValue(badge)).filter(Boolean);
}

function parseStorefrontSettingsWithFallback(
	settings: StorefrontSettingsPayload | null | undefined,
	fallback: StorefrontSettingsModel
): StorefrontSettingsModel {
	const sections = (settings?.homepage_sections ?? fallback.homepage_sections).map(
		(section, index) => {
			const type = normalizeHomepageSectionType(section.type);
			const parsed: StorefrontHomepageSectionModel = {
				id: toStringValue(section.id) || `${type}-${index + 1}`,
				type,
				enabled: section.enabled ?? true,
			};

			if (type === "hero") {
				parsed.hero = parseHero(section.hero);
			}
			if (type === "products") {
				parsed.product_section = parseProductSection(section.product_section);
			}
			if (type === "promo_cards") {
				const cards = parsePromoCards(section.promo_cards);
				parsed.promo_cards = cards;
				parsed.promo_card_limit = clamp(
					Number(section.promo_card_limit ?? cards.length ?? 1),
					1,
					STOREFRONT_LIMITS.max_section_promo_cards
				);
			}
			if (type === "badges") {
				parsed.badges = parseBadges(section.badges);
			}

			return parsed;
		}
	);

	return {
		site_title: toStringValue(settings?.site_title) || fallback.site_title,
		homepage_sections: sections.length > 0 ? sections : fallback.homepage_sections,
		footer: {
			brand_name: toStringValue(settings?.footer?.brand_name) || fallback.footer.brand_name,
			tagline: toStringValue(settings?.footer?.tagline) || fallback.footer.tagline,
			copyright: toStringValue(settings?.footer?.copyright) || fallback.footer.copyright,
			columns: (settings?.footer?.columns ?? fallback.footer.columns).map((column) => ({
				title: toStringValue(column.title),
				links: (column.links ?? []).map((link) => parseLink(link)),
			})),
			social_links: (settings?.footer?.social_links ?? fallback.footer.social_links).map((link) =>
				parseLink(link)
			),
			bottom_notice:
				toStringValue(settings?.footer?.bottom_notice) || fallback.footer.bottom_notice,
		},
	};
}

function loadSharedDefaultStorefrontSettings(): StorefrontSettingsModel {
	validateStorefrontDefaults(storefrontDefaultsJson);
	return parseStorefrontSettingsWithFallback(storefrontDefaultsJson as StorefrontSettingsPayload, {
		site_title: "Ecommerce",
		homepage_sections: [],
		footer: {
			brand_name: "Ecommerce",
			tagline: "",
			copyright: "",
			columns: [],
			social_links: [],
			bottom_notice: "",
		},
	});
}

function loadSharedStorefrontLimits(): StorefrontLimits {
	validateStorefrontLimits(storefrontLimitsJson);
	return storefrontLimitsJson as StorefrontLimits;
}

export const STOREFRONT_LIMITS = loadSharedStorefrontLimits();
const DEFAULT_STOREFRONT_SETTINGS = loadSharedDefaultStorefrontSettings();

const DEFAULT_HERO_TEMPLATE =
	DEFAULT_STOREFRONT_SETTINGS.homepage_sections.find(
		(section) => section.type === "hero" && section.hero
	)?.hero ?? parseHero(null);

const DEFAULT_PRODUCT_SECTION_TEMPLATE = DEFAULT_STOREFRONT_SETTINGS.homepage_sections.find(
	(section) => section.type === "products" && section.product_section
)?.product_section ?? {
	title: "Products",
	subtitle: "",
	source: "newest" as const,
	query: "",
	product_ids: [],
	sort: "created_at" as const,
	order: "desc" as const,
	limit: STOREFRONT_LIMITS.default_product_section_limit,
	show_stock: true,
	show_description: true,
	image_aspect: "square" as const,
};

const DEFAULT_PROMO_CARD_TEMPLATE = DEFAULT_STOREFRONT_SETTINGS.homepage_sections.find(
	(section) =>
		section.type === "promo_cards" && section.promo_cards && section.promo_cards.length > 0
)?.promo_cards?.[0] ?? {
	kicker: "",
	title: "",
	description: "",
	image_url: "",
	link: { label: "", url: "" },
};

export function createDefaultHeroSection(): StorefrontHeroModel {
	return {
		...DEFAULT_HERO_TEMPLATE,
		primary_cta: { ...DEFAULT_HERO_TEMPLATE.primary_cta },
		secondary_cta: { ...DEFAULT_HERO_TEMPLATE.secondary_cta },
	};
}

export function createDefaultProductSection(
	title = DEFAULT_PRODUCT_SECTION_TEMPLATE.title || "Products",
	source: StorefrontProductSource = DEFAULT_PRODUCT_SECTION_TEMPLATE.source
): StorefrontProductSectionModel {
	return {
		...DEFAULT_PRODUCT_SECTION_TEMPLATE,
		title,
		source,
		product_ids: [...DEFAULT_PRODUCT_SECTION_TEMPLATE.product_ids],
	};
}

export function createDefaultPromoCard(): StorefrontPromoCardModel {
	return {
		...DEFAULT_PROMO_CARD_TEMPLATE,
		link: { ...DEFAULT_PROMO_CARD_TEMPLATE.link },
	};
}

export function createDefaultStorefrontSettings(): StorefrontSettingsModel {
	return cloneStorefrontSettings(DEFAULT_STOREFRONT_SETTINGS);
}

export function cloneStorefrontSettings(
	settings: StorefrontSettingsModel
): StorefrontSettingsModel {
	return JSON.parse(JSON.stringify(settings)) as StorefrontSettingsModel;
}

export function parseStorefrontSettings(
	settings?: StorefrontSettingsPayload | null
): StorefrontSettingsModel {
	return parseStorefrontSettingsWithFallback(settings, DEFAULT_STOREFRONT_SETTINGS);
}

export function parseStorefrontSettingsResponse(
	response: StorefrontSettingsResponsePayload
): StorefrontSettingsResponseModel {
	return {
		settings: parseStorefrontSettings(response.settings),
		updated_at: parseDate(response.updated_at),
		has_draft_changes: response.has_draft_changes ?? false,
		draft_updated_at: parseDate(response.draft_updated_at),
		published_updated_at: parseDate(response.published_updated_at),
	};
}
