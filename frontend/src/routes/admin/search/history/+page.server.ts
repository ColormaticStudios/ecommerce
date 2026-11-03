import type { PageServerLoad } from "./$types";
import type { components, operations } from "$lib/api/generated/openapi";
import { serverRequest } from "$lib/server/api";

export const load: PageServerLoad = async (event) => {
	const { isAdmin } = await event.parent();
	const query: NonNullable<
		operations["listAdminSearchConfigurationHistory"]["parameters"]["query"]
	> = { page: 1, limit: 20 };
	const kind = event.url.searchParams.get("kind");
	if (kind === "synonym" || kind === "typo_profile" || kind === "ranking_profile")
		query.kind = kind;
	for (const key of ["page", "entity_id"] as const) {
		const value = Number(event.url.searchParams.get(key));
		if (Number.isSafeInteger(value) && value > 0) query[key] = value;
	}
	if (!isAdmin) return { history: null, query, errorMessage: "" };
	try {
		return {
			history: await serverRequest<components["schemas"]["SearchConfigurationAuditListResponse"]>(
				event,
				"/admin/search/configuration-history",
				query
			),
			query,
			errorMessage: "",
		};
	} catch {
		return { history: null, query, errorMessage: "Unable to load search configuration history." };
	}
};
