<script lang="ts">
	import type { components } from "$lib/api/generated/openapi";
	import type { CmsContentBlock } from "$lib/cms";
	import CmsVisualEditor from "$lib/admin/CmsVisualEditor.svelte";

	type EditableBlock = CmsContentBlock & { editorId: string };
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
	canPublish={true}
	{previewBlocks}
	{createBlock}
	onSave={() => undefined}
	onPublish={() => undefined}
	onRevert={() => undefined}
	onClose={() => undefined}
	onRefreshPreview={() => undefined}
/>
