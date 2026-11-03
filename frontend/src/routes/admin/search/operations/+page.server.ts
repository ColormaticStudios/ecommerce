import type { PageServerLoad } from "./$types";
import type { components } from "$lib/api/generated/openapi";
import { serverRequest } from "$lib/server/api";
export const load: PageServerLoad = async (
	event
): Promise<{
	freshness: components["schemas"]["SearchFreshness"] | null;
	operations: components["schemas"]["SearchOperationsStatus"] | null;
	incidents: components["schemas"]["SearchIndexIncidentListResponse"] | null;
	activeIncidents: components["schemas"]["SearchIndexIncident"][];
	errorMessage: string;
}> => {
	const { isAdmin } = await event.parent();
	if (!isAdmin)
		return {
			freshness: null,
			operations: null,
			incidents: null,
			activeIncidents: [],
			errorMessage: "",
		};
	try {
		const [freshness, operations, incidents, active] = await Promise.all([
			serverRequest<components["schemas"]["SearchFreshness"]>(event, "/admin/search/freshness"),
			serverRequest<components["schemas"]["SearchOperationsStatus"]>(
				event,
				"/admin/search/operations"
			),
			serverRequest<components["schemas"]["SearchIndexIncidentListResponse"]>(
				event,
				"/admin/search/incidents?page=1&limit=20"
			),
			serverRequest<components["schemas"]["SearchIndexIncidentListResponse"]>(
				event,
				"/admin/search/incidents?status=open&limit=100"
			),
		]);
		return { freshness, operations, incidents, activeIncidents: active.data, errorMessage: "" };
	} catch {
		return {
			freshness: null,
			operations: null,
			incidents: null,
			activeIncidents: [],
			errorMessage: "Unable to load search index health.",
		};
	}
};
