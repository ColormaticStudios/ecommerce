<script lang="ts">
	import { type API } from "$lib/api";
	import Alert from "$lib/components/alert.svelte";
	import { type ProductModel, type RelatedProductModel } from "$lib/models";
	import { uploadMediaFiles } from "$lib/media";
	import { getContext } from "svelte";

	interface Props {
		productId: number | null;
		initialProduct?: ProductModel | null;
		allowCreate?: boolean;
		clearOnDelete?: boolean;
		layout?: "stacked" | "split";
		showHeader?: boolean;
		showClear?: boolean;
		showMessages?: boolean;
		onProductCreated?: (product: ProductModel) => void;
		onProductUpdated?: (product: ProductModel) => void;
		onProductDeleted?: (productId: number) => void;
		onErrorMessage?: (message: string) => void;
		onStatusMessage?: (message: string) => void;
	}

	let {
		productId = $bindable(),
		initialProduct = null,
		allowCreate = false,
		clearOnDelete = false,
		layout = "stacked",
		showHeader = true,
		showClear = true,
		showMessages = true,
		onProductCreated,
		onProductUpdated,
		onProductDeleted,
		onErrorMessage,
		onStatusMessage,
	}: Props = $props();

	const api: API = getContext("api");

	let product = $state<ProductModel | null>(null);
	let loading = $state(false);
	let saving = $state(false);
	let deleting = $state(false);
	let uploading = $state(false);
	let mediaDeleting = $state<string | null>(null);
	let mediaReordering = $state(false);
	let relatedLoading = $state(false);
	let relatedSaving = $state(false);
	let errorMessage = $state("");
	let statusMessage = $state("");

	let sku = $state("");
	let name = $state("");
	let description = $state("");
	let price = $state("");
	let stock = $state("");
	let mediaFiles = $state<FileList | null>(null);
	let mediaInputRef = $state<HTMLInputElement | null>(null);
	let pendingMediaOrder = $state<string[] | null>(null);
	let relatedQuery = $state("");
	let relatedOptions = $state<ProductModel[]>([]);
	let relatedSelected = $state<RelatedProductModel[]>([]);

	const mediaFilesCount = $derived(mediaFiles ? mediaFiles.length : 0);
	const mediaOrderView = $derived(pendingMediaOrder ?? product?.images ?? []);
	const hasPendingMediaOrder = $derived(
		pendingMediaOrder != null &&
			product?.images != null &&
			pendingMediaOrder.join("|") !== product.images.join("|")
	);
	const resolvedProductId = $derived(
		productId != null && Number.isFinite(productId) && productId > 0 ? productId : null
	);
	const hasProduct = $derived(Boolean(product));
	const canEditProduct = $derived(resolvedProductId != null);

	let loadSequence = 0;
	let lastLoadedId = $state<number | null>(null);

	function clearMessages() {
		if (showMessages) {
			errorMessage = "";
			statusMessage = "";
		}
		onErrorMessage?.("");
		onStatusMessage?.("");
	}

	function setError(message: string) {
		if (showMessages) {
			errorMessage = message;
		}
		onErrorMessage?.(message);
	}

	function setStatus(message: string) {
		if (showMessages) {
			statusMessage = message;
		}
		onStatusMessage?.(message);
	}

	function resetForm() {
		sku = "";
		name = "";
		description = "";
		price = "";
		stock = "";
		mediaFiles = null;
		pendingMediaOrder = null;
		relatedQuery = "";
		relatedOptions = [];
		relatedSelected = [];
	}

	function hydrateForm(value: ProductModel) {
		sku = value.sku;
		name = value.name;
		description = value.description ?? "";
		price = String(value.price ?? "");
		stock = String(value.stock ?? "");
		pendingMediaOrder = null;
		relatedSelected = value.related_products ?? [];
	}

	function extractMediaId(url: string): string | null {
		try {
			const base = typeof window === "undefined" ? "http://localhost" : window.location.origin;
			const parsed = new URL(url, base);
			const segments = parsed.pathname.split("/").filter(Boolean);
			const mediaIndex = segments.indexOf("media");
			if (mediaIndex >= 0 && segments.length > mediaIndex + 1) {
				return segments[mediaIndex + 1];
			}
			return segments.length > 1 ? segments[segments.length - 2] : null;
		} catch {
			return null;
		}
	}

	async function loadProduct(id: number, seedProduct?: ProductModel | null) {
		const sequence = ++loadSequence;
		loading = true;
		clearMessages();
		if (!seedProduct) {
			product = null;
			resetForm();
		}
		try {
			const fetched = await api.getProduct(id);
			if (sequence !== loadSequence) {
				return;
			}
			product = fetched;
			hydrateForm(fetched);
			onProductUpdated?.(fetched);
		} catch (err) {
			console.error(err);
			if (sequence === loadSequence) {
				setError("Unable to load product.");
			}
		} finally {
			if (sequence === loadSequence) {
				loading = false;
			}
		}
	}

	async function saveProduct() {
		clearMessages();
		saving = true;
		try {
			const trimmedStock = String(stock ?? "").trim();
			const payload = {
				sku: sku.trim(),
				name: name.trim(),
				description: description.trim() || undefined,
				price: Number(price),
				stock: trimmedStock === "" ? undefined : Number(trimmedStock),
			};

			if (!payload.sku || !payload.name || Number.isNaN(payload.price)) {
				setError("Please provide SKU, name, and a valid price.");
				return;
			}

			let updated: ProductModel;
			if (resolvedProductId) {
				updated = await api.updateProduct(resolvedProductId, payload);
				const merged = {
					...updated,
					images:
						updated.images?.length || !product?.images?.length ? updated.images : product.images,
				};
				product = merged;
				updated = merged;
				hydrateForm(merged);
				onProductUpdated?.(merged);
				setStatus("Product updated.");
			} else if (allowCreate) {
				updated = await api.createProduct(payload);
				product = updated;
				productId = updated.id;
				hydrateForm(updated);
				onProductCreated?.(updated);
				onProductUpdated?.(updated);
				setStatus("Product created.");
			} else {
				setError("Please select a product to edit.");
			}
		} catch (err) {
			console.error(err);
			setError("Unable to save product.");
		} finally {
			saving = false;
		}
	}

	async function deleteProduct() {
		if (!resolvedProductId) {
			return;
		}
		if (!confirm("Delete this product? This cannot be undone.")) {
			return;
		}
		clearMessages();
		deleting = true;
		try {
			const deletedId = resolvedProductId;
			await api.deleteProduct(deletedId);
			product = null;
			resetForm();
			onProductDeleted?.(deletedId);
			setStatus("Product deleted.");
			if (clearOnDelete) {
				productId = null;
			}
		} catch (err) {
			console.error(err);
			setError("Unable to delete product.");
		} finally {
			deleting = false;
		}
	}

	async function uploadMedia() {
		if (!resolvedProductId || !mediaFiles || mediaFiles.length === 0) {
			return;
		}
		clearMessages();
		uploading = true;
		try {
			const mediaIds = await uploadMediaFiles(api, mediaFiles);
			const updated = await api.attachProductMedia(resolvedProductId, mediaIds);
			product = updated;
			hydrateForm(updated);
			onProductUpdated?.(updated);
			setStatus("Media attached.");
		} catch (err) {
			console.error(err);
			const error = err as { status?: number; body?: { error?: string } };
			if (error.status === 409 && error.body?.error) {
				setError(error.body.error);
			} else {
				setError("Unable to upload media.");
			}
		} finally {
			uploading = false;
		}
	}

	async function detachMedia(mediaUrl: string) {
		if (!resolvedProductId) {
			return;
		}
		const mediaId = extractMediaId(mediaUrl);
		if (!mediaId) {
			setError("Unable to find media ID for deletion.");
			return;
		}
		if (!confirm("Remove this image from the product?")) {
			return;
		}
		clearMessages();
		mediaDeleting = mediaId;
		try {
			const updated = await api.detachProductMedia(resolvedProductId, mediaId);
			product = updated;
			hydrateForm(updated);
			onProductUpdated?.(updated);
			setStatus("Media removed.");
		} catch (err) {
			console.error(err);
			setError("Unable to remove media.");
		} finally {
			mediaDeleting = null;
		}
	}

	function moveMedia(index: number, direction: -1 | 1) {
		if (!mediaOrderView.length) {
			return;
		}
		const nextIndex = index + direction;
		if (nextIndex < 0 || nextIndex >= mediaOrderView.length) {
			return;
		}

		const reordered = [...mediaOrderView];
		[reordered[index], reordered[nextIndex]] = [reordered[nextIndex], reordered[index]];
		pendingMediaOrder = reordered;
	}

	async function saveMediaOrder() {
		if (!resolvedProductId || !hasPendingMediaOrder || !pendingMediaOrder) {
			return;
		}

		const mediaIds = pendingMediaOrder
			.map((url) => extractMediaId(url))
			.filter((id): id is string => Boolean(id));

		if (mediaIds.length !== pendingMediaOrder.length) {
			setError("Unable to reorder media.");
			return;
		}

		mediaReordering = true;
		clearMessages();
		try {
			const updated = await api.updateProductMediaOrder(resolvedProductId, mediaIds);
			product = updated;
			pendingMediaOrder = null;
			onProductUpdated?.(updated);
			setStatus("Image order updated.");
		} catch (err) {
			console.error(err);
			setError("Unable to update image order.");
		} finally {
			mediaReordering = false;
		}
	}

	async function searchRelatedProducts() {
		if (!resolvedProductId || !relatedQuery.trim()) {
			relatedOptions = [];
			return;
		}
		relatedLoading = true;
		try {
			const page = await api.listProducts({
				q: relatedQuery.trim(),
				page: 1,
				limit: 10,
			});
			relatedOptions = page.data.filter(
				(item) =>
					item.id !== resolvedProductId &&
					!relatedSelected.some((selected) => selected.id === item.id)
			);
		} catch (err) {
			console.error(err);
			setError("Unable to search related products.");
		} finally {
			relatedLoading = false;
		}
	}

	function addRelatedProduct(option: ProductModel) {
		if (relatedSelected.some((item) => item.id === option.id)) {
			return;
		}
		relatedSelected = [
			...relatedSelected,
			{
				id: option.id,
				sku: option.sku,
				name: option.name,
				price: option.price,
			},
		];
		relatedOptions = relatedOptions.filter((item) => item.id !== option.id);
	}

	function removeRelatedProduct(productIdToRemove: number) {
		relatedSelected = relatedSelected.filter((item) => item.id !== productIdToRemove);
	}

	async function saveRelatedProducts() {
		if (!resolvedProductId) {
			return;
		}
		relatedSaving = true;
		clearMessages();
		try {
			const updated = await api.updateProductRelated(
				resolvedProductId,
				relatedSelected.map((item) => item.id)
			);
			product = updated;
			hydrateForm(updated);
			onProductUpdated?.(updated);
			setStatus("Related products updated.");
		} catch (err) {
			console.error(err);
			setError("Unable to update related products.");
		} finally {
			relatedSaving = false;
		}
	}

	function clearSelection() {
		productId = null;
		product = null;
		resetForm();
		clearMessages();
	}

	$effect(() => {
		if (resolvedProductId) {
			const seed =
				initialProduct && initialProduct.id === resolvedProductId ? initialProduct : null;
			if (seed && (!product || product.id !== seed.id)) {
				product = seed;
				hydrateForm(seed);
			}
			if (resolvedProductId !== lastLoadedId) {
				lastLoadedId = resolvedProductId;
				void loadProduct(resolvedProductId, seed);
			}
		} else {
			loadSequence += 1;
			loading = false;
			product = null;
			resetForm();
			clearMessages();
			lastLoadedId = null;
		}
	});
</script>

{#if showMessages}
	{#if errorMessage}
		<div class="mt-6">
			<Alert
				message={errorMessage}
				tone="error"
				icon="bi-x-circle-fill"
				onClose={() => {
					errorMessage = "";
					onErrorMessage?.("");
				}}
			/>
		</div>
	{/if}
	{#if statusMessage}
		<div class="mt-6">
			<Alert
				message={statusMessage}
				tone="success"
				icon="bi-check-circle-fill"
				onClose={() => {
					statusMessage = "";
					onStatusMessage?.("");
				}}
			/>
		</div>
	{/if}
{/if}

{#if loading && !hasProduct}
	<p class="mt-6 text-sm text-gray-500 dark:text-gray-400">Loading product…</p>
{:else if !allowCreate && !hasProduct}
	<p class="mt-6 text-sm text-gray-500 dark:text-gray-400">Product not found.</p>
{:else if layout === "split"}
	<div class="mt-6 grid gap-6 lg:grid-cols-[1.2fr_0.8fr]">
		<div
			class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
		>
			<div class="space-y-4 text-sm">
				<div>
					<label for="admin-product-sku" class="text-xs tracking-[0.2em] text-gray-500 uppercase">
						SKU
					</label>
					<input id="admin-product-sku" class="textinput mt-1" type="text" bind:value={sku} />
				</div>
				<div>
					<label for="admin-product-name" class="text-xs tracking-[0.2em] text-gray-500 uppercase">
						Name
					</label>
					<input id="admin-product-name" class="textinput mt-1" type="text" bind:value={name} />
				</div>
				<div>
					<label
						for="admin-product-description"
						class="text-xs tracking-[0.2em] text-gray-500 uppercase"
					>
						Description
					</label>
					<textarea
						id="admin-product-description"
						class="mt-1 w-full rounded-md border border-gray-300 bg-gray-200 px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-800"
						rows="4"
						bind:value={description}
					></textarea>
				</div>
				<div class="grid gap-4 sm:grid-cols-2">
					<div>
						<label
							for="admin-product-price"
							class="text-xs tracking-[0.2em] text-gray-500 uppercase"
						>
							Price
						</label>
						<input
							id="admin-product-price"
							class="textinput mt-1"
							type="number"
							min="0"
							step="0.01"
							bind:value={price}
						/>
					</div>
					<div>
						<label
							for="admin-product-stock"
							class="text-xs tracking-[0.2em] text-gray-500 uppercase"
						>
							Stock
						</label>
						<input
							id="admin-product-stock"
							class="textinput mt-1"
							type="number"
							min="0"
							bind:value={stock}
						/>
					</div>
				</div>
			</div>

			<div class="mt-6 flex flex-wrap justify-between">
				<button
					class="btn btn-primary cursor-pointer"
					type="button"
					onclick={saveProduct}
					disabled={saving}
				>
					<i class="bi bi-floppy-fill mr-1"></i>
					{saving ? "Saving..." : "Save changes"}
				</button>
				<button
					class="btn btn-danger cursor-pointer"
					type="button"
					disabled={deleting}
					onclick={deleteProduct}
				>
					<i class="bi bi-trash-fill mr-1"></i>
					{deleting ? "Deleting..." : "Delete product"}
				</button>
			</div>
		</div>

		<div
			class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
		>
			<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">Media</h2>
			{#if mediaOrderView.length}
				<div class="mt-4 grid grid-cols-2 gap-3">
					{#each mediaOrderView as image, index (image)}
						<div
							class="relative overflow-hidden rounded-lg border border-gray-200 dark:border-gray-800"
						>
							<img
								src={image}
								alt={product ? `${product.name} ${index + 1}` : `Product image ${index + 1}`}
								class="h-28 w-full object-cover"
							/>
							<button
								class="absolute top-2 right-2 flex h-8 w-8 cursor-pointer items-center justify-center rounded-full bg-white/90 text-gray-700 shadow-sm transition hover:bg-white dark:bg-gray-900/80 dark:text-gray-200 dark:hover:bg-gray-800"
								type="button"
								disabled={mediaDeleting !== null || mediaReordering}
								onclick={() => detachMedia(image)}
								aria-label="Remove image"
							>
								{#if mediaDeleting && extractMediaId(image) === mediaDeleting}
									<i class="bi bi-arrow-repeat animate-spin"></i>
								{:else}
									<i class="bi bi-trash"></i>
								{/if}
							</button>
							<div class="absolute right-2 bottom-2 flex gap-1">
								<button
									class="flex h-8 w-8 cursor-pointer items-center justify-center rounded-full bg-white/90 text-gray-700 shadow-sm transition hover:bg-white disabled:cursor-not-allowed disabled:opacity-50 dark:bg-gray-900/80 dark:text-gray-200 dark:hover:bg-gray-800"
									type="button"
									disabled={mediaReordering || index === 0}
									onclick={() => moveMedia(index, -1)}
									aria-label="Move image up"
								>
									<i class="bi bi-chevron-left"></i>
								</button>
								<button
									class="flex h-8 w-8 cursor-pointer items-center justify-center rounded-full bg-white/90 text-gray-700 shadow-sm transition hover:bg-white disabled:cursor-not-allowed disabled:opacity-50 dark:bg-gray-900/80 dark:text-gray-200 dark:hover:bg-gray-800"
									type="button"
									disabled={mediaReordering || index === mediaOrderView.length - 1}
									onclick={() => moveMedia(index, 1)}
									aria-label="Move image down"
								>
									<i class="bi bi-chevron-right"></i>
								</button>
							</div>
						</div>
					{/each}
				</div>
				{#if hasPendingMediaOrder}
					<button
						class="btn btn-primary mt-3 cursor-pointer"
						type="button"
						disabled={mediaReordering}
						onclick={saveMediaOrder}
					>
						<i class="bi bi-floppy-fill mr-1"></i>
						{mediaReordering ? "Saving..." : "Save order"}
					</button>
				{/if}
			{:else}
				<p class="mt-4 text-sm text-gray-500 dark:text-gray-400">No images yet.</p>
			{/if}

			<div class="mt-6 rounded-xl border border-dashed border-gray-300 p-4 dark:border-gray-700">
				<p class="text-xs tracking-[0.2em] text-gray-500 uppercase">Upload media</p>
				<input
					class="hidden"
					type="file"
					accept="image/*"
					multiple
					bind:this={mediaInputRef}
					onchange={(event) => {
						const target = event.target as HTMLInputElement;
						mediaFiles = target.files;
					}}
					disabled={!canEditProduct}
				/>
				<div class="mt-3 flex flex-wrap items-center gap-2">
					<button
						class="btn btn-regular cursor-pointer"
						type="button"
						disabled={!canEditProduct || uploading}
						onclick={() => mediaInputRef?.click()}
					>
						Choose files
					</button>
					<button
						class="btn btn-primary cursor-pointer"
						type="button"
						disabled={!canEditProduct || uploading || !mediaFilesCount}
						onclick={uploadMedia}
					>
						{uploading ? "Uploading..." : "Attach uploads"}
					</button>
					{#if mediaFilesCount}
						<span class="text-xs text-gray-500 dark:text-gray-400">{mediaFilesCount} selected</span>
					{/if}
				</div>
			</div>
		</div>
	</div>

	<div
		class="mt-6 rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
	>
		<div class="flex items-center justify-between">
			<p class="text-xs tracking-[0.2em] text-gray-500 uppercase">Related products</p>
			<button
				class="btn btn-primary cursor-pointer"
				type="button"
				disabled={!canEditProduct || relatedSaving}
				onclick={saveRelatedProducts}
			>
				<i class="bi bi-floppy-fill mr-1"></i>
				{relatedSaving ? "Saving..." : "Save related"}
			</button>
		</div>
		<form
			class="mt-3 flex flex-nowrap items-center gap-2"
			onsubmit={(event) => {
				event.preventDefault();
				searchRelatedProducts();
			}}
		>
			<input
				class="textinput min-w-0 flex-1"
				type="search"
				placeholder="Search products"
				bind:value={relatedQuery}
			/>
			<button
				class="aspect-square cursor-pointer rounded-md border border-gray-200 p-2 dark:border-gray-700"
				type="submit"
				disabled={!canEditProduct || relatedLoading}
				aria-label="Search related products"
			>
				<i class="bi bi-search"></i>
			</button>
		</form>

		{#if relatedOptions.length}
			<div class="mt-3 space-y-2">
				{#each relatedOptions as option (option.id)}
					<div
						class="flex items-center justify-between rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm dark:border-gray-800 dark:bg-gray-900"
					>
						<div>
							<p class="font-semibold text-gray-900 dark:text-gray-100">{option.name}</p>
							<p class="text-xs text-gray-500 dark:text-gray-400">SKU {option.sku}</p>
						</div>
						<button
							class="btn btn-regular cursor-pointer"
							type="button"
							onclick={() => addRelatedProduct(option)}
						>
							Add
						</button>
					</div>
				{/each}
			</div>
		{:else if relatedQuery.trim()}
			<p class="mt-3 text-xs text-gray-500 dark:text-gray-400">No matches.</p>
		{/if}

		{#if relatedSelected.length}
			<div class="mt-4 space-y-2">
				{#each relatedSelected as related (related.id)}
					<div
						class="flex items-center justify-between rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm dark:border-gray-800 dark:bg-gray-800"
					>
						<div>
							<p class="font-semibold text-gray-900 dark:text-gray-100">{related.name}</p>
							<p class="text-xs text-gray-500 dark:text-gray-400">SKU {related.sku}</p>
						</div>
						<button
							class="btn btn-danger cursor-pointer"
							type="button"
							onclick={() => removeRelatedProduct(related.id)}
						>
							<i class="bi bi-dash-circle-fill mr-1"></i>
							Remove
						</button>
					</div>
				{/each}
			</div>
		{:else}
			<p class="mt-4 text-xs text-gray-500 dark:text-gray-400">No related products selected.</p>
		{/if}
	</div>
{:else}
	<div
		class="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm dark:border-gray-800 dark:bg-gray-900"
	>
		{#if showHeader}
			<div class="flex items-center justify-between">
				<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
					{canEditProduct ? "Edit product" : "New product"}
				</h2>
				{#if showClear && canEditProduct}
					<button
						class="cursor-pointer text-xs text-gray-500 hover:underline"
						type="button"
						onclick={clearSelection}
					>
						Clear
					</button>
				{/if}
			</div>
		{/if}

		<div class="mt-4 space-y-4 text-sm">
			<div>
				<label for="sku" class="text-xs tracking-[0.2em] text-gray-500 uppercase">SKU</label>
				<input name="sku" class="textinput mt-1" type="text" bind:value={sku} />
			</div>
			<div>
				<label for="name" class="text-xs tracking-[0.2em] text-gray-500 uppercase">Name</label>
				<input name="name" class="textinput mt-1" type="text" bind:value={name} />
			</div>
			<div>
				<label for="description" class="text-xs tracking-[0.2em] text-gray-500 uppercase"
					>Description</label
				>
				<textarea
					name="description"
					class="mt-1 w-full rounded-md border border-gray-300 bg-gray-200 px-3 py-2 text-sm dark:border-gray-700 dark:bg-gray-800"
					rows="4"
					bind:value={description}
				></textarea>
			</div>
			<div class="grid gap-4 sm:grid-cols-2">
				<div>
					<label for="price" class="text-xs tracking-[0.2em] text-gray-500 uppercase">Price</label>
					<input
						name="price"
						class="textinput mt-1"
						type="number"
						min="0"
						step="0.01"
						bind:value={price}
					/>
				</div>
				<div>
					<label for="stock" class="text-xs tracking-[0.2em] text-gray-500 uppercase">Stock</label>
					<input name="stock" class="textinput mt-1" type="number" min="0" bind:value={stock} />
				</div>
			</div>

			<div class="rounded-xl border border-dashed border-gray-300 p-4 dark:border-gray-700">
				<p class="text-xs tracking-[0.2em] text-gray-500 uppercase">Upload media</p>
				<input
					class="hidden"
					type="file"
					accept="image/*"
					multiple
					bind:this={mediaInputRef}
					onchange={(event) => {
						const target = event.target as HTMLInputElement;
						mediaFiles = target.files;
					}}
					disabled={!canEditProduct}
				/>
				<div class="mt-3 flex flex-wrap items-center gap-2">
					<button
						class="btn btn-regular cursor-pointer"
						type="button"
						disabled={!canEditProduct || uploading}
						onclick={() => mediaInputRef?.click()}
					>
						Choose files
					</button>
					<button
						class="btn btn-primary cursor-pointer"
						type="button"
						disabled={!canEditProduct || uploading || !mediaFilesCount}
						onclick={uploadMedia}
					>
						{uploading ? "Uploading..." : "Attach uploads"}
					</button>
					{#if mediaFilesCount}
						<span class="text-xs text-gray-500 dark:text-gray-400">{mediaFilesCount} selected</span>
					{/if}
				</div>
				{#if !canEditProduct}
					<p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
						Select a product to upload images.
					</p>
				{/if}
			</div>
		</div>

		{#if mediaOrderView.length}
			<div class="mt-6">
				<p class="text-xs tracking-[0.2em] text-gray-500 uppercase">Images</p>
				<div class="mt-3 grid grid-cols-2 gap-3">
					{#each mediaOrderView as imageUrl, index (imageUrl)}
						<div
							class="relative overflow-hidden rounded-lg border border-gray-200 dark:border-gray-800"
						>
							<img src={imageUrl} alt="Product" class="h-28 w-full object-cover" />
							<button
								class="absolute top-2 right-2 flex h-8 w-8 cursor-pointer items-center justify-center rounded-full bg-white/90 text-gray-700 shadow-sm transition hover:bg-white dark:bg-gray-900/80 dark:text-gray-200 dark:hover:bg-gray-800"
								type="button"
								disabled={mediaDeleting !== null || mediaReordering}
								onclick={() => detachMedia(imageUrl)}
								aria-label="Remove image"
							>
								{#if mediaDeleting && extractMediaId(imageUrl) === mediaDeleting}
									<i class="bi bi-arrow-repeat animate-spin"></i>
								{:else}
									<i class="bi bi-trash"></i>
								{/if}
							</button>
							<div class="absolute right-2 bottom-2 flex gap-1">
								<button
									class="flex h-8 w-8 cursor-pointer items-center justify-center rounded-full bg-white/90 text-gray-700 shadow-sm transition hover:bg-white disabled:cursor-not-allowed disabled:opacity-50 dark:bg-gray-900/80 dark:text-gray-200 dark:hover:bg-gray-800"
									type="button"
									disabled={mediaReordering || index === 0}
									onclick={() => moveMedia(index, -1)}
									aria-label="Move image up"
								>
									<i class="bi bi-chevron-left"></i>
								</button>
								<button
									class="flex h-8 w-8 cursor-pointer items-center justify-center rounded-full bg-white/90 text-gray-700 shadow-sm transition hover:bg-white disabled:cursor-not-allowed disabled:opacity-50 dark:bg-gray-900/80 dark:text-gray-200 dark:hover:bg-gray-800"
									type="button"
									disabled={mediaReordering || index === mediaOrderView.length - 1}
									onclick={() => moveMedia(index, 1)}
									aria-label="Move image down"
								>
									<i class="bi bi-chevron-right"></i>
								</button>
							</div>
						</div>
					{/each}
				</div>
				{#if hasPendingMediaOrder}
					<button
						class="btn btn-primary mt-3 cursor-pointer"
						type="button"
						disabled={mediaReordering}
						onclick={saveMediaOrder}
					>
						{mediaReordering ? "Saving..." : "Save order"}
					</button>
				{/if}
			</div>
		{/if}

		<div class="mt-6 border-t border-gray-200 pt-6 dark:border-gray-800">
			<div class="flex items-center justify-between">
				<p class="text-xs tracking-[0.2em] text-gray-500 uppercase">Related products</p>
				<button
					class="btn btn-primary cursor-pointer"
					type="button"
					disabled={!canEditProduct || relatedSaving}
					onclick={saveRelatedProducts}
				>
					<i class="bi bi-floppy-fill mr-1"></i>
					{relatedSaving ? "Saving..." : "Save related"}
				</button>
			</div>
			<form
				class="mt-3 flex flex-nowrap items-center gap-2"
				onsubmit={(event) => {
					event.preventDefault();
					searchRelatedProducts();
				}}
			>
				<input
					class="textinput min-w-0 flex-1"
					type="search"
					placeholder="Search products"
					bind:value={relatedQuery}
				/>
				<button
					class="aspect-square cursor-pointer rounded-md border border-gray-200 p-2 dark:border-gray-700"
					type="submit"
					disabled={!canEditProduct || relatedLoading}
					aria-label="Search related products"
				>
					<i class="bi bi-search"></i>
				</button>
			</form>

			{#if relatedOptions.length}
				<div class="mt-3 space-y-2">
					{#each relatedOptions as option (option.id)}
						<div
							class="flex items-center justify-between rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm dark:border-gray-800 dark:bg-gray-900"
						>
							<div>
								<p class="font-semibold text-gray-900 dark:text-gray-100">{option.name}</p>
								<p class="text-xs text-gray-500 dark:text-gray-400">SKU {option.sku}</p>
							</div>
							<button
								class="btn btn-regular cursor-pointer"
								type="button"
								onclick={() => addRelatedProduct(option)}
							>
								Add
							</button>
						</div>
					{/each}
				</div>
			{:else if relatedQuery.trim()}
				<p class="mt-3 text-xs text-gray-500 dark:text-gray-400">No matches.</p>
			{/if}

			{#if relatedSelected.length}
				<div class="mt-4 space-y-2">
					{#each relatedSelected as related (related.id)}
						<div
							class="flex items-center justify-between rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm dark:border-gray-800 dark:bg-gray-800"
						>
							<div>
								<p class="font-semibold text-gray-900 dark:text-gray-100">{related.name}</p>
								<p class="text-xs text-gray-500 dark:text-gray-400">SKU {related.sku}</p>
							</div>
							<button
								class="btn btn-danger cursor-pointer"
								type="button"
								onclick={() => removeRelatedProduct(related.id)}
							>
								<i class="bi bi-dash-circle-fill mr-1"></i>
								Remove
							</button>
						</div>
					{/each}
				</div>
			{:else}
				<p class="mt-4 text-xs text-gray-500 dark:text-gray-400">No related products selected.</p>
			{/if}
		</div>

		<div class="mt-6 flex flex-wrap justify-between">
			<button
				class="btn btn-primary btn-large m-0! cursor-pointer"
				type="button"
				onclick={saveProduct}
				disabled={saving}
			>
				<i
					class={`bi ${
						saving ? "bi-floppy-fill" : canEditProduct ? "bi-floppy-fill" : "bi-patch-plus-fill"
					} mr-1`}
				></i>
				{saving ? "Saving..." : canEditProduct ? "Save changes" : "Create product"}
			</button>
			{#if canEditProduct}
				<button
					class="btn btn-danger m-0! cursor-pointer"
					type="button"
					disabled={deleting}
					onclick={deleteProduct}
				>
					<i class="bi bi-trash-fill"></i>
					{deleting ? "Deleting..." : "Delete product"}
				</button>
			{/if}
		</div>
	</div>
{/if}
