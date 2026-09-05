<script lang="ts">
	import Button from "$lib/components/Button.svelte";
	import TextInput from "$lib/components/TextInput.svelte";

	interface Props {
		value?: string;
		name: string;
		id?: string;
		autocomplete?: "current-password" | "new-password";
		required?: boolean;
		class?: string;
	}
	let {
		value = $bindable(),
		name,
		id,
		autocomplete,
		required = false,
		class: className = "",
	}: Props = $props();

	let hidden = $state(true);
</script>

<div class="flex w-full">
	<TextInput
		bind:value
		type={hidden ? "password" : "text"}
		{name}
		{id}
		{autocomplete}
		{required}
		class={className}
	/>
	<Button
		variant="regular"
		class="ml-2 px-3!"
		type="button"
		onclick={() => {
			hidden = !hidden;
		}}
		aria-label="Show/hide password"
	>
		<i class="bi {hidden ? 'bi-eye-fill' : 'bi-eye-slash-fill'}"></i>
	</Button>
</div>
