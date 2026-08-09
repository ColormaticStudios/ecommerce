<script lang="ts">
	import AdminFieldLabel from "$lib/admin/AdminFieldLabel.svelte";
	import { adminSurfaceVariantClasses } from "$lib/admin/tokens";
	import Button from "$lib/components/Button.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import IconButton from "$lib/components/IconButton.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
	import type { EditorLayout, EditorOption } from "./types";

	let {
		layout,
		options = $bindable(),
		onAddOption,
		onRemoveOption,
		onAddValue,
		onRemoveValue,
		onGenerate,
	}: {
		layout: EditorLayout;
		options: EditorOption[];
		onAddOption: () => void;
		onRemoveOption: (key: string) => void;
		onAddValue: (key: string) => void;
		onRemoveValue: (optionKey: string, valueKey: string) => void;
		onGenerate: () => void;
	} = $props();
	const sectionClass = $derived(
		layout === "split" ? adminSurfaceVariantClasses["panel-tight"] : ""
	);
	const collectionClass = $derived(
		layout === "split" ? "mt-4 space-y-4" : "mt-4 divide-y divide-stone-200 dark:divide-stone-800"
	);
	const itemClass = $derived(
		layout === "split" ? adminSurfaceVariantClasses.subsurface : "py-4 first:pt-0 last:pb-0"
	);
</script>

<div class={sectionClass}>
	<div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
		<div>
			<AdminFieldLabel>Options</AdminFieldLabel>
			<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
				Define the choice sets that can be combined into sellable variants.
			</p>
		</div>
		<div
			class={layout === "split"
				? "flex w-full shrink-0 flex-col gap-2 sm:w-48"
				: "flex flex-wrap gap-2"}
		>
			<Button
				tone="admin"
				variant="regular"
				type="button"
				class={layout === "split" ? "w-full justify-center whitespace-nowrap" : ""}
				onclick={onAddOption}><i class="bi bi-plus-lg mr-1"></i>Add option</Button
			>
			<Button
				variant="primary"
				type="button"
				class={layout === "split" ? "w-full justify-center whitespace-nowrap" : ""}
				onclick={onGenerate}><i class="bi bi-grid-3x3-gap-fill mr-1"></i>Generate variants</Button
			>
		</div>
	</div>
	{#if options.length === 0}<p class="mt-3 text-sm text-gray-500 dark:text-gray-400">
			No options yet. Add one to build a variant matrix.
		</p>{:else}
		<div class={collectionClass}>
			{#each options as option, optionIndex (option.key)}
				<div class={itemClass}>
					<div class="flex items-start justify-between gap-3">
						<div class="grid flex-1 gap-4 sm:grid-cols-2">
							<div>
								<AdminFieldLabel>Option name</AdminFieldLabel><TextInput
									tone="admin"
									class="mt-1"
									type="text"
									aria-label={`Option ${optionIndex + 1} name`}
									bind:value={option.name}
								/>
							</div>
							<div>
								<AdminFieldLabel>Display type</AdminFieldLabel><Dropdown
									tone="admin"
									class="mt-1"
									aria-label={`Option ${optionIndex + 1} display type`}
									bind:value={option.display_type}
									><option value="select">Select</option><option value="swatch">Swatch</option
									></Dropdown
								>
							</div>
						</div>
						<IconButton
							variant="danger"
							type="button"
							onclick={() => onRemoveOption(option.key)}
							aria-label={`Remove option ${optionIndex + 1}`}
							title="Remove option"><i class="bi bi-trash-fill"></i></IconButton
						>
					</div>
					<div class="mt-4 space-y-3">
						{#each option.values as value (value.key)}<div class="flex items-center gap-2">
								<TextInput
									tone="admin"
									class="flex-1"
									type="text"
									aria-label={`${option.name || `Option ${optionIndex + 1}`} value`}
									bind:value={value.value}
								/><IconButton
									variant="danger"
									type="button"
									onclick={() => onRemoveValue(option.key, value.key)}
									aria-label={`Remove value ${value.value || "value"}`}
									title="Remove value"><i class="bi bi-dash-lg"></i></IconButton
								>
							</div>{/each}<Button
							tone="admin"
							variant="regular"
							type="button"
							onclick={() => onAddValue(option.key)}
							><i class="bi bi-plus-lg mr-1"></i>Add value</Button
						>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>
