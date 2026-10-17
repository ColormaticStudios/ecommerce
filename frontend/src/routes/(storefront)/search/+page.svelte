<script lang="ts">
	import { navigating, page } from "$app/state";
	import { getContext, onMount, onDestroy } from "svelte";
	import type { components } from "$lib/api/generated/openapi";
	import type { API } from "$lib/api";
	import { LOCALIZATION_CONTEXT, type LocalizationRuntime } from "$lib/localization/runtime";
	import {
		recordSearchImpression,
		revokePendingSearchConsent,
		recordSearchClick,
		searchConsent,
		setSearchConsent,
		SEARCH_CONSENT_CHANGED,
		SEARCH_CONSENT_STORAGE_KEY,
	} from "$lib/search/analytics";
	import { afterNavigate, goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import EmptyStateCard from "$lib/components/EmptyStateCard.svelte";
	import FilterPanel from "$lib/components/FilterPanel.svelte";
	import SearchCombobox from "$lib/search/SearchCombobox.svelte";
	import type { ProductModel } from "$lib/models";
	import ProductCard from "$lib/components/ProductCard.svelte";
	import { formatPrice } from "$lib/utils";
	import { priceFacetBounds, visibleSearchFacets } from "./search-facets";
	import {
		buildSearchParams,
		defaultSearchSort,
		type SearchSort,
		type SearchUrlState,
	} from "./search-params";
	import type { PageData } from "./$types";

	interface Props {
		data: PageData;
	}
	let { data }: Props = $props();
	const api = getContext<API>("api");
	const localization = getContext<LocalizationRuntime>(LOCALIZATION_CONTEXT);
	let metadata = $state<PageData["metadata"]>(null);
	let suggestions = $state<PageData["suggestions"]>({
		suggestions: [],
		corrections: [],
	});
	let consent = $state(false);
	let impressionId = $state("");
	let impressionRevision = 0;
	let impressionEventId = "";
	let suggestionsRevision = 0;
	let suggestionTimer: ReturnType<typeof setTimeout> | undefined;
	const degraded = $derived(Boolean(metadata?.degraded));
	async function collectImpression() {
		const revision = ++impressionRevision;
		impressionId = "";
		if (!consent || !data.resultSnapshot || data.metadata?.degraded || data.draftPreview?.active)
			return;
		try {
			const response = await recordSearchImpression(
				api,
				data.resultSnapshot,
				(impressionEventId ||= crypto.randomUUID())
			);
			if (!response || revision !== impressionRevision || !searchConsent()) return;
			impressionId = response.impression_id;
		} catch {
			/* Search remains usable without analytics. */
		}
	}
	function consentChanged() {
		consent = searchConsent();
		void collectImpression();
	}
	onMount(() => {
		consent = searchConsent();
		void revokePendingSearchConsent(api);
		const storageChanged = (event: StorageEvent) => {
			if (event.key === SEARCH_CONSENT_STORAGE_KEY) consentChanged();
		};
		window.addEventListener(SEARCH_CONSENT_CHANGED, consentChanged);
		window.addEventListener("storage", storageChanged);
		return () => {
			window.removeEventListener(SEARCH_CONSENT_CHANGED, consentChanged);
			window.removeEventListener("storage", storageChanged);
		};
	});
	afterNavigate(() => {
		++suggestionsRevision;
		if (suggestionTimer) clearTimeout(suggestionTimer);
		impressionEventId = crypto.randomUUID();
		void collectImpression();
	});
	onDestroy(() => {
		++impressionRevision;
		++suggestionsRevision;
		if (suggestionTimer) clearTimeout(suggestionTimer);
	});
	function changeConsent(enabled: boolean) {
		if (enabled && !consent) impressionEventId = crypto.randomUUID();
		setSearchConsent(enabled, api);
		consent = searchConsent();
	}
	function requestSuggestions(event: Event) {
		if (suggestionTimer) clearTimeout(suggestionTimer);
		const revision = ++suggestionsRevision;
		const query = (event.currentTarget as HTMLInputElement).value;
		if (degraded) return;
		suggestionTimer = setTimeout(async () => {
			try {
				const response = await api.searchSuggestions(query);
				if (revision === suggestionsRevision) suggestions = response;
			} catch {
				if (revision === suggestionsRevision) suggestions = { suggestions: [], corrections: [] };
			}
		}, 200);
	}
	function clickProduct(product: ProductModel, index: number) {
		if (!consent || !impressionId || degraded) return;
		void recordSearchClick(api, {
			impressionId,
			productId: product.id,
			position: (currentPage - 1) * pageSize + index + 1,
		}).catch(() => {
			/* Persisted click can be retried before cart attribution. */
		});
	}

	let results = $state<ProductModel[]>([]);
	let errorMessage = $state("");
	let searchQuery = $state("");
	let draftQuery = $state("");
	let currentPage = $state(1);
	let pageSize = $state(12);
	let totalPages = $state(1);
	let totalResults = $state(0);
	let sortBy = $state<SearchSort>("created_at");
	let sortExplicit = $state(false);
	let sortOrder = $state<"asc" | "desc">("desc");
	const loading = $derived(Boolean(navigating.to));

	const pageSizeOptions = [8, 12, 24, 36];
	const sortOptions = $derived([
		{
			value: "relevance",
			label: $localization.translate("storefront.search.sort_relevance", "Relevance"),
		},
		{
			value: "created_at",
			label: $localization.translate("storefront.search.sort_newest", "Newest"),
		},
		{ value: "price", label: $localization.translate("storefront.search.price", "Price") },
		{ value: "name", label: $localization.translate("storefront.search.sort_name", "Name") },
	]);
	function facetLabel(facet: Facet) {
		if (facet.name === "brand")
			return $localization.translate("storefront.search.brands", "Brands");
		if (facet.name === "category")
			return $localization.translate("storefront.search.categories", "Categories");
		if (facet.name === "stock")
			return $localization.translate("storefront.search.availability", "Availability");
		if (facet.name === "price") return $localization.translate("storefront.search.price", "Price");
		return facet.label ?? facet.name;
	}
	function valueLabel(facet: Facet, value: Facet["values"][number]) {
		if (facet.name === "stock")
			return value.value === "true"
				? $localization.translate("storefront.search.in_stock", "In stock")
				: $localization.translate("storefront.search.out_of_stock", "Out of stock");
		if (facet.name === "price") {
			const { min, max } = priceFacetBounds(value);
			if (min !== undefined && max !== undefined) {
				if (min === max) return formatPrice(min);
				return $localization.translate("storefront.search.price_range", "{min} – {max}", {
					min: formatPrice(min),
					max: formatPrice(max),
				});
			}
			if (min !== undefined)
				return $localization.translate("storefront.search.price_from", "{min} and up", {
					min: formatPrice(min),
				});
			if (max !== undefined)
				return $localization.translate("storefront.search.price_to", "Up to {max}", {
					max: formatPrice(max),
				});
		}
		return value.label ?? value.value;
	}
	type Facet = components["schemas"]["SearchFacet"];
	let facets = $state<Facet[]>([]);
	const hasActiveFilters = $derived(
		searchQuery.trim() !== "" ||
			data.brandSlugs.length > 0 ||
			data.categorySlugs.length > 0 ||
			data.stockSelections.length > 0 ||
			data.priceRanges.length > 0 ||
			Object.values(data.attributeSelections).some((values) => values.length)
	);
	function selection(name: string): string[] {
		if (name === "brand") return data.brandSlugs;
		if (name === "category") return data.categorySlugs;
		if (name === "stock") return data.stockSelections.map(String);
		if (name === "price") return data.priceRanges;
		return name.startsWith("attribute:") ? (data.attributeSelections[name.slice(10)] ?? []) : [];
	}
	function toggleFacet(name: string, value: string, checked: boolean) {
		const current = selection(name);
		const values = checked
			? [...new Set([...current, value])]
			: current.filter((item) => item !== value);
		if (name === "brand") updateUrl({ brandSlugs: values, page: 1 });
		else if (name === "category") updateUrl({ categorySlugs: values, page: 1 });
		else if (name === "stock")
			updateUrl({ stockSelections: values.map((value) => value === "true"), page: 1 });
		else if (name === "price") updateUrl({ priceRanges: values, page: 1 });
		else if (name.startsWith("attribute:"))
			updateUrl({
				attributeSelections: { ...data.attributeSelections, [name.slice(10)]: values },
				page: 1,
			});
	}
	const fallbackFacets = $derived.by((): Facet[] => {
		const make = (
			name: string,
			label: string,
			values: Array<{ value: string; label: string }>,
			type: Facet["type"] = "terms"
		): Facet => ({
			name,
			label,
			type,
			values: values.map((value) => ({
				...value,
				count: 0,
				selected: selection(name).includes(value.value),
				disabled: false,
			})),
		});
		const brands: Array<{ value: string; label: string }> = [];
		for (const value of data.brandSlugs)
			if (!brands.some((item) => item.value === value)) brands.push({ value, label: value });
		const categories: Array<{ value: string; label: string }> = [];
		for (const value of data.categorySlugs)
			if (!categories.some((item) => item.value === value))
				categories.push({ value, label: value });
		return [
			make("brand", $localization.translate("storefront.search.brands", "Brands"), brands),
			make(
				"category",
				$localization.translate("storefront.search.categories", "Categories"),
				categories
			),
			make(
				"stock",
				$localization.translate("storefront.search.availability", "Availability"),
				[
					{
						value: "true",
						label: $localization.translate("storefront.search.in_stock", "In stock"),
					},
					{
						value: "false",
						label: $localization.translate("storefront.search.out_of_stock", "Out of stock"),
					},
				],
				"boolean"
			),
			make(
				"price",
				$localization.translate("storefront.search.price", "Price"),
				data.priceRanges.map((value) => ({ value, label: value })),
				"range"
			),
			...Object.entries(data.attributeSelections).map(([slug, values]) =>
				make(
					`attribute:${slug}`,
					slug,
					values.map((value) => ({ value, label: value })),
					"attribute"
				)
			),
		].filter((facet) => facet.values.length > 0);
	});
	const visibleFacets = $derived(visibleSearchFacets(facets, fallbackFacets));

	function updateUrl(next: Partial<SearchUrlState>) {
		const query = next.query ?? searchQuery;
		const params = buildSearchParams({
			query,
			brandSlugs: next.brandSlugs ?? data.brandSlugs,
			categorySlugs: next.categorySlugs ?? data.categorySlugs,
			stockSelections: next.stockSelections ?? data.stockSelections,
			priceRanges: next.priceRanges ?? data.priceRanges,
			attributeSelections: next.attributeSelections ?? data.attributeSelections,
			page: next.page ?? currentPage,
			limit: next.limit ?? pageSize,
			sort: next.sort ?? (sortExplicit ? sortBy : defaultSearchSort(query)),
			sortExplicit: next.sort !== undefined || sortExplicit,
			order: next.order ?? sortOrder,
		});
		for (const key of ["locale", "market"]) {
			if (page.url.searchParams.has(key)) params.set(key, page.url.searchParams.get(key) ?? "");
		}
		const path = resolve("/search");
		const queryString = params.toString();
		const nextUrl = queryString ? `${path}?${queryString}` : path;
		// @ts-expect-error Svelte's routing requirements are so strict man
		void goto(resolve(nextUrl), { replaceState: false, noScroll: true, keepFocus: true });
	}

	$effect(() => {
		metadata = data.metadata;
		facets = data.facets;
		suggestions = data.suggestions;
		results = data.results;
		errorMessage = data.errorMessage;
		searchQuery = data.searchQuery;
		draftQuery = data.draftQuery;
		currentPage = data.currentPage;
		pageSize = data.pageSize;
		totalPages = data.totalPages;
		totalResults = data.totalResults;
		sortBy = data.metadata?.degraded && data.sortBy === "relevance" ? "created_at" : data.sortBy;
		sortExplicit = data.sortExplicit;
		sortOrder = data.sortOrder;
	});
</script>

<section>
	<div class="mx-auto mt-12 max-w-6xl px-4">
		<div class="flex flex-col gap-6">
			<div>
				<h1 class="text-3xl font-semibold text-gray-900 dark:text-gray-100">
					{$localization.translate("storefront.search.title", "Product search")}
				</h1>
			</div>

			{#if degraded}<div
					role="status"
					class="rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-100"
				>
					{$localization.translate(
						"storefront.search.degraded",
						"Search is temporarily unavailable. Showing basic catalog browsing with your filters; keyword matching and relevance sorting are unavailable."
					)}
				</div>{/if}
			{#if metadata?.did_you_mean && !degraded}<div
					class="flex flex-wrap items-center gap-2 text-sm"
				>
					<span>{$localization.translate("storefront.search.did_you_mean", "Did you mean")}</span
					><Button
						type="button"
						onclick={() => updateUrl({ query: metadata?.did_you_mean ?? "", page: 1 })}
						>{metadata.did_you_mean}</Button
					>
				</div>{/if}
			{#if metadata?.relaxed && !degraded}<p
					role="status"
					class="text-sm text-gray-600 dark:text-gray-300"
				>
					{$localization.translate(
						"storefront.search.relaxed",
						"Showing related matches. Try the spelling suggestion or adjust your filters for closer results."
					)}
				</p>{/if}
			<FilterPanel>
				<form
					class="flex flex-col gap-3"
					onsubmit={(event) => {
						event.preventDefault();
						updateUrl({
							query: draftQuery.trim(),
							page: 1,
						});
					}}
				>
					<div class="flex flex-row flex-wrap items-center gap-3">
						<SearchCombobox
							label={$localization.translate("storefront.search.placeholder", "Search products")}
							suggestions={suggestions.suggestions}
							corrections={suggestions.corrections}
							bind:value={draftQuery}
							disabled={degraded}
							oninput={requestSuggestions}
							onselect={(query) => updateUrl({ query, page: 1 })}
						/>
						<Button type="submit" variant="primary" class="flex items-center gap-2">
							<i class={degraded ? "bi bi-funnel mr-1" : "bi bi-search mr-1"}></i>
							{degraded
								? $localization.translate("storefront.search.apply_filters", "Apply filters")
								: $localization.translate("storefront.search.submit", "Search")}
						</Button>
					</div>

					<div class="flex flex-wrap items-end gap-3">
						<div class="flex flex-wrap items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
							<span class="text-xs text-gray-600 dark:text-gray-400"
								>{$localization.translate("storefront.search.sort_by", "Sort by")}</span
							>
							<Dropdown
								tone="surface"
								full={false}
								class="min-w-40"
								bind:value={sortBy}
								onchange={(event) =>
									updateUrl({
										sort: event.currentTarget.value as SearchSort,
										order: "desc",
										page: 1,
									})}
							>
								{#each sortOptions.filter((option) => !degraded || option.value !== "relevance") as option, i (i)}
									<option value={option.value}>{option.label}</option>
								{/each}
							</Dropdown>
							<Button
								type="button"
								variant="regular"
								class="flex items-center gap-2 whitespace-nowrap"
								onclick={() => updateUrl({ order: sortOrder === "asc" ? "desc" : "asc", page: 1 })}
							>
								<i class={sortOrder === "asc" ? "bi bi-sort-up" : "bi bi-sort-down"}></i>
								{sortOrder === "asc"
									? $localization.translate("storefront.search.ascending", "Ascending")
									: $localization.translate("storefront.search.descending", "Descending")}
							</Button>
						</div>
						{#if hasActiveFilters}
							<Button
								type="button"
								variant="regular"
								class="whitespace-nowrap"
								onclick={() =>
									updateUrl({
										query: "",
										brandSlugs: [],
										categorySlugs: [],
										priceRanges: [],
										stockSelections: [],
										attributeSelections: {},
										page: 1,
									})}
							>
								<i class="bi bi-x-circle mr-1"></i>
								{$localization.translate("storefront.search.clear_filters", "Clear filters")}
							</Button>
						{/if}
					</div>
					{#if visibleFacets.length}<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
							{#each visibleFacets as facet (facet.name)}<fieldset class="min-w-0">
									<legend class="mb-2 text-sm font-medium text-gray-700 dark:text-gray-200"
										>{facetLabel(facet)}</legend
									>
									<div class="max-h-48 space-y-2 overflow-y-auto">
										{#each facet.values as value (value.value)}<label
												class="flex items-start gap-2 text-sm text-gray-600 dark:text-gray-300"
												><input
													type="checkbox"
													class="mt-1"
													checked={value.selected}
													disabled={loading || (value.disabled && !value.selected)}
													onchange={(event) =>
														toggleFacet(facet.name, value.value, event.currentTarget.checked)}
												/><span class="min-w-0 flex-1 break-words">{valueLabel(facet, value)}</span
												>{#if !degraded}<span class="text-gray-400 tabular-nums">{value.count}</span
													>{/if}</label
											>{/each}
									</div>
								</fieldset>{/each}
						</div>{/if}
				</form>
			</FilterPanel>

			<label class="flex items-start gap-2 text-sm text-gray-600 dark:text-gray-300"
				><input
					type="checkbox"
					class="mt-1"
					checked={consent}
					onchange={(event) => changeConsent(event.currentTarget.checked)}
				/><span
					>{$localization.translate(
						"storefront.search.consent",
						"Allow anonymous search activity to improve search. You can turn this off at any time; raw activity is retained for 90 days."
					)}</span
				></label
			>
		</div>
	</div>
</section>

<section class="mx-auto flex max-w-6xl flex-col gap-6 px-4 py-6">
	<div class="flex flex-wrap items-center justify-between gap-4">
		<div class="text-sm text-gray-500 dark:text-gray-400">
			{#if loading}
				{$localization.translate("storefront.search.loading", "Loading results…")}
			{:else if errorMessage}
				{$localization.translate("storefront.search.load_error", "Unable to load search results.")}
			{:else if searchQuery && !degraded}
				<span class="font-medium text-gray-700 dark:text-gray-200">
					{$localization.plural(
						"storefront.search.results",
						totalResults,
						{ one: "{count} result for “{query}”", other: "{count} results for “{query}”" },
						{ query: searchQuery }
					)}
				</span>
			{:else}
				<span class="font-medium text-gray-700 dark:text-gray-200">
					{$localization.plural("storefront.search.browse_count", totalResults, {
						one: "Browse {count} product",
						other: "Browse {count} products",
					})}
				</span>
			{/if}
		</div>
		<Badge tone="neutral" size="md" class="shadow-sm">
			{$localization.translate("storefront.search.page_count", "Page {page} of {pages}", {
				page: currentPage,
				pages: totalPages,
			})}
		</Badge>
	</div>

	{#if !errorMessage && results.length === 0}
		<EmptyStateCard
			title={$localization.translate("storefront.search.no_matches", "No matches found.")}
			description={$localization.translate(
				"storefront.search.no_matches_help",
				"Try a different keyword or clear filters."
			)}
			headingClass="text-3xl font-semibold"
			class="sm:px-10"
		>
			<div class="mt-4">
				<Button
					type="button"
					variant="primary"
					onclick={() =>
						updateUrl({
							query: "",
							brandSlugs: [],
							categorySlugs: [],
							priceRanges: [],
							stockSelections: [],
							attributeSelections: {},
							page: 1,
						})}
				>
					{$localization.translate("storefront.search.browse_all", "Browse all products")}
				</Button>
			</div>
		</EmptyStateCard>
	{:else}
		<div class="grid grid-cols-1 gap-6 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
			{#each results as product, index (product.id)}
				<ProductCard
					href={resolve(`/product/${product.id}`)}
					onclick={() => clickProduct(product, index)}
					onauxclick={(event) => {
						if (event.button === 1) clickProduct(product, index);
					}}
					data={{
						//id: product.id, // Unused
						name: product.name,
						brand: product.brand?.name,
						description: product.description,
						price: product.price,
						basePrice: product.base_price,
						discountAmount: product.discount_amount,
						finalPrice: product.final_price,
						priceRange: product.price_range,
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
				<span>{$localization.translate("storefront.search.per_page", "Per page")}</span>
				<Dropdown
					full={false}
					class="px-2 py-1 text-xs"
					bind:value={pageSize}
					onchange={() => updateUrl({ page: 1 })}
				>
					{#each pageSizeOptions as option, i (i)}
						<option value={option}>{option}</option>
					{/each}
				</Dropdown>
			</div>
			<span
				>{$localization.translate("storefront.search.page_count", "Page {page} of {pages}", {
					page: currentPage,
					pages: totalPages,
				})}</span
			>
			<div class="flex items-center gap-2">
				<Button
					variant="regular"
					size="small"
					class="flex items-center gap-2"
					type="button"
					disabled={currentPage <= 1}
					onclick={() => updateUrl({ page: currentPage - 1 })}
				>
					<i class="bi bi-arrow-left"></i>
					{$localization.translate("storefront.search.previous", "Previous")}
				</Button>
				<Button
					variant="regular"
					size="small"
					class="flex items-center gap-2"
					type="button"
					disabled={currentPage >= totalPages}
					onclick={() => updateUrl({ page: currentPage + 1 })}
				>
					{$localization.translate("storefront.search.next", "Next")}
					<i class="bi bi-arrow-right"></i>
				</Button>
			</div>
		</div>
	{/if}
</section>
