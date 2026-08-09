<script lang="ts">
	import { resolve } from "$app/paths";
	import Card from "$lib/components/Card.svelte";
	import ProductCard from "$lib/components/ProductCard.svelte";
	import type { ProductModel } from "$lib/models";
	import type { CmsBlock } from "./types";

	interface Props {
		block: CmsBlock<"product_rail">;
		products: ProductModel[];
	}

	let { block, products }: Props = $props();
</script>

<section class="mb-10">
	<div class="mb-5">
		<h2 class="text-2xl font-semibold text-gray-950 dark:text-gray-50">{block.title}</h2>
		{#if block.subtitle}
			<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{block.subtitle}</p>
		{/if}
	</div>
	{#if products.length === 0}
		<Card border="dashed" radius="xl" padding="sm" class="text-sm text-gray-500 dark:text-gray-400">
			No products found for this section.
		</Card>
	{:else}
		<div class="grid grid-cols-1 gap-6 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
			{#each products as product (product.id)}
				<ProductCard
					href={resolve(`/product/${product.id}`)}
					imageAspect={block.image_aspect ?? "square"}
					data={{
						name: product.name,
						brand: product.brand?.name,
						description: product.description,
						price: product.price,
						basePrice: product.base_price,
						discountAmount: product.discount_amount,
						finalPrice: product.final_price,
						priceRange: product.price_range,
						image: product.images?.[0],
						stock: product.stock,
					}}
				/>
			{/each}
		</div>
	{/if}
</section>
