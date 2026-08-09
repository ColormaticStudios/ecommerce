<script lang="ts">
	import { cmsMediaURL } from "$lib/cms";
	import type { CmsBlock } from "./types";

	interface Props {
		block: CmsBlock<"gallery">;
		blockKey: string;
	}

	let { block, blockKey }: Props = $props();
</script>

<section class="mb-10 grid gap-4 sm:grid-cols-2">
	{#each block.images as image, imageIndex (`${blockKey}-${imageIndex}`)}
		<figure>
			<img
				src={cmsMediaURL(image.media_id)}
				alt={image.alt ?? ""}
				class="aspect-4/3 w-full rounded-lg object-cover"
			/>
			{#if image.caption}
				<figcaption class="mt-2 text-sm text-gray-500 dark:text-gray-400">
					{image.caption}
				</figcaption>
			{/if}
		</figure>
	{/each}
</section>
