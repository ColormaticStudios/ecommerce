<script lang="ts">
	import { getContext, onMount, untrack } from "svelte";
	import { type API } from "$lib/api";
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminFloatingNotices from "$lib/admin/AdminFloatingNotices.svelte";
	import AdminListItem from "$lib/admin/AdminListItem.svelte";
	import AdminMetaText from "$lib/admin/AdminMetaText.svelte";
	import AdminPageHeader from "$lib/admin/AdminPageHeader.svelte";
	import AdminPaginationControls from "$lib/admin/AdminPaginationControls.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import AdminResourceActions from "$lib/admin/AdminResourceActions.svelte";
	import AdminSurface from "$lib/admin/AdminSurface.svelte";
	import {
		createAdminPaginatedResource,
		formatAdminDateTime,
		replaceItemById,
	} from "$lib/admin/state.svelte";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import { formatOrderStatusLabel, getOrderStatusTone } from "$lib/components/order-status";
	import { type OrderModel, type UserModel } from "$lib/models";
	import { formatPrice } from "$lib/utils";
	import type { PageData } from "./$types";

	interface Props {
		data: PageData;
	}

	let { data }: Props = $props();
	const initialData = untrack(() => $state.snapshot(data));
	const api: API = getContext("api");

	let orderUsersById = $state<Record<number, UserModel>>({});
	let unresolvedOrderUserIds = $state<Record<number, true>>({});
	let hasLoadError = $state(Boolean(initialData.errorMessage));
	const limitOptions = [10, 20, 50, 100];
	const {
		collection: orders,
		notices,
		sync,
	} = createAdminPaginatedResource<OrderModel>({
		initial: {
			items: initialData.orders,
			page: initialData.orderPage,
			totalPages: initialData.orderTotalPages,
			limit: initialData.orderLimit,
			total: initialData.orderTotal,
		},
		loadErrorMessage: "Unable to load orders.",
		loadPage: async ({ query, page, limit }) => {
			const response = await api.listAdminOrders({
				page,
				limit,
				q: query || undefined,
			});
			hasLoadError = false;
			return response;
		},
		onLoadError: () => {
			hasLoadError = true;
		},
		afterLoad: hydrateOrderUsers,
	});

	function mergeOrderUsers(usersToMerge: UserModel[]) {
		if (usersToMerge.length === 0) {
			return;
		}
		const next = { ...orderUsersById };
		for (const user of usersToMerge) {
			next[user.id] = user;
		}
		orderUsersById = next;
	}

	function getOrderCustomerLabel(order: OrderModel): string {
		if (order.user_id == null) {
			return order.guest_email ? `Guest (${order.guest_email})` : "Guest checkout";
		}
		const user = orderUsersById[order.user_id];
		if (!user) {
			return `Customer #${order.user_id}`;
		}
		if (user.name && user.name.trim().length > 0) {
			return `${user.name} (@${user.username})`;
		}
		return `@${user.username}`;
	}

	async function hydrateOrderUsers(orderList: OrderModel[]) {
		let missing: number[] = [];
		for (const order of orderList) {
			if (order.user_id == null) {
				continue;
			}
			if (
				!orderUsersById[order.user_id] &&
				!unresolvedOrderUserIds[order.user_id] &&
				!missing.includes(order.user_id)
			) {
				missing = [...missing, order.user_id];
			}
		}
		if (missing.length === 0) {
			return;
		}

		let scanPage = 1;
		let scanTotalPages = 1;
		const scanLimit = 100;
		try {
			while (missing.length > 0 && scanPage <= scanTotalPages) {
				const response = await api.listUsers({ page: scanPage, limit: scanLimit });
				mergeOrderUsers(response.data);
				for (const user of response.data) {
					missing = missing.filter((id) => id !== user.id);
				}
				scanTotalPages = Math.max(1, response.pagination.total_pages);
				scanPage += 1;
			}
			if (missing.length > 0) {
				const unresolved = { ...unresolvedOrderUserIds };
				for (const id of missing) {
					unresolved[id] = true;
				}
				unresolvedOrderUserIds = unresolved;
			}
		} catch (error) {
			console.error(error);
		}
	}

	async function updateOrder(orderId: number, status: OrderModel["status"]) {
		notices.clear();
		try {
			const updated = await api.updateOrderStatus(orderId, { status });
			orders.items = replaceItemById(orders.items, updated);
			notices.pushSuccess("Order status updated.");
		} catch (error) {
			console.error(error);
			const err = error as {
				status?: number;
				body?: {
					error?: string;
					product_name?: string;
					available?: number;
					requested?: number;
				};
			};
			if (err.status === 400 && err.body?.error === "Insufficient stock") {
				const productName = err.body.product_name || "A product";
				const available = err.body.available ?? 0;
				const requested = err.body.requested ?? 0;
				notices.pushError(
					`Cannot mark as PAID: ${productName} has ${available} in stock (requested ${requested}).`
				);
				return;
			}
			if (err.status === 400 && err.body?.error) {
				notices.pushError(err.body.error);
				return;
			}
			notices.pushError("Unable to update order.");
		}
	}

	onMount(() => {
		void hydrateOrderUsers(data.orders);
	});

	$effect(() => {
		sync(
			{
				items: data.orders,
				page: data.orderPage,
				totalPages: data.orderTotalPages,
				limit: data.orderLimit,
				total: data.orderTotal,
			},
			data.errorMessage
		);
		hasLoadError = Boolean(data.errorMessage);
	});
</script>

{#snippet orderPanelActions()}
	<AdminResourceActions
		searchPlaceholder="Search ID, user, status, address, item..."
		searchInputClass="w-72"
		bind:searchValue={orders.query}
		onSearch={orders.applySearch}
		onRefresh={orders.refresh}
		searchRefreshing={orders.loading}
		searchDisabled={orders.loading}
	/>
{/snippet}

{#snippet orderHeaderActions()}
	<AdminResourceActions countLabel={`${orders.total} orders`} />
{/snippet}

<section class="space-y-6">
	<AdminPageHeader title="Orders" actions={orderHeaderActions} />

	<AdminPanel
		title="Order Queue"
		meta={`${orders.items.length} shown`}
		headerActions={orderPanelActions}
	>
		{#if hasLoadError}
			<AdminEmptyState tone="error">Failed to load orders.</AdminEmptyState>
		{:else if orders.loading && orders.items.length === 0}
			<AdminEmptyState>Loading orders...</AdminEmptyState>
		{:else if orders.items.length === 0 && orders.hasSearch}
			<AdminEmptyState>No orders match "{orders.query}".</AdminEmptyState>
		{:else if orders.items.length === 0}
			<AdminEmptyState>No orders yet.</AdminEmptyState>
		{:else}
			<div class="space-y-4">
				{#each orders.items as order (order.id)}
					<AdminListItem class="p-4">
						<div class="flex flex-wrap items-start justify-between gap-4">
							<div class="space-y-1">
								<AdminMetaText>Order #{order.id}</AdminMetaText>
								<AdminMetaText tone="strong">{formatPrice(order.total)}</AdminMetaText>
								<AdminMetaText>Placed {formatAdminDateTime(order.created_at)}</AdminMetaText>
								<AdminMetaText>
									{getOrderCustomerLabel(order)} · {order.items.length} item{order.items.length ===
									1
										? ""
										: "s"}
								</AdminMetaText>
								<AdminMetaText>Payment {order.payment_method_display || "N/A"}</AdminMetaText>
								<AdminMetaText>Updated {formatAdminDateTime(order.updated_at)}</AdminMetaText>
							</div>
							<div class="flex flex-col items-end gap-2">
								<Badge tone={getOrderStatusTone(order.status)} size="md">
									{formatOrderStatusLabel(order.status)}
								</Badge>
								<div class="flex flex-wrap justify-end gap-2">
									<Button
										tone="admin"
										variant="regular"
										size="small"
										type="button"
										class="rounded-full"
										onclick={() => updateOrder(order.id, "PENDING")}>Pending</Button
									>
									<Button
										tone="admin"
										variant="regular"
										size="small"
										type="button"
										class="rounded-full"
										onclick={() => updateOrder(order.id, "PAID")}>Paid</Button
									>
									<Button
										tone="admin"
										variant="regular"
										size="small"
										type="button"
										class="rounded-full"
										onclick={() => updateOrder(order.id, "FAILED")}>Failed</Button
									>
									<Button
										tone="admin"
										variant="regular"
										size="small"
										type="button"
										class="rounded-full"
										onclick={() => updateOrder(order.id, "SHIPPED")}>Shipped</Button
									>
									<Button
										tone="admin"
										variant="regular"
										size="small"
										type="button"
										class="rounded-full"
										onclick={() => updateOrder(order.id, "DELIVERED")}>Delivered</Button
									>
									<Button
										tone="admin"
										variant="regular"
										size="small"
										type="button"
										class="rounded-full"
										onclick={() => updateOrder(order.id, "CANCELLED")}>Cancelled</Button
									>
									<Button
										tone="admin"
										variant="regular"
										size="small"
										type="button"
										class="rounded-full"
										onclick={() => updateOrder(order.id, "REFUNDED")}>Refunded</Button
									>
								</div>
							</div>
						</div>
						{#if order.shipping_address_pretty}
							<p class="mt-3 text-xs text-stone-500 dark:text-stone-400">
								Ship to: {order.shipping_address_pretty}
							</p>
						{/if}
						<AdminSurface as="details" variant="muted" class="mt-3">
							<summary
								class="cursor-pointer text-xs font-semibold tracking-[0.08em] text-stone-600 uppercase dark:text-stone-300"
							>
								Order items
							</summary>
							<div class="mt-2 space-y-2">
								{#each order.items as item (item.id)}
									<div
										class="flex flex-wrap items-center justify-between gap-2 text-xs text-stone-700 dark:text-stone-200"
									>
										<p>{item.product.name} ({item.product.sku}) x {item.quantity}</p>
										<p class="font-semibold">{formatPrice(item.price)}</p>
									</div>
								{/each}
							</div>
						</AdminSurface>
					</AdminListItem>
				{/each}

				<AdminPaginationControls
					page={orders.page}
					totalPages={orders.totalPages}
					totalItems={orders.total}
					limit={orders.limit}
					{limitOptions}
					onLimitChange={orders.updateLimit}
					onPrev={() => void orders.changePage(orders.page - 1)}
					onNext={() => void orders.changePage(orders.page + 1)}
				/>
			</div>
		{/if}
	</AdminPanel>
</section>

<AdminFloatingNotices
	statusMessage={notices.message}
	statusTone={notices.tone}
	onDismissStatus={notices.clear}
/>
