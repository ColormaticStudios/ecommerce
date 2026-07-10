<script lang="ts">
	import { getContext, onMount } from "svelte";
	import { DRAFT_PREVIEW_SYNC_EVENT, DRAFT_PREVIEW_SYNC_STORAGE_KEY, type API } from "$lib/api";
	import type { components } from "$lib/api/generated/openapi";
	import { cmsMediaURL, type CmsContentBlock } from "$lib/cms";
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminConfirmDialog from "$lib/admin/AdminConfirmDialog.svelte";
	import AdminFloatingNotices from "$lib/admin/AdminFloatingNotices.svelte";
	import CmsGlobalRegionEditor from "$lib/admin/CmsGlobalRegionEditor.svelte";
	import AdminListItem from "$lib/admin/AdminListItem.svelte";
	import AdminMasterDetailLayout from "$lib/admin/AdminMasterDetailLayout.svelte";
	import CmsVisualEditor from "$lib/admin/CmsVisualEditor.svelte";
	import AdminPageHeader from "$lib/admin/AdminPageHeader.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import AdminSurface from "$lib/admin/AdminSurface.svelte";
	import { createAdminNotices, createAdminSavePrompt } from "$lib/admin/state.svelte";
	import { formatCmsAdminError } from "$lib/admin/cms-errors";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import IconButton from "$lib/components/IconButton.svelte";
	import NumberInput from "$lib/components/NumberInput.svelte";
	import TabSwitcher, { type TabSwitcherItem } from "$lib/components/TabSwitcher.svelte";
	import TextArea from "$lib/components/TextArea.svelte";
	import TextInput from "$lib/components/TextInput.svelte";

	type CmsPageResponse = components["schemas"]["CmsPageResponse"];
	type CmsNavigationResponse = components["schemas"]["CmsNavigationResponse"];
	type CmsGlobalRegionResponse = components["schemas"]["CmsGlobalRegionResponse"];
	type CmsPagePayload = components["schemas"]["CmsPagePayload"];
	type CmsPreviewBlock = components["schemas"]["CmsPreviewBlock"];
	type CmsEntryVersion = components["schemas"]["CmsEntryVersion"];
	type CmsPublication = components["schemas"]["CmsPublication"];
	type CmsPageDeliveryResponse = components["schemas"]["CmsPageDeliveryResponse"];
	type CmsTargetingRuleInput = components["schemas"]["CmsTargetingRuleInput"];
	type CmsNavigationItemInput = components["schemas"]["CmsNavigationItemInput"];
	type CmsTab = "pages" | "navigation" | "global" | "redirects" | "operations";
	type CmsRedirectRule = components["schemas"]["CmsRedirectRule"];
	type CmsSEOResponse = components["schemas"]["CmsSEOResponse"];
	type CmsLocale = components["schemas"]["CmsLocale"];
	type CmsPageVariant = components["schemas"]["CmsPageVariant"];
	type CmsAuditEvent = components["schemas"]["CmsAuditEvent"];
	type CmsContentExport = components["schemas"]["CmsContentExport"];
	type CmsEntry = components["schemas"]["CmsEntry"];
	type CmsGovernance = components["schemas"]["CmsGovernance"];
	type CmsOperations = components["schemas"]["CmsOperations"];
	type SelectedEntity = { kind: "page"; id: number | null } | { kind: "global"; id: number | null };
	type NavPlacement = "top" | "hidden" | string;
	type GlobalRegionType = "announcement_bar" | "sitewide_banner" | "trust_strip" | "footer";
	type EditableBlock = CmsContentBlock & { editorId: string };
	type NavigationDropdown = {
		id: string;
		sourceId: number | null;
		label: string;
		sortOrder: number;
	};
	type NavigationCustomItem = {
		id: string;
		sourceId: number | null;
		label: string;
		itemType: "internal" | "external";
		targetRef: string;
		url: string;
		placement: NavPlacement;
		sortOrder: number;
		isEnabled: boolean;
	};
	type NavigationDraftRow = {
		pageId: number;
		path: string;
		title: string;
		label: string;
		placement: NavPlacement;
		sortOrder: number;
		isEnabled: boolean;
	};
	type SelectedNavigationItem =
		| { kind: "settings" }
		| { kind: "dropdown"; id: string }
		| { kind: "custom"; id: string }
		| { kind: "page"; id: number };
	type DeleteTarget =
		| { kind: "page"; id: number; label: string }
		| { kind: "navigation"; id: number; label: string }
		| { kind: "global"; id: number; label: string }
		| { kind: "redirect"; id: number; label: string }
		| { kind: "navigation_dropdown"; id: string; label: string }
		| { kind: "navigation_custom"; id: string; label: string }
		| { kind: "navigation_page"; id: number; label: string };
	type CampaignTemplateKey =
		| "seasonal_sale"
		| "collection_launch"
		| "bundle_spotlight"
		| "new_arrivals";
	type DeliveryRuleDraft = {
		id: string;
		enabled: boolean;
		markets: string;
		devices: Array<"desktop" | "mobile" | "tablet">;
		audience: "all" | "guest" | "authenticated";
		referrers: string;
		utmSources: string;
		segments: string;
	};

	const api: API = getContext("api");
	const notices = createAdminNotices();
	const savePrompt = createAdminSavePrompt({
		navigationMessage: "You have unsaved CMS changes. Leave this section and discard them?",
	});

	const cmsTabs: TabSwitcherItem[] = [
		{ id: "pages", label: "Pages", icon: "bi-file-earmark-text" },
		{ id: "navigation", label: "Navigation", icon: "bi-list-nested" },
		{ id: "global", label: "Global", icon: "bi-layout-text-sidebar-reverse" },
		{ id: "redirects", label: "Redirects", icon: "bi-signpost-split" },
		{ id: "operations", label: "Operations", icon: "bi-activity" },
	];
	const globalRegionOptions: Array<{ id: GlobalRegionType; label: string }> = [
		{ id: "announcement_bar", label: "Announcement bar" },
		{ id: "sitewide_banner", label: "Sitewide banner" },
		{ id: "trust_strip", label: "Trust strip" },
		{ id: "footer", label: "Footer" },
	];
	const campaignTemplates: Array<{ id: CampaignTemplateKey; label: string }> = [
		{ id: "seasonal_sale", label: "Seasonal sale" },
		{ id: "collection_launch", label: "Collection launch" },
		{ id: "bundle_spotlight", label: "Bundle spotlight" },
		{ id: "new_arrivals", label: "New arrivals" },
	];
	const pageSectionOptions: Array<{ id: CmsContentBlock["type"]; label: string }> = [
		{ id: "hero", label: "Hero" },
		{ id: "rich_text", label: "Text" },
		{ id: "image", label: "Image" },
		{ id: "cta", label: "Call to action" },
		{ id: "promo_banner", label: "Promotion banner" },
		{ id: "product_rail", label: "Products" },
		{ id: "category_tiles", label: "Categories" },
		{ id: "promotion_highlight", label: "Promotion highlight" },
		{ id: "inventory_message", label: "Inventory message" },
		{ id: "testimonial", label: "Review" },
		{ id: "social_embed", label: "Social embed" },
	];

	let activeTab = $state<CmsTab>("pages");
	let pages = $state<CmsPageResponse[]>([]);
	let navigationMenus = $state<CmsNavigationResponse[]>([]);
	let globalRegions = $state<CmsGlobalRegionResponse[]>([]);
	let selected = $state<SelectedEntity>({ kind: "page", id: null });
	let loading = $state(false);
	let saving = $state(false);
	let publishing = $state(false);
	let unpublishing = $state(false);
	let discardingDraft = $state(false);
	let previewingDraft = $state(false);
	let previewActive = $state(false);
	let loadError = $state("");
	let savedSnapshot = $state("");
	let editMode = $state(false);
	let previewBlocks = $state<CmsPreviewBlock[]>([]);
	let previewLoading = $state(false);
	let previewError = $state("");
	let editorIDSequence = 0;

	let pagePath = $state("");
	let pageSlug = $state("");
	let pageTitle = $state("");
	let pageTemplate = $state("default");
	let pageVisibility = $state<"public" | "hidden">("public");
	let pageBlocks = $state<EditableBlock[]>(defaultPageBlocks());
	let selectedPageSectionType = $state<CmsContentBlock["type"]>("hero");
	let uploadingBlockMedia = $state("");
	let localBlockMediaPreviews = $state<Record<string, string>>({});
	let deliveryLoading = $state(false);
	let deliverySaving = $state(false);
	let scheduleEnabled = $state(false);
	let schedulePublishAt = $state("");
	let scheduleExpiryEnabled = $state(false);
	let scheduleUnpublishAt = $state("");
	let scheduleTimezone = $state(Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC");
	let scheduleStatus = $state<"pending" | "active" | "completed" | "cancelled" | null>(null);
	let scheduleLastTransitionAt = $state<string | null>(null);
	let deliveryPublications = $state<CmsPublication[]>([]);
	let deliveryRules = $state<DeliveryRuleDraft[]>([]);
	let experimentEnabled = $state(false);
	let experimentName = $state("");
	let experimentStatus = $state<"draft" | "active" | "paused" | "completed">("draft");
	let experimentStickyKey = $state<"visitor" | "customer">("visitor");
	let experimentStartsAt = $state("");
	let experimentEndsAt = $state("");
	let controlVersionId = $state<number | null>(null);
	let variantVersionId = $state<number | null>(null);
	let controlAllocation = $state(50);

	let navigationKey = $state("");
	let navigationTitle = $state("");
	let navigationLocation = $state("");
	let navigationDropdowns = $state<NavigationDropdown[]>([]);
	let navigationCustomItems = $state<NavigationCustomItem[]>([]);
	let navigationRows = $state<NavigationDraftRow[]>([]);
	let selectedNavigationItem = $state<SelectedNavigationItem>({ kind: "settings" });

	let globalKey = $state("");
	let globalTitle = $state("");
	let globalRegion = $state<GlobalRegionType>("announcement_bar");
	let globalBlocks = $state<EditableBlock[]>(defaultGlobalBlocks("announcement_bar"));
	let redirects = $state<CmsRedirectRule[]>([]);
	let selectedRedirectId = $state<number | null>(null);
	let redirectSource = $state("");
	let redirectTarget = $state("");
	let redirectMatchType = $state<"exact" | "prefix">("exact");
	let redirectType = $state<301 | 302>(301);
	let redirectPriority = $state(0);
	let redirectEnabled = $state(true);
	let seoLoading = $state(false);
	let seoSaving = $state(false);
	let seoTitle = $state("");
	let seoDescription = $state("");
	let seoCanonicalURL = $state("");
	let seoRobots = $state<"index_follow" | "noindex_follow" | "index_nofollow" | "noindex_nofollow">(
		"index_follow"
	);
	let seoOGTitle = $state("");
	let seoOGDescription = $state("");
	let seoOGImageMediaID = $state("");
	let seoTwitterCard = $state<"summary" | "summary_large_image">("summary_large_image");
	let seoTwitterTitle = $state("");
	let seoTwitterDescription = $state("");
	let seoTwitterImageMediaID = $state("");
	let seoJSONLDType = $state<
		"" | "WebPage" | "FAQPage" | "BreadcrumbList" | "Organization" | "WebSite" | "Product"
	>("");
	let seoJSONLDName = $state("");
	let seoIssues = $state<string[]>([]);
	let cmsLocales = $state<CmsLocale[]>([]);
	let localeSaving = $state(false);
	let pageVariants = $state<CmsPageVariant[]>([]);
	let selectedVariantId = $state<number | null>(null);
	let variantLocale = $state("");
	let variantMarket = $state("");
	let variantPath = $state("");
	let variantSlug = $state("");
	let variantTitle = $state("");
	let variantBlocks = $state<EditableBlock[]>([]);
	let variantSaving = $state(false);
	let variantComment = $state("");
	let cmsAuditEvents = $state<CmsAuditEvent[]>([]);
	let restoreInput: HTMLInputElement;
	let pendingRestore = $state<CmsContentExport | null>(null);
	let pendingRestoreName = $state("");
	let restoreDialogOpen = $state(false);
	let restorePreviewSummary = $state("");
	let restoring = $state(false);
	let deleteTarget = $state<DeleteTarget | null>(null);
	let deleting = $state(false);
	let governance = $state<CmsGovernance>({
		approval_required: true,
		invalidation_webhook_url: "",
		roles: [],
	});
	let governanceSaving = $state(false);
	let operations = $state<CmsOperations | null>(null);
	let operationsLoading = $state(false);

	const selectedPage = $derived(
		selected.kind === "page" && selected.id !== null
			? pages.find((page) => page.page.id === selected.id)
			: null
	);
	const selectedGlobal = $derived(
		selected.kind === "global" && selected.id !== null
			? globalRegions.find((region) => region.region.id === selected.id)
			: null
	);
	const selectedNavigation = $derived(navigationMenus[0] ?? null);
	const selectedNavigationDropdown = $derived.by(() => {
		if (selectedNavigationItem.kind !== "dropdown") return null;
		const selectedId = selectedNavigationItem.id;
		return navigationDropdowns.find((dropdown) => dropdown.id === selectedId) ?? null;
	});
	const selectedNavigationCustomItem = $derived.by(() => {
		if (selectedNavigationItem.kind !== "custom") return null;
		const selectedId = selectedNavigationItem.id;
		return navigationCustomItems.find((item) => item.id === selectedId) ?? null;
	});
	const selectedNavigationPage = $derived.by(() => {
		if (selectedNavigationItem.kind !== "page") return null;
		const selectedId = selectedNavigationItem.id;
		return navigationRows.find((row) => row.pageId === selectedId) ?? null;
	});
	const selectedPageVariant = $derived(
		pageVariants.find((variant) => variant.id === selectedVariantId) ?? null
	);
	const currentSnapshot = $derived(captureSnapshot());
	const hasUnsavedChanges = $derived(!loading && currentSnapshot !== savedSnapshot);
	const isCreatingDraft = $derived(
		activeTab === "navigation"
			? selectedNavigation === null
			: (activeTab === "pages" || activeTab === "global") && selected.id === null
	);
	const currentEntry = $derived(
		activeTab === "navigation"
			? selectedNavigation?.entry
			: selected.kind === "page"
				? selectedPage?.entry
				: selectedGlobal?.entry
	);
	const isPublished = $derived(Boolean(currentEntry?.published_version_id));
	const hasDraftChanges = $derived(
		activeTab === "navigation"
			? (selectedNavigation?.has_unpublished_draft ?? navigationRows.length > 0)
			: selected.kind === "page"
				? (selectedPage?.has_unpublished_draft ?? selected.id === null)
				: (selectedGlobal?.has_unpublished_draft ?? selected.id === null)
	);
	const canPublish = $derived(
		!publishing &&
			!hasUnsavedChanges &&
			hasDraftChanges &&
			(activeTab === "navigation"
				? Boolean(selectedNavigation)
				: selected.kind === "page"
					? selected.id !== null
					: selected.id !== null)
	);
	const canPreviewDraft = $derived(
		!previewingDraft &&
			(activeTab === "navigation"
				? Boolean(selectedNavigation)
				: selected.kind === "page"
					? selected.id !== null
					: selected.kind === "global" && selected.id !== null)
	);
	const canUnpublish = $derived(
		!unpublishing &&
			isPublished &&
			(activeTab === "navigation"
				? Boolean(selectedNavigation)
				: selected.kind === "page"
					? selected.id !== null
					: selected.kind === "global" && selected.id !== null)
	);
	const canDiscardDraft = $derived(
		!discardingDraft &&
			hasDraftChanges &&
			(activeTab === "navigation"
				? Boolean(selectedNavigation)
				: selected.kind === "page"
					? selected.id !== null
					: selected.kind === "global" && selected.id !== null)
	);
	const pageVersionOptions = $derived.by(() => {
		const versions = [selectedPage?.current_version, selectedPage?.published_version].filter(
			(version): version is CmsEntryVersion => Boolean(version)
		);
		return versions.filter(
			(version, index) => versions.findIndex((candidate) => candidate.id === version.id) === index
		);
	});
	const deliverySaveDisabled = $derived(
		deliverySaving ||
			selected.id === null ||
			(scheduleEnabled && !schedulePublishAt) ||
			(experimentEnabled &&
				(!experimentName.trim() ||
					!experimentStartsAt ||
					!controlVersionId ||
					!variantVersionId ||
					controlAllocation < 1 ||
					controlAllocation > 99))
	);

	function editorId(prefix: string): string {
		editorIDSequence += 1;
		return `${prefix}-${editorIDSequence}`;
	}

	function entryIsPublished(record: { entry: CmsEntry }) {
		return Boolean(record.entry.published_version_id);
	}

	function defaultPageBlocks(): EditableBlock[] {
		return [
			{
				editorId: editorId("hero"),
				type: "hero",
				title: "Shipping",
				subtitle: "Useful storefront page copy.",
			},
			{ editorId: editorId("rich"), type: "rich_text", body: "Add page content here." },
		];
	}

	function defaultGlobalBlocks(region: GlobalRegionType): EditableBlock[] {
		if (region === "footer") {
			return [
				{
					editorId: editorId("footer"),
					type: "footer",
					brand_name: "Store",
					tagline: "Thoughtfully selected products for everyday use.",
					columns: [
						{
							title: "Shop",
							links: [
								{ label: "All products", url: "/search" },
								{ label: "New arrivals", url: "/search?sort=created_at" },
							],
						},
						{
							title: "Help",
							links: [
								{ label: "Shipping", url: "/shipping" },
								{ label: "Returns", url: "/returns" },
							],
						},
					],
					social_links: [],
					copyright: `© ${new Date().getFullYear()} Store`,
					layout: "columns",
				},
			];
		}
		if (region === "trust_strip") {
			return [
				{ editorId: editorId("trust"), type: "rich_text", body: "Free shipping over $100" },
				{ editorId: editorId("trust"), type: "rich_text", body: "30-day returns" },
			];
		}
		return [
			{
				editorId: editorId("promo"),
				type: "promo_banner",
				title: "Free domestic shipping over $100",
				body: "Applied automatically.",
				link: { label: "Shop now", url: "/search" },
			},
		];
	}

	function normalizeGlobalBlocks(
		region: GlobalRegionType,
		blocks: EditableBlock[]
	): EditableBlock[] {
		if (region !== "footer" || blocks.some((block) => block.type === "footer")) return blocks;
		const defaults = defaultGlobalBlocks("footer");
		const footer = defaults[0];
		if (!footer || footer.type !== "footer") return defaults;
		const promo = blocks.find((block) => block.type === "promo_banner");
		const text = blocks.find((block) => block.type === "rich_text");
		const cta = blocks.find((block) => block.type === "cta");
		return [
			{
				...footer,
				tagline: promo?.body || text?.body || footer.tagline,
				columns:
					promo?.link || cta
						? [
								...footer.columns,
								{
									title: promo?.title || "More",
									links: [
										promo?.link ?? { label: cta?.label ?? "Learn more", url: cta?.url ?? "/" },
									],
								},
							]
						: footer.columns,
			},
		];
	}

	function campaignTemplateBlocks(template: CampaignTemplateKey): EditableBlock[] {
		switch (template) {
			case "collection_launch":
				return [
					{ editorId: editorId("hero"), type: "hero", title: "Collection launch", subtitle: "" },
					{
						editorId: editorId("category-tiles"),
						type: "category_tiles",
						title: "Explore the collection",
						subtitle: "",
						category_slugs: [],
						image_aspect: "wide",
					},
					createProductRailBlock("Featured products", "newest"),
					createBlock("testimonial"),
				];
			case "bundle_spotlight":
				return [
					{ editorId: editorId("hero"), type: "hero", title: "Bundle spotlight", subtitle: "" },
					createBlock("promotion_highlight"),
					createProductRailBlock("Bundle picks", "manual"),
					createBlock("inventory_message"),
				];
			case "new_arrivals":
				return [
					{ editorId: editorId("hero"), type: "hero", title: "New arrivals", subtitle: "" },
					createProductRailBlock("Just added", "newest"),
					createBlock("category_tiles"),
					createBlock("social_embed"),
				];
			default:
				return [
					{ editorId: editorId("hero"), type: "hero", title: "Seasonal sale", subtitle: "" },
					createBlock("promotion_highlight"),
					createProductRailBlock("Sale picks", "search", "sale"),
					createBlock("category_tiles"),
				];
		}
	}

	function createProductRailBlock(
		title: string,
		source: Extract<CmsContentBlock, { type: "product_rail" }>["source"],
		query = ""
	): EditableBlock {
		return {
			editorId: editorId("product-rail"),
			type: "product_rail",
			title,
			subtitle: "",
			source,
			product_ids: [],
			query,
			category_slug: "",
			sort: "created_at",
			order: "desc",
			limit: 8,
			image_aspect: "square",
		};
	}

	function applyCampaignTemplate(template: CampaignTemplateKey) {
		pageTemplate = template;
		pageBlocks = campaignTemplateBlocks(template);
		void refreshCmsPreview();
	}

	function blocksFromPayload(
		payload: CmsPagePayload | undefined,
		fallback: EditableBlock[]
	): EditableBlock[] {
		const blocks = Array.isArray(payload?.blocks)
			? (payload.blocks as unknown as CmsContentBlock[])
			: [];
		if (blocks.length === 0) return fallback;
		return blocks.map((block, index) => ({
			...block,
			editorId: editorId(`${block.type}-${index}`),
		}));
	}

	function payloadFromBlocks(blocks: EditableBlock[]): CmsPagePayload {
		return {
			blocks: blocks.map(stripEditorId) as unknown as CmsPagePayload["blocks"],
		};
	}

	function stripEditorId(block: EditableBlock): CmsContentBlock {
		const next = { ...block } as Partial<EditableBlock>;
		delete next.editorId;
		return next as CmsContentBlock;
	}

	async function uploadEditorBlockMedia(block: EditableBlock, event: Event, categorySlug = "") {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		const previewKey = categorySlug ? `${block.editorId}:${categorySlug}` : block.editorId;
		uploadingBlockMedia = previewKey;
		const previousPreview = localBlockMediaPreviews[previewKey];
		if (previousPreview) URL.revokeObjectURL(previousPreview);
		localBlockMediaPreviews = {
			...localBlockMediaPreviews,
			[previewKey]: URL.createObjectURL(file),
		};
		try {
			const mediaID = await api.uploadMedia(file);
			if (block.type === "hero") block.image_media_id = mediaID;
			if (block.type === "image") block.media_id = mediaID;
			if (block.type === "category_tiles" && categorySlug) {
				block.category_media_ids = { ...block.category_media_ids, [categorySlug]: mediaID };
			}
		} catch (error) {
			console.error(error);
			notices.setError("Unable to upload CMS image.");
		} finally {
			uploadingBlockMedia = "";
			input.value = "";
		}
	}

	function blockMediaPreview(block: EditableBlock, categorySlug = ""): string {
		const previewKey = categorySlug ? `${block.editorId}:${categorySlug}` : block.editorId;
		if (localBlockMediaPreviews[previewKey]) return localBlockMediaPreviews[previewKey];
		if (block.type === "hero") return cmsMediaURL(block.image_media_id);
		if (block.type === "image") return cmsMediaURL(block.media_id);
		if (block.type === "category_tiles") {
			return cmsMediaURL(block.category_media_ids?.[categorySlug]);
		}
		return "";
	}

	function pagePayloadFromResponse(page: CmsPageResponse): CmsPagePayload | undefined {
		return page.current_version?.payload ?? page.published_version?.payload;
	}

	function globalPayloadFromResponse(region: CmsGlobalRegionResponse): CmsPagePayload | undefined {
		return region.current_version?.payload ?? region.published_version?.payload;
	}

	function captureSnapshot(): string {
		return JSON.stringify({
			activeTab,
			selected,
			pagePath,
			pageSlug,
			pageTitle,
			pageTemplate,
			pageVisibility,
			pageBlocks,
			navigationKey,
			navigationTitle,
			navigationLocation,
			navigationDropdowns,
			navigationCustomItems,
			navigationRows,
			globalKey,
			globalTitle,
			globalRegion,
			globalBlocks,
			redirectSource,
			redirectTarget,
			redirectMatchType,
			redirectType,
			redirectPriority,
			redirectEnabled,
			seoTitle,
			seoDescription,
			seoCanonicalURL,
			seoRobots,
			seoOGTitle,
			seoOGDescription,
			seoOGImageMediaID,
			seoTwitterCard,
			seoTwitterTitle,
			seoTwitterDescription,
			seoTwitterImageMediaID,
			seoJSONLDType,
			seoJSONLDName,
			scheduleEnabled,
			schedulePublishAt,
			scheduleExpiryEnabled,
			scheduleUnpublishAt,
			scheduleTimezone,
			deliveryRules,
			experimentEnabled,
			experimentName,
			experimentStatus,
			experimentStickyKey,
			experimentStartsAt,
			experimentEndsAt,
			controlVersionId,
			variantVersionId,
			controlAllocation,
			selectedVariantId,
			variantLocale,
			variantMarket,
			variantPath,
			variantSlug,
			variantTitle,
			variantBlocks,
			cmsLocales,
		});
	}

	function markSaved() {
		savedSnapshot = captureSnapshot();
	}

	function resetDelivery() {
		scheduleEnabled = false;
		schedulePublishAt = "";
		scheduleExpiryEnabled = false;
		scheduleUnpublishAt = "";
		scheduleStatus = null;
		scheduleLastTransitionAt = null;
		deliveryPublications = [];
		deliveryRules = [];
		experimentEnabled = false;
		experimentName = "";
		experimentStatus = "draft";
		experimentStickyKey = "visitor";
		experimentStartsAt = "";
		experimentEndsAt = "";
		controlVersionId = null;
		variantVersionId = null;
		controlAllocation = 50;
	}

	function localDateTime(value: string | Date | null | undefined): string {
		if (!value) return "";
		const date = new Date(value);
		const offset = date.getTimezoneOffset() * 60_000;
		return new Date(date.getTime() - offset).toISOString().slice(0, 16);
	}

	function splitValues(value: string): string[] {
		return value
			.split(",")
			.map((item) => item.trim())
			.filter(Boolean);
	}

	function addDeliveryRule() {
		deliveryRules = [
			...deliveryRules,
			{
				id: editorId("audience"),
				enabled: true,
				markets: "",
				devices: [],
				audience: "all",
				referrers: "",
				utmSources: "",
				segments: "",
			},
		];
	}

	function toggleRuleDevice(rule: DeliveryRuleDraft, device: "desktop" | "mobile" | "tablet") {
		rule.devices = rule.devices.includes(device)
			? rule.devices.filter((value) => value !== device)
			: [...rule.devices, device];
	}

	async function loadPageDelivery(page: CmsPageResponse) {
		const pageId = page.page.id;
		deliveryLoading = true;
		resetDelivery();
		try {
			const response = await api.getAdminCmsPageDelivery(pageId);
			if (selected.kind !== "page" || selected.id !== pageId) return;
			hydrateDelivery(response);
			markSaved();
		} catch (error) {
			console.error(error);
			notices.setError("Unable to load page delivery settings.");
		} finally {
			if (selected.kind === "page" && selected.id === pageId) deliveryLoading = false;
		}
	}

	function hydrateDelivery(response: CmsPageDeliveryResponse) {
		scheduleEnabled = Boolean(response.schedule);
		schedulePublishAt = localDateTime(response.schedule?.publish_at);
		scheduleExpiryEnabled = Boolean(response.schedule?.unpublish_at);
		scheduleUnpublishAt = localDateTime(response.schedule?.unpublish_at);
		scheduleTimezone = response.schedule?.timezone || scheduleTimezone;
		scheduleStatus = response.schedule?.status ?? null;
		scheduleLastTransitionAt = response.schedule?.last_transition_at ?? null;
		deliveryPublications = response.recent_publications;
		deliveryRules = response.targeting_rules.map((rule) => ({
			id: `rule-${rule.id}`,
			enabled: rule.is_enabled,
			markets: rule.markets.join(", "),
			devices: [...rule.device_classes],
			audience:
				rule.auth_states.length === 1 ? (rule.auth_states[0] as "guest" | "authenticated") : "all",
			referrers: rule.referrers.join(", "),
			utmSources: rule.utm_sources.join(", "),
			segments: rule.segment_keys.join(", "),
		}));
		experimentEnabled = Boolean(response.experiment);
		if (response.experiment) {
			experimentName = response.experiment.name;
			experimentStatus = response.experiment.status;
			experimentStickyKey = response.experiment.sticky_key;
			experimentStartsAt = localDateTime(response.experiment.starts_at);
			experimentEndsAt = localDateTime(response.experiment.ends_at);
			controlVersionId = response.experiment.variants[0]?.version_id ?? null;
			variantVersionId = response.experiment.variants[1]?.version_id ?? null;
			controlAllocation = (response.experiment.variants[0]?.allocation ?? 5000) / 100;
		}
	}

	async function savePageDelivery() {
		if (selected.kind !== "page" || selected.id === null || deliverySaving) return;
		deliverySaving = true;
		notices.clear();
		try {
			const targetingRules: CmsTargetingRuleInput[] = deliveryRules.map((rule, index) => ({
				priority: index,
				is_enabled: rule.enabled,
				markets: splitValues(rule.markets),
				device_classes: rule.devices,
				auth_states: rule.audience === "all" ? [] : [rule.audience],
				referrers: splitValues(rule.referrers),
				utm_sources: splitValues(rule.utmSources),
				segment_keys: splitValues(rule.segments),
			}));
			const schedule = scheduleEnabled
				? {
						publish_at: new Date(schedulePublishAt).toISOString(),
						unpublish_at:
							scheduleExpiryEnabled && scheduleUnpublishAt
								? new Date(scheduleUnpublishAt).toISOString()
								: null,
						timezone: scheduleTimezone,
					}
				: undefined;
			const experiment =
				experimentEnabled && controlVersionId && variantVersionId
					? {
							name: experimentName.trim(),
							status: experimentStatus,
							sticky_key: experimentStickyKey,
							starts_at: new Date(experimentStartsAt).toISOString(),
							ends_at: experimentEndsAt ? new Date(experimentEndsAt).toISOString() : null,
							variants: [
								{
									name: "Control",
									version_id: controlVersionId,
									allocation: Math.round(controlAllocation * 100),
								},
								{
									name: "Variant",
									version_id: variantVersionId,
									allocation: 10_000 - Math.round(controlAllocation * 100),
								},
							],
						}
					: undefined;
			const response = await api.updateAdminCmsPageDelivery(selected.id, {
				schedule,
				targeting_rules: targetingRules,
				experiment,
			});
			hydrateDelivery(response);
			markSaved();
			notices.setSuccess("Page delivery settings saved.");
		} catch (error) {
			console.error(error);
			notices.setError(formatCmsAdminError(error, "Unable to save page delivery settings."));
		} finally {
			deliverySaving = false;
		}
	}

	function revertPageDraft() {
		if (selectedPage) {
			openPage(selectedPage);
		} else {
			pageBlocks = defaultPageBlocks();
			pageTitle = "";
			pagePath = "";
			pageSlug = "";
			pageTemplate = "default";
			pageVisibility = "public";
			markSaved();
		}
		void refreshCmsPreview();
	}

	function newEntity(tab: CmsTab = activeTab) {
		if (!savePrompt.confirmDiscard()) return;
		openNewEntity(tab);
	}

	function openNewEntity(tab: CmsTab = activeTab) {
		activeTab = tab;
		if (tab === "pages") {
			selected = { kind: "page", id: null };
			pagePath = "";
			pageSlug = "";
			pageTitle = "";
			pageTemplate = "default";
			pageVisibility = "public";
			pageBlocks = defaultPageBlocks();
			resetDelivery();
		} else if (tab === "global") {
			selected = { kind: "global", id: null };
			globalKey = "";
			globalTitle = "";
			globalRegion = "announcement_bar";
			globalBlocks = defaultGlobalBlocks("announcement_bar");
		}
		markSaved();
	}

	function changeTab(tab: string) {
		if (tab === activeTab) return false;
		if (!savePrompt.confirmDiscard()) return false;
		activeTab = tab as CmsTab;
		if (activeTab === "pages" && selected.kind !== "page") {
			openNewEntity("pages");
		}
		if (activeTab === "global" && selected.kind !== "global") {
			openNewEntity("global");
		}
		markSaved();
		return true;
	}

	function selectPage(page: CmsPageResponse) {
		if (selected.kind === "page" && selected.id === page.page.id) return;
		if (!savePrompt.confirmDiscard()) return;
		openPage(page);
	}

	function openPage(page: CmsPageResponse) {
		activeTab = "pages";
		selected = { kind: "page", id: page.page.id };
		pagePath = page.page.path;
		pageSlug = page.page.slug;
		pageTitle = page.page.title;
		pageTemplate = page.page.template_key;
		pageVisibility = page.page.visibility;
		pageBlocks = blocksFromPayload(pagePayloadFromResponse(page), defaultPageBlocks());
		previewBlocks = [];
		previewError = "";
		newPageVariant();
		markSaved();
		void loadPageDelivery(page);
		void loadPageSEO(page.page.id);
		void loadPageGovernance(page.page.id, page.entry.id);
	}

	async function loadPageGovernance(pageID: number, entryID: number) {
		try {
			[pageVariants, cmsAuditEvents] = await Promise.all([
				api.listAdminCmsPageVariants(pageID),
				api.listAdminCmsAuditEvents(entryID),
			]);
			if (selectedVariantId !== null) {
				const refreshed = pageVariants.find((variant) => variant.id === selectedVariantId);
				if (refreshed) openPageVariant(refreshed);
				else newPageVariant();
			}
		} catch (error) {
			console.error(error);
			notices.setError("Unable to load localization and governance settings.");
		}
	}

	function newPageVariant() {
		selectedVariantId = null;
		variantLocale = cmsLocales.find((locale) => locale.enabled && !locale.is_default)?.code ?? "";
		variantMarket = "";
		variantPath = pagePath;
		variantSlug = pageSlug;
		variantTitle = pageTitle;
		variantBlocks = blocksFromPayload(payloadFromBlocks(pageBlocks), []);
		variantComment = "";
	}

	function selectPageVariant(variant: CmsPageVariant) {
		if (selectedVariantId === variant.id) return;
		if (!savePrompt.confirmDiscard()) return;
		openPageVariant(variant);
	}

	function openPageVariant(variant: CmsPageVariant) {
		selectedVariantId = variant.id;
		variantLocale = variant.locale;
		variantMarket = variant.market;
		variantPath = variant.path;
		variantSlug = variant.slug;
		variantTitle = variant.title;
		variantBlocks = blocksFromPayload(variant.payload, []);
		variantComment = "";
		markSaved();
	}

	async function savePageVariant() {
		if (selected.kind !== "page" || selected.id === null || variantSaving) return;
		variantSaving = true;
		try {
			const input = {
				locale: variantLocale,
				market: variantMarket || undefined,
				path: variantPath,
				slug: variantSlug || undefined,
				title: variantTitle,
				payload: payloadFromBlocks(variantBlocks),
			};
			const saved =
				selectedVariantId === null
					? await api.createAdminCmsPageVariant(selected.id, input)
					: await api.updateAdminCmsPageVariant(selected.id, selectedVariantId, input);
			pageVariants = upsertBy(pageVariants, saved, (variant) => variant.id);
			openPageVariant(saved);
			await loadPageGovernance(selected.id, selectedPage?.entry.id ?? 0);
			notices.setSuccess("Localized variant saved.");
		} catch (error) {
			console.error(error);
			notices.setError(formatCmsAdminError(error, "Unable to save localized variant."));
		} finally {
			variantSaving = false;
		}
	}

	async function transitionPageVariant(
		action: "submit" | "approve" | "request_changes" | "publish" | "rollback"
	) {
		if (selected.kind !== "page" || selected.id === null || selectedVariantId === null) return;
		try {
			const updated = await api.transitionAdminCmsPageVariant(
				selected.id,
				selectedVariantId,
				action,
				variantComment
			);
			pageVariants = upsertBy(pageVariants, updated, (variant) => variant.id);
			openPageVariant(updated);
			await loadPageGovernance(selected.id, selectedPage?.entry.id ?? 0);
			notices.setSuccess("Editorial workflow updated.");
		} catch (error) {
			console.error(error);
			notices.setError(formatCmsAdminError(error, "Unable to update editorial workflow."));
		}
	}

	async function removePageVariant() {
		if (selected.kind !== "page" || selected.id === null || selectedVariantId === null) return;
		await api.deleteAdminCmsPageVariant(selected.id, selectedVariantId);
		pageVariants = pageVariants.filter((variant) => variant.id !== selectedVariantId);
		newPageVariant();
		notices.setSuccess("Localized variant deleted.");
	}

	async function saveLocales() {
		localeSaving = true;
		try {
			const response = await api.updateAdminCmsLocales({
				locales: cmsLocales.map((locale) => ({
					code: locale.code,
					name: locale.name,
					enabled: locale.enabled,
					is_default: locale.is_default,
					fallback_locale: locale.fallback_locale || null,
				})),
			});
			cmsLocales = response.locales;
			markSaved();
			notices.setSuccess("Locale settings saved.");
		} catch (error) {
			console.error(error);
			notices.setError(formatCmsAdminError(error, "Unable to save locale settings."));
		} finally {
			localeSaving = false;
		}
	}

	function addLocale() {
		cmsLocales = [
			...cmsLocales,
			{ code: "", name: "", enabled: true, is_default: false, fallback_locale: null },
		];
	}

	async function exportCMS() {
		const content = await api.exportAdminCmsContent();
		const blob = new Blob([JSON.stringify(content, null, 2)], { type: "application/json" });
		const link = document.createElement("a");
		link.href = URL.createObjectURL(blob);
		link.download = `cms-export-${new Date().toISOString().slice(0, 10)}.json`;
		link.click();
		URL.revokeObjectURL(link.href);
	}

	async function selectRestoreFile(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		try {
			const parsed = JSON.parse(await file.text()) as Partial<CmsContentExport>;
			if (
				parsed.schema_version !== 1 ||
				!Array.isArray(parsed.locales) ||
				!Array.isArray(parsed.pages) ||
				!Array.isArray(parsed.navigation) ||
				!Array.isArray(parsed.global_regions) ||
				!Array.isArray(parsed.variants)
			) {
				throw new Error("Unsupported or incomplete CMS export.");
			}
			const content = parsed as CmsContentExport;
			const preview = await api.previewAdminCmsRestore(content);
			if (!preview.valid)
				throw new Error(preview.errors.join(" ") || "This backup cannot be restored.");
			pendingRestore = content;
			pendingRestoreName = file.name;
			restorePreviewSummary = `${preview.pages} pages, ${preview.navigation} menus, ${preview.global_regions} global regions, and ${preview.variants} localized variants will be restored.`;
			restoreDialogOpen = true;
		} catch (error) {
			console.error(error);
			pendingRestore = null;
			pendingRestoreName = "";
			restorePreviewSummary = "";
			notices.setError(error instanceof Error ? error.message : "Unable to read CMS export.");
		} finally {
			input.value = "";
		}
	}

	function cancelRestore() {
		if (restoring) return;
		restoreDialogOpen = false;
		pendingRestore = null;
		pendingRestoreName = "";
		restorePreviewSummary = "";
	}

	async function confirmRestore() {
		if (!pendingRestore || restoring) return;
		restoring = true;
		try {
			await api.restoreAdminCmsContent(pendingRestore);
			restoreDialogOpen = false;
			pendingRestore = null;
			pendingRestoreName = "";
			await loadCMS();
			notices.setSuccess("CMS content restored from backup.");
		} catch (error) {
			console.error(error);
			notices.setError(formatCmsAdminError(error, "Unable to restore CMS backup."));
		} finally {
			restoring = false;
		}
	}

	async function loadPageSEO(pageID: number) {
		seoLoading = true;
		try {
			const response = await api.getAdminCmsPageSEO(pageID);
			if (selected.kind !== "page" || selected.id !== pageID) return;
			hydrateSEO(response);
			markSaved();
		} catch (error) {
			console.error(error);
			notices.setError("Unable to load page SEO settings.");
		} finally {
			if (selected.kind === "page" && selected.id === pageID) seoLoading = false;
		}
	}

	function hydrateSEO(response: CmsSEOResponse) {
		const seo = response.metadata;
		seoTitle = seo.title;
		seoDescription = seo.description;
		seoCanonicalURL = seo.canonical_url;
		seoRobots = seo.robots;
		seoOGTitle = seo.og_title;
		seoOGDescription = seo.og_description;
		seoOGImageMediaID = seo.og_image_media_id ?? "";
		seoTwitterCard = seo.twitter_card;
		seoTwitterTitle = seo.twitter_title;
		seoTwitterDescription = seo.twitter_description;
		seoTwitterImageMediaID = seo.twitter_image_media_id ?? "";
		const item = seo.json_ld[0];
		seoJSONLDType = (item?.["@type"] as typeof seoJSONLDType) ?? "";
		seoJSONLDName = typeof item?.name === "string" ? item.name : "";
		seoIssues = response.issues;
	}

	async function savePageSEO() {
		if (selected.kind !== "page" || selected.id === null || seoSaving) return;
		seoSaving = true;
		try {
			const response = await api.updateAdminCmsPageSEO(selected.id, {
				title: seoTitle.trim(),
				description: seoDescription.trim(),
				canonical_url: seoCanonicalURL.trim(),
				robots: seoRobots,
				og_title: seoOGTitle.trim(),
				og_description: seoOGDescription.trim(),
				og_image_media_id: seoOGImageMediaID || null,
				twitter_card: seoTwitterCard,
				twitter_title: seoTwitterTitle.trim(),
				twitter_description: seoTwitterDescription.trim(),
				twitter_image_media_id: seoTwitterImageMediaID || null,
				json_ld: seoJSONLDType ? [{ "@type": seoJSONLDType, name: seoJSONLDName.trim() }] : [],
			});
			hydrateSEO(response);
			markSaved();
			notices.setSuccess("SEO settings saved.");
		} catch (error) {
			console.error(error);
			notices.setError(formatCmsAdminError(error, "Unable to save SEO settings."));
		} finally {
			seoSaving = false;
		}
	}

	async function uploadSEOMedia(event: Event, target: "og" | "twitter") {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		try {
			const mediaID = await api.uploadMedia(file);
			if (target === "og") seoOGImageMediaID = mediaID;
			else seoTwitterImageMediaID = mediaID;
		} catch (error) {
			console.error(error);
			notices.setError("Unable to upload social image.");
		} finally {
			input.value = "";
		}
	}

	function newRedirect() {
		if (!savePrompt.confirmDiscard()) return;
		openNewRedirect();
	}

	function openNewRedirect() {
		selectedRedirectId = null;
		redirectSource = "";
		redirectTarget = "";
		redirectMatchType = "exact";
		redirectType = 301;
		redirectPriority = 0;
		redirectEnabled = true;
		markSaved();
	}

	function selectRedirect(rule: CmsRedirectRule) {
		if (selectedRedirectId === rule.id) return;
		if (!savePrompt.confirmDiscard()) return;
		openRedirect(rule);
	}

	function openRedirect(rule: CmsRedirectRule) {
		selectedRedirectId = rule.id;
		redirectSource = rule.source_pattern;
		redirectTarget = rule.target_url;
		redirectMatchType = rule.match_type;
		redirectType = rule.redirect_type;
		redirectPriority = rule.priority;
		redirectEnabled = rule.is_enabled;
		markSaved();
	}

	async function saveRedirect() {
		const payload = {
			source_pattern: redirectSource.trim(),
			target_url: redirectTarget.trim(),
			match_type: redirectMatchType,
			redirect_type: redirectType,
			priority: Number(redirectPriority),
			is_enabled: redirectEnabled,
		};
		try {
			const saved =
				selectedRedirectId === null
					? await api.createAdminCmsRedirect(payload)
					: await api.updateAdminCmsRedirect(selectedRedirectId, payload);
			redirects = upsertBy(redirects, saved, (rule) => rule.id);
			openRedirect(saved);
			notices.setSuccess("Redirect saved.");
		} catch (error) {
			console.error(error);
			notices.setError(formatCmsAdminError(error, "Unable to save redirect."));
		}
	}

	function requestDelete(target: DeleteTarget, event?: MouseEvent) {
		event?.stopPropagation();
		const deletesPersistedEntity =
			target.kind === "page" ||
			target.kind === "navigation" ||
			target.kind === "global" ||
			target.kind === "redirect";
		if (deletesPersistedEntity && hasUnsavedChanges && !savePrompt.confirmDiscard()) return;
		deleteTarget = target;
	}

	function cancelDelete() {
		if (deleting) return;
		deleteTarget = null;
	}

	function requestSelectedRedirectDelete(event: MouseEvent) {
		if (selectedRedirectId === null) return;
		requestDelete(
			{
				kind: "redirect",
				id: selectedRedirectId,
				label: redirectSource || "this redirect",
			},
			event
		);
	}

	function requestCurrentDelete(event: MouseEvent) {
		if (activeTab === "pages" && selectedPage) {
			requestDelete(
				{ kind: "page", id: selectedPage.page.id, label: selectedPage.page.title },
				event
			);
		} else if (activeTab === "navigation" && selectedNavigation) {
			requestDelete(
				{
					kind: "navigation",
					id: selectedNavigation.menu.id,
					label: selectedNavigation.menu.title,
				},
				event
			);
		} else if (activeTab === "global" && selectedGlobal) {
			requestDelete(
				{ kind: "global", id: selectedGlobal.region.id, label: selectedGlobal.region.title },
				event
			);
		}
	}

	function deleteTitle(target: DeleteTarget) {
		switch (target.kind) {
			case "page":
				return "Delete page?";
			case "navigation":
				return "Delete navigation menu?";
			case "global":
				return "Delete global region?";
			case "redirect":
				return "Delete redirect?";
			case "navigation_dropdown":
				return "Delete dropdown?";
			case "navigation_custom":
				return "Delete link?";
			case "navigation_page":
				return "Hide page from navigation?";
		}
	}

	function deleteMessage(target: DeleteTarget) {
		switch (target.kind) {
			case "page":
				return `Delete "${target.label}" and remove it from the storefront? This cannot be undone.`;
			case "navigation":
				return `Delete "${target.label}" and remove its published navigation from the storefront? This cannot be undone.`;
			case "global":
				return `Delete "${target.label}" and remove that global region from the storefront? This cannot be undone.`;
			case "redirect":
				return `Delete redirect "${target.label}"? This cannot be undone.`;
			case "navigation_dropdown":
				return `Delete dropdown "${target.label}"? Pages and links inside it will move to the top level. Save the navigation draft to keep this change.`;
			case "navigation_custom":
				return `Delete link "${target.label}"? Save the navigation draft to keep this change.`;
			case "navigation_page":
				return `Hide "${target.label}" from navigation? Save the navigation draft to keep this change.`;
		}
	}

	async function confirmDelete() {
		const target = deleteTarget;
		if (!target || deleting) return;
		deleting = true;
		try {
			if (target.kind === "page") {
				await api.deleteAdminCmsPage(target.id);
				pages = pages.filter((page) => page.page.id !== target.id);
				if (selected.kind === "page" && selected.id === target.id) openNewEntity("pages");
				syncNavigationRows();
				notices.setSuccess("Page deleted.");
			} else if (target.kind === "navigation") {
				await api.deleteAdminCmsNavigation(target.id);
				navigationMenus = navigationMenus.filter((menu) => menu.menu.id !== target.id);
				navigationKey = "";
				navigationTitle = "";
				navigationLocation = "";
				navigationDropdowns = [];
				navigationCustomItems = [];
				navigationRows = [];
				selectedNavigationItem = { kind: "settings" };
				markSaved();
				notices.setSuccess("Navigation menu deleted.");
			} else if (target.kind === "global") {
				await api.deleteAdminCmsGlobalRegion(target.id);
				globalRegions = globalRegions.filter((region) => region.region.id !== target.id);
				if (selected.kind === "global" && selected.id === target.id) openNewEntity("global");
				notices.setSuccess("Global region deleted.");
			} else if (target.kind === "redirect") {
				await api.deleteAdminCmsRedirect(target.id);
				redirects = redirects.filter((rule) => rule.id !== target.id);
				if (selectedRedirectId === target.id) openNewRedirect();
				notices.setSuccess("Redirect deleted.");
			} else if (target.kind === "navigation_dropdown") {
				removeNavigationDropdown(target.id);
				notices.setSuccess("Dropdown removed from draft.");
			} else if (target.kind === "navigation_custom") {
				removeNavigationCustomItem(target.id);
				notices.setSuccess("Link removed from draft.");
			} else if (target.kind === "navigation_page") {
				updateNavigationRow(target.id, { placement: "hidden", isEnabled: false });
				if (selectedNavigationItem.kind === "page" && selectedNavigationItem.id === target.id) {
					selectedNavigationItem = { kind: "settings" };
				}
				notices.setSuccess("Page hidden from navigation draft.");
			}
			deleteTarget = null;
		} catch (error) {
			console.error(error);
			notices.setError(formatCmsAdminError(error, "Unable to delete CMS item."));
		} finally {
			deleting = false;
		}
	}

	function selectGlobal(region: CmsGlobalRegionResponse) {
		if (selected.kind === "global" && selected.id === region.region.id) return;
		if (!savePrompt.confirmDiscard()) return;
		openGlobal(region);
	}

	function openGlobal(region: CmsGlobalRegionResponse) {
		activeTab = "global";
		selected = { kind: "global", id: region.region.id };
		globalKey = region.region.key;
		globalTitle = region.region.title;
		globalRegion = normalizeGlobalRegion(region.region.region);
		globalBlocks = normalizeGlobalBlocks(
			globalRegion,
			blocksFromPayload(globalPayloadFromResponse(region), defaultGlobalBlocks(globalRegion))
		);
		markSaved();
	}

	function normalizeGlobalRegion(value: string): GlobalRegionType {
		return globalRegionOptions.some((option) => option.id === value)
			? (value as GlobalRegionType)
			: "announcement_bar";
	}

	function syncNavigationRows() {
		const menu = navigationMenus[0];
		if (menu) {
			navigationKey = menu.menu.key;
			navigationTitle = menu.menu.title;
			navigationLocation = menu.menu.location;
		}
		const dropdownItems = (menu?.items ?? []).filter((item) => item.item_type === "dropdown");
		navigationDropdowns = dropdownItems.map((item) => ({
			id: `dropdown-${item.id}`,
			sourceId: item.id,
			label: item.label,
			sortOrder: item.sort_order,
		}));
		navigationCustomItems = (menu?.items ?? [])
			.filter(
				(item): item is (typeof menu.items)[number] & { item_type: "internal" | "external" } =>
					item.item_type === "internal" || item.item_type === "external"
			)
			.map((item) => ({
				id: `custom-${item.id}`,
				sourceId: item.id,
				label: item.label,
				itemType: item.item_type,
				targetRef: item.target_ref,
				url: item.url,
				placement: item.is_enabled === false ? "hidden" : parentPlacement(item.parent_id, menu),
				sortOrder: item.sort_order,
				isEnabled: item.is_enabled,
			}));
		navigationRows = pages
			.map((page, index) => {
				const item = menu?.items.find((entry) => entry.target_ref === page.page.path);
				return {
					pageId: page.page.id,
					path: page.page.path,
					title: page.page.title,
					label: item?.label ?? page.page.title,
					placement: item?.is_enabled === false ? "hidden" : parentPlacement(item?.parent_id, menu),
					sortOrder: item?.sort_order ?? (index + 1) * 10,
					isEnabled: item?.is_enabled ?? page.page.visibility === "public",
				};
			})
			.sort((a, b) => a.sortOrder - b.sortOrder || a.title.localeCompare(b.title));
		if (
			selectedNavigationItem.kind !== "settings" &&
			!navigationSelectionExists(selectedNavigationItem)
		) {
			selectedNavigationItem = { kind: "settings" };
		}
	}

	function parentPlacement(
		parentId: number | null | undefined,
		menu: CmsNavigationResponse | undefined
	) {
		if (!parentId || !menu) return "top";
		const parent = menu.items.find((item) => item.id === parentId);
		if (parent?.item_type === "dropdown") {
			return `dropdown-${parent.id}`;
		}
		return parent?.target_ref ?? "top";
	}

	function navigationPayload(): CmsNavigationItemInput[] {
		const dropdownItems = navigationDropdowns
			.filter((dropdown) => dropdown.label.trim())
			.sort((a, b) => a.sortOrder - b.sortOrder || a.label.localeCompare(b.label));
		const dropdownPayload = dropdownItems.map((dropdown, index) => ({
			id: index + 1,
			parent_id: null,
			label: dropdown.label.trim(),
			item_type: "dropdown" as const,
			target_ref: "",
			url: "",
			sort_order: dropdown.sortOrder,
			is_enabled: true,
		}));
		const customPayload = navigationCustomItems
			.filter((item) => item.label.trim() && item.isEnabled && item.placement !== "hidden")
			.sort((a, b) => a.sortOrder - b.sortOrder || a.label.localeCompare(b.label))
			.map((item, index) => ({
				id: dropdownItems.length + index + 1,
				parent_id: dropdownParentId(item.placement, dropdownItems),
				label: item.label.trim(),
				item_type: item.itemType,
				target_ref: item.targetRef.trim() || item.url.trim(),
				url: item.url.trim() || item.targetRef.trim(),
				sort_order: item.sortOrder,
				is_enabled: true,
			}));
		const visible = navigationRows
			.filter((row) => row.isEnabled && row.placement !== "hidden")
			.sort((a, b) => a.sortOrder - b.sortOrder || a.title.localeCompare(b.title));
		const pagePayload = visible.map((row, index) => ({
			id: dropdownItems.length + customPayload.length + index + 1,
			parent_id: dropdownParentId(row.placement, dropdownItems),
			label: row.label.trim() || row.title,
			item_type: "page" as const,
			target_ref: row.path,
			url: row.path,
			sort_order: row.sortOrder,
			is_enabled: true,
		}));
		return [...dropdownPayload, ...customPayload, ...pagePayload];
	}

	function dropdownParentId(
		placement: NavPlacement,
		dropdowns: NavigationDropdown[]
	): number | null {
		if (!placement.startsWith("dropdown-") && !placement.includes("dropdown")) return null;
		return Math.max(0, dropdowns.findIndex((dropdown) => dropdown.id === placement) + 1) || null;
	}

	async function loadCMS() {
		loading = true;
		loadError = "";
		notices.clear();
		try {
			const [pageList, navList, globalList, redirectList, localeSettings, governanceSettings, ops] =
				await Promise.all([
					api.listAdminCmsPages(),
					api.listAdminCmsNavigation(),
					api.listAdminCmsGlobalRegions(),
					api.listAdminCmsRedirects(),
					api.getAdminCmsLocales(),
					api.getAdminCmsGovernance(),
					api.getAdminCmsOperations(),
				]);
			pages = pageList.data;
			navigationMenus = navList.data;
			globalRegions = globalList.data;
			redirects = redirectList;
			cmsLocales = localeSettings.locales;
			governance = governanceSettings;
			operations = ops;
			syncNavigationRows();
			if (selected.kind === "page" && selected.id !== null) {
				const refreshed = pages.find((page) => page.page.id === selected.id);
				if (refreshed) openPage(refreshed);
			} else if (selected.kind === "global" && selected.id !== null) {
				const refreshed = globalRegions.find((region) => region.region.id === selected.id);
				if (refreshed) openGlobal(refreshed);
			} else {
				markSaved();
			}
		} catch (error) {
			console.error(error);
			loadError = "Unable to load CMS content.";
			notices.setError(loadError);
		} finally {
			loading = false;
		}
	}

	async function saveGovernance() {
		governanceSaving = true;
		try {
			governance = await api.updateAdminCmsGovernance({
				approval_required: governance.approval_required,
				invalidation_webhook_url: governance.invalidation_webhook_url.trim(),
				roles: governance.roles
					.filter((role) => role.subject.trim())
					.map((role) => ({ subject: role.subject.trim(), role: role.role })),
			});
			notices.setSuccess("CMS governance settings saved.");
		} catch (error) {
			console.error(error);
			notices.setError(formatCmsAdminError(error, "Unable to save governance settings."));
		} finally {
			governanceSaving = false;
		}
	}

	async function refreshOperations() {
		operationsLoading = true;
		try {
			operations = await api.getAdminCmsOperations();
		} catch (error) {
			console.error(error);
			notices.setError(formatCmsAdminError(error, "Unable to load CMS operations."));
		} finally {
			operationsLoading = false;
		}
	}

	async function retryInvalidation(id: number) {
		try {
			await api.retryAdminCmsInvalidation(id);
			await refreshOperations();
			notices.setSuccess("Invalidation queued.");
		} catch (error) {
			console.error(error);
			notices.setError(formatCmsAdminError(error, "Unable to retry invalidation."));
		}
	}

	function addGovernanceRole() {
		governance = {
			...governance,
			roles: [...governance.roles, { subject: "", role: "author" }],
		};
	}

	function updateGovernanceRole(index: number, patch: Partial<(typeof governance.roles)[number]>) {
		governance = {
			...governance,
			roles: governance.roles.map((role, roleIndex) =>
				roleIndex === index ? { ...role, ...patch } : role
			),
		};
	}

	function removeGovernanceRole(index: number) {
		governance = {
			...governance,
			roles: governance.roles.filter((_, roleIndex) => roleIndex !== index),
		};
	}

	function refreshCMS() {
		if (!savePrompt.confirmDiscard()) return;
		void loadCMS();
	}

	async function refreshCmsPreview() {
		if (activeTab !== "pages") return;
		previewLoading = true;
		previewError = "";
		try {
			const response = await api.previewAdminCmsPayload({
				payload: payloadFromBlocks(pageBlocks),
			});
			previewBlocks = response.blocks;
		} catch (error) {
			console.error(error);
			previewBlocks = [];
			previewError = formatCmsAdminError(error, "Unable to evaluate CMS blocks.");
		} finally {
			previewLoading = false;
		}
	}

	async function saveCurrent() {
		if (saving) return;
		saving = true;
		notices.clear();
		try {
			if (activeTab === "pages" && selected.kind === "page") {
				const request = {
					path: pagePath.trim(),
					slug: pageSlug.trim(),
					title: pageTitle.trim(),
					template_key: pageTemplate.trim() || "default",
					visibility: pageVisibility,
					payload: payloadFromBlocks(pageBlocks),
				};
				const response =
					selected.id === null
						? await api.createAdminCmsPage(request)
						: await api.updateAdminCmsPage(selected.id, request);
				pages = upsertBy(pages, response, (entry) => entry.page.id);
				syncNavigationRows();
				openPage(response);
				void refreshCmsPreview();
			} else if (activeTab === "navigation") {
				const request = {
					key: navigationKey.trim() || "main",
					title: navigationTitle.trim() || "Main navigation",
					location: navigationLocation.trim() || "header",
					items: navigationPayload(),
				};
				const response = selectedNavigation
					? await api.updateAdminCmsNavigation(selectedNavigation.menu.id, request)
					: await api.createAdminCmsNavigation(request);
				navigationMenus = upsertBy(navigationMenus, response, (entry) => entry.menu.id);
				syncNavigationRows();
				markSaved();
			} else if (selected.kind === "global") {
				const request = {
					key: globalKey.trim(),
					title: globalTitle.trim(),
					region: globalRegion,
					payload: payloadFromBlocks(globalBlocks),
				};
				const response =
					selected.id === null
						? await api.createAdminCmsGlobalRegion(request)
						: await api.updateAdminCmsGlobalRegion(selected.id, request);
				globalRegions = upsertBy(globalRegions, response, (entry) => entry.region.id);
				openGlobal(response);
			}
			notices.setSuccess("CMS draft saved.");
		} catch (error) {
			console.error(error);
			notices.setError(formatCmsAdminError(error, "Unable to save CMS draft."));
		} finally {
			saving = false;
		}
	}

	async function publishCurrent() {
		if (publishing || !canPublish) return;
		publishing = true;
		notices.clear();
		try {
			if (activeTab === "pages" && selected.kind === "page" && selected.id !== null) {
				const response = await api.publishAdminCmsPage(
					selected.id,
					"Published from admin CMS editor"
				);
				pages = upsertBy(pages, response, (entry) => entry.page.id);
				openPage(response);
				editMode = false;
			} else if (activeTab === "navigation" && selectedNavigation) {
				const response = await api.publishAdminCmsNavigation(
					selectedNavigation.menu.id,
					"Published from admin CMS editor"
				);
				navigationMenus = upsertBy(navigationMenus, response, (entry) => entry.menu.id);
				syncNavigationRows();
				markSaved();
			} else if (selected.kind === "global" && selected.id !== null) {
				const response = await api.publishAdminCmsGlobalRegion(
					selected.id,
					"Published from admin CMS editor"
				);
				globalRegions = upsertBy(globalRegions, response, (entry) => entry.region.id);
				openGlobal(response);
			}
			notices.setSuccess("CMS content published.");
		} catch (error) {
			console.error(error);
			notices.setError(formatCmsAdminError(error, "Unable to publish CMS content."));
		} finally {
			publishing = false;
		}
	}

	function currentPreviewURL(): string {
		if (activeTab === "pages") return pagePath || "/";
		return "/";
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
		if (event.key !== DRAFT_PREVIEW_SYNC_STORAGE_KEY) return;
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

	async function previewDraft() {
		if (!canPreviewDraft) return;
		notices.clear();
		previewingDraft = true;
		let previewWindow: Window | null = null;
		try {
			if (previewActive) {
				await api.stopAdminPreview();
				previewActive = false;
				notices.setSuccess("Exited draft preview.");
				return;
			}
			previewWindow = window.open("", "_blank");
			if (!previewWindow) {
				notices.setError("Preview popup was blocked by the browser.");
				return;
			}
			await api.startAdminPreview();
			previewActive = true;
			previewWindow.location.href = currentPreviewURL();
			notices.setSuccess("Opened draft preview in a new tab.");
		} catch (error) {
			console.error(error);
			if (previewWindow && !previewWindow.closed) previewWindow.close();
			notices.setError(formatCmsAdminError(error, "Unable to open draft preview."));
			void loadPreviewState();
		} finally {
			previewingDraft = false;
		}
	}

	async function unpublishCurrent() {
		if (!canUnpublish) return;
		if (!confirm("Unpublish this CMS content? It will be hidden from the public storefront."))
			return;
		unpublishing = true;
		notices.clear();
		try {
			if (activeTab === "pages" && selected.kind === "page" && selected.id !== null) {
				const response = await api.unpublishAdminCmsPage(
					selected.id,
					"Unpublished from admin CMS editor"
				);
				pages = upsertBy(pages, response, (entry) => entry.page.id);
				openPage(response);
			} else if (activeTab === "navigation" && selectedNavigation) {
				const response = await api.unpublishAdminCmsNavigation(
					selectedNavigation.menu.id,
					"Unpublished from admin CMS editor"
				);
				navigationMenus = upsertBy(navigationMenus, response, (entry) => entry.menu.id);
				syncNavigationRows();
				markSaved();
			} else if (selected.kind === "global" && selected.id !== null) {
				const response = await api.unpublishAdminCmsGlobalRegion(
					selected.id,
					"Unpublished from admin CMS editor"
				);
				globalRegions = upsertBy(globalRegions, response, (entry) => entry.region.id);
				openGlobal(response);
			}
			notices.setSuccess("CMS content unpublished.");
		} catch (error) {
			console.error(error);
			notices.setError(formatCmsAdminError(error, "Unable to unpublish CMS content."));
		} finally {
			unpublishing = false;
		}
	}

	async function discardCurrentDraft() {
		if (!canDiscardDraft) return;
		if (!confirm("Discard this draft? Unsaved CMS changes will be lost.")) return;
		discardingDraft = true;
		notices.clear();
		try {
			if (activeTab === "pages" && selected.kind === "page" && selected.id !== null) {
				const id = selected.id;
				const response = await api.discardAdminCmsPageDraft(id);
				if (response) {
					pages = upsertBy(pages, response, (entry) => entry.page.id);
					openPage(response);
				} else {
					pages = pages.filter((page) => page.page.id !== id);
					openNewEntity("pages");
				}
			} else if (activeTab === "navigation" && selectedNavigation) {
				const id = selectedNavigation.menu.id;
				const response = await api.discardAdminCmsNavigationDraft(id);
				if (response) {
					navigationMenus = upsertBy(navigationMenus, response, (entry) => entry.menu.id);
					syncNavigationRows();
					markSaved();
				} else {
					navigationMenus = navigationMenus.filter((menu) => menu.menu.id !== id);
					navigationKey = "";
					navigationTitle = "";
					navigationLocation = "";
					navigationDropdowns = [];
					navigationCustomItems = [];
					syncNavigationRows();
					selectedNavigationItem = { kind: "settings" };
					markSaved();
				}
			} else if (selected.kind === "global" && selected.id !== null) {
				const id = selected.id;
				const response = await api.discardAdminCmsGlobalRegionDraft(id);
				if (response) {
					globalRegions = upsertBy(globalRegions, response, (entry) => entry.region.id);
					openGlobal(response);
				} else {
					globalRegions = globalRegions.filter((region) => region.region.id !== id);
					openNewEntity("global");
				}
			}
			notices.setSuccess("CMS draft discarded.");
		} catch (error) {
			console.error(error);
			notices.setError(formatCmsAdminError(error, "Unable to discard CMS draft."));
		} finally {
			discardingDraft = false;
		}
	}

	function upsertBy<T>(items: T[], next: T, getKey: (item: T) => number): T[] {
		const key = getKey(next);
		const index = items.findIndex((item) => getKey(item) === key);
		if (index === -1) return [next, ...items];
		return items.map((item, currentIndex) => (currentIndex === index ? next : item));
	}

	function addBlock(target: "page" | "global" | "variant", type: CmsContentBlock["type"]) {
		const block = createBlock(type);
		if (target === "page") pageBlocks = [...pageBlocks, block];
		else if (target === "global") globalBlocks = [...globalBlocks, block];
		else variantBlocks = [...variantBlocks, block];
	}

	function createBlock(type: CmsContentBlock["type"]): EditableBlock {
		switch (type) {
			case "hero":
				return { editorId: editorId("hero"), type, title: "Hero title", subtitle: "" };
			case "image":
				return { editorId: editorId("image"), type, media_id: "", alt: "", caption: "" };
			case "cta":
				return { editorId: editorId("cta"), type, label: "Learn more", url: "/", body: "" };
			case "promo_banner":
				return {
					editorId: editorId("promo"),
					type,
					title: "Promotion",
					body: "",
					link: { label: "", url: "" },
				};
			case "product_rail":
				return createProductRailBlock("Featured products", "newest");
			case "category_tiles":
				return {
					editorId: editorId("category-tiles"),
					type,
					title: "Shop by category",
					subtitle: "",
					category_slugs: [],
					category_media_ids: {},
					image_aspect: "square",
				};
			case "promotion_highlight":
				return {
					editorId: editorId("promotion-highlight"),
					type,
					title: "Promotion",
					body: "",
					badge: "",
					promotion_code: "",
					link: { label: "", url: "" },
				};
			case "inventory_message":
				return {
					editorId: editorId("inventory-message"),
					type,
					product_id: 1,
					low_stock_threshold: 5,
					in_stock_message: "In stock",
					low_stock_message: "Almost gone",
					out_of_stock_message: "Out of stock",
				};
			case "testimonial":
				return {
					editorId: editorId("testimonial"),
					type,
					quote: "",
					attribution: "",
					rating: 5,
				};
			case "social_embed":
				return {
					editorId: editorId("social-embed"),
					type,
					provider: "instagram",
					url: "",
					title: "",
				};
			default:
				return { editorId: editorId("rich"), type: "rich_text", body: "" };
		}
	}

	function removeBlock(target: "page" | "global" | "variant", editorIdToRemove: string) {
		if (target === "page")
			pageBlocks = pageBlocks.filter((block) => block.editorId !== editorIdToRemove);
		else if (target === "global")
			globalBlocks = globalBlocks.filter((block) => block.editorId !== editorIdToRemove);
		else variantBlocks = variantBlocks.filter((block) => block.editorId !== editorIdToRemove);
	}

	function moveBlock(
		target: "page" | "global" | "variant",
		editorIdToMove: string,
		direction: -1 | 1
	) {
		const blocks =
			target === "page" ? pageBlocks : target === "global" ? globalBlocks : variantBlocks;
		const index = blocks.findIndex((block) => block.editorId === editorIdToMove);
		const nextIndex = index + direction;
		if (index < 0 || nextIndex < 0 || nextIndex >= blocks.length) return;
		const next = [...blocks];
		[next[index], next[nextIndex]] = [next[nextIndex], next[index]];
		if (target === "page") pageBlocks = next;
		else if (target === "global") globalBlocks = next;
		else variantBlocks = next;
	}

	function updateNavigationRow(pageId: number, updates: Partial<NavigationDraftRow>) {
		navigationRows = navigationRows.map((row) =>
			row.pageId === pageId
				? {
						...row,
						...updates,
						isEnabled:
							updates.placement === "hidden"
								? false
								: updates.placement
									? true
									: (updates.isEnabled ?? row.isEnabled),
					}
				: row
		);
	}

	function updateNavigationCustomItem(id: string, updates: Partial<NavigationCustomItem>) {
		navigationCustomItems = navigationCustomItems.map((item) =>
			item.id === id
				? {
						...item,
						...updates,
						isEnabled:
							updates.placement === "hidden"
								? false
								: updates.placement
									? true
									: (updates.isEnabled ?? item.isEnabled),
					}
				: item
		);
	}

	function updateNavigationDropdown(id: string, updates: Partial<NavigationDropdown>) {
		navigationDropdowns = navigationDropdowns.map((dropdown) =>
			dropdown.id === id ? { ...dropdown, ...updates } : dropdown
		);
	}

	function addNavigationDropdown() {
		const dropdown = {
			id: editorId("dropdown"),
			sourceId: null,
			label: "",
			sortOrder: (navigationDropdowns.length + 1) * 10,
		};
		navigationDropdowns = [...navigationDropdowns, dropdown];
		selectedNavigationItem = { kind: "dropdown", id: dropdown.id };
	}

	function removeNavigationDropdown(id: string) {
		navigationDropdowns = navigationDropdowns.filter((dropdown) => dropdown.id !== id);
		navigationRows = navigationRows.map((row) =>
			row.placement === id ? { ...row, placement: "top" } : row
		);
		navigationCustomItems = navigationCustomItems.map((item) =>
			item.placement === id ? { ...item, placement: "top" } : item
		);
		if (selectedNavigationItem.kind === "dropdown" && selectedNavigationItem.id === id) {
			selectedNavigationItem = { kind: "settings" };
		}
	}

	function addNavigationCustomItem() {
		const item = {
			id: editorId("custom"),
			sourceId: null,
			label: "",
			itemType: "internal" as const,
			targetRef: "",
			url: "",
			placement: "top",
			sortOrder: (navigationCustomItems.length + navigationRows.length + 1) * 10,
			isEnabled: true,
		};
		navigationCustomItems = [...navigationCustomItems, item];
		selectedNavigationItem = { kind: "custom", id: item.id };
	}

	function removeNavigationCustomItem(id: string) {
		navigationCustomItems = navigationCustomItems.filter((item) => item.id !== id);
		if (selectedNavigationItem.kind === "custom" && selectedNavigationItem.id === id) {
			selectedNavigationItem = { kind: "settings" };
		}
	}

	function navigationSelectionExists(item: SelectedNavigationItem): boolean {
		if (item.kind === "settings") return true;
		if (item.kind === "dropdown")
			return navigationDropdowns.some((dropdown) => dropdown.id === item.id);
		if (item.kind === "custom")
			return navigationCustomItems.some((custom) => custom.id === item.id);
		return navigationRows.some((row) => row.pageId === item.id);
	}

	function handleGlobalRegionChange(value: string) {
		globalRegion = normalizeGlobalRegion(value);
		globalBlocks = defaultGlobalBlocks(globalRegion);
	}

	onMount(() => {
		newEntity("pages");
		void loadCMS();
		void loadPreviewState();
		window.addEventListener(DRAFT_PREVIEW_SYNC_EVENT, handlePreviewSyncEvent);
		window.addEventListener("storage", handlePreviewStorageEvent);
		return () => {
			window.removeEventListener(DRAFT_PREVIEW_SYNC_EVENT, handlePreviewSyncEvent);
			window.removeEventListener("storage", handlePreviewStorageEvent);
		};
	});

	$effect(() => {
		savePrompt.dirty = hasUnsavedChanges;
		savePrompt.blocked = saving || publishing || unpublishing || discardingDraft;
		savePrompt.saveAction = hasUnsavedChanges ? saveCurrent : null;
	});
</script>

{#snippet pageActions()}
	<Button tone="admin" size="small" onclick={() => newEntity("pages")}>
		<i class="bi bi-plus-lg mr-1"></i>
		New page
	</Button>
{/snippet}

{#snippet globalActions()}
	<Button tone="admin" size="small" onclick={() => newEntity("global")}>
		<i class="bi bi-plus-lg mr-1"></i>
		New region
	</Button>
{/snippet}

{#snippet navigationActions()}
	<Button tone="admin" size="small" onclick={addNavigationDropdown}>
		<i class="bi bi-folder-plus mr-1"></i>
		Dropdown
	</Button>
	<Button tone="admin" size="small" onclick={addNavigationCustomItem}>
		<i class="bi bi-link-45deg mr-1"></i>
		Link
	</Button>
{/snippet}

{#snippet redirectActions()}
	<Button tone="admin" size="small" onclick={newRedirect}
		><i class="bi bi-plus-lg mr-1"></i>New redirect</Button
	>
{/snippet}

{#snippet refreshAction()}
	<Button tone="admin" variant="regular" size="small" onclick={refreshCMS} disabled={loading}>
		<i class="bi bi-arrow-clockwise mr-1"></i>
		Refresh
	</Button>
{/snippet}

{#snippet draftControlGrid()}
	<div class="mb-5 grid grid-cols-3 gap-2">
		<Button
			tone="admin"
			variant="primary"
			size="regular"
			class="w-full"
			onclick={() => void saveCurrent()}
			disabled={saving}
		>
			<i class="bi bi-floppy-fill mr-1"></i>
			{saving ? "Saving..." : isCreatingDraft ? "Create draft" : "Save draft"}
		</Button>
		<Button
			tone="admin"
			variant="regular"
			size="regular"
			class="w-full"
			onclick={() => void previewDraft()}
			disabled={!canPreviewDraft}
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
			size="regular"
			class="w-full"
			onclick={() => void publishCurrent()}
			disabled={!canPublish}
		>
			<i class="bi bi-send-check-fill mr-1"></i>
			{publishing ? "Publishing..." : "Publish"}
		</Button>
		<Button
			tone="admin"
			variant="warning"
			size="regular"
			class="w-full"
			onclick={() => void unpublishCurrent()}
			disabled={!canUnpublish}
		>
			<i class="bi bi-eye-slash-fill mr-1"></i>
			{unpublishing ? "Unpublishing..." : "Unpublish"}
		</Button>
		<Button
			tone="admin"
			variant="warning"
			size="regular"
			class="w-full"
			onclick={() => void discardCurrentDraft()}
			disabled={!canDiscardDraft}
		>
			<i class="bi bi-arrow-counterclockwise mr-1"></i>
			{discardingDraft ? "Discarding..." : "Discard draft"}
		</Button>
		<Button
			tone="admin"
			variant="danger"
			size="regular"
			class="w-full"
			onclick={requestCurrentDelete}
			disabled={activeTab === "navigation" ? !selectedNavigation : !selected.id}
		>
			<i class="bi bi-trash-fill mr-1"></i>
			Delete
		</Button>
	</div>
{/snippet}

{#snippet pageSectionActions()}
	<label class="sr-only" for="page-section-type">Section type</label>
	<Dropdown
		id="page-section-type"
		tone="admin"
		full={false}
		class="min-w-44 py-1.5"
		bind:value={selectedPageSectionType}
	>
		{#each pageSectionOptions as option (option.id)}
			<option value={option.id}>{option.label}</option>
		{/each}
	</Dropdown>
	<Button
		tone="admin"
		variant="primary"
		size="small"
		onclick={() => addBlock("page", selectedPageSectionType)}
	>
		<i class="bi bi-plus-lg mr-1"></i>
		Add
	</Button>
{/snippet}

{#snippet blockEditor(blocks: EditableBlock[], target: "page" | "global" | "variant")}
	<div class="space-y-4">
		{#each blocks as block, index (block.editorId)}
			<AdminSurface as="section" variant="muted" class="space-y-4">
				<div class="flex flex-wrap items-center justify-between gap-3">
					<div class="flex items-center gap-2">
						<Badge tone="neutral">{block.type.replace("_", " ")}</Badge>
						<span class="text-xs text-stone-500">Block {index + 1}</span>
					</div>
					<div class="flex gap-1">
						<IconButton
							tone="admin"
							outlined={true}
							size="sm"
							aria-label="Move block up"
							title="Move block up"
							onclick={() => moveBlock(target, block.editorId, -1)}
							disabled={index === 0}
						>
							<i class="bi bi-arrow-up"></i>
						</IconButton>
						<IconButton
							tone="admin"
							outlined={true}
							size="sm"
							aria-label="Move block down"
							title="Move block down"
							onclick={() => moveBlock(target, block.editorId, 1)}
							disabled={index === blocks.length - 1}
						>
							<i class="bi bi-arrow-down"></i>
						</IconButton>
						<IconButton
							tone="admin"
							variant="danger"
							outlined={true}
							size="sm"
							aria-label="Remove block"
							title="Remove block"
							onclick={() => removeBlock(target, block.editorId)}
						>
							<i class="bi bi-trash"></i>
						</IconButton>
					</div>
				</div>

				{#if block.type === "hero"}
					<div class="grid gap-4 md:grid-cols-2">
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Headline</span>
							<TextInput tone="admin" bind:value={block.title} />
						</label>
						<div class="text-sm">
							<span class="mb-1 block font-medium">Image</span>
							<label
								class="flex min-h-24 cursor-pointer items-center justify-center rounded-lg border border-dashed border-stone-300 bg-white p-3 dark:border-stone-700 dark:bg-stone-950"
							>
								<input
									class="sr-only"
									type="file"
									accept="image/*"
									onchange={(event) => void uploadEditorBlockMedia(block, event)}
								/>
								{uploadingBlockMedia === block.editorId
									? "Uploading..."
									: blockMediaPreview(block)
										? "Replace image"
										: "Upload image"}
							</label>
							{#if blockMediaPreview(block)}<img
									src={blockMediaPreview(block)}
									alt=""
									class="mt-2 h-24 w-full rounded-lg object-cover"
								/>{/if}
						</div>
					</div>
					<label class="block text-sm">
						<span class="mb-1 block font-medium">Subtitle</span>
						<TextArea tone="admin" class="min-h-24" bind:value={block.subtitle} />
					</label>
				{:else if block.type === "rich_text"}
					<label class="block text-sm">
						<span class="mb-1 block font-medium">Body</span>
						<TextArea tone="admin" class="min-h-40" bind:value={block.body} />
					</label>
				{:else if block.type === "image"}
					<div class="grid gap-4 md:grid-cols-2">
						<div class="text-sm">
							<span class="mb-1 block font-medium">Image</span>
							<label
								class="flex min-h-24 cursor-pointer items-center justify-center rounded-lg border border-dashed border-stone-300 bg-white p-3 dark:border-stone-700 dark:bg-stone-950"
							>
								<input
									class="sr-only"
									type="file"
									accept="image/*"
									onchange={(event) => void uploadEditorBlockMedia(block, event)}
								/>
								{uploadingBlockMedia === block.editorId
									? "Uploading..."
									: blockMediaPreview(block)
										? "Replace image"
										: "Upload image"}
							</label>
							{#if blockMediaPreview(block)}<img
									src={blockMediaPreview(block)}
									alt={block.alt ?? ""}
									class="mt-2 h-24 w-full rounded-lg object-cover"
								/>{/if}
						</div>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Alt text</span>
							<TextInput tone="admin" bind:value={block.alt} />
						</label>
					</div>
					<label class="block text-sm">
						<span class="mb-1 block font-medium">Caption</span>
						<TextInput tone="admin" bind:value={block.caption} />
					</label>
				{:else if block.type === "cta"}
					<div class="grid gap-4 md:grid-cols-2">
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Button label</span>
							<TextInput tone="admin" bind:value={block.label} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">URL</span>
							<TextInput tone="admin" bind:value={block.url} />
						</label>
					</div>
					<label class="block text-sm">
						<span class="mb-1 block font-medium">Body</span>
						<TextArea tone="admin" class="min-h-24" bind:value={block.body} />
					</label>
				{:else if block.type === "promo_banner"}
					<div class="grid gap-4 md:grid-cols-2">
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Title</span>
							<TextInput tone="admin" bind:value={block.title} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Link label</span>
							<TextInput tone="admin" bind:value={block.link!.label} />
						</label>
					</div>
					<div class="grid gap-4 md:grid-cols-2">
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Body</span>
							<TextInput tone="admin" bind:value={block.body} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Link URL</span>
							<TextInput tone="admin" bind:value={block.link!.url} />
						</label>
					</div>
				{:else if block.type === "product_rail"}
					<div class="grid gap-4 md:grid-cols-2">
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Title</span>
							<TextInput tone="admin" bind:value={block.title} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Subtitle</span>
							<TextInput tone="admin" bind:value={block.subtitle} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Source</span>
							<select
								class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm dark:border-stone-700 dark:bg-stone-900"
								bind:value={block.source}
							>
								<option value="newest">Newest</option>
								<option value="manual">Manual IDs</option>
								<option value="search">Search</option>
								<option value="category">Category</option>
							</select>
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Limit</span>
							<NumberInput tone="admin" bind:value={block.limit} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Product IDs</span>
							<TextInput
								tone="admin"
								value={(block.product_ids ?? []).join(", ")}
								placeholder="1, 2, 3"
								oninput={(event) =>
									(block.product_ids = (event.currentTarget as HTMLInputElement).value
										.split(",")
										.map((value) => Number(value.trim()))
										.filter((value) => Number.isInteger(value) && value > 0))}
							/>
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Search query</span>
							<TextInput tone="admin" bind:value={block.query} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Category slug</span>
							<TextInput tone="admin" bind:value={block.category_slug} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Sort</span>
							<select
								class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm dark:border-stone-700 dark:bg-stone-900"
								bind:value={block.sort}
							>
								<option value="created_at">Created</option>
								<option value="name">Name</option>
								<option value="price">Price</option>
							</select>
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Order</span>
							<select
								class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm dark:border-stone-700 dark:bg-stone-900"
								bind:value={block.order}
							>
								<option value="desc">Descending</option>
								<option value="asc">Ascending</option>
							</select>
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Image aspect</span>
							<select
								class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm dark:border-stone-700 dark:bg-stone-900"
								bind:value={block.image_aspect}
							>
								<option value="square">Square</option>
								<option value="wide">Wide</option>
							</select>
						</label>
					</div>
				{:else if block.type === "category_tiles"}
					<div class="grid gap-4 md:grid-cols-2">
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Title</span>
							<TextInput tone="admin" bind:value={block.title} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Subtitle</span>
							<TextInput tone="admin" bind:value={block.subtitle} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Category slugs</span>
							<TextInput
								tone="admin"
								value={block.category_slugs.join(", ")}
								placeholder="new-arrivals, sale"
								oninput={(event) =>
									(block.category_slugs = (event.currentTarget as HTMLInputElement).value
										.split(",")
										.map((value) => value.trim())
										.filter(Boolean))}
							/>
						</label>
						<div class="space-y-2 text-sm md:col-span-2">
							<span class="block font-medium">Tile images</span>
							{#if block.category_slugs.length === 0}
								<p class="text-stone-500 dark:text-stone-400">
									Add categories to upload their tile images.
								</p>
							{:else}
								<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
									{#each block.category_slugs as slug (slug)}
										<div class="rounded-lg border border-stone-200 p-3 dark:border-stone-800">
											<div class="mb-2 truncate font-medium">{slug}</div>
											{#if blockMediaPreview(block, slug)}<img
													src={blockMediaPreview(block, slug)}
													alt=""
													class="mb-2 aspect-video w-full rounded object-cover"
												/>{/if}
											<label
												class="flex cursor-pointer items-center justify-center rounded-lg border border-dashed border-stone-300 px-3 py-2 text-xs dark:border-stone-700"
											>
												<input
													class="sr-only"
													type="file"
													accept="image/*"
													onchange={(event) => void uploadEditorBlockMedia(block, event, slug)}
												/>
												{uploadingBlockMedia === `${block.editorId}:${slug}`
													? "Uploading..."
													: blockMediaPreview(block, slug)
														? "Replace"
														: "Upload"}
											</label>
										</div>
									{/each}
								</div>
							{/if}
						</div>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Image aspect</span>
							<select
								class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm dark:border-stone-700 dark:bg-stone-900"
								bind:value={block.image_aspect}
							>
								<option value="square">Square</option>
								<option value="wide">Wide</option>
							</select>
						</label>
					</div>
				{:else if block.type === "promotion_highlight"}
					<div class="grid gap-4 md:grid-cols-2">
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Title</span>
							<TextInput tone="admin" bind:value={block.title} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Badge</span>
							<TextInput tone="admin" bind:value={block.badge} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Promotion code</span>
							<TextInput tone="admin" bind:value={block.promotion_code} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Campaign ID</span>
							<NumberInput tone="admin" bind:value={block.campaign_id} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Link label</span>
							<TextInput tone="admin" bind:value={block.link!.label} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Link URL</span>
							<TextInput tone="admin" bind:value={block.link!.url} />
						</label>
					</div>
					<label class="block text-sm">
						<span class="mb-1 block font-medium">Body</span>
						<TextArea tone="admin" class="min-h-24" bind:value={block.body} />
					</label>
				{:else if block.type === "inventory_message"}
					<div class="grid gap-4 md:grid-cols-2">
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Product ID</span>
							<NumberInput tone="admin" bind:value={block.product_id} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Low stock threshold</span>
							<NumberInput tone="admin" bind:value={block.low_stock_threshold} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">In stock message</span>
							<TextInput tone="admin" bind:value={block.in_stock_message} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Low stock message</span>
							<TextInput tone="admin" bind:value={block.low_stock_message} />
						</label>
						<label class="block text-sm md:col-span-2">
							<span class="mb-1 block font-medium">Out of stock message</span>
							<TextInput tone="admin" bind:value={block.out_of_stock_message} />
						</label>
					</div>
				{:else if block.type === "testimonial"}
					<div class="grid gap-4 md:grid-cols-2">
						<label class="block text-sm md:col-span-2">
							<span class="mb-1 block font-medium">Quote</span>
							<TextArea tone="admin" class="min-h-24" bind:value={block.quote} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Attribution</span>
							<TextInput tone="admin" bind:value={block.attribution} />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Rating</span>
							<NumberInput tone="admin" bind:value={block.rating} />
						</label>
					</div>
				{:else if block.type === "social_embed"}
					<div class="grid gap-4 md:grid-cols-2">
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Provider</span>
							<select
								class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm dark:border-stone-700 dark:bg-stone-900"
								bind:value={block.provider}
							>
								<option value="instagram">Instagram</option>
								<option value="tiktok">TikTok</option>
								<option value="youtube">YouTube</option>
							</select>
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Title</span>
							<TextInput tone="admin" bind:value={block.title} />
						</label>
						<label class="block text-sm md:col-span-2">
							<span class="mb-1 block font-medium">URL</span>
							<TextInput tone="admin" bind:value={block.url} />
						</label>
					</div>
				{/if}
			</AdminSurface>
		{/each}
	</div>
{/snippet}

<svelte:head>
	<title>CMS - Admin</title>
</svelte:head>

<AdminFloatingNotices
	showUnsaved={savePrompt.dirty}
	unsavedMessage="You have unsaved CMS changes."
	canSaveUnsaved={savePrompt.canSave}
	onSaveUnsaved={() => void savePrompt.save()}
	savingUnsaved={savePrompt.saving || saving}
	statusMessage={notices.message}
	statusTone={notices.tone}
	onDismissStatus={() => notices.clear()}
/>

{#if editMode && activeTab === "pages"}
	<CmsVisualEditor
		bind:blocks={pageBlocks}
		bind:pageTitle
		{pagePath}
		{hasUnsavedChanges}
		{canPublish}
		{saving}
		{publishing}
		{previewBlocks}
		{previewLoading}
		{previewError}
		{createBlock}
		onSave={() => void saveCurrent()}
		onPublish={() => void publishCurrent()}
		onRevert={revertPageDraft}
		onClose={() => (editMode = false)}
		onRefreshPreview={() => void refreshCmsPreview()}
	/>
{/if}

<div class="mx-auto w-full max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
	<AdminPageHeader title="CMS" />

	<TabSwitcher
		items={cmsTabs}
		bind:value={activeTab}
		ariaLabel="CMS sections"
		onChange={changeTab}
	/>

	{#if loadError}
		<AdminEmptyState tone="error" class="mt-6">{loadError}</AdminEmptyState>
	{/if}

	{#if activeTab !== "redirects" && activeTab !== "operations"}
		<AdminMasterDetailLayout class="mt-6">
			{#snippet master()}
				<AdminPanel
					title={activeTab === "pages"
						? "Pages"
						: activeTab === "navigation"
							? "Pages in navigation"
							: "Global regions"}
					headerActions={activeTab === "pages"
						? pageActions
						: activeTab === "navigation"
							? navigationActions
							: activeTab === "global"
								? globalActions
								: undefined}
				>
					{#if activeTab === "pages"}
						{#if pages.length === 0}
							<AdminEmptyState>No CMS pages yet.</AdminEmptyState>
						{/if}
						<div class="space-y-3">
							{#each pages as page (page.page.id)}
								<AdminListItem
									as="button"
									active={selected.kind === "page" && selected.id === page.page.id}
									interactive={selected.kind !== "page" || selected.id !== page.page.id}
									class="flex items-center justify-between gap-3 p-4"
									onclick={() => selectPage(page)}
								>
									<div class="min-w-0">
										<div class="truncate font-medium">{page.page.title}</div>
										<div class="truncate text-xs text-stone-500">{page.page.path}</div>
									</div>
									<div class="flex flex-wrap justify-end gap-1">
										<Badge tone={entryIsPublished(page) ? "success" : "warning"} size="sm"
											>{entryIsPublished(page) ? "Published" : "Unpublished"}</Badge
										>
										{#if page.has_unpublished_draft}<Badge tone="info" size="sm">Draft</Badge>{/if}
									</div>
								</AdminListItem>
							{/each}
						</div>
					{:else if activeTab === "navigation"}
						<div class="space-y-3">
							<AdminListItem
								as="button"
								active={selectedNavigationItem.kind === "settings"}
								interactive={selectedNavigationItem.kind !== "settings"}
								class="flex items-center justify-between gap-3 p-4"
								onclick={() => (selectedNavigationItem = { kind: "settings" })}
							>
								<div class="min-w-0">
									<div class="truncate font-medium">Navigation settings</div>
									<div class="truncate text-xs text-stone-500">
										{navigationLocation || "header"}
									</div>
								</div>
								<Badge tone="neutral" size="sm">Menu</Badge>
							</AdminListItem>

							{#each navigationDropdowns as dropdown (dropdown.id)}
								<AdminListItem
									as="button"
									active={selectedNavigationItem.kind === "dropdown" &&
										selectedNavigationItem.id === dropdown.id}
									interactive={selectedNavigationItem.kind !== "dropdown" ||
										selectedNavigationItem.id !== dropdown.id}
									class="flex items-center justify-between gap-3 p-4"
									onclick={() => (selectedNavigationItem = { kind: "dropdown", id: dropdown.id })}
								>
									<div class="min-w-0">
										<div class="truncate font-medium">{dropdown.label || "Untitled dropdown"}</div>
										<div class="truncate text-xs text-stone-500">Dropdown</div>
									</div>
									<Badge tone="success" size="sm">Dropdown</Badge>
								</AdminListItem>
							{/each}

							{#each navigationCustomItems as item (item.id)}
								<AdminListItem
									as="button"
									active={selectedNavigationItem.kind === "custom" &&
										selectedNavigationItem.id === item.id}
									interactive={selectedNavigationItem.kind !== "custom" ||
										selectedNavigationItem.id !== item.id}
									class="flex items-center justify-between gap-3 p-4"
									onclick={() => (selectedNavigationItem = { kind: "custom", id: item.id })}
								>
									<div class="min-w-0">
										<div class="truncate font-medium">{item.label || "Untitled link"}</div>
										<div class="truncate text-xs text-stone-500">
											{item.url || item.targetRef || "No target"}
										</div>
									</div>
									<Badge
										tone={item.placement === "hidden" || !item.isEnabled ? "neutral" : "success"}
										size="sm"
									>
										{item.placement === "hidden" || !item.isEnabled ? "Hidden" : "Link"}
									</Badge>
								</AdminListItem>
							{/each}

							{#if navigationRows.length === 0}
								<AdminEmptyState>Create pages before adding page links.</AdminEmptyState>
							{/if}
							{#each navigationRows as row (row.pageId)}
								<AdminListItem
									as="button"
									active={selectedNavigationItem.kind === "page" &&
										selectedNavigationItem.id === row.pageId}
									interactive={selectedNavigationItem.kind !== "page" ||
										selectedNavigationItem.id !== row.pageId}
									class="flex items-center justify-between gap-3 p-4"
									onclick={() => (selectedNavigationItem = { kind: "page", id: row.pageId })}
								>
									<div class="min-w-0">
										<div class="truncate font-medium">{row.label || row.title}</div>
										<div class="truncate text-xs text-stone-500">{row.path}</div>
									</div>
									<Badge
										tone={row.placement === "hidden" || !row.isEnabled ? "neutral" : "success"}
										size="sm"
									>
										{row.placement === "hidden" || !row.isEnabled
											? "Hidden"
											: row.placement === "top"
												? "Top level"
												: "Dropdown"}
									</Badge>
								</AdminListItem>
							{/each}
						</div>
					{:else}
						{#if globalRegions.length === 0}
							<AdminEmptyState>No CMS global regions yet.</AdminEmptyState>
						{/if}
						<div class="space-y-3">
							{#each globalRegions as region (region.region.id)}
								<AdminListItem
									as="button"
									active={selected.kind === "global" && selected.id === region.region.id}
									interactive={selected.kind !== "global" || selected.id !== region.region.id}
									class="flex items-center justify-between gap-3 p-4"
									onclick={() => selectGlobal(region)}
								>
									<div class="min-w-0">
										<div class="truncate font-medium">{region.region.title}</div>
										<div class="truncate text-xs text-stone-500">{region.region.region}</div>
									</div>
									<div class="flex flex-wrap justify-end gap-1">
										<Badge tone={entryIsPublished(region) ? "success" : "warning"} size="sm"
											>{entryIsPublished(region) ? "Published" : "Unpublished"}</Badge
										>
										{#if region.has_unpublished_draft}<Badge tone="info" size="sm">Draft</Badge
											>{/if}
									</div>
								</AdminListItem>
							{/each}
						</div>
					{/if}
				</AdminPanel>
			{/snippet}

			{#snippet detail()}
				<AdminPanel
					title={activeTab === "navigation"
						? "Navigation structure"
						: selected.id === null
							? "New draft"
							: "Edit draft"}
					headerActions={refreshAction}
				>
					<div class="mb-5 flex flex-wrap items-center gap-2">
						<Badge tone={isPublished ? "success" : "warning"}>
							{isPublished ? "Published" : "Unpublished"}
						</Badge>
						{#if hasDraftChanges}<Badge tone="info">Draft changes</Badge>{/if}
						{#if hasUnsavedChanges}
							<span class="text-sm text-stone-500">Unsaved changes</span>
						{/if}
						{#if activeTab === "pages"}
							<Button
								tone="admin"
								size="small"
								variant="primary"
								class="ml-auto"
								onclick={() => {
									editMode = true;
									void refreshCmsPreview();
								}}
							>
								<i class="bi bi-pencil-square mr-1"></i>
								Edit mode
							</Button>
						{/if}
					</div>

					{@render draftControlGrid()}

					{#if activeTab === "pages"}
						<div class="grid gap-4 md:grid-cols-2">
							<label class="block text-sm"
								><span class="mb-1 block font-medium">Title</span><TextInput
									tone="admin"
									bind:value={pageTitle}
								/></label
							>
							<label class="block text-sm"
								><span class="mb-1 block font-medium">Path</span><TextInput
									tone="admin"
									bind:value={pagePath}
									placeholder="/shipping"
								/></label
							>
							<label class="block text-sm"
								><span class="mb-1 block font-medium">Slug</span><TextInput
									tone="admin"
									bind:value={pageSlug}
								/></label
							>
						</div>
						<label class="mt-4 block text-sm">
							<span class="mb-1 block font-medium">Visibility</span>
							<select
								class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm dark:border-stone-700 dark:bg-stone-900"
								bind:value={pageVisibility}
							>
								<option value="public">Public</option>
								<option value="hidden">Hidden</option>
							</select>
						</label>
						<div class="mt-5 border-t border-stone-200 pt-5 dark:border-stone-800">
							<p class="mb-2 text-sm font-medium">Page presets</p>
							<div class="flex flex-wrap gap-2">
								{#each campaignTemplates as template (template.id)}
									<Button
										tone="admin"
										variant="regular"
										size="small"
										onclick={() => applyCampaignTemplate(template.id)}
									>
										{template.label}
									</Button>
								{/each}
							</div>
						</div>
					{:else if activeTab === "navigation"}
						{#if selectedNavigationItem.kind === "settings"}
							<div class="grid gap-4 md:grid-cols-3">
								<label class="block text-sm"
									><span class="mb-1 block font-medium">Key</span><TextInput
										tone="admin"
										bind:value={navigationKey}
									/></label
								>
								<label class="block text-sm"
									><span class="mb-1 block font-medium">Title</span><TextInput
										tone="admin"
										bind:value={navigationTitle}
									/></label
								>
								<label class="block text-sm"
									><span class="mb-1 block font-medium">Location</span><TextInput
										tone="admin"
										bind:value={navigationLocation}
									/></label
								>
							</div>
						{:else if selectedNavigationItem.kind === "dropdown" && selectedNavigationDropdown}
							<div class="grid gap-4 md:grid-cols-[1fr_8rem_auto]">
								<label class="block text-sm">
									<span class="mb-1 block font-medium">Dropdown label</span>
									<TextInput
										tone="admin"
										value={selectedNavigationDropdown.label}
										oninput={(event) =>
											updateNavigationDropdown(selectedNavigationDropdown.id, {
												label: (event.currentTarget as HTMLInputElement).value,
											})}
									/>
								</label>
								<label class="block text-sm">
									<span class="mb-1 block font-medium">Order</span>
									<NumberInput
										tone="admin"
										value={selectedNavigationDropdown.sortOrder}
										oninput={(event) =>
											updateNavigationDropdown(selectedNavigationDropdown.id, {
												sortOrder: Number((event.currentTarget as HTMLInputElement).value || 0),
											})}
									/>
								</label>
								<div class="flex items-end">
									<IconButton
										tone="admin"
										variant="danger"
										outlined={true}
										aria-label="Remove dropdown"
										title="Remove dropdown"
										onclick={(event) =>
											requestDelete(
												{
													kind: "navigation_dropdown",
													id: selectedNavigationDropdown.id,
													label: selectedNavigationDropdown.label || "Untitled dropdown",
												},
												event
											)}
									>
										<i class="bi bi-trash"></i>
									</IconButton>
								</div>
							</div>
						{:else if selectedNavigationItem.kind === "custom" && selectedNavigationCustomItem}
							<div class="grid gap-4 md:grid-cols-2">
								<label class="block text-sm">
									<span class="mb-1 block font-medium">Label</span>
									<TextInput
										tone="admin"
										value={selectedNavigationCustomItem.label}
										oninput={(event) =>
											updateNavigationCustomItem(selectedNavigationCustomItem.id, {
												label: (event.currentTarget as HTMLInputElement).value,
											})}
									/>
								</label>
								<label class="block text-sm">
									<span class="mb-1 block font-medium">Type</span>
									<select
										class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm dark:border-stone-700 dark:bg-stone-900"
										value={selectedNavigationCustomItem.itemType}
										onchange={(event) =>
											updateNavigationCustomItem(selectedNavigationCustomItem.id, {
												itemType: (event.currentTarget as HTMLSelectElement).value as
													| "internal"
													| "external",
											})}
									>
										<option value="internal">Internal</option>
										<option value="external">External</option>
									</select>
								</label>
								<label class="block text-sm">
									<span class="mb-1 block font-medium">Target</span>
									<TextInput
										tone="admin"
										value={selectedNavigationCustomItem.targetRef}
										placeholder="/search"
										oninput={(event) =>
											updateNavigationCustomItem(selectedNavigationCustomItem.id, {
												targetRef: (event.currentTarget as HTMLInputElement).value,
											})}
									/>
								</label>
								<label class="block text-sm">
									<span class="mb-1 block font-medium">URL</span>
									<TextInput
										tone="admin"
										value={selectedNavigationCustomItem.url}
										placeholder="/search"
										oninput={(event) =>
											updateNavigationCustomItem(selectedNavigationCustomItem.id, {
												url: (event.currentTarget as HTMLInputElement).value,
											})}
									/>
								</label>
								<label class="block text-sm">
									<span class="mb-1 block font-medium">Placement</span>
									<select
										class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm dark:border-stone-700 dark:bg-stone-900"
										value={selectedNavigationCustomItem.placement}
										onchange={(event) =>
											updateNavigationCustomItem(selectedNavigationCustomItem.id, {
												placement: (event.currentTarget as HTMLSelectElement).value,
											})}
									>
										<option value="top">Top level</option>
										<option value="hidden">Hidden</option>
										{#each navigationDropdowns as dropdown (dropdown.id)}
											<option value={dropdown.id}
												>Under {dropdown.label || "Untitled dropdown"}</option
											>
										{/each}
									</select>
								</label>
								<label class="block text-sm">
									<span class="mb-1 block font-medium">Order</span>
									<NumberInput
										tone="admin"
										value={selectedNavigationCustomItem.sortOrder}
										oninput={(event) =>
											updateNavigationCustomItem(selectedNavigationCustomItem.id, {
												sortOrder: Number((event.currentTarget as HTMLInputElement).value || 0),
											})}
									/>
								</label>
							</div>
							<div class="mt-5">
								<Button
									tone="admin"
									variant="danger"
									size="small"
									onclick={(event) =>
										requestDelete(
											{
												kind: "navigation_custom",
												id: selectedNavigationCustomItem.id,
												label: selectedNavigationCustomItem.label || "Untitled link",
											},
											event
										)}
								>
									Remove link
								</Button>
							</div>
						{:else if selectedNavigationItem.kind === "page" && selectedNavigationPage}
							<div class="grid gap-4 md:grid-cols-[1fr_1fr_8rem]">
								<label class="block text-sm">
									<span class="mb-1 block font-medium">{selectedNavigationPage.title}</span>
									<TextInput
										tone="admin"
										value={selectedNavigationPage.label}
										oninput={(event) =>
											updateNavigationRow(selectedNavigationPage.pageId, {
												label: (event.currentTarget as HTMLInputElement).value,
											})}
									/>
								</label>
								<label class="block text-sm">
									<span class="mb-1 block font-medium">Placement</span>
									<select
										class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm dark:border-stone-700 dark:bg-stone-900"
										value={selectedNavigationPage.placement}
										onchange={(event) =>
											updateNavigationRow(selectedNavigationPage.pageId, {
												placement: (event.currentTarget as HTMLSelectElement).value,
											})}
									>
										<option value="top">Top level</option>
										<option value="hidden">Hidden</option>
										{#each navigationDropdowns as dropdown (dropdown.id)}
											<option value={dropdown.id}
												>Under {dropdown.label || "Untitled dropdown"}</option
											>
										{/each}
									</select>
								</label>
								<label class="block text-sm">
									<span class="mb-1 block font-medium">Order</span>
									<NumberInput
										tone="admin"
										value={selectedNavigationPage.sortOrder}
										oninput={(event) =>
											updateNavigationRow(selectedNavigationPage.pageId, {
												sortOrder: Number((event.currentTarget as HTMLInputElement).value || 0),
											})}
									/>
								</label>
							</div>
							<AdminSurface
								as="div"
								variant="muted"
								class="mt-5 text-sm text-stone-600 dark:text-stone-300"
							>
								{selectedNavigationPage.path}
							</AdminSurface>
							<Button
								tone="admin"
								variant="danger"
								size="small"
								class="mt-4"
								onclick={(event) =>
									requestDelete(
										{
											kind: "navigation_page",
											id: selectedNavigationPage.pageId,
											label: selectedNavigationPage.title,
										},
										event
									)}
							>
								<i class="bi bi-eye-slash mr-1"></i>
								Hide from navigation
							</Button>
						{:else}
							<AdminEmptyState>Select a navigation item to edit.</AdminEmptyState>
						{/if}
					{:else}
						<div class="grid gap-4 md:grid-cols-3">
							<label class="block text-sm"
								><span class="mb-1 block font-medium">Key</span><TextInput
									tone="admin"
									bind:value={globalKey}
								/></label
							>
							<label class="block text-sm"
								><span class="mb-1 block font-medium">Title</span><TextInput
									tone="admin"
									bind:value={globalTitle}
								/></label
							>
							<label class="block text-sm">
								<span class="mb-1 block font-medium">Region</span>
								<select
									class="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm dark:border-stone-700 dark:bg-stone-900"
									bind:value={globalRegion}
									onchange={(event) =>
										handleGlobalRegionChange((event.target as HTMLSelectElement).value)}
								>
									{#each globalRegionOptions as option (option.id)}
										<option value={option.id}>{option.label}</option>
									{/each}
								</select>
							</label>
						</div>
						<div class="mt-6">
							<CmsGlobalRegionEditor region={globalRegion} bind:blocks={globalBlocks} />
						</div>
					{/if}
				</AdminPanel>
			{/snippet}
		</AdminMasterDetailLayout>
	{:else if activeTab === "redirects"}
		<AdminMasterDetailLayout class="mt-6">
			{#snippet master()}
				<AdminPanel title="Redirects" headerActions={redirectActions}>
					{#if redirects.length === 0}<AdminEmptyState>No redirects configured.</AdminEmptyState
						>{/if}
					<div class="space-y-3">
						{#each redirects as rule (rule.id)}
							<AdminListItem
								as="button"
								active={selectedRedirectId === rule.id}
								interactive={selectedRedirectId !== rule.id}
								class="flex items-center justify-between gap-3 p-4"
								onclick={() => selectRedirect(rule)}
							>
								<div class="min-w-0 text-left">
									<div class="truncate font-medium">{rule.source_pattern}</div>
									<div class="truncate text-xs text-stone-500">{rule.target_url}</div>
								</div>
								<Badge tone={rule.is_enabled ? "success" : "neutral"} size="sm"
									>{rule.redirect_type}</Badge
								>
							</AdminListItem>
						{/each}
					</div>
				</AdminPanel>
			{/snippet}
			{#snippet detail()}
				<AdminPanel title={selectedRedirectId === null ? "New redirect" : "Edit redirect"}>
					<div class="grid gap-4 md:grid-cols-2">
						<label class="block text-sm"
							><span class="mb-1 block font-medium">From</span><TextInput
								tone="admin"
								bind:value={redirectSource}
								placeholder="/old-path"
							/></label
						>
						<label class="block text-sm"
							><span class="mb-1 block font-medium">To</span><TextInput
								tone="admin"
								bind:value={redirectTarget}
								placeholder="/new-path"
							/></label
						>
						<label class="block text-sm"
							><span class="mb-1 block font-medium">Match</span><Dropdown
								tone="admin"
								bind:value={redirectMatchType}
								><option value="exact">Exact path</option><option value="prefix"
									>Path and everything below it</option
								></Dropdown
							></label
						>
						<label class="block text-sm"
							><span class="mb-1 block font-medium">Redirect type</span><Dropdown
								tone="admin"
								bind:value={redirectType}
								><option value={301}>Permanent (301)</option><option value={302}
									>Temporary (302)</option
								></Dropdown
							></label
						>
						<label class="block text-sm"
							><span class="mb-1 block font-medium">Priority</span><NumberInput
								tone="admin"
								bind:value={redirectPriority}
							/></label
						>
						<label class="flex items-center gap-2 self-end pb-2 text-sm font-medium"
							><input
								class="size-4 accent-stone-900 dark:accent-stone-100"
								type="checkbox"
								bind:checked={redirectEnabled}
							/>Enabled</label
						>
					</div>
					<div class="mt-6 flex justify-between gap-3">
						{#if selectedRedirectId !== null}<Button
								tone="admin"
								variant="danger"
								onclick={requestSelectedRedirectDelete}
								><i class="bi bi-trash mr-1"></i>Delete</Button
							>{:else}<span></span>{/if}
						<Button
							tone="admin"
							variant="primary"
							disabled={!redirectSource.trim() || !redirectTarget.trim()}
							onclick={() => void saveRedirect()}
							><i class="bi bi-floppy mr-1"></i>Save redirect</Button
						>
					</div>
				</AdminPanel>
			{/snippet}
		</AdminMasterDetailLayout>
	{:else}
		<div class="mt-6 grid gap-6 xl:grid-cols-[1fr_1fr]">
			<AdminPanel title="Governance">
				<div class="space-y-5">
					<label class="flex items-center gap-3 text-sm">
						<input
							type="checkbox"
							class="h-4 w-4 rounded border-stone-300"
							bind:checked={governance.approval_required}
						/>
						<span>Require approval before publishing</span>
					</label>
					<label class="block text-sm">
						<span class="mb-1 block font-medium">Invalidation webhook URL</span>
						<TextInput
							tone="admin"
							bind:value={governance.invalidation_webhook_url}
							placeholder="https://example.com/cms/invalidate"
						/>
					</label>
					<section aria-labelledby="cms-roles-heading">
						<div class="mb-3 flex items-center justify-between gap-3">
							<h3 id="cms-roles-heading" class="text-sm font-semibold">CMS roles</h3>
							<Button tone="admin" variant="regular" size="small" onclick={addGovernanceRole}>
								<i class="bi bi-plus-lg mr-1"></i>Add role
							</Button>
						</div>
						{#if governance.roles.length === 0}
							<AdminEmptyState>No explicit role assignments.</AdminEmptyState>
						{:else}
							<div class="space-y-3">
								{#each governance.roles as role, index (index)}
									<AdminSurface
										as="div"
										variant="muted"
										class="grid gap-3 p-3 md:grid-cols-[1fr_10rem_auto]"
									>
										<TextInput
											tone="admin"
											value={role.subject}
											placeholder="user@example.com"
											oninput={(event) =>
												updateGovernanceRole(index, {
													subject: (event.currentTarget as HTMLInputElement).value,
												})}
										/>
										<Dropdown
											tone="admin"
											value={role.role}
											onchange={(event) =>
												updateGovernanceRole(index, {
													role: (event.currentTarget as HTMLSelectElement).value as
														| "author"
														| "editor"
														| "publisher",
												})}
										>
											<option value="author">Author</option>
											<option value="editor">Editor</option>
											<option value="publisher">Publisher</option>
										</Dropdown>
										<IconButton
											tone="admin"
											variant="danger"
											outlined={true}
											size="sm"
											aria-label="Remove role"
											title="Remove role"
											onclick={() => removeGovernanceRole(index)}
										>
											<i class="bi bi-trash"></i>
										</IconButton>
									</AdminSurface>
								{/each}
							</div>
						{/if}
					</section>
					<div class="flex justify-end">
						<Button
							tone="admin"
							variant="primary"
							onclick={() => void saveGovernance()}
							disabled={governanceSaving}
						>
							<i class="bi bi-floppy mr-1"></i>{governanceSaving ? "Saving..." : "Save governance"}
						</Button>
					</div>
				</div>
			</AdminPanel>

			<AdminPanel title="Operations">
				<div class="space-y-5">
					<div class="flex items-center justify-between gap-3">
						<div class="grid grid-cols-2 gap-3 text-sm">
							<AdminSurface as="div" variant="muted" class="p-4">
								<div class="text-2xl font-semibold">{operations?.pending_schedules ?? 0}</div>
								<div class="text-stone-500 dark:text-stone-400">Pending schedules</div>
							</AdminSurface>
							<AdminSurface as="div" variant="muted" class="p-4">
								<div class="text-2xl font-semibold">{operations?.active_experiments ?? 0}</div>
								<div class="text-stone-500 dark:text-stone-400">Active experiments</div>
							</AdminSurface>
						</div>
						<Button
							tone="admin"
							variant="regular"
							onclick={() => void refreshOperations()}
							disabled={operationsLoading}
						>
							<i class="bi bi-arrow-clockwise mr-1"></i>{operationsLoading
								? "Refreshing..."
								: "Refresh"}
						</Button>
					</div>
					<section aria-labelledby="cms-invalidations-heading">
						<h3 id="cms-invalidations-heading" class="mb-3 text-sm font-semibold">Invalidations</h3>
						{#if !operations || operations.invalidations.length === 0}
							<AdminEmptyState>No invalidation events.</AdminEmptyState>
						{:else}
							<div class="space-y-3">
								{#each operations.invalidations as event (event.id)}
									<AdminListItem class="flex items-center justify-between gap-3 p-4">
										<div class="min-w-0">
											<div class="font-medium">{event.reason}</div>
											<div class="truncate text-xs text-stone-500">
												Entry {event.entry_id} · attempts {event.attempts}{event.last_error
													? ` · ${event.last_error}`
													: ""}
											</div>
										</div>
										<div class="flex items-center gap-2">
											<Badge
												tone={event.status === "sent"
													? "success"
													: event.status === "failed"
														? "danger"
														: "neutral"}
												size="sm"
											>
												{event.status}
											</Badge>
											<Button
												tone="admin"
												variant="regular"
												size="small"
												onclick={() => void retryInvalidation(event.id)}
												disabled={event.status === "pending"}
											>
												Retry
											</Button>
										</div>
									</AdminListItem>
								{/each}
							</div>
						{/if}
					</section>
				</div>
			</AdminPanel>
		</div>
	{/if}

	{#if activeTab === "pages"}
		<AdminPanel title="Languages and markets" class="mt-6">
			<div class="flex flex-wrap items-center justify-between gap-3">
				<p class="text-sm text-stone-500 dark:text-stone-400">
					Configure storefront languages and deterministic fallback order.
				</p>
				<Button tone="admin" size="small" onclick={addLocale}>
					<i class="bi bi-plus-lg mr-1"></i>Add language
				</Button>
			</div>
			<div
				class="mt-4 divide-y divide-stone-200 border-y border-stone-200 dark:divide-stone-800 dark:border-stone-800"
			>
				{#each cmsLocales as locale, index (index)}
					<div class="grid gap-3 py-4 md:grid-cols-[9rem_1fr_11rem_auto] md:items-end">
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Locale</span>
							<TextInput tone="admin" bind:value={locale.code} placeholder="fr-CA" />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Display name</span>
							<TextInput tone="admin" bind:value={locale.name} placeholder="French (Canada)" />
						</label>
						<label class="block text-sm">
							<span class="mb-1 block font-medium">Fallback</span>
							<Dropdown tone="admin" bind:value={locale.fallback_locale}>
								<option value={null}>No fallback</option>
								{#each cmsLocales.filter((candidate) => candidate !== locale && candidate.code) as candidate (candidate.code)}
									<option value={candidate.code}>{candidate.code}</option>
								{/each}
							</Dropdown>
						</label>
						<div class="flex flex-wrap items-center gap-4 pb-2 text-sm">
							<label class="flex items-center gap-2">
								<input
									class="size-4 accent-stone-900 dark:accent-stone-100"
									type="checkbox"
									bind:checked={locale.enabled}
								/>
								Enabled
							</label>
							<label class="flex items-center gap-2">
								<input
									class="size-4 accent-stone-900 dark:accent-stone-100"
									type="radio"
									name="default-cms-locale"
									checked={locale.is_default}
									onchange={() =>
										(cmsLocales = cmsLocales.map((item) => ({
											...item,
											is_default: item === locale,
										})))}
								/>
								Default
							</label>
							<IconButton
								tone="admin"
								variant="danger"
								outlined={true}
								size="sm"
								aria-label={`Remove ${locale.name || "language"}`}
								title="Remove language"
								disabled={locale.is_default}
								onclick={() =>
									(cmsLocales = cmsLocales.filter((_, itemIndex) => itemIndex !== index))}
							>
								<i class="bi bi-trash"></i>
							</IconButton>
						</div>
					</div>
				{/each}
			</div>
			<div class="mt-4 flex justify-end">
				<Button
					tone="admin"
					variant="primary"
					disabled={localeSaving}
					onclick={() => void saveLocales()}
				>
					<i class="bi bi-floppy mr-1"></i>{localeSaving ? "Saving..." : "Save languages"}
				</Button>
			</div>
		</AdminPanel>

		<AdminPanel title="Localized page variants" class="mt-6">
			{#if selected.id === null}
				<AdminEmptyState>Save the page before adding localized variants.</AdminEmptyState>
			{:else}
				<div class="grid gap-6 lg:grid-cols-[15rem_minmax(0,1fr)]">
					<div>
						<Button tone="admin" size="small" class="mb-3 w-full" onclick={newPageVariant}>
							<i class="bi bi-plus-lg mr-1"></i>New variant
						</Button>
						<div class="space-y-2">
							{#each pageVariants as variant (variant.id)}
								<AdminListItem
									as="button"
									active={variant.id === selectedVariantId}
									interactive={variant.id !== selectedVariantId}
									class="flex items-center justify-between gap-2 p-3"
									onclick={() => selectPageVariant(variant)}
								>
									<span class="min-w-0 text-left">
										<span class="block truncate text-sm font-medium"
											>{variant.locale}{variant.market ? ` / ${variant.market}` : ""}</span
										>
										<span class="block truncate text-xs text-stone-500">{variant.path}</span>
									</span>
									<Badge
										tone={variant.status === "published"
											? "success"
											: variant.status === "approved"
												? "neutral"
												: "warning"}
										size="sm">{variant.status.replace("_", " ")}</Badge
									>
								</AdminListItem>
							{/each}
						</div>
					</div>
					<div class="min-w-0">
						<div class="grid gap-4 md:grid-cols-2">
							<label class="block text-sm"
								><span class="mb-1 block font-medium">Language</span><Dropdown
									tone="admin"
									bind:value={variantLocale}
									><option value="">Choose language</option
									>{#each cmsLocales.filter((locale) => locale.enabled && !locale.is_default) as locale (locale.code)}<option
											value={locale.code}>{locale.name}</option
										>{/each}</Dropdown
								></label
							>
							<label class="block text-sm"
								><span class="mb-1 block font-medium">Market override</span><TextInput
									tone="admin"
									bind:value={variantMarket}
									placeholder="Optional, for example CA"
								/></label
							>
							<label class="block text-sm"
								><span class="mb-1 block font-medium">Title</span><TextInput
									tone="admin"
									bind:value={variantTitle}
								/></label
							>
							<label class="block text-sm"
								><span class="mb-1 block font-medium">Localized path</span><TextInput
									tone="admin"
									bind:value={variantPath}
									placeholder="/fr/livraison"
								/></label
							>
							<label class="block text-sm"
								><span class="mb-1 block font-medium">Slug</span><TextInput
									tone="admin"
									bind:value={variantSlug}
								/></label
							>
						</div>
						<div
							class="mt-5 flex flex-wrap items-center justify-between gap-3 border-t border-stone-200 pt-5 dark:border-stone-800"
						>
							<h3 class="text-sm font-semibold">Localized sections</h3>
							<div class="flex gap-2">
								<Dropdown tone="admin" full={false} bind:value={selectedPageSectionType}
									>{#each pageSectionOptions as option (option.id)}<option value={option.id}
											>{option.label}</option
										>{/each}</Dropdown
								>
								<Button
									tone="admin"
									size="small"
									onclick={() => addBlock("variant", selectedPageSectionType)}
									><i class="bi bi-plus-lg mr-1"></i>Add</Button
								>
							</div>
						</div>
						<div class="mt-4">{@render blockEditor(variantBlocks, "variant")}</div>
						<div
							class="mt-5 flex flex-wrap items-end gap-3 border-t border-stone-200 pt-5 dark:border-stone-800"
						>
							<label class="min-w-64 flex-1 text-sm"
								><span class="mb-1 block font-medium">Review note</span><TextInput
									tone="admin"
									bind:value={variantComment}
									placeholder="Context for the next reviewer"
								/></label
							>
							<Button
								tone="admin"
								variant="primary"
								disabled={variantSaving || !variantLocale || !variantTitle || !variantPath}
								onclick={() => void savePageVariant()}
								>{variantSaving
									? "Saving..."
									: selectedVariantId === null
										? "Create draft"
										: "Save draft"}</Button
							>
							{#if selectedPageVariant?.status === "draft" || selectedPageVariant?.status === "changes_requested"}<Button
									tone="admin"
									onclick={() => void transitionPageVariant("submit")}>Submit for review</Button
								>{/if}
							{#if selectedPageVariant?.status === "in_review"}<Button
									tone="admin"
									onclick={() => void transitionPageVariant("request_changes")}
									>Request changes</Button
								><Button
									tone="admin"
									variant="success"
									onclick={() => void transitionPageVariant("approve")}>Approve</Button
								>{/if}
							{#if selectedPageVariant?.status === "approved"}<Button
									tone="admin"
									variant="success"
									onclick={() => void transitionPageVariant("publish")}>Publish</Button
								>{/if}
							{#if selectedVariantId !== null}<IconButton
									tone="admin"
									variant="danger"
									outlined={true}
									aria-label="Delete localized variant"
									title="Delete localized variant"
									onclick={() => void removePageVariant()}><i class="bi bi-trash"></i></IconButton
								>{/if}
						</div>
					</div>
				</div>
			{/if}
		</AdminPanel>

		<AdminPanel title="Page sections" headerActions={pageSectionActions} class="mt-6">
			{#if pageBlocks.length === 0}
				<AdminEmptyState>Add a section to build this page.</AdminEmptyState>
			{:else}
				{@render blockEditor(pageBlocks, "page")}
			{/if}
		</AdminPanel>

		<AdminPanel title="Search and sharing" class="mt-6">
			{#if selected.id === null}
				<AdminEmptyState>Save the page before configuring SEO.</AdminEmptyState>
			{:else if seoLoading}
				<AdminEmptyState>Loading SEO settings...</AdminEmptyState>
			{:else}
				{#if seoIssues.length > 0}
					<div
						class="mb-6 space-y-1 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-100"
					>
						{#each seoIssues as issue (issue)}<p>{issue}</p>{/each}
					</div>
				{/if}
				<div class="grid gap-4 md:grid-cols-2">
					<label class="block text-sm"
						><span class="mb-1 block font-medium">Search title</span><TextInput
							tone="admin"
							bind:value={seoTitle}
						/><span class="mt-1 block text-xs text-stone-500">{seoTitle.length}/60</span></label
					>
					<label class="block text-sm"
						><span class="mb-1 block font-medium">Canonical URL</span><TextInput
							tone="admin"
							bind:value={seoCanonicalURL}
							placeholder={pagePath || "/page"}
						/></label
					>
					<label class="block text-sm md:col-span-2"
						><span class="mb-1 block font-medium">Meta description</span><TextArea
							tone="admin"
							class="min-h-24"
							bind:value={seoDescription}
						/><span class="mt-1 block text-xs text-stone-500">{seoDescription.length}/160</span
						></label
					>
					<label class="block text-sm"
						><span class="mb-1 block font-medium">Search engine access</span><Dropdown
							tone="admin"
							bind:value={seoRobots}
							><option value="index_follow">Index page and follow links</option><option
								value="noindex_follow">Hide page, follow links</option
							><option value="index_nofollow">Index page, ignore links</option><option
								value="noindex_nofollow">Hide page and ignore links</option
							></Dropdown
						></label
					>
				</div>

				<div class="mt-8 border-t border-stone-200 pt-6 dark:border-stone-800">
					<h3 class="text-sm font-semibold">Social sharing</h3>
					<div class="mt-4 grid gap-4 md:grid-cols-2">
						<label class="block text-sm"
							><span class="mb-1 block font-medium">Social title</span><TextInput
								tone="admin"
								bind:value={seoOGTitle}
							/></label
						>
						<label class="block text-sm"
							><span class="mb-1 block font-medium">Social description</span><TextInput
								tone="admin"
								bind:value={seoOGDescription}
							/></label
						>
						<div class="text-sm">
							<span class="mb-1 block font-medium">Social image</span><label
								class="flex min-h-20 cursor-pointer items-center justify-center rounded-lg border border-dashed border-stone-300 dark:border-stone-700"
								><input
									class="sr-only"
									type="file"
									accept="image/*"
									onchange={(event) => void uploadSEOMedia(event, "og")}
								/>{seoOGImageMediaID ? "Replace image" : "Upload image"}</label
							>{#if seoOGImageMediaID}<img
									class="mt-2 aspect-video w-full rounded-lg object-cover"
									src={cmsMediaURL(seoOGImageMediaID)}
									alt=""
								/>{/if}
						</div>
						<label class="block text-sm"
							><span class="mb-1 block font-medium">X card format</span><Dropdown
								tone="admin"
								bind:value={seoTwitterCard}
								><option value="summary">Compact</option><option value="summary_large_image"
									>Large image</option
								></Dropdown
							><span class="mt-3 mb-1 block font-medium">X title</span><TextInput
								tone="admin"
								bind:value={seoTwitterTitle}
							/><span class="mt-3 mb-1 block font-medium">X description</span><TextInput
								tone="admin"
								bind:value={seoTwitterDescription}
							/><span class="mt-3 mb-1 block font-medium">X image</span><span
								class="flex cursor-pointer items-center justify-center rounded-lg border border-dashed border-stone-300 px-3 py-2 dark:border-stone-700"
								><input
									class="sr-only"
									type="file"
									accept="image/*"
									onchange={(event) => void uploadSEOMedia(event, "twitter")}
								/>{seoTwitterImageMediaID ? "Replace image" : "Upload image"}</span
							></label
						>
					</div>
				</div>

				<div class="mt-8 border-t border-stone-200 pt-6 dark:border-stone-800">
					<h3 class="text-sm font-semibold">Structured data</h3>
					<div class="mt-4 grid gap-4 md:grid-cols-2">
						<label class="block text-sm"
							><span class="mb-1 block font-medium">Content type</span><Dropdown
								tone="admin"
								bind:value={seoJSONLDType}
								><option value="">None</option><option value="WebPage">Web page</option><option
									value="FAQPage">FAQ page</option
								><option value="BreadcrumbList">Breadcrumb list</option><option value="Organization"
									>Organization</option
								><option value="WebSite">Website</option><option value="Product">Product</option
								></Dropdown
							></label
						>
						{#if seoJSONLDType}<label class="block text-sm"
								><span class="mb-1 block font-medium">Name</span><TextInput
									tone="admin"
									bind:value={seoJSONLDName}
								/></label
							>{/if}
					</div>
				</div>
				<div class="mt-6 flex justify-end">
					<Button
						tone="admin"
						variant="primary"
						disabled={seoSaving}
						onclick={() => void savePageSEO()}
						><i class="bi bi-floppy mr-1"></i>{seoSaving ? "Saving..." : "Save SEO"}</Button
					>
				</div>
			{/if}
		</AdminPanel>

		<AdminPanel title="Delivery" class="mt-6">
			{#if selected.id === null}
				<AdminEmptyState>Save the page before configuring delivery.</AdminEmptyState>
			{:else if deliveryLoading}
				<AdminEmptyState>Loading delivery settings...</AdminEmptyState>
			{:else}
				<div class="space-y-8">
					<section aria-labelledby="delivery-schedule-heading">
						<div class="flex flex-wrap items-center justify-between gap-3">
							<div>
								<div class="flex items-center gap-2">
									<h3 id="delivery-schedule-heading" class="text-sm font-semibold">Schedule</h3>
									{#if scheduleStatus}
										<Badge
											tone={scheduleStatus === "active"
												? "success"
												: scheduleStatus === "cancelled"
													? "neutral"
													: "warning"}
											size="sm">{scheduleStatus}</Badge
										>
									{/if}
								</div>
								<p class="mt-1 text-sm text-stone-500 dark:text-stone-400">
									Publish this draft automatically at a specific time.
								</p>
							</div>
							<label class="flex items-center gap-2 text-sm font-medium">
								<input
									class="size-4 accent-stone-900 dark:accent-stone-100"
									type="checkbox"
									bind:checked={scheduleEnabled}
								/>
								Scheduled
							</label>
						</div>
						{#if scheduleEnabled}
							<div class="mt-4 grid gap-4 md:grid-cols-3">
								<label class="block text-sm">
									<span class="mb-1 block font-medium">Publish at</span>
									<TextInput tone="admin" type="datetime-local" bind:value={schedulePublishAt} />
								</label>
								<label class="block text-sm">
									<span class="mb-1 block font-medium">Timezone</span>
									<TextInput tone="admin" bind:value={scheduleTimezone} placeholder="UTC" />
								</label>
								<div class="text-sm">
									<label class="mb-2 flex items-center gap-2 font-medium">
										<input
											class="size-4 accent-stone-900 dark:accent-stone-100"
											type="checkbox"
											bind:checked={scheduleExpiryEnabled}
										/>
										Auto-expire
									</label>
									{#if scheduleExpiryEnabled}
										<TextInput
											aria-label="Unpublish at"
											tone="admin"
											type="datetime-local"
											bind:value={scheduleUnpublishAt}
										/>
									{/if}
								</div>
							</div>
						{/if}
						{#if scheduleLastTransitionAt}
							<p class="mt-3 text-xs text-stone-500 dark:text-stone-400">
								Last transition {new Date(scheduleLastTransitionAt).toLocaleString()}
							</p>
						{/if}
					</section>

					<section
						class="border-t border-stone-200 pt-8 dark:border-stone-800"
						aria-labelledby="delivery-audience-heading"
					>
						<div class="flex flex-wrap items-center justify-between gap-3">
							<div>
								<h3 id="delivery-audience-heading" class="text-sm font-semibold">Audiences</h3>
								<p class="mt-1 text-sm text-stone-500 dark:text-stone-400">
									Visitors matching any enabled audience can view this page.
								</p>
							</div>
							<Button tone="admin" size="small" onclick={addDeliveryRule}>
								<i class="bi bi-plus-lg mr-1"></i>Add audience
							</Button>
						</div>
						{#if deliveryRules.length === 0}
							<p class="mt-4 text-sm text-stone-500 dark:text-stone-400">
								Visible to every visitor.
							</p>
						{:else}
							<div
								class="mt-4 divide-y divide-stone-200 rounded-lg border border-stone-200 dark:divide-stone-800 dark:border-stone-800"
							>
								{#each deliveryRules as rule, index (rule.id)}
									<div class="p-4">
										<div class="mb-4 flex items-center justify-between gap-3">
											<label class="flex items-center gap-2 text-sm font-medium">
												<input
													class="size-4 accent-stone-900 dark:accent-stone-100"
													type="checkbox"
													bind:checked={rule.enabled}
												/>
												Audience {index + 1}
											</label>
											<IconButton
												tone="admin"
												variant="danger"
												outlined={true}
												size="sm"
												aria-label={`Remove audience ${index + 1}`}
												title="Remove audience"
												onclick={() =>
													(deliveryRules = deliveryRules.filter((item) => item.id !== rule.id))}
											>
												<i class="bi bi-trash"></i>
											</IconButton>
										</div>
										<div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
											<label class="block text-sm"
												><span class="mb-1 block font-medium">Countries or markets</span><TextInput
													tone="admin"
													bind:value={rule.markets}
													placeholder="US, CA"
												/></label
											>
											<label class="block text-sm"
												><span class="mb-1 block font-medium">Visitor type</span><Dropdown
													tone="admin"
													bind:value={rule.audience}
													><option value="all">Everyone</option><option value="guest">Guests</option
													><option value="authenticated">Signed-in customers</option></Dropdown
												></label
											>
											<label class="block text-sm"
												><span class="mb-1 block font-medium">Referring sites</span><TextInput
													tone="admin"
													bind:value={rule.referrers}
													placeholder="google.com"
												/></label
											>
											<label class="block text-sm"
												><span class="mb-1 block font-medium">Campaign sources</span><TextInput
													tone="admin"
													bind:value={rule.utmSources}
													placeholder="newsletter, social"
												/></label
											>
											<label class="block text-sm"
												><span class="mb-1 block font-medium">Customer segments</span><TextInput
													tone="admin"
													bind:value={rule.segments}
													placeholder="vip, wholesale"
												/></label
											>
											<fieldset class="text-sm">
												<legend class="mb-2 font-medium">Devices</legend>
												<div class="flex flex-wrap gap-3">
													{#each ["desktop", "mobile", "tablet"] as device (device)}<label
															class="flex items-center gap-2 capitalize"
															><input
																class="size-4 accent-stone-900 dark:accent-stone-100"
																type="checkbox"
																checked={rule.devices.includes(
																	device as "desktop" | "mobile" | "tablet"
																)}
																onchange={() =>
																	toggleRuleDevice(rule, device as "desktop" | "mobile" | "tablet")}
															/>{device}</label
														>{/each}
												</div>
											</fieldset>
										</div>
									</div>
								{/each}
							</div>
						{/if}
					</section>

					<section
						class="border-t border-stone-200 pt-8 dark:border-stone-800"
						aria-labelledby="delivery-experiment-heading"
					>
						<div class="flex flex-wrap items-center justify-between gap-3">
							<div>
								<h3 id="delivery-experiment-heading" class="text-sm font-semibold">Experiment</h3>
								<p class="mt-1 text-sm text-stone-500 dark:text-stone-400">
									Compare two saved page versions with sticky visitor assignment.
								</p>
							</div>
							<label class="flex items-center gap-2 text-sm font-medium"
								><input
									class="size-4 accent-stone-900 dark:accent-stone-100"
									type="checkbox"
									bind:checked={experimentEnabled}
								/>Enabled</label
							>
						</div>
						{#if experimentEnabled}
							<div class="mt-4 grid gap-4 md:grid-cols-2 lg:grid-cols-4">
								<label class="block text-sm lg:col-span-2"
									><span class="mb-1 block font-medium">Experiment name</span><TextInput
										tone="admin"
										bind:value={experimentName}
									/></label
								>
								<label class="block text-sm"
									><span class="mb-1 block font-medium">Status</span><Dropdown
										tone="admin"
										bind:value={experimentStatus}
										><option value="draft">Draft</option><option value="active">Active</option
										><option value="paused">Paused</option><option value="completed"
											>Completed</option
										></Dropdown
									></label
								>
								<label class="block text-sm"
									><span class="mb-1 block font-medium">Keep assignment by</span><Dropdown
										tone="admin"
										bind:value={experimentStickyKey}
										><option value="visitor">Visitor</option><option value="customer"
											>Customer account</option
										></Dropdown
									></label
								>
								<label class="block text-sm"
									><span class="mb-1 block font-medium">Starts at</span><TextInput
										tone="admin"
										type="datetime-local"
										bind:value={experimentStartsAt}
									/></label
								>
								<label class="block text-sm"
									><span class="mb-1 block font-medium">Ends at (optional)</span><TextInput
										tone="admin"
										type="datetime-local"
										bind:value={experimentEndsAt}
									/></label
								>
								<label class="block text-sm"
									><span class="mb-1 block font-medium">Control version</span><Dropdown
										tone="admin"
										bind:value={controlVersionId}
										><option value={null}>Select version</option
										>{#each pageVersionOptions as version (version.id)}<option value={version.id}
												>Version {version.version_number}{version.id ===
												selectedPage?.published_version?.id
													? " (published)"
													: " (draft)"}</option
											>{/each}</Dropdown
									></label
								>
								<label class="block text-sm"
									><span class="mb-1 block font-medium">Variant version</span><Dropdown
										tone="admin"
										bind:value={variantVersionId}
										><option value={null}>Select version</option
										>{#each pageVersionOptions as version (version.id)}<option value={version.id}
												>Version {version.version_number}{version.id ===
												selectedPage?.published_version?.id
													? " (published)"
													: " (draft)"}</option
											>{/each}</Dropdown
									></label
								>
								<label class="block text-sm"
									><span class="mb-1 block font-medium">Control traffic (%)</span><NumberInput
										tone="admin"
										min={1}
										max={99}
										bind:value={controlAllocation}
									/></label
								>
								<div class="text-sm">
									<span class="mb-1 block font-medium">Variant traffic</span>
									<div
										class="rounded-lg border border-stone-200 bg-stone-50 px-3 py-2 dark:border-stone-800 dark:bg-stone-900"
									>
										{100 - Number(controlAllocation || 0)}%
									</div>
								</div>
							</div>
						{/if}
					</section>

					{#if deliveryPublications.length > 0}
						<section
							class="border-t border-stone-200 pt-8 dark:border-stone-800"
							aria-labelledby="delivery-history-heading"
						>
							<h3 id="delivery-history-heading" class="text-sm font-semibold">
								Publication history
							</h3>
							<div
								class="mt-4 divide-y divide-stone-200 border-y border-stone-200 dark:divide-stone-800 dark:border-stone-800"
							>
								{#each deliveryPublications.slice(0, 5) as publication (publication.id)}
									<div class="flex flex-wrap items-center justify-between gap-2 py-3 text-sm">
										<div>
											<span class="font-medium">Published content</span>{#if publication.notes}<span
													class="ml-2 text-stone-500 dark:text-stone-400">{publication.notes}</span
												>{/if}
										</div>
										<time
											class="text-xs text-stone-500 dark:text-stone-400"
											datetime={publication.published_at}
											>{new Date(publication.published_at).toLocaleString()}</time
										>
									</div>
								{/each}
							</div>
						</section>
					{/if}

					<div class="flex justify-end border-t border-stone-200 pt-6 dark:border-stone-800">
						<Button
							tone="admin"
							variant="primary"
							onclick={() => void savePageDelivery()}
							disabled={deliverySaveDisabled}
						>
							<i class="bi bi-floppy mr-1"></i>{deliverySaving ? "Saving..." : "Save delivery"}
						</Button>
					</div>
				</div>
			{/if}
		</AdminPanel>

		<AdminPanel title="Audit trail" class="mt-6">
			{#if selected.id === null || cmsAuditEvents.length === 0}
				<AdminEmptyState>No recorded changes for this page yet.</AdminEmptyState>
			{:else}
				<div
					class="divide-y divide-stone-200 border-y border-stone-200 dark:divide-stone-800 dark:border-stone-800"
				>
					{#each cmsAuditEvents as event (event.id)}
						<div class="flex flex-wrap items-start justify-between gap-3 py-3 text-sm">
							<div class="min-w-0">
								<div class="font-medium">
									{event.action.replaceAll(".", " ").replaceAll("_", " ")}
								</div>
								<div class="truncate text-xs text-stone-500">
									{event.actor || "System"}{event.detail ? ` · ${event.detail}` : ""}
								</div>
							</div>
							<time class="text-xs text-stone-500" datetime={event.created_at}
								>{new Date(event.created_at).toLocaleString()}</time
							>
						</div>
					{/each}
				</div>
			{/if}
		</AdminPanel>
	{/if}

	<AdminPanel title="Backup and restore" class="mt-6">
		<div class="flex flex-wrap items-center justify-between gap-4">
			<p class="max-w-2xl text-sm text-stone-500 dark:text-stone-400">
				Export a portable CMS snapshot or replace CMS content with a previously exported backup.
			</p>
			<div class="flex flex-wrap gap-2">
				<Button tone="admin" variant="regular" onclick={() => void exportCMS()}>
					<i class="bi bi-download mr-1"></i>Export CMS backup
				</Button>
				<input
					bind:this={restoreInput}
					class="sr-only"
					type="file"
					accept="application/json,.json"
					onchange={(event) => void selectRestoreFile(event)}
				/>
				<Button tone="admin" variant="danger" onclick={() => restoreInput.click()}>
					<i class="bi bi-upload mr-1"></i>Restore from export
				</Button>
			</div>
		</div>
	</AdminPanel>
</div>

{#if restoreDialogOpen}
	<AdminConfirmDialog
		title="Restore CMS backup?"
		message={`Restoring ${pendingRestoreName} will replace all CMS content. ${restorePreviewSummary} This cannot be undone unless you have another backup.`}
		confirmLabel="Restore backup"
		busyLabel="Restoring..."
		busy={restoring}
		onConfirm={() => void confirmRestore()}
		onCancel={cancelRestore}
	/>
{/if}

{#if deleteTarget}
	<AdminConfirmDialog
		title={deleteTitle(deleteTarget)}
		message={deleteMessage(deleteTarget)}
		confirmLabel={deleteTarget.kind === "navigation_page" ? "Hide" : "Delete"}
		busyLabel={deleteTarget.kind === "navigation_page" ? "Hiding..." : "Deleting..."}
		busy={deleting}
		onConfirm={() => void confirmDelete()}
		onCancel={cancelDelete}
	/>
{/if}
