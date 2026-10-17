import type { LocalizationRuntime } from "$lib/localization/runtime";

type Messages = Pick<LocalizationRuntime, "translate">;
const sourceMessages: Messages = {
	translate: (_key, source = "", parameters = {}) =>
		source.replace(/\{([a-zA-Z][a-zA-Z0-9_]*)\}/g, (match, name: string) =>
			parameters[name] === undefined ? match : String(parameters[name])
		),
};

export function localDateTimeInput(value: string | null | undefined): string {
	if (!value) return "";
	const date = new Date(value);
	if (!Number.isFinite(date.getTime())) return "";
	const pad = (part: number, length = 2) => String(part).padStart(length, "0");
	return `${pad(date.getFullYear(), 4)}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}.${pad(date.getMilliseconds(), 3)}`;
}

export function localDateTime(value: string, messages: Messages = sourceMessages): string | null {
	if (!value.trim()) return null;
	const parts = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2})(?:\.(\d{1,3}))?)?$/.exec(
		value
	);
	if (!parts)
		throw new Error(
			messages.translate(
				"admin.search.merchandising.invalid_time",
				"Enter a valid local date and time."
			)
		);
	const date = new Date(value);
	const expected = [
		Number(parts[1]),
		Number(parts[2]) - 1,
		Number(parts[3]),
		Number(parts[4]),
		Number(parts[5]),
		Number(parts[6] ?? 0),
		Number((parts[7] ?? "0").padEnd(3, "0")),
	];
	const actual = [
		date.getFullYear(),
		date.getMonth(),
		date.getDate(),
		date.getHours(),
		date.getMinutes(),
		date.getSeconds(),
		date.getMilliseconds(),
	];
	// Reject calendar overflow and nonexistent local times at daylight-saving transitions.
	if (!Number.isFinite(date.getTime()) || expected.some((part, index) => part !== actual[index])) {
		throw new Error(
			messages.translate(
				"admin.search.merchandising.invalid_time",
				"Enter a valid local date and time."
			)
		);
	}
	return date.toISOString();
}

export function validateSchedule(
	startsAt: string,
	endsAt: string,
	messages: Messages = sourceMessages
) {
	const starts_at = localDateTime(startsAt, messages);
	const ends_at = localDateTime(endsAt, messages);
	if (starts_at && ends_at && starts_at >= ends_at) {
		throw new Error(
			messages.translate(
				"admin.search.merchandising.invalid_schedule",
				"End time must be after start time."
			)
		);
	}
	return { starts_at, ends_at };
}

export function merchandisingNumber(
	value: string | number,
	label: string,
	min: number,
	max: number,
	integer = true,
	messages: Messages = sourceMessages
): number {
	const number = typeof value === "string" && !value.trim() ? NaN : Number(value);
	if (
		!Number.isFinite(number) ||
		number < min ||
		number > max ||
		(integer && !Number.isInteger(number))
	) {
		throw new Error(
			integer
				? messages.translate(
						"admin.search.merchandising.invalid_integer",
						"{label} must be a whole number between {min} and {max}.",
						{ label, min, max }
					)
				: messages.translate(
						"admin.search.merchandising.invalid_number",
						"{label} must be a number between {min} and {max}.",
						{ label, min, max }
					)
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

export function ruleDraft(
	rule: MerchandisingRule,
	messages: Messages = sourceMessages
): MerchandisingDraft {
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
			name:
				target.product_name ||
				messages.translate("admin.search.merchandising.unavailable_product", "Unavailable product"),
			position: String(target.position ?? 1),
		})),
		multiplier: String(rule.action.multiplier ?? 2),
		startsAt: localDateTimeInput(rule.starts_at),
		endsAt: localDateTimeInput(rule.ends_at),
	};
}

export function buildMerchandisingInput(
	draft: MerchandisingDraft,
	messages: Messages = sourceMessages
): MerchandisingInput {
	if (!draft.name.trim())
		throw new Error(
			messages.translate("admin.search.merchandising.required_name", "Rule name is required.")
		);
	if (draft.name.trim().length > 120)
		throw new Error(
			messages.translate(
				"admin.search.merchandising.long_name",
				"Rule name must be 120 characters or fewer."
			)
		);
	if (draft.targets.length === 0)
		throw new Error(
			messages.translate(
				"admin.search.merchandising.required_target",
				"Select at least one product."
			)
		);
	if (new Set(draft.targets.map((target) => target.productId)).size !== draft.targets.length)
		throw new Error(
			messages.translate(
				"admin.search.merchandising.duplicate_product",
				"Select each product only once."
			)
		);
	const targets = draft.targets.map((target) => ({
		product_id: target.productId,
		...(draft.ruleType === "pin"
			? {
					position: merchandisingNumber(
						target.position,
						messages.translate("admin.search.merchandising.position", "Position"),
						1,
						10000,
						true,
						messages
					),
				}
			: {}),
	}));
	if (
		draft.ruleType === "pin" &&
		new Set(targets.map((target) => target.position)).size !== targets.length
	)
		throw new Error(
			messages.translate(
				"admin.search.merchandising.duplicate_position",
				"Pinned positions must be distinct within a rule."
			)
		);
	const multiplier =
		draft.ruleType === "boost"
			? merchandisingNumber(
					draft.multiplier,
					messages.translate("admin.search.merchandising.multiplier", "Multiplier"),
					1,
					100,
					false,
					messages
				)
			: undefined;
	if (multiplier !== undefined && multiplier <= 1)
		throw new Error(
			messages.translate(
				"admin.search.merchandising.invalid_multiplier",
				"Boost multiplier must be greater than 1."
			)
		);
	const schedule = validateSchedule(draft.startsAt, draft.endsAt, messages);
	return {
		name: draft.name.trim(),
		rule_type: draft.ruleType,
		priority: merchandisingNumber(
			draft.priority,
			messages.translate("admin.search.merchandising.priority", "Priority"),
			0,
			10000,
			true,
			messages
		),
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

export function formatMerchandisingSchedule(
	value: string | null,
	timezone: string,
	messages: Pick<LocalizationRuntime, "translate" | "formatDate">
): string {
	if (!value) return messages.translate("admin.search.merchandising.open_ended", "Open ended");
	if (!timezone)
		return messages.translate("admin.search.merchandising.loading_time", "Loading local time…");
	return messages.formatDate(value, {
		dateStyle: "medium",
		timeStyle: "short",
		timeZone: timezone,
	});
}
