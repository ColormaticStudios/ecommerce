import { expect, test } from "vitest";
import {
	formatProviderCurrencies,
	parseProviderCurrencies,
	parseProviderSecretData,
	providerRunbooks,
	summarizeReconciliationMismatch,
} from "./providers";

test("parseProviderSecretData returns string maps for valid JSON objects", () => {
	const result = parseProviderSecretData('{ "api_key": "sk_test", "merchant_id": "acct_123" }');

	expect(result.error).toBeNull();
	expect(result.value).toEqual({
		api_key: "sk_test",
		merchant_id: "acct_123",
	});
});

test("parseProviderSecretData rejects invalid shapes and non-string values", () => {
	expect(parseProviderSecretData("").error).toBe("Secret data is required.");
	expect(parseProviderSecretData("[]").error).toBe("Secret data must be a JSON object.");
	expect(parseProviderSecretData('{ "api_key": 7 }').error).toBe(
		"Secret data values must be strings."
	);
});

test("parseProviderCurrencies normalizes case and removes duplicates", () => {
	expect(parseProviderCurrencies("usd, eur\nUSD , cad")).toEqual(["USD", "EUR", "CAD"]);
	expect(formatProviderCurrencies(["USD", "EUR"])).toBe("USD, EUR");
});

test("provider runbooks expose the webhook outage and reconciliation mismatch procedures", () => {
	expect(providerRunbooks.map((runbook) => runbook.id)).toEqual([
		"webhook_outage",
		"reconciliation_mismatch",
	]);
	expect(providerRunbooks.every((runbook) => runbook.steps.length >= 5)).toBe(true);
});

test("summarizeReconciliationMismatch classifies provider drift by provider type", () => {
	expect(
		summarizeReconciliationMismatch({
			provider_type: "payment",
			drifts: [{ id: 1 }],
		} as never)
	).toBe(
		"Payment drift usually points to amount or status mismatches between the local ledger and provider transaction truth."
	);
	expect(
		summarizeReconciliationMismatch({
			provider_type: "shipping",
			drifts: [{ id: 1 }],
		} as never)
	).toBe("Shipping drift usually points to shipment status, service, or tracking mismatches.");
	expect(
		summarizeReconciliationMismatch({
			provider_type: "tax",
			drifts: [{ id: 1 }],
		} as never)
	).toBe(
		"Tax drift usually points to snapshot totals or line-level tax results no longer matching checkout inputs."
	);
	expect(summarizeReconciliationMismatch(null)).toBe(
		"Select a reconciliation run to classify the mismatch."
	);
});
