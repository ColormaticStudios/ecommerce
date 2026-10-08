import type { PageServerLoad } from "./$types";
import { serverRequest } from "$lib/server/api";
import { parseCategory, type CategoryModel } from "$lib/models";
import type { components } from "$lib/api/generated/openapi";
import type { MerchandisingRule, MerchandisingAudit } from "$lib/api/domains/search-merchandising";

export const load: PageServerLoad = async (event) => {
	const { isAdmin } = await event.parent();
	const rules: MerchandisingRule[] = [];
	const audit: MerchandisingAudit[] = [];
	const categories: CategoryModel[] = [];
	const errorMessages: string[] = [];
	if (!isAdmin) return { rules, audit, categories, errorMessages };
	const results = await Promise.allSettled([
		serverRequest<components["schemas"]["SearchMerchandisingRuleListResponse"]>(
			event,
			"/admin/search/merchandising-rules"
		),
		serverRequest<components["schemas"]["CategoryListResponse"]>(event, "/categories"),
		serverRequest<components["schemas"]["SearchMerchandisingAuditListResponse"]>(
			event,
			"/admin/search/merchandising-audit"
		),
	]);
	if (results[0].status === "fulfilled") rules.push(...results[0].value.data);
	else errorMessages.push("Unable to load merchandising rules.");
	if (results[1].status === "fulfilled")
		categories.push(...results[1].value.data.map(parseCategory));
	else errorMessages.push("Unable to load categories.");
	if (results[2].status === "fulfilled") audit.push(...results[2].value.data);
	else errorMessages.push("Unable to load rule history.");
	return { rules, audit, categories, errorMessages };
};
