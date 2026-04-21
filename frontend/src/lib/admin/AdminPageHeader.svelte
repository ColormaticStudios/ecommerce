<script lang="ts">
	interface Props {
		title: string;
		actions?: import("svelte").Snippet;
	}

	let { title, actions }: Props = $props();
</script>

<div class="flex flex-wrap items-center justify-between gap-4">
	<div class="min-w-0">
		<h1
			class="text-[clamp(2rem,2.4vw,2.65rem)] font-semibold tracking-[-0.03em] text-stone-950 dark:text-stone-50"
		>
			{title}
		</h1>
	</div>

	{#if actions}
		<div class="flex max-w-full min-w-0 flex-wrap items-center gap-2 self-center">
			{@render actions()}
		</div>
	{/if}
</div>
