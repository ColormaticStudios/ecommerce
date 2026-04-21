<script lang="ts">
	import type { HTMLSelectAttributes } from "svelte/elements";

	type DropdownTone = "filled" | "plain" | "surface" | "admin";

	interface Props extends Omit<HTMLSelectAttributes, "value"> {
		full?: boolean;
		tone?: DropdownTone;
		class?: string;
		value?: unknown;
		children?: import("svelte").Snippet;
	}

	let {
		full = true,
		tone = "filled",
		class: className = "",
		value = $bindable(),
		children,
		...rest
	}: Props = $props();

	const baseClasses = $derived.by(() => {
		switch (tone) {
			case "admin":
				return "cursor-pointer rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm text-stone-900 transition outline-none focus:border-stone-500 focus:ring-2 focus:ring-stone-200 dark:border-stone-700 dark:bg-stone-900 dark:text-stone-100 dark:focus:border-stone-500 dark:focus:ring-stone-800";
			case "plain":
				return "cursor-pointer rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100";
			case "surface":
				return "cursor-pointer rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700 shadow-sm dark:border-gray-800 dark:bg-gray-900 dark:text-gray-200";
			default:
				return "cursor-pointer rounded-md border border-gray-300 bg-gray-200 px-3 py-2 text-sm text-gray-900 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100";
		}
	});

	const widthClasses = $derived(full ? "w-full" : "");
</script>

<select bind:value class={`${baseClasses} ${widthClasses} ${className}`.trim()} {...rest}>
	{@render children?.()}
</select>
