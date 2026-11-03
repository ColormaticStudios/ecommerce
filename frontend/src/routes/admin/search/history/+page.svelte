<script lang="ts">
	import { getContext } from "svelte";
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import { SvelteURLSearchParams } from "svelte/reactivity";
	import { LOCALIZATION_CONTEXT, type LocalizationRuntime } from "$lib/localization/runtime";
	import type { PageData } from "./$types";
	import AdminPageHeader from "$lib/admin/AdminPageHeader.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import Button from "$lib/components/Button.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import NumberInput from "$lib/components/NumberInput.svelte";
	import Table from "$lib/admin/table/Table.svelte";
	import TableHead from "$lib/admin/table/TableHead.svelte";
	import TableBody from "$lib/admin/table/TableBody.svelte";
	import TableRow from "$lib/admin/table/TableRow.svelte";
	import TableCell from "$lib/admin/table/TableCell.svelte";
	let { data }: { data: PageData } = $props();
	const localization = getContext<LocalizationRuntime>(LOCALIZATION_CONTEXT);
	function pageHref(page: number) {
		const params = new SvelteURLSearchParams({ page: String(page) });
		if (data.query.kind) params.set("kind", data.query.kind);
		if (data.query.entity_id) params.set("entity_id", String(data.query.entity_id));
		return `/admin/search/history?${params}` as const;
	}
</script>

<section class="space-y-6">
	<AdminPageHeader
		title={$localization.translate("admin.search.history.title", "search configuration history")}
	/>
	<AdminPanel title={$localization.translate("admin.search.history.filters", "filter history")}>
		<form method="GET" class="flex flex-wrap items-end gap-4">
			<label class="grid gap-2"
				><span>{$localization.translate("admin.search.history.kind", "Configuration type")}</span
				><Dropdown tone="admin" name="kind" value={data.query.kind ?? ""}>
					<option value=""
						>{$localization.translate("admin.search.history.all", "All types")}</option
					>
					<option value="synonym"
						>{$localization.translate("admin.search.tabs.synonyms", "Synonyms")}</option
					>
					<option value="typo_profile"
						>{$localization.translate("admin.search.tabs.typo_profiles", "Typo profiles")}</option
					>
					<option value="ranking_profile"
						>{$localization.translate(
							"admin.search.tabs.ranking_profiles",
							"Ranking profiles"
						)}</option
					>
				</Dropdown></label
			>
			<label class="grid gap-2"
				><span
					>{$localization.translate(
						"admin.search.history.entity_id",
						"Configuration ID (optional)"
					)}</span
				><NumberInput
					tone="admin"
					name="entity_id"
					min={1}
					value={data.query.entity_id ?? ""}
				/></label
			>
			<Button tone="admin" type="submit"
				>{$localization.translate("admin.search.history.apply", "Apply filters")}</Button
			>
		</form>
	</AdminPanel>
	{#if data.errorMessage}<AdminEmptyState tone="error"
			>{$localization.translate(
				"admin.search.history.load_error",
				"Unable to load search configuration history."
			)}</AdminEmptyState
		>{/if}
	<AdminPanel
		title={$localization.translate("admin.search.history.changes", "configuration changes")}
	>
		{#if data.history?.items.length}
			<Table
				><TableHead
					><TableRow>
						<TableCell header
							>{$localization.translate(
								"admin.search.history.configuration",
								"Configuration"
							)}</TableCell
						>
						<TableCell header
							>{$localization.translate("admin.search.history.operation", "Operation")}</TableCell
						>
						<TableCell header
							>{$localization.translate("admin.search.history.actor", "Actor")}</TableCell
						>
						<TableCell header
							>{$localization.translate("admin.search.history.time", "Changed at")}</TableCell
						>
						<TableCell header
							>{$localization.translate(
								"admin.search.history.snapshots",
								"Before and after"
							)}</TableCell
						>
					</TableRow></TableHead
				><TableBody>
					{#each data.history.items as change (change.id)}
						{@const name = change.after?.name ?? change.before?.name}
						<TableRow
							><TableCell strong
								>{typeof name === "string" && name
									? name
									: $localization.translate(
											"admin.search.history.unnamed",
											"Unnamed configuration"
										)}
								<div class="text-xs font-normal text-stone-500">
									{change.kind} #{change.entity_id}
								</div></TableCell
							>
							<TableCell>{change.operation}</TableCell><TableCell
								>{change.actor_id ?? "—"}</TableCell
							><TableCell><time datetime={change.created_at}>{change.created_at}</time></TableCell>
							<TableCell
								><details>
									<summary class="cursor-pointer"
										>{$localization.translate(
											"admin.search.history.inspect",
											"Inspect changes"
										)}</summary
									>
									<div class="mt-3 grid gap-3">
										{#each [[$localization.translate("admin.search.history.before", "Before"), change.before], [$localization.translate("admin.search.history.after", "After"), change.after]] as [label, snapshot] (label)}
											<div>
												<p class="font-medium">{label}</p>
												<pre
													class="mt-1 max-h-72 max-w-lg overflow-auto rounded bg-stone-100 p-3 text-xs dark:bg-stone-900">{JSON.stringify(
														snapshot,
														null,
														2
													)}</pre>
											</div>
										{/each}
									</div>
								</details></TableCell
							></TableRow
						>
					{/each}
				</TableBody></Table
			>
		{:else}<AdminEmptyState
				>{$localization.translate(
					"admin.search.history.empty",
					"No configuration changes match these filters."
				)}</AdminEmptyState
			>{/if}
		{#if data.history}<div class="mt-4 flex flex-wrap items-center gap-3">
				<Button
					tone="admin"
					onclick={() => goto(resolve(pageHref((data.history?.pagination.page ?? 1) - 1)))}
					disabled={data.history.pagination.page <= 1}
					>{$localization.translate("storefront.search.previous", "Previous")}</Button
				>
				<span class="text-sm text-stone-500"
					>{$localization.translate(
						"admin.search.history.pagination",
						"Page {page} of {pages} · {total} changes",
						{
							page: data.history.pagination.page,
							pages: Math.max(1, data.history.pagination.total_pages),
							total: data.history.pagination.total,
						}
					)}</span
				>
				<Button
					tone="admin"
					onclick={() => goto(resolve(pageHref((data.history?.pagination.page ?? 1) + 1)))}
					disabled={data.history.pagination.page >= data.history.pagination.total_pages}
					>{$localization.translate("storefront.search.next", "Next")}</Button
				>
			</div>{/if}
	</AdminPanel>
</section>
