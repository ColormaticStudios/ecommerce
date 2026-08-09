<script lang="ts">
	import { untrack } from "svelte";
	import type { components } from "$lib/api/generated/openapi";
	import type { CmsContentBlock } from "$lib/cms";
	import CmsVisualEditor from "$lib/admin/CmsVisualEditor.svelte";
	import type { CmsEditableBlock } from "$lib/admin/cms/blocks";

	interface Props {
		invalidRawBlock?: boolean;
	}

	let { invalidRawBlock = false }: Props = $props();
	const initialInvalidRawBlock = untrack(() => invalidRawBlock);
	type EditableBlock = CmsEditableBlock;
	type CmsPreviewBlock = components["schemas"]["CmsPreviewBlock"];

	let pageTitle = $state("Launch campaign");
	let blocks = $state<EditableBlock[]>([
		{
			editorId: "hero-1",
			type: "hero",
			title: "Launch campaign",
			subtitle: "A seasonal page assembled with CMS edit mode.",
		},
		{
			editorId: "categories-1",
			type: "category_tiles",
			title: "Shop by category",
			subtitle: "",
			category_slugs: ["bags", "outerwear"],
			category_media_ids: {},
			image_aspect: "wide",
		},
		{
			editorId: "products-1",
			type: "product_rail",
			title: "Featured products",
			subtitle: "Live catalog picks",
			source: "newest",
			limit: 4,
			product_ids: [],
			sort: "created_at",
			order: "desc",
			image_aspect: "square",
		},
		{
			editorId: "text-empty-1",
			type: "rich_text",
			body: "",
		},
		{
			editorId: "promo-1",
			type: "promo_banner",
			title: "Weekend offer",
			body: "Free shipping through Sunday.",
			link: { label: "Shop the offer", url: "/sale" },
		},
		{
			editorId: "inventory-1",
			type: "inventory_message",
			product_id: 101,
			low_stock_threshold: 5,
			in_stock_message: "Ready to ship",
			low_stock_message: "Only a few left",
			out_of_stock_message: "Currently unavailable",
		},
		...(initialInvalidRawBlock
			? [
					{
						editorId: "unsupported-1",
						type: "future_personalization",
						editorProblem: {
							status: "unsupported" as const,
							title: "Unsupported “future_personalization” block",
							message:
								"This editor does not support this block type. It will be preserved when you save, but must be removed or supported before publishing.",
							raw: { type: "future_personalization", segment: "vip" },
						},
						editorRaw: { type: "future_personalization", segment: "vip" },
					} as unknown as EditableBlock,
				]
			: []),
	]);

	const previewBlocks: CmsPreviewBlock[] = [
		{ key: "category_tiles:1", type: "category_tiles", status: "ok", item_count: 2, messages: [] },
		{ key: "product_rail:2", type: "product_rail", status: "ok", item_count: 4, messages: [] },
	];

	function createBlock(type: CmsContentBlock["type"]): EditableBlock {
		return {
			editorId: `${type}-${Date.now()}`,
			type: "rich_text",
			body: "New content block",
		};
	}
</script>

<CmsVisualEditor
	bind:blocks
	bind:pageTitle
	pagePath="/launch"
	hasUnsavedChanges={true}
	canPublish={!invalidRawBlock}
	{previewBlocks}
	{createBlock}
	onSave={() => undefined}
	onPublish={() => undefined}
	onRevert={() => undefined}
	onClose={() => undefined}
	onRefreshPreview={() => undefined}
/>
