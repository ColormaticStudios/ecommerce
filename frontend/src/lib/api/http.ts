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

export function appendQueryParams(url: URL, params?: Record<string, unknown> | null): void {
	if (!params) {
		return;
	}
	for (const [key, value] of Object.entries(params)) {
		appendParam(url, key, value);
	}
}
