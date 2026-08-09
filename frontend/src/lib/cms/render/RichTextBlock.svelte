<script lang="ts">
	import type { CmsBlock } from "./types";

	interface Props {
		block: CmsBlock<"rich_text">;
	}

	let { block }: Props = $props();
</script>

<section class="cms-prose mb-8">
	<p>{block.body}</p>
</section>

<style>
	@reference "tailwindcss";

	.cms-prose {
		@apply max-w-3xl text-base leading-8 text-gray-700 dark:text-gray-200;
	}

	.cms-prose :global(p + p) {
		@apply mt-4;
	}

	.cms-prose :global(a) {
		@apply font-medium text-blue-700 underline underline-offset-4 dark:text-blue-300;
	}
</style>
