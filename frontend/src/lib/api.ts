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
import { API_BASE_URL } from "$lib/config";
import { fetchProduct, fetchProducts, type ListProductsQuery } from "$lib/api/openapi-client";
import {
	type StorefrontSettingsModel,
	type StorefrontSettingsResponseModel,
	parseStorefrontSettingsResponse,
} from "$lib/storefront";
import { appendQueryParams } from "$lib/api/http";
import type { components, paths } from "$lib/api/generated/openapi";

const API_ROUTE = "/api/v1";
export const DRAFT_PREVIEW_SYNC_EVENT = "draft-preview:changed";
export const DRAFT_PREVIEW_SYNC_STORAGE_KEY = "draft-preview:state";
export const STOREFRONT_SYNC_EVENT = "storefront:changed";
export const STOREFRONT_SYNC_STORAGE_KEY = "storefront:state";

type RegisterRequest = components["schemas"]["RegisterRequest"];
type LoginRequest = components["schemas"]["LoginRequest"];
type AuthResponse = components["schemas"]["AuthResponse"];
type UpdateProfileRequest = components["schemas"]["UpdateProfileRequest"];
type CreateOrderRequest = components["schemas"]["CreateOrderRequest"];
type ProcessPaymentRequest = components["schemas"]["ProcessPaymentRequest"];
type ProcessPaymentResponse = components["schemas"]["ProcessPaymentResponse"];
type MessageResponse = components["schemas"]["MessageResponse"];
type CartPayload = components["schemas"]["Cart"];
type CartItemPayload = components["schemas"]["CartItem"];
type ProductInput = components["schemas"]["ProductInput"];
type MediaIDsRequest = components["schemas"]["MediaIDsRequest"];
type UpdateRelatedRequest = components["schemas"]["UpdateRelatedRequest"];
type OrderPagePayload = components["schemas"]["OrderPage"];
type UserPagePayload = components["schemas"]["UserPage"];
type UpdateOrderStatusRequest = components["schemas"]["UpdateOrderStatusRequest"];
type CreateSavedPaymentMethodRequest = components["schemas"]["CreateSavedPaymentMethodRequest"];
type CreateSavedAddressRequest = components["schemas"]["CreateSavedAddressRequest"];
type StorefrontSettingsRequest = components["schemas"]["StorefrontSettingsRequest"];
type StorefrontSettingsResponse = components["schemas"]["StorefrontSettingsResponse"];
type DraftPreviewSessionResponse = components["schemas"]["DraftPreviewSessionResponse"];
type ListUserOrdersQuery = paths["/api/v1/me/orders"]["get"]["parameters"]["query"];
type ListAdminOrdersQuery = paths["/api/v1/admin/orders"]["get"]["parameters"]["query"];
type ListAdminProductsQuery = paths["/api/v1/admin/products"]["get"]["parameters"]["query"];
type ListUsersQuery = paths["/api/v1/admin/users"]["get"]["parameters"]["query"];
type ListOrdersParams = Omit<NonNullable<ListUserOrdersQuery>, "status"> & {
	status?: NonNullable<ListUserOrdersQuery>["status"] | "";
};

export interface DraftPreviewSessionModel {
	active: boolean;
	expires_at: Date | null;
}

function parseDraftPreviewSession(response: DraftPreviewSessionResponse): DraftPreviewSessionModel {
	return {
		active: response.active,
		expires_at: response.expires_at ? new Date(response.expires_at) : null,
	};
}

function broadcastDraftPreviewState(session: DraftPreviewSessionModel): void {
	if (typeof window === "undefined") {
		return;
	}

	window.dispatchEvent(
		new CustomEvent(DRAFT_PREVIEW_SYNC_EVENT, {
			detail: { active: session.active, expires_at: session.expires_at?.toISOString() ?? null },
		})
	);

	try {
		window.localStorage.setItem(
			DRAFT_PREVIEW_SYNC_STORAGE_KEY,
			JSON.stringify({
				active: session.active,
				expires_at: session.expires_at?.toISOString() ?? null,
				ts: Date.now(),
			})
		);
	} catch {
		// ignore storage sync failures
	}
}

function broadcastStorefrontStateChange(): void {
	if (typeof window === "undefined") {
		return;
	}

	window.dispatchEvent(new CustomEvent(STOREFRONT_SYNC_EVENT));

	try {
		window.localStorage.setItem(
			STOREFRONT_SYNC_STORAGE_KEY,
			JSON.stringify({
				ts: Date.now(),
			})
		);
	} catch {
		// ignore storage sync failures
	}
}

export class API {
	private baseUrl: string;
	private authenticated: boolean;
	private authStateResolved: boolean;

	constructor(baseUrl = API_BASE_URL) {
		this.baseUrl = baseUrl;
		this.authenticated = false;
		this.authStateResolved = false;
	}

	private readCookie(name: string): string {
		if (typeof document === "undefined") {
			return "";
		}
		const prefix = `${name}=`;
		const decoded = decodeURIComponent(document.cookie);
		for (const part of decoded.split(";")) {
			const cookie = part.trim();
			if (cookie.startsWith(prefix)) {
				return cookie.slice(prefix.length);
			}
		}
		return "";
	}

	private async request<T>(
		method: string,
		path: string,
		data?: object,
		params?: Record<string, unknown>
	): Promise<T> {
		const headers = new Headers();
		headers.append("Content-Type", "application/json");
		if (method !== "GET" && method !== "HEAD" && method !== "OPTIONS") {
			const csrfToken = this.readCookie("csrf_token");
			if (csrfToken) {
				headers.set("X-CSRF-Token", csrfToken);
			}
		}

		const url = new URL(`${this.baseUrl}${API_ROUTE}${path}`);
		appendQueryParams(url, params);

		const response = await fetch(url.toString(), {
			method,
			headers,
			body: method === "GET" ? undefined : JSON.stringify(data),
			credentials: "include",
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
			if (response.status === 401) {
				this.authenticated = false;
				this.authStateResolved = true;
			}
			this.handleCsrfForbidden(response.status, body);
			throw {
				status: response.status,
				statusText: response.statusText,
				body,
			};
		}

		return body as T;
	}

	private isCsrfForbidden(status: number, body: unknown): boolean {
		if (status !== 403 || typeof body !== "object" || body === null || !("error" in body)) {
			return false;
		}

		const message = String((body as { error?: unknown }).error ?? "").toLowerCase();
		return message.includes("csrf token");
	}

	private handleCsrfForbidden(status: number, body: unknown): void {
		if (!this.isCsrfForbidden(status, body)) {
			return;
		}

		this.authenticated = false;
		this.authStateResolved = true;

		if (typeof window === "undefined") {
			return;
		}

		if (!window.location.pathname.startsWith("/login")) {
			window.location.assign("/login?reason=reauth");
		}
	}

	// Authentication
	public async register(data: RegisterRequest): Promise<AuthResponse> {
		const response = await this.request<AuthResponse>("POST", "/auth/register", data);
		this.authenticated = true;
		this.authStateResolved = true;
		return response;
	}

	public async login(data: LoginRequest): Promise<AuthResponse> {
		const response = await this.request<AuthResponse>("POST", "/auth/login", data);
		this.authenticated = true;
		this.authStateResolved = true;
		return response;
	}

	public async logout(): Promise<void> {
		await this.request("POST", "/auth/logout");
		this.authenticated = false;
		this.authStateResolved = true;
	}

	public async createOrder(data: CreateOrderRequest): Promise<OrderModel> {
		const response = await this.request<OrderPayload>("POST", "/me/orders", data);
		return parseOrder(response);
	}

	public async processPayment(orderId: number, data?: ProcessPaymentRequest): Promise<OrderModel> {
		const response = await this.request<ProcessPaymentResponse>(
			"POST",
			`/me/orders/${orderId}/pay`,
			data
		);
		return parseOrder(response.order);
	}

	// Product Management
	public async listProducts(params?: ListProductsQuery): Promise<PageModel> {
		const {
			data: response,
			error,
			response: rawResponse,
		} = await fetchProducts(this.baseUrl, params);

		if (error || !response) {
			throw {
				status: rawResponse.status,
				statusText: rawResponse.statusText,
				body: error,
			};
		}

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
		const { data: response, error, response: rawResponse } = await fetchProduct(this.baseUrl, id);
		if (error || !response) {
			throw {
				status: rawResponse.status,
				statusText: rawResponse.statusText,
				body: error,
			};
		}

		const Product: ProductModel = parseProduct(response);

		return Product;
	}

	// Cart Operations
	public async viewCart(): Promise<CartModel> {
		const response = await this.request<CartPayload>("GET", "/me/cart");
		const cart = parseCart(response);

		return cart;
	}

	public async addToCart(data: components["schemas"]["AddCartItemRequest"]): Promise<CartModel> {
		const response = await this.request<CartPayload>("POST", "/me/cart", data);
		const cart = parseCart(response);

		return cart;
	}

	public async updateCartItem(
		itemId: number,
		data: components["schemas"]["UpdateCartItemRequest"]
	): Promise<CartItemModel> {
		const response = await this.request<CartItemPayload>("PATCH", `/me/cart/${itemId}`, data);
		const cartItem = parseCartItem(response);

		return cartItem;
	}

	public async removeCartItem(itemId: number): Promise<MessageResponse> {
		return await this.request("DELETE", `/me/cart/${itemId}`);
	}

	// Profile Management
	public async getProfile(): Promise<UserModel> {
		// There's a weird quirk about how Gin handles the routing so we have to hit `/me/`, not `/me`
		const response = await this.request<ProfileModel>("GET", "/me/");
		this.authenticated = true;
		this.authStateResolved = true;
		return parseProfile(response);
	}

	public async updateProfile(data: UpdateProfileRequest): Promise<UserModel> {
		const response = await this.request<ProfileModel>("PATCH", "/me/", data);
		return parseProfile(response);
	}

	public async uploadMedia(file: File): Promise<string> {
		const uploadUrl = new URL(`${this.baseUrl}${API_ROUTE}/media/uploads`);
		const metadata = `filename ${btoa(unescape(encodeURIComponent(file.name)))}`;

		const createResponse = await fetch(uploadUrl.toString(), {
			method: "POST",
			headers: {
				"Tus-Resumable": "1.0.0",
				"Upload-Length": String(file.size),
				"Upload-Metadata": metadata,
				...(this.readCookie("csrf_token") ? { "X-CSRF-Token": this.readCookie("csrf_token") } : {}),
			},
			credentials: "include",
		});

		if (!createResponse.ok) {
			const createText = await createResponse.text();
			let createBody: unknown = createText;
			try {
				createBody = createText ? JSON.parse(createText) : null;
			} catch {
				createBody = createText;
			}
			this.handleCsrfForbidden(createResponse.status, createBody);
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
				...(this.readCookie("csrf_token") ? { "X-CSRF-Token": this.readCookie("csrf_token") } : {}),
			},
			body: file,
			credentials: "include",
		});

		if (!patchResponse.ok) {
			const patchText = await patchResponse.text();
			let patchBody: unknown = patchText;
			try {
				patchBody = patchText ? JSON.parse(patchText) : null;
			} catch {
				patchBody = patchText;
			}
			this.handleCsrfForbidden(patchResponse.status, patchBody);
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

	public async attachProfilePhoto(mediaId: string): Promise<UserModel> {
		const response = await this.request<ProfileModel>("POST", "/me/profile-photo", {
			media_id: mediaId,
		});
		return parseProfile(response);
	}

	public async removeProfilePhoto(): Promise<UserModel> {
		const response = await this.request<ProfileModel>("DELETE", "/me/profile-photo");
		return parseProfile(response);
	}

	// Saved Payment Methods
	public async listSavedPaymentMethods(): Promise<SavedPaymentMethodModel[]> {
		const response = await this.request<components["schemas"]["SavedPaymentMethod"][]>(
			"GET",
			"/me/payment-methods"
		);
		return response.map(parseSavedPaymentMethod);
	}

	public async createSavedPaymentMethod(
		data: CreateSavedPaymentMethodRequest
	): Promise<SavedPaymentMethodModel> {
		const response = await this.request<components["schemas"]["SavedPaymentMethod"]>(
			"POST",
			"/me/payment-methods",
			data
		);
		return parseSavedPaymentMethod(response);
	}

	public async deleteSavedPaymentMethod(id: number): Promise<MessageResponse> {
		return await this.request("DELETE", `/me/payment-methods/${id}`);
	}

	public async setDefaultPaymentMethod(id: number): Promise<SavedPaymentMethodModel> {
		const response = await this.request<components["schemas"]["SavedPaymentMethod"]>(
			"PATCH",
			`/me/payment-methods/${id}/default`
		);
		return parseSavedPaymentMethod(response);
	}

	// Saved Addresses
	public async listSavedAddresses(): Promise<SavedAddressModel[]> {
		const response = await this.request<components["schemas"]["SavedAddress"][]>(
			"GET",
			"/me/addresses"
		);
		return response.map(parseSavedAddress);
	}

	public async createSavedAddress(data: CreateSavedAddressRequest): Promise<SavedAddressModel> {
		const response = await this.request<components["schemas"]["SavedAddress"]>(
			"POST",
			"/me/addresses",
			data
		);
		return parseSavedAddress(response);
	}

	public async deleteSavedAddress(id: number): Promise<MessageResponse> {
		return await this.request("DELETE", `/me/addresses/${id}`);
	}

	public async setDefaultAddress(id: number): Promise<SavedAddressModel> {
		const response = await this.request<components["schemas"]["SavedAddress"]>(
			"PATCH",
			`/me/addresses/${id}/default`
		);
		return parseSavedAddress(response);
	}

	// Admin Operations
	public async createProduct(data: ProductInput): Promise<ProductModel> {
		const response = await this.request<components["schemas"]["Product"]>(
			"POST",
			"/admin/products",
			data
		);
		return parseProduct(response);
	}

	public async updateProduct(id: number, data: ProductInput): Promise<ProductModel> {
		const response = await this.request<components["schemas"]["Product"]>(
			"PATCH",
			`/admin/products/${id}`,
			data
		);
		return parseProduct(response);
	}

	public async listAdminProducts(params?: ListAdminProductsQuery): Promise<PageModel> {
		const response = await this.request<components["schemas"]["ProductPage"]>(
			"GET",
			"/admin/products",
			undefined,
			params as Record<string, unknown>
		);
		return {
			data: response.data.map(parseProduct),
			pagination: response.pagination,
		};
	}

	public async getAdminProduct(id: number): Promise<ProductModel> {
		const response = await this.request<components["schemas"]["Product"]>(
			"GET",
			`/admin/products/${id}`
		);
		return parseProduct(response);
	}

	public async deleteProduct(id: number): Promise<MessageResponse> {
		return await this.request("DELETE", `/admin/products/${id}`);
	}

	public async attachProductMedia(id: number, mediaIds: string[]): Promise<ProductModel> {
		const payload: MediaIDsRequest = { media_ids: mediaIds };
		const response = await this.request<components["schemas"]["Product"]>(
			"POST",
			`/admin/products/${id}/media`,
			payload
		);
		return parseProduct(response);
	}

	public async detachProductMedia(id: number, mediaId: string): Promise<ProductModel> {
		const response = await this.request<components["schemas"]["Product"]>(
			"DELETE",
			`/admin/products/${id}/media/${mediaId}`
		);
		return parseProduct(response);
	}

	public async updateProductMediaOrder(id: number, mediaIds: string[]): Promise<ProductModel> {
		const payload: MediaIDsRequest = { media_ids: mediaIds };
		const response = await this.request<components["schemas"]["Product"]>(
			"PATCH",
			`/admin/products/${id}/media/order`,
			payload
		);
		return parseProduct(response);
	}

	public async updateProductRelated(id: number, relatedIds: number[]): Promise<ProductModel> {
		const payload: UpdateRelatedRequest = { related_ids: relatedIds };
		const response = await this.request<components["schemas"]["Product"]>(
			"PATCH",
			`/admin/products/${id}/related`,
			payload
		);
		return parseProduct(response);
	}

	public async publishProduct(id: number): Promise<ProductModel> {
		const response = await this.request<components["schemas"]["Product"]>(
			"POST",
			`/admin/products/${id}/publish`
		);
		return parseProduct(response);
	}

	public async unpublishProduct(id: number): Promise<ProductModel> {
		const response = await this.request<components["schemas"]["Product"]>(
			"POST",
			`/admin/products/${id}/unpublish`
		);
		return parseProduct(response);
	}

	public async discardProductDraft(id: number): Promise<ProductModel> {
		const response = await this.request<components["schemas"]["Product"]>(
			"DELETE",
			`/admin/products/${id}/draft`
		);
		return parseProduct(response);
	}

	public async getStorefrontSettings(): Promise<StorefrontSettingsResponseModel> {
		const response = await this.request<StorefrontSettingsResponse>("GET", "/storefront");
		return parseStorefrontSettingsResponse(response);
	}

	public async getAdminStorefrontSettings(): Promise<StorefrontSettingsResponseModel> {
		const response = await this.request<StorefrontSettingsResponse>("GET", "/admin/storefront");
		return parseStorefrontSettingsResponse(response);
	}

	public async updateStorefrontSettings(
		settings: StorefrontSettingsModel
	): Promise<StorefrontSettingsResponseModel> {
		const payload: StorefrontSettingsRequest = { settings };
		const response = await this.request<StorefrontSettingsResponse>(
			"PUT",
			"/admin/storefront",
			payload
		);
		const parsed = parseStorefrontSettingsResponse(response);
		broadcastStorefrontStateChange();
		return parsed;
	}

	public async publishStorefrontSettings(): Promise<StorefrontSettingsResponseModel> {
		const response = await this.request<StorefrontSettingsResponse>(
			"POST",
			"/admin/storefront/publish"
		);
		const parsed = parseStorefrontSettingsResponse(response);
		broadcastStorefrontStateChange();
		return parsed;
	}

	public async discardStorefrontDraft(): Promise<StorefrontSettingsResponseModel> {
		const response = await this.request<StorefrontSettingsResponse>(
			"DELETE",
			"/admin/storefront/draft"
		);
		const parsed = parseStorefrontSettingsResponse(response);
		broadcastStorefrontStateChange();
		return parsed;
	}

	public async getAdminPreviewSession(): Promise<DraftPreviewSessionModel> {
		const response = await this.request<DraftPreviewSessionResponse>("GET", "/admin/preview");
		return parseDraftPreviewSession(response);
	}

	public async startAdminPreview(): Promise<DraftPreviewSessionModel> {
		const response = await this.request<DraftPreviewSessionResponse>(
			"POST",
			"/admin/preview/start"
		);
		const session = parseDraftPreviewSession(response);
		broadcastDraftPreviewState(session);
		return session;
	}

	public async stopAdminPreview(): Promise<DraftPreviewSessionModel> {
		const response = await this.request<DraftPreviewSessionResponse>("POST", "/admin/preview/stop");
		const session = parseDraftPreviewSession(response);
		broadcastDraftPreviewState(session);
		return session;
	}

	// Order Management
	public async listOrders(
		params?: ListOrdersParams
	): Promise<{ data: OrderModel[]; pagination: OrderPagePayload["pagination"] }> {
		const query = {
			...params,
			status: params?.status === "" ? undefined : params?.status,
		};
		const response = await this.request<OrderPagePayload>("GET", "/me/orders", undefined, query);
		return {
			data: response.data.map(parseOrder),
			pagination: response.pagination,
		};
	}

	public async getOrderDetails(orderId: number): Promise<OrderModel> {
		const response = await this.request<OrderPayload>("GET", `/me/orders/${orderId}`);
		return parseOrder(response);
	}

	// Admin Order Management
	public async listAdminOrders(
		params?: ListAdminOrdersQuery
	): Promise<{ data: OrderModel[]; pagination: OrderPagePayload["pagination"] }> {
		const response = await this.request<OrderPagePayload>(
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
		const response = await this.request<OrderPayload>("GET", `/admin/orders/${orderId}`);
		return parseOrder(response);
	}

	public async updateOrderStatus(
		orderId: number,
		data: UpdateOrderStatusRequest
	): Promise<OrderModel> {
		const response = await this.request<OrderPayload>(
			"PATCH",
			`/admin/orders/${orderId}/status`,
			data
		);
		return parseOrder(response);
	}

	// Admin User Management
	public async listUsers(
		params?: ListUsersQuery
	): Promise<{ data: UserModel[]; pagination: UserPagePayload["pagination"] }> {
		const response = await this.request<UserPagePayload>("GET", "/admin/users", undefined, params);
		return {
			data: response.data.map(parseProfile),
			pagination: response.pagination,
		};
	}

	public async updateUserRole(userId: number, data: { role: string }): Promise<UserModel> {
		const response = await this.request<ProfileModel>("PATCH", `/admin/users/${userId}/role`, data);
		return parseProfile(response);
	}

	public async refreshAuthState(): Promise<boolean> {
		if (this.authStateResolved) {
			return this.authenticated;
		}

		try {
			await this.getProfile();
			return true;
		} catch (err) {
			const error = err as { status?: number };
			if (error.status === 401) {
				this.authenticated = false;
				this.authStateResolved = true;
				return false;
			}
			throw err;
		}
	}

	public isAuthenticated() {
		return this.authenticated;
	}
}
