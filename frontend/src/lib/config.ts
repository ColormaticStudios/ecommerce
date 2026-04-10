function normalizeBaseUrl(value: string): string {
	return value.replace(/\/+$/, "");
}

function readPublicApiBaseUrl(): string {
	const value =
		import.meta.env.PUBLIC_API_BASE_URL ||
		import.meta.env.STORYBOOK_PUBLIC_API_BASE_URL ||
		"http://localhost:3000";
	return String(value);
}

export const API_BASE_URL = normalizeBaseUrl(readPublicApiBaseUrl());
