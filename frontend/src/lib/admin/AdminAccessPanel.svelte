<script lang="ts">
	interface Props {
		isAuthenticated: boolean;
		accessError?: string;
	}

	let { isAuthenticated, accessError = "" }: Props = $props();
</script>

<div
	class="rounded-4xl border border-dashed border-stone-300 bg-white/90 p-8 text-stone-700 shadow-[0_20px_80px_-45px_rgba(41,37,36,0.45)] backdrop-blur dark:border-stone-700 dark:bg-stone-950/90 dark:text-stone-300"
>
	<p class="text-lg font-semibold text-stone-900 dark:text-stone-100">Access denied.</p>
	{#if !isAuthenticated}
		<p class="mt-2 text-sm">
			You must be signed in to an admin account to access the admin console.
		</p>
	{:else}
		<p class="mt-2 text-sm">{accessError || "Contact an administrator if you need access."}</p>
	{/if}
</div>
