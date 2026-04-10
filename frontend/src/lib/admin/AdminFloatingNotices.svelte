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
	const statusContainerClass = $derived.by(() => {
		if (statusTone === "success") {
			return "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/80 dark:bg-emerald-950/50 dark:text-emerald-200";
		}
		return "border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900/80 dark:bg-rose-950/50 dark:text-rose-200";
	});
	const statusButtonClass = $derived.by(() => {
		if (statusTone === "success") {
			return "h-7 w-7 bg-transparent text-emerald-700 hover:bg-emerald-100/80 dark:text-emerald-100 dark:hover:bg-emerald-900/60";
		}
		return "h-7 w-7 bg-transparent text-rose-700 hover:bg-rose-100/80 dark:text-rose-100 dark:hover:bg-rose-900/60";
	});
</script>

{#if hasStatus || showUnsaved}
	<div
		class="pointer-events-none fixed bottom-4 left-1/2 z-50 flex -translate-x-1/2 flex-col items-center gap-2"
	>
		{#if hasStatus}
			<div
				class={`pointer-events-auto flex items-center gap-2 rounded-full border px-2 py-1.5 text-xs font-semibold shadow-sm ${statusContainerClass}`}
			>
				<span class="ml-1 inline-flex min-w-0 items-center gap-1.5">
					<i class={`bi ${statusTone === "success" ? "bi-check-circle-fill" : "bi-x-circle-fill"}`}
					></i>
					<span class="min-w-0 break-words">{statusMessage}</span>
				</span>
				<IconButton
					variant={statusTone === "success" ? "success" : "danger"}
					size="sm"
					outlined={false}
					type="button"
					class={`shrink-0 ${statusButtonClass}`}
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
