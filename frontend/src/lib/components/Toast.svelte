<script lang="ts">
	import IconButton from "$lib/components/IconButton.svelte";

	export let message = "";
	export let visible = false;
	export let tone: "neutral" | "success" | "error" = "neutral";
	export let actionHref: string | undefined = undefined;
	export let actionLabel = "";
	export let position: "top-right" | "top-center" = "top-right";
	export let onClose: (() => void) | undefined = undefined;

	const toneClasses = {
		neutral:
			"border-gray-300 bg-gray-100 text-gray-900 dark:border-gray-700 dark:bg-gray-900/90 dark:text-gray-100",
		success:
			"border-emerald-300 bg-emerald-100 text-emerald-900 dark:border-emerald-700 dark:bg-emerald-900/80 dark:text-emerald-100",
		error:
			"border-red-300 bg-red-100 text-red-900 dark:border-red-700 dark:bg-red-900/80 dark:text-red-100",
	} as const;

	const positionClasses = {
		"top-right": "top-20 right-4",
		"top-center": "top-6 left-1/2 -translate-x-1/2",
	} as const;

	const iconClasses = {
		neutral: "bi bi-info-circle-fill",
		success: "bi bi-check-circle-fill",
		error: "bi bi-x-circle-fill",
	} as const;
</script>

{#if message}
	<div
		class={`fixed z-50 inline-flex max-w-md items-center gap-2 rounded-full border px-4 py-2 text-sm shadow-lg backdrop-blur transition-all duration-300 ${toneClasses[tone]} ${positionClasses[position]} ${visible ? "translate-y-0 opacity-100" : "-translate-y-2 opacity-0"}`}
		role="status"
		aria-live="polite"
	>
		<i class={iconClasses[tone]}></i>
		<span class="whitespace-nowrap">{message}</span>
		{#if actionHref && actionLabel}
			<a
				href={actionHref}
				class="ml-1 font-semibold text-blue-700 underline-offset-2 hover:underline dark:text-blue-300"
			>
				{actionLabel}
			</a>
		{/if}
		{#if onClose}
			<IconButton
				size="sm"
				variant={tone === "error" ? "danger" : tone === "success" ? "primary" : "neutral"}
				aria-label="Close notification"
				title="Close"
				onclick={onClose}
			>
				<i class="bi bi-x-lg"></i>
			</IconButton>
		{/if}
	</div>
{/if}
