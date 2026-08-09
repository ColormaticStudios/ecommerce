<script lang="ts">
	import AdminFieldLabel from "$lib/admin/AdminFieldLabel.svelte";
	import AdminMetaText from "$lib/admin/AdminMetaText.svelte";
	import { adminDividerBottomClass } from "$lib/admin/tokens";
	import Alert from "$lib/components/Alert.svelte";
	import Badge from "$lib/components/Badge.svelte";
	import Button from "$lib/components/Button.svelte";
	import type { EditorLayout } from "./types";

	let {
		layout,
		canEdit,
		defaultVariantSku,
		priceRange,
		isPublished,
		hasDraftChanges,
		hasUnsavedChanges,
		saving,
		publishing,
		unpublishing,
		discarding,
		previewing,
		previewActive,
		deleting,
		errorMessage,
		statusMessage,
		onClearError,
		onClearStatus,
		onSave,
		onPreview,
		onPublish,
		onUnpublish,
		onDiscard,
		onDelete,
	}: {
		layout: EditorLayout;
		canEdit: boolean;
		defaultVariantSku: string;
		priceRange: string;
		isPublished: boolean;
		hasDraftChanges: boolean;
		hasUnsavedChanges: boolean;
		saving: boolean;
		publishing: boolean;
		unpublishing: boolean;
		discarding: boolean;
		previewing: boolean;
		previewActive: boolean;
		deleting: boolean;
		errorMessage: string;
		statusMessage: string;
		onClearError: () => void;
		onClearStatus: () => void;
		onSave: () => void;
		onPreview: () => void;
		onPublish: () => void;
		onUnpublish: () => void;
		onDiscard: () => void;
		onDelete: () => void;
	} = $props();
	const stacked = $derived(layout === "stacked");
</script>

<div class="grid gap-4 sm:grid-cols-2">
	<div>
		<AdminFieldLabel>Default variant</AdminFieldLabel><AdminMetaText tone="strong" class="mt-1"
			>{defaultVariantSku || "No default variant selected"}</AdminMetaText
		>
	</div>
	<div>
		<AdminFieldLabel>Price range preview</AdminFieldLabel><AdminMetaText tone="strong" class="mt-1"
			>{priceRange}</AdminMetaText
		>
	</div>
</div>
{#if canEdit}<div class="mt-1 flex flex-wrap items-center gap-2 text-xs">
		<Badge tone={isPublished ? "success" : "warning"} size="sm"
			>{isPublished ? "Published" : "Unpublished"}</Badge
		>{#if hasDraftChanges}<Badge tone="info" size="sm">Draft changes</Badge>{/if}
	</div>{/if}
<div
	class={stacked
		? `${adminDividerBottomClass} mt-2 mb-6 grid grid-cols-1 gap-2 text-base sm:grid-cols-2`
		: "mt-6 flex flex-wrap items-center gap-2"}
>
	<Button
		tone="admin"
		variant="primary"
		size={stacked ? "large" : "regular"}
		class={stacked ? `w-full ${canEdit ? "" : "sm:col-span-2"}` : "min-w-40"}
		type="button"
		onclick={onSave}
		disabled={saving}
		><i class={`bi ${stacked && !canEdit ? "bi-patch-plus-fill" : "bi-floppy-fill"} mr-1`}
		></i>{saving ? "Saving..." : stacked && !canEdit ? "Create draft" : "Save draft"}</Button
	>
	{#if canEdit}
		<Button
			tone="admin"
			variant="regular"
			size={stacked ? "large" : "regular"}
			class={stacked ? "w-full" : ""}
			type="button"
			disabled={previewing}
			onclick={onPreview}
			><i class={`bi ${previewActive ? "bi-eye-slash-fill" : "bi-eye-fill"} mr-1`}></i>{previewing
				? previewActive
					? "Exiting..."
					: "Opening..."
				: previewActive
					? "Exit preview"
					: "Preview"}</Button
		>
		<Button
			tone="admin"
			variant="success"
			size={stacked ? "large" : "regular"}
			class={stacked ? "w-full" : ""}
			type="button"
			disabled={publishing || (!hasDraftChanges && !hasUnsavedChanges)}
			onclick={onPublish}
			><i class="bi bi-send-check-fill mr-1"></i>{publishing ? "Publishing..." : "Publish"}</Button
		>
		<Button
			tone="admin"
			variant="warning"
			size={stacked ? "large" : "regular"}
			class={stacked ? "w-full" : ""}
			type="button"
			disabled={unpublishing || !isPublished}
			onclick={onUnpublish}
			><i class="bi bi-eye-slash-fill mr-1"></i>{unpublishing
				? "Unpublishing..."
				: "Unpublish"}</Button
		>
		<Button
			tone="admin"
			variant="warning"
			size={stacked ? "large" : "regular"}
			class={stacked ? "w-full" : ""}
			type="button"
			disabled={discarding || (!hasDraftChanges && !hasUnsavedChanges)}
			onclick={onDiscard}
			><i class="bi bi-arrow-counterclockwise mr-1"></i>{discarding
				? "Discarding..."
				: "Discard draft"}</Button
		>
		<Button
			tone="admin"
			variant="danger"
			size={stacked ? "large" : "regular"}
			class={stacked ? "w-full" : ""}
			type="button"
			disabled={deleting}
			onclick={onDelete}
			><i class="bi bi-trash-fill mr-1"></i>{deleting ? "Deleting..." : "Delete"}</Button
		>
	{/if}
</div>
{#if errorMessage}<div class={stacked ? "mb-4" : "mt-4"}>
		<Alert message={errorMessage} tone="error" icon="bi-x-circle-fill" onClose={onClearError} />
	</div>{/if}
{#if statusMessage}<div class={stacked ? "mb-4" : "mt-4"}>
		<Alert
			message={statusMessage}
			tone="success"
			icon="bi-check-circle-fill"
			onClose={onClearStatus}
		/>
	</div>{/if}
