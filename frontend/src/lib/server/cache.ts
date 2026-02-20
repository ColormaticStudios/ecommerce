import type { RequestEvent } from "@sveltejs/kit";

const PUBLIC_CACHE_CONTROL = "public, max-age=1800, s-maxage=1800, stale-while-revalidate=60";
const PRIVATE_CACHE_CONTROL = "private, no-store";

function hasAuthenticatedContext(event: Pick<RequestEvent, "request">): boolean {
	const authorization = event.request.headers.get("authorization");
	if (authorization && authorization.toLowerCase().startsWith("bearer ")) {
		return true;
	}

	const cookieHeader = event.request.headers.get("cookie") ?? "";
	return cookieHeader.includes("session_token=") || cookieHeader.includes("draft_preview_token=");
}

export function setPublicPageCacheHeaders(
	event: Pick<RequestEvent, "request" | "setHeaders">
): void {
	if (hasAuthenticatedContext(event)) {
		event.setHeaders({
			"Cache-Control": PRIVATE_CACHE_CONTROL,
			Vary: "Cookie, Authorization",
		});
		return;
	}

	event.setHeaders({
		"Cache-Control": PUBLIC_CACHE_CONTROL,
		Vary: "Cookie, Authorization",
	});
}
