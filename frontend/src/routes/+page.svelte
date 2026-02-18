<script lang="ts">
	import { type ProductModel } from "$lib/models";
	import Alert from "$lib/components/Alert.svelte";
	import ProductCard from "$lib/components/ProductCard.svelte";
	import { type API } from "$lib/api";
	import { onMount, getContext } from "svelte";
	import { resolve } from "$app/paths";

	const api: API = getContext("api");
	let products = $state<ProductModel[]>([]);
	let loading = $state(true);
	let errorMessage = $state("");

	onMount(async () => {
		loading = true;
		errorMessage = "";
		try {
			const page = await api.listProducts({
				sort: "created_at",
				order: "desc",
				page: 1,
				limit: 12,
			});
			products = page.data;
		} catch (err) {
			console.error(err);
			errorMessage = "Unable to load catalog products.";
		} finally {
			loading = false;
		}
	});
</script>

<section class="mx-auto max-w-7xl px-4 py-8">
	<h1 class="mb-6 text-2xl font-semibold text-gray-900 dark:text-gray-100">Catalog</h1>

	{#if loading}
		<div class="text-sm text-gray-500 dark:text-gray-400">Loading products…</div>
	{:else if errorMessage}
		<Alert
			message={errorMessage}
			tone="error"
			icon="bi-x-circle-fill"
			onClose={() => (errorMessage = "")}
		/>
	{:else}
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
	{/if}
</section>
