<script lang="ts">
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import AdminProductEditorRoute from "$lib/admin/AdminProductEditorRoute.svelte";
	import type { ProductModel } from "$lib/models";
	import type { PageData } from "./$types";

	interface Props {
		data: PageData;
	}

	let { data }: Props = $props();

	function handleProductCreated(product: ProductModel) {
		void goto(resolve(`/admin/product/${product.id}`), { replaceState: true });
	}
</script>

<AdminProductEditorRoute
	productId={null}
	initialProduct={data.initialProduct}
	allowCreate={true}
	onProductCreated={handleProductCreated}
/>
