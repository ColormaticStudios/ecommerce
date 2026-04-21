<script lang="ts">
	import type { HTMLInputAttributes } from "svelte/elements";

	type InputTone = "default" | "admin";

	interface Props extends HTMLInputAttributes {
		full?: boolean;
		tone?: InputTone;
		class?: string;
		value?: string | number | undefined;
	}

	let {
		full = true,
		tone = "default",
		class: className = "",
		value = $bindable(),
		...rest
	}: Props = $props();

	const baseClasses = $derived(
		tone === "admin"
			? "rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm text-stone-900 transition outline-none focus:border-stone-500 focus:ring-2 focus:ring-stone-200 dark:border-stone-700 dark:bg-stone-900 dark:text-stone-100 dark:focus:border-stone-500 dark:focus:ring-stone-800"
			: "px-3 py-2 rounded-md border border-gray-300 bg-gray-200 dark:border-gray-700 dark:bg-gray-800"
	);

	const widthClasses = $derived(full ? "w-full" : "w-min");
</script>

<input bind:value class={`${baseClasses} ${widthClasses} ${className}`} {...rest} />
