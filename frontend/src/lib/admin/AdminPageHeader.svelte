<script lang="ts">
	interface Props {
		title: string;
		actions?: import("svelte").Snippet;
	}

	let { title, actions }: Props = $props();
</script>

<div class="admin-page-header items-center">
	<div>
		<h1 class="admin-page-title">{title}</h1>
	</div>

	{#if actions}
		<div class="flex flex-wrap items-center gap-2 self-center">
			{@render actions()}
		</div>
	{/if}
</div>
