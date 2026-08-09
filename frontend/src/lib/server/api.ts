import { ApiProblemError, isApiProblem, type ApiProblem } from "$lib/api/errors";
import { API_BASE_URL } from "$lib/config";
import type { RequestEvent } from "@sveltejs/kit";

const API_ROUTE = "/api/v1";

export type ServerAPIError = ApiProblem;

function isPlainObject(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function appendParam(url: URL, key: string, value: unknown): void {
	if (value === undefined || value === null || value === "") {
		return;
	}
	if (Array.isArray(value)) {
		for (const entry of value) {
			appendParam(url, key, entry);
		}
		return;
	}
	if (isPlainObject(value)) {
		for (const [nestedKey, nestedValue] of Object.entries(value)) {
			appendParam(url, `${key}[${nestedKey}]`, nestedValue);
		}
		return;
	}
	url.searchParams.append(key, String(value));
}

function appendQueryParams(url: URL, params?: Record<string, unknown>) {
	if (!params) {
		return;
	}
	for (const [key, value] of Object.entries(params)) {
		appendParam(url, key, value);
	}
}

export async function serverRequest<T>(
	event: Pick<RequestEvent, "request">,
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
	for (const name of ["user-agent", "referer", "x-correlation-id"]) {
		const value = event.request.headers.get(name);
		if (value) headers.set(name, value);
	}
	const host = event.request.headers.get("x-forwarded-host") ?? event.request.headers.get("host");
	if (host) headers.set("x-forwarded-host", host);
	headers.set("x-forwarded-proto", event.request.headers.get("x-forwarded-proto") ?? "http");

	// Use the server runtime fetch for the external API base URL.
	// SvelteKit's event.fetch can forward request context such as Origin,
	// which trips the API's CORS middleware during SSR/E2E loads.
	const response = await fetch(url.toString(), {
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
		throw new ApiProblemError(response.status, response.statusText, body);
	}

	return body as T;
}

export async function serverIsAuthenticated(
	event: Pick<RequestEvent, "request">
): Promise<boolean> {
	try {
		await serverRequest(event, "/me/");
		return true;
	} catch (err) {
		if (isApiProblem(err) && err.status === 401) {
			return false;
		}
		throw err;
	}
}
