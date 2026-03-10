<script lang="ts">
	import type { HTMLTextareaAttributes } from "svelte/elements";

	type TextAreaTone = "filled" | "surface" | "admin";

	interface Props extends Omit<HTMLTextareaAttributes, "value"> {
		full?: boolean;
		tone?: TextAreaTone;
		class?: string;
		value?: string | number | undefined;
	}

	let {
		full = true,
		tone = "filled",
		class: className = "",
		value = $bindable(),
		...rest
	}: Props = $props();

	const baseClasses = $derived.by(() => {
		switch (tone) {
			case "admin":
				return "admin-input";
			case "surface":
				return "rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700 shadow-sm dark:border-gray-800 dark:bg-gray-900 dark:text-gray-200";
			default:
				return "rounded-md border border-gray-300 bg-gray-200 px-3 py-2 text-sm text-gray-900 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100";
		}
	});

	const widthClasses = $derived(full ? "w-full" : "");
</script>

<textarea bind:value class={`${baseClasses} ${widthClasses} ${className}`.trim()} {...rest}
></textarea>
