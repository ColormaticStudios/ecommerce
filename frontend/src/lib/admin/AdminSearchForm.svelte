<script lang="ts">
	import IconButton from "$lib/components/IconButton.svelte";
	import TextInput from "$lib/components/TextInput.svelte";

	interface Props {
		value?: string;
		placeholder?: string;
		disabled?: boolean;
		fullWidth?: boolean;
		class?: string;
		inputClass?: string;
		onSearch?: () => void;
		onRefresh?: () => void;
		refreshing?: boolean;
	}

	let {
		value = $bindable(""),
		placeholder = "Search",
		disabled = false,
		fullWidth = false,
		class: className = "",
		inputClass = "",
		onSearch,
		onRefresh,
		refreshing = false,
	}: Props = $props();
</script>

<form
	class={`flex flex-wrap items-center gap-2 ${fullWidth ? "flex-1" : ""} ${className}`}
	onsubmit={(event) => {
		event.preventDefault();
		onSearch?.();
	}}
>
	<TextInput
		tone="admin"
		type="search"
		bind:value
		{placeholder}
		class={`min-w-[13rem] flex-1 ${inputClass}`}
	/>
	<IconButton
		tone="admin"
		type="submit"
		outlined={true}
		aria-label="Search"
		title="Search"
		class="border-stone-300 bg-white/85 text-stone-700 hover:bg-stone-100 dark:border-stone-700 dark:bg-stone-950/80 dark:text-stone-200 dark:hover:bg-stone-900"
		{disabled}
	>
		<i class="bi bi-search"></i>
	</IconButton>
	<IconButton
		tone="admin"
		type="button"
		outlined={true}
		aria-label="Refresh"
		title="Refresh"
		class="border-stone-300 bg-white/85 text-stone-700 hover:bg-stone-100 dark:border-stone-700 dark:bg-stone-950/80 dark:text-stone-200 dark:hover:bg-stone-900"
		{disabled}
		onclick={() => onRefresh?.()}
	>
		<i class={`bi ${refreshing ? "bi-arrow-repeat animate-spin" : "bi-arrow-clockwise"}`}></i>
	</IconButton>
</form>
