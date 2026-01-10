<script lang="ts">
	import { type ProductModel } from "$lib/models";
	import { type API } from "$lib/api";
	import { formatPrice } from "$lib/utils";
	import { onMount, getContext, onDestroy } from "svelte";
	import { page } from "$app/state";
	import { resolve } from "$app/paths";

	const api: API = getContext("api");
	const productID = page.params.id;

	let product = $state<ProductModel | null>(null);
	let selectedImage = $state(0);
	let loading = $state(true);
	let adding = $state(false);
	let quantity = $state(1);
	let toastMessage = $state("");
	let toastVisible = $state(false);
	let toastTimeout: ReturnType<typeof setTimeout> | null = null;
	let toastHideTimeout: ReturnType<typeof setTimeout> | null = null;

	function showToast(message: string) {
		toastMessage = message;
		toastVisible = true;

		if (toastTimeout) {
			clearTimeout(toastTimeout);
		}
		if (toastHideTimeout) {
			clearTimeout(toastHideTimeout);
		}

		toastTimeout = setTimeout(() => {
			toastVisible = false;
		}, 4200);

		toastHideTimeout = setTimeout(() => {
			toastMessage = "";
		}, 4600);
	}

	async function addToCart() {
		if (!product) {
			return;
		}

		api.tokenFromCookie();
		if (!api.isAuthenticated()) {
			showToast("Please log in to add items to cart.");
			return;
		}

		const clampedQuantity = Math.min(Math.max(1, Number(quantity) || 1), product.stock);
		quantity = clampedQuantity;
		adding = true;
		try {
			await api.addToCart({ product_id: product.id, quantity: clampedQuantity });
			window.dispatchEvent(new CustomEvent("cart:updated"));
			showToast("Added to cart.");
		} catch (err) {
			console.error(err);
			showToast("Could not add to cart.");
		} finally {
			adding = false;
		}
	}

	onMount(async () => {
		product = await api.getProduct(parseInt(productID ?? "0"));
		loading = false;
	});

	onDestroy(() => {
		if (toastTimeout) {
			clearTimeout(toastTimeout);
		}
		if (toastHideTimeout) {
			clearTimeout(toastHideTimeout);
		}
	});
</script>

{#if toastMessage}
	<div class={`toast ${toastVisible ? "toast-visible" : ""}`} role="status" aria-live="polite">
		<span>{toastMessage}</span>
		{#if toastMessage === "Added to cart."}
			<a href={resolve("/cart")} class="toast-link">Go to cart</a>
		{/if}
	</div>
{/if}

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

				<div class="h-12 flex-1 rounded bg-gray-200 dark:bg-gray-700"></div>
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
								onclick={() => (selectedImage = i)}
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
					<div
						class="flex items-center gap-1 rounded-lg border border-gray-200 bg-white px-2 py-1.5 sm:gap-2 sm:px-3 sm:py-2 dark:border-gray-800 dark:bg-gray-900"
					>
						<button
							type="button"
							class="h-8 w-8 rounded-full border border-gray-300 text-base text-gray-600 transition hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50 sm:h-9 sm:w-9 sm:text-lg dark:border-gray-700 dark:text-gray-200 dark:hover:bg-gray-800"
							disabled={quantity <= 1}
							onclick={() => (quantity = Math.max(1, quantity - 1))}
						>
							-
						</button>
						<input
							class="w-12 text-center text-base font-medium text-gray-900 outline-none sm:w-14 sm:text-lg dark:bg-gray-900 dark:text-gray-100"
							type="number"
							min="1"
							max={product.stock}
							bind:value={quantity}
						/>
						<button
							type="button"
							class="h-8 w-8 rounded-full border border-gray-300 text-base text-gray-600 transition hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50 sm:h-9 sm:w-9 sm:text-lg dark:border-gray-700 dark:text-gray-200 dark:hover:bg-gray-800"
							disabled={product.stock === 0 || quantity >= product.stock}
							onclick={() => (quantity = Math.min(product ? product.stock : 1000, quantity + 1))}
						>
							+
						</button>
					</div>
					<button
						class="flex-1 cursor-pointer rounded-lg bg-gray-900 px-4 py-3 text-lg font-medium text-white transition hover:bg-gray-800 disabled:cursor-not-allowed disabled:bg-gray-400 disabled:hover:bg-gray-400 dark:bg-gray-100 dark:text-gray-900 dark:hover:bg-gray-200"
						disabled={product.stock === 0 || adding}
						onclick={addToCart}
					>
						<i class="bi bi-cart-plus-fill"></i>
						{adding ? "Adding..." : "Add to cart"}
					</button>
				</div>

				<!-- Metadata -->
				<div
					class="mt-6 rounded-lg border border-gray-200 bg-gray-50 p-4 text-sm text-gray-600 shadow-md dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400"
				>
					<div><strong>SKU:</strong> {product.sku}</div>
					<div>
						<strong>Updated:</strong>
						{product.updated_at.toLocaleDateString()}
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
							href={resolve(`/product/${related.id}`)}
							class="rounded-lg border border-gray-200 bg-gray-50 p-3 transition hover:bg-gray-100 dark:border-gray-700 dark:bg-gray-800 dark:hover:bg-gray-700"
						>
							<div class="line-clamp-1 text-sm font-medium">
								{related.name}
							</div>
							{#if related.price != null}
								<div class="text-sm text-gray-600 dark:text-gray-400">
									{formatPrice(related.price)}
								</div>
							{/if}
						</a>
					{/each}
				</div>
			</div>
		{/if}
	{:else}
		<div class="text-red-500">Product not found.</div>
	{/if}
</section>

<style>
	.toast {
		position: fixed;
		top: 1.5rem;
		left: 50%;
		transform: translate(-50%, -20px);
		opacity: 0;
		padding: 0.75rem 1.25rem;
		border-radius: 999px;
		background: rgba(17, 24, 39, 0.92);
		color: white;
		font-size: 0.95rem;
		box-shadow: 0 12px 30px rgba(0, 0, 0, 0.15);
		transition:
			transform 220ms ease,
			opacity 220ms ease;
		display: inline-flex;
		align-items: center;
		gap: 0.75rem;
		backdrop-filter: blur(10px);
		-webkit-backdrop-filter: blur(10px);
		z-index: 50;
	}

	.toast-visible {
		transform: translate(-50%, 0);
		opacity: 1;
	}

	.toast-link {
		color: #93c5fd;
		font-weight: 600;
		text-decoration: none;
	}

	.toast-link:hover {
		text-decoration: underline;
	}
</style>
