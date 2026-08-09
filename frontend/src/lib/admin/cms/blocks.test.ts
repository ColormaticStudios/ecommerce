import { describe, expect, it } from "vitest";
import { assessCmsBlocks, hasBlockingCmsBlocks, serializeCmsBlock } from "./blocks";

const id = (prefix: string) => `editor-${prefix}`;

describe("CMS admin block assessment", () => {
	it("assesses every raw block without dropping invalid or unsupported values", () => {
		const raw = [
			{ type: "rich_text", body: "Valid" },
			{ type: "hero" },
			{ type: "future_block", html: '<img src=x onerror="alert(1)">' },
		];

		const blocks = assessCmsBlocks(raw, id);

		expect(blocks).toHaveLength(3);
		expect(blocks[0].editorProblem).toBeUndefined();
		expect(blocks[1].editorProblem?.status).toBe("invalid");
		expect(blocks[2].editorProblem?.status).toBe("unsupported");
		expect(hasBlockingCmsBlocks(blocks)).toBe(true);
		expect(blocks.map(serializeCmsBlock)).toEqual(raw);
	});

	it("preserves an explicitly empty blocks array instead of substituting defaults", () => {
		const fallback = assessCmsBlocks([{ type: "rich_text", body: "Fallback" }], id);
		expect(assessCmsBlocks([], id, fallback)).toEqual([]);
	});

	it("uses defaults only when a payload has no blocks value", () => {
		const fallback = assessCmsBlocks([{ type: "rich_text", body: "Fallback" }], id);
		expect(assessCmsBlocks(undefined, id, fallback)).toBe(fallback);
	});
});
