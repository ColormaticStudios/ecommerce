<script lang="ts">
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
			<div
				class={`pointer-events-auto flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs shadow-sm ${
					statusTone === "success"
						? "border-emerald-300 bg-emerald-50 text-emerald-800 dark:border-emerald-700 dark:bg-emerald-950 dark:text-emerald-200"
						: "border-red-300 bg-red-50 text-red-800 dark:border-red-700 dark:bg-red-950 dark:text-red-200"
				}`}
			>
				<i class={`bi ${statusTone === "success" ? "bi-check-circle-fill" : "bi-x-circle-fill"}`}
				></i>
				<span>{statusMessage}</span>
				<IconButton
					size="sm"
					type="button"
					variant="neutral"
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
				class="pointer-events-auto flex items-center gap-2 rounded-full border border-gray-300 bg-gray-100 px-2 py-1.5 text-xs text-gray-700 shadow-sm dark:border-gray-700 dark:bg-gray-900 dark:text-gray-200"
			>
				<span class="ml-1">{unsavedMessage}</span>
				<Button
					variant="regular"
					size="small"
					style="pill"
					type="button"
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
