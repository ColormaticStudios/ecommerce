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
	import Badge from "$lib/components/Badge.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import Table from "$lib/admin/table/Table.svelte";
	import TableHead from "$lib/admin/table/TableHead.svelte";
	import TableBody from "$lib/admin/table/TableBody.svelte";
	import TableRow from "$lib/admin/table/TableRow.svelte";
	import TableCell from "$lib/admin/table/TableCell.svelte";
	let { data }: { data: PageData } = $props();
	const api = getContext<API>("api");
	const notices = createAdminNotices();
	let freshness = $state<PageData["freshness"]>(null);
	let busy = $state(false);
	let operations = $state<PageData["operations"]>(null);
	let incidents = $state<PageData["incidents"]>(null);
	let activeIncidents = $state<PageData["activeIncidents"]>([]);
	let incidentPage = $state(1);
	let incidentLimit = $state(20);
	let incidentStatus = $state<"" | "open" | "resolved">("");
	let incidentLoading = $state(false);
	let incidentRevision = 0;
	const timestamp = (value: string | null) =>
		value ? new Date(value).toLocaleString() : "Still open";
	const duration = (seconds: number) =>
		seconds < 60
			? `${seconds}s`
			: seconds < 3600
				? `${Math.floor(seconds / 60)}m ${seconds % 60}s`
				: seconds < 86400
					? `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
					: `${Math.floor(seconds / 86400)}d ${Math.floor((seconds % 86400) / 3600)}h`;
	const reason = (value: string) =>
		value === "missing_baseline" ? "Missing rebuild baseline" : "Indexing lag";
	async function loadIncidents(page = incidentPage) {
		const revision = ++incidentRevision;
		incidentLoading = true;
		try {
			const [history, active] = await Promise.all([
				api.adminSearchIncidents({
					page,
					limit: incidentLimit,
					...(incidentStatus ? { status: incidentStatus } : {}),
				}),
				api.adminSearchIncidents({ page: 1, limit: 100, status: "open" }),
			]);
			if (revision !== incidentRevision) return;
			incidents = history;
			activeIncidents = active.data;
			incidentPage = history.pagination.page;
		} catch {
			if (revision === incidentRevision)
				notices.setError("Unable to load search incident history.");
		} finally {
			if (revision === incidentRevision) incidentLoading = false;
		}
	}
	$effect(() => {
		freshness = data.freshness;
		operations = data.operations;
		incidents = data.incidents;
		activeIncidents = data.activeIncidents;
		incidentPage = data.incidents?.pagination.page ?? 1;
		incidentLimit = data.incidents?.pagination.limit ?? 20;
		incidentStatus = "";
	});
	async function refresh() {
		busy = true;
		try {
			[freshness, operations] = await Promise.all([
				api.adminSearchFreshness(),
				api.adminSearchOperations(),
				loadIncidents(),
			]);
		} catch {
			notices.setError("Unable to load search index health.");
		} finally {
			busy = false;
		}
	}
	async function reindex() {
		if (
			!confirm(
				"Queue a full search index rebuild? Products remain available while the rebuild runs."
			)
		)
			return;
		busy = true;
		try {
			const job = await api.reindexAdminSearch();
			notices.setSuccess(`Rebuild queued (${job.job_id}).`);
			[freshness, operations] = await Promise.all([
				api.adminSearchFreshness(),
				api.adminSearchOperations(),
				loadIncidents(),
			]);
		} catch (error) {
			notices.setError(error instanceof Error ? error.message : "Unable to queue a rebuild.");
		} finally {
			busy = false;
		}
	}
</script>

<section class="space-y-6">
	<AdminPageHeader title="Search operations"
		>{#snippet actions()}<Button tone="admin" onclick={refresh} disabled={busy}>Refresh</Button
			>{/snippet}</AdminPageHeader
	>
	{#if data.errorMessage}<AdminEmptyState tone="error">{data.errorMessage}</AdminEmptyState>{/if}
	<AdminPanel title="index health">
		{#if freshness}
			<Badge tone={freshness.status === "healthy" ? "success" : "warning"}>{freshness.status}</Badge
			>
			<dl class="mt-4 grid gap-4 text-sm sm:grid-cols-2 lg:grid-cols-3">
				<div>
					<dt class="text-stone-500">Indexed products</dt>
					<dd class="mt-1 font-medium">{freshness.document_count}</dd>
				</div>
				<div>
					<dt class="text-stone-500">Indexing lag</dt>
					<dd class="mt-1 font-medium">{freshness.lag_seconds} seconds</dd>
				</div>
				<div>
					<dt class="text-stone-500">Pending jobs</dt>
					<dd class="mt-1 font-medium">{freshness.pending_jobs}</dd>
				</div>
				<div>
					<dt class="text-stone-500">Last indexed</dt>
					<dd class="mt-1">
						{freshness.last_indexed_at
							? new Date(freshness.last_indexed_at).toLocaleString()
							: "No baseline"}
					</dd>
				</div>
				<div>
					<dt class="text-stone-500">Last full rebuild</dt>
					<dd class="mt-1">
						{freshness.last_full_reindex_at
							? new Date(freshness.last_full_reindex_at).toLocaleString()
							: "Never"}
					</dd>
				</div>
			</dl>
		{:else}<AdminEmptyState>Index health is unavailable.</AdminEmptyState>{/if}
	</AdminPanel>
	{#if operations}<AdminPanel title="search protection"
			><dl class="grid gap-4 text-sm sm:grid-cols-2 lg:grid-cols-3">
				{#each [{ label: "Circuit state", value: operations.circuit_state }, { label: "Active searches", value: `${operations.active_searches} / ${operations.max_concurrent}` }, { label: "Search timeout", value: `${operations.search_timeout_ms} ms` }, { label: "Consecutive failures", value: operations.consecutive_failures }, { label: "Circuit failure threshold", value: operations.circuit_failure_threshold }, { label: "Circuit cooldown", value: `${operations.circuit_open_ms} ms` }, { label: "Pending rebuilds", value: `${operations.pending_reindexes} / ${operations.reindex_queue_limit}` }] as metric (metric.label)}<div
					>
						<dt class="text-stone-500">{metric.label}</dt>
						<dd class="mt-1 font-medium">{metric.value}</dd>
					</div>{/each}
			</dl>
			{#if operations.circuit_open_until}<p class="mt-4 text-sm text-stone-500">
					Circuit retry after {new Date(operations.circuit_open_until).toLocaleString()}.
				</p>{/if}</AdminPanel
		>{/if}
	<AdminPanel title="current stale episodes">
		<p class="mb-4 text-sm text-stone-500">
			Observed every minute independently of indexing workers. New installations have a five-minute
			baseline grace period. Open durations are measured when refreshed.
		</p>
		{#if incidents}
			{#each activeIncidents as incident (incident.id)}
				<div class="space-y-4 rounded-lg border border-stone-200 p-4 dark:border-stone-800">
					<div class="flex flex-wrap items-center gap-3">
						<Badge tone="warning">Open</Badge><span class="font-medium"
							>{reason(incident.reason)}</span
						><span class="text-sm text-stone-500">{duration(incident.duration_seconds)}</span>
					</div>
					<dl class="grid gap-4 text-sm sm:grid-cols-2 lg:grid-cols-3">
						{#each [{ label: "Started", value: timestamp(incident.opened_at) }, { label: "Detected", value: timestamp(incident.detected_at) }, { label: "Last observed", value: timestamp(incident.last_observed_at) }, { label: "Maximum lag", value: `${incident.max_lag_seconds} seconds` }, { label: "Maximum pending jobs", value: incident.max_pending_jobs }, { label: "Indexed products", value: incident.document_count }] as metric (metric.label)}
							<div>
								<dt class="text-stone-500">{metric.label}</dt>
								<dd class="mt-1 font-medium">{metric.value}</dd>
							</div>
						{/each}
					</dl>
				</div>
			{:else}<AdminEmptyState>No open stale index episodes.</AdminEmptyState>{/each}
		{:else}<AdminEmptyState>Current incidents are unavailable.</AdminEmptyState>{/if}
	</AdminPanel>
	<AdminPanel title="stale index history">
		<div class="mb-4 flex flex-wrap items-center justify-between gap-3">
			<p class="text-sm text-stone-500">
				Resolved episodes are retained for one year after recovery.
			</p>
			<Dropdown
				tone="admin"
				full={false}
				aria-label="Incident status"
				bind:value={incidentStatus}
				onchange={() => void loadIncidents(1)}
				disabled={incidentLoading}
			>
				<option value="">All episodes</option><option value="open">Open</option><option
					value="resolved">Resolved</option
				>
			</Dropdown>
		</div>
		{#if incidents?.data.length}
			<Table
				><TableHead
					><TableRow>
						{#each ["Status", "Reason", "Started", "Detected", "Recovered", "Duration", "Maximum lag", "Maximum pending jobs"] as label (label)}<TableCell
								header>{label}</TableCell
							>{/each}
					</TableRow></TableHead
				><TableBody>
					{#each incidents.data as incident (incident.id)}<TableRow>
							<TableCell
								><Badge tone={incident.status === "open" ? "warning" : "success"}
									>{incident.status === "open" ? "Open" : "Resolved"}</Badge
								></TableCell
							>
							<TableCell>{reason(incident.reason)}</TableCell><TableCell
								>{timestamp(incident.opened_at)}</TableCell
							><TableCell>{timestamp(incident.detected_at)}</TableCell><TableCell
								>{timestamp(incident.recovered_at)}</TableCell
							><TableCell numeric>{duration(incident.duration_seconds)}</TableCell><TableCell
								numeric>{incident.max_lag_seconds}s</TableCell
							><TableCell numeric>{incident.max_pending_jobs}</TableCell>
						</TableRow>{/each}
				</TableBody></Table
			>
		{:else if incidents}<AdminEmptyState>No recorded episodes match this filter.</AdminEmptyState>
		{:else}<AdminEmptyState>Incident history is unavailable.</AdminEmptyState>{/if}
		{#if incidents}
			<div class="mt-4 flex flex-wrap items-center justify-between gap-3 text-sm text-stone-500">
				<label class="flex items-center gap-2"
					>Per page <Dropdown
						tone="admin"
						full={false}
						bind:value={incidentLimit}
						onchange={() => void loadIncidents(1)}
						disabled={incidentLoading}
						>{#each [10, 20, 50] as limit (limit)}<option value={limit}>{limit}</option
							>{/each}</Dropdown
					></label
				>
				<span
					>Page {incidents.pagination.page} of {Math.max(1, incidents.pagination.total_pages)} ({incidents
						.pagination.total}
					{incidents.pagination.total === 1 ? "episode" : "episodes"})</span
				>
				<div class="flex gap-2">
					<Button
						tone="admin"
						size="small"
						disabled={incidentLoading || incidentPage <= 1}
						onclick={() => void loadIncidents(incidentPage - 1)}>Previous</Button
					><Button
						tone="admin"
						size="small"
						disabled={incidentLoading || incidentPage >= incidents.pagination.total_pages}
						onclick={() => void loadIncidents(incidentPage + 1)}>Next</Button
					>
				</div>
			</div>
		{/if}
	</AdminPanel>

	<AdminPanel title="rebuild index"
		><p class="mb-4 text-sm text-stone-600 dark:text-stone-300">
			Queue a rebuild from published catalog products. Requests are rejected when the rebuild queue
			is full. Index health updates after indexing succeeds.
		</p>
		<Button tone="admin" variant="primary" onclick={reindex} disabled={busy}>Queue rebuild</Button
		></AdminPanel
	>
</section>
<AdminFloatingNotices
	statusMessage={notices.message}
	statusTone={notices.tone}
	onDismissStatus={notices.clear}
/>
