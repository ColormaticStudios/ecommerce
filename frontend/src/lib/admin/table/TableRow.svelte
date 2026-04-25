<script lang="ts">
	interface Props {
		class?: string;
		children?: import("svelte").Snippet;
	}

	let { class: className = "", children }: Props = $props();
</script>

<tr class={`text-stone-800 dark:text-stone-100 ${className}`}>
	{@render children?.()}
</tr>
