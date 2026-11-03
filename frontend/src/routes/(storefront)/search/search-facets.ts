import type { components } from "$lib/api/generated/openapi";

type Facet = components["schemas"]["SearchFacet"];
type FacetValue = components["schemas"]["SearchFacetValue"];

export function priceFacetBounds(value: FacetValue): { min?: number; max?: number } {
	const [rawMin, rawMax] = value.value.split(":");
	const parse = (raw: string | undefined): number | undefined =>
		raw?.trim() && Number.isFinite(Number(raw)) ? Number(raw) : undefined;
	return {
		min: value.min_price ?? parse(rawMin),
		max: value.max_price ?? parse(rawMax),
	};
}

export function visibleSearchFacets(facets: Facet[], selectedFallbacks: Facet[]): Facet[] {
	const merged = facets.map((facet) => ({ ...facet, values: [...facet.values] }));
	for (const fallback of selectedFallbacks) {
		const selected = fallback.values.filter((value) => value.selected);
		if (!selected.length) continue;
		const facet = merged.find((item) => item.name === fallback.name);
		if (facet) {
			for (const value of selected) {
				if (!facet.values.some((item) => item.value === value.value)) facet.values.push(value);
			}
		} else merged.push({ ...fallback, values: selected });
	}
	return merged.filter((facet) => facet.values.length > 0);
}
