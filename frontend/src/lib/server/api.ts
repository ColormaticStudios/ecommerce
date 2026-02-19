import { API_BASE_URL } from "$lib/config";
import type { RequestEvent } from "@sveltejs/kit";

const API_ROUTE = "/api/v1";

export interface ServerAPIError {
	status: number;
	statusText: string;
	body: unknown;
}

function appendQueryParams(url: URL, params?: Record<string, unknown>) {
	if (!params) {
		return;
	}
	for (const [key, value] of Object.entries(params)) {
		if (value === undefined || value === null || value === "") {
			continue;
		}
		url.searchParams.set(key, String(value));
	}
}

export async function serverRequest<T>(
	event: Pick<RequestEvent, "fetch" | "request">,
	path: string,
	params?: Record<string, unknown>
): Promise<T> {
	const url = new URL(`${API_BASE_URL}${API_ROUTE}${path}`);
	appendQueryParams(url, params);

	const headers = new Headers();
	headers.set("Content-Type", "application/json");
	const cookie = event.request.headers.get("cookie");
	if (cookie) {
		headers.set("cookie", cookie);
	}

	const response = await event.fetch(url.toString(), {
		method: "GET",
		headers,
	});
	const text = await response.text();
	let body: unknown;
	try {
		body = text ? JSON.parse(text) : null;
	} catch {
		body = text;
	}

	if (!response.ok) {
		throw {
			status: response.status,
			statusText: response.statusText,
			body,
		} as ServerAPIError;
	}

	return body as T;
}

export async function serverIsAuthenticated(
	event: Pick<RequestEvent, "fetch" | "request">
): Promise<boolean> {
	try {
		await serverRequest(event, "/me/");
		return true;
	} catch (err) {
		const error = err as ServerAPIError;
		if (error.status === 401) {
			return false;
		}
		throw err;
	}
}
