import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub } from "$lib/storybook/api";
import {
	makeCheckoutCatalog,
	makeProviderCredential,
	makeProviderOverview,
	makeReconciliationRun,
	makeWebhookEventPage,
	makeWebhookEventRecord,
} from "$lib/storybook/factories";
import { makeAdminLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import AdminProvidersPage from "./+page.svelte";

type AdminProvidersData = ComponentProps<typeof AdminProvidersPage>["data"];

const healthyRun = makeReconciliationRun();

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
				getAdminProviderReconciliationRun: async () => healthyRun,
			}),
		}),
};
