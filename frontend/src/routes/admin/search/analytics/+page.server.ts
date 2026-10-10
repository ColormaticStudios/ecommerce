import type { PageServerLoad } from "./$types";
import type { components } from "$lib/api/generated/openapi";
import { serverRequest } from "$lib/server/api";
export const load: PageServerLoad = async (event) => {
	const { isAdmin } = await event.parent();
	if (!isAdmin) return { analytics: null, errorMessage: "" };
	const requested = Number(event.url.searchParams.get("days") ?? 7);
	const days = [7, 30, 90].includes(requested) ? requested : 7;
	try {
		return {
			analytics: await serverRequest<components["schemas"]["SearchAnalytics"]>(
				event,
				"/admin/search/analytics",
				{ days }
			),
			errorMessage: "",
		};
	} catch {
		return { analytics: null, errorMessage: "Unable to load search analytics." };
	}
};
