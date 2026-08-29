<script lang="ts">
	import "./main.css";
	import "bootstrap-icons/font/bootstrap-icons.css";
	import { API, STOREFRONT_SYNC_EVENT, STOREFRONT_SYNC_STORAGE_KEY } from "$lib/api";
	import { userStore } from "$lib/user";
	import { onMount, setContext, untrack } from "svelte";
	import { LOCALIZATION_CONTEXT, LocalizationRuntime } from "$lib/localization/runtime";
	import { localizationDomainForPath } from "$lib/localization/domain";
	import { page } from "$app/state";
	import { afterNavigate } from "$app/navigation";
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
	afterNavigate((navigation) => {
		if (navigation.to?.url) {
			void localization.setDomain(localizationDomainForPath(navigation.to.url.pathname));
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
