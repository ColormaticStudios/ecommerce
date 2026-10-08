export function utcDateTimeInput(value: string | null | undefined): string {
	if (!value) return "";
	const date = new Date(value);
	return Number.isFinite(date.getTime()) ? date.toISOString().replace(/(?:\.000)?Z$/, "") : "";
}

export function utcDateTime(value: string): string | null {
	if (!value.trim()) return null;
	if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(?::\d{2}(?:\.\d{1,3})?)?$/.test(value)) {
		throw new Error("Enter a valid UTC date and time.");
	}
	const date = new Date(`${value}Z`);
	if (!Number.isFinite(date.getTime()) || date.toISOString().slice(0, value.length) !== value) {
		throw new Error("Enter a valid UTC date and time.");
	}
	return date.toISOString();
}

export function validateSchedule(startsAt: string, endsAt: string) {
	const starts_at = utcDateTime(startsAt);
	const ends_at = utcDateTime(endsAt);
	if (starts_at && ends_at && starts_at >= ends_at) {
		throw new Error("End time must be after start time.");
	}
	return { starts_at, ends_at };
}

export function merchandisingNumber(
	value: string | number,
	label: string,
	min: number,
	max: number,
	integer = true
): number {
	const number = typeof value === "string" && !value.trim() ? NaN : Number(value);
	if (
		!Number.isFinite(number) ||
		number < min ||
		number > max ||
		(integer && !Number.isInteger(number))
	) {
		throw new Error(
			`${label} must be ${integer ? "a whole number" : "a number"} between ${min} and ${max}.`
		);
	}
	return number;
}

import type {
	MerchandisingInput,
	MerchandisingPatch,
	MerchandisingRule,
	MerchandisingPreviewRequest,
} from "$lib/api/domains/search-merchandising";

export interface MerchandisingDraft {
	name: string;
	ruleType: MerchandisingInput["rule_type"];
	priority: string | number;
	isActive: boolean;
	queryMode: "exact" | "prefix" | "contains";
	queryValue: string;
	categorySlugs: string[];
	channel: "" | "storefront";
	targets: Array<{ productId: number; name: string; position: string | number }>;
	multiplier: string | number;
	startsAt: string;
	endsAt: string;
}

export function emptyMerchandisingDraft(): MerchandisingDraft {
	return {
		name: "",
		ruleType: "pin",
		priority: "0",
		isActive: true,
		queryMode: "exact",
		queryValue: "",
		categorySlugs: [],
		channel: "storefront",
		targets: [],
		multiplier: "2",
		startsAt: "",
		endsAt: "",
	};
}

export function ruleDraft(rule: MerchandisingRule): MerchandisingDraft {
	return {
		name: rule.name,
		ruleType: rule.rule_type,
		priority: String(rule.priority),
		isActive: rule.is_active,
		queryMode: rule.predicate.query?.mode ?? "exact",
		queryValue: rule.predicate.query?.value ?? "",
		categorySlugs: [...(rule.predicate.category_slugs ?? [])],
		channel: rule.predicate.channel ?? "",
		targets: rule.action.targets.map((target) => ({
			productId: target.product_id,
			name: target.product_name || "Unavailable product",
			position: String(target.position ?? 1),
		})),
		multiplier: String(rule.action.multiplier ?? 2),
		startsAt: utcDateTimeInput(rule.starts_at),
		endsAt: utcDateTimeInput(rule.ends_at),
	};
}

export function buildMerchandisingInput(draft: MerchandisingDraft): MerchandisingInput {
	if (!draft.name.trim()) throw new Error("Rule name is required.");
	if (draft.name.trim().length > 120) throw new Error("Rule name must be 120 characters or fewer.");
	if (draft.targets.length === 0) throw new Error("Select at least one product.");
	if (new Set(draft.targets.map((target) => target.productId)).size !== draft.targets.length)
		throw new Error("Select each product only once.");
	const targets = draft.targets.map((target) => ({
		product_id: target.productId,
		...(draft.ruleType === "pin"
			? { position: merchandisingNumber(target.position, "Position", 1, 10000) }
			: {}),
	}));
	if (
		draft.ruleType === "pin" &&
		new Set(targets.map((target) => target.position)).size !== targets.length
	)
		throw new Error("Pinned positions must be distinct within a rule.");
	const multiplier =
		draft.ruleType === "boost"
			? merchandisingNumber(draft.multiplier, "Multiplier", 1, 100, false)
			: undefined;
	if (multiplier !== undefined && multiplier <= 1)
		throw new Error("Boost multiplier must be greater than 1.");
	const schedule = validateSchedule(draft.startsAt, draft.endsAt);
	return {
		name: draft.name.trim(),
		rule_type: draft.ruleType,
		priority: merchandisingNumber(draft.priority, "Priority", 0, 10000),
		is_active: draft.isActive,
		predicate: {
			...(draft.queryValue.trim()
				? { query: { mode: draft.queryMode, value: draft.queryValue.trim() } }
				: {}),
			...(draft.categorySlugs.length ? { category_slugs: draft.categorySlugs } : {}),
			...(draft.channel ? { channel: draft.channel } : {}),
		},
		action: { targets, ...(multiplier !== undefined ? { multiplier } : {}) },
		starts_at: schedule.starts_at ?? undefined,
		ends_at: schedule.ends_at ?? undefined,
	};
}

export function merchandisingPatch(input: MerchandisingInput): MerchandisingPatch {
	return {
		...input,
		starts_at: input.starts_at ?? undefined,
		ends_at: input.ends_at ?? undefined,
		clear_starts_at: !input.starts_at,
		clear_ends_at: !input.ends_at,
	};
}

export function previewRules(
	rules: MerchandisingRule[],
	draft: MerchandisingInput,
	editingId: number | null
): NonNullable<MerchandisingPreviewRequest["rules"]> {
	const retained = rules
		.filter((rule) => rule.id !== editingId)
		.map((rule) => ({
			id: rule.id,
			name: rule.name,
			rule_type: rule.rule_type,
			priority: rule.priority,
			is_active: rule.is_active,
			predicate: rule.predicate,
			action: {
				targets: rule.action.targets.map((target) => ({
					product_id: target.product_id,
					...(target.position !== undefined ? { position: target.position } : {}),
				})),
				...(rule.action.multiplier !== undefined ? { multiplier: rule.action.multiplier } : {}),
			},
			starts_at: rule.starts_at ?? undefined,
			ends_at: rule.ends_at ?? undefined,
		}));
	return [...retained, { ...draft, ...(editingId ? { id: editingId } : {}) }];
}
