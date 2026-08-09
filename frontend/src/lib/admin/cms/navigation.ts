export type NavPlacement = "top" | "hidden" | string;

export interface NavigationDropdown {
	id: string;
	sourceId: number | null;
	label: string;
	sortOrder: number;
}

export interface NavigationCustomItem {
	id: string;
	sourceId: number | null;
	label: string;
	itemType: "internal" | "external";
	targetRef: string;
	url: string;
	placement: NavPlacement;
	sortOrder: number;
	isEnabled: boolean;
}

export interface NavigationDraftRow {
	pageId: number;
	path: string;
	title: string;
	label: string;
	placement: NavPlacement;
	sortOrder: number;
	isEnabled: boolean;
}

export type SelectedNavigationItem =
	| { kind: "settings" }
	| { kind: "dropdown"; id: string }
	| { kind: "custom"; id: string }
	| { kind: "page"; id: number };

export function navigationItemExists(
	selection: SelectedNavigationItem,
	dropdowns: NavigationDropdown[],
	customItems: NavigationCustomItem[],
	rows: NavigationDraftRow[]
): boolean {
	switch (selection.kind) {
		case "settings":
			return true;
		case "dropdown":
			return dropdowns.some((item) => item.id === selection.id);
		case "custom":
			return customItems.some((item) => item.id === selection.id);
		case "page":
			return rows.some((item) => item.pageId === selection.id);
	}
}
