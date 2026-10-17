// Explicit URL context overrides browser preferences for canonical storefront pages.
export function appendBrowserLocalization(url: URL): void {
	if (
		typeof window === "undefined" ||
		url.pathname.startsWith("/api/v1/admin/") ||
		url.pathname.startsWith("/api/v1/auth/") ||
		url.pathname === "/api/v1/me/"
	)
		return;
	const current = new URL(window.location.href);
	for (const name of ["locale", "market"]) {
		if (!url.searchParams.has(name) && current.searchParams.has(name))
			url.searchParams.set(name, current.searchParams.get(name) ?? "");
	}
}

export function localizedPageUrl(url: URL, locale: string): string {
	const target = new URL(url);
	target.searchParams.set("locale", locale);
	if (!target.searchParams.has("market")) target.searchParams.set("market", "");
	return target.pathname + target.search + target.hash;
}

// Render the context into hrefs so new tabs and navigation without JavaScript
// keep the same locale/market as ordinary client-side navigation.
export function localizedHref(href: string, current: URL): string {
	if (!current.searchParams.has("locale") || href.startsWith("#")) return href;
	const target = new URL(href, current);
	if (
		target.origin !== current.origin ||
		target.pathname.startsWith("/admin") ||
		target.pathname.startsWith("/api/")
	)
		return href;
	for (const name of ["locale", "market"]) {
		if (!target.searchParams.has(name))
			target.searchParams.set(name, current.searchParams.get(name) ?? "");
	}
	return target.pathname + target.search + target.hash;
}
