<script lang="ts">
	import { type API } from "$lib/api";
	import { type OrderModel } from "$lib/models";
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
			orders = await api.listOrders({ page: 1, limit: 20 });
		} catch (err) {
			console.error(err);
			errorMessage = "Unable to load orders.";
		}
		loading = false;
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

	onMount(loadOrders);
</script>

<section class="mx-auto max-w-5xl px-4 py-10">
	<div class="flex flex-wrap items-end justify-between gap-4">
		<div>
			<h1 class="text-3xl font-semibold text-gray-900 dark:text-gray-100">Your Orders</h1>
		</div>
	</div>

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
		<p class="mt-4 text-red-500">{errorMessage}</p>
	{:else if orders.length === 0}
		<div
			class="mt-6 rounded-2xl border border-dashed border-gray-300 bg-white p-8 text-center text-gray-600 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-300"
		>
			<p class="text-lg font-medium">No orders yet.</p>
			<p class="mt-2 text-sm">Your future purchases will show up here.</p>
			<a href={resolve("/")} class="btn btn-large btn-primary mt-4">Start shopping</a>
		</div>
	{:else}
		<div class="mt-6 space-y-4">
			{#each orders as order (order.id)}
				<div
					class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-800 dark:bg-gray-900"
				>
					<div class="grid grid-cols-[1fr_auto] items-start gap-3 sm:items-center">
						<div>
							<p class="text-sm text-gray-500 dark:text-gray-400">Order #{order.id}</p>
							<p class="text-xl font-semibold text-gray-900 dark:text-gray-100">
								{formatPrice(order.total, $userStore?.currency ?? "USD")}
							</p>
						</div>
						<div class="flex flex-col items-end gap-1 justify-self-end text-right">
							<span
								class={`inline-flex w-min items-center rounded-full px-3 py-1 text-xs font-medium ${statusBadge(order.status)}`}
							>
								{order.status}
							</span>
							<p class="text-sm text-gray-600 dark:text-gray-400">
								{order.created_at.toLocaleDateString()}
							</p>
						</div>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</section>
