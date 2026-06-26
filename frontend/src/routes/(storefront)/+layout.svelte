<script lang="ts">
	/* eslint-disable svelte/no-navigation-without-resolve */
	import { DRAFT_PREVIEW_SYNC_EVENT, DRAFT_PREVIEW_SYNC_STORAGE_KEY } from "$lib/api";
	import type { API } from "$lib/api";
	import { cmsHref, isExternalHref, type CmsContentBlock } from "$lib/cms";
	import { userStore } from "$lib/user";
	import { getContext, onMount } from "svelte";
	import { invalidateAll } from "$app/navigation";
	import { resolve } from "$app/paths";
	import { navigating } from "$app/state";
	import type { LayoutData } from "./$types";

	const api = getContext<API>("api");

	let menuOpen = $state(false);
	let menuRef = $state<HTMLDivElement | null>(null);
	let cartCount = $state<number | null>(null);
	let cartCountLoading = $state(false);
	let cartCountLoaded = $state(false);
	let lastCartOwnerKey = $state<string | null>(null);
	const showNavigationSpinner = $derived(Boolean(navigating.to));
	let exitingDraftPreview = $state(false);
	let draftPreviewError = $state("");

	interface Props {
		data: LayoutData;
		children?: import("svelte").Snippet;
	}
	let { data, children }: Props = $props();
	let draftPreviewOverride = $state<LayoutData["draftPreview"] | null>(null);
	const draftPreview = $derived(
		draftPreviewOverride ?? data.draftPreview ?? { active: false, expires_at: null }
	);
	const draftPreviewActive = $derived(Boolean(draftPreview?.active));
	const headerNavigation = $derived(data.cmsNavigation);
	const globalRegions = $derived(data.cmsGlobalRegions ?? {});
	const draftPreviewExpiresLabel = $derived.by(() => {
		const raw = draftPreview?.expires_at;
		if (!raw) {
			return "";
		}
		const parsed = new Date(raw);
		if (Number.isNaN(parsed.valueOf())) {
			return "";
		}
		return parsed.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
	});

	async function refreshCartCount() {
		cartCountLoading = true;
		try {
			cartCount = await api.viewCartSummary();
		} catch (err) {
			if (typeof err === "object" && err && "status" in err && err.status === 403) {
				cartCount = 0;
				return;
			}
			console.error(err);
			cartCount = null;
		} finally {
			cartCountLoaded = true;
			cartCountLoading = false;
		}
	}

	function handleDraftPreviewSyncEvent(event: Event) {
		const syncEvent = event as CustomEvent<{ active?: unknown; expires_at?: unknown }>;
		if (typeof syncEvent.detail?.active !== "boolean") {
			return;
		}
		draftPreviewOverride = {
			active: syncEvent.detail.active,
			expires_at:
				typeof syncEvent.detail.expires_at === "string" ? syncEvent.detail.expires_at : null,
		};
		void invalidateAll();
	}

	function handleDraftPreviewStorageEvent(event: StorageEvent) {
		if (event.key !== DRAFT_PREVIEW_SYNC_STORAGE_KEY || !event.newValue) {
			return;
		}
		try {
			const parsed = JSON.parse(event.newValue) as { active?: unknown; expires_at?: unknown };
			if (typeof parsed.active !== "boolean") {
				return;
			}
			draftPreviewOverride = {
				active: parsed.active,
				expires_at: typeof parsed.expires_at === "string" ? parsed.expires_at : null,
			};
			void invalidateAll();
		} catch {
			// ignore malformed storage payloads
		}
	}

	onMount(() => {
		const unsubscribeUser = userStore.subscribe((user) => {
			const cartOwnerKey = user ? `user:${user.id}` : "guest";
			if ((!cartCountLoaded && !cartCountLoading) || lastCartOwnerKey !== cartOwnerKey) {
				lastCartOwnerKey = cartOwnerKey;
				void refreshCartCount();
			}
		});

		const handleClick = (event: MouseEvent) => {
			if (!menuOpen || !menuRef) {
				return;
			}
			if (!menuRef.contains(event.target as Node)) {
				menuOpen = false;
			}
		};

		const handleKeydown = (event: KeyboardEvent) => {
			if (event.key === "Escape") {
				menuOpen = false;
			}
		};

		window.addEventListener("click", handleClick);
		window.addEventListener("keydown", handleKeydown);
		window.addEventListener("cart:updated", refreshCartCount);
		window.addEventListener(DRAFT_PREVIEW_SYNC_EVENT, handleDraftPreviewSyncEvent as EventListener);
		window.addEventListener("storage", handleDraftPreviewStorageEvent);

		return () => {
			unsubscribeUser();
			window.removeEventListener("click", handleClick);
			window.removeEventListener("keydown", handleKeydown);
			window.removeEventListener("cart:updated", refreshCartCount);
			window.removeEventListener(
				DRAFT_PREVIEW_SYNC_EVENT,
				handleDraftPreviewSyncEvent as EventListener
			);
			window.removeEventListener("storage", handleDraftPreviewStorageEvent);
		};
	});

	async function exitDraftPreview() {
		if (exitingDraftPreview) {
			return;
		}
		exitingDraftPreview = true;
		draftPreviewError = "";
		try {
			const session = await api.stopAdminPreview();
			draftPreviewOverride = {
				active: session.active,
				expires_at: session.expires_at ? session.expires_at.toISOString() : null,
			};
			await invalidateAll();
			exitingDraftPreview = false;
		} catch (err) {
			console.error(err);
			exitingDraftPreview = false;
			draftPreviewError = "Could not exit draft view.";
		}
	}

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

	function regionBlocks(region: string): CmsContentBlock[] {
		return globalRegions[region]?.blocks ?? [];
	}

	const footerClass =
		"border-stone-200 bg-white text-stone-700 dark:border-stone-800 dark:bg-stone-950 dark:text-stone-200";
</script>

<svelte:head>
	<!-- <link rel="icon" href="" /> -->
</svelte:head>

<div class="flex min-h-screen flex-col">
	{#if showNavigationSpinner}
		<div
			class="pointer-events-none fixed top-4 left-1/2 z-50 -translate-x-1/2 rounded-full border border-gray-200 bg-white/95 px-3 py-1.5 text-xs font-medium text-gray-700 shadow-sm dark:border-gray-700 dark:bg-gray-900/95 dark:text-gray-200"
			role="status"
			aria-live="polite"
			aria-label="Page loading"
		>
			<span class="inline-flex items-center gap-2">
				<i class="bi bi-arrow-repeat inline-block animate-spin"></i>
				Loading...
			</span>
		</div>
	{/if}
	{#if draftPreviewActive}
		<div
			class="fixed top-14 left-1/2 z-40 flex -translate-x-1/2 items-center gap-2 rounded-full border border-amber-200 bg-amber-50/95 px-2 py-1.5 text-xs font-medium text-amber-800 shadow-sm backdrop-blur dark:border-amber-900/70 dark:bg-amber-950/80 dark:text-amber-100"
			role="status"
			aria-live="polite"
		>
			<span class="inline-flex items-center gap-1.5">
				<i class="bi bi-eye"></i>
				Viewing draft preview
				{#if draftPreviewExpiresLabel}
					<span class="text-[10px] text-amber-700/90 dark:text-amber-200/80">
						until {draftPreviewExpiresLabel}
					</span>
				{/if}
			</span>
			<button
				type="button"
				class="cursor-pointer rounded-full border border-amber-300 bg-white/70 px-2 py-0.5 text-[10px] font-semibold transition hover:bg-white dark:border-amber-700 dark:bg-amber-900/50 dark:hover:bg-amber-900/70"
				disabled={exitingDraftPreview}
				onclick={exitDraftPreview}
			>
				{exitingDraftPreview ? "Exiting..." : "Exit"}
			</button>
			{#if draftPreviewError}
				<span class="text-[10px] text-rose-700 dark:text-rose-300">{draftPreviewError}</span>
			{/if}
		</div>
	{/if}
	{#if regionBlocks("announcement_bar").length > 0}
		<div
			class="border-b border-gray-200 bg-gray-950 px-4 py-2 text-center text-sm text-white dark:border-gray-800 dark:bg-gray-100 dark:text-gray-950"
		>
			{#each regionBlocks("announcement_bar") as block, index (`announcement-${index}`)}
				{#if block.type === "promo_banner"}
					<span class="font-medium">{block.title}</span>
					{#if block.body}<span class="ml-2 text-gray-200 dark:text-gray-700">{block.body}</span
						>{/if}
					{#if block.link}
						<a
							class="ml-3 font-semibold underline underline-offset-4"
							href={hrefFor(block.link.url)}
							target={targetFor(block.link.url)}
							rel={relFor(block.link.url)}
						>
							{block.link.label}
						</a>
					{/if}
				{:else if block.type === "rich_text"}
					<span>{block.body}</span>
				{/if}
			{/each}
		</div>
	{/if}
	<nav class="flex items-center justify-between bg-gray-100 px-3 py-2 dark:bg-gray-900">
		<div class="flex items-center gap-2">
			<a href={resolve("/")} class="navlink text-2xl">Ecommerce</a>
			{#if headerNavigation?.items?.length}
				<div class="hidden items-center gap-1 pl-3 md:flex">
					{#each headerNavigation.items as item (item.id)}
						{#if item.itemType === "dropdown"}
							<div class="group relative">
								<button type="button" class="navlink flex items-center gap-1 text-sm font-medium">
									{item.label}
									<i class="bi bi-chevron-down text-[10px]"></i>
								</button>
								<div
									class="invisible absolute top-full left-0 z-20 mt-2 min-w-44 rounded-lg border border-gray-200 bg-white p-1 text-sm opacity-0 shadow-lg transition group-focus-within:visible group-focus-within:opacity-100 group-hover:visible group-hover:opacity-100 dark:border-gray-700 dark:bg-gray-900"
								>
									{#each item.children as child (child.id)}
										<a
											href={hrefFor(child.url)}
											target={targetFor(child.url)}
											rel={relFor(child.url)}
											class="menu-item"
										>
											{child.label}
										</a>
									{/each}
								</div>
							</div>
						{:else}
							<a
								href={hrefFor(item.url)}
								target={targetFor(item.url)}
								rel={relFor(item.url)}
								class="navlink text-sm font-medium"
							>
								{item.label}
							</a>
						{/if}
					{/each}
				</div>
			{/if}
		</div>
		<div class="flex items-center gap-3">
			<a href={resolve("/search")} class="navlink text-lg" aria-label="search">
				<i class="bi bi-search"></i>
			</a>
			<a
				href={resolve("/cart")}
				class="relative flex h-10 w-10 items-center justify-center rounded-full border border-gray-200 bg-white text-gray-700 shadow-sm transition hover:border-gray-300 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 dark:hover:border-gray-600 dark:hover:bg-gray-700"
				aria-label="View cart"
			>
				<i class="bi bi-cart text-lg"></i>
				{#if cartCountLoading}
					<span
						class="absolute -top-1 -right-1 rounded-full bg-gray-200 px-1 text-[10px] font-semibold text-gray-600 dark:bg-gray-700 dark:text-gray-100"
					>
						...
					</span>
				{:else if cartCount != null && cartCount > 0}
					<span
						class="absolute -top-1 -right-1 min-w-[1.15rem] rounded-full bg-blue-200 px-1 text-center text-[10px] font-semibold text-blue-700 dark:bg-blue-800/60 dark:text-blue-200"
					>
						{cartCount}
					</span>
				{/if}
			</a>
			{#if $userStore}
				<div class="relative" bind:this={menuRef}>
					<button
						type="button"
						class="flex cursor-pointer items-center gap-2 rounded-full border border-gray-200 bg-white px-3 py-1 text-sm text-gray-900 shadow-sm transition hover:border-gray-300 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100 dark:hover:border-gray-600 dark:hover:bg-gray-700"
						onclick={(event) => {
							event.stopPropagation();
							menuOpen = !menuOpen;
						}}
					>
						{#if $userStore.profile_photo_url}
							<img
								src={$userStore.profile_photo_url}
								alt="Profile"
								class="h-7 w-7 rounded-full object-cover"
							/>
						{:else}
							<span
								class="flex h-7 w-7 items-center justify-center rounded-full bg-gray-200 text-xs font-semibold text-gray-600 dark:bg-gray-700 dark:text-gray-200"
							>
								{($userStore.name || $userStore.username || "?").slice(0, 1).toUpperCase()}
							</span>
						{/if}
						<span class="max-w-35 truncate">
							{$userStore.name || $userStore.username}
						</span>
						<i class="bi bi-chevron-down text-xs"></i>
					</button>
					{#if menuOpen}
						<div
							class="absolute right-0 z-1 mt-2 w-44 rounded-lg border border-gray-200 bg-white p-1 text-sm shadow-lg dark:border-gray-700 dark:bg-gray-900"
						>
							{#if $userStore.role === "admin"}
								<a
									href={resolve("/admin/products")}
									class="menu-item"
									onclick={() => (menuOpen = false)}
								>
									Admin
									<i class="bi bi-shield-lock"></i>
								</a>
							{/if}
							<a href={resolve("/checkout")} class="menu-item" onclick={() => (menuOpen = false)}>
								Checkout
								<i class="bi bi-credit-card"></i>
							</a>
							<a href={resolve("/profile")} class="menu-item" onclick={() => (menuOpen = false)}>
								Edit profile
								<i class="bi bi-person"></i>
							</a>
							<a href={resolve("/orders")} class="menu-item" onclick={() => (menuOpen = false)}>
								Orders
								<i class="bi bi-receipt"></i>
							</a>
							<button
								type="button"
								class="menu-item cursor-pointer text-left"
								onclick={() => {
									menuOpen = false;
									void $userStore.logOut();
								}}
							>
								Sign out
								<i class="bi bi-box-arrow-right"></i>
							</button>
						</div>
					{/if}
				</div>
			{:else}
				<div>
					<a href={resolve("/login")} class="navlink">Log In</a>
					<a href={resolve("/signup")} class="navlink">Sign Up</a>
				</div>
			{/if}
		</div>
	</nav>
	{#if headerNavigation?.items?.length}
		<div
			class="flex gap-1 overflow-x-auto border-b border-gray-200 bg-white px-3 py-2 md:hidden dark:border-gray-800 dark:bg-gray-950"
		>
			{#each headerNavigation.items as item (item.id)}
				{#if item.itemType === "dropdown"}
					<span
						class="shrink-0 rounded-md px-2 py-1 text-sm font-semibold text-gray-500 dark:text-gray-400"
					>
						{item.label}
					</span>
					{#each item.children as child (child.id)}
						<a
							href={hrefFor(child.url)}
							target={targetFor(child.url)}
							rel={relFor(child.url)}
							class="shrink-0 rounded-md px-2 py-1 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-900"
						>
							{child.label}
						</a>
					{/each}
				{:else}
					<a
						href={hrefFor(item.url)}
						target={targetFor(item.url)}
						rel={relFor(item.url)}
						class="shrink-0 rounded-md px-2 py-1 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-900"
					>
						{item.label}
					</a>
				{/if}
			{/each}
		</div>
	{/if}
	{#if regionBlocks("sitewide_banner").length > 0}
		<section
			class="border-b border-gray-200 bg-blue-50 px-4 py-3 text-sm text-blue-950 dark:border-gray-800 dark:bg-blue-950/40 dark:text-blue-100"
		>
			<div class="mx-auto flex max-w-7xl flex-wrap items-center justify-center gap-x-3 gap-y-1">
				{#each regionBlocks("sitewide_banner") as block, index (`sitewide-${index}`)}
					{#if block.type === "promo_banner"}
						<strong>{block.title}</strong>
						{#if block.body}<span>{block.body}</span>{/if}
						{#if block.link}
							<a
								class="font-semibold underline underline-offset-4"
								href={hrefFor(block.link.url)}
								target={targetFor(block.link.url)}
								rel={relFor(block.link.url)}
							>
								{block.link.label}
							</a>
						{/if}
					{:else if block.type === "cta"}
						<span>{block.body}</span>
						<a class="font-semibold underline underline-offset-4" href={hrefFor(block.url)}>
							{block.label}
						</a>
					{/if}
				{/each}
			</div>
		</section>
	{/if}

	<main class="flex-1">
		{@render children?.()}
	</main>

	{#if regionBlocks("trust_strip").length > 0}
		<section
			class="border-t border-gray-200 bg-gray-50 px-4 py-4 text-sm text-gray-700 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-200"
		>
			<div class="mx-auto flex max-w-7xl flex-wrap justify-center gap-x-6 gap-y-2">
				{#each regionBlocks("trust_strip") as block, index (`trust-${index}`)}
					{#if block.type === "rich_text"}
						<span>{block.body}</span>
					{:else if block.type === "promo_banner"}
						<span class="font-medium">{block.title}</span>
					{/if}
				{/each}
			</div>
		</section>
	{/if}
	{#if regionBlocks("footer").length > 0}
		<section class="text-sm">
			<div class="mx-auto max-w-none">
				{#each regionBlocks("footer") as block, index (`footer-${index}`)}
					{#if block.type === "footer"}
						<div class={`border-t px-4 py-10 sm:px-6 ${footerClass}`}>
							<div class="mx-auto max-w-7xl">
								<div
									class={block.layout === "centered"
										? "text-center"
										: block.layout === "minimal"
											? "flex flex-wrap items-center justify-between gap-5"
											: "grid gap-10 md:grid-cols-[minmax(14rem,1.1fr)_2fr]"}
								>
									<div>
										<a href={resolve("/")} class="text-lg font-semibold">{block.brand_name}</a
										>{#if block.tagline}<p
												class={`mt-3 max-w-sm opacity-70 ${block.layout === "centered" ? "mx-auto" : ""}`}
											>
												{block.tagline}
											</p>{/if}
									</div>
									{#if block.layout !== "minimal"}<div
											class={`grid gap-7 ${block.layout === "centered" ? "mt-7 sm:grid-cols-3" : "sm:grid-cols-2 lg:grid-cols-3"}`}
										>
											{#each block.columns as column, columnIndex (columnIndex)}<div>
													<h2 class="font-semibold">{column.title}</h2>
													<ul class="mt-3 space-y-2 opacity-75">
														{#each column.links as link, linkIndex (linkIndex)}<li>
																<a
																	class="hover:underline"
																	href={hrefFor(link.url)}
																	target={targetFor(link.url)}
																	rel={relFor(link.url)}>{link.label}</a
																>
															</li>{/each}
													</ul>
												</div>{/each}
										</div>{/if}
								</div>
								<div
									class="mt-9 flex flex-wrap items-center justify-between gap-4 border-t border-current/15 pt-5 text-xs opacity-70"
								>
									<span>{block.copyright}</span>{#if block.social_links.length}<div
											class="flex flex-wrap gap-4"
										>
											{#each block.social_links as link, linkIndex (linkIndex)}<a
													class="hover:underline"
													href={hrefFor(link.url)}
													target={targetFor(link.url)}
													rel={relFor(link.url)}>{link.label}</a
												>{/each}
										</div>{/if}
								</div>
							</div>
						</div>
					{:else if block.type === "rich_text"}
						<p
							class="border-t border-stone-200 bg-white px-4 py-3 text-stone-600 dark:border-stone-800 dark:bg-stone-950 dark:text-stone-300"
						>
							{block.body}
						</p>
					{:else if block.type === "cta"}
						<a
							class="block bg-white px-4 py-3 font-semibold text-blue-700 underline underline-offset-4 dark:bg-stone-950 dark:text-blue-300"
							href={hrefFor(block.url)}
						>
							{block.label}
						</a>
					{/if}
				{/each}
			</div>
		</section>
	{/if}
</div>

<style>
	@reference "tailwindcss";

	.navlink {
		@apply px-2 dark:text-white;
		@apply hover:text-gray-500 dark:hover:text-gray-300;
		@apply transition-[color] duration-200;
	}

	.menu-item {
		@apply flex w-full items-center justify-between rounded-md px-3 py-2 text-gray-700 transition hover:bg-gray-100 hover:text-gray-900 dark:text-gray-200 dark:hover:bg-gray-800 dark:hover:text-white;
	}
</style>
