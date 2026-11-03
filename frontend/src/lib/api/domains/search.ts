import type { components } from "$lib/api/generated/openapi";
export type Synonym = components["schemas"]["SearchSynonymSet"];
export type SynonymInput = components["schemas"]["SearchSynonymSetInput"];
export type TypoProfile = components["schemas"]["SearchTypoToleranceProfile"];
export type TypoInput = components["schemas"]["SearchTypoToleranceProfileInput"];
export type RankingProfile = components["schemas"]["SearchRankingProfile"];
export type RankingInput = components["schemas"]["SearchRankingProfileInput"];
export type Request = <T>(
	method: string,
	path: string,
	body?: object,
	params?: Record<string, unknown>
) => Promise<T>;
export async function listSynonyms(request: Request): Promise<Synonym[]> {
	return (
		await request<components["schemas"]["SearchSynonymSetListResponse"]>(
			"GET",
			"/admin/search/synonyms"
		)
	).data;
}
export async function listTypoProfiles(request: Request): Promise<TypoProfile[]> {
	return (
		await request<components["schemas"]["SearchTypoToleranceProfileListResponse"]>(
			"GET",
			"/admin/search/typo-profiles"
		)
	).data;
}
export async function listRankingProfiles(request: Request): Promise<RankingProfile[]> {
	return (
		await request<components["schemas"]["SearchRankingProfileListResponse"]>(
			"GET",
			"/admin/search/ranking-profiles"
		)
	).data;
}
