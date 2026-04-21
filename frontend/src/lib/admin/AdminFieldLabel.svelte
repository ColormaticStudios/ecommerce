<script lang="ts">
	interface Props {
		as?: "label" | "p" | "span";
		for?: string;
		class?: string;
		children?: import("svelte").Snippet;
	}

	let { as = "p", for: htmlFor = "", class: className = "", children }: Props = $props();
</script>

<svelte:element
	this={as}
	for={as === "label" ? htmlFor : undefined}
	class={`text-[11px] font-semibold tracking-[0.18em] text-stone-500 uppercase dark:text-stone-400 ${className}`.trim()}
>
	{@render children?.()}
</svelte:element>
