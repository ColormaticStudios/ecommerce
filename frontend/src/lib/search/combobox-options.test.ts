import { expect, test } from "vitest";
import { searchOptions } from "./combobox-options";

test("structured choices retain identities even when labels and separators overlap", () => {
	const items = [
		{ label: "A /B", value: "C" },
		{ label: "A", value: "B /C" },
		{ label: "A", value: "D" },
	];
	expect(searchOptions(items, [], [])).toEqual(items);
	expect(searchOptions(items, [], []).map((item) => item.value)).toEqual(["C", "B /C", "D"]);
});

test("corrections are explicitly marked and deduplicated across suggestion overlap", () => {
	expect(searchOptions([], ["jacket", "jackets"], ["jackets", "jackets", "coat"])).toEqual([
		{ value: "jacket", label: "jacket", kind: "suggestion" },
		{ value: "jackets", label: "jackets", kind: "correction" },
		{ value: "coat", label: "coat", kind: "correction" },
	]);
});
