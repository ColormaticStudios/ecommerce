<script lang="ts">
	import AdminFieldLabel from "$lib/admin/AdminFieldLabel.svelte";
	import TextArea from "$lib/components/TextArea.svelte";
	import TextInput from "$lib/components/TextInput.svelte";

	let {
		name = $bindable(),
		subtitle = $bindable(),
		sku = $bindable(),
		description = $bindable(),
	}: {
		name: string;
		subtitle: string;
		sku: string;
		description: string;
	} = $props();
</script>

<div>
	<AdminFieldLabel as="label" for="admin-product-name">Name</AdminFieldLabel>
	<TextInput
		tone="admin"
		id="admin-product-name"
		name="name"
		class="mt-1"
		type="text"
		bind:value={name}
	/>
</div>
<div>
	<AdminFieldLabel as="label" for="admin-product-subtitle">Subtitle</AdminFieldLabel>
	<TextInput
		tone="admin"
		id="admin-product-subtitle"
		name="subtitle"
		class="mt-1"
		type="text"
		bind:value={subtitle}
	/>
</div>
<div>
	<AdminFieldLabel as="label" for="admin-product-sku">SKU</AdminFieldLabel>
	<TextInput
		tone="admin"
		id="admin-product-sku"
		name="sku"
		class="mt-1"
		type="text"
		bind:value={sku}
	/>
</div>
<div class="sm:col-span-2">
	<AdminFieldLabel as="label" for="admin-product-description">Description</AdminFieldLabel>
	<TextArea
		tone="admin"
		id="admin-product-description"
		name="description"
		class="mt-1"
		rows={4}
		bind:value={description}
	/>
</div>
