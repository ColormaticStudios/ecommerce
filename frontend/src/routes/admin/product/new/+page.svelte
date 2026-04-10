<script lang="ts">
	import AdminProductEditorRoute from "$lib/admin/AdminProductEditorRoute.svelte";
	import type { PageData } from "./$types";

	interface Props {
		data: PageData;
	}

	let { data }: Props = $props();
</script>

<AdminProductEditorRoute productId={null} initialProduct={data.initialProduct} allowCreate={true} />
