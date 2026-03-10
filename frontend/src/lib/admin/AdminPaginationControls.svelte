<script lang="ts">
	import Button from "$lib/components/Button.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";

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
	class="grid gap-3 border-t border-stone-200/80 pt-4 text-xs text-stone-500 sm:grid-cols-[1fr_auto_1fr] sm:items-center dark:border-stone-800 dark:text-stone-400"
>
	<div class="flex flex-wrap items-center gap-2 sm:justify-self-start">
		<span>Per page</span>
		<Dropdown
			tone="admin"
			full={false}
			class="px-2 py-1 text-xs"
			value={limit}
			onchange={(event) => onLimitChange?.(Number((event.target as HTMLSelectElement).value))}
		>
			{#each limitOptions as option, i (i)}
				<option value={option}>{option}</option>
			{/each}
		</Dropdown>
	</div>
	<span class="sm:justify-self-center">
		Page {page} of {totalPages}
		{#if totalItems !== null}
			({totalItems} total)
		{/if}
	</span>
	<div class="flex items-center gap-2 sm:justify-self-end">
		<Button
			tone="admin"
			variant="regular"
			size="small"
			type="button"
			class="rounded-full"
			disabled={page <= 1}
			onclick={() => onPrev?.()}
		>
			<i class="bi bi-arrow-left"></i>
			Prev
		</Button>
		<Button
			tone="admin"
			variant="regular"
			size="small"
			type="button"
			class="rounded-full"
			disabled={page >= totalPages}
			onclick={() => onNext?.()}
		>
			Next
			<i class="bi bi-arrow-right"></i>
		</Button>
	</div>
</div>
