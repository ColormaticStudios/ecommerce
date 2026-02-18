import {
	type ProductModel,
	type UserModel,
	type PageModel,
	type OrderModel,
	type CartModel,
	type CartItemModel,
	type ProfileModel,
	type OrderPayload,
	type SavedPaymentMethodModel,
	type SavedAddressModel,
	parseProduct,
	parseOrder,
	parseCart,
	parseCartItem,
	parseProfile,
	parseSavedPaymentMethod,
	parseSavedAddress,
} from "$lib/models";
import { getCookie, setCookie } from "$lib/cookie";

const API_ROUTE = "/api/v1";

interface OrderPageResponse {
	data: OrderPayload[];
	pagination: {
		limit: number;
		page: number;
		total: number;
		total_pages: number;
	};
}

interface PageResponse<T> {
	data: T[];
	pagination: {
		limit: number;
		page: number;
		total: number;
		total_pages: number;
	};
}

export class API {
	private baseUrl: string;
	private accessToken: string | undefined;

	constructor(baseUrl = "http://localhost:3000") {
		this.baseUrl = baseUrl;
		this.accessToken = undefined;
	}

	public setToken(token: string | undefined) {
		this.accessToken = token;
		if (token) {
			setCookie("accessToken", token, "Strict");
		}
	}

	private async request<T>(
		method: string,
		path: string,
		data?: object,
		params?: Record<string, unknown>
	): Promise<T> {
		const headers = new Headers();
		headers.append("Content-Type", "application/json");

		if (this.accessToken) {
			headers.set("Authorization", `Bearer ${this.accessToken}`);
		}

		const url = new URL(`${this.baseUrl}${API_ROUTE}${path}`);
		if (params) {
			Object.entries(params).forEach(([key, value]) => {
				if (value === undefined || value === null || value === "") {
					return;
				}
				url.searchParams.append(key, String(value));
			});
		}

		const response = await fetch(url.toString(), {
			method,
			headers,
			body: method === "GET" ? undefined : JSON.stringify(data),
		});

		const text = await response.text();
		// eslint-disable-next-line @typescript-eslint/no-explicit-any
		let body: any;

		try {
			body = text ? JSON.parse(text) : null;
		} catch {
			body = text;
		}

		if (!response.ok) {
			throw {
				status: response.status,
				statusText: response.statusText,
				body,
			};
		}

		return body as T;
	}

	// Authentication
	public async register(data: {
		username: string;
		email: string;
		password: string;
		name?: string;
	}): Promise<{ token: string; user: ProfileModel }> {
		return this.request("POST", "/auth/register", data);
	}

	public async login(data: {
		email: string;
		password: string;
	}): Promise<{ token: string; user: ProfileModel }> {
		return this.request("POST", "/auth/login", data);
	}

	public async createOrder(data: {
		items: Array<{ product_id: number; quantity: number }>;
	}): Promise<OrderModel> {
		const response = await this.request<OrderModel>("POST", "/me/orders", data);
		return parseOrder(response);
	}

	public async processPayment(
		orderId: number,
		data?: {
			payment_method_id?: number;
			address_id?: number;
			payment_method?: {
				cardholder_name: string;
				card_number: string;
				exp_month: number;
				exp_year: number;
			};
			address?: {
				full_name: string;
				line1: string;
				line2?: string;
				city: string;
				state?: string;
				postal_code: string;
				country: string;
			};
		}
	): Promise<OrderModel> {
		const response = await this.request<{ order?: OrderPayload } | OrderPayload>(
			"POST",
			`/me/orders/${orderId}/pay`,
			data
		);
		const payload = "order" in response ? response.order : (response as OrderPayload);
		if (!payload) {
			throw new Error("Missing order payload");
		}
		return parseOrder(payload);
	}

	// Product Management
	public async listProducts(params?: {
		q?: string;
		min_price?: number;
		max_price?: number;
		sort?: "price" | "name" | "created_at";
		order?: "asc" | "desc";
		page?: number;
		limit?: number;
	}): Promise<PageModel> {
		const response = await this.request<PageModel>("GET", "/products", undefined, params);

		const data = response.data.map(parseProduct).map((product) => {
			return {
				...product,
				cover_image: product.cover_image ?? product.images[0] ?? null,
			};
		});
		const page: PageModel = {
			data,
			pagination: response.pagination,
		};

		return page;
	}

	public async getProduct(id: number): Promise<ProductModel> {
		const response = await this.request<ProductModel>("GET", `/products/${id}`);
		const Product: ProductModel = parseProduct(response);

		return Product;
	}

	// Cart Operations
	public async viewCart(): Promise<CartModel> {
		const response = await this.request<CartModel>("GET", "/me/cart");
		const cart = parseCart(response);

		return cart;
	}

	public async addToCart(data: { product_id: number; quantity: number }): Promise<CartModel> {
		const response = await this.request<CartModel>("POST", "/me/cart", data);
		const cart = parseCart(response);

		return cart;
	}

	public async updateCartItem(itemId: number, data: { quantity: number }): Promise<CartItemModel> {
		const response = await this.request<CartItemModel>("PATCH", `/me/cart/${itemId}`, data);
		const cartItem = parseCartItem(response);

		return cartItem;
	}

	public async removeCartItem(itemId: number): Promise<{ message?: string }> {
		return await this.request("DELETE", `/me/cart/${itemId}`);
	}

	// Profile Management
	public async getProfile(): Promise<UserModel> {
		// There's a weird quirk about how Gin handles the routing so we have to hit `/me/`, not `/me`
		const response = await this.request<ProfileModel>("GET", "/me/");
		return parseProfile(response);
	}

	public async updateProfile(data: {
		name?: string;
		currency?: string;
		profile_photo_url?: string;
	}): Promise<UserModel> {
		const response = await this.request<ProfileModel>("PATCH", "/me/", data);
		return parseProfile(response);
	}

	public async uploadMedia(file: File): Promise<string> {
		if (!this.accessToken) {
			throw new Error("Not authenticated");
		}

		const uploadUrl = new URL(`${this.baseUrl}${API_ROUTE}/media/uploads`);
		const metadata = `filename ${btoa(unescape(encodeURIComponent(file.name)))}`;

		const createResponse = await fetch(uploadUrl.toString(), {
			method: "POST",
			headers: {
				"Tus-Resumable": "1.0.0",
				"Upload-Length": String(file.size),
				"Upload-Metadata": metadata,
				Authorization: `Bearer ${this.accessToken}`,
			},
		});

		if (!createResponse.ok) {
			throw new Error(`Failed to create upload: ${createResponse.statusText}`);
		}

		const location = createResponse.headers.get("Location");
		if (!location) {
			throw new Error("Upload location missing");
		}

		const resolvedLocation = location.startsWith("/") ? `${this.baseUrl}${location}` : location;

		const patchResponse = await fetch(resolvedLocation, {
			method: "PATCH",
			headers: {
				"Tus-Resumable": "1.0.0",
				"Upload-Offset": "0",
				"Content-Type": "application/offset+octet-stream",
				Authorization: `Bearer ${this.accessToken}`,
			},
			body: file,
		});

		if (!patchResponse.ok) {
			throw new Error(`Failed to upload media: ${patchResponse.statusText}`);
		}

		const parsed = new URL(location, this.baseUrl);
		const segments = parsed.pathname.split("/").filter(Boolean);
		const mediaId = segments[segments.length - 1];
		if (!mediaId) {
			throw new Error("Upload ID missing");
		}

		return mediaId;
	}

	public async attachProfilePhoto(mediaId: string): Promise<{ message?: string }> {
		return await this.request("POST", "/me/profile-photo", { media_id: mediaId });
	}

	public async removeProfilePhoto(): Promise<{ message?: string }> {
		return await this.request("DELETE", "/me/profile-photo");
	}

	// Saved Payment Methods
	public async listSavedPaymentMethods(): Promise<SavedPaymentMethodModel[]> {
		const response = await this.request<SavedPaymentMethodModel[]>("GET", "/me/payment-methods");
		return response.map(parseSavedPaymentMethod);
	}

	public async createSavedPaymentMethod(data: {
		cardholder_name: string;
		card_number: string;
		exp_month: number;
		exp_year: number;
		nickname?: string;
		set_default?: boolean;
	}): Promise<SavedPaymentMethodModel> {
		const response = await this.request<SavedPaymentMethodModel>("POST", "/me/payment-methods", data);
		return parseSavedPaymentMethod(response);
	}

	public async deleteSavedPaymentMethod(id: number): Promise<{ message?: string }> {
		return await this.request("DELETE", `/me/payment-methods/${id}`);
	}

	public async setDefaultPaymentMethod(id: number): Promise<SavedPaymentMethodModel> {
		const response = await this.request<SavedPaymentMethodModel>(
			"PATCH",
			`/me/payment-methods/${id}/default`
		);
		return parseSavedPaymentMethod(response);
	}

	// Saved Addresses
	public async listSavedAddresses(): Promise<SavedAddressModel[]> {
		const response = await this.request<SavedAddressModel[]>("GET", "/me/addresses");
		return response.map(parseSavedAddress);
	}

	public async createSavedAddress(data: {
		label?: string;
		full_name: string;
		line1: string;
		line2?: string;
		city: string;
		state?: string;
		postal_code: string;
		country: string;
		phone?: string;
		set_default?: boolean;
	}): Promise<SavedAddressModel> {
		const response = await this.request<SavedAddressModel>("POST", "/me/addresses", data);
		return parseSavedAddress(response);
	}

	public async deleteSavedAddress(id: number): Promise<{ message?: string }> {
		return await this.request("DELETE", `/me/addresses/${id}`);
	}

	public async setDefaultAddress(id: number): Promise<SavedAddressModel> {
		const response = await this.request<SavedAddressModel>("PATCH", `/me/addresses/${id}/default`);
		return parseSavedAddress(response);
	}

	// Admin Operations
	public async createProduct(data: {
		sku: string;
		name: string;
		description?: string;
		price: number;
		stock?: number;
		images?: string[];
	}): Promise<ProductModel> {
		const response = await this.request<ProductModel>("POST", "/admin/products", data);
		return parseProduct(response);
	}

	public async updateProduct(
		id: number,
		data: {
			sku?: string;
			name?: string;
			description?: string;
			price?: number;
			stock?: number;
			images?: string[];
		}
	): Promise<ProductModel> {
		const response = await this.request<ProductModel>("PATCH", `/admin/products/${id}`, data);
		return parseProduct(response);
	}

	public async deleteProduct(id: number): Promise<{ message?: string }> {
		return await this.request("DELETE", `/admin/products/${id}`);
	}

	public async attachProductMedia(id: number, mediaIds: string[]): Promise<ProductModel> {
		const response = await this.request<ProductModel>("POST", `/admin/products/${id}/media`, {
			media_ids: mediaIds,
		});
		return parseProduct(response);
	}

	public async detachProductMedia(id: number, mediaId: string): Promise<ProductModel> {
		const response = await this.request<ProductModel>(
			"DELETE",
			`/admin/products/${id}/media/${mediaId}`
		);
		return parseProduct(response);
	}

	public async updateProductMediaOrder(id: number, mediaIds: string[]): Promise<ProductModel> {
		const response = await this.request<ProductModel>(
			"PATCH",
			`/admin/products/${id}/media/order`,
			{
				media_ids: mediaIds,
			}
		);
		return parseProduct(response);
	}

	public async updateProductRelated(id: number, relatedIds: number[]): Promise<ProductModel> {
		const response = await this.request<ProductModel>("PATCH", `/admin/products/${id}/related`, {
			related_ids: relatedIds,
		});
		return parseProduct(response);
	}

	// Order Management
	public async listOrders(params?: {
		page?: number;
		limit?: number;
		status?: OrderModel["status"] | "";
		start_date?: string;
		end_date?: string;
	}): Promise<{ data: OrderModel[]; pagination: OrderPageResponse["pagination"] }> {
		const response = await this.request<OrderPageResponse>("GET", "/me/orders", undefined, params);
		return {
			data: response.data.map(parseOrder),
			pagination: response.pagination,
		};
	}

	public async getOrderDetails(orderId: number): Promise<OrderModel> {
		const response = await this.request<OrderModel>("GET", `/me/orders/${orderId}`);
		return parseOrder(response);
	}

	// Admin Order Management
	public async listAdminOrders(params?: {
		page?: number;
		limit?: number;
	}): Promise<{ data: OrderModel[]; pagination: PageResponse<OrderPayload>["pagination"] }> {
		const response = await this.request<PageResponse<OrderPayload>>(
			"GET",
			"/admin/orders",
			undefined,
			params
		);
		return {
			data: response.data.map(parseOrder),
			pagination: response.pagination,
		};
	}

	public async getAdminOrderDetails(orderId: number): Promise<OrderModel> {
		const response = await this.request<OrderModel>("GET", `/admin/orders/${orderId}`);
		return parseOrder(response);
	}

	public async updateOrderStatus(
		orderId: number,
		data: { status: OrderModel["status"] }
	): Promise<OrderModel> {
		const response = await this.request<OrderModel>(
			"PATCH",
			`/admin/orders/${orderId}/status`,
			data
		);
		return parseOrder(response);
	}

	// Admin User Management
	public async listUsers(params?: {
		page?: number;
		limit?: number;
	}): Promise<{ data: UserModel[]; pagination: PageResponse<ProfileModel>["pagination"] }> {
		const response = await this.request<PageResponse<ProfileModel>>(
			"GET",
			"/admin/users",
			undefined,
			params
		);
		return {
			data: response.data.map(parseProfile),
			pagination: response.pagination,
		};
	}

	public async updateUserRole(userId: number, data: { role: string }): Promise<UserModel> {
		const response = await this.request<ProfileModel>("PATCH", `/admin/users/${userId}/role`, data);
		return parseProfile(response);
	}

	// Auth Token Management
	public isAuthenticated() {
		return this.accessToken ? true : false;
	}

	public removeToken() {
		this.accessToken = undefined;
		setCookie("accessToken", "", "Strict");
	}

	public tokenFromCookie() {
		const tokenCookie = getCookie("accessToken");
		if (tokenCookie) {
			this.accessToken = tokenCookie;
		}
	}
}
