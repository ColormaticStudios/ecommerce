import assert from "node:assert/strict";
import test from "node:test";
import {
	formatProviderCurrencies,
	parseProviderCurrencies,
	parseProviderSecretData,
	providerRunbooks,
	summarizeReconciliationMismatch,
} from "./providers";

test("parseProviderSecretData returns string maps for valid JSON objects", () => {
	const result = parseProviderSecretData('{ "api_key": "sk_test", "merchant_id": "acct_123" }');

	assert.equal(result.error, null);
	assert.deepEqual(result.value, {
		api_key: "sk_test",
		merchant_id: "acct_123",
	});
});

test("parseProviderSecretData rejects invalid shapes and non-string values", () => {
	assert.equal(parseProviderSecretData("").error, "Secret data is required.");
	assert.equal(parseProviderSecretData("[]").error, "Secret data must be a JSON object.");
	assert.equal(
		parseProviderSecretData('{ "api_key": 7 }').error,
		"Secret data values must be strings."
	);
});

test("parseProviderCurrencies normalizes case and removes duplicates", () => {
	assert.deepEqual(parseProviderCurrencies("usd, eur\nUSD , cad"), ["USD", "EUR", "CAD"]);
	assert.equal(formatProviderCurrencies(["USD", "EUR"]), "USD, EUR");
});

test("provider runbooks expose the webhook outage and reconciliation mismatch procedures", () => {
	assert.deepEqual(
		providerRunbooks.map((runbook) => runbook.id),
		["webhook_outage", "reconciliation_mismatch"]
	);
	assert.ok(providerRunbooks.every((runbook) => runbook.steps.length >= 5));
});

test("summarizeReconciliationMismatch classifies provider drift by provider type", () => {
	assert.equal(
		summarizeReconciliationMismatch({
			provider_type: "payment",
			drifts: [{ id: 1 }],
		} as never),
		"Payment drift usually points to amount or status mismatches between the local ledger and provider transaction truth."
	);
	assert.equal(
		summarizeReconciliationMismatch({
			provider_type: "shipping",
			drifts: [{ id: 1 }],
		} as never),
		"Shipping drift usually points to shipment status, service, or tracking mismatches."
	);
	assert.equal(
		summarizeReconciliationMismatch({
			provider_type: "tax",
			drifts: [{ id: 1 }],
		} as never),
		"Tax drift usually points to snapshot totals or line-level tax results no longer matching checkout inputs."
	);
	assert.equal(
		summarizeReconciliationMismatch(null),
		"Select a reconciliation run to classify the mismatch."
	);
});
