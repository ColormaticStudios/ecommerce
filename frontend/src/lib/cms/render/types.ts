import type { CmsContentBlock } from "$lib/cms";

export type CmsBlock<Type extends CmsContentBlock["type"]> = Extract<
	CmsContentBlock,
	{ type: Type }
>;
