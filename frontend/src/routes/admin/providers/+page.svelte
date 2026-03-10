<script lang="ts">
	import { getContext } from "svelte";
	import { type API } from "$lib/api";
	import AdminBadge from "$lib/admin/AdminBadge.svelte";
	import AdminFloatingNotices from "$lib/admin/AdminFloatingNotices.svelte";
	import AdminPageHeader from "$lib/admin/AdminPageHeader.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import AdminResourceActions from "$lib/admin/AdminResourceActions.svelte";
	import { createAdminNotices } from "$lib/admin/state.svelte";
	import Button from "$lib/components/Button.svelte";
	import TabSwitcher from "$lib/components/TabSwitcher.svelte";
	import type { components } from "$lib/api/generated/openapi";
	import type { PageData } from "./$types";

	interface Props {
		data: PageData;
	}

	let { data }: Props = $props();
	const api: API = getContext("api");

	type CheckoutPluginCatalog = components["schemas"]["CheckoutPluginCatalog"];
	type CheckoutPlugin = components["schemas"]["CheckoutPlugin"];
	type CheckoutPluginType = CheckoutPlugin["type"];
	type ProviderView = "all" | "payment" | "shipping" | "tax";

	let providersSaving = $state(false);
	let providerCatalog = $state<CheckoutPluginCatalog>({
		payment: [],
		shipping: [],
		tax: [],
	});
	let providerView = $state<ProviderView>("all");
	const providerTabs = [
		{ id: "all", label: "All", icon: "bi-grid-1x2" },
		{ id: "payment", label: "Payment", icon: "bi-credit-card-2-front" },
		{ id: "shipping", label: "Shipping", icon: "bi-truck" },
		{ id: "tax", label: "Tax", icon: "bi-receipt" },
	];

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
	const notices = createAdminNotices();

	async function loadProviders() {
		providersSaving = true;
		notices.clear();
		try {
			providerCatalog = await api.listAdminCheckoutPlugins();
		} catch (error) {
			console.error(error);
			notices.pushError("Unable to load checkout providers.");
		} finally {
			providersSaving = false;
		}
	}

	async function updateProviderEnabled(
		type: CheckoutPluginType,
		providerID: string,
		enabled: boolean
	) {
		if (providersSaving) {
			return;
		}

		providersSaving = true;
		notices.clear();
		try {
			providerCatalog = await api.updateAdminCheckoutPlugin(type, providerID, { enabled });
			notices.pushSuccess("Provider settings updated.");
		} catch (error) {
			console.error(error);
			const err = error as { body?: { error?: string } };
			notices.pushError(err.body?.error ?? "Unable to update provider settings.");
		} finally {
			providersSaving = false;
		}
	}

	$effect(() => {
		providerCatalog = data.checkoutPlugins ?? { payment: [], shipping: [], tax: [] };
		if (data.errorMessage) {
			notices.pushError(data.errorMessage);
		}
	});
</script>

{#snippet providerActions()}
	<AdminResourceActions
		countLabel={`${providerCatalog.payment.length + providerCatalog.shipping.length + providerCatalog.tax.length} providers`}
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
		onclick={loadProviders}
		disabled={providersSaving}
	>
		Refresh
	</Button>
{/snippet}

<section class="space-y-6">
	<AdminPageHeader title="Providers" actions={providerActions} />

	<TabSwitcher items={providerTabs} bind:value={providerView} ariaLabel="Provider categories" />

	<div class={`grid gap-4 ${providerView === "all" ? "lg:grid-cols-3" : ""}`}>
		{#each providerSections as section (section.type)}
			<AdminPanel
				title={section.title}
				meta={`${section.plugins.length} provider${section.plugins.length === 1 ? "" : "s"}`}
			>
				<div class="space-y-3">
					{#if section.plugins.length === 0}
						<p class="admin-empty-state">No providers found.</p>
					{:else}
						{#each section.plugins as provider (provider.id)}
							<div class="admin-list-item p-4">
								<div class="flex items-start justify-between gap-3">
									<div>
										<p class="text-sm font-medium text-stone-950 dark:text-stone-50">
											{provider.name}
										</p>
										<p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
											{provider.description}
										</p>
									</div>
									<AdminBadge tone={provider.enabled ? "success" : "neutral"}>
										{provider.enabled ? "Enabled" : "Disabled"}
									</AdminBadge>
								</div>
								<div class="mt-4 flex justify-end">
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
					{/if}
				</div>
			</AdminPanel>
		{/each}
	</div>
</section>

<AdminFloatingNotices
	statusMessage={notices.message}
	statusTone={notices.tone}
	onDismissStatus={notices.clear}
/>
