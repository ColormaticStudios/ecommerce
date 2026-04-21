<script lang="ts">
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import { getContext, untrack } from "svelte";
	import { type API } from "$lib/api";
	import AdminFloatingNotices from "$lib/admin/AdminFloatingNotices.svelte";
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminListItem from "$lib/admin/AdminListItem.svelte";
	import AdminMasterDetailLayout from "$lib/admin/AdminMasterDetailLayout.svelte";
	import AdminPageHeader from "$lib/admin/AdminPageHeader.svelte";
	import AdminPaginationControls from "$lib/admin/AdminPaginationControls.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import AdminResourceActions from "$lib/admin/AdminResourceActions.svelte";
	import ProductEditor from "$lib/admin/ProductEditor.svelte";
	import {
		createAdminPaginatedResource,
		createAdminSavePrompt,
		removeItemById,
		upsertItemById,
	} from "$lib/admin/state.svelte";
	import ButtonLink from "$lib/components/ButtonLink.svelte";
	import Badge from "$lib/components/Badge.svelte";
	import IconButton from "$lib/components/IconButton.svelte";
	import TabSwitcher from "$lib/components/TabSwitcher.svelte";
	import { type ProductModel } from "$lib/models";
	import { formatPrice } from "$lib/utils";
	import type { PageData } from "./$types";

	interface Props {
		data: PageData;
	}

	let { data }: Props = $props();
	const initialData = untrack(() => $state.snapshot(data));
	const api: API = getContext("api");

	type MobileProductsPanel = "catalog" | "editor";

	let productDirty = $state(false);
	let productSaveAction = $state<(() => Promise<void>) | null>(null);
	let selectedProductId = $state<number | null>(null);
	let mobilePanel = $state<MobileProductsPanel>("catalog");
	let hasLoadError = $state(Boolean(initialData.errorMessage));
	const limitOptions = [10, 20, 50, 100];
	const mobilePanelTabs = [
		{ id: "catalog", label: "Catalog", icon: "bi-collection" },
		{ id: "editor", label: "Editor", icon: "bi-pencil-square" },
	];
	const savePrompt = createAdminSavePrompt({
		onSaveError: () => notices.pushError("Unable to save pending changes."),
		navigationMessage: "You have unsaved product changes. Leave this section and discard them?",
	});
	const {
		collection: catalog,
		notices,
		sync,
	} = createAdminPaginatedResource<ProductModel>({
		initial: {
			items: initialData.products,
			page: initialData.productPage,
			totalPages: initialData.productTotalPages,
			limit: initialData.productLimit,
			total: initialData.productTotal,
		},
		loadErrorMessage: "Unable to load products.",
		loadPage: async ({ query, page, limit }) => {
			const response = await api.listAdminProducts({
				q: query || undefined,
				page,
				limit,
			});
			hasLoadError = false;
			return response;
		},
		onLoadError: () => {
			hasLoadError = true;
		},
	});

	const selectedProduct = $derived(
		selectedProductId ? (catalog.items.find((item) => item.id === selectedProductId) ?? null) : null
	);
	const hasUnsavedChanges = $derived(productDirty);

	function setErrorMessage(message: string) {
		notices.setError(message);
	}

	function setStatusMessage(message: string) {
		notices.setSuccess(message);
	}

	function openEditor(productId: number | null = null) {
		selectedProductId = productId;
		mobilePanel = "editor";
	}

	function handleProductCreated(product: ProductModel) {
		catalog.items = upsertItemById(catalog.items, product);
		selectedProductId = product.id;
		mobilePanel = "editor";
	}

	function handleProductUpdated(updated: ProductModel) {
		catalog.items = upsertItemById(catalog.items, updated);
		selectedProductId = updated.id;
	}

	function handleProductDeleted(productId: number) {
		catalog.items = removeItemById(catalog.items, productId);
		if (selectedProductId === productId) {
			selectedProductId = null;
		}
	}

	$effect(() => {
		sync(
			{
				items: data.products,
				page: data.productPage,
				totalPages: data.productTotalPages,
				limit: data.productLimit,
				total: data.productTotal,
			},
			data.errorMessage
		);
		hasLoadError = Boolean(data.errorMessage);
	});

	$effect(() => {
		savePrompt.dirty = hasUnsavedChanges;
		savePrompt.saveAction = productSaveAction;
	});
</script>

{#snippet catalogActions()}
	<AdminResourceActions
		searchFullWidth={true}
		searchClass="sm:max-w-sm"
		searchPlaceholder="Search products"
		bind:searchValue={catalog.query}
		onSearch={catalog.applySearch}
		onRefresh={catalog.refresh}
		searchRefreshing={catalog.loading}
		searchDisabled={catalog.loading}
	/>
{/snippet}

{#snippet headerActions()}
	<AdminResourceActions countLabel={`${catalog.total} products`} actions={newProductAction} />
{/snippet}

{#snippet newProductAction()}
	<ButtonLink href={resolve("/admin/product/new")} variant="primary" tone="admin">
		New product
	</ButtonLink>
{/snippet}

<section class="space-y-6">
	<AdminPageHeader title="Products" actions={headerActions} />

	<AdminMasterDetailLayout columnsClass="lg:grid-cols-[1.15fr_0.95fr]">
		{#snippet lead()}
			<div class="lg:hidden">
				<TabSwitcher
					items={mobilePanelTabs}
					bind:value={mobilePanel}
					ariaLabel="Product admin panels"
				/>
			</div>
		{/snippet}
		{#snippet master()}
			<AdminPanel
				title="Catalog"
				meta={`${catalog.items.length} shown`}
				headerActions={catalogActions}
				class={`${mobilePanel === "catalog" ? "block" : "hidden"} lg:block`}
			>
				{#if hasLoadError}
					<AdminEmptyState tone="error">Failed to load products.</AdminEmptyState>
				{:else if catalog.loading && catalog.items.length === 0}
					<AdminEmptyState>Loading products...</AdminEmptyState>
				{:else if catalog.items.length === 0 && catalog.hasSearch}
					<AdminEmptyState>Your search didn't match any products.</AdminEmptyState>
				{:else if catalog.items.length === 0}
					<AdminEmptyState>No products yet. Start a new record in the editor.</AdminEmptyState>
				{:else}
					<div class="space-y-3">
						{#each catalog.items as product (product.id)}
							<AdminListItem
								active={selectedProductId === product.id}
								interactive={selectedProductId !== product.id}
								class="flex items-center justify-between gap-3"
							>
								<button
									type="button"
									class="flex flex-1 cursor-pointer items-center justify-between p-4 text-left"
									onclick={() => openEditor(product.id)}
								>
									<div>
										<p class="text-sm font-semibold text-stone-950 dark:text-stone-50">
											{product.name}
										</p>
										<p class="text-xs text-stone-500 dark:text-stone-400">
											SKU {product.sku} · {formatPrice(product.price)}
										</p>
										<div class="mt-2 flex flex-wrap items-center gap-1 text-[10px] font-semibold">
											<Badge tone={product.is_published ? "success" : "warning"}>
												{product.is_published ? "Published" : "Unpublished"}
											</Badge>
											{#if product.has_draft_changes}
												<Badge tone="info">Draft</Badge>
											{/if}
										</div>
									</div>
									<Badge
										tone={product.stock === 0
											? "danger"
											: product.stock <= 5
												? "warning"
												: "success"}
										size="md"
									>
										{product.stock} in stock
									</Badge>
								</button>
								<div class="mr-4 flex items-center gap-2">
									<IconButton
										outlined={true}
										class="border-stone-300 bg-white/85 text-stone-700 hover:bg-stone-100 dark:border-stone-700 dark:bg-stone-950/80 dark:text-stone-200 dark:hover:bg-stone-900"
										size="md"
										aria-label="Open full product editor"
										title="Open full product editor"
										onclick={() => goto(resolve(`/admin/product/${product.id}`))}
									>
										<i class="bi bi-pencil-square"></i>
									</IconButton>
								</div>
							</AdminListItem>
						{/each}

						<AdminPaginationControls
							page={catalog.page}
							totalPages={catalog.totalPages}
							limit={catalog.limit}
							{limitOptions}
							onLimitChange={catalog.updateLimit}
							onPrev={() => void catalog.changePage(catalog.page - 1)}
							onNext={() => void catalog.changePage(catalog.page + 1)}
						/>
					</div>
				{/if}
			</AdminPanel>
		{/snippet}
		{#snippet detail()}
			<div class={`${mobilePanel === "editor" ? "block" : "hidden"} lg:block`}>
				<ProductEditor
					bind:productId={selectedProductId}
					initialProduct={selectedProduct}
					allowCreate={true}
					clearOnDelete={true}
					layout="stacked"
					showMessages={false}
					onErrorMessage={setErrorMessage}
					onStatusMessage={setStatusMessage}
					onDirtyChange={(dirty) => (productDirty = dirty)}
					onSaveRequestChange={(action) => (productSaveAction = action)}
					onProductCreated={handleProductCreated}
					onProductUpdated={handleProductUpdated}
					onProductDeleted={handleProductDeleted}
				/>
			</div>
		{/snippet}
	</AdminMasterDetailLayout>
</section>

<AdminFloatingNotices
	showUnsaved={savePrompt.dirty}
	unsavedMessage="You have unsaved product changes."
	canSaveUnsaved={savePrompt.canSave}
	onSaveUnsaved={() => void savePrompt.save()}
	savingUnsaved={savePrompt.saving}
	statusMessage={notices.message}
	statusTone={notices.tone}
	onDismissStatus={notices.clear}
/>
