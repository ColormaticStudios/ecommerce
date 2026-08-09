<script lang="ts">
	import AdminFieldLabel from "$lib/admin/AdminFieldLabel.svelte";
	import { adminSurfaceVariantClasses } from "$lib/admin/tokens";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import IconButton from "$lib/components/IconButton.svelte";
	import NumberInput from "$lib/components/NumberInput.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
	import type { EditorLayout, EditorVariant } from "./types";

	let {
		layout,
		variants = $bindable(),
		defaultVariantSku = $bindable(),
		onAdd,
		onRemove,
	}: {
		layout: EditorLayout;
		variants: EditorVariant[];
		defaultVariantSku: string;
		onAdd: () => void;
		onRemove: (key: string) => void;
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
	<div class="flex items-center justify-between gap-3">
		<div>
			<AdminFieldLabel>Variants</AdminFieldLabel>
			<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
				Product price and stock are derived from the default variant.
			</p>
		</div>
		<Button
			tone="admin"
			variant="regular"
			type="button"
			class="min-w-38 whitespace-nowrap"
			onclick={onAdd}><i class="bi bi-plus-lg mr-1"></i>Add variant</Button
		>
	</div>
	<div class={collectionClass}>
		{#each variants as variant, variantIndex (variant.key)}
			<div class={itemClass}>
				<div class="flex items-start justify-between gap-3">
					<div class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-200">
						<input
							type="radio"
							name="default-variant"
							value={variant.sku}
							checked={defaultVariantSku === variant.sku}
							onchange={() => (defaultVariantSku = variant.sku)}
						/>Default variant
					</div>
					<IconButton
						variant="danger"
						type="button"
						onclick={() => onRemove(variant.key)}
						aria-label={`Remove variant ${variantIndex + 1}`}
						title="Remove variant"><i class="bi bi-trash-fill"></i></IconButton
					>
				</div>
				<div class="mt-4 grid gap-4 sm:grid-cols-2">
					<div>
						<AdminFieldLabel>Variant SKU</AdminFieldLabel><TextInput
							tone="admin"
							class="mt-1"
							type="text"
							aria-label={`Variant ${variantIndex + 1} SKU`}
							bind:value={variant.sku}
						/>
					</div>
					<div>
						<AdminFieldLabel>Title</AdminFieldLabel><TextInput
							tone="admin"
							class="mt-1"
							type="text"
							aria-label={`Variant ${variantIndex + 1} title`}
							bind:value={variant.title}
						/>
					</div>
					<div>
						<AdminFieldLabel>Price</AdminFieldLabel><NumberInput
							tone="admin"
							class="mt-1"
							allowDecimal={true}
							min="0"
							aria-label={`Variant ${variantIndex + 1} price`}
							bind:value={variant.price}
						/>
					</div>
					<div>
						<AdminFieldLabel>Stock</AdminFieldLabel><NumberInput
							tone="admin"
							class="mt-1"
							min="0"
							aria-label={`Variant ${variantIndex + 1} stock`}
							bind:value={variant.stock}
						/>
					</div>
					<div>
						<AdminFieldLabel>Compare-at price</AdminFieldLabel><NumberInput
							tone="admin"
							class="mt-1"
							allowDecimal={true}
							min="0"
							aria-label={`Variant ${variantIndex + 1} compare-at price`}
							bind:value={variant.compare_at_price}
						/>
					</div>
					<label class="mt-6 flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200"
						><input type="checkbox" bind:checked={variant.is_published} />Variant published</label
					>
				</div>
				{#if variant.selections.length}<div class="mt-4 flex flex-wrap gap-2">
						{#each variant.selections as selection (selection.key)}<Badge tone="neutral" size="sm"
								>{selection.option_name}: {selection.option_value}</Badge
							>{/each}
					</div>{/if}
			</div>
		{/each}
	</div>
</div>
