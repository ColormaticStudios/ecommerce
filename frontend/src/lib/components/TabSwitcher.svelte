<script lang="ts">
	import { tick } from "svelte";
	import { fade } from "svelte/transition";

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
	let scrollViewport: HTMLDivElement | null = $state(null);
	let tabList: HTMLDivElement | null = $state(null);
	let indicatorLeft = $state("0px");
	let indicatorWidth = $state("0px");
	let canScrollLeft = $state(false);
	let canScrollRight = $state(false);

	const activeIndex = $derived(
		Math.max(
			0,
			items.findIndex((item) => item.id === value)
		)
	);

	function syncIndicator() {
		if (!tabList || items.length === 0) {
			indicatorLeft = "0px";
			indicatorWidth = "0px";
			syncScrollAffordance();
			return;
		}

		const tabs = Array.from(tabList.querySelectorAll<HTMLButtonElement>('[role="tab"]'));
		const activeTab = tabs[activeIndex];
		if (!activeTab) {
			indicatorLeft = "0px";
			indicatorWidth = "0px";
			syncScrollAffordance();
			return;
		}

		indicatorLeft = `${activeTab.offsetLeft}px`;
		indicatorWidth = `${activeTab.offsetWidth}px`;
		activeTab.scrollIntoView({ block: "nearest", inline: "nearest" });
		syncScrollAffordance();
	}

	function syncScrollAffordance() {
		if (!scrollViewport) {
			canScrollLeft = false;
			canScrollRight = false;
			return;
		}

		const maxScrollLeft = scrollViewport.scrollWidth - scrollViewport.clientWidth;
		canScrollLeft = scrollViewport.scrollLeft > 2;
		canScrollRight = maxScrollLeft - scrollViewport.scrollLeft > 2;
	}

	function scheduleIndicatorSync(nextValue: string, nextItems: TabSwitcherItem[]) {
		const hasActiveTab = nextItems.some((item) => item.id === nextValue);
		if (!hasActiveTab && nextItems.length > 0) {
			void tick().then(syncIndicator);
			return;
		}

		void tick().then(syncIndicator);
	}

	function observeTabSizes(observedItems: TabSwitcherItem[]) {
		if (!tabList || !scrollViewport) {
			return;
		}

		const observer = new ResizeObserver(() => {
			syncIndicator();
		});
		const tabs = Array.from(tabList.querySelectorAll<HTMLButtonElement>('[role="tab"]'));

		observer.observe(scrollViewport);
		observer.observe(tabList);
		for (const tab of tabs) {
			observer.observe(tab);
		}

		if (tabs.length !== observedItems.length) {
			syncIndicator();
		}

		syncIndicator();

		return () => {
			observer.disconnect();
		};
	}

	$effect(() => {
		scheduleIndicatorSync(value, items);
	});

	$effect(() => {
		return observeTabSizes(items);
	});
</script>

<div class={`relative max-w-full ${className}`}>
	<div
		bind:this={scrollViewport}
		class="tab-switcher-scroll max-w-full overflow-x-auto overflow-y-hidden rounded-full border border-stone-200 bg-white px-1 text-[11px] font-semibold tracking-[0.18em] text-stone-500 uppercase shadow-sm dark:border-stone-800 dark:bg-stone-900 dark:text-stone-400"
		onscroll={syncScrollAffordance}
	>
		<div
			bind:this={tabList}
			class="relative inline-flex w-max min-w-full items-center gap-1"
			role="tablist"
			aria-label={ariaLabel}
		>
			{#if items.length > 0}
				<div
					class="pointer-events-none absolute top-1 bottom-1 rounded-full bg-stone-900 transition-[transform,width] duration-300 ease-out dark:bg-stone-100"
					style={`width: ${indicatorWidth}; transform: translateX(${indicatorLeft});`}
				></div>
			{/if}

			{#each items as item (item.id)}
				<button
					type="button"
					role="tab"
					aria-selected={value === item.id}
					class={`relative shrink-0 cursor-pointer rounded-full px-3 py-2 transition ${
						value === item.id
							? "text-white dark:text-stone-900"
							: "hover:text-stone-900 dark:hover:text-stone-100"
					}`}
					onclick={() => {
						value = item.id;
						onChange?.(item.id);
					}}
				>
					<span
						class="inline-flex items-center justify-center gap-1.5 leading-[1.15] whitespace-nowrap"
					>
						{#if item.icon}
							<i class={`bi ${item.icon} text-[0.95em]`}></i>
						{/if}
						<span class="block">{item.label}</span>
					</span>
				</button>
			{/each}
		</div>
	</div>

	{#if canScrollLeft}
		<div
			transition:fade={{ duration: 180 }}
			class="pointer-events-none absolute top-0 bottom-0 left-0 z-10 flex w-11 items-center rounded-l-full border-y border-l border-stone-200 bg-linear-to-r from-white via-white to-transparent dark:border-stone-800 dark:from-stone-900 dark:via-stone-900"
			aria-hidden="true"
		>
			<i class="bi bi-chevron-left ml-2.5 text-[10px] text-stone-400 dark:text-stone-500"></i>
		</div>
	{/if}

	{#if canScrollRight}
		<div
			transition:fade={{ duration: 180 }}
			class="pointer-events-none absolute top-0 right-0 bottom-0 z-10 flex w-11 items-center justify-end rounded-r-full border-y border-r border-stone-200 bg-linear-to-l from-white via-white to-transparent dark:border-stone-800 dark:from-stone-900 dark:via-stone-900"
			aria-hidden="true"
		>
			<i class="bi bi-chevron-right mr-2.5 text-[10px] text-stone-400 dark:text-stone-500"></i>
		</div>
	{/if}
</div>

<style>
	.tab-switcher-scroll {
		-ms-overflow-style: none;
		scrollbar-width: none;
	}

	.tab-switcher-scroll::-webkit-scrollbar {
		display: none;
	}
</style>
