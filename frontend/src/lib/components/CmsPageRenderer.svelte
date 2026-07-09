<script lang="ts">
	/* eslint-disable svelte/no-at-html-tags, svelte/no-navigation-without-resolve */
	import { resolve } from "$app/paths";
	import {
		cmsHref,
		cmsMediaURL,
		isExternalHref,
		type CmsContentBlock,
		type CmsPageModel,
	} from "$lib/cms";
	import type { CategoryModel, ProductModel } from "$lib/models";
	import ButtonLink from "$lib/components/ButtonLink.svelte";
	import Card from "$lib/components/Card.svelte";
	import ProductCard from "$lib/components/ProductCard.svelte";

	interface Props {
		page: CmsPageModel;
		productRails?: Record<string, ProductModel[]>;
		categoryTiles?: Record<string, CategoryModel[]>;
		inventoryProducts?: Record<string, ProductModel | null>;
	}

	let { page, productRails = {}, categoryTiles = {}, inventoryProducts = {} }: Props = $props();

	function hrefFor(url: string): string {
		const href = cmsHref(url);
		if (isExternalHref(href)) {
			return href;
		}
		const [pathWithQuery, hash = ""] = href.split("#", 2);
		const [pathname, search = ""] = pathWithQuery.split("?", 2);
		let resolved = resolve(pathname as "/");
		if (search) {
			resolved += `?${search}`;
		}
		if (hash) {
			resolved += `#${hash}`;
		}
		return resolved;
	}

	function targetFor(url: string): string | undefined {
		return isExternalHref(cmsHref(url)) ? "_blank" : undefined;
	}

	function relFor(url: string): string | undefined {
		return isExternalHref(cmsHref(url)) ? "noreferrer noopener" : undefined;
	}

	function text(value: string | undefined): string {
		return value ?? "";
	}

	function blockKey(block: CmsContentBlock, index: number): string {
		return `${block.type}-${index}`;
	}

	function productRailKey(index: number): string {
		return `product_rail:${index}`;
	}

	function categoryTilesKey(index: number): string {
		return `category_tiles:${index}`;
	}

	function inventoryMessageKey(index: number): string {
		return `inventory_message:${index}`;
	}

	function inventoryMessage(
		block: Extract<CmsContentBlock, { type: "inventory_message" }>,
		product: ProductModel | null | undefined
	): string {
		if (!product) return "";
		if (product.stock <= 0) return block.out_of_stock_message || "Out of stock";
		if (product.stock <= (block.low_stock_threshold ?? 5)) {
			return block.low_stock_message || "Low stock";
		}
		return block.in_stock_message || "In stock";
	}

	function providerLabel(provider: string): string {
		return provider.charAt(0).toUpperCase() + provider.slice(1);
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
			{#if block.type === "hero"}
				<section class="mb-10 overflow-hidden rounded-lg bg-gray-100 dark:bg-gray-800">
					{#if block.image_media_id}
						<img
							src={cmsMediaURL(block.image_media_id)}
							alt=""
							class="h-72 w-full object-cover sm:h-96"
						/>
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
									href={hrefFor(block.primary_cta.url)}
									target={targetFor(block.primary_cta.url)}
									rel={relFor(block.primary_cta.url)}
								>
									{block.primary_cta.label}
								</ButtonLink>
							</div>
						{/if}
					</div>
				</section>
			{:else if block.type === "rich_text"}
				<section class="cms-prose mb-8">
					<p>{block.body}</p>
				</section>
			{:else if block.type === "image"}
				<figure class="mb-10">
					<img
						src={cmsMediaURL(block.media_id)}
						alt={text(block.alt)}
						class="w-full rounded-lg object-cover"
					/>
					{#if block.caption}
						<figcaption class="mt-2 text-sm text-gray-500 dark:text-gray-400">
							{block.caption}
						</figcaption>
					{/if}
				</figure>
			{:else if block.type === "gallery"}
				<section class="mb-10 grid gap-4 sm:grid-cols-2">
					{#each block.images as image, imageIndex (`${blockKey(block, index)}-${imageIndex}`)}
						<figure>
							<img
								src={cmsMediaURL(image.media_id)}
								alt={text(image.alt)}
								class="aspect-4/3 w-full rounded-lg object-cover"
							/>
							{#if image.caption}
								<figcaption class="mt-2 text-sm text-gray-500 dark:text-gray-400">
									{image.caption}
								</figcaption>
							{/if}
						</figure>
					{/each}
				</section>
			{:else if block.type === "video"}
				<section class="mb-10">
					<div class="overflow-hidden rounded-lg border border-gray-200 dark:border-gray-800">
						<iframe
							src={block.url}
							title={text(block.title) || "CMS video"}
							class="aspect-video w-full"
							allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
							allowfullscreen
						></iframe>
					</div>
				</section>
			{:else if block.type === "faq"}
				<section
					class="mb-10 divide-y divide-gray-200 rounded-lg border border-gray-200 dark:divide-gray-800 dark:border-gray-800"
				>
					{#each block.items as item, itemIndex (`${blockKey(block, index)}-${itemIndex}`)}
						<details class="group px-5 py-4">
							<summary
								class="flex cursor-pointer list-none items-center justify-between gap-4 font-medium text-gray-950 dark:text-gray-50"
							>
								{item.question}
								<i class="bi bi-chevron-down text-sm transition group-open:rotate-180"></i>
							</summary>
							<p class="mt-3 leading-7 text-gray-600 dark:text-gray-300">{item.answer}</p>
						</details>
					{/each}
				</section>
			{:else if block.type === "cta"}
				<section class="mb-10 rounded-lg border border-gray-200 px-6 py-7 dark:border-gray-800">
					{#if block.body}
						<p class="mb-5 max-w-2xl leading-7 text-gray-600 dark:text-gray-300">{block.body}</p>
					{/if}
					<ButtonLink
						href={hrefFor(block.url)}
						target={targetFor(block.url)}
						rel={relFor(block.url)}
					>
						{block.label}
					</ButtonLink>
				</section>
			{:else if block.type === "promo_banner"}
				<section
					class="mb-10 rounded-lg bg-gray-950 px-6 py-7 text-white dark:bg-gray-100 dark:text-gray-950"
				>
					<h2 class="text-2xl font-semibold">{block.title}</h2>
					{#if block.body}
						<p class="mt-2 max-w-2xl text-gray-200 dark:text-gray-700">{block.body}</p>
					{/if}
					{#if block.link}
						<a
							href={hrefFor(block.link.url)}
							target={targetFor(block.link.url)}
							rel={relFor(block.link.url)}
							class="mt-5 inline-flex font-semibold underline decoration-white/40 underline-offset-4 dark:decoration-gray-600"
						>
							{block.link.label}
						</a>
					{/if}
				</section>
			{:else if block.type === "product_rail"}
				{@const products = productRails[productRailKey(index)] ?? []}
				<section class="mb-10">
					<div class="mb-5">
						<h2 class="text-2xl font-semibold text-gray-950 dark:text-gray-50">{block.title}</h2>
						{#if block.subtitle}
							<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{block.subtitle}</p>
						{/if}
					</div>
					{#if products.length === 0}
						<Card
							border="dashed"
							radius="xl"
							padding="sm"
							class="text-sm text-gray-500 dark:text-gray-400"
						>
							No products found for this section.
						</Card>
					{:else}
						<div class="grid grid-cols-1 gap-6 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
							{#each products as product (product.id)}
								<ProductCard
									href={resolve(`/product/${product.id}`)}
									imageAspect={block.image_aspect ?? "square"}
									data={{
										name: product.name,
										brand: product.brand?.name,
										description: product.description,
										price: product.price,
										basePrice: product.base_price,
										discountAmount: product.discount_amount,
										finalPrice: product.final_price,
										priceRange: product.price_range,
										image: product.images?.[0],
										stock: product.stock,
									}}
								/>
							{/each}
						</div>
					{/if}
				</section>
			{:else if block.type === "category_tiles"}
				{@const categories = categoryTiles[categoryTilesKey(index)] ?? []}
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
										<p
											class="text-lg font-semibold text-gray-950 group-hover:underline dark:text-gray-50"
										>
											{category.name}
										</p>
										{#if category.description}
											<p
												class="mt-2 line-clamp-2 text-sm leading-6 text-gray-600 dark:text-gray-300"
											>
												{category.description}
											</p>
										{/if}
									</div>
								</a>
							{/each}
						</div>
					{/if}
				</section>
			{:else if block.type === "promotion_highlight"}
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
							href={hrefFor(block.link.url)}
							target={targetFor(block.link.url)}
							rel={relFor(block.link.url)}
							class="mt-5 inline-flex font-semibold text-emerald-800 underline underline-offset-4 dark:text-emerald-200"
						>
							{block.link.label}
						</a>
					{/if}
				</section>
			{:else if block.type === "inventory_message"}
				{@const product = inventoryProducts[inventoryMessageKey(index)]}
				{@const message = inventoryMessage(block, product)}
				{#if message}
					<section
						class="mb-10 rounded-lg border border-gray-200 px-5 py-4 text-sm dark:border-gray-800"
					>
						<p class="font-semibold text-gray-950 dark:text-gray-50">{product?.name}</p>
						<p class="mt-1 text-gray-600 dark:text-gray-300">{message}</p>
					</section>
				{/if}
			{:else if block.type === "testimonial"}
				<section class="mb-10 rounded-lg border border-gray-200 px-6 py-7 dark:border-gray-800">
					{#if block.rating}
						<p class="mb-3 text-sm font-semibold text-amber-600 dark:text-amber-300">
							{"★".repeat(block.rating)}
						</p>
					{/if}
					<blockquote class="text-xl leading-8 text-gray-950 dark:text-gray-50">
						“{block.quote}”
					</blockquote>
					<p class="mt-4 text-sm font-medium text-gray-600 dark:text-gray-300">
						{block.attribution}
					</p>
				</section>
			{:else if block.type === "social_embed"}
				<section class="mb-10 rounded-lg border border-gray-200 px-6 py-7 dark:border-gray-800">
					<p class="text-xs font-semibold tracking-wide text-gray-500 uppercase dark:text-gray-400">
						{providerLabel(block.provider)}
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
			{:else if block.type === "custom_html"}
				<section class="cms-prose mb-8">
					{@html block.html}
				</section>
			{/if}
		{/each}
	{/if}
</article>

<style>
	@reference "tailwindcss";

	.cms-prose {
		@apply max-w-3xl text-base leading-8 text-gray-700 dark:text-gray-200;
	}

	.cms-prose :global(p + p) {
		@apply mt-4;
	}

	.cms-prose :global(a) {
		@apply font-medium text-blue-700 underline underline-offset-4 dark:text-blue-300;
	}
</style>
