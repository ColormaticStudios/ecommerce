<script lang="ts">
	import AdminFieldLabel from "$lib/admin/AdminFieldLabel.svelte";
	import Badge from "$lib/components/Badge.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
	import type { BrandModel, CategoryModel } from "$lib/models";
	import { asTrimmedString } from "./state";

	let {
		brands,
		categories,
		selectedBrandId = $bindable(),
		selectedCategoryIds = $bindable(),
	}: {
		brands: BrandModel[];
		categories: CategoryModel[];
		selectedBrandId: string;
		selectedCategoryIds: string[];
	} = $props();

	let query = $state("");
	let menuOpen = $state(false);
	const selectedCategories = $derived.by(() => {
		const selected = new Set(selectedCategoryIds);
		return categories.filter((category) => selected.has(String(category.id)));
	});
	const availableCategories = $derived.by(() => {
		const selected = new Set(selectedCategoryIds);
		const normalizedQuery = asTrimmedString(query).toLowerCase();
		return categories
			.filter((category) => !selected.has(String(category.id)) && category.is_active)
			.filter(
				(category) =>
					!normalizedQuery ||
					category.name.toLowerCase().includes(normalizedQuery) ||
					category.slug.toLowerCase().includes(normalizedQuery) ||
					(category.description ?? "").toLowerCase().includes(normalizedQuery)
			)
			.slice(0, 8);
	});

	function addCategory(category: CategoryModel) {
		const id = String(category.id);
		if (!selectedCategoryIds.includes(id)) selectedCategoryIds = [...selectedCategoryIds, id];
		query = "";
		menuOpen = false;
	}

	function removeCategory(id: number) {
		selectedCategoryIds = selectedCategoryIds.filter((selectedId) => selectedId !== String(id));
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === "Escape") {
			menuOpen = false;
			return;
		}
		if (event.key === "Enter" && availableCategories[0]) {
			event.preventDefault();
			addCategory(availableCategories[0]);
		}
	}
</script>

<div>
	<AdminFieldLabel as="label" for="admin-product-brand">Brand</AdminFieldLabel>
	<Dropdown tone="admin" id="admin-product-brand" class="mt-1" bind:value={selectedBrandId}>
		<option value="">No brand</option>
		{#each brands as brand (brand.id)}<option value={String(brand.id)}>{brand.name}</option>{/each}
	</Dropdown>
</div>
<div class="sm:col-span-2">
	<AdminFieldLabel>Categories</AdminFieldLabel>
	{#if categories.length === 0}
		<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">No categories exist yet.</p>
	{:else}
		<div
			class="mt-2 rounded-lg border border-stone-300 bg-white p-2 dark:border-stone-700 dark:bg-stone-900"
		>
			<div class="flex min-h-9 flex-wrap items-center gap-2">
				{#if selectedCategories.length === 0}
					<p class="px-1 text-sm text-stone-500 dark:text-stone-400">No categories assigned</p>
				{:else}
					{#each selectedCategories as category (category.id)}
						<span
							class="inline-flex max-w-full items-center gap-1 rounded-full border border-stone-200 bg-stone-100 py-1 pr-1 pl-2.5 text-xs font-semibold text-stone-700 dark:border-stone-800 dark:bg-stone-950 dark:text-stone-200"
						>
							<span class="truncate">{category.name}</span>
							{#if !category.is_active}<span
									class="text-[10px] font-medium text-stone-500 dark:text-stone-400">Inactive</span
								>{/if}
							<button
								type="button"
								class="inline-flex h-5 w-5 items-center justify-center rounded-full text-stone-500 hover:bg-stone-200 hover:text-stone-900 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-stone-500 dark:text-stone-400 dark:hover:bg-stone-800 dark:hover:text-stone-50"
								aria-label={`Remove category ${category.name}`}
								onclick={() => removeCategory(category.id)}><i class="bi bi-x text-xs"></i></button
							>
						</span>
					{/each}
				{/if}
			</div>
			<div class="relative mt-2">
				<TextInput
					tone="admin"
					type="search"
					placeholder="Search categories to add"
					aria-label="Search categories to add"
					bind:value={query}
					onfocus={() => (menuOpen = true)}
					oninput={() => (menuOpen = true)}
					onkeydown={handleKeydown}
				/>
				{#if menuOpen}
					<div
						class="absolute z-20 mt-2 max-h-64 w-full overflow-y-auto rounded-lg border border-stone-200 bg-white p-1 shadow-lg dark:border-stone-800 dark:bg-stone-950"
					>
						{#if availableCategories.length === 0}<p
								class="px-3 py-2 text-sm text-stone-500 dark:text-stone-400"
							>
								No matching categories
							</p>{:else}
							{#each availableCategories as category (category.id)}
								<button
									type="button"
									class="flex w-full items-center justify-between gap-3 rounded-md px-3 py-2 text-left text-sm text-stone-700 hover:bg-stone-100 focus-visible:bg-stone-100 focus-visible:outline-none dark:text-stone-200 dark:hover:bg-stone-900 dark:focus-visible:bg-stone-900"
									onclick={() => addCategory(category)}
								>
									<span class="min-w-0"
										><span class="block truncate font-medium"
											>{" ".repeat(category.depth * 2)}{category.name}</span
										><span class="block truncate text-xs text-stone-500 dark:text-stone-400"
											>/{category.slug}</span
										></span
									>
									{#if !category.is_active}<Badge tone="neutral" size="sm">Inactive</Badge>{/if}
								</button>
							{/each}
						{/if}
					</div>
				{/if}
			</div>
		</div>
	{/if}
</div>
