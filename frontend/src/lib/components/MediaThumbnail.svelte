<script lang="ts">
	interface Props {
		src?: string | null;
		alt?: string;
		href?: string;
		label?: string;
		class?: string;
		imageClass?: string;
		placeholder?: string;
		type?: "button";
		active?: boolean;
		onclick?: (event: MouseEvent) => void;
	}

	let {
		src = "",
		alt = "",
		href = "",
		label = "",
		class: className = "h-20 w-20 rounded-xl",
		imageClass = "object-cover",
		placeholder = "No image",
		type,
		active = false,
		onclick,
	}: Props = $props();

	const activeClasses = $derived(active ? "border-gray-900 dark:border-gray-100" : "");
	const classes = $derived(
		`flex shrink-0 items-center justify-center overflow-hidden border border-gray-200 bg-gray-100 text-center text-xs text-gray-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400 ${activeClasses} ${className}`.trim()
	);
</script>

{#if href}
	<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
	<a {href} class={classes} aria-label={label || undefined}>
		{#if src}
			<img {src} {alt} class={`h-full w-full ${imageClass}`.trim()} loading="lazy" />
		{:else}
			<span>{placeholder}</span>
		{/if}
	</a>
{:else if type === "button"}
	<button {type} class={`${classes} cursor-pointer`} {onclick} aria-label={label || undefined}>
		{#if src}
			<img {src} {alt} class={`h-full w-full ${imageClass}`.trim()} loading="lazy" />
		{:else}
			<span>{placeholder}</span>
		{/if}
	</button>
{:else}
	<div class={classes}>
		{#if src}
			<img {src} {alt} class={`h-full w-full ${imageClass}`.trim()} loading="lazy" />
		{:else}
			<span>{placeholder}</span>
		{/if}
	</div>
{/if}
