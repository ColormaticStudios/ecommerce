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

export function appendQueryParams(url: URL, params?: Record<string, unknown> | null): void {
	if (!params) {
		return;
	}
	for (const [key, value] of Object.entries(params)) {
		appendParam(url, key, value);
	}
}
