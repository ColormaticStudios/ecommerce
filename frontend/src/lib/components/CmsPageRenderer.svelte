<script lang="ts">
	import type { CmsContentBlock, CmsPageModel } from "$lib/cms";
	import CmsBlockRenderer from "$lib/cms/render/CmsBlockRenderer.svelte";
	import type { CategoryModel, ProductModel } from "$lib/models";

	interface Props {
		page: CmsPageModel;
		productRails?: Record<string, ProductModel[]>;
		categoryTiles?: Record<string, CategoryModel[]>;
		inventoryProducts?: Record<string, ProductModel | null>;
	}

	let { page, productRails = {}, categoryTiles = {}, inventoryProducts = {} }: Props = $props();

	function blockKey(block: CmsContentBlock, index: number): string {
		return `${block.type}-${index}`;
	}
</script>

<article class="mx-auto w-full max-w-5xl px-4 py-10 sm:py-12">
	{#if page.blocks.length === 0}
		<header class="py-14">
			<h1 class="max-w-3xl text-4xl font-semibold text-gray-950 dark:text-gray-50">
				{page.title}
			</h1>
		</header>
	{:else}
		{#each page.blocks as block, index (blockKey(block, index))}
			<CmsBlockRenderer {block} {index} {productRails} {categoryTiles} {inventoryProducts} />
		{/each}
	{/if}
</article>
