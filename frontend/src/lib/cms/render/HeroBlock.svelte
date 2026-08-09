<script lang="ts">
	import { cmsMediaURL } from "$lib/cms";
	import ButtonLink from "$lib/components/ButtonLink.svelte";
	import { cmsRenderHref, cmsRenderRel, cmsRenderTarget } from "./links";
	import type { CmsBlock } from "./types";

	interface Props {
		block: CmsBlock<"hero">;
	}

	let { block }: Props = $props();
</script>

<section class="mb-10 overflow-hidden rounded-lg bg-gray-100 dark:bg-gray-800">
	{#if block.image_media_id}
		<img src={cmsMediaURL(block.image_media_id)} alt="" class="h-72 w-full object-cover sm:h-96" />
	{/if}
	<div class="px-6 py-10 sm:px-8">
		<h1 class="max-w-3xl text-4xl font-semibold text-gray-950 dark:text-gray-50">
			{block.title}
		</h1>
		{#if block.subtitle}
			<p class="mt-4 max-w-2xl text-lg leading-8 text-gray-600 dark:text-gray-300">
				{block.subtitle}
			</p>
		{/if}
		{#if block.primary_cta}
			<div class="mt-7">
				<ButtonLink
					href={cmsRenderHref(block.primary_cta.url)}
					target={cmsRenderTarget(block.primary_cta.url)}
					rel={cmsRenderRel(block.primary_cta.url)}
				>
					{block.primary_cta.label}
				</ButtonLink>
			</div>
		{/if}
	</div>
</section>
