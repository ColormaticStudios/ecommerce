<script lang="ts">
	import { getContext, untrack } from "svelte";
	import type { API } from "$lib/api";
	import type { components } from "$lib/api/generated/openapi";
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminFloatingNotices from "$lib/admin/AdminFloatingNotices.svelte";
	import AdminPageHeader from "$lib/admin/AdminPageHeader.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import AdminProductVariantSelector from "$lib/admin/AdminProductVariantSelector.svelte";
	import AdminTable from "$lib/admin/table/Table.svelte";
	import AdminTableBody from "$lib/admin/table/TableBody.svelte";
	import AdminTableCell from "$lib/admin/table/TableCell.svelte";
	import AdminTableHead from "$lib/admin/table/TableHead.svelte";
	import AdminTableRow from "$lib/admin/table/TableRow.svelte";
	import { createAdminNotices } from "$lib/admin/state.svelte";
	import { searchAdminProducts } from "$lib/admin/productSearch";
	import {
		buildAction,
		buildCondition,
		buildScheduleInput,
		buildTargets,
		compactString,
		isoFromLocalDateTime,
		localDateTimeFromISO,
		parseOptionalPositiveInt,
		type PromotionLevelInput,
		type PromotionRuleInput,
	} from "$lib/admin/discounts";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import NumberInput from "$lib/components/NumberInput.svelte";
	import TextArea from "$lib/components/TextArea.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
	import { formatPrice } from "$lib/utils";
	import type { ProductModel } from "$lib/models";
	import type { PageData } from "./$types";

	interface Props {
		data: PageData;
	}

	type DiscountCampaign = components["schemas"]["DiscountCampaign"];
	type DiscountStatus = DiscountCampaign["status"];
	type ProductDiscountInput = components["schemas"]["ProductDiscountInput"];
	type PromotionInput = components["schemas"]["PromotionInput"];
	type PromotionTemplate = components["schemas"]["PromotionTemplate"];
	type PromotionEvaluationResponse = components["schemas"]["PromotionEvaluationResponse"];
	type PromotionEvaluationRequestLine = components["schemas"]["PromotionEvaluationRequestLine"];
	type DiscountLifecycleRunResponse = components["schemas"]["DiscountLifecycleRunResponse"];
	type DiscountReconciliationReport = components["schemas"]["DiscountReconciliationReport"];
	type DiscountEvaluationMetrics = components["schemas"]["DiscountEvaluationMetrics"];
	type PromotionAction = components["schemas"]["PromotionAction"];
	type PromotionTargetInput = components["schemas"]["PromotionTargetInput"];
	type TabId = "campaigns" | "product" | "promotion" | "templates" | "ops";

	let { data }: Props = $props();
	const api: API = getContext("api");
	const notices = createAdminNotices();
	const initialProducts = untrack(() => data.products);
	const initialCampaigns = untrack(() => data.campaigns);
	const initialTemplates = untrack(() => data.templates);
	const initialHistory = untrack(() => data.history.history);
	const initialAudit = untrack(() => data.audit.audit);
	const initialMetrics = untrack(() => data.metrics);

	let campaigns = $state<DiscountCampaign[]>(initialCampaigns);
	let templates = $state<PromotionTemplate[]>(initialTemplates);
	let history = $state(initialHistory);
	let audit = $state(initialAudit);
	let metrics = $state<DiscountEvaluationMetrics>(initialMetrics);
	let products = $state<ProductModel[]>(initialProducts);

	let tab = $state<TabId>("campaigns");
	let campaignStatusFilter = $state<DiscountStatus | "all">("all");
	let loadingCampaigns = $state(false);
	let saving = $state(false);
	let productSearch = $state("");
	let productLoading = $state(false);
	let selectedProductId = $state<number | null>(null);
	let selectedVariantId = $state<number | null>(null);
	let selectedProductIds = $state<number[]>([]);
	let editingCampaignId = $state<number | null>(null);

	let discountName = $state("");
	let discountMode = $state<ProductDiscountInput["discount_mode"]>("percent");
	let discountValue = $state("10");
	let discountStartsAt = $state(localDateTimeFromISO(new Date().toISOString()));
	let discountEndsAt = $state("");
	let discountPriority = $state("0");
	let discountStatus = $state<NonNullable<ProductDiscountInput["status"]>>("active");
	let discountExclusive = $state(false);
	let discountCouponCode = $state("");
	let discountCustomerSegment = $state("");
	let discountGlobalUsageCap = $state("");
	let discountPerCustomerUsageCap = $state("");
	let channelWeb = $state(true);
	let channelApp = $state(false);
	let channelAdmin = $state(false);

	let promotionName = $state("");
	let promotionStartsAt = $state(localDateTimeFromISO(new Date().toISOString()));
	let promotionEndsAt = $state("");
	let promotionPriority = $state("0");
	let promotionStatus = $state<NonNullable<PromotionInput["status"]>>("active");
	let promotionExclusive = $state(false);
	let promotionCouponCode = $state("");
	let promotionCustomerSegment = $state("");
	let promotionGlobalUsageCap = $state("");
	let promotionPerCustomerUsageCap = $state("");
	let promotionChannelsWeb = $state(true);
	let promotionChannelsApp = $state(false);
	let promotionChannelsAdmin = $state(false);
	let promotionRules = $state<PromotionRuleInput[]>([]);
	let promotionLevels = $state<PromotionLevelInput[]>([]);

	let conditionProductIds = $state("");
	let conditionVariantIds = $state("");
	let conditionCategoryIds = $state("");
	let conditionBrandIds = $state("");
	let conditionMinQuantity = $state("1");
	let conditionMinSubtotal = $state("");
	let actionMode = $state<PromotionAction["mode"]>("percent");
	let actionValue = $state("10");
	let actionTargetType = $state<NonNullable<PromotionAction["target_type"]>>("cart");
	let actionTargetIds = $state("");
	let actionProductIds = $state("");
	let actionVariantIds = $state("");
	let actionCategoryIds = $state("");
	let actionBrandIds = $state("");
	let actionSku = $state("");
	let ruleStackPolicy = $state<NonNullable<PromotionRuleInput["stack_policy"]>>("none");
	let ruleMaxApplications = $state("");

	let levelName = $state("");
	let levelPriority = $state("0");
	let levelTargetType = $state<PromotionTargetInput["target_type"]>("product");
	let levelTargetIds = $state("");
	let levelActionMode = $state<PromotionAction["mode"]>("percent");
	let levelActionValue = $state("10");
	let levelStackPolicy = $state<NonNullable<PromotionLevelInput["stack_policy"]>>("none");
	let levelMaxApplications = $state("");

	let previewLines = $state<PromotionEvaluationRequestLine[]>([]);
	let previewCouponCode = $state("");
	let previewChannel = $state<"web" | "app" | "admin">("web");
	let previewCustomerSegment = $state("");
	let previewLineQuantity = $state("1");
	let previewLineUnitPrice = $state("25");
	let previewResult = $state<PromotionEvaluationResponse | null>(null);
	let previewLoading = $state(false);

	let templateName = $state("");
	let templateDescription = $state("");
	let templateActive = $state(true);
	let selectedTemplateId = $state("");
	let templateOverrideName = $state("");
	let templateOverrideStartsAt = $state("");
	let templateOverrideEndsAt = $state("");
	let templateOverrideCouponCode = $state("");

	let scheduleCampaignId = $state("");
	let scheduleType = $state<"one_time" | "recurring">("one_time");
	let scheduleRecurrence = $state<"daily" | "weekly" | "monthly" | "">("");
	let scheduleWindowStart = $state(localDateTimeFromISO(new Date().toISOString()));
	let scheduleWindowEnd = $state("");
	let scheduleUntilAt = $state("");
	let scheduleTimezone = $state(Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC");
	let lifecycleReport = $state<DiscountLifecycleRunResponse | null>(null);
	let reconciliationReport = $state<DiscountReconciliationReport | null>(null);

	const tabOptions: Array<{ id: TabId; label: string }> = [
		{ id: "campaigns", label: "Campaigns" },
		{ id: "product", label: "Product discount" },
		{ id: "promotion", label: "Promotion" },
		{ id: "templates", label: "Templates" },
		{ id: "ops", label: "Operations" },
	];

	const activeCampaigns = $derived(campaigns.filter((campaign) => campaign.status === "active"));
	const scheduledCampaigns = $derived(
		campaigns.filter((campaign) => campaign.status === "scheduled")
	);
	const archivedCampaigns = $derived(
		campaigns.filter((campaign) => campaign.status === "archived")
	);
	const filteredCampaigns = $derived(
		campaignStatusFilter === "all"
			? campaigns
			: campaigns.filter((campaign) => campaign.status === campaignStatusFilter)
	);
	const selectedProduct = $derived(
		selectedProductId
			? (products.find((product) => product.id === selectedProductId) ?? null)
			: null
	);
	const selectedVariant = $derived(
		selectedProduct && selectedVariantId
			? (selectedProduct.variants.find((variant) => variant.id === selectedVariantId) ?? null)
			: null
	);

	function statusTone(
		status: DiscountStatus
	): "neutral" | "info" | "success" | "warning" | "danger" {
		switch (status) {
			case "active":
				return "success";
			case "scheduled":
				return "info";
			case "disabled":
				return "warning";
			case "archived":
				return "neutral";
		}
	}

	function formatDate(value: string | null | undefined): string {
		if (!value) {
			return "Open ended";
		}
		return new Intl.DateTimeFormat(undefined, {
			month: "short",
			day: "numeric",
			year: "numeric",
			hour: "numeric",
			minute: "2-digit",
		}).format(new Date(value));
	}

	function selectedChannels(prefix: "discount" | "promotion"): Array<"web" | "app" | "admin"> {
		const values: Array<"web" | "app" | "admin"> = [];
		if (prefix === "discount") {
			if (channelWeb) values.push("web");
			if (channelApp) values.push("app");
			if (channelAdmin) values.push("admin");
			return values;
		}
		if (promotionChannelsWeb) values.push("web");
		if (promotionChannelsApp) values.push("app");
		if (promotionChannelsAdmin) values.push("admin");
		return values;
	}

	async function refreshCampaigns(status: DiscountStatus | "all" = campaignStatusFilter) {
		loadingCampaigns = true;
		try {
			campaigns = await api.listAdminDiscountCampaigns(status === "all" ? {} : { status });
			campaignStatusFilter = status;
		} catch {
			notices.pushError("Unable to load campaigns.");
		} finally {
			loadingCampaigns = false;
		}
	}

	async function refreshDiagnostics() {
		try {
			const [nextHistory, nextAudit, nextMetrics] = await Promise.all([
				api.listAdminDiscountHistory(),
				api.listAdminDiscountAudit(),
				api.getAdminDiscountMetrics(),
			]);
			history = nextHistory.history;
			audit = nextAudit.audit;
			metrics = nextMetrics;
		} catch {
			notices.pushError("Unable to refresh discount diagnostics.");
		}
	}

	async function searchProducts() {
		productLoading = true;
		try {
			products = await searchAdminProducts(api, productSearch, 20);
		} catch {
			notices.pushError("Unable to search products.");
		} finally {
			productLoading = false;
		}
	}

	function addSelectedProduct() {
		if (!selectedProduct || selectedProductIds.includes(selectedProduct.id)) {
			return;
		}
		selectedProductIds = [...selectedProductIds, selectedProduct.id];
	}

	function removeProduct(productId: number) {
		selectedProductIds = selectedProductIds.filter((id) => id !== productId);
	}

	function productName(productId: number): string {
		return products.find((product) => product.id === productId)?.name ?? `Product #${productId}`;
	}

	function buildProductDiscountPayload(): ProductDiscountInput | null {
		const starts_at = isoFromLocalDateTime(discountStartsAt);
		if (!discountName.trim()) {
			notices.pushError("Campaign name is required.");
			return null;
		}
		if (selectedProductIds.length === 0) {
			notices.pushError("Select at least one product.");
			return null;
		}
		if (!starts_at) {
			notices.pushError("Start date is required.");
			return null;
		}
		const value = Number(discountValue);
		if (!Number.isFinite(value) || value <= 0) {
			notices.pushError("Discount value must be positive.");
			return null;
		}

		return {
			name: discountName.trim(),
			product_ids: selectedProductIds,
			discount_mode: discountMode,
			discount_value: value,
			starts_at,
			ends_at: isoFromLocalDateTime(discountEndsAt),
			priority: Number(discountPriority || 0),
			is_exclusive: discountExclusive,
			status: discountStatus,
			coupon_code: compactString(discountCouponCode) ?? null,
			channels: selectedChannels("discount"),
			customer_segment: compactString(discountCustomerSegment),
			global_usage_cap: parseOptionalPositiveInt(discountGlobalUsageCap),
			per_customer_usage_cap: parseOptionalPositiveInt(discountPerCustomerUsageCap),
		};
	}

	function editCampaign(campaign: DiscountCampaign) {
		if (campaign.type !== "product_discount") {
			notices.pushError("Promotion campaigns are edited by creating a replacement campaign.");
			return;
		}
		editingCampaignId = campaign.id;
		discountName = campaign.name;
		selectedProductIds = campaign.targets
			.filter((target) => target.target_type === "product")
			.map((target) => target.target_id);
		discountMode = campaign.discount_mode;
		discountValue = String(campaign.discount_value);
		discountStartsAt = localDateTimeFromISO(campaign.starts_at);
		discountEndsAt = localDateTimeFromISO(campaign.ends_at);
		discountPriority = String(campaign.priority);
		discountExclusive = campaign.is_exclusive;
		discountStatus = campaign.status === "disabled" ? "disabled" : "active";
		discountCouponCode = campaign.coupon_code ?? "";
		discountCustomerSegment = campaign.customer_segment ?? "";
		discountGlobalUsageCap = campaign.global_usage_cap ? String(campaign.global_usage_cap) : "";
		discountPerCustomerUsageCap = campaign.per_customer_usage_cap
			? String(campaign.per_customer_usage_cap)
			: "";
		channelWeb =
			(campaign.channels ?? []).includes("web") || (campaign.channels ?? []).length === 0;
		channelApp = (campaign.channels ?? []).includes("app");
		channelAdmin = (campaign.channels ?? []).includes("admin");
		tab = "product";
	}

	function resetDiscountForm() {
		editingCampaignId = null;
		discountName = "";
		selectedProductIds = [];
		discountMode = "percent";
		discountValue = "10";
		discountStartsAt = localDateTimeFromISO(new Date().toISOString());
		discountEndsAt = "";
		discountPriority = "0";
		discountStatus = "active";
		discountExclusive = false;
		discountCouponCode = "";
		discountCustomerSegment = "";
		discountGlobalUsageCap = "";
		discountPerCustomerUsageCap = "";
		channelWeb = true;
		channelApp = false;
		channelAdmin = false;
	}

	async function saveProductDiscount() {
		const payload = buildProductDiscountPayload();
		if (!payload) {
			return;
		}
		saving = true;
		try {
			const saved = editingCampaignId
				? await api.updateAdminDiscountCampaign(editingCampaignId, payload)
				: await api.createAdminDiscountCampaign(payload);
			campaigns = [saved, ...campaigns.filter((campaign) => campaign.id !== saved.id)];
			notices.pushSuccess(editingCampaignId ? "Discount updated." : "Discount created.");
			resetDiscountForm();
			tab = "campaigns";
		} catch (err) {
			const error = err as { body?: { error?: string } };
			notices.pushError(error.body?.error ?? "Unable to save product discount.");
		} finally {
			saving = false;
		}
	}

	function addRule() {
		const max = parseOptionalPositiveInt(ruleMaxApplications);
		promotionRules = [
			...promotionRules,
			{
				condition: buildCondition({
					productIds: conditionProductIds,
					variantIds: conditionVariantIds,
					categoryIds: conditionCategoryIds,
					brandIds: conditionBrandIds,
					minQuantity: conditionMinQuantity,
					minSubtotal: conditionMinSubtotal,
				}),
				action: buildAction({
					mode: actionMode,
					value: actionValue,
					targetType: actionTargetType,
					targetIds: actionTargetIds,
					productIds: actionProductIds,
					variantIds: actionVariantIds,
					categoryIds: actionCategoryIds,
					brandIds: actionBrandIds,
					sku: actionSku,
				}),
				stack_policy: ruleStackPolicy,
				max_applications_per_order: max,
			},
		];
	}

	function addLevel() {
		const targets = buildTargets(levelTargetType, levelTargetIds);
		if (!levelName.trim() || targets.length === 0) {
			notices.pushError("Level name and targets are required.");
			return;
		}
		promotionLevels = [
			...promotionLevels,
			{
				name: levelName.trim(),
				priority: Number(levelPriority || 0),
				action: buildAction({
					mode: levelActionMode,
					value: levelActionValue,
					targetType: levelTargetType,
					targetIds: levelTargetIds,
					productIds: "",
					variantIds: "",
					categoryIds: "",
					brandIds: "",
					sku: "",
				}),
				stack_policy: levelStackPolicy,
				max_applications_per_order: parseOptionalPositiveInt(levelMaxApplications),
				targets,
			},
		];
		levelName = "";
		levelTargetIds = "";
	}

	function buildPromotionPayload(): PromotionInput | null {
		const starts_at = isoFromLocalDateTime(promotionStartsAt);
		if (!promotionName.trim()) {
			notices.pushError("Promotion name is required.");
			return null;
		}
		if (!starts_at) {
			notices.pushError("Start date is required.");
			return null;
		}
		if (promotionRules.length === 0 && promotionLevels.length === 0) {
			notices.pushError("Add at least one rule or level.");
			return null;
		}

		return {
			name: promotionName.trim(),
			starts_at,
			ends_at: isoFromLocalDateTime(promotionEndsAt),
			priority: Number(promotionPriority || 0),
			is_exclusive: promotionExclusive,
			status: promotionStatus,
			coupon_code: compactString(promotionCouponCode) ?? null,
			channels: selectedChannels("promotion"),
			customer_segment: compactString(promotionCustomerSegment),
			global_usage_cap: parseOptionalPositiveInt(promotionGlobalUsageCap),
			per_customer_usage_cap: parseOptionalPositiveInt(promotionPerCustomerUsageCap),
			rules: promotionRules,
			levels: promotionLevels,
		};
	}

	async function createPromotion() {
		const payload = buildPromotionPayload();
		if (!payload) {
			return;
		}
		saving = true;
		try {
			const created = await api.createAdminPromotionCampaign(payload);
			campaigns = [created, ...campaigns];
			notices.pushSuccess("Promotion created.");
			tab = "campaigns";
		} catch (err) {
			const error = err as { body?: { error?: string } };
			notices.pushError(error.body?.error ?? "Unable to create promotion.");
		} finally {
			saving = false;
		}
	}

	function addPreviewLine() {
		if (!selectedProduct || !selectedVariant?.id) {
			notices.pushError("Select a product variant.");
			return;
		}
		const quantity = Number(previewLineQuantity);
		const unit_price = Number(previewLineUnitPrice);
		if (
			!Number.isInteger(quantity) ||
			quantity < 1 ||
			!Number.isFinite(unit_price) ||
			unit_price < 0
		) {
			notices.pushError("Preview quantity and unit price must be valid.");
			return;
		}
		previewLines = [
			...previewLines,
			{
				product_id: selectedProduct.id,
				product_variant_id: selectedVariant.id,
				brand_id: selectedProduct.brand?.id ?? null,
				category_ids: selectedProduct.categories.map((category) => category.id),
				sku: selectedVariant.sku,
				quantity,
				unit_price,
			},
		];
	}

	async function previewPromotion() {
		if (previewLines.length === 0) {
			notices.pushError("Add at least one preview line.");
			return;
		}
		previewLoading = true;
		try {
			previewResult = await api.previewAdminPromotion({
				coupon_code: compactString(previewCouponCode),
				channel: previewChannel,
				customer_segment: compactString(previewCustomerSegment),
				lines: previewLines,
			});
		} catch (err) {
			const error = err as { body?: { error?: string } };
			notices.pushError(error.body?.error ?? "Unable to preview promotion.");
		} finally {
			previewLoading = false;
		}
	}

	async function createTemplate() {
		const payload = buildPromotionPayload();
		if (!payload) {
			return;
		}
		if (!templateName.trim()) {
			notices.pushError("Template name is required.");
			return;
		}
		saving = true;
		try {
			const created = await api.createAdminPromotionTemplate({
				name: templateName.trim(),
				description: templateDescription.trim(),
				template: payload,
				is_active: templateActive,
			});
			templates = [created, ...templates];
			notices.pushSuccess("Template created.");
		} catch (err) {
			const error = err as { body?: { error?: string } };
			notices.pushError(error.body?.error ?? "Unable to create template.");
		} finally {
			saving = false;
		}
	}

	async function instantiateTemplate() {
		const templateId = Number(selectedTemplateId);
		if (!templateId) {
			notices.pushError("Select a template.");
			return;
		}
		saving = true;
		try {
			const created = await api.instantiateAdminPromotionTemplate(templateId, {
				name: compactString(templateOverrideName) ?? null,
				starts_at: isoFromLocalDateTime(templateOverrideStartsAt),
				ends_at: isoFromLocalDateTime(templateOverrideEndsAt),
				coupon_code: compactString(templateOverrideCouponCode) ?? null,
			});
			campaigns = [created, ...campaigns];
			notices.pushSuccess("Campaign created from template.");
			tab = "campaigns";
		} catch (err) {
			const error = err as { body?: { error?: string } };
			notices.pushError(error.body?.error ?? "Unable to instantiate template.");
		} finally {
			saving = false;
		}
	}

	async function disableCampaign(campaign: DiscountCampaign) {
		saving = true;
		try {
			const updated = await api.disableAdminDiscountCampaign(campaign.id);
			campaigns = campaigns.map((item) => (item.id === updated.id ? updated : item));
			notices.pushSuccess("Campaign disabled.");
		} catch {
			notices.pushError("Unable to disable campaign.");
		} finally {
			saving = false;
		}
	}

	async function archiveCampaign(campaign: DiscountCampaign) {
		saving = true;
		try {
			const updated = await api.archiveAdminDiscountCampaign(campaign.id);
			campaigns = campaigns.map((item) => (item.id === updated.id ? updated : item));
			notices.pushSuccess("Campaign archived.");
		} catch {
			notices.pushError("Unable to archive campaign.");
		} finally {
			saving = false;
		}
	}

	async function saveSchedule() {
		const campaignId = Number(scheduleCampaignId);
		const payload = buildScheduleInput({
			scheduleType,
			recurrence: scheduleRecurrence,
			windowStart: scheduleWindowStart,
			windowEnd: scheduleWindowEnd,
			untilAt: scheduleUntilAt,
			timezone: scheduleTimezone,
		});
		if (!campaignId || !payload) {
			notices.pushError("Select a campaign and complete the schedule window.");
			return;
		}
		saving = true;
		try {
			await api.scheduleAdminDiscountCampaign(campaignId, payload);
			notices.pushSuccess("Schedule saved.");
		} catch (err) {
			const error = err as { body?: { error?: string } };
			notices.pushError(error.body?.error ?? "Unable to save schedule.");
		} finally {
			saving = false;
		}
	}

	async function runLifecycle() {
		saving = true;
		try {
			lifecycleReport = await api.runAdminDiscountLifecycle();
			await refreshCampaigns("all");
			await refreshDiagnostics();
			notices.pushSuccess("Lifecycle run completed.");
		} catch {
			notices.pushError("Unable to run lifecycle.");
		} finally {
			saving = false;
		}
	}

	async function runReconciliation() {
		saving = true;
		try {
			reconciliationReport = await api.runAdminDiscountReconciliation();
			notices.pushSuccess("Reconciliation completed.");
		} catch {
			notices.pushError("Unable to run reconciliation.");
		} finally {
			saving = false;
		}
	}

	$effect(() => {
		campaigns = data.campaigns;
		templates = data.templates;
		history = data.history.history;
		audit = data.audit.audit;
		metrics = data.metrics;
		products = data.products;
	});
</script>

<svelte:head>
	<title>Discounts | Admin</title>
</svelte:head>

{#snippet statCard(label: string, value: string | number, icon: string)}
	<div
		class="rounded-lg border border-stone-200 bg-white p-5 shadow-sm dark:border-stone-800 dark:bg-stone-950"
	>
		<div class="flex items-center justify-between gap-3">
			<div>
				<p class="text-sm text-stone-500 dark:text-stone-400">{label}</p>
				<p class="mt-1 text-2xl font-semibold text-stone-950 dark:text-stone-50">{value}</p>
			</div>
			<i class={`bi ${icon} text-2xl text-stone-400`}></i>
		</div>
	</div>
{/snippet}

{#snippet fieldLabel(text: string, forId: string)}
	<label class="block text-sm font-medium text-stone-700 dark:text-stone-200" for={forId}>
		{text}
	</label>
{/snippet}

<div class="space-y-6">
	<AdminPageHeader title="Discounts" />
	{#each data.errorMessages as message (message)}
		<div
			class="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-200"
		>
			{message}
		</div>
	{/each}

	<div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
		{@render statCard("Active", activeCampaigns.length, "bi-lightning-charge")}
		{@render statCard("Scheduled", scheduledCampaigns.length, "bi-calendar-event")}
		{@render statCard("Archived", archivedCampaigns.length, "bi-archive")}
		{@render statCard("Matched evaluations", metrics.matched_evaluations, "bi-activity")}
	</div>

	<div class="flex flex-wrap gap-2">
		{#each tabOptions as item (item.id)}
			<Button
				tone="admin"
				variant={tab === item.id ? "primary" : "regular"}
				type="button"
				onclick={() => (tab = item.id)}
			>
				{item.id === "product" && editingCampaignId ? "Edit discount" : item.label}
			</Button>
		{/each}
	</div>

	{#if tab === "campaigns"}
		<AdminPanel title="Campaigns" meta={`${filteredCampaigns.length} shown`}>
			{#snippet headerActions()}
				<Dropdown tone="admin" full={false} bind:value={campaignStatusFilter}>
					<option value="all">All statuses</option>
					<option value="active">Active</option>
					<option value="scheduled">Scheduled</option>
					<option value="disabled">Disabled</option>
					<option value="archived">Archived</option>
				</Dropdown>
				<Button tone="admin" disabled={loadingCampaigns} onclick={() => refreshCampaigns()}>
					<i class="bi bi-arrow-clockwise"></i>
					Refresh
				</Button>
			{/snippet}
			{#if campaigns.length === 0}
				<AdminEmptyState>No discount campaigns yet.</AdminEmptyState>
			{:else}
				<AdminTable>
					<AdminTableHead>
						<tr>
							<AdminTableCell header>Name</AdminTableCell>
							<AdminTableCell header>Status</AdminTableCell>
							<AdminTableCell header>Type</AdminTableCell>
							<AdminTableCell header>Value</AdminTableCell>
							<AdminTableCell header>Window</AdminTableCell>
							<AdminTableCell header align="right">Actions</AdminTableCell>
						</tr>
					</AdminTableHead>
					<AdminTableBody>
						{#each filteredCampaigns as campaign (campaign.id)}
							<AdminTableRow>
								<AdminTableCell strong>
									<div>
										<p>{campaign.name}</p>
										<p class="text-xs text-stone-500 dark:text-stone-400">
											#{campaign.id} · priority {campaign.priority}
											{campaign.is_exclusive ? " · exclusive" : ""}
										</p>
									</div>
								</AdminTableCell>
								<AdminTableCell>
									<Badge tone={statusTone(campaign.status)}>{campaign.status}</Badge>
								</AdminTableCell>
								<AdminTableCell>{campaign.type.replace("_", " ")}</AdminTableCell>
								<AdminTableCell>
									{campaign.discount_mode === "percent"
										? `${campaign.discount_value}%`
										: formatPrice(campaign.discount_value)}
								</AdminTableCell>
								<AdminTableCell>
									<div class="text-xs">
										<p>{formatDate(campaign.starts_at)}</p>
										<p class="text-stone-500 dark:text-stone-400">
											to {formatDate(campaign.ends_at)}
										</p>
									</div>
								</AdminTableCell>
								<AdminTableCell align="right">
									<div class="flex justify-end gap-2">
										{#if campaign.type === "product_discount"}
											<Button tone="admin" size="small" onclick={() => editCampaign(campaign)}>
												Edit
											</Button>
										{/if}
										<Button
											tone="admin"
											size="small"
											disabled={campaign.status === "disabled" ||
												campaign.status === "archived" ||
												saving}
											onclick={() => disableCampaign(campaign)}
										>
											Disable
										</Button>
										<Button
											tone="admin"
											size="small"
											disabled={campaign.status === "archived" || saving}
											onclick={() => archiveCampaign(campaign)}
										>
											Archive
										</Button>
									</div>
								</AdminTableCell>
							</AdminTableRow>
						{/each}
					</AdminTableBody>
				</AdminTable>
			{/if}
		</AdminPanel>
	{/if}

	{#if tab === "product"}
		<AdminPanel title={editingCampaignId ? "Edit product discount" : "New product discount"}>
			<div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_22rem]">
				<div class="space-y-4">
					<div class="grid gap-4 md:grid-cols-2">
						<div>
							{@render fieldLabel("Name", "discount-name")}
							<TextInput id="discount-name" tone="admin" bind:value={discountName} />
						</div>
						<div>
							{@render fieldLabel("Status", "discount-status")}
							<Dropdown id="discount-status" tone="admin" bind:value={discountStatus}>
								<option value="active">Active</option>
								<option value="disabled">Disabled</option>
							</Dropdown>
						</div>
						<div>
							{@render fieldLabel("Mode", "discount-mode")}
							<Dropdown id="discount-mode" tone="admin" bind:value={discountMode}>
								<option value="percent">Percent</option>
								<option value="fixed">Fixed amount</option>
							</Dropdown>
						</div>
						<div>
							{@render fieldLabel("Value", "discount-value")}
							<NumberInput
								id="discount-value"
								tone="admin"
								allowDecimal
								bind:value={discountValue}
							/>
						</div>
						<div>
							{@render fieldLabel("Starts", "discount-starts")}
							<TextInput
								id="discount-starts"
								tone="admin"
								type="datetime-local"
								bind:value={discountStartsAt}
							/>
						</div>
						<div>
							{@render fieldLabel("Ends", "discount-ends")}
							<TextInput
								id="discount-ends"
								tone="admin"
								type="datetime-local"
								bind:value={discountEndsAt}
							/>
						</div>
					</div>

					<AdminProductVariantSelector
						{products}
						bind:searchQuery={productSearch}
						bind:selectedProductId
						bind:selectedVariantId
						loading={productLoading}
						onSearch={searchProducts}
					>
						{#snippet toolbarActions()}
							<Button tone="admin" type="button" onclick={addSelectedProduct}>
								<i class="bi bi-plus-lg"></i>
								Add product
							</Button>
						{/snippet}
					</AdminProductVariantSelector>

					<div class="flex flex-wrap gap-2">
						{#each selectedProductIds as productId (productId)}
							<button
								type="button"
								class="rounded-full border border-stone-300 bg-white px-3 py-1 text-sm text-stone-700 dark:border-stone-700 dark:bg-stone-950 dark:text-stone-200"
								onclick={() => removeProduct(productId)}
							>
								{productName(productId)} <i class="bi bi-x"></i>
							</button>
						{/each}
					</div>
				</div>

				<div class="space-y-4 rounded-lg border border-stone-200 p-4 dark:border-stone-800">
					<div>
						{@render fieldLabel("Priority", "discount-priority")}
						<NumberInput id="discount-priority" tone="admin" bind:value={discountPriority} />
					</div>
					<label class="flex items-center gap-2 text-sm text-stone-700 dark:text-stone-200">
						<input type="checkbox" bind:checked={discountExclusive} />
						Exclusive
					</label>
					<div>
						{@render fieldLabel("Coupon code", "discount-coupon")}
						<TextInput id="discount-coupon" tone="admin" bind:value={discountCouponCode} />
					</div>
					<div>
						{@render fieldLabel("Customer segment", "discount-segment")}
						<TextInput id="discount-segment" tone="admin" bind:value={discountCustomerSegment} />
					</div>
					<div class="grid grid-cols-2 gap-3">
						<div>
							{@render fieldLabel("Global cap", "discount-global-cap")}
							<NumberInput
								id="discount-global-cap"
								tone="admin"
								bind:value={discountGlobalUsageCap}
							/>
						</div>
						<div>
							{@render fieldLabel("Customer cap", "discount-customer-cap")}
							<NumberInput
								id="discount-customer-cap"
								tone="admin"
								bind:value={discountPerCustomerUsageCap}
							/>
						</div>
					</div>
					<div class="space-y-2 text-sm text-stone-700 dark:text-stone-200">
						<p class="font-medium">Channels</p>
						<label class="flex items-center gap-2"
							><input type="checkbox" bind:checked={channelWeb} /> Web</label
						>
						<label class="flex items-center gap-2"
							><input type="checkbox" bind:checked={channelApp} /> App</label
						>
						<label class="flex items-center gap-2"
							><input type="checkbox" bind:checked={channelAdmin} /> Admin</label
						>
					</div>
					<div class="flex gap-2">
						<Button tone="admin" variant="primary" disabled={saving} onclick={saveProductDiscount}>
							{editingCampaignId ? "Save changes" : "Create discount"}
						</Button>
						{#if editingCampaignId}
							<Button tone="admin" onclick={resetDiscountForm}>Cancel</Button>
						{/if}
					</div>
				</div>
			</div>
		</AdminPanel>
	{/if}

	{#if tab === "promotion"}
		<div class="grid gap-6 xl:grid-cols-[minmax(0,1.1fr)_minmax(22rem,0.9fr)]">
			<AdminPanel title="Promotion campaign">
				<div class="space-y-5">
					<div class="grid gap-4 md:grid-cols-2">
						<div>
							{@render fieldLabel("Name", "promotion-name")}
							<TextInput id="promotion-name" tone="admin" bind:value={promotionName} />
						</div>
						<div>
							{@render fieldLabel("Status", "promotion-status")}
							<Dropdown id="promotion-status" tone="admin" bind:value={promotionStatus}>
								<option value="active">Active</option>
								<option value="scheduled">Scheduled</option>
								<option value="disabled">Disabled</option>
							</Dropdown>
						</div>
						<div>
							{@render fieldLabel("Starts", "promotion-starts")}
							<TextInput
								id="promotion-starts"
								tone="admin"
								type="datetime-local"
								bind:value={promotionStartsAt}
							/>
						</div>
						<div>
							{@render fieldLabel("Ends", "promotion-ends")}
							<TextInput
								id="promotion-ends"
								tone="admin"
								type="datetime-local"
								bind:value={promotionEndsAt}
							/>
						</div>
						<div>
							{@render fieldLabel("Priority", "promotion-priority")}
							<NumberInput id="promotion-priority" tone="admin" bind:value={promotionPriority} />
						</div>
						<div>
							{@render fieldLabel("Coupon code", "promotion-coupon")}
							<TextInput id="promotion-coupon" tone="admin" bind:value={promotionCouponCode} />
						</div>
					</div>
					<div class="grid gap-4 md:grid-cols-2">
						<div>
							{@render fieldLabel("Customer segment", "promotion-segment")}
							<TextInput
								id="promotion-segment"
								tone="admin"
								bind:value={promotionCustomerSegment}
							/>
						</div>
						<div class="grid grid-cols-2 gap-3">
							<div>
								{@render fieldLabel("Global cap", "promotion-global-cap")}
								<NumberInput
									id="promotion-global-cap"
									tone="admin"
									bind:value={promotionGlobalUsageCap}
								/>
							</div>
							<div>
								{@render fieldLabel("Customer cap", "promotion-customer-cap")}
								<NumberInput
									id="promotion-customer-cap"
									tone="admin"
									bind:value={promotionPerCustomerUsageCap}
								/>
							</div>
						</div>
					</div>
					<div class="flex flex-wrap gap-4 text-sm text-stone-700 dark:text-stone-200">
						<label class="flex items-center gap-2"
							><input type="checkbox" bind:checked={promotionExclusive} /> Exclusive</label
						>
						<label class="flex items-center gap-2"
							><input type="checkbox" bind:checked={promotionChannelsWeb} /> Web</label
						>
						<label class="flex items-center gap-2"
							><input type="checkbox" bind:checked={promotionChannelsApp} /> App</label
						>
						<label class="flex items-center gap-2"
							><input type="checkbox" bind:checked={promotionChannelsAdmin} /> Admin</label
						>
					</div>

					<div class="rounded-lg border border-stone-200 p-4 dark:border-stone-800">
						<h3 class="font-semibold text-stone-950 dark:text-stone-50">Rule</h3>
						<div class="mt-3 grid gap-3 md:grid-cols-3">
							<TextInput tone="admin" placeholder="Product ids" bind:value={conditionProductIds} />
							<TextInput tone="admin" placeholder="Variant ids" bind:value={conditionVariantIds} />
							<TextInput
								tone="admin"
								placeholder="Category ids"
								bind:value={conditionCategoryIds}
							/>
							<TextInput tone="admin" placeholder="Brand ids" bind:value={conditionBrandIds} />
							<NumberInput
								tone="admin"
								placeholder="Min quantity"
								bind:value={conditionMinQuantity}
							/>
							<NumberInput
								tone="admin"
								allowDecimal
								placeholder="Min subtotal"
								bind:value={conditionMinSubtotal}
							/>
						</div>
						<div class="mt-3 grid gap-3 md:grid-cols-3">
							<Dropdown tone="admin" bind:value={actionMode}>
								<option value="percent">Percent off</option>
								<option value="fixed">Fixed off</option>
								<option value="fixed_price">Fixed price</option>
								<option value="free_item">Free item</option>
							</Dropdown>
							<NumberInput
								tone="admin"
								allowDecimal
								placeholder="Action value"
								bind:value={actionValue}
							/>
							<Dropdown tone="admin" bind:value={actionTargetType}>
								<option value="cart">Cart</option>
								<option value="product">Product</option>
								<option value="variant">Variant</option>
								<option value="category">Category</option>
								<option value="brand">Brand</option>
							</Dropdown>
							<TextInput tone="admin" placeholder="Target ids" bind:value={actionTargetIds} />
							<TextInput
								tone="admin"
								placeholder="Action product ids"
								bind:value={actionProductIds}
							/>
							<TextInput
								tone="admin"
								placeholder="Action variant ids"
								bind:value={actionVariantIds}
							/>
							<TextInput
								tone="admin"
								placeholder="Action category ids"
								bind:value={actionCategoryIds}
							/>
							<TextInput tone="admin" placeholder="Action brand ids" bind:value={actionBrandIds} />
							<TextInput tone="admin" placeholder="Free item SKU" bind:value={actionSku} />
							<Dropdown tone="admin" bind:value={ruleStackPolicy}>
								<option value="none">No stacking</option>
								<option value="additive">Additive</option>
							</Dropdown>
							<NumberInput
								tone="admin"
								placeholder="Max per order"
								bind:value={ruleMaxApplications}
							/>
						</div>
						<Button tone="admin" class="mt-3" onclick={addRule}>Add rule</Button>
					</div>

					<div class="rounded-lg border border-stone-200 p-4 dark:border-stone-800">
						<h3 class="font-semibold text-stone-950 dark:text-stone-50">Level</h3>
						<div class="mt-3 grid gap-3 md:grid-cols-3">
							<TextInput tone="admin" placeholder="Level name" bind:value={levelName} />
							<NumberInput tone="admin" placeholder="Priority" bind:value={levelPriority} />
							<Dropdown tone="admin" bind:value={levelTargetType}>
								<option value="product">Product</option>
								<option value="variant">Variant</option>
								<option value="category">Category</option>
								<option value="brand">Brand</option>
							</Dropdown>
							<TextInput tone="admin" placeholder="Target ids" bind:value={levelTargetIds} />
							<Dropdown tone="admin" bind:value={levelActionMode}>
								<option value="percent">Percent off</option>
								<option value="fixed">Fixed off</option>
								<option value="fixed_price">Fixed price</option>
								<option value="free_item">Free item</option>
							</Dropdown>
							<NumberInput
								tone="admin"
								allowDecimal
								placeholder="Value"
								bind:value={levelActionValue}
							/>
							<Dropdown tone="admin" bind:value={levelStackPolicy}>
								<option value="none">No stacking</option>
								<option value="additive">Additive</option>
							</Dropdown>
							<NumberInput
								tone="admin"
								placeholder="Max per order"
								bind:value={levelMaxApplications}
							/>
						</div>
						<Button tone="admin" class="mt-3" onclick={addLevel}>Add level</Button>
					</div>

					<div class="flex flex-wrap gap-2">
						<Badge tone="info">{promotionRules.length} rules</Badge>
						<Badge tone="info">{promotionLevels.length} levels</Badge>
						<Button tone="admin" variant="primary" disabled={saving} onclick={createPromotion}>
							Create promotion
						</Button>
					</div>
				</div>
			</AdminPanel>

			<AdminPanel title="Preview">
				<div class="space-y-4">
					<AdminProductVariantSelector
						{products}
						bind:searchQuery={productSearch}
						bind:selectedProductId
						bind:selectedVariantId
						loading={productLoading}
						onSearch={searchProducts}
					/>
					<div class="grid grid-cols-2 gap-3">
						<NumberInput tone="admin" placeholder="Quantity" bind:value={previewLineQuantity} />
						<NumberInput
							tone="admin"
							allowDecimal
							placeholder="Unit price"
							bind:value={previewLineUnitPrice}
						/>
					</div>
					<Button tone="admin" onclick={addPreviewLine}>Add preview line</Button>
					{#if previewLines.length}
						<div class="space-y-2 text-sm text-stone-700 dark:text-stone-200">
							{#each previewLines as line, index (index)}
								<div
									class="flex items-center justify-between rounded-lg border border-stone-200 px-3 py-2 dark:border-stone-800"
								>
									<span
										>Product #{line.product_id} · Variant #{line.product_variant_id} · Qty {line.quantity}</span
									>
									<button
										type="button"
										aria-label="Remove preview line"
										title="Remove preview line"
										onclick={() => (previewLines = previewLines.filter((_, i) => i !== index))}
									>
										<i class="bi bi-x-lg"></i>
									</button>
								</div>
							{/each}
						</div>
					{/if}
					<div class="grid gap-3 md:grid-cols-2">
						<TextInput tone="admin" placeholder="Coupon" bind:value={previewCouponCode} />
						<Dropdown tone="admin" bind:value={previewChannel}>
							<option value="web">Web</option>
							<option value="app">App</option>
							<option value="admin">Admin</option>
						</Dropdown>
						<TextInput
							tone="admin"
							placeholder="Customer segment"
							bind:value={previewCustomerSegment}
						/>
					</div>
					<Button
						tone="admin"
						variant="primary"
						disabled={previewLoading}
						onclick={previewPromotion}
					>
						{previewLoading ? "Previewing..." : "Preview"}
					</Button>
					{#if previewResult}
						<div class="rounded-lg border border-stone-200 p-4 text-sm dark:border-stone-800">
							<div class="flex justify-between">
								<span>Subtotal</span><span>{formatPrice(previewResult.subtotal)}</span>
							</div>
							<div class="flex justify-between text-emerald-700 dark:text-emerald-300">
								<span>Discount</span><span>-{formatPrice(previewResult.discount_total)}</span>
							</div>
							<div class="flex justify-between font-semibold">
								<span>Final</span><span>{formatPrice(previewResult.final_subtotal)}</span>
							</div>
						</div>
					{/if}
				</div>
			</AdminPanel>
		</div>
	{/if}

	{#if tab === "templates"}
		<div class="grid gap-6 lg:grid-cols-2">
			<AdminPanel title="Templates" meta={`${templates.length} saved`}>
				{#if templates.length === 0}
					<AdminEmptyState>No promotion templates yet.</AdminEmptyState>
				{:else}
					<div class="space-y-3">
						{#each templates as template (template.id)}
							<div class="rounded-lg border border-stone-200 p-4 dark:border-stone-800">
								<div class="flex items-start justify-between gap-3">
									<div>
										<p class="font-semibold text-stone-950 dark:text-stone-50">{template.name}</p>
										<p class="text-sm text-stone-500 dark:text-stone-400">{template.description}</p>
									</div>
									<Badge tone={template.is_active ? "success" : "neutral"}>
										{template.is_active ? "active" : "inactive"}
									</Badge>
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</AdminPanel>
			<AdminPanel title="Create or instantiate">
				<div class="space-y-4">
					<div>
						{@render fieldLabel("Template name", "template-name")}
						<TextInput id="template-name" tone="admin" bind:value={templateName} />
					</div>
					<TextArea tone="admin" placeholder="Description" bind:value={templateDescription} />
					<label class="flex items-center gap-2 text-sm text-stone-700 dark:text-stone-200">
						<input type="checkbox" bind:checked={templateActive} />
						Active
					</label>
					<Button tone="admin" variant="primary" disabled={saving} onclick={createTemplate}>
						Save current promotion as template
					</Button>
					<div class="border-t border-stone-200 pt-4 dark:border-stone-800">
						<Dropdown tone="admin" bind:value={selectedTemplateId}>
							<option value="">Select template</option>
							{#each templates as template (template.id)}
								<option value={String(template.id)}>{template.name}</option>
							{/each}
						</Dropdown>
						<div class="mt-3 grid gap-3 md:grid-cols-2">
							<TextInput
								tone="admin"
								placeholder="Campaign name"
								bind:value={templateOverrideName}
							/>
							<TextInput
								tone="admin"
								placeholder="Coupon"
								bind:value={templateOverrideCouponCode}
							/>
							<TextInput tone="admin" type="datetime-local" bind:value={templateOverrideStartsAt} />
							<TextInput tone="admin" type="datetime-local" bind:value={templateOverrideEndsAt} />
						</div>
						<Button tone="admin" class="mt-3" disabled={saving} onclick={instantiateTemplate}>
							Create from template
						</Button>
					</div>
				</div>
			</AdminPanel>
		</div>
	{/if}

	{#if tab === "ops"}
		<div class="grid gap-6 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
			<div class="space-y-6">
				<AdminPanel title="Schedule">
					<div class="space-y-3">
						<Dropdown tone="admin" bind:value={scheduleCampaignId}>
							<option value="">Select campaign</option>
							{#each campaigns as campaign (campaign.id)}
								<option value={String(campaign.id)}>{campaign.name}</option>
							{/each}
						</Dropdown>
						<div class="grid gap-3 md:grid-cols-2">
							<Dropdown tone="admin" bind:value={scheduleType}>
								<option value="one_time">One time</option>
								<option value="recurring">Recurring</option>
							</Dropdown>
							<Dropdown
								tone="admin"
								bind:value={scheduleRecurrence}
								disabled={scheduleType === "one_time"}
							>
								<option value="">No recurrence</option>
								<option value="daily">Daily</option>
								<option value="weekly">Weekly</option>
								<option value="monthly">Monthly</option>
							</Dropdown>
							<TextInput tone="admin" type="datetime-local" bind:value={scheduleWindowStart} />
							<TextInput tone="admin" type="datetime-local" bind:value={scheduleWindowEnd} />
							<TextInput tone="admin" type="datetime-local" bind:value={scheduleUntilAt} />
							<TextInput tone="admin" bind:value={scheduleTimezone} />
						</div>
						<Button tone="admin" variant="primary" disabled={saving} onclick={saveSchedule}>
							Save schedule
						</Button>
					</div>
				</AdminPanel>
				<AdminPanel title="Lifecycle">
					<div class="flex flex-wrap gap-2">
						<Button tone="admin" variant="primary" disabled={saving} onclick={runLifecycle}>
							Run lifecycle
						</Button>
						<Button tone="admin" disabled={saving} onclick={runReconciliation}>
							Run reconciliation
						</Button>
					</div>
					{#if lifecycleReport}
						<div class="mt-4 grid grid-cols-3 gap-2 text-sm">
							<Badge tone="success">{lifecycleReport.activated} activated</Badge>
							<Badge tone="warning">{lifecycleReport.deactivated} deactivated</Badge>
							<Badge tone="neutral">{lifecycleReport.archived} archived</Badge>
						</div>
					{/if}
					{#if reconciliationReport}
						<div class="mt-4 rounded-lg border border-stone-200 p-3 text-sm dark:border-stone-800">
							<p class="font-medium text-stone-950 dark:text-stone-50">
								{reconciliationReport.issues.length} issue{reconciliationReport.issues.length === 1
									? ""
									: "s"}
							</p>
							{#each reconciliationReport.issues as issue, index (index)}
								<p class="mt-2 text-stone-600 dark:text-stone-300">
									#{issue.campaign_id}: {issue.message}
								</p>
							{/each}
						</div>
					{/if}
				</AdminPanel>
			</div>
			<div class="space-y-6">
				<AdminPanel title="Evaluation metrics">
					<div class="grid gap-3 sm:grid-cols-2">
						{@render statCard("Evaluations", metrics.total_evaluations, "bi-speedometer2")}
						{@render statCard("Failures", metrics.failed_evaluations, "bi-exclamation-triangle")}
						{@render statCard("Last latency", `${metrics.last_latency_ms} ms`, "bi-stopwatch")}
						{@render statCard("Candidates", metrics.last_candidate_campaigns, "bi-funnel")}
					</div>
					{#if metrics.last_error}
						<p
							class="mt-4 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-200"
						>
							{metrics.last_error}
						</p>
					{/if}
				</AdminPanel>
				<AdminPanel title="History and audit">
					<div class="grid gap-4 lg:grid-cols-2">
						<div class="space-y-2">
							<h3 class="text-sm font-semibold text-stone-950 dark:text-stone-50">State history</h3>
							{#each history.slice(0, 8) as entry (entry.id)}
								<p class="text-sm text-stone-600 dark:text-stone-300">
									#{entry.campaign_id}: {entry.from_status} to {entry.to_status}
								</p>
							{:else}
								<AdminEmptyState>No lifecycle history.</AdminEmptyState>
							{/each}
						</div>
						<div class="space-y-2">
							<h3 class="text-sm font-semibold text-stone-950 dark:text-stone-50">Audit</h3>
							{#each audit.slice(0, 8) as entry (entry.id)}
								<p class="text-sm text-stone-600 dark:text-stone-300">
									#{entry.campaign_id}: {entry.summary}
								</p>
							{:else}
								<AdminEmptyState>No audit entries.</AdminEmptyState>
							{/each}
						</div>
					</div>
				</AdminPanel>
			</div>
		</div>
	{/if}
</div>

<AdminFloatingNotices
	statusMessage={notices.message}
	statusTone={notices.tone}
	onDismissStatus={notices.clear}
/>
