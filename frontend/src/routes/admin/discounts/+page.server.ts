import type { PageServerLoad } from "./$types";
import { parseProduct, type ProductModel } from "$lib/models";
import { serverRequest } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";

type DiscountCampaignListResponse = components["schemas"]["DiscountCampaignListResponse"];
type PromotionTemplateListResponse = components["schemas"]["PromotionTemplateListResponse"];
type DiscountStateHistoryListResponse = components["schemas"]["DiscountStateHistoryListResponse"];
type DiscountCampaignAuditListResponse = components["schemas"]["DiscountCampaignAuditListResponse"];
type DiscountEvaluationMetrics = components["schemas"]["DiscountEvaluationMetrics"];
type ProductPagePayload = components["schemas"]["ProductPage"];

const defaultMetrics: DiscountEvaluationMetrics = {
	total_evaluations: 0,
	failed_evaluations: 0,
	matched_evaluations: 0,
	total_latency_ms: 0,
	last_latency_ms: 0,
	last_line_count: 0,
	last_candidate_campaigns: 0,
	last_matched_campaigns: 0,
	last_evaluated_at: null,
	last_error: "",
};

export const load: PageServerLoad = async (event) => {
	const { isAdmin } = await event.parent();
	const errorMessages: string[] = [];

	if (!isAdmin) {
		return {
			campaigns: [],
			templates: [],
			history: { history: [] },
			audit: { audit: [] },
			metrics: defaultMetrics,
			products: [] as ProductModel[],
			errorMessages,
		};
	}

	try {
		const [campaigns, templates, history, audit, metrics, productPage] = await Promise.all([
			serverRequest<DiscountCampaignListResponse>(event, "/admin/discounts/campaigns"),
			serverRequest<PromotionTemplateListResponse>(event, "/admin/discounts/templates"),
			serverRequest<DiscountStateHistoryListResponse>(event, "/admin/discounts/history"),
			serverRequest<DiscountCampaignAuditListResponse>(event, "/admin/discounts/audit"),
			serverRequest<DiscountEvaluationMetrics>(event, "/admin/discounts/metrics"),
			serverRequest<ProductPagePayload>(event, "/admin/products", { page: 1, limit: 20 }),
		]);

		return {
			campaigns: campaigns.campaigns,
			templates: templates.templates,
			history,
			audit,
			metrics,
			products: productPage.data.map(parseProduct),
			errorMessages,
		};
	} catch (err) {
		console.error(err);
		errorMessages.push("Unable to load discount and promotion data.");
		return {
			campaigns: [],
			templates: [],
			history: { history: [] },
			audit: { audit: [] },
			metrics: defaultMetrics,
			products: [] as ProductModel[],
			errorMessages,
		};
	}
};
