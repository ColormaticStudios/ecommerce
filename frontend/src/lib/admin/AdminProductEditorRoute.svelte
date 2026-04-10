<script lang="ts">
	import { resolve } from "$app/paths";
	import AdminFloatingNotices from "$lib/admin/AdminFloatingNotices.svelte";
	import AdminPageHeader from "$lib/admin/AdminPageHeader.svelte";
	import { createAdminNotices, createAdminSavePrompt } from "$lib/admin/state.svelte";
	import ProductEditor from "$lib/admin/ProductEditor.svelte";
	import ButtonLink from "$lib/components/ButtonLink.svelte";
	import type { ProductModel } from "$lib/models";

	interface Props {
		productId?: number | null;
		initialProduct?: ProductModel | null;
		allowCreate?: boolean;
		onProductCreated?: (product: ProductModel) => void;
	}

	let {
		productId = null,
		initialProduct = null,
		allowCreate = false,
		onProductCreated,
	}: Props = $props();

	const hasProductId = $derived(productId != null && Number.isFinite(productId) && productId > 0);
	const canViewLive = $derived(Boolean(initialProduct?.is_published));

	let productDirty = $state(false);
	let productSaveAction = $state<(() => Promise<void>) | null>(null);
	const notices = createAdminNotices();
	const savePrompt = createAdminSavePrompt({
		onSaveError: () => notices.pushError("Unable to save pending changes."),
		navigationMessage: "You have unsaved product changes. Leave this section and discard them?",
	});

	function setErrorMessage(message: string) {
		notices.pushError(message);
	}

	function setStatusMessage(message: string) {
		notices.pushSuccess(message);
	}

	$effect(() => {
		savePrompt.dirty = productDirty;
		savePrompt.saveAction = productSaveAction;
	});
</script>

{#snippet productActions()}
	<ButtonLink href={resolve("/admin/products")} variant="regular" tone="admin" class="rounded-full">
		Back to products
	</ButtonLink>
	{#if hasProductId && canViewLive}
		<ButtonLink
			href={resolve(`/product/${productId}`)}
			variant="regular"
			tone="admin"
			class="rounded-full"
		>
			View live
		</ButtonLink>
	{/if}
{/snippet}

<section class="space-y-6">
	<AdminPageHeader title="Product Editor" actions={productActions} />

	<ProductEditor
		{productId}
		{initialProduct}
		{allowCreate}
		{onProductCreated}
		layout="split"
		showHeader={false}
		showClear={false}
		showMessages={false}
		onErrorMessage={setErrorMessage}
		onStatusMessage={setStatusMessage}
		onDirtyChange={(dirty) => (productDirty = dirty)}
		onSaveRequestChange={(action) => (productSaveAction = action)}
	/>
</section>

<AdminFloatingNotices
	showUnsaved={savePrompt.dirty}
	unsavedMessage="You have unsaved product changes."
	canSaveUnsaved={savePrompt.canSave}
	onSaveUnsaved={() => void savePrompt.save()}
	savingUnsaved={savePrompt.saving}
	statusMessage={notices.message}
	statusTone={notices.tone}
	onDismissStatus={notices.clear}
/>
