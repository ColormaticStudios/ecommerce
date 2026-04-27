<script lang="ts">
	import { getContext, onMount } from "svelte";
	import type { API } from "$lib/api";
	import type { components } from "$lib/api/generated/openapi";
	import AdminEmptyState from "$lib/admin/AdminEmptyState.svelte";
	import AdminFloatingNotices from "$lib/admin/AdminFloatingNotices.svelte";
	import AdminPageHeader from "$lib/admin/AdminPageHeader.svelte";
	import AdminSurface from "$lib/admin/AdminSurface.svelte";
	import { createAdminNotices, createAdminSavePrompt } from "$lib/admin/state.svelte";
	import Button from "$lib/components/Button.svelte";
	import TextInput from "$lib/components/TextInput.svelte";

	type WebsiteSettings = components["schemas"]["WebsiteSettings"];

	const api: API = getContext("api");
	const notices = createAdminNotices();
	const savePrompt = createAdminSavePrompt({
		navigationMessage: "You have unsaved website settings. Leave this section and discard them?",
	});

	const emptySettings: WebsiteSettings = {
		allow_guest_checkout: true,
		oidc_provider: "",
		oidc_client_id: "",
		oidc_client_secret: "",
		oidc_client_secret_configured: false,
		clear_oidc_client_secret: false,
		oidc_redirect_uri: "",
	};

	let loading = $state(true);
	let saving = $state(false);
	let loadErrorMessage = $state("");
	let updatedAt = $state<string | null>(null);
	let draft = $state<WebsiteSettings>({ ...emptySettings });
	let savedSnapshot = $state(JSON.stringify(emptySettings));
	const currentSnapshot = $derived(JSON.stringify(normalizeSettings(draft)));
	const hasUnsavedChanges = $derived(!loading && currentSnapshot !== savedSnapshot);
	const oidcEnabled = $derived(
		draft.oidc_provider.trim() !== "" &&
			draft.oidc_client_id.trim() !== "" &&
			draft.oidc_redirect_uri.trim() !== ""
	);

	function normalizeSettings(settings: WebsiteSettings): WebsiteSettings {
		return {
			allow_guest_checkout: settings.allow_guest_checkout,
			oidc_provider: settings.oidc_provider.trim(),
			oidc_client_id: settings.oidc_client_id.trim(),
			oidc_client_secret: settings.oidc_client_secret.trim(),
			oidc_client_secret_configured: settings.oidc_client_secret_configured,
			clear_oidc_client_secret: settings.clear_oidc_client_secret,
			oidc_redirect_uri: settings.oidc_redirect_uri.trim(),
		};
	}

	function applyLoadedSettings(settings: WebsiteSettings, nextUpdatedAt: string | null) {
		const normalized = normalizeSettings(settings);
		draft = {
			...normalized,
			oidc_client_secret: normalized.oidc_client_secret_configured ? "********" : "",
			clear_oidc_client_secret: false,
		};
		savedSnapshot = JSON.stringify(draft);
		updatedAt = nextUpdatedAt;
	}

	async function loadSettings() {
		loading = true;
		loadErrorMessage = "";
		notices.clear();
		try {
			const response = await api.getAdminWebsiteSettings();
			applyLoadedSettings(response.settings, response.updated_at ?? null);
		} catch (error) {
			console.error(error);
			loadErrorMessage = "Unable to load website settings.";
			notices.setError(loadErrorMessage);
		} finally {
			loading = false;
		}
	}

	async function saveSettings() {
		saving = true;
		notices.clear();
		try {
			const response = await api.updateWebsiteSettings(normalizeSettings(draft));
			applyLoadedSettings(response.settings, response.updated_at ?? null);
			notices.setSuccess("Website settings saved.");
		} catch (error) {
			console.error(error);
			notices.setError("Unable to save website settings.");
		} finally {
			saving = false;
		}
	}

	onMount(() => {
		void loadSettings();
	});

	$effect(() => {
		savePrompt.dirty = hasUnsavedChanges;
		savePrompt.blocked = saving;
		savePrompt.saveAction = hasUnsavedChanges ? saveSettings : null;
	});
</script>

<svelte:head>
	<title>Website settings - Admin</title>
</svelte:head>

<AdminFloatingNotices
	showUnsaved={savePrompt.dirty}
	unsavedMessage="You have unsaved website settings."
	canSaveUnsaved={savePrompt.canSave}
	onSaveUnsaved={() => void savePrompt.save()}
	savingUnsaved={savePrompt.saving || saving}
	statusMessage={notices.message}
	statusTone={notices.tone}
	onDismissStatus={() => notices.clear()}
/>

<div class="mx-auto w-full max-w-5xl px-4 py-8 sm:px-6 lg:px-8">
	<AdminPageHeader title="Website settings" />

	<div class="mt-4 flex flex-wrap items-center justify-between gap-3">
		<p class="text-sm text-stone-600 dark:text-stone-300">
			{updatedAt ? `Last saved ${new Date(updatedAt).toLocaleString()}` : "No saved timestamp"}
		</p>
		<Button
			tone="admin"
			variant="primary"
			disabled={loading || saving || !hasUnsavedChanges}
			onclick={saveSettings}
		>
			<i class="bi bi-floppy mr-1"></i>
			{saving ? "Saving..." : "Save changes"}
		</Button>
	</div>

	{#if loading}
		<AdminSurface variant="muted" as="div" class="mt-6 text-sm text-stone-600 dark:text-stone-300">
			Loading website settings...
		</AdminSurface>
	{:else}
		<div class="mt-6 space-y-6">
			{#if loadErrorMessage}
				<AdminEmptyState tone="error">{loadErrorMessage}</AdminEmptyState>
			{/if}

			<AdminSurface variant="subsurface">
				<h3 class="text-sm font-semibold text-stone-900 dark:text-stone-100">Checkout access</h3>
				<label class="mt-4 flex items-start gap-3 text-sm text-stone-700 dark:text-stone-200">
					<input
						class="mt-1 h-4 w-4 shrink-0"
						type="checkbox"
						bind:checked={draft.allow_guest_checkout}
					/>
					<span>Allow guests to create carts and start checkout without signing in.</span>
				</label>
			</AdminSurface>

			<AdminSurface variant="subsurface">
				<div class="flex flex-wrap items-center justify-between gap-3">
					<h3 class="text-sm font-semibold text-stone-900 dark:text-stone-100">OpenID Connect</h3>
					<span class="text-sm text-stone-600 dark:text-stone-300">
						{oidcEnabled ? "Enabled" : "Disabled"}
					</span>
				</div>
				<div class="mt-4 grid gap-4 md:grid-cols-2">
					<label class="block text-sm text-stone-700 dark:text-stone-200">
						<span class="mb-1 block font-medium">Provider URL</span>
						<TextInput
							tone="admin"
							bind:value={draft.oidc_provider}
							placeholder="https://issuer.example"
						/>
					</label>
					<label class="block text-sm text-stone-700 dark:text-stone-200">
						<span class="mb-1 block font-medium">Redirect URI</span>
						<TextInput
							tone="admin"
							bind:value={draft.oidc_redirect_uri}
							placeholder="https://shop.example/api/v1/auth/oidc/callback"
						/>
					</label>
					<label class="block text-sm text-stone-700 dark:text-stone-200">
						<span class="mb-1 block font-medium">Client ID</span>
						<TextInput tone="admin" bind:value={draft.oidc_client_id} placeholder="client-id" />
					</label>
					<label class="block text-sm text-stone-700 dark:text-stone-200">
						<span class="mb-1 block font-medium">Client secret</span>
						<TextInput
							tone="admin"
							type="password"
							bind:value={draft.oidc_client_secret}
							placeholder="Optional client secret"
							disabled={draft.clear_oidc_client_secret}
						/>
					</label>
				</div>
				<label class="mt-4 flex items-start gap-3 text-sm text-stone-700 dark:text-stone-200">
					<input
						class="mt-1 h-4 w-4 shrink-0"
						type="checkbox"
						bind:checked={draft.clear_oidc_client_secret}
						onchange={() => {
							if (draft.clear_oidc_client_secret) {
								draft.oidc_client_secret = "";
							}
						}}
					/>
					<span>Clear the stored OIDC client secret on save.</span>
				</label>
			</AdminSurface>
		</div>
	{/if}
</div>
