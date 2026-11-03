import type { components } from "$lib/api/generated/openapi";
import type { Request } from "./search";

export type IncidentQuery = { page?: number; limit?: number; status?: "open" | "resolved" };
export function listIncidents(request: Request, query: IncidentQuery = {}) {
	return request<components["schemas"]["SearchIndexIncidentListResponse"]>(
		"GET",
		"/admin/search/incidents",
		undefined,
		{
			page: query.page ?? 1,
			limit: query.limit ?? 20,
			...(query.status ? { status: query.status } : {}),
		}
	);
}
