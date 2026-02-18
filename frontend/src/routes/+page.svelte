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
	const skeletonCards = [0, 1, 2, 3, 4, 5, 6, 7];

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
		<div class="grid grid-cols-1 gap-6 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
			{#each skeletonCards as index (index)}
				<div
					class="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-800 dark:bg-gray-900"
				>
					<div class="aspect-[4/3] animate-pulse rounded-xl bg-gray-200 dark:bg-gray-800"></div>
					<div class="mt-4 h-5 w-3/4 animate-pulse rounded bg-gray-200 dark:bg-gray-800"></div>
					<div class="mt-2 h-4 w-1/2 animate-pulse rounded bg-gray-200 dark:bg-gray-800"></div>
					<div class="mt-4 h-9 w-full animate-pulse rounded-lg bg-gray-200 dark:bg-gray-800"></div>
				</div>
			{/each}
		</div>
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
