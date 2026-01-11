<script lang="ts">
	import { formatPrice } from "$lib/utils";

	type ProductCardData = {
		//id: number; // Unused
		name: string;
		description?: string | null;
		price?: number | null;
		image?: string | null;
		stock?: number | null;
	};

	type Props = {
		href: string;
		data: ProductCardData;
		showStock?: boolean;
		imageAspect?: "square" | "wide";
	};

	let { href, data, showStock = true, imageAspect = "square" }: Props = $props();

	const imageClass = $derived(imageAspect === "wide" ? "aspect-[4/3]" : "aspect-square");
</script>

<a
	{href}
	class="group flex h-full flex-col overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm transition hover:-translate-y-1 hover:border-gray-300 hover:shadow-md dark:border-gray-800 dark:bg-gray-900 dark:hover:border-gray-700"
>
	<div class={`${imageClass} overflow-hidden bg-gray-200 dark:bg-gray-800`}>
		{#if data.image}
			<img
				src={data.image}
				alt={data.name}
				class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
				loading="lazy"
			/>
		{:else}
			<div class="flex h-full items-center justify-center text-gray-400">No image</div>
		{/if}
	</div>
	<div class="flex flex-1 flex-col gap-2 p-4">
		<h2 class="line-clamp-1 text-base font-semibold text-gray-900 dark:text-gray-100">
			{data.name}
		</h2>

		{#if data.description}
			<p class="line-clamp-2 text-sm text-gray-600 dark:text-gray-400">{data.description}</p>
		{/if}

		<div class="mt-auto flex items-center justify-between">
			{#if data.price != null}
				<span class="text-lg font-semibold text-gray-900 dark:text-gray-100">
					{formatPrice(data.price)}
				</span>
			{/if}
			{#if showStock && data.stock != null}
				{#if data.stock === 0}
					<span class="text-xs font-semibold text-red-500">Out of stock</span>
				{:else if data.stock < 5}
					<span class="text-xs font-semibold text-amber-500">Low stock</span>
				{/if}
			{/if}
		</div>
	</div>
</a>
