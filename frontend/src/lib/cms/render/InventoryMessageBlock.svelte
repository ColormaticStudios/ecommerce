<script lang="ts">
	import type { ProductModel } from "$lib/models";
	import type { CmsBlock } from "./types";

	interface Props {
		block: CmsBlock<"inventory_message">;
		product: ProductModel | null | undefined;
	}

	let { block, product }: Props = $props();

	const message = $derived.by(() => {
		if (!product) return "";
		if (product.stock <= 0) return block.out_of_stock_message || "Out of stock";
		if (product.stock <= (block.low_stock_threshold ?? 5)) {
			return block.low_stock_message || "Low stock";
		}
		return block.in_stock_message || "In stock";
	});
</script>

{#if message}
	<section class="mb-10 rounded-lg border border-gray-200 px-5 py-4 text-sm dark:border-gray-800">
		<p class="font-semibold text-gray-950 dark:text-gray-50">{product?.name}</p>
		<p class="mt-1 text-gray-600 dark:text-gray-300">{message}</p>
	</section>
{/if}
