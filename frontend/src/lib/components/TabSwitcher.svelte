<script lang="ts">
	export interface TabSwitcherItem {
		id: string;
		label: string;
		icon?: string;
	}

	interface Props {
		items: TabSwitcherItem[];
		value: string;
		ariaLabel: string;
		class?: string;
		onChange?: (value: string) => void;
	}

	let { items, value = $bindable(), ariaLabel, class: className = "", onChange }: Props = $props();

	const activeIndex = $derived(
		Math.max(
			0,
			items.findIndex((item) => item.id === value)
		)
	);
	const indicatorWidth = $derived(
		items.length > 0 ? `calc((100% - 0.5rem) / ${items.length})` : "0px"
	);
</script>

<div
	class={`relative rounded-full border border-stone-200 bg-white p-1 text-[11px] font-semibold tracking-[0.18em] text-stone-500 uppercase shadow-sm dark:border-stone-800 dark:bg-stone-900 dark:text-stone-400 ${className}`}
	role="tablist"
	aria-label={ariaLabel}
>
	{#if items.length > 0}
		<div
			class="pointer-events-none absolute inset-y-1 left-1 rounded-full bg-stone-900 transition-transform duration-300 ease-out dark:bg-stone-100"
			style={`width: ${indicatorWidth}; transform: translateX(calc(${activeIndex} * 100%));`}
		></div>
	{/if}

	<div
		class="relative grid items-center gap-0"
		style={`grid-template-columns: repeat(${items.length}, minmax(0, 1fr));`}
	>
		{#each items as item (item.id)}
			<button
				type="button"
				role="tab"
				aria-selected={value === item.id}
				class={`min-w-0 cursor-pointer rounded-full px-3 py-2 transition ${
					value === item.id
						? "text-white dark:text-stone-900"
						: "hover:text-stone-900 dark:hover:text-stone-100"
				}`}
				onclick={() => {
					value = item.id;
					onChange?.(item.id);
				}}
			>
				<span class="inline-flex min-w-0 items-center justify-center gap-1.5 leading-[1.15]">
					{#if item.icon}
						<i class={`bi ${item.icon} text-[0.95em]`}></i>
					{/if}
					<span class="block whitespace-nowrap">{item.label}</span>
				</span>
			</button>
		{/each}
	</div>
</div>
