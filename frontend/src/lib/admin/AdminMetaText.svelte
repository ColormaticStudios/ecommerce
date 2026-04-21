<script lang="ts">
	interface Props {
		as?: "p" | "span";
		tone?: "muted" | "strong";
		class?: string;
		children?: import("svelte").Snippet;
	}

	let { as = "p", tone = "muted", class: className = "", children }: Props = $props();

	const toneClasses = $derived(
		tone === "strong"
			? "text-sm font-semibold text-stone-950 dark:text-stone-50"
			: "text-xs text-stone-500 dark:text-stone-400"
	);
</script>

<svelte:element this={as} class={`${toneClasses} ${className}`.trim()}>
	{@render children?.()}
</svelte:element>
