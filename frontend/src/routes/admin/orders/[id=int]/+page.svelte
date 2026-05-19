<script lang="ts">
	import { getContext, untrack } from "svelte";
	import { resolve } from "$app/paths";
	import type { API } from "$lib/api";
	import type { components } from "$lib/api/generated/openapi";
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminFloatingNotices from "$lib/admin/AdminFloatingNotices.svelte";
	import AdminMetaText from "$lib/admin/AdminMetaText.svelte";
	import AdminPageHeader from "$lib/admin/AdminPageHeader.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import AdminResourceActions from "$lib/admin/AdminResourceActions.svelte";
	import AdminSurface from "$lib/admin/AdminSurface.svelte";
	import { createAdminNotices, formatAdminDateTime } from "$lib/admin/state.svelte";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import ButtonLink from "$lib/components/ButtonLink.svelte";
	import { formatOrderStatusLabel, getOrderStatusTone } from "$lib/components/order-status";
	import type { OrderModel } from "$lib/models";
	import { formatPrice } from "$lib/utils";
	import type { PageData } from "./$types";

	type OrderPaymentLedger = components["schemas"]["OrderPaymentLedger"];
	type PaymentIntent = components["schemas"]["PaymentIntentRecord"];
	type PaymentTransaction = components["schemas"]["PaymentTransactionRecord"];

	interface Props {
		data: PageData;
	}

	let { data }: Props = $props();
	const initialData = untrack(() => $state.snapshot(data));
	const api: API = getContext("api");
	const notices = createAdminNotices();

	let order = $state<OrderModel | null>(initialData.order);
	let payments = $state<OrderPaymentLedger | null>(initialData.payments);
	let errorMessage = $state(initialData.errorMessage);
	let paymentErrorMessage = $state(initialData.paymentErrorMessage);
	let updatingStatus = $state<OrderModel["status"] | null>(null);
	let paymentActionKey = $state("");

	const statusOptions: OrderModel["status"][] = [
		"PENDING",
		"PAID",
		"FAILED",
		"SHIPPED",
		"DELIVERED",
		"CANCELLED",
		"REFUNDED",
	];

	const itemCount = $derived(order?.items.reduce((total, item) => total + item.quantity, 0) ?? 0);
	const customerLabel = $derived.by(() => {
		if (!order) {
			return "Unknown customer";
		}
		if (order.user_id != null) {
			return `Customer #${order.user_id}`;
		}
		return order.guest_email ? `Guest (${order.guest_email})` : "Guest checkout";
	});

	function formatDate(value: Date | string | null | undefined) {
		if (!value) {
			return "Not yet";
		}
		const date = value instanceof Date ? value : new Date(value);
		if (Number.isNaN(date.getTime())) {
			return "Not yet";
		}
		return formatAdminDateTime(date);
	}

	function formatLabel(value: string) {
		return value
			.toLowerCase()
			.split("_")
			.map((part) => part.charAt(0).toUpperCase() + part.slice(1))
			.join(" ");
	}

	function paymentStatusTone(status: PaymentIntent["status"] | PaymentTransaction["status"]) {
		switch (status) {
			case "CAPTURED":
			case "SUCCEEDED":
				return "success" as const;
			case "AUTHORIZED":
			case "PARTIALLY_CAPTURED":
			case "PENDING":
			case "REQUIRES_ACTION":
				return "warning" as const;
			case "FAILED":
				return "danger" as const;
			default:
				return "neutral" as const;
		}
	}

	async function refreshPayments(orderId: number) {
		try {
			payments = await api.getAdminOrderPayments(orderId);
			paymentErrorMessage = "";
		} catch (error) {
			console.error(error);
			paymentErrorMessage = "Unable to refresh payment activity.";
		}
	}

	async function updateStatus(status: OrderModel["status"]) {
		if (!order || updatingStatus) {
			return;
		}

		updatingStatus = status;
		notices.clear();
		try {
			order = await api.updateOrderStatus(order.id, { status });
			notices.pushSuccess("Order status updated.");
		} catch (error) {
			console.error(error);
			const err = error as { body?: { error?: string } };
			notices.pushError(err.body?.error ?? "Unable to update order.");
		} finally {
			updatingStatus = null;
		}
	}

	async function runPaymentAction(
		intent: PaymentIntent,
		action: "capture" | "void" | "refund",
		amount?: number
	) {
		if (!order || paymentActionKey) {
			return;
		}

		paymentActionKey = `${action}-${intent.id}`;
		notices.clear();
		try {
			const payload = amount == null ? {} : { amount };
			const response =
				action === "capture"
					? await api.captureAdminOrderPayment(order.id, intent.id, payload)
					: action === "void"
						? await api.voidAdminOrderPayment(order.id, intent.id)
						: await api.refundAdminOrderPayment(order.id, intent.id, payload);
			order = response.order;
			notices.pushSuccess(response.message);
			await refreshPayments(order.id);
		} catch (error) {
			console.error(error);
			const err = error as { body?: { error?: string } };
			notices.pushError(err.body?.error ?? "Unable to update payment.");
		} finally {
			paymentActionKey = "";
		}
	}

	$effect(() => {
		order = data.order;
		payments = data.payments;
		errorMessage = data.errorMessage;
		paymentErrorMessage = data.paymentErrorMessage;
	});
</script>

{#snippet headerActions()}
	<ButtonLink
		href={resolve("/admin/orders")}
		tone="admin"
		variant="regular"
		class="inline-flex items-center gap-2"
	>
		<i class="bi bi-arrow-left"></i>
		Orders
	</ButtonLink>
	{#if order}
		<AdminResourceActions countLabel={`Order #${order.id}`} />
	{/if}
{/snippet}

<section class="space-y-6">
	<AdminPageHeader title="Order Details" actions={headerActions} />

	{#if !order}
		<AdminPanel>
			<AdminEmptyState tone={errorMessage ? "error" : "default"}>
				{errorMessage || "You need admin access to view this order."}
			</AdminEmptyState>
		</AdminPanel>
	{:else}
		<div class="grid gap-6 xl:grid-cols-[minmax(0,1.45fr)_minmax(320px,0.75fr)]">
			<div class="space-y-6">
				<AdminPanel>
					<div class="flex flex-wrap items-start justify-between gap-5">
						<div class="space-y-2">
							<AdminMetaText>Placed {formatDate(order.created_at)}</AdminMetaText>
							<h2 class="text-2xl font-semibold text-stone-950 dark:text-stone-50">
								Order #{order.id}
							</h2>
							<div class="flex flex-wrap items-center gap-2">
								<Badge tone={getOrderStatusTone(order.status)} size="md">
									{formatOrderStatusLabel(order.status)}
								</Badge>
								<AdminMetaText>{customerLabel}</AdminMetaText>
							</div>
						</div>
						<div class="text-left sm:text-right">
							<p class="text-sm text-stone-500 dark:text-stone-400">Total</p>
							<p class="mt-1 text-3xl font-semibold text-stone-950 dark:text-stone-50">
								{formatPrice(order.total)}
							</p>
						</div>
					</div>

					<div class="mt-6 grid gap-3 md:grid-cols-3">
						<AdminSurface variant="muted" as="div">
							<p class="text-sm text-stone-500 dark:text-stone-400">Items</p>
							<p class="mt-1 font-semibold text-stone-950 dark:text-stone-50">
								{itemCount} item{itemCount === 1 ? "" : "s"}
							</p>
						</AdminSurface>
						<AdminSurface variant="muted" as="div">
							<p class="text-sm text-stone-500 dark:text-stone-400">Payment</p>
							<p class="mt-1 font-semibold text-stone-950 dark:text-stone-50">
								{order.payment_method_display || "No payment method recorded"}
							</p>
						</AdminSurface>
						<AdminSurface variant="muted" as="div">
							<p class="text-sm text-stone-500 dark:text-stone-400">Last updated</p>
							<p class="mt-1 font-semibold text-stone-950 dark:text-stone-50">
								{formatDate(order.updated_at)}
							</p>
						</AdminSurface>
					</div>
				</AdminPanel>

				<AdminPanel title="Items" meta={`${order.items.length} line items`}>
					{#if order.items.length === 0}
						<AdminEmptyState>No item details are available for this order.</AdminEmptyState>
					{:else}
						<ul class="space-y-3">
							{#each order.items as item (item.id)}
								<li
									class="flex flex-wrap items-center justify-between gap-4 rounded-2xl border border-stone-200 bg-white p-4 dark:border-stone-800 dark:bg-stone-950"
								>
									<div class="flex min-w-0 items-center gap-3">
										<a
											href={resolve(`/admin/product/${item.product.id}`)}
											class="flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-lg border border-stone-200 bg-stone-100 text-[10px] text-stone-500 dark:border-stone-800 dark:bg-stone-900"
										>
											{#if item.product.cover_image}
												<img
													src={item.product.cover_image}
													alt={item.product.name}
													class="h-full w-full object-cover"
													loading="lazy"
												/>
											{:else}
												No image
											{/if}
										</a>
										<div class="min-w-0">
											<a
												href={resolve(`/admin/product/${item.product.id}`)}
												class="block truncate font-semibold text-stone-950 hover:underline dark:text-stone-50"
											>
												{item.product.name}
											</a>
											<p class="mt-1 text-sm text-stone-600 dark:text-stone-300">
												{item.variant_title} · {item.variant_sku}
											</p>
										</div>
									</div>
									<div class="text-left sm:text-right">
										<p class="text-sm text-stone-500 dark:text-stone-400">
											{item.quantity} x {formatPrice(item.price)}
										</p>
										<p class="mt-1 font-semibold text-stone-950 dark:text-stone-50">
											{formatPrice(item.price * item.quantity)}
										</p>
									</div>
								</li>
							{/each}
						</ul>
					{/if}
				</AdminPanel>

				<AdminPanel title="Payment Activity" meta={`${payments?.intents.length ?? 0} intents`}>
					{#if paymentErrorMessage}
						<AdminEmptyState tone="error">{paymentErrorMessage}</AdminEmptyState>
					{:else if !payments || payments.intents.length === 0}
						<AdminEmptyState>No payment activity has been recorded.</AdminEmptyState>
					{:else}
						<div class="space-y-4">
							{#each payments.intents as intent (intent.id)}
								<AdminSurface variant="muted" as="article">
									<div class="flex flex-wrap items-start justify-between gap-3">
										<div>
											<p class="font-semibold text-stone-950 dark:text-stone-50">
												{intent.provider} intent #{intent.id}
											</p>
											<p class="mt-1 text-sm text-stone-600 dark:text-stone-300">
												Authorized {formatPrice(intent.authorized_amount, intent.currency)} · Captured
												{formatPrice(intent.captured_amount, intent.currency)} · Refundable {formatPrice(
													intent.refundable_amount,
													intent.currency
												)}
											</p>
										</div>
										<Badge tone={paymentStatusTone(intent.status)}>
											{formatLabel(intent.status)}
										</Badge>
									</div>
									<div class="mt-4 flex flex-wrap gap-2">
										{#if intent.status === "AUTHORIZED" || intent.status === "PARTIALLY_CAPTURED"}
											<Button
												type="button"
												tone="admin"
												size="small"
												disabled={Boolean(paymentActionKey)}
												onclick={() => runPaymentAction(intent, "capture")}
											>
												{paymentActionKey === `capture-${intent.id}` ? "Capturing..." : "Capture"}
											</Button>
										{/if}
										{#if intent.status === "AUTHORIZED" || intent.status === "REQUIRES_ACTION"}
											<Button
												type="button"
												tone="admin"
												size="small"
												disabled={Boolean(paymentActionKey)}
												onclick={() => runPaymentAction(intent, "void")}
											>
												{paymentActionKey === `void-${intent.id}` ? "Voiding..." : "Void"}
											</Button>
										{/if}
										{#if intent.refundable_amount > 0}
											<Button
												type="button"
												tone="admin"
												size="small"
												disabled={Boolean(paymentActionKey)}
												onclick={() => runPaymentAction(intent, "refund")}
											>
												{paymentActionKey === `refund-${intent.id}` ? "Refunding..." : "Refund"}
											</Button>
										{/if}
									</div>
									{#if intent.transactions.length > 0}
										<ol class="mt-4 space-y-2">
											{#each intent.transactions as transaction (transaction.id)}
												<li
													class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-stone-200 bg-white p-3 text-sm dark:border-stone-800 dark:bg-stone-950"
												>
													<span class="font-medium text-stone-950 dark:text-stone-50">
														{formatLabel(transaction.operation)} ·
														{formatPrice(transaction.amount, intent.currency)}
													</span>
													<span
														class="flex flex-wrap items-center gap-2 text-stone-500 dark:text-stone-400"
													>
														<Badge tone={paymentStatusTone(transaction.status)}>
															{formatLabel(transaction.status)}
														</Badge>
														{formatDate(transaction.created_at)}
													</span>
												</li>
											{/each}
										</ol>
									{/if}
								</AdminSurface>
							{/each}
						</div>
					{/if}
				</AdminPanel>
			</div>

			<aside class="space-y-6">
				<AdminPanel title="Status">
					<div class="grid grid-cols-2 gap-2 sm:grid-cols-3 xl:grid-cols-2">
						{#each statusOptions as status (status)}
							<Button
								type="button"
								tone="admin"
								variant={order.status === status ? "primary" : "regular"}
								size="small"
								disabled={Boolean(updatingStatus)}
								onclick={() => updateStatus(status)}
							>
								{updatingStatus === status ? "Updating..." : formatOrderStatusLabel(status)}
							</Button>
						{/each}
					</div>
				</AdminPanel>

				<AdminPanel title="Customer">
					<dl class="space-y-4 text-sm">
						<div>
							<dt class="text-stone-500 dark:text-stone-400">Identity</dt>
							<dd class="mt-1 font-medium text-stone-950 dark:text-stone-50">{customerLabel}</dd>
						</div>
						<div>
							<dt class="text-stone-500 dark:text-stone-400">Checkout session</dt>
							<dd class="mt-1 font-medium text-stone-950 dark:text-stone-50">
								#{order.checkout_session_id}
							</dd>
						</div>
						<div>
							<dt class="text-stone-500 dark:text-stone-400">Confirmation token</dt>
							<dd class="mt-1 font-mono text-xs break-all text-stone-700 dark:text-stone-200">
								{order.confirmation_token || "Not available"}
							</dd>
						</div>
					</dl>
				</AdminPanel>

				<AdminPanel title="Fulfillment">
					<dl class="space-y-4 text-sm">
						<div>
							<dt class="text-stone-500 dark:text-stone-400">Ship to</dt>
							<dd class="mt-1 text-stone-950 dark:text-stone-50">
								{order.shipping_address_pretty || "No shipping address recorded"}
							</dd>
						</div>
						<div>
							<dt class="text-stone-500 dark:text-stone-400">Lifecycle</dt>
							<dd class="mt-1 text-stone-950 dark:text-stone-50">
								Created {formatDate(order.created_at)}
								<br />
								Updated {formatDate(order.updated_at)}
							</dd>
						</div>
					</dl>
				</AdminPanel>
			</aside>
		</div>
	{/if}
</section>

<AdminFloatingNotices
	statusMessage={notices.message}
	statusTone={notices.tone}
	onDismissStatus={notices.clear}
/>
