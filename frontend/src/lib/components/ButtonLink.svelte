<script lang="ts">
	import type { HTMLAnchorAttributes } from "svelte/elements";

	type ButtonVariant = "regular" | "primary" | "warning" | "danger";
	type ButtonSize = "regular" | "large";

	interface Props extends HTMLAnchorAttributes {
		variant?: ButtonVariant;
		size?: ButtonSize;
		class?: string;
		children?: import("svelte").Snippet;
	}

	let {
		variant = "regular",
		size = "regular",
		class: className = "",
		children,
		...rest
	}: Props = $props();

	const baseClasses =
		"cursor-pointer rounded-lg px-4 py-2 transition-[background-color,border-color] duration-200";
	const sizeClasses = $derived(size === "large" ? "text-lg" : "");

	const variantClasses = $derived.by(() => {
		switch (variant) {
			case "primary":
				return "text-white border border-blue-400 bg-blue-500 hover:border-blue-500 hover:bg-blue-600";
			case "warning":
				return "text-white border border-orange-400 bg-orange-500 hover:border-orange-300 hover:bg-orange-400";
			case "danger":
				return "text-white border border-red-400 bg-red-500 hover:border-red-300 hover:bg-red-400";
			default:
				return "border border-gray-300 bg-gray-200 hover:border-gray-200 hover:bg-gray-100 dark:border-gray-600 dark:bg-gray-700 hover:dark:border-gray-700 hover:dark:bg-gray-800";
		}
	});
</script>

<a class={`${baseClasses} ${sizeClasses} ${variantClasses} ${className}`} {...rest}>
	{@render children?.()}
</a>
