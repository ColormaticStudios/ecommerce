import type { components } from "$lib/api/generated/openapi";
import type {
	BrandModel,
	CheckoutOrderTrackingModel,
	CartItemModel,
	CartModel,
	OrderItemModel,
	OrderModel,
	ProductAttributeDefinitionModel,
	ProductModel,
	ProductVariantModel,
	SavedAddressModel,
	SavedPaymentMethodModel,
	ShipmentModel,
	TrackingEventModel,
	UserModel,
} from "$lib/models";
import {
	cloneStorefrontSettings,
	createDefaultProductSection,
	createDefaultStorefrontSettings,
	type StorefrontSettingsModel,
	type StorefrontSettingsResponseModel,
} from "$lib/storefront";

type ProviderCredential = components["schemas"]["ProviderCredential"];
type ProviderOperationsOverview = components["schemas"]["ProviderOperationsOverview"];
type ProviderReconciliationRun = components["schemas"]["ProviderReconciliationRun"];
type WebhookEventRecord = components["schemas"]["WebhookEventRecord"];
type WebhookEventPage = components["schemas"]["WebhookEventPage"];
type CheckoutPlugin = components["schemas"]["CheckoutPlugin"];
type CheckoutPluginCatalog = components["schemas"]["CheckoutPluginCatalog"];
type CheckoutPluginField = components["schemas"]["CheckoutPluginField"];
type CheckoutPluginState = components["schemas"]["CheckoutPluginState"];
type CheckoutQuoteResponse = components["schemas"]["CheckoutQuoteResponse"];
type AuthResponse = components["schemas"]["AuthResponse"];
type AuthUser = components["schemas"]["User"];
type DraftPreviewSession = {
	active: boolean;
	expires_at: Date | null;
};

const now = new Date("2026-04-07T12:00:00.000Z");

function merge<T>(base: T, overrides: Partial<T>): T {
	return { ...base, ...overrides };
}

export function makeUser(overrides: Partial<UserModel> = {}): UserModel {
	return merge(
		{
			id: 1,
			subject: "user-subject-1",
			username: "story-user",
			email: "story@example.com",
			name: "Story User",
			role: "customer",
			currency: "USD",
			profile_photo_url: null,
			created_at: now,
			updated_at: now,
			deleted_at: null,
		},
		overrides
	);
}

export function makeAuthUser(overrides: Partial<AuthUser> = {}): AuthUser {
	return {
		id: 1,
		subject: "user-subject-1",
		username: "story-user",
		email: "story@example.com",
		name: "Story User",
		role: "customer",
		currency: "USD",
		profile_photo_url: null,
		created_at: now.toISOString(),
		updated_at: now.toISOString(),
		deleted_at: null,
		...overrides,
	};
}

export function makeAuthResponse(overrides: Partial<AuthResponse> = {}): AuthResponse {
	return {
		user: makeAuthUser(),
		...overrides,
	};
}

export function makeBrand(overrides: Partial<BrandModel> = {}): BrandModel {
	return merge(
		{
			id: 1,
			name: "Colormatic",
			slug: "colormatic",
			description: "Studio brand for route stories.",
			logo_media_id: null,
			is_active: true,
		},
		overrides
	);
}

export function makeVariant(overrides: Partial<ProductVariantModel> = {}): ProductVariantModel {
	return merge(
		{
			id: 11,
			sku: "story-product-default",
			title: "Default Variant",
			price: 129,
			compare_at_price: null,
			stock: 12,
			position: 1,
			is_published: true,
			weight_grams: null,
			length_cm: null,
			width_cm: null,
			height_cm: null,
			selections: [],
		},
		overrides
	);
}

export function makeProduct(overrides: Partial<ProductModel> = {}): ProductModel {
	const variant = makeVariant({
		id: 11,
		sku: "story-product-default",
		title: "Default Variant",
		price: 129,
	});

	return merge(
		{
			created_at: now,
			deleted_at: null,
			updated_at: now,
			draft_updated_at: null,
			is_published: true,
			has_draft_changes: false,
			id: 101,
			sku: "story-product",
			name: "Field Jacket",
			subtitle: "Transitional layer",
			description: "Utility-forward outerwear with a lightweight shell and soft lining.",
			price: 129,
			stock: 12,
			images: [
				"https://images.unsplash.com/photo-1523381210434-271e8be1f52b?auto=format&fit=crop&w=900&q=80",
				"https://images.unsplash.com/photo-1512436991641-6745cdb1723f?auto=format&fit=crop&w=900&q=80",
			],
			cover_image:
				"https://images.unsplash.com/photo-1523381210434-271e8be1f52b?auto=format&fit=crop&w=900&q=80",
			brand: makeBrand(),
			default_variant_id: variant.id,
			default_variant_sku: variant.sku,
			price_range: { min: 129, max: 129 },
			options: [],
			variants: [variant],
			attributes: [],
			seo: {
				title: null,
				description: null,
				canonical_path: null,
				og_image_media_id: null,
				noindex: false,
			},
			related_products: [],
		},
		overrides
	);
}

export function makeCartItem(overrides: Partial<CartItemModel> = {}): CartItemModel {
	const product = makeProduct();
	const variant = makeVariant({
		id: product.default_variant_id ?? 11,
		sku: product.default_variant_sku ?? "story-product-default",
		title: "Default Variant",
		price: product.price,
		stock: product.stock,
	});

	return merge(
		{
			id: 301,
			cart_id: 201,
			product_variant_id: variant.id ?? 11,
			quantity: 1,
			product_variant: variant,
			product,
			created_at: now,
			updated_at: now,
			deleted_at: null,
		},
		overrides
	);
}

export function makeCart(overrides: Partial<CartModel> = {}): CartModel {
	const item = makeCartItem();
	return merge(
		{
			id: 201,
			user_id: 1,
			items: [item],
			created_at: now,
			updated_at: now,
			deleted_at: null,
		},
		overrides
	);
}

export function makeOrderItem(overrides: Partial<OrderItemModel> = {}): OrderItemModel {
	const product = makeProduct();
	const variant = makeVariant({
		id: product.default_variant_id ?? 11,
		sku: product.default_variant_sku ?? "story-product-default",
		title: "Default Variant",
		price: product.price,
		stock: product.stock,
	});

	return merge(
		{
			id: 401,
			order_id: 501,
			product_variant_id: variant.id ?? 11,
			variant_sku: variant.sku,
			variant_title: variant.title,
			quantity: 1,
			price: variant.price,
			product_variant: variant,
			product,
			created_at: now,
			updated_at: now,
			deleted_at: null,
		},
		overrides
	);
}

export function makeOrder(overrides: Partial<OrderModel> = {}): OrderModel {
	const item = makeOrderItem();
	return merge(
		{
			id: 501,
			user_id: 1,
			checkout_session_id: 601,
			guest_email: null,
			confirmation_token: null,
			status: "PENDING",
			can_cancel: true,
			total: item.price * item.quantity,
			payment_method_display: "Visa •••• 4242",
			shipping_address_pretty: "Story User, 1 Market St, San Francisco, CA 94105, US",
			created_at: now,
			updated_at: now,
			deleted_at: null,
			items: [item],
		},
		overrides
	);
}

export function makeTrackingEvent(overrides: Partial<TrackingEventModel> = {}): TrackingEventModel {
	return merge(
		{
			id: 901,
			provider: "shippo",
			provider_event_id: "evt_901",
			status: "IN_TRANSIT",
			tracking_number: "1Z999AA10123456784",
			location: "Oakland, CA",
			description: "Package departed regional facility.",
			occurred_at: now,
		},
		overrides
	);
}

export function makeShipment(overrides: Partial<ShipmentModel> = {}): ShipmentModel {
	const trackingEvent = makeTrackingEvent();
	return merge(
		{
			id: 851,
			order_id: 501,
			snapshot_id: 751,
			provider: "shippo",
			shipment_rate_id: 611,
			provider_shipment_id: "ship_851",
			status: "IN_TRANSIT",
			currency: "USD",
			service_code: "ups_ground",
			service_name: "UPS Ground",
			amount: 12,
			shipping_address_pretty: "Story User, 1 Market St, San Francisco, CA 94105, US",
			tracking_number: trackingEvent.tracking_number,
			tracking_url: "https://example.com/track/1Z999AA10123456784",
			label_url: "https://example.com/label/ship_851.pdf",
			purchased_at: new Date("2026-04-05T14:30:00.000Z"),
			finalized_at: new Date("2026-04-05T15:00:00.000Z"),
			delivered_at: null,
			rates: [],
			packages: [
				{
					id: 771,
					reference: "PKG-1",
					weight_grams: 1200,
					length_cm: 30,
					width_cm: 20,
					height_cm: 10,
				},
			],
			tracking_events: [trackingEvent],
		},
		overrides
	);
}

export function makeCheckoutOrderTracking(
	overrides: Partial<CheckoutOrderTrackingModel> = {}
): CheckoutOrderTrackingModel {
	return merge(
		{
			order_id: 501,
			shipments: [makeShipment()],
		},
		overrides
	);
}

export function makeSavedPaymentMethod(
	overrides: Partial<SavedPaymentMethodModel> = {}
): SavedPaymentMethodModel {
	return merge(
		{
			id: 701,
			user_id: 1,
			type: "card",
			brand: "Visa",
			last4: "4242",
			exp_month: 12,
			exp_year: 2030,
			cardholder_name: "Story User",
			nickname: "Primary Visa",
			is_default: true,
			created_at: now,
			updated_at: now,
			deleted_at: null,
		},
		overrides
	);
}

export function makeSavedAddress(overrides: Partial<SavedAddressModel> = {}): SavedAddressModel {
	return merge(
		{
			id: 801,
			user_id: 1,
			label: "Studio",
			full_name: "Story User",
			line1: "1 Market St",
			line2: "",
			city: "San Francisco",
			state: "CA",
			postal_code: "94105",
			country: "US",
			phone: "",
			is_default: true,
			created_at: now,
			updated_at: now,
			deleted_at: null,
		},
		overrides
	);
}

export function makeAttributeDefinition(
	overrides: Partial<ProductAttributeDefinitionModel> = {}
): ProductAttributeDefinitionModel {
	return merge(
		{
			id: 1,
			key: "material",
			slug: "material",
			type: "text",
			filterable: true,
			sortable: false,
		},
		overrides
	);
}

export function makeCheckoutField(
	overrides: Partial<CheckoutPluginField> = {}
): CheckoutPluginField {
	return merge(
		{
			key: "card_number",
			label: "Card number",
			type: "text",
			required: true,
			placeholder: "4111111111111111",
			help_text: "",
			options: [],
		},
		overrides
	);
}

export function makeCheckoutState(
	overrides: Partial<CheckoutPluginState> = {}
): CheckoutPluginState {
	return merge(
		{
			code: "ok",
			severity: "info",
			message: "Ready",
		},
		overrides
	);
}

export function makeCheckoutPlugin(overrides: Partial<CheckoutPlugin> = {}): CheckoutPlugin {
	return merge(
		{
			id: "dummy-card",
			type: "payment",
			name: "Dummy Card Gateway",
			description: "Collect card details for the story harness.",
			status: "ready",
			enabled: true,
			fields: [
				makeCheckoutField({ key: "cardholder_name", label: "Cardholder name" }),
				makeCheckoutField({ key: "card_number", label: "Card number" }),
				makeCheckoutField({ key: "exp_month", label: "Exp month", type: "number" }),
				makeCheckoutField({ key: "exp_year", label: "Exp year", type: "number" }),
			],
			states: [],
		},
		overrides
	);
}

export function makeCheckoutCatalog(
	overrides: Partial<CheckoutPluginCatalog> = {}
): CheckoutPluginCatalog {
	return merge(
		{
			payment: [
				makeCheckoutPlugin(),
				makeCheckoutPlugin({
					id: "dummy-wallet",
					name: "Dummy Wallet",
					description: "Shortcut wallet payment flow.",
					fields: [],
				}),
			],
			shipping: [
				makeCheckoutPlugin({
					id: "dummy-ground",
					type: "shipping",
					name: "Dummy Ground Carrier",
					description: "Collect a shipping address and service level.",
					fields: [
						makeCheckoutField({ key: "full_name", label: "Recipient name" }),
						makeCheckoutField({ key: "line1", label: "Address line 1" }),
						makeCheckoutField({ key: "line2", label: "Address line 2", required: false }),
						makeCheckoutField({ key: "city", label: "City" }),
						makeCheckoutField({ key: "state", label: "State/Province" }),
						makeCheckoutField({ key: "postal_code", label: "Postal code" }),
						makeCheckoutField({ key: "country", label: "Country" }),
						makeCheckoutField({
							key: "service_level",
							label: "Service level",
							type: "select",
							options: [
								{ value: "standard", label: "Standard" },
								{ value: "express", label: "Express" },
							],
						}),
					],
				}),
				makeCheckoutPlugin({
					id: "dummy-pickup",
					type: "shipping",
					name: "Dummy Pickup",
					description: "Collect pickup instructions only.",
					fields: [
						makeCheckoutField({ key: "pickup_code", label: "Pickup code", required: false }),
					],
				}),
			],
			tax: [
				makeCheckoutPlugin({
					id: "dummy-tax",
					type: "tax",
					name: "Dummy Tax Engine",
					description: "Automatic sales tax calculation.",
					fields: [],
				}),
			],
		},
		overrides
	);
}

export function makeCheckoutQuote(
	overrides: Partial<CheckoutQuoteResponse> = {}
): CheckoutQuoteResponse {
	return merge(
		{
			snapshot_id: 901,
			expires_at: "2026-04-07T13:00:00.000Z",
			currency: "USD",
			subtotal: 129,
			shipping: 12,
			tax: 10.32,
			total: 151.32,
			valid: true,
			payment_states: [],
			shipping_states: [],
			tax_states: [],
		},
		overrides
	);
}

export function makeStorefrontSettings(
	overrides: Partial<StorefrontSettingsModel> = {}
): StorefrontSettingsModel {
	const settings = cloneStorefrontSettings(createDefaultStorefrontSettings());
	settings.site_title = "Colormatic Supply";
	settings.homepage_sections = [
		{
			id: "hero-1",
			type: "hero",
			enabled: true,
			hero: {
				eyebrow: "Spring Edit",
				title: "Useful gear with sharper edges",
				subtitle: "A route-story storefront with deliberate states and less generic filler.",
				background_image_url:
					"https://images.unsplash.com/photo-1483985988355-763728e1935b?auto=format&fit=crop&w=1600&q=80",
				background_image_media_id: "",
				primary_cta: { label: "Shop new arrivals", url: "/search?q=jacket" },
				secondary_cta: { label: "Browse all", url: "/search" },
			},
		},
		{
			id: "products-1",
			type: "products",
			enabled: true,
			product_section: {
				...createDefaultProductSection("Featured products", "manual"),
				subtitle: "A small manual selection for Storybook.",
				source: "manual",
				product_ids: [101, 102],
				show_stock: true,
				show_description: true,
				image_aspect: "wide",
			},
		},
	];
	return merge(settings, overrides);
}

export function makeStorefrontResponse(
	overrides: Partial<StorefrontSettingsResponseModel> = {}
): StorefrontSettingsResponseModel {
	return merge(
		{
			settings: makeStorefrontSettings(),
			updated_at: now,
			has_draft_changes: false,
			draft_updated_at: null,
			published_updated_at: now,
		},
		overrides
	);
}

export function makeDraftPreviewSession(
	overrides: Partial<DraftPreviewSession> = {}
): DraftPreviewSession {
	return merge(
		{
			active: false,
			expires_at: null,
		},
		overrides
	);
}

export function makeProviderCredential(
	overrides: Partial<ProviderCredential> = {}
): ProviderCredential {
	return merge(
		{
			id: 1,
			provider_type: "payment",
			provider_id: "dummy-card",
			environment: "sandbox",
			label: "Primary sandbox credential",
			key_version: "kv_2026_04",
			supported_currencies: ["USD"],
			settlement_currency: "USD",
			fx_mode: "same_currency_only",
			last_rotated_at: "2026-04-07T10:15:00.000Z",
			updated_at: "2026-04-07T10:15:00.000Z",
		},
		overrides
	);
}

export function makeProviderOverview(
	overrides: Partial<ProviderOperationsOverview> = {}
): ProviderOperationsOverview {
	return merge(
		{
			runtime_environment: "sandbox",
			credential_service_configured: true,
			webhook_events: {
				pending_count: 0,
				processed_count: 18,
				dead_letter_count: 0,
				rejected_count: 0,
			},
		},
		overrides
	);
}

export function makeReconciliationRun(
	overrides: Partial<ProviderReconciliationRun> = {}
): ProviderReconciliationRun {
	return merge(
		{
			id: 1,
			provider_type: "payment",
			provider_id: "dummy-card",
			environment: "sandbox",
			trigger: "MANUAL",
			status: "SUCCEEDED",
			checked_count: 24,
			drift_count: 0,
			error_count: 0,
			started_at: "2026-04-07T09:30:00.000Z",
			finished_at: "2026-04-07T09:31:00.000Z",
			drifts: [],
		},
		overrides
	);
}

export function makeWebhookEventPage(overrides: Partial<WebhookEventPage> = {}): WebhookEventPage {
	return merge(
		{
			data: [],
			pagination: {
				page: 1,
				limit: 5,
				total: 0,
				total_pages: 1,
			},
		},
		overrides
	);
}

export function makeWebhookEventRecord(
	overrides: Partial<WebhookEventRecord> = {}
): WebhookEventRecord {
	return merge(
		{
			id: 1,
			provider: "dummy-card",
			provider_event_id: "evt_story_1",
			event_type: "payment.authorized",
			signature_valid: true,
			payload: '{"ok":true}',
			received_at: "2026-04-07T09:40:00.000Z",
			processed_at: "2026-04-07T09:40:05.000Z",
			attempt_count: 1,
			last_error: "",
			status: "PROCESSED",
		},
		overrides
	);
}
