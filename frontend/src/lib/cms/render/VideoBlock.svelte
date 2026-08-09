<script lang="ts">
	import type { CmsBlock } from "./types";

	interface Props {
		block: CmsBlock<"video">;
	}

	let { block }: Props = $props();
</script>

<section class="mb-10">
	<div class="overflow-hidden rounded-lg border border-gray-200 dark:border-gray-800">
		<iframe
			src={block.url}
			title={block.title || "CMS video"}
			class="aspect-video w-full"
			allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
			allowfullscreen
		></iframe>
	</div>
</section>
