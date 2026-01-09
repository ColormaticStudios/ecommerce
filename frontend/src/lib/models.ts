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
