import type { PageServerLoad } from "./$types";
import { serverRequest } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";
export const load: PageServerLoad = async (event) => {
	const { isAdmin } = await event.parent();
	if (!isAdmin) return { items: [], errorMessage: "" };
	try {
		const response = await serverRequest<components["schemas"]["SearchRankingProfileListResponse"]>(
			event,
			"/admin/search/ranking-profiles"
		);
		return { items: response.data, errorMessage: "" };
	} catch {
		return { items: [], errorMessage: "Unable to load ranking profiles." };
	}
};
