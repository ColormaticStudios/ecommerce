<script lang="ts">
	type Align = "left" | "right" | "center";

	interface Props {
		header?: boolean;
		align?: Align;
		nowrap?: boolean;
		strong?: boolean;
		numeric?: boolean;
		class?: string;
		children?: import("svelte").Snippet;
	}

	let {
		header = false,
		align = "left",
		nowrap = false,
		strong = false,
		numeric = false,
		class: className = "",
		children,
	}: Props = $props();

	const alignClass = $derived(
		align === "right" ? "text-right" : align === "center" ? "text-center" : ""
	);
	const nowrapClass = $derived(nowrap ? "whitespace-nowrap" : "");
	const strongClass = $derived(strong ? "font-medium" : "");
	const numericClass = $derived(numeric ? "tabular-nums" : "");
	const classes = $derived(
		`px-4 py-3 ${alignClass} ${nowrapClass} ${strongClass} ${numericClass} ${className}`.trim()
	);
</script>

{#if header}
	<th class={classes}>
		{@render children?.()}
	</th>
{:else}
	<td class={classes}>
		{@render children?.()}
	</td>
{/if}
