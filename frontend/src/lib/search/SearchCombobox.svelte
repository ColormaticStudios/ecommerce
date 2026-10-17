<script lang="ts">
	import { getContext, tick } from "svelte";
	import { LOCALIZATION_CONTEXT, type LocalizationRuntime } from "$lib/localization/runtime";
	import { searchOptions, type SearchOption } from "./combobox-options";
	import TextInput from "$lib/components/TextInput.svelte";
	let {
		value = $bindable(""),
		suggestions = [],
		items = [],
		corrections = [],
		disabled = false,
		tone = "default",
		showEmpty = false,
		label,
		oninput,
		onselect,
	}: {
		value?: string;
		suggestions?: string[];
		items?: SearchOption[];
		corrections?: string[];
		disabled?: boolean;
		tone?: "default" | "admin";
		showEmpty?: boolean;
		label: string;
		oninput: (event: Event) => void;
		onselect: (value: string) => void;
	} = $props();
	const id = $props.id();
	const localization = getContext<LocalizationRuntime>(LOCALIZATION_CONTEXT);
	let open = $state(false);
	let active = $state(-1);
	const options = $derived(searchOptions(items, suggestions, corrections));
	const expanded = $derived(
		open && !disabled && (showEmpty || !!value.trim()) && options.length > 0
	);
	function select(option: SearchOption) {
		value = option.label;
		open = false;
		active = -1;
		onselect(option.value);
	}
	function keydown(event: KeyboardEvent) {
		if (event.key === "Escape") {
			event.preventDefault();
			open = false;
			active = -1;
			return;
		}
		if (event.key === "Tab") {
			open = false;
			return;
		}
		if (event.key === "ArrowDown" || event.key === "ArrowUp") {
			if (!options.length) return;
			event.preventDefault();
			open = true;
			active =
				event.key === "ArrowDown"
					? (active + 1) % options.length
					: active <= 0
						? options.length - 1
						: active - 1;
		} else if (expanded && event.key === "Enter" && active >= 0 && options[active]) {
			event.preventDefault();
			select(options[active]);
		} else if (expanded && (event.key === "Home" || event.key === "End")) {
			event.preventDefault();
			active = event.key === "Home" ? 0 : options.length - 1;
		}
		if (active >= 0 && ["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) {
			void tick().then(() =>
				document.getElementById(`${id}-option-${active}`)?.scrollIntoView({ block: "nearest" })
			);
		}
	}
</script>

<div class="relative min-w-0 flex-1">
	<TextInput
		{tone}
		type="search"
		role="combobox"
		aria-label={label}
		placeholder={label}
		aria-autocomplete="list"
		aria-expanded={expanded}
		aria-controls={`${id}-list`}
		aria-activedescendant={expanded && active >= 0 && options[active]
			? `${id}-option-${active}`
			: undefined}
		bind:value
		{disabled}
		onkeydown={keydown}
		onfocus={() => {
			open = true;
		}}
		onclick={() => {
			open = true;
		}}
		onblur={() => {
			open = false;
		}}
		oninput={(event) => {
			active = -1;
			open = true;
			oninput(event);
		}}
	/>
	{#if expanded}
		<ul
			id={`${id}-list`}
			role="listbox"
			aria-label={label}
			class="absolute z-20 mt-1 max-h-64 w-full overflow-y-auto rounded-lg border border-gray-300 bg-white p-1 shadow-lg dark:border-gray-700 dark:bg-gray-900"
		>
			{#each options as option, index (option.value)}
				<li id={`${id}-option-${index}`} role="option" aria-selected={index === active}>
					<button
						type="button"
						tabindex="-1"
						class={`w-full rounded px-3 py-2 text-left text-sm ${index === active ? "bg-gray-200 dark:bg-gray-700" : "hover:bg-gray-100 dark:hover:bg-gray-800"}`}
						onmousedown={(event) => event.preventDefault()}
						onclick={() => select(option)}
						>{#if option.kind === "correction"}<span
								class="block text-xs text-gray-500 dark:text-gray-400"
								>{$localization.translate("storefront.search.correction_label", "Did you mean") +
									" "}</span
							>{/if}{option.label}</button
					>
				</li>
			{/each}
		</ul>
	{/if}
</div>
