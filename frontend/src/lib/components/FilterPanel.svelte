<script lang="ts">
	import Card from "$lib/components/Card.svelte";

	interface Props {
		class?: string;
		children?: import("svelte").Snippet;
	}

	let { class: className = "", children }: Props = $props();
</script>

<Card tone="soft" padding="sm" class={className}>
	{@render children?.()}
</Card>
