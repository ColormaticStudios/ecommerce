import type { CmsEditableBlock } from "./blocks";

export interface CmsEditorHistory {
	entries: CmsEditableBlock[][];
	index: number;
}

export function cloneCmsBlocks(blocks: CmsEditableBlock[]): CmsEditableBlock[] {
	return structuredClone(blocks);
}

export function createCmsEditorHistory(blocks: CmsEditableBlock[]): CmsEditorHistory {
	return { entries: [cloneCmsBlocks(blocks)], index: 0 };
}

export function recordCmsEditorHistory(
	history: CmsEditorHistory,
	blocks: CmsEditableBlock[]
): CmsEditorHistory {
	const current = history.entries[history.index];
	if (current && blocksEqual(current, blocks)) return history;
	return {
		entries: [...history.entries.slice(0, history.index + 1), cloneCmsBlocks(blocks)],
		index: history.index + 1,
	};
}

export function restoreCmsEditorHistory(
	history: CmsEditorHistory,
	index: number
): { history: CmsEditorHistory; blocks: CmsEditableBlock[] } | null {
	const entry = history.entries[index];
	if (!entry) return null;
	return {
		history: { ...history, index },
		blocks: cloneCmsBlocks(entry),
	};
}

function blocksEqual(left: CmsEditableBlock[], right: CmsEditableBlock[]): boolean {
	if (left.length !== right.length) return false;
	return left.every((block, index) => deepEqual(block, right[index]));
}

function deepEqual(left: unknown, right: unknown): boolean {
	if (Object.is(left, right)) return true;
	if (typeof left !== "object" || left === null || typeof right !== "object" || right === null) {
		return false;
	}
	if (Array.isArray(left) || Array.isArray(right)) {
		return (
			Array.isArray(left) &&
			Array.isArray(right) &&
			left.length === right.length &&
			left.every((value, index) => deepEqual(value, right[index]))
		);
	}
	const leftRecord = left as Record<string, unknown>;
	const rightRecord = right as Record<string, unknown>;
	const leftKeys = Object.keys(leftRecord);
	const rightKeys = Object.keys(rightRecord);
	return (
		leftKeys.length === rightKeys.length &&
		leftKeys.every(
			(key) =>
				Object.prototype.hasOwnProperty.call(rightRecord, key) &&
				deepEqual(leftRecord[key], rightRecord[key])
		)
	);
}
