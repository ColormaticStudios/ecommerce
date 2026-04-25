<script lang="ts">
	interface Props {
		children?: import("svelte").Snippet;
	}

	let { children }: Props = $props();
</script>

<thead
	class="bg-stone-50 text-left text-xs font-semibold text-stone-500 dark:bg-stone-900 dark:text-stone-400"
>
	{@render children?.()}
</thead>
