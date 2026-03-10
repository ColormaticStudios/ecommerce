<script lang="ts">
	interface Props {
		class?: string;
		columnsClass?: string;
		lead?: import("svelte").Snippet;
		master?: import("svelte").Snippet;
		detail?: import("svelte").Snippet;
		masterClass?: string;
		detailClass?: string;
	}

	let {
		class: className = "",
		columnsClass = "lg:grid-cols-2",
		lead,
		master,
		detail,
		masterClass = "",
		detailClass = "",
	}: Props = $props();
</script>

<div class={`space-y-6 ${className}`}>
	{#if lead}
		{@render lead()}
	{/if}

	<div class={`grid items-start gap-6 ${columnsClass}`}>
		<div class={masterClass}>
			{@render master?.()}
		</div>
		<div class={detailClass}>
			{@render detail?.()}
		</div>
	</div>
</div>
