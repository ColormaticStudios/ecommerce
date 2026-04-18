<script lang="ts">
	import { getContext, onMount } from "svelte";
	import { type API } from "$lib/api";
	import {
		formatProviderCurrencies,
		parseProviderCurrencies,
		parseProviderSecretData,
		providerRunbooks,
		summarizeReconciliationMismatch,
	} from "$lib/admin/providers";
	import AdminFloatingNotices from "$lib/admin/AdminFloatingNotices.svelte";
	import AdminMasterDetailLayout from "$lib/admin/AdminMasterDetailLayout.svelte";
	import AdminPageHeader from "$lib/admin/AdminPageHeader.svelte";
	import AdminPaginationControls from "$lib/admin/AdminPaginationControls.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import AdminResourceActions from "$lib/admin/AdminResourceActions.svelte";
	import { createAdminNotices } from "$lib/admin/state.svelte";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import Dropdown from "$lib/components/Dropdown.svelte";
	import TabSwitcher from "$lib/components/TabSwitcher.svelte";
	import TextArea from "$lib/components/TextArea.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
	import type { components } from "$lib/api/generated/openapi";
	import type { PageData } from "./$types";

	interface Props {
		data: PageData;
	}

	type CheckoutPluginCatalog = components["schemas"]["CheckoutPluginCatalog"];
	type CheckoutPlugin = components["schemas"]["CheckoutPlugin"];
	type CheckoutPluginType = CheckoutPlugin["type"];
	type ProviderCredential = components["schemas"]["ProviderCredential"];
	type ProviderCredentialEnvironment = ProviderCredential["environment"];
	type ProviderCredentialFxMode = ProviderCredential["fx_mode"];
	type ProviderOperationsOverview = components["schemas"]["ProviderOperationsOverview"];
	type ProviderReconciliationRun = components["schemas"]["ProviderReconciliationRun"];
	type ProviderReconciliationRunPage = components["schemas"]["ProviderReconciliationRunPage"];
	type WebhookEventPage = components["schemas"]["WebhookEventPage"];
	type WebhookEventRecord = components["schemas"]["WebhookEventRecord"];
	type ProviderView = "all" | CheckoutPluginType;

	interface ProviderOption {
		id: string;
		name: string;
		type: CheckoutPluginType;
	}

	const emptyProviderCatalog: CheckoutPluginCatalog = { payment: [], shipping: [], tax: [] };
	const emptyProviderOverview: ProviderOperationsOverview = {
		runtime_environment: "sandbox",
		credential_service_configured: false,
		webhook_events: {
			pending_count: 0,
			processed_count: 0,
			dead_letter_count: 0,
			rejected_count: 0,
		},
	};
	const emptyReconciliationRuns: ProviderReconciliationRunPage = {
		data: [],
		pagination: { page: 1, limit: 10, total: 0, total_pages: 0 },
	};
	const emptyWebhookEvents: WebhookEventPage = {
		data: [],
		pagination: { page: 1, limit: 5, total: 0, total_pages: 0 },
	};
	const providerTabs = [
		{ id: "all", label: "All", icon: "bi-grid-1x2" },
		{ id: "payment", label: "Payment", icon: "bi-credit-card-2-front" },
		{ id: "shipping", label: "Shipping", icon: "bi-truck" },
		{ id: "tax", label: "Tax", icon: "bi-receipt" },
	];
	const runPageSizeOptions = [10, 20, 50];

	let { data }: Props = $props();
	const api: API = getContext("api");
	const notices = createAdminNotices();

	let providersSaving = $state(false);
	let credentialsLoading = $state(false);
	let credentialSaving = $state(false);
	let credentialRotatingId = $state<number | null>(null);
	let runsLoading = $state(false);
	let runCreating = $state(false);
	let runDetailLoading = $state(false);

	let providerCatalogState = $state<CheckoutPluginCatalog | null>(null);
	let providerCredentialsState = $state<ProviderCredential[] | null>(null);
	let providerOverviewState = $state<ProviderOperationsOverview | null>(null);
	let reconciliationRunsState = $state<ProviderReconciliationRunPage | null>(null);
	let rejectedWebhookEventsState = $state<WebhookEventPage | null>(null);
	let deadLetterWebhookEventsState = $state<WebhookEventPage | null>(null);
	let runDetailsById = $state<Record<number, ProviderReconciliationRun>>({});

	let providerView = $state<ProviderView>("all");

	let credentialProviderType = $state<CheckoutPluginType>("payment");
	let credentialProviderId = $state("");
	let credentialEnvironment = $state<ProviderCredentialEnvironment>("sandbox");
	let credentialLabel = $state("");
	let credentialSecretData = $state('{\n  "api_key": ""\n}');
	let credentialSupportedCurrencies = $state("");
	let credentialSettlementCurrency = $state("");
	let credentialFxMode = $state<ProviderCredentialFxMode>("same_currency_only");

	let reconciliationFilterType = $state<ProviderView>("all");
	let reconciliationFilterProviderId = $state("");
	let reconciliationLimit = $state<number>(10);
	let selectedRunId = $state<number | null>(null);
	let createRunProviderType = $state<CheckoutPluginType>("payment");
	let createRunProviderId = $state("");

	const providerCatalog = $derived.by(
		() => providerCatalogState ?? data.checkoutPlugins ?? emptyProviderCatalog
	);
	const providerCredentials = $derived.by(
		() => providerCredentialsState ?? data.providerCredentials ?? []
	);
	const providerOverview = $derived.by(
		() => providerOverviewState ?? data.providerOverview ?? emptyProviderOverview
	);
	const reconciliationRuns = $derived.by(
		() => reconciliationRunsState ?? data.reconciliationRuns ?? emptyReconciliationRuns
	);
	const rejectedWebhookEvents = $derived.by(
		() => rejectedWebhookEventsState ?? data.rejectedWebhookEvents ?? emptyWebhookEvents
	);
	const deadLetterWebhookEvents = $derived.by(
		() => deadLetterWebhookEventsState ?? data.deadLetterWebhookEvents ?? emptyWebhookEvents
	);
	const totalProviderCount = $derived(
		providerCatalog.payment.length + providerCatalog.shipping.length + providerCatalog.tax.length
	);
	const providerSections = $derived.by(() => {
		const sections = [
			{ type: "payment" as const, title: "Payment", plugins: providerCatalog.payment },
			{ type: "shipping" as const, title: "Shipping", plugins: providerCatalog.shipping },
			{ type: "tax" as const, title: "Tax", plugins: providerCatalog.tax },
		];
		return providerView === "all"
			? sections
			: sections.filter((section) => section.type === providerView);
	});
	const sortedCredentials = $derived.by(() => {
		return [...providerCredentials].sort((left, right) => {
			if (left.provider_type !== right.provider_type) {
				return left.provider_type.localeCompare(right.provider_type);
			}
			if (left.provider_id !== right.provider_id) {
				return left.provider_id.localeCompare(right.provider_id);
			}
			return left.environment.localeCompare(right.environment);
		});
	});
	const visibleCredentials = $derived.by(() => {
		return providerView === "all"
			? sortedCredentials
			: sortedCredentials.filter((credential) => credential.provider_type === providerView);
	});
	const visibleCredentialCount = $derived(visibleCredentials.length);
	const providerCredentialCounts = $derived.by(() => {
		const counts: Record<string, number> = {};
		for (const credential of providerCredentials) {
			const key = providerKey(credential.provider_type, credential.provider_id);
			counts[key] = (counts[key] ?? 0) + 1;
		}
		return counts;
	});
	const credentialProviderOptions = $derived.by(() =>
		listProvidersForType(credentialProviderType).map((provider) => ({
			id: provider.id,
			name: provider.name,
			type: provider.type,
		}))
	);
	const runCreateProviderOptions = $derived.by(() =>
		listProvidersForType(createRunProviderType).map((provider) => ({
			id: provider.id,
			name: provider.name,
			type: provider.type,
		}))
	);
	const runFilterProviderOptions = $derived.by(() => {
		if (reconciliationFilterType !== "all") {
			return listProvidersForType(reconciliationFilterType).map((provider) => ({
				id: provider.id,
				name: provider.name,
				type: provider.type,
			}));
		}

		const unique: Record<string, ProviderOption> = {};
		for (const type of ["payment", "shipping", "tax"] as const) {
			for (const provider of listProvidersForType(type)) {
				if (!(provider.id in unique)) {
					unique[provider.id] = {
						id: provider.id,
						name: provider.name,
						type,
					};
				}
			}
		}
		return Object.values(unique);
	});
	const matchingCredential = $derived.by(
		() =>
			providerCredentials.find(
				(credential) =>
					credential.provider_type === credentialProviderType &&
					credential.provider_id === credentialProviderId &&
					credential.environment === credentialEnvironment
			) ?? null
	);
	const parsedSecretData = $derived(parseProviderSecretData(credentialSecretData));
	const selectedRunSummary = $derived.by(
		() => reconciliationRuns.data.find((run) => run.id === selectedRunId) ?? null
	);
	const selectedRunDetail = $derived.by(() => {
		if (selectedRunId === null) {
			return null;
		}
		return runDetailsById[selectedRunId] ?? selectedRunSummary;
	});
	const selectedRunHasLoadedDetail = $derived.by(() => {
		if (selectedRunId === null) {
			return false;
		}
		return selectedRunId in runDetailsById;
	});
	const reconciliationMismatchSummary = $derived.by(() =>
		summarizeReconciliationMismatch(selectedRunDetail)
	);
	const safeRunPagination = $derived.by(() => ({
		...reconciliationRuns.pagination,
		page: Math.max(reconciliationRuns.pagination.page, 1),
		total_pages: Math.max(reconciliationRuns.pagination.total_pages, 1),
	}));

	function providerKey(type: CheckoutPluginType, providerId: string): string {
		return `${type}:${providerId}`;
	}

	function providerTypeLabel(type: CheckoutPluginType): string {
		switch (type) {
			case "payment":
				return "Payment";
			case "shipping":
				return "Shipping";
			case "tax":
				return "Tax";
		}
	}

	function listProvidersForType(type: CheckoutPluginType): CheckoutPlugin[] {
		switch (type) {
			case "payment":
				return providerCatalog.payment;
			case "shipping":
				return providerCatalog.shipping;
			case "tax":
				return providerCatalog.tax;
		}
	}

	function findProvider(type: CheckoutPluginType, providerId: string): CheckoutPlugin | null {
		return listProvidersForType(type).find((provider) => provider.id === providerId) ?? null;
	}

	function providerDisplayName(type: CheckoutPluginType, providerId: string): string {
		return findProvider(type, providerId)?.name ?? providerId;
	}

	function formatDateTime(value: string | null | undefined): string {
		if (!value) {
			return "Not available";
		}

		const date = new Date(value);
		if (Number.isNaN(date.getTime())) {
			return value;
		}

		return new Intl.DateTimeFormat(undefined, {
			dateStyle: "medium",
			timeStyle: "short",
		}).format(date);
	}

	function statusTone(status: ProviderReconciliationRun["status"]): "success" | "danger" {
		return status === "SUCCEEDED" ? "success" : "danger";
	}

	function driftTone(severity: "ERROR" | "WARN"): "danger" | "warning" {
		return severity === "ERROR" ? "danger" : "warning";
	}

	function webhookStatusTone(
		status: WebhookEventRecord["status"]
	): "danger" | "warning" | "success" {
		switch (status) {
			case "REJECTED":
				return "danger";
			case "DEAD_LETTER":
				return "warning";
			case "PROCESSED":
				return "success";
			default:
				return "warning";
		}
	}

	function syncProviderSelection(current: string, options: ProviderOption[]): string {
		if (options.some((option) => option.id === current)) {
			return current;
		}
		return options[0]?.id ?? "";
	}

	function resetCredentialForm() {
		credentialProviderType = providerView === "all" ? "payment" : providerView;
		credentialProviderId = "";
		credentialEnvironment = "sandbox";
		credentialLabel = "";
		credentialSecretData = '{\n  "api_key": ""\n}';
		credentialSupportedCurrencies = "";
		credentialSettlementCurrency = "";
		credentialFxMode = "same_currency_only";
	}

	function loadCredentialIntoForm(credential: ProviderCredential) {
		credentialProviderType = credential.provider_type;
		credentialProviderId = credential.provider_id;
		credentialEnvironment = credential.environment;
		credentialLabel = credential.label;
		credentialSecretData = "{\n  \n}";
		credentialSupportedCurrencies = formatProviderCurrencies(credential.supported_currencies);
		credentialSettlementCurrency = credential.settlement_currency ?? "";
		credentialFxMode = credential.fx_mode;
	}

	function shouldIncludeRunInFilters(run: ProviderReconciliationRun): boolean {
		if (reconciliationFilterType !== "all" && run.provider_type !== reconciliationFilterType) {
			return false;
		}
		if (
			reconciliationFilterProviderId.trim() &&
			run.provider_id !== reconciliationFilterProviderId.trim()
		) {
			return false;
		}
		return true;
	}

	async function refreshAll() {
		await Promise.all([
			loadProviders({ quiet: true }),
			loadCredentials({ quiet: true }),
			loadProviderOverview({ quiet: true }),
			loadWebhookEvents({ quiet: true }),
			loadReconciliationRuns({
				page: safeRunPagination.page,
				limit: reconciliationLimit,
				quiet: true,
				preserveSelection: true,
			}),
		]);
		notices.pushSuccess("Provider operations refreshed.");
	}

	async function loadProviderOverview({ quiet = false }: { quiet?: boolean } = {}) {
		if (!quiet) {
			notices.clear();
		}
		try {
			providerOverviewState = await api.getAdminProviderOperationsOverview();
		} catch (error) {
			console.error(error);
			notices.pushError("Unable to load provider operations overview.");
		}
	}

	async function loadWebhookEvents({ quiet = false }: { quiet?: boolean } = {}) {
		if (!quiet) {
			notices.clear();
		}
		try {
			const [rejected, deadLetter] = await Promise.all([
				api.listAdminWebhookEvents({ status: "REJECTED", page: 1, limit: 5 }),
				api.listAdminWebhookEvents({ status: "DEAD_LETTER", page: 1, limit: 5 }),
			]);
			rejectedWebhookEventsState = rejected;
			deadLetterWebhookEventsState = deadLetter;
		} catch (error) {
			console.error(error);
			notices.pushError("Unable to load webhook event health.");
		}
	}

	async function loadProviders({ quiet = false }: { quiet?: boolean } = {}) {
		providersSaving = true;
		if (!quiet) {
			notices.clear();
		}
		try {
			providerCatalogState = await api.listAdminCheckoutPlugins();
		} catch (error) {
			console.error(error);
			notices.pushError("Unable to load checkout providers.");
		} finally {
			providersSaving = false;
		}
	}

	async function loadCredentials({ quiet = false }: { quiet?: boolean } = {}) {
		credentialsLoading = true;
		if (!quiet) {
			notices.clear();
		}
		try {
			providerCredentialsState = await api.listAdminProviderCredentials();
		} catch (error) {
			console.error(error);
			notices.pushError("Unable to load provider credentials.");
		} finally {
			credentialsLoading = false;
		}
	}

	async function updateProviderEnabled(
		type: CheckoutPluginType,
		providerId: string,
		enabled: boolean
	) {
		if (providersSaving) {
			return;
		}

		providersSaving = true;
		notices.clear();
		try {
			providerCatalogState = await api.updateAdminCheckoutPlugin(type, providerId, { enabled });
			notices.pushSuccess("Provider settings updated.");
		} catch (error) {
			console.error(error);
			const err = error as { body?: { error?: string } };
			notices.pushError(err.body?.error ?? "Unable to update provider settings.");
		} finally {
			providersSaving = false;
		}
	}

	async function saveCredential() {
		if (credentialSaving) {
			return;
		}

		const secretData = parseProviderSecretData(credentialSecretData);
		if (secretData.error) {
			notices.pushError(secretData.error);
			return;
		}
		if (!credentialProviderId) {
			notices.pushError("Select a provider before storing credentials.");
			return;
		}

		credentialSaving = true;
		notices.clear();
		try {
			const saved = await api.upsertAdminProviderCredential({
				provider_type: credentialProviderType,
				provider_id: credentialProviderId,
				environment: credentialEnvironment,
				label: credentialLabel.trim() || undefined,
				secret_data: secretData.value,
				supported_currencies: parseProviderCurrencies(credentialSupportedCurrencies),
				settlement_currency: credentialSettlementCurrency.trim() || undefined,
				fx_mode: credentialFxMode,
			});
			providerCredentialsState = [
				saved,
				...providerCredentials.filter(
					(credential) =>
						!(
							credential.provider_type === saved.provider_type &&
							credential.provider_id === saved.provider_id &&
							credential.environment === saved.environment
						)
				),
			];
			credentialSecretData = "{\n  \n}";
			notices.pushSuccess(matchingCredential ? "Credential replaced." : "Credential stored.");
		} catch (error) {
			console.error(error);
			const err = error as { body?: { error?: string } };
			notices.pushError(err.body?.error ?? "Unable to store provider credential.");
		} finally {
			credentialSaving = false;
		}
	}

	async function rotateCredential(credential: ProviderCredential) {
		if (credentialRotatingId !== null) {
			return;
		}

		credentialRotatingId = credential.id;
		notices.clear();
		try {
			const rotated = await api.rotateAdminProviderCredential(credential.id);
			providerCredentialsState = providerCredentials.map((entry) =>
				entry.id === rotated.id ? rotated : entry
			);
			notices.pushSuccess("Credential rotated.");
		} catch (error) {
			console.error(error);
			const err = error as { body?: { error?: string } };
			notices.pushError(err.body?.error ?? "Unable to rotate provider credential.");
		} finally {
			credentialRotatingId = null;
		}
	}

	async function loadReconciliationRuns({
		page = 1,
		limit = reconciliationLimit,
		quiet = false,
		preserveSelection = true,
	}: {
		page?: number;
		limit?: number;
		quiet?: boolean;
		preserveSelection?: boolean;
	} = {}) {
		runsLoading = true;
		if (!quiet) {
			notices.clear();
		}
		try {
			const nextRuns = await api.listAdminProviderReconciliationRuns({
				page,
				limit,
				provider_type: reconciliationFilterType === "all" ? undefined : reconciliationFilterType,
				provider_id: reconciliationFilterProviderId.trim() || undefined,
			});
			reconciliationRunsState = nextRuns;
			reconciliationLimit = limit;

			const currentSelection =
				preserveSelection && selectedRunId !== null
					? (nextRuns.data.find((run) => run.id === selectedRunId)?.id ?? null)
					: null;
			selectedRunId = currentSelection ?? nextRuns.data[0]?.id ?? null;

			if (selectedRunId !== null && !(selectedRunId in runDetailsById)) {
				void loadReconciliationRunDetail(selectedRunId, { quiet: true });
			}
		} catch (error) {
			console.error(error);
			notices.pushError("Unable to load reconciliation runs.");
		} finally {
			runsLoading = false;
		}
	}

	async function loadReconciliationRunDetail(runId: number, { quiet = false } = {}) {
		if (runDetailLoading && selectedRunId === runId) {
			return;
		}

		runDetailLoading = true;
		if (!quiet) {
			notices.clear();
		}
		try {
			const run = await api.getAdminProviderReconciliationRun(runId);
			runDetailsById = { ...runDetailsById, [run.id]: run };
		} catch (error) {
			console.error(error);
			notices.pushError("Unable to load reconciliation run details.");
		} finally {
			runDetailLoading = false;
		}
	}

	async function selectRun(runId: number) {
		selectedRunId = runId;
		if (!(runId in runDetailsById)) {
			await loadReconciliationRunDetail(runId);
		}
	}

	async function createReconciliationRun() {
		if (runCreating) {
			return;
		}
		if (!createRunProviderId) {
			notices.pushError("Select a provider before running reconciliation.");
			return;
		}

		runCreating = true;
		notices.clear();
		try {
			const run = await api.createAdminProviderReconciliationRun({
				provider_type: createRunProviderType,
				provider_id: createRunProviderId,
			});
			runDetailsById = { ...runDetailsById, [run.id]: run };
			if (shouldIncludeRunInFilters(run)) {
				selectedRunId = run.id;
			}
			await loadReconciliationRuns({
				page: 1,
				limit: reconciliationLimit,
				quiet: true,
				preserveSelection: false,
			});
			selectedRunId = run.id;
			notices.pushSuccess("Reconciliation run completed.");
		} catch (error) {
			console.error(error);
			const err = error as { body?: { error?: string } };
			notices.pushError(err.body?.error ?? "Unable to run reconciliation.");
		} finally {
			runCreating = false;
		}
	}

	function applyReconciliationFilters() {
		void loadReconciliationRuns({
			page: 1,
			limit: reconciliationLimit,
			preserveSelection: false,
		});
	}

	function changeReconciliationPage(nextPage: number) {
		void loadReconciliationRuns({
			page: nextPage,
			limit: reconciliationLimit,
		});
	}

	function changeReconciliationLimit(limit: number) {
		void loadReconciliationRuns({
			page: 1,
			limit,
			preserveSelection: false,
		});
	}

	$effect(() => {
		const nextProviderId = syncProviderSelection(credentialProviderId, credentialProviderOptions);
		if (nextProviderId !== credentialProviderId) {
			credentialProviderId = nextProviderId;
		}
	});

	$effect(() => {
		const nextProviderId = syncProviderSelection(createRunProviderId, runCreateProviderOptions);
		if (nextProviderId !== createRunProviderId) {
			createRunProviderId = nextProviderId;
		}
	});

	$effect(() => {
		if (
			reconciliationFilterProviderId &&
			!runFilterProviderOptions.some((provider) => provider.id === reconciliationFilterProviderId)
		) {
			reconciliationFilterProviderId = "";
		}
	});

	onMount(() => {
		reconciliationLimit = data.reconciliationRuns?.pagination.limit ?? 10;
		if (selectedRunId === null) {
			selectedRunId = data.reconciliationRuns?.data[0]?.id ?? null;
		}
		for (const message of data.errorMessages ?? []) {
			notices.pushError(message);
		}
		if (selectedRunId !== null) {
			void loadReconciliationRunDetail(selectedRunId, { quiet: true });
		}
	});
</script>

{#snippet providerActions()}
	<AdminResourceActions
		countLabel={`${totalProviderCount} providers`}
		actions={providerRefreshAction}
	/>
{/snippet}

{#snippet providerRefreshAction()}
	<Button
		tone="admin"
		type="button"
		variant="regular"
		size="small"
		class="rounded-full"
		onclick={refreshAll}
		disabled={providersSaving || credentialsLoading || runsLoading}
	>
		Refresh all
	</Button>
{/snippet}

{#snippet providerToggleHeaderActions()}
	<TabSwitcher items={providerTabs} bind:value={providerView} ariaLabel="Provider categories" />
{/snippet}

<section class="space-y-6">
	<AdminPageHeader title="Providers" actions={providerActions} />

	<AdminPanel
		title="Provider Toggles"
		meta={`${totalProviderCount} provider${totalProviderCount === 1 ? "" : "s"}`}
		headerActions={providerToggleHeaderActions}
	>
		<div class="space-y-6">
			<div class="flex flex-wrap gap-2">
				<Badge
					tone={providerOverview.runtime_environment === "production" ? "warning" : "info"}
					size="md"
				>
					Runtime {providerOverview.runtime_environment}
				</Badge>
				<Badge
					tone={providerOverview.credential_service_configured ? "success" : "warning"}
					size="md"
				>
					{providerOverview.credential_service_configured
						? "Credential encryption ready"
						: "Credential encryption off"}
				</Badge>
				<Badge
					tone={providerOverview.webhook_events.rejected_count > 0 ? "danger" : "neutral"}
					size="md"
				>
					{providerOverview.webhook_events.rejected_count} rejected webhooks
				</Badge>
				<Badge
					tone={providerOverview.webhook_events.dead_letter_count > 0 ? "warning" : "neutral"}
					size="md"
				>
					{providerOverview.webhook_events.dead_letter_count} dead-letter webhooks
				</Badge>
				<Badge tone="neutral" size="md">{visibleCredentialCount} credentials</Badge>
				<Badge tone="neutral" size="md">
					{reconciliationRuns.pagination.total} reconciliation runs
				</Badge>
			</div>

			<div class={`space-y-0 xl:grid xl:gap-6 ${providerView === "all" ? "xl:grid-cols-3" : ""}`}>
				{#each providerSections as section, index (section.type)}
					<div
						class={index === 0
							? "pt-0 xl:min-w-0"
							: "border-t border-stone-200 pt-6 xl:min-w-0 xl:border-t-0 xl:border-l xl:pt-0 xl:pl-6 dark:border-stone-800"}
					>
						<div class="flex flex-wrap items-start justify-between gap-3">
							<div>
								<h3 class="text-lg font-semibold text-stone-950 dark:text-stone-50">
									{section.title}
								</h3>
								<p class="mt-1 text-sm text-stone-500 dark:text-stone-400">
									{section.plugins.length} provider{section.plugins.length === 1 ? "" : "s"}
								</p>
							</div>
						</div>

						<div class="mt-4 space-y-3">
							{#if section.plugins.length === 0}
								<p class="admin-empty-state">No providers found.</p>
							{:else}
								<div class="border-y border-stone-200/90 dark:border-stone-800">
									{#each section.plugins as provider, providerIndex (provider.id)}
										<div
											class={`py-4 ${providerIndex > 0 ? "border-t border-stone-200/90 dark:border-stone-800" : ""}`}
										>
											<div class="flex items-start justify-between gap-3">
												<div>
													<p class="text-sm font-medium text-stone-950 dark:text-stone-50">
														{provider.name}
													</p>
													<p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
														{provider.description}
													</p>
												</div>
												<Badge tone={provider.enabled ? "success" : "neutral"}>
													{provider.enabled ? "Enabled" : "Disabled"}
												</Badge>
											</div>
											<div class="mt-4 flex flex-wrap items-center justify-between gap-3">
												<p class="text-xs text-stone-500 dark:text-stone-400">
													{providerCredentialCounts[providerKey(section.type, provider.id)] ?? 0}
													credential{providerCredentialCounts[
														providerKey(section.type, provider.id)
													] === 1
														? ""
														: "s"}
													configured
												</p>
												{#if section.type === "tax"}
													<Button
														tone="admin"
														type="button"
														variant="regular"
														size="small"
														class="rounded-full"
														onclick={() => updateProviderEnabled("tax", provider.id, true)}
														disabled={providersSaving || provider.enabled}
													>
														{provider.enabled ? "Active Tax Provider" : "Set Active"}
													</Button>
												{:else}
													<Button
														tone="admin"
														type="button"
														variant="regular"
														size="small"
														class="rounded-full"
														onclick={() =>
															updateProviderEnabled(section.type, provider.id, !provider.enabled)}
														disabled={providersSaving}
													>
														{provider.enabled ? "Disable" : "Enable"}
													</Button>
												{/if}
											</div>
										</div>
									{/each}
								</div>
							{/if}
						</div>
					</div>
				{/each}
			</div>
		</div>
	</AdminPanel>

	<AdminPanel
		title="Operational Runbooks"
		meta="Live signals for webhook outages and reconciliation mismatch handling"
	>
		<div class="space-y-4">
			<div class="grid gap-4 xl:grid-cols-2">
				{#each providerRunbooks as runbook (runbook.id)}
					<div class="rounded-[1.75rem] border border-stone-200/80 p-5 dark:border-stone-800">
						<div class="flex flex-wrap items-start justify-between gap-3">
							<div>
								<h3 class="text-sm font-semibold text-stone-950 dark:text-stone-50">
									{runbook.title}
								</h3>
								<p class="mt-2 text-sm text-stone-500 dark:text-stone-400">
									{runbook.summary}
								</p>
							</div>
							{#if runbook.id === "webhook_outage"}
								<div class="flex flex-wrap gap-2">
									<Badge
										tone={providerOverview.runtime_environment === "production"
											? "warning"
											: "info"}
									>
										{providerOverview.runtime_environment}
									</Badge>
									<Badge
										tone={providerOverview.webhook_events.rejected_count > 0 ? "danger" : "neutral"}
									>
										{providerOverview.webhook_events.rejected_count} rejected
									</Badge>
									<Badge
										tone={providerOverview.webhook_events.pending_count > 0 ? "warning" : "neutral"}
									>
										{providerOverview.webhook_events.pending_count} pending
									</Badge>
									<Badge
										tone={providerOverview.webhook_events.dead_letter_count > 0
											? "warning"
											: "neutral"}
									>
										{providerOverview.webhook_events.dead_letter_count} dead-letter
									</Badge>
								</div>
							{:else}
								<Badge tone={selectedRunDetail?.drift_count ? "warning" : "neutral"}>
									{selectedRunDetail?.drift_count ?? 0} current drifts
								</Badge>
							{/if}
						</div>

						{#if runbook.id === "webhook_outage"}
							<div class="mt-4 grid gap-3 md:grid-cols-2">
								<div class="rounded-2xl border border-stone-200/80 p-4 dark:border-stone-800">
									<p class="text-xs tracking-[0.2em] text-stone-500 uppercase dark:text-stone-400">
										Recent signature rejects
									</p>
									<div class="mt-3 space-y-3">
										{#if rejectedWebhookEvents.data.length === 0}
											<p class="text-sm text-stone-500 dark:text-stone-400">
												No rejected webhook events recorded.
											</p>
										{:else}
											{#each rejectedWebhookEvents.data as event (event.id)}
												<div class="rounded-2xl bg-stone-50/80 p-3 dark:bg-stone-950/40">
													<div class="flex flex-wrap items-start justify-between gap-2">
														<div>
															<p class="text-sm font-medium text-stone-950 dark:text-stone-50">
																{event.provider}
															</p>
															<p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
																{formatDateTime(event.received_at)}
															</p>
														</div>
														<Badge tone={webhookStatusTone(event.status)}>
															{event.status}
														</Badge>
													</div>
													<p class="mt-2 text-xs break-all text-stone-600 dark:text-stone-300">
														{event.last_error}
													</p>
												</div>
											{/each}
										{/if}
									</div>
								</div>

								<div class="rounded-2xl border border-stone-200/80 p-4 dark:border-stone-800">
									<p class="text-xs tracking-[0.2em] text-stone-500 uppercase dark:text-stone-400">
										Recent dead-letter events
									</p>
									<div class="mt-3 space-y-3">
										{#if deadLetterWebhookEvents.data.length === 0}
											<p class="text-sm text-stone-500 dark:text-stone-400">
												No dead-letter webhook events recorded.
											</p>
										{:else}
											{#each deadLetterWebhookEvents.data as event (event.id)}
												<div class="rounded-2xl bg-stone-50/80 p-3 dark:bg-stone-950/40">
													<div class="flex flex-wrap items-start justify-between gap-2">
														<div>
															<p class="text-sm font-medium text-stone-950 dark:text-stone-50">
																{event.provider}
															</p>
															<p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
																{event.provider_event_id}
															</p>
														</div>
														<Badge tone={webhookStatusTone(event.status)}>
															{event.status}
														</Badge>
													</div>
													<p class="mt-2 text-xs break-all text-stone-600 dark:text-stone-300">
														{event.last_error}
													</p>
												</div>
											{/each}
										{/if}
									</div>
								</div>
							</div>
						{:else}
							<div class="mt-4 rounded-2xl border border-stone-200/80 p-4 dark:border-stone-800">
								<p class="text-sm text-stone-600 dark:text-stone-300">
									{reconciliationMismatchSummary}
								</p>
							</div>
						{/if}

						<div class="mt-4 space-y-3">
							{#each runbook.steps as step, index (step.title)}
								<div class="flex items-start gap-3">
									<div
										class="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-stone-950 text-xs font-semibold text-white dark:bg-stone-100 dark:text-stone-950"
									>
										{index + 1}
									</div>
									<div>
										<p class="text-sm font-medium text-stone-950 dark:text-stone-50">
											{step.title}
										</p>
										<p class="mt-1 text-sm text-stone-500 dark:text-stone-400">
											{step.description}
										</p>
									</div>
								</div>
							{/each}
						</div>
					</div>
				{/each}
			</div>
		</div>
	</AdminPanel>

	<AdminMasterDetailLayout columnsClass="xl:grid-cols-[1.02fr_0.98fr]">
		{#snippet master()}
			<div class="space-y-6">
				<AdminPanel
					title="Reconciliation History"
					meta={`${reconciliationRuns.pagination.total} run${
						reconciliationRuns.pagination.total === 1 ? "" : "s"
					}`}
				>
					<div class="space-y-4">
						<div class="grid gap-3 md:grid-cols-3">
							<label class="space-y-2 text-sm text-stone-600 dark:text-stone-300">
								<span>Provider type</span>
								<Dropdown tone="admin" bind:value={reconciliationFilterType}>
									<option value="all">All types</option>
									<option value="payment">Payment</option>
									<option value="shipping">Shipping</option>
									<option value="tax">Tax</option>
								</Dropdown>
							</label>
							<label class="space-y-2 text-sm text-stone-600 dark:text-stone-300">
								<span>Provider</span>
								<Dropdown tone="admin" bind:value={reconciliationFilterProviderId}>
									<option value="">All providers</option>
									{#each runFilterProviderOptions as provider (provider.id)}
										<option value={provider.id}>{provider.name}</option>
									{/each}
								</Dropdown>
							</label>
							<div class="flex items-end gap-2">
								<Button
									tone="admin"
									type="button"
									variant="regular"
									class="rounded-full"
									onclick={applyReconciliationFilters}
									disabled={runsLoading}
								>
									Apply filters
								</Button>
								<Button
									tone="admin"
									type="button"
									variant="regular"
									class="rounded-full"
									onclick={() =>
										loadReconciliationRuns({
											page: safeRunPagination.page,
											limit: reconciliationLimit,
										})}
									disabled={runsLoading}
								>
									Refresh
								</Button>
							</div>
						</div>

						{#if reconciliationRuns.data.length === 0}
							<p class="admin-empty-state">No reconciliation runs found.</p>
						{:else}
							<div class="space-y-3">
								{#each reconciliationRuns.data as run (run.id)}
									<button
										type="button"
										class={`admin-list-item w-full p-4 text-left ${
											selectedRunId === run.id
												? "ring-1 ring-stone-900/15 dark:ring-stone-100/15"
												: ""
										}`}
										onclick={() => void selectRun(run.id)}
									>
										<div class="flex flex-wrap items-start justify-between gap-3">
											<div>
												<p class="text-sm font-medium text-stone-950 dark:text-stone-50">
													{providerDisplayName(run.provider_type, run.provider_id)}
												</p>
												<p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
													{providerTypeLabel(run.provider_type)} • {run.provider_id}
												</p>
											</div>
											<div class="flex flex-wrap gap-2">
												<Badge tone={statusTone(run.status)}>{run.status}</Badge>
												<Badge tone={run.environment === "production" ? "warning" : "info"}>
													{run.environment}
												</Badge>
											</div>
										</div>
										<div class="mt-3 flex flex-wrap gap-2">
											<Badge tone="neutral">{run.trigger}</Badge>
											<Badge tone="neutral">{run.checked_count} checked</Badge>
											<Badge tone="neutral">{run.drift_count} drifts</Badge>
											<Badge tone="neutral">{run.error_count} errors</Badge>
										</div>
										<p class="mt-3 text-xs text-stone-500 dark:text-stone-400">
											Started {formatDateTime(run.started_at)}
										</p>
									</button>
								{/each}
							</div>
						{/if}

						<AdminPaginationControls
							page={safeRunPagination.page}
							totalPages={safeRunPagination.total_pages}
							totalItems={safeRunPagination.total}
							limit={reconciliationLimit}
							limitOptions={runPageSizeOptions}
							onLimitChange={changeReconciliationLimit}
							onPrev={() => changeReconciliationPage(Math.max(safeRunPagination.page - 1, 1))}
							onNext={() =>
								changeReconciliationPage(
									Math.min(safeRunPagination.page + 1, safeRunPagination.total_pages)
								)}
						/>
					</div>
				</AdminPanel>

				<AdminPanel
					title="Reconciliation Detail"
					meta={selectedRunDetail
						? `${providerDisplayName(
								selectedRunDetail.provider_type,
								selectedRunDetail.provider_id
							)} • Run #${selectedRunDetail.id}`
						: "Run and inspect provider reconciliation"}
				>
					<div class="space-y-4">
						<div class="grid gap-3 md:grid-cols-[0.95fr_1.05fr_auto]">
							<label class="space-y-2 text-sm text-stone-600 dark:text-stone-300">
								<span>Run type</span>
								<Dropdown tone="admin" bind:value={createRunProviderType}>
									<option value="payment">Payment</option>
									<option value="shipping">Shipping</option>
									<option value="tax">Tax</option>
								</Dropdown>
							</label>
							<label class="space-y-2 text-sm text-stone-600 dark:text-stone-300">
								<span>Provider</span>
								<Dropdown tone="admin" bind:value={createRunProviderId}>
									{#if runCreateProviderOptions.length === 0}
										<option value="">No providers available</option>
									{:else}
										{#each runCreateProviderOptions as provider (provider.id)}
											<option value={provider.id}>{provider.name}</option>
										{/each}
									{/if}
								</Dropdown>
							</label>
							<div class="flex items-end">
								<Button
									tone="admin"
									type="button"
									variant="primary"
									class="w-full rounded-full"
									onclick={() => void createReconciliationRun()}
									disabled={runCreating || !createRunProviderId}
								>
									{runCreating ? "Running..." : "Run now"}
								</Button>
							</div>
						</div>

						{#if selectedRunDetail}
							<div class="space-y-4">
								<div class="flex flex-wrap gap-2">
									<Badge tone={statusTone(selectedRunDetail.status)}>
										{selectedRunDetail.status}
									</Badge>
									<Badge tone={selectedRunDetail.environment === "production" ? "warning" : "info"}>
										{selectedRunDetail.environment}
									</Badge>
									<Badge tone="neutral">{selectedRunDetail.trigger}</Badge>
								</div>

								<div class="grid gap-3 sm:grid-cols-3">
									<div class="rounded-2xl border border-stone-200/80 p-4 dark:border-stone-800">
										<p
											class="text-xs tracking-[0.2em] text-stone-500 uppercase dark:text-stone-400"
										>
											Checked
										</p>
										<p class="mt-2 text-2xl font-semibold text-stone-950 dark:text-stone-50">
											{selectedRunDetail.checked_count}
										</p>
									</div>
									<div class="rounded-2xl border border-stone-200/80 p-4 dark:border-stone-800">
										<p
											class="text-xs tracking-[0.2em] text-stone-500 uppercase dark:text-stone-400"
										>
											Drifts
										</p>
										<p class="mt-2 text-2xl font-semibold text-stone-950 dark:text-stone-50">
											{selectedRunDetail.drift_count}
										</p>
									</div>
									<div class="rounded-2xl border border-stone-200/80 p-4 dark:border-stone-800">
										<p
											class="text-xs tracking-[0.2em] text-stone-500 uppercase dark:text-stone-400"
										>
											Errors
										</p>
										<p class="mt-2 text-2xl font-semibold text-stone-950 dark:text-stone-50">
											{selectedRunDetail.error_count}
										</p>
									</div>
								</div>

								<div class="grid gap-3 md:grid-cols-2">
									<div class="rounded-2xl border border-stone-200/80 p-4 dark:border-stone-800">
										<p
											class="text-xs tracking-[0.2em] text-stone-500 uppercase dark:text-stone-400"
										>
											Started
										</p>
										<p class="mt-2 text-sm text-stone-700 dark:text-stone-200">
											{formatDateTime(selectedRunDetail.started_at)}
										</p>
									</div>
									<div class="rounded-2xl border border-stone-200/80 p-4 dark:border-stone-800">
										<p
											class="text-xs tracking-[0.2em] text-stone-500 uppercase dark:text-stone-400"
										>
											Finished
										</p>
										<p class="mt-2 text-sm text-stone-700 dark:text-stone-200">
											{formatDateTime(selectedRunDetail.finished_at)}
										</p>
									</div>
								</div>

								<div class="space-y-3">
									<div class="flex items-center justify-between gap-3">
										<h3 class="text-sm font-semibold text-stone-950 dark:text-stone-50">
											Drift Findings
										</h3>
										{#if runDetailLoading}
											<span class="text-xs text-stone-500 dark:text-stone-400">
												Loading detail...
											</span>
										{/if}
									</div>
									{#if !selectedRunHasLoadedDetail && runDetailLoading}
										<p class="admin-empty-state">Loading drift details...</p>
									{:else if (selectedRunDetail.drifts ?? []).length === 0}
										<p class="admin-empty-state">No drifts recorded for this run.</p>
									{:else}
										<div class="space-y-3">
											{#each selectedRunDetail.drifts ?? [] as drift (drift.id)}
												<div class="admin-list-item p-4">
													<div class="flex flex-wrap items-start justify-between gap-3">
														<div>
															<p class="text-sm font-medium text-stone-950 dark:text-stone-50">
																{drift.message}
															</p>
															<p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
																{drift.entity_type} #{drift.entity_id} • {drift.field_name}
															</p>
														</div>
														<Badge tone={driftTone(drift.severity)}>
															{drift.severity}
														</Badge>
													</div>
													<div class="mt-3 grid gap-3 md:grid-cols-2">
														<div
															class="rounded-2xl border border-stone-200/80 p-3 dark:border-stone-800"
														>
															<p
																class="text-xs tracking-[0.2em] text-stone-500 uppercase dark:text-stone-400"
															>
																Expected
															</p>
															<p
																class="mt-2 font-mono text-xs break-all text-stone-700 dark:text-stone-200"
															>
																{drift.expected_value}
															</p>
														</div>
														<div
															class="rounded-2xl border border-stone-200/80 p-3 dark:border-stone-800"
														>
															<p
																class="text-xs tracking-[0.2em] text-stone-500 uppercase dark:text-stone-400"
															>
																Actual
															</p>
															<p
																class="mt-2 font-mono text-xs break-all text-stone-700 dark:text-stone-200"
															>
																{drift.actual_value}
															</p>
														</div>
													</div>
													<p class="mt-3 text-xs text-stone-500 dark:text-stone-400">
														Provider reference: {drift.provider_reference}
													</p>
												</div>
											{/each}
										</div>
									{/if}
								</div>
							</div>
						{:else}
							<p class="admin-empty-state">
								Select a reconciliation run to inspect its drift details.
							</p>
						{/if}
					</div>
				</AdminPanel>
			</div>
		{/snippet}

		{#snippet detail()}
			<div class="space-y-6">
				<AdminPanel
					title={matchingCredential ? "Replace Credential" : "Store Credential"}
					meta={matchingCredential
						? `${providerDisplayName(
								matchingCredential.provider_type,
								matchingCredential.provider_id
							)} • ${matchingCredential.environment}`
						: "Create or replace provider credentials"}
				>
					<div class="space-y-4">
						<p class="text-sm text-stone-500 dark:text-stone-400">
							{#if matchingCredential}
								Updating this record replaces the stored secret payload for the selected
								provider/environment pair.
							{:else}
								Store encrypted provider credentials plus operational metadata used by the payments,
								shipping, and tax flows.
							{/if}
						</p>

						<div class="grid gap-3 md:grid-cols-2">
							<label class="space-y-2 text-sm text-stone-600 dark:text-stone-300">
								<span>Provider type</span>
								<Dropdown tone="admin" bind:value={credentialProviderType}>
									<option value="payment">Payment</option>
									<option value="shipping">Shipping</option>
									<option value="tax">Tax</option>
								</Dropdown>
							</label>
							<label class="space-y-2 text-sm text-stone-600 dark:text-stone-300">
								<span>Provider</span>
								<Dropdown tone="admin" bind:value={credentialProviderId}>
									{#if credentialProviderOptions.length === 0}
										<option value="">No providers available</option>
									{:else}
										{#each credentialProviderOptions as provider (provider.id)}
											<option value={provider.id}>{provider.name}</option>
										{/each}
									{/if}
								</Dropdown>
							</label>
							<label class="space-y-2 text-sm text-stone-600 dark:text-stone-300">
								<span>Environment</span>
								<Dropdown tone="admin" bind:value={credentialEnvironment}>
									<option value="sandbox">Sandbox</option>
									<option value="production">Production</option>
								</Dropdown>
							</label>
							<label class="space-y-2 text-sm text-stone-600 dark:text-stone-300">
								<span>Label</span>
								<TextInput
									tone="admin"
									placeholder="Primary processing key"
									bind:value={credentialLabel}
								/>
							</label>
							<label class="space-y-2 text-sm text-stone-600 dark:text-stone-300">
								<span>Supported currencies</span>
								<TextInput
									tone="admin"
									placeholder="USD, EUR, CAD"
									bind:value={credentialSupportedCurrencies}
								/>
							</label>
							<label class="space-y-2 text-sm text-stone-600 dark:text-stone-300">
								<span>Settlement currency</span>
								<TextInput
									tone="admin"
									placeholder="USD"
									bind:value={credentialSettlementCurrency}
								/>
							</label>
							<label class="space-y-2 text-sm text-stone-600 md:col-span-2 dark:text-stone-300">
								<span>FX mode</span>
								<Dropdown tone="admin" bind:value={credentialFxMode}>
									<option value="same_currency_only">same_currency_only</option>
									<option value="provider_managed">provider_managed</option>
								</Dropdown>
							</label>
						</div>

						<label class="space-y-2 text-sm text-stone-600 dark:text-stone-300">
							<span>Secret data JSON</span>
							<TextArea
								tone="admin"
								rows={10}
								class="font-mono text-xs"
								bind:value={credentialSecretData}
							/>
						</label>
						{#if parsedSecretData.error}
							<p class="text-sm text-rose-600 dark:text-rose-300">{parsedSecretData.error}</p>
						{:else}
							<p class="text-sm text-stone-500 dark:text-stone-400">
								{Object.keys(parsedSecretData.value).length} secret field{Object.keys(
									parsedSecretData.value
								).length === 1
									? ""
									: "s"} ready to encrypt.
							</p>
						{/if}

						<div class="flex flex-wrap gap-2">
							<Button
								tone="admin"
								type="button"
								variant="primary"
								class="rounded-full"
								onclick={() => void saveCredential()}
								disabled={credentialSaving || !credentialProviderId}
							>
								{credentialSaving
									? "Saving..."
									: matchingCredential
										? "Replace credential"
										: "Store credential"}
							</Button>
							<Button
								tone="admin"
								type="button"
								variant="regular"
								class="rounded-full"
								onclick={resetCredentialForm}
								disabled={credentialSaving}
							>
								New credential
							</Button>
						</div>
					</div>
				</AdminPanel>

				<AdminPanel
					title="Credential Inventory"
					meta={`${visibleCredentialCount} record${visibleCredentialCount === 1 ? "" : "s"}`}
				>
					<div class="space-y-3">
						<div class="flex items-center justify-between gap-3">
							<p class="text-sm text-stone-500 dark:text-stone-400">
								Stored metadata only. Secrets never come back from the API.
							</p>
							<Button
								tone="admin"
								type="button"
								variant="regular"
								size="small"
								class="rounded-full"
								onclick={() => loadCredentials()}
								disabled={credentialsLoading}
							>
								Refresh
							</Button>
						</div>

						{#if visibleCredentials.length === 0}
							<p class="admin-empty-state">No credentials stored for this view.</p>
						{:else}
							{#each visibleCredentials as credential (credential.id)}
								<div
									class={`admin-list-item w-full p-4 text-left ${
										matchingCredential?.id === credential.id
											? "ring-1 ring-stone-900/15 dark:ring-stone-100/15"
											: ""
									}`}
								>
									<div class="flex flex-wrap items-start justify-between gap-3">
										<div>
											<p class="text-sm font-medium text-stone-950 dark:text-stone-50">
												{providerDisplayName(credential.provider_type, credential.provider_id)}
											</p>
											<p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
												{providerTypeLabel(credential.provider_type)} • {credential.provider_id}
											</p>
										</div>
										<div class="flex flex-wrap gap-2">
											<Badge tone={credential.environment === "production" ? "warning" : "info"}>
												{credential.environment}
											</Badge>
											<Badge tone="neutral">Key {credential.key_version}</Badge>
										</div>
									</div>
									<div class="mt-3 flex flex-wrap gap-2">
										{#if credential.label}
											<Badge tone="neutral">{credential.label}</Badge>
										{/if}
										{#if credential.supported_currencies.length > 0}
											<Badge tone="neutral">
												{formatProviderCurrencies(credential.supported_currencies)}
											</Badge>
										{/if}
										{#if credential.settlement_currency}
											<Badge tone="neutral">
												Settles in {credential.settlement_currency}
											</Badge>
										{/if}
										<Badge tone="neutral">{credential.fx_mode}</Badge>
									</div>
									<div class="mt-4 flex flex-wrap items-center justify-between gap-3">
										<p class="text-xs text-stone-500 dark:text-stone-400">
											Updated {formatDateTime(credential.updated_at)}
										</p>
										<div class="flex flex-wrap gap-2">
											<Button
												tone="admin"
												type="button"
												variant="regular"
												size="small"
												class="rounded-full"
												onclick={() => loadCredentialIntoForm(credential)}
												disabled={credentialRotatingId !== null}
											>
												Load
											</Button>
											<Button
												tone="admin"
												type="button"
												variant="regular"
												size="small"
												class="rounded-full"
												onclick={() => void rotateCredential(credential)}
												disabled={credentialRotatingId !== null}
											>
												{credentialRotatingId === credential.id ? "Rotating..." : "Rotate key"}
											</Button>
										</div>
									</div>
								</div>
							{/each}
						{/if}
					</div>
				</AdminPanel>
			</div>
		{/snippet}
	</AdminMasterDetailLayout>
</section>

<AdminFloatingNotices
	statusMessage={notices.message}
	statusTone={notices.tone}
	onDismissStatus={notices.clear}
/>
