import type { PageServerLoad } from "./$types";
import { serverRequest } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";

type CheckoutPluginCatalogPayload = components["schemas"]["CheckoutPluginCatalog"];
type ProviderCredentialListPayload = components["schemas"]["ProviderCredentialListResponse"];
type ProviderOperationsOverviewPayload = components["schemas"]["ProviderOperationsOverview"];
type ProviderReconciliationRunPagePayload = components["schemas"]["ProviderReconciliationRunPage"];
type WebhookEventPagePayload = components["schemas"]["WebhookEventPage"];

const emptyCheckoutPlugins: CheckoutPluginCatalogPayload = { payment: [], shipping: [], tax: [] };
const emptyProviderOverview: ProviderOperationsOverviewPayload = {
	runtime_environment: "sandbox",
	credential_service_configured: false,
	webhook_events: {
		pending_count: 0,
		processed_count: 0,
		dead_letter_count: 0,
		rejected_count: 0,
	},
};
const emptyReconciliationRuns: ProviderReconciliationRunPagePayload = {
	data: [],
	pagination: {
		page: 1,
		limit: 10,
		total: 0,
		total_pages: 0,
	},
};
const emptyWebhookEvents: WebhookEventPagePayload = {
	data: [],
	pagination: {
		page: 1,
		limit: 5,
		total: 0,
		total_pages: 0,
	},
};

export const load: PageServerLoad = async (event) => {
	const { isAdmin } = await event.parent();

	let checkoutPlugins: CheckoutPluginCatalogPayload = emptyCheckoutPlugins;
	let providerCredentials = [] as ProviderCredentialListPayload["data"];
	let providerOverview: ProviderOperationsOverviewPayload = emptyProviderOverview;
	let reconciliationRuns: ProviderReconciliationRunPagePayload = emptyReconciliationRuns;
	let rejectedWebhookEvents: WebhookEventPagePayload = emptyWebhookEvents;
	let deadLetterWebhookEvents: WebhookEventPagePayload = emptyWebhookEvents;
	const errorMessages: string[] = [];

	if (!isAdmin) {
		return {
			checkoutPlugins,
			providerCredentials,
			providerOverview,
			reconciliationRuns,
			rejectedWebhookEvents,
			deadLetterWebhookEvents,
			errorMessages,
		};
	}

	const [
		checkoutPluginsResult,
		providerCredentialsResult,
		providerOverviewResult,
		reconciliationRunsResult,
		rejectedWebhookEventsResult,
		deadLetterWebhookEventsResult,
	] = await Promise.allSettled([
		serverRequest<CheckoutPluginCatalogPayload>(event, "/admin/checkout/plugins"),
		serverRequest<ProviderCredentialListPayload>(event, "/admin/providers/credentials"),
		serverRequest<ProviderOperationsOverviewPayload>(event, "/admin/providers/overview"),
		serverRequest<ProviderReconciliationRunPagePayload>(
			event,
			"/admin/providers/reconciliation/runs",
			{ page: 1, limit: 10 }
		),
		serverRequest<WebhookEventPagePayload>(event, "/admin/webhooks/events", {
			status: "REJECTED",
			page: 1,
			limit: 5,
		}),
		serverRequest<WebhookEventPagePayload>(event, "/admin/webhooks/events", {
			status: "DEAD_LETTER",
			page: 1,
			limit: 5,
		}),
	]);

	if (checkoutPluginsResult.status === "fulfilled") {
		checkoutPlugins = checkoutPluginsResult.value;
	} else {
		console.error(checkoutPluginsResult.reason);
		errorMessages.push("Unable to load checkout providers.");
	}

	if (providerCredentialsResult.status === "fulfilled") {
		providerCredentials = providerCredentialsResult.value.data;
	} else {
		console.error(providerCredentialsResult.reason);
		errorMessages.push("Unable to load provider credentials.");
	}

	if (providerOverviewResult.status === "fulfilled") {
		providerOverview = providerOverviewResult.value;
	} else {
		console.error(providerOverviewResult.reason);
		errorMessages.push("Unable to load provider operations overview.");
	}

	if (reconciliationRunsResult.status === "fulfilled") {
		reconciliationRuns = reconciliationRunsResult.value;
	} else {
		console.error(reconciliationRunsResult.reason);
		errorMessages.push("Unable to load reconciliation runs.");
	}

	if (rejectedWebhookEventsResult.status === "fulfilled") {
		rejectedWebhookEvents = rejectedWebhookEventsResult.value;
	} else {
		console.error(rejectedWebhookEventsResult.reason);
		errorMessages.push("Unable to load rejected webhook events.");
	}

	if (deadLetterWebhookEventsResult.status === "fulfilled") {
		deadLetterWebhookEvents = deadLetterWebhookEventsResult.value;
	} else {
		console.error(deadLetterWebhookEventsResult.reason);
		errorMessages.push("Unable to load dead-letter webhook events.");
	}

	return {
		checkoutPlugins,
		providerCredentials,
		providerOverview,
		reconciliationRuns,
		rejectedWebhookEvents,
		deadLetterWebhookEvents,
		errorMessages,
	};
};
