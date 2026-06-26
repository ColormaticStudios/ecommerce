<script lang="ts">
	import Button from "$lib/components/Button.svelte";

	interface Props {
		title: string;
		message: string;
		confirmLabel: string;
		busyLabel?: string;
		busy?: boolean;
		onConfirm: () => void;
		onCancel: () => void;
	}

	let {
		title,
		message,
		confirmLabel,
		busyLabel = confirmLabel,
		busy = false,
		onConfirm,
		onCancel,
	}: Props = $props();
</script>

<div class="fixed inset-0 z-100 flex items-center justify-center p-4" role="presentation">
	<button
		type="button"
		class="absolute inset-0 bg-stone-950/55"
		aria-label="Cancel"
		disabled={busy}
		onclick={onCancel}
	></button>
	<div
		class="relative w-full max-w-md rounded-lg border border-stone-200 bg-white p-5 shadow-2xl dark:border-stone-700 dark:bg-stone-950"
		role="alertdialog"
		tabindex="-1"
		aria-modal="true"
		aria-labelledby="admin-confirm-title"
		aria-describedby="admin-confirm-message"
		onkeydown={(event) => {
			if (event.key === "Escape" && !busy) onCancel();
		}}
	>
		<h2 id="admin-confirm-title" class="text-base font-semibold text-stone-950 dark:text-stone-50">
			{title}
		</h2>
		<p id="admin-confirm-message" class="mt-2 text-sm text-stone-600 dark:text-stone-300">
			{message}
		</p>
		<div class="mt-6 flex justify-end gap-2">
			<Button autofocus tone="admin" variant="regular" disabled={busy} onclick={onCancel}>
				Cancel
			</Button>
			<Button tone="admin" variant="danger" disabled={busy} onclick={onConfirm}>
				{busy ? busyLabel : confirmLabel}
			</Button>
		</div>
	</div>
</div>
