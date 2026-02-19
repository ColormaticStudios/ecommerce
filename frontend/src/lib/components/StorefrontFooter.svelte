<script lang="ts">
	/* eslint-disable svelte/no-navigation-without-resolve */
	import { resolve } from "$app/paths";
	import type { StorefrontFooterModel } from "$lib/storefront";

	interface Props {
		footer: StorefrontFooterModel;
	}

	let { footer }: Props = $props();

	function hasScheme(url: string): boolean {
		return /^[a-z][a-z\d+.-]*:/i.test(url);
	}

	function isHttpExternal(url: string): boolean {
		return /^https?:\/\//i.test(url);
	}

	function hrefFor(url: string): string {
		const value = (url || "").trim();
		if (!value) {
			return "#";
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

	function linkRel(url: string): string | undefined {
		return isHttpExternal(url) ? "noreferrer noopener" : undefined;
	}

	function linkTarget(url: string): string | undefined {
		return isHttpExternal(url) ? "_blank" : undefined;
	}
</script>

<footer class="mt-14 border-t border-gray-200 bg-gray-50 dark:border-gray-800 dark:bg-gray-950">
	<div class="mx-auto max-w-7xl px-4 py-10">
		<div class="grid gap-8 md:grid-cols-[1.6fr_2fr]">
			<div>
				<p class="text-xl font-semibold text-gray-900 dark:text-gray-100">{footer.brand_name}</p>
				<p class="mt-2 max-w-xl text-sm text-gray-600 dark:text-gray-300">{footer.tagline}</p>
			</div>

			<div class="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
				{#each footer.columns as column, index (index)}
					<div>
						<p class="text-xs font-semibold text-gray-500 dark:text-gray-400">
							{column.title}
						</p>
						<div class="mt-3 space-y-2">
							{#each column.links as link, linkIndex (linkIndex)}
								<a
									href={hrefFor(link.url)}
									target={linkTarget(link.url)}
									rel={linkRel(link.url)}
									class="block text-sm text-gray-700 transition hover:text-gray-900 dark:text-gray-300 dark:hover:text-gray-100"
								>
									{link.label}
								</a>
							{/each}
						</div>
					</div>
				{/each}
			</div>
		</div>

		<div
			class="mt-10 flex flex-wrap items-center justify-between gap-4 border-t border-gray-200 pt-4 text-xs text-gray-500 dark:border-gray-800 dark:text-gray-400"
		>
			<div class="flex flex-wrap items-center gap-3">
				<span>{footer.copyright}</span>
				<span>{footer.bottom_notice}</span>
			</div>
			{#if footer.social_links.length > 0}
				<div class="flex flex-wrap items-center gap-3">
					{#each footer.social_links as link, index (index)}
						<a
							href={hrefFor(link.url)}
							target={linkTarget(link.url)}
							rel={linkRel(link.url)}
							class="text-xs font-semibold tracking-[0.16em] uppercase transition hover:text-gray-700 dark:hover:text-gray-200"
						>
							{link.label}
						</a>
					{/each}
				</div>
			{/if}
		</div>
	</div>
</footer>
