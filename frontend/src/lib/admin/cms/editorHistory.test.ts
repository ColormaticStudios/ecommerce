import { describe, expect, it } from "vitest";
import type { CmsEditableBlock } from "./blocks";
import {
	cloneCmsBlocks,
	createCmsEditorHistory,
	recordCmsEditorHistory,
	restoreCmsEditorHistory,
} from "./editorHistory";

const initial: CmsEditableBlock[] = [
	{ editorId: "hero-1", type: "hero", title: "Draft", subtitle: "Original" },
];

function hero(block: CmsEditableBlock) {
	if (block.type !== "hero") throw new Error(`Expected hero block, received ${block.type}`);
	return block;
}

describe("CMS visual editor history", () => {
	it("keeps typed snapshots isolated from later edits", () => {
		const history = createCmsEditorHistory(initial);
		const edited = cloneCmsBlocks(initial);
		hero(edited[0]).title = "Edited";

		const next = recordCmsEditorHistory(history, edited);
		hero(edited[0]).title = "Edited again";

		expect(hero(next.entries[0][0]).title).toBe("Draft");
		expect(hero(next.entries[1][0]).title).toBe("Edited");
	});

	it("restores invalid raw block data without sharing references", () => {
		const raw = { type: "future_block", settings: { enabled: true } };
		const invalid = {
			editorId: "unsupported-1",
			type: "future_block",
			editorProblem: {
				status: "unsupported" as const,
				title: "Unsupported block",
				message: "Preserved",
				raw,
			},
			editorRaw: raw,
		} as unknown as CmsEditableBlock;
		const history = createCmsEditorHistory([invalid]);
		const restored = restoreCmsEditorHistory(history, 0);

		expect(restored?.blocks[0].editorRaw).toEqual(raw);
		expect(restored?.blocks[0]).not.toBe(invalid);
		expect(restored?.blocks[0].editorRaw).not.toBe(raw);
	});

	it("does not record an unchanged snapshot and truncates redo history after a new edit", () => {
		const first = createCmsEditorHistory(initial);
		expect(recordCmsEditorHistory(first, cloneCmsBlocks(initial))).toBe(first);

		const second = recordCmsEditorHistory(first, [
			{ ...initial[0], title: "Second" } as CmsEditableBlock,
		]);
		const restored = restoreCmsEditorHistory(second, 0)!;
		const replacement = recordCmsEditorHistory(restored.history, [
			{ ...initial[0], title: "Replacement" } as CmsEditableBlock,
		]);

		expect(replacement.entries).toHaveLength(2);
		expect(hero(replacement.entries[1][0]).title).toBe("Replacement");
	});
});
