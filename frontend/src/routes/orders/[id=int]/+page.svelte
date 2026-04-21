<script lang="ts">
	import { type API } from "$lib/api";
	import Alert from "$lib/components/Alert.svelte";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import ButtonLink from "$lib/components/ButtonLink.svelte";
	import Card from "$lib/components/Card.svelte";
	import { formatOrderStatusLabel, getOrderStatusTone } from "$lib/components/order-status";
	import Toast from "$lib/components/Toast.svelte";
	import { type OrderModel, type ShipmentModel } from "$lib/models";
	import { formatPrice } from "$lib/utils";
	import { userStore } from "$lib/user";
	import { resolve } from "$app/paths";
	import { getContext } from "svelte";
	import { onDestroy } from "svelte";
	import type { PageData } from "./$types";
	import { shouldShowShipmentEmptyState } from "./page-state";

	const api: API = getContext("api");

	interface Props {
		data: PageData;
	}

	let { data }: Props = $props();

	let isAuthenticated = $state<boolean | null>(null);
	let order = $state<OrderModel | null>(null);
	let shipments = $state<ShipmentModel[]>([]);
	let errorMessage = $state("");
	let trackingErrorMessage = $state("");
	let toastMessage = $state("");
	let toastVisible = $state(false);
	let cancelling = $state(false);

	const currency = $derived($userStore?.currency ?? "USD");

	let toastTimeout: ReturnType<typeof setTimeout> | null = null;
	let toastHideTimeout: ReturnType<typeof setTimeout> | null = null;

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
		}, 3200);
		toastHideTimeout = setTimeout(() => {
			toastMessage = "";
		}, 3600);
	}

	function formatDate(value: Date | null, includeTime = true) {
		if (!value) {
			return "Not yet";
		}
		return value.toLocaleString(undefined, {
			year: "numeric",
			month: "short",
			day: "numeric",
			...(includeTime ? { hour: "numeric", minute: "2-digit" } : {}),
		});
	}

	function formatStatusLabel(value: string) {
		return value
			.toLowerCase()
			.split("_")
			.map((part) => part.charAt(0).toUpperCase() + part.slice(1))
			.join(" ");
	}

	function getShipmentStatusTone(status: ShipmentModel["status"]) {
		switch (status) {
			case "DELIVERED":
				return "success" as const;
			case "IN_TRANSIT":
				return "info" as const;
			case "EXCEPTION":
				return "danger" as const;
			case "LABEL_PURCHASED":
				return "neutral" as const;
			default:
				return "neutral" as const;
		}
	}

	function getLatestEvent(shipment: ShipmentModel) {
		return shipment.tracking_events
			.slice()
			.sort((left, right) => right.occurred_at.getTime() - left.occurred_at.getTime())[0];
	}

	function getSortedTrackingEvents(shipment: ShipmentModel) {
		return shipment.tracking_events
			.slice()
			.sort((left, right) => right.occurred_at.getTime() - left.occurred_at.getTime());
	}

	function getShipmentEmptyMessage(currentOrder: OrderModel) {
		switch (currentOrder.status) {
			case "CANCELLED":
			case "REFUNDED":
				return "This order is closed, so no shipment updates are expected.";
			case "DELIVERED":
				return "Delivery is complete, but the carrier did not provide a visible timeline.";
			default:
				return "A shipping label has not been purchased yet. Tracking will appear here once fulfillment starts.";
		}
	}

	function getItemCount(currentOrder: OrderModel) {
		return currentOrder.items.reduce((total, item) => total + item.quantity, 0);
	}

	async function cancelOrder() {
		if (!order || cancelling) {
			return;
		}
		if (typeof window !== "undefined") {
			const confirmed = window.confirm(
				"Cancel this order? This cannot be undone and eligible items will be restocked."
			);
			if (!confirmed) {
				return;
			}
		}

		cancelling = true;
		errorMessage = "";
		try {
			order = await api.cancelOrder(order.id);
			showToast("Order cancelled.");
		} catch (err) {
			console.error(err);
			const error = err as { body?: { error?: string } };
			errorMessage = error.body?.error ?? "Unable to cancel order.";
		} finally {
			cancelling = false;
		}
	}

	onDestroy(() => {
		clearToast();
	});

	$effect(() => {
		isAuthenticated = data.isAuthenticated;
		order = data.order;
		shipments = data.shipments;
		errorMessage = data.errorMessage;
		trackingErrorMessage = data.trackingErrorMessage;
	});
</script>

<section class="mx-auto max-w-5xl px-4 py-10">
	<Toast
		message={toastMessage}
		visible={toastVisible}
		tone="success"
		position="top-center"
		onClose={clearToast}
	/>

	<a
		href={resolve("/orders")}
		class="inline-flex items-center gap-2 text-sm font-medium text-gray-600 transition hover:text-gray-900 dark:text-gray-300 dark:hover:text-gray-100"
	>
		<i class="bi bi-arrow-left"></i>
		Back to orders
	</a>

	{#if isAuthenticated === false}
		<Card class="mt-6" padding="xl">
			<h1 class="text-2xl font-semibold text-gray-900 dark:text-gray-100">Order details</h1>
			<p class="mt-3 text-gray-600 dark:text-gray-300">
				Please
				<a href={resolve("/login")} class="text-blue-600 hover:underline dark:text-blue-400">
					log in
				</a>
				to view your order history and shipment updates.
			</p>
		</Card>
	{:else if !order}
		<Card class="mt-6" padding="xl">
			<h1 class="text-2xl font-semibold text-gray-900 dark:text-gray-100">Order details</h1>
			<div class="mt-4">
				<Alert
					message={errorMessage || "Order not found."}
					tone="error"
					icon="bi-x-circle-fill"
					onClose={() => (errorMessage = "")}
				/>
			</div>
		</Card>
	{:else}
		<div class="mt-6 space-y-6">
			{#if errorMessage}
				<Alert
					message={errorMessage}
					tone="error"
					icon="bi-x-circle-fill"
					onClose={() => (errorMessage = "")}
				/>
			{/if}

			<Card padding="none" overflowHidden={true}>
				<div class="p-6">
					<h1 class="mb-4 text-3xl font-semibold text-gray-900 dark:text-gray-100">
						Order details
					</h1>
					<div class="flex flex-wrap items-center gap-3">
						<p class="text-sm font-medium text-gray-500 dark:text-gray-400">
							Order #{order.id}, placed {formatDate(order.created_at)}.
						</p>
						<Badge tone={getOrderStatusTone(order.status)}>
							{formatOrderStatusLabel(order.status)}
						</Badge>
					</div>

					<div
						class="mt-6 grid grid-cols-[minmax(0,1fr)_11rem] items-start gap-4 sm:grid-cols-[minmax(0,1fr)_13rem]"
					>
						<div>
							<Card
								as="dl"
								tone="muted"
								radius="xl"
								padding="none"
								overflowHidden={true}
								class="grid sm:grid-cols-3"
							>
								<div class="bg-white p-4 dark:bg-gray-900">
									<dt class="text-sm text-gray-500 dark:text-gray-400">Items</dt>
									<dd class="mt-1 text-base font-semibold text-gray-900 dark:text-gray-100">
										{getItemCount(order)} item{getItemCount(order) === 1 ? "" : "s"}
									</dd>
								</div>
								<div
									class="border-t border-gray-200 bg-white p-4 sm:border-t-0 sm:border-l dark:border-gray-800 dark:bg-gray-900"
								>
									<dt class="text-sm text-gray-500 dark:text-gray-400">Shipments</dt>
									<dd class="mt-1 text-base font-semibold text-gray-900 dark:text-gray-100">
										{shipments.length} shipment{shipments.length === 1 ? "" : "s"}
									</dd>
								</div>
								<div
									class="border-t border-gray-200 bg-white p-4 sm:border-t-0 sm:border-l dark:border-gray-800 dark:bg-gray-900"
								>
									<dt class="text-sm text-gray-500 dark:text-gray-400">Placed</dt>
									<dd class="mt-1 text-base font-semibold text-gray-900 dark:text-gray-100">
										{formatDate(order.created_at, false)}
									</dd>
								</div>
							</Card>

							<Card
								as="div"
								tone="muted"
								radius="xl"
								padding="none"
								overflowHidden={true}
								class="mt-4 grid sm:grid-cols-3"
							>
								<div class="bg-white p-4 dark:bg-gray-900">
									<p class="text-sm text-gray-500 dark:text-gray-400">Payment method</p>
									<p class="mt-2 text-sm text-gray-800 dark:text-gray-200">
										{order.payment_method_display || "No payment method recorded"}
									</p>
								</div>
								<div
									class="border-t border-gray-200 bg-white p-4 sm:border-t-0 sm:border-l dark:border-gray-800 dark:bg-gray-900"
								>
									<p class="text-sm text-gray-500 dark:text-gray-400">Shipping address</p>
									<p class="mt-2 text-sm text-gray-800 dark:text-gray-200">
										{order.shipping_address_pretty || "No shipping address recorded"}
									</p>
								</div>
								<div
									class="border-t border-gray-200 bg-white p-4 sm:border-t-0 sm:border-l dark:border-gray-800 dark:bg-gray-900"
								>
									<p class="text-sm text-gray-500 dark:text-gray-400">Last updated</p>
									<p class="mt-2 text-sm text-gray-800 dark:text-gray-200">
										{formatDate(order.updated_at)}
									</p>
								</div>
							</Card>
						</div>

						<div class="flex flex-col gap-4">
							<Card tone="muted" radius="xl" padding="none" overflowHidden={true}>
								<div class="bg-white p-5 dark:bg-gray-900">
									<p class="text-sm text-gray-500 dark:text-gray-400">Order total</p>
									<p class="mt-1 text-3xl font-semibold text-gray-900 dark:text-gray-100">
										{formatPrice(order.total, currency)}
									</p>
								</div>
								{#if shipments[0]?.tracking_url || order.can_cancel}
									<div
										class="flex flex-col gap-2 border-t border-gray-200 bg-white p-4 dark:border-gray-800 dark:bg-gray-900"
									>
										{#if shipments[0]?.tracking_url}
											<ButtonLink
												href={shipments[0].tracking_url}
												target="_blank"
												rel="noreferrer"
												variant="primary"
												class="inline-flex items-center gap-2"
											>
												<i class="bi bi-truck"></i>
												Track package
											</ButtonLink>
										{/if}
										{#if order.can_cancel}
											<Button
												type="button"
												variant="regular"
												disabled={cancelling}
												class="inline-flex items-center gap-2"
												onclick={cancelOrder}
											>
												<i class="bi bi-x-circle"></i>
												{cancelling ? "Cancelling..." : "Cancel order"}
											</Button>
										{/if}
									</div>
								{/if}
							</Card>
						</div>
					</div>
				</div>
			</Card>

			<div class="grid gap-6 lg:grid-cols-[minmax(0,1.6fr)_minmax(320px,1fr)]">
				<div class="space-y-6">
					<Card padding="lg">
						<div class="flex flex-wrap items-center justify-between gap-3">
							<div>
								<h2 class="text-xl font-semibold text-gray-900 dark:text-gray-100">
									Shipment tracking
								</h2>
							</div>
						</div>

						{#if trackingErrorMessage}
							<div class="mt-4">
								<Alert
									message={trackingErrorMessage}
									tone="error"
									icon="bi-exclamation-triangle-fill"
									onClose={() => (trackingErrorMessage = "")}
								/>
							</div>
						{/if}

						{#if shouldShowShipmentEmptyState(shipments, trackingErrorMessage)}
							<Card
								tone="muted"
								border="dashed"
								radius="xl"
								padding="md"
								class="mt-5 text-sm text-gray-600 dark:text-gray-300"
							>
								<p class="font-medium text-gray-900 dark:text-gray-100">
									No shipment activity yet.
								</p>
								<p class="mt-2">{getShipmentEmptyMessage(order)}</p>
							</Card>
						{:else}
							<div class="mt-5 space-y-4">
								{#each shipments as shipment, index (shipment.id)}
									{@const latestEvent = getLatestEvent(shipment)}
									<Card as="article" tone="muted" padding="none" overflowHidden={true}>
										<div class="flex flex-wrap items-start justify-between gap-3 p-5">
											<div>
												<p class="text-sm font-medium text-gray-500 dark:text-gray-400">
													Shipment {index + 1}
												</p>
												<h3 class="mt-1 text-lg font-semibold text-gray-900 dark:text-gray-100">
													{shipment.service_name}
												</h3>
												{#if latestEvent}
													<p class="mt-2 text-sm text-gray-600 dark:text-gray-300">
														{latestEvent.description}
													</p>
												{/if}
											</div>
											<div class="flex flex-col items-start gap-2 sm:items-end">
												<Badge tone={getShipmentStatusTone(shipment.status)}>
													{formatStatusLabel(shipment.status)}
												</Badge>
												{#if shipment.tracking_url}
													<ButtonLink
														href={shipment.tracking_url}
														target="_blank"
														rel="noreferrer"
														variant="regular"
														class="inline-flex items-center gap-2"
													>
														Open carrier tracking
														<i class="bi bi-arrow-up-right"></i>
													</ButtonLink>
												{/if}
											</div>
										</div>

										<div
											class="mx-5 mb-5 grid overflow-hidden rounded-xl border border-gray-200 bg-gray-100 sm:grid-cols-2 xl:grid-cols-4 dark:border-gray-800 dark:bg-gray-800"
										>
											<div class="bg-white p-4 dark:bg-gray-900">
												<p class="text-sm text-gray-500 dark:text-gray-400">Tracking number</p>
												<p
													class="mt-2 font-mono text-sm break-all text-gray-800 dark:text-gray-200"
												>
													{shipment.tracking_number}
												</p>
											</div>
											<div
												class="border-t border-gray-200 bg-white p-4 sm:border-t-0 sm:border-l dark:border-gray-800 dark:bg-gray-900"
											>
												<p class="text-sm text-gray-500 dark:text-gray-400">Provider</p>
												<p class="mt-2 text-sm text-gray-800 dark:text-gray-200">
													{shipment.provider}
												</p>
											</div>
											<div
												class="border-t border-gray-200 bg-white p-4 xl:border-t-0 xl:border-l dark:border-gray-800 dark:bg-gray-900"
											>
												<p class="text-sm text-gray-500 dark:text-gray-400">Purchased</p>
												<p class="mt-2 text-sm text-gray-800 dark:text-gray-200">
													{formatDate(shipment.purchased_at)}
												</p>
											</div>
											<div
												class="border-t border-gray-200 bg-white p-4 sm:border-l dark:border-gray-800 dark:bg-gray-900"
											>
												<p class="text-sm text-gray-500 dark:text-gray-400">Packages</p>
												<p class="mt-2 text-sm text-gray-800 dark:text-gray-200">
													{shipment.packages.length}
												</p>
											</div>
										</div>

										<div
											class="grid gap-5 border-t border-gray-200 p-5 lg:grid-cols-[minmax(0,1fr)_280px] dark:border-gray-800"
										>
											<div>
												<h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
													Carrier timeline
												</h4>
												{#if shipment.tracking_events.length === 0}
													<p class="mt-3 text-sm text-gray-600 dark:text-gray-300">
														No carrier scans yet.
													</p>
												{:else}
													<ol class="mt-4 space-y-3">
														{#each getSortedTrackingEvents(shipment) as event (event.id)}
															<li
																class="rounded-xl border border-gray-200 bg-white p-4 dark:border-gray-800 dark:bg-gray-900"
															>
																<div class="flex flex-wrap items-start justify-between gap-3">
																	<div>
																		<p class="font-medium text-gray-900 dark:text-gray-100">
																			{event.description}
																		</p>
																		<p class="mt-1 text-sm text-gray-600 dark:text-gray-300">
																			{event.location}
																		</p>
																	</div>
																	<div class="text-right text-sm text-gray-500 dark:text-gray-400">
																		<p>{formatStatusLabel(event.status)}</p>
																		<p class="mt-1">{formatDate(event.occurred_at)}</p>
																	</div>
																</div>
															</li>
														{/each}
													</ol>
												{/if}
											</div>

											<div
												class="rounded-xl border border-gray-200 bg-white p-4 dark:border-gray-800 dark:bg-gray-900"
											>
												<h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
													Shipment details
												</h4>
												<dl class="mt-4 space-y-3 text-sm">
													<div>
														<dt class="text-gray-500 dark:text-gray-400">Shipping amount</dt>
														<dd class="mt-1 font-medium text-gray-900 dark:text-gray-100">
															{formatPrice(shipment.amount, shipment.currency)}
														</dd>
													</div>
													<div>
														<dt class="text-gray-500 dark:text-gray-400">Delivered</dt>
														<dd class="mt-1 font-medium text-gray-900 dark:text-gray-100">
															{formatDate(shipment.delivered_at)}
														</dd>
													</div>
													<div>
														<dt class="text-gray-500 dark:text-gray-400">Destination</dt>
														<dd class="mt-1 text-gray-900 dark:text-gray-100">
															{shipment.shipping_address_pretty}
														</dd>
													</div>
												</dl>
											</div>
										</div>
									</Card>
								{/each}
							</div>
						{/if}
					</Card>
				</div>

				<aside class="space-y-6">
					<Card padding="lg">
						<h2 class="text-xl font-semibold text-gray-900 dark:text-gray-100">Order contents</h2>
						{#if order.items.length === 0}
							<p class="mt-4 text-sm text-gray-600 dark:text-gray-300">
								No item details are available for this order.
							</p>
						{:else}
							<ul class="mt-5 space-y-3">
								{#each order.items as item (item.id)}
									<Card as="li" tone="muted" radius="xl" padding="sm">
										<div class="flex items-start gap-3">
											<a
												href={resolve(`/product/${item.product.id}`)}
												class="flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-lg border border-gray-200 bg-gray-100 text-[10px] text-gray-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400"
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
											<div class="min-w-0 flex-1">
												<a
													href={resolve(`/product/${item.product.id}`)}
													class="block truncate font-medium text-gray-900 hover:underline dark:text-gray-100"
												>
													{item.product.name}
												</a>
												<p class="mt-1 text-sm text-gray-600 dark:text-gray-300">
													{item.variant_title}
												</p>
												<div class="mt-3 flex items-center justify-between gap-3 text-sm">
													<span class="text-gray-600 dark:text-gray-300">
														Qty {item.quantity}
													</span>
													<span class="font-medium text-gray-900 dark:text-gray-100">
														{formatPrice(item.price * item.quantity, currency)}
													</span>
												</div>
											</div>
										</div>
									</Card>
								{/each}
							</ul>
						{/if}
					</Card>
				</aside>
			</div>
		</div>
	{/if}
</section>
