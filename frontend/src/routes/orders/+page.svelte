<script lang="ts">
	import { type API } from "$lib/api";
	import { type OrderModel } from "$lib/models";
	import Alert from "$lib/components/Alert.svelte";
	import Badge from "$lib/components/Badge.svelte";
	import ButtonLink from "$lib/components/ButtonLink.svelte";
	import Card from "$lib/components/Card.svelte";
	import EmptyStateCard from "$lib/components/EmptyStateCard.svelte";
	import FilterPanel from "$lib/components/FilterPanel.svelte";
	import MediaThumbnail from "$lib/components/MediaThumbnail.svelte";
	import Toast from "$lib/components/Toast.svelte";
	import Button from "$lib/components/Button.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import { formatOrderStatusLabel, getOrderStatusTone } from "$lib/components/order-status";
	import { formatPrice } from "$lib/utils";
	import { userStore } from "$lib/user";
	import { getContext } from "svelte";
	import { onDestroy, onMount } from "svelte";
	import { navigating } from "$app/state";
	import { goto } from "$app/navigation";
	import { SvelteURLSearchParams } from "svelte/reactivity";
	import { resolve } from "$app/paths";
	import type { PageData } from "./$types";

	const api: API = getContext("api");

	interface Props {
		data: PageData;
	}
	let { data }: Props = $props();

	let orders = $state<OrderModel[]>([]);
	let errorMessage = $state("");
	let isAuthenticated = $state(false);
	const loading = $derived(Boolean(navigating.to));
	const currency = $derived($userStore?.currency ?? "USD");
	let page = $state(1);
	let limit = $state("10");
	let totalPages = $state(1);
	let totalOrders = $state(0);
	let statusFilter = $state<"" | OrderModel["status"]>("");
	let startDate = $state("");
	let endDate = $state("");
	let toastMessage = $state("");
	let toastVisible = $state(false);
	let cancellingOrderId = $state<number | null>(null);
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

	function buildSearchParams(next: {
		page?: number;
		limit?: string;
		status?: "" | OrderModel["status"];
		startDate?: string;
		endDate?: string;
	}) {
		const params = new SvelteURLSearchParams();
		const resolvedPage = next.page ?? page;
		const resolvedLimit = next.limit ?? limit;
		const resolvedStatus = next.status ?? statusFilter;
		const resolvedStartDate = next.startDate ?? startDate;
		const resolvedEndDate = next.endDate ?? endDate;

		if (resolvedPage > 1) {
			params.set("page", String(resolvedPage));
		}
		if (resolvedLimit !== "10") {
			params.set("limit", resolvedLimit);
		}
		if (resolvedStatus) {
			params.set("status", resolvedStatus);
		}
		if (resolvedStartDate) {
			params.set("start_date", resolvedStartDate);
		}
		if (resolvedEndDate) {
			params.set("end_date", resolvedEndDate);
		}

		return params;
	}

	function updateUrl(next: {
		page?: number;
		limit?: string;
		status?: "" | OrderModel["status"];
		startDate?: string;
		endDate?: string;
	}) {
		const params = buildSearchParams(next);
		const path = resolve("/orders");
		const queryString = params.toString();
		const nextUrl = queryString ? `${path}?${queryString}` : path;
		// @ts-expect-error Svelte's routing requirements are strict for resolved URLs with queries
		void goto(resolve(nextUrl), { replaceState: false, noScroll: false, keepFocus: false });
	}

	function applyFilters() {
		updateUrl({ page: 1, limit, status: statusFilter, startDate, endDate });
	}

	function clearFilters() {
		updateUrl({ page: 1, status: "", startDate: "", endDate: "" });
	}

	function goToPage(nextPage: number) {
		if (nextPage < 1 || nextPage > totalPages || loading) {
			return;
		}
		updateUrl({ page: nextPage });
	}

	async function cancelOrder(orderId: number) {
		if (cancellingOrderId !== null) {
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
		cancellingOrderId = orderId;
		errorMessage = "";
		try {
			const updated = await api.cancelOrder(orderId);
			orders = orders.map((order) => (order.id === updated.id ? updated : order));
			showToast("Order cancelled.");
		} catch (err) {
			console.error(err);
			const error = err as { body?: { error?: string } };
			errorMessage = error.body?.error ?? "Unable to cancel order.";
		} finally {
			cancellingOrderId = null;
		}
	}

	function formatDate(value: Date) {
		return value.toLocaleString(undefined, {
			year: "numeric",
			month: "short",
			day: "numeric",
			hour: "numeric",
			minute: "2-digit",
		});
	}

	onMount(() => {
		if (typeof window !== "undefined") {
			const flag = window.sessionStorage.getItem("orders_toast");
			if (flag === "order_placed") {
				window.sessionStorage.removeItem("orders_toast");
				showToast("Order placed successfully.");
			}
		}
	});

	onDestroy(() => {
		clearToast();
	});

	$effect(() => {
		orders = data.orders;
		errorMessage = data.errorMessage;
		isAuthenticated = data.isAuthenticated;
		page = data.page;
		limit = data.limit;
		totalPages = data.totalPages;
		totalOrders = data.totalOrders;
		statusFilter = data.statusFilter;
		startDate = data.startDate;
		endDate = data.endDate;
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

	<div class="flex flex-wrap items-end justify-between gap-4">
		<div>
			<h1 class="text-3xl font-semibold text-gray-900 dark:text-gray-100">Your Orders</h1>
		</div>
	</div>

	{#if isAuthenticated}
		<FilterPanel class="mt-6">
			<div class="grid gap-3 sm:grid-cols-4">
				<div class="sm:col-span-1">
					<label for="statusFilter" class="mb-1 block text-sm text-gray-600 dark:text-gray-300">
						Status
					</label>
					<Dropdown id="statusFilter" bind:value={statusFilter}>
						<option value="">All statuses</option>
						<option value="PENDING">Pending</option>
						<option value="PAID">Paid</option>
						<option value="SHIPPED">Shipped</option>
						<option value="DELIVERED">Delivered</option>
						<option value="CANCELLED">Cancelled</option>
						<option value="REFUNDED">Refunded</option>
						<option value="FAILED">Failed</option>
					</Dropdown>
				</div>
				<div class="sm:col-span-1">
					<label for="startDate" class="mb-1 block text-sm text-gray-600 dark:text-gray-300">
						From date
					</label>
					<input
						id="startDate"
						type="date"
						class="w-full rounded-md border border-gray-300 bg-gray-200 px-3 py-2 dark:border-gray-700 dark:bg-gray-800"
						bind:value={startDate}
					/>
				</div>
				<div class="sm:col-span-1">
					<label for="endDate" class="mb-1 block text-sm text-gray-600 dark:text-gray-300">
						To date
					</label>
					<input
						id="endDate"
						type="date"
						class="w-full rounded-md border border-gray-300 bg-gray-200 px-3 py-2 dark:border-gray-700 dark:bg-gray-800"
						bind:value={endDate}
					/>
				</div>
				<div class="sm:col-span-1">
					<label for="limit" class="mb-1 block text-sm text-gray-600 dark:text-gray-300"
						>Per page</label
					>
					<Dropdown id="limit" bind:value={limit}>
						<option value="10">10</option>
						<option value="20">20</option>
						<option value="50">50</option>
					</Dropdown>
				</div>
			</div>
			<div class="mt-3 flex flex-wrap gap-2">
				<Button type="button" variant="primary" onclick={applyFilters} disabled={loading}>
					Apply filters
				</Button>
				<Button
					type="button"
					variant="regular"
					class="cursor-pointer rounded-lg border border-gray-300 bg-gray-200 px-4 py-2 transition-[background-color,border-color] duration-200 hover:border-gray-200 hover:bg-gray-100 dark:border-gray-600 dark:bg-gray-700"
					onclick={clearFilters}
					disabled={loading}
				>
					Clear
				</Button>
			</div>
		</FilterPanel>
	{/if}

	{#if !isAuthenticated}
		<p class="mt-4 text-gray-600 dark:text-gray-300">
			Please
			<a href={resolve("/login")} class="text-blue-600 hover:underline dark:text-blue-400">
				log in
			</a>
			to view your orders.
		</p>
	{:else if errorMessage}
		<div class="mt-4">
			<Alert
				message={errorMessage}
				tone="error"
				icon="bi-x-circle-fill"
				onClose={() => (errorMessage = "")}
			/>
		</div>
	{:else if orders.length === 0}
		<EmptyStateCard
			title="No orders yet."
			description="Your future purchases will show up here."
			class="mt-6"
			headingClass="text-lg font-medium"
		>
			<ButtonLink href={resolve("/")} variant="primary" size="large" class="mt-4 block">
				Start shopping
			</ButtonLink>
		</EmptyStateCard>
	{:else}
		<p class="mt-4 text-sm text-gray-600 dark:text-gray-400">
			Showing page {page} of {totalPages} ({totalOrders} total orders)
		</p>
		<div class="mt-6 space-y-4">
			{#each orders as order (order.id)}
				<Card padding="md">
					<div class="grid grid-cols-[1fr_auto] items-start gap-3 sm:items-center">
						<div>
							<p class="text-sm text-gray-500 dark:text-gray-400">Order #{order.id}</p>
							<p class="text-xl font-semibold text-gray-900 dark:text-gray-100">
								{formatPrice(order.total, currency)}
							</p>
						</div>
						<div class="flex flex-col items-end gap-1 justify-self-end text-right">
							<Badge tone={getOrderStatusTone(order.status)} class="w-min">
								{formatOrderStatusLabel(order.status)}
							</Badge>
							<p class="text-sm text-gray-600 dark:text-gray-400">
								{formatDate(order.created_at)}
							</p>
						</div>
					</div>

					<div class="mt-4 border-t border-gray-200 pt-4 dark:border-gray-800">
						<Card
							tone="muted"
							radius="xl"
							padding="sm"
							shadow="none"
							class="grid gap-3 text-sm sm:grid-cols-2"
						>
							<div class="min-w-0">
								<p
									class="text-xs font-semibold tracking-wide text-gray-500 uppercase dark:text-gray-400"
								>
									Payment
								</p>
								<p class="mt-1 wrap-break-word text-gray-800 dark:text-gray-200">
									{order.payment_method_display || "No payment method recorded"}
								</p>
							</div>
							<div class="min-w-0">
								<p
									class="text-xs font-semibold tracking-wide text-gray-500 uppercase dark:text-gray-400"
								>
									Shipping
								</p>
								<p class="mt-1 wrap-break-word text-gray-800 dark:text-gray-200">
									{order.shipping_address_pretty || "No shipping address recorded"}
								</p>
							</div>
						</Card>

						{#if order.items.length === 0}
							<p class="mt-4 text-sm text-gray-500 dark:text-gray-400">
								No item details available.
							</p>
						{:else}
							<ul class="mt-4 space-y-3">
								{#each order.items as item (item.id)}
									<Card
										as="li"
										radius="xl"
										padding="sm"
										shadow="none"
										class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"
									>
										<div class="flex min-w-0 items-center gap-3">
											<MediaThumbnail
												href={resolve(`/product/${item.product.id}`)}
												src={item.product.cover_image}
												alt={item.product.name}
												class="h-14 w-14 rounded-lg text-[10px]"
											/>
											<div class="min-w-0">
												<a
													href={resolve(`/product/${item.product.id}`)}
													class="block truncate font-medium text-gray-900 hover:underline dark:text-gray-100"
												>
													{item.product.name}
												</a>
												<p class="text-sm text-gray-500 dark:text-gray-400">
													Qty {item.quantity} x {formatPrice(item.price, currency)}
												</p>
											</div>
										</div>
										<p class="text-right font-medium text-gray-900 dark:text-gray-100">
											{formatPrice(item.price * item.quantity, currency)}
										</p>
									</Card>
								{/each}
							</ul>
						{/if}

						<div class="mt-4 flex items-center justify-between text-sm">
							<span class="text-gray-600 dark:text-gray-400">Order total</span>
							<span class="font-semibold text-gray-900 dark:text-gray-100">
								{formatPrice(order.total, currency)}
							</span>
						</div>

						<div class="mt-4 flex flex-wrap items-center justify-between gap-3">
							<ButtonLink
								href={resolve(`/orders/${order.id}`)}
								variant="regular"
								class="inline-flex items-center gap-2"
							>
								<i class="bi bi-truck"></i>
								View details
							</ButtonLink>

							{#if order.can_cancel}
								<Button
									size="small"
									type="button"
									variant="regular"
									disabled={cancellingOrderId !== null}
									onclick={() => cancelOrder(order.id)}
								>
									{cancellingOrderId === order.id ? "Cancelling..." : "Cancel order"}
								</Button>
							{/if}
						</div>
					</div>
				</Card>
			{/each}
		</div>

		<div class="mt-6 flex items-center justify-end gap-2">
			<Button size="small" onclick={() => goToPage(page - 1)} disabled={page <= 1 || loading}>
				<i class="bi bi-arrow-left"></i>
				Previous
			</Button>
			<Button
				size="small"
				onclick={() => goToPage(page + 1)}
				disabled={page >= totalPages || loading}
			>
				Next
				<i class="bi bi-arrow-right"></i>
			</Button>
		</div>
	{/if}
</section>
