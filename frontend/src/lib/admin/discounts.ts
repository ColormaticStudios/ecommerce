import type { components } from "$lib/api/generated/openapi";

export type ProductDiscountInput = components["schemas"]["ProductDiscountInput"];
export type PromotionInput = components["schemas"]["PromotionInput"];
export type PromotionCondition = components["schemas"]["PromotionCondition"];
export type PromotionAction = components["schemas"]["PromotionAction"];
export type PromotionRuleInput = components["schemas"]["PromotionRuleInput"];
export type PromotionLevelInput = components["schemas"]["PromotionLevelInput"];
export type PromotionTargetInput = components["schemas"]["PromotionTargetInput"];
export type DiscountScheduleInput = components["schemas"]["DiscountScheduleInput"];

export function parseIdList(value: string): number[] {
	const ids = value
		.split(/[\s,]+/)
		.map((entry) => Number(entry.trim()))
		.filter((entry) => Number.isInteger(entry) && entry > 0);
	return Array.from(new Set(ids));
}

export function parseOptionalPositiveInt(value: string): number | null {
	const trimmed = value.trim();
	if (!trimmed) {
		return null;
	}
	const parsed = Number(trimmed);
	return Number.isInteger(parsed) && parsed > 0 ? parsed : null;
}

export function parseOptionalMoney(value: string): number | undefined {
	const trimmed = value.trim();
	if (!trimmed) {
		return undefined;
	}
	const parsed = Number(trimmed);
	return Number.isFinite(parsed) && parsed >= 0 ? parsed : undefined;
}

export function isoFromLocalDateTime(value: string): string | null {
	const trimmed = value.trim();
	if (!trimmed) {
		return null;
	}
	const normalized = /^\d{4}-\d{2}-\d{2}$/.test(trimmed) ? `${trimmed}T00:00` : trimmed;
	const parsed = new Date(normalized);
	return Number.isNaN(parsed.valueOf()) ? null : parsed.toISOString();
}

export function localDateTimeFromISO(value: string | null | undefined): string {
	if (!value) {
		return "";
	}
	const parsed = new Date(value);
	if (Number.isNaN(parsed.valueOf())) {
		return "";
	}
	const offsetMs = parsed.getTimezoneOffset() * 60 * 1000;
	return new Date(parsed.getTime() - offsetMs).toISOString().slice(0, 16);
}

export function localDateFromISO(value: string | null | undefined): string {
	if (!value) {
		return "";
	}
	const parsed = new Date(value);
	if (Number.isNaN(parsed.valueOf())) {
		return "";
	}
	const offsetMs = parsed.getTimezoneOffset() * 60 * 1000;
	return new Date(parsed.getTime() - offsetMs).toISOString().slice(0, 10);
}

export function compactString(value: string): string | undefined {
	const trimmed = value.trim();
	return trimmed || undefined;
}

export function nullableString(value: string): string | null | undefined {
	const trimmed = value.trim();
	return trimmed || undefined;
}

export function buildCondition(input: {
	productIds: string;
	variantIds: string;
	categoryIds: string;
	brandIds: string;
	minQuantity: string;
	minSubtotal: string;
}): PromotionCondition {
	const condition: PromotionCondition = {};
	const product_ids = parseIdList(input.productIds);
	const product_variant_ids = parseIdList(input.variantIds);
	const category_ids = parseIdList(input.categoryIds);
	const brand_ids = parseIdList(input.brandIds);
	const min_quantity = parseOptionalPositiveInt(input.minQuantity);
	const min_subtotal = parseOptionalMoney(input.minSubtotal);

	if (product_ids.length) condition.product_ids = product_ids;
	if (product_variant_ids.length) condition.product_variant_ids = product_variant_ids;
	if (category_ids.length) condition.category_ids = category_ids;
	if (brand_ids.length) condition.brand_ids = brand_ids;
	if (min_quantity !== null) condition.min_quantity = min_quantity;
	if (min_subtotal !== undefined && min_subtotal > 0) condition.min_subtotal = min_subtotal;

	return condition;
}

export function buildAction(input: {
	mode: PromotionAction["mode"];
	value: string;
	targetType: NonNullable<PromotionAction["target_type"]>;
	targetIds: string;
	productIds: string;
	variantIds: string;
	categoryIds: string;
	brandIds: string;
	sku: string;
}): PromotionAction {
	const action: PromotionAction = { mode: input.mode };
	const value = parseOptionalMoney(input.value);
	const target_ids = parseIdList(input.targetIds);
	const product_ids = parseIdList(input.productIds);
	const product_variant_ids = parseIdList(input.variantIds);
	const category_ids = parseIdList(input.categoryIds);
	const brand_ids = parseIdList(input.brandIds);

	if (value !== undefined) action.value = value;
	if (input.targetType) action.target_type = input.targetType;
	if (target_ids.length) action.target_ids = target_ids;
	if (product_ids.length) action.product_ids = product_ids;
	if (product_variant_ids.length) action.product_variant_ids = product_variant_ids;
	if (category_ids.length) action.category_ids = category_ids;
	if (brand_ids.length) action.brand_ids = brand_ids;
	if (compactString(input.sku)) action.sku = input.sku.trim();

	return action;
}

export function buildTargets(targetType: PromotionTargetInput["target_type"], ids: string) {
	return parseIdList(ids).map((target_id) => ({ target_type: targetType, target_id }));
}

export function buildScheduleInput(input: {
	scheduleType: DiscountScheduleInput["schedule_type"];
	recurrence: NonNullable<DiscountScheduleInput["recurrence"]> | "";
	windowStart: string;
	windowEnd: string;
	untilAt: string;
	timezone: string;
}): DiscountScheduleInput | null {
	const window_start = isoFromLocalDateTime(input.windowStart);
	const window_end = isoFromLocalDateTime(input.windowEnd);
	if (!window_start || !window_end) {
		return null;
	}

	return {
		schedule_type: input.scheduleType,
		recurrence: input.scheduleType === "recurring" ? input.recurrence || "daily" : undefined,
		window_start,
		window_end,
		until_at: isoFromLocalDateTime(input.untilAt),
		timezone: input.timezone.trim() || Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
	};
}

export function buildProductDiscountInput(input: {
	name: string;
	productIds: number[];
	discountMode: ProductDiscountInput["discount_mode"];
	discountValue: string;
	startsAt: string;
	endsAt: string;
	priority: string;
	isExclusive: boolean;
	status: NonNullable<ProductDiscountInput["status"]>;
	couponCode: string;
	channels: Array<"web" | "app" | "admin">;
	customerSegment: string;
	globalUsageCap: string;
	perCustomerUsageCap: string;
}): { payload: ProductDiscountInput | null; error: string | null } {
	const starts_at = isoFromLocalDateTime(input.startsAt);
	const name = input.name.trim();
	if (!name) {
		return { payload: null, error: "Campaign name is required." };
	}
	if (input.productIds.length === 0) {
		return { payload: null, error: "Select at least one product." };
	}
	if (!starts_at) {
		return { payload: null, error: "Start date is required." };
	}
	const ends_at = isoFromLocalDateTime(input.endsAt);
	if (input.endsAt.trim() && !ends_at) {
		return { payload: null, error: "End date must be a valid date and time." };
	}
	if (ends_at && new Date(ends_at) <= new Date(starts_at)) {
		return { payload: null, error: "End date must be after the start date." };
	}
	const value = Number(input.discountValue);
	if (!Number.isFinite(value) || value <= 0) {
		return { payload: null, error: "Discount value must be positive." };
	}

	return {
		payload: {
			name,
			product_ids: input.productIds,
			discount_mode: input.discountMode,
			discount_value: value,
			starts_at,
			ends_at,
			priority: Number(input.priority || 0),
			is_exclusive: input.isExclusive,
			status: input.status,
			coupon_code: compactString(input.couponCode) ?? null,
			channels: input.channels,
			customer_segment: compactString(input.customerSegment),
			global_usage_cap: parseOptionalPositiveInt(input.globalUsageCap),
			per_customer_usage_cap: parseOptionalPositiveInt(input.perCustomerUsageCap),
		},
		error: null,
	};
}
