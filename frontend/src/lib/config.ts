import { env } from "$env/dynamic/public";

function normalizeBaseUrl(value: string): string {
	return value.replace(/\/+$/, "");
}

export const API_BASE_URL = normalizeBaseUrl(env.PUBLIC_API_BASE_URL || "http://localhost:3000");
