<script lang="ts">
	import ButtonLink from "$lib/components/ButtonLink.svelte";
	import { cmsRenderHref, cmsRenderRel, cmsRenderTarget } from "./links";
	import type { CmsBlock } from "./types";

	interface Props {
		block: CmsBlock<"cta">;
	}

	let { block }: Props = $props();
</script>

<section class="mb-10 rounded-lg border border-gray-200 px-6 py-7 dark:border-gray-800">
	{#if block.body}
		<p class="mb-5 max-w-2xl leading-7 text-gray-600 dark:text-gray-300">{block.body}</p>
	{/if}
	<ButtonLink
		href={cmsRenderHref(block.url)}
		target={cmsRenderTarget(block.url)}
		rel={cmsRenderRel(block.url)}
	>
		{block.label}
	</ButtonLink>
</section>
