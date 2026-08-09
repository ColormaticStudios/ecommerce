<script lang="ts">
	import type { components } from "$lib/api/generated/openapi";
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminListItem from "$lib/admin/AdminListItem.svelte";
	import Badge from "$lib/components/Badge.svelte";

	type CmsGlobalRegionResponse = components["schemas"]["CmsGlobalRegionResponse"];
	interface Props {
		regions: CmsGlobalRegionResponse[];
		selectedId: number | null;
		onSelect: (region: CmsGlobalRegionResponse) => void;
	}
	let { regions, selectedId, onSelect }: Props = $props();
</script>

{#if regions.length === 0}<AdminEmptyState>No CMS global regions yet.</AdminEmptyState>{/if}
<div class="space-y-3">
	{#each regions as region (region.region.id)}
		<AdminListItem
			as="button"
			active={selectedId === region.region.id}
			interactive={selectedId !== region.region.id}
			class="flex items-center justify-between gap-3 p-4"
			onclick={() => onSelect(region)}
		>
			<div class="min-w-0">
				<div class="truncate font-medium">{region.region.title}</div>
				<div class="truncate text-xs text-stone-500">{region.region.region}</div>
			</div>
			<div class="flex flex-wrap justify-end gap-1">
				<Badge tone={region.entry.published_version_id ? "success" : "warning"} size="sm"
					>{region.entry.published_version_id ? "Published" : "Unpublished"}</Badge
				>
				{#if region.has_unpublished_draft}<Badge tone="info" size="sm">Draft</Badge>{/if}
			</div>
		</AdminListItem>
	{/each}
</div>
