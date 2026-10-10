<script lang="ts">
	import { getContext } from "svelte";
	import type { API } from "$lib/api";
	import type { PageData } from "./$types";
	import AdminPageHeader from "$lib/admin/AdminPageHeader.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminFloatingNotices from "$lib/admin/AdminFloatingNotices.svelte";
	import { createAdminNotices } from "$lib/admin/state.svelte";
	import Button from "$lib/components/Button.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import Table from "$lib/admin/table/Table.svelte";
	import TableCell from "$lib/admin/table/TableCell.svelte";
	import TableHead from "$lib/admin/table/TableHead.svelte";
	import TableBody from "$lib/admin/table/TableBody.svelte";
	import TableRow from "$lib/admin/table/TableRow.svelte";
	let { data }: { data: PageData } = $props();
	const api = getContext<API>("api");
	const notices = createAdminNotices();
	let analytics = $state<PageData["analytics"]>(null);
	let days = $state(7);
	let loading = $state(false);
	let revision = 0;
	$effect(() => {
		analytics = data.analytics;
		days = data.analytics?.days ?? 7;
	});
	async function refresh() {
		const requestRevision = ++revision;
		loading = true;
		try {
			const response = await api.adminSearchAnalytics(days);
			if (requestRevision === revision) analytics = response;
		} catch {
			if (requestRevision === revision) notices.setError("Unable to load search analytics.");
		} finally {
			if (requestRevision === revision) loading = false;
		}
	}
	const percentage = (value: number) => `${(value * 100).toFixed(1)}%`;
</script>

<section class="space-y-6">
	<AdminPageHeader title="Search analytics"
		>{#snippet actions()}<Dropdown tone="admin" full={false} bind:value={days} onchange={refresh}
				><option value={7}>Last 7 days</option><option value={30}>Last 30 days</option><option
					value={90}>Last 90 days</option
				></Dropdown
			><Button tone="admin" disabled={loading} onclick={refresh}>Refresh</Button
			>{/snippet}</AdminPageHeader
	>
	{#if data.errorMessage}<AdminEmptyState tone="error">{data.errorMessage}</AdminEmptyState>{/if}
	<p class="text-sm text-stone-600 dark:text-stone-300">
		Only opted-in anonymous storefront activity is included. Raw events expire after 90 days. Cart
		and paid-order attribution follows the last clicked result for that product within seven days.
	</p>
	{#if analytics}
		<AdminPanel title="search activity"
			><dl class="grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
				{#each [{ label: "Searches", value: analytics.searches }, { label: "Distinct sessions", value: analytics.unique_sessions }, { label: "Zero results", value: analytics.zero_results }, { label: "Zero-result rate", value: percentage(analytics.zero_result_rate) }, { label: "Clicks", value: analytics.clicks }, { label: "Cart additions", value: analytics.add_to_carts }, { label: "Paid orders", value: analytics.paid_orders }, { label: "Click-through rate", value: percentage(analytics.ctr) }, { label: "Conversion rate", value: percentage(analytics.conversion_rate) }, { label: "CTR at 5", value: percentage(analytics.ctr_at_5) }, { label: "CTR at 10", value: percentage(analytics.ctr_at_10) }, { label: "Conversion at 5", value: percentage(analytics.conversion_at_5) }, { label: "Conversion at 10", value: percentage(analytics.conversion_at_10) }, { label: "95th percentile latency", value: `${analytics.p95_latency_ms.toFixed(1)} ms` }, { label: "99th percentile latency", value: `${analytics.p99_latency_ms.toFixed(1)} ms` }, { label: "Average search latency", value: `${analytics.average_latency_ms.toFixed(1)} ms` }] as metric (metric.label)}<div
					>
						<dt class="text-sm text-stone-500">{metric.label}</dt>
						<dd class="mt-1 text-2xl font-semibold tabular-nums">{metric.value}</dd>
					</div>{/each}
			</dl></AdminPanel
		>
		<AdminPanel title="query performance">
			{#if analytics.queries.length}<Table
					><TableHead
						><TableRow
							>{#each ["Query", "Searches", "Zero results", "Clicks", "Cart additions", "Paid orders", "CTR", "Conversion", "Latency"] as label (label)}<TableCell
									header>{label}</TableCell
								>{/each}</TableRow
						></TableHead
					><TableBody
						>{#each analytics.queries as query (query.query)}<TableRow
								><TableCell strong>{query.query || "Browse"}</TableCell><TableCell numeric
									>{query.searches}</TableCell
								><TableCell numeric>{query.zero_results}</TableCell><TableCell numeric
									>{query.clicks}</TableCell
								><TableCell numeric>{query.add_to_carts}</TableCell><TableCell numeric
									>{query.paid_orders}</TableCell
								><TableCell numeric>{percentage(query.ctr)}</TableCell><TableCell numeric
									>{percentage(query.conversion_rate)}</TableCell
								><TableCell numeric>{query.average_latency_ms.toFixed(1)} ms</TableCell></TableRow
							>{/each}</TableBody
					></Table
				>
			{:else}<AdminEmptyState>No opted-in search activity in this period.</AdminEmptyState>{/if}
		</AdminPanel>
		<AdminPanel title="filter usage"
			><dl class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
				{#each Object.entries(analytics.facet_usage) as [name, count] (name)}<div>
						<dt class="text-sm text-stone-500">{name}</dt>
						<dd class="mt-1 font-medium">{count}</dd>
					</div>{:else}<AdminEmptyState>No filter activity in this period.</AdminEmptyState>{/each}
			</dl></AdminPanel
		>
		<AdminPanel title="public discovery"
			><p class="mb-4 text-sm text-stone-500">
				Popular and trending queries use seven-day activity and need at least five distinct opted-in
				sessions.
			</p>
			<dl class="grid gap-4 sm:grid-cols-2">
				<div>
					<dt class="font-medium">Popular searches</dt>
					<dd class="mt-2 text-sm">{analytics.popular.join(", ") || "No qualifying queries"}</dd>
				</div>
				<div>
					<dt class="font-medium">Trending searches</dt>
					<dd class="mt-2 text-sm">{analytics.trending.join(", ") || "No qualifying queries"}</dd>
				</div>
			</dl></AdminPanel
		>
	{:else}<AdminEmptyState>Search analytics are unavailable.</AdminEmptyState>{/if}
</section>
<AdminFloatingNotices
	statusMessage={notices.message}
	statusTone={notices.tone}
	onDismissStatus={notices.clear}
/>
