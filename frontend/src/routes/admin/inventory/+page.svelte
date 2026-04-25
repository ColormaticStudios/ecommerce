<script lang="ts">
	import { getContext, untrack } from "svelte";
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import { resolve } from "$app/paths";
	import type { API } from "$lib/api";
	import type { components } from "$lib/api/generated/openapi";
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminFloatingNotices from "$lib/admin/AdminFloatingNotices.svelte";
	import AdminPageHeader from "$lib/admin/AdminPageHeader.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import AdminProductVariantSelector from "$lib/admin/AdminProductVariantSelector.svelte";
	import AdminTable from "$lib/admin/table/Table.svelte";
	import AdminTableBody from "$lib/admin/table/TableBody.svelte";
	import AdminTableCell from "$lib/admin/table/TableCell.svelte";
	import AdminTableHead from "$lib/admin/table/TableHead.svelte";
	import AdminTableRow from "$lib/admin/table/TableRow.svelte";
	import { createAdminNotices } from "$lib/admin/state.svelte";
	import { searchAdminProducts } from "$lib/admin/productSearch";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import NumberInput from "$lib/components/NumberInput.svelte";
	import type { ProductModel, ProductVariantModel } from "$lib/models";
	import type { PageData } from "./$types";

	interface Props {
		data: PageData;
	}

	type InventoryReservation = components["schemas"]["InventoryReservation"];
	type InventoryAlert = components["schemas"]["InventoryAlert"];
	type ReservationStatus = InventoryReservation["status"];
	type AlertStatus = InventoryAlert["status"];
	type InventoryReservationList = components["schemas"]["InventoryReservationList"];
	type InventoryAlertList = components["schemas"]["InventoryAlertList"];
	type InventoryThresholdList = components["schemas"]["InventoryThresholdList"];
	type InventoryAdjustmentResponse = components["schemas"]["InventoryAdjustmentResponse"];
	type InventoryReconciliationReport = components["schemas"]["InventoryReconciliationReport"];
	type InventoryTimeline = components["schemas"]["InventoryTimeline"];
	type InventoryAdjustmentReason = components["schemas"]["InventoryAdjustmentReason"];

	const statusOptions: ReservationStatus[] = ["ACTIVE", "CONSUMED", "RELEASED", "EXPIRED"];
	const alertStatusOptions: AlertStatus[] = ["OPEN", "ACKED", "RESOLVED"];
	const adjustmentReasons: InventoryAdjustmentReason[] = [
		"CYCLE_COUNT_GAIN",
		"CYCLE_COUNT_LOSS",
		"DAMAGE",
		"SHRINKAGE",
		"RETURN_RESTOCK",
		"CORRECTION",
	];

	let { data }: Props = $props();
	const initialProducts = untrack(() => data.products);
	const api: API = getContext("api");
	const notices = createAdminNotices();

	let reservationsState = $state<InventoryReservationList | null>(null);
	let alertsState = $state<InventoryAlertList | null>(null);
	let thresholdsState = $state<InventoryThresholdList | null>(null);
	let loading = $state(false);
	let alertLoading = $state(false);
	let thresholdSaving = $state(false);
	let selectedStatus = $state<ReservationStatus | "all">("ACTIVE");
	let selectedAlertStatus = $state<AlertStatus | "all">("OPEN");
	let products = $state<ProductModel[]>(initialProducts);
	let productSearch = $state("");
	let productLoading = $state(false);
	let selectedThresholdProductId = $state<number | null>(null);
	let selectedThresholdVariantId = $state<number | null>(null);
	let thresholdQuantity = $state("5");
	let adjustmentDelta = $state("0");
	let adjustmentReason = $state<InventoryAdjustmentReason>("CORRECTION");
	let adjustmentNotes = $state("");
	let adjustmentSaving = $state(false);
	let reconciliationReport = $state<InventoryReconciliationReport | null>(null);
	let timeline = $state<InventoryTimeline | null>(null);
	let diagnosticsLoading = $state(false);

	const reservations = $derived(reservationsState ?? data.reservations ?? { items: [] });
	const alerts = $derived(alertsState ?? data.alerts ?? { items: [] });
	const thresholds = $derived(thresholdsState ?? data.thresholds ?? { items: [] });
	const activeCount = $derived(
		reservations.items.filter((reservation) => reservation.status === "ACTIVE").length
	);
	const expiringSoonCount = $derived(
		reservations.items.filter((reservation) => {
			if (reservation.status !== "ACTIVE") {
				return false;
			}
			return new Date(reservation.expires_at).getTime() - Date.now() <= 5 * 60 * 1000;
		}).length
	);
	const openAlertCount = $derived(alerts.items.filter((alert) => alert.status === "OPEN").length);
	const ackedAlertCount = $derived(alerts.items.filter((alert) => alert.status === "ACKED").length);
	const defaultThreshold = $derived(
		thresholds.items.find((threshold) => threshold.product_variant_id === null) ??
			thresholds.items[0] ??
			null
	);
	const selectedThresholdProduct = $derived(
		selectedThresholdProductId
			? (products.find((product) => product.id === selectedThresholdProductId) ?? null)
			: null
	);
	const selectedThresholdVariant = $derived(
		selectedThresholdVariantId && selectedThresholdProduct
			? (selectedThresholdProduct.variants.find(
					(variant) => variant.id === selectedThresholdVariantId
				) ?? null)
			: null
	);

	$effect(() => {
		if (reservationsState === null) {
			selectedStatus = data.status[0] ?? "ACTIVE";
		}
		if (alertsState === null) {
			selectedAlertStatus = data.alertStatus[0] ?? "OPEN";
		}
	});

	function statusTone(
		status: ReservationStatus
	): "neutral" | "info" | "success" | "warning" | "danger" {
		switch (status) {
			case "ACTIVE":
				return "info";
			case "CONSUMED":
				return "success";
			case "RELEASED":
				return "neutral";
			case "EXPIRED":
				return "warning";
		}
	}

	function formatDate(value: string): string {
		return new Intl.DateTimeFormat(undefined, {
			month: "short",
			day: "numeric",
			hour: "numeric",
			minute: "2-digit",
		}).format(new Date(value));
	}

	function alertTone(alert: InventoryAlert): "neutral" | "info" | "success" | "warning" | "danger" {
		if (alert.status === "RESOLVED" || alert.alert_type === "RECOVERY") {
			return "success";
		}
		if (alert.alert_type === "OUT_OF_STOCK") {
			return "danger";
		}
		if (alert.status === "ACKED") {
			return "warning";
		}
		return "info";
	}

	function reservationSubject(reservation: InventoryReservation): string {
		if (reservation.order_id) {
			return `Order #${reservation.order_id}`;
		}
		if (reservation.checkout_session_id) {
			return `Checkout #${reservation.checkout_session_id}`;
		}
		return reservation.owner_type || "Inventory hold";
	}

	function variantLabel(variant: ProductVariantModel): string {
		const optionLabel = variant.selections
			.map((selection) => `${selection.option_name}: ${selection.option_value}`)
			.join(", ");
		return optionLabel || variant.title || variant.sku;
	}

	function thresholdLabel(productVariantId: number | null | undefined): string {
		if (!productVariantId) {
			return "Default";
		}
		for (const product of products) {
			const variant = product.variants.find((item) => item.id === productVariantId);
			if (variant) {
				return `${product.name} · ${variantLabel(variant)}`;
			}
		}
		return `Variant #${productVariantId}`;
	}

	async function loadReservations(status: ReservationStatus | "all" = selectedStatus) {
		loading = true;
		try {
			const params = status === "all" ? { limit: 100 } : { status: [status], limit: 100 };
			reservationsState = await api.listAdminInventoryReservations(params);
			const url = new URL(page.url);
			url.searchParams.delete("status");
			if (status !== "all") {
				url.searchParams.append("status", status);
			}
			// eslint-disable-next-line svelte/no-navigation-without-resolve
			await goto(`${resolve("/admin/inventory")}${url.search}`, {
				replaceState: true,
				noScroll: true,
				keepFocus: true,
			});
		} catch {
			notices.pushError("Unable to load inventory reservations.");
		} finally {
			loading = false;
		}
	}

	async function loadAlerts(status: AlertStatus | "all" = selectedAlertStatus) {
		alertLoading = true;
		try {
			const params = status === "all" ? { limit: 100 } : { status: [status], limit: 100 };
			alertsState = await api.listAdminInventoryAlerts(params);
		} catch {
			notices.pushError("Unable to load inventory alerts.");
		} finally {
			alertLoading = false;
		}
	}

	function setStatus(status: ReservationStatus | "all") {
		selectedStatus = status;
		void loadReservations(status);
	}

	function setAlertStatus(status: AlertStatus | "all") {
		selectedAlertStatus = status;
		void loadAlerts(status);
	}

	async function searchProducts() {
		productLoading = true;
		try {
			products = await searchAdminProducts(api, productSearch, 5);
		} catch {
			notices.pushError("Unable to search products.");
		} finally {
			productLoading = false;
		}
	}

	function useDefaultThreshold() {
		selectedThresholdProductId = null;
		selectedThresholdVariantId = null;
	}

	async function ackAlert(alert: InventoryAlert) {
		alertLoading = true;
		try {
			const updated = await api.ackAdminInventoryAlert(alert.id);
			alertsState = {
				items: alerts.items.map((item) => (item.id === updated.id ? updated : item)),
			};
			notices.pushSuccess("Inventory alert acknowledged.");
		} catch {
			notices.pushError("Unable to acknowledge inventory alert.");
		} finally {
			alertLoading = false;
		}
	}

	async function resolveAlert(alert: InventoryAlert) {
		alertLoading = true;
		try {
			const updated = await api.resolveAdminInventoryAlert(alert.id);
			alertsState = {
				items: alerts.items.map((item) => (item.id === updated.id ? updated : item)),
			};
			notices.pushSuccess("Inventory alert resolved.");
		} catch {
			notices.pushError("Unable to resolve inventory alert.");
		} finally {
			alertLoading = false;
		}
	}

	async function saveThreshold() {
		const quantity = Number(thresholdQuantity);
		const variantId = selectedThresholdVariantId;
		if (!Number.isInteger(quantity) || quantity < 0) {
			notices.pushError("Threshold must be zero or higher.");
			return;
		}
		if (selectedThresholdProductId !== null && variantId === null) {
			notices.pushError("Select a variant for this threshold.");
			return;
		}
		thresholdSaving = true;
		try {
			const saved = await api.upsertAdminInventoryThreshold({
				product_variant_id: variantId,
				low_stock_quantity: quantity,
			});
			const existing = thresholds.items.some((threshold) => threshold.id === saved.id);
			thresholdsState = {
				items: existing
					? thresholds.items.map((threshold) => (threshold.id === saved.id ? saved : threshold))
					: [saved, ...thresholds.items],
			};
			notices.pushSuccess("Inventory threshold saved.");
		} catch {
			notices.pushError("Unable to save inventory threshold.");
		} finally {
			thresholdSaving = false;
		}
	}

	async function deleteThreshold(id: number) {
		thresholdSaving = true;
		try {
			await api.deleteAdminInventoryThreshold(id);
			thresholdsState = {
				items: thresholds.items.filter((threshold) => threshold.id !== id),
			};
			notices.pushSuccess("Inventory threshold removed.");
		} catch {
			notices.pushError("Unable to remove inventory threshold.");
		} finally {
			thresholdSaving = false;
		}
	}

	async function createAdjustment() {
		if (!selectedThresholdVariantId) {
			notices.pushError("Select a variant to adjust.");
			return;
		}
		const quantityDelta = Number(adjustmentDelta);
		if (!Number.isInteger(quantityDelta) || quantityDelta === 0) {
			notices.pushError("Adjustment quantity must be a non-zero integer.");
			return;
		}
		adjustmentSaving = true;
		try {
			const response: InventoryAdjustmentResponse = await api.createAdminInventoryAdjustment({
				product_variant_id: selectedThresholdVariantId,
				quantity_delta: quantityDelta,
				reason_code: adjustmentReason,
				notes: adjustmentNotes.trim() || undefined,
			});
			adjustmentDelta = "0";
			adjustmentNotes = "";
			notices.pushSuccess(`Inventory adjusted. Available ${response.availability.available}.`);
			await loadTimeline();
		} catch {
			notices.pushError("Unable to create inventory adjustment.");
		} finally {
			adjustmentSaving = false;
		}
	}

	async function runReconciliation() {
		diagnosticsLoading = true;
		try {
			reconciliationReport = await api.runAdminInventoryReconciliation();
			notices.pushSuccess(
				reconciliationReport.issues.length === 0
					? "Inventory reconciliation passed."
					: "Inventory reconciliation found issues."
			);
		} catch {
			notices.pushError("Unable to run inventory reconciliation.");
		} finally {
			diagnosticsLoading = false;
		}
	}

	async function loadTimeline() {
		if (!selectedThresholdVariantId) {
			notices.pushError("Select a variant to inspect.");
			return;
		}
		diagnosticsLoading = true;
		try {
			timeline = await api.getAdminInventoryTimeline(selectedThresholdVariantId, { limit: 20 });
		} catch {
			notices.pushError("Unable to load inventory timeline.");
		} finally {
			diagnosticsLoading = false;
		}
	}
</script>

<svelte:head>
	<title>Inventory | Admin</title>
</svelte:head>

<div class="space-y-6">
	<AdminPageHeader title="Inventory" />

	{#each data.errorMessages as message (message)}
		<div
			class="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-200"
		>
			{message}
		</div>
	{/each}

	{#snippet toolbarActions()}
		<Button tone="admin" variant="regular" onclick={useDefaultThreshold}>Default</Button>
	{/snippet}

	<div class="flex flex-row-reverse flex-wrap gap-2">
		<span
			class="inline-flex items-center gap-2 rounded-full border border-rose-200 bg-rose-50 px-3 py-1.5 text-sm text-rose-800 shadow-sm dark:border-rose-900/70 dark:bg-rose-950/40 dark:text-rose-100"
		>
			<span class="font-medium">Open alerts</span>
			<span class="font-semibold tabular-nums">{openAlertCount}</span>
		</span>
		<span
			class="inline-flex items-center gap-2 rounded-full border border-sky-200 bg-sky-50 px-3 py-1.5 text-sm text-sky-800 shadow-sm dark:border-sky-900/70 dark:bg-sky-950/40 dark:text-sky-100"
		>
			<span class="font-medium">Acked alerts</span>
			<span class="font-semibold tabular-nums">{ackedAlertCount}</span>
		</span>
		<span
			class="inline-flex items-center gap-2 rounded-full border border-stone-200 bg-white px-3 py-1.5 text-sm text-stone-700 shadow-sm dark:border-stone-800 dark:bg-stone-950 dark:text-stone-200"
		>
			<span class="font-medium">Active holds</span>
			<span class="font-semibold text-stone-950 tabular-nums dark:text-stone-50">{activeCount}</span
			>
		</span>
		<span
			class="inline-flex items-center gap-2 rounded-full border border-amber-200 bg-amber-50 px-3 py-1.5 text-sm text-amber-800 shadow-sm dark:border-amber-900/70 dark:bg-amber-950/40 dark:text-amber-100"
		>
			<span class="font-medium">Expiring soon</span>
			<span class="font-semibold tabular-nums">{expiringSoonCount}</span>
		</span>
	</div>

	<AdminPanel title="Alerts" meta={`${alerts.items.length} shown`}>
		{#snippet headerActions()}
			<Button tone="admin" size="small" disabled={alertLoading} onclick={() => loadAlerts()}>
				<i class="bi bi-arrow-clockwise"></i>
				Refresh
			</Button>
		{/snippet}

		<div class="mb-4 flex flex-wrap gap-2">
			<Button
				tone="admin"
				size="small"
				variant={selectedAlertStatus === "all" ? "primary" : "regular"}
				onclick={() => setAlertStatus("all")}
			>
				All
			</Button>
			{#each alertStatusOptions as status (status)}
				<Button
					tone="admin"
					size="small"
					variant={selectedAlertStatus === status ? "primary" : "regular"}
					onclick={() => setAlertStatus(status)}
				>
					{status}
				</Button>
			{/each}
		</div>

		{#if alerts.items.length === 0}
			<AdminEmptyState>No alerts for this view.</AdminEmptyState>
		{:else}
			<AdminTable>
				<AdminTableHead>
					<tr>
						<AdminTableCell header>Alert</AdminTableCell>
						<AdminTableCell header>Status</AdminTableCell>
						<AdminTableCell header>Variant</AdminTableCell>
						<AdminTableCell header align="right">Available</AdminTableCell>
						<AdminTableCell header align="right">Threshold</AdminTableCell>
						<AdminTableCell header>Opened</AdminTableCell>
						<AdminTableCell header align="right">Actions</AdminTableCell>
					</tr>
				</AdminTableHead>
				<AdminTableBody>
					{#each alerts.items as alert (alert.id)}
						<AdminTableRow>
							<AdminTableCell strong>
								<Badge tone={alertTone(alert)}>{alert.alert_type}</Badge>
							</AdminTableCell>
							<AdminTableCell>{alert.status}</AdminTableCell>
							<AdminTableCell>#{alert.product_variant_id}</AdminTableCell>
							<AdminTableCell align="right" numeric>{alert.available}</AdminTableCell>
							<AdminTableCell align="right" numeric>{alert.threshold}</AdminTableCell>
							<AdminTableCell nowrap>{formatDate(alert.opened_at)}</AdminTableCell>
							<AdminTableCell>
								<div class="flex justify-end gap-2">
									{#if alert.status === "OPEN"}
										<Button
											tone="admin"
											size="small"
											disabled={alertLoading}
											onclick={() => ackAlert(alert)}
										>
											Ack
										</Button>
									{/if}
									{#if alert.status !== "RESOLVED"}
										<Button
											tone="admin"
											size="small"
											variant="primary"
											disabled={alertLoading}
											onclick={() => resolveAlert(alert)}
										>
											Resolve
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

	<AdminPanel
		title="Thresholds, adjustments, and diagnostics"
		meta={defaultThreshold ? `Default ${defaultThreshold.low_stock_quantity}` : "No default"}
	>
		<div class="grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(18rem,0.8fr)]">
			<div class="space-y-3">
				<AdminProductVariantSelector
					{products}
					bind:searchQuery={productSearch}
					bind:selectedProductId={selectedThresholdProductId}
					bind:selectedVariantId={selectedThresholdVariantId}
					loading={productLoading}
					onSearch={searchProducts}
					productMeta={(product) => ({
						label: `${product.stock} in stock`,
						tone: product.stock === 0 ? "danger" : product.stock <= 5 ? "warning" : "success",
					})}
					{toolbarActions}
				/>
			</div>

			<div class="space-y-4">
				<div class="space-y-3 rounded-lg border border-stone-200 p-4 dark:border-stone-800">
					<div>
						<p class="text-sm font-semibold text-stone-950 dark:text-stone-50">
							{selectedThresholdVariant
								? variantLabel(selectedThresholdVariant)
								: selectedThresholdProduct
									? "Select a variant"
									: "Default threshold"}
						</p>
						<p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
							{selectedThresholdProduct
								? `${selectedThresholdProduct.name} · ${selectedThresholdVariant?.sku ?? ""}`
								: "Applies when a variant does not have an override."}
						</p>
					</div>
					<label class="block text-sm font-medium text-stone-700 dark:text-stone-200">
						<span class="mb-1 block">Low stock quantity</span>
						<NumberInput tone="admin" min="0" step="1" bind:value={thresholdQuantity} />
					</label>
					<Button tone="admin" variant="primary" disabled={thresholdSaving} onclick={saveThreshold}>
						Save threshold
					</Button>
				</div>

				<div class="space-y-3 rounded-lg border border-stone-200 p-4 dark:border-stone-800">
					<div>
						<p class="text-sm font-semibold text-stone-950 dark:text-stone-50">
							Inventory adjustment
						</p>
						<p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
							{selectedThresholdVariant
								? "Applies to the selected variant."
								: "Select a variant before applying an adjustment."}
						</p>
					</div>
					<label class="block text-sm font-medium text-stone-700 dark:text-stone-200">
						<span class="mb-1 block">Quantity delta</span>
						<NumberInput tone="admin" step="1" bind:value={adjustmentDelta} />
					</label>
					<label class="block text-sm font-medium text-stone-700 dark:text-stone-200">
						<span class="mb-1 block">Reason</span>
						<select
							class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm text-stone-900 transition outline-none focus:border-stone-500 focus:ring-2 focus:ring-stone-200 dark:border-stone-700 dark:bg-stone-900 dark:text-stone-100 dark:focus:border-stone-500 dark:focus:ring-stone-800"
							bind:value={adjustmentReason}
						>
							{#each adjustmentReasons as reason (reason)}
								<option value={reason}>{reason}</option>
							{/each}
						</select>
					</label>
					<textarea
						class="min-h-24 w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm text-stone-900 transition outline-none focus:border-stone-500 focus:ring-2 focus:ring-stone-200 dark:border-stone-700 dark:bg-stone-900 dark:text-stone-100 dark:focus:border-stone-500 dark:focus:ring-stone-800"
						placeholder="Notes"
						bind:value={adjustmentNotes}
					></textarea>
					<div class="flex flex-wrap gap-2">
						<Button
							tone="admin"
							variant="primary"
							disabled={adjustmentSaving}
							onclick={createAdjustment}
						>
							Apply adjustment
						</Button>
						<Button tone="admin" disabled={diagnosticsLoading} onclick={loadTimeline}>
							<i class="bi bi-clock-history"></i>
							Timeline
						</Button>
					</div>
				</div>
			</div>
		</div>

		{#if thresholds.items.length > 0}
			<div class="mt-4 flex flex-wrap gap-2">
				{#each thresholds.items as threshold (threshold.id)}
					<span
						class="inline-flex items-center gap-2 rounded-full border border-stone-200 bg-white px-3 py-1.5 text-sm text-stone-700 dark:border-stone-800 dark:bg-stone-950 dark:text-stone-200"
					>
						<span class="font-medium">
							{thresholdLabel(threshold.product_variant_id)}
						</span>
						<span class="font-semibold tabular-nums">{threshold.low_stock_quantity}</span>
						<button
							type="button"
							class="-mr-1 inline-flex h-5 w-5 items-center justify-center rounded-full text-stone-400 transition hover:bg-stone-100 hover:text-rose-600 disabled:opacity-50 dark:hover:bg-stone-800 dark:hover:text-rose-300"
							aria-label="Remove threshold"
							disabled={thresholdSaving}
							onclick={() => deleteThreshold(threshold.id)}
						>
							<i class="bi bi-x"></i>
						</button>
					</span>
				{/each}
			</div>
		{/if}
		<div class="mt-4 grid gap-4 lg:grid-cols-2">
			<div class="rounded-lg border border-stone-200 p-4 dark:border-stone-800">
				<div class="flex flex-wrap items-center justify-between gap-2">
					<div>
						<p class="text-sm font-semibold text-stone-950 dark:text-stone-50">Reconciliation</p>
						<p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
							{reconciliationReport ? `${reconciliationReport.issues.length} issues` : "No run yet"}
						</p>
					</div>
					<Button tone="admin" disabled={diagnosticsLoading} onclick={runReconciliation}>Run</Button
					>
				</div>
				{#if reconciliationReport}
					{#if reconciliationReport.issues.length === 0}
						<p class="mt-3 text-sm text-stone-600 dark:text-stone-300">No drift detected.</p>
					{:else}
						<div class="mt-3 max-h-56 space-y-2 overflow-y-auto pr-1">
							{#each reconciliationReport.issues as issue, index (`${issue.issue_type}-${issue.entity_type}-${issue.entity_id ?? index}`)}
								<div
									class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-900 dark:border-amber-900/70 dark:bg-amber-950/40 dark:text-amber-100"
								>
									<p class="font-semibold">{issue.issue_type}</p>
									<p class="mt-1 text-xs">
										Variant #{issue.product_variant_id} · expected {issue.expected}, actual
										{issue.actual}
									</p>
								</div>
							{/each}
						</div>
					{/if}
				{/if}
			</div>

			<div class="rounded-lg border border-stone-200 p-4 dark:border-stone-800">
				<p class="text-sm font-semibold text-stone-950 dark:text-stone-50">Timeline</p>
				{#if !timeline}
					<p class="mt-2 text-sm text-stone-500 dark:text-stone-400">
						Load a variant timeline to inspect recent movements, reservations, and adjustments.
					</p>
				{:else}
					<div class="mt-3 max-h-72 space-y-2 overflow-y-auto pr-1">
						{#each timeline.adjustments as adjustment (adjustment.id)}
							<div
								class="rounded-lg border border-stone-200 px-3 py-2 text-sm dark:border-stone-800"
							>
								<p class="font-semibold text-stone-950 dark:text-stone-50">
									Adjustment {adjustment.quantity_delta}
								</p>
								<p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
									{adjustment.reason_code} · {formatDate(adjustment.created_at)}
								</p>
							</div>
						{/each}
						{#each timeline.movements as movement (movement.id)}
							<div
								class="rounded-lg border border-stone-200 px-3 py-2 text-sm dark:border-stone-800"
							>
								<p class="font-semibold text-stone-950 dark:text-stone-50">
									{movement.movement_type}
									{movement.quantity_delta}
								</p>
								<p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
									{movement.reason_code || movement.reference_type || "movement"} ·
									{formatDate(movement.created_at)}
								</p>
							</div>
						{/each}
						{#if timeline.adjustments.length === 0 && timeline.movements.length === 0}
							<p class="text-sm text-stone-500 dark:text-stone-400">No timeline events yet.</p>
						{/if}
					</div>
				{/if}
			</div>
		</div>
	</AdminPanel>

	<AdminPanel title="Reservations" meta={`${reservations.items.length} shown`}>
		{#snippet headerActions()}
			<div class="flex flex-wrap items-center gap-2">
				<Button tone="admin" size="small" disabled={loading} onclick={() => loadReservations()}>
					<i class="bi bi-arrow-clockwise"></i>
					Refresh
				</Button>
			</div>
		{/snippet}

		<div class="mb-4 flex flex-wrap gap-2">
			<Button
				tone="admin"
				size="small"
				variant={selectedStatus === "all" ? "primary" : "regular"}
				onclick={() => setStatus("all")}
			>
				All
			</Button>
			{#each statusOptions as status (status)}
				<Button
					tone="admin"
					size="small"
					variant={selectedStatus === status ? "primary" : "regular"}
					onclick={() => setStatus(status)}
				>
					{status}
				</Button>
			{/each}
		</div>

		{#if reservations.items.length === 0}
			<AdminEmptyState>No reservations for this view.</AdminEmptyState>
		{:else}
			<AdminTable>
				<AdminTableHead>
					<tr>
						<AdminTableCell header>Reservation</AdminTableCell>
						<AdminTableCell header>Status</AdminTableCell>
						<AdminTableCell header>Variant</AdminTableCell>
						<AdminTableCell header align="right">Qty</AdminTableCell>
						<AdminTableCell header>Expires</AdminTableCell>
						<AdminTableCell header>Owner</AdminTableCell>
					</tr>
				</AdminTableHead>
				<AdminTableBody>
					{#each reservations.items as reservation (reservation.id)}
						<AdminTableRow>
							<AdminTableCell strong>#{reservation.id}</AdminTableCell>
							<AdminTableCell>
								<Badge tone={statusTone(reservation.status)}>{reservation.status}</Badge>
							</AdminTableCell>
							<AdminTableCell>#{reservation.product_variant_id}</AdminTableCell>
							<AdminTableCell align="right" numeric>{reservation.quantity}</AdminTableCell>
							<AdminTableCell nowrap>{formatDate(reservation.expires_at)}</AdminTableCell>
							<AdminTableCell>{reservationSubject(reservation)}</AdminTableCell>
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
