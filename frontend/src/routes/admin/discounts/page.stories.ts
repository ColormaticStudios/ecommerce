import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import type { components } from "$lib/api/generated/openapi";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub } from "$lib/storybook/api";
import { makeProduct, makeVariant } from "$lib/storybook/factories";
import { makeAdminLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import AdminDiscountsPage from "./+page.svelte";

type AdminDiscountsData = ComponentProps<typeof AdminDiscountsPage>["data"];
type DiscountCampaign = components["schemas"]["DiscountCampaign"];
type PromotionTemplate = components["schemas"]["PromotionTemplate"];
type DiscountEvaluationMetrics = components["schemas"]["DiscountEvaluationMetrics"];

const now = new Date("2026-05-21T16:00:00.000Z");

const activeDiscount: DiscountCampaign = {
	id: 1,
	name: "Spring jacket markdown",
	type: "product_discount",
	status: "active",
	starts_at: now.toISOString(),
	ends_at: new Date(now.getTime() + 7 * 24 * 60 * 60 * 1000).toISOString(),
	discount_mode: "percent",
	discount_value: 15,
	priority: 10,
	is_exclusive: false,
	coupon_code: null,
	channels: ["web"],
	customer_segment: null,
	global_usage_cap: null,
	per_customer_usage_cap: null,
	targets: [{ id: 11, target_type: "product", target_id: 101 }],
	created_at: now.toISOString(),
	updated_at: now.toISOString(),
};

const scheduledPromotion: DiscountCampaign = {
	id: 2,
	name: "Spend 100 get tote",
	type: "promotion",
	status: "scheduled",
	starts_at: new Date(now.getTime() + 24 * 60 * 60 * 1000).toISOString(),
	ends_at: null,
	discount_mode: "fixed",
	discount_value: 0,
	priority: 20,
	is_exclusive: true,
	coupon_code: "TOTE",
	channels: ["web", "app"],
	customer_segment: null,
	global_usage_cap: 500,
	per_customer_usage_cap: 1,
	targets: [],
	created_at: now.toISOString(),
	updated_at: now.toISOString(),
};

const template: PromotionTemplate = {
	id: 1,
	name: "Category threshold",
	description: "Spend threshold promotion scoped to category ids.",
	template_json: "{}",
	template: {},
	is_active: true,
	created_at: now.toISOString(),
	updated_at: now.toISOString(),
};

const metrics: DiscountEvaluationMetrics = {
	total_evaluations: 142,
	failed_evaluations: 1,
	matched_evaluations: 83,
	total_latency_ms: 920,
	last_latency_ms: 8,
	last_line_count: 3,
	last_candidate_campaigns: 4,
	last_matched_campaigns: 2,
	last_evaluated_at: now.toISOString(),
	last_error: "",
};

const meta = {
	title: "Routes/Admin/Discounts",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function createData(overrides: Partial<AdminDiscountsData> = {}): AdminDiscountsData {
	const products = [
		makeProduct({
			id: 101,
			sku: "field-jacket",
			name: "Field Jacket",
			variants: [makeVariant({ id: 101, sku: "field-jacket-default", title: "Default" })],
			default_variant_id: 101,
			default_variant_sku: "field-jacket-default",
		}),
		makeProduct({
			id: 202,
			sku: "canvas-tote",
			name: "Canvas Tote",
			price: 28,
			base_price: 32,
			discount_amount: 4,
			final_price: 28,
			variants: [makeVariant({ id: 202, sku: "canvas-tote-default", title: "Default", price: 28 })],
			default_variant_id: 202,
			default_variant_sku: "canvas-tote-default",
		}),
	];
	return {
		...makeAdminLayoutData(),
		campaigns: [activeDiscount, scheduledPromotion],
		templates: [template],
		history: {
			history: [
				{
					id: 1,
					campaign_id: 2,
					from_status: "disabled",
					to_status: "scheduled",
					reason: "schedule_created",
					source: "admin",
					actor: "story-admin",
					changed_at: now.toISOString(),
				},
			],
		},
		audit: {
			audit: [
				{
					id: 1,
					campaign_id: 1,
					event_type: "campaign_created",
					source: "admin",
					actor: "story-admin",
					summary: "Created product discount campaign.",
					before_json: "{}",
					after_json: "{}",
					changed_at: now.toISOString(),
				},
			],
		},
		metrics,
		products,
		errorMessages: [],
		...overrides,
	};
}

function createDiscountsApiStub(data = createData()) {
	return createApiStub({
		listAdminDiscountCampaigns: async () => data.campaigns,
		createAdminDiscountCampaign: async () => activeDiscount,
		updateAdminDiscountCampaign: async () => activeDiscount,
		disableAdminDiscountCampaign: async () => ({ ...activeDiscount, status: "disabled" }),
		archiveAdminDiscountCampaign: async () => ({ ...activeDiscount, status: "archived" }),
		createAdminPromotionCampaign: async () => scheduledPromotion,
		previewAdminPromotion: async () => ({
			subtotal: 129,
			discount_total: 19.35,
			final_subtotal: 109.65,
			lines: [
				{
					product_id: 101,
					product_variant_id: 101,
					quantity: 1,
					base_price: 129,
					discount_amount: 19.35,
					final_price: 109.65,
					applied_campaigns: [
						{ id: 1, level_id: null, name: "Spring jacket markdown", discount_amount: 19.35 },
					],
				},
			],
		}),
		listAdminPromotionTemplates: async () => data.templates,
		createAdminPromotionTemplate: async () => template,
		instantiateAdminPromotionTemplate: async () => scheduledPromotion,
		scheduleAdminDiscountCampaign: async () => ({
			id: 1,
			campaign_id: 1,
			schedule_type: "one_time",
			window_start: now.toISOString(),
			window_end: new Date(now.getTime() + 60 * 60 * 1000).toISOString(),
			timezone: "UTC",
			recurrence: null,
			until_at: null,
			last_run_at: null,
			next_run_at: null,
		}),
		runAdminDiscountLifecycle: async () => ({ activated: 1, deactivated: 0, archived: 0 }),
		listAdminDiscountHistory: async () => data.history,
		listAdminDiscountAudit: async () => data.audit,
		getAdminDiscountMetrics: async () => data.metrics,
		runAdminDiscountReconciliation: async () => ({
			checked_at: now.toISOString(),
			issues: [],
		}),
		listAdminProducts: async () => ({
			data: data.products,
			pagination: { page: 1, limit: 20, total: data.products.length, total_pages: 1 },
		}),
	});
}

export const Campaigns: Story = {
	render: () => {
		const data = createData();
		return renderRouteStory({
			component: AdminDiscountsPage,
			componentProps: { data },
			api: createDiscountsApiStub(data),
		});
	},
};

export const Empty: Story = {
	render: () => {
		const data = createData({
			campaigns: [],
			templates: [],
			history: { history: [] },
			audit: { audit: [] },
			metrics: { ...metrics, total_evaluations: 0, matched_evaluations: 0 },
		});
		return renderRouteStory({
			component: AdminDiscountsPage,
			componentProps: { data },
			api: createDiscountsApiStub(data),
		});
	},
};

export const LoadError: Story = {
	render: () => {
		const data = createData({
			campaigns: [],
			templates: [],
			errorMessages: ["Unable to load discount and promotion data."],
		});
		return renderRouteStory({
			component: AdminDiscountsPage,
			componentProps: { data },
			api: createDiscountsApiStub(data),
		});
	},
};
