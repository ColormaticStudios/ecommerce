<script lang="ts">
	import type { HTMLInputAttributes } from "svelte/elements";

interface Props extends Omit<HTMLInputAttributes, "type"> {
	full?: boolean;
	allowDecimal?: boolean;
	class?: string;
	value?: string | number | undefined;
}

let {
	full = true,
	allowDecimal = false,
	class: className = "",
	value = $bindable(),
	inputmode = "numeric",
	...rest
}: Props = $props();

	const baseClasses =
		"px-3 py-2 rounded-md border border-gray-300 bg-gray-200 dark:border-gray-700 dark:bg-gray-800";
	const widthClasses = $derived(full ? "w-full" : "");
	let isComposing = $state(false);

function sanitize(raw: string): string {
	if (!allowDecimal) {
		return raw.replace(/\D/g, "");
	}

	let out = raw.replace(/[^0-9.]/g, "");
	const firstDot = out.indexOf(".");
	if (firstDot >= 0) {
		out = out.slice(0, firstDot + 1) + out.slice(firstDot + 1).replaceAll(".", "");
	}
	return out;
}

function syncValue(normalized: string) {
	value = typeof value === "number" ? Number(normalized || 0) : normalized;
}

function handleInput(event: Event) {
	const target = event.currentTarget as HTMLInputElement;
	const inputEvent = event as InputEvent;
	const previous = value == null ? "" : String(value);
	if (isComposing || inputEvent.isComposing) {
		return;
	}

	// In some browsers, typing an invalid character in number inputs yields badInput + empty value.
	// Preserve the previous valid value instead of clearing the field.
	if (target.validity.badInput) {
		target.value = previous;
		return;
	}

	const normalized = sanitize(target.value);
	const insertedData = inputEvent.data ?? "";
	const insertedNormalized = sanitize(insertedData);
	const isDeletion = inputEvent.inputType?.startsWith("delete") ?? false;
	const onlyInvalidInsertion =
		insertedData.length > 0 && insertedNormalized.length === 0 && !isDeletion;

	if (normalized === "" && previous !== "" && onlyInvalidInsertion) {
		target.value = previous;
		return;
	}

	if (target.value !== normalized) {
		target.value = normalized;
	}
	syncValue(normalized);
}

function handleKeyDown(event: KeyboardEvent) {
	if (event.isComposing || event.keyCode === 229) {
		return;
	}
	if (event.ctrlKey || event.metaKey || event.altKey) {
		return;
	}
	const allowedControlKeys = new Set([
		"Backspace",
		"Delete",
		"Tab",
		"Escape",
		"Enter",
		"Home",
		"End",
		"ArrowLeft",
		"ArrowRight",
		"ArrowUp",
		"ArrowDown",
	]);
	if (allowedControlKeys.has(event.key)) {
		return;
	}
	if (/^\d$/.test(event.key)) {
		return;
	}
	if (allowDecimal && event.key === ".") {
		const current = value == null ? "" : String(value);
		if (!current.includes(".")) {
			return;
		}
	}
	event.preventDefault();
}

function handleCompositionStart() {
	isComposing = true;
}

function handleCompositionEnd(event: CompositionEvent) {
	isComposing = false;
	const target = event.currentTarget as HTMLInputElement;
	const previous = value == null ? "" : String(value);
	const normalized = sanitize(target.value);
	if (normalized === "" && previous !== "" && target.value !== "") {
		target.value = previous;
		return;
	}
	target.value = normalized;
	syncValue(normalized);
}
</script>

	<input
	type="number"
	bind:value
	inputmode={allowDecimal ? "decimal" : inputmode}
	class={`${baseClasses} ${widthClasses} ${className}`}
	oninput={handleInput}
	onkeydown={handleKeyDown}
	oncompositionstart={handleCompositionStart}
	oncompositionend={handleCompositionEnd}
	{...rest}
/>

<style>
	input::-webkit-inner-spin-button,
	input::-webkit-outer-spin-button {
		-webkit-appearance: none;
		margin: 0;
	}

	input {
		-moz-appearance: textfield;
		appearance: textfield;
	}
</style>
