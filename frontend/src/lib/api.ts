import {
	type BrandModel,
	type CategoryModel,
	type ProductModel,
	type ProductAttributeDefinitionModel,
	type UserModel,
	type PageModel,
	type OrderModel,
	type CartModel,
	type CartItemModel,
	type ProfileModel,
	type OrderPayload,
	type SavedPaymentMethodModel,
	type SavedAddressModel,
	parseBrand,
	parseCategory,
	parseProductAttributeDefinition,
	parseProduct,
	parseOrder,
	parseProfile,
} from "$lib/models";
import { API_BASE_URL } from "$lib/config";
import { fetchProduct, fetchProducts, type ListProductsQuery } from "$lib/api/openapi-client";
import { buildOIDCLoginUrl } from "$lib/auth";
import { appendQueryParams } from "$lib/api/http";
import type { components, paths } from "$lib/api/generated/openapi";
import * as cartDomain from "$lib/api/domains/cart";
import * as ordersDomain from "$lib/api/domains/orders";
import * as profileDomain from "$lib/api/domains/profile";

const API_ROUTE = "/api/v1";
const TUS_VERSION = "1.0.0";
const TUS_CHUNK_SIZE = 5 * 1024 * 1024;
const TUS_MAX_RESUME_ATTEMPTS = 3;
export const DRAFT_PREVIEW_SYNC_EVENT = "draft-preview:changed";
export const DRAFT_PREVIEW_SYNC_STORAGE_KEY = "draft-preview:state";
export const STOREFRONT_SYNC_EVENT = "storefront:changed";
export const STOREFRONT_SYNC_STORAGE_KEY = "storefront:state";

type RegisterRequest = components["schemas"]["RegisterRequest"];
type LoginRequest = components["schemas"]["LoginRequest"];
type AuthResponse = components["schemas"]["AuthResponse"];
type CheckoutPluginCatalog = components["schemas"]["CheckoutPluginCatalog"];
type CheckoutPluginType = components["schemas"]["CheckoutPlugin"]["type"];
type UpdateCheckoutPluginRequest = components["schemas"]["UpdateCheckoutPluginRequest"];
type ProviderCredential = components["schemas"]["ProviderCredential"];
type ProviderCredentialRequest = components["schemas"]["ProviderCredentialRequest"];
type ProviderCredentialListResponse = components["schemas"]["ProviderCredentialListResponse"];
type ProviderOperationsOverview = components["schemas"]["ProviderOperationsOverview"];
type ProviderReconciliationRun = components["schemas"]["ProviderReconciliationRun"];
type ProviderReconciliationRunRequest = components["schemas"]["ProviderReconciliationRunRequest"];
type ProviderReconciliationRunPage = components["schemas"]["ProviderReconciliationRunPage"];
type WebhookEventPage = components["schemas"]["WebhookEventPage"];
type InventoryReservationList = components["schemas"]["InventoryReservationList"];
type InventoryAlert = components["schemas"]["InventoryAlert"];
type InventoryAlertList = components["schemas"]["InventoryAlertList"];
type InventoryThreshold = components["schemas"]["InventoryThreshold"];
type InventoryThresholdList = components["schemas"]["InventoryThresholdList"];
type InventoryThresholdRequest = components["schemas"]["InventoryThresholdRequest"];
type InventoryAdjustmentRequest = components["schemas"]["InventoryAdjustmentRequest"];
type InventoryAdjustmentResponse = components["schemas"]["InventoryAdjustmentResponse"];
type InventoryReconciliationReport = components["schemas"]["InventoryReconciliationReport"];
type DiscountCampaign = components["schemas"]["DiscountCampaign"];
type DiscountCampaignListResponse = components["schemas"]["DiscountCampaignListResponse"];
type DiscountSchedule = components["schemas"]["DiscountSchedule"];
type DiscountScheduleInput = components["schemas"]["DiscountScheduleInput"];
type DiscountLifecycleRunResponse = components["schemas"]["DiscountLifecycleRunResponse"];
type DiscountStateHistoryListResponse = components["schemas"]["DiscountStateHistoryListResponse"];
type DiscountCampaignAuditListResponse = components["schemas"]["DiscountCampaignAuditListResponse"];
type DiscountEvaluationMetrics = components["schemas"]["DiscountEvaluationMetrics"];
type DiscountReconciliationReport = components["schemas"]["DiscountReconciliationReport"];
type ProductDiscountInput = components["schemas"]["ProductDiscountInput"];
type PromotionInput = components["schemas"]["PromotionInput"];
type PromotionEvaluationRequest = components["schemas"]["PromotionEvaluationRequest"];
type PromotionEvaluationResponse = components["schemas"]["PromotionEvaluationResponse"];
type PromotionTemplate = components["schemas"]["PromotionTemplate"];
type PromotionTemplateInput = components["schemas"]["PromotionTemplateInput"];
type PromotionTemplateInstantiateInput = components["schemas"]["PromotionTemplateInstantiateInput"];
type PromotionTemplateListResponse = components["schemas"]["PromotionTemplateListResponse"];
type InventoryTimeline = components["schemas"]["InventoryTimeline"];
type PurchaseOrder = components["schemas"]["PurchaseOrder"];
type PurchaseOrderList = components["schemas"]["PurchaseOrderList"];
type PurchaseOrderRequest = components["schemas"]["PurchaseOrderRequest"];
type PurchaseOrderReceiveRequest = components["schemas"]["PurchaseOrderReceiveRequest"];
type PurchaseOrderReceiptResponse = components["schemas"]["PurchaseOrderReceiptResponse"];
type MessageResponse = components["schemas"]["MessageResponse"];
type ProductUpsertInput = components["schemas"]["ProductUpsertInput"];
type MediaIDsRequest = components["schemas"]["MediaIDsRequest"];
type UpdateRelatedRequest = components["schemas"]["UpdateRelatedRequest"];
type BrandListResponse = components["schemas"]["BrandListResponse"];
type CategoryListResponse = components["schemas"]["CategoryListResponse"];
type ProductAttributeDefinitionListResponse =
	components["schemas"]["ProductAttributeDefinitionListResponse"];
type BrandInput = components["schemas"]["BrandInput"];
type CategoryInput = components["schemas"]["CategoryInput"];
type ProductAttributeDefinitionInput = components["schemas"]["ProductAttributeDefinitionInput"];
type OrderPagePayload = components["schemas"]["OrderPage"];
type OrderPaymentLedger = components["schemas"]["OrderPaymentLedger"];
type AdminOrderPaymentAmountRequest = components["schemas"]["AdminOrderPaymentAmountRequest"];
type AdminOrderPaymentLifecycleResponse =
	components["schemas"]["AdminOrderPaymentLifecycleResponse"];
type AdminOrderPaymentLifecycleModel = Omit<AdminOrderPaymentLifecycleResponse, "order"> & {
	order: OrderModel;
};
type UserPagePayload = components["schemas"]["UserPage"];
type UpdateOrderStatusRequest = components["schemas"]["UpdateOrderStatusRequest"];
type WebsiteSettings = components["schemas"]["WebsiteSettings"];
type WebsiteSettingsRequest = components["schemas"]["WebsiteSettingsRequest"];
type WebsiteSettingsResponse = components["schemas"]["WebsiteSettingsResponse"];
type DraftPreviewSessionResponse = components["schemas"]["DraftPreviewSessionResponse"];
type CmsPageListResponse = components["schemas"]["CmsPageListResponse"];
type CmsPageResponse = components["schemas"]["CmsPageResponse"];
type CmsPageDraftRequest = components["schemas"]["CmsPageDraftRequest"];
type CmsNavigationListResponse = components["schemas"]["CmsNavigationListResponse"];
type CmsNavigationResponse = components["schemas"]["CmsNavigationResponse"];
type CmsNavigationDraftRequest = components["schemas"]["CmsNavigationDraftRequest"];
type CmsGlobalRegionListResponse = components["schemas"]["CmsGlobalRegionListResponse"];
type CmsGlobalRegionResponse = components["schemas"]["CmsGlobalRegionResponse"];
type CmsGlobalRegionDraftRequest = components["schemas"]["CmsGlobalRegionDraftRequest"];
type CmsPublishRequest = components["schemas"]["CmsPublishRequest"];
type CmsPreviewRequest = components["schemas"]["CmsPreviewRequest"];
type CmsPreviewResponse = components["schemas"]["CmsPreviewResponse"];
type CmsPageDeliveryRequest = components["schemas"]["CmsPageDeliveryRequest"];
type CmsPageDeliveryResponse = components["schemas"]["CmsPageDeliveryResponse"];
type CmsContentEventRequest = components["schemas"]["CmsContentEventRequest"];
type CmsSEOInput = components["schemas"]["CmsSEOInput"];
type CmsSEOResponse = components["schemas"]["CmsSEOResponse"];
type CmsRedirectInput = components["schemas"]["CmsRedirectInput"];
type CmsRedirectRule = components["schemas"]["CmsRedirectRule"];
type CmsLocaleSettings = components["schemas"]["CmsLocaleSettings"];
type CmsLocaleSettingsInput = components["schemas"]["CmsLocaleSettingsInput"];
type CmsPageVariant = components["schemas"]["CmsPageVariant"];
type CmsPageVariantInput = components["schemas"]["CmsPageVariantInput"];
type CmsAuditEvent = components["schemas"]["CmsAuditEvent"];
type CmsContentExport = components["schemas"]["CmsContentExport"];
type CmsRestorePreview = components["schemas"]["CmsRestorePreview"];
type CmsGovernance = components["schemas"]["CmsGovernance"];
type CmsGovernanceInput = components["schemas"]["CmsGovernanceInput"];
type CmsOperations = components["schemas"]["CmsOperations"];
type ListUserOrdersQuery = paths["/api/v1/me/orders"]["get"]["parameters"]["query"];
type ListAdminBrandsQuery = NonNullable<
	paths["/api/v1/admin/brands"]["get"]["parameters"]["query"]
>;
type ListAdminCategoriesQuery = NonNullable<
	paths["/api/v1/admin/categories"]["get"]["parameters"]["query"]
>;
type ListAdminOrdersQuery = paths["/api/v1/admin/orders"]["get"]["parameters"]["query"];
type ListAdminProductsQuery = paths["/api/v1/admin/products"]["get"]["parameters"]["query"];
type ListUsersQuery = paths["/api/v1/admin/users"]["get"]["parameters"]["query"];
type ListAdminProviderCredentialsQuery =
	paths["/api/v1/admin/providers/credentials"]["get"]["parameters"]["query"];
type ListAdminWebhookEventsQuery =
	paths["/api/v1/admin/webhooks/events"]["get"]["parameters"]["query"];
type ListAdminProviderReconciliationRunsQuery =
	paths["/api/v1/admin/providers/reconciliation/runs"]["get"]["parameters"]["query"];
type ListAdminInventoryReservationsQuery =
	paths["/api/v1/admin/inventory/reservations"]["get"]["parameters"]["query"];
type ListAdminInventoryAlertsQuery =
	paths["/api/v1/admin/inventory/alerts"]["get"]["parameters"]["query"];
type ListAdminInventoryThresholdsQuery =
	paths["/api/v1/admin/inventory/thresholds"]["get"]["parameters"]["query"];
type GetAdminInventoryTimelineQuery =
	paths["/api/v1/admin/inventory/variants/{product_variant_id}/timeline"]["get"]["parameters"]["query"];
type ListAdminPurchaseOrdersQuery =
	paths["/api/v1/admin/purchase-orders"]["get"]["parameters"]["query"];
type ListAdminDiscountCampaignsQuery = NonNullable<
	paths["/api/v1/admin/discounts/campaigns"]["get"]["parameters"]["query"]
>;
type ListAdminPromotionTemplatesQuery = NonNullable<
	paths["/api/v1/admin/discounts/templates"]["get"]["parameters"]["query"]
>;
type ListAdminDiscountHistoryQuery = NonNullable<
	paths["/api/v1/admin/discounts/history"]["get"]["parameters"]["query"]
>;
type ListAdminDiscountAuditQuery = NonNullable<
	paths["/api/v1/admin/discounts/audit"]["get"]["parameters"]["query"]
>;
export type ListOrdersParams = Omit<NonNullable<ListUserOrdersQuery>, "status"> & {
	status?: NonNullable<ListUserOrdersQuery>["status"] | "";
};

export interface DraftPreviewSessionModel {
	active: boolean;
	expires_at: Date | null;
}

export interface RequestOptions {
	headers?: Record<string, string>;
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
			detail: {
				active: session.active,
				expires_at: session.expires_at?.toISOString() ?? null,
			},
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

	public bootstrapAuthState(authenticated: boolean): void {
		this.authenticated = authenticated;
		this.authStateResolved = true;
	}

	private async request<T>(
		method: string,
		path: string,
		data?: object,
		params?: Record<string, unknown>,
		options?: RequestOptions
	): Promise<T> {
		const headers = new Headers();
		headers.append("Content-Type", "application/json");
		if (method !== "GET" && method !== "HEAD" && method !== "OPTIONS") {
			const csrfToken = this.readCookie("csrf_token");
			if (csrfToken) {
				headers.set("X-CSRF-Token", csrfToken);
			}
		}
		for (const [key, value] of Object.entries(options?.headers ?? {})) {
			headers.set(key, value);
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

	public buildOIDCLoginURL(redirectPath?: string): string {
		return buildOIDCLoginUrl(this.baseUrl, redirectPath);
	}

	public async createOrder(
		data: components["schemas"]["CreateCheckoutOrderRequest"],
		idempotencyKey?: string
	): Promise<OrderModel> {
		return ordersDomain.createOrder(this.request.bind(this), data, idempotencyKey);
	}

	public async processPayment(
		orderId: number,
		data: components["schemas"]["AuthorizeCheckoutOrderPaymentRequest"],
		idempotencyKey?: string
	): Promise<OrderModel> {
		return ordersDomain.processPayment(this.request.bind(this), orderId, data, idempotencyKey);
	}

	public async listCheckoutPlugins(): Promise<components["schemas"]["CheckoutPluginCatalog"]> {
		return ordersDomain.listCheckoutPlugins(this.request.bind(this));
	}

	public async quoteCheckout(
		data: components["schemas"]["CheckoutQuoteRequest"]
	): Promise<components["schemas"]["CheckoutQuoteResponse"]> {
		return ordersDomain.quoteCheckout(this.request.bind(this), data);
	}

	public async quoteOrderShippingRates(
		orderId: number,
		data: components["schemas"]["CheckoutOrderShippingRatesRequest"],
		idempotencyKey?: string
	): Promise<components["schemas"]["CheckoutOrderShippingRatesResponse"]> {
		return ordersDomain.quoteOrderShippingRates(
			this.request.bind(this),
			orderId,
			data,
			idempotencyKey
		);
	}

	public async finalizeOrderTax(
		orderId: number,
		data: components["schemas"]["CheckoutOrderTaxFinalizeRequest"],
		idempotencyKey?: string
	): Promise<components["schemas"]["CheckoutOrderTaxFinalizeResponse"]> {
		return ordersDomain.finalizeOrderTax(this.request.bind(this), orderId, data, idempotencyKey);
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
	public async viewCart() {
		return cartDomain.viewCart(this.request.bind(this));
	}

	public async viewCartSummary(): Promise<number> {
		return cartDomain.viewCartSummary(this.request.bind(this));
	}

	public async addToCart(data: components["schemas"]["AddCartItemRequest"]): Promise<CartModel> {
		return cartDomain.addToCart(this.request.bind(this), data);
	}

	public async updateCartItem(
		itemId: number,
		data: components["schemas"]["UpdateCartItemRequest"]
	): Promise<CartItemModel> {
		return cartDomain.updateCartItem(this.request.bind(this), itemId, data);
	}

	public async removeCartItem(itemId: number): Promise<MessageResponse> {
		return cartDomain.removeCartItem(this.request.bind(this), itemId);
	}

	// Profile Management
	public async getProfile(): Promise<UserModel> {
		const response = await profileDomain.getProfile(this.request.bind(this));
		this.authenticated = true;
		this.authStateResolved = true;
		return response;
	}

	public async updateProfile(
		data: components["schemas"]["UpdateProfileRequest"]
	): Promise<UserModel> {
		return profileDomain.updateProfile(this.request.bind(this), data);
	}

	public async uploadMedia(file: File): Promise<string> {
		const storageKey = this.mediaUploadStorageKey(file);
		let uploadLocation = this.readStoredUploadLocation(storageKey);
		let offset = uploadLocation ? await this.getUploadOffset(uploadLocation, file.size) : null;

		if (offset === null) {
			uploadLocation = await this.createMediaUpload(file);
			offset = 0;
			this.storeUploadLocation(storageKey, uploadLocation);
		}

		if (!uploadLocation) {
			throw new Error("Upload location missing");
		}
		const activeUploadLocation = uploadLocation;

		let resumeAttempts = 0;
		while (offset < file.size) {
			let response: Response;
			try {
				response = await fetch(activeUploadLocation, {
					method: "PATCH",
					headers: {
						"Tus-Resumable": TUS_VERSION,
						"Upload-Offset": String(offset),
						"Content-Type": "application/offset+octet-stream",
						...this.csrfHeader(),
					},
					body: file.slice(offset, Math.min(offset + TUS_CHUNK_SIZE, file.size)),
					credentials: "include",
				});
			} catch (error) {
				if (resumeAttempts >= TUS_MAX_RESUME_ATTEMPTS) throw error;
				const resumedOffset = await this.getUploadOffset(activeUploadLocation, file.size);
				if (resumedOffset === null) throw error;
				offset = resumedOffset;
				resumeAttempts += 1;
				continue;
			}

			if (response.ok) {
				const nextOffset = this.parseUploadOffset(response.headers.get("Upload-Offset"), file.size);
				if (nextOffset === null || nextOffset <= offset) {
					throw new Error("Upload server returned an invalid offset");
				}
				offset = nextOffset;
				resumeAttempts = 0;
				continue;
			}

			await this.handleUploadResponseError(response, "Failed to upload media");
			if (resumeAttempts >= TUS_MAX_RESUME_ATTEMPTS) {
				throw new Error(`Failed to upload media: ${response.statusText}`);
			}
			const resumedOffset = await this.getUploadOffset(activeUploadLocation, file.size);
			if (resumedOffset === null) {
				throw new Error(`Failed to upload media: ${response.statusText}`);
			}
			offset = resumedOffset;
			resumeAttempts += 1;
		}

		this.removeStoredUploadLocation(storageKey);
		const segments = new URL(activeUploadLocation, this.baseUrl).pathname
			.split("/")
			.filter(Boolean);
		const mediaId = segments.at(-1);
		if (!mediaId) {
			throw new Error("Upload ID missing");
		}
		return mediaId;
	}

	private async createMediaUpload(file: File): Promise<string> {
		const metadata = `filename ${btoa(unescape(encodeURIComponent(file.name)))}`;
		const response = await fetch(new URL(`${this.baseUrl}${API_ROUTE}/media/uploads`).toString(), {
			method: "POST",
			headers: {
				"Tus-Resumable": TUS_VERSION,
				"Upload-Length": String(file.size),
				"Upload-Metadata": metadata,
				...this.csrfHeader(),
			},
			credentials: "include",
		});
		await this.handleUploadResponseError(response, "Failed to create upload");
		if (!response.ok) throw new Error(`Failed to create upload: ${response.statusText}`);

		const location = response.headers.get("Location");
		if (!location) throw new Error("Upload location missing");
		return new URL(location, this.baseUrl).toString();
	}

	private async getUploadOffset(location: string, size: number): Promise<number | null> {
		try {
			const response = await fetch(location, {
				method: "HEAD",
				headers: { "Tus-Resumable": TUS_VERSION, ...this.csrfHeader() },
				credentials: "include",
			});
			if (!response.ok) return null;
			return this.parseUploadOffset(response.headers.get("Upload-Offset"), size);
		} catch {
			return null;
		}
	}

	private parseUploadOffset(value: string | null, size: number): number | null {
		if (!value || !/^\d+$/.test(value)) return null;
		const offset = Number(value);
		return Number.isSafeInteger(offset) && offset >= 0 && offset <= size ? offset : null;
	}

	private shouldResumeUpload(status: number): boolean {
		return status === 409 || status >= 500;
	}

	private async handleUploadResponseError(response: Response, message: string): Promise<void> {
		if (response.ok) return;

		const text = await response.text();
		let body: unknown = text;
		try {
			body = text ? JSON.parse(text) : null;
		} catch {
			// Keep non-JSON tus error responses as text.
		}
		this.handleCsrfForbidden(response.status, body);
		if (!this.shouldResumeUpload(response.status)) {
			throw new Error(`${message}: ${response.statusText}`);
		}
	}

	private csrfHeader(): Record<string, string> {
		const token = this.readCookie("csrf_token");
		return token ? { "X-CSRF-Token": token } : {};
	}

	private mediaUploadStorageKey(file: File): string {
		return `media-upload:${this.baseUrl}:${file.name}:${file.size}:${file.lastModified}:${file.type}`;
	}

	private readStoredUploadLocation(key: string): string | null {
		try {
			return typeof window === "undefined" ? null : window.localStorage.getItem(key);
		} catch {
			return null;
		}
	}

	private storeUploadLocation(key: string, location: string): void {
		try {
			if (typeof window !== "undefined") window.localStorage.setItem(key, location);
		} catch {
			// Uploads remain resumable in this page when storage is unavailable.
		}
	}

	private removeStoredUploadLocation(key: string): void {
		try {
			if (typeof window !== "undefined") window.localStorage.removeItem(key);
		} catch {
			// Nothing to clean up when storage is unavailable.
		}
	}

	public async attachProfilePhoto(mediaId: string): Promise<UserModel> {
		return profileDomain.attachProfilePhoto(this.request.bind(this), mediaId);
	}

	public async removeProfilePhoto(): Promise<UserModel> {
		return profileDomain.removeProfilePhoto(this.request.bind(this));
	}

	// Saved Payment Methods
	public async listSavedPaymentMethods(): Promise<SavedPaymentMethodModel[]> {
		return profileDomain.listSavedPaymentMethods(this.request.bind(this));
	}

	public async createSavedPaymentMethod(
		data: components["schemas"]["CreateSavedPaymentMethodRequest"]
	): Promise<SavedPaymentMethodModel> {
		return profileDomain.createSavedPaymentMethod(this.request.bind(this), data);
	}

	public async deleteSavedPaymentMethod(id: number): Promise<MessageResponse> {
		return profileDomain.deleteSavedPaymentMethod(this.request.bind(this), id);
	}

	public async setDefaultPaymentMethod(id: number): Promise<SavedPaymentMethodModel> {
		return profileDomain.setDefaultPaymentMethod(this.request.bind(this), id);
	}

	// Saved Addresses
	public async listSavedAddresses(): Promise<SavedAddressModel[]> {
		return profileDomain.listSavedAddresses(this.request.bind(this));
	}

	public async createSavedAddress(
		data: components["schemas"]["CreateSavedAddressRequest"]
	): Promise<SavedAddressModel> {
		return profileDomain.createSavedAddress(this.request.bind(this), data);
	}

	public async deleteSavedAddress(id: number): Promise<MessageResponse> {
		return profileDomain.deleteSavedAddress(this.request.bind(this), id);
	}

	public async setDefaultAddress(id: number): Promise<SavedAddressModel> {
		return profileDomain.setDefaultAddress(this.request.bind(this), id);
	}

	// Admin Operations
	public async listAdminCheckoutPlugins(): Promise<CheckoutPluginCatalog> {
		return await this.request<CheckoutPluginCatalog>("GET", "/admin/checkout/plugins");
	}

	public async updateAdminCheckoutPlugin(
		type: CheckoutPluginType,
		id: string,
		data: UpdateCheckoutPluginRequest
	): Promise<CheckoutPluginCatalog> {
		return await this.request<CheckoutPluginCatalog>(
			"PATCH",
			`/admin/checkout/plugins/${type}/${id}`,
			data
		);
	}

	public async listAdminProviderCredentials(
		params: ListAdminProviderCredentialsQuery = {}
	): Promise<ProviderCredential[]> {
		const response = await this.request<ProviderCredentialListResponse>(
			"GET",
			"/admin/providers/credentials",
			undefined,
			params as Record<string, unknown>
		);
		return response.data;
	}

	public async upsertAdminProviderCredential(
		data: ProviderCredentialRequest
	): Promise<ProviderCredential> {
		const response = await this.request<components["schemas"]["ProviderCredentialEnvelope"]>(
			"POST",
			"/admin/providers/credentials",
			data
		);
		return response.credential;
	}

	public async rotateAdminProviderCredential(id: number): Promise<ProviderCredential> {
		const response = await this.request<components["schemas"]["ProviderCredentialEnvelope"]>(
			"POST",
			`/admin/providers/credentials/${id}/rotate`
		);
		return response.credential;
	}

	public async getAdminProviderOperationsOverview(): Promise<ProviderOperationsOverview> {
		return await this.request<ProviderOperationsOverview>("GET", "/admin/providers/overview");
	}

	public async listAdminWebhookEvents(
		params: ListAdminWebhookEventsQuery = {}
	): Promise<WebhookEventPage> {
		return await this.request<WebhookEventPage>(
			"GET",
			"/admin/webhooks/events",
			undefined,
			params as Record<string, unknown>
		);
	}

	public async listAdminProviderReconciliationRuns(
		params: ListAdminProviderReconciliationRunsQuery = {}
	): Promise<ProviderReconciliationRunPage> {
		return await this.request<ProviderReconciliationRunPage>(
			"GET",
			"/admin/providers/reconciliation/runs",
			undefined,
			params as Record<string, unknown>
		);
	}

	public async listAdminInventoryReservations(
		params: ListAdminInventoryReservationsQuery = {}
	): Promise<InventoryReservationList> {
		return await this.request<InventoryReservationList>(
			"GET",
			"/admin/inventory/reservations",
			undefined,
			params as Record<string, unknown>
		);
	}

	public async listAdminInventoryAlerts(
		params: ListAdminInventoryAlertsQuery = {}
	): Promise<InventoryAlertList> {
		return await this.request<InventoryAlertList>(
			"GET",
			"/admin/inventory/alerts",
			undefined,
			params as Record<string, unknown>
		);
	}

	public async ackAdminInventoryAlert(id: number): Promise<InventoryAlert> {
		return await this.request<InventoryAlert>("POST", `/admin/inventory/alerts/${id}/ack`);
	}

	public async resolveAdminInventoryAlert(id: number): Promise<InventoryAlert> {
		return await this.request<InventoryAlert>("POST", `/admin/inventory/alerts/${id}/resolve`);
	}

	public async listAdminInventoryThresholds(
		params: ListAdminInventoryThresholdsQuery = {}
	): Promise<InventoryThresholdList> {
		return await this.request<InventoryThresholdList>(
			"GET",
			"/admin/inventory/thresholds",
			undefined,
			params as Record<string, unknown>
		);
	}

	public async upsertAdminInventoryThreshold(
		data: InventoryThresholdRequest
	): Promise<InventoryThreshold> {
		return await this.request<InventoryThreshold>("PUT", "/admin/inventory/thresholds", data);
	}

	public async deleteAdminInventoryThreshold(id: number): Promise<MessageResponse> {
		return await this.request<MessageResponse>("DELETE", `/admin/inventory/thresholds/${id}`);
	}

	public async createAdminInventoryAdjustment(
		data: InventoryAdjustmentRequest
	): Promise<InventoryAdjustmentResponse> {
		return await this.request<InventoryAdjustmentResponse>(
			"POST",
			"/admin/inventory/adjustments",
			data
		);
	}

	public async runAdminInventoryReconciliation(): Promise<InventoryReconciliationReport> {
		return await this.request<InventoryReconciliationReport>(
			"POST",
			"/admin/inventory/reconciliation"
		);
	}

	public async listAdminDiscountCampaigns(
		params: ListAdminDiscountCampaignsQuery = {}
	): Promise<DiscountCampaign[]> {
		const response = await this.request<DiscountCampaignListResponse>(
			"GET",
			"/admin/discounts/campaigns",
			undefined,
			params
		);
		return response.campaigns;
	}

	public async createAdminDiscountCampaign(data: ProductDiscountInput): Promise<DiscountCampaign> {
		return await this.request<DiscountCampaign>("POST", "/admin/discounts/campaigns", data);
	}

	public async updateAdminDiscountCampaign(
		id: number,
		data: ProductDiscountInput
	): Promise<DiscountCampaign> {
		return await this.request<DiscountCampaign>("PATCH", `/admin/discounts/campaigns/${id}`, data);
	}

	public async disableAdminDiscountCampaign(id: number): Promise<DiscountCampaign> {
		return await this.request<DiscountCampaign>("POST", `/admin/discounts/campaigns/${id}/disable`);
	}

	public async scheduleAdminDiscountCampaign(
		id: number,
		data: DiscountScheduleInput
	): Promise<DiscountSchedule> {
		return await this.request<DiscountSchedule>(
			"POST",
			`/admin/discounts/campaigns/${id}/schedule`,
			data
		);
	}

	public async archiveAdminDiscountCampaign(id: number): Promise<DiscountCampaign> {
		return await this.request<DiscountCampaign>("POST", `/admin/discounts/campaigns/${id}/archive`);
	}

	public async createAdminPromotionCampaign(data: PromotionInput): Promise<DiscountCampaign> {
		return await this.request<DiscountCampaign>("POST", "/admin/discounts/promotions", data);
	}

	public async previewAdminPromotion(
		data: PromotionEvaluationRequest
	): Promise<PromotionEvaluationResponse> {
		return await this.request<PromotionEvaluationResponse>(
			"POST",
			"/admin/discounts/promotions/preview",
			data
		);
	}

	public async listAdminPromotionTemplates(
		params: ListAdminPromotionTemplatesQuery = {}
	): Promise<PromotionTemplate[]> {
		const response = await this.request<PromotionTemplateListResponse>(
			"GET",
			"/admin/discounts/templates",
			undefined,
			params
		);
		return response.templates;
	}

	public async createAdminPromotionTemplate(
		data: PromotionTemplateInput
	): Promise<PromotionTemplate> {
		return await this.request<PromotionTemplate>("POST", "/admin/discounts/templates", data);
	}

	public async instantiateAdminPromotionTemplate(
		id: number,
		data: PromotionTemplateInstantiateInput
	): Promise<DiscountCampaign> {
		return await this.request<DiscountCampaign>(
			"POST",
			`/admin/discounts/templates/${id}/instantiate`,
			data
		);
	}

	public async runAdminDiscountLifecycle(): Promise<DiscountLifecycleRunResponse> {
		return await this.request<DiscountLifecycleRunResponse>(
			"POST",
			"/admin/discounts/lifecycle/run"
		);
	}

	public async listAdminDiscountHistory(
		params: ListAdminDiscountHistoryQuery = {}
	): Promise<DiscountStateHistoryListResponse> {
		return await this.request<DiscountStateHistoryListResponse>(
			"GET",
			"/admin/discounts/history",
			undefined,
			params
		);
	}

	public async listAdminDiscountAudit(
		params: ListAdminDiscountAuditQuery = {}
	): Promise<DiscountCampaignAuditListResponse> {
		return await this.request<DiscountCampaignAuditListResponse>(
			"GET",
			"/admin/discounts/audit",
			undefined,
			params
		);
	}

	public async getAdminDiscountMetrics(): Promise<DiscountEvaluationMetrics> {
		return await this.request<DiscountEvaluationMetrics>("GET", "/admin/discounts/metrics");
	}

	public async runAdminDiscountReconciliation(): Promise<DiscountReconciliationReport> {
		return await this.request<DiscountReconciliationReport>(
			"POST",
			"/admin/discounts/reconciliation/run"
		);
	}

	public async getAdminInventoryTimeline(
		productVariantId: number,
		params: GetAdminInventoryTimelineQuery = {}
	): Promise<InventoryTimeline> {
		return await this.request<InventoryTimeline>(
			"GET",
			`/admin/inventory/variants/${productVariantId}/timeline`,
			undefined,
			params as Record<string, unknown>
		);
	}

	public async listAdminPurchaseOrders(
		params: ListAdminPurchaseOrdersQuery = {}
	): Promise<PurchaseOrderList> {
		return await this.request<PurchaseOrderList>(
			"GET",
			"/admin/purchase-orders",
			undefined,
			params as Record<string, unknown>
		);
	}

	public async createAdminPurchaseOrder(data: PurchaseOrderRequest): Promise<PurchaseOrder> {
		return await this.request<PurchaseOrder>("POST", "/admin/purchase-orders", data);
	}

	public async issueAdminPurchaseOrder(id: number): Promise<PurchaseOrder> {
		return await this.request<PurchaseOrder>("POST", `/admin/purchase-orders/${id}/issue`);
	}

	public async cancelAdminPurchaseOrder(id: number): Promise<PurchaseOrder> {
		return await this.request<PurchaseOrder>("POST", `/admin/purchase-orders/${id}/cancel`);
	}

	public async receiveAdminPurchaseOrder(
		id: number,
		data: PurchaseOrderReceiveRequest
	): Promise<PurchaseOrderReceiptResponse> {
		return await this.request<PurchaseOrderReceiptResponse>(
			"POST",
			`/admin/purchase-orders/${id}/receive`,
			data
		);
	}

	public async createAdminProviderReconciliationRun(
		data: ProviderReconciliationRunRequest
	): Promise<ProviderReconciliationRun> {
		const response = await this.request<components["schemas"]["ProviderReconciliationRunEnvelope"]>(
			"POST",
			"/admin/providers/reconciliation/runs",
			data
		);
		return response.run;
	}

	public async getAdminProviderReconciliationRun(id: number): Promise<ProviderReconciliationRun> {
		const response = await this.request<components["schemas"]["ProviderReconciliationRunEnvelope"]>(
			"GET",
			`/admin/providers/reconciliation/runs/${id}`
		);
		return response.run;
	}

	public async createProduct(data: ProductUpsertInput): Promise<ProductModel> {
		const response = await this.request<components["schemas"]["Product"]>(
			"POST",
			"/admin/products",
			data
		);
		return parseProduct(response);
	}

	public async updateProduct(id: number, data: ProductUpsertInput): Promise<ProductModel> {
		const response = await this.request<components["schemas"]["Product"]>(
			"PATCH",
			`/admin/products/${id}`,
			data
		);
		return parseProduct(response);
	}

	public async listAdminBrands(params: ListAdminBrandsQuery = {}): Promise<BrandModel[]> {
		const response = await this.request<BrandListResponse>(
			"GET",
			"/admin/brands",
			undefined,
			params
		);
		return response.data.map(parseBrand);
	}

	public async createAdminBrand(data: BrandInput): Promise<BrandModel> {
		const response = await this.request<components["schemas"]["Brand"]>(
			"POST",
			"/admin/brands",
			data
		);
		return parseBrand(response);
	}

	public async updateAdminBrand(id: number, data: BrandInput): Promise<BrandModel> {
		const response = await this.request<components["schemas"]["Brand"]>(
			"PATCH",
			`/admin/brands/${id}`,
			data
		);
		return parseBrand(response);
	}

	public async deleteAdminBrand(id: number): Promise<MessageResponse> {
		return await this.request("DELETE", `/admin/brands/${id}`);
	}

	public async listAdminCategories(
		params: ListAdminCategoriesQuery = {}
	): Promise<CategoryModel[]> {
		const response = await this.request<CategoryListResponse>(
			"GET",
			"/admin/categories",
			undefined,
			params
		);
		return response.data.map(parseCategory);
	}

	public async createAdminCategory(data: CategoryInput): Promise<CategoryModel> {
		const response = await this.request<components["schemas"]["Category"]>(
			"POST",
			"/admin/categories",
			data
		);
		return parseCategory(response);
	}

	public async updateAdminCategory(id: number, data: CategoryInput): Promise<CategoryModel> {
		const response = await this.request<components["schemas"]["Category"]>(
			"PATCH",
			`/admin/categories/${id}`,
			data
		);
		return parseCategory(response);
	}

	public async deleteAdminCategory(id: number): Promise<MessageResponse> {
		return await this.request("DELETE", `/admin/categories/${id}`);
	}

	public async listBrands(): Promise<BrandModel[]> {
		const response = await this.request<BrandListResponse>("GET", "/brands");
		return response.data.map(parseBrand);
	}

	public async listCategories(): Promise<CategoryModel[]> {
		const response = await this.request<CategoryListResponse>("GET", "/categories");
		return response.data.map(parseCategory);
	}

	public async listAdminProductAttributes(): Promise<ProductAttributeDefinitionModel[]> {
		const response = await this.request<ProductAttributeDefinitionListResponse>(
			"GET",
			"/admin/product-attributes"
		);
		return response.data.map(parseProductAttributeDefinition);
	}

	public async createAdminProductAttribute(
		data: ProductAttributeDefinitionInput
	): Promise<ProductAttributeDefinitionModel> {
		const response = await this.request<components["schemas"]["ProductAttributeDefinition"]>(
			"POST",
			"/admin/product-attributes",
			data
		);
		return parseProductAttributeDefinition(response);
	}

	public async updateAdminProductAttribute(
		id: number,
		data: ProductAttributeDefinitionInput
	): Promise<ProductAttributeDefinitionModel> {
		const response = await this.request<components["schemas"]["ProductAttributeDefinition"]>(
			"PATCH",
			`/admin/product-attributes/${id}`,
			data
		);
		return parseProductAttributeDefinition(response);
	}

	public async deleteAdminProductAttribute(id: number): Promise<MessageResponse> {
		return await this.request("DELETE", `/admin/product-attributes/${id}`);
	}

	public async listProductAttributes(): Promise<ProductAttributeDefinitionModel[]> {
		const response = await this.request<ProductAttributeDefinitionListResponse>(
			"GET",
			"/product-attributes"
		);
		return response.data.map(parseProductAttributeDefinition);
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

	public async listAdminCmsPages(): Promise<CmsPageListResponse> {
		return this.request<CmsPageListResponse>("GET", "/admin/cms/pages");
	}

	public async getAdminCmsLocales(): Promise<CmsLocaleSettings> {
		return this.request<CmsLocaleSettings>("GET", "/admin/cms/locales");
	}

	public async updateAdminCmsLocales(data: CmsLocaleSettingsInput): Promise<CmsLocaleSettings> {
		return this.request<CmsLocaleSettings>("PUT", "/admin/cms/locales", data);
	}

	public async listAdminCmsPageVariants(pageId: number): Promise<CmsPageVariant[]> {
		return this.request<CmsPageVariant[]>("GET", `/admin/cms/pages/${pageId}/variants`);
	}

	public async createAdminCmsPageVariant(
		pageId: number,
		data: CmsPageVariantInput
	): Promise<CmsPageVariant> {
		return this.request<CmsPageVariant>("POST", `/admin/cms/pages/${pageId}/variants`, data);
	}

	public async updateAdminCmsPageVariant(
		pageId: number,
		variantId: number,
		data: CmsPageVariantInput
	): Promise<CmsPageVariant> {
		return this.request<CmsPageVariant>(
			"PUT",
			`/admin/cms/pages/${pageId}/variants/${variantId}`,
			data
		);
	}

	public async deleteAdminCmsPageVariant(pageId: number, variantId: number): Promise<void> {
		await this.request("DELETE", `/admin/cms/pages/${pageId}/variants/${variantId}`);
	}

	public async transitionAdminCmsPageVariant(
		pageId: number,
		variantId: number,
		action: "submit" | "approve" | "request_changes" | "publish" | "rollback",
		comment = ""
	): Promise<CmsPageVariant> {
		const response = await this.request<CmsPageVariant>(
			"POST",
			`/admin/cms/pages/${pageId}/variants/${variantId}/${action}`,
			{ comment }
		);
		if (action === "publish" || action === "rollback") broadcastStorefrontStateChange();
		return response;
	}

	public async listAdminCmsAuditEvents(entryId?: number): Promise<CmsAuditEvent[]> {
		return this.request<CmsAuditEvent[]>("GET", "/admin/cms/audit", undefined, {
			entry_id: entryId,
			limit: 100,
		});
	}

	public async exportAdminCmsContent(): Promise<CmsContentExport> {
		return this.request<CmsContentExport>("GET", "/admin/cms/export");
	}

	public async restoreAdminCmsContent(content: CmsContentExport): Promise<void> {
		await this.request("POST", "/admin/cms/export", content);
		broadcastStorefrontStateChange();
	}

	public async previewAdminCmsRestore(content: CmsContentExport): Promise<CmsRestorePreview> {
		return this.request<CmsRestorePreview>("POST", "/admin/cms/restore/preview", content);
	}

	public async getAdminCmsGovernance(): Promise<CmsGovernance> {
		return this.request<CmsGovernance>("GET", "/admin/cms/governance");
	}

	public async updateAdminCmsGovernance(data: CmsGovernanceInput): Promise<CmsGovernance> {
		return this.request<CmsGovernance>("PUT", "/admin/cms/governance", data);
	}

	public async getAdminCmsOperations(): Promise<CmsOperations> {
		return this.request<CmsOperations>("GET", "/admin/cms/operations");
	}

	public async retryAdminCmsInvalidation(id: number): Promise<void> {
		await this.request("POST", `/admin/cms/operations/invalidation/${id}/retry`);
	}

	public async createAdminCmsPage(data: CmsPageDraftRequest): Promise<CmsPageResponse> {
		return this.request<CmsPageResponse>("POST", "/admin/cms/pages", data);
	}

	public async updateAdminCmsPage(id: number, data: CmsPageDraftRequest): Promise<CmsPageResponse> {
		return this.request<CmsPageResponse>("PATCH", `/admin/cms/pages/${id}`, data);
	}

	public async deleteAdminCmsPage(id: number): Promise<void> {
		await this.request("DELETE", `/admin/cms/pages/${id}`);
		broadcastStorefrontStateChange();
	}

	public async discardAdminCmsPageDraft(id: number): Promise<CmsPageResponse | null> {
		const response = await this.request<CmsPageResponse | null>(
			"DELETE",
			`/admin/cms/pages/${id}/draft`
		);
		broadcastStorefrontStateChange();
		return response;
	}

	public async publishAdminCmsPage(id: number, notes = ""): Promise<CmsPageResponse> {
		const payload: CmsPublishRequest = { notes };
		const response = await this.request<CmsPageResponse>(
			"POST",
			`/admin/cms/pages/${id}/publish`,
			payload
		);
		broadcastStorefrontStateChange();
		return response;
	}

	public async unpublishAdminCmsPage(id: number, notes = ""): Promise<CmsPageResponse> {
		const payload: CmsPublishRequest = { notes };
		const response = await this.request<CmsPageResponse>(
			"POST",
			`/admin/cms/pages/${id}/unpublish`,
			payload
		);
		broadcastStorefrontStateChange();
		return response;
	}

	public async rollbackAdminCmsPage(
		id: number,
		versionId: number,
		notes = ""
	): Promise<CmsPageResponse> {
		const response = await this.request<CmsPageResponse>(
			"POST",
			`/admin/cms/pages/${id}/rollback`,
			{ version_id: versionId, notes }
		);
		broadcastStorefrontStateChange();
		return response;
	}

	public async previewAdminCmsPayload(data: CmsPreviewRequest): Promise<CmsPreviewResponse> {
		return this.request<CmsPreviewResponse>("POST", "/admin/cms/preview", data);
	}

	public async getAdminCmsPageDelivery(id: number): Promise<CmsPageDeliveryResponse> {
		return this.request<CmsPageDeliveryResponse>("GET", `/admin/cms/pages/${id}/delivery`);
	}

	public async updateAdminCmsPageDelivery(
		id: number,
		data: CmsPageDeliveryRequest
	): Promise<CmsPageDeliveryResponse> {
		return this.request<CmsPageDeliveryResponse>("PUT", `/admin/cms/pages/${id}/delivery`, data);
	}

	public async recordCmsContentEvent(data: CmsContentEventRequest): Promise<void> {
		await this.request("POST", "/content/events", data);
	}

	public async getAdminCmsPageSEO(id: number): Promise<CmsSEOResponse> {
		return this.request<CmsSEOResponse>("GET", `/admin/cms/pages/${id}/seo`);
	}

	public async updateAdminCmsPageSEO(id: number, data: CmsSEOInput): Promise<CmsSEOResponse> {
		return this.request<CmsSEOResponse>("PUT", `/admin/cms/pages/${id}/seo`, data);
	}

	public async listAdminCmsRedirects(): Promise<CmsRedirectRule[]> {
		return this.request<CmsRedirectRule[]>("GET", "/admin/cms/redirects");
	}

	public async createAdminCmsRedirect(data: CmsRedirectInput): Promise<CmsRedirectRule> {
		return this.request<CmsRedirectRule>("POST", "/admin/cms/redirects", data);
	}

	public async updateAdminCmsRedirect(
		id: number,
		data: CmsRedirectInput
	): Promise<CmsRedirectRule> {
		return this.request<CmsRedirectRule>("PATCH", `/admin/cms/redirects/${id}`, data);
	}

	public async deleteAdminCmsRedirect(id: number): Promise<void> {
		await this.request("DELETE", `/admin/cms/redirects/${id}`);
	}

	public async listAdminCmsNavigation(): Promise<CmsNavigationListResponse> {
		return this.request<CmsNavigationListResponse>("GET", "/admin/cms/navigation");
	}

	public async createAdminCmsNavigation(
		data: CmsNavigationDraftRequest
	): Promise<CmsNavigationResponse> {
		return this.request<CmsNavigationResponse>("POST", "/admin/cms/navigation", data);
	}

	public async updateAdminCmsNavigation(
		id: number,
		data: CmsNavigationDraftRequest
	): Promise<CmsNavigationResponse> {
		return this.request<CmsNavigationResponse>("PATCH", `/admin/cms/navigation/${id}`, data);
	}

	public async deleteAdminCmsNavigation(id: number): Promise<void> {
		await this.request("DELETE", `/admin/cms/navigation/${id}`);
		broadcastStorefrontStateChange();
	}

	public async discardAdminCmsNavigationDraft(id: number): Promise<CmsNavigationResponse | null> {
		const response = await this.request<CmsNavigationResponse | null>(
			"DELETE",
			`/admin/cms/navigation/${id}/draft`
		);
		broadcastStorefrontStateChange();
		return response;
	}

	public async publishAdminCmsNavigation(id: number, notes = ""): Promise<CmsNavigationResponse> {
		const payload: CmsPublishRequest = { notes };
		const response = await this.request<CmsNavigationResponse>(
			"POST",
			`/admin/cms/navigation/${id}/publish`,
			payload
		);
		broadcastStorefrontStateChange();
		return response;
	}

	public async unpublishAdminCmsNavigation(id: number, notes = ""): Promise<CmsNavigationResponse> {
		const payload: CmsPublishRequest = { notes };
		const response = await this.request<CmsNavigationResponse>(
			"POST",
			`/admin/cms/navigation/${id}/unpublish`,
			payload
		);
		broadcastStorefrontStateChange();
		return response;
	}

	public async listAdminCmsGlobalRegions(): Promise<CmsGlobalRegionListResponse> {
		return this.request<CmsGlobalRegionListResponse>("GET", "/admin/cms/global");
	}

	public async createAdminCmsGlobalRegion(
		data: CmsGlobalRegionDraftRequest
	): Promise<CmsGlobalRegionResponse> {
		return this.request<CmsGlobalRegionResponse>("POST", "/admin/cms/global", data);
	}

	public async updateAdminCmsGlobalRegion(
		id: number,
		data: CmsGlobalRegionDraftRequest
	): Promise<CmsGlobalRegionResponse> {
		return this.request<CmsGlobalRegionResponse>("PATCH", `/admin/cms/global/${id}`, data);
	}

	public async deleteAdminCmsGlobalRegion(id: number): Promise<void> {
		await this.request("DELETE", `/admin/cms/global/${id}`);
		broadcastStorefrontStateChange();
	}

	public async discardAdminCmsGlobalRegionDraft(
		id: number
	): Promise<CmsGlobalRegionResponse | null> {
		const response = await this.request<CmsGlobalRegionResponse | null>(
			"DELETE",
			`/admin/cms/global/${id}/draft`
		);
		broadcastStorefrontStateChange();
		return response;
	}

	public async publishAdminCmsGlobalRegion(
		id: number,
		notes = ""
	): Promise<CmsGlobalRegionResponse> {
		const payload: CmsPublishRequest = { notes };
		const response = await this.request<CmsGlobalRegionResponse>(
			"POST",
			`/admin/cms/global/${id}/publish`,
			payload
		);
		broadcastStorefrontStateChange();
		return response;
	}

	public async unpublishAdminCmsGlobalRegion(
		id: number,
		notes = ""
	): Promise<CmsGlobalRegionResponse> {
		const payload: CmsPublishRequest = { notes };
		const response = await this.request<CmsGlobalRegionResponse>(
			"POST",
			`/admin/cms/global/${id}/unpublish`,
			payload
		);
		broadcastStorefrontStateChange();
		return response;
	}

	public async getAdminWebsiteSettings(): Promise<WebsiteSettingsResponse> {
		return this.request<WebsiteSettingsResponse>("GET", "/admin/website");
	}

	public async updateWebsiteSettings(settings: WebsiteSettings): Promise<WebsiteSettingsResponse> {
		const payload: WebsiteSettingsRequest = { settings };
		return this.request<WebsiteSettingsResponse>("PUT", "/admin/website", payload);
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
	public async listOrders(params?: ListOrdersParams): Promise<{
		data: OrderModel[];
		pagination: OrderPagePayload["pagination"];
	}> {
		return ordersDomain.listOrders(this.request.bind(this), params);
	}

	public async getOrderDetails(orderId: number): Promise<OrderModel> {
		return ordersDomain.getOrderDetails(this.request.bind(this), orderId);
	}

	public async cancelOrder(orderId: number): Promise<OrderModel> {
		return ordersDomain.cancelOrder(this.request.bind(this), orderId);
	}

	public async claimGuestOrder(
		data: components["schemas"]["ClaimGuestOrderRequest"]
	): Promise<{ message: string; order: OrderModel }> {
		return ordersDomain.claimGuestOrder(this.request.bind(this), data);
	}

	// Admin Order Management
	public async listAdminOrders(params?: ListAdminOrdersQuery): Promise<{
		data: OrderModel[];
		pagination: OrderPagePayload["pagination"];
	}> {
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

	public async getAdminOrderPayments(orderId: number): Promise<OrderPaymentLedger> {
		return await this.request<OrderPaymentLedger>("GET", `/admin/orders/${orderId}/payments`);
	}

	public async captureAdminOrderPayment(
		orderId: number,
		intentId: number,
		data: AdminOrderPaymentAmountRequest = {}
	): Promise<AdminOrderPaymentLifecycleModel> {
		const response = await this.request<AdminOrderPaymentLifecycleResponse>(
			"POST",
			`/admin/orders/${orderId}/payments/${intentId}/capture`,
			data
		);
		return { ...response, order: parseOrder(response.order) };
	}

	public async voidAdminOrderPayment(
		orderId: number,
		intentId: number
	): Promise<AdminOrderPaymentLifecycleModel> {
		const response = await this.request<AdminOrderPaymentLifecycleResponse>(
			"POST",
			`/admin/orders/${orderId}/payments/${intentId}/void`
		);
		return { ...response, order: parseOrder(response.order) };
	}

	public async refundAdminOrderPayment(
		orderId: number,
		intentId: number,
		data: AdminOrderPaymentAmountRequest = {}
	): Promise<AdminOrderPaymentLifecycleModel> {
		const response = await this.request<AdminOrderPaymentLifecycleResponse>(
			"POST",
			`/admin/orders/${orderId}/payments/${intentId}/refund`,
			data
		);
		return { ...response, order: parseOrder(response.order) };
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
			if (error.status === 401 || error.status === 404) {
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
