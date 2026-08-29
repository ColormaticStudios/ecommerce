import { describe, expect, it } from "vitest";
import { parseAcceptedLanguages, selectInitialLocale } from "./server";

const locales = [
	{ code: "en-US", name: "English", is_enabled: true, is_default: true, default_for_markets: [] },
	{ code: "fr", name: "French", is_enabled: true, is_default: false, default_for_markets: ["CA"] },
];

describe("server localization selection", () => {
	it("orders accepted languages by quality", () => {
		expect(parseAcceptedLanguages("en-US;q=0.5, fr;q=0.9, de;q=0")).toEqual(["fr", "en-US"]);
	});

	it("uses request choices, account, market, then global default", () => {
		expect(
			selectInitialLocale({
				locales,
				queryLocale: "en-US",
				cookieLocale: "fr",
				accountLocale: "fr",
			})
		).toBe("en-US");
		expect(selectInitialLocale({ locales, cookieLocale: "fr", accountLocale: "en-US" })).toBe("fr");
		expect(selectInitialLocale({ locales, headerLocale: "fr", accountLocale: "en-US" })).toBe("fr");
		expect(
			selectInitialLocale({ locales, acceptedLanguages: ["fr"], accountLocale: "en-US" })
		).toBe("fr");
		expect(selectInitialLocale({ locales, accountLocale: "fr" })).toBe("fr");
		expect(selectInitialLocale({ locales, acceptedLanguages: ["fr-CA"] })).toBe("fr");
		expect(selectInitialLocale({ locales, market: "ca" })).toBe("fr");
		expect(selectInitialLocale({ locales })).toBe("en-US");
	});
});
