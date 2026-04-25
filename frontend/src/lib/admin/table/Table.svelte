<script lang="ts">
	interface Props {
		class?: string;
		children?: import("svelte").Snippet;
	}

	let { class: className = "", children }: Props = $props();
</script>

<div
	class={`overflow-hidden rounded-lg border border-stone-200 dark:border-stone-800 ${className}`}
>
	<div class="overflow-x-auto">
		<table class="min-w-full divide-y divide-stone-200 text-sm dark:divide-stone-800">
			{@render children?.()}
		</table>
	</div>
</div>
