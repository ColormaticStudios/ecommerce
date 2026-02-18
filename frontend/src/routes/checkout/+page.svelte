<script lang="ts">
	import { type API } from "$lib/api";
	import { type CartModel, type OrderModel } from "$lib/models";
	import Alert from "$lib/components/alert.svelte";
	import Button from "$lib/components/Button.svelte";
	import { formatPrice } from "$lib/utils";
	import { userStore } from "$lib/user";
	import { getContext, onMount } from "svelte";
	import { resolve } from "$app/paths";

	const api: API = getContext("api");

	let cart = $state<CartModel | null>(null);
	let order = $state<OrderModel | null>(null);
	let loading = $state(true);
	let errorMessage = $state("");
	let statusMessage = $state("");
	let processing = $state(false);
	let authChecked = $state(false);
	let stockWarning = $state<{ product_id: number; message: string } | null>(null);
	let orderPlaced = $state(false);
	const skeletonRows = [0, 1, 2];

	const total = $derived(
		cart ? cart.items.reduce((sum, item) => sum + item.quantity * item.product.price, 0) : 0
	);

	async function loadCart() {
		api.tokenFromCookie();
		authChecked = true;
		if (!api.isAuthenticated()) {
			loading = false;
			return;
		}

		loading = true;
		errorMessage = "";
		try {
			cart = await api.viewCart();
		} catch (err) {
			console.error(err);
			errorMessage = "Unable to load your cart.";
		}
		loading = false;
	}

	async function placeOrder() {
		if (!cart || cart.items.length === 0) {
			return;
		}

		processing = true;
		errorMessage = "";
		statusMessage = "";
		stockWarning = null;
		orderPlaced = false;

		try {
			const created = await api.createOrder({
				items: cart.items.map((item) => ({
					product_id: item.product_id,
					quantity: item.quantity,
				})),
			});
			order = await api.processPayment(created.id);
			// Keep cart contents intact in the UI so users can review what they just purchased.
			if (order?.status) {
				statusMessage = `Payment status: ${order.status}`;
			} else {
				statusMessage = "Payment processed.";
			}
			orderPlaced = true;
			window.dispatchEvent(new CustomEvent("cart:updated"));
		} catch (err) {
			console.error(err);
			const error = err as {
				status?: number;
				body?: {
					error?: string;
					product_id?: number;
					product_name?: string;
					available?: number;
					requested?: number;
				};
			};
			if (error?.status === 400 && error.body?.error === "Insufficient stock") {
				const productName = error.body.product_name || "This item";
				const available = error.body.available ?? 0;
				const requested = error.body.requested ?? 0;
				stockWarning = {
					product_id: error.body.product_id ?? 0,
					message: `${productName} only has ${available} available. You requested ${requested}.`,
				};
			} else {
				errorMessage = "Unable to place your order.";
			}
		} finally {
			processing = false;
		}
	}

	onMount(loadCart);
</script>

<section class="mx-auto max-w-6xl px-4 py-10">
	<div class="flex flex-wrap items-end justify-between gap-4">
		<div>
			<h1 class="text-3xl font-semibold text-gray-900 dark:text-gray-100">Checkout</h1>
		</div>
		{#if cart && cart.items.length}
			<p class="text-sm text-gray-600 dark:text-gray-300">
				Total {formatPrice(total, $userStore?.currency ?? "USD")}
			</p>
		{/if}
	</div>

	{#if !authChecked}
		<div class="mt-6 grid gap-6 lg:grid-cols-[1.6fr_0.8fr]">
			<div class="space-y-4">
				{#each skeletonRows as index (index)}
					<div
						class="flex items-center justify-between rounded-2xl border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-800 dark:bg-gray-900"
					>
						<div class="h-4 w-1/2 animate-pulse rounded bg-gray-200 dark:bg-gray-800"></div>
						<div class="h-4 w-16 animate-pulse rounded bg-gray-200 dark:bg-gray-800"></div>
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
			to continue to checkout.
		</p>
	{:else if loading}
		<div class="mt-6 grid gap-6 lg:grid-cols-[1.6fr_0.8fr]">
			<div class="space-y-4">
				{#each skeletonRows as index (index)}
					<div
						class="flex items-center justify-between rounded-2xl border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-800 dark:bg-gray-900"
					>
						<div class="h-4 w-1/2 animate-pulse rounded bg-gray-200 dark:bg-gray-800"></div>
						<div class="h-4 w-16 animate-pulse rounded bg-gray-200 dark:bg-gray-800"></div>
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
		<p class="mt-4 text-gray-600 dark:text-gray-300">
			Your cart is empty. Visit the
			<a href={resolve("/")} class="text-blue-600 hover:underline dark:text-blue-400"> store </a>
			to add items.
		</p>
		{:else}
			<div class="mt-6 grid items-start gap-6 lg:grid-cols-[1.6fr_0.8fr]">
			<div class="space-y-4">
				{#each cart.items as item (item.id)}
					<div
						class="flex items-center justify-between rounded-2xl border border-gray-200 bg-white px-4 py-4 text-sm shadow-sm dark:border-gray-800 dark:bg-gray-900"
					>
						<div>
							<p class="font-medium text-gray-900 dark:text-gray-100">{item.product.name}</p>
							<p class="text-gray-600 dark:text-gray-400">
								Qty {item.quantity} ·
								{formatPrice(item.product.price, $userStore?.currency ?? "USD")}
							</p>
							{#if stockWarning && stockWarning.product_id === item.product_id}
								<p class="mt-2 text-sm font-medium text-amber-600 dark:text-amber-300">
									<i class="bi bi-exclamation-triangle-fill mr-1"></i>
									{stockWarning.message}
								</p>
							{/if}
						</div>
						<p class="font-medium text-gray-900 dark:text-gray-100">
							{formatPrice(item.product.price * item.quantity, $userStore?.currency ?? "USD")}
						</p>
					</div>
				{/each}
			</div>

			<div
				class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
			>
				<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Payment</h3>
				<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
					This is a mock checkout. Payments are processed instantly.
				</p>
				<div class="mt-6 space-y-2 text-sm text-gray-600 dark:text-gray-300">
					<div class="flex items-center justify-between">
						<span>Subtotal</span>
						<span class="font-medium text-gray-900 dark:text-gray-100">
							{formatPrice(total, $userStore?.currency ?? "USD")}
						</span>
					</div>
					<!-- TODO: Tax and shipping -->
				</div>
				<Button
					variant="primary"
					size="large"
					class="mt-6! w-full"
					type="button"
					disabled={processing || orderPlaced}
					onclick={placeOrder}
				>
					<i class="bi bi-cart-check-fill mr-1"></i>
					{processing ? "Processing..." : orderPlaced ? "Order placed" : "Place order"}
				</Button>

				{#if statusMessage}
					<div class="mt-4">
						<Alert
							message={statusMessage}
							tone="success"
							icon="bi-check-circle-fill"
							onClose={() => (statusMessage = "")}
						/>
					</div>
				{/if}
				{#if order}
					<p class="mt-2 text-sm text-gray-600 dark:text-gray-300">
						Order {order.id ? `#${order.id}` : ""}
						{order.status ? `· ${order.status}` : ""}
					</p>
				{/if}
			</div>
		</div>
	{/if}
</section>
