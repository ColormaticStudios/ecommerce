import { afterEach, expect, test, vi } from "vitest";
import { appendBrowserLocalization, localizedPageUrl } from "./url";
afterEach(() => vi.unstubAllGlobals());
test("browser API calls preserve canonical locale and explicit empty market", () => {
	vi.stubGlobal("window", { location: { href: "https://shop.test/search?locale=fr&market=" } });
	const api = new URL("https://api.test/api/v1/products");
	appendBrowserLocalization(api);
	expect(api.searchParams.get("locale")).toBe("fr");
	expect(api.searchParams.has("market")).toBe(true);
	expect(api.searchParams.get("market")).toBe("");
	const explicit = new URL(
		"https://api.test/api/v1/localization/bundles/en-US?locale=en-US&market=US"
	);
	appendBrowserLocalization(explicit);
	expect(explicit.searchParams.get("locale")).toBe("en-US");
});
test("locale changes keep filters and fragments in their canonical URL", () => {
	expect(
		localizedPageUrl(
			new URL("https://shop.test/search?q=boots&locale=en-US&market=CA#results"),
			"fr"
		)
	).toBe("/search?q=boots&locale=fr&market=CA#results");
});

test("rendered links preserve canonical context for new tabs and do not change admin or external URLs", async () => {
	const { localizedHref } = await import("./url");
	const current = new URL("https://shop.test/search?locale=fr&market=CA");
	expect(localizedHref("/product/42", current)).toBe("/product/42?locale=fr&market=CA");
	expect(localizedHref("/search?category_slug=jackets#results", current)).toBe(
		"/search?category_slug=jackets&locale=fr&market=CA#results"
	);
	expect(localizedHref("/search?locale=en-US&market=", current)).toBe(
		"/search?locale=en-US&market="
	);
	for (const href of [
		"/admin/products",
		"/api/v1/products",
		"https://external.test/search",
		"#results",
	])
		expect(localizedHref(href, current)).toBe(href);
});
