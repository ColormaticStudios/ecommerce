import { expect, test } from "vitest";
import {
	merchandisingNumber,
	localDateTime,
	localDateTimeInput,
	validateSchedule,
} from "./search-merchandising";

test("schedule inputs use local timezone and persist UTC", () => {
	const expected = new Date(2026, 9, 10, 12, 30).toISOString();
	expect(localDateTime("2026-10-10T12:30")).toBe(expected);
	expect(localDateTime(localDateTimeInput("2026-10-10T14:30:00+02:00"))).toBe(
		"2026-10-10T12:30:00.000Z"
	);
	expect(validateSchedule("", "")).toEqual({ starts_at: null, ends_at: null });
});

test.each(["invalid", "2026-02-30T12:00", "2026-10-10T25:00", "2026-10-10"])(
	"invalid local date %s is rejected",
	(value) => {
		expect(() => localDateTime(value)).toThrow("valid local");
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
	expect(localDateTime(localDateTimeInput(original))).toBe(original);
});
test("local schedules account for summer offset and reject nonexistent DST wall time", () => {
	const previous = process.env.TZ;
	process.env.TZ = "Europe/Paris";
	try {
		expect(localDateTime("2026-10-10T12:30")).toBe("2026-10-10T10:30:00.000Z");
		expect(localDateTime("2026-12-10T12:30")).toBe("2026-12-10T11:30:00.000Z");
		expect(() => localDateTime("2026-03-29T02:30")).toThrow("valid local");
	} finally {
		if (previous === undefined) delete process.env.TZ;
		else process.env.TZ = previous;
	}
});

test("validation and missing product names use the supplied localization runtime", () => {
	const messages = { translate: (key: string) => `translated:${key}` };
	expect(() => buildMerchandisingInput(emptyMerchandisingDraft(), messages)).toThrow(
		"translated:admin.search.merchandising.required_name"
	);
	expect(() => localDateTime("invalid", messages)).toThrow(
		"translated:admin.search.merchandising.invalid_time"
	);
	expect(
		ruleDraft({ ...rule, action: { targets: [{ product_id: 1, product_name: "" }] } }, messages)
			.targets[0].name
	).toBe("translated:admin.search.merchandising.unavailable_product");
});

import { formatMerchandisingSchedule } from "./search-merchandising";
import { createFallbackLocalization } from "$lib/localization/runtime";

test("rule list and history show the browser zone rather than UTC or server time", () => {
	const messages = createFallbackLocalization();
	const instant = "2026-11-01T00:00:00Z";
	expect(formatMerchandisingSchedule(instant, "America/Los_Angeles", messages)).toBe(
		"Oct 31, 2026, 5:00 PM"
	);
	expect(formatMerchandisingSchedule(instant, "Europe/Berlin", messages)).toBe(
		"Nov 1, 2026, 1:00 AM"
	);
	expect(formatMerchandisingSchedule(instant, "", messages)).toBe("Loading local time…");
	expect(formatMerchandisingSchedule(null, "America/Los_Angeles", messages)).toBe("Open ended");
});
