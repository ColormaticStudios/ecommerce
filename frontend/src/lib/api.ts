import { type ProductModel, type PageModel, type ProfileModel, parseProduct } from "$lib/models";
import { getCookie, setCookie } from "$lib/cookie";

const API_ROUTE = "/api/v1";

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

	private async request(
		method: string,
		path: string,
		data?: object,
		params?: Record<string, unknown>
	) {
		const headers = new Headers();
		headers.append("Content-Type", "application/json");

		if (this.accessToken) {
			headers.set("Authorization", `Bearer ${this.accessToken}`);
		}

		const url = new URL(`${this.baseUrl}${API_ROUTE}${path}`);
		if (params) {
			Object.entries(params).forEach(([key, value]) => {
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

		return body;
	}

	// Authentication
	public async register(data: {
		username: string;
		email: string;
		password: string;
		name?: string;
	}) {
		return this.request("POST", "/auth/register", data);
	}

	public async login(data: { email: string; password: string }) {
		return this.request("POST", "/auth/login", data);
	}

	public async createOrder(data: { items: Array<{ product_id: number; quantity: number }> }) {
		return this.request("POST", "/me/orders", data);
	}

	public async processPayment(orderId: number) {
		return this.request("POST", `/me/orders/${orderId}/pay`);
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
		const response = await this.request("GET", "/products", undefined, params);

		const page: PageModel = {
			data: [],
			pagination: response.pagination,
		};
		response.data.forEach((elem: ProductModel) => {
			page.data.push(parseProduct(elem));
		});

		return page;
	}

	public async getProduct(id: number): Promise<ProductModel> {
		const response = await this.request("GET", `/products/${id}`);
		const Product: ProductModel = parseProduct(response);

		return Product;
	}

	// Cart Operations
	public async viewCart() {
		return this.request("GET", "/me/cart");
	}

	public async addToCart(data: { product_id: number; quantity: number }) {
		return this.request("POST", "/me/cart", data);
	}

	public async updateCartItem(itemId: number, data: { quantity: number }) {
		return this.request("PATCH", `/me/cart/${itemId}`, data);
	}

	public async removeCartItem(itemId: number) {
		return this.request("DELETE", `/me/cart/${itemId}`);
	}

	// Profile Management
	public async getProfile(): Promise<ProfileModel> {
		return this.request("GET", "/me/");
	}

	public async updateProfile(data: {
		name?: string;
		currency?: string;
		profile_photo_url?: string;
	}) {
		return this.request("PATCH", "/me", data);
	}

	// Admin Operations
	public async createProduct(data: {
		sku: string;
		name: string;
		description?: string;
		price: number;
		stock?: number;
		images?: string[];
	}) {
		return this.request("POST", "/admin/products", data);
	}

	// Order Management
	public async listOrders(params?: { page?: number; limit?: number }) {
		return this.request("GET", "/me/orders", undefined, params);
	}

	public async getOrderDetails(orderId: number) {
		return this.request("GET", `/me/orders/${orderId}`);
	}

	public isAuthenticated() {
		return this.accessToken ? true : false;
	}

	public removeToken() {
		this.accessToken = undefined;
	}

	public tokenFromCookie() {
		const tokenCookie = getCookie("accessToken");
		if (tokenCookie) {
			this.accessToken = tokenCookie;
		}
	}
}
