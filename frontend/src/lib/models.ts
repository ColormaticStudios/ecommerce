export interface UserModel {
	id: number;
	subject: string;
	username: string;
	email: string;
	name: string | null;
	role: string;
	currency: string;
	profile_photo_url: string | null;
	created_at?: Date;
	updated_at?: Date;
	deleted_at?: Date | null;
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
	created_at?: string | Date;
	updated_at?: string | Date;
	deleted_at?: string | Date | null;
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
	ID?: number;
	id?: number;
	SKU?: string;
	sku?: string;
	Name?: string;
	name?: string;
	Price?: number;
	price?: number;
}

interface ProductPayload
	extends Omit<ProductModel, "created_at" | "updated_at" | "deleted_at" | "related_products"> {
	created_at: string | Date;
	updated_at: string | Date;
	deleted_at: string | Date | null;
	related_products?: RelatedProductPayload[];
}

function parseRelatedProduct(product: RelatedProductPayload): RelatedProductModel {
	return {
		id: product.id ?? product.ID ?? 0,
		sku: product.sku ?? product.SKU ?? "",
		name: product.name ?? product.Name ?? "",
		price: product.price ?? product.Price,
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
		images: product.images,
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
	related_products: RelatedProductModel[];
}

export interface RelatedProductModel {
	id: number;
	sku: string;
	name: string;
	price?: number;
}

export function parseCart(cart: CartModel): CartModel {
	return {
		id: cart.id,
		user_id: cart.user_id,
		items: cart.items.map(parseCartItem),
	};
}

export interface CartModel {
	id: number;
	user_id: number;
	items: CartItemModel[];
}

export function parseCartItem(cartItem: CartItemModel): CartItemModel {
	return {
		id: cartItem.id,
		product_id: cartItem.product_id,
		quantity: cartItem.quantity,
		product: parseProduct(cartItem.product),
	};
}

export interface CartItemModel {
	id: number;
	product_id: number;
	quantity: number;
	product: ProductModel;
}

export function parseOrder(order: OrderModel): OrderModel {
	return {
		id: order.id,
		user_id: order.user_id,
		status: order.status,
		total: order.total,
		created_at: new Date(order.created_at),
		items: order.items.map(parseOrderItem),
	};
}

export interface OrderModel {
	id: number;
	user_id: number;
	status: "PENDING" | "PAID" | "FAILED";
	total: number;
	created_at: Date;
	items: OrderItemModel[];
}

export function parseOrderItem(orderItem: OrderItemModel): OrderItemModel {
	return {
		id: orderItem.id,
		product_id: orderItem.product_id,
		quantity: orderItem.quantity,
		price: orderItem.price,
		product: parseProduct(orderItem.product),
	};
}

export interface OrderItemModel {
	id: number;
	product_id: number;
	quantity: number;
	price: number;
	product: ProductModel;
}
