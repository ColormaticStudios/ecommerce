<script lang="ts">
	import { type API } from "$lib/api";
	import AdminSearchForm from "$lib/admin/AdminSearchForm.svelte";
	import { API_BASE_URL } from "$lib/config";
	import Button from "$lib/components/Button.svelte";
	import ButtonInput from "$lib/components/ButtonInput.svelte";
	import IconButton from "$lib/components/IconButton.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
	import { type BrandModel } from "$lib/models";
	import { getContext, onDestroy, onMount } from "svelte";

	type NoticeCallback = (message: string) => void;

	interface Props {
		onError?: NoticeCallback;
		onStatus?: NoticeCallback;
	}

	const api: API = getContext("api");

	let { onError, onStatus }: Props = $props();

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

	const hasAppliedSearch = $derived(appliedSearchQuery.length > 0);
	const isEditingBrand = $derived(selectedBrandId !== null);
	const logoImageUrl = $derived.by(() => {
		if (logoPreviewUrl) {
			return logoPreviewUrl;
		}
		if (!logoMediaId.trim()) {
			return null;
		}
		return `${API_BASE_URL}/media/${encodeURIComponent(logoMediaId.trim())}/original.webp`;
	});

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
	}

	function loadIntoForm(brand: BrandModel) {
		clearLogoPreview();
		selectedBrandId = brand.id;
		name = brand.name;
		slug = brand.slug;
		description = brand.description ?? "";
		logoMediaId = brand.logo_media_id ?? "";
		isActive = brand.is_active;
	}

	async function loadBrands(query = appliedSearchQuery) {
		loading = true;
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
		} catch (err) {
			console.error(err);
			onError?.("Unable to load brands.");
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
			onStatus?.("Brand image uploaded. Save brand to keep this logo.");
		} catch (err) {
			console.error(err);
			clearLogoPreview();
			onError?.("Unable to upload brand image.");
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
		onStatus?.("Brand image removed. Save brand to keep this change.");
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
			onStatus?.(isUpdate ? "Brand updated." : "Brand created.");
		} catch (err) {
			console.error(err);
			const error = err as { body?: { error?: string } };
			onError?.(error.body?.error ?? "Unable to save brand.");
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
			onStatus?.("Brand deleted.");
		} catch (err) {
			console.error(err);
			const error = err as { body?: { error?: string } };
			onError?.(error.body?.error ?? "Unable to delete brand.");
		} finally {
			deleting = false;
		}
	}

	onMount(() => {
		void loadBrands();
	});

	onDestroy(() => {
		clearLogoPreview();
	});
</script>

<div class="mt-6 grid items-start gap-6 xl:grid-cols-[0.95fr_1.05fr]">
	<section
		class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
	>
		<div class="flex flex-wrap items-center justify-between gap-3">
			<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Brands</h2>
			<AdminSearchForm
				fullWidth={true}
				class="sm:max-w-xs"
				placeholder="Search brands"
				bind:value={searchQuery}
				onSearch={applyBrandSearch}
				onRefresh={refreshBrands}
				refreshing={loading}
				disabled={loading}
			/>
		</div>

		<div class="mt-5 space-y-3">
			{#if loading && brands.length === 0}
				<p
					class="rounded-2xl border border-dashed border-gray-300 px-4 py-8 text-sm text-gray-500 dark:border-gray-700 dark:text-gray-400"
				>
					Loading brands...
				</p>
			{:else if brands.length === 0 && hasAppliedSearch}
				<p
					class="rounded-2xl border border-dashed border-gray-300 px-4 py-8 text-sm text-gray-500 dark:border-gray-700 dark:text-gray-400"
				>
					Your search didn&apos;t match any brands.
				</p>
			{:else if brands.length === 0}
				<p
					class="rounded-2xl border border-dashed border-gray-300 px-4 py-8 text-sm text-gray-500 dark:border-gray-700 dark:text-gray-400"
				>
					There are no brands.
				</p>
			{:else}
				{#each brands as brand (brand.id)}
					<button
						type="button"
						class={`flex w-full cursor-pointer items-center justify-between gap-3 rounded-xl border px-4 py-3 text-left transition ${
							selectedBrandId === brand.id
								? "border-gray-900 bg-gray-50 shadow-sm dark:border-gray-100 dark:bg-gray-800"
								: "border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50 dark:border-gray-800 dark:bg-gray-900 dark:hover:border-gray-700 dark:hover:bg-gray-800"
						}`}
						onclick={() => loadIntoForm(brand)}
					>
						<div class="min-w-0">
							<div class="flex flex-wrap items-center gap-2">
								<p class="truncate text-sm font-semibold text-gray-900 dark:text-gray-100">
									{brand.name}
								</p>
								<span
									class="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-600 dark:bg-gray-800 dark:text-gray-300"
								>
									/{brand.slug}
								</span>
							</div>
							<p class="mt-1 line-clamp-2 text-xs text-gray-500 dark:text-gray-400">
								{brand.description || "No description"}
							</p>
						</div>
						<span
							class={`shrink-0 rounded-full px-2.5 py-1 text-[11px] font-semibold ${
								brand.is_active
									? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200"
									: "bg-gray-200 text-gray-600 dark:bg-gray-800 dark:text-gray-300"
							}`}
						>
							{brand.is_active ? "Active" : "Inactive"}
						</span>
					</button>
				{/each}
			{/if}
		</div>
	</section>

	<section
		class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
	>
		<div class="flex flex-wrap items-start justify-between gap-3">
			<div>
				<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
					{isEditingBrand ? "Edit Brand" : "New Brand"}
				</h2>
				<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
					Brands appear in storefront filters and can be assigned from the product editor.
				</p>
			</div>
			{#if isEditingBrand}
				<div
					class="rounded-full bg-gray-100 px-3 py-1 text-xs font-semibold text-gray-600 dark:bg-gray-800 dark:text-gray-300"
				>
					ID {selectedBrandId}
				</div>
			{/if}
		</div>

		<div class="mt-6 grid gap-5 md:grid-cols-2">
			<label class="block">
				<span class="text-xs font-semibold tracking-[0.2em] text-gray-500 uppercase">Name</span>
				<TextInput class="mt-2 w-full" type="text" bind:value={name} />
			</label>

			<label class="block">
				<span class="text-xs font-semibold tracking-[0.2em] text-gray-500 uppercase">Slug</span>
				<TextInput class="mt-2 w-full" type="text" bind:value={slug} />
			</label>

			<label class="block md:col-span-2">
				<span class="text-xs font-semibold tracking-[0.2em] text-gray-500 uppercase">
					Description
				</span>
				<textarea
					class="mt-2 min-h-32 w-full rounded-md border border-gray-300 bg-gray-200 px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-800"
					bind:value={description}
				></textarea>
			</label>

			<div class="rounded-lg border border-gray-200 p-3 md:col-span-2 dark:border-gray-800">
				<div class="flex flex-wrap items-center justify-between gap-3">
					<div>
						<p class="text-sm text-gray-600 dark:text-gray-300">Brand image</p>
						<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
							Upload a logo or brand image for storefront use.
						</p>
					</div>
					<div class="flex items-center gap-2">
						<ButtonInput
							type="file"
							accept="image/*"
							onchange={(event) => void uploadLogo(event)}
							disabled={uploadingLogo || saving || deleting}
							variant="regular"
							size="small"
						>
							<i class="bi bi-upload mr-1"></i>
							{uploadingLogo ? "Uploading..." : "Upload image"}
						</ButtonInput>
						<IconButton
							variant="danger"
							type="button"
							onclick={removeLogo}
							disabled={!logoMediaId && !logoPreviewUrl}
							aria-label="Remove brand image"
						>
							<i class="bi bi-trash"></i>
						</IconButton>
					</div>
				</div>

				{#if logoImageUrl}
					<img
						src={logoImageUrl}
						alt="Brand preview"
						class="mt-3 h-36 w-full rounded-md bg-gray-100 object-contain dark:bg-gray-800"
					/>
				{/if}
			</div>
		</div>

		<div
			class="mt-5 rounded-2xl border border-gray-200 bg-gray-50 px-4 py-3 dark:border-gray-800 dark:bg-gray-950/40"
		>
			<label class="flex items-center justify-between gap-4">
				<div>
					<p class="text-sm font-semibold text-gray-900 dark:text-gray-100">Active in storefront</p>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
						Inactive brands remain assignable in admin but are hidden from public brand listings.
					</p>
				</div>
				<input class="h-4 w-4 shrink-0" type="checkbox" bind:checked={isActive} />
			</label>
		</div>

		<div class="mt-6 flex flex-wrap items-center gap-3">
			<Button
				type="button"
				variant="primary"
				class="whitespace-nowrap"
				disabled={saving || deleting || uploadingLogo}
				onclick={() => void saveBrand()}
			>
				<i class="bi {isEditingBrand ? 'bi-floppy-fill' : 'bi-plus-lg'} mr-1"></i>
				{saving ? "Saving..." : isEditingBrand ? "Save changes" : "Create brand"}
			</Button>
			<Button
				type="button"
				variant="regular"
				class="whitespace-nowrap"
				disabled={saving || deleting || uploadingLogo}
				onclick={resetForm}
			>
				Clear
			</Button>
			{#if isEditingBrand}
				<Button
					type="button"
					variant="danger"
					class="whitespace-nowrap"
					disabled={saving || deleting || uploadingLogo}
					onclick={() => void deleteBrand()}
				>
					{deleting ? "Deleting..." : "Delete brand"}
				</Button>
			{/if}
		</div>
	</section>
</div>
