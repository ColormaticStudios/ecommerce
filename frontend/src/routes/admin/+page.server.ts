import type { PageServerLoad } from "./$types";
import {
	parseOrder,
	parseProduct,
	parseProfile,
	type OrderModel,
	type ProductModel,
	type UserModel,
} from "$lib/models";
import type { components } from "$lib/api/generated/openapi";
import { serverRequest, type ServerAPIError } from "$lib/server/api";

type ProfilePayload = components["schemas"]["User"];
type ProductPagePayload = components["schemas"]["ProductPage"];
type OrderPagePayload = components["schemas"]["OrderPage"];
type UserPagePayload = components["schemas"]["UserPage"];
type CheckoutPluginCatalogPayload = components["schemas"]["CheckoutPluginCatalog"];

type AdminTab = "products" | "orders" | "users" | "providers" | "storefront";
const defaultAdminPageLimit = 10;

function normalizeTab(value: string | null): AdminTab {
	if (value === "orders" || value === "users" || value === "providers" || value === "storefront") {
		return value;
	}
	return "products";
}

export const load: PageServerLoad = async (event) => {
	const initialTab = normalizeTab(event.url.searchParams.get("tab"));

	let isAuthenticated = false;
	let isAdmin = false;
	let accessError = "";
	let products: ProductModel[] = [];
	const productPage = 1;
	let productTotalPages = 1;
	const productLimit = defaultAdminPageLimit;
	const orderPage = 1;
	let orderTotalPages = 1;
	const orderLimit = defaultAdminPageLimit;
	let orderTotal = 0;
	const userPage = 1;
	let userTotalPages = 1;
	const userLimit = defaultAdminPageLimit;
	let userTotal = 0;
	let orders: OrderModel[] = [];
	let users: UserModel[] = [];
	let checkoutPlugins: CheckoutPluginCatalogPayload | null = null;
	let errorMessage = "";

	try {
		const profilePayload = await serverRequest<ProfilePayload>(event, "/me/");
		isAuthenticated = true;
		isAdmin = parseProfile(profilePayload).role === "admin";
	} catch (err) {
		const error = err as ServerAPIError;
		if (error.status !== 401) {
			console.error(err);
			accessError = "Unable to check admin access.";
		}
		return {
			initialTab,
			isAuthenticated,
			isAdmin,
			accessError,
			products,
			productPage,
			productTotalPages,
			productLimit,
			orderPage,
			orderTotalPages,
			orderLimit,
			orderTotal,
			userPage,
			userTotalPages,
			userLimit,
			userTotal,
			orders,
			users,
			checkoutPlugins,
			errorMessage,
		};
	}

	if (!isAdmin) {
		return {
			initialTab,
			isAuthenticated,
			isAdmin,
			accessError,
			products,
			productPage,
			productTotalPages,
			productLimit,
			orderPage,
			orderTotalPages,
			orderLimit,
			orderTotal,
			userPage,
			userTotalPages,
			userLimit,
			userTotal,
			orders,
			users,
			checkoutPlugins,
			errorMessage,
		};
	}

	try {
		const [productsPayload, ordersPayload, usersPayload, checkoutPluginsPayload] =
			await Promise.all([
				serverRequest<ProductPagePayload>(event, "/admin/products", {
					page: productPage,
					limit: productLimit,
				}),
				serverRequest<OrderPagePayload>(event, "/admin/orders", {
					page: orderPage,
					limit: orderLimit,
				}),
				serverRequest<UserPagePayload>(event, "/admin/users", { page: userPage, limit: userLimit }),
				serverRequest<CheckoutPluginCatalogPayload>(event, "/admin/checkout/plugins"),
			]);

		products = productsPayload.data.map(parseProduct);
		productTotalPages = Math.max(1, productsPayload.pagination.total_pages);
		orders = ordersPayload.data.map(parseOrder);
		users = usersPayload.data.map(parseProfile);
		orderTotalPages = Math.max(1, ordersPayload.pagination.total_pages);
		orderTotal = ordersPayload.pagination.total;
		userTotalPages = Math.max(1, usersPayload.pagination.total_pages);
		userTotal = usersPayload.pagination.total;
		checkoutPlugins = checkoutPluginsPayload;
	} catch (err) {
		console.error(err);
		errorMessage = "Unable to load one or more admin data sections.";
	}

	return {
		initialTab,
		isAuthenticated,
		isAdmin,
		accessError,
		products,
		productPage,
		productTotalPages,
		productLimit,
		orderPage,
		orderTotalPages,
		orderLimit,
		orderTotal,
		userPage,
		userTotalPages,
		userLimit,
		userTotal,
		orders,
		users,
		checkoutPlugins,
		errorMessage,
	};
};
