# Ecommerce API Documentation

## 1. Overview
This API powers a simple e-commerce backend server.

Endpoints are grouped by role:
- **Public** – no authentication required.
- **User** – authenticated via JWT (`Authorization: Bearer <token>`).
- **Admin** – authenticated + role `"admin"`.

All responses use JSON. Errors are returned with an `error` field and an HTTP status code.

---

## 2. Authentication

Note: `/auth/register` and `/auth/login` can be disabled by setting `DISABLE_LOCAL_SIGN_IN` to `true`.

### 2.1 Register
**POST** `/auth/register`
Creates a new user account and returns a JWT.

| Field | Type | Validation | Description |
|-------|------|------------|-------------|
| `username` | string | required, 3‑50 chars | User’s login name |
| `email` | string | required, email format | Account email |
| `password` | string | required, min 6 chars | Plain‑text password |
| `name` | string | optional | Full name |

**Request Example**
```/dev/null/auth-register#L1-10
{
  "username": "johndoe",
  "email": "john@example.com",
  "password": "password123",
  "name": "John Doe"
}
```

**Success Response** – HTTP 201
```/dev/null/auth-success#L1-10
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": 1,
    "subject": "abc123",
    "username": "johndoe",
    "email": "john@example.com",
    "name": "John Doe",
    "role": "customer",
    "currency": "USD",
    "profile_photo_url": null
  }
}
```

**Error Responses**
- HTTP 400 – Validation error
- HTTP 409 – Email or username already taken

---

### 2.2 Login
**POST** `/auth/login`
Returns a JWT if credentials are correct.

| Field | Type | Validation | Description |
|-------|------|------------|-------------|
| `email` | string | required, email format | Account email |
| `password` | string | required | Plain‑text password |

**Request Example**
```/dev/null/auth-login#L1-10
{
  "email": "john@example.com",
  "password": "password123"
}
```

**Success Response** – HTTP 200
```/dev/null/auth-success#L1-10
{
  "token": "...",
  "user": { ... }
}
```

**Error Responses**
- HTTP 400 – Validation error
- HTTP 401 – Invalid credentials

---

### 2.3 OpenID Connect Login
**GET** `/auth/oidc/login`
Initiates OIDC authentication flow.

| Query Param | Type | Default | Description |
|-------------|------|---------|-------------|
| `redirect` | string | – | Redirect URL after successful login |

**Success Response** – HTTP 302
Redirects to OIDC provider login page.

**Error Responses**
- HTTP 400 – Validation error

---

### 2.4 OIDC Callback
**GET** `/auth/oidc/callback`
Handles OIDC callback and returns JWT.

| Query Param | Type | Default | Description |
|-------------|------|---------|-------------|
| `code` | string | – | OIDC authorization code |
| `state` | string | – | State parameter from initial request |

**Success Response** – HTTP 200
```/dev/null/auth-success#L1-10
{
  "token": "...",
  "user": { ... }
}
```

**Error Responses**
- HTTP 400 – Validation error
- HTTP 401 – Invalid credentials

---

## 3. Products

### 3.1 List Products
**GET** `/products`
Public – paginated, searchable, filterable, sortable.

| Query Param | Type | Default | Description |
|-------------|------|---------|-------------|
| `q` | string | – | Search term (case‑insensitive on `name`) |
| `min_price` | float | – | Minimum price filter |
| `max_price` | float | – | Maximum price filter |
| `sort` | string | `created_at` | `price`, `name`, or `created_at` |
| `order` | string | `desc` | `asc` or `desc` |
| `page` | int | `1` | Page number |
| `limit` | int | `20` | Items per page (max 100) |

**Success Response** – HTTP 200
```/dev/null/products-list#L1-15
{
  "data": [
    {
      "id": 1,
      "sku": "PROD-001",
      "name": "Sample Product",
      "description": "A great item",
      "price": 29.99,
      "stock": 100,
      "images": ["https://example.com/img1.jpg"]
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 1,
    "total_pages": 1
  }
}
```

---

### 3.2 Get Product Details
**GET** `/products/:id`
Public – returns a single product and its related products.

**Success Response** – HTTP 200
```/dev/null/product-detail#L1-15
{
  "id": 1,
  "sku": "PROD-001",
  "name": "Sample Product",
  "description": "A great item",
  "price": 29.99,
  "stock": 100,
  "images": ["https://example.com/img1.jpg"],
  "related_products": [
    {
      "id": 2,
      "sku": "PROD-002",
      "name": "Related Item"
    }
  ]
}
```

---

## 4. Cart (Authenticated User)

All cart routes require the `Authorization` header.

### 4.1 View Cart
**GET** `/me/cart`

**Success Response** – HTTP 200
```/dev/null/cart-view#L1-20
{
  "id": 1,
  "user_id": 1,
  "items": [
    {
      "id": 5,
      "product_id": 1,
      "quantity": 2,
      "product": {
        "id": 1,
        "sku": "PROD-001",
        "name": "Sample Product",
        "price": 29.99
      }
    }
  ]
}
```

---

### 4.2 Add Item
**POST** `/me/cart`

| Field | Type | Validation | Description |
|-------|------|------------|-------------|
| `product_id` | integer | required | Product to add |
| `quantity` | integer | required, ≥1 | Number of units |

**Request Example**
```/dev/null/cart-add#L1-10
{
  "product_id": 1,
  "quantity": 2
}
```

**Success Response** – HTTP 200
```/dev/null/cart-view#L1-20
{ ... }   // Same format as GET /me/cart
```

**Error Responses**
- HTTP 400 – Validation error or insufficient stock
- HTTP 404 – Product not found

---

### 4.3 Update Item Quantity
**PATCH** `/me/cart/:itemId`

| Field | Type | Validation | Description |
|-------|------|------------|-------------|
| `quantity` | integer | required, ≥1 | New quantity |

**Request Example**
```/dev/null/cart-update#L1-10
{
  "quantity": 3
}
```

**Success Response** – HTTP 200
```/dev/null/cart-item#L1-15
{
  "id": 5,
  "product_id": 1,
  "quantity": 3,
  "product": { ... }
}
```

---

### 4.4 Remove Item
**DELETE** `/me/cart/:itemId`

**Success Response** – HTTP 200
```/dev/null/cart-delete#L1-5
{ "message": "Cart item deleted successfully" }
```

---

## 5. Orders (Authenticated User)

### 5.1 List Orders
**GET** `/me/orders`

**Success Response** – HTTP 200
```/dev/null/orders-list#L1-25
[
  {
    "id": 10,
    "user_id": 1,
    "status": "PENDING",
    "total": 59.98,
    "created_at": "2024-01-01T12:00:00Z",
    "items": [
      {
        "id": 101,
        "product_id": 1,
        "quantity": 2,
        "price": 29.99,
        "product": { ... }
      }
    ]
  }
]
```

---

### 5.2 Order Details
**GET** `/me/orders/:id`

**Success Response** – HTTP 200
```/dev/null/order-detail#L1-25
{ ... }   // Same format as list item
```

---

### 5.3 Create Order
**POST** `/me/orders`

| Field | Type | Validation | Description |
|-------|------|------------|-------------|
| `items` | array of objects | required | List of products |
| `items[].product_id` | integer | required | Product ID |
| `items[].quantity` | integer | required, ≥1 | Quantity |

**Request Example**
```/dev/null/order-create#L1-15
{
  "items": [
    { "product_id": 1, "quantity": 2 },
    { "product_id": 3, "quantity": 1 }
  ]
}
```

**Success Response** – HTTP 201
```/dev/null/order-detail#L1-25
{ ... }   // Order object
```

**Error Responses**
- HTTP 400 – Validation error or insufficient stock

---

### 5.4 Process Payment
**POST** `/me/orders/:id/pay`

The API mocks payment processing and sets the order status to `PAID`.

**Success Response** – HTTP 200
```/dev/null/payment-success#L1-15
{
  "message": "Payment processed successfully",
  "order": { ... }   // Updated order with status PAID
}
```

---

## 6. User Profile (Authenticated User)

### 6.1 Get Profile
**GET** `/me/`

**Success Response** – HTTP 200
```/dev/null/user-profile#L1-15
{
  "id": 1,
  "subject": "abc123",
  "username": "johndoe",
  "email": "john@example.com",
  "name": "John Doe",
  "role": "customer",
  "currency": "USD",
  "profile_photo_url": null
}
```

### 6.2 Update Profile
**PATCH** `/me/`

| Field | Type | Validation | Description |
|-------|------|------------|-------------|
| `name` | string | optional | New name |
| `currency` | string | optional, 3‑letter code | Preferred currency |
| `profile_photo_url` | string | optional | URL to profile picture |

**Request Example**
```/dev/null/user-update#L1-10
{
  "name": "Johnny",
  "currency": "EUR"
}
```

**Success Response** – HTTP 200
```/dev/null/user-profile#L1-15
{ ... }   // Updated user object
```

---

## 7. Admin Operations

All admin routes require the `Authorization` header with a user whose role is `"admin"`.

### 7.1 Create Product
**POST** `/admin/products`

| Field | Type | Validation | Description |
|-------|------|------------|-------------|
| `sku` | string | required | Unique product code |
| `name` | string | required | Product name |
| `description` | string | optional |
| `price` | float | required, >0 |
| `stock` | integer | optional, defaults to 0 |
| `images` | array of strings | optional | URLs |

**Request Example**
```/dev/null/admin-product-create#L1-15
{
  "sku": "PROD-001",
  "name": "New Item",
  "description": "High quality",
  "price": 49.99,
  "stock": 50,
  "images": ["https://example.com/img.jpg"]
}
```

**Success Response** – HTTP 201
```/dev/null/admin-product-detail#L1-15
{ ... }   // Created product object
```

---

### 7.2 Update Product
**PATCH** `/admin/products/:id`

Accepts the same fields as create. Fields omitted are unchanged.

### 7.3 Delete Product
**DELETE** `/admin/products/:id`

**Success Response** – HTTP 200
```/dev/null/admin-delete#L1-5
{ "message": "Product deleted successfully" }
```

---

### 7.4 List Orders
**GET** `/admin/orders`

Returns all orders with details.

### 7.5 Order Detail
**GET** `/admin/orders/:id`

### 7.6 Update Order Status
**PATCH** `/admin/orders/:id/status`

| Field | Type | Validation | Description |
|-------|------|------------|-------------|
| `status` | string | required, one of `PENDING`, `PAID`, `FAILED` |

**Request Example**
```/dev/null/admin-order-status#L1-5
{ "status": "PAID" }
```

### 7.7 List Users
**GET** `/admin/users`

### 7.8 Update User Role
**PATCH** `/admin/users/:id/role`

| Field | Type | Validation | Description |
|-------|------|------------|-------------|
| `role` | string | required, `admin` or `customer` |

**Request Example**
```/dev/null/admin-user-role#L1-5
{ "role": "admin" }
```

---

## 8. Error Handling

All error responses contain:
- `error`: human‑readable message
- Optional `field` or `fields` for validation errors

```/dev/null/error-response#L1-5
{ "error": "Invalid email or password" }
```

---

## 9. Authentication Token Structure

The JWT issued contains:

| Claim | Description |
|-------|-------------|
| `sub` | User’s unique subject ID |
| `email` | User’s email |
| `role` | `"admin"` or `"customer"` |
| `name` | User’s display name |
| `exp` | Expiration (7 days) |
| `iat` | Issued at |

Sample payload (decoded base64):
```/dev/null/jwt-payload#L1-10
{
  "sub": "abc123",
  "email": "john@example.com",
  "role": "customer",
  "name": "John Doe",
  "exp": 1700000000,
  "iat": 1699405200
}
```

---

## 10. Media Uploads

Uploads use the TUS resumable protocol at `/media/uploads`. Clients must send `Authorization: Bearer <token>` on all requests.

### Create Upload (TUS)
**POST** `/media/uploads`

Headers:
- `Tus-Resumable: 1.0.0`
- `Upload-Length: <bytes>`
- `Upload-Metadata: filename <base64>`

**Success Response** – HTTP 201
`Location` header includes the upload URL. The upload ID is the last path segment.

### Attach Profile Photo
**POST** `/me/profile-photo`

| Field | Type | Validation | Description |
|-------|------|------------|-------------|
| `media_id` | string | required | Upload ID from TUS |

**Success Response** – HTTP 200
Returns the user with `profile_photo_url` set.

**Error Responses**
- HTTP 400 – Media is not an image
- HTTP 409 – Media is still processing
- HTTP 413 – Profile photo too large

### Remove Profile Photo
**DELETE** `/me/profile-photo`

**Success Response** – HTTP 200

### Attach Product Media (Admin)
**POST** `/admin/products/{id}/media`

| Field | Type | Validation | Description |
|-------|------|------------|-------------|
| `media_ids` | array | required | Upload IDs from TUS |

**Success Response** – HTTP 200
Returns the product with `images` set from attached media.

### Detach Product Media (Admin)
**DELETE** `/admin/products/{id}/media/{mediaId}`

**Success Response** – HTTP 200

---

## 11. Rate Limiting & CORS

- **Rate limit:** 100 requests per second per IP (enforced by tollbooth).
- **CORS:** All origins allowed (`*`). In production, restrict to your front‑end domains.

---

## 12. Sample cURL Commands

**Register**
```/dev/null/curl-register#L1-7
curl -X POST https://api.example.com/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"jane","email":"jane@example.com","password":"secret","name":"Jane"}'
```

**Login**
```/dev/null/curl-login#L1-6
curl -X POST https://api.example.com/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"jane@example.com","password":"secret"}'
```

**Get Products**
```/dev/null/curl-get-products#L1-5
curl https://api.example.com/products?page=1&limit=20
```

**Add to Cart (authenticated)**
```/dev/null/curl-add-cart#L1-7
curl -X POST https://api.example.com/me/cart \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"product_id":1,"quantity":2}'
```

**Create Order (authenticated)**
```/dev/null/curl-create-order#L1-9
curl -X POST https://api.example.com/me/orders \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"items":[{"product_id":1,"quantity":2}]}'
```

**Admin Create Product**
```/dev/null/curl-admin-create-product#L1-9
curl -X POST https://api.example.com/admin/products \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{"sku":"PROD-002","name":"Admin Item","price":19.99}'
```

**Admin Update Product**
```/dev/null/curl-admin-update-product#L1-9
curl -X PUT https://api.example.com/admin/products/1 \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{"name":"Updated Admin Item","price":29.99}'
```

**Admin Delete Product**
```/dev/null/curl-admin-delete-product#L1-7
curl -X DELETE https://api.example.com/admin/products/1 \
  -H "Authorization: Bearer <admin-token>"
```
