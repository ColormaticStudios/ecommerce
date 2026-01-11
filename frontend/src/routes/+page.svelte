<script lang="ts">
	import { type ProductModel } from "$lib/models";
	import ProductCard from "$lib/components/ProductCard.svelte";
	import { type API } from "$lib/api";
	import { onMount, getContext } from "svelte";
	import { resolve } from "$app/paths";

	const api: API = getContext("api");
	let products: ProductModel[] = [];

	onMount(async () => {
		const page = await api.listProducts({
			sort: "created_at",
			order: "desc",
			page: 1,
			limit: 12,
		});
		products = page.data;
	});
</script>

<section class="mx-auto max-w-7xl px-4 py-8">
	<h1 class="mb-6 text-2xl font-semibold text-gray-900 dark:text-gray-100">Catalog</h1>

	<div class="grid grid-cols-1 gap-6 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
		{#each products as product (product.sku)}
			<ProductCard
				href={resolve(`/product/${product.id}`)}
				data={{
					//id: product.id, // Unused
					name: product.name,
					description: product.description,
					price: product.price,
					image: product.images?.[0],
					stock: product.stock,
				}}
			/>
		{/each}
	</div>
</section>
