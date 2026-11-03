import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { makeAdminLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import Page from "./+page.svelte";
const meta = {
	title: "Routes/Admin/Search/Operations",
	component: RouteStoryHarness,
} satisfies Meta;
export default meta;
type Story = StoryObj;
type Data = ComponentProps<typeof Page>["data"];

function data(overrides: Partial<Data> = {}): Data {
	return {
		...makeAdminLayoutData(),
		freshness: null,
		errorMessage: "",
		operations: null,
		incidents: { data: [], pagination: { page: 1, limit: 20, total: 0, total_pages: 0 } },
		activeIncidents: [],
		...overrides,
	};
}
export const Empty: Story = {
	render: () =>
		renderRouteStory({ component: Page, componentProps: { data: data({ freshness: null }) } }),
};
export const LoadFailure: Story = {
	render: () =>
		renderRouteStory({
			component: Page,
			componentProps: {
				data: data({ errorMessage: "Unable to load search settings.", incidents: null }),
			},
		}),
};
const freshness = {
	status: "healthy" as const,
	lag_seconds: 12,
	pending_jobs: 2,
	document_count: 10000,
	last_indexed_at: "2026-10-07T12:00:00Z",
	last_full_reindex_at: "2026-10-07T11:00:00Z",
};
const operations = {
	max_concurrent: 50,
	search_timeout_ms: 5000,
	circuit_failure_threshold: 5,
	circuit_open_ms: 30000,
	reindex_queue_limit: 1,
	active_searches: 3,
	circuit_state: "closed" as const,
	consecutive_failures: 0,
	circuit_open_until: null,
	pending_reindexes: 0,
};
export const Healthy: Story = {
	render: () =>
		renderRouteStory({
			component: Page,
			componentProps: { data: data({ freshness, operations }) },
		}),
};
export const CircuitOpen: Story = {
	render: () =>
		renderRouteStory({
			component: Page,
			componentProps: {
				data: data({
					freshness: { ...freshness, status: "degraded", lag_seconds: 120 },
					operations: {
						...operations,
						circuit_state: "open",
						consecutive_failures: 5,
						circuit_open_until: "2026-10-07T12:00:30Z",
					},
				}),
			},
		}),
};

const openIncident = {
	id: 1,
	index_name: "products",
	reason: "index_lag" as const,
	status: "open" as const,
	opened_at: "2026-10-07T11:30:00Z",
	detected_at: "2026-10-07T11:30:30Z",
	last_observed_at: "2026-10-07T12:00:00Z",
	recovered_at: null,
	duration_seconds: 1800,
	max_lag_seconds: 2100,
	max_pending_jobs: 250,
	document_count: 10000,
};
export const OpenStaleEpisode: Story = {
	render: () =>
		renderRouteStory({
			component: Page,
			componentProps: {
				data: data({
					freshness: { ...freshness, status: "stale", lag_seconds: 2100, pending_jobs: 250 },
					operations,
					activeIncidents: [openIncident],
					incidents: {
						data: [openIncident],
						pagination: { page: 1, limit: 20, total: 1, total_pages: 1 },
					},
				}),
			},
		}),
};
export const ResolvedHistory: Story = {
	render: () =>
		renderRouteStory({
			component: Page,
			componentProps: {
				data: data({
					freshness,
					operations,
					incidents: {
						data: [{ ...openIncident, status: "resolved", recovered_at: "2026-10-07T12:00:00Z" }],
						pagination: { page: 1, limit: 20, total: 21, total_pages: 2 },
					},
				}),
			},
		}),
};
export const MissingBaselineEpisode: Story = {
	render: () =>
		renderRouteStory({
			component: Page,
			componentProps: {
				data: data({
					freshness: {
						...freshness,
						status: "stale",
						document_count: 0,
						last_full_reindex_at: null,
					},
					operations,
					activeIncidents: [
						{
							...openIncident,
							reason: "missing_baseline",
							duration_seconds: 172800,
							document_count: 0,
							max_lag_seconds: 0,
							max_pending_jobs: 0,
						},
					],
				}),
			},
		}),
};
