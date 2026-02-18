<script lang="ts">
	import { type API } from "$lib/api";
	import { type CartModel } from "$lib/models";
	import Alert from "$lib/components/alert.svelte";
	import Button from "$lib/components/Button.svelte";
	import ButtonLink from "$lib/components/ButtonLink.svelte";
	import NumberInput from "$lib/components/NumberInput.svelte";
	import { formatPrice } from "$lib/utils";
	import { userStore } from "$lib/user";
	import { getContext, onMount } from "svelte";
	import { resolve } from "$app/paths";

	const api: API = getContext("api");

	let cart = $state<CartModel | null>(null);
	let loading = $state(true);
	let errorMessage = $state("");
	let updatingItemId = $state<number | null>(null);
	let authChecked = $state(false);
	const skeletonRows = [0, 1, 2];

	const total = $derived(
		cart ? cart.items.reduce((sum, item) => sum + item.quantity * item.product.price, 0) : 0
	);

	async function loadCart(options?: { silent?: boolean }) {
		api.tokenFromCookie();
		authChecked = true;
		if (!api.isAuthenticated()) {
			loading = false;
			return;
		}

		if (!options?.silent && !cart) {
			loading = true;
		}
		errorMessage = "";
		try {
			cart = await api.viewCart();
		} catch (err) {
			console.error(err);
			errorMessage = "Unable to load your cart.";
		}
		if (!options?.silent) {
			loading = false;
		}
	}

	async function updateItemQuantity(itemId: number, quantity: number) {
		if (quantity < 1) {
			return;
		}

		updatingItemId = itemId;
		try {
			await api.updateCartItem(itemId, { quantity });
			await loadCart({ silent: true });
			window.dispatchEvent(new CustomEvent("cart:updated"));
		} catch (err) {
			console.error(err);
			errorMessage = "Unable to update that item.";
		}
		updatingItemId = null;
	}

	async function removeItem(itemId: number) {
		updatingItemId = itemId;
		try {
			await api.removeCartItem(itemId);
			await loadCart({ silent: true });
			window.dispatchEvent(new CustomEvent("cart:updated"));
		} catch (err) {
			console.error(err);
			errorMessage = "Unable to remove that item.";
		}
		updatingItemId = null;
	}

	function increaseQuantity(itemId: number, quantity: number) {
		updateItemQuantity(itemId, quantity + 1);
	}

	function decreaseQuantity(itemId: number, quantity: number) {
		if (quantity <= 1) {
			return;
		}
		updateItemQuantity(itemId, quantity - 1);
	}

	onMount(loadCart);
</script>

<section class="mx-auto max-w-6xl px-4 py-10">
	<div class="flex flex-wrap items-end justify-between gap-4">
		<div>
			<h1 class="text-3xl font-semibold text-gray-900 dark:text-gray-100">Your Cart</h1>
		</div>
		{#if cart && cart.items.length}
			<p class="text-sm text-gray-600 dark:text-gray-300">
				{cart.items.length} item{cart.items.length === 1 ? "" : "s"}
			</p>
		{/if}
	</div>

	{#if !authChecked}
		<div class="mt-6 grid gap-6 lg:grid-cols-[1.6fr_0.8fr]">
			<div class="space-y-4">
				{#each skeletonRows as index (index)}
					<div
						class="flex gap-4 rounded-2xl border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-800 dark:bg-gray-900"
					>
						<div class="h-20 w-20 animate-pulse rounded-xl bg-gray-200 dark:bg-gray-800"></div>
						<div class="flex-1 space-y-3">
							<div class="h-4 w-1/2 animate-pulse rounded bg-gray-200 dark:bg-gray-800"></div>
							<div class="h-3 w-1/3 animate-pulse rounded bg-gray-200 dark:bg-gray-800"></div>
							<div class="h-8 w-32 animate-pulse rounded bg-gray-200 dark:bg-gray-800"></div>
						</div>
					</div>
				{/each}
			</div>
			<div
				class="h-64 animate-pulse rounded-2xl border border-gray-200 bg-gray-100 dark:border-gray-800 dark:bg-gray-900"
			></div>
		</div>
	{:else if !api.isAuthenticated()}
		<p class="mt-4 text-gray-600 dark:text-gray-300">
			Please
			<a href={resolve("/login")} class="text-blue-600 hover:underline dark:text-blue-400">
				log in
			</a>
			to view your cart.
		</p>
	{:else if loading}
		<div class="mt-6 grid gap-6 lg:grid-cols-[1.6fr_0.8fr]">
			<div class="space-y-4">
				{#each skeletonRows as index (index)}
					<div
						class="flex gap-4 rounded-2xl border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-800 dark:bg-gray-900"
					>
						<div class="h-20 w-20 animate-pulse rounded-xl bg-gray-200 dark:bg-gray-800"></div>
						<div class="flex-1 space-y-3">
							<div class="h-4 w-1/2 animate-pulse rounded bg-gray-200 dark:bg-gray-800"></div>
							<div class="h-3 w-1/3 animate-pulse rounded bg-gray-200 dark:bg-gray-800"></div>
							<div class="h-8 w-32 animate-pulse rounded bg-gray-200 dark:bg-gray-800"></div>
						</div>
					</div>
				{/each}
			</div>
			<div
				class="h-64 animate-pulse rounded-2xl border border-gray-200 bg-gray-100 dark:border-gray-800 dark:bg-gray-900"
			></div>
		</div>
	{:else if errorMessage}
		<div class="mt-4">
			<Alert
				message={errorMessage}
				tone="error"
				icon="bi-x-circle-fill"
				onClose={() => (errorMessage = "")}
			/>
		</div>
	{:else if !cart || cart.items.length === 0}
		<div
			class="mt-6 rounded-2xl border border-dashed border-gray-300 bg-white p-8 text-center text-gray-600 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-300"
		>
			<p class="text-2xl font-medium">Your cart is empty.</p>
			<div class="mt-6">
				<ButtonLink href={resolve("/")} variant="primary" size="large">Continue shopping</ButtonLink>
			</div>
		</div>
	{:else}
		<div class="mt-6 grid gap-6 lg:grid-cols-[1.6fr_0.8fr]">
			<div class="space-y-4">
				{#each cart.items as item (item.id)}
					<div
						class="flex flex-col gap-4 rounded-2xl border border-gray-200 bg-white p-4 shadow-sm sm:flex-row sm:items-center dark:border-gray-800 dark:bg-gray-900"
					>
						<div class="h-20 w-20 overflow-hidden rounded-xl bg-gray-100 dark:bg-gray-800">
							{#if item.product.images?.length}
								<img
									src={item.product.images[0]}
									alt={item.product.name}
									class="h-full w-full object-cover"
								/>
							{:else}
								<div class="flex h-full w-full items-center justify-center text-xs text-gray-500">
									No image
								</div>
							{/if}
						</div>

						<div class="flex-1">
							<h2 class="line-clamp-1 text-lg font-medium text-gray-900 dark:text-gray-100">
								{item.product.name}
							</h2>
							<p class="text-sm text-gray-500 dark:text-gray-400">
								{formatPrice(item.product.price, $userStore?.currency ?? "USD")}
							</p>
						</div>

						<div class="flex items-center gap-2">
							<button
								class="aspect-square h-8 w-8 rounded-full border border-gray-300 text-lg text-gray-600 transition hover:bg-gray-100 dark:border-gray-700 dark:text-gray-200 dark:hover:bg-gray-800"
								type="button"
								disabled={updatingItemId === item.id}
								onclick={() => decreaseQuantity(item.id, item.quantity)}
								aria-label="Decrease quantity"
							>
								<i class="bi bi-dash"></i>
							</button>
								<NumberInput
									class="w-16 rounded-full border border-gray-300 p-1 text-center dark:border-gray-700"
									full={false}
									min="1"
									value={item.quantity}
									disabled={updatingItemId === item.id}
									onchange={(event) =>
										updateItemQuantity(item.id, Number((event.target as HTMLInputElement).value))}
								/>
							<button
								class="aspect-square h-8 w-8 rounded-full border border-gray-300 text-lg text-gray-600 transition hover:bg-gray-100 dark:border-gray-700 dark:text-gray-200 dark:hover:bg-gray-800"
								type="button"
								disabled={updatingItemId === item.id}
								onclick={() => increaseQuantity(item.id, item.quantity)}
								aria-label="Increase quantity"
							>
								<i class="bi bi-plus"></i>
							</button>
							<Button
								variant="regular"
								type="button"
								disabled={updatingItemId === item.id}
								onclick={() => removeItem(item.id)}
							>
								Remove
							</Button>
						</div>
					</div>
				{/each}
			</div>

			<div
				class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
			>
				<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Order summary</h3>
				<div class="mt-4 space-y-2 text-sm text-gray-600 dark:text-gray-300">
					<div class="flex items-center justify-between">
						<span>Subtotal</span>
						<span class="font-medium text-gray-900 dark:text-gray-100">
							{formatPrice(total, $userStore?.currency ?? "USD")}
						</span>
					</div>
					<div class="flex items-center justify-between">
						<span>Shipping</span>
						<span>Calculated at checkout</span>
					</div>
				</div>
				<div class="mt-6 border-t border-gray-200 pt-4 dark:border-gray-800">
					<ButtonLink
						href={resolve("/checkout")}
						variant="primary"
						size="large"
						class="m-0! block w-full text-center"
					>
						Go to checkout
						<i class="bi bi-arrow-right"></i>
					</ButtonLink>
					<a
						href={resolve("/")}
						class="mt-3 block text-center text-sm text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
					>
						Continue shopping
					</a>
				</div>
			</div>
		</div>
	{/if}
</section>
