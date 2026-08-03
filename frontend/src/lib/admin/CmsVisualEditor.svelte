<script lang="ts">
	import { getContext, onMount, tick } from "svelte";
	import type { API } from "$lib/api";
	import type { components } from "$lib/api/generated/openapi";
	import { searchAdminProducts } from "$lib/admin/productSearch";
	import { cmsMediaURL, type CmsContentBlock } from "$lib/cms";
	import type { CategoryModel, ProductModel } from "$lib/models";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import IconButton from "$lib/components/IconButton.svelte";
	import NumberInput from "$lib/components/NumberInput.svelte";
	import TextInput from "$lib/components/TextInput.svelte";

	type EditableBlock = CmsContentBlock & { editorId: string };
	type CmsPreviewBlock = components["schemas"]["CmsPreviewBlock"];
	type DragSource =
		| { kind: "block"; id: string }
		| { kind: "library"; type: CmsContentBlock["type"]; block: EditableBlock };

	interface Props {
		blocks: EditableBlock[];
		pageTitle: string;
		pagePath: string;
		hasUnsavedChanges?: boolean;
		canPublish?: boolean;
		saving?: boolean;
		publishing?: boolean;
		previewBlocks?: CmsPreviewBlock[];
		previewLoading?: boolean;
		previewError?: string;
		createBlock: (type: CmsContentBlock["type"]) => EditableBlock;
		onSave: () => void | Promise<void>;
		onPublish: () => void | Promise<void>;
		onRevert: () => void;
		onClose: () => void;
		onRefreshPreview: () => void | Promise<void>;
	}

	let {
		blocks = $bindable(),
		pageTitle = $bindable(),
		pagePath,
		hasUnsavedChanges = false,
		canPublish = false,
		saving = false,
		publishing = false,
		previewBlocks = [],
		previewLoading = false,
		previewError = "",
		createBlock,
		onSave,
		onPublish,
		onRevert,
		onClose,
		onRefreshPreview,
	}: Props = $props();

	const api: API = getContext("api");

	let sidebarCollapsed = $state(false);
	let selectedBlockId = $state("");
	let dragSource = $state<DragSource | null>(null);
	let insertIndex = $state<number | null>(null);
	let insertBarTop = $state(0);
	let pointerInsideCanvas = $state(false);
	let dragPointerX = $state(0);
	let dragPointerY = $state(0);
	let dragStartX = $state(0);
	let dragStartY = $state(0);
	let dragMoved = $state(false);
	let dragOffsetX = $state(0);
	let dragOffsetY = $state(0);
	let dragWidth = $state(0);
	let dragHeight = $state(0);
	let history = $state<string[]>([]);
	let historyIndex = $state(-1);
	let uploadingImage = $state(false);
	let uploadError = $state("");
	let productSearch = $state("");
	let productSearchLoading = $state(false);
	let productSearchError = $state("");
	let productResults = $state<ProductModel[]>([]);
	let productLookup = $state<Record<number, ProductModel>>({});
	let categorySearch = $state("");
	let categoriesLoading = $state(false);
	let categoryError = $state("");
	let categoryResults = $state<CategoryModel[]>([]);
	let localImagePreviews = $state<Record<string, string>>({});
	let suppressNextBlockClick = $state(false);

	const libraryBlocks: Array<{
		type: CmsContentBlock["type"];
		label: string;
		icon: string;
		description: string;
	}> = [
		{ type: "hero", label: "Hero", icon: "bi-stars", description: "Headline, media, and CTA" },
		{ type: "rich_text", label: "Text", icon: "bi-text-paragraph", description: "Editorial copy" },
		{ type: "image", label: "Image", icon: "bi-image", description: "Single media block" },
		{ type: "cta", label: "CTA", icon: "bi-cursor", description: "Standalone action" },
		{ type: "promo_banner", label: "Banner", icon: "bi-megaphone", description: "Site message" },
		{ type: "product_rail", label: "Products", icon: "bi-grid", description: "Live catalog rail" },
		{
			type: "category_tiles",
			label: "Categories",
			icon: "bi-collection",
			description: "Active category tiles",
		},
		{
			type: "promotion_highlight",
			label: "Promotion",
			icon: "bi-percent",
			description: "Campaign callout",
		},
		{
			type: "inventory_message",
			label: "Inventory",
			icon: "bi-box-seam",
			description: "Stock-aware message",
		},
		{ type: "testimonial", label: "Review", icon: "bi-chat-quote", description: "Customer proof" },
		{ type: "social_embed", label: "Social", icon: "bi-play-btn", description: "Allowlisted post" },
	];

	const selectedBlock = $derived(
		blocks.find((block) => block.editorId === selectedBlockId) ?? null
	);
	const canUndo = $derived(historyIndex > 0);
	const canRedo = $derived(historyIndex >= 0 && historyIndex < history.length - 1);
	const draggedBlock = $derived.by(() => {
		const source = dragSource;
		if (!source) return null;
		return source.kind === "block"
			? blocks.find((block) => block.editorId === source.id)
			: source.block;
	});
	const selectedPreview = $derived.by(() => {
		if (!selectedBlock) return null;
		const index = blocks.findIndex((block) => block.editorId === selectedBlock.editorId);
		return previewBlocks.find((block) => block.key === `${selectedBlock.type}:${index}`) ?? null;
	});

	onMount(() => {
		const previousBodyOverflow = document.body.style.overflow;
		const previousDocumentOverflow = document.documentElement.style.overflow;
		document.body.style.overflow = "hidden";
		document.documentElement.style.overflow = "hidden";
		resetHistory();
		selectedBlockId = blocks[0]?.editorId ?? "";
		void onRefreshPreview();
		return () => {
			document.body.style.overflow = previousBodyOverflow;
			document.documentElement.style.overflow = previousDocumentOverflow;
			clearBlockPointerDrag();
		};
	});

	function blockLabel(block: EditableBlock): string {
		return block.type.replaceAll("_", " ");
	}

	function cloneBlocks(value: EditableBlock[]): EditableBlock[] {
		return JSON.parse(JSON.stringify(value)) as EditableBlock[];
	}

	function snapshot(value = blocks): string {
		return JSON.stringify(value);
	}

	function resetHistory() {
		history = [snapshot()];
		historyIndex = 0;
	}

	function recordHistory() {
		const next = snapshot();
		if (history[historyIndex] === next) return;
		history = [...history.slice(0, historyIndex + 1), next];
		historyIndex = history.length - 1;
	}

	function restore(index: number) {
		const raw = history[index];
		if (!raw) return;
		blocks = JSON.parse(raw) as EditableBlock[];
		historyIndex = index;
		if (!blocks.some((block) => block.editorId === selectedBlockId)) {
			selectedBlockId = blocks[0]?.editorId ?? "";
		}
	}

	function undo() {
		if (canUndo) restore(historyIndex - 1);
	}

	function redo() {
		if (canRedo) restore(historyIndex + 1);
	}

	function commit(nextBlocks: EditableBlock[]) {
		blocks = cloneBlocks(nextBlocks);
		recordHistory();
		void onRefreshPreview();
	}

	function addBlock(type: CmsContentBlock["type"], afterSelected = false) {
		const block = createBlock(type);
		if (afterSelected && selectedBlockId) {
			const index = blocks.findIndex((item) => item.editorId === selectedBlockId);
			if (index >= 0) {
				commit([...blocks.slice(0, index + 1), block, ...blocks.slice(index + 1)]);
				selectedBlockId = block.editorId;
				return;
			}
		}
		commit([...blocks, block]);
		selectedBlockId = block.editorId;
	}

	function removeSelected() {
		if (!selectedBlock) return;
		const index = blocks.findIndex((block) => block.editorId === selectedBlock.editorId);
		const next = blocks.filter((block) => block.editorId !== selectedBlock.editorId);
		commit(next);
		selectedBlockId = next[Math.min(index, next.length - 1)]?.editorId ?? "";
	}

	function insertDraggedSource(targetIndex: number | null) {
		const source = dragSource;
		if (!source) return;
		const boundedIndex = Math.max(0, Math.min(targetIndex ?? blocks.length, blocks.length));
		const next = cloneBlocks(blocks);
		let block: EditableBlock;
		let finalIndex = boundedIndex;
		if (source.kind === "block") {
			const sourceIndex = next.findIndex((item) => item.editorId === source.id);
			if (sourceIndex < 0) return;
			[block] = next.splice(sourceIndex, 1);
			if (sourceIndex < finalIndex) finalIndex -= 1;
		} else {
			block = cloneBlocks([source.block])[0];
		}
		next.splice(Math.max(0, finalIndex), 0, block);
		selectedBlockId = block.editorId;
		commit(next);
	}

	function startBlockPointerDrag(event: PointerEvent, id: string) {
		event.preventDefault();
		event.stopPropagation();
		const blockElement = (event.currentTarget as HTMLElement).closest<HTMLElement>(
			"[data-cms-block-id]"
		);
		if (!blockElement) return;
		const rect = blockElement.getBoundingClientRect();
		dragSource = { kind: "block", id };
		insertIndex = blocks.findIndex((block) => block.editorId === id);
		pointerInsideCanvas = true;
		dragPointerX = event.clientX;
		dragPointerY = event.clientY;
		dragStartX = event.clientX;
		dragStartY = event.clientY;
		dragMoved = true;
		dragOffsetX = event.clientX - rect.left;
		dragOffsetY = event.clientY - rect.top;
		dragWidth = rect.width;
		dragHeight = rect.height;
		selectedBlockId = id;
		document.body.style.userSelect = "none";
		window.addEventListener("pointermove", handleBlockPointerMove);
		window.addEventListener("pointerup", handleBlockPointerUp, { once: true });
		window.addEventListener("pointercancel", handleBlockPointerCancel, { once: true });
	}

	function startLibraryPointerDrag(event: PointerEvent, item: (typeof libraryBlocks)[number]) {
		event.preventDefault();
		event.stopPropagation();
		const sourceElement = event.currentTarget as HTMLElement;
		const rect = sourceElement.getBoundingClientRect();
		const block = createBlock(item.type);
		dragSource = { kind: "library", type: item.type, block };
		insertIndex = null;
		pointerInsideCanvas = false;
		dragPointerX = event.clientX;
		dragPointerY = event.clientY;
		dragStartX = event.clientX;
		dragStartY = event.clientY;
		dragMoved = false;
		dragOffsetX = event.clientX - rect.left;
		dragOffsetY = event.clientY - rect.top;
		dragWidth = Math.min(rect.width, 360);
		dragHeight = rect.height;
		document.body.style.userSelect = "none";
		window.addEventListener("pointermove", handleBlockPointerMove);
		window.addEventListener("pointerup", handleBlockPointerUp, { once: true });
		window.addEventListener("pointercancel", handleBlockPointerCancel, { once: true });
	}

	function handleBlockPointerMove(event: PointerEvent) {
		if (!dragSource) return;
		event.preventDefault();
		dragPointerX = event.clientX;
		dragPointerY = event.clientY;
		if (Math.hypot(event.clientX - dragStartX, event.clientY - dragStartY) > 6) {
			dragMoved = true;
		}
		updateInsertionFromPoint(event.clientX, event.clientY);
	}

	function handleBlockPointerUp(event: PointerEvent) {
		event.preventDefault();
		const source = dragSource;
		if (source?.kind === "library" && !dragMoved) {
			addBlock(source.type, true);
		} else if (pointerInsideCanvas) {
			insertDraggedSource(insertIndex);
		}
		clearBlockPointerDrag();
	}

	function handleBlockPointerCancel() {
		clearBlockPointerDrag();
	}

	function clearBlockPointerDrag() {
		dragSource = null;
		insertIndex = null;
		insertBarTop = 0;
		pointerInsideCanvas = false;
		dragPointerX = 0;
		dragPointerY = 0;
		dragStartX = 0;
		dragStartY = 0;
		dragMoved = false;
		dragOffsetX = 0;
		dragOffsetY = 0;
		dragWidth = 0;
		dragHeight = 0;
		document.body.style.userSelect = "";
		window.removeEventListener("pointermove", handleBlockPointerMove);
		window.removeEventListener("pointerup", handleBlockPointerUp);
		window.removeEventListener("pointercancel", handleBlockPointerCancel);
	}

	function updateInsertionFromPoint(clientX: number, clientY: number) {
		const canvas = document.querySelector<HTMLElement>("[data-cms-canvas]");
		const canvasRect = canvas?.getBoundingClientRect();
		pointerInsideCanvas = Boolean(
			canvasRect &&
			clientX >= canvasRect.left &&
			clientX <= canvasRect.right &&
			clientY >= canvasRect.top &&
			clientY <= canvasRect.bottom
		);
		if (!pointerInsideCanvas) {
			insertIndex = null;
			insertBarTop = 0;
			return;
		}
		const blockElements = Array.from(document.querySelectorAll<HTMLElement>("[data-cms-block-id]"));
		if (blockElements.length === 0) {
			insertIndex = 0;
			insertBarTop = 0;
			return;
		}
		let nextIndex = blockElements.length;
		let nextTop = blockElements[blockElements.length - 1].getBoundingClientRect().bottom + 8;
		for (const [index, element] of blockElements.entries()) {
			const rect = element.getBoundingClientRect();
			if (clientY < rect.top + rect.height / 2) {
				nextIndex = index;
				nextTop = rect.top - 8;
				break;
			}
		}
		insertIndex = nextIndex;
		insertBarTop = nextTop;
	}

	function dragGhostStyle(): string {
		return [
			`left: ${dragPointerX - dragOffsetX}px`,
			`top: ${dragPointerY - dragOffsetY}px`,
			`width: ${dragWidth}px`,
			`min-height: ${dragHeight}px`,
		].join("; ");
	}

	function insertBarStyle(): string {
		return `top: ${insertBarTop}px`;
	}

	function updateSelected(updates: Partial<EditableBlock>, track = false) {
		if (!selectedBlock) return;
		blocks = blocks.map((block) =>
			block.editorId === selectedBlock.editorId
				? ({ ...block, ...updates } as EditableBlock)
				: block
		);
		if (track) {
			recordHistory();
			void onRefreshPreview();
		}
	}

	function commitInlineEdit() {
		recordHistory();
		void onRefreshPreview();
	}

	function commitInlineText(block: EditableBlock, key: string, event: FocusEvent) {
		const value = (event.currentTarget as HTMLElement).innerText.replace(/\r\n/g, "\n");
		if (String(block[key as keyof EditableBlock] ?? "") === value) return;
		blocks = blocks.map((candidate) =>
			candidate.editorId === block.editorId
				? ({ ...candidate, [key]: value } as EditableBlock)
				: candidate
		);
		commitInlineEdit();
	}

	function focusEditableText(block: EditableBlock, event: PointerEvent) {
		event.stopPropagation();
		suppressNextBlockClick = true;
		if (selectedBlockId === block.editorId) return;
		selectedBlockId = block.editorId;
		void tick().then(() => {
			(event.currentTarget as HTMLElement).focus();
		});
	}

	function selectBlockFromCanvas(id: string) {
		if (suppressNextBlockClick) {
			suppressNextBlockClick = false;
			return;
		}
		selectedBlockId = id;
	}

	async function uploadBlockImage(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		if (
			!file ||
			!selectedBlock ||
			(selectedBlock.type !== "image" && selectedBlock.type !== "hero")
		)
			return;
		uploadingImage = true;
		uploadError = "";
		const previewURL = URL.createObjectURL(file);
		const editorId = selectedBlock.editorId;
		const previousPreviewURL = localImagePreviews[editorId];
		if (previousPreviewURL) URL.revokeObjectURL(previousPreviewURL);
		localImagePreviews = { ...localImagePreviews, [editorId]: previewURL };
		try {
			const mediaId = await api.uploadMedia(file);
			if (selectedBlock.type === "hero") updateSelected({ image_media_id: mediaId }, true);
			else updateSelected({ media_id: mediaId }, true);
		} catch (error) {
			console.error(error);
			uploadError = "Unable to upload image.";
		} finally {
			uploadingImage = false;
			input.value = "";
		}
	}

	function imagePreview(block: Extract<EditableBlock, { type: "image" }>): string {
		return localImagePreviews[block.editorId] ?? cmsMediaURL(block.media_id);
	}

	function heroImagePreview(block: Extract<EditableBlock, { type: "hero" }>): string {
		return localImagePreviews[block.editorId] ?? cmsMediaURL(block.image_media_id);
	}

	function categoryImagePreview(
		block: Extract<EditableBlock, { type: "category_tiles" }>,
		slug: string
	): string {
		return (
			localImagePreviews[`${block.editorId}:${slug}`] ??
			cmsMediaURL(block.category_media_ids?.[slug])
		);
	}

	async function uploadCategoryImage(
		block: Extract<EditableBlock, { type: "category_tiles" }>,
		slug: string,
		event: Event
	) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		uploadingImage = true;
		uploadError = "";
		const previewKey = `${block.editorId}:${slug}`;
		const previousPreviewURL = localImagePreviews[previewKey];
		if (previousPreviewURL) URL.revokeObjectURL(previousPreviewURL);
		localImagePreviews = { ...localImagePreviews, [previewKey]: URL.createObjectURL(file) };
		try {
			const mediaID = await api.uploadMedia(file);
			updateSelected(
				{ category_media_ids: { ...block.category_media_ids, [slug]: mediaID } },
				true
			);
		} catch (error) {
			console.error(error);
			uploadError = "Unable to upload image.";
		} finally {
			uploadingImage = false;
			input.value = "";
		}
	}

	async function runProductSearch() {
		productSearchLoading = true;
		productSearchError = "";
		try {
			productResults = await searchAdminProducts(api, productSearch, 8);
			productLookup = {
				...productLookup,
				...Object.fromEntries(productResults.map((product) => [product.id, product])),
			};
		} catch (error) {
			console.error(error);
			productSearchError = "Unable to search products.";
		} finally {
			productSearchLoading = false;
		}
	}

	async function loadCategories() {
		categoriesLoading = true;
		categoryError = "";
		try {
			const categories = await api.listCategories();
			const needle = categorySearch.trim().toLowerCase();
			categoryResults = needle
				? categories.filter((category) =>
						`${category.name} ${category.description ?? ""}`.toLowerCase().includes(needle)
					)
				: categories;
		} catch (error) {
			console.error(error);
			categoryError = "Unable to load categories.";
		} finally {
			categoriesLoading = false;
		}
	}

	function toggleProduct(
		block: Extract<EditableBlock, { type: "product_rail" }>,
		product: ProductModel
	) {
		const selectedIds = block.product_ids ?? [];
		const nextIds = selectedIds.includes(product.id)
			? selectedIds.filter((id) => id !== product.id)
			: [...selectedIds, product.id];
		updateSelected({ product_ids: nextIds }, true);
	}

	function selectedProducts(block: Extract<EditableBlock, { type: "product_rail" }>) {
		return (block.product_ids ?? []).map((id) => ({
			id,
			product: productLookup[id] ?? null,
		}));
	}

	function inventoryProduct(block: Extract<EditableBlock, { type: "inventory_message" }>) {
		return productLookup[block.product_id] ?? null;
	}

	function removeProduct(
		block: Extract<EditableBlock, { type: "product_rail" }>,
		productId: number
	) {
		updateSelected(
			{ product_ids: (block.product_ids ?? []).filter((id) => id !== productId) },
			true
		);
	}

	function toggleCategory(
		block: Extract<EditableBlock, { type: "category_tiles" }>,
		category: CategoryModel
	) {
		const selectedSlugs = block.category_slugs ?? [];
		const nextSlugs = selectedSlugs.includes(category.slug)
			? selectedSlugs.filter((slug) => slug !== category.slug)
			: [...selectedSlugs, category.slug];
		const categoryMediaIDs = { ...block.category_media_ids };
		if (!nextSlugs.includes(category.slug)) delete categoryMediaIDs[category.slug];
		updateSelected({ category_slugs: nextSlugs, category_media_ids: categoryMediaIDs }, true);
	}

	function keySelect(event: KeyboardEvent, id: string) {
		if (event.key === "Enter" || event.key === " ") {
			event.preventDefault();
			selectedBlockId = id;
		}
	}

	function previewTone(status: string | undefined): "neutral" | "success" | "warning" {
		if (status === "ok") return "success";
		if (status === "degraded") return "warning";
		return "neutral";
	}

	function libraryPreviewClass(type: CmsContentBlock["type"]): string {
		switch (type) {
			case "hero":
				return "h-14 bg-stone-900 dark:bg-stone-100";
			case "promo_banner":
			case "promotion_highlight":
				return "h-10 bg-emerald-600 dark:bg-emerald-400";
			case "product_rail":
				return "grid h-14 grid-cols-3 gap-1";
			case "category_tiles":
				return "grid h-14 grid-cols-2 gap-1";
			case "testimonial":
				return "h-12 border-l-4 border-amber-400 bg-stone-100 dark:bg-stone-800";
			default:
				return "h-12 bg-stone-100 dark:bg-stone-800";
		}
	}
</script>

<div
	class="fixed inset-0 z-50 flex flex-col bg-stone-100 text-stone-950 dark:bg-stone-950 dark:text-stone-50"
>
	<header
		class="flex min-h-14 items-center justify-between gap-3 border-b border-stone-200 bg-white px-3 dark:border-stone-800 dark:bg-stone-950"
	>
		<div class="flex min-w-0 items-center gap-2">
			<IconButton
				tone="admin"
				outlined={true}
				size="sm"
				aria-label={sidebarCollapsed ? "Show component library" : "Hide component library"}
				title={sidebarCollapsed ? "Show component library" : "Hide component library"}
				onclick={() => (sidebarCollapsed = !sidebarCollapsed)}
			>
				<i class={`bi ${sidebarCollapsed ? "bi-layout-sidebar-inset" : "bi-layout-sidebar"}`}></i>
			</IconButton>
			<div class="min-w-0">
				<input
					class="w-full min-w-40 bg-transparent text-sm font-semibold outline-none"
					bind:value={pageTitle}
					onblur={recordHistory}
					aria-label="Page title"
				/>
				<p class="truncate text-xs text-stone-500">{pagePath || "Unsaved page"}</p>
			</div>
		</div>
		<div class="flex items-center gap-1">
			<IconButton
				tone="admin"
				outlined={true}
				size="sm"
				aria-label="Undo"
				title="Undo"
				onclick={undo}
				disabled={!canUndo}
			>
				<i class="bi bi-arrow-counterclockwise"></i>
			</IconButton>
			<IconButton
				tone="admin"
				outlined={true}
				size="sm"
				aria-label="Redo"
				title="Redo"
				onclick={redo}
				disabled={!canRedo}
			>
				<i class="bi bi-arrow-clockwise"></i>
			</IconButton>
			<Button tone="admin" variant="regular" size="small" onclick={onRevert}>
				<i class="bi bi-arrow-return-left mr-1"></i>
				Revert
			</Button>
			<Button tone="admin" variant="primary" size="small" onclick={onSave} disabled={saving}>
				<i class="bi bi-floppy mr-1"></i>
				{saving ? "Saving..." : "Save"}
			</Button>
			<Button
				tone="admin"
				variant="success"
				size="small"
				onclick={onPublish}
				disabled={!canPublish || publishing}
			>
				<i class="bi bi-send-check mr-1"></i>
				{publishing ? "Publishing..." : "Publish"}
			</Button>
			<Button tone="admin" variant="regular" size="small" onclick={onClose}>Done</Button>
		</div>
	</header>

	<div
		class={`grid min-h-0 flex-1 ${sidebarCollapsed ? "grid-cols-[0_minmax(0,1fr)]" : "grid-cols-[19rem_minmax(0,1fr)]"}`}
	>
		<aside
			class="min-h-0 overflow-y-auto border-r border-stone-200 bg-white transition-[width] dark:border-stone-800 dark:bg-stone-950"
			class:pointer-events-none={sidebarCollapsed}
			class:opacity-0={sidebarCollapsed}
		>
			<div class="space-y-5 p-4">
				<section>
					<div class="mb-3 flex items-center justify-between gap-2">
						<h2 class="text-sm font-semibold">Components</h2>
					</div>
					<div class="grid gap-2">
						{#each libraryBlocks as item (item.type)}
							<button
								type="button"
								class="w-full overflow-hidden rounded-lg border border-stone-200 bg-white text-left text-stone-950 transition hover:border-stone-400 hover:bg-stone-50 dark:border-stone-800 dark:bg-stone-900 dark:text-stone-50 dark:hover:border-stone-600 dark:hover:bg-stone-800"
								onpointerdown={(event) => startLibraryPointerDrag(event, item)}
							>
								<div
									class="border-b border-stone-200 bg-stone-50 p-2 dark:border-stone-800 dark:bg-stone-950"
								>
									<div class={libraryPreviewClass(item.type)}>
										{#if item.type === "product_rail"}
											<div class="rounded bg-stone-200 dark:bg-stone-700"></div>
											<div class="rounded bg-stone-300 dark:bg-stone-600"></div>
											<div class="rounded bg-stone-200 dark:bg-stone-700"></div>
										{:else if item.type === "category_tiles"}
											<div class="rounded bg-sky-100 dark:bg-sky-950"></div>
											<div class="rounded bg-emerald-100 dark:bg-emerald-950"></div>
										{:else if item.type === "rich_text"}
											<div class="space-y-1 p-2">
												<div class="h-1.5 w-3/4 rounded bg-stone-300 dark:bg-stone-600"></div>
												<div class="h-1.5 w-full rounded bg-stone-200 dark:bg-stone-700"></div>
												<div class="h-1.5 w-2/3 rounded bg-stone-200 dark:bg-stone-700"></div>
											</div>
										{/if}
									</div>
								</div>
								<span class="flex items-start gap-3 p-3">
									<i class={`bi ${item.icon} mt-0.5 text-stone-600 dark:text-stone-300`}></i>
									<span class="min-w-0">
										<span class="block text-sm font-medium">{item.label}</span>
										<span class="block text-xs leading-5 text-stone-500 dark:text-stone-400">
											{item.description}
										</span>
									</span>
								</span>
							</button>
						{/each}
					</div>
				</section>

				<section class="border-t border-stone-200 pt-4 dark:border-stone-800">
					<div class="mb-3 flex items-center justify-between gap-2">
						<h2 class="text-sm font-semibold">Selected</h2>
						{#if selectedBlock}
							<Badge tone="neutral">{blockLabel(selectedBlock)}</Badge>
						{/if}
					</div>
					{#if selectedBlock}
						<div class="space-y-3">
							{@render selectedInspector(selectedBlock)}
							<div class="flex gap-2 border-t border-stone-200 pt-3 dark:border-stone-800">
								<Button tone="admin" size="small" variant="danger" onclick={removeSelected}>
									<i class="bi bi-trash mr-1"></i>
									Remove
								</Button>
								<Button
									tone="admin"
									size="small"
									onclick={() => addBlock(selectedBlock.type, true)}
								>
									<i class="bi bi-copy mr-1"></i>
									Duplicate type
								</Button>
							</div>
							{#if selectedPreview}
								<div class="rounded-lg border border-stone-200 p-3 text-sm dark:border-stone-800">
									<div class="mb-2 flex items-center justify-between">
										<span class="font-medium">Preview evaluation</span>
										<Badge tone={previewTone(selectedPreview.status)}
											>{selectedPreview.status}</Badge
										>
									</div>
									<p class="text-xs text-stone-500">{selectedPreview.item_count} matched items</p>
									{#each selectedPreview.messages as message, index (`${selectedPreview.key}-${index}`)}
										<p class="mt-2 text-xs text-amber-700 dark:text-amber-300">{message}</p>
									{/each}
								</div>
							{/if}
						</div>
					{:else}
						<p class="text-sm text-stone-500">Select a component on the page.</p>
					{/if}
				</section>

				<section class="border-t border-stone-200 pt-4 dark:border-stone-800">
					<div class="mb-2 flex items-center justify-between gap-2">
						<h2 class="text-sm font-semibold">Page check</h2>
						<Button tone="admin" size="small" onclick={onRefreshPreview} disabled={previewLoading}>
							{previewLoading ? "Checking..." : "Check"}
						</Button>
					</div>
					{#if previewError}
						<p class="text-sm text-red-600 dark:text-red-300">{previewError}</p>
					{:else if previewBlocks.length === 0}
						<p class="text-sm text-stone-500">No evaluated commerce blocks yet.</p>
					{:else}
						<div class="space-y-2">
							{#each previewBlocks as block (block.key)}
								<div class="rounded-lg border border-stone-200 p-2 text-xs dark:border-stone-800">
									<div class="flex items-center justify-between gap-2">
										<span class="font-medium">{block.key}</span>
										<Badge tone={previewTone(block.status)}>{block.status}</Badge>
									</div>
									<p class="mt-1 text-stone-500">{block.item_count} matched items</p>
								</div>
							{/each}
						</div>
					{/if}
				</section>
			</div>
		</aside>

		<main class="min-h-0 overflow-y-auto bg-stone-100 p-5 dark:bg-stone-900">
			<div
				class="mx-auto max-w-5xl rounded-lg bg-white shadow-sm ring-1 ring-stone-200 dark:bg-stone-950 dark:ring-stone-800"
			>
				<div
					class="border-b border-stone-200 px-4 py-3 text-xs text-stone-500 dark:border-stone-800"
				>
					{hasUnsavedChanges ? "Unsaved draft" : "Draft saved"}
				</div>
				<div class="p-4 sm:p-6">
					{#if blocks.length === 0}
						<button
							type="button"
							class="flex min-h-72 w-full items-center justify-center rounded-lg border border-dashed border-stone-300 text-sm text-stone-500 dark:border-stone-700"
							onclick={() => addBlock("hero")}
						>
							Add the first component
						</button>
					{:else}
						<div class="space-y-4" data-cms-canvas>
							{#each blocks as block, index (block.editorId)}
								{@const selected = block.editorId === selectedBlockId}
								<section
									role="button"
									tabindex="0"
									data-cms-block-id={block.editorId}
									class="group relative rounded-lg border p-1 transition"
									class:border-stone-900={selected}
									class:bg-stone-50={selected}
									class:dark:border-stone-100={selected}
									class:dark:bg-stone-900={selected}
									class:opacity-35={dragSource?.kind === "block" &&
										dragSource.id === block.editorId}
									class:border-transparent={!selected}
									class:hover:border-stone-300={!selected}
									class:dark:hover:border-stone-700={!selected}
									onclick={() => selectBlockFromCanvas(block.editorId)}
									onkeydown={(event) => keySelect(event, block.editorId)}
								>
									<div class="absolute top-2 right-2 z-10 flex items-center gap-1">
										<Badge tone={selected ? "success" : "neutral"}>Block {index + 1}</Badge>
									</div>
									{#if selected}
										<div
											class="absolute top-3 -left-12 z-20 flex flex-col gap-1 rounded-full border border-stone-200 bg-white p-1 shadow-sm dark:border-stone-700 dark:bg-stone-900"
										>
											<button
												type="button"
												class="flex h-9 w-9 cursor-grab touch-none items-center justify-center rounded-full text-stone-700 hover:bg-stone-100 active:cursor-grabbing dark:text-stone-200 dark:hover:bg-stone-800"
												aria-label="Drag block"
												title="Drag block"
												onpointerdown={(event) => startBlockPointerDrag(event, block.editorId)}
											>
												<i class="bi bi-grip-vertical"></i>
											</button>
											<button
												type="button"
												class="flex h-9 w-9 items-center justify-center rounded-full text-red-600 hover:bg-red-50 dark:text-red-300 dark:hover:bg-red-950"
												aria-label="Delete block"
												title="Delete block"
												onclick={removeSelected}
											>
												<i class="bi bi-trash"></i>
											</button>
										</div>
									{/if}
									{@render blockCanvas(block)}
								</section>
							{/each}
						</div>
					{/if}
				</div>
			</div>
		</main>
	</div>

	{#if draggedBlock && pointerInsideCanvas && insertIndex !== null}
		<div
			class="pointer-events-none fixed right-8 left-80 z-60 h-1 rounded-full bg-blue-500 shadow-[0_0_0_4px_rgba(59,130,246,0.18)]"
			style={insertBarStyle()}
			aria-hidden="true"
		></div>
		<div
			class="pointer-events-none fixed z-60 rounded-lg border border-blue-500 bg-white p-1 opacity-95 shadow-2xl ring-2 ring-blue-500/30 dark:bg-stone-950"
			style={dragGhostStyle()}
			aria-hidden="true"
		>
			{@render blockCanvas(draggedBlock)}
		</div>
	{/if}
</div>

{#snippet editableText(block: EditableBlock, key: string, value: string, className: string)}
	<span
		contenteditable={block.editorId === selectedBlockId ? "true" : "false"}
		role="textbox"
		aria-readonly={block.editorId === selectedBlockId ? "false" : "true"}
		tabindex={block.editorId === selectedBlockId ? 0 : undefined}
		data-empty={value.length === 0 ? "true" : undefined}
		data-placeholder="Click to edit"
		class={`${className} cms-inline-text whitespace-pre-wrap`}
		onpointerdown={(event) => focusEditableText(block, event)}
		onkeydown={(event) => event.stopPropagation()}
		onblur={(event) => commitInlineText(block, key, event)}>{value}</span
	>
{/snippet}

{#snippet blockCanvas(block: EditableBlock)}
	{#if block.type === "hero"}
		<div class="overflow-hidden rounded-md bg-stone-100 dark:bg-stone-800">
			{#if block.image_media_id}
				<img src={cmsMediaURL(block.image_media_id)} alt="" class="h-56 w-full object-cover" />
			{/if}
			<div class="p-6">
				<h1 class="max-w-3xl text-3xl font-semibold">
					{@render editableText(block, "title", block.title, "outline-none")}
				</h1>
				<p class="mt-3 max-w-2xl leading-7 text-stone-600 dark:text-stone-300">
					{@render editableText(block, "subtitle", block.subtitle ?? "", "outline-none")}
				</p>
			</div>
		</div>
	{:else if block.type === "rich_text"}
		<p class="max-w-3xl rounded-md px-3 py-4 leading-8 text-stone-700 dark:text-stone-200">
			{@render editableText(block, "body", block.body, "outline-none")}
		</p>
	{:else if block.type === "image"}
		<figure class="rounded-md">
			{#if imagePreview(block)}
				<img
					src={imagePreview(block)}
					alt={block.alt ?? ""}
					class="max-h-96 w-full rounded-md object-cover"
				/>
			{:else}
				<div
					class="flex h-56 items-center justify-center rounded-md bg-stone-100 text-sm text-stone-500 dark:bg-stone-800"
				>
					Image URL not set
				</div>
			{/if}
			{#if block.caption}
				<figcaption class="mt-2 text-sm text-stone-500">{block.caption}</figcaption>
			{/if}
		</figure>
	{:else if block.type === "cta"}
		<div class="rounded-md border border-stone-200 p-5 dark:border-stone-800">
			<p class="mb-4 text-stone-600 dark:text-stone-300">
				{@render editableText(block, "body", block.body ?? "", "outline-none")}
			</p>
			<span class="inline-flex rounded-lg bg-stone-900 px-4 py-2 text-sm font-medium text-white">
				{@render editableText(block, "label", block.label, "outline-none")}
			</span>
		</div>
	{:else if block.type === "promo_banner" || block.type === "promotion_highlight"}
		<div class="rounded-md bg-stone-950 p-6 text-white">
			<h2 class="text-2xl font-semibold">
				{@render editableText(block, "title", block.title, "outline-none")}
			</h2>
			<p class="mt-2 text-stone-200">
				{@render editableText(block, "body", block.body ?? "", "outline-none")}
			</p>
		</div>
	{:else if block.type === "product_rail"}
		<div>
			<h2 class="text-2xl font-semibold">
				{@render editableText(block, "title", block.title, "outline-none")}
			</h2>
			<p class="mt-1 text-sm text-stone-500">
				{@render editableText(block, "subtitle", block.subtitle ?? "", "outline-none")}
			</p>
			<div class="mt-4 grid grid-cols-2 gap-3 md:grid-cols-4">
				{#each Array.from({ length: Math.min(block.limit || 4, 4) }, (_, productIndex) => productIndex) as productIndex (productIndex)}
					<div
						class="aspect-square rounded-md bg-stone-100 p-3 text-xs text-stone-500 dark:bg-stone-800"
					>
						Product {productIndex + 1}
					</div>
				{/each}
			</div>
		</div>
	{:else if block.type === "category_tiles"}
		<div>
			<h2 class="text-2xl font-semibold">
				{@render editableText(block, "title", block.title, "outline-none")}
			</h2>
			<div class="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
				{#each block.category_slugs.length ? block.category_slugs : ["category"] as slug (slug)}
					<div class="overflow-hidden rounded-md border border-stone-200 dark:border-stone-800">
						{#if categoryImagePreview(block, slug)}
							<img
								src={categoryImagePreview(block, slug)}
								alt=""
								class="aspect-video w-full object-cover"
							/>
						{/if}
						<p class="p-4 font-medium">{slug}</p>
					</div>
				{/each}
			</div>
		</div>
	{:else if block.type === "inventory_message"}
		<div class="rounded-md border border-stone-200 p-4 text-sm dark:border-stone-800">
			<p class="font-semibold">{inventoryProduct(block)?.name ?? "Select a product"}</p>
			<p class="mt-1 text-stone-600 dark:text-stone-300">
				{block.low_stock_message || "Almost gone"}
			</p>
		</div>
	{:else if block.type === "testimonial"}
		<blockquote class="rounded-md border border-stone-200 p-6 dark:border-stone-800">
			<p class="text-xl leading-8">
				{@render editableText(block, "quote", block.quote, "outline-none")}
			</p>
			<footer class="mt-4 text-sm font-medium text-stone-600 dark:text-stone-300">
				{@render editableText(block, "attribution", block.attribution, "outline-none")}
			</footer>
		</blockquote>
	{:else if block.type === "social_embed"}
		<div class="rounded-md border border-stone-200 p-6 dark:border-stone-800">
			<p class="text-sm font-semibold">{block.provider}</p>
			<p class="mt-2 text-lg">{block.title || "Social post"}</p>
		</div>
	{:else}
		<div
			class="rounded-md border border-dashed border-stone-300 p-6 text-sm text-stone-500 dark:border-stone-700"
		>
			{blockLabel(block)}
		</div>
	{/if}
{/snippet}

{#snippet selectedInspector(block: EditableBlock)}
	{#if block.type === "hero"}
		<div class="space-y-2 text-sm">
			<span class="block font-medium">Background image</span>
			<label
				class="flex min-h-24 cursor-pointer items-center justify-center rounded-lg border border-dashed border-stone-300 bg-stone-50 px-3 py-4 text-center text-sm text-stone-600 dark:border-stone-700 dark:bg-stone-900 dark:text-stone-300"
			>
				<input
					class="sr-only"
					type="file"
					accept="image/*"
					onchange={(event) => void uploadBlockImage(event)}
				/>
				<span>{uploadingImage ? "Uploading..." : "Upload image"}</span>
			</label>
			{#if heroImagePreview(block)}
				<img src={heroImagePreview(block)} alt="" class="h-24 w-full rounded-lg object-cover" />
			{/if}
			{#if uploadError}<p class="text-xs text-red-600 dark:text-red-300">{uploadError}</p>{/if}
		</div>
	{:else if block.type === "image"}
		<div class="space-y-2 text-sm">
			<span class="block font-medium">Image</span>
			<label
				class="flex min-h-24 cursor-pointer items-center justify-center rounded-lg border border-dashed border-stone-300 bg-stone-50 px-3 py-4 text-center text-sm text-stone-600 dark:border-stone-700 dark:bg-stone-900 dark:text-stone-300"
			>
				<input
					class="sr-only"
					type="file"
					accept="image/*"
					onchange={(event) => void uploadBlockImage(event)}
				/>
				<span>{uploadingImage ? "Uploading..." : "Upload image"}</span>
			</label>
			{#if imagePreview(block)}
				<img
					src={imagePreview(block)}
					alt={block.alt ?? ""}
					class="h-24 w-full rounded-lg object-cover"
				/>
			{/if}
			{#if uploadError}<p class="text-xs text-red-600 dark:text-red-300">{uploadError}</p>{/if}
		</div>
		<label class="block text-sm">
			<span class="mb-1 block font-medium">Alt text</span>
			<TextInput
				tone="admin"
				value={block.alt ?? ""}
				oninput={(event) =>
					updateSelected({ alt: (event.currentTarget as HTMLInputElement).value })}
				onblur={commitInlineEdit}
			/>
		</label>
		<label class="block text-sm">
			<span class="mb-1 block font-medium">Caption</span>
			<TextInput
				tone="admin"
				value={block.caption ?? ""}
				oninput={(event) =>
					updateSelected({ caption: (event.currentTarget as HTMLInputElement).value })}
				onblur={commitInlineEdit}
			/>
		</label>
	{:else if block.type === "cta"}
		<label class="block text-sm">
			<span class="mb-1 block font-medium">Destination</span>
			<TextInput
				tone="admin"
				value={block.url}
				placeholder="/collections/new"
				oninput={(event) =>
					updateSelected({ url: (event.currentTarget as HTMLInputElement).value })}
				onblur={commitInlineEdit}
			/>
		</label>
	{:else if block.type === "promo_banner"}
		<label class="block text-sm">
			<span class="mb-1 block font-medium">Link label</span>
			<TextInput
				tone="admin"
				value={block.link?.label ?? ""}
				placeholder="Shop now"
				oninput={(event) =>
					updateSelected({
						link: {
							label: (event.currentTarget as HTMLInputElement).value,
							url: block.link?.url ?? "",
						},
					})}
				onblur={commitInlineEdit}
			/>
		</label>
		<label class="block text-sm">
			<span class="mb-1 block font-medium">Link destination</span>
			<TextInput
				tone="admin"
				value={block.link?.url ?? ""}
				placeholder="/sale"
				oninput={(event) =>
					updateSelected({
						link: {
							label: block.link?.label ?? "",
							url: (event.currentTarget as HTMLInputElement).value,
						},
					})}
				onblur={commitInlineEdit}
			/>
		</label>
	{:else if block.type === "product_rail"}
		<label class="block text-sm">
			<span class="mb-1 block font-medium">Source</span>
			<select
				class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm dark:border-stone-700 dark:bg-stone-900"
				value={block.source}
				onchange={(event) =>
					updateSelected(
						{ source: (event.currentTarget as HTMLSelectElement).value as typeof block.source },
						true
					)}
			>
				<option value="newest">Newest</option>
				<option value="manual">Specific products</option>
				<option value="search">Matching products</option>
				<option value="category">Category</option>
			</select>
		</label>
		<div class="grid grid-cols-2 gap-2">
			<label class="block text-sm">
				<span class="mb-1 block font-medium">Sort by</span>
				<select
					class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm dark:border-stone-700 dark:bg-stone-900"
					value={block.sort ?? "created_at"}
					onchange={(event) =>
						updateSelected(
							{
								sort: (event.currentTarget as HTMLSelectElement).value as NonNullable<
									typeof block.sort
								>,
							},
							true
						)}
				>
					<option value="created_at">Created</option><option value="name">Name</option><option
						value="price">Price</option
					>
				</select>
			</label>
			<label class="block text-sm">
				<span class="mb-1 block font-medium">Order</span>
				<select
					class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm dark:border-stone-700 dark:bg-stone-900"
					value={block.order ?? "desc"}
					onchange={(event) =>
						updateSelected(
							{
								order: (event.currentTarget as HTMLSelectElement).value as NonNullable<
									typeof block.order
								>,
							},
							true
						)}
				>
					<option value="desc">Descending</option><option value="asc">Ascending</option>
				</select>
			</label>
		</div>
		<label class="block text-sm">
			<span class="mb-1 block font-medium">Image shape</span>
			<select
				class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm dark:border-stone-700 dark:bg-stone-900"
				value={block.image_aspect ?? "square"}
				onchange={(event) =>
					updateSelected(
						{
							image_aspect: (event.currentTarget as HTMLSelectElement).value as NonNullable<
								typeof block.image_aspect
							>,
						},
						true
					)}
			>
				<option value="square">Square</option><option value="wide">Wide</option>
			</select>
		</label>
		<label class="block text-sm">
			<span class="mb-1 block font-medium">Limit</span>
			<NumberInput
				tone="admin"
				value={block.limit}
				oninput={(event) =>
					updateSelected({ limit: Number((event.currentTarget as HTMLInputElement).value) })}
				onblur={commitInlineEdit}
			/>
		</label>
		{#if block.source === "manual"}
			<div class="space-y-2 text-sm">
				<span class="block font-medium">Products</span>
				<div class="flex gap-2">
					<TextInput tone="admin" placeholder="Search products" bind:value={productSearch} />
					<Button
						tone="admin"
						size="small"
						onclick={() => void runProductSearch()}
						disabled={productSearchLoading}
					>
						{productSearchLoading ? "Searching..." : "Search"}
					</Button>
				</div>
				{#if productSearchError}
					<p class="text-xs text-red-600 dark:text-red-300">{productSearchError}</p>
				{/if}
				{#if selectedProducts(block).length}
					<div
						class="rounded-lg border border-stone-200 bg-stone-50 p-2 dark:border-stone-800 dark:bg-stone-900"
					>
						<div class="mb-2 text-xs font-medium text-stone-500 dark:text-stone-400">
							Shown products
						</div>
						<div class="space-y-1">
							{#each selectedProducts(block) as item (item.id)}
								<div
									class="flex items-center justify-between gap-2 rounded-md bg-white px-2 py-1.5 text-xs dark:bg-stone-950"
								>
									<span class="min-w-0 truncate">
										{item.product?.name ?? `Product ${item.id}`}
									</span>
									<button
										type="button"
										class="shrink-0 rounded-full p-1 text-stone-500 hover:bg-stone-100 hover:text-red-600 dark:hover:bg-stone-800 dark:hover:text-red-300"
										aria-label="Remove product"
										title="Remove product"
										onclick={() => removeProduct(block, item.id)}
									>
										<i class="bi bi-x-lg"></i>
									</button>
								</div>
							{/each}
						</div>
					</div>
				{/if}
				<div class="space-y-2">
					{#each productResults as product (product.id)}
						<button
							type="button"
							class="flex w-full items-center justify-between gap-2 rounded-lg border border-stone-200 px-3 py-2 text-left text-sm dark:border-stone-800"
							onclick={() => toggleProduct(block, product)}
						>
							<span>{product.name}</span>
							<Badge tone={(block.product_ids ?? []).includes(product.id) ? "success" : "neutral"}>
								{(block.product_ids ?? []).includes(product.id) ? "Selected" : "Add"}
							</Badge>
						</button>
					{/each}
				</div>
			</div>
		{:else if block.source === "search"}
			<label class="block text-sm">
				<span class="mb-1 block font-medium">Product match</span>
				<TextInput
					tone="admin"
					value={block.query ?? ""}
					placeholder="Summer, jacket, tote..."
					oninput={(event) =>
						updateSelected({ query: (event.currentTarget as HTMLInputElement).value })}
					onblur={commitInlineEdit}
				/>
			</label>
		{:else if block.source === "category"}
			<div class="space-y-2 text-sm">
				<span class="block font-medium">Category</span>
				<div class="flex gap-2">
					<TextInput tone="admin" placeholder="Search categories" bind:value={categorySearch} />
					<Button
						tone="admin"
						size="small"
						onclick={() => void loadCategories()}
						disabled={categoriesLoading}
					>
						{categoriesLoading ? "Loading..." : "Search"}
					</Button>
				</div>
				{#if categoryError}
					<p class="text-xs text-red-600 dark:text-red-300">{categoryError}</p>
				{/if}
				<div class="space-y-2">
					{#each categoryResults as category (category.id)}
						<button
							type="button"
							class="flex w-full items-center justify-between gap-2 rounded-lg border border-stone-200 px-3 py-2 text-left text-sm dark:border-stone-800"
							onclick={() => updateSelected({ category_slug: category.slug }, true)}
						>
							<span>{category.name}</span>
							<Badge tone={block.category_slug === category.slug ? "success" : "neutral"}>
								{block.category_slug === category.slug ? "Selected" : "Choose"}
							</Badge>
						</button>
					{/each}
				</div>
			</div>
		{/if}
	{:else if block.type === "category_tiles"}
		<label class="block text-sm">
			<span class="mb-1 block font-medium">Image shape</span>
			<select
				class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm dark:border-stone-700 dark:bg-stone-900"
				value={block.image_aspect ?? "square"}
				onchange={(event) =>
					updateSelected(
						{
							image_aspect: (event.currentTarget as HTMLSelectElement).value as NonNullable<
								typeof block.image_aspect
							>,
						},
						true
					)}
			>
				<option value="square">Square</option><option value="wide">Wide</option>
			</select>
		</label>
		<div class="space-y-2 text-sm">
			<span class="block font-medium">Categories</span>
			<div class="flex gap-2">
				<TextInput tone="admin" placeholder="Search categories" bind:value={categorySearch} />
				<Button
					tone="admin"
					size="small"
					onclick={() => void loadCategories()}
					disabled={categoriesLoading}
				>
					{categoriesLoading ? "Loading..." : "Search"}
				</Button>
			</div>
			{#if categoryError}
				<p class="text-xs text-red-600 dark:text-red-300">{categoryError}</p>
			{/if}
			<div class="space-y-2">
				{#each categoryResults as category (category.id)}
					<button
						type="button"
						class="flex w-full items-center justify-between gap-2 rounded-lg border border-stone-200 px-3 py-2 text-left text-sm dark:border-stone-800"
						onclick={() => toggleCategory(block, category)}
					>
						<span>{category.name}</span>
						<Badge tone={block.category_slugs.includes(category.slug) ? "success" : "neutral"}>
							{block.category_slugs.includes(category.slug) ? "Selected" : "Add"}
						</Badge>
					</button>
				{/each}
			</div>
			{#if block.category_slugs.length}
				<p class="text-xs text-stone-500">{block.category_slugs.length} selected</p>
			{/if}
		</div>
		<div class="space-y-2 text-sm">
			<span class="block font-medium">Tile images</span>
			{#each block.category_slugs as slug (slug)}
				<div
					class="flex items-center gap-3 rounded-lg border border-stone-200 p-2 dark:border-stone-800"
				>
					{#if categoryImagePreview(block, slug)}
						<img
							src={categoryImagePreview(block, slug)}
							alt=""
							class="size-12 rounded object-cover"
						/>
					{:else}
						<div class="size-12 rounded bg-stone-100 dark:bg-stone-800"></div>
					{/if}
					<span class="min-w-0 flex-1 truncate">{slug}</span>
					<label
						class="cursor-pointer rounded-lg border border-stone-300 px-2.5 py-1.5 text-xs dark:border-stone-700"
					>
						<input
							class="sr-only"
							type="file"
							accept="image/*"
							onchange={(event) => void uploadCategoryImage(block, slug, event)}
						/>
						{categoryImagePreview(block, slug) ? "Replace" : "Upload"}
					</label>
				</div>
			{/each}
			{#if uploadError}<p class="text-xs text-red-600 dark:text-red-300">{uploadError}</p>{/if}
		</div>
	{:else if block.type === "promotion_highlight"}
		<label class="block text-sm">
			<span class="mb-1 block font-medium">Badge</span>
			<TextInput
				tone="admin"
				value={block.badge ?? ""}
				oninput={(event) =>
					updateSelected({ badge: (event.currentTarget as HTMLInputElement).value })}
				onblur={commitInlineEdit}
			/>
		</label>
		<label class="block text-sm">
			<span class="mb-1 block font-medium">Promotion code</span>
			<TextInput
				tone="admin"
				value={block.promotion_code ?? ""}
				oninput={(event) =>
					updateSelected({ promotion_code: (event.currentTarget as HTMLInputElement).value })}
				onblur={commitInlineEdit}
			/>
		</label>
		<label class="block text-sm">
			<span class="mb-1 block font-medium">Campaign</span>
			<NumberInput
				tone="admin"
				value={block.campaign_id ?? 0}
				oninput={(event) =>
					updateSelected({
						campaign_id: Number((event.currentTarget as HTMLInputElement).value) || undefined,
					})}
				onblur={commitInlineEdit}
			/>
		</label>
		<label class="block text-sm">
			<span class="mb-1 block font-medium">Link label</span>
			<TextInput
				tone="admin"
				value={block.link?.label ?? ""}
				oninput={(event) =>
					updateSelected({
						link: {
							label: (event.currentTarget as HTMLInputElement).value,
							url: block.link?.url ?? "",
						},
					})}
				onblur={commitInlineEdit}
			/>
		</label>
		<label class="block text-sm">
			<span class="mb-1 block font-medium">Link destination</span>
			<TextInput
				tone="admin"
				value={block.link?.url ?? ""}
				oninput={(event) =>
					updateSelected({
						link: {
							label: block.link?.label ?? "",
							url: (event.currentTarget as HTMLInputElement).value,
						},
					})}
				onblur={commitInlineEdit}
			/>
		</label>
	{:else if block.type === "inventory_message"}
		<div class="space-y-2 text-sm">
			<span class="block font-medium">Product</span>
			{#if inventoryProduct(block)}
				<div
					class="flex items-center justify-between gap-2 rounded-lg border border-stone-200 bg-stone-50 px-3 py-2 dark:border-stone-800 dark:bg-stone-900"
				>
					<span class="min-w-0 truncate font-medium">{inventoryProduct(block)?.name}</span>
					<Badge tone="success">Selected</Badge>
				</div>
			{/if}
			<div class="flex gap-2">
				<TextInput tone="admin" placeholder="Search products" bind:value={productSearch} />
				<Button
					tone="admin"
					size="small"
					onclick={() => void runProductSearch()}
					disabled={productSearchLoading}>{productSearchLoading ? "Searching..." : "Search"}</Button
				>
			</div>
			{#if productSearchError}<p class="text-xs text-red-600 dark:text-red-300">
					{productSearchError}
				</p>{/if}
			<div class="space-y-1">
				{#each productResults as product (product.id)}
					<button
						type="button"
						class="flex w-full items-center justify-between gap-2 rounded-lg border border-stone-200 px-3 py-2 text-left dark:border-stone-800"
						onclick={() => updateSelected({ product_id: product.id }, true)}
					>
						<span class="min-w-0 truncate">{product.name}</span><Badge
							tone={block.product_id === product.id ? "success" : "neutral"}
							>{block.product_id === product.id ? "Selected" : "Choose"}</Badge
						>
					</button>
				{/each}
			</div>
		</div>
		<label class="block text-sm">
			<span class="mb-1 block font-medium">Low stock threshold</span>
			<NumberInput
				tone="admin"
				value={block.low_stock_threshold ?? 5}
				oninput={(event) =>
					updateSelected({
						low_stock_threshold: Number((event.currentTarget as HTMLInputElement).value),
					})}
				onblur={commitInlineEdit}
			/>
		</label>
		{#each [["in_stock_message", "In stock message"], ["low_stock_message", "Low stock message"], ["out_of_stock_message", "Out of stock message"]] as messageField (messageField[0])}
			<label class="block text-sm">
				<span class="mb-1 block font-medium">{messageField[1]}</span>
				<TextInput
					tone="admin"
					value={String(
						block[
							messageField[0] as "in_stock_message" | "low_stock_message" | "out_of_stock_message"
						] ?? ""
					)}
					oninput={(event) =>
						updateSelected({ [messageField[0]]: (event.currentTarget as HTMLInputElement).value })}
					onblur={commitInlineEdit}
				/>
			</label>
		{/each}
	{:else if block.type === "testimonial"}
		<label class="block text-sm">
			<span class="mb-1 block font-medium">Rating</span>
			<NumberInput
				tone="admin"
				value={block.rating ?? 5}
				oninput={(event) =>
					updateSelected({ rating: Number((event.currentTarget as HTMLInputElement).value) })}
				onblur={commitInlineEdit}
			/>
		</label>
	{:else if block.type === "social_embed"}
		<label class="block text-sm">
			<span class="mb-1 block font-medium">Provider</span>
			<select
				class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm dark:border-stone-700 dark:bg-stone-900"
				value={block.provider}
				onchange={(event) =>
					updateSelected(
						{ provider: (event.currentTarget as HTMLSelectElement).value as typeof block.provider },
						true
					)}
			>
				<option value="instagram">Instagram</option>
				<option value="tiktok">TikTok</option>
				<option value="youtube">YouTube</option>
			</select>
		</label>
		<label class="block text-sm">
			<span class="mb-1 block font-medium">URL</span>
			<TextInput
				tone="admin"
				value={block.url}
				oninput={(event) =>
					updateSelected({ url: (event.currentTarget as HTMLInputElement).value })}
				onblur={commitInlineEdit}
			/>
		</label>
	{/if}
	<!-- eslint-disable svelte/no-navigation-without-resolve -->
	<a
		class="inline-flex text-sm font-medium text-stone-700 underline underline-offset-4 dark:text-stone-200"
		href={pagePath || "/"}
		target="_blank"
		rel="noreferrer"
	>
		Open page
	</a>
	<!-- eslint-enable svelte/no-navigation-without-resolve -->
{/snippet}

<style>
	.cms-inline-text {
		display: inline-block;
		min-width: 1ch;
		min-height: 1.5em;
	}

	.cms-inline-text[data-empty="true"]::before {
		content: attr(data-placeholder);
		color: rgb(120 113 108);
		font-style: italic;
	}

	.cms-inline-text[data-empty="true"]:focus::before {
		content: "";
	}
</style>
