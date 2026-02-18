export interface UserModel {
	id: number;
	subject: string;
	username: string;
	email: string;
	name: string | null;
	role: string;
	currency: string;
	profile_photo_url: string | null;
	created_at: Date;
	updated_at: Date;
	deleted_at: Date | null;
}

export interface ProfileModel {
	id: number;
	subject: string;
	username: string;
	email: string;
	name: string | null;
	role: string;
	currency: string;
	profile_photo_url: string | null;
	created_at: string | Date;
	updated_at: string | Date;
	deleted_at: string | Date | null;
}

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

interface RelatedProductPayload {
	id: number;
	sku: string;
	name: string;
	description?: string | null;
	price?: number;
	cover_image?: string | null;
	stock?: number;
}

interface ProductPayload extends Omit<
	ProductModel,
	"created_at" | "updated_at" | "deleted_at" | "related_products" | "images"
> {
	created_at: string | Date;
	updated_at: string | Date;
	deleted_at: string | Date | null;
	related_products?: RelatedProductPayload[];
	images?: string[];
	cover_image?: string;
}

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
		id: product.id,
		sku: product.sku,
		name: product.name,
		description: product.description,
		price: product.price,
		stock: product.stock,
		images: product.images ?? [],
		cover_image: coverImage ?? undefined,
		related_products: (product.related_products ?? []).map(parseRelatedProduct),
	};
}

export interface ProductModel {
	created_at: Date;
	deleted_at: Date | null;
	updated_at: Date;
	id: number;
	sku: string;
	name: string;
	description: string;
	price: number;
	stock: number;
	images: string[];
	cover_image?: string;
	related_products: RelatedProductModel[];
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

interface CartItemPayload extends Omit<
	CartItemModel,
	"created_at" | "updated_at" | "deleted_at" | "product"
> {
	product: ProductPayload;
	created_at: string | Date;
	updated_at: string | Date;
	deleted_at: string | Date | null;
}

interface CartPayload extends Omit<
	CartModel,
	"created_at" | "updated_at" | "deleted_at" | "items"
> {
	items?: CartItemPayload[];
	created_at: string | Date;
	updated_at: string | Date;
	deleted_at: string | Date | null;
}

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
		product_id: cartItem.product_id,
		quantity: cartItem.quantity,
		product: parseProduct(cartItem.product),
		created_at: parseDate(cartItem.created_at) ?? new Date(),
		updated_at: parseDate(cartItem.updated_at) ?? new Date(),
		deleted_at: parseDate(cartItem.deleted_at),
	};
}

export interface CartItemModel {
	id: number;
	cart_id: number;
	product_id: number;
	quantity: number;
	product: ProductModel;
	created_at: Date;
	updated_at: Date;
	deleted_at: Date | null;
}

interface OrderItemPayload extends Omit<
	OrderItemModel,
	"created_at" | "updated_at" | "deleted_at" | "product"
> {
	product: ProductPayload;
	created_at: string | Date;
	updated_at: string | Date;
	deleted_at: string | Date | null;
}

export interface OrderPayload extends Omit<
	OrderModel,
	"created_at" | "updated_at" | "deleted_at" | "items"
> {
	items?: OrderItemPayload[];
	payment_method_display: string | null;
	shipping_address_pretty: string | null;
	created_at: string | Date;
	updated_at: string | Date;
	deleted_at: string | Date | null;
}

export function parseOrder(order: OrderPayload): OrderModel {
	return {
		id: order.id,
		user_id: order.user_id,
		status: order.status,
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
	user_id: number;
	status: "PENDING" | "PAID" | "FAILED";
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
		product_id: orderItem.product_id,
		quantity: orderItem.quantity,
		price: orderItem.price,
		product: parseProduct(orderItem.product),
		created_at: parseDate(orderItem.created_at) ?? new Date(),
		updated_at: parseDate(orderItem.updated_at) ?? new Date(),
		deleted_at: parseDate(orderItem.deleted_at),
	};
}

export interface OrderItemModel {
	id: number;
	order_id: number;
	product_id: number;
	quantity: number;
	price: number;
	product: ProductModel;
	created_at: Date;
	updated_at: Date;
	deleted_at: Date | null;
}

interface SavedPaymentMethodPayload extends Omit<
	SavedPaymentMethodModel,
	"created_at" | "updated_at" | "deleted_at"
> {
	created_at: string | Date;
	updated_at: string | Date;
	deleted_at: string | Date | null;
}

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

interface SavedAddressPayload extends Omit<
	SavedAddressModel,
	"created_at" | "updated_at" | "deleted_at"
> {
	created_at: string | Date;
	updated_at: string | Date;
	deleted_at: string | Date | null;
}

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
