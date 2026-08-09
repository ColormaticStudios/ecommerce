<script lang="ts">
	import type { components } from "$lib/api/generated/openapi";
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminListItem from "$lib/admin/AdminListItem.svelte";
	import Badge from "$lib/components/Badge.svelte";

	type CmsPageResponse = components["schemas"]["CmsPageResponse"];
	interface Props {
		pages: CmsPageResponse[];
		selectedId: number | null;
		onSelect: (page: CmsPageResponse) => void;
	}
	let { pages, selectedId, onSelect }: Props = $props();
</script>

{#if pages.length === 0}<AdminEmptyState>No CMS pages yet.</AdminEmptyState>{/if}
<div class="space-y-3">
	{#each pages as page (page.page.id)}
		<AdminListItem
			as="button"
			active={selectedId === page.page.id}
			interactive={selectedId !== page.page.id}
			class="flex items-center justify-between gap-3 p-4"
			onclick={() => onSelect(page)}
		>
			<div class="min-w-0">
				<div class="truncate font-medium">{page.page.title}</div>
				<div class="truncate text-xs text-stone-500">{page.page.path}</div>
			</div>
			<div class="flex flex-wrap justify-end gap-1">
				<Badge tone={page.entry.published_version_id ? "success" : "warning"} size="sm"
					>{page.entry.published_version_id ? "Published" : "Unpublished"}</Badge
				>
				{#if page.has_unpublished_draft}<Badge tone="info" size="sm">Draft</Badge>{/if}
			</div>
		</AdminListItem>
	{/each}
</div>
