<script lang="ts">
	import Button from "$lib/components/Button.svelte";

	interface Props {
		page: number;
		totalPages: number;
		totalItems?: number | null;
		limit: number;
		limitOptions: number[];
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
		onLimitChange,
		onPrev,
		onNext,
	}: Props = $props();
</script>

<div
	class="grid gap-3 pt-2 text-xs text-gray-500 sm:grid-cols-[1fr_auto_1fr] sm:items-center dark:text-gray-400"
>
	<div class="flex flex-wrap items-center gap-2 sm:justify-self-start">
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
	<span class="sm:justify-self-center">
		Page {page} of {totalPages}
		{#if totalItems !== null}
			({totalItems} total)
		{/if}
	</span>
	<div class="flex items-center gap-2 sm:justify-self-end">
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
