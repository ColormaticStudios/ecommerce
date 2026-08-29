<script lang="ts">
	import { getContext, onDestroy, onMount } from "svelte";
	import type { API } from "$lib/api";
	import type { components } from "$lib/api/generated/openapi";

	type Assignee = components["schemas"]["LocalizationAssignee"];

	interface Props {
		id: string;
		value?: number | null;
		placeholder?: string;
		ariaLabel?: string;
		disabled?: boolean;
	}

	let {
		id,
		value = $bindable(null),
		placeholder = "Search people",
		ariaLabel = "Assignee",
		disabled = false,
	}: Props = $props();

	const api: API = getContext("api");
	const listboxID = $derived(`${id}-listbox`);
	let search = $state("");
	let options = $state<Assignee[]>([]);
	let open = $state(false);
	let loading = $state(false);
	let selected = $state<Assignee | null>(null);
	let highlightedIndex = $state(-1);
	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	let blurTimer: ReturnType<typeof setTimeout> | undefined;
	let requestSequence = 0;

	async function loadOptions(query: string) {
		const sequence = ++requestSequence;
		loading = true;
		try {
			const response = await api.listAdminLocalizationAssignees({
				q: query.trim() || undefined,
				limit: 20,
			});
			if (sequence !== requestSequence) return;
			options = response.assignees;
			highlightedIndex = options.length ? 0 : -1;
			if (value !== null) {
				const current = options.find((option) => option.id === value);
				if (current) {
					selected = current;
					search = current.name;
				}
			}
		} finally {
			if (sequence === requestSequence) loading = false;
		}
	}

	function scheduleSearch(query: string) {
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(() => void loadOptions(query), 180);
	}

	function handleInput(event: Event) {
		search = (event.currentTarget as HTMLInputElement).value;
		if (selected) {
			selected = null;
			value = null;
		}
		open = true;
		scheduleSearch(search);
	}

	function choose(option: Assignee) {
		if (blurTimer) clearTimeout(blurTimer);
		selected = option;
		value = option.id;
		search = option.name;
		open = false;
	}

	function clearSelection() {
		selected = null;
		value = null;
		search = "";
		open = true;
		void loadOptions("");
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === "ArrowDown") {
			event.preventDefault();
			open = true;
			highlightedIndex = Math.min(highlightedIndex + 1, options.length - 1);
		} else if (event.key === "ArrowUp") {
			event.preventDefault();
			highlightedIndex = Math.max(highlightedIndex - 1, 0);
		} else if (event.key === "Enter" && open && highlightedIndex >= 0) {
			event.preventDefault();
			choose(options[highlightedIndex]);
		} else if (event.key === "Escape") {
			open = false;
		}
	}

	function handleFocus() {
		if (blurTimer) clearTimeout(blurTimer);
		open = true;
		if (!options.length) void loadOptions(value === null ? search : String(value));
	}

	function handleBlur() {
		blurTimer = setTimeout(() => {
			open = false;
			if (selected) search = selected.name;
		}, 120);
	}

	onMount(() => {
		void loadOptions(value === null ? "" : String(value));
	});

	onDestroy(() => {
		if (searchTimer) clearTimeout(searchTimer);
		if (blurTimer) clearTimeout(blurTimer);
	});
</script>

<div class="relative">
	<div class="relative">
		<input
			{id}
			type="search"
			role="combobox"
			aria-label={ariaLabel}
			aria-expanded={open}
			aria-controls={listboxID}
			aria-autocomplete="list"
			aria-activedescendant={open && highlightedIndex >= 0
				? `${listboxID}-${options[highlightedIndex]?.id}`
				: undefined}
			{placeholder}
			{disabled}
			value={search}
			oninput={handleInput}
			onfocus={handleFocus}
			onblur={handleBlur}
			onkeydown={handleKeydown}
			class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 pr-9 text-sm text-stone-900 transition outline-none focus:border-stone-500 focus:ring-2 focus:ring-stone-200 disabled:bg-stone-100 disabled:text-stone-400 dark:border-stone-700 dark:bg-stone-900 dark:text-stone-100 dark:focus:border-stone-500 dark:focus:ring-stone-800 dark:disabled:bg-stone-950"
		/>
		{#if selected && !disabled}
			<button
				type="button"
				aria-label="Clear assignee"
				class="absolute top-1/2 right-2 -translate-y-1/2 cursor-pointer rounded p-1 text-stone-400 hover:text-stone-900 dark:hover:text-stone-100"
				onmousedown={(event) => event.preventDefault()}
				onclick={clearSelection}
			>
				<i class="bi bi-x-lg"></i>
			</button>
		{/if}
	</div>

	{#if open && !disabled}
		<div
			id={listboxID}
			role="listbox"
			class="absolute z-40 mt-1 max-h-64 w-full overflow-y-auto rounded-xl border border-stone-200 bg-white p-1 shadow-xl dark:border-stone-700 dark:bg-stone-950"
		>
			{#if loading}
				<p class="px-3 py-2 text-xs text-stone-500">Searching people…</p>
			{:else if options.length === 0}
				<p class="px-3 py-2 text-xs text-stone-500">No eligible people found.</p>
			{:else}
				{#each options as option, index (option.id)}
					<button
						id={`${listboxID}-${option.id}`}
						type="button"
						role="option"
						aria-selected={value === option.id}
						class={`flex w-full cursor-pointer items-center gap-3 rounded-lg px-3 py-2 text-left ${index === highlightedIndex ? "bg-stone-100 dark:bg-stone-800" : "hover:bg-stone-50 dark:hover:bg-stone-900"}`}
						onmouseenter={() => (highlightedIndex = index)}
						onmousedown={(event) => event.preventDefault()}
						onclick={() => choose(option)}
					>
						<span class="min-w-0 flex-1">
							<span class="block truncate text-sm font-medium text-stone-950 dark:text-stone-50"
								>{option.name}</span
							>
							<span class="block truncate text-xs text-stone-500">{option.email}</span>
						</span>
						<span
							class="rounded-full bg-stone-100 px-2 py-0.5 text-[10px] font-semibold text-stone-600 capitalize dark:bg-stone-800 dark:text-stone-300"
							>{option.localization_role}</span
						>
					</button>
				{/each}
			{/if}
		</div>
	{/if}
</div>
