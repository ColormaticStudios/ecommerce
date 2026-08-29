<script lang="ts">
	import { getContext, onMount, untrack } from "svelte";
	import { resolve } from "$app/paths";
	import type { API } from "$lib/api";
	import type { components } from "$lib/api/generated/openapi";
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminFloatingNotices from "$lib/admin/AdminFloatingNotices.svelte";
	import AdminPageHeader from "$lib/admin/AdminPageHeader.svelte";
	import AdminPaginationControls from "$lib/admin/AdminPaginationControls.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import LocalizationAssigneePicker from "$lib/admin/localization/LocalizationAssigneePicker.svelte";
	import { createAdminNotices } from "$lib/admin/state.svelte";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import TabSwitcher, { type TabSwitcherItem } from "$lib/components/TabSwitcher.svelte";
	import TextArea from "$lib/components/TextArea.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
	import { LOCALIZATION_CONTEXT, type LocalizationRuntime } from "$lib/localization/runtime";

	type LocalizationLocale = components["schemas"]["LocalizationLocale"];
	type TranslationQueueItem = components["schemas"]["TranslationQueueItem"];
	type TranslationComment = components["schemas"]["TranslationComment"];
	type TranslationRelease = components["schemas"]["TranslationRelease"];
	type TranslationReleaseQuality = components["schemas"]["TranslationReleaseQuality"];
	type GlossaryTerm = components["schemas"]["LocalizationGlossaryTerm"];
	type ImportReport = components["schemas"]["TranslationImportReport"];
	type DocumentFormat = components["schemas"]["TranslationDocumentFormat"];
	type LocalizationRollout = components["schemas"]["LocalizationRollout"];
	type LocalizationMetrics = components["schemas"]["LocalizationMetricsResponse"];
	type TranslationKeyUsage = components["schemas"]["TranslationKeyUsage"];
	type TranslationKeyUsageInput = components["schemas"]["TranslationKeyUsageInput"];
	type WorkspaceTab = "workspace" | "glossary" | "delivery" | "operations";

	interface Props {
		initialTab?: WorkspaceTab;
	}

	let { initialTab = "workspace" }: Props = $props();

	const api: API = getContext("api");
	const localization = getContext<LocalizationRuntime>(LOCALIZATION_CONTEXT);
	const notices = createAdminNotices();
	const tabs: TabSwitcherItem[] = [
		{ id: "workspace", label: "Workspace", icon: "bi-translate" },
		{ id: "glossary", label: "Glossary", icon: "bi-journal-text" },
		{ id: "delivery", label: "Releases & files", icon: "bi-box-arrow-up" },
		{ id: "operations", label: "Rollout & health", icon: "bi-activity" },
	];
	const namespaces = ["storefront", "checkout", "admin", "errors", "communications"];

	let activeTab = $state<WorkspaceTab>(untrack(() => initialTab));
	let locales = $state<LocalizationLocale[]>([]);
	let locale = $state("");
	let namespace = $state("");
	let stateFilter = $state<"" | "draft" | "review" | "published">("");
	let query = $state("");
	let assignee = $state<number | null>(null);
	let missingOnly = $state(false);
	let staleOnly = $state(false);
	let queue = $state<TranslationQueueItem[]>([]);
	let queuePage = $state(1);
	let queueLimit = $state(25);
	let queueTotal = $state(0);
	let queueTotalPages = $state(1);
	let selectedKeyID = $state<number | null>(null);
	let selectedKeyIDs = $state<number[]>([]);
	let value = $state("");
	let valueAssignee = $state<number | null>(null);
	let changeSummary = $state("");
	let comments = $state<TranslationComment[]>([]);
	let newComment = $state("");
	let releases = $state<TranslationRelease[]>([]);
	let releaseQuality = $state<Record<number, TranslationReleaseQuality>>({});
	let releaseName = $state("");
	let releaseNotes = $state("");
	let glossary = $state<GlossaryTerm[]>([]);
	let glossarySource = $state("");
	let glossaryTranslation = $state("");
	let glossaryDescription = $state("");
	let glossaryLocked = $state(true);
	let documentFormat = $state<DocumentFormat>("json");
	let importContent = $state("");
	let importReport = $state<ImportReport | null>(null);
	let rollouts = $state<LocalizationRollout[]>([]);
	let metrics = $state<LocalizationMetrics | null>(null);
	let loading = $state(true);
	let saving = $state(false);
	let bulkWorking = $state(false);
	let usages = $state<TranslationKeyUsage[]>([]);
	let usageDrafts = $state<TranslationKeyUsageInput[]>([]);
	let uploadingUsage = $state<number | null>(null);

	const selectedItem = $derived(queue.find((item) => item.key.id === selectedKeyID) ?? null);
	const latestValue = $derived(selectedItem?.latest_value ?? null);
	const publishedValue = $derived(selectedItem?.published_value ?? null);
	const dirty = $derived(value !== (latestValue?.value ?? ""));

	function errorMessage(error: unknown, fallback: string): string {
		return error instanceof Error && error.message ? error.message : fallback;
	}

	async function initialize() {
		loading = true;
		try {
			const [localeResponse, releaseResponse, rolloutResponse, metricsResponse] = await Promise.all(
				[
					api.listAdminLocalizationLocales(),
					api.listAdminLocalizationReleases(),
					api.listAdminLocalizationRollouts(),
					api.getAdminLocalizationMetrics(),
				]
			);
			locales = localeResponse.locales.filter((item) => item.is_enabled);
			locale = localeResponse.default_locale || locales[0]?.code || "";
			releases = releaseResponse.releases;
			await loadReleaseQuality();
			rollouts = rolloutResponse.rollouts;
			metrics = metricsResponse;
			await Promise.all([loadQueue(), loadGlossary()]);
		} catch (error) {
			notices.setError(
				errorMessage(
					error,
					$localization.translate(
						"admin.localization.operations.load_error",
						"Unable to load the translation workspace."
					)
				)
			);
		} finally {
			loading = false;
		}
	}

	async function loadQueue(preserveSelection = false, requestedPage = queuePage) {
		if (!locale) return;
		const response = await api.listAdminLocalizationKeys({
			locale,
			namespace: namespace || undefined,
			state: stateFilter || undefined,
			q: query.trim() || undefined,
			assignee_id: assignee ?? undefined,
			missing: missingOnly ? true : undefined,
			stale: staleOnly ? true : undefined,
			page: requestedPage,
			limit: queueLimit,
		});
		const responseTotalPages = Math.max(1, response.pagination.total_pages);
		if (response.pagination.total > 0 && requestedPage > responseTotalPages) {
			await loadQueue(preserveSelection, responseTotalPages);
			return;
		}
		queue = response.items;
		queuePage = response.pagination.page;
		queueTotal = response.pagination.total;
		queueTotalPages = responseTotalPages;
		selectedKeyIDs = selectedKeyIDs.filter((id) => queue.some((item) => item.key.id === id));
		if (!preserveSelection || !queue.some((item) => item.key.id === selectedKeyID)) {
			await selectItem(queue[0] ?? null);
		}
	}

	async function applyFilters() {
		loading = true;
		try {
			queuePage = 1;
			await Promise.all([loadQueue(false, 1), loadGlossary()]);
		} catch (error) {
			notices.setError(errorMessage(error, "Unable to filter translations."));
		} finally {
			loading = false;
		}
	}

	async function changeQueuePage(page: number) {
		if (loading || page < 1 || page > queueTotalPages || page === queuePage) return;
		loading = true;
		try {
			await loadQueue(false, page);
		} catch (error) {
			notices.setError(errorMessage(error, "Unable to load that page of translations."));
		} finally {
			loading = false;
		}
	}

	async function changeQueueLimit(limit: number) {
		if (loading || limit === queueLimit) return;
		queueLimit = limit;
		queuePage = 1;
		loading = true;
		try {
			await loadQueue(false, 1);
		} catch (error) {
			notices.setError(errorMessage(error, "Unable to change the translation page size."));
		} finally {
			loading = false;
		}
	}

	function handleSearchKeydown(event: KeyboardEvent) {
		if (event.key !== "Enter") return;
		event.preventDefault();
		void applyFilters();
	}

	async function selectItem(item: TranslationQueueItem | null) {
		selectedKeyID = item?.key.id ?? null;
		value = item?.latest_value?.value ?? "";
		valueAssignee = item?.latest_value?.assignee_id ?? null;
		changeSummary = "";
		comments = [];
		usages = [];
		usageDrafts = [];
		if (item) {
			try {
				usages = (await api.listAdminLocalizationKeyUsages(item.key.id)).usages;
				usageDrafts = usages.map((usage) => ({
					route: usage.route,
					component: usage.component,
					description: usage.description,
					position: usage.position,
					screenshot_media_id: usage.screenshot_media_id,
				}));
			} catch (error) {
				notices.setError(errorMessage(error, "Unable to load translation usage context."));
			}
		}
		if (item?.latest_value) {
			try {
				comments = (await api.listAdminLocalizationComments(item.latest_value.id)).comments;
			} catch (error) {
				notices.setError(errorMessage(error, "Unable to load review comments."));
			}
		}
	}

	function addUsage() {
		usageDrafts = [
			...usageDrafts,
			{ route: "", component: "", description: "", position: usageDrafts.length },
		];
		usages = [...usages];
	}

	function removeUsage(index: number) {
		usageDrafts = usageDrafts
			.filter((_, candidate) => candidate !== index)
			.map((usage, position) => ({ ...usage, position }));
		usages = usages.filter((_, candidate) => candidate !== index);
	}

	async function uploadUsageScreenshot(index: number, event: Event) {
		const file = (event.currentTarget as HTMLInputElement).files?.[0];
		if (!file) return;
		uploadingUsage = index;
		try {
			const mediaID = await api.uploadMedia(file);
			usageDrafts[index] = { ...usageDrafts[index], screenshot_media_id: mediaID };
			usageDrafts = [...usageDrafts];
			notices.setSuccess("Screenshot uploaded. Save context to attach it.");
		} catch (error) {
			notices.setError(errorMessage(error, "Unable to upload the usage screenshot."));
		} finally {
			uploadingUsage = null;
		}
	}

	async function saveUsages() {
		if (!selectedItem) return;
		saving = true;
		try {
			const response = await api.replaceAdminLocalizationKeyUsages(selectedItem.key.id, {
				usages: usageDrafts,
			});
			usages = response.usages;
			usageDrafts = response.usages.map((usage) => ({
				route: usage.route,
				component: usage.component,
				description: usage.description,
				position: usage.position,
				screenshot_media_id: usage.screenshot_media_id,
			}));
			notices.setSuccess("Translation usage context saved.");
		} catch (error) {
			notices.setError(errorMessage(error, "Unable to save translation usage context."));
		} finally {
			saving = false;
		}
	}

	async function saveDraft() {
		if (!selectedItem || !value.trim()) return;
		saving = true;
		try {
			await api.putAdminLocalizationValue(selectedItem.key.id, locale, {
				value: value.trim(),
				assignee_id: valueAssignee,
				expected_version: latestValue?.version ?? 0,
				change_summary: changeSummary.trim() || undefined,
			});
			await loadQueue(true);
			await selectItem(queue.find((item) => item.key.id === selectedKeyID) ?? null);
			notices.setSuccess("Draft translation saved.");
		} catch (error) {
			notices.setError(errorMessage(error, "Unable to save the draft translation."));
		} finally {
			saving = false;
		}
	}

	async function transitionSelected(target: "review" | "published") {
		if (!latestValue) return;
		saving = true;
		try {
			if (target === "review") {
				await api.submitAdminLocalizationValueReview(latestValue.id, {
					change_summary: changeSummary.trim() || undefined,
				});
			} else {
				await api.publishAdminLocalizationValue(latestValue.id, {
					change_summary: changeSummary.trim() || undefined,
				});
			}
			await loadQueue(true);
			await selectItem(queue.find((item) => item.key.id === selectedKeyID) ?? null);
			notices.setSuccess(
				target === "review" ? "Translation submitted for review." : "Translation published."
			);
		} catch (error) {
			notices.setError(errorMessage(error, "Unable to change the translation state."));
		} finally {
			saving = false;
		}
	}

	async function bulkTransition(target: "review" | "published") {
		bulkWorking = true;
		let completed = 0;
		try {
			for (const item of queue.filter((candidate) => selectedKeyIDs.includes(candidate.key.id))) {
				const candidate = item.latest_value;
				if (!candidate) continue;
				if (target === "review" && candidate.state === "draft") {
					await api.submitAdminLocalizationValueReview(candidate.id, {});
					completed += 1;
				}
				if (target === "published" && candidate.state === "review") {
					await api.publishAdminLocalizationValue(candidate.id, {});
					completed += 1;
				}
			}
			await loadQueue(true);
			notices.setSuccess(`${completed} translation${completed === 1 ? "" : "s"} updated.`);
		} catch (error) {
			notices.setError(errorMessage(error, "Unable to complete the bulk action."));
		} finally {
			bulkWorking = false;
		}
	}

	function toggleSelection(id: number) {
		selectedKeyIDs = selectedKeyIDs.includes(id)
			? selectedKeyIDs.filter((candidate) => candidate !== id)
			: [...selectedKeyIDs, id];
	}

	async function addComment() {
		if (!latestValue || !newComment.trim()) return;
		try {
			await api.createAdminLocalizationComment(latestValue.id, newComment.trim());
			newComment = "";
			comments = (await api.listAdminLocalizationComments(latestValue.id)).comments;
			notices.setSuccess("Review comment added.");
		} catch (error) {
			notices.setError(errorMessage(error, "Unable to add the review comment."));
		}
	}

	async function loadGlossary() {
		if (!locale) return;
		glossary = (await api.listAdminLocalizationGlossary(locale)).terms;
	}

	async function saveGlossaryTerm() {
		try {
			await api.putAdminLocalizationGlossaryTerm({
				locale,
				source_term: glossarySource.trim(),
				translated_term: glossaryTranslation.trim(),
				description: glossaryDescription.trim() || undefined,
				is_locked: glossaryLocked,
			});
			glossarySource = glossaryTranslation = glossaryDescription = "";
			await loadGlossary();
			notices.setSuccess("Glossary term saved.");
		} catch (error) {
			notices.setError(errorMessage(error, "Unable to save the glossary term."));
		}
	}

	async function removeGlossaryTerm(id: number) {
		try {
			await api.deleteAdminLocalizationGlossaryTerm(id);
			await loadGlossary();
			notices.setSuccess("Glossary term deleted.");
		} catch (error) {
			notices.setError(errorMessage(error, "Unable to delete the glossary term."));
		}
	}

	async function createRelease() {
		try {
			await api.createAdminLocalizationRelease({
				name: releaseName.trim(),
				notes: releaseNotes.trim() || undefined,
			});
			releaseName = releaseNotes = "";
			await reloadReleases();
			notices.setSuccess("Release snapshot created.");
		} catch (error) {
			notices.setError(errorMessage(error, "Unable to create the release."));
		}
	}

	async function activateRelease(id: number) {
		try {
			await api.activateAdminLocalizationRelease(id);
			await reloadReleases();
			notices.setSuccess("Translation release activated.");
		} catch (error) {
			notices.setError(errorMessage(error, "Unable to activate the release."));
		}
	}

	async function rollbackRelease(id: number) {
		try {
			await api.rollbackAdminLocalizationRelease(id);
			await reloadReleases();
			notices.setSuccess("Prior translation snapshot restored.");
		} catch (error) {
			notices.setError(errorMessage(error, "Unable to restore the prior release."));
		}
	}

	async function loadReleaseQuality() {
		const reports = await Promise.all(
			releases.map(
				async (release) =>
					[release.id, await api.getAdminLocalizationReleaseQuality(release.id)] as const
			)
		);
		releaseQuality = Object.fromEntries(reports);
	}

	async function reloadReleases() {
		releases = (await api.listAdminLocalizationReleases()).releases;
		await loadReleaseQuality();
	}

	async function exportDocument() {
		try {
			const document = await api.exportAdminLocalization({
				locale,
				namespace: namespace || undefined,
				format: documentFormat,
			});
			const blob = new Blob([document.content], { type: "text/plain;charset=utf-8" });
			const url = URL.createObjectURL(blob);
			const anchor = window.document.createElement("a");
			anchor.href = url;
			anchor.download = document.filename;
			anchor.click();
			URL.revokeObjectURL(url);
			notices.setSuccess("Translation export downloaded.");
		} catch (error) {
			notices.setError(errorMessage(error, "Unable to export translations."));
		}
	}

	async function importDocument(dryRun: boolean) {
		try {
			importReport = await api.importAdminLocalization({
				locale,
				namespace: namespace || undefined,
				format: documentFormat,
				content: importContent,
				dry_run: dryRun,
			});
			if (!dryRun) await loadQueue(true);
			notices.setSuccess(dryRun ? "Import validation complete." : "Import complete.");
		} catch (error) {
			notices.setError(errorMessage(error, "Unable to import translations."));
		}
	}

	async function readImportFile(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		if (file) importContent = await file.text();
	}

	function updateRolloutEnabled(index: number, enabled: boolean) {
		rollouts[index] = { ...rollouts[index], is_enabled: enabled };
	}

	function updateRolloutPercentage(index: number, percentage: number) {
		rollouts[index] = { ...rollouts[index], percentage };
	}

	async function saveRollouts() {
		saving = true;
		try {
			const response = await api.replaceAdminLocalizationRollouts({
				rollouts: rollouts.map(({ locale, domain, is_enabled, percentage }) => ({
					locale,
					domain,
					is_enabled,
					percentage,
				})),
			});
			rollouts = response.rollouts;
			notices.setSuccess(
				$localization.translate(
					"admin.localization.operations.save_confirmation",
					"Localization rollout controls saved."
				)
			);
		} catch (error) {
			notices.setError(
				errorMessage(
					error,
					$localization.translate(
						"admin.localization.operations.save_error",
						"Unable to save localization rollout controls."
					)
				)
			);
		} finally {
			saving = false;
		}
	}

	async function refreshMetrics() {
		try {
			metrics = await api.getAdminLocalizationMetrics();
		} catch (error) {
			notices.setError(
				errorMessage(
					error,
					$localization.translate(
						"admin.localization.operations.metrics_error",
						"Unable to refresh localization health metrics."
					)
				)
			);
		}
	}

	function metricLabel(type: components["schemas"]["LocalizationMetricType"]): string {
		return type.replaceAll("_", " ");
	}

	function handleShortcut(event: KeyboardEvent) {
		if (!(event.metaKey || event.ctrlKey) || event.key !== "Enter" || activeTab !== "workspace")
			return;
		event.preventDefault();
		if (event.shiftKey && latestValue?.state === "draft" && !dirty) {
			void transitionSelected("review");
			return;
		}
		if (dirty) void saveDraft();
	}

	onMount(() => {
		void initialize();
	});
</script>

<svelte:window onkeydown={handleShortcut} />

<div class="space-y-6 pb-20">
	<AdminPageHeader title="Translations">
		{#snippet actions()}
			<Badge
				tone={queue.some((item) => item.missing || item.stale) ? "warning" : "success"}
				size="md"
			>
				{queue.filter((item) => item.missing || item.stale).length} on this page need attention
			</Badge>
		{/snippet}
	</AdminPageHeader>

	<TabSwitcher items={tabs} bind:value={activeTab} ariaLabel="Translation workspace sections" />

	{#if activeTab === "workspace"}
		<div class="grid min-h-[34rem] gap-5 xl:grid-cols-[minmax(17rem,0.75fr)_minmax(0,1.5fr)]">
			<AdminPanel title="Translation queue" meta={loading ? "Loading…" : `${queueTotal} keys`}>
				<div class="grid gap-3 md:grid-cols-2">
					<TextInput
						tone="admin"
						bind:value={query}
						placeholder="Search translation keys or source text"
						aria-label="Search translation keys"
						onkeydown={handleSearchKeydown}
						class="md:col-span-2"
					/>
					<Dropdown tone="admin" bind:value={locale} aria-label="Target locale">
						{#each locales as item (item.code)}<option value={item.code}>{item.name}</option>{/each}
					</Dropdown>
					<Dropdown tone="admin" bind:value={namespace} aria-label="Namespace">
						<option value="">All namespaces</option>
						{#each namespaces as item (item)}<option value={item}>{item}</option>{/each}
					</Dropdown>
					<Dropdown tone="admin" bind:value={stateFilter} aria-label="Workflow state">
						<option value="">All states</option>
						<option value="draft">Draft</option><option value="review">Review</option><option
							value="published">Published</option
						>
					</Dropdown>
					<LocalizationAssigneePicker
						id="localization-assignee-filter"
						bind:value={assignee}
						placeholder="Filter by assignee"
						ariaLabel="Filter by assignee"
					/>
					<Button
						class="md:col-span-2"
						tone="admin"
						variant="primary"
						onclick={applyFilters}
						disabled={loading}>Apply filters</Button
					>
				</div>
				<div
					class="mt-3 flex flex-wrap items-center gap-4 text-sm text-stone-600 dark:text-stone-300"
				>
					<label class="inline-flex items-center gap-2"
						><input type="checkbox" bind:checked={missingOnly} /> Missing only</label
					>
					<label class="inline-flex items-center gap-2"
						><input type="checkbox" bind:checked={staleOnly} /> Stale only</label
					>
				</div>
				{#if selectedKeyIDs.length}
					<div class="mt-3 flex flex-wrap items-center gap-2">
						<span class="mr-auto text-sm text-stone-600 dark:text-stone-300"
							>{selectedKeyIDs.length} selected</span
						>
						<Button
							tone="admin"
							size="small"
							onclick={() => bulkTransition("review")}
							disabled={bulkWorking}>Submit selected</Button
						>
						<Button
							tone="admin"
							size="small"
							variant="success"
							onclick={() => bulkTransition("published")}
							disabled={bulkWorking}>Publish selected</Button
						>
					</div>
				{/if}

				{#if !loading && queue.length === 0}
					<div class="mt-5"><AdminEmptyState>No keys match these filters.</AdminEmptyState></div>
				{:else}
					<div
						class="mt-5 max-h-[44rem] divide-y divide-stone-200 overflow-y-auto rounded-lg border border-stone-200 bg-white dark:divide-stone-800 dark:border-stone-800 dark:bg-stone-950"
					>
						{#each queue as item (item.key.id)}
							<div
								class={`p-3 transition ${selectedKeyID === item.key.id ? "bg-stone-100 dark:bg-stone-800" : "hover:bg-stone-50 dark:hover:bg-stone-900"}`}
							>
								<div class="flex items-start gap-2">
									<input
										type="checkbox"
										checked={selectedKeyIDs.includes(item.key.id)}
										onchange={() => toggleSelection(item.key.id)}
										aria-label={`Select ${item.key.key}`}
									/>
									<button
										type="button"
										class="min-w-0 flex-1 cursor-pointer text-left"
										onclick={() => selectItem(item)}
									>
										<span
											class="block truncate text-xs font-semibold text-stone-950 dark:text-stone-50"
											>{item.key.key}</span
										>
										<span class="mt-1 line-clamp-2 block text-xs text-stone-500 dark:text-stone-400"
											>{item.key.source_text}</span
										>
									</button>
									<Badge
										tone={item.missing
											? "danger"
											: item.stale
												? "warning"
												: item.latest_value?.state === "published"
													? "success"
													: "info"}
										size="xs"
									>
										{item.missing
											? "missing"
											: item.stale
												? "stale"
												: (item.latest_value?.state ?? "new")}
									</Badge>
								</div>
							</div>
						{/each}
					</div>
				{/if}
				{#if queueTotal > 0}
					<div class="mt-4">
						<AdminPaginationControls
							page={queuePage}
							totalPages={queueTotalPages}
							totalItems={queueTotal}
							limit={queueLimit}
							limitOptions={[10, 25, 50]}
							onLimitChange={(limit) => void changeQueueLimit(limit)}
							onPrev={() => void changeQueuePage(queuePage - 1)}
							onNext={() => void changeQueuePage(queuePage + 1)}
						/>
					</div>
				{/if}
			</AdminPanel>

			<AdminPanel
				title={selectedItem
					? `${selectedItem.key.namespace}.${selectedItem.key.key}`
					: "Translation editor"}
				meta={latestValue ? `Version ${latestValue.version}` : "New translation"}
			>
				{#if !selectedItem}
					<AdminEmptyState>Select a translation key to begin.</AdminEmptyState>
				{:else}
					<div class="grid gap-5 lg:grid-cols-2">
						<div>
							<p class="mb-2 text-xs font-semibold tracking-wide text-stone-500 uppercase">
								Source
							</p>
							<div
								class="min-h-36 rounded-xl border border-stone-200 bg-stone-50 p-4 text-sm whitespace-pre-wrap text-stone-900 dark:border-stone-800 dark:bg-stone-900 dark:text-stone-100"
							>
								{selectedItem.key.source_text}
							</div>
							<p class="mt-2 text-xs text-stone-500">
								{selectedItem.key.description || "No translator note."}
							</p>
						</div>
						<div>
							<p class="mb-2 text-xs font-semibold tracking-wide text-stone-500 uppercase">
								{locale}
							</p>
							<TextArea
								tone="admin"
								bind:value
								rows={7}
								placeholder="Enter the translated text"
								aria-label="Translated text"
							/>
						</div>
					</div>

					{#if publishedValue && publishedValue.value !== value}
						<div
							class="mt-5 rounded-xl border border-sky-200 bg-sky-50/60 p-4 dark:border-sky-900 dark:bg-sky-950/30"
						>
							<p
								class="text-xs font-semibold tracking-wide text-sky-700 uppercase dark:text-sky-200"
							>
								Published → current draft
							</p>
							<div class="mt-2 grid gap-3 text-sm md:grid-cols-2">
								<div class="line-through opacity-70">{publishedValue.value}</div>
								<div>{value || "Empty"}</div>
							</div>
						</div>
					{/if}

					{#if selectedItem.validation_issues.length}
						<div class="mt-5 space-y-2">
							{#each selectedItem.validation_issues as issue (issue.code + issue.path)}<p
									class="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-200"
								>
									{issue.detail}
								</p>{/each}
						</div>
					{/if}

					<div class="mt-5 grid gap-3 md:grid-cols-2">
						{#key selectedKeyID}
							<LocalizationAssigneePicker
								id={`localization-assignee-editor-${selectedKeyID ?? "none"}`}
								bind:value={valueAssignee}
								placeholder="Assign to a person"
								ariaLabel="Translation assignee"
							/>
						{/key}
						<TextInput
							tone="admin"
							bind:value={changeSummary}
							placeholder="Change summary"
							aria-label="Change summary"
						/>
					</div>
					<div class="mt-4 flex flex-wrap gap-2">
						<Button
							tone="admin"
							variant="primary"
							onclick={saveDraft}
							disabled={saving || !dirty || !value.trim()}>Save draft</Button
						>
						<Button
							tone="admin"
							onclick={() => transitionSelected("review")}
							disabled={saving || dirty || latestValue?.state !== "draft"}>Submit for review</Button
						>
						<Button
							tone="admin"
							variant="success"
							onclick={() => transitionSelected("published")}
							disabled={saving || dirty || latestValue?.state !== "review"}>Publish</Button
						>
						<a
							class="inline-flex items-center rounded-lg border border-stone-300 px-4 py-2 text-sm text-stone-700 dark:border-stone-700 dark:text-stone-200"
							href={resolve(selectedItem.preview_url as "/")}
							target="_blank"
							rel="noreferrer">Preview</a
						>
					</div>
					<p class="mt-2 text-xs text-stone-500">
						Save: Ctrl/Command + Enter · Submit: Ctrl/Command + Shift + Enter
					</p>

					<div class="mt-7 border-t border-stone-200 pt-5 dark:border-stone-800">
						<div class="flex items-center justify-between gap-3">
							<h3 class="text-sm font-semibold text-stone-950 dark:text-stone-50">Usage context</h3>
							<Button tone="admin" size="small" onclick={addUsage}>Add location</Button>
						</div>
						<div class="mt-3 space-y-3">
							{#each usageDrafts as usage, index (index)}
								<div class="rounded-xl border border-stone-200 p-3 dark:border-stone-800">
									<div class="grid gap-2 md:grid-cols-2">
										<TextInput
											tone="admin"
											bind:value={usage.route}
											placeholder="Route, for example /cart"
										/>
										<TextInput
											tone="admin"
											bind:value={usage.component}
											placeholder="Component or source file"
										/>
									</div>
									<TextArea
										class="mt-2"
										tone="admin"
										bind:value={usage.description}
										rows={2}
										placeholder="Explain where and how this copy appears"
									/>
									{#if usages[index]?.screenshot_url}<img
											class="mt-3 max-h-56 rounded-lg border border-stone-200 object-contain dark:border-stone-800"
											src={usages[index].screenshot_url ?? ""}
											alt="Translation usage screenshot"
										/>{/if}
									<div class="mt-3 flex flex-wrap items-center gap-2">
										<label
											class="cursor-pointer rounded-lg border border-stone-300 px-3 py-2 text-xs dark:border-stone-700"
										>
											{uploadingUsage === index ? "Uploading…" : "Upload screenshot"}
											<input
												class="sr-only"
												type="file"
												accept="image/*"
												onchange={(event) => void uploadUsageScreenshot(index, event)}
											/>
										</label>
										<Button
											tone="admin"
											size="small"
											variant="danger"
											onclick={() => removeUsage(index)}>Remove</Button
										>
									</div>
								</div>
							{/each}
						</div>
						{#if usageDrafts.length}<Button
								class="mt-3"
								tone="admin"
								variant="primary"
								onclick={saveUsages}
								disabled={saving}>Save context</Button
							>{/if}
					</div>

					<div class="mt-7 border-t border-stone-200 pt-5 dark:border-stone-800">
						<h3 class="text-sm font-semibold text-stone-950 dark:text-stone-50">Review comments</h3>
						<div class="mt-3 space-y-2">
							{#each comments as comment (comment.id)}<div
									class="rounded-lg bg-stone-100 px-3 py-2 text-sm dark:bg-stone-900"
								>
									<p>{comment.comment}</p>
									<p class="mt-1 text-[11px] text-stone-500">
										{comment.author_name} · {new Date(comment.created_at).toLocaleString()}
									</p>
								</div>{/each}
						</div>
						<div class="mt-3 flex gap-2">
							<TextInput
								tone="admin"
								bind:value={newComment}
								placeholder="Add a review comment"
								aria-label="Review comment"
							/><Button
								tone="admin"
								onclick={addComment}
								disabled={!latestValue || !newComment.trim()}>Comment</Button
							>
						</div>
					</div>
				{/if}
			</AdminPanel>
		</div>
	{:else if activeTab === "glossary"}
		<div class="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(20rem,0.6fr)]">
			<AdminPanel title="Terminology" meta={`${glossary.length} terms`}>
				{#if glossary.length === 0}<AdminEmptyState
						>No glossary terms exist for {locale}.</AdminEmptyState
					>{:else}
					<div class="space-y-2">
						{#each glossary as term (term.id)}<div
								class="flex items-center gap-3 rounded-xl border border-stone-200 p-3 dark:border-stone-800"
							>
								<div class="min-w-0 flex-1">
									<p class="font-medium text-stone-950 dark:text-stone-50">
										{term.source_term} → {term.translated_term}
									</p>
									<p class="text-xs text-stone-500">{term.description || "No note"}</p>
								</div>
								{#if term.is_locked}<Badge tone="warning" size="xs">Locked</Badge>{/if}<Button
									tone="admin"
									size="small"
									variant="danger"
									onclick={() => removeGlossaryTerm(term.id)}>Delete</Button
								>
							</div>{/each}
					</div>
				{/if}
			</AdminPanel>
			<AdminPanel title="Add or update a term">
				<div class="space-y-3">
					<TextInput tone="admin" bind:value={glossarySource} placeholder="Source term" /><TextInput
						tone="admin"
						bind:value={glossaryTranslation}
						placeholder="Required translation"
					/><TextArea
						tone="admin"
						bind:value={glossaryDescription}
						rows={3}
						placeholder="Translator guidance"
					/><label class="flex items-center gap-2 text-sm"
						><input type="checkbox" bind:checked={glossaryLocked} /> Enforce this translation during review</label
					><Button
						tone="admin"
						variant="primary"
						onclick={saveGlossaryTerm}
						disabled={!glossarySource.trim() || !glossaryTranslation.trim()}>Save term</Button
					>
				</div>
			</AdminPanel>
		</div>
	{:else if activeTab === "delivery"}
		<div class="grid gap-5 xl:grid-cols-2">
			<AdminPanel title="Translation releases" meta={`${releases.length} snapshots`}>
				<div class="grid gap-2 sm:grid-cols-[1fr_1fr_auto]">
					<TextInput tone="admin" bind:value={releaseName} placeholder="Release name" /><TextInput
						tone="admin"
						bind:value={releaseNotes}
						placeholder="Release notes"
					/><Button
						tone="admin"
						variant="primary"
						onclick={createRelease}
						disabled={!releaseName.trim()}>Create snapshot</Button
					>
				</div>
				<div class="mt-5 space-y-2">
					{#each releases as release (release.id)}<div
							class="flex items-center gap-3 rounded-xl border border-stone-200 p-3 dark:border-stone-800"
						>
							<div class="min-w-0 flex-1">
								<p class="font-medium text-stone-950 dark:text-stone-50">{release.name}</p>
								<p class="truncate text-xs text-stone-500">{release.snapshot_hash}</p>
							</div>
							<Badge
								tone={release.status === "active"
									? "success"
									: release.status === "draft"
										? "info"
										: "neutral"}>{release.status}</Badge
							>{#if releaseQuality[release.id]}<Badge
									tone={releaseQuality[release.id].ready ? "success" : "warning"}
									size="xs"
									>{releaseQuality[release.id].ready
										? "Ready"
										: `${releaseQuality[release.id].missing_count} missing`}</Badge
								>{/if}{#if release.status === "draft"}<Button
									tone="admin"
									size="small"
									onclick={() => activateRelease(release.id)}
									disabled={!releaseQuality[release.id]?.ready}>Activate</Button
								>{:else if release.status === "superseded"}<Button
									tone="admin"
									size="small"
									onclick={() => rollbackRelease(release.id)}
									disabled={!releaseQuality[release.id]?.ready}>Restore</Button
								>{/if}
						</div>{/each}
				</div>
			</AdminPanel>
			<AdminPanel title="Import and export" meta={documentFormat.toUpperCase()}>
				<div class="flex flex-wrap gap-2">
					<Dropdown
						tone="admin"
						full={false}
						bind:value={documentFormat}
						aria-label="Translation document format"
						><option value="json">JSON</option><option value="csv">CSV</option><option value="xliff"
							>XLIFF 2.0</option
						></Dropdown
					><Button tone="admin" onclick={exportDocument}>Export current selection</Button><input
						type="file"
						accept=".json,.csv,.xlf,.xliff"
						onchange={readImportFile}
						class="text-sm text-stone-600 dark:text-stone-300"
					/>
				</div>
				<TextArea
					tone="admin"
					bind:value={importContent}
					rows={10}
					class="mt-4 font-mono text-xs"
					placeholder="Paste or choose a translation document"
				/>
				<div class="mt-3 flex gap-2">
					<Button tone="admin" onclick={() => importDocument(true)} disabled={!importContent.trim()}
						>Validate import</Button
					><Button
						tone="admin"
						variant="primary"
						onclick={() => importDocument(false)}
						disabled={!importContent.trim()}>Import drafts</Button
					>
				</div>
				{#if importReport}<div class="mt-4 grid grid-cols-4 gap-2 text-center text-xs">
						<div class="rounded-lg bg-emerald-50 p-2 dark:bg-emerald-950/40">
							<strong class="block text-lg">{importReport.created}</strong>Created
						</div>
						<div class="rounded-lg bg-stone-100 p-2 dark:bg-stone-900">
							<strong class="block text-lg">{importReport.unchanged}</strong>Unchanged
						</div>
						<div class="rounded-lg bg-amber-50 p-2 dark:bg-amber-950/40">
							<strong class="block text-lg">{importReport.conflicts}</strong>Conflicts
						</div>
						<div class="rounded-lg bg-rose-50 p-2 dark:bg-rose-950/40">
							<strong class="block text-lg">{importReport.invalid}</strong>Invalid
						</div>
					</div>{/if}
			</AdminPanel>
		</div>
	{:else}
		<div class="grid gap-5 xl:grid-cols-[minmax(0,1.35fr)_minmax(20rem,0.65fr)]">
			<AdminPanel title="Locale rollout controls" meta={`${rollouts.length} gates`}>
				<p class="mb-4 text-sm text-stone-600 dark:text-stone-300">
					Rollouts are evaluated deterministically per account or browser session. The default
					locale remains fully enabled so every gated request has a safe fallback.
				</p>
				<div class="overflow-x-auto">
					<table class="w-full min-w-[42rem] text-left text-sm">
						<thead class="text-xs tracking-wide text-stone-500 uppercase">
							<tr>
								<th class="px-3 py-2">Locale</th>
								<th class="px-3 py-2">Domain</th>
								<th class="px-3 py-2">Enabled</th>
								<th class="px-3 py-2">Audience</th>
							</tr>
						</thead>
						<tbody>
							{#each rollouts as rollout, index (`${rollout.locale}-${rollout.domain}`)}
								{@const localeRecord = locales.find((item) => item.code === rollout.locale)}
								{@const locked = Boolean(localeRecord?.is_default)}
								<tr class="border-t border-stone-200 dark:border-stone-800">
									<td class="px-3 py-3 font-medium">
										{rollout.locale}
										{#if locked}<Badge tone="info" size="xs">Default</Badge>{/if}
									</td>
									<td class="px-3 py-3 capitalize">{rollout.domain}</td>
									<td class="px-3 py-3">
										<input
											type="checkbox"
											checked={rollout.is_enabled}
											disabled={locked}
											aria-label={`Enable ${rollout.locale} for ${rollout.domain}`}
											onchange={(event) =>
												updateRolloutEnabled(
													index,
													(event.currentTarget as HTMLInputElement).checked
												)}
										/>
									</td>
									<td class="px-3 py-3">
										<Dropdown
											tone="admin"
											full={false}
											value={rollout.percentage}
											disabled={locked || !rollout.is_enabled}
											aria-label={`${rollout.locale} ${rollout.domain} rollout percentage`}
											onchange={(event) =>
												updateRolloutPercentage(
													index,
													Number((event.currentTarget as HTMLSelectElement).value)
												)}
										>
											{#each [0, 10, 25, 50, 75, 100] as percentage (percentage)}
												<option value={percentage}>{percentage}%</option>
											{/each}
										</Dropdown>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
				<div class="mt-4 flex justify-end">
					<Button tone="admin" variant="primary" onclick={saveRollouts} disabled={saving}
						>Save rollout controls</Button
					>
				</div>
			</AdminPanel>

			<div class="space-y-5">
				<AdminPanel title="Localization health" meta="Runtime totals">
					{#if metrics}
						<div class="grid grid-cols-2 gap-3">
							<div class="rounded-xl bg-stone-100 p-3 dark:bg-stone-900">
								<p class="text-xs text-stone-500">Missing keys</p>
								<p class="mt-1 text-2xl font-semibold">
									{(metrics.missing_key_rate * 100).toFixed(1)}%
								</p>
								<p class="text-xs text-stone-500">{metrics.missing_key_count} messages</p>
							</div>
							<div class="rounded-xl bg-stone-100 p-3 dark:bg-stone-900">
								<p class="text-xs text-stone-500">Fallback hits</p>
								<p class="mt-1 text-2xl font-semibold">
									{(metrics.fallback_hit_rate * 100).toFixed(1)}%
								</p>
								<p class="text-xs text-stone-500">{metrics.fallback_hit_count} messages</p>
							</div>
							<div class="rounded-xl bg-stone-100 p-3 dark:bg-stone-900">
								<p class="text-xs text-stone-500">Rollout fallbacks</p>
								<p class="mt-1 text-2xl font-semibold">{metrics.rollout_fallback_count}</p>
							</div>
							<div class="rounded-xl bg-stone-100 p-3 dark:bg-stone-900">
								<p class="text-xs text-stone-500">Average publish latency</p>
								<p class="mt-1 text-2xl font-semibold">
									{Math.round(metrics.average_publish_latency_ms / 1000)}s
								</p>
							</div>
						</div>
						{#if metrics.locale_rates.length}
							<div class="mt-4 space-y-2">
								{#each metrics.locale_rates as rate (rate.locale)}
									<div
										class="grid grid-cols-[1fr_auto_auto] gap-3 rounded-xl border border-stone-200 p-3 text-xs dark:border-stone-800"
									>
										<strong>{rate.locale}</strong>
										<span>{(rate.missing_key_rate * 100).toFixed(1)}% missing</span>
										<span>{(rate.fallback_hit_rate * 100).toFixed(1)}% fallback</span>
									</div>
								{/each}
							</div>
						{/if}
						<div class="mt-4 flex justify-end">
							<Button tone="admin" size="small" onclick={refreshMetrics}>Refresh</Button>
						</div>
					{:else}
						<AdminEmptyState>No localization metrics have been recorded.</AdminEmptyState>
					{/if}
				</AdminPanel>

				<AdminPanel title="Hotspots" meta={`${metrics?.hotspots.length ?? 0} signals`}>
					{#if metrics?.hotspots.length}
						<div class="space-y-2">
							{#each metrics.hotspots as hotspot (`${hotspot.metric_type}-${hotspot.locale}-${hotspot.domain}-${hotspot.key}`)}
								<div class="rounded-xl border border-stone-200 p-3 dark:border-stone-800">
									<div class="flex items-center justify-between gap-3">
										<p class="text-sm font-medium capitalize">{metricLabel(hotspot.metric_type)}</p>
										<Badge
											tone={hotspot.metric_type === "missing_key" ? "danger" : "warning"}
											size="xs">{hotspot.count}</Badge
										>
									</div>
									<p class="mt-1 text-xs break-all text-stone-500">
										{hotspot.locale || "all locales"} · {hotspot.domain ||
											"all domains"}{hotspot.key ? ` · ${hotspot.key}` : ""}
									</p>
								</div>
							{/each}
						</div>
					{:else}
						<AdminEmptyState>No localization hotspots detected.</AdminEmptyState>
					{/if}
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
