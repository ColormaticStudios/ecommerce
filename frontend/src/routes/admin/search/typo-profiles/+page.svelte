<script lang="ts">
	import { getContext, onMount } from "svelte";
	import type { API } from "$lib/api";
	import type { TypoProfile, TypoInput } from "$lib/api/domains/search";
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
	let items: TypoProfile[] = $derived(data.items);
	let selectedId = $state<number | undefined>();
	let form = $state<TypoInput>({
		name: "",
		minimum_token_length: 4,
		one_edit_minimum_length: 4,
		two_edit_minimum_length: 8,
		strict_mode: false,
		is_active: false,
	});
	let saving = $state(false);
	let query = $state("");
	let savedSnapshot = $state("");
	const snapshot = $derived(JSON.stringify(form));
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
	function toInput(value: TypoProfile): TypoInput {
		return {
			name: structuredClone(value.name),
			minimum_token_length: structuredClone(value.minimum_token_length),
			one_edit_minimum_length: structuredClone(value.one_edit_minimum_length),
			two_edit_minimum_length: structuredClone(value.two_edit_minimum_length),
			strict_mode: structuredClone(value.strict_mode),
			is_active: structuredClone(value.is_active),
		};
	}
	function open(value?: TypoProfile) {
		if (!savePrompt.confirmDiscard()) return;
		selectedId = value?.id;
		form = value
			? toInput(value)
			: {
					name: "",
					minimum_token_length: 4,
					one_edit_minimum_length: 4,
					two_edit_minimum_length: 8,
					strict_mode: false,
					is_active: false,
				};

		savedSnapshot = snapshot;
	}
	async function refresh() {
		try {
			items = await api.listAdminSearchTypoProfiles();
		} catch {
			notices.setError("Unable to load typo profiles.");
		}
	}
	async function save() {
		saving = true;
		notices.clear();
		try {
			const body: TypoInput = form;
			const saved = await api.saveAdminSearchTypoProfile(body, selectedId);
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
			await api.deleteAdminSearchTypoProfile(selectedId);
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
	<AdminPageHeader title="Search typo profiles"
		>{#snippet actions()}<AdminResourceActions
				countLabel={`${items.length} typo profiles`}
			/>{/snippet}</AdminPageHeader
	>
	{#if data.errorMessage}<AdminEmptyState tone="error">{data.errorMessage}</AdminEmptyState>{/if}
	<AdminMasterDetailLayout>
		{#snippet master()}<AdminPanel title="typo profiles">
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
						>{:else}<AdminEmptyState>No typo profiles found.</AdminEmptyState>{/each}
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
						>Minimum token length<NumberInput
							tone="admin"
							min="1"
							max="100"
							required
							bind:value={form.minimum_token_length}
							class="mt-1"
						/></label
					><label class="block text-sm font-medium text-stone-700 dark:text-stone-300"
						>Minimum length for one edit<NumberInput
							tone="admin"
							min="1"
							max="100"
							required
							bind:value={form.one_edit_minimum_length}
							class="mt-1"
						/></label
					><label class="block text-sm font-medium text-stone-700 dark:text-stone-300"
						>Minimum length for two edits<NumberInput
							tone="admin"
							min="1"
							max="100"
							required
							bind:value={form.two_edit_minimum_length}
							class="mt-1"
						/></label
					><label class="flex items-center gap-2 text-sm"
						><input type="checkbox" bind:checked={form.strict_mode} />Strict matching</label
					><label class="flex items-center gap-2 text-sm"
						><input type="checkbox" bind:checked={form.is_active} />Use as the active profile</label
					>
					<p class="text-sm text-stone-500">
						Activating this profile replaces the current active profile.
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
