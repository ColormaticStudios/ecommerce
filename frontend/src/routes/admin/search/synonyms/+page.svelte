<script lang="ts">
	import { getContext, onMount } from "svelte";
	import type { API } from "$lib/api";
	import type { Synonym, SynonymInput } from "$lib/api/domains/search";
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
	import Dropdown from "$lib/components/Dropdown.svelte";
	import TextArea from "$lib/components/TextArea.svelte";

	let { data }: { data: PageData } = $props();
	const api = getContext<API>("api");
	const notices = createAdminNotices();
	const savePrompt = createAdminSavePrompt({
		navigationMessage: "You have unsaved search settings. Leave and discard them?",
	});
	let items: Synonym[] = $derived(data.items);
	let selectedId = $state<number | undefined>();
	let form = $state<SynonymInput>({ name: "", direction: "bi", terms: [], is_active: true });
	let saving = $state(false);
	let query = $state("");
	let savedSnapshot = $state("");
	let terms = $state("");
	const snapshot = $derived(JSON.stringify({ form, terms }));
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
	function toInput(value: Synonym): SynonymInput {
		return {
			name: structuredClone(value.name),
			direction: structuredClone(value.direction),
			terms: structuredClone(value.terms),
			is_active: structuredClone(value.is_active),
		};
	}
	function open(value?: Synonym) {
		if (!savePrompt.confirmDiscard()) return;
		selectedId = value?.id;
		form = value ? toInput(value) : { name: "", direction: "bi", terms: [], is_active: true };
		terms = value ? value.terms.join("\n") : "";
		savedSnapshot = snapshot;
	}
	async function refresh() {
		try {
			items = await api.listAdminSearchSynonyms();
		} catch {
			notices.setError("Unable to load synonym sets.");
		}
	}
	async function save() {
		saving = true;
		notices.clear();
		try {
			const body: SynonymInput = {
				...form,
				terms: terms
					.split("\n")
					.map((term) => term.trim())
					.filter(Boolean),
			};
			const saved = await api.saveAdminSearchSynonym(body, selectedId);
			selectedId = saved.id;
			form = toInput(saved);
			terms = saved.terms.join("\n");
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
			await api.deleteAdminSearchSynonym(selectedId);
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
	<AdminPageHeader title="Search synonyms"
		>{#snippet actions()}<AdminResourceActions
				countLabel={`${items.length} synonym sets`}
			/>{/snippet}</AdminPageHeader
	>
	{#if data.errorMessage}<AdminEmptyState tone="error">{data.errorMessage}</AdminEmptyState>{/if}
	<AdminMasterDetailLayout>
		{#snippet master()}<AdminPanel title="synonym sets">
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
									tone={item.is_active ? "success" : "neutral"}
									>{item.is_active ? "Enabled" : "Disabled"}</Badge
								>
							</div></AdminListItem
						>{:else}<AdminEmptyState>No synonym sets found.</AdminEmptyState>{/each}
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
					><label class="block text-sm font-medium text-stone-700 dark:text-stone-300"
						>Direction<Dropdown tone="admin" bind:value={form.direction} class="mt-1"
							><option value="bi">Equivalent terms</option><option value="uni"
								>First term expands to the others</option
							></Dropdown
						></label
					>
					<label class="block text-sm font-medium text-stone-700 dark:text-stone-300"
						>Terms, one per line<TextArea
							tone="admin"
							required
							bind:value={terms}
							class="mt-1"
						/></label
					>
					<p class="text-sm text-stone-500">
						Equivalent terms expand in both directions. For one-way expansion, put the source first.
					</p>
					<label class="flex items-center gap-2 text-sm"
						><input type="checkbox" bind:checked={form.is_active} />Enabled</label
					>
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
