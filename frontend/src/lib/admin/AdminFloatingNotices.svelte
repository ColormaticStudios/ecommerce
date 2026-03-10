<script lang="ts">
	import AdminBadge from "$lib/admin/AdminBadge.svelte";
	import Button from "$lib/components/Button.svelte";
	import IconButton from "$lib/components/IconButton.svelte";

	type NoticeTone = "success" | "error" | null;

	interface Props {
		showUnsaved?: boolean;
		unsavedMessage?: string;
		canSaveUnsaved?: boolean;
		onSaveUnsaved?: (() => void) | undefined;
		savingUnsaved?: boolean;
		statusMessage?: string;
		statusTone?: NoticeTone;
		onDismissStatus?: (() => void) | undefined;
	}

	let {
		showUnsaved = false,
		unsavedMessage = "You have unsaved changes.",
		canSaveUnsaved = false,
		onSaveUnsaved,
		savingUnsaved = false,
		statusMessage = "",
		statusTone = null,
		onDismissStatus,
	}: Props = $props();

	const hasStatus = $derived(statusMessage.trim().length > 0 && statusTone !== null);
</script>

{#if hasStatus || showUnsaved}
	<div
		class="pointer-events-none fixed bottom-4 left-1/2 z-50 flex -translate-x-1/2 flex-col items-center gap-2"
	>
		{#if hasStatus}
			<div class="pointer-events-auto flex items-center gap-2 rounded-full shadow-sm">
				<AdminBadge tone={statusTone === "success" ? "success" : "danger"} size="md">
					<i class={`bi ${statusTone === "success" ? "bi-check-circle-fill" : "bi-x-circle-fill"}`}
					></i>
					<span>{statusMessage}</span>
				</AdminBadge>
				<IconButton
					tone="admin"
					size="sm"
					type="button"
					variant="neutral"
					class="bg-white/85 text-stone-600 shadow-sm dark:bg-stone-950/85 dark:text-stone-200"
					onclick={() => onDismissStatus?.()}
					aria-label="Dismiss status message"
					title="Dismiss"
				>
					<i class="bi bi-x-lg"></i>
				</IconButton>
			</div>
		{/if}

		{#if showUnsaved}
			<div
				class="pointer-events-auto flex items-center gap-2 rounded-full border border-stone-300 bg-white/95 px-2 py-1.5 text-xs text-stone-700 shadow-sm dark:border-stone-700 dark:bg-stone-950/95 dark:text-stone-200"
			>
				<span class="ml-1">{unsavedMessage}</span>
				<Button
					tone="admin"
					variant="regular"
					size="small"
					style="pill"
					type="button"
					class="rounded-full"
					disabled={!canSaveUnsaved || savingUnsaved}
					onclick={() => onSaveUnsaved?.()}
				>
					<i class="bi bi-floppy-fill mr-1"></i>
					{savingUnsaved ? "Saving..." : "Save"}
				</Button>
			</div>
		{/if}
	</div>
{/if}
