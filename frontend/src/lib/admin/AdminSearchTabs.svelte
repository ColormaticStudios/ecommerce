<script lang="ts">
	import { getContext } from "svelte";
	import { page } from "$app/state";
	import { resolve } from "$app/paths";
	import { LOCALIZATION_CONTEXT, type LocalizationRuntime } from "$lib/localization/runtime";
	const localization = getContext<LocalizationRuntime>(LOCALIZATION_CONTEXT);
	const tabs = $derived([
		{
			slug: "analytics",
			href: "/admin/search/analytics" as const,
			label: $localization.translate("admin.search.tabs.analytics", "Analytics"),
		},
		{
			slug: "synonyms",
			href: "/admin/search/synonyms" as const,
			label: $localization.translate("admin.search.tabs.synonyms", "Synonyms"),
		},
		{
			slug: "typo-profiles",
			href: "/admin/search/typo-profiles" as const,
			label: $localization.translate("admin.search.tabs.typo_profiles", "Typo profiles"),
		},
		{
			slug: "ranking-profiles",
			href: "/admin/search/ranking-profiles" as const,
			label: $localization.translate("admin.search.tabs.ranking_profiles", "Ranking profiles"),
		},
		{
			slug: "merchandising",
			href: "/admin/search/merchandising" as const,
			label: $localization.translate("admin.search.tabs.merchandising", "Merchandising"),
		},
		{
			slug: "operations",
			href: "/admin/search/operations" as const,
			label: $localization.translate("admin.search.tabs.operations", "Operations"),
		},
		{
			slug: "history",
			href: "/admin/search/history" as const,
			label: $localization.translate("admin.search.tabs.history", "History"),
		},
	]);
</script>

<nav
	aria-label={$localization.translate("admin.search.tabs.label", "{search} sections", {
		search: $localization.translate("admin.navigation.search", "Search"),
	})}
	class="mb-6 flex flex-wrap gap-2 border-b border-stone-200 pb-3 dark:border-stone-800"
>
	{#each tabs as tab (tab.slug)}
		{@const current = page.url.pathname.startsWith(`/admin/search/${tab.slug}`)}
		<a
			href={resolve(tab.href)}
			aria-current={current ? "page" : undefined}
			class={`rounded-lg px-3 py-2 text-sm font-medium transition ${current ? "bg-stone-900 text-white dark:bg-stone-100 dark:text-stone-900" : "text-stone-600 hover:bg-stone-100 dark:text-stone-300 dark:hover:bg-stone-900"}`}
		>
			{tab.label}
		</a>
	{/each}
</nav>
