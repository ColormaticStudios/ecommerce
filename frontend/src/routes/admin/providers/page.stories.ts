import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub } from "$lib/storybook/api";
import {
	makeCheckoutCatalog,
	makeProviderCredential,
	makeProviderOperation,
	makeProviderOverview,
	makeProviderReconciliationCase,
	makeReconciliationRun,
	makeWebhookEventPage,
	makeWebhookEventRecord,
} from "$lib/storybook/factories";
import { makeAdminLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import AdminProvidersPage from "./+page.svelte";

type AdminProvidersData = ComponentProps<typeof AdminProvidersPage>["data"];

const healthyRun = makeReconciliationRun();
const healthyOperation = makeProviderOperation();
const unknownCase = makeProviderReconciliationCase();

const meta = {
	title: "Routes/Admin/Providers",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function createData(overrides: Partial<AdminProvidersData> = {}): AdminProvidersData {
	return {
		...makeAdminLayoutData(),
		checkoutPlugins: makeCheckoutCatalog(),
		providerCredentials: [makeProviderCredential()],
		providerOverview: makeProviderOverview(),
		providerOperations: {
			data: [healthyOperation],
			pagination: { page: 1, limit: 10, total: 1, total_pages: 1 },
		},
		reconciliationCases: {
			data: [],
			pagination: { page: 1, limit: 10, total: 0, total_pages: 0 },
		},
		reconciliationRuns: {
			data: [healthyRun],
			pagination: { page: 1, limit: 10, total: 1, total_pages: 1 },
		},
		rejectedWebhookEvents: makeWebhookEventPage(),
		deadLetterWebhookEvents: makeWebhookEventPage(),
		errorMessages: [],
		...overrides,
	};
}

export const Healthy: Story = {
	render: () =>
		renderRouteStory({
			component: AdminProvidersPage,
			componentProps: { data: createData() },
			api: createApiStub({
				getAdminProviderOperation: async () => healthyOperation,
				getAdminProviderReconciliationRun: async () => healthyRun,
			}),
		}),
};

export const Degraded: Story = {
	render: () =>
		renderRouteStory({
			component: AdminProvidersPage,
			componentProps: {
				data: createData({
					providerOverview: makeProviderOverview({
						runtime_environment: "production",
						webhook_events: {
							pending_count: 3,
							processed_count: 12,
							dead_letter_count: 1,
							rejected_count: 2,
						},
					}),
					reconciliationRuns: {
						data: [
							makeReconciliationRun({
								id: 2,
								status: "FAILED",
								environment: "production",
								drift_count: 2,
								error_count: 1,
								drifts: [
									{
										id: 1,
										entity_type: "payment",
										entity_id: 501,
										provider_reference: "pay_123",
										severity: "ERROR",
										field_name: "amount",
										expected_value: "151.32",
										actual_value: "149.32",
										message: "Authorized amount differs from order total.",
									},
								],
							}),
						],
						pagination: { page: 1, limit: 10, total: 1, total_pages: 1 },
					},
					rejectedWebhookEvents: makeWebhookEventPage({
						data: [
							makeWebhookEventRecord({
								id: 21,
								status: "REJECTED",
								last_error: "Signature mismatch",
							}),
						],
						pagination: { page: 1, limit: 5, total: 1, total_pages: 1 },
					}),
					deadLetterWebhookEvents: makeWebhookEventPage({
						data: [
							makeWebhookEventRecord({
								id: 22,
								status: "DEAD_LETTER",
								last_error: "Retries exhausted",
							}),
						],
						pagination: { page: 1, limit: 5, total: 1, total_pages: 1 },
					}),
					errorMessages: ["Unable to load provider operations overview."],
				}),
			},
			api: createApiStub({
				getAdminProviderOperation: async () => healthyOperation,
				getAdminProviderReconciliationRun: async () =>
					makeReconciliationRun({
						id: 2,
						status: "FAILED",
						environment: "production",
						drift_count: 2,
						error_count: 1,
						drifts: [
							{
								id: 1,
								entity_type: "payment",
								entity_id: 501,
								provider_reference: "pay_123",
								severity: "ERROR",
								field_name: "amount",
								expected_value: "151.32",
								actual_value: "149.32",
								message: "Authorized amount differs from order total.",
							},
						],
					}),
			}),
		}),
};

export const Unknown: Story = {
	render: () => {
		const operation = makeProviderOperation({
			status: "RECONCILIATION_REQUIRED",
			provider_outcome: "UNKNOWN",
			provider_reference: undefined,
			retryable: true,
			available_actions: ["query_outcome"],
			completed_at: null,
			problem: {
				type: "https://ecommerce.local/problems/provider-operation",
				title: "Provider operation requires attention",
				status: 500,
				detail: "Provider outcome lookup remained ambiguous.",
				code: "provider_operation_error",
				correlation_id: "corr-story-41",
				instance: "urn:provider-operation:op_story_capture_41",
			},
			attempts: [
				{
					id: 81,
					attempt_number: 1,
					phase: "provider",
					outcome: "UNKNOWN",
					provider_outcome: "UNKNOWN",
					operation_key: "op_story_capture_41",
					retryable: false,
					problem: {
						type: "https://ecommerce.local/problems/provider-operation",
						title: "Provider operation requires attention",
						status: 500,
						detail: "Provider timed out before acknowledging the request.",
						code: "provider_operation_error",
						correlation_id: "corr-story-41",
					},
					started_at: "2026-04-07T09:30:00.000Z",
					finished_at: "2026-04-07T09:30:30.000Z",
				},
			],
			reconciliation_cases: [unknownCase],
		});
		return renderRouteStory({
			component: AdminProvidersPage,
			componentProps: {
				data: createData({
					providerOverview: makeProviderOverview({
						operations: {
							total_count: 1,
							active_count: 0,
							unknown_count: 1,
							finalize_retry_count: 0,
							compensation_retry_count: 0,
							failed_count: 0,
							completed_count: 0,
						},
						reconciliation_cases: { open_count: 1, unassigned_count: 1 },
					}),
					providerOperations: {
						data: [operation],
						pagination: { page: 1, limit: 10, total: 1, total_pages: 1 },
					},
					reconciliationCases: {
						data: [unknownCase],
						pagination: { page: 1, limit: 10, total: 1, total_pages: 1 },
					},
				}),
			},
			api: createApiStub({
				getAdminProviderOperation: async () => operation,
				getAdminProviderReconciliationCase: async () => unknownCase,
				getAdminProviderReconciliationRun: async () => healthyRun,
				queryAdminProviderOperationOutcome: async () => operation,
			}),
		});
	},
};

export const CompensationFailed: Story = {
	render: () => {
		const operation = makeProviderOperation({
			id: 42,
			operation_key: "op_story_capture_41:compensate:refund",
			operation: "refund",
			status: "COMPENSATION_RETRY",
			provider_outcome: "FAILED",
			parent_operation_id: 41,
			retryable: true,
			available_actions: ["retry_compensation"],
			completed_at: null,
			next_attempt_at: "2026-04-07T10:00:00.000Z",
			problem: {
				type: "https://ecommerce.local/problems/provider-operation",
				title: "Provider operation requires attention",
				status: 500,
				detail: "Compensation attempts are exhausted and require operator review.",
				code: "provider_operation_error",
				correlation_id: "corr-story-41",
			},
		});
		return renderRouteStory({
			component: AdminProvidersPage,
			componentProps: {
				data: createData({
					providerOverview: makeProviderOverview({
						operations: {
							total_count: 1,
							active_count: 0,
							unknown_count: 0,
							finalize_retry_count: 0,
							compensation_retry_count: 1,
							failed_count: 0,
							completed_count: 0,
						},
					}),
					providerOperations: {
						data: [operation],
						pagination: { page: 1, limit: 10, total: 1, total_pages: 1 },
					},
				}),
			},
			api: createApiStub({
				getAdminProviderOperation: async () => operation,
				getAdminProviderReconciliationRun: async () => healthyRun,
				retryCompensationAdminProviderOperation: async () => operation,
			}),
		});
	},
};

export const CapabilityBlocker: Story = {
	render: () => {
		const operation = makeProviderOperation({
			provider_type: "shipping",
			provider_id: "legacy-carrier",
			operation: "buy_label",
			status: "RECONCILIATION_REQUIRED",
			provider_outcome: "UNKNOWN",
			retryable: false,
			available_actions: [],
			completed_at: null,
			problem: {
				type: "https://ecommerce.local/problems/provider-capability",
				title: "Outcome lookup unavailable",
				status: 412,
				detail: "This provider does not expose operation-key outcome lookup.",
				code: "provider_capability_unavailable",
				correlation_id: "corr-story-41",
			},
		});
		return renderRouteStory({
			component: AdminProvidersPage,
			componentProps: {
				data: createData({
					providerOperations: {
						data: [operation],
						pagination: { page: 1, limit: 10, total: 1, total_pages: 1 },
					},
				}),
			},
			api: createApiStub({
				getAdminProviderOperation: async () => operation,
				getAdminProviderReconciliationRun: async () => healthyRun,
			}),
		});
	},
};

export const Empty: Story = {
	render: () =>
		renderRouteStory({
			component: AdminProvidersPage,
			componentProps: {
				data: createData({
					checkoutPlugins: { payment: [], shipping: [], tax: [] },
					providerCredentials: [],
					providerOverview: makeProviderOverview({
						operations: {
							total_count: 0,
							active_count: 0,
							unknown_count: 0,
							finalize_retry_count: 0,
							compensation_retry_count: 0,
							failed_count: 0,
							completed_count: 0,
						},
						reconciliation_cases: { open_count: 0, unassigned_count: 0 },
					}),
					providerOperations: {
						data: [],
						pagination: { page: 1, limit: 10, total: 0, total_pages: 0 },
					},
					reconciliationCases: {
						data: [],
						pagination: { page: 1, limit: 10, total: 0, total_pages: 0 },
					},
					reconciliationRuns: {
						data: [],
						pagination: { page: 1, limit: 10, total: 0, total_pages: 0 },
					},
				}),
			},
			api: createApiStub(),
		}),
};

export const NoRuns: Story = {
	render: () =>
		renderRouteStory({
			component: AdminProvidersPage,
			componentProps: {
				data: createData({
					reconciliationRuns: {
						data: [],
						pagination: { page: 1, limit: 10, total: 0, total_pages: 1 },
					},
				}),
			},
			api: createApiStub({
				getAdminProviderOperation: async () => healthyOperation,
			}),
		}),
};
