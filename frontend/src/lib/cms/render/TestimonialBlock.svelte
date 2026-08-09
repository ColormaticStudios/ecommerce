<script lang="ts">
	import type { CmsBlock } from "./types";

	interface Props {
		block: CmsBlock<"testimonial">;
	}

	let { block }: Props = $props();
</script>

<section class="mb-10 rounded-lg border border-gray-200 px-6 py-7 dark:border-gray-800">
	{#if block.rating}
		<p class="mb-3 text-sm font-semibold text-amber-600 dark:text-amber-300">
			{"★".repeat(block.rating)}
		</p>
	{/if}
	<blockquote class="text-xl leading-8 text-gray-950 dark:text-gray-50">“{block.quote}”</blockquote>
	<p class="mt-4 text-sm font-medium text-gray-600 dark:text-gray-300">
		{block.attribution}
	</p>
</section>
