import { parseProfile } from "$lib/models";
import { serverRequest, type ServerAPIError } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";
import type { RequestEvent } from "@sveltejs/kit";

type ProfilePayload = components["schemas"]["User"];

export interface AdminAccessData {
	isAuthenticated: boolean;
	isAdmin: boolean;
	accessError: string;
}

export async function loadAdminAccess(
	event: Pick<RequestEvent, "fetch" | "request">
): Promise<AdminAccessData> {
	let isAuthenticated = false;
	let isAdmin = false;
	let accessError = "";

	try {
		const profilePayload = await serverRequest<ProfilePayload>(event, "/me/");
		isAuthenticated = true;
		isAdmin = parseProfile(profilePayload).role === "admin";
	} catch (err) {
		const error = err as ServerAPIError;
		if (error.status !== 401) {
			console.error(err);
			accessError = "Unable to check admin access.";
		}
	}

	return {
		isAuthenticated,
		isAdmin,
		accessError,
	};
}
