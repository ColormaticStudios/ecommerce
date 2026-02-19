<script lang="ts">
	import { type API } from "$lib/api";
	import AdminFloatingNotices from "$lib/admin/AdminFloatingNotices.svelte";
	import { checkAdminAccess } from "$lib/admin/auth";
	import ButtonLink from "$lib/components/ButtonLink.svelte";
	import ProductEditor from "$lib/admin/ProductEditor.svelte";
	import { getContext, onMount } from "svelte";
	import { page } from "$app/state";
	import { resolve } from "$app/paths";

	const api: API = getContext("api");
	const productId = $derived(Number(page.params.id));
	const hasProductId = $derived(Number.isFinite(productId) && productId > 0);

	let authChecked = $state(false);
	let loading = $state(true);
	let isAuthenticated = $state(false);
	let isAdmin = $state(false);
	let accessError = $state("");
	let noticeMessage = $state("");
	let noticeTone = $state<"success" | "error" | null>(null);
	let noticeSaving = $state(false);
	let productDirty = $state(false);
	let productSaveAction = $state<(() => Promise<void>) | null>(null);
	const hasUnsavedChanges = $derived(productDirty);
	const canSaveUnsaved = $derived(productSaveAction !== null && !noticeSaving);

	function clearMessages() {
		noticeMessage = "";
		noticeTone = null;
	}

	function setNotice(tone: "success" | "error", message: string) {
		noticeTone = tone;
		noticeMessage = message;
	}

	function setErrorMessage(message: string) {
		if (!message.trim()) {
			return;
		}
		setNotice("error", message);
	}

	function setStatusMessage(message: string) {
		if (!message.trim()) {
			return;
		}
		setNotice("success", message);
	}

	function setProductDirty(dirty: boolean) {
		productDirty = dirty;
	}

	function setProductSaveRequest(action: (() => Promise<void>) | null) {
		productSaveAction = action;
	}

	async function saveUnsavedChanges() {
		if (!productSaveAction || noticeSaving) {
			return;
		}
		noticeSaving = true;
		try {
			await productSaveAction();
		} catch (err) {
			console.error(err);
			setNotice("error", "Unable to save pending changes.");
		} finally {
			noticeSaving = false;
		}
	}

	onMount(async () => {
		loading = true;
		authChecked = false;
		accessError = "";
		clearMessages();
		try {
			const result = await checkAdminAccess(api);
			isAuthenticated = result.isAuthenticated;
			isAdmin = result.isAdmin;
		} catch (err) {
			console.error(err);
			isAdmin = false;
			accessError = "Unable to check admin access.";
		} finally {
			authChecked = true;
			loading = false;
		}
	});
</script>

<section class="mx-auto max-w-5xl px-4 py-10">
	{#if !authChecked || loading}
		<div
			class="mt-6 rounded-2xl border border-gray-200 bg-white p-6 text-sm text-gray-600 shadow-sm dark:border-gray-800 dark:bg-gray-900 dark:text-gray-300"
		>
			Loading admin console...
		</div>
	{:else if !isAuthenticated}
		<div
			class="mt-6 rounded-2xl border border-dashed border-gray-300 bg-white p-6 text-gray-600 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-300"
		>
			<p class="text-lg font-medium">Access denied.</p>
			<p class="mt-2 text-sm">
				You must be signed in to an admin account to access the admin console.
			</p>
		</div>
	{:else if !isAdmin}
		<div
			class="mt-6 rounded-2xl border border-dashed border-gray-300 bg-white p-6 text-gray-600 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-300"
		>
			<p class="text-lg font-medium">Access denied.</p>
			{#if accessError}
				<p class="mt-2 text-sm">{accessError}</p>
			{:else}
				<p class="mt-2 text-sm">Contact an administrator if you need access.</p>
			{/if}
		</div>
	{:else}
		<div class="flex flex-wrap items-start justify-between gap-4">
			<div>
				<h1 class="mt-2 text-2xl font-semibold text-gray-900 dark:text-gray-100">Product editor</h1>
			</div>
			<div class="flex items-center gap-2">
				<ButtonLink href={resolve("/admin")} variant="regular">Back to admin</ButtonLink>
				{#if hasProductId}
					<ButtonLink href={resolve(`/product/${productId}`)} variant="regular"
						>View live</ButtonLink
					>
				{/if}
			</div>
		</div>
		<ProductEditor
			{productId}
			layout="split"
			showHeader={false}
			showClear={false}
			showMessages={false}
			onErrorMessage={setErrorMessage}
			onStatusMessage={setStatusMessage}
			onDirtyChange={setProductDirty}
			onSaveRequestChange={setProductSaveRequest}
		/>
	{/if}
</section>

{#if isAdmin}
	<AdminFloatingNotices
		showUnsaved={hasUnsavedChanges}
		unsavedMessage="You have unsaved product changes."
		{canSaveUnsaved}
		onSaveUnsaved={saveUnsavedChanges}
		savingUnsaved={noticeSaving}
		statusMessage={noticeMessage}
		statusTone={noticeTone}
		onDismissStatus={clearMessages}
	/>
{/if}
