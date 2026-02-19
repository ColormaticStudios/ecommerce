<script lang="ts">
	import type { HTMLButtonAttributes } from "svelte/elements";

	type ButtonVariant = "regular" | "primary" | "warning" | "danger";
	type ButtonSize = "small" | "regular" | "large";
	type ButtonStyle = "regular" | "pill";

	interface Props extends HTMLButtonAttributes {
		variant?: ButtonVariant;
		size?: ButtonSize;
		style?: ButtonStyle;
		class?: string;
		children?: import("svelte").Snippet;
	}

	let {
		variant = "regular",
		size = "regular",
		style = "regular",
		class: className = "",
		type = "button",
		children,
		...rest
	}: Props = $props();

	const baseClasses =
		"cursor-pointer transition-[background-color,border-color] duration-200 disabled:cursor-auto";
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
				return "text-white border border-blue-400 bg-blue-500 hover:border-blue-500 hover:bg-blue-600 disabled:text-gray-300 disabled:border-blue-300 disabled:bg-blue-400";
			case "warning":
				return "text-white border border-orange-400 bg-orange-500 hover:border-orange-300 hover:bg-orange-400 disabled:text-gray-300 disabled:border-orange-600 disabled:bg-orange-700";
			case "danger":
				return "text-white border border-red-400 bg-red-500 hover:border-red-300 hover:bg-red-400 disabled:text-gray-300 disabled:border-red-600 disabled:bg-red-700";
			default:
				return "border border-gray-300 bg-gray-200 hover:border-gray-200 hover:bg-gray-100 dark:border-gray-600 dark:bg-gray-700 hover:dark:border-gray-700 hover:dark:bg-gray-800 disabled:text-gray-400 disabled:border-white disabled:bg-gray-100 disabled:dark:border-gray-500 disabled:dark:bg-gray-600";
		}
	});
</script>

<button
	{type}
	class={`${baseClasses} ${shapeClasses} ${sizeClasses} ${variantClasses} ${className}`}
	{...rest}
>
	{@render children?.()}
</button>
