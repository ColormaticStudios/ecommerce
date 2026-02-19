export function appendQueryParams(url: URL, params?: Record<string, unknown> | null): void {
	if (!params) {
		return;
	}
	for (const [key, value] of Object.entries(params)) {
		if (value === undefined || value === null || value === "") {
			continue;
		}
		url.searchParams.append(key, String(value));
	}
}
