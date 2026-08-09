<script lang="ts">
	/* eslint-disable svelte/no-navigation-without-resolve */
	import { cmsRenderHref, cmsRenderRel, cmsRenderTarget } from "./links";
	import type { CmsBlock } from "./types";

	interface Props {
		block: CmsBlock<"promo_banner">;
	}

	let { block }: Props = $props();
</script>

<section
	class="mb-10 rounded-lg bg-gray-950 px-6 py-7 text-white dark:bg-gray-100 dark:text-gray-950"
>
	<h2 class="text-2xl font-semibold">{block.title}</h2>
	{#if block.body}
		<p class="mt-2 max-w-2xl text-gray-200 dark:text-gray-700">{block.body}</p>
	{/if}
	{#if block.link}
		<a
			href={cmsRenderHref(block.link.url)}
			target={cmsRenderTarget(block.link.url)}
			rel={cmsRenderRel(block.link.url)}
			class="mt-5 inline-flex font-semibold underline decoration-white/40 underline-offset-4 dark:decoration-gray-600"
		>
			{block.link.label}
		</a>
	{/if}
</section>
