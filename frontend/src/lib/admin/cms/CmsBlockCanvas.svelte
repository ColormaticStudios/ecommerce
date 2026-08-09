<script lang="ts">
	import { cmsMediaURL } from "$lib/cms";
	import CmsBlockProblemCard from "./CmsBlockProblemCard.svelte";
	import { cmsBlockLabel } from "./blockCatalog";
	import type { CmsEditableBlock } from "./blocks";

	type EditableBlock = CmsEditableBlock;

	interface Props {
		block: CmsEditableBlock;
		blockNumber: number;
		selected?: boolean;
		localImagePreviews?: Record<string, string>;
		inventoryProductName?: string;
		focusEditableText: (block: CmsEditableBlock, event: PointerEvent) => void;
		commitInlineText: (block: CmsEditableBlock, key: string, event: FocusEvent) => void;
	}
	let {
		block,
		blockNumber,
		selected = false,
		localImagePreviews = {},
		inventoryProductName,
		focusEditableText,
		commitInlineText,
	}: Props = $props();

	function blockLabel(value: CmsEditableBlock): string {
		return cmsBlockLabel(value.type);
	}
	function imagePreview(value: Extract<CmsEditableBlock, { type: "image" }>): string {
		return localImagePreviews[value.editorId] || cmsMediaURL(value.media_id);
	}
	function categoryImagePreview(
		value: Extract<CmsEditableBlock, { type: "category_tiles" }>,
		slug: string
	): string {
		return (
			localImagePreviews[`${value.editorId}:${slug}`] ||
			cmsMediaURL(value.category_media_ids?.[slug] ?? "")
		);
	}
</script>

{#snippet editableText(block: EditableBlock, key: string, value: string, className: string)}
	<span
		contenteditable={selected ? "true" : "false"}
		role="textbox"
		aria-readonly={selected ? "false" : "true"}
		tabindex={selected ? 0 : undefined}
		data-empty={value.length === 0 ? "true" : undefined}
		data-placeholder="Click to edit"
		class={`${className} cms-inline-text whitespace-pre-wrap`}
		onpointerdown={(event) => focusEditableText(block, event)}
		onkeydown={(event) => event.stopPropagation()}
		onblur={(event) => commitInlineText(block, key, event)}>{value}</span
	>
{/snippet}

{#snippet blockCanvas(block: EditableBlock)}
	{#if block.editorProblem}
		<CmsBlockProblemCard problem={block.editorProblem} {blockNumber} />
	{:else if block.type === "hero"}
		<div class="overflow-hidden rounded-md bg-stone-100 dark:bg-stone-800">
			{#if block.image_media_id}
				<img src={cmsMediaURL(block.image_media_id)} alt="" class="h-56 w-full object-cover" />
			{/if}
			<div class="p-6">
				<h1 class="max-w-3xl text-3xl font-semibold">
					{@render editableText(block, "title", block.title, "outline-none")}
				</h1>
				<p class="mt-3 max-w-2xl leading-7 text-stone-600 dark:text-stone-300">
					{@render editableText(block, "subtitle", block.subtitle ?? "", "outline-none")}
				</p>
			</div>
		</div>
	{:else if block.type === "rich_text"}
		<p class="max-w-3xl rounded-md px-3 py-4 leading-8 text-stone-700 dark:text-stone-200">
			{@render editableText(block, "body", block.body, "outline-none")}
		</p>
	{:else if block.type === "image"}
		<figure class="rounded-md">
			{#if imagePreview(block)}
				<img
					src={imagePreview(block)}
					alt={block.alt ?? ""}
					class="max-h-96 w-full rounded-md object-cover"
				/>
			{:else}
				<div
					class="flex h-56 items-center justify-center rounded-md bg-stone-100 text-sm text-stone-500 dark:bg-stone-800"
				>
					Image URL not set
				</div>
			{/if}
			{#if block.caption}
				<figcaption class="mt-2 text-sm text-stone-500">{block.caption}</figcaption>
			{/if}
		</figure>
	{:else if block.type === "cta"}
		<div class="rounded-md border border-stone-200 p-5 dark:border-stone-800">
			<p class="mb-4 text-stone-600 dark:text-stone-300">
				{@render editableText(block, "body", block.body ?? "", "outline-none")}
			</p>
			<span class="inline-flex rounded-lg bg-stone-900 px-4 py-2 text-sm font-medium text-white">
				{@render editableText(block, "label", block.label, "outline-none")}
			</span>
		</div>
	{:else if block.type === "promo_banner" || block.type === "promotion_highlight"}
		<div class="rounded-md bg-stone-950 p-6 text-white">
			<h2 class="text-2xl font-semibold">
				{@render editableText(block, "title", block.title, "outline-none")}
			</h2>
			<p class="mt-2 text-stone-200">
				{@render editableText(block, "body", block.body ?? "", "outline-none")}
			</p>
		</div>
	{:else if block.type === "product_rail"}
		<div>
			<h2 class="text-2xl font-semibold">
				{@render editableText(block, "title", block.title, "outline-none")}
			</h2>
			<p class="mt-1 text-sm text-stone-500">
				{@render editableText(block, "subtitle", block.subtitle ?? "", "outline-none")}
			</p>
			<div class="mt-4 grid grid-cols-2 gap-3 md:grid-cols-4">
				{#each Array.from({ length: Math.min(block.limit || 4, 4) }, (_, productIndex) => productIndex) as productIndex (productIndex)}
					<div
						class="aspect-square rounded-md bg-stone-100 p-3 text-xs text-stone-500 dark:bg-stone-800"
					>
						Product {productIndex + 1}
					</div>
				{/each}
			</div>
		</div>
	{:else if block.type === "category_tiles"}
		<div>
			<h2 class="text-2xl font-semibold">
				{@render editableText(block, "title", block.title, "outline-none")}
			</h2>
			<div class="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
				{#each block.category_slugs.length ? block.category_slugs : ["category"] as slug (slug)}
					<div class="overflow-hidden rounded-md border border-stone-200 dark:border-stone-800">
						{#if categoryImagePreview(block, slug)}
							<img
								src={categoryImagePreview(block, slug)}
								alt=""
								class="aspect-video w-full object-cover"
							/>
						{/if}
						<p class="p-4 font-medium">{slug}</p>
					</div>
				{/each}
			</div>
		</div>
	{:else if block.type === "inventory_message"}
		<div class="rounded-md border border-stone-200 p-4 text-sm dark:border-stone-800">
			<p class="font-semibold">{inventoryProductName ?? "Select a product"}</p>
			<p class="mt-1 text-stone-600 dark:text-stone-300">
				{block.low_stock_message || "Almost gone"}
			</p>
		</div>
	{:else if block.type === "testimonial"}
		<blockquote class="rounded-md border border-stone-200 p-6 dark:border-stone-800">
			<p class="text-xl leading-8">
				{@render editableText(block, "quote", block.quote, "outline-none")}
			</p>
			<footer class="mt-4 text-sm font-medium text-stone-600 dark:text-stone-300">
				{@render editableText(block, "attribution", block.attribution, "outline-none")}
			</footer>
		</blockquote>
	{:else if block.type === "social_embed"}
		<div class="rounded-md border border-stone-200 p-6 dark:border-stone-800">
			<p class="text-sm font-semibold">{block.provider}</p>
			<p class="mt-2 text-lg">{block.title || "Social post"}</p>
		</div>
	{:else}
		<div
			class="rounded-md border border-dashed border-stone-300 p-6 text-sm text-stone-500 dark:border-stone-700"
		>
			{blockLabel(block)}
		</div>
	{/if}
{/snippet}

{@render blockCanvas(block)}

<style>
	.cms-inline-text {
		display: inline-block;
		min-width: 1ch;
		min-height: 1.5em;
	}
	.cms-inline-text[data-empty="true"]::before {
		content: attr(data-placeholder);
		color: rgb(120 113 108);
		font-style: italic;
	}
	.cms-inline-text[data-empty="true"]:focus::before {
		content: "";
	}
</style>
