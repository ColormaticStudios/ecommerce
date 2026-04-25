<script lang="ts">
	import { slide } from "svelte/transition";
	import AdminListItem from "$lib/admin/AdminListItem.svelte";
	import Button from "$lib/components/Button.svelte";
	import Badge from "$lib/components/Badge.svelte";
	import type { ProductModel, ProductVariantModel } from "$lib/models";
	import { formatPrice } from "$lib/utils";

	type BadgeTone = "neutral" | "info" | "success" | "warning" | "danger";

	interface ProductMeta {
		label: string;
		tone?: BadgeTone;
	}

	interface Props {
		products: ProductModel[];
		searchQuery?: string;
		selectedProductId?: number | null;
		selectedVariantId?: number | null;
		loading?: boolean;
		class?: string;
		searchPlaceholder?: string;
		searchButtonLabel?: string;
		emptyMessage?: string;
		loadingMessage?: string;
		productMeta?: (product: ProductModel) => ProductMeta;
		onSearch?: () => void;
		toolbarActions?: import("svelte").Snippet;
	}

	let {
		products,
		searchQuery = $bindable(""),
		selectedProductId = $bindable<number | null>(null),
		selectedVariantId = $bindable<number | null>(null),
		loading = false,
		class: className = "",
		searchPlaceholder = "Search products",
		searchButtonLabel = "Search",
		emptyMessage = "No products found.",
		loadingMessage = "Loading products...",
		productMeta,
		onSearch,
		toolbarActions,
	}: Props = $props();

	const selectedProduct = $derived(
		selectedProductId
			? (products.find((product) => product.id === selectedProductId) ?? null)
			: null
	);

	function variantLabel(variant: ProductVariantModel): string {
		const optionLabel = variant.selections
			.map((selection) => `${selection.option_name}: ${selection.option_value}`)
			.join(", ");
		return optionLabel || variant.title || variant.sku;
	}

	function productMetaLabel(product: ProductModel): ProductMeta {
		return (
			productMeta?.(product) ?? { label: `${product.variants.length} variants`, tone: "neutral" }
		);
	}

	function selectProduct(product: ProductModel) {
		selectedProductId = product.id;
		const variant =
			product.variants.find((item) => item.id === product.default_variant_id) ??
			product.variants[0] ??
			null;
		selectedVariantId = variant?.id ?? null;
	}

	function selectVariant(variant: ProductVariantModel) {
		selectedVariantId = variant.id;
	}

	function submitSearch(event: SubmitEvent) {
		event.preventDefault();
		onSearch?.();
	}

	$effect(() => {
		if (!selectedProduct) {
			if (selectedProductId !== null) {
				selectedProductId = null;
			}
			if (selectedVariantId !== null) {
				selectedVariantId = null;
			}
			return;
		}

		if (!selectedProduct.variants.some((variant) => variant.id === selectedVariantId)) {
			const fallback =
				selectedProduct.variants.find(
					(variant) => variant.id === selectedProduct.default_variant_id
				) ??
				selectedProduct.variants[0] ??
				null;
			selectedVariantId = fallback?.id ?? null;
		}
	});
</script>

<div class={`space-y-3 ${className}`.trim()}>
	<form class="flex flex-col gap-2 sm:flex-row" onsubmit={submitSearch}>
		<input
			class="min-w-0 flex-1 rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm text-stone-900 transition outline-none focus:border-stone-500 focus:ring-2 focus:ring-stone-200 dark:border-stone-700 dark:bg-stone-900 dark:text-stone-100 dark:focus:border-stone-500 dark:focus:ring-stone-800"
			placeholder={searchPlaceholder}
			bind:value={searchQuery}
		/>
		<Button tone="admin" type="submit" disabled={loading}>
			<i class="bi bi-search"></i>
			{searchButtonLabel}
		</Button>
		{#if toolbarActions}
			<div class="flex flex-wrap gap-2">{@render toolbarActions()}</div>
		{/if}
	</form>

	{#if loading && products.length === 0}
		<div
			class="rounded-lg border border-stone-200 bg-white px-4 py-3 text-sm text-stone-500 dark:border-stone-800 dark:bg-stone-950 dark:text-stone-400"
		>
			{loadingMessage}
		</div>
	{:else if products.length === 0}
		<div
			class="rounded-lg border border-stone-200 bg-white px-4 py-3 text-sm text-stone-500 dark:border-stone-800 dark:bg-stone-950 dark:text-stone-400"
		>
			{emptyMessage}
		</div>
	{:else}
		<div class="max-h-96 space-y-2 overflow-y-auto pr-1">
			{#each products as product (product.id)}
				<AdminListItem
					active={selectedProductId === product.id}
					interactive={selectedProductId !== product.id}
					class="overflow-hidden"
				>
					<button
						type="button"
						class="flex w-full cursor-pointer items-center justify-between gap-3 p-4 text-left"
						aria-expanded={selectedProductId === product.id}
						onclick={() => selectProduct(product)}
					>
						<div class="min-w-0">
							<p class="truncate text-sm font-semibold text-stone-950 dark:text-stone-50">
								{product.name}
							</p>
							<p class="truncate text-xs text-stone-500 dark:text-stone-400">
								SKU {product.sku} · {formatPrice(product.price)}
							</p>
						</div>
						<Badge tone={productMetaLabel(product).tone ?? "neutral"} size="md">
							{productMetaLabel(product).label}
						</Badge>
					</button>
					{#if selectedProductId === product.id}
						<div
							class="border-t border-stone-200 bg-stone-50/70 px-4 py-3 dark:border-stone-800 dark:bg-stone-900/40"
							transition:slide={{ duration: 180 }}
						>
							<div class="flex flex-wrap gap-2">
								{#each product.variants as variant (variant.id ?? variant.sku)}
									<button
										type="button"
										class={`rounded-full border px-3 py-1.5 text-sm transition ${
											selectedVariantId === variant.id
												? "border-stone-900 bg-stone-900 text-white dark:border-stone-100 dark:bg-stone-100 dark:text-stone-950"
												: "border-stone-200 bg-white text-stone-700 hover:border-stone-400 dark:border-stone-700 dark:bg-stone-950 dark:text-stone-200 dark:hover:border-stone-500"
										}`}
										onclick={() => selectVariant(variant)}
									>
										{variantLabel(variant)}
									</button>
								{/each}
							</div>
						</div>
					{/if}
				</AdminListItem>
			{/each}
		</div>
	{/if}
</div>
