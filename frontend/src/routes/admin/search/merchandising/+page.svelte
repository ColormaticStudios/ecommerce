<script lang="ts">
	import { getContext, untrack } from "svelte";
	import type { API } from "$lib/api";
	import type { components } from "$lib/api/generated/openapi";
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
		utcDateTime,
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
		untrack(() => (initialRule ? ruleDraft(initialRule) : emptyMerchandisingDraft()))
	);
	const notices = createAdminNotices();
	const savePrompt = createAdminSavePrompt({
		navigationMessage: "You have unsaved rule changes. Leave this section and discard them?",
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
	let products = $state<components["schemas"]["Product"][]>([]);
	let deleteTarget = $state<MerchandisingRule | null>(null);
	let previewQuery = $state(untrack(() => initialRule?.predicate.query?.value ?? ""));
	let previewCategory = $state("");
	let previewStock = $state(false);
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
				? `${translated} Refresh the rule list before saving again.`
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
				draft = ruleDraft(selected);
				captureSavedSnapshot();
			} else if (editingId !== null) reset();
			changed();
			notify("Rule list refreshed.");
			await loadAudit(auditRuleId);
		} catch (error) {
			report(error, "Unable to load rules.");
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
			if (request === auditRequest) report(error, "Unable to load rule history.");
		} finally {
			if (request === auditRequest) auditLoading = false;
		}
	}
	function edit(rule: MerchandisingRule) {
		if (editingId === rule.id || !savePrompt.confirmDiscard()) return;
		editingId = rule.id;
		draft = ruleDraft(rule);
		captureSavedSnapshot();
		audit = [];
		changed();
		void loadAudit(rule.id);
	}
	async function save() {
		if (busy) return;
		try {
			const input = buildMerchandisingInput(draft);
			busy = true;
			const saved = editingId
				? await api.updateAdminSearchMerchandisingRule(editingId, merchandisingPatch(input))
				: await api.createAdminSearchMerchandisingRule(input);
			rules = [saved, ...rules.filter((rule) => rule.id !== saved.id)];
			editingId = saved.id;
			draft = ruleDraft(saved);
			captureSavedSnapshot();
			changed();
			notify("Rule saved.");
			await loadAudit(saved.id);
		} catch (error) {
			report(error, "Unable to save rule.");
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
			notify("Rule deleted.");
			await loadAudit(null);
			changed();
		} catch (error) {
			report(error, "Unable to delete rule.");
		} finally {
			busy = false;
		}
	}
	async function searchProducts() {
		const request = ++productRequest;
		productLoading = true;
		try {
			const result = await api.previewAdminSearch({
				filters: { q: searchQuery.trim() || undefined, page: 1, limit: 20 },
				rules: [],
			});
			if (request === productRequest) {
				products = result.baseline.items;
				if (!products.length) notify("No published products matched this search.");
			}
		} catch (error) {
			if (request === productRequest) report(error, "Unable to search products.");
		} finally {
			if (request === productRequest) productLoading = false;
		}
	}
	function toggleProduct(product: components["schemas"]["Product"]) {
		if (draft.targets.some((target) => target.productId === product.id)) {
			draft.targets = draft.targets.filter((target) => target.productId !== product.id);
			changed();
			return;
		}
		draft.targets = [
			...draft.targets,
			{
				productId: product.id,
				name: product.name || "Unnamed product",
				position: String(draft.targets.length + 1),
			},
		];
		changed();
	}
	async function runPreview(withDraft: boolean) {
		try {
			const input = withDraft ? buildMerchandisingInput(draft) : null;
			const at = utcDateTime(previewAt);
			const revision = inputRevision;
			previewLoading = true;
			preview = await api.previewAdminSearch({
				filters: {
					q: previewQuery.trim() || undefined,
					category_slug: previewCategory ? [previewCategory] : undefined,
					has_variant_stock: previewStock ? [true] : undefined,
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
					? "Preview includes unsaved changes. Nothing was saved."
					: "Preview uses saved rules."
			);
		} catch (error) {
			report(error, "Unable to preview search.");
		} finally {
			previewLoading = false;
		}
	}
	function toggleCategory(slug: string, checked: boolean) {
		draft.categorySlugs = checked
			? [...draft.categorySlugs, slug]
			: draft.categorySlugs.filter((value) => value !== slug);
		changed();
	}
	function utcLabel(value: string | null) {
		return value
			? new Date(value).toISOString().replace("T", " ").replace(".000Z", " UTC")
			: "Open ended";
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
				<dt class="text-stone-500">Rule</dt>
				<dd>{rule.name} · {rule.rule_type}</dd>
			</div>
			<div>
				<dt class="text-stone-500">Status</dt>
				<dd>
					{rule.is_active ? "Active" : "Inactive"} · priority {rule.priority} · version {rule.version}
				</dd>
			</div>
			<div>
				<dt class="text-stone-500">Query</dt>
				<dd>
					{rule.predicate.query
						? `${rule.predicate.query.mode}: ${rule.predicate.query.value}`
						: "Any query"}
				</dd>
			</div>
			<div>
				<dt class="text-stone-500">Categories</dt>
				<dd>
					{rule.predicate.category_slugs
						?.map(
							(slug) => data.categories.find((category) => category.slug === slug)?.name ?? slug
						)
						.join(", ") || "Any category"}
				</dd>
			</div>
			<div>
				<dt class="text-stone-500">Channel</dt>
				<dd>{rule.predicate.channel || "Any channel"}</dd>
			</div>
			<div>
				<dt class="text-stone-500">Schedule</dt>
				<dd>{utcLabel(rule.starts_at)} → {utcLabel(rule.ends_at)}</dd>
			</div>
			<div>
				<dt class="text-stone-500">Products</dt>
				<dd>
					<ul>
						{#each rule.action.targets as target (target.product_id)}<li>
								{target.product_name || "Unavailable product"}{target.position
									? ` · position ${target.position}`
									: ""}
							</li>{/each}
					</ul>
				</dd>
			</div>
			{#if rule.action.multiplier}<div>
					<dt class="text-stone-500">Multiplier</dt>
					<dd>{rule.action.multiplier}×</dd>
				</div>{/if}
		</dl>
	{:else}<p class="mt-2 text-sm text-stone-500">No rule.</p>{/if}
{/snippet}

<div class="space-y-6 pb-16">
	<AdminPageHeader title="Search merchandising">
		{#snippet actions()}
			<AdminResourceActions countLabel={`${rules.length} rules`} />
		{/snippet}
	</AdminPageHeader>
	{#each data.errorMessages as error, index (index)}
		<AdminEmptyState tone="error">{error}</AdminEmptyState>
	{/each}
	<AdminMasterDetailLayout columnsClass="xl:grid-cols-[0.8fr_1.2fr]">
		{#snippet master()}
			<AdminPanel title="rules">
				{#snippet headerActions()}
					<IconButton
						tone="admin"
						outlined
						aria-label="Refresh rules"
						title="Refresh rules"
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
									<Badge tone="neutral">{rule.rule_type}</Badge>
									<Badge tone={rule.is_active ? "success" : "neutral"}
										>{rule.is_active ? "Active" : "Inactive"}</Badge
									>
								</div>
								<p class="text-sm text-stone-600 dark:text-stone-300">
									Priority {rule.priority} · version {rule.version}
								</p>
								<p class="text-xs text-stone-500">
									{utcLabel(rule.starts_at)} → {utcLabel(rule.ends_at)}
								</p>
								<p class="mt-2 text-sm break-words">
									{rule.action.targets
										.map((target) => target.product_name || "Unavailable product")
										.join(", ")}
								</p>
							</div>
						</AdminListItem>
					{:else}<AdminEmptyState>No merchandising rules yet.</AdminEmptyState>{/each}
				</div>
			</AdminPanel>
		{/snippet}
		{#snippet detail()}
			<AdminPanel title={editingId ? "edit rule" : "create rule"}>
				<div class="space-y-5" role="group" aria-label="Rule editor" oninput={changed}>
					<div class="grid gap-5 md:grid-cols-2">
						<label class="grid gap-2"
							><span class="text-sm font-medium text-stone-700 dark:text-stone-200">Rule name</span
							><TextInput tone="admin" bind:value={draft.name} required maxlength={120} /></label
						>
						<label class="grid gap-2"
							><span class="text-sm font-medium text-stone-700 dark:text-stone-200">Action</span
							><Dropdown tone="admin" bind:value={draft.ruleType}
								><option value="pin">Pin</option><option value="bury">Bury</option><option
									value="hide">Hide</option
								><option value="boost">Boost</option><option value="include">Include</option
								></Dropdown
							></label
						>
						<label class="grid gap-2"
							><span class="text-sm font-medium text-stone-700 dark:text-stone-200">Priority</span
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
						Higher priorities take precedence. Price, name, and newest sorts bypass pin, bury, and
						boost; hide and include still respect shopper filters.
					</p>
					<AdminSurface variant="muted" as="div">
						<label class="flex items-center justify-between gap-4">
							<div>
								<AdminMetaText tone="strong">Active in storefront</AdminMetaText><AdminMetaText
									class="mt-1"
									>Disabled rules remain available for editing and preview.</AdminMetaText
								>
							</div>
							<input class="h-4 w-4 shrink-0" type="checkbox" bind:checked={draft.isActive} />
						</label>
					</AdminSurface>
					<div class="grid gap-5 md:grid-cols-2">
						<label class="grid gap-2"
							><span class="text-sm font-medium text-stone-700 dark:text-stone-200"
								>Query match</span
							><Dropdown tone="admin" bind:value={draft.queryMode}
								><option value="exact">Exact</option><option value="prefix">Prefix</option><option
									value="contains">Contains</option
								></Dropdown
							></label
						>
						<label class="grid gap-2"
							><span class="text-sm font-medium text-stone-700 dark:text-stone-200"
								>Query (blank matches all)</span
							><TextInput tone="admin" bind:value={draft.queryValue} /></label
						>
						<label class="grid gap-2"
							><span class="text-sm font-medium text-stone-700 dark:text-stone-200">Channel</span
							><Dropdown tone="admin" bind:value={draft.channel}
								><option value="">Any channel</option><option value="storefront">Storefront</option
								></Dropdown
							></label
						>
					</div>
					<fieldset>
						<legend class="mb-3"
							><span class="text-sm font-medium text-stone-700 dark:text-stone-200"
								>Category context (any selected)</span
							></legend
						>
						<div class="flex flex-wrap gap-4">
							{#each data.categories as category (category.id)}<label
									class="flex items-center gap-2"
									><input
										class="h-4 w-4 shrink-0"
										type="checkbox"
										checked={draft.categorySlugs.includes(category.slug)}
										onchange={(event) => toggleCategory(category.slug, event.currentTarget.checked)}
									/>{category.name}</label
								>{:else}<span class="text-sm text-stone-500">No categories available.</span>{/each}
						</div>
					</fieldset>
					<div class="grid gap-4 md:grid-cols-2">
						<label class="grid gap-2"
							><span class="text-sm font-medium text-stone-700 dark:text-stone-200"
								>Starts at (UTC, optional)</span
							><TextInput
								type="datetime-local"
								step="any"
								tone="admin"
								bind:value={draft.startsAt}
							/></label
						><label class="grid gap-2"
							><span class="text-sm font-medium text-stone-700 dark:text-stone-200"
								>Ends at (UTC, optional)</span
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
								>Score multiplier</span
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
						<p class="text-sm font-medium text-stone-700 dark:text-stone-200">products</p>
						<AdminSearchForm
							fullWidth
							placeholder="Search products"
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
									<span class="min-w-0 break-words">{product.name || "Unnamed product"}</span>
									<Badge tone={selected ? "success" : "neutral"}
										>{selected ? "Selected" : "Add"}</Badge
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
											>Position</span
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
									}}>Remove</Button
								>
							</AdminSurface>{:else}<p class="text-sm text-stone-500">
								Select published products to target.
							</p>{/each}
					</div>
					<div class="flex flex-wrap items-center gap-3">
						<Button tone="admin" variant="primary" disabled={busy} onclick={save}>
							<i class={`bi ${editingId ? "bi-floppy-fill" : "bi-plus-lg"} mr-1`}></i>{busy
								? "Saving…"
								: editingId
									? "Save changes"
									: "Create rule"}
						</Button>
						<Button tone="admin" disabled={busy} onclick={requestClear}
							><i class="bi bi-x-lg mr-1"></i>Clear</Button
						>
						{#if editingId}
							<Button
								tone="admin"
								variant="danger"
								disabled={busy}
								onclick={() => (deleteTarget = rules.find((rule) => rule.id === editingId) ?? null)}
								><i class="bi bi-trash mr-1"></i>Delete rule</Button
							>
						{/if}
					</div>
				</div>
			</AdminPanel>
		{/snippet}
	</AdminMasterDetailLayout>
	<AdminPanel title="preview">
		<div class="space-y-4" oninput={changed} role="group" aria-label="Preview controls">
			<div class="grid gap-4 md:grid-cols-3">
				<label class="grid gap-2"
					><span class="text-sm font-medium text-stone-700 dark:text-stone-200">Search query</span
					><TextInput tone="admin" bind:value={previewQuery} /></label
				><label class="grid gap-2"
					><span class="text-sm font-medium text-stone-700 dark:text-stone-200"
						>Category filter</span
					><Dropdown tone="admin" bind:value={previewCategory}
						><option value="">All categories</option
						>{#each data.categories as category (category.id)}<option value={category.slug}
								>{category.name}</option
							>{/each}</Dropdown
					></label
				><label class="grid gap-2"
					><span class="text-sm font-medium text-stone-700 dark:text-stone-200"
						>Simulate time (UTC, optional)</span
					><TextInput type="datetime-local" step="any" tone="admin" bind:value={previewAt} /></label
				>
			</div>
			<div class="flex flex-wrap items-center gap-3">
				<label class="flex items-center gap-2"
					><input class="h-4 w-4 shrink-0" type="checkbox" bind:checked={previewStock} /> In stock only</label
				><label class="flex items-center gap-2"
					><span class="text-sm font-medium text-stone-700 dark:text-stone-200">Sort</span><Dropdown
						tone="admin"
						full={false}
						bind:value={previewSort}
						><option value="">Search default</option><option value="relevance">Relevance</option
						><option value="price">Price</option><option value="name">Name</option><option
							value="created_at">Newest</option
						></Dropdown
					></label
				><Dropdown
					tone="admin"
					full={false}
					aria-label="Preview sort direction"
					bind:value={previewOrder}
					><option value="desc">Descending</option><option value="asc">Ascending</option></Dropdown
				>
			</div>
			<div class="flex flex-wrap gap-2">
				<Button
					tone="admin"
					type="button"
					disabled={previewLoading}
					onclick={() => runPreview(false)}>Preview saved rules</Button
				><Button
					tone="admin"
					type="button"
					disabled={previewLoading}
					onclick={() => runPreview(true)}>Preview unsaved rule</Button
				>
			</div>
		</div>
		{#if previewLoading}<p role="status" class="mt-4">Loading preview…</p>{/if}
		{#if preview}
			{#if previewStale}<p class="mt-4 text-amber-800 dark:text-amber-200">
					Inputs changed. Run preview again to compare the latest changes.
				</p>{/if}
			<div class="mt-5 grid gap-5 md:grid-cols-2">
				{#each [{ title: "baseline", result: preview.baseline }, { title: "proposed", result: preview.proposed }] as side (side.title)}<AdminSurface
						as="div"
						variant="subsurface"
					>
						<h3 class="font-semibold">{side.title}</h3>
						<p class="text-sm text-stone-500">{side.result.pagination.total} results</p>
						<ol class="mt-3 space-y-2">
							{#each side.result.items as product, index (product.id)}
								{@const score = side.result.explanations.find(
									(row) => row.product_id === product.id
								)}
								<li>
									<span class="text-stone-500">{index + 1}.</span>
									{product.name || "Unnamed product"}{#if score}<span
											class="text-xs text-stone-500"
										>
											· score {(score.adjusted_score ?? score.score).toFixed(3)}</span
										>{/if}
								</li>{:else}<li>No matching products.</li>{/each}
						</ol>
					</AdminSurface>{/each}
			</div>
			<div class="mt-5 space-y-2">
				<h3 class="font-semibold">rule decisions and conflicts</h3>
				{#each preview.proposed.rule_decisions as decision, index (index)}<p
						class={decision.outcome === "conflict"
							? "text-sm text-amber-800 dark:text-amber-200"
							: "text-sm text-stone-600 dark:text-stone-300"}
					>
						{decision.rule_name}{decision.product_name ? ` · ${decision.product_name}` : ""}: {decision.outcome}
						· {decision.reason.replaceAll("_", " ")}
					</p>{:else}<p class="text-sm text-stone-500">No rule decisions.</p>{/each}
			</div>
		{/if}
	</AdminPanel>
	<AdminPanel title="rule history">
		{#snippet headerActions()}<Button
				tone="admin"
				type="button"
				disabled={auditLoading}
				onclick={() => loadAudit(null)}>All history</Button
			>{/snippet}
		<p class="mb-3 text-sm text-stone-500">
			{auditRuleId === null
				? "Showing all rules, including deleted rules."
				: "Showing the selected rule."}
		</p>
		{#if auditLoading}<p role="status">Loading history…</p>{/if}
		{#each audit as entry (entry.id)}<AdminSurface as="details" variant="subsurface" class="mb-3">
				<summary class="cursor-pointer text-sm font-medium text-stone-700 dark:text-stone-200"
					>{entry.operation} · {entry.after?.name ?? entry.before?.name ?? "Deleted rule"} · {utcLabel(
						entry.created_at
					)} · {entry.actor_id ? `Actor ${entry.actor_id}` : "System"}</summary
				>
				<div class="mt-3 grid gap-4 md:grid-cols-2">
					<div>
						<p class="font-medium">before</p>
						{@render snapshot(entry.before)}
					</div>
					<div>
						<p class="font-medium">after</p>
						{@render snapshot(entry.after)}
					</div>
				</div>
			</AdminSurface>{:else}<AdminEmptyState>No history available.</AdminEmptyState>{/each}
	</AdminPanel>
</div>
<AdminFloatingNotices
	showUnsaved={savePrompt.dirty}
	unsavedMessage="You have unsaved rule changes."
	canSaveUnsaved={savePrompt.canSave}
	onSaveUnsaved={() => void savePrompt.save()}
	savingUnsaved={savePrompt.saving}
	statusMessage={notices.message}
	statusTone={notices.tone}
	onDismissStatus={notices.clear}
/>
{#if deleteTarget}<AdminConfirmDialog
		title="delete rule"
		message={`Delete “${deleteTarget.name}”? This change takes effect immediately.`}
		confirmLabel="Delete rule"
		{busy}
		onConfirm={() => void remove()}
		onCancel={() => (deleteTarget = null)}
	/>{/if}
