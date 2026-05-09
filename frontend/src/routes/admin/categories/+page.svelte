<script lang="ts">
	import { type API } from "$lib/api";
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminFieldLabel from "$lib/admin/AdminFieldLabel.svelte";
	import AdminFloatingNotices from "$lib/admin/AdminFloatingNotices.svelte";
	import AdminListItem from "$lib/admin/AdminListItem.svelte";
	import AdminMasterDetailLayout from "$lib/admin/AdminMasterDetailLayout.svelte";
	import AdminPageHeader from "$lib/admin/AdminPageHeader.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import AdminResourceActions from "$lib/admin/AdminResourceActions.svelte";
	import AdminSurface from "$lib/admin/AdminSurface.svelte";
	import { createAdminNotices, createAdminSavePrompt } from "$lib/admin/state.svelte";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import NumberInput from "$lib/components/NumberInput.svelte";
	import TextArea from "$lib/components/TextArea.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
	import { type CategoryModel } from "$lib/models";
	import { getContext, onMount } from "svelte";

	const api: API = getContext("api");

	let categories = $state<CategoryModel[]>([]);
	let loading = $state(false);
	let saving = $state(false);
	let deleting = $state(false);
	let selectedCategoryId = $state<number | null>(null);
	let searchQuery = $state("");
	let appliedSearchQuery = $state("");
	let name = $state("");
	let slug = $state("");
	let description = $state("");
	let parentId = $state("");
	let sortOrder = $state("0");
	let isActive = $state(true);
	let hasLoadError = $state(false);
	let savedSnapshot = $state("");
	const notices = createAdminNotices();
	const savePrompt = createAdminSavePrompt({
		navigationMessage: "You have unsaved category changes. Leave this section and discard them?",
	});

	const hasAppliedSearch = $derived(appliedSearchQuery.length > 0);
	const isEditingCategory = $derived(selectedCategoryId !== null);
	const parentOptions = $derived(
		categories.filter((category) => category.id !== selectedCategoryId)
	);
	const currentSnapshot = $derived(
		JSON.stringify({
			selectedCategoryId,
			name: name.trim(),
			slug: slug.trim(),
			description: description.trim(),
			parentId,
			sortOrder,
			isActive,
		})
	);
	const formHasUnsavedChanges = $derived(currentSnapshot !== savedSnapshot);

	function captureSavedSnapshot() {
		savedSnapshot = currentSnapshot;
	}

	function resetForm() {
		selectedCategoryId = null;
		name = "";
		slug = "";
		description = "";
		parentId = "";
		sortOrder = "0";
		isActive = true;
		captureSavedSnapshot();
	}

	function loadIntoForm(category: CategoryModel) {
		selectedCategoryId = category.id;
		name = category.name;
		slug = category.slug;
		description = category.description ?? "";
		parentId = category.parent_id?.toString() ?? "";
		sortOrder = category.sort_order.toString();
		isActive = category.is_active;
		captureSavedSnapshot();
	}

	async function loadCategories(query = appliedSearchQuery) {
		loading = true;
		hasLoadError = false;
		notices.clear();
		try {
			const normalizedQuery = query.trim();
			categories = await api.listAdminCategories({
				...(normalizedQuery ? { q: normalizedQuery } : {}),
				include_inactive: true,
			});
			appliedSearchQuery = normalizedQuery;
			if (selectedCategoryId !== null) {
				const refreshed = categories.find((category) => category.id === selectedCategoryId);
				if (refreshed) {
					loadIntoForm(refreshed);
				}
			}
		} catch (error) {
			console.error(error);
			hasLoadError = true;
			notices.setError("Unable to load categories.");
		} finally {
			loading = false;
		}
	}

	function applyCategorySearch() {
		void loadCategories(searchQuery);
	}

	function refreshCategories() {
		searchQuery = appliedSearchQuery;
		void loadCategories(appliedSearchQuery);
	}

	function categoryDepthLabel(category: CategoryModel) {
		return category.depth === 0 ? "Top level" : `Level ${category.depth + 1}`;
	}

	async function saveCategory() {
		if (saving) {
			return;
		}

		saving = true;
		try {
			const isUpdate = selectedCategoryId !== null;
			const numericSortOrder = Number.parseInt(sortOrder || "0", 10);
			const payload = {
				name: name.trim(),
				slug: slug.trim() || undefined,
				description: description.trim() || undefined,
				parent_id: parentId ? Number.parseInt(parentId, 10) : undefined,
				sort_order: Number.isNaN(numericSortOrder) ? 0 : numericSortOrder,
				is_active: isActive,
			};

			const saved =
				selectedCategoryId === null
					? await api.createAdminCategory(payload)
					: await api.updateAdminCategory(selectedCategoryId, payload);

			loadIntoForm(saved);
			await loadCategories(appliedSearchQuery);
			captureSavedSnapshot();
			notices.setSuccess(isUpdate ? "Category updated." : "Category created.");
		} catch (error) {
			console.error(error);
			const err = error as { body?: { error?: string } };
			notices.setError(err.body?.error ?? "Unable to save category.");
		} finally {
			saving = false;
		}
	}

	async function deleteCategory() {
		if (selectedCategoryId === null || deleting) {
			return;
		}

		if (!window.confirm(`Delete category "${name.trim() || "this category"}"?`)) {
			return;
		}

		deleting = true;
		try {
			await api.deleteAdminCategory(selectedCategoryId);
			resetForm();
			await loadCategories(appliedSearchQuery);
			notices.setSuccess("Category deleted.");
		} catch (error) {
			console.error(error);
			const err = error as { body?: { error?: string } };
			notices.setError(err.body?.error ?? "Unable to delete category.");
		} finally {
			deleting = false;
		}
	}

	onMount(() => {
		captureSavedSnapshot();
		void loadCategories();
	});

	$effect(() => {
		savePrompt.dirty = formHasUnsavedChanges;
		savePrompt.blocked = saving || deleting;
		savePrompt.saveAction = formHasUnsavedChanges ? saveCategory : null;
	});
</script>

{#snippet categoryActions()}
	<AdminResourceActions
		searchFullWidth={true}
		searchClass="sm:max-w-xs"
		searchPlaceholder="Search categories"
		bind:searchValue={searchQuery}
		onSearch={applyCategorySearch}
		onRefresh={refreshCategories}
		searchRefreshing={loading}
		searchDisabled={loading}
	/>
{/snippet}

{#snippet categoryHeaderActions()}
	<AdminResourceActions countLabel={`${categories.length} categories`} />
{/snippet}

<section class="space-y-6">
	<AdminPageHeader title="Categories" actions={categoryHeaderActions} />

	<AdminMasterDetailLayout class="mt-6" columnsClass="xl:grid-cols-[0.95fr_1.05fr]">
		{#snippet master()}
			<AdminPanel title="Categories" headerActions={categoryActions}>
				<div class="space-y-3">
					{#if hasLoadError}
						<AdminEmptyState tone="error">Failed to load categories.</AdminEmptyState>
					{:else if loading && categories.length === 0}
						<AdminEmptyState>Loading categories...</AdminEmptyState>
					{:else if categories.length === 0 && hasAppliedSearch}
						<AdminEmptyState>Your search didn&apos;t match any categories.</AdminEmptyState>
					{:else if categories.length === 0}
						<AdminEmptyState>There are no categories.</AdminEmptyState>
					{:else}
						{#each categories as category (category.id)}
							<AdminListItem
								as="button"
								active={selectedCategoryId === category.id}
								interactive={selectedCategoryId !== category.id}
								class="flex items-center justify-between gap-3 p-4"
								onclick={() => loadIntoForm(category)}
							>
								<div class="min-w-0">
									<div class="flex flex-wrap items-center gap-2">
										<p class="truncate text-sm font-semibold text-stone-950 dark:text-stone-50">
											{category.name}
										</p>
										<Badge tone="neutral">/{category.slug}</Badge>
									</div>
									<p class="mt-1 line-clamp-2 text-xs text-stone-500 dark:text-stone-400">
										{category.description || categoryDepthLabel(category)}
									</p>
								</div>
								<div class="flex shrink-0 items-center gap-2">
									<Badge tone="neutral">{categoryDepthLabel(category)}</Badge>
									<Badge tone={category.is_active ? "success" : "neutral"}>
										{category.is_active ? "Active" : "Inactive"}
									</Badge>
								</div>
							</AdminListItem>
						{/each}
					{/if}
				</div>
			</AdminPanel>
		{/snippet}

		{#snippet detail()}
			<AdminPanel
				title={isEditingCategory ? "Edit Category" : "New Category"}
				meta={isEditingCategory ? `ID ${selectedCategoryId}` : ""}
			>
				<div class="grid gap-5 md:grid-cols-2">
					<label class="block">
						<AdminFieldLabel as="span">Name</AdminFieldLabel>
						<TextInput tone="admin" class="mt-2 w-full" type="text" bind:value={name} />
					</label>

					<label class="block">
						<AdminFieldLabel as="span">Slug</AdminFieldLabel>
						<TextInput tone="admin" class="mt-2 w-full" type="text" bind:value={slug} />
					</label>

					<label class="block">
						<AdminFieldLabel as="span">Parent</AdminFieldLabel>
						<Dropdown tone="admin" class="mt-2" bind:value={parentId}>
							<option value="">Top level</option>
							{#each parentOptions as category (category.id)}
								<option value={category.id.toString()}>
									{" ".repeat(category.depth * 2)}{category.name}
								</option>
							{/each}
						</Dropdown>
					</label>

					<label class="block">
						<AdminFieldLabel as="span">Sort order</AdminFieldLabel>
						<NumberInput tone="admin" class="mt-2" bind:value={sortOrder} />
					</label>

					<label class="block md:col-span-2">
						<AdminFieldLabel as="span">Description</AdminFieldLabel>
						<TextArea tone="admin" class="mt-2 min-h-32" bind:value={description} />
					</label>
				</div>

				<AdminSurface variant="muted" as="div" class="mt-5">
					<label class="flex items-center justify-between gap-4">
						<div>
							<p class="text-sm font-semibold text-stone-950 dark:text-stone-50">
								Active in catalog
							</p>
							<p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
								Inactive categories stay available for admin organization while hidden from
								customer-facing category lists.
							</p>
						</div>
						<input class="h-4 w-4 shrink-0" type="checkbox" bind:checked={isActive} />
					</label>
				</AdminSurface>

				<div class="mt-6 flex flex-wrap items-center gap-3">
					<Button
						tone="admin"
						type="button"
						variant="primary"
						class="rounded-full whitespace-nowrap"
						disabled={saving || deleting}
						onclick={() => void saveCategory()}
					>
						<i class="bi {isEditingCategory ? 'bi-floppy-fill' : 'bi-plus-lg'} mr-1"></i>
						{saving ? "Saving..." : isEditingCategory ? "Save changes" : "Create category"}
					</Button>
					<Button
						tone="admin"
						type="button"
						variant="regular"
						class="rounded-full whitespace-nowrap"
						disabled={saving || deleting}
						onclick={resetForm}
					>
						<i class="bi bi-x-lg mr-1"></i>
						Clear
					</Button>
					{#if isEditingCategory}
						<Button
							tone="admin"
							type="button"
							variant="danger"
							class="rounded-full whitespace-nowrap"
							disabled={saving || deleting}
							onclick={() => void deleteCategory()}
						>
							<i class="bi bi-trash mr-1"></i>
							{deleting ? "Deleting..." : "Delete category"}
						</Button>
					{/if}
				</div>
			</AdminPanel>
		{/snippet}
	</AdminMasterDetailLayout>
</section>

<AdminFloatingNotices
	showUnsaved={savePrompt.dirty}
	unsavedMessage="You have unsaved category changes."
	canSaveUnsaved={savePrompt.canSave}
	onSaveUnsaved={() => void savePrompt.save()}
	savingUnsaved={savePrompt.saving}
	statusMessage={notices.message}
	statusTone={notices.tone}
	onDismissStatus={notices.clear}
/>
