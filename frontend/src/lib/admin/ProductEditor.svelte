<script lang="ts">
	import { resolve } from "$app/paths";
	import { DRAFT_PREVIEW_SYNC_EVENT, DRAFT_PREVIEW_SYNC_STORAGE_KEY, type API } from "$lib/api";
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminFieldLabel from "$lib/admin/AdminFieldLabel.svelte";

	import AdminSurface from "$lib/admin/AdminSurface.svelte";
	import {
		adminDividerTopClass,
		adminListItemBaseClass,
		adminSurfaceVariantClasses,
	} from "$lib/admin/tokens";
	import AdminSearchForm from "$lib/admin/AdminSearchForm.svelte";
	import Alert from "$lib/components/Alert.svelte";

	import Button from "$lib/components/Button.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import IconButton from "$lib/components/IconButton.svelte";
	import NumberInput from "$lib/components/NumberInput.svelte";
	import TextArea from "$lib/components/TextArea.svelte";
	import TextInput from "$lib/components/TextInput.svelte";

	import {
		type BrandModel,
		type CategoryModel,
		type ProductAttributeDefinitionModel,
		type ProductModel,
		type RelatedProductModel,
	} from "$lib/models";
	import { uploadMediaFiles } from "$lib/media";
	import IdentitySection from "./product-editor/IdentitySection.svelte";
	import MediaSection from "./product-editor/MediaSection.svelte";
	import OptionsSection from "./product-editor/OptionsSection.svelte";
	import OrganizationSection from "./product-editor/OrganizationSection.svelte";
	import PublicationSection from "./product-editor/PublicationSection.svelte";
	import SeoSection from "./product-editor/SeoSection.svelte";
	import VariantsSection from "./product-editor/VariantsSection.svelte";
	import { extractMediaId, moveItem } from "./product-editor/media";
	import { buildProductPayload } from "./product-editor/payload";
	import { mapMediaUploadProblem, mapProductEditorProblem } from "./product-editor/problems";
	import {
		asTrimmedString,
		buildProductSnapshot as serializeProductSnapshot,
		createAttributeValue,
		createEditorKeyFactory,
		createOption,
		createOptionValue,
		createVariant,
		editorValuesFromProduct,
		emptyEditorValues,
		normalizedNumber,
	} from "./product-editor/state";
	import type {
		EditorAttributeValue,
		EditorOption,
		EditorVariant,
		ProductAttributeDefinitionInput,
		ProductEditorValues,
	} from "./product-editor/types";
	import { validateProductPayload } from "./product-editor/validation";
	import { generateVariants } from "./product-editor/variants";
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
	const nextEditorKey = createEditorKeyFactory();

	const splitEditorSectionClass = adminSurfaceVariantClasses["panel-tight"];
	const nestedEditorSectionClass = adminSurfaceVariantClasses.subsurface;

	const mutedPanelClass = adminSurfaceVariantClasses.muted;

	const sectionDividerTopClass = adminDividerTopClass;

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

	type MessageScope = "product" | "media" | "related";
	type MessageTone = "error" | "success";

	function editorValues(): ProductEditorValues {
		return {
			sku,
			name,
			subtitle,
			description,
			selectedBrandId,
			selectedCategoryIds,
			seoTitle,
			seoDescription,
			seoCanonicalPath,
			seoOgImageMediaId,
			seoNoIndex,
			options,
			variants,
			attributeValues,
			defaultVariantSku,
			relatedSelected,
		};
	}

	function buildProductSnapshot(): string {
		return serializeProductSnapshot(resolvedProductId, editorValues());
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

	function applyEditorValues(values: ProductEditorValues) {
		({
			sku,
			name,
			subtitle,
			description,
			selectedBrandId,
			selectedCategoryIds,
			seoTitle,
			seoDescription,
			seoCanonicalPath,
			seoOgImageMediaId,
			seoNoIndex,
			options,
			variants,
			attributeValues,
			defaultVariantSku,
			relatedSelected,
		} = values);
	}

	function resetForm() {
		applyEditorValues(emptyEditorValues(nextEditorKey));
		mediaFiles = null;
		pendingMediaOrder = null;
		relatedQuery = "";
		relatedOptions = [];
		relatedLastSearchedQuery = "";
		captureSavedSnapshot();
	}

	function hydrateForm(value: ProductModel) {
		applyEditorValues(editorValuesFromProduct(value, nextEditorKey));
		pendingMediaOrder = null;
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
			attributeDefinitionErrorMessage = mapProductEditorProblem(
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
			attributeDefinitionErrorMessage = mapProductEditorProblem(
				err,
				"Unable to delete attribute definition."
			);
		} finally {
			attributeDefinitionDeletingId = null;
		}
	}

	function addAttributeValue() {
		attributeValues = [
			...attributeValues,
			createAttributeValue(nextEditorKey, attributeValues.length + 1),
		];
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
		options = [...options, createOption(nextEditorKey, options.length + 1)];
	}

	function removeOption(optionKey: string) {
		options = options.filter((option) => option.key !== optionKey);
	}

	function addOptionValue(optionKey: string) {
		options = options.map((option) =>
			option.key === optionKey
				? {
						...option,
						values: [
							...option.values,
							createOptionValue(nextEditorKey, "", option.values.length + 1),
						],
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
								: [createOptionValue(nextEditorKey)],
					}
				: option
		);
	}

	function addManualVariant() {
		const seed = variants.find((variant) => variant.sku === defaultVariantSku) ?? variants[0];
		const nextVariants = [
			...variants,
			createVariant(nextEditorKey, {
				sku: `${asTrimmedString(sku)}-${variants.length + 1}`.replace(/^-/, ""),
				title: `Variant ${variants.length + 1}`,
				price: seed?.price ?? "",
				compare_at_price: seed?.compare_at_price ?? "",
				stock: seed?.stock ?? "0",
			}),
		];
		variants = nextVariants;
		if (!defaultVariantSku) {
			defaultVariantSku = nextVariants[0]?.sku ?? "";
		}
	}

	function removeVariant(variantKey: string) {
		const remaining = variants.filter((variant) => variant.key !== variantKey);
		variants = remaining.length > 0 ? remaining : [createVariant(nextEditorKey)];
		if (!variants.some((variant) => variant.sku === defaultVariantSku)) {
			defaultVariantSku = variants[0]?.sku ?? "";
		}
	}

	function generateVariantsFromOptions() {
		variants = generateVariants(options, variants, sku, name, defaultVariantSku, nextEditorKey);
		if (!variants.some((variant) => variant.sku === defaultVariantSku)) {
			defaultVariantSku = variants[0]?.sku ?? "";
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
			const payload = buildProductPayload(editorValues(), product?.images ?? []);
			const validationError = validateProductPayload(
				payload,
				attributeValues,
				attributeDefinitions
			);
			if (validationError) {
				setMessage("product", "error", validationError);
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
			setMessage(
				"product",
				"error",
				mapProductEditorProblem(err, "Unable to publish product draft.")
			);
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
			setMessage(
				"product",
				"error",
				mapProductEditorProblem(err, "Unable to discard product draft.")
			);
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
			setMessage("product", "error", mapProductEditorProblem(err, "Unable to unpublish product."));
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
				mapProductEditorProblem(err, "Unable to open product draft preview.")
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
			setMessage("media", "error", mapMediaUploadProblem(err));
		} finally {
			uploading = false;
		}
	}

	async function detachMedia(mediaUrl: string) {
		if (!resolvedProductId) {
			return;
		}
		const mediaId = extractMediaId(
			mediaUrl,
			typeof window === "undefined" ? "http://localhost" : window.location.origin
		);
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

		pendingMediaOrder = moveItem(mediaOrderView, index, direction);
	}

	function discardMediaOrderChanges() {
		pendingMediaOrder = null;
	}

	async function saveMediaOrder() {
		if (!resolvedProductId || !hasPendingMediaOrder || !pendingMediaOrder) {
			return;
		}

		const mediaIds = pendingMediaOrder
			.map((url) =>
				extractMediaId(
					url,
					typeof window === "undefined" ? "http://localhost" : window.location.origin
				)
			)
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
	<IdentitySection bind:name bind:subtitle bind:sku bind:description />
	<OrganizationSection {brands} {categories} bind:selectedBrandId bind:selectedCategoryIds />
{/snippet}

{#snippet OptionsEditorSection(layoutMode: "split" | "stacked")}
	<OptionsSection
		layout={layoutMode}
		bind:options
		onAddOption={addOption}
		onRemoveOption={removeOption}
		onAddValue={addOptionValue}
		onRemoveValue={removeOptionValue}
		onGenerate={generateVariantsFromOptions}
	/>
{/snippet}

{#snippet VariantsEditorSection(layoutMode: "split" | "stacked")}
	<VariantsSection
		layout={layoutMode}
		bind:variants
		bind:defaultVariantSku
		onAdd={addManualVariant}
		onRemove={removeVariant}
	/>
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

{#snippet SEOEditorSection(layoutMode: "split" | "stacked")}
	<SeoSection
		layout={layoutMode}
		bind:title={seoTitle}
		bind:description={seoDescription}
		bind:canonicalPath={seoCanonicalPath}
		bind:ogImageMediaId={seoOgImageMediaId}
		bind:noIndex={seoNoIndex}
	/>
{/snippet}

{#snippet PublicationEditorSection(layoutMode: "split" | "stacked")}
	<PublicationSection
		layout={layoutMode}
		canEdit={canEditProduct}
		defaultVariantSku={defaultVariantSku || variants[0]?.sku || ""}
		priceRange={editorPriceRangePreview}
		{isPublished}
		{hasDraftChanges}
		{hasUnsavedChanges}
		{saving}
		{publishing}
		{unpublishing}
		discarding={discardingDraft}
		previewing={previewingDraft}
		{previewActive}
		{deleting}
		errorMessage={showMessages ? productErrorMessage : ""}
		statusMessage={showMessages ? productStatusMessage : ""}
		onClearError={() => clearMessage("product", "error")}
		onClearStatus={() => clearMessage("product", "success")}
		onSave={() => void saveProduct()}
		onPreview={() => void previewDraft()}
		onPublish={() => void publishProduct()}
		onUnpublish={() => void unpublishProduct()}
		onDiscard={() => void discardDraft()}
		onDelete={() => void deleteProduct()}
	/>
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

{#snippet MediaEditorSection(
	layoutMode: "split" | "stacked",
	showHint = false,
	showImages = true,
	showUpload = true,
	messages = showMessages
)}
	<MediaSection
		layout={layoutMode}
		images={mediaOrderView}
		productName={product?.name ?? name}
		canEdit={canEditProduct}
		bind:files={mediaFiles}
		{uploading}
		deletingMediaId={mediaDeleting}
		reordering={mediaReordering}
		hasPendingOrder={hasPendingMediaOrder}
		showMessages={messages}
		errorMessage={mediaErrorMessage}
		statusMessage={mediaStatusMessage}
		showUploadHint={showHint}
		{showImages}
		{showUpload}
		onUpload={() => void uploadMedia()}
		onDetach={(url) => void detachMedia(url)}
		onMove={moveMedia}
		onSaveOrder={() => void saveMediaOrder()}
		onDiscardOrder={discardMediaOrderChanges}
		onClearError={() => clearMessage("media", "error")}
		onClearStatus={() => clearMessage("media", "success")}
	/>
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
				{@render PublicationEditorSection("split")}
			</div>
		</AdminSurface>

		<div class="columns-1 gap-6 md:columns-2 2xl:columns-3">
			<div class="mb-6 break-inside-avoid">
				{@render OptionsEditorSection("split")}
			</div>
			<div class="mb-6 break-inside-avoid">
				{@render VariantsEditorSection("split")}
			</div>
			<div class="mb-6 break-inside-avoid">
				{@render AttributesSection("split")}
			</div>
			<div class="mb-6 break-inside-avoid">
				{@render SEOEditorSection("split")}
			</div>
			<div class="mb-6 break-inside-avoid">
				<AdminSurface as="div">
					<AdminFieldLabel>Images</AdminFieldLabel>
					{#if mediaOrderView.length === 0}
						<p class="mt-4 text-sm text-gray-500 dark:text-gray-400">No images yet.</p>
					{/if}
					<div class="mt-4">{@render MediaEditorSection("split")}</div>
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
				{@render PublicationEditorSection("stacked")}
			</div>
			{@render MediaEditorSection("stacked", true, false, true)}
		</div>

		<div class={`${sectionDividerTopClass} mt-6`}>
			{@render OptionsEditorSection("stacked")}
		</div>

		<div class={`${sectionDividerTopClass} mt-6`}>
			{@render VariantsEditorSection("stacked")}
		</div>

		<div class={`${sectionDividerTopClass} mt-6`}>
			{@render AttributesSection("stacked")}
		</div>

		<div class={`${sectionDividerTopClass} mt-6`}>
			{@render SEOEditorSection("stacked")}
		</div>

		{#if mediaOrderView.length}
			<div class={`${sectionDividerTopClass} mt-6`}>
				<AdminFieldLabel>Images</AdminFieldLabel>
				{@render MediaEditorSection("stacked", false, true, false, false)}
			</div>
		{/if}

		<div class={`${sectionDividerTopClass} mt-6`}>
			{@render RelatedProducts("stacked")}
		</div>
	</AdminSurface>
{/if}
