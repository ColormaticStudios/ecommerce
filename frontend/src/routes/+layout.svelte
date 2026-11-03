<script lang="ts">
	import "./main.css";
	import "bootstrap-icons/font/bootstrap-icons.css";
	import { API, STOREFRONT_SYNC_EVENT, STOREFRONT_SYNC_STORAGE_KEY } from "$lib/api";
	import { userStore } from "$lib/user";
	import { onMount, setContext, untrack } from "svelte";
	import { LOCALIZATION_CONTEXT, LocalizationRuntime } from "$lib/localization/runtime";
	import { localizationDomainForPath } from "$lib/localization/domain";
	import { page } from "$app/state";
	import { afterNavigate, beforeNavigate, goto } from "$app/navigation";
	import type { LayoutData } from "./$types";

	interface Props {
		data: LayoutData;
		children?: import("svelte").Snippet;
	}

	let { data, children }: Props = $props();

	const api = new API();
	setContext("api", api);
	const initialLocalizationDomain = untrack(() => localizationDomainForPath(page.url.pathname));
	const localization = new LocalizationRuntime(
		{
			...untrack(() => data.localization ?? { locale: "en-US", locales: [], bundle: null }),
			domain: initialLocalizationDomain,
		},
		(locale, domain) => api.getLocalizationBundle(locale, undefined, domain)
	);
	setContext(LOCALIZATION_CONTEXT, localization);
	beforeNavigate((navigation) => {
		const from = navigation.from?.url;
		const target = navigation.to?.url;
		if (
			!from ||
			!target ||
			target.pathname.startsWith("/admin") ||
			target.pathname.startsWith("/api/") ||
			navigation.willUnload ||
			from.origin !== target.origin ||
			!from.searchParams.has("locale") ||
			target.searchParams.has("locale")
		)
			return;
		const canonical = new URL(target);
		canonical.searchParams.set("locale", from.searchParams.get("locale") ?? "");
		if (!canonical.searchParams.has("market"))
			canonical.searchParams.set("market", from.searchParams.get("market") ?? "");
		navigation.cancel();
		// SvelteKit navigation URLs already contain the configured base path.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		void goto(canonical, { replaceState: navigation.type === "popstate" });
	});
	afterNavigate((navigation) => {
		if (navigation.to?.url) {
			const domain = localizationDomainForPath(navigation.to.url.pathname);
			const selected = navigation.to.url.searchParams.get("locale");
			if (
				selected &&
				localization.locales.some((locale) => locale.code === selected && locale.is_enabled)
			)
				void localization.setLocale(selected, false, domain);
			else void localization.setDomain(domain);
		}
	});

	onMount(() => {
		const refreshLocalization = () => {
			void localization.refresh();
		};
		const handleStorage = (event: StorageEvent) => {
			if (event.key === STOREFRONT_SYNC_STORAGE_KEY) refreshLocalization();
		};
		document.documentElement.lang = localization.locale;
		api.bootstrapAuthState(Boolean(data.isAuthenticated));
		void userStore.load(api);
		window.addEventListener(STOREFRONT_SYNC_EVENT, refreshLocalization);
		window.addEventListener("storage", handleStorage);
		return () => {
			window.removeEventListener(STOREFRONT_SYNC_EVENT, refreshLocalization);
			window.removeEventListener("storage", handleStorage);
		};
	});
</script>

{@render children?.()}
