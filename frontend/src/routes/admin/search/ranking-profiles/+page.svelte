<script lang="ts">
	import { getContext, onMount, untrack } from "svelte";
	import type { API } from "$lib/api";
	import type { RankingProfile, RankingInput } from "$lib/api/domains/search";
	import type { PageData } from "./$types";
	import AdminPageHeader from "$lib/admin/AdminPageHeader.svelte";
	import AdminMasterDetailLayout from "$lib/admin/AdminMasterDetailLayout.svelte";
	import AdminPanel from "$lib/admin/AdminPanel.svelte";
	import AdminListItem from "$lib/admin/AdminListItem.svelte";
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminResourceActions from "$lib/admin/AdminResourceActions.svelte";
	import AdminFloatingNotices from "$lib/admin/AdminFloatingNotices.svelte";
	import { createAdminNotices, createAdminSavePrompt } from "$lib/admin/state.svelte";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import TextInput from "$lib/components/TextInput.svelte";
	import NumberInput from "$lib/components/NumberInput.svelte";

	let { data }: { data: PageData } = $props();
	const api = getContext<API>("api");
	const notices = createAdminNotices();
	const savePrompt = createAdminSavePrompt({
		navigationMessage: "You have unsaved search settings. Leave and discard them?",
	});
	let items: RankingProfile[] = $derived(data.items);
	let selectedId = $state<number | undefined>();
	function newRankingInput(profiles: RankingProfile[]): RankingInput {
		const currentDefault = profiles.find((profile) => profile.is_default);
		return {
			name: "",
			weights: structuredClone(
				currentDefault?.weights ?? {
					token_coverage: 8,
					exact_phrase: 6,
					name: 5,
					brand: 2,
					attributes: 2,
					recency: 0.5,
					availability: 1,
					sales: 1,
					margin: 0,
					conversion: 0,
				}
			),
			is_default: false,
		};
	}
	let form = $state<RankingInput>(newRankingInput(untrack(() => data.items)));

	let saving = $state(false);
	let query = $state("");
	let savedSnapshot = $state("");
	const snapshot = $derived(JSON.stringify(form));
	const weightFields = [
		{ key: "token_coverage", label: "Token coverage" },
		{ key: "exact_phrase", label: "Exact phrase" },
		{ key: "name", label: "Product name" },
		{ key: "brand", label: "Brand" },
		{ key: "attributes", label: "Attributes" },
		{ key: "recency", label: "Recency" },
		{ key: "availability", label: "Availability" },
		{ key: "sales", label: "Sales" },
		{ key: "margin", label: "Gross margin" },
		{ key: "conversion", label: "Purchase conversion" },
	] as const;
	const dirty = $derived(savedSnapshot !== "" && snapshot !== savedSnapshot);
	const visibleItems = $derived(
		items.filter((item) => item.name.toLowerCase().includes(query.toLowerCase()))
	);
	$effect(() => {
		savePrompt.dirty = dirty;
		savePrompt.blocked = saving;
		savePrompt.saveAction = dirty ? save : null;
	});
	onMount(() => {
		savedSnapshot = snapshot;
	});
	function toInput(value: RankingProfile): RankingInput {
		return {
			name: structuredClone(value.name),
			weights: structuredClone(value.weights),
			is_default: structuredClone(value.is_default),
		};
	}
	function open(value?: RankingProfile) {
		if (!savePrompt.confirmDiscard()) return;
		selectedId = value?.id;
		form = value ? toInput(value) : newRankingInput(items);

		savedSnapshot = snapshot;
	}
	async function refresh() {
		try {
			items = await api.listAdminSearchRankingProfiles();
		} catch {
			notices.setError("Unable to load ranking profiles.");
		}
	}
	async function save() {
		saving = true;
		notices.clear();
		try {
			const body: RankingInput = form;
			const saved = await api.saveAdminSearchRankingProfile(body, selectedId);
			selectedId = saved.id;
			form = toInput(saved);
			savedSnapshot = snapshot;
			await refresh();
			notices.setSuccess("Search settings saved.");
		} catch (error) {
			notices.setError(error instanceof Error ? error.message : "Unable to save search settings.");
		} finally {
			saving = false;
		}
	}
	async function remove() {
		if (!selectedId || !confirm("Delete this search configuration?")) return;
		saving = true;
		try {
			await api.deleteAdminSearchRankingProfile(selectedId);
			savedSnapshot = snapshot;
			open();
			await refresh();
			notices.setSuccess("Search configuration deleted.");
		} catch (error) {
			notices.setError(
				error instanceof Error ? error.message : "Unable to delete search configuration."
			);
		} finally {
			saving = false;
		}
	}
</script>

<section class="space-y-6 pb-16">
	<AdminPageHeader title="Search ranking profiles"
		>{#snippet actions()}<AdminResourceActions
				countLabel={`${items.length} ranking profiles`}
			/>{/snippet}</AdminPageHeader
	>
	{#if data.errorMessage}<AdminEmptyState tone="error">{data.errorMessage}</AdminEmptyState>{/if}
	<AdminMasterDetailLayout>
		{#snippet master()}<AdminPanel title="ranking profiles">
				<AdminResourceActions
					bind:searchValue={query}
					searchPlaceholder="Filter by name"
					onSearch={() => {}}
					onRefresh={refresh}
				/>
				<div class="mt-4 space-y-2">
					{#each visibleItems as item (item.id)}<AdminListItem
							as="button"
							interactive
							active={item.id === selectedId}
							onclick={() => open(item)}
							><div class="flex items-center justify-between gap-2">
								<span class="font-medium">{item.name}</span><Badge
									tone={item.is_default ? "success" : "neutral"}
									>{item.is_default ? "Default" : `Version ${item.version}`}</Badge
								>
							</div></AdminListItem
						>{:else}<AdminEmptyState>No ranking profiles found.</AdminEmptyState>{/each}
				</div>
			</AdminPanel>{/snippet}
		{#snippet detail()}<AdminPanel
				title={selectedId ? "edit configuration" : "create configuration"}
			>
				<form
					class="space-y-4"
					onsubmit={(event) => {
						event.preventDefault();
						void save();
					}}
				>
					<label class="block text-sm font-medium text-stone-700 dark:text-stone-300"
						>Name<TextInput tone="admin" required bind:value={form.name} class="mt-1" /></label
					>
					<p class="text-sm text-stone-500">
						Weights range from 0 to 100. Zero disables a boost. Gross margin uses the lowest
						published variant margin; unknown costs contribute zero. Purchase conversion uses
						consented impressions and paid purchases after the seven-day attribution window.
					</p>
					<div class="grid gap-4 sm:grid-cols-2">
						{#each weightFields as field (field.key)}<label
								class="block text-sm font-medium text-stone-700 dark:text-stone-300"
								>{field.label}<NumberInput
									tone="admin"
									min="0"
									max="100"
									allowDecimal
									bind:value={form.weights[field.key]}
									class="mt-1"
								/></label
							>{/each}
					</div>
					<label class="flex items-center gap-2 text-sm"
						><input type="checkbox" bind:checked={form.is_default} />Use as the default profile</label
					>
					<p class="text-sm text-stone-500">
						Selecting a default replaces the previous default. Changes take effect without a
						reindex.
					</p>
					<div class="flex flex-wrap gap-2">
						<Button tone="admin" variant="primary" type="submit" disabled={saving}
							>{saving ? "Saving…" : selectedId ? "Save changes" : "Create"}</Button
						><Button tone="admin" type="button" onclick={() => open()} disabled={saving}
							>Clear</Button
						>{#if selectedId}<Button
								tone="admin"
								variant="danger"
								type="button"
								onclick={remove}
								disabled={saving}>Delete</Button
							>{/if}
					</div>
				</form></AdminPanel
			>{/snippet}
	</AdminMasterDetailLayout>
</section>
<AdminFloatingNotices
	statusMessage={notices.message}
	statusTone={notices.tone}
	onDismissStatus={notices.clear}
/>
