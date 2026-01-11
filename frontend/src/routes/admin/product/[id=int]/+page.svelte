<script lang="ts">
	import { type API } from "$lib/api";
	import { checkAdminAccess } from "$lib/admin/auth";
	import ProductEditor from "$lib/admin/ProductEditor.svelte";
	import { getContext, onMount } from "svelte";
	import { page } from "$app/state";
	import { resolve } from "$app/paths";

	const api: API = getContext("api");
	const productId = $derived(Number(page.params.id));
	const hasProductId = $derived(Number.isFinite(productId) && productId > 0);

	let authChecked = $state(false);
	let isAdmin = $state(false);
	onMount(async () => {
		authChecked = true;
		const result = await checkAdminAccess(api);
		isAdmin = result.isAdmin;
	});
</script>

<section class="mx-auto max-w-5xl px-4 py-10">
	<div class="flex flex-wrap items-start justify-between gap-4">
		<div>
			<h1 class="mt-2 text-2xl font-semibold text-gray-900 dark:text-gray-100">Product editor</h1>
		</div>
		<div class="flex items-center gap-2">
			<a href={resolve("/admin")} class="btn btn-regular"> Back to admin </a>
			{#if hasProductId}
				<a href={resolve(`/product/${productId}`)} class="btn btn-regular"> View live </a>
			{/if}
		</div>
	</div>

	{#if !authChecked}
		<p class="mt-6 text-sm text-gray-500 dark:text-gray-400">Checking access…</p>
	{:else if !api.isAuthenticated()}
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
		</div>
	{:else}
		<ProductEditor {productId} layout="split" showHeader={false} showClear={false} />
	{/if}
</section>
