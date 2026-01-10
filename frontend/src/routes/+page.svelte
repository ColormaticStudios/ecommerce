<script lang="ts">
	import { type ProductModel } from "$lib/models";
	import { type API } from "$lib/api";
	import { formatPrice } from "$lib/utils";
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
	<h1 class="mb-6 text-2xl font-semibold text-gray-900 dark:text-gray-100">New Arrivals</h1>

	<div class="grid grid-cols-1 gap-6 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
		{#each products as product (product.sku)}
			<a
				href={resolve(`/product/${product.id}`)}
				class="block rounded-xl border border-gray-200 bg-gray-50 transition
					hover:border-gray-300 hover:bg-gray-100 hover:shadow-sm
					dark:border-gray-800 dark:bg-gray-900 dark:hover:border-gray-700 dark:hover:bg-gray-800"
			>
				<!-- Image -->
				<div class="aspect-square overflow-hidden rounded-t-xl bg-gray-200 dark:bg-gray-700">
					{#if product.images?.length}
						<img
							src={product.images[0]}
							alt={product.name}
							class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
							loading="lazy"
						/>
					{:else}
						<div class="flex h-full items-center justify-center text-gray-400">No image</div>
					{/if}
				</div>

				<!-- Content -->
				<div class="flex flex-col gap-2 p-4">
					<h2 class="line-clamp-1 text-base font-medium text-gray-900 dark:text-gray-100">
						{product.name}
					</h2>

					<p class="line-clamp-2 text-sm text-gray-600 dark:text-gray-400">
						{product.description}
					</p>

					<div class="mt-2 flex items-center justify-between">
						<span class="text-lg font-semibold text-gray-900 dark:text-gray-100">
							{formatPrice(product.price)}
						</span>

						{#if product.stock === 0}
							<span class="text-xs font-medium text-red-500"> Out of stock </span>
						{:else if product.stock < 5}
							<span class="text-xs font-medium text-amber-500"> Low stock </span>
						{/if}
					</div>
				</div>
			</a>
		{/each}
	</div>
</section>
