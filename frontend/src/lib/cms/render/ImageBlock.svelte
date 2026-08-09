<script lang="ts">
	import { cmsMediaURL } from "$lib/cms";
	import type { CmsBlock } from "./types";

	interface Props {
		block: CmsBlock<"image">;
	}

	let { block }: Props = $props();
</script>

<figure class="mb-10">
	<img
		src={cmsMediaURL(block.media_id)}
		alt={block.alt ?? ""}
		class="w-full rounded-lg object-cover"
	/>
	{#if block.caption}
		<figcaption class="mt-2 text-sm text-gray-500 dark:text-gray-400">
			{block.caption}
		</figcaption>
	{/if}
</figure>
