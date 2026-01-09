<script lang="ts">
	import { type ProductModel } from "$lib/models";
	import { type API } from "$lib/api";
	import { formatPrice } from "$lib/utils";
	import { onMount, getContext } from "svelte";
	import { page } from "$app/state";

	const api: API = getContext("api");
	const productID = page.params.id;

	let product: ProductModel | null = null;
	let selectedImage = 0;
	let loading = true;

	onMount(async () => {
		product = await api.getProduct(parseInt(productID ?? "0"));
		loading = false;
	});
</script>

<section class="mx-auto max-w-7xl px-4 py-8">
	{#if loading}
		<div class="grid animate-pulse grid-cols-1 gap-8 md:grid-cols-2">
			<!-- Image skeleton -->
			<div class="aspect-square rounded-xl bg-gray-200 dark:bg-gray-700"></div>

			<!-- Details skeleton -->
			<div class="space-y-4">
				<div class="h-6 w-3/4 rounded bg-gray-200 dark:bg-gray-700"></div>
				<div class="h-4 w-full rounded bg-gray-200 dark:bg-gray-700"></div>
				<div class="h-4 w-5/6 rounded bg-gray-200 dark:bg-gray-700"></div>

				<div class="h-8 w-1/3 rounded bg-gray-200 dark:bg-gray-700"></div>

				<div class="flex gap-3">
					<div class="h-12 flex-1 rounded bg-gray-200 dark:bg-gray-700"></div>
					<!--<div class="h-12 w-28 rounded bg-gray-200 dark:bg-gray-700"></div>-->
					<!-- Re-enable when wishlist is implemented -->
				</div>
			</div>
		</div>
	{:else if product}
		<div class="grid grid-cols-1 gap-8 md:grid-cols-2">
			<!-- Image gallery -->
			<div class="flex flex-col gap-4">
				<div
					class="aspect-square overflow-hidden rounded-xl border border-gray-200 bg-gray-100 dark:border-gray-700 dark:bg-gray-800"
				>
					{#if product.images?.length}
						<img
							src={product.images[selectedImage]}
							alt={product.name}
							class="h-full w-full object-cover"
						/>
					{:else}
						<div class="flex h-full items-center justify-center text-gray-400">
							No image available
						</div>
					{/if}
				</div>

				{#if product.images?.length > 1}
					<div class="flex gap-2">
						{#each product.images as img, i (i)}
							<button
								type="button"
								class="h-16 w-16 overflow-hidden rounded-md border
									{selectedImage === i
									? 'border-gray-900 dark:border-gray-100'
									: 'border-gray-300 dark:border-gray-600'}
									bg-gray-100 dark:bg-gray-700"
								on:click={() => (selectedImage = i)}
								aria-label={`View image ${i + 1}`}
							>
								<img src={img} alt="" class="h-full w-full object-cover" />
							</button>
						{/each}
					</div>
				{/if}
			</div>

			<!-- Product details -->
			<div class="flex flex-col gap-4">
				<h1 class="text-2xl font-semibold text-gray-900 dark:text-gray-100">
					{product.name}
				</h1>

				<p class="text-gray-600 dark:text-gray-400">
					{product.description}
				</p>

				<div class="flex items-center gap-4">
					<span class="text-3xl font-bold text-gray-900 dark:text-gray-100">
						{formatPrice(product.price)}
					</span>

					{#if product.stock === 0}
						<span class="text-sm font-medium text-red-500"> Out of stock </span>
					{:else if product.stock <= 5}
						<span class="text-sm font-medium text-amber-500">
							Only {product.stock} left in stock
						</span>
					{:else}
						<span class="text-sm font-medium text-green-500"> In stock </span>
					{/if}
				</div>

				<!-- Actions -->
				<div class="mt-4 flex gap-3">
					<button
						class="flex-1 cursor-pointer rounded-lg bg-gray-900 px-4 py-3 text-lg font-medium text-white transition hover:bg-gray-800 disabled:cursor-not-allowed disabled:bg-gray-400 dark:bg-gray-100 dark:text-gray-900 dark:hover:bg-gray-200"
						disabled={product.stock === 0}
					>
						<i class="bi bi-bag-plus-fill"></i>
						Add to cart
					</button>

					<!--<button
						class="cursor-pointer rounded-lg border border-gray-300 px-4 py-3 text-lg font-medium text-gray-900 transition hover:bg-gray-100 dark:border-gray-600 dark:text-gray-100 dark:hover:bg-gray-700"
					>
					  <i class="bi bi-list-stars"></i>
						Add to wishlist
					</button>-->
					<!-- Wishlist is not implemented -->
				</div>

				<!-- Metadata -->
				<div
					class="mt-6 rounded-lg border border-gray-200 bg-gray-50 p-4 text-sm text-gray-600 shadow-md dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400"
				>
					<div><strong>SKU:</strong> {product.sku}</div>
					<div>
						<strong>Updated:</strong>
						{product.UpdatedAt.toLocaleDateString()}
					</div>
				</div>
			</div>
		</div>

		<!-- Related products -->
		{#if product.related_products?.length}
			<div class="mt-12">
				<h2 class="mb-4 text-xl font-semibold text-gray-900 dark:text-gray-100">
					Related Products
				</h2>

				<div class="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4">
					{#each product.related_products as related (related.sku)}
						<a
							href={`/product/${related.id}`}
							class="rounded-lg border border-gray-200 bg-gray-50 p-3 transition hover:bg-gray-100 dark:border-gray-700 dark:bg-gray-800 dark:hover:bg-gray-700"
						>
							<div class="line-clamp-1 text-sm font-medium">
								{related.name}
							</div>
							<!--<div class="text-sm text-gray-600 dark:text-gray-400">
								{formatPrice(related.price)}
							</div>-->
							<!-- RelatedProduct currently does not have price -->
						</a>
					{/each}
				</div>
			</div>
		{/if}
	{:else}
		<div class="text-red-500">Product not found.</div>
	{/if}
</section>
