import type { RequestEvent } from "@sveltejs/kit";

export const PUBLIC_CACHE_CONTROL =
	"public, max-age=1800, s-maxage=1800, stale-while-revalidate=60";
export const PRIVATE_CACHE_CONTROL = "private, no-store";

export function hasAuthenticatedContext(event: Pick<RequestEvent, "request">): boolean {
	const authorization = event.request.headers.get("authorization");
	if (authorization) return true;
	return /(?:^|;\s*)(?:session_token|draft_preview_token)=/.test(
		event.request.headers.get("cookie") ?? ""
	);
}

export function setPublicPageCacheHeaders(
	event: Pick<RequestEvent, "request" | "setHeaders"> & Partial<Pick<RequestEvent, "locals">>,
	personalized = false
): void {
	const shared =
		!personalized &&
		!hasAuthenticatedContext(event) &&
		event.locals?.publicLocalization?.cacheable === true;
	event.setHeaders({ "Cache-Control": shared ? PUBLIC_CACHE_CONTROL : PRIVATE_CACHE_CONTROL });
}
