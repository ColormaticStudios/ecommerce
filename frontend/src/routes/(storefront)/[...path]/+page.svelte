<script lang="ts">
	/* eslint-disable svelte/no-at-html-tags */
	import CmsPageRenderer from "$lib/components/CmsPageRenderer.svelte";
	import { cmsMediaURL } from "$lib/cms";
	import type { PageData } from "./$types";

	interface Props {
		data: PageData;
	}

	let { data }: Props = $props();
	const robots = $derived(
		data.draftPreviewActive
			? "noindex, nofollow"
			: data.page.seo?.robots.replace("_", ", ").replace("_", ", ")
	);
	const jsonLD = $derived(
		(data.page.seo?.json_ld ?? [])
			.map(
				(item) =>
					`<script type="application/ld+json">${JSON.stringify(item).replaceAll("<", "\\u003c")}</` +
					"script>"
			)
			.join("")
	);
</script>

<svelte:head>
	<title>{data.page.seo?.title || data.page.title}</title>
	{#if data.page.seo?.description}<meta
			name="description"
			content={data.page.seo.description}
		/>{/if}
	{#if data.page.seo?.canonical_url}<link rel="canonical" href={data.page.seo.canonical_url} />{/if}
	{#each data.page.localization?.alternates ?? [] as alternate (`${alternate.locale}:${alternate.market ?? ""}`)}
		<link rel="alternate" hreflang={alternate.locale} href={alternate.path} />
	{/each}
	{#if robots}<meta name="robots" content={robots} />{/if}
	{#if data.page.seo?.og_title}<meta property="og:title" content={data.page.seo.og_title} />{/if}
	{#if data.page.seo?.og_description}<meta
			property="og:description"
			content={data.page.seo.og_description}
		/>{/if}
	{#if data.page.seo?.og_image_media_id}<meta
			property="og:image"
			content={cmsMediaURL(data.page.seo.og_image_media_id)}
		/>{/if}
	{#if data.page.seo?.twitter_card}<meta
			name="twitter:card"
			content={data.page.seo.twitter_card}
		/>{/if}
	{#if data.page.seo?.twitter_title}<meta
			name="twitter:title"
			content={data.page.seo.twitter_title}
		/>{/if}
	{#if data.page.seo?.twitter_description}<meta
			name="twitter:description"
			content={data.page.seo.twitter_description}
		/>{/if}
	{#if data.page.seo?.twitter_image_media_id}<meta
			name="twitter:image"
			content={cmsMediaURL(data.page.seo.twitter_image_media_id)}
		/>{/if}
	{@html jsonLD}
</svelte:head>

<CmsPageRenderer
	page={data.page}
	productRails={data.productRails}
	categoryTiles={data.categoryTiles}
	inventoryProducts={data.inventoryProducts}
/>
