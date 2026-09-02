<script lang="ts">
	/* eslint-disable svelte/no-navigation-without-resolve */
	import { cmsRenderHref, cmsRenderRel, cmsRenderTarget } from "./links";
	import type { CmsBlock } from "./types";
	import type { components } from "$lib/api/generated/openapi";

	interface Props {
		block: CmsBlock<"promotion_highlight">;
		campaign?: components["schemas"]["ActiveDiscountCampaign"] | null;
	}

	let { block, campaign = undefined }: Props = $props();
	const visible = $derived(!block.campaign_id || Boolean(campaign));
	const title = $derived(campaign?.name ?? block.title);
	const badge = $derived(campaign?.coupon_code ?? block.badge ?? block.promotion_code);
	const discountSummary = $derived(
		campaign?.discount_value == null
			? ""
			: campaign.discount_mode === "percent"
				? `${campaign.discount_value}% off`
				: campaign.discount_mode === "fixed"
					? `${campaign.discount_value} off`
					: ""
	);
</script>

{#if visible}
	<section
		class="mb-10 rounded-lg border border-gray-200 bg-emerald-50 px-6 py-7 dark:border-gray-800 dark:bg-emerald-950/30"
	>
		{#if badge}
			<p
				class="mb-3 text-xs font-semibold tracking-wide text-emerald-700 uppercase dark:text-emerald-300"
			>
				{badge}
			</p>
		{/if}
		<h2 class="text-2xl font-semibold text-gray-950 dark:text-gray-50">{title}</h2>
		{#if discountSummary}<p class="mt-2 font-semibold text-emerald-800 dark:text-emerald-200">
				{discountSummary}
			</p>{/if}
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
{/if}
