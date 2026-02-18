<script lang="ts">
	import { type API } from "$lib/api";
	import { type CartModel, type OrderModel, type SavedAddressModel, type SavedPaymentMethodModel } from "$lib/models";
	import Alert from "$lib/components/alert.svelte";
	import Button from "$lib/components/Button.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
	import NumberInput from "$lib/components/NumberInput.svelte";
	import { formatPrice } from "$lib/utils";
	import { userStore } from "$lib/user";
	import { getContext, onMount } from "svelte";
	import { resolve } from "$app/paths";
	import { goto } from "$app/navigation";

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

	let savedPaymentMethods = $state<SavedPaymentMethodModel[]>([]);
	let savedAddresses = $state<SavedAddressModel[]>([]);
	let useNewPaymentMethod = $state(false);
	let useNewAddress = $state(false);
	let selectedPaymentMethodId = $state<number | null>(null);
	let selectedAddressId = $state<number | null>(null);

	let cardholderName = $state("");
	let cardNumber = $state("");
	let expMonth = $state("");
	let expYear = $state("");
	let paymentNickname = $state("");
	let savePaymentMethod = $state(false);

	let addressLabel = $state("");
	let fullName = $state("");
	let line1 = $state("");
	let line2 = $state("");
	let city = $state("");
	let region = $state("");
	let postalCode = $state("");
	let country = $state("US");
	let phone = $state("");
	let saveAddress = $state(false);

	const total = $derived(
		cart ? cart.items.reduce((sum, item) => sum + item.quantity * item.product.price, 0) : 0
	);

	async function loadCheckoutData() {
		const [cartResponse, paymentMethodsResponse, addressesResponse] = await Promise.all([
			api.viewCart(),
			api.listSavedPaymentMethods(),
			api.listSavedAddresses(),
		]);

		cart = cartResponse;
		savedPaymentMethods = paymentMethodsResponse;
		savedAddresses = addressesResponse;

		const defaultPayment = savedPaymentMethods.find((method) => method.is_default) ?? savedPaymentMethods[0];
		selectedPaymentMethodId = defaultPayment?.id ?? null;
		useNewPaymentMethod = !defaultPayment;

		const defaultAddress = savedAddresses.find((address) => address.is_default) ?? savedAddresses[0];
		selectedAddressId = defaultAddress?.id ?? null;
		useNewAddress = !defaultAddress;
	}

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
			await loadCheckoutData();
		} catch (err) {
			console.error(err);
			errorMessage = "Unable to load your checkout data.";
		} finally {
			loading = false;
		}
	}

	function validateCheckoutInput() {
		if (useNewPaymentMethod) {
			if (!cardholderName.trim() || !cardNumber.trim() || !expMonth.trim() || !expYear.trim()) {
				return "Please complete the payment method fields.";
			}
		}
		if (!useNewPaymentMethod && !selectedPaymentMethodId) {
			return "Please select a saved payment method or enter a new one.";
		}
		if (useNewAddress) {
			if (!fullName.trim() || !line1.trim() || !city.trim() || !postalCode.trim() || !country.trim()) {
				return "Please complete the shipping address fields.";
			}
		}
		if (!useNewAddress && !selectedAddressId) {
			return "Please select a saved address or enter a new one.";
		}
		return "";
	}

	async function placeOrder() {
		if (!cart || cart.items.length === 0) {
			return;
		}

		const validationError = validateCheckoutInput();
		if (validationError) {
			errorMessage = validationError;
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

			const paymentPayload: {
				payment_method_id?: number;
				address_id?: number;
				payment_method?: {
					cardholder_name: string;
					card_number: string;
					exp_month: number;
					exp_year: number;
				};
				address?: {
					full_name: string;
					line1: string;
					line2?: string;
					city: string;
					state?: string;
					postal_code: string;
					country: string;
				};
			} = {};

			if (useNewPaymentMethod) {
				const month = Number(expMonth);
				const year = Number(expYear);
				if (savePaymentMethod) {
					const saved = await api.createSavedPaymentMethod({
						cardholder_name: cardholderName.trim(),
						card_number: cardNumber,
						exp_month: month,
						exp_year: year,
						nickname: paymentNickname.trim() || undefined,
						set_default: savedPaymentMethods.length === 0,
					});
					paymentPayload.payment_method_id = saved.id;
				} else {
					paymentPayload.payment_method = {
						cardholder_name: cardholderName.trim(),
						card_number: cardNumber,
						exp_month: month,
						exp_year: year,
					};
				}
			} else if (selectedPaymentMethodId) {
				paymentPayload.payment_method_id = selectedPaymentMethodId;
			}

			if (useNewAddress) {
				if (saveAddress) {
					const saved = await api.createSavedAddress({
						label: addressLabel.trim() || undefined,
						full_name: fullName.trim(),
						line1: line1.trim(),
						line2: line2.trim() || undefined,
						city: city.trim(),
						state: region.trim() || undefined,
						postal_code: postalCode.trim(),
						country: country.trim().toUpperCase(),
						phone: phone.trim() || undefined,
						set_default: savedAddresses.length === 0,
					});
					paymentPayload.address_id = saved.id;
				} else {
					paymentPayload.address = {
						full_name: fullName.trim(),
						line1: line1.trim(),
						line2: line2.trim() || undefined,
						city: city.trim(),
						state: region.trim() || undefined,
						postal_code: postalCode.trim(),
						country: country.trim().toUpperCase(),
					};
				}
			} else if (selectedAddressId) {
				paymentPayload.address_id = selectedAddressId;
			}

			order = await api.processPayment(created.id, paymentPayload);
			if (order?.status) {
				statusMessage = `Payment status: ${order.status}`;
			} else {
				statusMessage = "Payment processed.";
			}
			orderPlaced = true;
			window.dispatchEvent(new CustomEvent("cart:updated"));
			await goto(resolve("/orders"));
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
				errorMessage = error.body?.error ?? "Unable to place your order.";
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
	{:else if !cart || cart.items.length === 0}
		<p class="mt-4 text-gray-600 dark:text-gray-300">
			Your cart is empty. Visit the
			<a href={resolve("/")} class="text-blue-600 hover:underline dark:text-blue-400"> store </a>
			to add items.
		</p>
	{:else}
		<div class="mt-6 space-y-6">
			<div
				class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
			>
				<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
					Payment and shipping
				</h3>
				<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
					Mock checkout: no real provider integration yet.
				</p>

				<div class="mt-5 grid gap-6 lg:grid-cols-2">
					<div class="space-y-3">
						<h4 class="text-sm font-semibold text-gray-700 dark:text-gray-200">Payment method</h4>
						<label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
							<input type="checkbox" bind:checked={useNewPaymentMethod} />
							Use a new payment method
						</label>
						{#if !useNewPaymentMethod}
							<select
								class="w-full rounded-md border border-gray-300 bg-gray-200 px-3 py-2 dark:border-gray-700 dark:bg-gray-800"
								bind:value={selectedPaymentMethodId}
							>
								<option value={null}>Select saved method</option>
								{#each savedPaymentMethods as method (method.id)}
									<option value={method.id}>
										{method.nickname || `${method.brand} •••• ${method.last4}`}
										{method.is_default ? " (Default)" : ""}
									</option>
								{/each}
							</select>
						{:else}
							<div class="grid gap-3">
								<TextInput bind:value={cardholderName} placeholder="Cardholder name" />
								<TextInput bind:value={cardNumber} placeholder="Card number" />
								<div class="grid grid-cols-2 gap-3">
									<NumberInput bind:value={expMonth} placeholder="Exp month" min={1} max={12} />
									<NumberInput bind:value={expYear} placeholder="Exp year" min={2024} max={2200} />
								</div>
								<label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
									<input type="checkbox" bind:checked={savePaymentMethod} />
									Save this payment method
								</label>
								{#if savePaymentMethod}
									<TextInput bind:value={paymentNickname} placeholder="Nickname (optional)" />
								{/if}
							</div>
						{/if}
					</div>

					<div class="space-y-3 border-t border-gray-200 pt-5 lg:border-t-0 lg:border-l lg:pt-0 lg:pl-6 dark:border-gray-800">
						<h4 class="text-sm font-semibold text-gray-700 dark:text-gray-200">Shipping address</h4>
						<label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
							<input type="checkbox" bind:checked={useNewAddress} />
							Use a new shipping address
						</label>
						{#if !useNewAddress}
							<select
								class="w-full rounded-md border border-gray-300 bg-gray-200 px-3 py-2 dark:border-gray-700 dark:bg-gray-800"
								bind:value={selectedAddressId}
							>
								<option value={null}>Select saved address</option>
								{#each savedAddresses as address (address.id)}
									<option value={address.id}>
										{address.label || address.line1}, {address.city}
										{address.is_default ? " (Default)" : ""}
									</option>
								{/each}
							</select>
						{:else}
							<div class="grid gap-3">
								<TextInput bind:value={addressLabel} placeholder="Label (optional, e.g. Home)" />
								<TextInput bind:value={fullName} placeholder="Full name" />
								<TextInput bind:value={line1} placeholder="Address line 1" />
								<TextInput bind:value={line2} placeholder="Address line 2 (optional)" />
								<div class="grid grid-cols-2 gap-3">
									<TextInput bind:value={city} placeholder="City" />
									<TextInput bind:value={region} placeholder="State / Province" />
								</div>
								<div class="grid grid-cols-2 gap-3">
									<TextInput bind:value={postalCode} placeholder="Postal code" />
									<TextInput bind:value={country} maxlength={2} placeholder="Country (US)" />
								</div>
								<TextInput bind:value={phone} placeholder="Phone (optional)" />
								<label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
									<input type="checkbox" bind:checked={saveAddress} />
									Save this address
								</label>
							</div>
						{/if}
					</div>
				</div>
			</div>

			<div class="grid items-start gap-6 lg:grid-cols-[1.6fr_0.8fr]">
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
					<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Order summary</h3>
					<div class="mt-4 space-y-2 text-sm text-gray-600 dark:text-gray-300">
						<div class="flex items-center justify-between">
							<span>Subtotal</span>
							<span class="font-medium text-gray-900 dark:text-gray-100">
								{formatPrice(total, $userStore?.currency ?? "USD")}
							</span>
						</div>
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

					{#if errorMessage}
						<div class="mt-4">
							<Alert
								message={errorMessage}
								tone="error"
								icon="bi-x-circle-fill"
								onClose={() => (errorMessage = "")}
							/>
						</div>
					{/if}

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
		</div>
	{/if}
</section>
