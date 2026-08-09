import { isCmsContentBlock, type CmsContentBlock } from "$lib/cms";

export type CmsBlockProblemStatus = "invalid" | "unsupported";

export interface CmsBlockProblem {
	status: CmsBlockProblemStatus;
	title: string;
	message: string;
	raw: unknown;
}

export type CmsValidEditableBlock = CmsContentBlock & {
	editorId: string;
	editorProblem?: never;
	editorRaw?: never;
};

export interface CmsProblemEditableBlock {
	type: "invalid_block";
	editorId: string;
	editorProblem: CmsBlockProblem;
	editorRaw: unknown;
}

export type CmsEditableBlock = CmsValidEditableBlock | CmsProblemEditableBlock;

const supportedBlockTypes = new Set([
	"hero",
	"rich_text",
	"image",
	"gallery",
	"video",
	"faq",
	"cta",
	"promo_banner",
	"product_rail",
	"category_tiles",
	"promotion_highlight",
	"inventory_message",
	"testimonial",
	"social_embed",
	"custom_html",
	"footer",
]);

function rawBlockType(value: unknown): string | null {
	if (typeof value !== "object" || value === null || Array.isArray(value)) return null;
	const type = (value as Record<string, unknown>).type;
	return typeof type === "string" && type.trim() ? type : null;
}

function problemFor(value: unknown): CmsBlockProblem {
	const type = rawBlockType(value);
	if (type && !supportedBlockTypes.has(type)) {
		return {
			status: "unsupported",
			title: `Unsupported “${type}” block`,
			message:
				"This editor does not support this block type. It will be preserved when you save, but must be removed or supported before publishing.",
			raw: value,
		};
	}
	return {
		status: "invalid",
		title: type ? `Invalid “${type}” block` : "Invalid CMS block",
		message:
			"This block has missing or invalid fields. It will be preserved when you save, but must be removed or corrected before publishing.",
		raw: value,
	};
}

export function assessCmsBlocks(
	value: unknown,
	createEditorId: (prefix: string) => string,
	fallback: CmsEditableBlock[] = []
): CmsEditableBlock[] {
	if (value === undefined || value === null) return fallback;
	const candidates = Array.isArray(value) ? value : [value];
	return candidates.map((candidate, index) => {
		if (isCmsContentBlock(candidate)) {
			return {
				...candidate,
				editorId: createEditorId(`${candidate.type}-${index}`),
			};
		}
		const problem = problemFor(candidate);
		return {
			type: "invalid_block",
			editorId: createEditorId(`${problem.status}-${index}`),
			editorProblem: problem,
			editorRaw: candidate,
		};
	});
}

export function serializeCmsBlock(block: CmsEditableBlock): unknown {
	if (block.type === "invalid_block") return block.editorRaw;
	const content: Record<string, unknown> = { ...block };
	delete content.editorId;
	return content;
}

export function hasBlockingCmsBlocks(blocks: CmsEditableBlock[]): boolean {
	return blocks.some((block) => Boolean(block.editorProblem));
}
