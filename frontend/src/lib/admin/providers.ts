import type { components } from "$lib/api/generated/openapi";

export interface ParsedProviderSecretData {
	value: Record<string, string>;
	error: string | null;
}

export interface ProviderRunbookStep {
	title: string;
	description: string;
}

export interface ProviderRunbook {
	id: "webhook_outage" | "reconciliation_mismatch";
	title: string;
	summary: string;
	steps: ProviderRunbookStep[];
}

type ProviderReconciliationRun = components["schemas"]["ProviderReconciliationRun"];
type ProviderOperation = components["schemas"]["ProviderOperation"];

export interface ProviderOperationGuidance {
	label: string;
	description: string;
	tone: "success" | "danger" | "warning" | "info" | "neutral";
}

export const providerRunbooks: ProviderRunbook[] = [
	{
		id: "webhook_outage",
		title: "Webhook Outage",
		summary:
			"Use backlog and signature-failure signals before replaying provider events or retrying delivery.",
		steps: [
			{
				title: "Confirm scope",
				description:
					"Check pending and dead-letter webhook counts, then note which provider IDs are affected.",
			},
			{
				title: "Check signature failures first",
				description:
					"A spike in rejected webhook events means credentials or environment are mismatched, not that delivery is transiently failing.",
			},
			{
				title: "Verify runtime and credentials",
				description:
					"Confirm the runtime environment is correct and the matching provider credentials exist for that environment.",
			},
			{
				title: "Restore ingestion before replay",
				description:
					"Fix credential or environment mismatches first, then replay provider events and watch attempt counts drain.",
			},
			{
				title: "Pause retries if the queue is losing",
				description:
					"If backlog growth outpaces processing, stop replaying and stabilize instance health before adding more load.",
			},
		],
	},
	{
		id: "reconciliation_mismatch",
		title: "Reconciliation Mismatch",
		summary:
			"Classify the drift, inspect local and provider truth, fix the root cause, then run reconciliation again.",
		steps: [
			{
				title: "Run a targeted report",
				description:
					"Use reconciliation history for the affected provider type and provider ID rather than editing rows directly.",
			},
			{
				title: "Classify the mismatch",
				description:
					"Payment drift is usually amount or status mismatch, shipping drift is shipment or tracking mismatch, and tax drift is snapshot total mismatch.",
			},
			{
				title: "Check local vs provider truth",
				description:
					"Review provider call audit rows, payment or shipment records, and recent webhook activity before mutating anything.",
			},
			{
				title: "Fix root cause first",
				description:
					"Typical causes are wrong runtime environment, missing credential rotation, stale webhook backlog, or incorrect snapshot inputs.",
			},
			{
				title: "Re-run and keep the audit trail",
				description:
					"Do not manually clear drift without a clean follow-up run; the run history is the P4 audit trail.",
			},
		],
	},
];

export function describeProviderOperationState(
	operation: Pick<ProviderOperation, "status" | "available_actions">
): ProviderOperationGuidance {
	if (operation.status === "COMPLETED") {
		return {
			label: "Healthy",
			description: "The provider effect and local finalization are both complete.",
			tone: "success",
		};
	}
	if (operation.status === "COMPENSATION_RETRY") {
		return {
			label: "Compensation failed",
			description:
				"The reversal is not complete. Queue only the existing compensation workflow after reviewing the attempt history.",
			tone: "danger",
		};
	}
	if (operation.status === "FINALIZE_RETRY") {
		return {
			label: "Local finalization blocked",
			description:
				"The provider succeeded, but local state still needs an idempotent finalization retry.",
			tone: "warning",
		};
	}
	if (operation.status === "OUTCOME_UNKNOWN" || operation.status === "RECONCILIATION_REQUIRED") {
		if ((operation.available_actions ?? []).includes("query_outcome")) {
			return {
				label: "Provider outcome unknown",
				description:
					"Query provider truth before any mutation or compensation decision. Never repeat the original operation blindly.",
				tone: "warning",
			};
		}
		return {
			label: "Capability blocker",
			description:
				"Automatic outcome lookup is unavailable. Follow the provider runbook and resolve the case with evidence.",
			tone: "danger",
		};
	}
	return {
		label: "In progress",
		description:
			"The durable workflow is active; wait for its current lease or queued step to finish.",
		tone: "info",
	};
}

export function parseProviderSecretData(raw: string): ParsedProviderSecretData {
	const normalized = raw.trim();
	if (!normalized) {
		return {
			value: {},
			error: "Secret data is required.",
		};
	}

	let parsed: unknown;
	try {
		parsed = JSON.parse(normalized);
	} catch {
		return {
			value: {},
			error: "Secret data must be valid JSON.",
		};
	}

	if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
		return {
			value: {},
			error: "Secret data must be a JSON object.",
		};
	}

	const value: Record<string, string> = {};
	for (const [key, entry] of Object.entries(parsed)) {
		const trimmedKey = key.trim();
		if (!trimmedKey) {
			return {
				value: {},
				error: "Secret data keys must not be blank.",
			};
		}
		if (typeof entry !== "string") {
			return {
				value: {},
				error: "Secret data values must be strings.",
			};
		}
		value[trimmedKey] = entry;
	}

	if (Object.keys(value).length === 0) {
		return {
			value: {},
			error: "Secret data must include at least one key.",
		};
	}

	return { value, error: null };
}

export function parseProviderCurrencies(raw: string): string[] {
	if (!raw.trim()) {
		return [];
	}

	const seen = new Set<string>();
	const values: string[] = [];
	for (const entry of raw.split(/[,\n]/)) {
		const currency = entry.trim().toUpperCase();
		if (!currency || seen.has(currency)) {
			continue;
		}
		seen.add(currency);
		values.push(currency);
	}
	return values;
}

export function formatProviderCurrencies(currencies: string[] | undefined): string {
	return (currencies ?? []).join(", ");
}

export function summarizeReconciliationMismatch(
	run: Pick<ProviderReconciliationRun, "provider_type" | "drifts"> | null
): string {
	if (!run) {
		return "Select a reconciliation run to classify the mismatch.";
	}

	if ((run.drifts ?? []).length === 0) {
		return "No drift was recorded on this run.";
	}

	switch (run.provider_type) {
		case "payment":
			return "Payment drift usually points to amount or status mismatches between the local ledger and provider transaction truth.";
		case "shipping":
			return "Shipping drift usually points to shipment status, service, or tracking mismatches.";
		case "tax":
			return "Tax drift usually points to snapshot totals or line-level tax results no longer matching checkout inputs.";
	}
}
