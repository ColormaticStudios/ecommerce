<script lang="ts">
	import { resolve } from "$app/paths";
	import { adminNavItems, type AdminSectionId } from "$lib/admin/navigation";
	import IconButton from "$lib/components/IconButton.svelte";

	interface Props {
		activeSection: AdminSectionId;
		collapsed?: boolean;
		mobile?: boolean;
		onToggleCollapse?: (() => void) | undefined;
		onClose?: (() => void) | undefined;
	}

	let {
		activeSection,
		collapsed = false,
		mobile = false,
		onToggleCollapse,
		onClose,
	}: Props = $props();
</script>

<div
	class="flex h-full flex-col overflow-hidden border-r border-gray-300 bg-gray-100/95 dark:border-gray-800 dark:bg-gray-900/95"
>
	<div
		class="flex items-center justify-between gap-3 border-b border-gray-200 px-4 py-4 dark:border-gray-800"
	>
		{#if !collapsed || mobile}
			<div>
				<p class="text-sm font-semibold text-gray-950 dark:text-gray-50">Admin</p>
				<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Operations console</p>
			</div>
		{/if}

		{#if mobile}
			<IconButton
				aria-label="Close admin drawer"
				title="Close admin drawer"
				class="text-gray-700 dark:text-gray-200"
				onclick={() => onClose?.()}
			>
				<i class="bi bi-x-lg"></i>
			</IconButton>
		{:else}
			<IconButton
				aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
				title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
				class="text-gray-700 dark:text-gray-200"
				onclick={() => onToggleCollapse?.()}
			>
				<i class={`bi ${collapsed ? "bi-chevron-right" : "bi-chevron-left"}`}></i>
			</IconButton>
		{/if}
	</div>

	<nav
		class={`flex-1 space-y-1 overflow-y-auto ${mobile ? "p-4" : "p-3"}`}
		aria-label="Admin sections"
	>
		{#each adminNavItems as item (item.id)}
			<a
				href={resolve(item.href)}
				class={`group flex items-center gap-3 rounded-xl px-3 py-2.5 transition ${
					item.id === activeSection
						? "bg-gray-900 text-white shadow-sm dark:bg-gray-100 dark:text-gray-900"
						: mobile
							? "text-gray-700 hover:bg-gray-200 dark:text-gray-300 dark:hover:bg-gray-800"
							: "text-gray-700 hover:bg-gray-200/70 dark:text-gray-300 dark:hover:bg-gray-800"
				}`}
				aria-current={item.id === activeSection ? "page" : undefined}
				title={collapsed && !mobile ? item.label : undefined}
			>
				<span
					class={`inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-sm ${
						item.id === activeSection
							? "bg-white/12 text-white dark:bg-gray-200 dark:text-gray-900"
							: mobile
								? "bg-gray-200 text-gray-700 dark:bg-gray-800 dark:text-gray-200"
								: "bg-white text-gray-700 dark:bg-gray-800 dark:text-gray-200"
					}`}
				>
					<i class={`bi ${item.icon}`}></i>
				</span>
				{#if !collapsed || mobile}
					<span class="truncate text-sm font-medium">{item.label}</span>
				{/if}
			</a>
		{/each}
	</nav>
</div>
