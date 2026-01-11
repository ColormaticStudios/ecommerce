<script lang="ts">
	import { type API } from "$lib/api";
	import { checkAdminAccess } from "$lib/admin/auth";
	import Alert from "$lib/components/alert.svelte";
	import { type OrderModel, type ProductModel, type UserModel } from "$lib/models";
	import { formatPrice } from "$lib/utils";
	import { getContext, onMount } from "svelte";
	import { resolve } from "$app/paths";
	import ProductEditor from "$lib/admin/ProductEditor.svelte";

	const api: API = getContext("api");

	let activeTab = $state<"products" | "orders" | "users">("products");
	let authChecked = $state(false);
	let loading = $state(true);
	let isAdmin = $state(false);
	let errorMessage = $state("");
	let statusMessage = $state("");
	const tabIndex = $derived(activeTab === "products" ? 0 : activeTab === "orders" ? 1 : 2);

	let productQuery = $state("");
	let products = $state<ProductModel[]>([]);
	let productsLoaded = $state(false);
	let productPage = $state(1);
	let productTotalPages = $state(1);
	let productLimit = $state(20);
	const productLimitOptions = [10, 20, 50, 100];
	let orders = $state<OrderModel[]>([]);
	let users = $state<UserModel[]>([]);

	let productsLoading = $state(false);
	let ordersLoading = $state(false);
	let usersLoading = $state(false);

	let selectedProductId = $state<number | null>(null);
	const selectedProduct = $derived(
		selectedProductId ? (products.find((item) => item.id === selectedProductId) ?? null) : null
	);
	const hasProductSearch = $derived(productQuery.trim().length > 0);

	function clearMessages() {
		errorMessage = "";
		statusMessage = "";
	}

	async function loadProducts() {
		productsLoading = true;
		productsLoaded = false;
		clearMessages();
		try {
			const page = await api.listProducts({
				q: productQuery.trim() || undefined,
				page: productPage,
				limit: productLimit,
			});
			products = page.data;
			productTotalPages = Math.max(1, page.pagination.total_pages);
		} catch (err) {
			console.error(err);
			errorMessage = "Unable to load products.";
		} finally {
			productsLoaded = true;
			productsLoading = false;
		}
	}

	async function loadOrders() {
		ordersLoading = true;
		clearMessages();
		try {
			orders = await api.listAdminOrders({ page: 1, limit: 50 });
		} catch (err) {
			console.error(err);
			errorMessage = "Unable to load orders.";
		} finally {
			ordersLoading = false;
		}
	}

	async function loadUsers() {
		usersLoading = true;
		clearMessages();
		try {
			users = await api.listUsers({ page: 1, limit: 50 });
		} catch (err) {
			console.error(err);
			errorMessage = "Unable to load users.";
		} finally {
			usersLoading = false;
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

	function updateProductLimit(event: Event) {
		const target = event.target as HTMLSelectElement;
		const nextLimit = Number(target.value);
		productLimit = Number.isNaN(nextLimit) ? 20 : nextLimit;
		productPage = 1;
		void loadProducts();
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
		errorMessage = message;
	}

	function setStatusMessage(message: string) {
		statusMessage = message;
	}

	async function updateOrder(orderId: number, status: OrderModel["status"]) {
		clearMessages();
		try {
			const updated = await api.updateOrderStatus(orderId, { status });
			orders = orders.map((order) => (order.id === updated.id ? updated : order));
			statusMessage = "Order status updated.";
		} catch (err) {
			console.error(err);
			errorMessage = "Unable to update order.";
		}
	}

	async function updateRole(userId: number, role: string) {
		clearMessages();
		try {
			const updated = await api.updateUserRole(userId, { role });
			users = users.map((user) => (user.id === updated.id ? updated : user));
			statusMessage = "User role updated.";
		} catch (err) {
			console.error(err);
			errorMessage = "Unable to update role.";
		}
	}

	onMount(async () => {
		authChecked = true;
		const result = await checkAdminAccess(api);
		isAdmin = result.isAdmin;
		if (isAdmin) {
			await Promise.all([loadProducts(), loadOrders(), loadUsers()]);
		}
		loading = false;
	});

	$effect(() => {
		if (!isAdmin) {
			return;
		}
		if (activeTab === "products" && !productsLoaded && !productsLoading) {
			void loadProducts();
		}
	});
</script>

<section class="mx-auto max-w-6xl px-4 py-10">
	<div class="flex flex-wrap items-start justify-between gap-6">
		<div>
			<h1 class="mt-2 text-3xl font-semibold text-gray-900 dark:text-gray-100">Admin</h1>
		</div>
		<div
			class="relative rounded-full border border-gray-200 bg-white p-1 text-xs font-semibold tracking-[0.2em] text-gray-500 uppercase shadow-sm dark:border-gray-800 dark:bg-gray-900 dark:text-gray-400"
		>
			<div
				class="pointer-events-none absolute inset-y-1 left-1 w-[calc((100%-0.5rem)/3)] rounded-full bg-gray-900 transition-transform duration-300 ease-out dark:bg-gray-100"
				style={`transform: translateX(${tabIndex * 100}%);`}
			></div>
			<div class="relative grid grid-cols-3 items-center gap-0">
				<button
					type="button"
					class={`cursor-pointer rounded-full px-4 py-2 transition ${
						activeTab === "products"
							? "text-white dark:text-gray-900"
							: "hover:text-gray-900 dark:hover:text-gray-200"
					}`}
					onclick={() => (activeTab = "products")}
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
					onclick={() => (activeTab = "orders")}
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
					onclick={() => (activeTab = "users")}
				>
					Users
				</button>
			</div>
		</div>
	</div>

	{#if !authChecked}
		<p class="mt-6 text-sm text-gray-500 dark:text-gray-400">Checking your access…</p>
	{:else if !api.isAuthenticated()}
		<p class="mt-6 text-gray-600 dark:text-gray-300">
			Please
			<a href={resolve("/login")} class="text-blue-600 hover:underline dark:text-blue-400">
				log in
			</a>
			to access the admin console.
		</p>
	{:else if loading}
		<p class="mt-6 text-sm text-gray-500 dark:text-gray-400">Loading admin data…</p>
	{:else if !isAdmin}
		<div
			class="mt-6 rounded-2xl border border-dashed border-gray-300 bg-white p-6 text-gray-600 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-300"
		>
			<p class="text-lg font-medium">You do not have access to the admin console.</p>
			<p class="mt-2 text-sm">Contact an administrator if you need access.</p>
		</div>
	{:else}
		{#if errorMessage}
			<div class="mt-6">
				<Alert
					message={errorMessage}
					tone="error"
					icon="bi-x-circle-fill"
					onClose={() => (errorMessage = "")}
				/>
			</div>
		{/if}
		{#if statusMessage}
			<div class="mt-6">
				<Alert
					message={statusMessage}
					tone="success"
					icon="bi-check-circle-fill"
					onClose={() => (statusMessage = "")}
				/>
			</div>
		{/if}

		{#if activeTab === "products"}
			<div class="mt-6 grid gap-6 lg:grid-cols-[1.3fr_0.9fr]">
				<div
					class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
				>
					<div class="flex flex-wrap items-center justify-between gap-3">
						<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Catalog</h2>
						<form
							class="flex flex-1 items-center gap-2 sm:max-w-xs"
							onsubmit={(event) => {
								event.preventDefault();
								applyProductSearch();
							}}
						>
							<input
								class="textinput"
								type="search"
								placeholder="Search products"
								bind:value={productQuery}
							/>
							<button
								class="aspect-square cursor-pointer rounded-md border border-gray-200 p-2 dark:border-gray-700"
								type="submit"
								aria-label="Search"
							>
								<i class="bi bi-search"></i>
							</button>
						</form>
					</div>

					{#if productsLoading}
						<div class="mt-6 space-y-3">
							<div class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-gray-800"></div>
							<div class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-gray-800"></div>
							<div class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-gray-800"></div>
						</div>
					{:else if products.length === 0 && hasProductSearch}
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
									<a
										href={resolve(`/admin/product/${product.id}`)}
										class="flex h-10 w-10 cursor-pointer items-center justify-center rounded-full border border-gray-200 text-gray-600 transition hover:bg-gray-100 dark:border-gray-700 dark:text-gray-200 dark:hover:border-gray-600 dark:hover:bg-gray-700"
										aria-label="Edit product"
									>
										<i class="bi bi-wrench-adjustable"></i>
									</a>
								</div>
							{/each}
							<div
								class="flex flex-wrap items-center justify-between gap-3 pt-2 text-xs text-gray-500 dark:text-gray-400"
							>
								<div class="flex items-center gap-2">
									<span>Per page</span>
									<select
										class="cursor-pointer rounded-md border border-gray-300 bg-gray-100 px-2 py-1 text-xs dark:border-gray-700 dark:bg-gray-800"
										value={productLimit}
										onchange={updateProductLimit}
									>
										{#each productLimitOptions as option, i (i)}
											<option value={option}>{option}</option>
										{/each}
									</select>
								</div>
								<span>
									Page {productPage} of {productTotalPages}
								</span>
								<div class="flex items-center gap-2">
									<button
										class="btn btn-regular cursor-pointer"
										type="button"
										disabled={productPage <= 1}
										onclick={() => changeProductPage(productPage - 1)}
									>
										Prev
									</button>
									<button
										class="btn btn-regular cursor-pointer"
										type="button"
										disabled={productPage >= productTotalPages}
										onclick={() => changeProductPage(productPage + 1)}
									>
										Next
									</button>
								</div>
							</div>
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
				<div class="flex items-center justify-between">
					<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Orders</h2>
					<button
						class="btn btn-regular cursor-pointer"
						type="button"
						onclick={loadOrders}
						disabled={ordersLoading}
					>
						Refresh
					</button>
				</div>
				{#if ordersLoading}
					<div class="mt-6 space-y-3">
						<div class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-gray-800"></div>
						<div class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-gray-800"></div>
						<div class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-gray-800"></div>
					</div>
				{:else if orders.length === 0}
					<p class="mt-6 text-sm text-gray-500 dark:text-gray-400">No orders yet.</p>
				{:else}
					<div class="mt-6 space-y-4">
						{#each orders as order (order.id)}
							<div class="rounded-xl border border-gray-200 p-4 dark:border-gray-800">
								<div class="flex flex-wrap items-start justify-between gap-4">
									<div>
										<p class="text-sm text-gray-500 dark:text-gray-400">Order #{order.id}</p>
										<p class="text-lg font-semibold text-gray-900 dark:text-gray-100">
											{formatPrice(order.total)}
										</p>
										<p class="text-xs text-gray-500 dark:text-gray-400">
											{order.created_at.toLocaleDateString()}
										</p>
									</div>
									<div class="flex flex-col items-end gap-2">
										<span
											class={`rounded-full px-3 py-1 text-xs font-semibold ${
												order.status === "PAID"
													? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200"
													: order.status === "FAILED"
														? "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200"
														: "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200"
											}`}
										>
											{order.status}
										</span>
										<div class="flex gap-2">
											<button
												class="btn btn-regular cursor-pointer"
												type="button"
												onclick={() => updateOrder(order.id, "PENDING")}
											>
												Pending
											</button>
											<button
												class="btn btn-regular cursor-pointer"
												type="button"
												onclick={() => updateOrder(order.id, "PAID")}
											>
												Paid
											</button>
											<button
												class="btn btn-regular cursor-pointer"
												type="button"
												onclick={() => updateOrder(order.id, "FAILED")}
											>
												Failed
											</button>
										</div>
									</div>
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</div>
		{/if}

		{#if activeTab === "users"}
			<div
				class="mt-6 rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
			>
				<div class="flex items-center justify-between">
					<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Users</h2>
					<button
						class="btn btn-regular cursor-pointer"
						type="button"
						onclick={loadUsers}
						disabled={usersLoading}
					>
						Refresh
					</button>
				</div>
				{#if usersLoading}
					<div class="mt-6 space-y-3">
						<div class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-gray-800"></div>
						<div class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-gray-800"></div>
						<div class="h-10 animate-pulse rounded-lg bg-gray-100 dark:bg-gray-800"></div>
					</div>
				{:else if users.length === 0}
					<p class="mt-6 text-sm text-gray-500 dark:text-gray-400">No users found.</p>
				{:else}
					<div class="mt-6 space-y-4">
						{#each users as user (user.id)}
							<div
								class="flex flex-wrap items-center justify-between gap-4 rounded-xl border border-gray-200 p-4 text-sm dark:border-gray-800"
							>
								<div>
									<p class="font-semibold text-gray-900 dark:text-gray-100">
										{user.name || user.username}
									</p>
									<p class="text-xs text-gray-500 dark:text-gray-400">
										@{user.username} · {user.email}
									</p>
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
					</div>
				{/if}
			</div>
		{/if}
	{/if}
</section>
