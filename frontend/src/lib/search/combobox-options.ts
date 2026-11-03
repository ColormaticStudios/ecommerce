export interface SearchOption {
	value: string;
	label: string;
	kind?: "suggestion" | "correction";
}

export function searchOptions(
	items: SearchOption[],
	suggestions: string[],
	corrections: string[]
): SearchOption[] {
	const corrected = new Set(corrections);
	const options = [
		...items,
		...suggestions.map((value) => ({ value, label: value, kind: "suggestion" as const })),
		...corrections.map((value) => ({ value, label: value, kind: "correction" as const })),
	];
	const seen = new Set<string>();
	return options
		.filter((option) => {
			if (seen.has(option.value)) return false;
			seen.add(option.value);
			return true;
		})
		.map((option) => (corrected.has(option.value) ? { ...option, kind: "correction" } : option));
}
