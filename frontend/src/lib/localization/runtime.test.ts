import { describe, expect, it, vi } from "vitest";
import { LocalizationRuntime, type LocalizationBundle } from "./runtime";

function bundle(locale = "en-US"): LocalizationBundle {
	return {
		resolution: {
			requested_locale: locale,
			resolved_locale: locale,
			source: "explicit",
			fallback_chain: [locale],
			used_fallback: false,
		},
		release: {
			id: 1,
			name: "test",
			snapshot_hash: "a".repeat(64),
			version: "b".repeat(64),
			published_at: "2026-08-11T00:00:00Z",
		},
		messages: {
			"storefront.greeting": {
				value: "Hello, {name}!",
				requested_locale: locale,
				source_locale: locale,
				used_fallback: false,
				missing_translation: false,
			},
			"storefront.items.one": {
				value: "{count} item",
				requested_locale: locale,
				source_locale: locale,
				used_fallback: false,
				missing_translation: false,
			},
			"storefront.items.other": {
				value: "{count} items",
				requested_locale: locale,
				source_locale: locale,
				used_fallback: false,
				missing_translation: false,
			},
		},
	};
}

describe("LocalizationRuntime", () => {
	it("interpolates and pluralizes release-backed messages", () => {
		const runtime = new LocalizationRuntime({ locale: "en-US", locales: [], bundle: bundle() });
		expect(runtime.translate("storefront.greeting", "", { name: "Ada" })).toBe("Hello, Ada!");
		expect(
			runtime.plural("storefront.items", 1, { one: "{count} item", other: "{count} items" })
		).toBe("1 item");
		expect(
			runtime.plural("storefront.items", 3, { one: "{count} item", other: "{count} items" })
		).toBe("3 items");
	});

	it("reports an unknown key once without exposing the internal key", () => {
		const warning = vi.spyOn(console, "warn").mockImplementation(() => undefined);
		const runtime = new LocalizationRuntime({ locale: "en-US", locales: [], bundle: bundle() });
		expect(runtime.translate("storefront.unknown", "Fallback")).toBe("Fallback");
		runtime.translate("storefront.unknown", "Fallback");
		expect(runtime.missingCount).toBe(1);
		expect(warning).toHaveBeenCalledOnce();
		warning.mockRestore();
	});

	it("treats the default locale source catalog as a valid runtime value", () => {
		const warning = vi.spyOn(console, "warn").mockImplementation(() => undefined);
		const sourceBundle = bundle();
		sourceBundle.messages["storefront.greeting"].missing_translation = true;
		const runtime = new LocalizationRuntime({
			locale: "en-US",
			locales: [
				{
					code: "en-US",
					name: "English",
					is_enabled: true,
					is_default: true,
					default_for_markets: [],
				},
			],
			bundle: sourceBundle,
		});

		expect(runtime.translate("storefront.greeting", "Fallback", { name: "Ada" })).toBe(
			"Hello, Ada!"
		);
		expect(runtime.missingCount).toBe(0);
		expect(warning).not.toHaveBeenCalled();
		warning.mockRestore();
	});

	it("loads and caches locale bundles", async () => {
		const loader = vi.fn(async (locale: string) => bundle(locale));
		const runtime = new LocalizationRuntime(
			{
				locale: "en-US",
				locales: [
					{
						code: "en-US",
						name: "English",
						is_enabled: true,
						is_default: true,
						default_for_markets: [],
					},
					{
						code: "fr",
						name: "French",
						is_enabled: true,
						is_default: false,
						default_for_markets: [],
					},
				],
				bundle: bundle(),
			},
			loader
		);
		await runtime.setLocale("fr", false);
		await runtime.setLocale("fr", false);
		expect(loader).toHaveBeenCalledOnce();
		await runtime.refresh();
		expect(loader).toHaveBeenCalledTimes(2);
		expect(runtime.locale).toBe("fr");
	});

	it("caches bundles per locale and rollout domain", async () => {
		const loader = vi.fn(async (locale: string) => bundle(locale));
		const runtime = new LocalizationRuntime(
			{
				locale: "en-US",
				locales: [
					{
						code: "en-US",
						name: "English",
						is_enabled: true,
						is_default: true,
						default_for_markets: [],
					},
				],
				bundle: bundle(),
				domain: "storefront",
			},
			loader
		);

		await runtime.setDomain("checkout");
		await runtime.setDomain("storefront");
		await runtime.setDomain("checkout");

		expect(loader).toHaveBeenCalledOnce();
		expect(loader).toHaveBeenCalledWith("en-US", "checkout");
	});

	it("keeps in-memory translation lookup below the local performance p95 budget", () => {
		const runtime = new LocalizationRuntime({ locale: "en-US", locales: [], bundle: bundle() });
		const durations: number[] = [];
		for (let index = 0; index < 10_000; index += 1) {
			const started = performance.now();
			runtime.translate("storefront.greeting", "", { name: "Ada" });
			durations.push(performance.now() - started);
		}
		durations.sort((left, right) => left - right);
		const p95 = durations[Math.floor(durations.length * 0.95)];
		expect(p95).toBeLessThan(1);
	});
});
