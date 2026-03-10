<script lang="ts">
	import AdminAccessPanel from "$lib/admin/AdminAccessPanel.svelte";
	import AdminShell from "$lib/admin/AdminShell.svelte";
	import type { LayoutData } from "./$types";

	interface Props {
		data: LayoutData;
		children?: import("svelte").Snippet;
	}

	let { data, children }: Props = $props();
</script>

{#if !data.isAuthenticated || !data.isAdmin}
	<section class="mx-auto max-w-4xl px-4 py-10">
		<AdminAccessPanel isAuthenticated={data.isAuthenticated} accessError={data.accessError} />
	</section>
{:else}
	<AdminShell>
		{@render children?.()}
	</AdminShell>
{/if}
