import { describe, expect, it } from "vitest";
import { navigationItemExists, type NavigationDraftRow } from "./navigation";

const rows: NavigationDraftRow[] = [
	{
		pageId: 7,
		path: "/about",
		title: "About",
		label: "About",
		placement: "top",
		sortOrder: 0,
		isEnabled: true,
	},
];

describe("CMS navigation selection", () => {
	it("recognizes current settings and domain items", () => {
		expect(navigationItemExists({ kind: "settings" }, [], [], rows)).toBe(true);
		expect(navigationItemExists({ kind: "page", id: 7 }, [], [], rows)).toBe(true);
		expect(navigationItemExists({ kind: "dropdown", id: "missing" }, [], [], rows)).toBe(false);
	});
});
