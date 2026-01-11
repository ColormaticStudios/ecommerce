<script lang="ts">
	import { getContext } from "svelte";
	import { goto } from "$app/navigation";
	import { page } from "$app/stores";
	import { resolve } from "$app/paths";
	import { type API } from "$lib/api";
	import { type ProductModel } from "$lib/models";
	import ProductCard from "$lib/components/ProductCard.svelte";
	import { SvelteURLSearchParams } from "svelte/reactivity";

	const api: API = getContext("api");

	let results = $state<ProductModel[]>([]);
	let loading = $state(false);
	let errorMessage = $state("");
	let searchQuery = $state("");
	let draftQuery = $state("");
	let currentPage = $state(1);
	let pageSize = $state(12);
	let totalPages = $state(1);
	let totalResults = $state(0);
	let sortBy = $state<"created_at" | "price" | "name">("created_at");
	let sortOrder = $state<"asc" | "desc">("desc");
	let hasLoaded = $state(false);

	const pageSizeOptions = [8, 12, 24, 36];
	const sortOptions: Array<{ value: "created_at" | "price" | "name"; label: string }> = [
		{ value: "created_at", label: "Newest" },
		{ value: "price", label: "Price" },
		{ value: "name", label: "Name" },
	];

	function normalizeSort(value: string | null) {
		if (value === "price" || value === "name" || value === "created_at") {
			return value;
		}
		return "created_at";
	}

	function normalizeOrder(value: string | null) {
		if (value === "asc" || value === "desc") {
			return value;
		}
		return "desc";
	}

	function normalizeLimit(value: number) {
		if (pageSizeOptions.includes(value)) {
			return value;
		}
		return 12;
	}

	function buildSearchParams(next: {
		query?: string;
		page?: number;
		limit?: number;
		sort?: "created_at" | "price" | "name";
		order?: "asc" | "desc";
	}) {
		const params = new SvelteURLSearchParams();
		if (next.query) {
			params.set("q", next.query);
		}
		if (next.page && next.page > 1) {
			params.set("page", String(next.page));
		}
		if (next.limit && next.limit !== 12) {
			params.set("limit", String(next.limit));
		}
		if (next.sort && next.sort !== "created_at") {
			params.set("sort", next.sort);
		}
		if (next.order && next.order !== "desc") {
			params.set("order", next.order);
		}
		return params;
	}

	function updateUrl(next: {
		query?: string;
		page?: number;
		limit?: number;
		sort?: "created_at" | "price" | "name";
		order?: "asc" | "desc";
	}) {
		const params = buildSearchParams({
			query: next.query ?? searchQuery,
			page: next.page ?? currentPage,
			limit: next.limit ?? pageSize,
			sort: next.sort ?? sortBy,
			order: next.order ?? sortOrder,
		});
		const path = resolve("/search");
		const queryString = params.toString();
		const nextUrl = queryString ? `${path}?${queryString}` : path;
		// @ts-expect-error Svelte's routing requirements are so strict man
		void goto(resolve(nextUrl), { replaceState: false, noScroll: true, keepFocus: true });
	}

	async function loadResults() {
		loading = true;
		errorMessage = "";
		try {
			const page = await api.listProducts({
				q: searchQuery.trim() || undefined,
				page: currentPage,
				limit: pageSize,
				sort: sortBy,
				order: sortOrder,
			});
			results = page.data;
			totalPages = Math.max(1, page.pagination.total_pages);
			totalResults = page.pagination.total;
		} catch (err) {
			console.error(err);
			errorMessage = "Unable to load search results.";
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		const params = $page.url.searchParams;
		const nextQuery = params.get("q") ?? "";
		const nextPage = Math.max(1, Number(params.get("page") ?? 1));
		const nextLimit = normalizeLimit(Number(params.get("limit") ?? 12));
		const nextSort = normalizeSort(params.get("sort"));
		const nextOrder = normalizeOrder(params.get("order"));

		let shouldLoad = false;
		if (searchQuery !== nextQuery) {
			searchQuery = nextQuery;
			draftQuery = nextQuery;
			shouldLoad = true;
		}
		if (currentPage !== nextPage) {
			currentPage = nextPage;
			shouldLoad = true;
		}
		if (pageSize !== nextLimit) {
			pageSize = nextLimit;
			shouldLoad = true;
		}
		if (sortBy !== nextSort) {
			sortBy = nextSort;
			shouldLoad = true;
		}
		if (sortOrder !== nextOrder) {
			sortOrder = nextOrder;
			shouldLoad = true;
		}
		if (shouldLoad || !hasLoaded) {
			hasLoaded = true;
			void loadResults();
		}
	});
</script>

<section>
	<div class="mx-auto mt-12 max-w-6xl px-4">
		<div class="flex flex-col gap-6">
			<div>
				<h1 class="text-3xl font-semibold text-gray-900 dark:text-gray-100">Product Search</h1>
			</div>

			<form
				class="flex flex-col gap-3 rounded-2xl border border-gray-200 bg-white/80 p-4 shadow-sm backdrop-blur dark:border-gray-800 dark:bg-gray-900/70"
				onsubmit={(event) => {
					event.preventDefault();
					updateUrl({ query: draftQuery.trim(), page: 1 });
				}}
			>
				<div class="flex flex-row flex-wrap items-center gap-3">
					<input
						type="search"
						placeholder="Search products"
						class="textinput min-w-[16rem] flex-1"
						bind:value={draftQuery}
					/>
					<button type="submit" class="btn btn-primary flex items-center gap-2">
						<i class="bi bi-search mr-1"></i>
						Search
					</button>
				</div>
				<div
					class="flex flex-wrap items-center justify-between text-sm text-gray-600 dark:text-gray-300"
				>
					<div class="flex items-center gap-2">
						<span class="text-xs tracking-[0.2em] text-gray-400 uppercase dark:text-gray-500">
							Sort by
						</span>
						<select
							class="rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700 shadow-sm dark:border-gray-800 dark:bg-gray-900 dark:text-gray-200"
							bind:value={sortBy}
							onchange={() => updateUrl({ page: 1 })}
						>
							{#each sortOptions as option, i (i)}
								<option value={option.value}>{option.label}</option>
							{/each}
						</select>
						<button
							type="button"
							class="btn btn-regular flex items-center gap-2"
							onclick={() => updateUrl({ order: sortOrder === "asc" ? "desc" : "asc", page: 1 })}
						>
							<i class={sortOrder === "asc" ? "bi bi-sort-up" : "bi bi-sort-down"}></i>
							{sortOrder === "asc" ? "Ascending" : "Descending"}
						</button>
					</div>
					{#if searchQuery}
						<button
							type="button"
							class="btn btn-regular"
							onclick={() => updateUrl({ query: "", page: 1 })}
						>
							<i class="bi bi-x-circle mr-1"></i>
							Clear search
						</button>
					{/if}
				</div>
			</form>
		</div>
	</div>
</section>

<section class="mx-auto flex max-w-6xl flex-col gap-6 px-4 py-6">
	<div class="flex flex-wrap items-center justify-between gap-4">
		<div class="text-sm text-gray-500 dark:text-gray-400">
			{#if loading}
				Loading results...
			{:else if errorMessage}
				{errorMessage}
			{:else if searchQuery}
				<span class="font-medium text-gray-700 dark:text-gray-200">
					{totalResults}
					{totalResults === 1 ? "result" : "results"} for "{searchQuery}"
				</span>
			{:else}
				<span class="font-medium text-gray-700 dark:text-gray-200">
					Browse {totalResults} products
				</span>
			{/if}
		</div>
		<div
			class="rounded-full border border-gray-200 bg-white px-3 py-1 text-xs font-semibold tracking-[0.2em] text-gray-500 uppercase shadow-sm dark:border-gray-800 dark:bg-gray-900 dark:text-gray-400"
		>
			Page {currentPage} of {totalPages}
		</div>
	</div>

	{#if !loading && !errorMessage && results.length === 0}
		<div
			class="rounded-2xl border border-dashed border-gray-300 bg-gray-50 px-6 py-10 text-center sm:px-10 dark:border-gray-700 dark:bg-gray-900"
		>
			<h2 class="text-3xl font-semibold text-gray-900 dark:text-gray-100">No matches found.</h2>
			<p class="mt-2 text-sm text-gray-600 dark:text-gray-400">
				Try a different keyword or clear filters.
			</p>
			<div class="mt-4">
				<button
					type="button"
					class="btn btn-primary"
					onclick={() => updateUrl({ query: "", page: 1 })}
				>
					Browse all products
				</button>
			</div>
		</div>
	{:else}
		<div
			class="grid grid-cols-1 gap-6 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5"
		>
			{#each results as product (product.sku)}
				<ProductCard
					href={resolve(`/product/${product.id}`)}
					data={{
						//id: product.id, // Unused
						name: product.name,
						description: product.description,
						price: product.price,
						image: product.images?.[0],
						stock: product.stock,
					}}
				/>
			{/each}
		</div>

		<div
			class="flex flex-wrap items-center justify-between gap-3 text-xs text-gray-500 dark:text-gray-400"
		>
			<div class="flex items-center gap-2">
				<span>Per page</span>
				<select
					class="cursor-pointer rounded-md border border-gray-300 bg-gray-100 px-2 py-1 text-xs dark:border-gray-700 dark:bg-gray-800"
					bind:value={pageSize}
					onchange={() => updateUrl({ page: 1 })}
				>
					{#each pageSizeOptions as option, i (i)}
						<option value={option}>{option}</option>
					{/each}
				</select>
			</div>
			<span>Page {currentPage} of {totalPages}</span>
			<div class="flex items-center gap-2">
				<button
					class="btn btn-regular flex items-center gap-2"
					type="button"
					disabled={currentPage <= 1}
					onclick={() => updateUrl({ page: currentPage - 1 })}
				>
					<i class="bi bi-arrow-left"></i>
					Prev
				</button>
				<button
					class="btn btn-regular flex items-center gap-2"
					type="button"
					disabled={currentPage >= totalPages}
					onclick={() => updateUrl({ page: currentPage + 1 })}
				>
					Next
					<i class="bi bi-arrow-right"></i>
				</button>
			</div>
		</div>
	{/if}
</section>
