<script lang="ts">
	import Button from "$lib/components/Button.svelte";
	import TextInput from "$lib/components/TextInput.svelte";

	interface Props {
		value: string;
		name: string;
		placeholder: string;
	}
	let { value = $bindable(), name, placeholder }: Props = $props();

	let hidden = $state(true);
</script>

<div class="flex w-full">
	<TextInput bind:value type={hidden ? "password" : "text"} {name} {placeholder} required />
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
