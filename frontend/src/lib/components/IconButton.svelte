<script lang="ts">
	import type { HTMLButtonAttributes } from "svelte/elements";

	type IconButtonVariant = "neutral" | "danger" | "primary";
	type IconButtonSize = "sm" | "md" | "lg";

	interface Props extends HTMLButtonAttributes {
		variant?: IconButtonVariant;
		size?: IconButtonSize;
		outlined?: boolean;
		class?: string;
		children?: import("svelte").Snippet;
	}

	let {
		variant = "neutral",
		size = "md",
		outlined = false,
		class: className = "",
		type = "button",
		children,
		...rest
	}: Props = $props();

	const baseClasses =
		"inline-flex shrink-0 cursor-pointer items-center justify-center rounded-full bg-transparent transition-colors duration-200 focus-visible:outline-2 focus-visible:outline-offset-2 disabled:cursor-not-allowed disabled:opacity-50";

	const sizeClasses = $derived.by(() => {
		switch (size) {
			case "sm":
				return "h-8 w-8 text-sm";
			case "lg":
				return "h-11 w-11 text-lg";
			default:
				return "h-9 w-9 text-base";
		}
	});

	const outlineClasses = $derived.by(() => {
		if (!outlined) {
			return "";
		}
		switch (variant) {
			case "danger":
				return "border border-red-300 dark:border-red-700";
			case "primary":
				return "border border-blue-300 dark:border-blue-700";
			default:
				return "border border-gray-300 dark:border-gray-700";
		}
	});

	const variantClasses = $derived.by(() => {
		switch (variant) {
			case "danger":
				return "text-red-600 hover:bg-red-100 focus-visible:outline-red-500 dark:text-red-300 dark:hover:bg-red-900/50";
			case "primary":
				return "text-blue-600 hover:bg-blue-100 focus-visible:outline-blue-500 dark:text-blue-300 dark:hover:bg-blue-900/50";
			default:
				return "text-gray-700 hover:bg-gray-100 focus-visible:outline-gray-500 dark:text-gray-200 dark:hover:bg-gray-800";
		}
	});
</script>

<button
	{type}
	class={`${baseClasses} ${sizeClasses} ${outlineClasses} ${variantClasses} ${className}`}
	{...rest}
>
	{@render children?.()}
</button>
