<script lang="ts">
	import Card from "$lib/components/Card.svelte";

	interface Props {
		title: string;
		description?: string;
		class?: string;
		headingClass?: string;
		children?: import("svelte").Snippet;
	}

	let {
		title,
		description = "",
		class: className = "",
		headingClass = "text-2xl font-medium",
		children,
	}: Props = $props();
</script>

<Card
	border="dashed"
	padding="xl"
	class={`text-center text-gray-600 dark:text-gray-300 ${className}`.trim()}
>
	<h2 class={`text-gray-900 dark:text-gray-100 ${headingClass}`.trim()}>{title}</h2>
	{#if description}
		<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{description}</p>
	{/if}
	{@render children?.()}
</Card>
