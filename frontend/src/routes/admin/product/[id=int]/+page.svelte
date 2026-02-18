<script lang="ts">
	import { type API } from "$lib/api";
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
let isAuthenticated = $state(false);
let isAdmin = $state(false);
	let accessError = $state("");
	onMount(async () => {
		authChecked = true;
		accessError = "";
		try {
			const result = await checkAdminAccess(api);
			isAuthenticated = result.isAuthenticated;
			isAdmin = result.isAdmin;
		} catch (err) {
			console.error(err);
			isAdmin = false;
			accessError = "Unable to check admin access.";
		}
	});
</script>

<section class="mx-auto max-w-5xl px-4 py-10">
	<div class="flex flex-wrap items-start justify-between gap-4">
		<div>
			<h1 class="mt-2 text-2xl font-semibold text-gray-900 dark:text-gray-100">Product editor</h1>
		</div>
		<div class="flex items-center gap-2">
			<ButtonLink href={resolve("/admin")} variant="regular">Back to admin</ButtonLink>
			{#if hasProductId}
				<ButtonLink href={resolve(`/product/${productId}`)} variant="regular">View live</ButtonLink>
			{/if}
		</div>
	</div>

	{#if !authChecked}
		<p class="mt-6 text-sm text-gray-500 dark:text-gray-400">Checking access…</p>
	{:else if !isAuthenticated}
		<p class="mt-6 text-gray-600 dark:text-gray-300">
			Please
			<a href={resolve("/login")} class="text-blue-600 hover:underline dark:text-blue-400">
				log in
			</a>
			to access this page.
		</p>
	{:else if !isAdmin}
		<div
			class="mt-6 rounded-2xl border border-dashed border-gray-300 bg-white p-6 text-gray-600 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-300"
		>
			<p class="text-lg font-medium">You do not have access to this page.</p>
			{#if accessError}
				<p class="mt-2 text-sm">{accessError}</p>
			{/if}
		</div>
	{:else}
		<ProductEditor {productId} layout="split" showHeader={false} showClear={false} />
	{/if}
</section>
