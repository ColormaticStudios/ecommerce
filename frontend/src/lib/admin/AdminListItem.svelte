<script lang="ts">
	import {
		adminListItemActiveClass,
		adminListItemBaseClass,
		adminListItemInteractiveClass,
	} from "$lib/admin/tokens";

	interface Props {
		as?: "div" | "button";
		type?: "button" | "submit";
		active?: boolean;
		interactive?: boolean;
		disabled?: boolean;
		class?: string;
		onclick?: (event: MouseEvent) => void;
		children?: import("svelte").Snippet;
	}

	let {
		as = "div",
		type = "button",
		active = false,
		interactive = false,
		disabled = false,
		class: className = "",
		onclick,
		children,
	}: Props = $props();

	const activeClasses = $derived(active ? adminListItemActiveClass : "");
	const interactiveClasses = $derived(interactive ? adminListItemInteractiveClass : "");
	const buttonClasses = $derived(as === "button" ? "w-full text-left" : "");
	const classes = $derived(
		`${adminListItemBaseClass} ${activeClasses} ${interactiveClasses} ${buttonClasses} ${className}`.trim()
	);
</script>

{#if as === "button"}
	<button {type} class={classes} {disabled} {onclick}>
		{@render children?.()}
	</button>
{:else}
	<div class={classes}>
		{@render children?.()}
	</div>
{/if}
