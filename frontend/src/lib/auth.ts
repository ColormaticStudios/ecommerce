const FALLBACK_REDIRECT_PATH = "/";
const PLACEHOLDER_ORIGIN = "https://storefront.local";
const API_ROUTE = "/api/v1";

function normalizeBaseUrl(value: string): string {
	return value.replace(/\/+$/, "");
}

export function sanitizeAuthRedirectPath(
	value: string | null | undefined,
	fallback = FALLBACK_REDIRECT_PATH
): string {
	if (!value) {
		return fallback;
	}

	try {
		const url = new URL(value, PLACEHOLDER_ORIGIN);
		if (url.origin !== PLACEHOLDER_ORIGIN) {
			return fallback;
		}

		return `${url.pathname}${url.search}${url.hash}` || fallback;
	} catch {
		return fallback;
	}
}

export function buildOIDCLoginUrl(apiBaseUrl: string, redirectPath?: string): string {
	const url = new URL(`${normalizeBaseUrl(apiBaseUrl)}${API_ROUTE}/auth/oidc/login`);
	url.searchParams.set("redirect", sanitizeAuthRedirectPath(redirectPath));
	return url.toString();
}
