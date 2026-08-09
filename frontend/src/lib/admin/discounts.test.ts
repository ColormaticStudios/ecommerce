import { expect, test } from "vitest";
import {
	buildAction,
	buildCondition,
	buildProductDiscountInput,
	buildScheduleInput,
	buildTargets,
	localDateFromISO,
	parseIdList,
	parseOptionalPositiveInt,
} from "./discounts";

test("parseIdList keeps valid unique positive ids", () => {
	expect(parseIdList("1, 2 2\n3, nope, -4, 5.5")).toEqual([1, 2, 3]);
});

test("parseOptionalPositiveInt rejects invalid caps", () => {
	expect(parseOptionalPositiveInt("")).toBeNull();
	expect(parseOptionalPositiveInt("0")).toBeNull();
	expect(parseOptionalPositiveInt("3.5")).toBeNull();
	expect(parseOptionalPositiveInt("7")).toBe(7);
});

test("buildCondition omits empty invalid fields", () => {
	expect(
		buildCondition({
			productIds: "10,11",
			variantIds: "",
			categoryIds: "4",
			brandIds: "bad",
			minQuantity: "2",
			minSubtotal: "50",
		})
	).toEqual({
		product_ids: [10, 11],
		category_ids: [4],
		min_quantity: 2,
		min_subtotal: 50,
	});
});

test("buildAction maps target ids and optional free item sku", () => {
	expect(
		buildAction({
			mode: "free_item",
			value: "",
			targetType: "cart",
			targetIds: "",
			productIds: "9",
			variantIds: "",
			categoryIds: "",
			brandIds: "",
			sku: "BONUS-SKU",
		})
	).toEqual({
		mode: "free_item",
		target_type: "cart",
		product_ids: [9],
		sku: "BONUS-SKU",
	});
});

test("buildTargets returns typed target entries", () => {
	expect(buildTargets("category", "3, 4")).toEqual([
		{ target_type: "category", target_id: 3 },
		{ target_type: "category", target_id: 4 },
	]);
});

test("buildScheduleInput validates required dates", () => {
	expect(
		buildScheduleInput({
			scheduleType: "one_time",
			recurrence: "",
			windowStart: "",
			windowEnd: "2026-05-21T09:00",
			untilAt: "",
			timezone: "UTC",
		})
	).toBeNull();

	const schedule = buildScheduleInput({
		scheduleType: "recurring",
		recurrence: "weekly",
		windowStart: "2026-05-21T09:00",
		windowEnd: "2026-05-21T17:00",
		untilAt: "",
		timezone: "UTC",
	});
	expect(schedule?.schedule_type).toBe("recurring");
	expect(schedule?.recurrence).toBe("weekly");
	expect(schedule?.timezone).toBe("UTC");
});

test("buildProductDiscountInput preserves an end date", () => {
	const { payload, error } = buildProductDiscountInput({
		name: "  5% off  ",
		productIds: [3],
		discountMode: "percent",
		discountValue: "5",
		startsAt: "2026-05-23T21:28",
		endsAt: "2026-05-24T21:28",
		priority: "0",
		isExclusive: false,
		status: "active",
		couponCode: "",
		channels: ["web"],
		customerSegment: "",
		globalUsageCap: "",
		perCustomerUsageCap: "",
	});

	expect(error).toBeNull();
	expect(payload?.name).toBe("5% off");
	expect(payload?.product_ids).toEqual([3]);
	expect(payload?.ends_at).toBe(new Date("2026-05-24T21:28").toISOString());
});

test("buildProductDiscountInput treats date-only end dates as local midnight", () => {
	const { payload, error } = buildProductDiscountInput({
		name: "5% off",
		productIds: [3],
		discountMode: "percent",
		discountValue: "5",
		startsAt: "2026-05-23T21:28",
		endsAt: "2026-05-24",
		priority: "0",
		isExclusive: false,
		status: "active",
		couponCode: "",
		channels: ["web"],
		customerSegment: "",
		globalUsageCap: "",
		perCustomerUsageCap: "",
	});

	expect(error).toBeNull();
	expect(payload?.ends_at).toBe(new Date("2026-05-24T00:00").toISOString());
});

test("localDateFromISO formats existing end dates for the date input", () => {
	expect(localDateFromISO(new Date("2026-05-24T21:28").toISOString())).toBe("2026-05-24");
	expect(localDateFromISO(null)).toBe("");
});

test("buildProductDiscountInput keeps open-ended discounts explicit", () => {
	const { payload, error } = buildProductDiscountInput({
		name: "5% off",
		productIds: [3],
		discountMode: "percent",
		discountValue: "5",
		startsAt: "2026-05-23T21:28",
		endsAt: "",
		priority: "0",
		isExclusive: false,
		status: "active",
		couponCode: "",
		channels: ["web"],
		customerSegment: "",
		globalUsageCap: "",
		perCustomerUsageCap: "",
	});

	expect(error).toBeNull();
	expect(payload?.ends_at).toBeNull();
});

test("buildProductDiscountInput rejects invalid end dates instead of dropping them", () => {
	const { payload, error } = buildProductDiscountInput({
		name: "5% off",
		productIds: [3],
		discountMode: "percent",
		discountValue: "5",
		startsAt: "2026-05-23T21:28",
		endsAt: "not-a-date",
		priority: "0",
		isExclusive: false,
		status: "active",
		couponCode: "",
		channels: ["web"],
		customerSegment: "",
		globalUsageCap: "",
		perCustomerUsageCap: "",
	});

	expect(payload).toBeNull();
	expect(error).toBe("End date must be a valid date and time.");
});

test("buildProductDiscountInput rejects end dates before the start date", () => {
	const { payload, error } = buildProductDiscountInput({
		name: "5% off",
		productIds: [3],
		discountMode: "percent",
		discountValue: "5",
		startsAt: "2026-05-23T21:28",
		endsAt: "2026-05-23T21:27",
		priority: "0",
		isExclusive: false,
		status: "active",
		couponCode: "",
		channels: ["web"],
		customerSegment: "",
		globalUsageCap: "",
		perCustomerUsageCap: "",
	});

	expect(payload).toBeNull();
	expect(error).toBe("End date must be after the start date.");
});
