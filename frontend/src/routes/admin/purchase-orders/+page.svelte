<script lang="ts">
	import { getContext, untrack } from "svelte";
	import type { API } from "$lib/api";
	import type { components } from "$lib/api/generated/openapi";
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminFloatingNotices from "$lib/admin/AdminFloatingNotices.svelte";
	import AdminPageHeader from "$lib/admin/AdminPageHeader.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import AdminProductVariantSelector from "$lib/admin/AdminProductVariantSelector.svelte";
	import { searchAdminProducts } from "$lib/admin/productSearch";
	import { createAdminNotices } from "$lib/admin/state.svelte";
	import AdminTable from "$lib/admin/table/Table.svelte";
	import AdminTableBody from "$lib/admin/table/TableBody.svelte";
	import AdminTableCell from "$lib/admin/table/TableCell.svelte";
	import AdminTableHead from "$lib/admin/table/TableHead.svelte";
	import AdminTableRow from "$lib/admin/table/TableRow.svelte";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import NumberInput from "$lib/components/NumberInput.svelte";
	import type { ProductModel, ProductVariantModel } from "$lib/models";
	import type { PageData } from "./$types";

	interface Props {
		data: PageData;
	}

	type PurchaseOrder = components["schemas"]["PurchaseOrder"];
	type PurchaseOrderItem = components["schemas"]["PurchaseOrderItem"];

	let { data }: Props = $props();
	const api: API = getContext("api");
	const notices = createAdminNotices();
	const initialProducts = untrack(() => data.products);
	const initialPurchaseOrders = untrack(() => data.purchaseOrders.items);

	let purchaseOrders = $state<PurchaseOrder[]>(initialPurchaseOrders);
	let products = $state<ProductModel[]>(initialProducts);
	let productSearch = $state("");
	let supplierName = $state("");
	let notes = $state("");
	let selectedProductId = $state<number | null>(null);
	let selectedVariantId = $state<number | null>(null);
	let quantity = $state("1");
	let saving = $state(false);
	let loadingProducts = $state(false);
	let receiveQuantities = $state<Record<number, string>>({});

	const selectedProduct = $derived(
		selectedProductId
			? (products.find((product) => product.id === selectedProductId) ?? null)
			: null
	);
	const selectedVariant = $derived(
		selectedProduct && selectedVariantId
			? (selectedProduct.variants.find((variant) => variant.id === selectedVariantId) ?? null)
			: null
	);

	function statusTone(status: PurchaseOrder["status"]) {
		switch (status) {
			case "DRAFT":
				return "neutral";
			case "ISSUED":
				return "info";
			case "PARTIALLY_RECEIVED":
				return "warning";
			case "RECEIVED":
				return "success";
			case "CANCELLED":
				return "danger";
		}
	}

	function variantLabel(variant: ProductVariantModel): string {
		const optionLabel = variant.selections
			.map((selection) => `${selection.option_name}: ${selection.option_value}`)
			.join(", ");
		return optionLabel || variant.title || variant.sku;
	}

	function productVariantLabel(productVariantId: number): string {
		for (const product of products) {
			const variant = product.variants.find((item) => item.id === productVariantId);
			if (variant) {
				return `${product.name} · ${variantLabel(variant)}`;
			}
		}
		return `Variant #${productVariantId}`;
	}

	function openQuantity(item: PurchaseOrderItem): number {
		return item.quantity_ordered - item.quantity_received;
	}

	async function searchProducts() {
		loadingProducts = true;
		try {
			products = await searchAdminProducts(api, productSearch, 20);
		} catch {
			notices.pushError("Unable to search products.");
		} finally {
			loadingProducts = false;
		}
	}

	async function createPurchaseOrder() {
		if (!selectedVariant) {
			notices.pushError("Select a variant.");
			return;
		}
		const qty = Number(quantity);
		if (!Number.isInteger(qty) || qty < 1) {
			notices.pushError("Quantity must be positive.");
			return;
		}
		saving = true;
		try {
			const created = await api.createAdminPurchaseOrder({
				supplier: supplierName.trim() ? { name: supplierName.trim() } : undefined,
				notes: notes.trim() || undefined,
				items: [{ product_variant_id: selectedVariant.id ?? 0, quantity_ordered: qty }],
			});
			purchaseOrders = [created, ...purchaseOrders];
			notices.pushSuccess("Purchase order created.");
		} catch {
			notices.pushError("Unable to create purchase order.");
		} finally {
			saving = false;
		}
	}

	function updateOrder(updated: PurchaseOrder) {
		purchaseOrders = purchaseOrders.map((order) => (order.id === updated.id ? updated : order));
	}

	async function issue(order: PurchaseOrder) {
		saving = true;
		try {
			updateOrder(await api.issueAdminPurchaseOrder(order.id));
			notices.pushSuccess("Purchase order issued.");
		} catch {
			notices.pushError("Unable to issue purchase order.");
		} finally {
			saving = false;
		}
	}

	async function cancel(order: PurchaseOrder) {
		saving = true;
		try {
			updateOrder(await api.cancelAdminPurchaseOrder(order.id));
			notices.pushSuccess("Purchase order cancelled.");
		} catch {
			notices.pushError("Unable to cancel purchase order.");
		} finally {
			saving = false;
		}
	}

	async function receive(order: PurchaseOrder) {
		const items = order.items
			.map((item) => ({
				purchase_order_item_id: item.id,
				quantity_received: Number(receiveQuantities[item.id] || 0),
			}))
			.filter((item) => item.quantity_received > 0);
		if (items.length === 0) {
			notices.pushError("Enter a received quantity.");
			return;
		}
		saving = true;
		try {
			const response = await api.receiveAdminPurchaseOrder(order.id, { items });
			updateOrder(response.purchase_order);
			receiveQuantities = {};
			notices.pushSuccess("Inventory received.");
		} catch {
			notices.pushError("Unable to receive purchase order.");
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head>
	<title>Purchase Orders | Admin</title>
</svelte:head>

<div class="space-y-6">
	<AdminPageHeader title="Purchase Orders" />

	{#each data.errorMessages as message (message)}
		<div
			class="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-200"
		>
			{message}
		</div>
	{/each}

	<AdminPanel title="New purchase order">
		<div class="grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(18rem,0.8fr)]">
			<div class="space-y-3">
				<AdminProductVariantSelector
					{products}
					bind:searchQuery={productSearch}
					bind:selectedProductId
					bind:selectedVariantId
					loading={loadingProducts}
					onSearch={searchProducts}
				/>
			</div>
			<div class="space-y-3 rounded-lg border border-stone-200 p-4 dark:border-stone-800">
				<input
					class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm text-stone-900 transition outline-none focus:border-stone-500 focus:ring-2 focus:ring-stone-200 dark:border-stone-700 dark:bg-stone-900 dark:text-stone-100 dark:focus:border-stone-500 dark:focus:ring-stone-800"
					placeholder="Supplier name"
					bind:value={supplierName}
				/>
				<input
					class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm text-stone-900 transition outline-none focus:border-stone-500 focus:ring-2 focus:ring-stone-200 dark:border-stone-700 dark:bg-stone-900 dark:text-stone-100 dark:focus:border-stone-500 dark:focus:ring-stone-800"
					placeholder="Notes"
					bind:value={notes}
				/>
				<NumberInput tone="admin" min="1" step="1" bind:value={quantity} />
				<p class="text-sm font-medium text-stone-900 dark:text-stone-100">
					{selectedVariant ? variantLabel(selectedVariant) : "No variant selected"}
				</p>
				<Button tone="admin" variant="primary" disabled={saving} onclick={createPurchaseOrder}>
					Create draft
				</Button>
			</div>
		</div>
	</AdminPanel>

	<AdminPanel title="Purchase orders" meta={`${purchaseOrders.length} shown`}>
		{#if purchaseOrders.length === 0}
			<AdminEmptyState>No purchase orders yet.</AdminEmptyState>
		{:else}
			<AdminTable>
				<AdminTableHead>
					<tr>
						<AdminTableCell header>PO</AdminTableCell>
						<AdminTableCell header>Status</AdminTableCell>
						<AdminTableCell header>Supplier</AdminTableCell>
						<AdminTableCell header>Items</AdminTableCell>
						<AdminTableCell header align="right">Actions</AdminTableCell>
					</tr>
				</AdminTableHead>
				<AdminTableBody>
					{#each purchaseOrders as order (order.id)}
						<AdminTableRow>
							<AdminTableCell strong>#{order.id}</AdminTableCell>
							<AdminTableCell
								><Badge tone={statusTone(order.status)}>{order.status}</Badge></AdminTableCell
							>
							<AdminTableCell>{order.supplier?.name ?? "Unassigned"}</AdminTableCell>
							<AdminTableCell>
								<div class="space-y-2">
									{#each order.items as item (item.id)}
										<div class="flex flex-wrap items-center gap-2">
											<span>{productVariantLabel(item.product_variant_id)}</span>
											<span class="text-stone-500">
												{item.quantity_received}/{item.quantity_ordered}
											</span>
											{#if order.status === "ISSUED" || order.status === "PARTIALLY_RECEIVED"}
												<NumberInput
													tone="admin"
													full={false}
													class="w-20"
													min="0"
													max={openQuantity(item)}
													step="1"
													placeholder={`${openQuantity(item)}`}
													bind:value={receiveQuantities[item.id]}
												/>
											{/if}
										</div>
									{/each}
								</div>
							</AdminTableCell>
							<AdminTableCell>
								<div class="flex justify-end gap-2">
									{#if order.status === "DRAFT"}
										<Button
											tone="admin"
											size="small"
											disabled={saving}
											onclick={() => issue(order)}
										>
											Issue
										</Button>
										<Button
											tone="admin"
											size="small"
											disabled={saving}
											onclick={() => cancel(order)}
										>
											Cancel
										</Button>
									{/if}
									{#if order.status === "ISSUED" || order.status === "PARTIALLY_RECEIVED"}
										<Button
											tone="admin"
											size="small"
											variant="primary"
											disabled={saving}
											onclick={() => receive(order)}
										>
											Receive
										</Button>
									{/if}
								</div>
							</AdminTableCell>
						</AdminTableRow>
					{/each}
				</AdminTableBody>
			</AdminTable>
		{/if}
	</AdminPanel>
</div>

<AdminFloatingNotices
	statusMessage={notices.message}
	statusTone={notices.tone}
	onDismissStatus={notices.clear}
/>
