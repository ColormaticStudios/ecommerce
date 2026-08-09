export function extractMediaId(url: string, origin = "http://localhost"): string | null {
	try {
		const parsed = new URL(url, origin);
		const segments = parsed.pathname.split("/").filter(Boolean);
		const mediaIndex = segments.indexOf("media");
		if (mediaIndex >= 0 && segments.length > mediaIndex + 1) return segments[mediaIndex + 1];
		return segments.length > 1 ? segments[segments.length - 2] : null;
	} catch {
		return null;
	}
}

export function moveItem<T>(items: readonly T[], index: number, direction: -1 | 1): T[] {
	const nextIndex = index + direction;
	if (index < 0 || index >= items.length || nextIndex < 0 || nextIndex >= items.length)
		return [...items];
	const reordered = [...items];
	[reordered[index], reordered[nextIndex]] = [reordered[nextIndex], reordered[index]];
	return reordered;
}
