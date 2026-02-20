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
	class={`flex items-center gap-2 ${fullWidth ? "flex-1" : ""} ${className}`}
	onsubmit={(event) => {
		event.preventDefault();
		onSearch?.();
	}}
>
	<TextInput type="search" bind:value {placeholder} class={inputClass} />
	<IconButton type="submit" outlined={true} aria-label="Search" title="Search" {disabled}>
		<i class="bi bi-search"></i>
	</IconButton>
	<IconButton
		type="button"
		outlined={true}
		aria-label="Refresh"
		title="Refresh"
		{disabled}
		onclick={() => onRefresh?.()}
	>
		<i class={`bi ${refreshing ? "bi-arrow-repeat animate-spin" : "bi-arrow-clockwise"}`}></i>
	</IconButton>
</form>
