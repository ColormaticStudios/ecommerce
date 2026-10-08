import { expect, test } from "vitest";
import {
	merchandisingNumber,
	utcDateTime,
	utcDateTimeInput,
	validateSchedule,
} from "./search-merchandising";

test("schedule inputs use UTC without the browser timezone", () => {
	expect(utcDateTime("2026-10-10T12:30")).toBe("2026-10-10T12:30:00.000Z");
	expect(utcDateTimeInput("2026-10-10T14:30:00+02:00")).toBe("2026-10-10T12:30:00");
	expect(validateSchedule("", "")).toEqual({ starts_at: null, ends_at: null });
});

test.each(["invalid", "2026-02-30T12:00", "2026-10-10T25:00", "2026-10-10"])(
	"invalid UTC date %s is rejected",
	(value) => {
		expect(() => utcDateTime(value)).toThrow("valid UTC");
	}
);

test("schedules require an end after the start", () => {
	expect(() => validateSchedule("2026-10-10T12:00", "2026-10-10T12:00")).toThrow("after");
	expect(() => validateSchedule("2026-10-10T12:00", "2026-10-09T12:00")).toThrow("after");
});

test("numeric actions reject invalid values and allow fractional multipliers", () => {
	for (const value of ["", "NaN", "Infinity", "-1", "1.5", "101"]) {
		expect(() => merchandisingNumber(value, "Position", 1, 100)).toThrow();
	}
	expect(merchandisingNumber("2", "Position", 1, 100)).toBe(2);
	expect(merchandisingNumber("1.5", "Multiplier", 0.01, 100, false)).toBe(1.5);
});

import {
	buildMerchandisingInput,
	emptyMerchandisingDraft,
	merchandisingPatch,
	previewRules,
	ruleDraft,
} from "./search-merchandising";
import type { MerchandisingRule } from "$lib/api/domains/search-merchandising";
const rule: MerchandisingRule = {
	id: 1,
	name: "Pin jacket",
	rule_type: "pin",
	priority: 1,
	is_active: false,
	predicate: { query: { mode: "exact", value: "jacket" }, channel: "storefront" },
	action: { targets: [{ product_id: 101, product_name: "Field Jacket", position: 1 }] },
	starts_at: "2026-10-10T12:00:00Z",
	ends_at: null,
	version: 1,
	updated_by: null,
	created_at: "2026-10-01T00:00:00Z",
	updated_at: "2026-10-01T00:00:00Z",
};

test("editing retains product names, inactive state, predicates and UTC schedule", () => {
	const draft = ruleDraft(rule);
	expect(draft.targets[0].name).toBe("Field Jacket");
	const input = buildMerchandisingInput(draft);
	expect(input).toMatchObject({
		is_active: false,
		predicate: rule.predicate,
		action: { targets: [{ product_id: 101, position: 1 }] },
		starts_at: "2026-10-10T12:00:00.000Z",
	});
	expect(input.action.targets[0]).not.toHaveProperty("product_name");
});

test("clearing schedules uses explicit PATCH clear fields", () => {
	const draft = ruleDraft(rule);
	draft.startsAt = "";
	const patch = merchandisingPatch(buildMerchandisingInput(draft));
	expect(patch.starts_at).toBeUndefined();
	expect(patch.clear_starts_at).toBe(true);
	expect(patch.clear_ends_at).toBe(true);
});

test("pin validates distinct positions, products and required fields", () => {
	const draft = emptyMerchandisingDraft();
	expect(() => buildMerchandisingInput(draft)).toThrow("name");
	draft.name = "Pin";
	expect(() => buildMerchandisingInput(draft)).toThrow("product");
	draft.targets = [
		{ productId: 1, name: "One", position: "1" },
		{ productId: 2, name: "Two", position: "1" },
	];
	expect(() => buildMerchandisingInput(draft)).toThrow("distinct");
	draft.targets[1].productId = 1;
	expect(() => buildMerchandisingInput(draft)).toThrow("only once");
});

test("boost validates multiplier and removes pin-only positions", () => {
	const draft = ruleDraft(rule);
	draft.ruleType = "boost";
	draft.multiplier = "1";
	expect(() => buildMerchandisingInput(draft)).toThrow("greater than 1");
	draft.multiplier = "1.5";
	expect(buildMerchandisingInput(draft).action).toEqual({
		targets: [{ product_id: 101 }],
		multiplier: 1.5,
	});
});

test("unsaved preview replaces the editing rule and retains other identities without persisting", () => {
	const draft = ruleDraft(rule);
	draft.name = "Unsaved name";
	const other = { ...rule, id: 2, name: "Other" };
	const input = buildMerchandisingInput(draft);
	const replacement = previewRules([rule, other], input, rule.id);
	expect(replacement.map((row) => row.id)).toEqual([2, 1]);
	expect(replacement[1].name).toBe("Unsaved name");
	expect(rule.name).toBe("Pin jacket");
	expect(previewRules([rule], input, null)[1].id).toBeUndefined();
});

test("schedule editing preserves seconds and milliseconds", () => {
	const original = "2026-10-10T12:30:45.250Z";
	expect(utcDateTime(utcDateTimeInput(original))).toBe(original);
});
