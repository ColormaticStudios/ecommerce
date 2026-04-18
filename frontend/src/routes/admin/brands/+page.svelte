<script lang="ts">
	import { type API } from "$lib/api";
	import AdminFloatingNotices from "$lib/admin/AdminFloatingNotices.svelte";
	import AdminMasterDetailLayout from "$lib/admin/AdminMasterDetailLayout.svelte";
	import AdminPageHeader from "$lib/admin/AdminPageHeader.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import AdminResourceActions from "$lib/admin/AdminResourceActions.svelte";
	import Badge from "$lib/components/Badge.svelte";
	import { createAdminNotices, createAdminSavePrompt } from "$lib/admin/state.svelte";
	import { API_BASE_URL } from "$lib/config";
	import Button from "$lib/components/Button.svelte";
	import ButtonInput from "$lib/components/ButtonInput.svelte";
	import IconButton from "$lib/components/IconButton.svelte";
	import TextArea from "$lib/components/TextArea.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
	import { type BrandModel } from "$lib/models";
	import { getContext, onDestroy, onMount } from "svelte";

	const api: API = getContext("api");

	let brands = $state<BrandModel[]>([]);
	let loading = $state(false);
	let saving = $state(false);
	let deleting = $state(false);
	let uploadingLogo = $state(false);
	let selectedBrandId = $state<number | null>(null);
	let searchQuery = $state("");
	let appliedSearchQuery = $state("");
	let name = $state("");
	let slug = $state("");
	let description = $state("");
	let logoMediaId = $state("");
	let logoPreviewUrl = $state<string | null>(null);
	let isActive = $state(true);
	let hasLoadError = $state(false);
	let savedSnapshot = $state("");
	const notices = createAdminNotices();
	const savePrompt = createAdminSavePrompt({
		navigationMessage: "You have unsaved brand changes. Leave this section and discard them?",
	});

	const hasAppliedSearch = $derived(appliedSearchQuery.length > 0);
	const isEditingBrand = $derived(selectedBrandId !== null);
	const currentSnapshot = $derived(
		JSON.stringify({
			selectedBrandId,
			name: name.trim(),
			slug: slug.trim(),
			description: description.trim(),
			logoMediaId: logoMediaId.trim(),
			isActive,
		})
	);
	const formHasUnsavedChanges = $derived(currentSnapshot !== savedSnapshot);
	const logoImageUrl = $derived.by(() => {
		if (logoPreviewUrl) {
			return logoPreviewUrl;
		}
		if (!logoMediaId.trim()) {
			return null;
		}
		return `${API_BASE_URL}/media/${encodeURIComponent(logoMediaId.trim())}/original.webp`;
	});

	function captureSavedSnapshot() {
		savedSnapshot = currentSnapshot;
	}

	function clearLogoPreview() {
		if (logoPreviewUrl) {
			URL.revokeObjectURL(logoPreviewUrl);
		}
		logoPreviewUrl = null;
	}

	function resetForm() {
		clearLogoPreview();
		selectedBrandId = null;
		name = "";
		slug = "";
		description = "";
		logoMediaId = "";
		isActive = true;
		captureSavedSnapshot();
	}

	function loadIntoForm(brand: BrandModel) {
		clearLogoPreview();
		selectedBrandId = brand.id;
		name = brand.name;
		slug = brand.slug;
		description = brand.description ?? "";
		logoMediaId = brand.logo_media_id ?? "";
		isActive = brand.is_active;
		captureSavedSnapshot();
	}

	async function loadBrands(query = appliedSearchQuery) {
		loading = true;
		hasLoadError = false;
		notices.clear();
		try {
			const normalizedQuery = query.trim();
			brands = await api.listAdminBrands(normalizedQuery ? { q: normalizedQuery } : {});
			appliedSearchQuery = normalizedQuery;
			if (selectedBrandId !== null) {
				const refreshed = brands.find((brand) => brand.id === selectedBrandId);
				if (refreshed) {
					loadIntoForm(refreshed);
				}
			}
		} catch (error) {
			console.error(error);
			hasLoadError = true;
			notices.setError("Unable to load brands.");
		} finally {
			loading = false;
		}
	}

	function applyBrandSearch() {
		void loadBrands(searchQuery);
	}

	function refreshBrands() {
		searchQuery = appliedSearchQuery;
		void loadBrands(appliedSearchQuery);
	}

	async function uploadLogo(event: Event) {
		if (uploadingLogo) {
			return;
		}

		const target = event.target as HTMLInputElement;
		const file = target.files?.[0];
		target.value = "";
		if (!file) {
			return;
		}

		uploadingLogo = true;
		const nextPreviewUrl = URL.createObjectURL(file);
		clearLogoPreview();
		logoPreviewUrl = nextPreviewUrl;

		try {
			logoMediaId = await api.uploadMedia(file);
			notices.setSuccess("Brand image uploaded. Save brand to keep this logo.");
		} catch (error) {
			console.error(error);
			clearLogoPreview();
			notices.setError("Unable to upload brand image.");
		} finally {
			uploadingLogo = false;
		}
	}

	function removeLogo() {
		if (!logoMediaId && !logoPreviewUrl) {
			return;
		}

		clearLogoPreview();
		logoMediaId = "";
		notices.setSuccess("Brand image removed. Save brand to keep this change.");
	}

	async function saveBrand() {
		if (saving) {
			return;
		}

		saving = true;
		try {
			const isUpdate = selectedBrandId !== null;
			const payload = {
				name: name.trim(),
				slug: slug.trim(),
				description: description.trim() || undefined,
				logo_media_id: logoMediaId.trim() || undefined,
				is_active: isActive,
			};

			const saved =
				selectedBrandId === null
					? await api.createAdminBrand(payload)
					: await api.updateAdminBrand(selectedBrandId, payload);

			loadIntoForm(saved);
			await loadBrands(appliedSearchQuery);
			captureSavedSnapshot();
			notices.setSuccess(isUpdate ? "Brand updated." : "Brand created.");
		} catch (error) {
			console.error(error);
			const err = error as { body?: { error?: string } };
			notices.setError(err.body?.error ?? "Unable to save brand.");
		} finally {
			saving = false;
		}
	}

	async function deleteBrand() {
		if (selectedBrandId === null || deleting) {
			return;
		}

		if (!window.confirm(`Delete brand "${name.trim() || "this brand"}"?`)) {
			return;
		}

		deleting = true;
		try {
			await api.deleteAdminBrand(selectedBrandId);
			resetForm();
			await loadBrands(appliedSearchQuery);
			notices.setSuccess("Brand deleted.");
		} catch (error) {
			console.error(error);
			const err = error as { body?: { error?: string } };
			notices.setError(err.body?.error ?? "Unable to delete brand.");
		} finally {
			deleting = false;
		}
	}

	onMount(() => {
		captureSavedSnapshot();
		void loadBrands();
	});

	onDestroy(() => {
		clearLogoPreview();
	});

	$effect(() => {
		savePrompt.dirty = formHasUnsavedChanges;
		savePrompt.blocked = saving || deleting || uploadingLogo;
		savePrompt.saveAction = formHasUnsavedChanges ? saveBrand : null;
	});
</script>

{#snippet brandActions()}
	<AdminResourceActions
		searchFullWidth={true}
		searchClass="sm:max-w-xs"
		searchPlaceholder="Search brands"
		bind:searchValue={searchQuery}
		onSearch={applyBrandSearch}
		onRefresh={refreshBrands}
		searchRefreshing={loading}
		searchDisabled={loading}
	/>
{/snippet}

{#snippet brandHeaderActions()}
	<AdminResourceActions countLabel={`${brands.length} brands`} />
{/snippet}

<section class="space-y-6">
	<AdminPageHeader title="Brands" actions={brandHeaderActions} />

	<AdminMasterDetailLayout class="mt-6" columnsClass="xl:grid-cols-[0.95fr_1.05fr]">
		{#snippet master()}
			<AdminPanel title="Brands" headerActions={brandActions}>
				<div class="space-y-3">
					{#if hasLoadError}
						<p class="admin-empty-state admin-empty-state-error">Failed to load brands.</p>
					{:else if loading && brands.length === 0}
						<p class="admin-empty-state">Loading brands...</p>
					{:else if brands.length === 0 && hasAppliedSearch}
						<p class="admin-empty-state">Your search didn&apos;t match any brands.</p>
					{:else if brands.length === 0}
						<p class="admin-empty-state">There are no brands.</p>
					{:else}
						{#each brands as brand (brand.id)}
							<button
								type="button"
								class={`admin-list-item flex w-full cursor-pointer items-center justify-between gap-3 p-4 text-left ${
									selectedBrandId === brand.id
										? "admin-list-item-active"
										: "admin-list-item-interactive"
								}`}
								onclick={() => loadIntoForm(brand)}
							>
								<div class="min-w-0">
									<div class="flex flex-wrap items-center gap-2">
										<p class="truncate text-sm font-semibold text-stone-950 dark:text-stone-50">
											{brand.name}
										</p>
										<Badge tone="neutral">/{brand.slug}</Badge>
									</div>
									<p class="mt-1 line-clamp-2 text-xs text-stone-500 dark:text-stone-400">
										{brand.description || "No description"}
									</p>
								</div>
								<Badge tone={brand.is_active ? "success" : "neutral"}>
									{brand.is_active ? "Active" : "Inactive"}
								</Badge>
							</button>
						{/each}
					{/if}
				</div>
			</AdminPanel>
		{/snippet}

		{#snippet detail()}
			<AdminPanel
				title={isEditingBrand ? "Edit Brand" : "New Brand"}
				meta={isEditingBrand ? `ID ${selectedBrandId}` : ""}
			>
				<div class="grid gap-5 md:grid-cols-2">
					<label class="block">
						<span class="admin-label">Name</span>
						<TextInput tone="admin" class="mt-2 w-full" type="text" bind:value={name} />
					</label>

					<label class="block">
						<span class="admin-label">Slug</span>
						<TextInput tone="admin" class="mt-2 w-full" type="text" bind:value={slug} />
					</label>

					<label class="block md:col-span-2">
						<span class="admin-label">Description</span>
						<TextArea tone="admin" class="mt-2 min-h-32" bind:value={description} />
					</label>

					<div class="admin-muted-surface md:col-span-2">
						<div class="flex flex-wrap items-center justify-between gap-3">
							<p class="text-sm text-stone-700 dark:text-stone-200">Brand image</p>
							<div class="flex items-center gap-2">
								<ButtonInput
									tone="admin"
									type="file"
									accept="image/*"
									onchange={(event) => void uploadLogo(event)}
									disabled={uploadingLogo || saving || deleting}
									variant="regular"
									size="small"
									class="rounded-full"
								>
									<i class="bi bi-upload mr-1"></i>
									{uploadingLogo ? "Uploading..." : "Upload image"}
								</ButtonInput>
								<IconButton
									tone="admin"
									variant="danger"
									type="button"
									onclick={removeLogo}
									disabled={!logoMediaId && !logoPreviewUrl}
									aria-label="Remove brand image"
									class="bg-white/85 dark:bg-stone-950/80"
								>
									<i class="bi bi-trash"></i>
								</IconButton>
							</div>
						</div>

						{#if logoImageUrl}
							<img
								src={logoImageUrl}
								alt="Brand preview"
								class="mt-3 h-36 w-full rounded-xl bg-stone-100 object-contain dark:bg-stone-900"
							/>
						{/if}
					</div>
				</div>

				<div class="admin-muted-surface mt-5">
					<label class="flex items-center justify-between gap-4">
						<div>
							<p class="text-sm font-semibold text-stone-950 dark:text-stone-50">
								Active in storefront
							</p>
							<p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
								Inactive brands remain assignable in admin but are hidden from public brand
								listings.
							</p>
						</div>
						<input class="h-4 w-4 shrink-0" type="checkbox" bind:checked={isActive} />
					</label>
				</div>

				<div class="mt-6 flex flex-wrap items-center gap-3">
					<Button
						tone="admin"
						type="button"
						variant="primary"
						class="rounded-full whitespace-nowrap"
						disabled={saving || deleting || uploadingLogo}
						onclick={() => void saveBrand()}
					>
						<i class="bi {isEditingBrand ? 'bi-floppy-fill' : 'bi-plus-lg'} mr-1"></i>
						{saving ? "Saving..." : isEditingBrand ? "Save changes" : "Create brand"}
					</Button>
					<Button
						tone="admin"
						type="button"
						variant="regular"
						class="rounded-full whitespace-nowrap"
						disabled={saving || deleting || uploadingLogo}
						onclick={resetForm}
					>
						<i class="bi bi-x-lg mr-1"></i>
						Clear
					</Button>
					{#if isEditingBrand}
						<Button
							tone="admin"
							type="button"
							variant="danger"
							class="rounded-full whitespace-nowrap"
							disabled={saving || deleting || uploadingLogo}
							onclick={() => void deleteBrand()}
						>
							<i class="bi bi-trash mr-1"></i>
							{deleting ? "Deleting..." : "Delete brand"}
						</Button>
					{/if}
				</div>
			</AdminPanel>
		{/snippet}
	</AdminMasterDetailLayout>
</section>

<AdminFloatingNotices
	showUnsaved={savePrompt.dirty}
	unsavedMessage="You have unsaved brand changes."
	canSaveUnsaved={savePrompt.canSave}
	onSaveUnsaved={() => void savePrompt.save()}
	savingUnsaved={savePrompt.saving}
	statusMessage={notices.message}
	statusTone={notices.tone}
	onDismissStatus={notices.clear}
/>
