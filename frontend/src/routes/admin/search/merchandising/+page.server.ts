import type { PageServerLoad } from "./$types";
import { LocalizationRuntime, createFallbackLocalization } from "$lib/localization/runtime";
import { serverRequest } from "$lib/server/api";
import {
	parseCategory,
	parseBrand,
	parseProductAttributeDefinition,
	type CategoryModel,
	type BrandModel,
	type ProductAttributeDefinitionModel,
} from "$lib/models";
import type { components } from "$lib/api/generated/openapi";
import type { MerchandisingRule, MerchandisingAudit } from "$lib/api/domains/search-merchandising";

export const load: PageServerLoad = async (event) => {
	const parent = await event.parent();
	const { isAdmin } = parent;
	const messages = parent.localization
		? new LocalizationRuntime(parent.localization)
		: createFallbackLocalization();
	const rules: MerchandisingRule[] = [];
	const audit: MerchandisingAudit[] = [];
	const categories: CategoryModel[] = [];
	const brands: BrandModel[] = [];
	const attributes: ProductAttributeDefinitionModel[] = [];
	const errorMessages: string[] = [];
	if (!isAdmin) return { rules, audit, categories, brands, attributes, errorMessages };
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
		serverRequest<components["schemas"]["BrandListResponse"]>(event, "/brands"),
		serverRequest<components["schemas"]["ProductAttributeDefinitionListResponse"]>(
			event,
			"/product-attributes"
		),
	]);
	if (results[0].status === "fulfilled") rules.push(...results[0].value.data);
	else
		errorMessages.push(
			messages.translate(
				"admin.search.merchandising.load_rules_error",
				"Unable to load merchandising rules."
			)
		);
	if (results[1].status === "fulfilled")
		categories.push(...results[1].value.data.map(parseCategory));
	else
		errorMessages.push(
			messages.translate(
				"admin.search.merchandising.load_categories_error",
				"Unable to load categories."
			)
		);
	if (results[2].status === "fulfilled") audit.push(...results[2].value.data);
	else
		errorMessages.push(
			messages.translate("admin.search.merchandising.history_error", "Unable to load rule history.")
		);
	if (results[3].status === "fulfilled") brands.push(...results[3].value.data.map(parseBrand));
	else
		errorMessages.push(
			messages.translate("admin.search.merchandising.load_brands_error", "Unable to load brands.")
		);
	if (results[4].status === "fulfilled")
		attributes.push(...results[4].value.data.map(parseProductAttributeDefinition));
	else
		errorMessages.push(
			messages.translate(
				"admin.search.merchandising.load_attributes_error",
				"Unable to load attributes."
			)
		);
	return { rules, audit, categories, brands, attributes, errorMessages };
};
