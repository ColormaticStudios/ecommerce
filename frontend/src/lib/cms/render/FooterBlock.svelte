<script lang="ts">
	/* eslint-disable svelte/no-navigation-without-resolve */
	import { cmsRenderHref, cmsRenderRel, cmsRenderTarget } from "./links";
	import type { CmsBlock } from "./types";

	interface Props {
		block: CmsBlock<"footer">;
	}

	let { block }: Props = $props();
</script>

<section class="mb-10 border-t border-gray-200 py-10 text-sm dark:border-gray-800">
	<div
		class={block.layout === "centered"
			? "text-center"
			: block.layout === "minimal"
				? "flex flex-wrap items-center justify-between gap-5"
				: "grid gap-10 md:grid-cols-[minmax(14rem,1.1fr)_2fr]"}
	>
		<div>
			<a href={cmsRenderHref("/")} class="text-lg font-semibold">{block.brand_name}</a>
			{#if block.tagline}
				<p class={`mt-3 max-w-sm opacity-70 ${block.layout === "centered" ? "mx-auto" : ""}`}>
					{block.tagline}
				</p>
			{/if}
		</div>
		{#if block.layout !== "minimal"}
			<div
				class={`grid gap-7 ${block.layout === "centered" ? "mt-7 sm:grid-cols-3" : "sm:grid-cols-2 lg:grid-cols-3"}`}
			>
				{#each block.columns as column, columnIndex (columnIndex)}
					<div>
						<p class="font-semibold">{column.title}</p>
						<ul class="mt-3 space-y-2 opacity-70">
							{#each column.links as link, linkIndex (linkIndex)}
								<li>
									<a
										href={cmsRenderHref(link.url)}
										target={cmsRenderTarget(link.url)}
										rel={cmsRenderRel(link.url)}
										class="hover:underline"
									>
										{link.label}
									</a>
								</li>
							{/each}
						</ul>
					</div>
				{/each}
			</div>
		{/if}
	</div>
	<div
		class="mt-9 flex flex-wrap items-center justify-between gap-4 border-t border-current/15 pt-5 text-xs opacity-70"
	>
		<span>{block.copyright}</span>
		{#if block.social_links.length}
			<div class="flex flex-wrap gap-4">
				{#each block.social_links as link, linkIndex (linkIndex)}
					<a
						class="hover:underline"
						href={cmsRenderHref(link.url)}
						target={cmsRenderTarget(link.url)}
						rel={cmsRenderRel(link.url)}
					>
						{link.label}
					</a>
				{/each}
			</div>
		{/if}
	</div>
</section>
