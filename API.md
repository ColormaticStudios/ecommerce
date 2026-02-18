# Ecommerce API Documentation

## 1. Overview

This API powers the ecommerce backend server.

Base URL:

- Development: `http://localhost:3000/api/v1`
- Production: `https://api.example.com/api/v1` (replace with your deployment URL)

Authentication:

- **Public** endpoints require no auth.
- **User** endpoints require JWT: `Authorization: Bearer <token>`.
- **Admin** endpoints require JWT with role `admin`.

All responses use JSON. Errors are returned as `{ "error": "message" }` with an HTTP status code.

### 1.1 Conventions

- IDs are numeric and use `id`.
- Timestamps use `created_at`, `updated_at`, `deleted_at` (RFC3339 strings or `null`).
- All field names are `snake_case`.

### 1.2 Core Object Shapes

**User**

```json
{
	"id": 1,
	"subject": "abc123",
	"username": "johndoe",
	"email": "john@example.com",
	"name": "John Doe",
	"profile_photo_url": "https://.../media/...",
	"role": "customer",
	"currency": "USD",
	"created_at": "2024-01-01T12:00:00Z",
	"updated_at": "2024-01-01T12:00:00Z",
	"deleted_at": null
}
```

**Product**

```json
{
	"id": 1,
	"sku": "PROD-001",
	"name": "Product Name",
	"description": "Product description",
	"price": 29.99,
	"stock": 100,
	"images": ["https://.../media/..."],
	"cover_image": "https://.../media/...",
	"related_products": [
		{
			"id": 2,
			"sku": "PROD-002",
			"name": "Related",
			"description": "Related product description",
			"price": 19.99,
			"stock": 12,
			"cover_image": "https://.../media/..."
		}
	],
	"created_at": "2024-01-01T12:00:00Z",
	"updated_at": "2024-01-01T12:00:00Z",
	"deleted_at": null
}
```

**Cart**

```json
{
	"id": 1,
	"user_id": 1,
	"items": [
		{
			"id": 1,
			"cart_id": 1,
			"product_id": 1,
			"quantity": 2,
			"product": {
				"id": 1,
				"sku": "PROD-001",
				"name": "Product Name",
				"price": 29.99,
				"stock": 100,
				"images": [],
				"cover_image": "https://.../media/..."
			}
		}
	],
	"created_at": "2024-01-01T12:00:00Z",
	"updated_at": "2024-01-01T12:00:00Z",
	"deleted_at": null
}
```

**Order**

```json
{
	"id": 1,
	"user_id": 1,
	"status": "PENDING",
	"total": 59.98,
	"payment_method_display": "Visa •••• 4242",
	"shipping_address_pretty": "123 Main St, Portland, OR, 97201, US",
	"items": [
		{
			"id": 1,
			"order_id": 1,
			"product_id": 1,
			"quantity": 2,
			"price": 29.99,
			"product": {
				"id": 1,
				"sku": "PROD-001",
				"name": "Product Name",
				"price": 29.99,
				"stock": 100,
				"images": []
			}
		}
	],
	"created_at": "2024-01-01T12:00:00Z",
	"updated_at": "2024-01-01T12:00:00Z",
	"deleted_at": null
}
```

---

## 2. Authentication

Note: `/auth/register` and `/auth/login` can be disabled by setting `DISABLE_LOCAL_SIGN_IN` to `true`.

### 2.1 Register

**POST** `/auth/register`

Creates a new user account and returns a JWT.

**Request**

```json
{
	"username": "johndoe",
	"email": "john@example.com",
	"password": "password123",
	"name": "John Doe"
}
```

**Success Response** – HTTP 201

```json
{
	"token": "<jwt>",
	"user": {
		"id": 1,
		"username": "johndoe",
		"email": "john@example.com",
		"role": "customer",
		"currency": "USD"
	}
}
```

**Error Responses**

- HTTP 400 – Validation error
- HTTP 409 – Email or username already taken

### 2.2 Login

**POST** `/auth/login`

**Request**

```json
{ "email": "john@example.com", "password": "password123" }
```

**Success Response** – HTTP 200

```json
{ "token": "<jwt>", "user": { "id": 1, "username": "johndoe" } }
```

**Error Responses**

- HTTP 400 – Validation error
- HTTP 401 – Invalid credentials

### 2.3 OpenID Connect Login

**GET** `/auth/oidc/login`

**Query Params**

- `redirect` (optional)

**Success Response** – HTTP 302

### 2.4 OIDC Callback

**GET** `/auth/oidc/callback`

**Query Params**

- `code` (required)
- `state` (required)

**Success Response** – HTTP 200

```json
{ "token": "<jwt>", "user": { "id": 1, "username": "johndoe" } }
```

---

## 3. Products (Public)

### 3.1 List Products

**GET** `/products`

**Query Params**

- `q` (string) search by name
- `min_price` (number)
- `max_price` (number)
- `sort` (`price` | `name` | `created_at`)
- `order` (`asc` | `desc`)
- `page` (number, default 1)
- `limit` (number, default 20, max 100)

**Success Response** – HTTP 200

```json
{
	"data": [
		{
			"id": 1,
			"sku": "PROD-001",
			"name": "Product Name",
			"price": 29.99,
			"stock": 100,
			"images": []
		}
	],
	"pagination": { "page": 1, "limit": 20, "total": 1, "total_pages": 1 }
}
```

### 3.2 Get Product

**GET** `/products/{id}`

**Success Response** – HTTP 200

```json
{
	"id": 1,
	"sku": "PROD-001",
	"name": "Product Name",
	"price": 29.99,
	"stock": 100,
	"images": []
}
```

---

## 4. Profile (User)

### 4.1 Get Profile

**GET** `/me/`

**Success Response** – HTTP 200

```json
{ "id": 1, "username": "johndoe", "email": "john@example.com" }
```

### 4.2 Update Profile

**PATCH** `/me/`

**Request**

```json
{
	"name": "John Doe",
	"currency": "USD",
	"profile_photo_url": "https://.../media/..."
}
```

**Success Response** – HTTP 200

```json
{ "id": 1, "name": "John Doe", "currency": "USD" }
```

### 4.3 Saved Payment Methods

**GET** `/me/payment-methods`

Lists saved payment methods for the authenticated user.

**POST** `/me/payment-methods`

Creates a saved payment method (dummy, stores card metadata only).

**Request**

```json
{
	"cardholder_name": "Jane Doe",
	"card_number": "4242 4242 4242 4242",
	"exp_month": 12,
	"exp_year": 2030,
	"nickname": "Personal Visa",
	"set_default": true
}
```

**PATCH** `/me/payment-methods/{id}/default`

Sets the selected method as default.

**DELETE** `/me/payment-methods/{id}`

Deletes a saved payment method.

### 4.4 Saved Addresses

**GET** `/me/addresses`

Lists saved addresses for the authenticated user.

**POST** `/me/addresses`

Creates a saved address.

**Request**

```json
{
	"label": "Home",
	"full_name": "Jane Doe",
	"line1": "123 Main St",
	"line2": "Apt 4",
	"city": "Portland",
	"state": "OR",
	"postal_code": "97201",
	"country": "US",
	"phone": "+1-555-555-0100",
	"set_default": true
}
```

**PATCH** `/me/addresses/{id}/default`

Sets the selected address as default.

**DELETE** `/me/addresses/{id}`

Deletes a saved address.

---

## 5. Cart (User)

### 5.1 View Cart

**GET** `/me/cart`

**Success Response** – HTTP 200

```json
{
	"id": 1,
	"user_id": 1,
	"items": [
		{
			"id": 10,
			"cart_id": 1,
			"product_id": 2,
			"quantity": 2,
			"product": {
				"id": 2,
				"sku": "PROD-002",
				"name": "Product Name",
				"price": 29.99,
				"stock": 100,
				"images": []
			}
		}
	],
	"created_at": "2024-01-01T12:00:00Z",
	"updated_at": "2024-01-01T12:00:00Z",
	"deleted_at": null
}
```

### 5.2 Add Cart Item

**POST** `/me/cart`

**Request**

```json
{ "product_id": 1, "quantity": 2 }
```

**Success Response** – HTTP 200

```json
{
	"id": 1,
	"user_id": 1,
	"items": [
		{
			"id": 10,
			"cart_id": 1,
			"product_id": 1,
			"quantity": 2,
			"product": {
				"id": 1,
				"sku": "PROD-001",
				"name": "Product Name",
				"price": 29.99,
				"stock": 100,
				"images": []
			}
		}
	],
	"created_at": "2024-01-01T12:00:00Z",
	"updated_at": "2024-01-01T12:00:00Z",
	"deleted_at": null
}
```

### 5.3 Update Cart Item

**PATCH** `/me/cart/{itemId}`

**Request**

```json
{ "quantity": 3 }
```

**Success Response** – HTTP 200

```json
{
	"id": 10,
	"cart_id": 1,
	"product_id": 1,
	"quantity": 3,
	"product": {
		"id": 1,
		"sku": "PROD-001",
		"name": "Product Name",
		"price": 29.99,
		"stock": 100,
		"images": []
	},
	"created_at": "2024-01-01T12:00:00Z",
	"updated_at": "2024-01-01T12:00:00Z",
	"deleted_at": null
}
```

### 5.4 Remove Cart Item

**DELETE** `/me/cart/{itemId}`

---

## 6. Orders (User)

### 6.1 List Orders

**GET** `/me/orders`

**Query Params**

- `status` (`PENDING` | `PAID` | `FAILED`) optional
- `start_date` (`YYYY-MM-DD`) optional
- `end_date` (`YYYY-MM-DD`) optional (inclusive)
- `page` (number, default 1)
- `limit` (number, default 20, max 100)

**Success Response** – HTTP 200

```json
{
	"data": [
		{
			"id": 1,
			"user_id": 1,
			"status": "PENDING",
			"total": 59.98,
			"payment_method_display": "Visa •••• 4242",
			"shipping_address_pretty": "123 Main St, Portland, OR, 97201, US",
			"items": [
				{
					"id": 1,
					"order_id": 1,
					"product_id": 1,
					"quantity": 2,
					"price": 29.99,
					"product": {
						"id": 1,
						"sku": "PROD-001",
						"name": "Product Name",
						"price": 29.99,
						"stock": 100,
						"images": [],
						"cover_image": "https://.../media/..."
					}
				}
			],
			"created_at": "2024-01-01T12:00:00Z",
			"updated_at": "2024-01-01T12:00:00Z",
			"deleted_at": null
		}
	],
	"pagination": { "page": 1, "limit": 20, "total": 1, "total_pages": 1 }
}
```

### 6.2 Get Order

**GET** `/me/orders/{id}`

**Success Response** – HTTP 200

```json
{
	"id": 1,
	"user_id": 1,
	"status": "PENDING",
	"total": 59.98,
	"payment_method_display": "Visa •••• 4242",
	"shipping_address_pretty": "123 Main St, Portland, OR, 97201, US",
	"items": [
		{
			"id": 1,
			"order_id": 1,
			"product_id": 1,
			"quantity": 2,
			"price": 29.99,
			"product": {
				"id": 1,
				"sku": "PROD-001",
				"name": "Product Name",
				"price": 29.99,
				"stock": 100,
				"images": [],
				"cover_image": "https://.../media/..."
			}
		}
	],
	"created_at": "2024-01-01T12:00:00Z",
	"updated_at": "2024-01-01T12:00:00Z",
	"deleted_at": null
}
```

### 6.3 Create Order

**POST** `/me/orders`

**Request**

```json
{ "items": [{ "product_id": 1, "quantity": 2 }] }
```

**Success Response** – HTTP 201

```json
{
	"id": 1,
	"user_id": 1,
	"status": "PENDING",
	"total": 59.98,
	"payment_method_display": null,
	"shipping_address_pretty": null,
	"items": [
		{
			"id": 1,
			"order_id": 1,
			"product_id": 1,
			"quantity": 2,
			"price": 29.99,
			"product": {
				"id": 1,
				"sku": "PROD-001",
				"name": "Product Name",
				"price": 29.99,
				"stock": 100,
				"images": [],
				"cover_image": "https://.../media/..."
			}
		}
	],
	"created_at": "2024-01-01T12:00:00Z",
	"updated_at": "2024-01-01T12:00:00Z",
	"deleted_at": null
}
```

### 6.4 Process Payment (Mock)

**POST** `/me/orders/{id}/pay`

You can pay with:
- saved IDs (`payment_method_id`, `address_id`)
- inline one-time values (`payment_method`, `address`)
- omitted fields if defaults exist on account.

**Request (saved values)**

```json
{
	"payment_method_id": 3,
	"address_id": 7
}
```

**Request (one-time values)**

```json
{
	"payment_method": {
		"cardholder_name": "Jane Doe",
		"card_number": "4242424242424242",
		"exp_month": 12,
		"exp_year": 2030
	},
	"address": {
		"full_name": "Jane Doe",
		"line1": "123 Main St",
		"line2": "Apt 4",
		"city": "Portland",
		"state": "OR",
		"postal_code": "97201",
		"country": "US"
	}
}
```

**Success Response** – HTTP 200

```json
{
	"message": "Payment processed successfully",
	"order": {
		"id": 1,
		"user_id": 1,
		"status": "PAID",
		"total": 59.98,
		"payment_method_display": "Visa •••• 4242",
		"shipping_address_pretty": "123 Main St, Apt 4, Portland, OR, 97201, US",
		"items": [
			{
				"id": 1,
				"order_id": 1,
				"product_id": 1,
				"quantity": 2,
				"price": 29.99,
				"product": {
					"id": 1,
					"sku": "PROD-001",
					"name": "Product Name",
					"price": 29.99,
					"stock": 100,
					"images": [],
					"cover_image": "https://.../media/..."
				}
			}
		],
		"created_at": "2024-01-01T12:00:00Z",
		"updated_at": "2024-01-01T12:00:00Z",
		"deleted_at": null
	}
}
```

---

## 7. Admin: Products

### 7.1 Create Product

**POST** `/admin/products`

### 7.2 Update Product

**PATCH** `/admin/products/{id}`

### 7.3 Delete Product

**DELETE** `/admin/products/{id}`

### 7.4 Reorder Product Media

**PATCH** `/admin/products/{id}/media/order`

**Request**

```json
{ "media_ids": ["<media-id-1>", "<media-id-2>"] }
```

### 7.5 Update Related Products

**PATCH** `/admin/products/{id}/related`

**Request**

```json
{ "related_ids": [2, 3, 4] }
```

---

## 8. Admin: Orders

### 8.1 List Orders

**GET** `/admin/orders`

### 8.2 Get Order

**GET** `/admin/orders/{id}`

### 8.3 Update Order Status

**PATCH** `/admin/orders/{id}/status`

---

## 9. Admin: Users

### 9.1 List Users

**GET** `/admin/users`

### 9.2 Update User Role

**PATCH** `/admin/users/{id}/role`

**Request**

```json
{ "role": "admin" }
```

---

## 10. Media Uploads

Uploads use the TUS resumable protocol at `/media/uploads`. Clients must send `Authorization: Bearer <token>` on all requests.

### 10.1 Create Upload (TUS)

**POST** `/media/uploads`

Headers:

- `Tus-Resumable: 1.0.0`
- `Upload-Length: <bytes>`
- `Upload-Metadata: filename <base64>`

**Success Response** – HTTP 201
`Location` header includes the upload URL. The upload ID is the last path segment.

### 10.2 Attach Profile Photo

**POST** `/me/profile-photo`

**Request**

```json
{ "media_id": "<tus-upload-id>" }
```

**Error Responses**

- HTTP 400 – Media is not an image
- HTTP 409 – Media is still processing
- HTTP 413 – Profile photo too large

### 10.3 Remove Profile Photo

**DELETE** `/me/profile-photo`

### 10.4 Attach Product Media (Admin)

**POST** `/admin/products/{id}/media`

**Request**

```json
{ "media_ids": ["<tus-upload-id>"] }
```

### 10.5 Detach Product Media (Admin)

**DELETE** `/admin/products/{id}/media/{mediaId}`
