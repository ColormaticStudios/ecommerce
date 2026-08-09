<script lang="ts">
	import {
		cmsBlockLibrary,
		cmsLibraryPreviewClass,
		type CmsBlockLibraryItem,
	} from "./blockCatalog";

	interface Props {
		onPointerDown: (event: PointerEvent, item: CmsBlockLibraryItem) => void;
	}

	let { onPointerDown }: Props = $props();
</script>

<section>
	<div class="mb-3 flex items-center justify-between gap-2">
		<h2 class="text-sm font-semibold">Components</h2>
	</div>
	<div class="grid gap-2">
		{#each cmsBlockLibrary as item (item.type)}
			<button
				type="button"
				class="w-full overflow-hidden rounded-lg border border-stone-200 bg-white text-left text-stone-950 transition hover:border-stone-400 hover:bg-stone-50 dark:border-stone-800 dark:bg-stone-900 dark:text-stone-50 dark:hover:border-stone-600 dark:hover:bg-stone-800"
				onpointerdown={(event) => onPointerDown(event, item)}
			>
				<div
					class="border-b border-stone-200 bg-stone-50 p-2 dark:border-stone-800 dark:bg-stone-950"
				>
					<div class={cmsLibraryPreviewClass(item.type)}>
						{#if item.type === "product_rail"}
							<div class="rounded bg-stone-200 dark:bg-stone-700"></div>
							<div class="rounded bg-stone-300 dark:bg-stone-600"></div>
							<div class="rounded bg-stone-200 dark:bg-stone-700"></div>
						{:else if item.type === "category_tiles"}
							<div class="rounded bg-sky-100 dark:bg-sky-950"></div>
							<div class="rounded bg-emerald-100 dark:bg-emerald-950"></div>
						{:else if item.type === "rich_text"}
							<div class="space-y-1 p-2">
								<div class="h-1.5 w-3/4 rounded bg-stone-300 dark:bg-stone-600"></div>
								<div class="h-1.5 w-full rounded bg-stone-200 dark:bg-stone-700"></div>
								<div class="h-1.5 w-2/3 rounded bg-stone-200 dark:bg-stone-700"></div>
							</div>
						{/if}
					</div>
				</div>
				<span class="flex items-start gap-3 p-3">
					<i class={`bi ${item.icon} mt-0.5 text-stone-600 dark:text-stone-300`}></i>
					<span class="min-w-0"
						><span class="block text-sm font-medium">{item.label}</span><span
							class="block text-xs leading-5 text-stone-500 dark:text-stone-400"
							>{item.description}</span
						></span
					>
				</span>
			</button>
		{/each}
	</div>
</section>
