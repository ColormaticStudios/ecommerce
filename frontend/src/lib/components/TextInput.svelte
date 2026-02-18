<script lang="ts">
	import type { HTMLInputAttributes } from "svelte/elements";

	interface Props extends HTMLInputAttributes {
		full?: boolean;
		class?: string;
		value?: string | number | undefined;
	}

	let { full = true, class: className = "", value = $bindable(), ...rest }: Props = $props();

	const baseClasses =
		"px-3 py-2 rounded-md border border-gray-300 bg-gray-200 dark:border-gray-700 dark:bg-gray-800";

	const widthClasses = $derived(full ? "w-full" : "w-min");
</script>

<input bind:value class={`${baseClasses} ${widthClasses} ${className}`} {...rest} />
