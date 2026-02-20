<script lang="ts">
	import type { HTMLInputAttributes } from "svelte/elements";

	type ButtonVariant = "regular" | "primary" | "success" | "warning" | "danger";
	type ButtonSize = "small" | "regular" | "large";
	type ButtonStyle = "regular" | "pill";

	interface Props extends Omit<HTMLInputAttributes, "size"> {
		variant?: ButtonVariant;
		size?: ButtonSize;
		style?: ButtonStyle;
		class?: string;
		inputClass?: string;
		children?: import("svelte").Snippet;
	}

	let {
		variant = "regular",
		size = "regular",
		style = "regular",
		class: className = "",
		inputClass = "hidden",
		disabled = false,
		children,
		...rest
	}: Props = $props();

	const baseClasses =
		"inline-flex cursor-pointer items-center justify-center transition-[background-color,border-color] duration-200";
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
	const disabledClasses = $derived(
		disabled ? "cursor-auto opacity-60 pointer-events-none select-none" : ""
	);

	const variantClasses = $derived.by(() => {
		switch (variant) {
			case "primary":
				return "text-white border border-blue-400 bg-blue-500 hover:border-blue-500 hover:bg-blue-600";
			case "success":
				return "text-white border border-emerald-400 bg-emerald-500 hover:border-emerald-500 hover:bg-emerald-600";
			case "warning":
				return "text-white border border-orange-400 bg-orange-500 hover:border-orange-300 hover:bg-orange-400";
			case "danger":
				return "text-white border border-red-400 bg-red-500 hover:border-red-300 hover:bg-red-400";
			default:
				return "border border-gray-300 bg-gray-200 hover:border-gray-200 hover:bg-gray-100 dark:border-gray-600 dark:bg-gray-700 hover:dark:border-gray-700 hover:dark:bg-gray-800";
		}
	});
</script>

<label
	class={`${baseClasses} ${shapeClasses} ${sizeClasses} ${variantClasses} ${disabledClasses} ${className}`}
>
	<input class={inputClass} {disabled} {...rest} />
	{@render children?.()}
</label>
