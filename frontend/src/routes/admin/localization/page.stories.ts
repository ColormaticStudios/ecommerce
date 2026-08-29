import type { Meta, StoryObj } from "@storybook/sveltekit";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub } from "$lib/storybook/api";
import { renderRouteStory } from "$lib/storybook/render";
import type { components } from "$lib/api/generated/openapi";
import AdminLocalizationPage from "./+page.svelte";
import LocalizationAdminWorkspace from "$lib/admin/localization/LocalizationAdminWorkspace.svelte";

const now = "2026-08-12T20:00:00.000Z";
const sourceKey: components["schemas"]["TranslationKey"] = {
	id: 41,
	namespace: "storefront",
	key: "cart.checkout",
	source_text: "Checkout",
	description: "Primary action in the cart summary.",
	owner_domain: "checkout",
	is_deprecated: false,
	created_at: now,
	updated_at: now,
};

const publishedValue: components["schemas"]["TranslationValue"] = {
	id: 71,
	translation_key_id: 41,
	locale: "fr-FR",
	value: "Passer la commande",
	state: "published",
	version: 1,
	created_at: now,
	updated_at: now,
	validation_issues: [],
};

const rolloutDomains: components["schemas"]["LocalizationRolloutDomain"][] = [
	"account",
	"admin",
	"checkout",
	"communications",
	"errors",
	"storefront",
];

const rolloutRows: components["schemas"]["LocalizationRollout"][] = ["en-US", "fr-FR"].flatMap(
	(locale) =>
		rolloutDomains.map((domain) => ({
			locale,
			domain,
			is_enabled: true,
			percentage: locale === "fr-FR" ? 100 : domain === "communications" ? 25 : 100,
			updated_at: now,
		}))
);

const healthMetrics: components["schemas"]["LocalizationMetricsResponse"] = {
	lookup_count: 1_000,
	missing_key_count: 12,
	missing_key_rate: 0.012,
	fallback_hit_count: 37,
	fallback_hit_rate: 0.037,
	rollout_fallback_count: 8,
	publish_count: 14,
	average_publish_latency_ms: 86_400_000,
	maximum_publish_latency_ms: 172_800_000,
	rollback_count: 0,
	locale_rates: [
		{
			locale: "fr-FR",
			lookup_count: 600,
			missing_key_count: 12,
			fallback_hit_count: 37,
			missing_key_rate: 0.02,
			fallback_hit_rate: 0.0617,
		},
	],
	generated_at: now,
	hotspots: [
		{
			metric_type: "missing_key",
			locale: "fr-FR",
			domain: "checkout",
			key: "checkout.payment_declined",
			count: 9,
			average_value: 0,
			maximum_value: 0,
			last_seen_at: now,
		},
		{
			metric_type: "fallback_hit",
			locale: "fr-FR",
			domain: "communications",
			key: "communications.shipment_updated.email.body",
			count: 7,
			average_value: 0,
			maximum_value: 0,
			last_seen_at: now,
		},
	],
};

const meta = {
	title: "Routes/Admin/Localization",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function apiForQueue(
	items: components["schemas"]["TranslationQueueItem"][],
	pagination: components["schemas"]["Pagination"] = {
		page: 1,
		limit: 25,
		total: items.length,
		total_pages: 1,
	}
) {
	return createApiStub({
		listAdminLocalizationLocales: async () => ({
			default_locale: "fr-FR",
			locales: [
				{
					code: "en-US",
					name: "English (United States)",
					is_enabled: true,
					is_default: false,
					fallback_locale: null,
					default_for_markets: [],
				},
				{
					code: "fr-FR",
					name: "French (France)",
					is_enabled: true,
					is_default: true,
					fallback_locale: "en-US",
					default_for_markets: ["FRA"],
				},
			],
		}),
		listAdminLocalizationReleases: async () => ({
			releases: [
				{
					id: 8,
					name: "Summer storefront",
					status: "active",
					notes: "Approved storefront copy.",
					snapshot_hash: "b7c9a2f1543d",
					published_at: now,
					published_by: 1,
					created_at: now,
					updated_at: now,
				},
				{
					id: 7,
					name: "Spring storefront",
					status: "superseded",
					notes: "Known-good release available for recovery.",
					snapshot_hash: "4a13e5cc9956",
					published_at: now,
					published_by: 1,
					created_at: now,
					updated_at: now,
				},
			],
		}),
		getAdminLocalizationReleaseQuality: async (id) => ({
			release_id: id,
			ready: true,
			required_locales: ["en-US", "fr-FR"],
			critical_namespaces: ["checkout", "errors", "communications"],
			missing_count: 0,
			missing: [],
		}),
		listAdminLocalizationKeys: async () => ({
			items,
			pagination,
		}),
		listAdminLocalizationAssignees: async () => ({
			assignees: [
				{
					id: 3,
					name: "Morgan Editor",
					email: "morgan@example.com",
					localization_role: "editor",
				},
				{
					id: 4,
					name: "Priya Publisher",
					email: "priya@example.com",
					localization_role: "publisher",
				},
			],
		}),
		listAdminLocalizationRollouts: async () => ({ rollouts: rolloutRows }),
		replaceAdminLocalizationRollouts: async (input) => ({
			rollouts: input.rollouts.map((rollout) => ({ ...rollout, updated_at: now })),
		}),
		getAdminLocalizationMetrics: async () => healthMetrics,
		listAdminLocalizationGlossary: async () => ({
			terms: [
				{
					id: 4,
					locale: "fr-FR",
					source_term: "Checkout",
					translated_term: "Passer la commande",
					description: "Approved commerce terminology.",
					is_locked: true,
					created_at: now,
					updated_at: now,
				},
			],
		}),
		listAdminLocalizationComments: async () => ({
			comments: [
				{
					id: 12,
					translation_value_id: 72,
					author_name: "Morgan Editor",
					comment: "Use the approved glossary wording before publishing.",
					created_at: now,
				},
			],
		}),
		listAdminLocalizationKeyUsages: async (keyId) => ({
			usages: [
				{
					id: 31,
					translation_key_id: keyId,
					route: "/checkout",
					component: "CheckoutSignInRequiredBanner.svelte",
					description: "Shown above checkout when authentication is required.",
					position: 0,
					created_at: now,
					updated_at: now,
				},
			],
		}),
		replaceAdminLocalizationKeyUsages: async (_keyId, input) => ({
			usages: input.usages.map((usage, index) => ({
				id: index + 31,
				translation_key_id: _keyId,
				route: usage.route,
				component: usage.component,
				description: usage.description,
				position: usage.position,
				created_at: now,
				updated_at: now,
			})),
		}),
	});
}

export const ReviewQueue: Story = {
	render: () =>
		renderRouteStory({
			component: AdminLocalizationPage,
			api: apiForQueue([
				{
					key: sourceKey,
					locale: "fr-FR",
					latest_value: {
						...publishedValue,
						id: 72,
						value: "Finaliser",
						state: "review",
						version: 2,
						assignee_id: 3,
					},
					published_value: publishedValue,
					missing: false,
					stale: false,
					validation_issues: [],
					preview_url: "/cart?locale=fr-FR",
				},
				{
					key: { ...sourceKey, id: 42, key: "cart.empty", source_text: "Your cart is empty" },
					locale: "fr-FR",
					missing: true,
					stale: false,
					validation_issues: [],
					preview_url: "/cart?locale=fr-FR",
				},
			]),
		}),
};

export const ValidationBlocked: Story = {
	render: () =>
		renderRouteStory({
			component: AdminLocalizationPage,
			api: apiForQueue([
				{
					key: { ...sourceKey, source_text: "Checkout as {customer}" },
					locale: "fr-FR",
					latest_value: {
						...publishedValue,
						id: 73,
						value: "Passer la commande",
						state: "draft",
						version: 2,
						validation_issues: [
							{
								path: "/value",
								code: "placeholder_mismatch",
								detail: "Translated placeholders must exactly match the source placeholders.",
							},
						],
					},
					published_value: publishedValue,
					missing: false,
					stale: true,
					validation_issues: [
						{
							path: "/value",
							code: "placeholder_mismatch",
							detail: "Translated placeholders must exactly match the source placeholders.",
						},
					],
					preview_url: "/cart?locale=fr-FR",
				},
			]),
		}),
};

export const PaginatedQueue: Story = {
	render: () =>
		renderRouteStory({
			component: AdminLocalizationPage,
			api: apiForQueue(
				[
					{
						key: sourceKey,
						locale: "fr-FR",
						latest_value: publishedValue,
						published_value: publishedValue,
						missing: false,
						stale: false,
						validation_issues: [],
						preview_url: "/cart?locale=fr-FR",
					},
				],
				{ page: 2, limit: 25, total: 64, total_pages: 3 }
			),
		}),
};

export const RolloutAndHealth: Story = {
	render: () =>
		renderRouteStory({
			component: LocalizationAdminWorkspace,
			componentProps: { initialTab: "operations" },
			api: apiForQueue([]),
		}),
};

export const ReleaseRecovery: Story = {
	render: () =>
		renderRouteStory({
			component: LocalizationAdminWorkspace,
			componentProps: { initialTab: "delivery" },
			api: apiForQueue([]),
		}),
};
