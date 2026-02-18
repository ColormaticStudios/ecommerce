<script lang="ts">
	export let message: string;
	export let tone: "error" | "success" = "error";
	export let onClose: (() => void) | undefined;
	export let icon: string | null = null;

	const toneStyles = {
		error: {
			container:
				"border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900/30 dark:text-red-200",
			hover: "hover:bg-red-100 dark:hover:bg-red-900/40",
		},
		success: {
			container:
				"border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-200",
			hover: "hover:bg-emerald-100 dark:hover:bg-emerald-900/40",
		},
	};

	$: styles = toneStyles[tone] ?? toneStyles.error;
</script>

<div
	class={`flex items-start justify-between gap-3 rounded-xl border px-4 py-3 text-sm ${styles.container}`}
>
	<span class="min-w-0 break-words [overflow-wrap:anywhere]">
		{#if icon}
			<i class={`bi ${icon} mr-1`}></i>
		{/if}
		{message}
	</span>
	{#if onClose}
		<button
			class={`flex h-6 w-6 cursor-pointer items-center justify-center rounded-full text-xl transition ${styles.hover}`}
			type="button"
			aria-label="Dismiss message"
			onclick={onClose}
		>
			<i class="bi bi-x"></i>
		</button>
	{/if}
</div>
