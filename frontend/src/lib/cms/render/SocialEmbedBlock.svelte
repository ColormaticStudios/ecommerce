<script lang="ts">
	/* eslint-disable svelte/no-navigation-without-resolve */
	import type { CmsBlock } from "./types";

	interface Props {
		block: CmsBlock<"social_embed">;
	}

	let { block }: Props = $props();
	const providerLabel = $derived(block.provider.charAt(0).toUpperCase() + block.provider.slice(1));
</script>

<section class="mb-10 rounded-lg border border-gray-200 px-6 py-7 dark:border-gray-800">
	<p class="text-xs font-semibold tracking-wide text-gray-500 uppercase dark:text-gray-400">
		{providerLabel}
	</p>
	<h2 class="mt-2 text-xl font-semibold text-gray-950 dark:text-gray-50">
		{block.title || "Social post"}
	</h2>
	<a
		href={block.url}
		target="_blank"
		rel="noreferrer noopener"
		class="mt-4 inline-flex font-semibold text-blue-700 underline underline-offset-4 dark:text-blue-300"
	>
		View post
	</a>
</section>
