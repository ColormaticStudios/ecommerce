export type AdminSectionId =
	| "products"
	| "brands"
	| "categories"
	| "orders"
	| "discounts"
	| "inventory"
	| "purchase-orders"
	| "users"
	| "providers"
	| "localization"
	| "cms"
	| "website";

type AdminRouteHref =
	| "/admin/products"
	| "/admin/brands"
	| "/admin/categories"
	| "/admin/orders"
	| "/admin/discounts"
	| "/admin/inventory"
	| "/admin/purchase-orders"
	| "/admin/users"
	| "/admin/providers"
	| "/admin/localization"
	| "/admin/cms"
	| "/admin/website";

export interface AdminNavItem {
	id: AdminSectionId;
	label: string;
	messageKey: string;
	href: AdminRouteHref;
	icon: string;
	matchPrefixes: string[];
}

export const adminNavItems: AdminNavItem[] = [
	{
		id: "products",
		label: "Products",
		messageKey: "admin.navigation.products",
		href: "/admin/products",
		icon: "bi-box-seam",
		matchPrefixes: ["/admin/products", "/admin/product"],
	},
	{
		id: "brands",
		label: "Brands",
		messageKey: "admin.navigation.brands",
		href: "/admin/brands",
		icon: "bi-tags",
		matchPrefixes: ["/admin/brands"],
	},
	{
		id: "categories",
		label: "Categories",
		messageKey: "admin.navigation.categories",
		href: "/admin/categories",
		icon: "bi-diagram-2",
		matchPrefixes: ["/admin/categories"],
	},
	{
		id: "orders",
		label: "Orders",
		messageKey: "admin.navigation.orders",
		href: "/admin/orders",
		icon: "bi-receipt-cutoff",
		matchPrefixes: ["/admin/orders"],
	},
	{
		id: "discounts",
		label: "Discounts",
		messageKey: "admin.navigation.discounts",
		href: "/admin/discounts",
		icon: "bi-percent",
		matchPrefixes: ["/admin/discounts"],
	},
	{
		id: "inventory",
		label: "Inventory",
		messageKey: "admin.navigation.inventory",
		href: "/admin/inventory",
		icon: "bi-boxes",
		matchPrefixes: ["/admin/inventory"],
	},
	{
		id: "purchase-orders",
		label: "Purchase Orders",
		messageKey: "admin.navigation.purchase_orders",
		href: "/admin/purchase-orders",
		icon: "bi-clipboard-check",
		matchPrefixes: ["/admin/purchase-orders"],
	},
	{
		id: "users",
		label: "Users",
		messageKey: "admin.navigation.users",
		href: "/admin/users",
		icon: "bi-people",
		matchPrefixes: ["/admin/users"],
	},
	{
		id: "providers",
		label: "Providers",
		messageKey: "admin.navigation.providers",
		href: "/admin/providers",
		icon: "bi-diagram-3",
		matchPrefixes: ["/admin/providers"],
	},
	{
		id: "localization",
		label: "Translations",
		messageKey: "admin.navigation.translations",
		href: "/admin/localization",
		icon: "bi-translate",
		matchPrefixes: ["/admin/localization"],
	},
	{
		id: "cms",
		label: "CMS",
		messageKey: "admin.navigation.cms",
		href: "/admin/cms",
		icon: "bi-layout-text-window-reverse",
		matchPrefixes: ["/admin/cms"],
	},
	{
		id: "website",
		label: "Website",
		messageKey: "admin.navigation.website",
		href: "/admin/website",
		icon: "bi-sliders",
		matchPrefixes: ["/admin/website"],
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
