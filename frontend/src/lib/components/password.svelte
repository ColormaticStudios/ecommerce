<script lang="ts">
	interface Props {
		value: string;
		name: string;
		placeholder: string;
	}
	let { value = $bindable(), name, placeholder }: Props = $props();

	let hidden = $state(true);
</script>

<div class="flex w-full">
	<input
		class="textinput"
		bind:value
		type={hidden ? "password" : "text"}
		{name}
		{placeholder}
		required
	/>
	<button
		class="textinput not-full ml-2 cursor-pointer"
		type="button"
		onclick={() => {
			hidden = !hidden;
		}}
		aria-label="Show/hide password"
	>
		<i class="bi {hidden ? 'bi-eye-fill' : 'bi-eye-slash-fill'}"></i>
	</button>
</div>
