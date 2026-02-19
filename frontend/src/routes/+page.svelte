<script lang="ts">
	/* eslint-disable svelte/no-navigation-without-resolve */
	import { resolve } from "$app/paths";
	import type { PageData } from "./$types";
	import Alert from "$lib/components/Alert.svelte";
	import ButtonLink from "$lib/components/ButtonLink.svelte";
	import ProductCard from "$lib/components/ProductCard.svelte";

	interface Props {
		data: PageData;
	}

	let { data }: Props = $props();

	function hasScheme(url: string): boolean {
		return /^[a-z][a-z\d+.-]*:/i.test(url);
	}

	function isHttpExternal(url: string): boolean {
		return /^https?:\/\//i.test(url);
	}

	function hrefFor(url: string): string {
		const value = (url || "").trim();
		if (!value) {
			return resolve("/");
		}
		if (hasScheme(value)) {
			return value;
		}

		const normalized = value.startsWith("/") ? value : `/${value}`;
		const [pathWithQuery, hash = ""] = normalized.split("#", 2);
		const [pathname, search = ""] = pathWithQuery.split("?", 2);
		const resolvedPath = resolve(pathname as "/");

		let suffix = "";
		if (search) {
			suffix += `?${search}`;
		}
		if (hash) {
			suffix += `#${hash}`;
		}

		return `${resolvedPath}${suffix}`;
	}

	function linkTarget(url: string): string | undefined {
		return isHttpExternal(url) ? "_blank" : undefined;
	}

	function linkRel(url: string): string | undefined {
		return isHttpExternal(url) ? "noreferrer noopener" : undefined;
	}
</script>

<section class="mx-auto max-w-7xl px-4 py-10">
	{#if data.errorMessage}
		<Alert message={data.errorMessage} tone="error" icon="bi-x-circle-fill" onClose={undefined} />
	{/if}

	{#each data.homepageSections as section (section.id)}
		{#if section.type === "hero" && section.hero}
			<section class="mb-10 overflow-hidden rounded-3xl bg-gray-100 dark:bg-gray-800">
				{#if section.hero.background_image_url}
					<img
						src={section.hero.background_image_url}
						alt=""
						class="h-full w-full object-cover opacity-35"
					/>
				{/if}
				<div class="px-6 py-18 sm:px-10">
					<p class="text-xs font-semibold text-gray-500 dark:text-gray-300">
						{section.hero.eyebrow}
					</p>
					<h1
						class="mt-3 max-w-3xl text-4xl font-semibold text-gray-900 sm:text-5xl dark:text-gray-50"
					>
						{section.hero.title}
					</h1>
					<p class="mt-4 max-w-2xl text-lg text-gray-600 dark:text-gray-300">
						{section.hero.subtitle}
					</p>
					<div class="mt-8 flex flex-wrap items-center gap-3">
						{#if section.hero.primary_cta.label && section.hero.primary_cta.url}
							<ButtonLink
								href={hrefFor(section.hero.primary_cta.url)}
								variant="primary"
								target={linkTarget(section.hero.primary_cta.url)}
								rel={linkRel(section.hero.primary_cta.url)}
								class="text-sm font-semibold"
							>
								{section.hero.primary_cta.label}
							</ButtonLink>
						{/if}
						{#if section.hero.secondary_cta.label && section.hero.secondary_cta.url}
							<ButtonLink
								href={hrefFor(section.hero.secondary_cta.url)}
								variant="regular"
								target={linkTarget(section.hero.secondary_cta.url)}
								rel={linkRel(section.hero.secondary_cta.url)}
								class="text-sm font-semibold text-gray-700 dark:text-gray-100"
							>
								{section.hero.secondary_cta.label}
							</ButtonLink>
						{/if}
					</div>
				</div>
			</section>
		{:else if section.type === "products" && section.product_section}
			<section class="mb-10">
				<div class="mb-5">
					<h2 class="text-2xl font-semibold text-gray-900 dark:text-gray-100">
						{section.product_section.title}
					</h2>
					{#if section.product_section.subtitle}
						<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
							{section.product_section.subtitle}
						</p>
					{/if}
				</div>

				{#if section.products.length === 0}
					<div
						class="rounded-xl border border-dashed border-gray-300 bg-white px-4 py-6 text-sm text-gray-500 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-400"
					>
						No products found for this section.
					</div>
				{:else}
					<div class="grid grid-cols-1 gap-6 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
						{#each section.products as product (product.id)}
							<ProductCard
								href={resolve(`/product/${product.id}`)}
								showStock={section.product_section.show_stock}
								imageAspect={section.product_section.image_aspect}
								data={{
									name: product.name,
									description: section.product_section.show_description
										? product.description
										: undefined,
									price: product.price,
									image: product.images?.[0],
									stock: product.stock,
								}}
							/>
						{/each}
					</div>
				{/if}
			</section>
		{:else if section.type === "promo_cards"}
			<section class="mb-10 grid gap-4 md:grid-cols-3">
				{#each (section.promo_cards ?? []).slice(0, section.promo_card_limit ?? 1) as card, index (index)}
					<article
						class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-gray-800 dark:bg-gray-900"
					>
						{#if card.image_url}
							<img
								src={card.image_url}
								alt=""
								class="pointer-events-none h-full w-full object-cover opacity-18"
							/>
						{/if}
						<div class="relative">
							<p class="text-xs font-semibold text-gray-500 dark:text-gray-400">
								{card.kicker}
							</p>
							<h3 class="mt-2 text-xl font-semibold text-gray-900 dark:text-gray-100">
								{card.title}
							</h3>
							<p class="mt-2 text-sm text-gray-600 dark:text-gray-300">{card.description}</p>
							{#if card.link.label && card.link.url}
								<a
									href={hrefFor(card.link.url)}
									target={linkTarget(card.link.url)}
									rel={linkRel(card.link.url)}
									class="mt-4 inline-flex items-center text-sm font-semibold text-blue-600 hover:text-blue-700"
								>
									{card.link.label}
								</a>
							{/if}
						</div>
					</article>
				{/each}
			</section>
		{:else if section.type === "badges"}
			<section class="mb-10 flex flex-wrap gap-2">
				{#each section.badges ?? [] as badge, index (index)}
					<span
						class="rounded-full border border-gray-200 bg-gray-100 px-3 py-1 text-xs font-semibold text-gray-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
					>
						{badge}
					</span>
				{/each}
			</section>
		{/if}
	{/each}
</section>
