<script lang="ts">
	interface Props {
		children?: import("svelte").Snippet;
	}

	let { children }: Props = $props();
</script>

<tbody class="divide-y divide-stone-200 bg-white dark:divide-stone-800 dark:bg-stone-950">
	{@render children?.()}
</tbody>
