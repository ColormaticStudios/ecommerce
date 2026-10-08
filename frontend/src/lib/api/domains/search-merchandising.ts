import type { components } from "$lib/api/generated/openapi";

export type MerchandisingRule = components["schemas"]["SearchMerchandisingRule"];
export type MerchandisingInput = components["schemas"]["SearchMerchandisingRuleInput"];
export type MerchandisingPatch = components["schemas"]["SearchMerchandisingRulePatch"];
export type MerchandisingAudit = components["schemas"]["SearchMerchandisingAudit"];
export type MerchandisingPreviewRequest = components["schemas"]["SearchPreviewRequest"];
export type MerchandisingPreview = components["schemas"]["SearchPreviewResponse"];

type Request = <T>(
	method: string,
	path: string,
	body?: object,
	params?: Record<string, unknown>
) => Promise<T>;
const resource = "/admin/search/merchandising-rules";

export async function listRules(request: Request): Promise<MerchandisingRule[]> {
	const response = await request<components["schemas"]["SearchMerchandisingRuleListResponse"]>(
		"GET",
		resource
	);
	return response.data;
}
export function createRule(request: Request, body: MerchandisingInput): Promise<MerchandisingRule> {
	return request("POST", resource, body);
}
export function updateRule(
	request: Request,
	id: number,
	body: MerchandisingPatch
): Promise<MerchandisingRule> {
	return request("PATCH", `${resource}/${id}`, body);
}
export function deleteRule(request: Request, id: number): Promise<void> {
	return request("DELETE", `${resource}/${id}`);
}
export async function listAudit(request: Request, id: number): Promise<MerchandisingAudit[]> {
	const response = await request<components["schemas"]["SearchMerchandisingAuditListResponse"]>(
		"GET",
		`${resource}/${id}/audit`
	);
	return response.data;
}
export async function listAllAudit(request: Request): Promise<MerchandisingAudit[]> {
	const response = await request<components["schemas"]["SearchMerchandisingAuditListResponse"]>(
		"GET",
		"/admin/search/merchandising-audit"
	);
	return response.data;
}
export function preview(
	request: Request,
	body: MerchandisingPreviewRequest
): Promise<MerchandisingPreview> {
	return request("POST", "/admin/search/preview", body);
}
