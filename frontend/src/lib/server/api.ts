import { ApiProblemError, isApiProblem, type ApiProblem } from "$lib/api/errors";
import { API_BASE_URL } from "$lib/config";
import type { RequestEvent } from "@sveltejs/kit";

const API_ROUTE = "/api/v1";

export type ServerAPIError = ApiProblem;

function isPlainObject(value: unknown): value is Record<string, unknown> {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function appendParam(url: URL, key: string, value: unknown, nested = false): void {
	if (value === undefined || value === null || value === "") {
		return;
	}
	if (Array.isArray(value)) {
		for (const [index, entry] of value.entries()) {
			appendParam(url, nested ? `${key}[${index}]` : key, entry, nested);
		}
		return;
	}
	if (isPlainObject(value)) {
		for (const [nestedKey, nestedValue] of Object.entries(value)) {
			appendParam(url, `${key}[${nestedKey}]`, nestedValue, true);
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
	event: Pick<RequestEvent, "request" | "getClientAddress"> &
		Partial<Pick<RequestEvent, "url" | "locals" | "cookies">>,
	path: string,
	params?: Record<string, unknown>
): Promise<T> {
	const url = new URL(`${API_BASE_URL}${API_ROUTE}${path}`);
	appendQueryParams(url, params);
	const pageUrl = event.url ?? new URL(event.request.url);
	const registryRequest = path === "/localization/locales";
	const localizedPublicRequest =
		!path.startsWith("/admin/") && !path.startsWith("/auth/") && path !== "/me/";
	if (!registryRequest && localizedPublicRequest && pageUrl.searchParams.has("locale"))
		url.searchParams.set("locale", pageUrl.searchParams.get("locale") ?? "");
	if (!registryRequest && localizedPublicRequest && pageUrl.searchParams.has("market"))
		url.searchParams.set("market", pageUrl.searchParams.get("market") ?? "");

	if (registryRequest) url.searchParams.set("market", "");
	const headers = new Headers();
	headers.set("Content-Type", "application/json");
	// SvelteKit resolves trusted ingress hops. Never relay an incoming XFF value.
	headers.set("X-Forwarded-For", event.getClientAddress());
	const cookie = event.cookies
		? event.cookies
				.getAll()
				.map(({ name, value }) => `${name}=${encodeURIComponent(value)}`)
				.join("; ")
		: event.request.headers.get("cookie");
	if (cookie) {
		headers.set(
			"cookie",
			registryRequest
				? cookie
						.split(";")
						.filter((part) => !/^\s*(?:locale|market)=/.test(part))
						.join(";")
				: cookie
		);
	}
	for (const name of [
		"user-agent",
		"referer",
		"x-correlation-id",
		"accept-language",
		"x-locale",
		"x-market",
	]) {
		const value = event.request.headers.get(name);
		if (
			value &&
			!(registryRequest && ["x-locale", "accept-language", "x-market"].includes(name)) &&
			!(
				pageUrl.searchParams.has("locale") &&
				(name === "x-locale" || name === "accept-language")
			) &&
			!(pageUrl.searchParams.has("market") && name === "x-market")
		)
			headers.set(name, value);
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
	if (path.startsWith("/localization/bundles/")) {
		if (
			event.locals?.publicLocalization &&
			!response.headers.get("cache-control")?.startsWith("public,")
		)
			event.locals.publicLocalization.cacheable = false;
		const rollout = response.headers
			.getSetCookie()
			.map((cookie) => /^localization_rollout=([^;]+)/.exec(cookie)?.[1])
			.find(Boolean);
		if (rollout && event.cookies)
			event.cookies.set("localization_rollout", rollout, {
				path: "/",
				httpOnly: true,
				sameSite: "lax",
				secure: pageUrl.protocol === "https:",
				maxAge: 31536000,
			});
	}
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
	event: Pick<RequestEvent, "request" | "getClientAddress">
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
