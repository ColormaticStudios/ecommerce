<script lang="ts">
	import { type API } from "$lib/api";
	import {
		type CartModel,
		type OrderModel,
		type SavedAddressModel,
		type SavedPaymentMethodModel,
	} from "$lib/models";
	import Alert from "$lib/components/Alert.svelte";
	import Button from "$lib/components/Button.svelte";
	import NumberInput from "$lib/components/NumberInput.svelte";
	import { formatPrice } from "$lib/utils";
	import { userStore } from "$lib/user";
	import { getContext, untrack } from "svelte";
	import { resolve } from "$app/paths";
	import { goto } from "$app/navigation";
	import type { components } from "$lib/api/generated/openapi";
	import type { PageData } from "./$types";

	const api: API = getContext("api");

	type CheckoutProvider = components["schemas"]["CheckoutPlugin"];
	type CheckoutProviderField = components["schemas"]["CheckoutPluginField"];
	type CheckoutProviderState = components["schemas"]["CheckoutPluginState"];
	type CheckoutQuoteResponse = components["schemas"]["CheckoutQuoteResponse"];

	interface Props {
		data: PageData;
	}
	let { data }: Props = $props();

	let cart = $state<CartModel | null>(null);
	let order = $state<OrderModel | null>(null);
	let errorMessage = $state("");
	let statusMessage = $state("");
	let processing = $state(false);
	let quoting = $state(false);
	let isAuthenticated = $state(false);
	let orderPlaced = $state(false);

	let paymentProviders = $state<CheckoutProvider[]>([]);
	let shippingProviders = $state<CheckoutProvider[]>([]);
	let taxProviders = $state<CheckoutProvider[]>([]);
	let savedPaymentMethods = $state<SavedPaymentMethodModel[]>([]);
	let savedAddresses = $state<SavedAddressModel[]>([]);

	let selectedPaymentProviderId = $state("");
	let selectedShippingProviderId = $state("");
	let autoTaxProviderId = $state("");

	let paymentMode = $state<"select" | "details">("select");
	let shippingMode = $state<"select" | "details">("select");

	let selectedSavedPaymentMethodId = $state("");
	let selectedSavedAddressId = $state("");
	let savePaymentMethodToProfile = $state(false);
	let saveAddressToProfile = $state(false);

	let paymentData = $state<Record<string, string>>({});
	let shippingData = $state<Record<string, string>>({});
	let taxData = $state<Record<string, string>>({});
	let quote = $state<CheckoutQuoteResponse | null>(null);

	const activePaymentProvider = $derived(findProvider("payment", selectedPaymentProviderId));
	const activeShippingProvider = $derived(findProvider("shipping", selectedShippingProviderId));
	const activeTaxProvider = $derived(findProvider("tax", autoTaxProviderId));
	const subtotal = $derived(
		cart ? cart.items.reduce((sum, item) => sum + item.quantity * item.product.price, 0) : 0
	);
	const paymentUsesCard = $derived(providerUsesCardFields(activePaymentProvider));
	const shippingUsesAddress = $derived(providerUsesAddressFields(activeShippingProvider));
	const selectedSavedPaymentMethod = $derived(
		savedPaymentMethods.find(
			(candidate) => String(candidate.id) === selectedSavedPaymentMethodId
		) ?? null
	);

	function findProvider(
		type: "payment" | "shipping" | "tax",
		providerId: string
	): CheckoutProvider | null {
		const source =
			type === "payment"
				? paymentProviders
				: type === "shipping"
					? shippingProviders
					: taxProviders;
		return source.find((provider) => provider.id === providerId) ?? null;
	}

	function initDataForProvider(
		fields: CheckoutProviderField[] | undefined,
		dataMap: Record<string, string>
	) {
		for (const field of fields ?? []) {
			if (dataMap[field.key] !== undefined) {
				continue;
			}
			if (field.type === "checkbox") {
				dataMap[field.key] = "false";
				continue;
			}
			if (field.type === "select") {
				dataMap[field.key] = field.options?.[0]?.value ?? "";
				continue;
			}
			dataMap[field.key] = "";
		}
	}

	function providerUsesCardFields(provider: CheckoutProvider | null): boolean {
		if (!provider) {
			return false;
		}
		const keys = new Set((provider.fields ?? []).map((field) => field.key));
		return keys.has("card_number") || (keys.has("exp_month") && keys.has("exp_year"));
	}

	function providerUsesAddressFields(provider: CheckoutProvider | null): boolean {
		if (!provider) {
			return false;
		}
		const keys = new Set((provider.fields ?? []).map((field) => field.key));
		return keys.has("line1") && keys.has("city") && keys.has("postal_code") && keys.has("country");
	}

	function providerLogoMark(provider: CheckoutProvider): string {
		const words = provider.name
			.split(/\s+/)
			.map((value) => value.trim())
			.filter(Boolean);
		if (words.length === 0) {
			return "PV";
		}
		if (words.length === 1) {
			return words[0].slice(0, 2).toUpperCase();
		}
		return `${words[0][0] ?? "P"}${words[1][0] ?? "V"}`.toUpperCase();
	}

	function providerLogoColor(providerID: string): string {
		switch (providerID) {
			case "dummy-card":
				return "bg-emerald-100 text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-300";
			case "dummy-wallet":
				return "bg-sky-100 text-sky-700 dark:bg-sky-950/60 dark:text-sky-300";
			case "dummy-ground":
				return "bg-orange-100 text-orange-700 dark:bg-orange-950/60 dark:text-orange-300";
			case "dummy-pickup":
				return "bg-indigo-100 text-indigo-700 dark:bg-indigo-950/60 dark:text-indigo-300";
			default:
				return "bg-gray-200 text-gray-700 dark:bg-gray-800 dark:text-gray-200";
		}
	}

	function maskedCardDisplay(last4: string | undefined): string {
		const digits = (last4 ?? "").padStart(4, "0").slice(-4);
		return `•••• •••• •••• ${digits}`;
	}

	function selectPaymentProvider(providerID: string) {
		selectedPaymentProviderId = providerID;
		selectedSavedPaymentMethodId = "";
		savePaymentMethodToProfile = false;
		const provider = findProvider("payment", providerID);
		initDataForProvider(provider?.fields, paymentData);
		paymentMode = "details";
		quote = null;
	}

	function selectShippingProvider(providerID: string) {
		selectedShippingProviderId = providerID;
		selectedSavedAddressId = "";
		saveAddressToProfile = false;
		const provider = findProvider("shipping", providerID);
		initDataForProvider(provider?.fields, shippingData);
		shippingMode = "details";
		quote = null;
	}

	function applySavedPaymentMethod(paymentMethodID: string) {
		selectedSavedPaymentMethodId = paymentMethodID;
		if (!paymentMethodID) {
			return;
		}
		const method = savedPaymentMethods.find(
			(candidate) => String(candidate.id) === paymentMethodID
		);
		if (!method) {
			return;
		}
		const last4 = method.last4.padStart(4, "0").slice(-4);
		paymentData.cardholder_name = method.cardholder_name;
		paymentData.exp_month = String(method.exp_month);
		paymentData.exp_year = String(method.exp_year);
		paymentData.card_number = `411111111111${last4}`;
	}

	function applySavedAddress(addressID: string) {
		selectedSavedAddressId = addressID;
		if (!addressID) {
			return;
		}
		const address = savedAddresses.find((candidate) => String(candidate.id) === addressID);
		if (!address) {
			return;
		}
		shippingData.full_name = address.full_name;
		shippingData.line1 = address.line1;
		shippingData.line2 = address.line2 || "";
		shippingData.city = address.city;
		shippingData.state = address.state || "";
		shippingData.postal_code = address.postal_code;
		shippingData.country = address.country;
	}

	function chooseAutomaticTaxProvider() {
		if (autoTaxProviderId || taxProviders.length === 0) {
			return;
		}
		autoTaxProviderId = taxProviders[0].id;
		initDataForProvider(findProvider("tax", autoTaxProviderId)?.fields, taxData);
	}

	function syncPaymentProviderState() {
		const selectedExists = paymentProviders.some(
			(provider) => provider.id === selectedPaymentProviderId
		);
		if (!selectedExists) {
			selectedPaymentProviderId = "";
			selectedSavedPaymentMethodId = "";
			savePaymentMethodToProfile = false;
		}

		if (paymentProviders.length === 1) {
			const onlyProvider = paymentProviders[0];
			if (selectedPaymentProviderId !== onlyProvider.id) {
				selectedPaymentProviderId = onlyProvider.id;
				selectedSavedPaymentMethodId = "";
				savePaymentMethodToProfile = false;
				quote = null;
			}
			initDataForProvider(onlyProvider.fields, paymentData);
			paymentMode = "details";
			return;
		}

		if (selectedPaymentProviderId) {
			initDataForProvider(activePaymentProvider?.fields, paymentData);
			paymentMode = "details";
			return;
		}
		paymentMode = "select";
	}

	function syncShippingProviderState() {
		const selectedExists = shippingProviders.some(
			(provider) => provider.id === selectedShippingProviderId
		);
		if (!selectedExists) {
			selectedShippingProviderId = "";
			selectedSavedAddressId = "";
			saveAddressToProfile = false;
		}

		if (shippingProviders.length === 1) {
			const onlyProvider = shippingProviders[0];
			if (selectedShippingProviderId !== onlyProvider.id) {
				selectedShippingProviderId = onlyProvider.id;
				selectedSavedAddressId = "";
				saveAddressToProfile = false;
				quote = null;
			}
			initDataForProvider(onlyProvider.fields, shippingData);
			shippingMode = "details";
			return;
		}

		if (selectedShippingProviderId) {
			initDataForProvider(activeShippingProvider?.fields, shippingData);
			shippingMode = "details";
			return;
		}
		shippingMode = "select";
	}

	async function maybeSaveProfileData() {
		if (
			paymentUsesCard &&
			savePaymentMethodToProfile &&
			!selectedSavedPaymentMethodId &&
			paymentData.card_number &&
			paymentData.cardholder_name &&
			paymentData.exp_month &&
			paymentData.exp_year
		) {
			await api.createSavedPaymentMethod({
				cardholder_name: paymentData.cardholder_name.trim(),
				card_number: paymentData.card_number.trim(),
				exp_month: Number(paymentData.exp_month),
				exp_year: Number(paymentData.exp_year),
				nickname: activePaymentProvider?.name,
				set_default: false,
			});
		}

		if (
			shippingUsesAddress &&
			saveAddressToProfile &&
			!selectedSavedAddressId &&
			shippingData.full_name &&
			shippingData.line1 &&
			shippingData.city &&
			shippingData.postal_code &&
			shippingData.country
		) {
			await api.createSavedAddress({
				label: activeShippingProvider?.name,
				full_name: shippingData.full_name.trim(),
				line1: shippingData.line1.trim(),
				line2: shippingData.line2?.trim() || undefined,
				city: shippingData.city.trim(),
				state: shippingData.state?.trim() || undefined,
				postal_code: shippingData.postal_code.trim(),
				country: shippingData.country.trim().toUpperCase(),
				phone: undefined,
				set_default: false,
			});
		}
	}

	async function refreshQuote() {
		if (!cart || cart.items.length === 0) {
			return false;
		}
		if (!selectedPaymentProviderId || !selectedShippingProviderId) {
			errorMessage = "Select a payment and shipping option before requesting an estimate.";
			return false;
		}

		quoting = true;
		errorMessage = "";
		try {
			quote = await api.quoteCheckout({
				payment_provider_id: selectedPaymentProviderId,
				shipping_provider_id: selectedShippingProviderId,
				tax_provider_id: "",
				payment_data: paymentData,
				shipping_data: shippingData,
				tax_data: taxData,
			});
			if (!quote.valid) {
				errorMessage = "Some checkout details are invalid. Review the messages below.";
				return false;
			}
			return true;
		} catch (err) {
			const error = err as { body?: { error?: string } };
			errorMessage = error.body?.error ?? "Unable to calculate quote.";
			return false;
		} finally {
			quoting = false;
		}
	}

	async function placeOrder() {
		if (!cart || cart.items.length === 0) {
			return;
		}

		processing = true;
		errorMessage = "";
		statusMessage = "";
		orderPlaced = false;

		try {
			const isQuoteValid = await refreshQuote();
			if (!isQuoteValid) {
				return;
			}

			await maybeSaveProfileData();

			const created = await api.createOrder({
				items: cart.items.map((item) => ({
					product_id: item.product_id,
					quantity: item.quantity,
				})),
			});

			order = await api.processPayment(created.id, {
				payment_provider_id: selectedPaymentProviderId,
				shipping_provider_id: selectedShippingProviderId,
				tax_provider_id: "",
				payment_data: paymentData,
				shipping_data: shippingData,
				tax_data: taxData,
			});

			statusMessage = order?.status ? `Payment status: ${order.status}` : "Payment processed.";
			orderPlaced = true;
			window.dispatchEvent(new CustomEvent("cart:updated"));
			if (typeof window !== "undefined") {
				window.sessionStorage.setItem("orders_toast", "order_placed");
			}
			await goto(resolve("/orders"));
		} catch (err) {
			const error = err as { body?: { error?: string } };
			errorMessage = error.body?.error ?? "Unable to place your order.";
		} finally {
			processing = false;
		}
	}

	function stateTone(severity: CheckoutProviderState["severity"]) {
		switch (severity) {
			case "error":
				return "border-red-200 bg-red-50 text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-300";
			case "warning":
				return "border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-300";
			case "success":
				return "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300";
			default:
				return "border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-900 dark:bg-blue-950/30 dark:text-blue-300";
		}
	}

	$effect(() => {
		isAuthenticated = data.isAuthenticated;
		cart = data.cart;
		errorMessage = data.errorMessage;
		paymentProviders = data.plugins?.payment ?? [];
		shippingProviders = data.plugins?.shipping ?? [];
		taxProviders = data.plugins?.tax ?? [];
		savedPaymentMethods = data.savedPaymentMethods ?? [];
		savedAddresses = data.savedAddresses ?? [];
		untrack(() => {
			syncPaymentProviderState();
			syncShippingProviderState();
			chooseAutomaticTaxProvider();
		});
	});
</script>

{#snippet providerChoiceButton(provider: CheckoutProvider, onSelect: () => void)}
	<button
		type="button"
		class="cursor-pointer rounded-xl border border-gray-300 bg-white p-3 text-left transition hover:border-gray-400 dark:border-gray-700 dark:bg-gray-900 dark:hover:border-gray-500"
		onclick={onSelect}
	>
		<div class="flex items-start gap-3">
			<div
				class={`inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-xs font-semibold ${providerLogoColor(provider.id)}`}
			>
				{providerLogoMark(provider)}
			</div>
			<div class="min-w-0">
				<p class="font-medium text-gray-900 dark:text-gray-100">{provider.name}</p>
				<p class="mt-1 line-clamp-2 text-xs text-gray-500 dark:text-gray-400">
					{provider.description}
				</p>
			</div>
		</div>
	</button>
{/snippet}

<section class="mx-auto max-w-6xl px-4 py-10">
	<div class="flex flex-wrap items-end justify-between gap-4">
		<div>
			<h1 class="text-3xl font-semibold text-gray-900 dark:text-gray-100">Checkout</h1>
		</div>
		{#if cart && cart.items.length}
			<p class="text-sm text-gray-600 dark:text-gray-300">
				Subtotal {formatPrice(subtotal, $userStore?.currency ?? "USD")}
			</p>
		{/if}
	</div>

	{#if !isAuthenticated}
		<p class="mt-4 text-gray-600 dark:text-gray-300">
			Please
			<a href={resolve("/login")} class="text-blue-600 hover:underline dark:text-blue-400">log in</a
			>
			to continue to checkout.
		</p>
	{:else if !cart || cart.items.length === 0}
		<p class="mt-4 text-gray-600 dark:text-gray-300">
			Your cart is empty. Visit the
			<a href={resolve("/")} class="text-blue-600 hover:underline dark:text-blue-400">store</a>
			to add items.
		</p>
	{:else}
		<div class="mt-6 space-y-6">
			<div
				class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
			>
				<div class="flex flex-wrap items-center justify-between gap-3">
					<div>
						<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Checkout options</h3>
						<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
							Choose your payment and shipping providers. Taxes are selected automatically.
						</p>
					</div>
				</div>

				<div class="mt-5 grid gap-6 lg:grid-cols-2">
					<div class="space-y-3">
						<h4 class="text-sm font-semibold text-gray-700 dark:text-gray-200">Payment provider</h4>
						{#if paymentMode === "select"}
							<div class="grid gap-3 sm:grid-cols-2">
								{#each paymentProviders as provider (provider.id)}
									{@render providerChoiceButton(provider, () => selectPaymentProvider(provider.id))}
								{/each}
							</div>
						{:else if activePaymentProvider}
							<div class="space-y-3 rounded-xl border border-gray-200 p-4 dark:border-gray-700">
								<div class="flex items-center justify-between gap-3">
									<div class="flex items-center gap-3">
										<div
											class={`inline-flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-full text-xs leading-none font-semibold ${providerLogoColor(activePaymentProvider.id)}`}
										>
											{providerLogoMark(activePaymentProvider)}
										</div>
										<div>
											<p class="font-medium text-gray-900 dark:text-gray-100">
												{activePaymentProvider.name}
											</p>
											<p class="text-xs text-gray-500 dark:text-gray-400">
												{activePaymentProvider.description}
											</p>
										</div>
									</div>
									{#if paymentProviders.length > 1}
										<Button
											variant="regular"
											size="small"
											type="button"
											onclick={() => (paymentMode = "select")}>Back</Button
										>
									{/if}
								</div>

								{#if paymentUsesCard && savedPaymentMethods.length > 0}
									<label
										class="block text-sm text-gray-700 dark:text-gray-200"
										for="saved-payment-method"
									>
										Use a saved card
									</label>
									<select
										id="saved-payment-method"
										class="w-full rounded-md border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800"
										bind:value={selectedSavedPaymentMethodId}
										onchange={(event) => applySavedPaymentMethod(event.currentTarget.value)}
									>
										<option value="">Enter card manually</option>
										{#each savedPaymentMethods as method (method.id)}
											<option value={String(method.id)}>
												{method.nickname || `${method.brand} •••• ${method.last4}`}
											</option>
										{/each}
									</select>
								{/if}

								{#each activePaymentProvider.fields ?? [] as field (field.key)}
									<label
										class="block text-sm text-gray-700 dark:text-gray-200"
										for={`payment-${field.key}`}
									>
										{field.label}{field.required ? " *" : ""}
									</label>
									{#if selectedSavedPaymentMethodId && field.key === "card_number"}
										<input
											id={`payment-${field.key}`}
											type="text"
											class="w-full rounded-md border border-gray-300 px-3 py-2 text-gray-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
											value={maskedCardDisplay(selectedSavedPaymentMethod?.last4)}
											readonly
										/>
									{:else if field.type === "select"}
										<select
											id={`payment-${field.key}`}
											class="w-full rounded-md border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800"
											bind:value={paymentData[field.key]}
										>
											{#each field.options ?? [] as option (option.value)}
												<option value={option.value}>{option.label}</option>
											{/each}
										</select>
									{:else if field.type === "checkbox"}
										<label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
											<input
												type="checkbox"
												checked={paymentData[field.key] === "true"}
												onchange={(event) => {
													paymentData[field.key] = event.currentTarget.checked ? "true" : "false";
												}}
											/>
											<span>{field.help_text || field.label}</span>
										</label>
									{:else if field.type === "number"}
										<NumberInput
											id={`payment-${field.key}`}
											class="w-full"
											bind:value={paymentData[field.key]}
										/>
									{:else}
										<input
											id={`payment-${field.key}`}
											type="text"
											class="w-full rounded-md border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800"
											bind:value={paymentData[field.key]}
										/>
									{/if}
									{#if field.help_text}
										<p class="text-xs text-gray-500 dark:text-gray-400">{field.help_text}</p>
									{/if}
								{/each}

								{#if paymentUsesCard && !selectedSavedPaymentMethodId}
									<label
										class="mt-1 flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200"
									>
										<input type="checkbox" bind:checked={savePaymentMethodToProfile} />
										<span>Save this card to my profile</span>
									</label>
								{/if}
							</div>
						{/if}
					</div>

					<div class="space-y-3">
						<h4 class="text-sm font-semibold text-gray-700 dark:text-gray-200">
							Shipping provider
						</h4>
						{#if shippingMode === "select"}
							<div class="grid gap-3 sm:grid-cols-2">
								{#each shippingProviders as provider (provider.id)}
									{@render providerChoiceButton(provider, () =>
										selectShippingProvider(provider.id)
									)}
								{/each}
							</div>
						{:else if activeShippingProvider}
							<div class="space-y-3 rounded-xl border border-gray-200 p-4 dark:border-gray-700">
								<div class="flex items-center justify-between gap-3">
									<div class="flex items-center gap-3">
										<div
											class={`inline-flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-full text-xs leading-none font-semibold ${providerLogoColor(activeShippingProvider.id)}`}
										>
											{providerLogoMark(activeShippingProvider)}
										</div>
										<div>
											<p class="font-medium text-gray-900 dark:text-gray-100">
												{activeShippingProvider.name}
											</p>
											<p class="text-xs text-gray-500 dark:text-gray-400">
												{activeShippingProvider.description}
											</p>
										</div>
									</div>
									{#if shippingProviders.length > 1}
										<Button
											variant="regular"
											size="small"
											type="button"
											onclick={() => (shippingMode = "select")}>Back</Button
										>
									{/if}
								</div>

								{#if shippingUsesAddress && savedAddresses.length > 0}
									<label class="block text-sm text-gray-700 dark:text-gray-200" for="saved-address">
										Use a saved address
									</label>
									<select
										id="saved-address"
										class="w-full rounded-md border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800"
										bind:value={selectedSavedAddressId}
										onchange={(event) => applySavedAddress(event.currentTarget.value)}
									>
										<option value="">Enter address manually</option>
										{#each savedAddresses as address (address.id)}
											<option value={String(address.id)}>
												{address.label || `${address.full_name} - ${address.line1}`}
											</option>
										{/each}
									</select>
								{/if}

								{#each activeShippingProvider.fields ?? [] as field (field.key)}
									<label
										class="block text-sm text-gray-700 dark:text-gray-200"
										for={`shipping-${field.key}`}
									>
										{field.label}{field.required ? " *" : ""}
									</label>
									{#if field.type === "select"}
										<select
											id={`shipping-${field.key}`}
											class="w-full rounded-md border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800"
											bind:value={shippingData[field.key]}
										>
											{#each field.options ?? [] as option (option.value)}
												<option value={option.value}>{option.label}</option>
											{/each}
										</select>
									{:else if field.type === "checkbox"}
										<label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
											<input
												type="checkbox"
												checked={shippingData[field.key] === "true"}
												onchange={(event) => {
													shippingData[field.key] = event.currentTarget.checked ? "true" : "false";
												}}
											/>
											<span>{field.help_text || field.label}</span>
										</label>
									{:else if field.type === "number"}
										<NumberInput
											id={`shipping-${field.key}`}
											class="w-full"
											bind:value={shippingData[field.key]}
										/>
									{:else}
										<input
											id={`shipping-${field.key}`}
											type="text"
											class="w-full rounded-md border border-gray-300 px-3 py-2 dark:border-gray-700 dark:bg-gray-800"
											bind:value={shippingData[field.key]}
										/>
									{/if}
									{#if field.help_text}
										<p class="text-xs text-gray-500 dark:text-gray-400">{field.help_text}</p>
									{/if}
								{/each}

								{#if shippingUsesAddress && !selectedSavedAddressId}
									<label
										class="mt-1 flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200"
									>
										<input type="checkbox" bind:checked={saveAddressToProfile} />
										<span>Save this address to my profile</span>
									</label>
								{/if}
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
					<div class="flex items-center justify-between gap-3">
						<h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Order summary</h3>
						<Button
							variant="regular"
							size="small"
							type="button"
							disabled={quoting}
							onclick={refreshQuote}
						>
							{quoting ? "Updating quote..." : "Refresh"}
						</Button>
					</div>
					<div class="mt-4 space-y-2 text-sm text-gray-600 dark:text-gray-300">
						<div class="flex items-center justify-between">
							<span>Subtotal</span>
							<span class="font-medium text-gray-900 dark:text-gray-100">
								{formatPrice(quote?.subtotal ?? subtotal, $userStore?.currency ?? "USD")}
							</span>
						</div>
						<div class="flex items-center justify-between">
							<span>Shipping</span>
							<span class="font-medium text-gray-900 dark:text-gray-100">
								{formatPrice(quote?.shipping ?? 0, $userStore?.currency ?? "USD")}
							</span>
						</div>
						<div class="flex items-center justify-between">
							<span>Tax</span>
							<span class="font-medium text-gray-900 dark:text-gray-100">
								{formatPrice(quote?.tax ?? 0, $userStore?.currency ?? "USD")}
							</span>
						</div>
						<div
							class="mt-2 flex items-center justify-between border-t border-gray-200 pt-2 dark:border-gray-700"
						>
							<span class="font-semibold text-gray-900 dark:text-gray-100">Estimated total</span>
							<span class="font-semibold text-gray-900 dark:text-gray-100">
								{formatPrice(quote?.total ?? subtotal, $userStore?.currency ?? "USD")}
							</span>
						</div>
					</div>

					<div class="mt-4 space-y-2">
						{#each quote?.payment_states ?? [] as state (state.code + state.message)}
							<p class={`rounded-md border px-3 py-2 text-xs ${stateTone(state.severity)}`}>
								{state.message}
							</p>
						{/each}
						{#each quote?.shipping_states ?? [] as state (state.code + state.message)}
							<p class={`rounded-md border px-3 py-2 text-xs ${stateTone(state.severity)}`}>
								{state.message}
							</p>
						{/each}
						{#each quote?.tax_states ?? [] as state (state.code + state.message)}
							<p class={`rounded-md border px-3 py-2 text-xs ${stateTone(state.severity)}`}>
								{state.message}
							</p>
						{/each}
					</div>

					{#if activeTaxProvider}
						<p class="mt-3 text-xs text-gray-500 dark:text-gray-400">
							Taxes are automatically calculated with {activeTaxProvider.name}.
						</p>
					{/if}

					<div class="mt-4">
						<Button
							variant="primary"
							size="large"
							class="w-full"
							type="button"
							disabled={processing || quoting || orderPlaced}
							onclick={placeOrder}
						>
							{processing ? "Processing..." : orderPlaced ? "Order placed" : "Place order"}
						</Button>
					</div>

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
