import type { components } from "$lib/api/generated/openapi";

export type LocalizationRolloutDomain = components["schemas"]["LocalizationRolloutDomain"];

export function localizationDomainForPath(pathname: string): LocalizationRolloutDomain {
	if (pathname.startsWith("/admin")) return "admin";
	if (pathname === "/cart" || pathname.startsWith("/checkout")) return "checkout";
	if (
		pathname.startsWith("/login") ||
		pathname.startsWith("/signup") ||
		pathname.startsWith("/profile") ||
		pathname.startsWith("/orders")
	) {
		return "account";
	}
	return "storefront";
}
