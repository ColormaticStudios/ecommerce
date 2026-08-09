<script lang="ts">
	import AdminFieldLabel from "$lib/admin/AdminFieldLabel.svelte";
	import { adminSurfaceVariantClasses } from "$lib/admin/tokens";
	import Alert from "$lib/components/Alert.svelte";
	import Button from "$lib/components/Button.svelte";
	import IconButton from "$lib/components/IconButton.svelte";
	import { extractMediaId } from "./media";
	import type { EditorLayout } from "./types";

	let {
		layout,
		images,
		productName,
		canEdit,
		files = $bindable(),
		uploading,
		deletingMediaId,
		reordering,
		hasPendingOrder,
		showMessages,
		errorMessage,
		statusMessage,
		showUploadHint = false,
		showImages = true,
		showUpload = true,
		onUpload,
		onDetach,
		onMove,
		onSaveOrder,
		onDiscardOrder,
		onClearError,
		onClearStatus,
	}: {
		layout: EditorLayout;
		images: string[];
		productName: string;
		canEdit: boolean;
		files: FileList | null;
		uploading: boolean;
		deletingMediaId: string | null;
		reordering: boolean;
		hasPendingOrder: boolean;
		showMessages: boolean;
		errorMessage: string;
		statusMessage: string;
		showUploadHint?: boolean;
		showImages?: boolean;
		showUpload?: boolean;
		onUpload: () => void;
		onDetach: (url: string) => void;
		onMove: (index: number, direction: -1 | 1) => void;
		onSaveOrder: () => void;
		onDiscardOrder: () => void;
		onClearError: () => void;
		onClearStatus: () => void;
	} = $props();
	let input = $state<HTMLInputElement | null>(null);
	const fileCount = $derived(files?.length ?? 0);
	const mediaClass = $derived(
		layout === "split"
			? adminSurfaceVariantClasses.media
			: "relative overflow-hidden rounded-[1rem]"
	);
</script>

{#if showImages && images.length}
	<div class="max-h-64 overflow-y-auto pr-1">
		<div class="grid grid-cols-2 gap-3">
			{#each images as image, index (image)}<div class={`${mediaClass} relative overflow-hidden`}>
					<img
						src={image}
						alt={productName ? `${productName} ${index + 1}` : `Product image ${index + 1}`}
						class="h-42 w-full object-cover"
					/><IconButton
						tone="admin"
						class="absolute top-2 right-2 border border-stone-300 bg-white/95 shadow-sm backdrop-blur-sm hover:bg-stone-50 disabled:opacity-45 dark:border-stone-700 dark:bg-stone-950/85 dark:hover:bg-stone-900"
						size="sm"
						disabled={deletingMediaId !== null || reordering}
						onclick={() => onDetach(image)}
						aria-label="Remove image"
						title="Remove image"
						variant="danger"
						>{#if deletingMediaId && extractMediaId(image, typeof window === "undefined" ? "http://localhost" : window.location.origin) === deletingMediaId}<i
								class="bi bi-arrow-repeat inline-block animate-spin"
							></i>{:else}<i class="bi bi-trash"></i>{/if}</IconButton
					>
					<div class="absolute right-2 bottom-2 flex gap-1">
						<IconButton
							tone="admin"
							class="h-5 w-5 border border-stone-300 bg-white/95 text-[10px] shadow-sm backdrop-blur-sm hover:bg-stone-50 disabled:opacity-45 dark:border-stone-700 dark:bg-stone-950/85 dark:hover:bg-stone-900"
							size="sm"
							disabled={reordering || index === 0}
							onclick={() => onMove(index, -1)}
							aria-label="Move image left"
							title="Move image left"><i class="bi bi-chevron-left"></i></IconButton
						><IconButton
							tone="admin"
							class="h-5 w-5 border border-stone-300 bg-white/95 text-[10px] shadow-sm backdrop-blur-sm hover:bg-stone-50 disabled:opacity-45 dark:border-stone-700 dark:bg-stone-950/85 dark:hover:bg-stone-900"
							size="sm"
							disabled={reordering || index === images.length - 1}
							onclick={() => onMove(index, 1)}
							aria-label="Move image right"
							title="Move image right"><i class="bi bi-chevron-right"></i></IconButton
						>
					</div>
				</div>{/each}
		</div>
	</div>
	{#if hasPendingOrder}<div class="mt-3 flex flex-wrap gap-2">
			<Button
				tone="admin"
				variant="primary"
				type="button"
				disabled={reordering}
				onclick={onSaveOrder}
				><i class="bi bi-floppy-fill mr-1"></i>{reordering ? "Saving..." : "Save order"}</Button
			><Button
				tone="admin"
				variant="regular"
				type="button"
				disabled={reordering}
				onclick={onDiscardOrder}><i class="bi bi-x-circle mr-1"></i>Discard changes</Button
			>
		</div>{/if}
{/if}
{#if showUpload}
	<div class={layout === "split" ? `${adminSurfaceVariantClasses.muted} mt-6` : ""}>
		<AdminFieldLabel>Upload media</AdminFieldLabel><input
			class="hidden"
			type="file"
			accept="image/*"
			multiple
			bind:this={input}
			onchange={(event) => (files = (event.target as HTMLInputElement).files)}
			disabled={!canEdit}
		/>
		<div class="mt-3 flex flex-wrap items-center gap-2">
			<Button
				tone="admin"
				variant="regular"
				type="button"
				disabled={!canEdit || uploading}
				onclick={() => input?.click()}>Choose files</Button
			><Button
				tone="admin"
				variant="primary"
				type="button"
				disabled={!canEdit || uploading || !fileCount}
				onclick={onUpload}>{uploading ? "Uploading..." : "Attach uploads"}</Button
			>{#if fileCount}<span class="text-xs text-gray-500 dark:text-gray-400"
					>{fileCount} selected</span
				>{/if}
		</div>
		{#if showUploadHint && !canEdit}<p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
				Select a product to upload images.
			</p>{/if}
	</div>
{/if}
{#if showMessages && errorMessage}<div class="mt-4">
		<Alert message={errorMessage} tone="error" icon="bi-x-circle-fill" onClose={onClearError} />
	</div>{/if}
{#if showMessages && statusMessage}<div class="mt-4">
		<Alert
			message={statusMessage}
			tone="success"
			icon="bi-check-circle-fill"
			onClose={onClearStatus}
		/>
	</div>{/if}
