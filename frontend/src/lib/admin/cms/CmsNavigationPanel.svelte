<script lang="ts">
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminListItem from "$lib/admin/AdminListItem.svelte";
	import type {
		NavigationCustomItem,
		NavigationDraftRow,
		NavigationDropdown,
		SelectedNavigationItem,
	} from "./navigation";
	import Badge from "$lib/components/Badge.svelte";

	interface Props {
		location: string;
		dropdowns: NavigationDropdown[];
		customItems: NavigationCustomItem[];
		rows: NavigationDraftRow[];
		selected: SelectedNavigationItem;
		onSelect: (item: SelectedNavigationItem) => void;
	}
	let { location, dropdowns, customItems, rows, selected, onSelect }: Props = $props();
</script>

<div class="space-y-3">
	<AdminListItem
		as="button"
		active={selected.kind === "settings"}
		interactive={selected.kind !== "settings"}
		class="flex items-center justify-between gap-3 p-4"
		onclick={() => onSelect({ kind: "settings" })}
	>
		<div class="min-w-0">
			<div class="truncate font-medium">Navigation settings</div>
			<div class="truncate text-xs text-stone-500">{location || "header"}</div>
		</div>
		<Badge tone="neutral" size="sm">Menu</Badge>
	</AdminListItem>
	{#each dropdowns as dropdown (dropdown.id)}
		<AdminListItem
			as="button"
			active={selected.kind === "dropdown" && selected.id === dropdown.id}
			interactive={selected.kind !== "dropdown" || selected.id !== dropdown.id}
			class="flex items-center justify-between gap-3 p-4"
			onclick={() => onSelect({ kind: "dropdown", id: dropdown.id })}
		>
			<div class="min-w-0">
				<div class="truncate font-medium">{dropdown.label || "Untitled dropdown"}</div>
				<div class="truncate text-xs text-stone-500">Dropdown</div>
			</div>
			<Badge tone="success" size="sm">Dropdown</Badge>
		</AdminListItem>
	{/each}
	{#each customItems as item (item.id)}
		<AdminListItem
			as="button"
			active={selected.kind === "custom" && selected.id === item.id}
			interactive={selected.kind !== "custom" || selected.id !== item.id}
			class="flex items-center justify-between gap-3 p-4"
			onclick={() => onSelect({ kind: "custom", id: item.id })}
		>
			<div class="min-w-0">
				<div class="truncate font-medium">{item.label || "Untitled link"}</div>
				<div class="truncate text-xs text-stone-500">
					{item.url || item.targetRef || "No target"}
				</div>
			</div>
			<Badge tone={item.placement === "hidden" || !item.isEnabled ? "neutral" : "success"} size="sm"
				>{item.placement === "hidden" || !item.isEnabled ? "Hidden" : "Link"}</Badge
			>
		</AdminListItem>
	{/each}
	{#if rows.length === 0}<AdminEmptyState>Create pages before adding page links.</AdminEmptyState
		>{/if}
	{#each rows as row (row.pageId)}
		<AdminListItem
			as="button"
			active={selected.kind === "page" && selected.id === row.pageId}
			interactive={selected.kind !== "page" || selected.id !== row.pageId}
			class="flex items-center justify-between gap-3 p-4"
			onclick={() => onSelect({ kind: "page", id: row.pageId })}
		>
			<div class="min-w-0">
				<div class="truncate font-medium">{row.label || row.title}</div>
				<div class="truncate text-xs text-stone-500">{row.path}</div>
			</div>
			<Badge tone={row.placement === "hidden" || !row.isEnabled ? "neutral" : "success"} size="sm"
				>{row.placement === "hidden" || !row.isEnabled
					? "Hidden"
					: row.placement === "top"
						? "Top level"
						: "Dropdown"}</Badge
			>
		</AdminListItem>
	{/each}
</div>
