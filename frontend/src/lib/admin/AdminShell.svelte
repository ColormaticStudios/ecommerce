<script lang="ts">
	import { beforeNavigate, goto } from "$app/navigation";
	import { onMount, setContext } from "svelte";
	import { cubicOut } from "svelte/easing";
	import { fade, fly } from "svelte/transition";
	import { page } from "$app/state";
	import {
		ADMIN_DIRTY_NAVIGATION_CONTEXT,
		createAdminDirtyNavigationController,
		toAdminNavigationTarget,
	} from "$lib/admin/dirty-navigation";
	import AdminSidebar from "$lib/admin/AdminSidebar.svelte";
	import { getActiveAdminSection } from "$lib/admin/navigation";
	import IconButton from "$lib/components/IconButton.svelte";

	interface Props {
		children?: import("svelte").Snippet;
	}

	let { children }: Props = $props();

	const collapsedSidebarStorageKey = "admin-sidebar-collapsed";
	let drawerOpen = $state(false);
	let sidebarCollapsed = $state(false);
	const activeSection = $derived(getActiveAdminSection(page.url.pathname));
	const dirtyNavigation = createAdminDirtyNavigationController((message) =>
		window.confirm(message)
	);
	setContext(ADMIN_DIRTY_NAVIGATION_CONTEXT, dirtyNavigation);

	beforeNavigate((navigation) => {
		const currentTarget = toAdminNavigationTarget(page.url);
		const nextTarget = toAdminNavigationTarget(navigation.to?.url);
		if (
			!navigation.to?.route?.id ||
			!dirtyNavigation.shouldBlockNavigation(currentTarget, nextTarget)
		) {
			return;
		}

		navigation.cancel();
		if (!dirtyNavigation.confirmNavigation()) {
			return;
		}

		if (!nextTarget) {
			return;
		}

		dirtyNavigation.allowNextNavigation(nextTarget);
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		void goto(nextTarget, {
			replaceState: navigation.type === "popstate",
		});
	});

	onMount(() => {
		const saved = window.localStorage.getItem(collapsedSidebarStorageKey);
		if (saved === "true") {
			sidebarCollapsed = true;
		}

		function handleBeforeUnload(event: BeforeUnloadEvent) {
			if (!dirtyNavigation.dirty) {
				return;
			}
			event.preventDefault();
			event.returnValue = "";
		}

		window.addEventListener("beforeunload", handleBeforeUnload);
		return () => {
			window.removeEventListener("beforeunload", handleBeforeUnload);
		};
	});

	$effect(() => {
		const pathname = page.url.pathname;
		if (pathname) {
			drawerOpen = false;
		}
	});

	$effect(() => {
		window.localStorage.setItem(collapsedSidebarStorageKey, sidebarCollapsed ? "true" : "false");
	});
</script>

<section class="min-h-screen lg:h-screen lg:overflow-hidden">
	<IconButton
		aria-label="Open admin sections"
		title="Open admin sections"
		class="m-4 text-gray-800 shadow-sm lg:hidden dark:text-gray-100"
		onclick={() => (drawerOpen = true)}
	>
		<i class="bi bi-layout-sidebar-inset"></i>
	</IconButton>

	<div class="grid min-h-screen items-stretch lg:h-full lg:grid-cols-[auto_minmax(0,1fr)]">
		<aside
			class={`hidden border-r border-gray-200 bg-gray-100 transition-[width] duration-200 lg:block lg:overflow-y-auto lg:overscroll-contain dark:border-gray-800 dark:bg-gray-900 ${
				sidebarCollapsed ? "w-20" : "w-64"
			}`}
		>
			<AdminSidebar
				{activeSection}
				collapsed={sidebarCollapsed}
				onToggleCollapse={() => (sidebarCollapsed = !sidebarCollapsed)}
			/>
		</aside>

		<div
			class="min-w-0 bg-gray-100 px-4 py-6 sm:px-6 lg:overflow-y-auto lg:overscroll-contain lg:px-8 dark:bg-gray-950"
		>
			{@render children?.()}
		</div>
	</div>
</section>

{#if drawerOpen}
	<div
		class="fixed inset-0 z-40 lg:hidden"
		aria-label="Admin section drawer"
		role="dialog"
		aria-modal="true"
	>
		<button
			type="button"
			class="absolute inset-0 backdrop-blur-[2px]"
			style="background-color: rgb(3 7 18 / 0.45);"
			aria-label="Close admin drawer"
			onclick={() => (drawerOpen = false)}
			transition:fade={{ duration: 180 }}
		></button>
		<div
			class="absolute inset-y-0 left-0 w-[min(88vw,20rem)]"
			transition:fly={{ x: -28, duration: 220, opacity: 0, easing: cubicOut }}
		>
			<div class="h-full shadow-2xl">
				<AdminSidebar {activeSection} mobile={true} onClose={() => (drawerOpen = false)} />
			</div>
		</div>
	</div>
{/if}
