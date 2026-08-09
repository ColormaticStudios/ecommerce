<script lang="ts">
	/* eslint-disable svelte/no-navigation-without-resolve */
	import { cmsRenderHref, cmsRenderRel, cmsRenderTarget } from "./links";
	import type { CmsBlock } from "./types";

	interface Props {
		block: CmsBlock<"promotion_highlight">;
	}

	let { block }: Props = $props();
</script>

<section
	class="mb-10 rounded-lg border border-gray-200 bg-emerald-50 px-6 py-7 dark:border-gray-800 dark:bg-emerald-950/30"
>
	{#if block.badge || block.promotion_code}
		<p
			class="mb-3 text-xs font-semibold tracking-wide text-emerald-700 uppercase dark:text-emerald-300"
		>
			{block.badge || block.promotion_code}
		</p>
	{/if}
	<h2 class="text-2xl font-semibold text-gray-950 dark:text-gray-50">{block.title}</h2>
	{#if block.body}
		<p class="mt-2 max-w-2xl leading-7 text-gray-700 dark:text-gray-200">{block.body}</p>
	{/if}
	{#if block.link}
		<a
			href={cmsRenderHref(block.link.url)}
			target={cmsRenderTarget(block.link.url)}
			rel={cmsRenderRel(block.link.url)}
			class="mt-5 inline-flex font-semibold text-emerald-800 underline underline-offset-4 dark:text-emerald-200"
		>
			{block.link.label}
		</a>
	{/if}
</section>
