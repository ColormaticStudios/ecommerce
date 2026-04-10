function normalizeBaseUrl(value: string): string {
	return value.replace(/\/+$/, "");
}

function readPublicApiBaseUrl(): string {
	const serverValue =
		typeof process !== "undefined"
			? process.env.PUBLIC_API_BASE_URL || process.env.STORYBOOK_PUBLIC_API_BASE_URL
			: undefined;
	const clientValue =
		(typeof __PUBLIC_API_BASE_URL__ !== "undefined" && __PUBLIC_API_BASE_URL__) ||
		(typeof __STORYBOOK_PUBLIC_API_BASE_URL__ !== "undefined" &&
			__STORYBOOK_PUBLIC_API_BASE_URL__) ||
		undefined;
	const value = serverValue || clientValue || "http://localhost:3000";
	return String(value);
}

export const API_BASE_URL = normalizeBaseUrl(readPublicApiBaseUrl());
