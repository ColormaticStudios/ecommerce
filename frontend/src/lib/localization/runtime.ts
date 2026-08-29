import type { components } from "$lib/api/generated/openapi";
import { writable, type Unsubscriber } from "svelte/store";
import type { LocalizationRolloutDomain } from "./domain";

export const LOCALIZATION_CONTEXT = "localization";
export const LOCALIZATION_MISSING_EVENT = "localization:missing";
export const LOCALIZATION_COOKIE = "locale";

export type LocalizationBundle = components["schemas"]["LocalizationBundle"];
export type LocalizationLocale = components["schemas"]["LocalizationLocale"];
export type LocalizationParameter = string | number | boolean;
export type LocalizationParameters = Record<string, LocalizationParameter>;
export type BundleLoader = (
	locale: string,
	domain: LocalizationRolloutDomain
) => Promise<LocalizationBundle>;

export interface LocalizationBootstrap {
	locale: string;
	locales: LocalizationLocale[];
	bundle: LocalizationBundle | null;
	domain?: LocalizationRolloutDomain;
}

function interpolate(template: string, parameters: LocalizationParameters): string {
	return template.replace(/\{([a-zA-Z][a-zA-Z0-9_]*)\}/g, (match, name: string) => {
		const value = parameters[name];
		return value === undefined ? match : String(value);
	});
}

export class LocalizationRuntime {
	private readonly state = writable(this);
	private readonly cache = new Map<string, LocalizationBundle>();
	private readonly reportedMissingKeys = new Set<string>();
	private loader: BundleLoader | null;
	private currentBundle: LocalizationBundle | null;
	private requestedLocale: string;
	private currentDomain: LocalizationRolloutDomain;
	private readonly displaySourceForMissing: boolean;
	readonly locales: LocalizationLocale[];
	locale: string;
	missingCount = 0;

	constructor(
		bootstrap: LocalizationBootstrap,
		loader: BundleLoader | null = null,
		displaySourceForMissing = false
	) {
		this.locale = bootstrap.locale;
		this.requestedLocale = bootstrap.bundle?.resolution.requested_locale || bootstrap.locale;
		this.currentDomain = bootstrap.domain ?? "storefront";
		this.locales = bootstrap.locales;
		this.currentBundle = bootstrap.bundle;
		this.loader = loader;
		this.displaySourceForMissing = displaySourceForMissing;
		if (bootstrap.bundle) {
			this.cache.set(this.cacheKey(this.requestedLocale, this.currentDomain), bootstrap.bundle);
		}
	}

	subscribe(run: (runtime: LocalizationRuntime) => void): Unsubscriber {
		return this.state.subscribe(run);
	}

	setLoader(loader: BundleLoader): void {
		this.loader = loader;
	}

	translate(key: string, source = "", parameters: LocalizationParameters = {}): string {
		const message = this.currentBundle?.messages[key];
		if (!message) {
			if (!this.displaySourceForMissing) {
				this.reportMissing(key, "unknown_key");
			}
			return interpolate(source || key, parameters);
		}
		if (message.missing_translation && !this.isDefaultLocaleSource(message.source_locale)) {
			this.reportMissing(key, "source_fallback");
		}
		return interpolate(message.value || source, parameters);
	}

	has(key: string): boolean {
		return Boolean(this.currentBundle?.messages[key]);
	}

	plural(
		key: string,
		count: number,
		sources: Partial<Record<Intl.LDMLPluralRule, string>>,
		parameters: LocalizationParameters = {}
	): string {
		const category = new Intl.PluralRules(this.locale).select(count);
		const categoryKey = `${key}.${category}`;
		const otherKey = `${key}.other`;
		const selectedKey = this.has(categoryKey) ? categoryKey : otherKey;
		const source = sources[category] ?? sources.other ?? "";
		return this.translate(selectedKey, source, { ...parameters, count });
	}

	select(
		key: string,
		selection: string,
		sources: Record<string, string>,
		parameters: LocalizationParameters = {}
	): string {
		const selectedKey = `${key}.${selection}`;
		const fallbackKey = `${key}.other`;
		return this.translate(
			this.has(selectedKey) ? selectedKey : fallbackKey,
			sources[selection] ?? sources.other ?? "",
			{
				...parameters,
				selection,
			}
		);
	}

	formatNumber(value: number, options?: Intl.NumberFormatOptions): string {
		return new Intl.NumberFormat(this.locale, options).format(value);
	}

	formatCurrency(value: number, currency: string): string {
		return this.formatNumber(value, { style: "currency", currency });
	}

	formatDate(value: Date | string | number, options?: Intl.DateTimeFormatOptions): string {
		return new Intl.DateTimeFormat(this.locale, options).format(new Date(value));
	}

	async setLocale(
		locale: string,
		persist = true,
		domain: LocalizationRolloutDomain = this.currentDomain
	): Promise<void> {
		const configured = this.locales.find(
			(candidate) => candidate.code === locale && candidate.is_enabled
		);
		if (!configured) {
			throw new Error(`Locale ${locale} is not enabled`);
		}
		const cacheKey = this.cacheKey(locale, domain);
		let bundle = this.cache.get(cacheKey) ?? null;
		if (!bundle) {
			if (!this.loader) {
				throw new Error("Localization bundle loader is unavailable");
			}
			bundle = await this.loader(locale, domain);
			this.cache.set(cacheKey, bundle);
		}
		this.requestedLocale = locale;
		this.currentDomain = domain;
		this.locale = bundle.resolution.resolved_locale;
		this.currentBundle = bundle;
		if (typeof document !== "undefined") {
			document.documentElement.lang = this.locale;
			if (persist) {
				document.cookie = `${LOCALIZATION_COOKIE}=${encodeURIComponent(this.requestedLocale)}; Path=/; SameSite=Lax; Max-Age=31536000`;
			}
		}
		this.state.set(this);
	}

	async refresh(): Promise<void> {
		this.cache.delete(this.cacheKey(this.requestedLocale, this.currentDomain));
		await this.setLocale(this.requestedLocale, false, this.currentDomain);
	}

	async setDomain(domain: LocalizationRolloutDomain): Promise<void> {
		if (domain === this.currentDomain) return;
		await this.setLocale(this.requestedLocale, false, domain);
	}

	private cacheKey(locale: string, domain: LocalizationRolloutDomain): string {
		return `${locale}\u0000${domain}`;
	}

	private reportMissing(key: string, reason: "unknown_key" | "source_fallback"): void {
		if (this.reportedMissingKeys.has(key)) {
			return;
		}
		this.reportedMissingKeys.add(key);
		this.missingCount += 1;
		console.warn("Missing localization message", { key, locale: this.locale, reason });
		if (typeof window !== "undefined") {
			window.dispatchEvent(
				new CustomEvent(LOCALIZATION_MISSING_EVENT, {
					detail: { key, locale: this.locale, reason, missing_count: this.missingCount },
				})
			);
		}
	}

	private isDefaultLocaleSource(sourceLocale: string): boolean {
		const defaultLocale = this.locales.find((locale) => locale.is_default)?.code;
		return Boolean(
			defaultLocale && this.locale === defaultLocale && sourceLocale === defaultLocale
		);
	}
}

export function createFallbackLocalization(): LocalizationRuntime {
	const locales: LocalizationLocale[] = [
		{
			code: "en-US",
			name: "English (United States)",
			is_enabled: true,
			is_default: true,
			default_for_markets: [],
		},
		{
			code: "fr",
			name: "French",
			is_enabled: true,
			is_default: false,
			fallback_locale: "en-US",
			default_for_markets: [],
		},
	];
	const emptyBundle = (locale: string): LocalizationBundle => ({
		resolution: {
			requested_locale: locale,
			resolved_locale: locale,
			source: "explicit",
			fallback_chain: locale === "en-US" ? ["en-US"] : [locale, "en-US"],
			used_fallback: false,
		},
		release: {
			id: 0,
			name: "storybook-fallback",
			snapshot_hash: "0".repeat(64),
			version: "0".repeat(64),
			published_at: "1970-01-01T00:00:00Z",
		},
		messages: {},
	});
	return new LocalizationRuntime(
		{ locale: "en-US", locales, bundle: emptyBundle("en-US") },
		async (locale) => emptyBundle(locale),
		true
	);
}
