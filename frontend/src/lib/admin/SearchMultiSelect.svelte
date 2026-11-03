<script lang="ts">
	import { getContext } from "svelte";
	import SearchCombobox from "$lib/search/SearchCombobox.svelte";
	import Button from "$lib/components/Button.svelte";
	import { LOCALIZATION_CONTEXT, type LocalizationRuntime } from "$lib/localization/runtime";
	let {
		options,
		values = $bindable([]),
		label,
		searchLabel,
		onchange = () => {},
	}: {
		options: { value: string; label: string }[];
		values?: string[];
		label: string;
		searchLabel?: string;
		onchange?: () => void;
	} = $props();
	const localization = getContext<LocalizationRuntime>(LOCALIZATION_CONTEXT);
	let query = $state("");
	const available = $derived(
		options
			.filter(
				(option) =>
					!values.includes(option.value) &&
					`${option.label} ${option.value}`.toLowerCase().includes(query.toLowerCase())
			)
			.slice(0, 8)
	);
	function select(selection: string) {
		const option = options.find((option) => option.value === selection);
		if (option && !values.includes(option.value)) {
			values = [...values, option.value];
			query = "";
			onchange();
		}
	}
</script>

<fieldset class="space-y-2">
	<legend class="text-sm font-medium text-stone-700 dark:text-stone-200">{label}</legend>
	<div class="flex flex-wrap gap-2">
		{#each values as value (value)}
			{@const name = options.find((option) => option.value === value)?.label ?? value}
			<Button
				tone="admin"
				size="small"
				aria-label={$localization.translate("admin.search.selection.remove", "Remove {name}", {
					name,
				})}
				onclick={() => {
					values = values.filter((selected) => selected !== value);
					onchange();
				}}>{name} <i class="bi bi-x" aria-hidden="true"></i></Button
			>
		{/each}
	</div>
	<SearchCombobox
		tone="admin"
		showEmpty
		bind:value={query}
		items={available.map((option) => ({
			value: option.value,
			label: `${option.label} /${option.value}`,
		}))}
		label={searchLabel ??
			$localization.translate("admin.search.selection.search", "Search options to add")}
		oninput={() => {}}
		onselect={select}
	/>
</fieldset>
