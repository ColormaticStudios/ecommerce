import type { PageServerLoad } from "./$types";
import { serverRequest } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";
export const load: PageServerLoad = async (event) => {
	const { isAdmin } = await event.parent();
	if (!isAdmin) return { items: [], errorMessage: "" };
	try {
		const response = await serverRequest<
			components["schemas"]["SearchTypoToleranceProfileListResponse"]
		>(event, "/admin/search/typo-profiles");
		return { items: response.data, errorMessage: "" };
	} catch {
		return { items: [], errorMessage: "Unable to load typo profiles." };
	}
};
