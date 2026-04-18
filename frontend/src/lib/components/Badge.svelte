<script lang="ts">
	import type { HTMLAttributes } from "svelte/elements";

	type BadgeTone = "neutral" | "info" | "success" | "warning" | "danger";
	type BadgeSize = "xs" | "sm" | "md";

	interface Props extends HTMLAttributes<HTMLSpanElement> {
		tone?: BadgeTone;
		size?: BadgeSize;
		class?: string;
		children?: import("svelte").Snippet;
	}

	let { tone = "neutral", size = "sm", class: className = "", children, ...rest }: Props = $props();

	const toneClasses = $derived.by(() => {
		switch (tone) {
			case "info":
				return "border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-900/80 dark:bg-sky-950/50 dark:text-sky-200";
			case "success":
				return "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/80 dark:bg-emerald-950/50 dark:text-emerald-200";
			case "warning":
				return "border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900/80 dark:bg-amber-950/50 dark:text-amber-200";
			case "danger":
				return "border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900/80 dark:bg-rose-950/50 dark:text-rose-200";
			default:
				return "border-stone-200 bg-stone-100 text-stone-700 dark:border-stone-800 dark:bg-stone-900 dark:text-stone-300";
		}
	});

	const sizeClasses = $derived.by(() => {
		switch (size) {
			case "xs":
				return "px-2 py-0.5 text-[10px] font-semibold";
			case "md":
				return "px-3 py-1 text-xs font-semibold";
			default:
				return "px-2.5 py-1 text-[11px] font-semibold";
		}
	});
</script>

<span
	class={`inline-flex items-center gap-1 rounded-full border ${toneClasses} ${sizeClasses} ${className}`}
	{...rest}
>
	{@render children?.()}
</span>
