<script lang="ts">
	import type { CmsBlock } from "./types";

	interface Props {
		block: CmsBlock<"faq">;
		blockKey: string;
	}

	let { block, blockKey }: Props = $props();
</script>

<section
	class="mb-10 divide-y divide-gray-200 rounded-lg border border-gray-200 dark:divide-gray-800 dark:border-gray-800"
>
	{#each block.items as item, itemIndex (`${blockKey}-${itemIndex}`)}
		<details class="group px-5 py-4">
			<summary
				class="flex cursor-pointer list-none items-center justify-between gap-4 font-medium text-gray-950 dark:text-gray-50"
			>
				{item.question}
				<i class="bi bi-chevron-down text-sm transition group-open:rotate-180"></i>
			</summary>
			<p class="mt-3 leading-7 text-gray-600 dark:text-gray-300">{item.answer}</p>
		</details>
	{/each}
</section>
