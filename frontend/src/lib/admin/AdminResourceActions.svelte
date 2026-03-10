<script lang="ts">
	import AdminBadge from "$lib/admin/AdminBadge.svelte";
	import AdminSearchForm from "$lib/admin/AdminSearchForm.svelte";

	interface Props {
		countLabel?: string;
		searchValue?: string;
		searchPlaceholder?: string;
		searchDisabled?: boolean;
		searchRefreshing?: boolean;
		searchFullWidth?: boolean;
		searchClass?: string;
		searchInputClass?: string;
		class?: string;
		onSearch?: () => void;
		onRefresh?: () => void;
		actions?: import("svelte").Snippet;
	}

	let {
		countLabel = "",
		searchValue = $bindable(""),
		searchPlaceholder = "Search",
		searchDisabled = false,
		searchRefreshing = false,
		searchFullWidth = false,
		searchClass = "",
		searchInputClass = "",
		class: className = "",
		onSearch,
		onRefresh,
		actions,
	}: Props = $props();

	const hasSearch = $derived(Boolean(onSearch || onRefresh));
</script>

<div class={`flex flex-wrap items-center gap-2 ${className}`}>
	{#if countLabel.trim()}
		<AdminBadge tone="neutral" size="md">{countLabel}</AdminBadge>
	{/if}

	{#if hasSearch}
		<AdminSearchForm
			fullWidth={searchFullWidth}
			class={searchClass}
			inputClass={searchInputClass}
			placeholder={searchPlaceholder}
			bind:value={searchValue}
			{onSearch}
			{onRefresh}
			refreshing={searchRefreshing}
			disabled={searchDisabled}
		/>
	{/if}

	{#if actions}
		{@render actions()}
	{/if}
</div>
