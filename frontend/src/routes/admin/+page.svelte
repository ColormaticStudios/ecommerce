<script lang="ts">
	import { type API } from "$lib/api";
	import AdminFloatingNotices from "$lib/admin/AdminFloatingNotices.svelte";
	import AdminPaginationControls from "$lib/admin/AdminPaginationControls.svelte";
	import AdminSearchForm from "$lib/admin/AdminSearchForm.svelte";
	import Button from "$lib/components/Button.svelte";
	import IconButton from "$lib/components/IconButton.svelte";
	import { type OrderModel, type ProductModel, type UserModel } from "$lib/models";
	import { formatPrice } from "$lib/utils";
	import { getContext, onMount } from "svelte";
	import { goto, replaceState } from "$app/navigation";
	import { resolve } from "$app/paths";
	import { page } from "$app/state";
	import ProductEditor from "$lib/admin/ProductEditor.svelte";
	import StorefrontEditor from "$lib/admin/StorefrontEditor.svelte";
	import type { components } from "$lib/api/generated/openapi";
	import type { PageData } from "./$types";

	const api: API = getContext("api");
	type CheckoutPluginCatalog = components["schemas"]["CheckoutPluginCatalog"];
	type CheckoutPlugin = components["schemas"]["CheckoutPlugin"];
	type CheckoutPluginType = CheckoutPlugin["type"];
	type AdminTab = "products" | "orders" | "users" | "providers" | "storefront";
	type NoticeTone = "success" | "error" | null;
	type SaveAction = (() => Promise<void>) | null;
	interface Props {
		data: PageData;
	}
	let { data }: Props = $props();

	let activeTab = $state<AdminTab>("products");
	let isAuthenticated = $state(false);
	let isAdmin = $state(false);
	let accessError = $state("");
	let noticeMessage = $state("");
	let noticeTone = $state<NoticeTone>(null);
	let noticeSaving = $state(false);
	let productDirty = $state(false);
	let storefrontDirty = $state(false);
	let productSaveAction = $state<SaveAction>(null);
	let storefrontSaveAction = $state<SaveAction>(null);
	const tabIndex = $derived.by(() => {
		switch (activeTab) {
			case "products":
				return 0;
			case "orders":
				return 1;
			case "users":
				return 2;
			case "providers":
				return 3;
			default:
				return 4;
		}
	});

	let productQuery = $state("");
	let products = $state<ProductModel[]>([]);
	let productPage = $state(1);
	let productTotalPages = $state(1);
	let productLimit = $state(10);
	let orderLimit = $state(10);
	let userLimit = $state(10);
	const limitOptions = [10, 20, 50, 100];
	let orderQuery = $state("");
	let orderPage = $state(1);
	let orderTotalPages = $state(1);
	let orderTotal = $state(0);
	let userQuery = $state("");
	let userPage = $state(1);
	let userTotalPages = $state(1);
	let userTotal = $state(0);
	let orders = $state<OrderModel[]>([]);
	let users = $state<UserModel[]>([]);
	let orderUsersById = $state<Record<number, UserModel>>({});
	let unresolvedOrderUserIds = $state<Record<number, true>>({});

	let productsLoading = $state(false);
	let ordersLoading = $state(false);
	let usersLoading = $state(false);
	let providersSaving = $state(false);
	let providerCatalog = $state<CheckoutPluginCatalog>({
		payment: [],
		shipping: [],
		tax: [],
	});

	let selectedProductId = $state<number | null>(null);
	const selectedProduct = $derived(
		selectedProductId ? (products.find((item) => item.id === selectedProductId) ?? null) : null
	);
	const hasProductSearch = $derived(productQuery.trim().length > 0);
	const hasOrderSearch = $derived(orderQuery.trim().length > 0);
	const hasUserSearch = $derived(userQuery.trim().length > 0);
	const unsavedContexts = $derived.by(() => {
		const contexts: string[] = [];
		if (productDirty) {
			contexts.push("product");
		}
		if (storefrontDirty) {
			contexts.push("storefront");
		}
		return contexts;
	});
	const hasUnsavedChanges = $derived(unsavedContexts.length > 0);
	const unsavedMessage = $derived.by(() => {
		if (unsavedContexts.length === 0) {
			return "You have unsaved changes.";
		}
		if (unsavedContexts.length === 1) {
			return `You have unsaved ${unsavedContexts[0]} changes.`;
		}
		return "You have unsaved product and storefront changes.";
	});
	const activeSaveAction = $derived.by(() => {
		if (activeTab === "products" && productDirty && productSaveAction) {
			return productSaveAction;
		}
		if (activeTab === "storefront" && storefrontDirty && storefrontSaveAction) {
			return storefrontSaveAction;
		}
		if (productDirty && productSaveAction) {
			return productSaveAction;
		}
		if (storefrontDirty && storefrontSaveAction) {
			return storefrontSaveAction;
		}
		return null;
	});
	const canSaveUnsaved = $derived(activeSaveAction !== null && !noticeSaving);

	function isAdminTab(value: string | null): value is AdminTab {
		return (
			value === "products" ||
			value === "orders" ||
			value === "users" ||
			value === "providers" ||
			value === "storefront"
		);
	}

	function tabFromURL(): AdminTab | null {
		if (typeof window === "undefined") {
			return null;
		}
		const value = new URL(window.location.href).searchParams.get("tab");
		return isAdminTab(value) ? value : null;
	}

	function syncTabToURL(tab: AdminTab) {
		if (typeof window === "undefined") {
			return;
		}
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		replaceState(`${resolve("/admin")}?tab=${tab}`, page.state);
	}

	function setActiveTab(tab: AdminTab, syncURL = true) {
		activeTab = tab;
		if (syncURL) {
			syncTabToURL(tab);
		}
	}

	function clearMessages() {
		noticeMessage = "";
		noticeTone = null;
	}

	function setNotice(tone: Exclude<NoticeTone, null>, message: string) {
		noticeTone = tone;
		noticeMessage = message;
	}

	function toUserDirectory(usersToIndex: UserModel[]): Record<number, UserModel> {
		const index: Record<number, UserModel> = {};
		for (const user of usersToIndex) {
			index[user.id] = user;
		}
		return index;
	}

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
		} catch (err) {
			console.error(err);
		}
	}

	async function loadProducts() {
		productsLoading = true;
		clearMessages();
		try {
			const page = await api.listAdminProducts({
				q: productQuery.trim() || undefined,
				page: productPage,
				limit: productLimit,
			});
			products = page.data;
			productTotalPages = Math.max(1, page.pagination.total_pages);
		} catch (err) {
			console.error(err);
			setNotice("error", "Unable to load products.");
		} finally {
			productsLoading = false;
		}
	}

	async function loadOrders() {
		const nextPage = orderPage;
		const nextLimit = orderLimit;
		ordersLoading = true;
		clearMessages();
		try {
			const response = await api.listAdminOrders({
				page: nextPage,
				limit: nextLimit,
				q: orderQuery.trim() || undefined,
			});
			orders = response.data;
			orderPage = Math.max(1, response.pagination.page);
			orderTotalPages = Math.max(1, response.pagination.total_pages);
			orderTotal = response.pagination.total;
			await hydrateOrderUsers(response.data);
		} catch (err) {
			console.error(err);
			setNotice("error", "Unable to load orders.");
		} finally {
			ordersLoading = false;
		}
	}

	async function loadUsers() {
		const nextPage = userPage;
		const nextLimit = userLimit;
		usersLoading = true;
		clearMessages();
		try {
			const response = await api.listUsers({
				page: nextPage,
				limit: nextLimit,
				q: userQuery.trim() || undefined,
			});
			users = response.data;
			mergeOrderUsers(response.data);
			userPage = Math.max(1, response.pagination.page);
			userTotalPages = Math.max(1, response.pagination.total_pages);
			userTotal = response.pagination.total;
		} catch (err) {
			console.error(err);
			setNotice("error", "Unable to load users.");
		} finally {
			usersLoading = false;
		}
	}

	async function loadProviders() {
		providersSaving = true;
		clearMessages();
		try {
			providerCatalog = await api.listAdminCheckoutPlugins();
		} catch (err) {
			console.error(err);
			setNotice("error", "Unable to load checkout providers.");
		} finally {
			providersSaving = false;
		}
	}

	async function updateProviderEnabled(
		type: CheckoutPluginType,
		providerID: string,
		enabled: boolean
	) {
		if (providersSaving) {
			return;
		}

		providersSaving = true;
		clearMessages();
		try {
			providerCatalog = await api.updateAdminCheckoutPlugin(type, providerID, { enabled });
			setNotice("success", "Provider settings updated.");
		} catch (err) {
			console.error(err);
			const error = err as { body?: { error?: string } };
			setNotice("error", error.body?.error ?? "Unable to update provider settings.");
		} finally {
			providersSaving = false;
		}
	}

	async function changeProductPage(nextPage: number) {
		if (nextPage < 1 || nextPage > productTotalPages || nextPage === productPage) {
			return;
		}
		productPage = nextPage;
		await loadProducts();
	}

	function applyProductSearch() {
		productPage = 1;
		void loadProducts();
	}

	function updateProductLimit(nextLimit: number) {
		productLimit = Number.isNaN(nextLimit) ? 20 : nextLimit;
		productPage = 1;
		void loadProducts();
	}

	async function changeOrderPage(nextPage: number) {
		if (nextPage < 1 || nextPage > orderTotalPages || nextPage === orderPage) {
			return;
		}
		orderPage = nextPage;
		await loadOrders();
	}

	function updateOrderLimit(nextLimit: number) {
		orderLimit = Number.isNaN(nextLimit) ? 20 : nextLimit;
		orderPage = 1;
		void loadOrders();
	}

	function applyOrderSearch() {
		orderPage = 1;
		void loadOrders();
	}

	async function changeUserPage(nextPage: number) {
		if (nextPage < 1 || nextPage > userTotalPages || nextPage === userPage) {
			return;
		}
		userPage = nextPage;
		await loadUsers();
	}

	function updateUserLimit(nextLimit: number) {
		userLimit = Number.isNaN(nextLimit) ? 20 : nextLimit;
		userPage = 1;
		void loadUsers();
	}

	function applyUserSearch() {
		userPage = 1;
		void loadUsers();
	}

	function formatDateTime(value: Date) {
		return value.toLocaleString();
	}

	function handleProductCreated(product: ProductModel) {
		products = [product, ...products];
		selectedProductId = product.id;
	}

	function handleProductUpdated(updated: ProductModel) {
		const exists = products.some((item) => item.id === updated.id);
		products = exists
			? products.map((item) => (item.id === updated.id ? updated : item))
			: [updated, ...products];
	}

	function handleProductDeleted(productId: number) {
		products = products.filter((item) => item.id !== productId);
		if (selectedProductId === productId) {
			selectedProductId = null;
		}
	}

	function setErrorMessage(message: string) {
		if (!message.trim()) {
			if (noticeTone === "error") {
				clearMessages();
			}
			return;
		}
		setNotice("error", message);
	}

	function setStatusMessage(message: string) {
		if (!message.trim()) {
			if (noticeTone === "success") {
				clearMessages();
			}
			return;
		}
		setNotice("success", message);
	}

	function setProductDirty(dirty: boolean) {
		productDirty = dirty;
	}

	function setStorefrontDirty(dirty: boolean) {
		storefrontDirty = dirty;
	}

	function setProductSaveRequest(action: SaveAction) {
		productSaveAction = action;
	}

	function setStorefrontSaveRequest(action: SaveAction) {
		storefrontSaveAction = action;
	}

	async function saveUnsavedChanges() {
		if (!activeSaveAction || noticeSaving) {
			return;
		}
		noticeSaving = true;
		try {
			await activeSaveAction();
		} catch (err) {
			console.error(err);
			setNotice("error", "Unable to save pending changes.");
		} finally {
			noticeSaving = false;
		}
	}

	async function updateOrder(orderId: number, status: OrderModel["status"]) {
		clearMessages();
		try {
			const updated = await api.updateOrderStatus(orderId, { status });
			orders = orders.map((order) => (order.id === updated.id ? updated : order));
			setNotice("success", "Order status updated.");
		} catch (err) {
			console.error(err);
			const error = err as {
				status?: number;
				body?: {
					error?: string;
					product_name?: string;
					available?: number;
					requested?: number;
				};
			};
			if (error.status === 400 && error.body?.error === "Insufficient stock") {
				const productName = error.body.product_name || "A product";
				const available = error.body.available ?? 0;
				const requested = error.body.requested ?? 0;
				setNotice(
					"error",
					`Cannot mark as PAID: ${productName} has ${available} in stock (requested ${requested}).`
				);
				return;
			}
			if (error.status === 400 && error.body?.error) {
				setNotice("error", error.body.error);
				return;
			}
			setNotice("error", "Unable to update order.");
		}
	}

	async function updateRole(userId: number, role: string) {
		clearMessages();
		try {
			const updated = await api.updateUserRole(userId, { role });
			users = users.map((user) => (user.id === updated.id ? updated : user));
			mergeOrderUsers([updated]);
			setNotice("success", "User role updated.");
		} catch (err) {
			console.error(err);
			setNotice("error", "Unable to update role.");
		}
	}

	onMount(() => {
		const initialTab = tabFromURL();
		if (initialTab) {
			setActiveTab(initialTab, false);
		}
		void hydrateOrderUsers(data.orders);

		const handlePopState = () => {
			const nextTab = tabFromURL();
			if (nextTab) {
				setActiveTab(nextTab, false);
			}
		};
		window.addEventListener("popstate", handlePopState);

		return () => {
			window.removeEventListener("popstate", handlePopState);
		};
	});

	$effect(() => {
		activeTab = data.initialTab;
		isAuthenticated = data.isAuthenticated;
		isAdmin = data.isAdmin;
		accessError = data.accessError;
		products = data.products;
		productPage = data.productPage;
		productTotalPages = data.productTotalPages;
		productLimit = data.productLimit;
		orders = data.orders;
		orderPage = data.orderPage;
		orderTotalPages = data.orderTotalPages;
		orderLimit = data.orderLimit;
		orderTotal = data.orderTotal;
		users = data.users;
		orderUsersById = toUserDirectory(data.users);
		userPage = data.userPage;
		userTotalPages = data.userTotalPages;
		userLimit = data.userLimit;
		userTotal = data.userTotal;
		providerCatalog = data.checkoutPlugins ?? { payment: [], shipping: [], tax: [] };
		if (data.errorMessage) {
			noticeTone = "error";
			noticeMessage = data.errorMessage;
		}
	});
</script>

<section class="mx-auto max-w-6xl px-4 py-10">
	{#if !isAuthenticated}
		<div
			class="mt-6 rounded-2xl border border-dashed border-gray-300 bg-white p-6 text-gray-600 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-300"
		>
			<p class="text-lg font-medium">Access denied.</p>
			<p class="mt-2 text-sm">
				You must be signed in to an admin account to access the admin console.
			</p>
		</div>
	{:else if !isAdmin}
		<div
			class="mt-6 rounded-2xl border border-dashed border-gray-300 bg-white p-6 text-gray-600 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-300"
		>
			<p class="text-lg font-medium">Access denied.</p>
			<p class="mt-2 text-sm">
				{accessError || "Contact an administrator if you need access."}
			</p>
		</div>
	{:else}
		<div class="flex flex-wrap items-start justify-between gap-6">
			<div>
				<h1 class="mt-2 text-3xl font-semibold text-gray-900 dark:text-gray-100">Admin</h1>
			</div>
			<div
				class="relative rounded-full border border-gray-200 bg-white p-1 text-xs font-semibold tracking-[0.2em] text-gray-500 uppercase shadow-sm dark:border-gray-800 dark:bg-gray-900 dark:text-gray-400"
			>
				<div
					class="pointer-events-none absolute inset-y-1 left-1 w-[calc((100%-0.5rem)/5)] rounded-full bg-gray-900 transition-transform duration-300 ease-out dark:bg-gray-100"
					style={`transform: translateX(${tabIndex * 100}%);`}
				></div>
				<div class="relative grid grid-cols-5 items-center gap-0">
					<button
						type="button"
						class={`cursor-pointer rounded-full px-4 py-2 transition ${
							activeTab === "products"
								? "text-white dark:text-gray-900"
								: "hover:text-gray-900 dark:hover:text-gray-200"
						}`}
						onclick={() => setActiveTab("products")}
					>
						Products
					</button>
					<button
						type="button"
						class={`cursor-pointer rounded-full px-4 py-2 transition ${
							activeTab === "orders"
								? "text-white dark:text-gray-900"
								: "hover:text-gray-900 dark:hover:text-gray-200"
						}`}
						onclick={() => setActiveTab("orders")}
					>
						Orders
					</button>
					<button
						type="button"
						class={`cursor-pointer rounded-full px-4 py-2 transition ${
							activeTab === "users"
								? "text-white dark:text-gray-900"
								: "hover:text-gray-900 dark:hover:text-gray-200"
						}`}
						onclick={() => setActiveTab("users")}
					>
						Users
					</button>
					<button
						type="button"
						class={`cursor-pointer rounded-full px-4 py-2 transition ${
							activeTab === "providers"
								? "text-white dark:text-gray-900"
								: "hover:text-gray-900 dark:hover:text-gray-200"
						}`}
						onclick={() => setActiveTab("providers")}
					>
						Providers
					</button>
					<button
						type="button"
						class={`cursor-pointer rounded-full px-4 py-2 transition ${
							activeTab === "storefront"
								? "text-white dark:text-gray-900"
								: "hover:text-gray-900 dark:hover:text-gray-200"
						}`}
						onclick={() => setActiveTab("storefront")}
					>
						Storefront
					</button>
				</div>
			</div>
		</div>

		{#if activeTab === "products"}
			<div class="mt-6 grid gap-6 lg:grid-cols-[1.3fr_0.9fr]">
				<div
					class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
				>
					<div class="flex flex-wrap items-center justify-between gap-3">
						<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Catalog</h2>
						<AdminSearchForm
							fullWidth={true}
							class="sm:max-w-xs"
							placeholder="Search products"
							bind:value={productQuery}
							onSearch={applyProductSearch}
							onRefresh={loadProducts}
							refreshing={productsLoading}
							disabled={productsLoading}
						/>
					</div>

					{#if products.length === 0 && hasProductSearch}
						<p class="mt-6 text-sm text-gray-500 dark:text-gray-400">
							Your search didn't match any products.
						</p>
					{:else if products.length === 0}
						<p class="mt-6 text-sm text-gray-500 dark:text-gray-400">
							No products yet. Create your first one in the panel.
						</p>
					{:else}
						<div class="mt-6 space-y-3">
							{#each products as product (product.id)}
								<div
									class={`flex w-full items-center justify-between gap-3 rounded-xl border px-4 py-3 transition ${
										selectedProductId === product.id
											? "border-gray-900 bg-gray-50 shadow-sm dark:border-gray-100 dark:bg-gray-800"
											: "border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50 dark:border-gray-800 dark:bg-gray-900 dark:hover:border-gray-700 dark:hover:bg-gray-800"
									}`}
								>
									<button
										type="button"
										class="flex flex-1 cursor-pointer items-center justify-between text-left"
										onclick={() => (selectedProductId = product.id)}
									>
										<div>
											<p class="text-sm font-semibold text-gray-900 dark:text-gray-100">
												{product.name}
											</p>
											<p class="text-xs text-gray-500 dark:text-gray-400">
												SKU {product.sku} · {formatPrice(product.price)}
											</p>
											<div class="mt-1 flex flex-wrap items-center gap-1 text-[10px] font-semibold">
												<span
													class={`rounded-full px-2 py-0.5 ${
														product.is_published
															? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200"
															: "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200"
													}`}
												>
													{product.is_published ? "Published" : "Unpublished"}
												</span>
												{#if product.has_draft_changes}
													<span
														class="rounded-full bg-blue-100 px-2 py-0.5 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200"
													>
														Draft
													</span>
												{/if}
											</div>
										</div>
										<span
											class={`rounded-full px-3 py-1 text-xs font-semibold ${
												product.stock === 0
													? "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200"
													: product.stock <= 5
														? "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200"
														: "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200"
											}`}
										>
											{product.stock} in stock
										</span>
									</button>
									<IconButton
										outlined={true}
										size="md"
										aria-label="Edit product"
										title="Edit product"
										onclick={() => goto(resolve(`/admin/product/${product.id}`))}
									>
										<i class="bi bi-wrench-adjustable"></i>
									</IconButton>
								</div>
							{/each}
							<AdminPaginationControls
								page={productPage}
								totalPages={productTotalPages}
								limit={productLimit}
								{limitOptions}
								loading={productsLoading}
								onLimitChange={updateProductLimit}
								onPrev={() => void changeProductPage(productPage - 1)}
								onNext={() => void changeProductPage(productPage + 1)}
							/>
						</div>
					{/if}
				</div>

				<ProductEditor
					bind:productId={selectedProductId}
					initialProduct={selectedProduct}
					allowCreate={true}
					clearOnDelete={true}
					layout="stacked"
					showMessages={false}
					onErrorMessage={setErrorMessage}
					onStatusMessage={setStatusMessage}
					onDirtyChange={setProductDirty}
					onSaveRequestChange={setProductSaveRequest}
					onProductCreated={handleProductCreated}
					onProductUpdated={handleProductUpdated}
					onProductDeleted={handleProductDeleted}
				/>
			</div>
		{/if}

		{#if activeTab === "orders"}
			<div
				class="mt-6 rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
			>
				<div class="flex flex-wrap items-center justify-between gap-3">
					<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Orders</h2>
					<div class="flex items-center gap-2">
						<AdminSearchForm
							placeholder="Search ID, user, status, address, item..."
							inputClass="w-72"
							bind:value={orderQuery}
							onSearch={applyOrderSearch}
							onRefresh={loadOrders}
							refreshing={ordersLoading}
							disabled={ordersLoading}
						/>
					</div>
				</div>
				{#if orders.length === 0 && hasOrderSearch}
					<p class="mt-6 text-sm text-gray-500 dark:text-gray-400">
						No orders match "{orderQuery}".
					</p>
				{:else if orders.length === 0}
					<p class="mt-6 text-sm text-gray-500 dark:text-gray-400">No orders yet.</p>
				{:else}
					<div class="mt-6 space-y-4">
						{#each orders as order (order.id)}
							<div class="rounded-xl border border-gray-200 p-4 dark:border-gray-800">
								<div class="flex flex-wrap items-start justify-between gap-4">
									<div class="space-y-1">
										<p class="text-sm text-gray-500 dark:text-gray-400">Order #{order.id}</p>
										<p class="text-lg font-semibold text-gray-900 dark:text-gray-100">
											{formatPrice(order.total)}
										</p>
										<p class="text-xs text-gray-500 dark:text-gray-400">
											Placed {formatDateTime(order.created_at)}
										</p>
										<p class="text-xs text-gray-500 dark:text-gray-400">
											{getOrderCustomerLabel(order)} · {order.items.length} item{order.items
												.length === 1
												? ""
												: "s"}
										</p>
										<p class="text-xs text-gray-500 dark:text-gray-400">
											Payment {order.payment_method_display || "N/A"}
										</p>
										<p class="text-xs text-gray-500 dark:text-gray-400">
											Updated {formatDateTime(order.updated_at)}
										</p>
									</div>
									<div class="flex flex-col items-end gap-2">
										<span
											class={`rounded-full px-3 py-1 text-xs font-semibold ${
												order.status === "PAID"
													? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200"
													: order.status === "SHIPPED"
														? "bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200"
														: order.status === "DELIVERED"
															? "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-200"
															: order.status === "CANCELLED"
																? "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200"
																: order.status === "REFUNDED"
																	? "bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-200"
																	: order.status === "FAILED"
																		? "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200"
																		: "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200"
											}`}
										>
											{order.status}
										</span>
										<div class="flex flex-wrap justify-end gap-2">
											<Button
												variant="regular"
												size="small"
												type="button"
												onclick={() => updateOrder(order.id, "PENDING")}
											>
												Pending
											</Button>
											<Button
												variant="regular"
												size="small"
												type="button"
												onclick={() => updateOrder(order.id, "PAID")}
											>
												Paid
											</Button>
											<Button
												variant="regular"
												size="small"
												type="button"
												onclick={() => updateOrder(order.id, "FAILED")}
											>
												Failed
											</Button>
											<Button
												variant="regular"
												size="small"
												type="button"
												onclick={() => updateOrder(order.id, "SHIPPED")}
											>
												Shipped
											</Button>
											<Button
												variant="regular"
												size="small"
												type="button"
												onclick={() => updateOrder(order.id, "DELIVERED")}
											>
												Delivered
											</Button>
											<Button
												variant="regular"
												size="small"
												type="button"
												onclick={() => updateOrder(order.id, "CANCELLED")}
											>
												Cancelled
											</Button>
											<Button
												variant="regular"
												size="small"
												type="button"
												onclick={() => updateOrder(order.id, "REFUNDED")}
											>
												Refunded
											</Button>
										</div>
									</div>
								</div>
								{#if order.shipping_address_pretty}
									<p class="mt-3 text-xs text-gray-500 dark:text-gray-400">
										Ship to: {order.shipping_address_pretty}
									</p>
								{/if}
								<details class="mt-3 rounded-lg bg-gray-50 p-3 dark:bg-gray-800/50">
									<summary
										class="cursor-pointer text-xs font-semibold tracking-[0.08em] text-gray-600 uppercase dark:text-gray-300"
									>
										Order items
									</summary>
									<div class="mt-2 space-y-2">
										{#each order.items as item (item.id)}
											<div
												class="flex flex-wrap items-center justify-between gap-2 text-xs text-gray-700 dark:text-gray-200"
											>
												<p>
													{item.product.name} ({item.product.sku}) x {item.quantity}
												</p>
												<p class="font-semibold">{formatPrice(item.price)}</p>
											</div>
										{/each}
									</div>
								</details>
							</div>
						{/each}
						<AdminPaginationControls
							page={orderPage}
							totalPages={orderTotalPages}
							totalItems={orderTotal}
							limit={orderLimit}
							{limitOptions}
							onLimitChange={updateOrderLimit}
							onPrev={() => void changeOrderPage(orderPage - 1)}
							onNext={() => void changeOrderPage(orderPage + 1)}
						/>
					</div>
				{/if}
			</div>
		{/if}

		{#if activeTab === "users"}
			<div
				class="mt-6 rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
			>
				<div class="flex flex-wrap items-center justify-between gap-3">
					<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Users</h2>
					<div class="flex items-center gap-2">
						<AdminSearchForm
							placeholder="Search ID, username, email, role..."
							inputClass="w-72"
							bind:value={userQuery}
							onSearch={applyUserSearch}
							onRefresh={loadUsers}
							refreshing={usersLoading}
							disabled={usersLoading}
						/>
					</div>
				</div>
				{#if users.length === 0 && hasUserSearch}
					<p class="mt-6 text-sm text-gray-500 dark:text-gray-400">
						No users match "{userQuery}".
					</p>
				{:else if users.length === 0}
					<p class="mt-6 text-sm text-gray-500 dark:text-gray-400">No users found.</p>
				{:else}
					<div class="mt-6 space-y-4">
						{#each users as user (user.id)}
							<div
								class="flex flex-wrap items-start justify-between gap-4 rounded-xl border border-gray-200 p-4 text-sm dark:border-gray-800"
							>
								<div class="space-y-1">
									<p class="flex items-center gap-2 font-semibold text-gray-900 dark:text-gray-100">
										<span>{user.name || user.username}</span>
										{#if user.role === "admin"}
											<span
												class="inline-flex items-center rounded-full bg-sky-100 px-2 py-0.5 text-[10px] font-semibold tracking-[0.08em] text-sky-700 uppercase dark:bg-sky-900/40 dark:text-sky-200"
												title="Admin"
												aria-label="Admin user"
											>
												<i class="bi bi-shield-fill-check mr-1"></i>
												Admin
											</span>
										{/if}
									</p>
									<p class="text-xs text-gray-500 dark:text-gray-400">
										@{user.username} · {user.email}
									</p>
									<p class="text-xs text-gray-500 dark:text-gray-400">
										ID {user.id} · Currency {user.currency}
									</p>
									<p class="text-xs text-gray-500 dark:text-gray-400">
										Created {formatDateTime(user.created_at)} · Updated {formatDateTime(
											user.updated_at
										)}
									</p>
									<p class="text-xs break-all text-gray-500 dark:text-gray-400">
										Subject {user.subject}
									</p>
									{#if user.deleted_at}
										<p class="text-xs font-semibold text-red-600 dark:text-red-300">
											Deleted {formatDateTime(user.deleted_at)}
										</p>
									{/if}
								</div>
								<div class="flex items-center gap-3">
									<span class="text-xs tracking-[0.2em] text-gray-500 uppercase">Role</span>
									<select
										class="cursor-pointer rounded-md border border-gray-300 bg-gray-100 px-3 py-1 text-sm dark:border-gray-700 dark:bg-gray-800"
										value={user.role}
										onchange={(event) =>
											updateRole(user.id, (event.target as HTMLSelectElement).value)}
									>
										<option value="customer">Customer</option>
										<option value="admin">Admin</option>
									</select>
								</div>
							</div>
						{/each}
						<AdminPaginationControls
							page={userPage}
							totalPages={userTotalPages}
							totalItems={userTotal}
							limit={userLimit}
							{limitOptions}
							onLimitChange={updateUserLimit}
							onPrev={() => void changeUserPage(userPage - 1)}
							onNext={() => void changeUserPage(userPage + 1)}
						/>
					</div>
				{/if}
			</div>
		{/if}

		{#if activeTab === "providers"}
			<div
				class="mt-6 rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
			>
				<div class="flex flex-wrap items-center justify-between gap-3">
					<div>
						<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
							Checkout Providers
						</h2>
						<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
							Enable or disable providers. Tax providers allow only one active provider at a time.
						</p>
					</div>
					<Button
						type="button"
						variant="regular"
						size="small"
						onclick={loadProviders}
						disabled={providersSaving}
					>
						Refresh
					</Button>
				</div>

				<div class="mt-6 grid gap-4 lg:grid-cols-3">
					{#each [{ type: "payment" as const, title: "Payment", plugins: providerCatalog.payment }, { type: "shipping" as const, title: "Shipping", plugins: providerCatalog.shipping }, { type: "tax" as const, title: "Tax", plugins: providerCatalog.tax }] as section (section.type)}
						<div class="rounded-xl border border-gray-200 p-4 dark:border-gray-800">
							<div class="flex items-center justify-between gap-3">
								<h3
									class="text-sm font-semibold tracking-[0.08em] text-gray-700 uppercase dark:text-gray-200"
								>
									{section.title}
								</h3>
								<span class="text-xs text-gray-500 dark:text-gray-400">
									{section.plugins.length} provider{section.plugins.length === 1 ? "" : "s"}
								</span>
							</div>
							<div class="mt-3 space-y-3">
								{#if section.plugins.length === 0}
									<p class="text-xs text-gray-500 dark:text-gray-400">No providers found.</p>
								{:else}
									{#each section.plugins as provider (provider.id)}
										<div class="rounded-lg border border-gray-200 p-3 dark:border-gray-700">
											<div class="flex items-start justify-between gap-3">
												<div>
													<p class="text-sm font-medium text-gray-900 dark:text-gray-100">
														{provider.name}
													</p>
													<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
														{provider.description}
													</p>
												</div>
												<span
													class={`rounded-full px-2 py-0.5 text-[10px] font-semibold tracking-[0.08em] uppercase ${
														provider.enabled
															? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200"
															: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300"
													}`}
												>
													{provider.enabled ? "Enabled" : "Disabled"}
												</span>
											</div>
											<div class="mt-3 flex justify-end">
												{#if section.type === "tax"}
													<Button
														type="button"
														variant="regular"
														size="small"
														onclick={() => updateProviderEnabled("tax", provider.id, true)}
														disabled={providersSaving || provider.enabled}
													>
														{provider.enabled ? "Active Tax Provider" : "Set Active"}
													</Button>
												{:else}
													<Button
														type="button"
														variant="regular"
														size="small"
														onclick={() =>
															updateProviderEnabled(section.type, provider.id, !provider.enabled)}
														disabled={providersSaving}
													>
														{provider.enabled ? "Disable" : "Enable"}
													</Button>
												{/if}
											</div>
										</div>
									{/each}
								{/if}
							</div>
						</div>
					{/each}
				</div>
			</div>
		{/if}

		{#if activeTab === "storefront"}
			<StorefrontEditor
				showInlineMessages={false}
				showInlineUnsavedNotice={false}
				onErrorMessage={setErrorMessage}
				onStatusMessage={setStatusMessage}
				onDirtyChange={setStorefrontDirty}
				onSaveRequestChange={setStorefrontSaveRequest}
			/>
		{/if}
	{/if}
</section>

<AdminFloatingNotices
	showUnsaved={hasUnsavedChanges}
	{unsavedMessage}
	{canSaveUnsaved}
	onSaveUnsaved={saveUnsavedChanges}
	savingUnsaved={noticeSaving}
	statusMessage={noticeMessage}
	statusTone={noticeTone}
	onDismissStatus={clearMessages}
/>
