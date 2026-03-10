<script lang="ts">
	import type { HTMLAnchorAttributes } from "svelte/elements";

	type ButtonVariant = "regular" | "primary" | "warning" | "danger";
	type ButtonSize = "small" | "regular" | "large";
	type ButtonStyle = "regular" | "pill";
	type ButtonTone = "default" | "admin";

	interface Props extends HTMLAnchorAttributes {
		variant?: ButtonVariant;
		size?: ButtonSize;
		style?: ButtonStyle;
		tone?: ButtonTone;
		class?: string;
		children?: import("svelte").Snippet;
	}

	let {
		variant = "regular",
		size = "regular",
		style = "regular",
		tone = "default",
		class: className = "",
		children,
		...rest
	}: Props = $props();

	const baseClasses = "cursor-pointer transition-[background-color,border-color] duration-200";
	const shapeClasses = $derived(style === "pill" ? "rounded-full" : "rounded-lg");
	const sizeClasses = $derived.by(() => {
		switch (size) {
			case "small":
				return "px-2.5 py-1.5 text-xs";
			case "large":
				return "px-4 py-2 text-lg";
			default:
				return "px-4 py-2";
		}
	});

	const variantClasses = $derived.by(() => {
		switch (variant) {
			case "primary":
				return tone === "admin"
					? "border border-stone-900 bg-stone-900 text-white hover:border-stone-800 hover:bg-stone-800 dark:border-stone-100 dark:bg-stone-100 dark:text-stone-900 dark:hover:border-stone-200 dark:hover:bg-stone-200"
					: "text-white border border-blue-400 bg-blue-500 hover:border-blue-500 hover:bg-blue-600";
			case "warning":
				return "text-white border border-orange-400 bg-orange-500 hover:border-orange-300 hover:bg-orange-400";
			case "danger":
				return "text-white border border-red-400 bg-red-500 hover:border-red-300 hover:bg-red-400";
			default:
				return tone === "admin"
					? "border border-stone-300 bg-white text-stone-800 hover:border-stone-400 hover:bg-stone-100 dark:border-stone-700 dark:bg-stone-950 dark:text-stone-100 dark:hover:border-stone-600 dark:hover:bg-stone-900"
					: "border border-gray-300 bg-gray-200 hover:border-gray-200 hover:bg-gray-100 dark:border-gray-600 dark:bg-gray-700 hover:dark:border-gray-700 hover:dark:bg-gray-800";
		}
	});
</script>

<a class={`${baseClasses} ${shapeClasses} ${sizeClasses} ${variantClasses} ${className}`} {...rest}>
	{@render children?.()}
</a>
