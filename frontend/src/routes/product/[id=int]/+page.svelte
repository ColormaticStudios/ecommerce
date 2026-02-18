<script lang="ts">
	import { type ProductModel } from "$lib/models";
	import { type API } from "$lib/api";
	import IconButton from "$lib/components/IconButton.svelte";
	import QuantitySelector from "$lib/components/QuantitySelector.svelte";
	import Toast from "$lib/components/Toast.svelte";
	import { formatPrice } from "$lib/utils";
	import ProductCard from "$lib/components/ProductCard.svelte";
	import { userStore } from "$lib/user";
	import { getContext, onDestroy } from "svelte";
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import { resolve } from "$app/paths";

	const api: API = getContext("api");
	const productId = $derived(Number(page.params.id));

	let product = $state<ProductModel | null>(null);
	let selectedImage = $state(0);
	let loading = $state(true);
	let adding = $state(false);
	let quantity = $state(1);
	let toastMessage = $state("");
	let toastVisible = $state(false);
	let toastTimeout: ReturnType<typeof setTimeout> | null = null;
	let toastHideTimeout: ReturnType<typeof setTimeout> | null = null;
	let loadSequence = 0;

	function clearToast() {
		if (toastTimeout) {
			clearTimeout(toastTimeout);
		}
		if (toastHideTimeout) {
			clearTimeout(toastHideTimeout);
		}
		toastVisible = false;
		toastMessage = "";
	}

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

		const authenticated = await api.refreshAuthState();
		if (!authenticated) {
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

	$effect(() => {
		const id = productId;
		if (!Number.isFinite(id) || id <= 0) {
			product = null;
			loading = false;
			return;
		}

		const sequence = ++loadSequence;
		loading = true;
		product = null;
		selectedImage = 0;
		quantity = 1;

		(async () => {
			try {
				const fetched = await api.getProduct(id);
				if (sequence !== loadSequence) {
					return;
				}
				product = fetched;
			} catch (err) {
				console.error(err);
				if (sequence === loadSequence) {
					product = null;
				}
			} finally {
				if (sequence === loadSequence) {
					loading = false;
				}
			}
		})();
	});

	onDestroy(() => {
		clearToast();
	});
</script>

<Toast
	message={toastMessage}
	visible={toastVisible}
	tone={toastMessage === "Could not add to cart." ? "error" : "success"}
	position="top-center"
	actionHref={toastMessage === "Added to cart." ? resolve("/cart") : undefined}
	actionLabel={toastMessage === "Added to cart." ? "Go to cart" : ""}
	onClose={clearToast}
/>

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
								class="h-16 w-16 cursor-pointer overflow-hidden rounded-md border
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
				<div class="flex items-start justify-between gap-3">
					<h1 class="text-2xl font-semibold text-gray-900 dark:text-gray-100">
						{product.name}
					</h1>
					{#if $userStore?.role === "admin"}
						<IconButton
							size="md"
							outlined={true}
							aria-label="Edit product"
							title="Edit product"
							onclick={() => goto(resolve(`/admin/product/${product!.id}`))}
						>
							<i class="bi bi-wrench-adjustable"></i>
						</IconButton>
					{/if}
				</div>

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
				<div class="mt-4 flex flex-wrap gap-3">
					<QuantitySelector bind:value={quantity} min={1} max={product.stock} />
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

				<div class="flex gap-4 overflow-x-auto pb-2">
					{#each product.related_products as related (related.sku)}
						<div class="w-64 shrink-0">
							<ProductCard
								href={resolve(`/product/${related.id}`)}
								showStock={true}
								imageAspect="wide"
								data={{
									//id: related.id, // Unused
									name: related.name,
									description: related.description,
									price: related.price,
									image: related.cover_image,
									stock: related.stock,
								}}
							/>
						</div>
					{/each}
				</div>
			</div>
		{/if}
	{:else}
		<div class="text-red-500">Product not found.</div>
	{/if}
</section>
