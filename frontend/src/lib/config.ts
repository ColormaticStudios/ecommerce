import { readServerPublicRuntimeEnv, resolvePublicApiBaseUrl } from "$lib/public-env";

function normalizeBaseUrl(value: string): string {
	return value.replace(/\/+$/, "");
}

function readPublicApiBaseUrl(): string {
	return resolvePublicApiBaseUrl({
		serverEnv: typeof process !== "undefined" ? readServerPublicRuntimeEnv() : undefined,
		clientEnv: globalThis.__PUBLIC_ENV__,
	});
}

export const API_BASE_URL = normalizeBaseUrl(readPublicApiBaseUrl());
