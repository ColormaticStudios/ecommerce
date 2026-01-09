export interface UserModel {
	id: number;
	subject: string;
	username: string;
	email: string;
	name: string | null;
	role: string;
	currency: string;
	profile_photo_url: string | null;
}

// Why is ProfileModel different from UserModel? No idea.
export interface ProfileModel {
	ID: number;
	Subject: string;
	Username: string;
	Email: string;
	name: string | null;
	role: string;
	currency: string;
	profile_photo_url: string | null;
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

// The argument `product` is technically not a product, the dates are strings. It's close enough though.
export function parseProduct(product: ProductModel): ProductModel {
	return {
		CreatedAt: new Date(product.CreatedAt),
		DeletedAt: new Date(product.DeletedAt),
		UpdatedAt: new Date(product.UpdatedAt),
		ID: product.ID,
		sku: product.sku,
		name: product.name,
		description: product.description,
		price: product.price,
		stock: product.stock,
		images: product.images,
		related_products: product.related_products,
	};
}

export interface ProductModel {
	CreatedAt: Date;
	DeletedAt: Date;
	UpdatedAt: Date;
	ID: number;
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
