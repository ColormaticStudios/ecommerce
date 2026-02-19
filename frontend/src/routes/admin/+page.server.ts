import type { PageServerLoad } from "./$types";
import { parseOrder, parseProduct, parseProfile, type OrderModel, type ProductModel, type UserModel } from "$lib/models";
import type { components } from "$lib/api/generated/openapi";
import { serverRequest, type ServerAPIError } from "$lib/server/api";

type ProfilePayload = components["schemas"]["User"];
type ProductPagePayload = components["schemas"]["ProductPage"];
type OrderPagePayload = components["schemas"]["OrderPage"];
type UserPagePayload = components["schemas"]["UserPage"];

type AdminTab = "products" | "orders" | "users" | "storefront";

function normalizeTab(value: string | null): AdminTab {
	if (value === "orders" || value === "users" || value === "storefront") {
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
	const productLimit = 20;
	let orders: OrderModel[] = [];
	let users: UserModel[] = [];
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
			orders,
			users,
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
			orders,
			users,
			errorMessage,
		};
	}

	try {
		const [productsPayload, ordersPayload, usersPayload] = await Promise.all([
			serverRequest<ProductPagePayload>(event, "/products", {
				page: productPage,
				limit: productLimit,
			}),
			serverRequest<OrderPagePayload>(event, "/admin/orders", { page: 1, limit: 50 }),
			serverRequest<UserPagePayload>(event, "/admin/users", { page: 1, limit: 50 }),
		]);

		products = productsPayload.data.map(parseProduct);
		productTotalPages = Math.max(1, productsPayload.pagination.total_pages);
		orders = ordersPayload.data.map(parseOrder);
		users = usersPayload.data.map(parseProfile);
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
		orders,
		users,
		errorMessage,
	};
};
