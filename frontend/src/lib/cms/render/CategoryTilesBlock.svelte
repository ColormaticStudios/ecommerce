<script lang="ts">
	import { resolve } from "$app/paths";
	import { cmsMediaURL } from "$lib/cms";
	import Card from "$lib/components/Card.svelte";
	import type { CategoryModel } from "$lib/models";
	import type { CmsBlock } from "./types";

	interface Props {
		block: CmsBlock<"category_tiles">;
		categories: CategoryModel[];
	}

	let { block, categories }: Props = $props();
</script>

<section class="mb-10">
	<div class="mb-5">
		<h2 class="text-2xl font-semibold text-gray-950 dark:text-gray-50">{block.title}</h2>
		{#if block.subtitle}
			<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{block.subtitle}</p>
		{/if}
	</div>
	{#if categories.length === 0}
		<Card border="dashed" radius="xl" padding="sm" class="text-sm text-gray-500">
			No active categories found for this section.
		</Card>
	{:else}
		<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
			{#each categories as category (category.id)}
				<a
					href={resolve(`/search?category_slug=${encodeURIComponent(category.slug)}`)}
					class="group overflow-hidden rounded-lg border border-gray-200 transition hover:border-gray-400 dark:border-gray-800 dark:hover:border-gray-600"
				>
					{#if block.category_media_ids?.[category.slug]}
						<img
							src={cmsMediaURL(block.category_media_ids[category.slug])}
							alt=""
							class={block.image_aspect === "wide"
								? "aspect-video w-full object-cover"
								: "aspect-square w-full object-cover"}
						/>
					{/if}
					<div class="p-5">
						<p class="text-lg font-semibold text-gray-950 group-hover:underline dark:text-gray-50">
							{category.name}
						</p>
						{#if category.description}
							<p class="mt-2 line-clamp-2 text-sm leading-6 text-gray-600 dark:text-gray-300">
								{category.description}
							</p>
						{/if}
					</div>
				</a>
			{/each}
		</div>
	{/if}
</section>
