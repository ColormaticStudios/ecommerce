<script lang="ts">
	import AdminSurface from "$lib/admin/AdminSurface.svelte";

	interface Props {
		title?: string;
		meta?: string;
		class?: string;
		headerActions?: import("svelte").Snippet;
		children?: import("svelte").Snippet;
	}

	let { title = "", meta = "", class: className = "", headerActions, children }: Props = $props();
</script>

<AdminSurface class={className}>
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
						<span
							class="inline-flex items-center gap-2 rounded-full border border-stone-200/90 bg-white/80 px-3 py-1 text-xs font-medium text-stone-600 shadow-sm dark:border-stone-800 dark:bg-stone-950/70 dark:text-stone-300"
						>
							{meta}
						</span>
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
</AdminSurface>
