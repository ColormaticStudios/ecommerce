<script lang="ts">
	import Button from "$lib/components/Button.svelte";

	interface Props {
		page: number;
		totalPages: number;
		totalItems?: number | null;
		limit: number;
		limitOptions: number[];
		loading?: boolean;
		loadingLabel?: string;
		onLimitChange?: (limit: number) => void;
		onPrev?: () => void;
		onNext?: () => void;
	}

	let {
		page,
		totalPages,
		totalItems = null,
		limit,
		limitOptions,
		loading = false,
		loadingLabel = "Refreshing...",
		onLimitChange,
		onPrev,
		onNext,
	}: Props = $props();
</script>

<div
	class="flex flex-wrap items-center justify-between gap-3 pt-2 text-xs text-gray-500 dark:text-gray-400"
>
	{#if loading}
		<span class="text-xs text-gray-500 dark:text-gray-400">{loadingLabel}</span>
	{/if}
	<div class="flex items-center gap-2">
		<span>Per page</span>
		<select
			class="cursor-pointer rounded-md border border-gray-300 bg-gray-100 px-2 py-1 text-xs dark:border-gray-700 dark:bg-gray-800"
			value={limit}
			onchange={(event) => onLimitChange?.(Number((event.target as HTMLSelectElement).value))}
		>
			{#each limitOptions as option, i (i)}
				<option value={option}>{option}</option>
			{/each}
		</select>
	</div>
	<span>
		Page {page} of {totalPages}
		{#if totalItems !== null}
			({totalItems} total)
		{/if}
	</span>
	<div class="flex items-center gap-2">
		<Button
			variant="regular"
			size="small"
			type="button"
			disabled={page <= 1}
			onclick={() => onPrev?.()}
		>
			<i class="bi bi-arrow-left"></i>
			Prev
		</Button>
		<Button
			variant="regular"
			size="small"
			type="button"
			disabled={page >= totalPages}
			onclick={() => onNext?.()}
		>
			Next
			<i class="bi bi-arrow-right"></i>
		</Button>
	</div>
</div>
