<script lang="ts">
	interface Props {
		tone?: "default" | "error";
		class?: string;
		children?: import("svelte").Snippet;
	}

	let { tone = "default", class: className = "", children }: Props = $props();

	const toneClasses = $derived(
		tone === "error"
			? "border-rose-300 bg-rose-50/80 text-rose-700 dark:border-rose-900/80 dark:bg-rose-950/35 dark:text-rose-200"
			: "border-stone-300 bg-stone-50/75 text-stone-500 dark:border-stone-700 dark:bg-stone-900/50 dark:text-stone-400"
	);
</script>

<p class={`rounded-2xl border border-dashed px-4 py-8 text-sm ${toneClasses} ${className}`.trim()}>
	{@render children?.()}
</p>
