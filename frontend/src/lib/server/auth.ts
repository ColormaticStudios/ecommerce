import { serverRequest } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";
import type { RequestEvent } from "@sveltejs/kit";

export type AuthConfigModel = components["schemas"]["AuthConfigResponse"];

const FALLBACK_AUTH_CONFIG: AuthConfigModel = {
	local_sign_in_enabled: true,
	oidc_enabled: false,
	oidc_display_name: "",
	allow_guest_checkout: true,
};

export async function loadAuthConfig(
	event: Pick<RequestEvent, "request">
): Promise<AuthConfigModel> {
	try {
		return await serverRequest<AuthConfigModel>(event, "/auth/config");
	} catch (err) {
		console.error("Failed to load auth config", err);
		return FALLBACK_AUTH_CONFIG;
	}
}
