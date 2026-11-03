<script lang="ts">
	import AdminSearchTabs from "$lib/admin/AdminSearchTabs.svelte";
	let { children }: { children: import("svelte").Snippet } = $props();
</script>

<AdminSearchTabs />
{@render children()}
