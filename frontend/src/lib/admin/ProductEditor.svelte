<script lang="ts">
	import { resolve } from "$app/paths";
	import { DRAFT_PREVIEW_SYNC_EVENT, DRAFT_PREVIEW_SYNC_STORAGE_KEY, type API } from "$lib/api";
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminFieldLabel from "$lib/admin/AdminFieldLabel.svelte";
	import AdminMetaText from "$lib/admin/AdminMetaText.svelte";
	import AdminSurface from "$lib/admin/AdminSurface.svelte";
	import {
		adminDividerBottomClass,
		adminDividerTopClass,
		adminListItemBaseClass,
		adminSurfaceVariantClasses,
	} from "$lib/admin/tokens";
	import AdminSearchForm from "$lib/admin/AdminSearchForm.svelte";
	import Alert from "$lib/components/Alert.svelte";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import IconButton from "$lib/components/IconButton.svelte";
	import NumberInput from "$lib/components/NumberInput.svelte";
	import TextArea from "$lib/components/TextArea.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
	import type { components } from "$lib/api/generated/openapi";
	import {
		type BrandModel,
		type CategoryModel,
		type ProductAttributeDefinitionModel,
		type ProductModel,
		type RelatedProductModel,
	} from "$lib/models";
	import { uploadMediaFiles } from "$lib/media";
	import { getContext, onDestroy, onMount, untrack } from "svelte";

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
		onDirtyChange?: (dirty: boolean) => void;
		onSaveRequestChange?: (saveAction: (() => Promise<void>) | null) => void;
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
		onDirtyChange,
		onSaveRequestChange,
	}: Props = $props();

	const api: API = getContext("api");

	type ProductUpsertInput = components["schemas"]["ProductUpsertInput"];
	type ProductAttributeDefinitionInput = components["schemas"]["ProductAttributeDefinitionInput"];
	type EditorOptionValue = {
		key: string;
		value: string;
		position: number;
	};
	type EditorOption = {
		key: string;
		name: string;
		display_type: string;
		position: number;
		values: EditorOptionValue[];
	};
	type EditorVariantSelection = {
		key: string;
		option_name: string;
		option_value: string;
		position: number;
	};
	type EditorVariant = {
		key: string;
		sku: string;
		title: string;
		price: string;
		compare_at_price: string;
		stock: string;
		is_published: boolean;
		selections: EditorVariantSelection[];
	};
	type EditorAttributeValue = {
		key: string;
		product_attribute_id: string;
		type: ProductAttributeDefinitionModel["type"] | "";
		text_value: string;
		number_value: string;
		boolean_value: boolean;
		enum_value: string;
		position: number;
	};

	let product = $state<ProductModel | null>(null);
	let brands = $state<BrandModel[]>([]);
	let categories = $state<CategoryModel[]>([]);
	let attributeDefinitions = $state<ProductAttributeDefinitionModel[]>([]);
	let loading = $state(false);
	let saving = $state(false);
	let publishing = $state(false);
	let unpublishing = $state(false);
	let discardingDraft = $state(false);
	let previewingDraft = $state(false);
	let previewActive = $state(false);
	let deleting = $state(false);
	let uploading = $state(false);
	let mediaDeleting = $state<string | null>(null);
	let mediaReordering = $state(false);
	let relatedLoading = $state(false);
	let relatedSaving = $state(false);
	let productErrorMessage = $state("");
	let productStatusMessage = $state("");
	let mediaErrorMessage = $state("");
	let mediaStatusMessage = $state("");
	let relatedErrorMessage = $state("");
	let relatedStatusMessage = $state("");
	let attributeDefinitionErrorMessage = $state("");
	let attributeDefinitionStatusMessage = $state("");
	let attributeDefinitionSaving = $state(false);
	let attributeDefinitionDeletingId = $state<number | null>(null);

	let sku = $state("");
	let name = $state("");
	let subtitle = $state("");
	let description = $state("");
	let selectedBrandId = $state("");
	let selectedCategoryIds = $state<string[]>([]);
	let categorySearchQuery = $state("");
	let categoryMenuOpen = $state(false);
	let seoTitle = $state("");
	let seoDescription = $state("");
	let seoCanonicalPath = $state("");
	let seoOgImageMediaId = $state("");
	let seoNoIndex = $state(false);
	let options = $state<EditorOption[]>([]);
	let variants = $state<EditorVariant[]>([]);
	let attributeValues = $state<EditorAttributeValue[]>([]);
	let attributeDefinitionEditingId = $state<number | null>(null);
	let attributeDefinitionKey = $state("");
	let attributeDefinitionSlug = $state("");
	let attributeDefinitionType = $state<ProductAttributeDefinitionModel["type"]>("text");
	let attributeDefinitionFilterable = $state(true);
	let attributeDefinitionSortable = $state(false);
	let attributeDefinitionEnumValuesText = $state("");
	let defaultVariantSku = $state("");
	let mediaFiles = $state<FileList | null>(null);
	let mediaInputRef = $state<HTMLInputElement | null>(null);
	let pendingMediaOrder = $state<string[] | null>(null);
	let relatedQuery = $state("");
	let relatedOptions = $state<ProductModel[]>([]);
	let relatedSelected = $state<RelatedProductModel[]>([]);
	let relatedLastSearchedQuery = $state("");
	let savedSnapshot = $state("");
	let savedProductSnapshot = $state("");

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
	const isPublished = $derived(product?.is_published ?? false);
	const hasDraftChanges = $derived(product?.has_draft_changes ?? false);
	const relatedBaseline = $derived(product?.related_products ?? []);
	const selectedCategories = $derived.by(() => {
		const selected = new Set(selectedCategoryIds);
		return categories.filter((category) => selected.has(String(category.id)));
	});
	const availableCategoryOptions = $derived.by(() => {
		const selected = new Set(selectedCategoryIds);
		const query = asTrimmedString(categorySearchQuery).toLowerCase();
		return categories
			.filter((category) => !selected.has(String(category.id)))
			.filter((category) => category.is_active)
			.filter((category) => {
				if (!query) {
					return true;
				}
				return (
					category.name.toLowerCase().includes(query) ||
					category.slug.toLowerCase().includes(query) ||
					(category.description ?? "").toLowerCase().includes(query)
				);
			})
			.slice(0, 8);
	});
	const hasPendingRelatedChanges = $derived.by(() => {
		const selectedIds = [...relatedSelected.map((item) => item.id)].sort((a, b) => a - b).join("|");
		const baselineIds = [...relatedBaseline.map((item) => item.id)].sort((a, b) => a - b).join("|");
		return selectedIds !== baselineIds;
	});
	const hasPendingUploadSelection = $derived(mediaFilesCount > 0);
	const hasPendingProductChanges = $derived(buildProductSnapshot() !== savedProductSnapshot);
	const currentSnapshot = $derived(buildDraftSnapshot());
	const hasUnsavedChanges = $derived(currentSnapshot !== savedSnapshot);

	let loadSequence = 0;
	let lastLoadedId: number | null = null;
	let activeSelectionId: number | null = null;
	let lastSeedSignature = "";
	let lastDirtyNotification: boolean | null = null;
	let lastSaveActionDirty: boolean | null = null;
	let lastDirtyHandler: Props["onDirtyChange"] = undefined;
	let lastSaveHandler: Props["onSaveRequestChange"] = undefined;
	let editorKeySeed = 0;

	const splitEditorSectionClass = adminSurfaceVariantClasses["panel-tight"];
	const nestedEditorSectionClass = adminSurfaceVariantClasses.subsurface;
	const mediaCardClass = adminSurfaceVariantClasses.media;
	const mutedPanelClass = adminSurfaceVariantClasses.muted;
	const overlayIconButtonClusterClass = "flex gap-1";
	const overlayIconButtonSurfaceClass =
		"border border-stone-300 bg-white/95 shadow-sm backdrop-blur-sm hover:bg-stone-50 disabled:opacity-45 dark:border-stone-700 dark:bg-stone-950/85 dark:hover:bg-stone-900";
	const overlayIconButtonMiniClass = "h-5 w-5 text-[10px]";
	const sectionDividerTopClass = adminDividerTopClass;
	const sectionDividerBottomClass = adminDividerBottomClass;

	function editorSectionClass(layoutMode: "split" | "stacked"): string {
		return layoutMode === "split" ? splitEditorSectionClass : "";
	}

	function editorCollectionClass(layoutMode: "split" | "stacked"): string {
		return layoutMode === "split"
			? "mt-4 space-y-4"
			: "mt-4 divide-y divide-stone-200 dark:divide-stone-800";
	}

	function editorItemClass(layoutMode: "split" | "stacked"): string {
		return layoutMode === "split" ? nestedEditorSectionClass : "py-4 first:pt-0 last:pb-0";
	}

	function mutedEditorPanelClass(layoutMode: "split" | "stacked"): string {
		return layoutMode === "split" ? mutedPanelClass : "";
	}

	function relatedResultsClass(layoutMode: "split" | "stacked"): string {
		return layoutMode === "split"
			? "mt-3 space-y-2"
			: "mt-4 divide-y divide-stone-200 dark:divide-stone-800";
	}

	function relatedResultItemClass(layoutMode: "split" | "stacked"): string {
		return layoutMode === "split"
			? `${adminListItemBaseClass} p-4 text-sm`
			: "flex items-center justify-between gap-3 py-3 text-sm";
	}

	function relatedSelectedListClass(layoutMode: "split" | "stacked"): string {
		return layoutMode === "split"
			? "mt-4 space-y-2"
			: "mt-4 divide-y divide-stone-200 dark:divide-stone-800";
	}

	function relatedSelectedItemClass(layoutMode: "split" | "stacked"): string {
		return layoutMode === "split"
			? `${mutedPanelClass} flex items-center justify-between px-3 py-2 text-sm`
			: "flex items-center justify-between gap-3 py-3 text-sm";
	}

	function mediaItemClass(layoutMode: "split" | "stacked"): string {
		return layoutMode === "split"
			? `${mediaCardClass} relative overflow-hidden`
			: "relative overflow-hidden rounded-[1rem]";
	}

	type MessageScope = "product" | "media" | "related";
	type MessageTone = "error" | "success";

	function normalizedNumber(value: string): number | null | "invalid" {
		const trimmed = String(value ?? "").trim();
		if (trimmed === "") {
			return null;
		}
		const parsed = Number(trimmed);
		return Number.isNaN(parsed) ? "invalid" : parsed;
	}

	function asTrimmedString(value: unknown): string {
		return String(value ?? "").trim();
	}

	function nextEditorKey(prefix: string): string {
		editorKeySeed += 1;
		return `${prefix}-${editorKeySeed}`;
	}

	function createOptionValue(value = "", position = 1): EditorOptionValue {
		return {
			key: nextEditorKey("option-value"),
			value,
			position,
		};
	}

	function createOption(name = "", displayType = "select", values: string[] = []): EditorOption {
		return {
			key: nextEditorKey("option"),
			name,
			display_type: displayType,
			position: options.length + 1,
			values:
				values.length > 0
					? values.map((value, index) => createOptionValue(value, index + 1))
					: [createOptionValue("", 1)],
		};
	}

	function createVariantSelection(
		optionName = "",
		optionValue = "",
		position = 1
	): EditorVariantSelection {
		return {
			key: nextEditorKey("variant-selection"),
			option_name: optionName,
			option_value: optionValue,
			position,
		};
	}

	function createVariant(overrides: Partial<EditorVariant> = {}): EditorVariant {
		const variant: EditorVariant = {
			key: nextEditorKey("variant"),
			sku: "",
			title: "",
			price: "",
			compare_at_price: "",
			stock: "0",
			is_published: true,
			selections: [],
			...overrides,
		};
		return variant;
	}

	function createAttributeValue(
		overrides: Partial<EditorAttributeValue> = {}
	): EditorAttributeValue {
		return {
			key: nextEditorKey("attribute"),
			product_attribute_id: "",
			type: "",
			text_value: "",
			number_value: "",
			boolean_value: false,
			enum_value: "",
			position: attributeValues.length + 1,
			...overrides,
		};
	}

	function variantSeed(): Pick<EditorVariant, "price" | "compare_at_price" | "stock"> {
		const source = variants.find((variant) => variant.sku === defaultVariantSku) ?? variants[0];
		if (source) {
			return {
				price: source.price,
				compare_at_price: source.compare_at_price,
				stock: source.stock,
			};
		}
		return {
			price: "",
			compare_at_price: "",
			stock: "0",
		};
	}

	function normalizeEditorOptionsForSnapshot() {
		return options.map((option, optionIndex) => ({
			name: asTrimmedString(option.name),
			display_type: asTrimmedString(option.display_type) || "select",
			position: optionIndex + 1,
			values: option.values.map((value, valueIndex) => ({
				value: asTrimmedString(value.value),
				position: valueIndex + 1,
			})),
		}));
	}

	function normalizeEditorVariantsForSnapshot() {
		return variants.map((variant, variantIndex) => ({
			sku: asTrimmedString(variant.sku),
			title: asTrimmedString(variant.title),
			price: normalizedNumber(variant.price),
			compare_at_price: normalizedNumber(variant.compare_at_price),
			stock: normalizedNumber(variant.stock),
			is_published: variant.is_published,
			position: variantIndex + 1,
			selections: variant.selections.map((selection, selectionIndex) => ({
				option_name: asTrimmedString(selection.option_name),
				option_value: asTrimmedString(selection.option_value),
				position: selectionIndex + 1,
			})),
		}));
	}

	function normalizeEditorAttributesForSnapshot() {
		return attributeValues.map((attribute, index) => ({
			product_attribute_id: Number(attribute.product_attribute_id),
			type: attribute.type,
			position: index + 1,
			text_value: asTrimmedString(attribute.text_value),
			number_value: normalizedNumber(attribute.number_value),
			boolean_value: attribute.boolean_value,
			enum_value: asTrimmedString(attribute.enum_value),
		}));
	}

	function buildProductSnapshot(): string {
		return JSON.stringify({
			product_id: resolvedProductId,
			fields: {
				sku: asTrimmedString(sku),
				name: asTrimmedString(name),
				subtitle: asTrimmedString(subtitle),
				description: asTrimmedString(description),
				brand_id: asTrimmedString(selectedBrandId),
				category_ids: selectedCategoryIds.map(Number).sort((a, b) => a - b),
				default_variant_sku: asTrimmedString(defaultVariantSku),
			},
			seo: {
				title: asTrimmedString(seoTitle),
				description: asTrimmedString(seoDescription),
				canonical_path: asTrimmedString(seoCanonicalPath),
				og_image_media_id: asTrimmedString(seoOgImageMediaId),
				noindex: seoNoIndex,
			},
			options: normalizeEditorOptionsForSnapshot(),
			variants: normalizeEditorVariantsForSnapshot(),
			related_product_ids: [...relatedSelected.map((item) => item.id)].sort((a, b) => a - b),
			attributes: normalizeEditorAttributesForSnapshot(),
		});
	}

	function buildDraftSnapshot(): string {
		const relatedIDs = [...relatedSelected.map((item) => item.id)].sort((a, b) => a - b);
		const mediaOrder = pendingMediaOrder ?? product?.images ?? [];
		return JSON.stringify({
			product: JSON.parse(buildProductSnapshot()),
			media_order: mediaOrder,
			pending_upload_count: mediaFilesCount,
			related_product_ids: relatedIDs,
		});
	}

	function captureSavedSnapshot() {
		savedProductSnapshot = untrack(() => buildProductSnapshot());
		savedSnapshot = untrack(() => buildDraftSnapshot());
	}

	function clearMessages(scope?: MessageScope) {
		if (!scope || scope === "product") {
			productErrorMessage = "";
			productStatusMessage = "";
		}
		if (!scope || scope === "media") {
			mediaErrorMessage = "";
			mediaStatusMessage = "";
		}
		if (!scope || scope === "related") {
			relatedErrorMessage = "";
			relatedStatusMessage = "";
		}
		if (!scope) {
			onErrorMessage?.("");
			onStatusMessage?.("");
		}
	}

	function clearMessage(scope: MessageScope, tone: MessageTone) {
		if (scope === "product") {
			if (tone === "error") {
				productErrorMessage = "";
				onErrorMessage?.("");
			} else {
				productStatusMessage = "";
				onStatusMessage?.("");
			}
			return;
		}
		if (scope === "media") {
			if (tone === "error") {
				mediaErrorMessage = "";
				onErrorMessage?.("");
			} else {
				mediaStatusMessage = "";
				onStatusMessage?.("");
			}
			return;
		}
		if (tone === "error") {
			relatedErrorMessage = "";
			onErrorMessage?.("");
		} else {
			relatedStatusMessage = "";
			onStatusMessage?.("");
		}
	}

	function setMessage(scope: MessageScope, tone: MessageTone, message: string) {
		if (scope === "product") {
			if (tone === "error") {
				productErrorMessage = message;
				onErrorMessage?.(message);
			} else {
				productStatusMessage = message;
				onStatusMessage?.(message);
			}
			return;
		}
		if (scope === "media") {
			if (tone === "error") {
				mediaErrorMessage = message;
				onErrorMessage?.(message);
			} else {
				mediaStatusMessage = message;
				onStatusMessage?.(message);
			}
			return;
		}
		if (tone === "error") {
			relatedErrorMessage = message;
			onErrorMessage?.(message);
		} else {
			relatedStatusMessage = message;
			onStatusMessage?.(message);
		}
	}

	function clearAllMessages() {
		clearMessages();
	}

	function applyUpdatedProduct(updated: ProductModel, options?: { hydrate?: boolean }) {
		product = updated;
		if (options?.hydrate ?? true) {
			hydrateForm(updated);
		}
		captureSavedSnapshot();
		onProductUpdated?.(updated);
		onErrorMessage?.("");
		onStatusMessage?.("");
	}

	function readableActionError(err: unknown, fallback: string): string {
		const error = err as { body?: { error?: string } };
		const apiMessage = error?.body?.error;
		if (typeof apiMessage === "string" && apiMessage.trim() !== "") {
			return apiMessage;
		}
		return fallback;
	}

	async function loadPreviewState() {
		try {
			const session = await api.getAdminPreviewSession();
			previewActive = session.active;
		} catch {
			previewActive = false;
		}
	}

	function handlePreviewSyncEvent(event: Event) {
		const syncEvent = event as CustomEvent<{ active?: unknown }>;
		if (typeof syncEvent.detail?.active === "boolean") {
			previewActive = syncEvent.detail.active;
			return;
		}
		void loadPreviewState();
	}

	function handlePreviewStorageEvent(event: StorageEvent) {
		if (event.key !== DRAFT_PREVIEW_SYNC_STORAGE_KEY) {
			return;
		}
		if (!event.newValue) {
			void loadPreviewState();
			return;
		}
		try {
			const parsed = JSON.parse(event.newValue) as { active?: unknown };
			if (typeof parsed.active === "boolean") {
				previewActive = parsed.active;
				return;
			}
		} catch {
			// ignore malformed storage payloads
		}
		void loadPreviewState();
	}

	function resetForm() {
		sku = "";
		name = "";
		subtitle = "";
		description = "";
		selectedBrandId = "";
		selectedCategoryIds = [];
		categorySearchQuery = "";
		categoryMenuOpen = false;
		seoTitle = "";
		seoDescription = "";
		seoCanonicalPath = "";
		seoOgImageMediaId = "";
		seoNoIndex = false;
		options = [];
		variants = [createVariant()];
		attributeValues = [];
		defaultVariantSku = "";
		mediaFiles = null;
		pendingMediaOrder = null;
		relatedQuery = "";
		relatedOptions = [];
		relatedSelected = [];
		relatedLastSearchedQuery = "";
		captureSavedSnapshot();
	}

	function hydrateForm(value: ProductModel) {
		sku = value.sku;
		name = value.name;
		subtitle = value.subtitle ?? "";
		description = value.description ?? "";
		selectedBrandId = value.brand ? String(value.brand.id) : "";
		selectedCategoryIds = (value.categories ?? []).map((category) => String(category.id));
		categorySearchQuery = "";
		categoryMenuOpen = false;
		seoTitle = value.seo.title ?? "";
		seoDescription = value.seo.description ?? "";
		seoCanonicalPath = value.seo.canonical_path ?? "";
		seoOgImageMediaId = value.seo.og_image_media_id ?? "";
		seoNoIndex = value.seo.noindex;
		options = (value.options ?? []).map((option, optionIndex) => ({
			key: nextEditorKey("option"),
			name: option.name,
			display_type: option.display_type,
			position: option.position || optionIndex + 1,
			values:
				option.values.length > 0
					? option.values.map((valueItem, valueIndex) => ({
							key: nextEditorKey("option-value"),
							value: valueItem.value,
							position: valueItem.position || valueIndex + 1,
						}))
					: [createOptionValue("", 1)],
		}));
		variants =
			(value.variants ?? []).length > 0
				? value.variants.map((variant) => ({
						key: nextEditorKey("variant"),
						sku: variant.sku,
						title: variant.title,
						price: String(variant.price),
						compare_at_price:
							variant.compare_at_price == null ? "" : String(variant.compare_at_price),
						stock: String(variant.stock),
						is_published: variant.is_published,
						selections: (variant.selections ?? []).map((selection, selectionIndex) => ({
							key: nextEditorKey("variant-selection"),
							option_name: selection.option_name,
							option_value: selection.option_value,
							position: selection.position || selectionIndex + 1,
						})),
					}))
				: [createVariant()];
		attributeValues = (value.attributes ?? []).map((attribute, index) =>
			createAttributeValue({
				product_attribute_id: String(attribute.product_attribute_id),
				type: attribute.type,
				text_value: attribute.text_value ?? "",
				number_value: attribute.number_value == null ? "" : String(attribute.number_value),
				boolean_value: attribute.boolean_value ?? false,
				enum_value: attribute.enum_value ?? "",
				position: attribute.position || index + 1,
			})
		);
		defaultVariantSku = value.default_variant_sku ?? value.variants?.[0]?.sku ?? "";
		pendingMediaOrder = null;
		relatedSelected = value.related_products ?? [];
	}

	async function loadBrands() {
		try {
			brands = await api.listAdminBrands();
		} catch (err) {
			console.error(err);
		}
	}

	async function loadCategories() {
		try {
			categories = await api.listAdminCategories({ include_inactive: true });
		} catch (err) {
			console.error(err);
		}
	}

	async function loadAttributeDefinitions() {
		try {
			attributeDefinitions = await api.listAdminProductAttributes();
		} catch (err) {
			console.error(err);
		}
	}

	function attributeDefinitionById(
		productAttributeID: string
	): ProductAttributeDefinitionModel | undefined {
		return attributeDefinitions.find((attribute) => String(attribute.id) === productAttributeID);
	}

	function resetAttributeDefinitionForm() {
		attributeDefinitionEditingId = null;
		attributeDefinitionKey = "";
		attributeDefinitionSlug = "";
		attributeDefinitionType = "text";
		attributeDefinitionFilterable = true;
		attributeDefinitionSortable = false;
		attributeDefinitionEnumValuesText = "";
	}

	function editAttributeDefinition(definition: ProductAttributeDefinitionModel) {
		attributeDefinitionEditingId = definition.id;
		attributeDefinitionKey = definition.key;
		attributeDefinitionSlug = definition.slug;
		attributeDefinitionType = definition.type;
		attributeDefinitionFilterable = definition.filterable;
		attributeDefinitionSortable = definition.sortable;
		attributeDefinitionEnumValuesText = (definition.enum_values ?? []).join("\n");
		attributeDefinitionErrorMessage = "";
		attributeDefinitionStatusMessage = "";
	}

	function updateAttributeDefinitionType(nextType: ProductAttributeDefinitionModel["type"]) {
		attributeDefinitionType = nextType;
		if (nextType !== "number") {
			attributeDefinitionSortable = false;
		}
	}

	function parseAttributeDefinitionEnumValues(): string[] {
		return attributeDefinitionEnumValuesText
			.split(/\r?\n|,/)
			.map((value) => asTrimmedString(value))
			.filter((value) => value !== "");
	}

	function buildAttributeDefinitionPayload(): ProductAttributeDefinitionInput | null {
		const key = asTrimmedString(attributeDefinitionKey);
		const slug = asTrimmedString(attributeDefinitionSlug);

		if (!key) {
			attributeDefinitionErrorMessage = "Attribute name is required.";
			return null;
		}

		if (attributeDefinitionSortable && attributeDefinitionType !== "number") {
			attributeDefinitionErrorMessage = "Only number attributes can be sortable.";
			return null;
		}

		const enumValues = parseAttributeDefinitionEnumValues();
		if (attributeDefinitionType === "enum" && enumValues.length === 0) {
			attributeDefinitionErrorMessage = "Enum attributes need at least one allowed value.";
			return null;
		}
		const normalizedEnumValues = enumValues.map((value) => value.toLowerCase());
		if (
			normalizedEnumValues.some((value, index) => normalizedEnumValues.indexOf(value) !== index)
		) {
			attributeDefinitionErrorMessage = "Enum attribute values must be unique.";
			return null;
		}

		return {
			key,
			slug: slug || undefined,
			type: attributeDefinitionType,
			filterable: attributeDefinitionFilterable,
			sortable: attributeDefinitionSortable,
			enum_values: attributeDefinitionType === "enum" ? enumValues : [],
		};
	}

	async function saveAttributeDefinition() {
		attributeDefinitionErrorMessage = "";
		attributeDefinitionStatusMessage = "";
		const payload = buildAttributeDefinitionPayload();
		if (!payload) {
			return;
		}

		attributeDefinitionSaving = true;
		try {
			const saved =
				attributeDefinitionEditingId == null
					? await api.createAdminProductAttribute(payload)
					: await api.updateAdminProductAttribute(attributeDefinitionEditingId, payload);
			attributeDefinitions =
				attributeDefinitionEditingId == null
					? [...attributeDefinitions, saved]
					: attributeDefinitions.map((definition) =>
							definition.id === saved.id ? saved : definition
						);
			attributeValues = attributeValues.map((attribute) =>
				Number(attribute.product_attribute_id) === saved.id
					? {
							...attribute,
							type: saved.type,
							enum_value:
								saved.type === "enum" && saved.enum_values.includes(attribute.enum_value)
									? attribute.enum_value
									: "",
						}
					: attribute
			);
			attributeDefinitionStatusMessage =
				attributeDefinitionEditingId == null
					? "Attribute definition created."
					: "Attribute definition updated.";
			resetAttributeDefinitionForm();
		} catch (err) {
			console.error(err);
			attributeDefinitionErrorMessage = readableActionError(
				err,
				"Unable to save attribute definition."
			);
		} finally {
			attributeDefinitionSaving = false;
		}
	}

	async function deleteAttributeDefinition(definition: ProductAttributeDefinitionModel) {
		attributeDefinitionErrorMessage = "";
		attributeDefinitionStatusMessage = "";
		attributeDefinitionDeletingId = definition.id;
		try {
			await api.deleteAdminProductAttribute(definition.id);
			attributeDefinitions = attributeDefinitions.filter((item) => item.id !== definition.id);
			attributeValues = attributeValues.filter(
				(attribute) => Number(attribute.product_attribute_id) !== definition.id
			);
			if (attributeDefinitionEditingId === definition.id) {
				resetAttributeDefinitionForm();
			}
			attributeDefinitionStatusMessage = "Attribute definition deleted.";
		} catch (err) {
			console.error(err);
			attributeDefinitionErrorMessage = readableActionError(
				err,
				"Unable to delete attribute definition."
			);
		} finally {
			attributeDefinitionDeletingId = null;
		}
	}

	function validateProductAttributes(): string | null {
		const selected: number[] = [];
		for (const attribute of attributeValues) {
			const productAttributeID = Number(attribute.product_attribute_id);
			if (!Number.isInteger(productAttributeID) || productAttributeID <= 0) {
				return "Select an attribute for each assigned attribute row.";
			}
			if (selected.includes(productAttributeID)) {
				return "Each product attribute can only be assigned once.";
			}
			selected.push(productAttributeID);

			if (attribute.type === "number") {
				if (asTrimmedString(attribute.number_value) === "") {
					return "Number attributes need a value.";
				}
				if (!Number.isFinite(Number(attribute.number_value))) {
					return "Number attributes need a valid numeric value.";
				}
			}
			if (attribute.type === "text" && asTrimmedString(attribute.text_value) === "") {
				return "Text attributes need a value.";
			}
			if (attribute.type === "enum" && asTrimmedString(attribute.enum_value) === "") {
				return "Enum attributes need a value.";
			}
			const definition = attributeDefinitionById(attribute.product_attribute_id);
			if (
				attribute.type === "enum" &&
				definition &&
				!definition.enum_values.includes(asTrimmedString(attribute.enum_value))
			) {
				return "Enum attributes need one of the allowed values.";
			}
		}
		return null;
	}

	function addCategory(category: CategoryModel) {
		const id = String(category.id);
		if (!selectedCategoryIds.includes(id)) {
			selectedCategoryIds = [...selectedCategoryIds, id];
		}
		categorySearchQuery = "";
		categoryMenuOpen = false;
	}

	function removeCategory(categoryId: number) {
		const id = String(categoryId);
		selectedCategoryIds = selectedCategoryIds.filter((selectedId) => selectedId !== id);
	}

	function handleCategorySearchKeydown(event: KeyboardEvent) {
		if (event.key === "Escape") {
			categoryMenuOpen = false;
			return;
		}
		if (event.key !== "Enter") {
			return;
		}
		const first = availableCategoryOptions[0];
		if (!first) {
			return;
		}
		event.preventDefault();
		addCategory(first);
	}

	function optionalString(value: string): string | undefined {
		const trimmed = asTrimmedString(value);
		return trimmed === "" ? undefined : trimmed;
	}

	function buildProductPayload(): ProductUpsertInput {
		const optionPayload = options.map((option, optionIndex) => ({
			name: asTrimmedString(option.name),
			position: optionIndex + 1,
			display_type: optionalString(option.display_type) ?? "select",
			values: option.values.map((value, valueIndex) => ({
				value: asTrimmedString(value.value),
				position: valueIndex + 1,
			})),
		}));

		const variantPayload = variants.map((variant, variantIndex) => ({
			sku: asTrimmedString(variant.sku),
			title: asTrimmedString(variant.title),
			price: Number(variant.price),
			compare_at_price:
				asTrimmedString(variant.compare_at_price) === ""
					? undefined
					: Number(variant.compare_at_price),
			stock: Number(variant.stock),
			position: variantIndex + 1,
			is_published: variant.is_published,
			selections: variant.selections.map((selection, selectionIndex) => ({
				option_name: asTrimmedString(selection.option_name),
				option_value: asTrimmedString(selection.option_value),
				position: selectionIndex + 1,
			})),
		}));

		return {
			sku: asTrimmedString(sku),
			name: asTrimmedString(name),
			subtitle: optionalString(subtitle),
			description: asTrimmedString(description),
			images: product?.images ?? [],
			related_product_ids: relatedSelected.map((item) => item.id),
			category_ids: selectedCategoryIds.map(Number).filter((id) => Number.isInteger(id) && id > 0),
			brand_id: selectedBrandId ? Number(selectedBrandId) : undefined,
			default_variant_sku:
				optionalString(defaultVariantSku) ?? optionalString(variantPayload[0]?.sku ?? ""),
			options: optionPayload,
			variants: variantPayload,
			attributes: attributeValues
				.map((attribute, index) => {
					const productAttributeID = Number(attribute.product_attribute_id);
					if (!Number.isInteger(productAttributeID) || productAttributeID <= 0) {
						return null;
					}
					const payload: NonNullable<ProductUpsertInput["attributes"]>[number] = {
						product_attribute_id: productAttributeID,
						position: index + 1,
					};
					if (attribute.type === "text") {
						payload.text_value = optionalString(attribute.text_value);
					}
					if (attribute.type === "number") {
						payload.number_value =
							asTrimmedString(attribute.number_value) === ""
								? undefined
								: Number(attribute.number_value);
					}
					if (attribute.type === "boolean") {
						payload.boolean_value = attribute.boolean_value;
					}
					if (attribute.type === "enum") {
						payload.enum_value = optionalString(attribute.enum_value);
					}
					return payload;
				})
				.filter((attribute): attribute is NonNullable<typeof attribute> => attribute !== null),
			seo: {
				title: optionalString(seoTitle),
				description: optionalString(seoDescription),
				canonical_path: optionalString(seoCanonicalPath),
				og_image_media_id: optionalString(seoOgImageMediaId),
				noindex: seoNoIndex,
			},
		};
	}

	function addAttributeValue() {
		attributeValues = [...attributeValues, createAttributeValue()];
	}

	function removeAttributeValue(attributeKey: string) {
		attributeValues = attributeValues.filter((attribute) => attribute.key !== attributeKey);
	}

	function updateAttributeSelection(attributeKey: string, nextValue: string) {
		const definition = attributeDefinitionById(nextValue);
		attributeValues = attributeValues.map((attribute) =>
			attribute.key === attributeKey
				? {
						...attribute,
						product_attribute_id: nextValue,
						type: definition?.type ?? "",
						text_value: "",
						number_value: "",
						boolean_value: false,
						enum_value: "",
					}
				: attribute
		);
	}

	function attributeDefinitionAlreadyAssigned(definitionId: number, currentKey: string): boolean {
		return attributeValues.some(
			(attribute) =>
				attribute.key !== currentKey && Number(attribute.product_attribute_id) === definitionId
		);
	}

	function addOption() {
		options = [...options, createOption()];
	}

	function removeOption(optionKey: string) {
		options = options.filter((option) => option.key !== optionKey);
	}

	function addOptionValue(optionKey: string) {
		options = options.map((option) =>
			option.key === optionKey
				? {
						...option,
						values: [...option.values, createOptionValue("", option.values.length + 1)],
					}
				: option
		);
	}

	function removeOptionValue(optionKey: string, valueKey: string) {
		options = options.map((option) =>
			option.key === optionKey
				? {
						...option,
						values:
							option.values.filter((value) => value.key !== valueKey).length > 0
								? option.values.filter((value) => value.key !== valueKey)
								: [createOptionValue("", 1)],
					}
				: option
		);
	}

	function addManualVariant() {
		const seed = variantSeed();
		const nextVariants = [
			...variants,
			createVariant({
				sku: `${asTrimmedString(sku)}-${variants.length + 1}`.replace(/^-/, ""),
				title: `Variant ${variants.length + 1}`,
				price: seed.price,
				compare_at_price: seed.compare_at_price,
				stock: seed.stock,
			}),
		];
		variants = nextVariants;
		if (!defaultVariantSku) {
			defaultVariantSku = nextVariants[0]?.sku ?? "";
		}
	}

	function removeVariant(variantKey: string) {
		const remaining = variants.filter((variant) => variant.key !== variantKey);
		variants = remaining.length > 0 ? remaining : [createVariant()];
		if (!variants.some((variant) => variant.sku === defaultVariantSku)) {
			defaultVariantSku = variants[0]?.sku ?? "";
		}
	}

	function optionValueMatrix() {
		return options
			.map((option) => ({
				name: asTrimmedString(option.name),
				values: option.values.map((value) => asTrimmedString(value.value)).filter(Boolean),
			}))
			.filter((option) => option.name !== "" && option.values.length > 0);
	}

	function variantSelectionKey(
		selections: Array<{ option_name: string; option_value: string }>
	): string {
		return selections
			.map(
				(selection) =>
					`${selection.option_name.toLowerCase()}=${selection.option_value.toLowerCase()}`
			)
			.sort()
			.join("|");
	}

	function buildVariantSku(baseSku: string, selections: string[]): string {
		const suffix = selections
			.map((value) =>
				value
					.toUpperCase()
					.replace(/[^A-Z0-9]+/g, "-")
					.replace(/^-+|-+$/g, "")
			)
			.filter(Boolean)
			.join("-");
		return suffix ? `${baseSku}-${suffix}` : baseSku;
	}

	function generateVariantsFromOptions() {
		const matrix = optionValueMatrix();
		if (matrix.length === 0) {
			const seed = variantSeed();
			variants = [
				createVariant({
					sku: asTrimmedString(sku),
					title: asTrimmedString(name) || "Default Variant",
					price: seed.price,
					compare_at_price: seed.compare_at_price,
					stock: seed.stock,
				}),
			];
			defaultVariantSku = variants[0]?.sku ?? "";
			return;
		}

		const existingByKey = new Map(
			variants.map((variant) => [variantSelectionKey(variant.selections), variant])
		);

		let combinations: Array<Array<{ option_name: string; option_value: string }>> = [[]];
		for (const option of matrix) {
			combinations = combinations.flatMap((selectionSet) =>
				option.values.map((value) => [
					...selectionSet,
					{ option_name: option.name, option_value: value },
				])
			);
		}

		const generated = combinations.map((selectionSet) => {
			const selectionKey = variantSelectionKey(selectionSet);
			const existing = existingByKey.get(selectionKey);
			const seed = variantSeed();
			return createVariant({
				key: existing?.key ?? nextEditorKey("variant"),
				sku:
					existing?.sku ??
					buildVariantSku(
						asTrimmedString(sku),
						selectionSet.map((item) => item.option_value)
					),
				title: existing?.title ?? selectionSet.map((item) => item.option_value).join(" / "),
				price: existing?.price ?? seed.price,
				compare_at_price: existing?.compare_at_price ?? seed.compare_at_price,
				stock: existing?.stock ?? seed.stock,
				is_published: existing?.is_published ?? true,
				selections: selectionSet.map((selection, selectionIndex) =>
					createVariantSelection(selection.option_name, selection.option_value, selectionIndex + 1)
				),
			});
		});

		variants = generated;
		if (!generated.some((variant) => variant.sku === defaultVariantSku)) {
			defaultVariantSku = generated[0]?.sku ?? "";
		}
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
		clearMessages("product");
		if (!seedProduct) {
			product = null;
			resetForm();
		}
		try {
			const fetched = await api.getAdminProduct(id);
			if (sequence !== loadSequence) {
				return;
			}
			product = fetched;
			hydrateForm(fetched);
			captureSavedSnapshot();
			onProductUpdated?.(fetched);
		} catch (err) {
			console.error(err);
			if (sequence === loadSequence) {
				setMessage("product", "error", "Unable to load product.");
			}
		} finally {
			if (sequence === loadSequence) {
				loading = false;
			}
		}
	}

	async function saveProduct() {
		clearMessages("product");
		saving = true;
		try {
			const payload = buildProductPayload();

			if (!payload.sku || !payload.name) {
				setMessage("product", "error", "Please provide SKU and product name.");
				return;
			}
			if (payload.variants.length === 0) {
				setMessage("product", "error", "Add at least one variant before saving.");
				return;
			}
			if (
				payload.variants.some(
					(variant) => Number.isNaN(variant.price) || Number.isNaN(variant.stock)
				)
			) {
				setMessage("product", "error", "Each variant needs a valid price and stock value.");
				return;
			}
			const attributeError = validateProductAttributes();
			if (attributeError) {
				setMessage("product", "error", attributeError);
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
				applyUpdatedProduct(merged);
				setMessage("product", "success", "Product draft saved.");
			} else if (allowCreate) {
				updated = await api.createProduct(payload);
				productId = updated.id;
				applyUpdatedProduct(updated);
				onProductCreated?.(updated);
				setMessage("product", "success", "Product draft created.");
			} else {
				setMessage("product", "error", "Please select a product to edit.");
			}
		} catch (err) {
			console.error(err);
			setMessage("product", "error", "Unable to save product.");
		} finally {
			saving = false;
		}
	}

	async function publishProduct() {
		if (!resolvedProductId) {
			return;
		}
		clearMessages("product");
		publishing = true;
		try {
			if (hasUnsavedChanges) {
				await saveAllPendingChanges();
				if (hasUnsavedChanges) {
					return;
				}
			}
			const updated = await api.publishProduct(resolvedProductId);
			applyUpdatedProduct(updated);
			setMessage("product", "success", "Product draft published.");
		} catch (err) {
			console.error(err);
			setMessage("product", "error", readableActionError(err, "Unable to publish product draft."));
		} finally {
			publishing = false;
		}
	}

	async function discardDraft() {
		if (!resolvedProductId) {
			return;
		}
		if (!confirm("Discard all unpublished draft changes for this product?")) {
			return;
		}
		clearMessages("product");
		discardingDraft = true;
		try {
			const updated = await api.discardProductDraft(resolvedProductId);
			applyUpdatedProduct(updated);
			setMessage("product", "success", "Product draft discarded.");
		} catch (err) {
			console.error(err);
			setMessage("product", "error", readableActionError(err, "Unable to discard product draft."));
		} finally {
			discardingDraft = false;
		}
	}

	async function unpublishProduct() {
		if (!resolvedProductId || !isPublished) {
			return;
		}
		if (!confirm("Unpublish this product? It will be hidden from the public storefront.")) {
			return;
		}
		clearMessages("product");
		unpublishing = true;
		try {
			if (hasUnsavedChanges) {
				await saveAllPendingChanges();
				if (hasUnsavedChanges) {
					return;
				}
			}
			const updated = await api.unpublishProduct(resolvedProductId);
			applyUpdatedProduct(updated);
			setMessage("product", "success", "Product unpublished.");
		} catch (err) {
			console.error(err);
			setMessage("product", "error", readableActionError(err, "Unable to unpublish product."));
		} finally {
			unpublishing = false;
		}
	}

	async function previewDraft() {
		if (!resolvedProductId) {
			return;
		}
		clearMessages("product");
		previewingDraft = true;
		let previewWindow: Window | null = null;
		try {
			if (previewActive) {
				await api.stopAdminPreview();
				previewActive = false;
				setMessage("product", "success", "Exited draft preview.");
				return;
			}

			previewWindow = window.open("", "_blank");
			if (!previewWindow) {
				setMessage("product", "error", "Preview popup was blocked by the browser.");
				return;
			}
			await api.startAdminPreview();
			previewActive = true;
			previewWindow.location.href = resolve(`/product/${resolvedProductId}`);
			setMessage("product", "success", "Opened draft preview in a new tab.");
		} catch (err) {
			console.error(err);
			if (previewWindow && !previewWindow.closed) {
				previewWindow.close();
			}
			setMessage(
				"product",
				"error",
				readableActionError(err, "Unable to open product draft preview.")
			);
			void loadPreviewState();
		} finally {
			previewingDraft = false;
		}
	}

	async function deleteProduct() {
		if (!resolvedProductId) {
			return;
		}
		if (!confirm("Delete this product? This cannot be undone.")) {
			return;
		}
		clearMessages("product");
		deleting = true;
		try {
			const deletedId = resolvedProductId;
			await api.deleteProduct(deletedId);
			product = null;
			resetForm();
			onProductDeleted?.(deletedId);
			setMessage("product", "success", "Product deleted.");
			if (clearOnDelete) {
				productId = null;
			}
		} catch (err) {
			console.error(err);
			setMessage("product", "error", "Unable to delete product.");
		} finally {
			deleting = false;
		}
	}

	async function uploadMedia() {
		if (!resolvedProductId || !mediaFiles || mediaFiles.length === 0) {
			return;
		}
		clearMessages("media");
		uploading = true;
		try {
			const mediaIds = await uploadMediaFiles(api, mediaFiles);
			const updated = await api.attachProductMedia(resolvedProductId, mediaIds);
			applyUpdatedProduct(updated);
			setMessage("media", "success", "Media attached.");
		} catch (err) {
			console.error(err);
			const error = err as { status?: number; body?: { error?: string } };
			if (error.status === 409 && error.body?.error) {
				setMessage("media", "error", error.body.error);
			} else {
				setMessage("media", "error", "Unable to upload media.");
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
			setMessage("media", "error", "Unable to find media ID for deletion.");
			return;
		}
		if (!confirm("Remove this image from the product?")) {
			return;
		}
		clearMessages("media");
		mediaDeleting = mediaId;
		try {
			const updated = await api.detachProductMedia(resolvedProductId, mediaId);
			applyUpdatedProduct(updated);
			setMessage("media", "success", "Media removed.");
		} catch (err) {
			console.error(err);
			setMessage("media", "error", "Unable to remove media.");
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

	function discardMediaOrderChanges() {
		pendingMediaOrder = null;
	}

	async function saveMediaOrder() {
		if (!resolvedProductId || !hasPendingMediaOrder || !pendingMediaOrder) {
			return;
		}

		const mediaIds = pendingMediaOrder
			.map((url) => extractMediaId(url))
			.filter((id): id is string => Boolean(id));

		if (mediaIds.length !== pendingMediaOrder.length) {
			setMessage("media", "error", "Unable to reorder media.");
			return;
		}

		mediaReordering = true;
		clearMessages("media");
		try {
			const updated = await api.updateProductMediaOrder(resolvedProductId, mediaIds);
			applyUpdatedProduct(updated, { hydrate: false });
			pendingMediaOrder = null;
			setMessage("media", "success", "Image order updated.");
		} catch (err) {
			console.error(err);
			setMessage("media", "error", "Unable to update image order.");
		} finally {
			mediaReordering = false;
		}
	}

	async function searchRelatedProducts() {
		const query = relatedQuery.trim();
		if (!resolvedProductId || !query) {
			relatedOptions = [];
			relatedLastSearchedQuery = "";
			return;
		}
		relatedLoading = true;
		relatedLastSearchedQuery = query;
		try {
			const page = await api.listAdminProducts({
				q: query,
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
			setMessage("related", "error", "Unable to search related products.");
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
				description: option.description,
				price: option.price,
				stock: option.stock,
				cover_image: option.images[0],
			},
		];
		relatedOptions = relatedOptions.filter((item) => item.id !== option.id);
	}

	function removeRelatedProduct(productIdToRemove: number) {
		relatedSelected = relatedSelected.filter((item) => item.id !== productIdToRemove);
	}

	function discardRelatedChanges() {
		relatedSelected = relatedBaseline;
		relatedOptions = [];
		relatedQuery = "";
		relatedLastSearchedQuery = "";
		clearMessages("related");
	}

	async function saveRelatedProducts() {
		if (!resolvedProductId) {
			return;
		}
		relatedSaving = true;
		clearMessages("related");
		try {
			const updated = await api.updateProductRelated(
				resolvedProductId,
				relatedSelected.map((item) => item.id)
			);
			applyUpdatedProduct(updated);
			setMessage("related", "success", "Related products updated.");
		} catch (err) {
			console.error(err);
			setMessage("related", "error", "Unable to update related products.");
		} finally {
			relatedSaving = false;
		}
	}

	function clearSelection() {
		productId = null;
		product = null;
		resetForm();
		clearAllMessages();
		captureSavedSnapshot();
	}

	async function saveAllPendingChanges() {
		if (hasPendingProductChanges) {
			await saveProduct();
		}
		if (hasPendingUploadSelection) {
			await uploadMedia();
		}
		if (hasPendingMediaOrder) {
			await saveMediaOrder();
		}
		if (hasPendingRelatedChanges) {
			await saveRelatedProducts();
		}
	}

	$effect(() => {
		const dirty = hasUnsavedChanges;
		if (onDirtyChange !== lastDirtyHandler || dirty !== lastDirtyNotification) {
			lastDirtyHandler = onDirtyChange;
			lastDirtyNotification = dirty;
			onDirtyChange?.(dirty);
		}
		if (onSaveRequestChange !== lastSaveHandler || dirty !== lastSaveActionDirty) {
			lastSaveHandler = onSaveRequestChange;
			lastSaveActionDirty = dirty;
			onSaveRequestChange?.(dirty ? saveAllPendingChanges : null);
		}
	});

	const editorPriceRangePreview = $derived.by(() => {
		const prices = variants
			.map((variant) => normalizedNumber(variant.price))
			.filter((value): value is number => typeof value === "number");
		if (prices.length === 0) {
			return "Set variant prices to preview range";
		}
		const min = Math.min(...prices);
		const max = Math.max(...prices);
		return min === max ? String(min) : `${min} to ${max}`;
	});

	$effect(() => {
		if (variants.length === 0) {
			defaultVariantSku = "";
			return;
		}
		if (!defaultVariantSku || !variants.some((variant) => variant.sku === defaultVariantSku)) {
			defaultVariantSku = variants[0]?.sku ?? "";
		}
	});

	onDestroy(() => {
		if (typeof window !== "undefined") {
			window.removeEventListener(DRAFT_PREVIEW_SYNC_EVENT, handlePreviewSyncEvent as EventListener);
			window.removeEventListener("storage", handlePreviewStorageEvent);
		}
		onDirtyChange?.(false);
		onSaveRequestChange?.(null);
	});

	onMount(() => {
		window.addEventListener(DRAFT_PREVIEW_SYNC_EVENT, handlePreviewSyncEvent as EventListener);
		window.addEventListener("storage", handlePreviewStorageEvent);
		void loadBrands();
		void loadCategories();
		void loadAttributeDefinitions();
		void loadPreviewState();
	});

	$effect(() => {
		if (resolvedProductId) {
			const seed =
				initialProduct && initialProduct.id === resolvedProductId ? initialProduct : null;
			const seedSignature = seed ? `${seed.id}:${seed.updated_at.getTime()}` : "";
			if (resolvedProductId !== activeSelectionId) {
				activeSelectionId = resolvedProductId;
				lastSeedSignature = "";
			}
			if (seed && seedSignature !== lastSeedSignature) {
				product = seed;
				hydrateForm(seed);
				captureSavedSnapshot();
				lastSeedSignature = seedSignature;
			}
			if (resolvedProductId !== lastLoadedId) {
				lastLoadedId = resolvedProductId;
				void loadProduct(resolvedProductId, seed);
			}
		} else {
			if (activeSelectionId !== null || savedSnapshot === "") {
				loadSequence += 1;
				loading = false;
				product = null;
				resetForm();
				clearAllMessages();
				lastLoadedId = null;
				lastSeedSignature = "";
				activeSelectionId = null;
				captureSavedSnapshot();
			}
		}
	});
</script>

{#snippet BasicInfoSection()}
	<div>
		<AdminFieldLabel as="label" for="admin-product-name">Name</AdminFieldLabel>
		<TextInput
			tone="admin"
			id="admin-product-name"
			name="name"
			class="mt-1"
			type="text"
			bind:value={name}
		/>
	</div>
	<div>
		<AdminFieldLabel as="label" for="admin-product-subtitle">Subtitle</AdminFieldLabel>
		<TextInput
			tone="admin"
			id="admin-product-subtitle"
			name="subtitle"
			class="mt-1"
			type="text"
			bind:value={subtitle}
		/>
	</div>
	<div>
		<AdminFieldLabel as="label" for="admin-product-sku">SKU</AdminFieldLabel>
		<TextInput
			tone="admin"
			id="admin-product-sku"
			name="sku"
			class="mt-1"
			type="text"
			bind:value={sku}
		/>
	</div>
	<div>
		<AdminFieldLabel as="label" for="admin-product-brand">Brand</AdminFieldLabel>
		<Dropdown tone="admin" id="admin-product-brand" class="mt-1" bind:value={selectedBrandId}>
			<option value="">No brand</option>
			{#each brands as brand (brand.id)}
				<option value={String(brand.id)}>{brand.name}</option>
			{/each}
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
								{#if !category.is_active}
									<span class="text-[10px] font-medium text-stone-500 dark:text-stone-400">
										Inactive
									</span>
								{/if}
								<button
									type="button"
									class="inline-flex h-5 w-5 items-center justify-center rounded-full text-stone-500 hover:bg-stone-200 hover:text-stone-900 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-stone-500 dark:text-stone-400 dark:hover:bg-stone-800 dark:hover:text-stone-50"
									aria-label={`Remove category ${category.name}`}
									onclick={() => removeCategory(category.id)}
								>
									<i class="bi bi-x text-xs"></i>
								</button>
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
						bind:value={categorySearchQuery}
						onfocus={() => (categoryMenuOpen = true)}
						oninput={() => (categoryMenuOpen = true)}
						onkeydown={handleCategorySearchKeydown}
					/>
					{#if categoryMenuOpen}
						<div
							class="absolute z-20 mt-2 max-h-64 w-full overflow-y-auto rounded-lg border border-stone-200 bg-white p-1 shadow-lg dark:border-stone-800 dark:bg-stone-950"
						>
							{#if availableCategoryOptions.length === 0}
								<p class="px-3 py-2 text-sm text-stone-500 dark:text-stone-400">
									No matching categories
								</p>
							{:else}
								{#each availableCategoryOptions as category (category.id)}
									<button
										type="button"
										class="flex w-full items-center justify-between gap-3 rounded-md px-3 py-2 text-left text-sm text-stone-700 hover:bg-stone-100 focus-visible:bg-stone-100 focus-visible:outline-none dark:text-stone-200 dark:hover:bg-stone-900 dark:focus-visible:bg-stone-900"
										onclick={() => addCategory(category)}
									>
										<span class="min-w-0">
											<span class="block truncate font-medium">
												{" ".repeat(category.depth * 2)}{category.name}
											</span>
											<span class="block truncate text-xs text-stone-500 dark:text-stone-400">
												/{category.slug}
											</span>
										</span>
										{#if !category.is_active}
											<Badge tone="neutral" size="sm">Inactive</Badge>
										{/if}
									</button>
								{/each}
							{/if}
						</div>
					{/if}
				</div>
			</div>
		{/if}
	</div>
	<div class="sm:col-span-2">
		<AdminFieldLabel as="label" for="admin-product-description">Description</AdminFieldLabel>
		<TextArea
			tone="admin"
			id="admin-product-description"
			name="description"
			class="mt-1"
			rows={4}
			bind:value={description}
		/>
	</div>
{/snippet}

{#snippet OptionsSection(layoutMode: "split" | "stacked")}
	<div class={editorSectionClass(layoutMode)}>
		<div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
			<div>
				<AdminFieldLabel>Options</AdminFieldLabel>
				<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
					Define the choice sets that can be combined into sellable variants.
				</p>
			</div>
			<div
				class={layoutMode === "split"
					? "flex w-full shrink-0 flex-col gap-2 sm:w-48"
					: "flex flex-wrap gap-2"}
			>
				<Button
					tone="admin"
					variant="regular"
					type="button"
					class={layoutMode === "split" ? "w-full justify-center whitespace-nowrap" : ""}
					onclick={addOption}
				>
					<i class="bi bi-plus-lg mr-1"></i>
					Add option
				</Button>
				<Button
					variant="primary"
					type="button"
					class={layoutMode === "split" ? "w-full justify-center whitespace-nowrap" : ""}
					onclick={generateVariantsFromOptions}
				>
					<i class="bi bi-grid-3x3-gap-fill mr-1"></i>
					Generate variants
				</Button>
			</div>
		</div>
		{#if options.length === 0}
			<p class="mt-3 text-sm text-gray-500 dark:text-gray-400">
				No options yet. Add one to build a variant matrix.
			</p>
		{:else}
			<div class={editorCollectionClass(layoutMode)}>
				{#each options as option, optionIndex (option.key)}
					<div class={editorItemClass(layoutMode)}>
						<div class="flex items-start justify-between gap-3">
							<div class="grid flex-1 gap-4 sm:grid-cols-2">
								<div>
									<AdminFieldLabel>Option name</AdminFieldLabel>
									<TextInput
										tone="admin"
										class="mt-1"
										type="text"
										aria-label={`Option ${optionIndex + 1} name`}
										bind:value={option.name}
									/>
								</div>
								<div>
									<AdminFieldLabel>Display type</AdminFieldLabel>
									<Dropdown
										tone="admin"
										class="mt-1"
										aria-label={`Option ${optionIndex + 1} display type`}
										bind:value={option.display_type}
									>
										<option value="select">Select</option>
										<option value="swatch">Swatch</option>
									</Dropdown>
								</div>
							</div>
							<IconButton
								variant="danger"
								type="button"
								onclick={() => removeOption(option.key)}
								aria-label={`Remove option ${optionIndex + 1}`}
								title="Remove option"
							>
								<i class="bi bi-trash-fill"></i>
							</IconButton>
						</div>

						<div class="mt-4 space-y-3">
							{#each option.values as value (value.key)}
								<div class="flex items-center gap-2">
									<TextInput
										tone="admin"
										class="flex-1"
										type="text"
										aria-label={`${option.name || `Option ${optionIndex + 1}`} value`}
										bind:value={value.value}
									/>
									<IconButton
										variant="danger"
										type="button"
										onclick={() => removeOptionValue(option.key, value.key)}
										aria-label={`Remove value ${value.value || "value"}`}
										title="Remove value"
									>
										<i class="bi bi-dash-lg"></i>
									</IconButton>
								</div>
							{/each}
							<Button
								tone="admin"
								variant="regular"
								type="button"
								onclick={() => addOptionValue(option.key)}
							>
								<i class="bi bi-plus-lg mr-1"></i>
								Add value
							</Button>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>
{/snippet}

{#snippet VariantsSection(layoutMode: "split" | "stacked")}
	<div class={editorSectionClass(layoutMode)}>
		<div class="flex items-center justify-between gap-3">
			<div>
				<AdminFieldLabel>Variants</AdminFieldLabel>
				<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
					Product price and stock are derived from the default variant.
				</p>
			</div>
			<Button
				tone="admin"
				variant="regular"
				type="button"
				class="min-w-38 whitespace-nowrap"
				onclick={addManualVariant}
			>
				<i class="bi bi-plus-lg mr-1"></i>
				Add variant
			</Button>
		</div>

		<div class={editorCollectionClass(layoutMode)}>
			{#each variants as variant, variantIndex (variant.key)}
				<div class={editorItemClass(layoutMode)}>
					<div class="flex items-start justify-between gap-3">
						<div
							class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-200"
						>
							<input
								type="radio"
								name="default-variant"
								value={variant.sku}
								checked={defaultVariantSku === variant.sku}
								onchange={() => (defaultVariantSku = variant.sku)}
							/>
							Default variant
						</div>
						<IconButton
							variant="danger"
							type="button"
							onclick={() => removeVariant(variant.key)}
							aria-label={`Remove variant ${variantIndex + 1}`}
							title="Remove variant"
						>
							<i class="bi bi-trash-fill"></i>
						</IconButton>
					</div>

					<div class="mt-4 grid gap-4 sm:grid-cols-2">
						<div>
							<AdminFieldLabel>Variant SKU</AdminFieldLabel>
							<TextInput
								tone="admin"
								class="mt-1"
								type="text"
								aria-label={`Variant ${variantIndex + 1} SKU`}
								bind:value={variant.sku}
							/>
						</div>
						<div>
							<AdminFieldLabel>Title</AdminFieldLabel>
							<TextInput
								tone="admin"
								class="mt-1"
								type="text"
								aria-label={`Variant ${variantIndex + 1} title`}
								bind:value={variant.title}
							/>
						</div>
						<div>
							<AdminFieldLabel>Price</AdminFieldLabel>
							<NumberInput
								tone="admin"
								class="mt-1"
								allowDecimal={true}
								min="0"
								aria-label={`Variant ${variantIndex + 1} price`}
								bind:value={variant.price}
							/>
						</div>
						<div>
							<AdminFieldLabel>Stock</AdminFieldLabel>
							<NumberInput
								tone="admin"
								class="mt-1"
								min="0"
								aria-label={`Variant ${variantIndex + 1} stock`}
								bind:value={variant.stock}
							/>
						</div>
						<div>
							<AdminFieldLabel>Compare-at price</AdminFieldLabel>
							<NumberInput
								tone="admin"
								class="mt-1"
								allowDecimal={true}
								min="0"
								aria-label={`Variant ${variantIndex + 1} compare-at price`}
								bind:value={variant.compare_at_price}
							/>
						</div>
						<label class="mt-6 flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
							<input type="checkbox" bind:checked={variant.is_published} />
							Variant published
						</label>
					</div>

					{#if variant.selections.length}
						<div class="mt-4 flex flex-wrap gap-2">
							{#each variant.selections as selection (selection.key)}
								<Badge tone="neutral" size="sm">
									{selection.option_name}: {selection.option_value}
								</Badge>
							{/each}
						</div>
					{/if}
				</div>
			{/each}
		</div>
	</div>
{/snippet}

{#snippet AttributesSection(layoutMode: "split" | "stacked")}
	<div class={editorSectionClass(layoutMode)}>
		<div class="flex items-center justify-between gap-3">
			<div>
				<AdminFieldLabel>Attributes</AdminFieldLabel>
				<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
					Assign typed merchandising attributes for filtering and discovery.
				</p>
			</div>
			<Button
				tone="admin"
				variant="regular"
				type="button"
				class="min-w-38 whitespace-nowrap"
				onclick={addAttributeValue}
			>
				<i class="bi bi-plus-lg mr-1"></i>
				Add attribute
			</Button>
		</div>

		{#if attributeValues.length === 0}
			<p class="mt-3 text-sm text-gray-500 dark:text-gray-400">No attributes assigned yet.</p>
		{:else}
			<div class={editorCollectionClass(layoutMode)}>
				{#each attributeValues as attribute, attributeIndex (attribute.key)}
					{@const definition = attributeDefinitionById(attribute.product_attribute_id)}
					<div class={editorItemClass(layoutMode)}>
						<div class="grid gap-4 md:grid-cols-[minmax(0,1fr)_auto]">
							<div class="grid gap-4 sm:grid-cols-2">
								<div>
									<AdminFieldLabel>Attribute</AdminFieldLabel>
									<Dropdown
										tone="admin"
										class="mt-1"
										aria-label={`Attribute ${attributeIndex + 1}`}
										value={attribute.product_attribute_id}
										onchange={(event) =>
											updateAttributeSelection(
												attribute.key,
												(event.target as HTMLSelectElement).value
											)}
									>
										<option value="">Select attribute</option>
										{#each attributeDefinitions as definition (definition.id)}
											<option
												value={String(definition.id)}
												disabled={attributeDefinitionAlreadyAssigned(definition.id, attribute.key)}
											>
												{definition.key}
											</option>
										{/each}
									</Dropdown>
								</div>
								<div>
									<AdminFieldLabel>Value</AdminFieldLabel>
									{#if attribute.type === "number"}
										<NumberInput
											tone="admin"
											class="mt-1"
											allowDecimal={true}
											aria-label={`Attribute ${attributeIndex + 1} value`}
											bind:value={attribute.number_value}
										/>
									{:else if attribute.type === "boolean"}
										<label
											class="mt-3 flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200"
										>
											<input type="checkbox" bind:checked={attribute.boolean_value} />
											Enabled
										</label>
									{:else if attribute.type === "enum"}
										<Dropdown
											tone="admin"
											class="mt-1"
											aria-label={`Attribute ${attributeIndex + 1} enum value`}
											bind:value={attribute.enum_value}
										>
											<option value="">Select value</option>
											{#each definition?.enum_values ?? [] as enumValue (enumValue)}
												<option value={enumValue}>{enumValue}</option>
											{/each}
										</Dropdown>
									{:else}
										<TextInput
											tone="admin"
											class="mt-1"
											type="text"
											aria-label={`Attribute ${attributeIndex + 1} text value`}
											bind:value={attribute.text_value}
										/>
									{/if}
								</div>
							</div>
							<div class="flex justify-end">
								<IconButton
									variant="danger"
									type="button"
									onclick={() => removeAttributeValue(attribute.key)}
									aria-label={`Remove attribute ${attributeIndex + 1}`}
									title="Remove attribute"
								>
									<i class="bi bi-trash-fill"></i>
								</IconButton>
							</div>
						</div>
					</div>
				{/each}
			</div>
		{/if}

		{#if attributeDefinitions.length === 0}
			<p class="mt-3 text-xs text-amber-600 dark:text-amber-300">
				No product attribute definitions exist yet.
			</p>
		{/if}

		<div class={`${mutedEditorPanelClass(layoutMode)} mt-5`}>
			<div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
				<div>
					<AdminFieldLabel>Attribute definitions</AdminFieldLabel>
					<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
						Create reusable product attributes before assigning them above.
					</p>
				</div>
				{#if attributeDefinitionEditingId != null}
					<Button
						tone="admin"
						variant="regular"
						size="small"
						type="button"
						onclick={resetAttributeDefinitionForm}
					>
						Cancel edit
					</Button>
				{/if}
			</div>

			{#if attributeDefinitionErrorMessage}
				<div class="mt-4">
					<Alert
						message={attributeDefinitionErrorMessage}
						tone="error"
						onClose={() => (attributeDefinitionErrorMessage = "")}
					/>
				</div>
			{/if}
			{#if attributeDefinitionStatusMessage}
				<div class="mt-4">
					<Alert
						message={attributeDefinitionStatusMessage}
						tone="success"
						onClose={() => (attributeDefinitionStatusMessage = "")}
					/>
				</div>
			{/if}

			<div class="mt-4 grid gap-3 sm:grid-cols-2">
				<div>
					<AdminFieldLabel>Definition name</AdminFieldLabel>
					<TextInput
						tone="admin"
						class="mt-1"
						type="text"
						aria-label="Attribute definition name"
						bind:value={attributeDefinitionKey}
					/>
				</div>
				<div>
					<AdminFieldLabel>Slug</AdminFieldLabel>
					<TextInput
						tone="admin"
						class="mt-1"
						type="text"
						aria-label="Attribute definition slug"
						placeholder="Generated when blank"
						bind:value={attributeDefinitionSlug}
					/>
				</div>
				<div>
					<AdminFieldLabel>Type</AdminFieldLabel>
					<Dropdown
						tone="admin"
						class="mt-1"
						aria-label="Attribute definition type"
						value={attributeDefinitionType}
						onchange={(event) =>
							updateAttributeDefinitionType(
								(event.target as HTMLSelectElement).value as ProductAttributeDefinitionModel["type"]
							)}
					>
						<option value="text">Text</option>
						<option value="number">Number</option>
						<option value="boolean">Boolean</option>
						<option value="enum">Enum</option>
					</Dropdown>
				</div>
				{#if attributeDefinitionType === "enum"}
					<div>
						<AdminFieldLabel>Allowed values</AdminFieldLabel>
						<TextArea
							tone="admin"
							class="mt-1 min-h-24"
							aria-label="Attribute definition enum values"
							placeholder="One value per line"
							bind:value={attributeDefinitionEnumValuesText}
						/>
					</div>
				{/if}
				<div class="flex flex-col justify-end gap-2 pt-2">
					<label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
						<input type="checkbox" bind:checked={attributeDefinitionFilterable} />
						Filterable
					</label>
					<label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
						<input
							type="checkbox"
							bind:checked={attributeDefinitionSortable}
							disabled={attributeDefinitionType !== "number"}
						/>
						Sortable
					</label>
				</div>
			</div>

			<div class="mt-4 flex justify-end">
				<Button
					tone="admin"
					variant="primary"
					type="button"
					disabled={attributeDefinitionSaving}
					onclick={saveAttributeDefinition}
				>
					{attributeDefinitionSaving
						? "Saving..."
						: attributeDefinitionEditingId == null
							? "Create definition"
							: "Update definition"}
				</Button>
			</div>

			{#if attributeDefinitions.length}
				<div class="mt-5 divide-y divide-stone-200 dark:divide-stone-800">
					{#each attributeDefinitions as definition (definition.id)}
						<div class="flex items-center justify-between gap-3 py-3 text-sm">
							<div class="min-w-0">
								<div class="font-medium text-stone-900 dark:text-stone-100">
									{definition.key}
								</div>
								<div
									class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-stone-500 dark:text-stone-400"
								>
									<span>{definition.slug}</span>
									<span>{definition.type}</span>
									{#if definition.type === "enum" && definition.enum_values.length}
										<span>{definition.enum_values.join(", ")}</span>
									{/if}
									{#if definition.filterable}
										<span>filterable</span>
									{/if}
									{#if definition.sortable}
										<span>sortable</span>
									{/if}
								</div>
							</div>
							<div class="flex shrink-0 gap-2">
								<IconButton
									variant="neutral"
									type="button"
									onclick={() => editAttributeDefinition(definition)}
									aria-label={`Edit attribute definition ${definition.key}`}
									title="Edit definition"
								>
									<i class="bi bi-pencil-fill"></i>
								</IconButton>
								<IconButton
									variant="danger"
									type="button"
									disabled={attributeDefinitionDeletingId === definition.id}
									onclick={() => deleteAttributeDefinition(definition)}
									aria-label={`Delete attribute definition ${definition.key}`}
									title="Delete definition"
								>
									<i class="bi bi-trash-fill"></i>
								</IconButton>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	</div>
{/snippet}

{#snippet SEOSection(layoutMode: "split" | "stacked")}
	<div class={editorSectionClass(layoutMode)}>
		<AdminFieldLabel>SEO</AdminFieldLabel>
		<div class="mt-4 grid gap-4 sm:grid-cols-2">
			<div>
				<AdminFieldLabel>SEO title</AdminFieldLabel>
				<TextInput
					tone="admin"
					class="mt-1"
					type="text"
					aria-label="SEO title"
					bind:value={seoTitle}
				/>
			</div>
			<div>
				<AdminFieldLabel>Canonical path</AdminFieldLabel>
				<TextInput
					tone="admin"
					class="mt-1"
					type="text"
					aria-label="Canonical path"
					bind:value={seoCanonicalPath}
				/>
			</div>
			<div class="sm:col-span-2">
				<AdminFieldLabel>SEO description</AdminFieldLabel>
				<TextArea
					tone="admin"
					class="mt-1"
					rows={3}
					aria-label="SEO description"
					bind:value={seoDescription}
				/>
			</div>
			<div>
				<AdminFieldLabel>OG image media ID</AdminFieldLabel>
				<TextInput
					tone="admin"
					class="mt-1"
					type="text"
					aria-label="OG image media ID"
					bind:value={seoOgImageMediaId}
				/>
			</div>
			<label class="mt-6 flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
				<input type="checkbox" bind:checked={seoNoIndex} />
				Prevent indexing
			</label>
		</div>
	</div>
{/snippet}

{#snippet VariantSummarySection()}
	<div class="grid gap-4 sm:grid-cols-2">
		<div>
			<AdminFieldLabel>Default variant</AdminFieldLabel>
			<AdminMetaText tone="strong" class="mt-1">
				{defaultVariantSku || variants[0]?.sku || "No default variant selected"}
			</AdminMetaText>
		</div>
		<div>
			<AdminFieldLabel>Price range preview</AdminFieldLabel>
			<AdminMetaText tone="strong" class="mt-1">{editorPriceRangePreview}</AdminMetaText>
		</div>
	</div>
{/snippet}

{#snippet ProductStateChips()}
	{#if canEditProduct}
		<div class="mt-1 flex flex-wrap items-center gap-2 text-xs">
			<Badge tone={isPublished ? "success" : "warning"} size="sm">
				{isPublished ? "Published" : "Unpublished"}
			</Badge>
			{#if hasDraftChanges}
				<Badge tone="info" size="sm">Draft changes</Badge>
			{/if}
		</div>
	{/if}
{/snippet}

{#snippet DismissibleAlert(
	scope: MessageScope,
	tone: MessageTone,
	message: string,
	marginClass: string = "mt-4"
)}
	<div class={marginClass}>
		<Alert
			{message}
			{tone}
			icon={tone === "error" ? "bi-x-circle-fill" : "bi-check-circle-fill"}
			onClose={() => clearMessage(scope, tone)}
		/>
	</div>
{/snippet}

{#snippet ProductActionButtons(layoutMode: "split" | "stacked")}
	{@const isStacked = layoutMode === "stacked"}
	<Button
		tone="admin"
		variant="primary"
		size={isStacked ? "large" : "regular"}
		class={isStacked ? `w-full ${canEditProduct ? "" : "sm:col-span-2"}` : "min-w-40"}
		type="button"
		onclick={saveProduct}
		disabled={saving}
	>
		<i
			class={`bi ${
				saving
					? "bi-floppy-fill"
					: isStacked && !canEditProduct
						? "bi-patch-plus-fill"
						: "bi-floppy-fill"
			} mr-1`}
		></i>
		{saving ? "Saving..." : isStacked && !canEditProduct ? "Create draft" : "Save draft"}
	</Button>
	{#if canEditProduct}
		<Button
			tone="admin"
			variant="regular"
			size={isStacked ? "large" : "regular"}
			class={isStacked ? "w-full" : ""}
			type="button"
			disabled={previewingDraft}
			onclick={previewDraft}
		>
			<i class={`bi ${previewActive ? "bi-eye-slash-fill" : "bi-eye-fill"} mr-1`}></i>
			{previewingDraft
				? previewActive
					? "Exiting..."
					: "Opening..."
				: previewActive
					? "Exit preview"
					: "Preview"}
		</Button>
		<Button
			tone="admin"
			variant="success"
			size={isStacked ? "large" : "regular"}
			class={isStacked ? "w-full" : ""}
			type="button"
			disabled={publishing || (!hasDraftChanges && !hasUnsavedChanges)}
			onclick={publishProduct}
		>
			<i class="bi bi-send-check-fill mr-1"></i>
			{publishing ? "Publishing..." : "Publish"}
		</Button>
		<Button
			tone="admin"
			variant="warning"
			size={isStacked ? "large" : "regular"}
			class={isStacked ? "w-full" : ""}
			type="button"
			disabled={unpublishing || !isPublished}
			onclick={unpublishProduct}
		>
			<i class="bi bi-eye-slash-fill mr-1"></i>
			{unpublishing ? "Unpublishing..." : "Unpublish"}
		</Button>
		<Button
			tone="admin"
			variant="warning"
			size={isStacked ? "large" : "regular"}
			class={isStacked ? "w-full" : ""}
			type="button"
			disabled={discardingDraft || (!hasDraftChanges && !hasUnsavedChanges)}
			onclick={discardDraft}
		>
			<i class="bi bi-arrow-counterclockwise mr-1"></i>
			{discardingDraft ? "Discarding..." : "Discard draft"}
		</Button>
		<Button
			tone="admin"
			variant="danger"
			size={isStacked ? "large" : "regular"}
			class={isStacked ? "w-full" : ""}
			type="button"
			disabled={deleting}
			onclick={deleteProduct}
		>
			<i class="bi bi-trash-fill mr-1"></i>
			{deleting ? "Deleting..." : "Delete"}
		</Button>
	{/if}
{/snippet}

{#snippet MediaUpload(showHint: boolean, layoutMode: "split" | "stacked")}
	<div class={mutedEditorPanelClass(layoutMode)}>
		<AdminFieldLabel>Upload media</AdminFieldLabel>
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
			<Button
				tone="admin"
				variant="regular"
				type="button"
				disabled={!canEditProduct || uploading}
				onclick={() => mediaInputRef?.click()}
			>
				Choose files
			</Button>
			<Button
				tone="admin"
				variant="primary"
				type="button"
				disabled={!canEditProduct || uploading || !mediaFilesCount}
				onclick={uploadMedia}
			>
				{uploading ? "Uploading..." : "Attach uploads"}
			</Button>
			{#if mediaFilesCount}
				<span class="text-xs text-gray-500 dark:text-gray-400">{mediaFilesCount} selected</span>
			{/if}
		</div>
		{#if showHint && !canEditProduct}
			<p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
				Select a product to upload images.
			</p>
		{/if}
	</div>
{/snippet}

{#snippet MediaGrid(layoutMode: "split" | "stacked")}
	<div class="max-h-64 overflow-y-auto pr-1">
		<div class="grid grid-cols-2 gap-3">
			{#each mediaOrderView as image, index (image)}
				<div class={mediaItemClass(layoutMode)}>
					<img
						src={image}
						alt={product ? `${product.name} ${index + 1}` : `Product image ${index + 1}`}
						class="h-42 w-full object-cover"
					/>
					<IconButton
						tone="admin"
						class={`absolute top-2 right-2 ${overlayIconButtonSurfaceClass}`}
						size="sm"
						disabled={mediaDeleting !== null || mediaReordering}
						onclick={() => detachMedia(image)}
						aria-label="Remove image"
						title="Remove image"
						variant="danger"
					>
						{#if mediaDeleting && extractMediaId(image) === mediaDeleting}
							<i class="bi bi-arrow-repeat inline-block animate-spin"></i>
						{:else}
							<i class="bi bi-trash"></i>
						{/if}
					</IconButton>
					<div class={`absolute right-2 bottom-2 ${overlayIconButtonClusterClass}`}>
						<IconButton
							tone="admin"
							class={`${overlayIconButtonSurfaceClass} ${overlayIconButtonMiniClass}`}
							size="sm"
							disabled={mediaReordering || index === 0}
							onclick={() => moveMedia(index, -1)}
							aria-label="Move image left"
							title="Move image left"
						>
							<i class="bi bi-chevron-left"></i>
						</IconButton>
						<IconButton
							tone="admin"
							class={`${overlayIconButtonSurfaceClass} ${overlayIconButtonMiniClass}`}
							size="sm"
							disabled={mediaReordering || index === mediaOrderView.length - 1}
							onclick={() => moveMedia(index, 1)}
							aria-label="Move image right"
							title="Move image right"
						>
							<i class="bi bi-chevron-right"></i>
						</IconButton>
					</div>
				</div>
			{/each}
		</div>
	</div>
	{#if hasPendingMediaOrder}
		<div class="mt-3 flex flex-wrap gap-2">
			<Button
				tone="admin"
				variant="primary"
				type="button"
				disabled={mediaReordering}
				onclick={saveMediaOrder}
			>
				<i class="bi bi-floppy-fill mr-1"></i>
				{mediaReordering ? "Saving..." : "Save order"}
			</Button>
			<Button
				tone="admin"
				variant="regular"
				type="button"
				disabled={mediaReordering}
				onclick={discardMediaOrderChanges}
			>
				<i class="bi bi-x-circle mr-1"></i>
				Discard changes
			</Button>
		</div>
	{/if}
{/snippet}

{#snippet RelatedProducts(layoutMode: "split" | "stacked")}
	<div class="flex items-center justify-between">
		<AdminFieldLabel>Related products</AdminFieldLabel>
		{#if hasPendingRelatedChanges}
			<div class="flex items-center gap-2">
				<Button
					tone="admin"
					variant="regular"
					type="button"
					disabled={!canEditProduct || relatedSaving}
					onclick={discardRelatedChanges}
				>
					<i class="bi bi-x-circle mr-1"></i>
					Discard changes
				</Button>
				<Button
					tone="admin"
					variant="primary"
					type="button"
					disabled={!canEditProduct || relatedSaving}
					onclick={saveRelatedProducts}
				>
					<i class="bi bi-floppy-fill mr-1"></i>
					{relatedSaving ? "Saving..." : "Save related"}
				</Button>
			</div>
		{/if}
	</div>
	<AdminSearchForm
		fullWidth={true}
		class="mt-3 w-full"
		placeholder="Search products"
		bind:value={relatedQuery}
		onSearch={() => void searchRelatedProducts()}
		onRefresh={() => void searchRelatedProducts()}
		refreshing={relatedLoading}
		disabled={!canEditProduct || relatedLoading}
	/>

	{#if relatedLoading && relatedOptions.length === 0 && relatedLastSearchedQuery !== ""}
		<AdminEmptyState class="mt-3">Searching products...</AdminEmptyState>
	{:else if relatedOptions.length}
		<div class={relatedResultsClass(layoutMode)}>
			{#each relatedOptions as option (option.id)}
				<div class={relatedResultItemClass(layoutMode)}>
					<div class="min-w-0">
						<p class="truncate font-semibold text-stone-950 dark:text-stone-50">{option.name}</p>
						<p class="text-xs text-stone-500 dark:text-stone-400">SKU {option.sku}</p>
					</div>
					<IconButton
						tone="admin"
						variant="primary"
						type="button"
						onclick={() => addRelatedProduct(option)}
						aria-label={`Add ${option.name} as related product`}
						title="Add related product"
					>
						<i class="bi bi-plus-lg"></i>
					</IconButton>
				</div>
			{/each}
		</div>
	{:else if !relatedLoading && relatedLastSearchedQuery !== "" && relatedLastSearchedQuery === relatedQuery.trim()}
		<AdminEmptyState class="mt-3">Your search didn&apos;t match any products.</AdminEmptyState>
	{/if}

	{#if relatedSelected.length}
		<div class={relatedSelectedListClass(layoutMode)}>
			{#each relatedSelected as related (related.id)}
				<div class={relatedSelectedItemClass(layoutMode)}>
					<div>
						<p class="font-semibold text-gray-900 dark:text-gray-100">{related.name}</p>
						<p class="text-xs text-gray-500 dark:text-gray-400">SKU {related.sku}</p>
					</div>
					<IconButton
						tone="admin"
						variant="danger"
						type="button"
						onclick={() => removeRelatedProduct(related.id)}
						aria-label={`Remove ${related.name} from related products`}
						title="Remove related product"
					>
						<i class="bi bi-dash-lg"></i>
					</IconButton>
				</div>
			{/each}
		</div>
	{:else}
		<p class="mt-4 text-xs text-gray-500 dark:text-gray-400">No related products selected.</p>
	{/if}

	{#if showMessages}
		{#if relatedErrorMessage}
			{@render DismissibleAlert("related", "error", relatedErrorMessage)}
		{/if}
		{#if relatedStatusMessage}
			{@render DismissibleAlert("related", "success", relatedStatusMessage)}
		{/if}
	{/if}
{/snippet}

{#if loading && !hasProduct}
	<AdminSurface as="div" class="mt-6">
		<p class="text-sm text-gray-500 dark:text-gray-400">Loading product details...</p>
	</AdminSurface>
{:else if !allowCreate && !hasProduct}
	<p class="mt-6 text-sm text-gray-500 dark:text-gray-400">Product not found.</p>
{:else if layout === "split"}
	<div class="mt-6 space-y-6">
		<AdminSurface as="div">
			<div class="grid gap-4 text-sm sm:grid-cols-2">
				{@render BasicInfoSection()}
			</div>

			<div class={`${sectionDividerTopClass} mt-6`}>
				{@render VariantSummarySection()}
			</div>

			{@render ProductStateChips()}

			<div class="mt-6 flex flex-wrap items-center gap-2">
				{@render ProductActionButtons("split")}
			</div>
			{#if showMessages}
				{#if productErrorMessage}
					{@render DismissibleAlert("product", "error", productErrorMessage)}
				{/if}
				{#if productStatusMessage}
					{@render DismissibleAlert("product", "success", productStatusMessage)}
				{/if}
			{/if}
		</AdminSurface>

		<div class="columns-1 gap-6 md:columns-2 2xl:columns-3">
			<div class="mb-6 break-inside-avoid">
				{@render OptionsSection("split")}
			</div>
			<div class="mb-6 break-inside-avoid">
				{@render VariantsSection("split")}
			</div>
			<div class="mb-6 break-inside-avoid">
				{@render AttributesSection("split")}
			</div>
			<div class="mb-6 break-inside-avoid">
				{@render SEOSection("split")}
			</div>
			<div class="mb-6 break-inside-avoid">
				<AdminSurface as="div">
					<AdminFieldLabel>Images</AdminFieldLabel>
					{#if mediaOrderView.length}
						<div class="mt-4">
							{@render MediaGrid("split")}
						</div>
					{:else}
						<p class="mt-4 text-sm text-gray-500 dark:text-gray-400">No images yet.</p>
					{/if}

					<div class="mt-6">
						{@render MediaUpload(false, "split")}
					</div>
					{#if showMessages}
						{#if mediaErrorMessage}
							{@render DismissibleAlert("media", "error", mediaErrorMessage)}
						{/if}
						{#if mediaStatusMessage}
							{@render DismissibleAlert("media", "success", mediaStatusMessage)}
						{/if}
					{/if}
				</AdminSurface>
			</div>
			<div class="mb-6 break-inside-avoid">
				<AdminSurface as="div">
					{@render RelatedProducts("split")}
				</AdminSurface>
			</div>
		</div>
	</div>
{:else}
	<AdminSurface as="div">
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

		<div class="mt-4 space-y-6 text-sm">
			<div class="grid gap-4 sm:grid-cols-2">
				{@render BasicInfoSection()}
			</div>
			<div class={sectionDividerTopClass}>
				{@render VariantSummarySection()}
			</div>
			{@render ProductStateChips()}
			<div
				class={`${sectionDividerBottomClass} mt-2 mb-6 grid grid-cols-1 gap-2 text-base sm:grid-cols-2`}
			>
				{@render ProductActionButtons("stacked")}
			</div>
			{#if showMessages}
				{#if productErrorMessage}
					{@render DismissibleAlert("product", "error", productErrorMessage, "mb-4")}
				{/if}
				{#if productStatusMessage}
					{@render DismissibleAlert("product", "success", productStatusMessage, "mb-4")}
				{/if}
			{/if}
			{@render MediaUpload(true, "stacked")}
			{#if showMessages}
				{#if mediaErrorMessage}
					{@render DismissibleAlert("media", "error", mediaErrorMessage)}
				{/if}
				{#if mediaStatusMessage}
					{@render DismissibleAlert("media", "success", mediaStatusMessage)}
				{/if}
			{/if}
		</div>

		<div class={`${sectionDividerTopClass} mt-6`}>
			{@render OptionsSection("stacked")}
		</div>

		<div class={`${sectionDividerTopClass} mt-6`}>
			{@render VariantsSection("stacked")}
		</div>

		<div class={`${sectionDividerTopClass} mt-6`}>
			{@render AttributesSection("stacked")}
		</div>

		<div class={`${sectionDividerTopClass} mt-6`}>
			{@render SEOSection("stacked")}
		</div>

		{#if mediaOrderView.length}
			<div class={`${sectionDividerTopClass} mt-6`}>
				<AdminFieldLabel>Images</AdminFieldLabel>
				{@render MediaGrid("stacked")}
			</div>
		{/if}

		<div class={`${sectionDividerTopClass} mt-6`}>
			{@render RelatedProducts("stacked")}
		</div>
	</AdminSurface>
{/if}
