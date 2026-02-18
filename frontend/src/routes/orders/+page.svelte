<script lang="ts">
	import { type API } from "$lib/api";
	import { type OrderModel } from "$lib/models";
	import Alert from "$lib/components/alert.svelte";
	import ButtonLink from "$lib/components/ButtonLink.svelte";
	import { formatPrice } from "$lib/utils";
	import { userStore } from "$lib/user";
	import { getContext, onMount } from "svelte";
	import { resolve } from "$app/paths";

	const api: API = getContext("api");

	let orders = $state<OrderModel[]>([]);
	let loading = $state(true);
	let errorMessage = $state("");
	let authChecked = $state(false);
	const skeletonRows = [0, 1, 2];
	const currency = $derived($userStore?.currency ?? "USD");
	let page = $state(1);
	let limit = $state("10");
	let totalPages = $state(1);
	let totalOrders = $state(0);
	let statusFilter = $state<"" | OrderModel["status"]>("");
	let startDate = $state("");
	let endDate = $state("");

	async function loadOrders() {
		api.tokenFromCookie();
		authChecked = true;
		if (!api.isAuthenticated()) {
			loading = false;
			return;
		}

		loading = true;
		errorMessage = "";
		try {
			const response = await api.listOrders({
				page,
				limit: Number(limit),
				status: statusFilter,
				start_date: startDate || undefined,
				end_date: endDate || undefined,
			});
			totalPages = Math.max(1, response.pagination.total_pages);
			totalOrders = response.pagination.total;
			const missingItems = response.data.filter((order) => order.items.length === 0);

			if (missingItems.length > 0) {
				const detailResults = await Promise.allSettled(
					missingItems.map((order) => api.getOrderDetails(order.id))
				);
				const detailsById = new Map<number, OrderModel>();

				for (const result of detailResults) {
					if (result.status === "fulfilled") {
						detailsById.set(result.value.id, result.value);
					}
				}

				orders = response.data.map((order) => detailsById.get(order.id) ?? order);
			} else {
				orders = response.data;
			}
		} catch (err) {
			console.error(err);
			errorMessage = "Unable to load orders.";
		}
		loading = false;
	}

	function applyFilters() {
		page = 1;
		loadOrders();
	}

	function clearFilters() {
		statusFilter = "";
		startDate = "";
		endDate = "";
		page = 1;
		loadOrders();
	}

	function goToPage(nextPage: number) {
		if (nextPage < 1 || nextPage > totalPages || loading) {
			return;
		}
		page = nextPage;
		loadOrders();
	}

	function statusBadge(status: OrderModel["status"]) {
		switch (status) {
			case "PAID":
				return "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-200";
			case "FAILED":
				return "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200";
			default:
				return "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200";
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

	onMount(loadOrders);
</script>

<section class="mx-auto max-w-5xl px-4 py-10">
	<div class="flex flex-wrap items-end justify-between gap-4">
		<div>
			<h1 class="text-3xl font-semibold text-gray-900 dark:text-gray-100">Your Orders</h1>
		</div>
	</div>

	{#if authChecked && api.isAuthenticated()}
		<div class="mt-6 rounded-2xl border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-800 dark:bg-gray-900">
			<div class="grid gap-3 sm:grid-cols-4">
				<div class="sm:col-span-1">
					<label for="statusFilter" class="mb-1 block text-sm text-gray-600 dark:text-gray-300">
						Status
					</label>
					<select
						id="statusFilter"
						class="w-full rounded-md border border-gray-300 bg-gray-200 px-3 py-2 dark:border-gray-700 dark:bg-gray-800"
						bind:value={statusFilter}
					>
						<option value="">All statuses</option>
						<option value="PENDING">Pending</option>
						<option value="PAID">Paid</option>
						<option value="FAILED">Failed</option>
					</select>
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
					<label for="limit" class="mb-1 block text-sm text-gray-600 dark:text-gray-300">Per page</label>
					<select
						id="limit"
						class="w-full rounded-md border border-gray-300 bg-gray-200 px-3 py-2 dark:border-gray-700 dark:bg-gray-800"
						bind:value={limit}
					>
						<option value="10">10</option>
						<option value="20">20</option>
						<option value="50">50</option>
					</select>
				</div>
			</div>
			<div class="mt-3 flex flex-wrap gap-2">
				<button
					type="button"
					class="cursor-pointer rounded-lg border border-blue-400 bg-blue-500 px-4 py-2 text-white transition-[background-color,border-color] duration-200 hover:border-blue-500 hover:bg-blue-600"
					onclick={applyFilters}
					disabled={loading}
				>
					Apply filters
				</button>
				<button
					type="button"
					class="cursor-pointer rounded-lg border border-gray-300 bg-gray-200 px-4 py-2 transition-[background-color,border-color] duration-200 hover:border-gray-200 hover:bg-gray-100 dark:border-gray-600 dark:bg-gray-700"
					onclick={clearFilters}
					disabled={loading}
				>
					Clear
				</button>
			</div>
		</div>
	{/if}

	{#if !authChecked}
		<div class="mt-6 space-y-4">
			{#each skeletonRows as index (index)}
				<div
					class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-800 dark:bg-gray-900"
				>
					<div class="flex items-center justify-between">
						<div class="h-4 w-28 animate-pulse rounded bg-gray-200 dark:bg-gray-800"></div>
						<div class="h-6 w-20 animate-pulse rounded-full bg-gray-200 dark:bg-gray-800"></div>
					</div>
					<div class="mt-4 h-5 w-32 animate-pulse rounded bg-gray-200 dark:bg-gray-800"></div>
					<div class="mt-2 h-4 w-40 animate-pulse rounded bg-gray-200 dark:bg-gray-800"></div>
				</div>
			{/each}
		</div>
	{:else if !api.isAuthenticated()}
		<p class="mt-4 text-gray-600 dark:text-gray-300">
			Please
			<a href={resolve("/login")} class="text-blue-600 hover:underline dark:text-blue-400">
				log in
			</a>
			to view your orders.
		</p>
	{:else if loading}
		<div class="mt-6 space-y-4">
			{#each skeletonRows as index (index)}
				<div
					class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-800 dark:bg-gray-900"
				>
					<div class="flex items-center justify-between">
						<div class="h-4 w-28 animate-pulse rounded bg-gray-200 dark:bg-gray-800"></div>
						<div class="h-6 w-20 animate-pulse rounded-full bg-gray-200 dark:bg-gray-800"></div>
					</div>
					<div class="mt-4 h-5 w-32 animate-pulse rounded bg-gray-200 dark:bg-gray-800"></div>
					<div class="mt-2 h-4 w-40 animate-pulse rounded bg-gray-200 dark:bg-gray-800"></div>
				</div>
			{/each}
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
	{:else if orders.length === 0}
		<div
			class="mt-6 flex flex-col items-center rounded-2xl border border-dashed border-gray-300 bg-white p-8 text-gray-600 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-300"
		>
			<p class="text-lg font-medium">No orders yet.</p>
			<p class="mt-2 text-sm">Your future purchases will show up here.</p>
			<ButtonLink href={resolve("/")} variant="primary" size="large" class="mt-4 block">
				Start shopping
			</ButtonLink>
		</div>
	{:else}
		<p class="mt-4 text-sm text-gray-600 dark:text-gray-400">
			Showing page {page} of {totalPages} ({totalOrders} total orders)
		</p>
		<div class="mt-6 space-y-4">
			{#each orders as order (order.id)}
				<div
					class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-800 dark:bg-gray-900"
				>
					<div class="grid grid-cols-[1fr_auto] items-start gap-3 sm:items-center">
						<div>
							<p class="text-sm text-gray-500 dark:text-gray-400">Order #{order.id}</p>
							<p class="text-xl font-semibold text-gray-900 dark:text-gray-100">
								{formatPrice(order.total, currency)}
							</p>
						</div>
						<div class="flex flex-col items-end gap-1 justify-self-end text-right">
							<span
								class={`inline-flex w-min items-center rounded-full px-3 py-1 text-xs font-medium ${statusBadge(order.status)}`}
							>
								{order.status}
							</span>
							<p class="text-sm text-gray-600 dark:text-gray-400">
								{formatDate(order.created_at)}
							</p>
						</div>
					</div>

					<div class="mt-4 border-t border-gray-200 pt-4 dark:border-gray-800">
						{#if order.items.length === 0}
							<p class="text-sm text-gray-500 dark:text-gray-400">No item details available.</p>
						{:else}
							<ul class="space-y-3">
								{#each order.items as item (item.id)}
									<li
										class="flex flex-col gap-3 rounded-xl border border-gray-200 p-3 sm:flex-row sm:items-center sm:justify-between dark:border-gray-800"
									>
										<div class="flex min-w-0 items-center gap-3">
											<a
												href={resolve(`/product/${item.product.id}`)}
												class="flex h-14 w-14 shrink-0 items-center justify-center overflow-hidden rounded-lg border border-gray-200 bg-gray-100 text-[10px] text-gray-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400"
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
									</li>
								{/each}
							</ul>
						{/if}

						<div class="mt-4 flex items-center justify-between text-sm">
							<span class="text-gray-600 dark:text-gray-400">Order total</span>
							<span class="font-semibold text-gray-900 dark:text-gray-100">
								{formatPrice(order.total, currency)}
							</span>
						</div>
					</div>
				</div>
			{/each}
		</div>

		<div class="mt-6 flex items-center justify-end gap-2">
			<button
				type="button"
				class="cursor-pointer rounded-lg border border-gray-300 bg-gray-200 px-4 py-2 text-sm transition-[background-color,border-color] duration-200 hover:border-gray-200 hover:bg-gray-100 disabled:cursor-auto disabled:opacity-50 dark:border-gray-600 dark:bg-gray-700 hover:dark:border-gray-700 hover:dark:bg-gray-800"
				onclick={() => goToPage(page - 1)}
				disabled={page <= 1 || loading}
			>
				Previous
			</button>
			<button
				type="button"
				class="cursor-pointer rounded-lg border border-gray-300 bg-gray-200 px-4 py-2 text-sm transition-[background-color,border-color] duration-200 hover:border-gray-200 hover:bg-gray-100 disabled:cursor-auto disabled:opacity-50 dark:border-gray-600 dark:bg-gray-700 hover:dark:border-gray-700 hover:dark:bg-gray-800"
				onclick={() => goToPage(page + 1)}
				disabled={page >= totalPages || loading}
			>
				Next
			</button>
		</div>
	{/if}
</section>
