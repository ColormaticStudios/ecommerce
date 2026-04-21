<script lang="ts">
	type CardTone = "default" | "soft" | "muted" | "sky" | "amber";
	type CardPadding = "none" | "sm" | "md" | "lg" | "xl";
	type CardRadius = "lg" | "xl" | "2xl" | "3xl";
	type CardShadow = "none" | "sm" | "md";
	type CardBorder = "solid" | "dashed";

	interface Props {
		as?: string;
		href?: string;
		type?: "button" | "submit";
		disabled?: boolean;
		class?: string;
		tone?: CardTone;
		padding?: CardPadding;
		radius?: CardRadius;
		shadow?: CardShadow;
		border?: CardBorder;
		overflowHidden?: boolean;
		interactive?: boolean;
		onclick?: (event: MouseEvent) => void;
		children?: import("svelte").Snippet;
	}

	let {
		as = "div",
		href = "",
		type = "button",
		disabled = false,
		class: className = "",
		tone = "default",
		padding = "md",
		radius = "2xl",
		shadow = "sm",
		border = "solid",
		overflowHidden = false,
		interactive = false,
		onclick,
		children,
	}: Props = $props();

	const toneClasses = $derived.by(() => {
		switch (tone) {
			case "soft":
				return "border-gray-200 bg-white/80 backdrop-blur dark:border-gray-800 dark:bg-gray-900/70";
			case "muted":
				return "border-gray-200 bg-gray-50 dark:border-gray-800 dark:bg-gray-950/50";
			case "sky":
				return "border-sky-200 bg-sky-50 dark:border-sky-900/60 dark:bg-sky-950/40";
			case "amber":
				return "border-amber-200 bg-amber-50 dark:border-amber-900/70 dark:bg-amber-950/40";
			default:
				return "border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-900";
		}
	});

	const paddingClasses = $derived.by(() => {
		switch (padding) {
			case "none":
				return "";
			case "sm":
				return "p-4";
			case "lg":
				return "p-6";
			case "xl":
				return "p-8";
			default:
				return "p-5";
		}
	});

	const radiusClasses = $derived.by(() => {
		switch (radius) {
			case "lg":
				return "rounded-lg";
			case "xl":
				return "rounded-xl";
			case "3xl":
				return "rounded-3xl";
			default:
				return "rounded-2xl";
		}
	});

	const shadowClasses = $derived.by(() => {
		switch (shadow) {
			case "none":
				return "";
			case "md":
				return "shadow-md";
			default:
				return "shadow-sm";
		}
	});

	const borderClasses = $derived(border === "dashed" ? "border-dashed" : "border");
	const overflowClasses = $derived(overflowHidden ? "overflow-hidden" : "");
	const interactiveClasses = $derived(
		interactive
			? "transition hover:-translate-y-1 hover:border-gray-300 hover:shadow-md dark:hover:border-gray-700"
			: ""
	);
	const classes = $derived(
		`${radiusClasses} ${borderClasses} ${toneClasses} ${paddingClasses} ${shadowClasses} ${overflowClasses} ${interactiveClasses} ${className}`.trim()
	);
</script>

{#if href}
	<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
	<a {href} class={classes}>
		{@render children?.()}
	</a>
{:else if as === "button"}
	<button {type} class={classes} {disabled} {onclick}>
		{@render children?.()}
	</button>
{:else}
	<svelte:element this={as} class={classes}>
		{@render children?.()}
	</svelte:element>
{/if}
