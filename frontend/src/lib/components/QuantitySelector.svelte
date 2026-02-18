<script lang="ts">
	import IconButton from "$lib/components/IconButton.svelte";
	import NumberInput from "$lib/components/NumberInput.svelte";

	interface Props {
		value?: number | string;
		min?: number;
		max?: number;
		disabled?: boolean;
		class?: string;
		inputClass?: string;
		onDecrease?: () => void;
		onIncrease?: () => void;
		onCommit?: (value: number) => void;
	}

	let {
		value = $bindable(1),
		min = 1,
		max,
		disabled = false,
		class: className = "",
		inputClass = "",
		onDecrease,
		onIncrease,
		onCommit,
	}: Props = $props();

	const numericValue = $derived(Number.isFinite(Number(value)) ? Number(value) : min);
	const canDecrease = $derived(!disabled && numericValue > min);
	const canIncrease = $derived(!disabled && (max == null || numericValue < max));

	function clamp(next: number) {
		const lower = Math.max(min, next);
		if (max == null) {
			return lower;
		}
		return Math.min(max, lower);
	}

	function commit(next: number) {
		const normalized = clamp(next);
		if (onCommit) {
			onCommit(normalized);
			return;
		}
		value = normalized;
	}

	function decrease() {
		if (!canDecrease) {
			return;
		}
		if (onDecrease) {
			onDecrease();
			return;
		}
		commit(numericValue - 1);
	}

	function increase() {
		if (!canIncrease) {
			return;
		}
		if (onIncrease) {
			onIncrease();
			return;
		}
		commit(numericValue + 1);
	}

	function handleChange(event: Event) {
		const target = event.currentTarget as HTMLInputElement;
		const parsed = Number(target.value);
		if (!Number.isFinite(parsed)) {
			commit(min);
			return;
		}
		commit(parsed);
	}
</script>

<div
	class={`flex items-center gap-1 rounded-lg border border-gray-200 bg-white p-1 sm:gap-2 dark:border-gray-800 dark:bg-gray-900 ${className}`}
>
	<IconButton
		type="button"
		size="md"
		disabled={!canDecrease}
		onclick={decrease}
		aria-label="Decrease quantity"
	>
		<i class="bi bi-dash-lg"></i>
	</IconButton>
	<NumberInput
		class={`w-14 py-1! text-center text-base font-medium text-gray-900 outline-none sm:text-lg dark:bg-gray-900 dark:text-gray-100 ${inputClass}`}
		full={false}
		{min}
		{max}
		{disabled}
		bind:value
		onchange={handleChange}
	/>
	<IconButton
		type="button"
		size="md"
		disabled={!canIncrease}
		onclick={increase}
		aria-label="Increase quantity"
	>
		<i class="bi bi-plus-lg"></i>
	</IconButton>
</div>
