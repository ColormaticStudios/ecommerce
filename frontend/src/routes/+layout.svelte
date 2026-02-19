<script lang="ts">
	import "./main.css";
	import "bootstrap-icons/font/bootstrap-icons.css";
	import { API } from "$lib/api";
	import StorefrontFooter from "$lib/components/StorefrontFooter.svelte";
	import { userStore } from "$lib/user";
	import { onMount, setContext } from "svelte";
	import { resolve } from "$app/paths";
	import { navigating } from "$app/state";
	import type { LayoutData } from "./$types";

	const api = new API();
	setContext("api", api);

	let menuOpen = $state(false);
	let menuRef = $state<HTMLDivElement | null>(null);
	let cartCount = $state<number | null>(null);
	let cartCountLoading = $state(false);
	let cartCountLoaded = $state(false);
	const showNavigationSpinner = $derived(Boolean(navigating.to));

	async function refreshCartCount() {
		const authenticated = await api.refreshAuthState();
		if (!authenticated) {
			cartCount = null;
			cartCountLoaded = true;
			return;
		}
		cartCountLoading = true;
		try {
			const cart = await api.viewCart();
			cartCount = cart.items.length;
		} catch (err) {
			console.error(err);
			cartCount = null;
		} finally {
			cartCountLoaded = true;
			cartCountLoading = false;
		}
	}

	onMount(() => {
		void userStore.load(api);

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

		return () => {
			window.removeEventListener("click", handleClick);
			window.removeEventListener("keydown", handleKeydown);
			window.removeEventListener("cart:updated", refreshCartCount);
		};
	});

	$effect(() => {
		if ($userStore) {
			if (!cartCountLoaded && !cartCountLoading) {
				void refreshCartCount();
			}
		} else {
			cartCount = null;
			cartCountLoaded = false;
		}
	});

	interface Props {
		data: LayoutData;
		children?: import("svelte").Snippet;
	}
	let { data, children }: Props = $props();
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
				<i class="bi bi-arrow-repeat animate-spin"></i>
				Loading...
			</span>
		</div>
	{/if}
	<nav class="flex items-center justify-between bg-gray-100 px-3 py-2 dark:bg-gray-900">
		<div class="flex items-center gap-2">
			<a href={resolve("/")} class="navlink text-2xl">{data.storefront.site_title || "Ecommerce"}</a
			>
		</div>
		<div class="flex items-center gap-3">
			<a href={resolve("/search")} class="navlink text-lg" aria-label="search">
				<i class="bi bi-search"></i>
			</a>
			{#if $userStore}
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
							class="absolute right-0 mt-2 w-44 rounded-lg border border-gray-200 bg-white p-1 text-sm shadow-lg dark:border-gray-700 dark:bg-gray-900"
						>
							{#if $userStore.role === "admin"}
								<a href={resolve("/admin")} class="menu-item" onclick={() => (menuOpen = false)}>
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
					<a href={resolve("/login")} class="navlink text-xl">Log In</a>
					<a href={resolve("/signup")} class="navlink text-xl">Sign Up</a>
				</div>
			{/if}
		</div>
	</nav>

	<main class="flex-1">
		{@render children?.()}
	</main>

	<StorefrontFooter footer={data.storefront.footer} />
</div>

<style>
	@reference "tailwindcss";

	a.navlink {
		@apply px-2 dark:text-white;
		@apply hover:text-gray-500 dark:hover:text-gray-300;
		@apply transition-[color] duration-200;
	}

	.menu-item {
		@apply flex w-full items-center justify-between rounded-md px-3 py-2 text-gray-700 transition hover:bg-gray-100 hover:text-gray-900 dark:text-gray-200 dark:hover:bg-gray-800 dark:hover:text-white;
	}
</style>
