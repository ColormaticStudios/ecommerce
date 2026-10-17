<script lang="ts">
	import { getContext, onMount, untrack } from "svelte";
	import type { API } from "$lib/api";
	import type { ProductModel } from "$lib/models";
	import { searchAdminProducts } from "$lib/admin/productSearch";
	import SearchMultiSelect from "$lib/admin/SearchMultiSelect.svelte";
	import { merchandisingLabel } from "$lib/admin/search-merchandising-labels";
	import type {
		MerchandisingRule,
		MerchandisingPreview,
		MerchandisingAudit,
	} from "$lib/api/domains/search-merchandising";
	import { ApiProblemError, getLocalizedApiErrorMessage } from "$lib/api/errors";
	import { LOCALIZATION_CONTEXT, type LocalizationRuntime } from "$lib/localization/runtime";
	import {
		buildMerchandisingInput,
		emptyMerchandisingDraft,
		merchandisingPatch,
		previewRules,
		ruleDraft,
		localDateTime,
		formatMerchandisingSchedule,
	} from "$lib/admin/search-merchandising";
	import AdminPageHeader from "$lib/admin/AdminPageHeader.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminConfirmDialog from "$lib/admin/AdminConfirmDialog.svelte";
	import AdminFloatingNotices from "$lib/admin/AdminFloatingNotices.svelte";
	import AdminListItem from "$lib/admin/AdminListItem.svelte";
	import AdminMasterDetailLayout from "$lib/admin/AdminMasterDetailLayout.svelte";
	import AdminMetaText from "$lib/admin/AdminMetaText.svelte";
	import AdminResourceActions from "$lib/admin/AdminResourceActions.svelte";
	import AdminSurface from "$lib/admin/AdminSurface.svelte";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
	import NumberInput from "$lib/components/NumberInput.svelte";
	import type { PageData } from "./$types";
	import IconButton from "$lib/components/IconButton.svelte";
	import AdminSearchForm from "$lib/admin/AdminSearchForm.svelte";
	import { createAdminNotices, createAdminSavePrompt } from "$lib/admin/state.svelte";

	interface Props {
		data: PageData & { initialRule?: MerchandisingRule; initialPreview?: MerchandisingPreview };
	}
	let { data }: Props = $props();
	const initialRule = untrack(() => data.initialRule);
	const initialPreview = untrack(() => data.initialPreview);
	const api = getContext<API>("api");
	const localization = getContext<LocalizationRuntime>(LOCALIZATION_CONTEXT);
	let rules = $state<MerchandisingRule[]>(untrack(() => data.rules));
	let allAudit = $state<MerchandisingAudit[]>(untrack(() => data.audit));
	let audit = $state<MerchandisingAudit[]>(
		untrack(() =>
			initialRule ? data.audit.filter((entry) => entry.rule_id === initialRule.id) : data.audit
		)
	);
	let auditRuleId = $state<number | null>(untrack(() => initialRule?.id ?? null));
	let editingId = $state<number | null>(untrack(() => initialRule?.id ?? null));
	let draft = $state(
		untrack(() => (initialRule ? ruleDraft(initialRule, localization) : emptyMerchandisingDraft()))
	);
	const notices = createAdminNotices();
	const savePrompt = createAdminSavePrompt({
		navigationMessage: localization.translate(
			"admin.search.merchandising.discard_prompt",
			"You have unsaved rule changes. Leave this section and discard them?"
		),
	});
	let savedSnapshot = $state(untrack(() => JSON.stringify(draft)));
	const currentSnapshot = $derived(JSON.stringify(draft));
	const formHasUnsavedChanges = $derived(currentSnapshot !== savedSnapshot);
	function captureSavedSnapshot() {
		savedSnapshot = currentSnapshot;
	}
	function requestClear() {
		if (savePrompt.confirmDiscard()) reset();
	}
	let busy = $state(false);
	let auditLoading = $state(false);
	let searchQuery = $state("");
	let productLoading = $state(false);
	let products = $state<ProductModel[]>([]);
	let deleteTarget = $state<MerchandisingRule | null>(null);
	let previewQuery = $state(untrack(() => initialRule?.predicate.query?.value ?? ""));
	let previewCategories = $state<string[]>([]);
	let previewBrands = $state<string[]>([]);
	let previewStock = $state<string[]>([]);
	let previewPriceMin = $state<string | number>("");
	let previewPriceMax = $state<string | number>("");
	let previewAttributes = $state<Array<{ slug: string; values: string }>>([]);
	let timezone = $state("");
	onMount(() => {
		timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
		if (initialRule) {
			draft = ruleDraft(initialRule, localization);
			captureSavedSnapshot();
		}
	});
	const categoryOptions = $derived(
		data.categories.map((category) => ({ value: category.slug, label: category.name }))
	);
	const brandOptions = $derived(
		data.brands.map((brand) => ({ value: brand.slug, label: brand.name }))
	);
	let previewSort = $state<"" | "relevance" | "created_at" | "price" | "name">("");
	let previewOrder = $state<"asc" | "desc">("desc");
	let previewAt = $state("");
	let preview = $state<MerchandisingPreview | null>(untrack(() => initialPreview ?? null));
	let previewLoading = $state(false);
	let previewStale = $state(false);
	let inputRevision = 0;
	let auditRequest = 0;
	let productRequest = 0;

	function notify(text: string, error = false) {
		notices.set(error ? "error" : "success", text);
	}
	function report(error: unknown, fallback: string) {
		const translated = getLocalizedApiErrorMessage(
			error,
			(key, source, params) => localization.translate(key, source, params),
			fallback
		);
		notify(
			error instanceof ApiProblemError && error.status === 409
				? localization.translate(
						"admin.search.merchandising.conflict_help",
						"{message} Refresh the rule list before saving again.",
						{ message: translated }
					)
				: translated,
			true
		);
	}
	function changed() {
		inputRevision++;
		previewStale = true;
	}
	function reset() {
		auditRequest++;
		productRequest++;
		auditLoading = false;
		productLoading = false;
		editingId = null;
		draft = emptyMerchandisingDraft();
		captureSavedSnapshot();
		products = [];
		auditRuleId = null;
		audit = allAudit;
		changed();
	}
	async function refresh() {
		if (!savePrompt.confirmDiscard()) return;
		busy = true;
		try {
			rules = await api.listAdminSearchMerchandisingRules();
			const selected = rules.find((rule) => rule.id === editingId);
			if (selected) {
				draft = ruleDraft(selected, localization);
				captureSavedSnapshot();
			} else if (editingId !== null) reset();
			changed();
			notify(
				localization.translate("admin.search.merchandising.refreshed", "Rule list refreshed.")
			);
			await loadAudit(auditRuleId);
		} catch (error) {
			report(
				error,
				localization.translate("admin.search.merchandising.load_error", "Unable to load rules.")
			);
		} finally {
			busy = false;
		}
	}
	async function loadAudit(id: number | null) {
		auditRuleId = id;
		const request = ++auditRequest;
		auditLoading = true;
		try {
			const rows = await api.listAllAdminSearchMerchandisingAudit();
			if (request === auditRequest) {
				allAudit = rows;
				audit = id === null ? rows : rows.filter((entry) => entry.rule_id === id);
			}
		} catch (error) {
			if (request === auditRequest)
				report(
					error,
					localization.translate(
						"admin.search.merchandising.history_error",
						"Unable to load rule history."
					)
				);
		} finally {
			if (request === auditRequest) auditLoading = false;
		}
	}
	function edit(rule: MerchandisingRule) {
		if (editingId === rule.id || !savePrompt.confirmDiscard()) return;
		editingId = rule.id;
		draft = ruleDraft(rule, localization);
		captureSavedSnapshot();
		audit = [];
		changed();
		void loadAudit(rule.id);
	}
	async function save() {
		if (busy) return;
		try {
			const input = buildMerchandisingInput(draft, localization);
			busy = true;
			const saved = editingId
				? await api.updateAdminSearchMerchandisingRule(editingId, merchandisingPatch(input))
				: await api.createAdminSearchMerchandisingRule(input);
			rules = [saved, ...rules.filter((rule) => rule.id !== saved.id)];
			editingId = saved.id;
			draft = ruleDraft(saved, localization);
			captureSavedSnapshot();
			changed();
			notify(localization.translate("admin.search.merchandising.saved", "Rule saved."));
			await loadAudit(saved.id);
		} catch (error) {
			report(
				error,
				localization.translate("admin.search.merchandising.save_error", "Unable to save rule.")
			);
		} finally {
			busy = false;
		}
	}
	async function remove() {
		if (!deleteTarget) return;
		const target = deleteTarget;
		busy = true;
		try {
			await api.deleteAdminSearchMerchandisingRule(target.id);
			rules = rules.filter((rule) => rule.id !== target.id);
			if (editingId === target.id) reset();
			deleteTarget = null;
			notify(localization.translate("admin.search.merchandising.deleted", "Rule deleted."));
			await loadAudit(null);
			changed();
		} catch (error) {
			report(
				error,
				localization.translate("admin.search.merchandising.delete_error", "Unable to delete rule.")
			);
		} finally {
			busy = false;
		}
	}
	async function searchProducts() {
		const request = ++productRequest;
		productLoading = true;
		try {
			const result = await searchAdminProducts(api, searchQuery, 20);
			if (request === productRequest) {
				products = result;
				if (!products.length)
					notify(
						localization.translate(
							"admin.search.merchandising.no_products",
							"No products matched this search."
						)
					);
			}
		} catch (error) {
			if (request === productRequest)
				report(
					error,
					localization.translate(
						"admin.search.merchandising.product_error",
						"Unable to search products."
					)
				);
		} finally {
			if (request === productRequest) productLoading = false;
		}
	}
	function toggleProduct(product: ProductModel) {
		if (draft.targets.some((target) => target.productId === product.id)) {
			draft.targets = draft.targets.filter((target) => target.productId !== product.id);
			changed();
			return;
		}
		draft.targets = [
			...draft.targets,
			{
				productId: product.id,
				name:
					product.name ||
					localization.translate("admin.search.merchandising.unnamed_product", "Unnamed product"),
				position: String(draft.targets.length + 1),
			},
		];
		changed();
	}
	async function runPreview(withDraft: boolean) {
		try {
			const input = withDraft ? buildMerchandisingInput(draft, localization) : null;
			const at = localDateTime(previewAt, localization);
			const min = previewPriceMin === "" ? undefined : Number(previewPriceMin);
			const max = previewPriceMax === "" ? undefined : Number(previewPriceMax);
			if (
				(min !== undefined && (!Number.isFinite(min) || min < 0)) ||
				(max !== undefined && (!Number.isFinite(max) || max < 0)) ||
				(min !== undefined && max !== undefined && max < min)
			)
				throw new Error(
					localization.translate("admin.search.preview.invalid_price", "Enter a valid price range.")
				);
			const attributes: Record<string, string[]> = {};
			for (const row of previewAttributes) {
				if (!row.slug) continue;
				const values = row.values
					.split(",")
					.map((value) => value.trim())
					.filter(Boolean);
				if (!values.length) continue;
				attributes[row.slug] = [...new Set([...(attributes[row.slug] ?? []), ...values])];
			}
			const revision = inputRevision;
			previewLoading = true;
			preview = await api.previewAdminSearch({
				filters: {
					q: previewQuery.trim() || undefined,
					category_slug: previewCategories.length ? previewCategories : undefined,
					brand_slug: previewBrands.length ? previewBrands : undefined,
					has_variant_stock: previewStock.length
						? previewStock.map((value) => value === "true")
						: undefined,
					price_range:
						min !== undefined || max !== undefined ? [`${min ?? 0}:${max ?? ""}`] : undefined,
					attribute: Object.keys(attributes).length ? attributes : undefined,
					sort: previewSort || undefined,
					order: previewOrder,
					page: 1,
					limit: 20,
				},
				...(input ? { rules: previewRules(rules, input, editingId) } : {}),
				...(at ? { at } : {}),
			});
			previewStale = revision !== inputRevision;
			notify(
				withDraft
					? localization.translate(
							"admin.search.merchandising.preview_unsaved_notice",
							"Preview includes unsaved changes. Nothing was saved."
						)
					: localization.translate(
							"admin.search.merchandising.preview_saved_notice",
							"Preview uses saved rules."
						)
			);
		} catch (error) {
			report(
				error,
				localization.translate(
					"admin.search.merchandising.preview_error",
					"Unable to preview search."
				)
			);
		} finally {
			previewLoading = false;
		}
	}
	$effect(() => {
		savePrompt.dirty = formHasUnsavedChanges;
		savePrompt.blocked = busy || productLoading;
		savePrompt.saveAction = formHasUnsavedChanges ? save : null;
	});
</script>

{#snippet snapshot(rule: MerchandisingRule | null)}
	{#if rule}
		<dl class="mt-2 space-y-2 text-sm">
			<div>
				<dt class="text-stone-500">
					{$localization.translate("admin.search.merchandising.rule", "Rule")}
				</dt>
				<dd>{rule.name} · {merchandisingLabel(rule.rule_type, $localization)}</dd>
			</div>
			<div>
				<dt class="text-stone-500">
					{$localization.translate("admin.search.merchandising.status", "Status")}
				</dt>
				<dd>
					{rule.is_active
						? $localization.translate("admin.search.merchandising.active", "Active")
						: $localization.translate("admin.search.merchandising.inactive", "Inactive")} · {$localization.translate(
						"admin.search.merchandising.priority_version",
						"Priority {priority} · version {version}",
						{ priority: rule.priority, version: rule.version }
					)}
				</dd>
			</div>
			<div>
				<dt class="text-stone-500">
					{$localization.translate("admin.search.merchandising.query", "Query")}
				</dt>
				<dd>
					{rule.predicate.query
						? `${merchandisingLabel(rule.predicate.query.mode, $localization)}: ${rule.predicate.query.value}`
						: $localization.translate("admin.search.merchandising.any_query", "Any query")}
				</dd>
			</div>
			<div>
				<dt class="text-stone-500">
					{$localization.translate("admin.search.merchandising.categories", "Categories")}
				</dt>
				<dd>
					{rule.predicate.category_slugs
						?.map(
							(slug) => data.categories.find((category) => category.slug === slug)?.name ?? slug
						)
						.join(", ") ||
						$localization.translate("admin.search.merchandising.any_category", "Any category")}
				</dd>
			</div>
			<div>
				<dt class="text-stone-500">
					{$localization.translate("admin.search.merchandising.channel", "Channel")}
				</dt>
				<dd>
					{(rule.predicate.channel
						? merchandisingLabel(rule.predicate.channel, $localization)
						: "") ||
						$localization.translate("admin.search.merchandising.any_channel", "Any channel")}
				</dd>
			</div>
			<div>
				<dt class="text-stone-500">
					{$localization.translate("admin.search.merchandising.schedule", "Schedule")}
				</dt>
				<dd>
					{formatMerchandisingSchedule(rule.starts_at, timezone, $localization)} → {formatMerchandisingSchedule(
						rule.ends_at,
						timezone,
						$localization
					)}
				</dd>
			</div>
			<div>
				<dt class="text-stone-500">
					{$localization.translate("admin.search.merchandising.products", "Products")}
				</dt>
				<dd>
					<ul>
						{#each rule.action.targets as target (target.product_id)}<li>
								{target.product_name ||
									$localization.translate(
										"admin.search.merchandising.unavailable_product",
										"Unavailable product"
									)}{target.position
									? ` · ${$localization.translate("admin.search.merchandising.position_value", "Position {position}", { position: target.position })}`
									: ""}
							</li>{/each}
					</ul>
				</dd>
			</div>
			{#if rule.action.multiplier}<div>
					<dt class="text-stone-500">
						{$localization.translate("admin.search.merchandising.multiplier", "Multiplier")}
					</dt>
					<dd>{rule.action.multiplier}×</dd>
				</div>{/if}
		</dl>
	{:else}<p class="mt-2 text-sm text-stone-500">
			{$localization.translate("admin.search.merchandising.no_rule", "No rule.")}
		</p>{/if}
{/snippet}

<div class="space-y-6 pb-16">
	<AdminPageHeader
		title={$localization.translate("admin.search.merchandising.title", "search merchandising")}
	>
		{#snippet actions()}
			<AdminResourceActions
				countLabel={$localization.plural("admin.search.merchandising.rule_count", rules.length, {
					one: "{count} rule",
					other: "{count} rules",
				})}
			/>
		{/snippet}
	</AdminPageHeader>
	{#each data.errorMessages as error, index (index)}
		<AdminEmptyState tone="error">{error}</AdminEmptyState>
	{/each}
	<AdminMasterDetailLayout columnsClass="xl:grid-cols-[0.8fr_1.2fr]">
		{#snippet master()}
			<AdminPanel title={$localization.translate("admin.search.merchandising.rules", "rules")}>
				{#snippet headerActions()}
					<IconButton
						tone="admin"
						outlined
						aria-label={$localization.translate(
							"admin.search.merchandising.refresh",
							"Refresh rules"
						)}
						title={$localization.translate("admin.search.merchandising.refresh", "Refresh rules")}
						disabled={busy}
						onclick={refresh}
						><i class={`bi ${busy ? "bi-arrow-repeat animate-spin" : "bi-arrow-clockwise"}`}
						></i></IconButton
					>
				{/snippet}
				<div class="space-y-3">
					{#each rules as rule (rule.id)}
						<AdminListItem
							as="button"
							active={editingId === rule.id}
							interactive={editingId !== rule.id}
							disabled={busy}
							onclick={() => edit(rule)}
							class="p-4"
						>
							<div class="min-w-0">
								<AdminMetaText tone="strong" class="break-words">{rule.name}</AdminMetaText>
								<div class="mt-2 flex flex-wrap gap-2">
									<Badge tone="neutral">{merchandisingLabel(rule.rule_type, $localization)}</Badge>
									<Badge tone={rule.is_active ? "success" : "neutral"}
										>{rule.is_active
											? $localization.translate("admin.search.merchandising.active", "Active")
											: $localization.translate(
													"admin.search.merchandising.inactive",
													"Inactive"
												)}</Badge
									>
								</div>
								<p class="text-sm text-stone-600 dark:text-stone-300">
									{$localization.translate(
										"admin.search.merchandising.priority_version",
										"Priority {priority} · version {version}",
										{ priority: rule.priority, version: rule.version }
									)}
								</p>
								<p class="text-xs text-stone-500">
									{formatMerchandisingSchedule(rule.starts_at, timezone, $localization)} → {formatMerchandisingSchedule(
										rule.ends_at,
										timezone,
										$localization
									)}
								</p>
								<p class="mt-2 text-sm break-words">
									{rule.action.targets
										.map(
											(target) =>
												target.product_name ||
												$localization.translate(
													"admin.search.merchandising.unavailable_product",
													"Unavailable product"
												)
										)
										.join(", ")}
								</p>
							</div>
						</AdminListItem>
					{:else}<AdminEmptyState
							>{$localization.translate(
								"admin.search.merchandising.empty",
								"No merchandising rules yet."
							)}</AdminEmptyState
						>{/each}
				</div>
			</AdminPanel>
		{/snippet}
		{#snippet detail()}
			<AdminPanel
				title={editingId
					? $localization.translate("admin.search.merchandising.edit_rule", "edit rule")
					: $localization.translate("admin.search.merchandising.create_rule_title", "create rule")}
			>
				<div
					class="space-y-5"
					role="group"
					aria-label={$localization.translate("admin.search.merchandising.editor", "Rule editor")}
					oninput={changed}
				>
					<div class="grid gap-5 md:grid-cols-2">
						<label class="grid gap-2"
							><span class="text-sm font-medium text-stone-700 dark:text-stone-200"
								>{$localization.translate("admin.search.merchandising.name", "Rule name")}</span
							><TextInput tone="admin" bind:value={draft.name} required maxlength={120} /></label
						>
						<label class="grid gap-2"
							><span class="text-sm font-medium text-stone-700 dark:text-stone-200"
								>{$localization.translate("admin.search.merchandising.action", "Action")}</span
							><Dropdown tone="admin" bind:value={draft.ruleType}
								><option value="pin"
									>{$localization.translate("admin.search.merchandising.pin", "Pin")}</option
								><option value="bury"
									>{$localization.translate("admin.search.merchandising.bury", "Bury")}</option
								><option value="hide"
									>{$localization.translate("admin.search.merchandising.hide", "Hide")}</option
								><option value="boost"
									>{$localization.translate("admin.search.merchandising.boost", "Boost")}</option
								><option value="include"
									>{$localization.translate(
										"admin.search.merchandising.include",
										"Include"
									)}</option
								></Dropdown
							></label
						>
						<label class="grid gap-2"
							><span class="text-sm font-medium text-stone-700 dark:text-stone-200"
								>{$localization.translate("admin.search.merchandising.priority", "Priority")}</span
							><NumberInput
								tone="admin"
								bind:value={draft.priority}
								min={0}
								max={10000}
								required
							/></label
						>
					</div>
					<p class="text-sm text-stone-600 dark:text-stone-300">
						{$localization.translate(
							"admin.search.merchandising.priority_help",
							"Higher priorities take precedence. Price, name, and newest sorts bypass pin, bury, and boost; hide and include still respect shopper filters."
						)}
					</p>
					<AdminSurface variant="muted" as="div">
						<label class="flex items-center justify-between gap-4">
							<div>
								<AdminMetaText tone="strong"
									>{$localization.translate(
										"admin.search.merchandising.active_storefront",
										"Active in storefront"
									)}</AdminMetaText
								><AdminMetaText class="mt-1"
									>{$localization.translate(
										"admin.search.merchandising.disabled_help",
										"Disabled rules remain available for editing and preview."
									)}</AdminMetaText
								>
							</div>
							<input class="h-4 w-4 shrink-0" type="checkbox" bind:checked={draft.isActive} />
						</label>
					</AdminSurface>
					<div class="grid gap-5 md:grid-cols-2">
						<label class="grid gap-2"
							><span class="text-sm font-medium text-stone-700 dark:text-stone-200"
								>{$localization.translate(
									"admin.search.merchandising.query_match",
									"Query match"
								)}</span
							><Dropdown tone="admin" bind:value={draft.queryMode}
								><option value="exact"
									>{$localization.translate("admin.search.merchandising.exact", "Exact")}</option
								><option value="prefix"
									>{$localization.translate("admin.search.merchandising.prefix", "Prefix")}</option
								><option value="contains"
									>{$localization.translate(
										"admin.search.merchandising.contains",
										"Contains"
									)}</option
								></Dropdown
							></label
						>
						<label class="grid gap-2"
							><span class="text-sm font-medium text-stone-700 dark:text-stone-200"
								>{$localization.translate(
									"admin.search.merchandising.query_help",
									"Query (blank matches all)"
								)}</span
							><TextInput tone="admin" bind:value={draft.queryValue} /></label
						>
						<label class="grid gap-2"
							><span class="text-sm font-medium text-stone-700 dark:text-stone-200"
								>{$localization.translate("admin.search.merchandising.channel", "Channel")}</span
							><Dropdown tone="admin" bind:value={draft.channel}
								><option value=""
									>{$localization.translate(
										"admin.search.merchandising.any_channel",
										"Any channel"
									)}</option
								><option value="storefront"
									>{$localization.translate(
										"admin.search.merchandising.storefront",
										"Storefront"
									)}</option
								></Dropdown
							></label
						>
					</div>
					<SearchMultiSelect
						options={categoryOptions}
						bind:values={draft.categorySlugs}
						searchLabel={$localization.translate(
							"admin.search.selection.context",
							"Search category context to add"
						)}
						label={$localization.translate(
							"admin.search.merchandising.category_context",
							"Category context (any selected)"
						)}
						onchange={changed}
					/>
					<p class="text-sm text-stone-500">
						{$localization.translate(
							"admin.search.merchandising.timezone",
							"Times use your browser timezone: {timezone}. Saved as UTC.",
							{ timezone }
						)}
					</p>
					<div class="grid gap-4 md:grid-cols-2">
						<label class="grid gap-2"
							><span class="text-sm font-medium text-stone-700 dark:text-stone-200"
								>{$localization.translate(
									"admin.search.merchandising.starts_at",
									"Starts at (local time, optional)"
								)}</span
							><TextInput
								type="datetime-local"
								step="any"
								tone="admin"
								bind:value={draft.startsAt}
							/></label
						><label class="grid gap-2"
							><span class="text-sm font-medium text-stone-700 dark:text-stone-200"
								>{$localization.translate(
									"admin.search.merchandising.ends_at",
									"Ends at (local time, optional)"
								)}</span
							><TextInput
								type="datetime-local"
								step="any"
								tone="admin"
								bind:value={draft.endsAt}
							/></label
						>
					</div>
					{#if draft.ruleType === "boost"}<label class="grid max-w-xs gap-2"
							><span class="text-sm font-medium text-stone-700 dark:text-stone-200"
								>{$localization.translate(
									"admin.search.merchandising.score_multiplier",
									"Score multiplier"
								)}</span
							><NumberInput
								tone="admin"
								allowDecimal
								bind:value={draft.multiplier}
								min={1.01}
								max={100}
								required
							/></label
						>{/if}
					<div class="space-y-3">
						<p class="text-sm font-medium text-stone-700 dark:text-stone-200">
							{$localization.translate("admin.search.merchandising.products_title", "products")}
						</p>
						<AdminSearchForm
							fullWidth
							placeholder={$localization.translate(
								"admin.search.merchandising.search_products",
								"Search products"
							)}
							bind:value={searchQuery}
							disabled={productLoading || busy}
							refreshing={productLoading}
							onSearch={searchProducts}
							onRefresh={searchProducts}
						/>
						<div class="max-h-96 space-y-2 overflow-y-auto">
							{#each products as product (product.id)}
								{@const selected = draft.targets.some((target) => target.productId === product.id)}
								<button
									type="button"
									class="flex w-full cursor-pointer items-center justify-between gap-2 rounded-lg border border-stone-200 px-3 py-2 text-left text-sm transition hover:bg-stone-50 disabled:cursor-default dark:border-stone-800 dark:hover:bg-stone-900"
									aria-pressed={selected}
									disabled={busy || productLoading}
									onclick={() => toggleProduct(product)}
								>
									<span class="min-w-0 break-words"
										>{product.name ||
											$localization.translate(
												"admin.search.merchandising.unnamed_product",
												"Unnamed product"
											)}</span
									>
									<Badge tone={selected ? "success" : "neutral"}
										>{selected
											? $localization.translate("admin.search.merchandising.selected", "Selected")
											: $localization.translate("admin.search.merchandising.add", "Add")}</Badge
									>
								</button>
							{/each}
						</div>
						{#each draft.targets as target (target.productId)}<AdminSurface
								as="div"
								variant="subsurface"
								class="flex flex-wrap items-center gap-3"
							>
								<span class="min-w-40 flex-1">{target.name}</span
								>{#if draft.ruleType === "pin"}<label class="flex items-center gap-2"
										><span class="text-sm font-medium text-stone-700 dark:text-stone-200"
											>{$localization.translate(
												"admin.search.merchandising.position",
												"Position"
											)}</span
										><NumberInput
											tone="admin"
											full={false}
											class="w-24"
											min={1}
											max={10000}
											bind:value={target.position}
											required
										/></label
									>{/if}<Button
									tone="admin"
									type="button"
									onclick={() => {
										draft.targets = draft.targets.filter(
											(row) => row.productId !== target.productId
										);
										changed();
									}}
									>{$localization.translate("admin.search.merchandising.remove", "Remove")}</Button
								>
							</AdminSurface>{:else}<p class="text-sm text-stone-500">
								{$localization.translate(
									"admin.search.merchandising.target_help",
									"Select products to target. Only published products can appear in storefront results."
								)}
							</p>{/each}
					</div>
					<div class="flex flex-wrap items-center gap-3">
						<Button tone="admin" variant="primary" disabled={busy} onclick={save}>
							<i class={`bi ${editingId ? "bi-floppy-fill" : "bi-plus-lg"} mr-1`}></i>{busy
								? $localization.translate("admin.search.merchandising.saving", "Saving…")
								: editingId
									? $localization.translate("admin.search.merchandising.save", "Save changes")
									: $localization.translate(
											"admin.search.merchandising.create_rule",
											"Create rule"
										)}
						</Button>
						<Button tone="admin" disabled={busy} onclick={requestClear}
							><i class="bi bi-x-lg mr-1"></i>{$localization.translate(
								"admin.search.merchandising.clear",
								"Clear"
							)}</Button
						>
						{#if editingId}
							<Button
								tone="admin"
								variant="danger"
								disabled={busy}
								onclick={() => (deleteTarget = rules.find((rule) => rule.id === editingId) ?? null)}
								><i class="bi bi-trash mr-1"></i>{$localization.translate(
									"admin.search.merchandising.delete_rule",
									"Delete rule"
								)}</Button
							>
						{/if}
					</div>
				</div>
			</AdminPanel>
		{/snippet}
	</AdminMasterDetailLayout>
	<AdminPanel title={$localization.translate("admin.search.merchandising.preview", "preview")}>
		<div
			class="space-y-4"
			oninput={changed}
			role="group"
			aria-label={$localization.translate(
				"admin.search.merchandising.preview_controls",
				"Preview controls"
			)}
		>
			<div class="grid gap-4 md:grid-cols-3">
				<label class="grid gap-2"
					><span class="text-sm font-medium text-stone-700 dark:text-stone-200"
						>{$localization.translate(
							"admin.search.merchandising.search_query",
							"Search query"
						)}</span
					><TextInput tone="admin" bind:value={previewQuery} /></label
				><label class="grid gap-2"
					><span class="text-sm font-medium text-stone-700 dark:text-stone-200"
						>{$localization.translate(
							"admin.search.preview.simulate_time",
							"Simulate time (local time, optional)"
						)}</span
					><TextInput type="datetime-local" step="any" tone="admin" bind:value={previewAt} /></label
				>
			</div>
			<p class="text-sm text-stone-500">
				{$localization.translate(
					"admin.search.merchandising.timezone",
					"Times use your browser timezone: {timezone}. Saved as UTC.",
					{ timezone }
				)}
			</p>
			<div class="grid gap-4 md:grid-cols-2">
				<SearchMultiSelect
					options={categoryOptions}
					bind:values={previewCategories}
					searchLabel={$localization.translate(
						"admin.search.selection.categories",
						"Search categories to add"
					)}
					label={$localization.translate("admin.search.preview.categories", "Categories")}
					onchange={changed}
				/>
				<SearchMultiSelect
					options={brandOptions}
					bind:values={previewBrands}
					searchLabel={$localization.translate(
						"admin.search.selection.brands",
						"Search brands to add"
					)}
					label={$localization.translate("admin.search.preview.brands", "Brands")}
					onchange={changed}
				/>
				<SearchMultiSelect
					options={[
						{
							value: "true",
							label: $localization.translate("storefront.search.in_stock", "In stock"),
						},
						{
							value: "false",
							label: $localization.translate("storefront.search.out_of_stock", "Out of stock"),
						},
					]}
					bind:values={previewStock}
					searchLabel={$localization.translate(
						"admin.search.selection.stock",
						"Search stock to add"
					)}
					label={$localization.translate("admin.search.preview.stock", "Stock")}
					onchange={changed}
				/>
				<div class="grid grid-cols-2 gap-3">
					<label class="grid gap-2"
						><span class="text-sm font-medium"
							>{$localization.translate("admin.search.preview.price_min", "Minimum price")}</span
						><NumberInput tone="admin" min={0} allowDecimal bind:value={previewPriceMin} /></label
					>
					<label class="grid gap-2"
						><span class="text-sm font-medium"
							>{$localization.translate("admin.search.preview.price_max", "Maximum price")}</span
						><NumberInput tone="admin" min={0} allowDecimal bind:value={previewPriceMax} /></label
					>
				</div>
			</div>
			<fieldset class="space-y-3">
				<legend class="mb-2 text-sm font-medium"
					>{$localization.translate(
						"admin.search.preview.attributes",
						"Attributes (comma-separated values)"
					)}</legend
				>
				{#each previewAttributes as row, index (index)}
					<div class="flex flex-wrap gap-2">
						<Dropdown
							tone="admin"
							full={false}
							aria-label={$localization.translate("admin.search.preview.attribute", "Attribute")}
							bind:value={row.slug}
						>
							<option value=""
								>{$localization.translate(
									"admin.search.preview.select_attribute",
									"Select an attribute"
								)}</option
							>
							{#each data.attributes.filter((attribute) => attribute.filterable) as attribute (attribute.id)}<option
									value={attribute.slug}>{attribute.key}</option
								>{/each}
						</Dropdown>
						<TextInput
							tone="admin"
							class="min-w-40 flex-1"
							aria-label={$localization.translate(
								"admin.search.preview.attribute_values",
								"Attribute values"
							)}
							bind:value={row.values}
						/>
						<Button
							tone="admin"
							onclick={() => {
								previewAttributes = previewAttributes.filter((_, rowIndex) => rowIndex !== index);
								changed();
							}}
							>{$localization.translate(
								"admin.search.preview.remove_attribute",
								"Remove attribute"
							)}</Button
						>
					</div>
				{/each}
				<Button
					tone="admin"
					onclick={() => {
						previewAttributes = [...previewAttributes, { slug: "", values: "" }];
						changed();
					}}
					>{$localization.translate("admin.search.preview.add_attribute", "Add attribute")}</Button
				>
			</fieldset>
			<div class="flex flex-wrap items-center gap-3">
				<label class="flex items-center gap-2"
					><span class="text-sm font-medium text-stone-700 dark:text-stone-200"
						>{$localization.translate("admin.search.merchandising.sort", "Sort")}</span
					><Dropdown tone="admin" full={false} bind:value={previewSort}
						><option value=""
							>{$localization.translate(
								"admin.search.merchandising.sort_default",
								"Search default"
							)}</option
						><option value="relevance"
							>{$localization.translate(
								"admin.search.merchandising.relevance",
								"Relevance"
							)}</option
						><option value="price"
							>{$localization.translate("admin.search.merchandising.price", "Price")}</option
						><option value="name"
							>{$localization.translate("admin.search.merchandising.sort_name", "Name")}</option
						><option value="created_at"
							>{$localization.translate("admin.search.merchandising.newest", "Newest")}</option
						></Dropdown
					></label
				><Dropdown
					tone="admin"
					full={false}
					aria-label={$localization.translate(
						"admin.search.merchandising.sort_direction",
						"Preview sort direction"
					)}
					bind:value={previewOrder}
					><option value="desc"
						>{$localization.translate(
							"admin.search.merchandising.descending",
							"Descending"
						)}</option
					><option value="asc"
						>{$localization.translate("admin.search.merchandising.ascending", "Ascending")}</option
					></Dropdown
				>
			</div>
			<div class="flex flex-wrap gap-2">
				<Button
					tone="admin"
					type="button"
					disabled={previewLoading}
					onclick={() => runPreview(false)}
					>{$localization.translate(
						"admin.search.merchandising.preview_saved",
						"Preview saved rules"
					)}</Button
				><Button
					tone="admin"
					type="button"
					disabled={previewLoading}
					onclick={() => runPreview(true)}
					>{$localization.translate(
						"admin.search.merchandising.preview_unsaved",
						"Preview unsaved rule"
					)}</Button
				>
			</div>
		</div>
		{#if previewLoading}<p role="status" class="mt-4">
				{$localization.translate("admin.search.merchandising.loading_preview", "Loading preview…")}
			</p>{/if}
		{#if preview}
			{#if previewStale}<p class="mt-4 text-amber-800 dark:text-amber-200">
					{$localization.translate(
						"admin.search.merchandising.stale_preview",
						"Inputs changed. Run preview again to compare the latest changes."
					)}
				</p>{/if}
			<div class="mt-5 grid gap-5 md:grid-cols-2">
				{#each [{ title: $localization.translate("admin.search.merchandising.baseline", "baseline"), result: preview.baseline }, { title: $localization.translate("admin.search.merchandising.proposed", "proposed"), result: preview.proposed }] as side (side.title)}<AdminSurface
						as="div"
						variant="subsurface"
					>
						<h3 class="font-semibold">{side.title}</h3>
						<p class="text-sm text-stone-500">
							{$localization.plural(
								"admin.search.merchandising.result_count",
								side.result.pagination.total,
								{ one: "{count} result", other: "{count} results" }
							)}
						</p>
						<ol class="mt-3 space-y-2">
							{#each side.result.items as product, index (product.id)}
								{@const score = side.result.explanations.find(
									(row) => row.product_id === product.id
								)}
								<li>
									<span class="text-stone-500">{index + 1}.</span>
									{product.name ||
										$localization.translate(
											"admin.search.merchandising.unnamed_product",
											"Unnamed product"
										)}{#if score}<span class="text-xs text-stone-500">
											· {$localization.translate(
												"admin.search.merchandising.score",
												"Score {score}",
												{
													score: $localization.formatNumber(score.adjusted_score ?? score.score, {
														minimumFractionDigits: 3,
														maximumFractionDigits: 3,
													}),
												}
											)}</span
										>{/if}
								</li>{:else}<li>
									{$localization.translate(
										"admin.search.merchandising.no_matching",
										"No matching products."
									)}
								</li>{/each}
						</ol>
					</AdminSurface>{/each}
			</div>
			<div class="mt-5 space-y-2">
				<h3 class="font-semibold">
					{$localization.translate(
						"admin.search.merchandising.decisions",
						"rule decisions and conflicts"
					)}
				</h3>
				{#each preview.proposed.rule_decisions as decision, index (index)}<p
						class={decision.outcome === "conflict"
							? "text-sm text-amber-800 dark:text-amber-200"
							: "text-sm text-stone-600 dark:text-stone-300"}
					>
						{decision.rule_name}{decision.product_name ? ` · ${decision.product_name}` : ""}: {merchandisingLabel(
							decision.outcome,
							localization
						)}
						· {merchandisingLabel(decision.reason, $localization)}
					</p>{:else}<p class="text-sm text-stone-500">
						{$localization.translate(
							"admin.search.merchandising.no_decisions",
							"No rule decisions."
						)}
					</p>{/each}
			</div>
		{/if}
	</AdminPanel>
	<AdminPanel title={$localization.translate("admin.search.merchandising.history", "rule history")}>
		{#snippet headerActions()}<Button
				tone="admin"
				type="button"
				disabled={auditLoading}
				onclick={() => loadAudit(null)}
				>{$localization.translate("admin.search.merchandising.all_history", "All history")}</Button
			>{/snippet}
		<p class="mb-3 text-sm text-stone-500">
			{auditRuleId === null
				? $localization.translate(
						"admin.search.merchandising.show_all_history",
						"Showing all rules, including deleted rules."
					)
				: $localization.translate(
						"admin.search.merchandising.show_selected_history",
						"Showing the selected rule."
					)}
		</p>
		{#if auditLoading}<p role="status">
				{$localization.translate("admin.search.merchandising.loading_history", "Loading history…")}
			</p>{/if}
		{#each audit as entry (entry.id)}<AdminSurface as="details" variant="subsurface" class="mb-3">
				<summary class="cursor-pointer text-sm font-medium text-stone-700 dark:text-stone-200"
					>{merchandisingLabel(entry.operation, $localization)} · {entry.after?.name ??
						entry.before?.name ??
						$localization.translate("admin.search.merchandising.deleted_rule", "Deleted rule")} · {formatMerchandisingSchedule(
						entry.created_at,
						timezone,
						$localization
					)} · {entry.actor_id
						? $localization.translate("admin.search.merchandising.actor", "Actor {id}", {
								id: entry.actor_id,
							})
						: $localization.translate("admin.search.merchandising.system", "System")}</summary
				>
				<div class="mt-3 grid gap-4 md:grid-cols-2">
					<div>
						<p class="font-medium">
							{$localization.translate("admin.search.merchandising.before", "before")}
						</p>
						{@render snapshot(entry.before)}
					</div>
					<div>
						<p class="font-medium">
							{$localization.translate("admin.search.merchandising.after", "after")}
						</p>
						{@render snapshot(entry.after)}
					</div>
				</div>
			</AdminSurface>{:else}<AdminEmptyState
				>{$localization.translate(
					"admin.search.merchandising.empty_history",
					"No history available."
				)}</AdminEmptyState
			>{/each}
	</AdminPanel>
</div>
<AdminFloatingNotices
	showUnsaved={savePrompt.dirty}
	unsavedMessage={$localization.translate(
		"admin.search.merchandising.unsaved",
		"You have unsaved rule changes."
	)}
	canSaveUnsaved={savePrompt.canSave}
	onSaveUnsaved={() => void savePrompt.save()}
	savingUnsaved={savePrompt.saving}
	statusMessage={notices.message}
	statusTone={notices.tone}
	onDismissStatus={notices.clear}
/>
{#if deleteTarget}<AdminConfirmDialog
		title={$localization.translate("admin.search.merchandising.delete_title", "delete rule")}
		message={$localization.translate(
			"admin.search.merchandising.delete_confirm",
			"Delete “{name}”? This change takes effect immediately.",
			{ name: deleteTarget.name }
		)}
		confirmLabel={$localization.translate("admin.search.merchandising.delete_rule", "Delete rule")}
		{busy}
		onConfirm={() => void remove()}
		onCancel={() => (deleteTarget = null)}
	/>{/if}
