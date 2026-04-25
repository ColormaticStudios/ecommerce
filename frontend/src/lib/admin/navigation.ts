export type AdminSectionId =
	| "products"
	| "brands"
	| "orders"
	| "inventory"
	| "purchase-orders"
	| "users"
	| "providers"
	| "storefront";

type AdminRouteHref =
	| "/admin/products"
	| "/admin/brands"
	| "/admin/orders"
	| "/admin/inventory"
	| "/admin/purchase-orders"
	| "/admin/users"
	| "/admin/providers"
	| "/admin/storefront";

export interface AdminNavItem {
	id: AdminSectionId;
	label: string;
	href: AdminRouteHref;
	icon: string;
	matchPrefixes: string[];
}

export const adminNavItems: AdminNavItem[] = [
	{
		id: "products",
		label: "Products",
		href: "/admin/products",
		icon: "bi-box-seam",
		matchPrefixes: ["/admin/products", "/admin/product"],
	},
	{
		id: "brands",
		label: "Brands",
		href: "/admin/brands",
		icon: "bi-tags",
		matchPrefixes: ["/admin/brands"],
	},
	{
		id: "orders",
		label: "Orders",
		href: "/admin/orders",
		icon: "bi-receipt-cutoff",
		matchPrefixes: ["/admin/orders"],
	},
	{
		id: "inventory",
		label: "Inventory",
		href: "/admin/inventory",
		icon: "bi-boxes",
		matchPrefixes: ["/admin/inventory"],
	},
	{
		id: "purchase-orders",
		label: "Purchase Orders",
		href: "/admin/purchase-orders",
		icon: "bi-clipboard-check",
		matchPrefixes: ["/admin/purchase-orders"],
	},
	{
		id: "users",
		label: "Users",
		href: "/admin/users",
		icon: "bi-people",
		matchPrefixes: ["/admin/users"],
	},
	{
		id: "providers",
		label: "Providers",
		href: "/admin/providers",
		icon: "bi-diagram-3",
		matchPrefixes: ["/admin/providers"],
	},
	{
		id: "storefront",
		label: "Storefront",
		href: "/admin/storefront",
		icon: "bi-window-stack",
		matchPrefixes: ["/admin/storefront"],
	},
];

export function getActiveAdminSection(pathname: string): AdminSectionId {
	if (pathname === "/admin") {
		return "products";
	}

	for (const item of adminNavItems) {
		for (const prefix of item.matchPrefixes) {
			if (pathname === prefix || pathname.startsWith(`${prefix}/`)) {
				return item.id;
			}
		}
	}
	return "products";
}
