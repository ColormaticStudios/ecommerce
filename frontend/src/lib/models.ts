import type { components } from "$lib/api/generated/openapi";

type UserRole = components["schemas"]["User"]["role"];
type OrderStatus = components["schemas"]["Order"]["status"];

export interface UserModel {
	id: number;
	subject: string;
	username: string;
	email: string;
	name: string | null;
	role: UserRole;
	currency: string;
	profile_photo_url: string | null;
	created_at: Date;
	updated_at: Date;
	deleted_at: Date | null;
}

export type ProfileModel = components["schemas"]["User"];

export function parseProfile(profile: ProfileModel): UserModel {
	return {
		id: profile.id,
		subject: profile.subject,
		username: profile.username,
		email: profile.email,
		name: profile.name,
		role: profile.role,
		currency: profile.currency,
		profile_photo_url: profile.profile_photo_url,
		created_at: parseDate(profile.created_at) ?? new Date(),
		updated_at: parseDate(profile.updated_at) ?? new Date(),
		deleted_at: parseDate(profile.deleted_at),
	};
}

export interface PageModel {
	data: ProductModel[];
	pagination: {
		limit: number;
		page: number;
		total: number;
		total_pages: number;
	};
}

type RelatedProductPayload = components["schemas"]["RelatedProduct"];
type ProductPayload = components["schemas"]["Product"];
type BrandPayload = components["schemas"]["Brand"];
type ProductAttributeDefinitionPayload = components["schemas"]["ProductAttributeDefinition"];
type ProductOptionPayload = components["schemas"]["ProductOption"];
type ProductOptionValuePayload = components["schemas"]["ProductOptionValue"];
type ProductVariantPayload = components["schemas"]["ProductVariant"];
type ProductVariantSelectionPayload = components["schemas"]["ProductVariantSelection"];
type ProductAttributeValuePayload = components["schemas"]["ProductAttributeValue"];
type ProductPriceRangePayload = components["schemas"]["ProductPriceRange"];
type ProductSEOPayload = components["schemas"]["ProductSEO"];

function parseRelatedProduct(product: RelatedProductPayload): RelatedProductModel {
	return {
		id: product.id,
		sku: product.sku,
		name: product.name,
		description: product.description ?? null,
		price: product.price,
		cover_image: product.cover_image ?? null,
		stock: product.stock ?? 0,
	};
}

export function parseBrand(brand: BrandPayload): BrandModel {
	return {
		id: brand.id,
		name: brand.name,
		slug: brand.slug,
		description: brand.description ?? null,
		logo_media_id: brand.logo_media_id ?? null,
		is_active: brand.is_active,
	};
}

export function parseProductAttributeDefinition(
	attribute: ProductAttributeDefinitionPayload
): ProductAttributeDefinitionModel {
	return {
		id: attribute.id,
		key: attribute.key,
		slug: attribute.slug,
		type: attribute.type,
		filterable: attribute.filterable,
		sortable: attribute.sortable,
	};
}

function parseProductOptionValue(value: ProductOptionValuePayload): ProductOptionValueModel {
	return {
		id: value.id ?? null,
		value: value.value,
		position: value.position,
	};
}

function parseProductOption(option: ProductOptionPayload): ProductOptionModel {
	return {
		id: option.id ?? null,
		name: option.name,
		position: option.position,
		display_type: option.display_type,
		values: (option.values ?? []).map(parseProductOptionValue),
	};
}

function parseProductVariantSelection(
	selection: ProductVariantSelectionPayload
): ProductVariantSelectionModel {
	return {
		product_option_value_id: selection.product_option_value_id ?? null,
		option_name: selection.option_name,
		option_value: selection.option_value,
		position: selection.position,
	};
}

function parseProductVariant(variant: ProductVariantPayload): ProductVariantModel {
	return {
		id: variant.id ?? null,
		sku: variant.sku,
		title: variant.title,
		price: variant.price,
		compare_at_price: variant.compare_at_price ?? null,
		stock: variant.stock,
		position: variant.position,
		is_published: variant.is_published,
		weight_grams: variant.weight_grams ?? null,
		length_cm: variant.length_cm ?? null,
		width_cm: variant.width_cm ?? null,
		height_cm: variant.height_cm ?? null,
		selections: (variant.selections ?? []).map(parseProductVariantSelection),
	};
}

function parseProductAttributeValue(
	attribute: ProductAttributeValuePayload
): ProductAttributeValueModel {
	return {
		product_attribute_id: attribute.product_attribute_id,
		key: attribute.key,
		slug: attribute.slug,
		type: attribute.type as ProductAttributeValueModel["type"],
		text_value: attribute.text_value ?? null,
		number_value: attribute.number_value ?? null,
		boolean_value: attribute.boolean_value ?? null,
		enum_value: attribute.enum_value ?? null,
		position: attribute.position,
	};
}

function parseProductPriceRange(range: ProductPriceRangePayload): ProductPriceRangeModel {
	return {
		min: range.min,
		max: range.max,
	};
}

function parseProductSEO(seo: ProductSEOPayload): ProductSEOModel {
	return {
		title: seo.title ?? null,
		description: seo.description ?? null,
		canonical_path: seo.canonical_path ?? null,
		og_image_media_id: seo.og_image_media_id ?? null,
		noindex: seo.noindex ?? false,
	};
}

function parseDate(value: string | Date | null | undefined): Date | null {
	if (!value) {
		return null;
	}
	if (value instanceof Date) {
		return value;
	}
	const parsed = new Date(value);
	return Number.isNaN(parsed.valueOf()) ? null : parsed;
}

// The argument `product` is technically not a product, the dates are strings. It's close enough though.
export function parseProduct(product: ProductPayload): ProductModel {
	const coverImage = product.cover_image ?? product.images?.[0] ?? null;
	return {
		created_at: parseDate(product.created_at) ?? new Date(),
		deleted_at: parseDate(product.deleted_at),
		updated_at: parseDate(product.updated_at) ?? new Date(),
		draft_updated_at: parseDate(product.draft_updated_at),
		is_published: product.is_published ?? true,
		has_draft_changes: product.has_draft_changes ?? false,
		id: product.id,
		sku: product.sku,
		name: product.name,
		subtitle: product.subtitle ?? null,
		description: product.description,
		price: product.price,
		stock: product.stock,
		images: product.images ?? [],
		cover_image: coverImage ?? undefined,
		brand: product.brand ? parseBrand(product.brand) : null,
		default_variant_id: product.default_variant_id ?? null,
		default_variant_sku: product.default_variant_sku ?? null,
		price_range: parseProductPriceRange(product.price_range),
		options: (product.options ?? []).map(parseProductOption),
		variants: (product.variants ?? []).map(parseProductVariant),
		attributes: (product.attributes ?? []).map(parseProductAttributeValue),
		seo: parseProductSEO(product.seo),
		related_products: (product.related_products ?? []).map(parseRelatedProduct),
	};
}

export interface ProductModel {
	created_at: Date;
	deleted_at: Date | null;
	updated_at: Date;
	draft_updated_at: Date | null;
	is_published: boolean;
	has_draft_changes: boolean;
	id: number;
	sku: string;
	name: string;
	subtitle: string | null;
	description: string;
	price: number;
	stock: number;
	images: string[];
	cover_image?: string;
	brand: BrandModel | null;
	default_variant_id: number | null;
	default_variant_sku: string | null;
	price_range: ProductPriceRangeModel;
	options: ProductOptionModel[];
	variants: ProductVariantModel[];
	attributes: ProductAttributeValueModel[];
	seo: ProductSEOModel;
	related_products: RelatedProductModel[];
}

export interface BrandModel {
	id: number;
	name: string;
	slug: string;
	description: string | null;
	logo_media_id: string | null;
	is_active: boolean;
}

export interface ProductAttributeDefinitionModel {
	id: number;
	key: string;
	slug: string;
	type: "text" | "number" | "boolean" | "enum";
	filterable: boolean;
	sortable: boolean;
}

export interface ProductOptionValueModel {
	id: number | null;
	value: string;
	position: number;
}

export interface ProductOptionModel {
	id: number | null;
	name: string;
	position: number;
	display_type: string;
	values: ProductOptionValueModel[];
}

export interface ProductVariantSelectionModel {
	product_option_value_id: number | null;
	option_name: string;
	option_value: string;
	position: number;
}

export interface ProductVariantModel {
	id: number | null;
	sku: string;
	title: string;
	price: number;
	compare_at_price: number | null;
	stock: number;
	position: number;
	is_published: boolean;
	weight_grams: number | null;
	length_cm: number | null;
	width_cm: number | null;
	height_cm: number | null;
	selections: ProductVariantSelectionModel[];
}

export interface ProductAttributeValueModel {
	product_attribute_id: number;
	key: string;
	slug: string;
	type: "text" | "number" | "boolean" | "enum";
	text_value: string | null;
	number_value: number | null;
	boolean_value: boolean | null;
	enum_value: string | null;
	position: number;
}

export interface ProductPriceRangeModel {
	min: number;
	max: number;
}

export interface ProductSEOModel {
	title: string | null;
	description: string | null;
	canonical_path: string | null;
	og_image_media_id: string | null;
	noindex: boolean;
}

export interface RelatedProductModel {
	id: number;
	sku: string;
	name: string;
	description: string | null;
	price?: number;
	cover_image: string | null;
	stock: number;
}

type CartItemPayload = components["schemas"]["CartItem"];
type CartPayload = components["schemas"]["Cart"];

export function parseCart(cart: CartPayload): CartModel {
	return {
		id: cart.id,
		user_id: cart.user_id,
		items: (cart.items ?? []).map(parseCartItem),
		created_at: parseDate(cart.created_at) ?? new Date(),
		updated_at: parseDate(cart.updated_at) ?? new Date(),
		deleted_at: parseDate(cart.deleted_at),
	};
}

export interface CartModel {
	id: number;
	user_id: number;
	items: CartItemModel[];
	created_at: Date;
	updated_at: Date;
	deleted_at: Date | null;
}

export function parseCartItem(cartItem: CartItemPayload): CartItemModel {
	return {
		id: cartItem.id,
		cart_id: cartItem.cart_id,
		product_variant_id: cartItem.product_variant_id,
		quantity: cartItem.quantity,
		product_variant: parseProductVariant(cartItem.product_variant),
		product: parseProduct(cartItem.product),
		created_at: parseDate(cartItem.created_at) ?? new Date(),
		updated_at: parseDate(cartItem.updated_at) ?? new Date(),
		deleted_at: parseDate(cartItem.deleted_at),
	};
}

export interface CartItemModel {
	id: number;
	cart_id: number;
	product_variant_id: number;
	quantity: number;
	product_variant: ProductVariantModel;
	product: ProductModel;
	created_at: Date;
	updated_at: Date;
	deleted_at: Date | null;
}

type OrderItemPayload = components["schemas"]["OrderItem"];
export type OrderPayload = components["schemas"]["Order"];

export function parseOrder(order: OrderPayload): OrderModel {
	return {
		id: order.id,
		user_id: order.user_id ?? null,
		checkout_session_id: order.checkout_session_id,
		guest_email: order.guest_email ?? null,
		confirmation_token: order.confirmation_token ?? null,
		status: order.status,
		can_cancel: order.can_cancel,
		total: order.total,
		payment_method_display: order.payment_method_display || null,
		shipping_address_pretty: order.shipping_address_pretty || null,
		created_at: parseDate(order.created_at) ?? new Date(),
		updated_at: parseDate(order.updated_at) ?? new Date(),
		deleted_at: parseDate(order.deleted_at),
		items: (order.items ?? []).map(parseOrderItem),
	};
}

export interface OrderModel {
	id: number;
	user_id: number | null;
	checkout_session_id: number;
	guest_email: string | null;
	confirmation_token: string | null;
	status: OrderStatus;
	can_cancel: boolean;
	total: number;
	payment_method_display: string | null;
	shipping_address_pretty: string | null;
	created_at: Date;
	updated_at: Date;
	deleted_at: Date | null;
	items: OrderItemModel[];
}

export function parseOrderItem(orderItem: OrderItemPayload): OrderItemModel {
	return {
		id: orderItem.id,
		order_id: orderItem.order_id,
		product_variant_id: orderItem.product_variant_id,
		variant_sku: orderItem.variant_sku,
		variant_title: orderItem.variant_title,
		quantity: orderItem.quantity,
		price: orderItem.price,
		product_variant: parseProductVariant(orderItem.product_variant),
		product: parseProduct(orderItem.product),
		created_at: parseDate(orderItem.created_at) ?? new Date(),
		updated_at: parseDate(orderItem.updated_at) ?? new Date(),
		deleted_at: parseDate(orderItem.deleted_at),
	};
}

export interface OrderItemModel {
	id: number;
	order_id: number;
	product_variant_id: number;
	variant_sku: string;
	variant_title: string;
	quantity: number;
	price: number;
	product_variant: ProductVariantModel;
	product: ProductModel;
	created_at: Date;
	updated_at: Date;
	deleted_at: Date | null;
}

type SavedPaymentMethodPayload = components["schemas"]["SavedPaymentMethod"];

export interface SavedPaymentMethodModel {
	id: number;
	user_id: number;
	type: string;
	brand: string;
	last4: string;
	exp_month: number;
	exp_year: number;
	cardholder_name: string;
	nickname: string;
	is_default: boolean;
	created_at: Date;
	updated_at: Date;
	deleted_at: Date | null;
}

export function parseSavedPaymentMethod(
	paymentMethod: SavedPaymentMethodPayload
): SavedPaymentMethodModel {
	return {
		id: paymentMethod.id,
		user_id: paymentMethod.user_id,
		type: paymentMethod.type,
		brand: paymentMethod.brand,
		last4: paymentMethod.last4,
		exp_month: paymentMethod.exp_month,
		exp_year: paymentMethod.exp_year,
		cardholder_name: paymentMethod.cardholder_name,
		nickname: paymentMethod.nickname,
		is_default: paymentMethod.is_default,
		created_at: parseDate(paymentMethod.created_at) ?? new Date(),
		updated_at: parseDate(paymentMethod.updated_at) ?? new Date(),
		deleted_at: parseDate(paymentMethod.deleted_at),
	};
}

type SavedAddressPayload = components["schemas"]["SavedAddress"];

export interface SavedAddressModel {
	id: number;
	user_id: number;
	label: string;
	full_name: string;
	line1: string;
	line2: string;
	city: string;
	state: string;
	postal_code: string;
	country: string;
	phone: string;
	is_default: boolean;
	created_at: Date;
	updated_at: Date;
	deleted_at: Date | null;
}

export function parseSavedAddress(address: SavedAddressPayload): SavedAddressModel {
	return {
		id: address.id,
		user_id: address.user_id,
		label: address.label,
		full_name: address.full_name,
		line1: address.line1,
		line2: address.line2,
		city: address.city,
		state: address.state,
		postal_code: address.postal_code,
		country: address.country,
		phone: address.phone,
		is_default: address.is_default,
		created_at: parseDate(address.created_at) ?? new Date(),
		updated_at: parseDate(address.updated_at) ?? new Date(),
		deleted_at: parseDate(address.deleted_at),
	};
}
