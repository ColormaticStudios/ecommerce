<script lang="ts">
	import type { CmsContentBlock } from "$lib/cms";
	import type { CategoryModel, ProductModel } from "$lib/models";
	import CategoryTilesBlock from "./CategoryTilesBlock.svelte";
	import CtaBlock from "./CtaBlock.svelte";
	import CustomHtmlBlock from "./CustomHtmlBlock.svelte";
	import FaqBlock from "./FaqBlock.svelte";
	import FooterBlock from "./FooterBlock.svelte";
	import GalleryBlock from "./GalleryBlock.svelte";
	import HeroBlock from "./HeroBlock.svelte";
	import ImageBlock from "./ImageBlock.svelte";
	import InventoryMessageBlock from "./InventoryMessageBlock.svelte";
	import { categoryTilesKey, inventoryMessageKey, productRailKey } from "./keys";
	import ProductRailBlock from "./ProductRailBlock.svelte";
	import PromoBannerBlock from "./PromoBannerBlock.svelte";
	import PromotionHighlightBlock from "./PromotionHighlightBlock.svelte";
	import RichTextBlock from "./RichTextBlock.svelte";
	import SocialEmbedBlock from "./SocialEmbedBlock.svelte";
	import TestimonialBlock from "./TestimonialBlock.svelte";
	import VideoBlock from "./VideoBlock.svelte";

	interface Props {
		block: CmsContentBlock;
		index: number;
		productRails?: Record<string, ProductModel[]>;
		categoryTiles?: Record<string, CategoryModel[]>;
		inventoryProducts?: Record<string, ProductModel | null>;
	}

	let {
		block,
		index,
		productRails = {},
		categoryTiles = {},
		inventoryProducts = {},
	}: Props = $props();

	const blockKey = $derived(`${block.type}-${index}`);

	function assertNever(value: never): never {
		throw new Error(`Unhandled validated CMS block: ${JSON.stringify(value)}`);
	}
</script>

{#if block.type === "hero"}
	<HeroBlock {block} />
{:else if block.type === "rich_text"}
	<RichTextBlock {block} />
{:else if block.type === "image"}
	<ImageBlock {block} />
{:else if block.type === "gallery"}
	<GalleryBlock {block} {blockKey} />
{:else if block.type === "video"}
	<VideoBlock {block} />
{:else if block.type === "faq"}
	<FaqBlock {block} {blockKey} />
{:else if block.type === "cta"}
	<CtaBlock {block} />
{:else if block.type === "promo_banner"}
	<PromoBannerBlock {block} />
{:else if block.type === "product_rail"}
	<ProductRailBlock {block} products={productRails[productRailKey(index)] ?? []} />
{:else if block.type === "category_tiles"}
	<CategoryTilesBlock {block} categories={categoryTiles[categoryTilesKey(index)] ?? []} />
{:else if block.type === "promotion_highlight"}
	<PromotionHighlightBlock {block} />
{:else if block.type === "inventory_message"}
	<InventoryMessageBlock {block} product={inventoryProducts[inventoryMessageKey(index)]} />
{:else if block.type === "testimonial"}
	<TestimonialBlock {block} />
{:else if block.type === "social_embed"}
	<SocialEmbedBlock {block} />
{:else if block.type === "footer"}
	<FooterBlock {block} />
{:else if block.type === "custom_html"}
	<CustomHtmlBlock {block} />
{:else}
	{assertNever(block)}
{/if}
