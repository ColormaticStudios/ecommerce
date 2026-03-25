<script lang="ts">
	interface Props {
		title?: string;
		meta?: string;
		class?: string;
		headerActions?: import("svelte").Snippet;
		children?: import("svelte").Snippet;
	}

	let { title = "", meta = "", class: className = "", headerActions, children }: Props = $props();
</script>

<section class={`admin-surface ${className}`}>
	{#if title || meta || headerActions}
		<div class="flex flex-wrap items-start justify-between gap-3">
			{#if title}
				<div class="min-w-0">
					{#if title}
						<h2 class="text-lg font-semibold text-stone-950 dark:text-stone-50">{title}</h2>
					{/if}
				</div>
			{/if}

			{#if meta || headerActions}
				<div class="flex max-w-full min-w-0 flex-wrap items-center gap-2">
					{#if meta}
						<span class="admin-page-meta mt-0">{meta}</span>
					{/if}
					{#if headerActions}
						{@render headerActions()}
					{/if}
				</div>
			{/if}
		</div>
	{/if}

	<div class={title || meta || headerActions ? "mt-6" : ""}>
		{@render children?.()}
	</div>
</section>
