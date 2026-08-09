<script lang="ts">
	import type { CmsBlockProblem } from "./blocks";

	interface Props {
		problem: CmsBlockProblem;
		blockNumber: number;
		compact?: boolean;
	}

	let { problem, blockNumber, compact = false }: Props = $props();

	const rawJSON = $derived.by(() => {
		try {
			return JSON.stringify(problem.raw, null, 2) ?? String(problem.raw);
		} catch {
			return String(problem.raw);
		}
	});
</script>

<div
	class={`rounded-lg border border-red-300 bg-red-50 text-red-950 dark:border-red-900 dark:bg-red-950/40 dark:text-red-100 ${compact ? "p-4" : "p-5"}`}
	role="alert"
>
	<div class="flex items-start gap-3">
		<i class="bi bi-exclamation-triangle mt-0.5 text-red-600 dark:text-red-300"></i>
		<div class="min-w-0 flex-1">
			<p class="font-semibold">Block {blockNumber}: {problem.title}</p>
			<p class="mt-1 text-sm leading-6 text-red-800 dark:text-red-200">{problem.message}</p>
			<details class="mt-3">
				<summary class="cursor-pointer text-sm font-medium">Inspect preserved raw block</summary>
				<pre
					class="mt-2 max-h-64 overflow-auto rounded-md bg-white p-3 text-xs wrap-break-word whitespace-pre-wrap text-stone-800 dark:bg-stone-950 dark:text-stone-200">{rawJSON}</pre>
			</details>
		</div>
	</div>
</div>
