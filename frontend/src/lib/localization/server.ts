import type { LocalizationLocale } from "./runtime";

interface WeightedLocale {
	locale: string;
	quality: number;
	order: number;
}

export function parseAcceptedLanguages(header: string): string[] {
	return header
		.split(",")
		.map((raw, order): WeightedLocale | null => {
			const [localePart, ...parameters] = raw.split(";");
			const locale = localePart.trim();
			if (!locale || locale === "*") return null;
			let quality = 1;
			for (const parameter of parameters) {
				const [name, value] = parameter.trim().split("=", 2);
				if (name.toLowerCase() === "q") {
					const parsed = Number(value);
					quality = Number.isFinite(parsed) && parsed >= 0 && parsed <= 1 ? parsed : 0;
				}
			}
			return quality > 0 ? { locale, quality, order } : null;
		})
		.filter((value): value is WeightedLocale => value !== null)
		.sort((left, right) => right.quality - left.quality || left.order - right.order)
		.map((value) => value.locale);
}

export function selectInitialLocale(input: {
	locales: LocalizationLocale[];
	queryLocale?: string;
	cookieLocale?: string;
	headerLocale?: string;
	accountLocale?: string;
	acceptedLanguages?: string[];
	market?: string;
}): string {
	const enabled = input.locales.filter((locale) => locale.is_enabled);
	const exact = new Map(enabled.map((locale) => [locale.code.toLowerCase(), locale.code]));
	const match = (candidate: string | undefined): string | null => {
		if (!candidate) return null;
		const exactMatch = exact.get(candidate.trim().toLowerCase());
		if (exactMatch) return exactMatch;
		const base = candidate.trim().split("-", 1)[0]?.toLowerCase();
		return enabled.find((locale) => locale.code.toLowerCase() === base)?.code ?? null;
	};
	for (const candidate of [
		input.queryLocale,
		input.cookieLocale,
		input.headerLocale,
		...(input.acceptedLanguages ?? []),
		input.accountLocale,
	]) {
		const selected = match(candidate);
		if (selected) return selected;
	}
	const market = input.market?.trim().toUpperCase();
	if (market) {
		const marketDefault = enabled.find((locale) => locale.default_for_markets.includes(market));
		if (marketDefault) return marketDefault.code;
	}
	return enabled.find((locale) => locale.is_default)?.code ?? enabled[0]?.code ?? "en-US";
}
